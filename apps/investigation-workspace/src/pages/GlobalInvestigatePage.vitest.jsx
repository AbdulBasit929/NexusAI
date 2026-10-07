import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import GlobalInvestigatePage, { searchScopeNote } from './GlobalInvestigatePage.jsx'
import laneAbstain from '../lib/__fixtures__/lane_abstain.json'

function Landing() {
  const location = useLocation()
  return <p data-testid="landing">{location.pathname}{location.search}</p>
}

const summaries = {
  alpha: { evidence_total: 4, evidence_completed: 4, evidence_in_flight: 0, evidence_failed: 0 },
  bravo: { evidence_total: 10, evidence_completed: 8, evidence_in_flight: 0, evidence_failed: 2 },
  empty: { evidence_total: 0, evidence_completed: 0, evidence_in_flight: 0, evidence_failed: 0 },
  down: { evidence_total: 3, evidence_completed: 3, evidence_in_flight: 0, evidence_failed: 0 },
}
const answers = {
  alpha: { enterprise: { executive_answer: 'The number appears 12 times in this case.', result_state: 'complete' } },
  bravo: { enterprise: { executive_answer: '', result_state: 'no_match' } },
  // What the governed SQL lane sends when it declines to answer a question it could not verify.
  abstained: laneAbstain,
}
const json = body => ({ ok: true, status: 200, headers: { get: () => null }, json: async () => body })

function open(caseIds, { asked = [] } = {}) {
  window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds }
  globalThis.fetch = vi.fn(async (url, init = {}) => {
    const path = new URL(String(url)).pathname
    const caseId = init.headers?.['X-Forensic-Collection-ID'] || new URL(String(url)).searchParams.get('collection_id')
    if (path.endsWith('/collections/status')) return json({ summary: summaries[caseId] || summaries.alpha, record_families: [], recent_jobs: [], recent_evidence: [], missing_kb_assets: [] })
    if (path.endsWith('/query/capabilities')) return json({})
    if (path.endsWith('/query/hybrid')) {
      asked.push(caseId)
      if (caseId === 'down') return { ok: false, status: 500, headers: { get: () => 'REQ-9' }, json: async () => ({}) }
      return json(answers[caseId] || answers.alpha)
    }
    return json({})
  })
  return render(<MemoryRouter initialEntries={['/investigate']}><Routes><Route path="/investigate" element={<GlobalInvestigatePage />} /><Route path="/cases/:id/investigate" element={<Landing />} /></Routes></MemoryRouter>)
}

afterEach(() => {
  vi.restoreAllMocks()
  delete window.__INVESTIGATION_WORKSPACE_CONFIG__
})

describe('scope wording', () => {
  it('states exactly which cases and sources a question will search', () => {
    expect(searchScopeNote({ searching: 2, skipped: 1, sources: 14, ready: 12 })).toBe('Searching 2 cases, 1 skipped: 12 of 14 sources are ready; the rest are not searched until they finish.')
    expect(searchScopeNote({ searching: 1, skipped: 0, sources: 4, ready: 4 })).toBe('Searching 1 case: all 4 sources are ready.')
    expect(searchScopeNote({ searching: 0, skipped: 2, sources: 0, ready: 0 })).toBe('No case has evidence to search yet.')
  })
})

describe('GlobalInvestigatePage', () => {
  it('needs no case to be chosen: one question is asked of every case with evidence', async () => {
    const asked = []
    open(['alpha', 'bravo', 'empty'], { asked })
    const user = userEvent.setup()
    expect(await screen.findByText(/Searching 2 cases, 1 skipped/)).toBeTruthy()
    expect(screen.queryByRole('radio')).toBeNull()
    await user.type(screen.getByLabelText('Your question'), 'Who contacted 0300?')
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    expect(await screen.findByRole('heading', { name: 'Found in 1 of 2 cases.' })).toBeTruthy()
    expect([...asked].sort()).toEqual(['alpha', 'bravo'])
    expect(screen.getByText(/Not searched: empty \(no evidence yet\)/)).toBeTruthy()
  })

  it('shows each case with its own outcome, best first, and the coverage of each case', async () => {
    open(['bravo', 'alpha'])
    const user = userEvent.setup()
    await screen.findByText(/Searching 2 cases/)
    await user.type(screen.getByLabelText('Your question'), 'Who contacted 0300?')
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    await screen.findByRole('heading', { name: 'Found in 1 of 2 cases.' })
    const items = [...document.querySelectorAll('.ax-case')]
    expect(items.map(item => item.querySelector('.ax-case__name').textContent)).toEqual(['alpha', 'bravo'])
    expect(items[0].textContent).toContain('The number appears 12 times in this case.')
    expect(items[0].querySelector('.ax-case__coverage').textContent).toBe('4 of 4 sources ready')
    expect(items[1].querySelector('.ax-case__coverage').textContent).toBe('8 of 10 sources ready')
    expect(items[1].querySelector('.ax-case__state').textContent).toBe('No matches')
    const chips = within(screen.getByRole('list', { name: 'Outcome by case' }))
    expect(chips.getByText('Answered')).toBeTruthy()
    expect(chips.getByText('No matches')).toBeTruthy()
  })

  it('names a case that could not be searched, and the others still answer', async () => {
    open(['alpha', 'down'])
    const user = userEvent.setup()
    await screen.findByText(/Searching 2 cases/)
    await user.type(screen.getByLabelText('Your question'), 'Anything?')
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    await screen.findByRole('heading', { name: 'Found in 1 of 2 cases.' })
    const down = [...document.querySelectorAll('.ax-case')].find(item => item.textContent.includes('down'))
    expect(down.querySelector('.ax-case__state').textContent).toBe('Could not search')
  })

  it('shows a lane abstention as a case that needs a choice, with its reason, and the page stays up', async () => {
    open(['alpha', 'abstained'])
    const user = userEvent.setup()
    await screen.findByText(/Searching 2 cases/)
    await user.type(screen.getByLabelText('Your question'), 'What is the smallest position of the towers?')
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    await screen.findByRole('heading', { name: 'Found in 1 of 2 cases.' })
    const abstained = document.getElementById('ax-abstained')
    expect(abstained.querySelector('.ax-case__state').textContent).toBe('Needs a choice')
    expect(abstained.textContent).toMatch(/did not use the field the question names \(position\)/)
    expect(screen.getByLabelText('Your question')).toBeTruthy()
  })

  it('lets the search be narrowed, and only asks the chosen cases', async () => {
    const asked = []
    open(['alpha', 'bravo'], { asked })
    const user = userEvent.setup()
    await screen.findByText(/Searching 2 cases/)
    await user.click(screen.getByText(/^Scope:/))
    await user.click(screen.getByRole('checkbox', { name: /bravo/ }))
    expect(await screen.findByText(/Searching 1 case, 1 skipped/)).toBeTruthy()
    await user.type(screen.getByLabelText('Your question'), 'Anything?')
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    await waitFor(() => expect(asked).toEqual(['bravo']))
  })

  it('carries a follow-up into the case that answered', async () => {
    open(['alpha'])
    const user = userEvent.setup()
    await screen.findByText(/Searching 1 case/)
    await user.type(screen.getByLabelText('Your question'), 'Who contacted 0300?')
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    await user.click(await screen.findByRole('link', { name: /Continue in this case/ }))
    expect(screen.getByTestId('landing').textContent).toBe('/cases/alpha/investigate?question=Who%20contacted%200300%3F')
  })

  it('keeps the ask button off until there is a question, and shows an honest empty state with no case', async () => {
    open(['alpha'])
    expect((await screen.findByRole('button', { name: 'Ask' })).disabled).toBe(true)
  })

  it('has an empty state when no case is configured', () => {
    open([])
    expect(screen.getByRole('heading', { name: 'No cases are configured' })).toBeTruthy()
  })

  it('opens the evidence behind one case’s answer in a panel, and closes it with Escape', async () => {
    open(['alpha', 'bravo'])
    const user = userEvent.setup()
    await screen.findByText(/Searching 2 cases/)
    await user.type(screen.getByLabelText('Your question'), 'Who contacted 0300?')
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    await screen.findByRole('heading', { name: 'Found in 1 of 2 cases.' })
    const alpha = document.getElementById('ax-alpha')
    await user.click(within(alpha).getByRole('button', { name: /Evidence/ }))
    const panel = screen.getByRole('complementary', { name: 'Evidence for this answer' })
    expect(within(panel).getByText('alpha')).toBeTruthy()
    expect(panel.querySelector('.ch-panel__q').textContent).toBe('Who contacted 0300?')
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('complementary', { name: 'Evidence for this answer' })).toBeNull()
  })
})
