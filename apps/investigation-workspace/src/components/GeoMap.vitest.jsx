import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import { GeoMap } from './GeoMap.jsx'

// MapLibre needs WebGL, which jsdom does not have: the library is replaced and the page's own behaviour is tested (what it says about the basemap, the summary for a
// screen reader, the empty state). How the map draws is checked in a real browser (docs/data/analysis-views.md).
const made = []
vi.mock('maplibre-gl', () => {
  class MapStub {
    constructor(options) { this.options = options; this.layers = []; made.push(this) }
    addControl() {}
    on() {}
    once() {}
    off() {}
    fire() {}
    resize() {}
    remove() {}
    getSource() { return null }
  }
  class Control {}
  class LngLatBounds { extend() { return this } }
  return { Map: MapStub, NavigationControl: Control, ScaleControl: Control, AttributionControl: Control, LngLatBounds, setWorkerUrl() {} }
})

const site = (id, lat, lon, value = 1, radius = 0) => ({ id, lat, lon, label: id, value, radius })
const answer = body => ({ ok: true, status: 200, headers: { get: () => 'application/json' }, json: async () => body })

describe('GeoMap', () => {
  beforeEach(() => { made.length = 0 })
  afterEach(() => { vi.restoreAllMocks() })

  it('says there is no basemap, and draws a plain background, when none is configured', async () => {
    globalThis.fetch = vi.fn(async () => ({ ok: false, status: 404, headers: { get: () => 'text/html' } }))
    render(<GeoMap sites={[site('a', 33.1, 73.1)]} />)
    expect(await screen.findByText('no basemap: plain background')).toBeInTheDocument()
    await waitFor(() => expect(made).toHaveLength(1))
    expect(Object.keys(made[0].options.style.sources)).toEqual([])
    expect(made[0].options.style.layers.map(layer => layer.id)).toEqual(['bg'])
  })

  it('uses the offline basemap through the tile cache when the page has one configured, and says so', async () => {
    globalThis.fetch = vi.fn(async () => answer({ tiles: { tilejson: '/tiles/pakistan.json', attribution: 'test' } }))
    render(<GeoMap sites={[site('a', 33.1, 73.1)]} />)
    expect(await screen.findByText('offline basemap')).toBeInTheDocument()
    await waitFor(() => expect(made).toHaveLength(1))
    expect(made[0].options.style.sources.base).toMatchObject({ type: 'raster', url: '/tiles/pakistan.json' })
    expect(made[0].options.style.layers.map(layer => layer.id)).toEqual(['bg', 'base'])
  })

  it('describes the map in words for a screen reader, with the sites and the track', async () => {
    globalThis.fetch = vi.fn(async () => ({ ok: false, status: 404, headers: { get: () => '' } }))
    render(<GeoMap sites={[site('a', 33.1, 73.1), site('b', 33.2, 73.2), site('c', null, null)]} track={[{ lat: 1, lon: 2 }, { lat: 3, lon: 4 }, { lat: 5, lon: 6 }]} label="Map of the cell sites" />)
    const map = await screen.findByRole('application')
    expect(map.getAttribute('aria-label')).toBe('Map of the cell sites. 2 cell sites with a position, and a track through 3 of them. The same sites are in the table beside the map.')
    expect(map).toHaveAttribute('tabindex', '0')
  })

  it('draws the map again, in the new colours, when the theme is changed', async () => {
    globalThis.fetch = vi.fn(async () => ({ ok: false, status: 404, headers: { get: () => '' } }))
    render(<GeoMap sites={[site('a', 33.1, 73.1)]} />)
    await waitFor(() => expect(made).toHaveLength(1))
    act(() => { globalThis.document.documentElement.setAttribute('data-theme', 'light') })
    await waitFor(() => expect(made).toHaveLength(2))
    act(() => { globalThis.document.documentElement.removeAttribute('data-theme') })
    await waitFor(() => expect(made).toHaveLength(3))
  })

  it('says why a map is empty rather than leaving it blank', async () => {
    globalThis.fetch = vi.fn(async () => ({ ok: false, status: 404, headers: { get: () => '' } }))
    render(<GeoMap sites={[site('c', null, null), site('d', undefined, undefined)]} />)
    expect(await screen.findByText(/None of these cell sites has a position in the records/)).toBeInTheDocument()
  })
})
