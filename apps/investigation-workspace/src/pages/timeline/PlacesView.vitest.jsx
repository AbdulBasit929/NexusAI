import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { PlacesView } from './PlacesView.jsx'
import { getPlaces } from '../../lib/apiClient.js'

// MapLibre needs WebGL; the map is replaced by a stub that shows the sites and the track it is given and lets a test "click" a site.
vi.mock('../../components/GeoMap.jsx', () => ({
  GeoMap: ({ sites, track, selectedId, onSelect }) => (
    <div data-testid="geo-map" data-sites={sites.length} data-positioned={sites.filter(s => Number.isFinite(s.lat)).length} data-track={track.map(p => p.cell_key).join(',')} data-selected={selectedId || ''}>
      {sites.map(s => <button key={s.id} type="button" onClick={() => onSelect(s.id)}>{`site ${s.id}`}</button>)}
    </div>
  ),
}))
vi.mock('../../lib/apiClient.js', async importOriginal => ({ ...(await importOriginal()), getPlaces: vi.fn() }))

// Synthetic numbers from the reserved range. Subscriber A has located visits (two at Alpha, one at Beta) and one visit at a cell with no position; B has none located.
const A = '923000001001'
const B = '923000001002'
const response = {
  available: true,
  caveats: ['A cell site is a coverage area, not the position of a person.'],
  totals: { sites: 3, sites_with_position: 2 },
  sites: [
    { cell_key: 'k1', name: 'Site Alpha', lat: 33.1, lon: 73.1, events: 10, visitors: 1, radius_m: 1500 },
    { cell_key: 'k2', name: 'Site Beta', lat: 33.2, lon: 73.2, events: 5, visitors: 1, radius_m: 0 },
    { cell_key: 'k3', name: null, lat: null, lon: null, events: 7, visitors: 2, radius_m: 0 },
  ],
  visits: [
    { subject: A, cell_key: 'k1', hour: '2026-03-01T08:00:00+05:00', events: 3 },
    { subject: A, cell_key: 'k1', hour: '2026-03-01T09:00:00+05:00', events: 2 },
    { subject: A, cell_key: 'k2', hour: '2026-03-01T10:00:00+05:00', events: 4 },
    { subject: A, cell_key: 'k3', hour: '2026-03-02T10:00:00+05:00', events: 2 },
    { subject: B, cell_key: 'k3', hour: '2026-03-02T11:00:00+05:00', events: 5 },
  ],
}

const mount = () => render(<MemoryRouter><PlacesView caseId="case-a" /></MemoryRouter>)
// the page chooses the subscriber to follow in an effect after the first paint: wait for it before acting on the map or the slider
const settled = async who => { const map = await screen.findByTestId('geo-map'); await waitFor(() => expect(screen.getByRole('combobox', { name: 'Subscriber to follow' })).toHaveValue(who)); return map }

describe('PlacesView', () => {
  beforeEach(() => { getPlaces.mockReset() })
  afterEach(() => { vi.restoreAllMocks() })

  it('starts on the subscriber with the most located visits and draws its track through the sites in time order, once per stay', async () => {
    getPlaces.mockResolvedValue(response)
    mount()
    const map = await settled(A)
    expect(map.dataset.sites).toBe('3')
    expect(map.dataset.positioned).toBe('2')
    expect(map.dataset.track).toBe('k1,k2')
    const choice = screen.getByRole('combobox', { name: 'Subscriber to follow' })
    expect(within(choice).getAllByRole('option').map(option => option.textContent)).toEqual([
      expect.stringMatching(/11 events, 9 on a located cell/),
      expect.stringMatching(/5 events, no located cell/),
    ])
  })

  it('says how many sites have a position and that a cell site is a coverage area', async () => {
    getPlaces.mockResolvedValue(response)
    mount()
    await screen.findByTestId('geo-map')
    expect(screen.getByText(/2 of 3 cell sites have a position in the records/)).toBeInTheDocument()
    expect(screen.getByText(/not the position of a person/)).toBeInTheDocument()
  })

  it('plays along the time slider: the track grows with the position of the slider', async () => {
    getPlaces.mockResolvedValue(response)
    mount()
    await settled(A)
    const slider = screen.getByRole('slider', { name: 'Time along the movement' })
    expect(slider).toHaveAttribute('max', '3')
    fireEvent.change(slider, { target: { value: '1' } })
    expect(screen.getByTestId('geo-map').dataset.track).toBe('k1')
    expect(screen.getByText('2026-03-01 03:00')).toBeInTheDocument()
    fireEvent.change(slider, { target: { value: '3' } })
    expect(screen.getByTestId('geo-map').dataset.track).toBe('k1,k2')
  })

  it('follows another subscriber, and a subscriber with no located cell has no track and no player', async () => {
    getPlaces.mockResolvedValue(response)
    mount()
    await settled(A)
    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Subscriber to follow' }), B)
    expect(screen.getByTestId('geo-map').dataset.track).toBe('')
    expect(screen.getByRole('button', { name: 'Play' })).toBeDisabled()
    expect(screen.getByText('no positioned visits')).toBeInTheDocument()
  })

  it('opens a site from the list or the map, with its facts and a question about the subscriber\'s cells', async () => {
    getPlaces.mockResolvedValue(response)
    mount()
    await userEvent.click(await screen.findByRole('button', { name: 'site k2' }))
    expect(screen.getByTestId('geo-map').dataset.selected).toBe('k2')
    const panel = screen.getByRole('complementary', { name: 'Cell sites' })
    expect(within(panel).getByRole('heading', { name: 'Site Beta' })).toBeInTheDocument()
    expect(within(panel).getByText('not stated')).toBeInTheDocument()
    const ask = within(panel).getByRole('link', { name: /Ask about this subscriber/ })
    expect(ask.getAttribute('href')).toContain('/cases/case-a/investigate?question=')
    expect(decodeURIComponent(ask.getAttribute('href'))).toContain(A)
    await userEvent.click(within(panel).getByRole('button', { name: 'Clear' }))
    expect(within(panel).getByRole('heading', { name: 'Busiest cell sites' })).toBeInTheDocument()
  })

  it('lists the busiest sites first and marks the ones with no position', async () => {
    getPlaces.mockResolvedValue(response)
    mount()
    await screen.findByTestId('geo-map')
    const items = within(screen.getByRole('complementary', { name: 'Cell sites' })).getAllByRole('listitem')
    expect(items[0].textContent).toMatch(/Site Alpha.*10 events/)
    expect(items[1].textContent).toMatch(/k3.*7 events.*no position/)
    expect(items[2].textContent).toMatch(/Site Beta.*5 events/)
  })

  it('says why a case cannot be drawn on a map, and reports a failed request', async () => {
    getPlaces.mockResolvedValueOnce({ available: false, reason: 'The case has no typed call records.' })
    const first = mount()
    expect(await screen.findByText('This case cannot be drawn on a map yet')).toBeInTheDocument()
    first.unmount()
    getPlaces.mockRejectedValueOnce(Object.assign(new Error('x'), { reference: 'HTTP-503' }))
    mount()
    expect(await screen.findByText('The places could not be read')).toBeInTheDocument()
    expect(screen.getByText(/HTTP-503/)).toBeInTheDocument()
  })
})
