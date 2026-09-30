import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import GlobalActivityPage from './GlobalActivityPage.jsx'

function open(caseIds) {
  window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds }
  globalThis.fetch = vi.fn()
  return render(<MemoryRouter initialEntries={['/activity']}><GlobalActivityPage /></MemoryRouter>)
}

afterEach(() => {
  vi.restoreAllMocks()
  delete window.__INVESTIGATION_WORKSPACE_CONFIG__
})

describe('GlobalActivityPage', () => {
  it('keeps workspace audit history unavailable while offering case-scoped recovery', () => {
    open(['case-alpha', 'case-beta'])
    expect(screen.getByRole('heading', { name: 'Workspace-wide audit history is not available' })).toBeTruthy()
    expect(screen.getByText(/available total cannot show who acted/i)).toBeTruthy()
    const records = screen.getAllByRole('link', { name: 'Open working record', exact: true })
    expect(records[0].getAttribute('href')).toBe('/cases/case-alpha/activity')
    expect(records).toHaveLength(2)
    expect(screen.getByText(/Custody events, exports and team-wide viewer activity remain absent/i)).toBeTruthy()
    // The shell's sidebar badges read collection status; the page itself must never request activity history.
    const requested = globalThis.fetch.mock.calls.map(([url]) => String(url))
    expect(requested.every(url => url.includes('/collections/status'))).toBe(true)
  })

  it('does not invent a working record when no collection is configured', () => {
    open([])
    expect(screen.getByRole('heading', { name: 'No case working records are available' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Add evidence' }).getAttribute('href')).toBe('/cases/new')
    expect(screen.queryByRole('link', { name: 'Open working record' })).toBeNull()
  })
})
