import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { filterQuestions, QuestionHistory, relativeTime } from './QuestionHistory.jsx'
import { clearWorkspaceStateForTests } from '../lib/workspaceState.js'

const question = (id, query, over = {}) => ({ id, query, label: '', state: 'answered', pinned: false, updatedAt: new Date(Date.now() - 12 * 60_000).toISOString(), ...over })
const seed = (caseId, entries) => localStorage.setItem(`nexusai.viewer.questions.${caseId}`, JSON.stringify(entries))
const show = () => render(<MemoryRouter><QuestionHistory caseIds={['case-a', 'case-b']} /></MemoryRouter>)

afterEach(() => { cleanup(); localStorage.clear(); clearWorkspaceStateForTests() })

describe('relative time', () => {
  const now = Date.parse('2026-09-29T12:00:00Z')
  it('is short and never in the future', () => {
    expect(relativeTime('2026-09-29T11:59:40Z', now)).toBe('just now')
    expect(relativeTime('2026-09-29T11:48:00Z', now)).toBe('12m ago')
    expect(relativeTime('2026-09-29T07:00:00Z', now)).toBe('5h ago')
    expect(relativeTime('2026-09-27T12:00:00Z', now)).toBe('2d ago')
    expect(relativeTime('2026-09-29T12:30:00Z', now)).toBe('just now')
    expect(relativeTime('not a date', now)).toBe('')
  })
})

describe('filterQuestions', () => {
  const entries = [{ query: 'Who called most?', label: 'Top caller', caseId: 'case-a' }, { query: 'List vehicles', label: '', caseId: 'case-b' }]
  it('matches the question, its label or its case, and returns everything for an empty filter', () => {
    expect(filterQuestions(entries, '')).toHaveLength(2)
    expect(filterQuestions(entries, 'top')).toHaveLength(1)
    expect(filterQuestions(entries, 'CASE-B')).toHaveLength(1)
    expect(filterQuestions(entries, 'zzz')).toHaveLength(0)
  })
})

describe('QuestionHistory', () => {
  it('shows an honest empty state when nothing has been asked', () => {
    show()
    expect(screen.getByText(/Questions you ask are saved here/)).toBeTruthy()
    expect(screen.queryByRole('searchbox')).toBeNull()
  })

  it('lists pinned then recent questions from every case, with case and time, and says it is browser-local', () => {
    seed('case-a', [question('q1', 'Who called most often?', { pinned: true })])
    seed('case-b', [question('q2', 'List all vehicles')])
    show()
    const region = screen.getByRole('region', { name: 'Your questions' })
    expect(within(region).getByText('Saved in this browser only')).toBeTruthy()
    const pinned = within(region).getByRole('region', { name: 'Pinned' })
    expect(pinned.textContent).toContain('Who called most often?')
    const recent = within(region).getByRole('region', { name: 'Recent' })
    expect(recent.textContent).toContain('List all vehicles')
    expect(recent.textContent).toContain('case-b')
    expect(recent.textContent).toContain('12m ago')
    expect(within(recent).getByRole('link').getAttribute('href')).toBe('/cases/case-b/investigate?question=List%20all%20vehicles')
  })

  it('filters only the saved questions and says so when nothing matches', async () => {
    seed('case-a', [question('q1', 'Who called most often?'), question('q2', 'List all vehicles')])
    show()
    const user = userEvent.setup()
    await user.type(screen.getByRole('searchbox', { name: 'Filter your questions' }), 'vehicles')
    expect(screen.queryByText('Who called most often?')).toBeNull()
    expect(screen.getByText('List all vehicles')).toBeTruthy()
    await user.clear(screen.getByRole('searchbox', { name: 'Filter your questions' }))
    await user.type(screen.getByRole('searchbox', { name: 'Filter your questions' }), 'nothing here')
    expect(screen.getByRole('status').textContent).toMatch(/Only your saved questions were searched/)
  })

  it('pins with a visible toggle button that reports its state, moving the question to Pinned', async () => {
    seed('case-a', [question('q2', 'List all vehicles')])
    show()
    const user = userEvent.setup()
    const pin = screen.getByRole('button', { name: 'Pin question: List all vehicles' })
    expect(pin.getAttribute('aria-pressed')).toBe('false')
    await user.click(pin)
    expect(screen.getByRole('region', { name: 'Pinned' }).textContent).toContain('List all vehicles')
    expect(screen.getByRole('button', { name: 'Unpin question: List all vehicles' }).getAttribute('aria-pressed')).toBe('true')
  })

  it('renames a question and can cancel', async () => {
    seed('case-a', [question('q2', 'List all vehicles')])
    show()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Rename question: List all vehicles' }))
    await user.click(screen.getByRole('button', { name: 'Cancel rename' }))
    expect(screen.getByText('List all vehicles')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Rename question: List all vehicles' }))
    const input = screen.getByLabelText('Rename question')
    await user.clear(input)
    await user.type(input, 'Vehicles')
    await user.click(screen.getByRole('button', { name: 'Save question name' }))
    expect(screen.getByText('Vehicles')).toBeTruthy()
  })

  it('shows eight recent questions first and reveals the rest on request', async () => {
    seed('case-a', Array.from({ length: 11 }, (_, index) => question(`q${index}`, `Question number ${index}`, { updatedAt: new Date(Date.now() - index * 60_000).toISOString() })))
    show()
    const user = userEvent.setup()
    expect(screen.getAllByRole('link')).toHaveLength(8)
    const more = screen.getByRole('button', { name: 'Show all 11' })
    expect(more.getAttribute('aria-expanded')).toBe('false')
    await user.click(more)
    expect(screen.getAllByRole('link')).toHaveLength(11)
    expect(screen.getByRole('button', { name: 'Show fewer' }).getAttribute('aria-expanded')).toBe('true')
  })
})
