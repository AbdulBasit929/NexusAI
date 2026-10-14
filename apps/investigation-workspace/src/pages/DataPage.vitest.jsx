import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import DataPage from './DataPage.jsx'

// Synthetic numbers from the project's reserved range: the page itself is only ever fed real values on the owner's machine.
const inventory = {
  collection: 'case-a',
  generated_utc: '2026-10-14 12:00:00',
  files: [
    { id: 'F01', name: 'export-one.xlsx', layout: 'call-c1-company', layout_label: 'Call records, 16 columns', family: 'call_records', kind: 'target_cdr', rows: 120, repeats: 2, subscribers: 1, first: '2026-01-01 08:00:00', last: '2026-01-31 20:00:00', columns: ['CALL_START_DT_TM', 'MSISDN'] },
    { id: 'F02', name: 'dump-two.xlsx', layout: 'geofence-c6-telenor', layout_label: 'Geofence (tower dump)', family: 'geofence_dump', kind: 'tower_dump', rows: 80, repeats: 0, subscribers: 40, first: '2026-01-05 00:00:00', last: '2026-01-06 00:00:00', columns: ['LAT', 'LONGITUDE'] },
  ],
  subscribers: [{
    number: '923000000001', layout: 'call-c1-company', label: 'Call records, 16 columns', events: 118, calls: 100, sms: 18, outgoing: 60, incoming: 58, counterparties: 3, total_duration_s: 7200, cells: 2, first: '2026-01-01 08:00:00', last: '2026-01-31 20:00:00',
    top_counterparties: [{ number: '923000000002', kind: 'person_like', events: 30, outgoing: 20, incoming: 10, duration_s: 900, first: '2026-01-02 09:00:00', last: '2026-01-30 10:00:00' }],
    devices: [{ imsi: '001010000000001', imei: '00000000000001', events: 118, first: '2026-01-01 08:00:00', last: '2026-01-31 20:00:00' }],
  }],
  towers: [
    { cell_key: '?:1:100', name: 'Site Alpha', names_seen: ['Site Alpha', 'Alpha 2'], layout: 'call-c1-company', lat: 33.1, lon: 73.1, events: 90, numbers: 3, call_events: 90, dump_events: 0, files: 1, lac: '1', first: '2026-01-01 08:00:00', last: '2026-01-31 20:00:00' },
    { cell_key: '?:1:200', name: 'Site Beta', names_seen: ['Site Beta'], layout: 'geofence-c6-telenor', lat: 33.2, lon: 73.2, events: 40, numbers: 30, call_events: 0, dump_events: 40, files: 1, lac: '1', first: '2026-01-05 00:00:00', last: '2026-01-06 00:00:00' },
    { cell_key: '?:1:300', name: null, names_seen: [], layout: 'geofence-c7-ufone', lat: null, lon: null, events: 10, numbers: 5, call_events: 0, dump_events: 10, files: 1, lac: '1', first: null, last: null },
  ],
  towers_total: 3,
  sessions: { total: 50, subscribers: [{ number: '923000000001', layout: 'ipdr-i1-jazz-ufdr', sessions: 50, public_addresses: 2, bytes_up: 1000, bytes_down: 5000, first: '2026-01-02 00:00:00', last: '2026-01-03 00:00:00' }],
    top_public_addresses: [{ address: '192.0.2.10', sessions: 30, subscribers: 2, port_from: 1000, port_to: 2000 }], top_applications: [{ application: 'Video', sessions: 20 }], top_hosts: [{ host: 'example.test', sessions: 9 }] },
  app_usage: [{ application: 'Chat', category: 'Social', records: 12, numbers: 1 }],
}

function mount(path = '/cases/case-a/data') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes><Route path="/cases/:id/data" element={<DataPage />} /></Routes>
    </MemoryRouter>,
  )
}

describe('DataPage', () => {
  beforeEach(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a', 'case-b'], dataInventoryCaseIds: ['case-a'] }
    globalThis.fetch = vi.fn(async url => {
      const text = String(url)
      if (text.includes('/real-data/inventory.json')) return { ok: true, status: 200, headers: { get: () => 'application/json' }, json: async () => inventory }
      return { ok: true, status: 200, headers: { get: () => 'application/json' }, json: async () => ({ summary: {}, record_families: [], recent_evidence: [], recent_jobs: [] }) }
    })
  })
  afterEach(() => { vi.restoreAllMocks(); delete window.__INVESTIGATION_WORKSPACE_CONFIG__ })

  it('shows the inventory: totals, the real-data note and six sections', async () => {
    mount()
    await screen.findByRole('note')
    expect(screen.getByRole('note').textContent).toMatch(/real data supplied for testing/i)
    expect(screen.getAllByRole('tab').map(tab => tab.textContent)).toEqual(['Overview', 'Files', 'Subscribers', 'Towers', 'Internet records', 'Downloads'])
    expect(screen.getByText('rows read').previousSibling.textContent).toBe('200')
    expect(screen.getByRole('img', { name: /map of 2 towers/i })).toBeTruthy()
  })

  it('lists the files with their columns and searches them', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(await screen.findByRole('tab', { name: 'Files' }))
    const table = screen.getByRole('table', { name: 'files' })
    expect(within(table).getAllByRole('row')).toHaveLength(3)
    await user.type(screen.getByRole('searchbox', { name: /search files/i }), 'dump')
    expect(within(screen.getByRole('table', { name: 'files' })).getAllByRole('row')).toHaveLength(2)
    expect(screen.getByText('1 of 2 files')).toBeTruthy()
  })

  it('opens a subscriber with its other parties and devices', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(await screen.findByRole('tab', { name: 'Subscribers' }))
    await user.click(within(screen.getByRole('table', { name: 'subscribers' })).getByText('923000000001'))
    expect(await screen.findByRole('heading', { name: /most frequent other parties of 923000000001/i })).toBeTruthy()
    expect(screen.getByRole('heading', { name: /devices seen with 923000000001/i })).toBeTruthy()
    expect(screen.getByText('00000000000001')).toBeTruthy()
  })

  it('shows towers with names, marks the selected one and says what has no position', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(await screen.findByRole('tab', { name: 'Towers' }))
    expect(screen.getByText(/1 towers have no position/i)).toBeTruthy()
    await user.click(within(screen.getByRole('table', { name: 'towers' })).getByText('Site Alpha'))
    expect(await screen.findByText('Alpha 2')).toBeTruthy()
    expect(document.querySelectorAll('.dp-map__dot.is-selected')).toHaveLength(1)
  })

  it('offers every table as a download', async () => {
    const user = userEvent.setup()
    mount()
    await user.click(await screen.findByRole('tab', { name: 'Downloads' }))
    const links = screen.getAllByRole('link').filter(link => String(link.getAttribute('href')).startsWith('/real-data/'))
    expect(links).toHaveLength(9)
    expect(links.every(link => link.getAttribute('href').endsWith('.csv'))).toBe(true)
  })

  it('says plainly when the case has no inventory, and never shows another case\'s data', async () => {
    mount('/cases/case-b/data')
    await waitFor(() => expect(screen.getByRole('heading', { name: 'No data inventory for this case' })).toBeTruthy())
    expect(screen.queryByRole('tab')).toBeNull()
  })
})
