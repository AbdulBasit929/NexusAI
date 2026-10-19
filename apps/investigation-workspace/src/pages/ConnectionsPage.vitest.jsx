import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import ConnectionsPage from './ConnectionsPage.jsx'
import { getConnections } from '../lib/apiClient.js'

// The canvas (Cytoscape) is replaced by a stub that shows what the page hands it and lets a test "click" a number; the chart itself is checked in the browser.
vi.mock('../components/LinkChart.jsx', async () => {
  const React = await import('react')
  return {
    LinkChart: React.forwardRef(function LinkChartStub({ model, selectedId, slots, pathIds, sharedIds, onSelectNode }, ref) {
      React.useImperativeHandle(ref, () => ({ fit() {}, png: () => null, zoomTo() {} }))
      return (
        <div data-testid="link-chart" data-nodes={model.nodes.map(n => n.id).join(',')} data-selected={selectedId || ''} data-slots={`${slots?.A || ''}|${slots?.B || ''}`} data-path={(pathIds || []).join(',')} data-shared={(sharedIds || []).join(',')}>
          {model.nodes.map(n => <button key={n.id} type="button" onClick={() => onSelectNode(n.id)}>{`node ${n.id}`}</button>)}
        </div>
      )
    }),
  }
})
vi.mock('../lib/apiClient.js', async importOriginal => ({ ...(await importOriginal()), getConnections: vi.fn() }))

// Synthetic numbers from the reserved range. Two subscribers who both contact 010, a service that one of them contacts.
const A = '923000001001'
const B = '923000001002'
const C = '923000001010'
const response = {
  available: true,
  contract: 'forensics.analysis-views/v1',
  time_zone: 'Asia/Karachi',
  truncated: false,
  per_subject: 60,
  caveats: ['A link is a record of contact, not a finding about the people.'],
  totals: { subjects: 2, counterparties: 2, events: 40, pairs: 3, hubs: 0, service_pairs: 1 },
  subjects: [
    { number: A, events: 25, contacts: 2, first: '2026-02-02T00:23:31+05:00', last: '2026-05-02T22:19:27+05:00', imsi: '001010000001001', imei: '00000000001001' },
    { number: B, events: 15, contacts: 1, first: '2026-02-03T07:22:46+05:00', last: '2026-05-01T23:55:21+05:00', imsi: '001010000001002', imei: '00000000001002' },
  ],
  edges: [
    { source: A, target: C, events: 10, outgoing: 6, incoming: 4, unknown: 0, duration_s: 600, first: '2026-02-02T00:23:31+05:00', last: '2026-04-02T10:00:00+05:00', is_service: false, is_hub: false },
    { source: B, target: C, events: 6, outgoing: 3, incoming: 3, unknown: 0, duration_s: 300, first: '2026-02-03T07:22:46+05:00', last: '2026-04-05T10:00:00+05:00', is_service: false, is_hub: false },
    { source: A, target: '8888', events: 5, outgoing: 5, incoming: 0, unknown: 0, duration_s: 0, first: '2026-02-04T00:00:00+05:00', last: '2026-03-01T00:00:00+05:00', is_service: true, is_hub: false },
  ],
}

function mount(path = `/cases/case-a/connections`) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes><Route path="/cases/:id/connections" element={<ConnectionsPage />} /></Routes>
    </MemoryRouter>,
  )
}

describe('ConnectionsPage', () => {
  beforeEach(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a'] }
    globalThis.fetch = vi.fn(async () => ({ ok: true, status: 200, headers: { get: () => 'application/json' }, json: async () => ({ summary: {}, record_families: [], recent_evidence: [], recent_jobs: [] }) }))
    getConnections.mockReset()
  })
  afterEach(() => { vi.restoreAllMocks(); delete window.__INVESTIGATION_WORKSPACE_CONFIG__ })

  it('draws the subscribers and their ordinary contacts, leaves services out until asked, and says what the chart is', async () => {
    getConnections.mockResolvedValue(response)
    mount()
    const chart = await screen.findByTestId('link-chart')
    expect(chart.dataset.nodes.split(',').sort()).toEqual([A, B, C].sort())
    expect(screen.getByRole('heading', { level: 1, name: 'Connections' })).toBeInTheDocument()
    expect(screen.getByLabelText('Legend')).toBeInTheDocument()
    expect(screen.getByText(/A link is a record of contact, not a finding about the people/)).toBeInTheDocument()
    expect(getConnections).toHaveBeenCalledWith(expect.objectContaining({ caseId: 'case-a', perSubject: 60 }))
  })

  it('keeps the targets of the address: only the chosen subscriber and its own contacts', async () => {
    getConnections.mockResolvedValue(response)
    mount(`/cases/case-a/connections?t=${A}`)
    const chart = await screen.findByTestId('link-chart')
    expect(chart.dataset.nodes.split(',').sort()).toEqual([A, C].sort())
  })

  it('selects a number from the chart: the address, the panel and the question links follow', async () => {
    getConnections.mockResolvedValue(response)
    mount()
    await userEvent.click(await screen.findByRole('button', { name: `node ${A}` }))
    await waitFor(() => expect(screen.getByTestId('link-chart').dataset.selected).toBe(A))
    const panel = screen.getByRole('complementary', { name: 'Selected number' })
    const asks = within(panel).getAllByRole('link')
    expect(asks.length).toBeGreaterThan(0)
    expect(asks.every(link => link.getAttribute('href').startsWith('/cases/case-a/investigate?question='))).toBe(true)
  })

  it('has a table of every number the chart draws, as the way in without the canvas', async () => {
    getConnections.mockResolvedValue(response)
    mount()
    await screen.findByTestId('link-chart')
    const table = screen.getByRole('region', { name: 'The same numbers as a table' })
    expect(within(table).getAllByRole('row').length).toBe(1 + 3)
    await userEvent.click(within(table).getByRole('tab', { name: /^Links/ }))
    expect(within(table).getAllByRole('row').length).toBeGreaterThan(2)
    expect(within(table).getByRole('button', { name: /Save as CSV/ })).toBeInTheDocument()
  })

  it('offers a way past the number buttons to the table, for a keyboard user', async () => {
    getConnections.mockResolvedValue(response)
    mount()
    await screen.findByTestId('link-chart')
    await userEvent.click(screen.getByRole('button', { name: 'Skip to the table of the same numbers' }))
    expect(document.activeElement).toBe(screen.getByRole('region', { name: 'The same numbers as a table' }))
  })

  it('reads two numbers from the address and marks their shared contact on the chart', async () => {
    getConnections.mockResolvedValue(response)
    mount(`/cases/case-a/connections?a=${A}&b=${B}`)
    const chart = await screen.findByTestId('link-chart')
    expect(chart.dataset.slots).toBe(`${A}|${B}`)
    expect(chart.dataset.shared).toBe(C)
  })

  it('says why a case cannot be drawn, and draws nothing over part of the records', async () => {
    getConnections.mockResolvedValue({ available: false, reason: 'The case has call-record files that are not typed yet.' })
    mount()
    expect(await screen.findByText('This case cannot be drawn as connections yet')).toBeInTheDocument()
    expect(screen.getByText(/not typed yet/)).toBeInTheDocument()
    expect(screen.queryByTestId('link-chart')).not.toBeInTheDocument()
  })

  it('reports a failed request with its reference and changes nothing', async () => {
    getConnections.mockRejectedValue(Object.assign(new Error('x'), { reference: 'HTTP-500' }))
    mount()
    expect(await screen.findByText('The connections could not be read')).toBeInTheDocument()
    expect(screen.getByText(/HTTP-500/)).toBeInTheDocument()
  })
})
