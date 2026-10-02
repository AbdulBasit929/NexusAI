import { afterEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import GlobalActivityPage from './GlobalActivityPage.jsx'

const status = {
  summary: {}, record_families: [],
  recent_evidence: [{ evidence_id: 'e1', source_file: 'scan.pdf', processing_status: 'failed', updated_at: '2026-02-02T10:00:00Z' }, { evidence_id: 'e2', source_file: 'calls.csv', processing_status: 'completed', updated_at: '2026-02-03T10:00:00Z' }],
  recent_jobs: [], missing_kb_assets: [],
}

function open(caseIds, respond = () => new Response(JSON.stringify(status), { status: 200, headers: { 'content-type': 'application/json' } })) {
  window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds, tenantId: 'default' }
  globalThis.fetch = vi.fn(async () => respond())
  return render(<MemoryRouter initialEntries={['/activity']}><GlobalActivityPage /></MemoryRouter>)
}

vi.mock('../components/charts/EChart.jsx', () => ({ default: () => null }))

afterEach(() => {
  vi.restoreAllMocks()
  globalThis.localStorage?.clear()
  delete window.__INVESTIGATION_WORKSPACE_CONFIG__
})

describe('GlobalActivityPage', () => {
  it('opens on what needs attention across cases and lets an item be marked reviewed', async () => {
    open(['case-alpha', 'case-beta'])
    expect(await screen.findByRole('heading', { name: 'Evidence added: scan.pdf' })).toBeTruthy()
    expect(screen.getAllByText('case-alpha').length).toBeGreaterThan(0)
    expect(screen.getAllByText('case-beta').length).toBeGreaterThan(0)
    fireEvent.click(screen.getByRole('button', { name: 'Mark as reviewed' }))
    fireEvent.click(screen.getByRole('button', { name: 'Mark as reviewed' }))
    expect(screen.getByRole('heading', { name: 'You are all caught up' })).toBeTruthy()
    expect(globalThis.localStorage.getItem('nexusai.viewer.activity.reviewed')).toContain('case-alpha|evidence-e1')
  })

  it('narrows to one case and says honestly when a case could not report', async () => {
    open(['case-alpha', 'case-beta'], (() => { let calls = 0; return () => (++calls === 2 ? new Response('{}', { status: 500 }) : new Response(JSON.stringify(status), { status: 200, headers: { 'content-type': 'application/json' } })) })())
    await waitFor(() => expect(screen.getByText(/1 of 2 cases could not report/)).toBeTruthy())
  })

  it('does not invent a working record when no collection is configured', () => {
    open([])
    expect(screen.getByRole('heading', { name: 'No case working records are available' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Add evidence' }).getAttribute('href')).toBe('/cases/new')
  })
})
