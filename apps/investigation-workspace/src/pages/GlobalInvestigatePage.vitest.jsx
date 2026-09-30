import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import GlobalInvestigatePage, { caseScopeNote, investigateTarget } from './GlobalInvestigatePage.jsx'

function Landing() {
  const location = useLocation()
  return <p data-testid="landing">{location.pathname}{location.search}</p>
}

function open(caseIds, status) {
  window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds }
  globalThis.fetch = vi.fn(async () => ({ ok: true, status: 200, json: async () => status, text: async () => JSON.stringify(status) }))
  return render(<MemoryRouter initialEntries={['/investigate']}><Routes><Route path="/investigate" element={<GlobalInvestigatePage />} /><Route path="/cases/:id/investigate" element={<Landing />} /></Routes></MemoryRouter>)
}

afterEach(() => {
  vi.restoreAllMocks()
  delete window.__INVESTIGATION_WORKSPACE_CONFIG__
})

describe('scope wording', () => {
  it('states exactly what a question will and will not search', () => {
    expect(caseScopeNote({ total: 43, ready: 42, inFlight: 0, failed: 1 })).toBe('Answers cover the 42 ready sources only; 1 is not ready and will not be searched.')
    expect(caseScopeNote({ total: 4, ready: 4, inFlight: 0, failed: 0 })).toBe('Answers cover all 4 ready sources in this case.')
    expect(caseScopeNote({ total: 0, ready: 0, inFlight: 0, failed: 0 })).toMatch(/no evidence yet/)
  })

  it('builds a case-scoped link from a question', () => {
    expect(investigateTarget('case/a', '  Who called most? ')).toBe('/cases/case%2Fa/investigate?question=Who%20called%20most%3F')
  })
})

describe('GlobalInvestigatePage', () => {
  it('routes the question to the chosen case and never claims to search every case', async () => {
    open(['case-alpha', 'case-beta'], { summary: { evidence_total: 3, evidence_completed: 3, evidence_in_flight: 0, evidence_failed: 0 } })
    const user = userEvent.setup()
    expect(await screen.findByText(/Answers cover all 3 ready sources/)).toBeTruthy()
    expect(screen.getByText(/Scope: 1 case\./)).toBeTruthy()
    await user.click(screen.getByRole('radio', { name: /case-beta/ }))
    await user.type(screen.getByLabelText('2. Your question'), 'Who called most often?')
    await user.click(screen.getByRole('button', { name: /Ask in case-beta/ }))
    expect(screen.getByTestId('landing').textContent).toBe('/cases/case-beta/investigate?question=Who%20called%20most%20often%3F')
  })

  it('keeps the ask button off until there is a question', async () => {
    open(['case-alpha'], { summary: { evidence_total: 3, evidence_completed: 3 } })
    expect(await screen.findByRole('button', { name: /Ask in case-alpha/ })).toHaveProperty('disabled', true)
  })

  it('shows an honest empty state with no configured case', () => {
    open([], {})
    expect(screen.getByRole('heading', { name: 'No cases are configured' })).toBeTruthy()
  })
})
