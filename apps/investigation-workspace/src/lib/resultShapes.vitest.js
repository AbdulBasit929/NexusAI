import { describe, expect, it } from 'vitest'
import { answerRows, graphTargets, mapSites, resultShapes } from './resultShapes.js'

// Synthetic numbers from the reserved range: nothing here is a real subscriber.
const answer = (columns, rows, shape) => ({
  result: { columns: columns.map(key => ({ key, label: key })) },
  original: { enterprise: { result_shape: shape, data_grid: { rows } } },
})

const edges = [
  { src: '923000001001', dst: '923000001002', events: 4 },
  { src: '923000001002', dst: '923000001003', events: 2 },
]
const sites = [
  { cell_key: '410-01:1:100', label: 'Site Alpha', lat: 33.1, lon: 73.1, events: 9, radius_m: 1500 },
  { cell_key: '410-01:1:100', label: 'Site Alpha', lat: 33.1, lon: 73.1, events: 3, radius_m: 1500 },
  { cell_key: '410-01:1:200', label: 'Site Beta', lat: '33.2', lon: '73.2', events: 1 },
  { cell_key: '410-01:1:300', label: 'No position', lat: null, lon: null, events: 5 },
]

describe('answerRows', () => {
  it('reads the rows the table shows, or nothing', () => {
    expect(answerRows(answer(['src', 'dst'], edges))).toHaveLength(2)
    expect(answerRows({ original: null })).toEqual([])
    expect(answerRows(undefined)).toEqual([])
  })
})

describe('resultShapes', () => {
  it('offers only the table for a plain list, and starts on it', () => {
    const shapes = resultShapes(answer(['label'], [{ label: 'a' }, { label: 'b' }]))
    expect(shapes.views.map(view => view.id)).toEqual(['table'])
    expect(shapes.first).toBe('table')
  })

  it('adds a link chart when the rows have a source and a destination, and starts on it', () => {
    const shapes = resultShapes(answer(['src', 'dst', 'events'], edges, 'graph'))
    expect(shapes.views.map(view => view.id)).toEqual(['table', 'graph'])
    expect(shapes.views[1].label).toBe('Link chart')
    expect(shapes.first).toBe('graph')
  })

  it('does not draw a link chart when no row has both ends', () => {
    const shapes = resultShapes(answer(['src', 'dst'], [{ src: '923000001001', dst: null }, { src: null, dst: '923000001002' }]))
    expect(shapes.views.map(view => view.id)).toEqual(['table'])
  })

  it('adds a map only when at least one row has a usable position', () => {
    expect(resultShapes(answer(['lat', 'lon'], sites, 'map')).views.map(view => view.id)).toEqual(['table', 'map'])
    expect(resultShapes(answer(['lat', 'lon'], [{ lat: null, lon: null }, { lat: '', lon: 'x' }])).views.map(view => view.id)).toEqual(['table'])
    expect(resultShapes(answer(['lat', 'lon'], sites, 'map')).first).toBe('map')
  })

  it('matches the column names whatever their case', () => {
    expect(resultShapes(answer(['SRC', 'DST'], [{ SRC: 'a', DST: 'b', src: 'a', dst: 'b' }])).views.map(view => view.id)).toContain('graph')
  })

  it('adds a chart for a ranking, and starts on it only when the service says the answer is ranked', () => {
    const rows = [{ label: 'a', m1: 3 }, { label: 'b', m1: 7 }]
    const ranked = resultShapes(answer(['label', 'm1'], rows, 'ranked'))
    expect(ranked.views.map(view => view.id)).toEqual(['table', 'chart'])
    expect(ranked.first).toBe('chart')
    expect(resultShapes(answer(['label', 'm1'], rows)).first).toBe('table')
  })

  it('falls back to the graph, then the map, then the table when the stated shape is not among the views', () => {
    expect(resultShapes(answer(['src', 'dst'], edges, 'map')).first).toBe('graph')
    expect(resultShapes(answer(['lat', 'lon'], sites, 'ranked')).first).toBe('map')
    expect(resultShapes(answer(['label'], [{ label: 'a' }], 'graph')).first).toBe('table')
  })
})

describe('graphTargets', () => {
  it('starts from the numbers that are never a destination', () => {
    expect(graphTargets(edges)).toEqual(['923000001001'])
  })

  it('falls back to every source when the links form a loop, and never returns more than five', () => {
    const loop = [{ src: 'a', dst: 'b' }, { src: 'b', dst: 'a' }]
    expect(graphTargets(loop)).toEqual(['a', 'b'])
    const many = Array.from({ length: 9 }, (_, index) => ({ src: `s${index}`, dst: 'hub' }))
    expect(graphTargets(many)).toHaveLength(5)
  })
})

describe('mapSites', () => {
  it('adds the events of a repeated cell into one site and keeps its radius', () => {
    const out = mapSites(sites)
    expect(out).toHaveLength(2)
    const alpha = out.find(site => site.id === '410-01:1:100')
    expect(alpha).toMatchObject({ lat: 33.1, lon: 73.1, label: 'Site Alpha', value: 12, radius: 1500 })
  })

  it('reads a position given as text and gives a site without a stated radius a radius of zero', () => {
    const beta = mapSites(sites).find(site => site.id === '410-01:1:200')
    expect(beta).toMatchObject({ lat: 33.2, lon: 73.2, radius: 0, value: 1 })
  })

  it('leaves out a row with no position, and counts a row with no count as one event', () => {
    expect(mapSites(sites).some(site => site.label === 'No position')).toBe(false)
    expect(mapSites([{ lat: 1, lon: 2 }])).toEqual([{ id: '1,2', lat: 1, lon: 2, label: '1,2', value: 1, radius: 0 }])
  })
})
