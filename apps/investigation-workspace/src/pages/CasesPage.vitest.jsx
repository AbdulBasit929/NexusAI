import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import CasesPage from './CasesPage.jsx'

const summaries = {
  'case-2': { evidence_total: 2, evidence_completed: 2, evidence_in_flight: 0, evidence_failed: 0, accepted_rows: 240 },
  'case-10': { evidence_total: 8, evidence_completed: 8, evidence_in_flight: 0, evidence_failed: 0, accepted_rows: 8642 },
  'case-review': { evidence_total: 3, evidence_completed: 2, evidence_in_flight: 0, evidence_failed: 1, accepted_rows: 400 },
}

function response(body, status = 200) {
  return { ok: status >= 200 && status < 300, status, headers: { get: () => null }, json: async () => body }
}

function open(caseIds = ['case-10', 'case-2', 'case-review', 'case-denied'], view = 'table') {
  window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds }
  localStorage.setItem('nexusai.cases.view', view)
  return render(<MemoryRouter><CasesPage /></MemoryRouter>)
}

describe('CasesPage', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn(async (_url, options) => {
      const caseId = options.headers['X-Forensic-Collection-ID']
      if (caseId === 'case-denied') return response({}, 403)
      return response({
        collection_id: caseId,
        summary: summaries[caseId],
        record_families: caseId === 'case-10' ? [{ record_type: 'cdr', accepted_rows: 7000 }, { record_type: 'anpr', accepted_rows: 1642 }] : [],
        recent_evidence: [{ evidence_id: `${caseId}-source`, source_file: `${caseId}.csv`, updated_at: caseId === 'case-review' ? '2026-09-28T10:00:00Z' : caseId === 'case-10' ? '2026-09-28T09:00:00Z' : '2026-09-28T08:00:00Z' }],
        recent_jobs: [],
      })
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    delete window.__INVESTIGATION_WORKSPACE_CONFIG__
  })

  it('renders a searchable, sortable directory from real collection status', async () => {
    open()
    await screen.findByRole('heading', { name: 'Some statuses are unavailable' })
    const table = screen.getByRole('table', { name: /4 of 4 configured collections/i })
    const rows = within(table).getAllByRole('row').slice(1)
    expect(rows.map(row => row.textContent)).toEqual([
      expect.stringContaining('case-review'),
      expect.stringContaining('case-10'),
      expect.stringContaining('case-2'),
      expect.stringContaining('case-denied'),
    ])
    expect(within(rows[1]).getByRole('img', { name: /case-10 evidence readiness/i })).toBeTruthy()
    expect(within(rows[1]).getByRole('img', { name: /case-10 accepted rows by evidence family/i })).toBeTruthy()
    expect(rows[1]).toHaveTextContent(/28 Sep(?:t)? 2026, 09:00 UTC/)

    await userEvent.click(within(table).getByRole('button', { name: /Evidence , not sorted/i }))
    await waitFor(() => expect(within(table).getAllByRole('row')[1]).toHaveTextContent('case-10'))
    expect(within(table).getAllByRole('columnheader')[1]).toHaveAttribute('aria-sort', 'descending')

    await userEvent.type(screen.getByRole('searchbox', { name: 'Search cases' }), 'review')
    expect(within(table).getAllByRole('row')).toHaveLength(2)
    expect(within(table).getByText('case-review')).toBeTruthy()
  })

  it('shows cases as cards by default, each with its readiness and a link, and remembers the table choice', async () => {
    open(undefined, 'cards')
    await screen.findByRole('heading', { name: 'Some statuses are unavailable' })
    const cards = document.querySelectorAll('.case-tile')
    expect(cards).toHaveLength(4)
    const review = [...cards].find(card => card.textContent.includes('case-review'))
    expect(within(review).getByText('2 of 3 sources ready')).toBeTruthy()
    expect(within(review).getByRole('link', { name: 'Review evidence: case-review' }).getAttribute('href')).toBe('/cases/case-review/evidence')
    expect(within(review).getByText('Needs review')).toBeTruthy()
    const denied = [...cards].find(card => card.textContent.includes('case-denied'))
    expect(within(denied).getByText('Outside your access scope.')).toBeTruthy()
    expect(within(denied).queryByRole('link')).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: /Table/ }))
    expect(screen.getByRole('table', { name: /4 of 4 configured collections/i })).toBeTruthy()
    expect(localStorage.getItem('nexusai.cases.view')).toBe('table')
  })

  it('keeps unavailable status distinct from zero evidence', async () => {
    open()
    const denied = await screen.findByRole('rowheader', { name: /case-denied access restricted/i })
    const row = denied.closest('tr')
    expect(row).toHaveTextContent('Outside your access scope')
    expect(row).toHaveTextContent('Not available')
    expect(row).not.toHaveTextContent('0 evidence')
    expect(within(row).queryByRole('link')).toBeNull()
  })

  it('moves between openable case rows with the keyboard', async () => {
    open()
    const review = await screen.findByRole('link', { name: 'case-review' })
    review.focus()
    await userEvent.keyboard('{ArrowDown}')
    expect(screen.getByRole('link', { name: 'case-10' })).toHaveFocus()
    await userEvent.keyboard('{End}')
    expect(screen.getByRole('link', { name: 'case-2' })).toHaveFocus()
    await userEvent.keyboard('{Home}')
    expect(review).toHaveFocus()
  })

  it('renders the authoritative empty configuration state without requesting status', () => {
    open([])
    expect(screen.getByRole('heading', { name: 'No collections are configured' })).toBeTruthy()
    expect(globalThis.fetch).not.toHaveBeenCalled()
  })
})
