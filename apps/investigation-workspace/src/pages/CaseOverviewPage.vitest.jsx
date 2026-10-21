import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import CaseOverviewPage from './CaseOverviewPage.jsx'
import { clearCaseNotesForTests } from '../lib/caseNotes.js'
import { clearWorkspaceStateForTests } from '../lib/workspaceState.js'

const mockCaseStatus = {
  summary: {
    evidence_total: 12,
    evidence_completed: 10,
    evidence_in_flight: 1,
    evidence_failed: 1,
    accepted_rows: 4200,
    duplicate_rows: 50,
    rejected_rows: 20,
    completed_jobs_missing_kb_asset: 0,
  },
  record_families: [
    { record_type: 'cdr', accepted_rows: 3200 },
    { record_type: 'ipdr', accepted_rows: 1000 },
  ],
}

const mockActivityResponse = {
  records: {
    activity_by_day: [
      { activity_date: '2026-08-01T00:00:00Z', record_type: 'cdr', event_count: 120 },
      { activity_date: '2026-08-02T00:00:00Z', record_type: 'cdr', event_count: 450 },
    ],
  },
}

function renderOverview(caseId = 'case-alpha') {
  window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: [caseId] }
  return render(
    <MemoryRouter initialEntries={[`/cases/${caseId}/overview`]}>
      <Routes>
        <Route path="/cases/:id/overview" element={<CaseOverviewPage />} />
        <Route path="/cases/:id/investigate" element={<div data-testid="investigate-route">Investigate</div>} />
        <Route path="/cases/:id/evidence" element={<div data-testid="evidence-route">Evidence</div>} />
        <Route path="/cases/:id/connections" element={<div data-testid="connections-route">Connections</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('CaseOverviewPage', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn(async (url) => {
      const urlString = String(url)
      if (urlString.includes('/collections/status')) {
        return { ok: true, status: 200, json: async () => mockCaseStatus }
      }
      if (urlString.includes('activity_by_day')) {
        return { ok: true, status: 200, json: async () => mockActivityResponse }
      }
      if (urlString.includes('/capabilities')) {
        return { ok: true, status: 200, json: async () => ({ sample_queries: ['Who called most?', 'Show roaming calls'] }) }
      }
      return { ok: true, status: 200, json: async () => ({}) }
    })
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
    globalThis.localStorage?.clear()
    clearCaseNotesForTests()
    clearWorkspaceStateForTests()
    delete window.__INVESTIGATION_WORKSPACE_CONFIG__
  })

  it('renders the Palantir-class Case Dossier Hero with bento metrics and status badge', async () => {
    renderOverview('case-alpha')

    // Wait for case overview data to load
    expect(await screen.findByText(/Forensic Case Dossier/i)).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1, name: 'case-alpha' })).toBeInTheDocument()

    // Status & Verdict
    expect(screen.getAllByText('1 source needs review').length).toBeGreaterThanOrEqual(1)

    // Hero Bento Metrics Strip
    expect(screen.getByText('Sources')).toBeInTheDocument()
    expect(screen.getAllByText('12').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Structured Records')).toBeInTheDocument()
    expect(screen.getAllByText('4,200').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Ingestion Yield')).toBeInTheDocument()
    expect(screen.getByText('98%')).toBeInTheDocument()

    // Action buttons
    expect(screen.getByRole('link', { name: /ask about this case/i })).toHaveAttribute('href', '/cases/case-alpha/investigate')
    expect(screen.getAllByRole('link', { name: /add evidence/i }).some(link => link.getAttribute('href') === '/cases/case-alpha/evidence#add-evidence')).toBe(true)
    expect(screen.getByRole('link', { name: /connections graph/i })).toHaveAttribute('href', '/cases/case-alpha/connections')
  })

  it('copies the case identifier on copy button click with tactile feedback', async () => {
    const user = userEvent.setup()
    const writeTextMock = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: writeTextMock },
      writable: true,
      configurable: true,
    })

    renderOverview('case-alpha')

    const copyBtn = await screen.findByRole('button', { name: /copy case identifier/i })
    expect(copyBtn).toBeInTheDocument()

    await user.click(copyBtn)
    expect(writeTextMock).toHaveBeenCalledWith('case-alpha')
    expect(screen.getByText('Copied')).toBeInTheDocument()
  })

  it('renders the Evidence Composition panel with record families and share bars', async () => {
    renderOverview('case-alpha')

    expect(await screen.findByRole('heading', { name: /what is in this case/i })).toBeInTheDocument()
    expect(screen.getByText('Call detail records')).toBeInTheDocument()
    expect(screen.getByText('Internet session records')).toBeInTheDocument()
    expect(screen.getByText('3,200')).toBeInTheDocument()
    expect(screen.getByText('1,000')).toBeInTheDocument()
  })

  it('renders data-quality notes and 3-column integrity KPI cards', async () => {
    renderOverview('case-alpha')

    expect(await screen.findByRole('heading', { name: /data-quality notes/i })).toBeInTheDocument()
    expect(screen.getByText('Accepted')).toBeInTheDocument()
    expect(screen.getByText('Duplicate')).toBeInTheDocument()
    expect(screen.getByText('Rejected')).toBeInTheDocument()
    expect(screen.getByText('20 structured rows were rejected across the collection.')).toBeInTheDocument()
    expect(screen.getByText('50 duplicate structured rows were excluded across the collection.')).toBeInTheDocument()
  })

  it('submits a question in Ask About This Case and navigates to Investigate', async () => {
    const user = userEvent.setup()
    renderOverview('case-alpha')

    const input = await screen.findByPlaceholderText(/ask about the evidence/i)
    await user.type(input, 'Who called 03001234567?')

    const submitBtn = screen.getByRole('button', { name: 'Ask' })
    expect(submitBtn).toBeEnabled()

    await user.click(submitBtn)
    expect(await screen.findByTestId('investigate-route')).toBeInTheDocument()
  })

  it('allows analyst to record, display, and delete investigative lead notes', async () => {
    const user = userEvent.setup()
    renderOverview('case-alpha')

    expect(await screen.findByRole('heading', { name: /analyst notes & leads/i })).toBeInTheDocument()
    expect(screen.getByText(/no analyst notes recorded yet/i)).toBeInTheDocument()

    const noteInput = screen.getByRole('textbox', { name: /new case note text/i })
    await user.type(noteInput, 'Target suspect acquired burner SIM at 14:00')

    const addBtn = screen.getByRole('button', { name: /add note/i })
    await user.click(addBtn)

    // Note is rendered
    expect(screen.getByText('Target suspect acquired burner SIM at 14:00')).toBeInTheDocument()
    expect(screen.getByText('lead')).toBeInTheDocument()

    // Delete note
    const deleteBtn = screen.getByRole('button', { name: /delete note/i })
    await user.click(deleteBtn)

    expect(screen.queryByText('Target suspect acquired burner SIM at 14:00')).not.toBeInTheDocument()
    expect(screen.getByText(/no analyst notes recorded yet/i)).toBeInTheDocument()
  })
})
