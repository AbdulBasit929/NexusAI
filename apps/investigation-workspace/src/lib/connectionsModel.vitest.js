import { describe, expect, it } from 'vitest'
import { buildModel, collapse, communities, emptyFilters, filterModel, filtersFromSearch, filtersToSearch, inferSharedContacts, layoutGraph, modelFromRows, normaliseNumber, shortestPath, sharedContacts, summarise } from './connectionsModel.js'
import { carrierFor, carrierOfImsi, carrierOfNumber, formatMobile } from './carriers.js'

const edge = (source, target, events, extra = {}) => ({ source, target, events, outgoing: events, incoming: 0, unknown: 0, duration_s: 0, first: '2026-02-01 10:00:00', last: '2026-02-20 10:00:00', is_service: false, is_hub: false, ...extra })
const response = {
  subjects: [
    { number: '923000001000', events: 100, contacts: 3, imsi: '410010000000001' },
    { number: '923400001001', events: 80, contacts: 3, imsi: '410060000000002' },   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    { number: '923300001002', events: 60, contacts: 2 },   // real-data-ok: an invented number in a carrier's range, for the carrier tests
  ],
  edges: [
    edge('923000001000', '923000001010', 30), edge('923000001000', '923000001011', 20), edge('923000001000', '786', 9, { is_service: true }),
    edge('923400001001', '923000001010', 25, { first: '2026-03-01 10:00:00', last: '2026-03-02 10:00:00' }), edge('923400001001', '923000001011', 5), edge('923400001001', '923000001099', 4, { is_hub: true }),   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    edge('923300001002', '923000001011', 12), edge('923300001002', '923000001099', 3, { is_hub: true }),   // real-data-ok: an invented number in a carrier's range, for the carrier tests
  ],
  totals: { pairs: 8 },
}

describe('carriers', () => {
  it('finds a carrier from the SIM first, then the number range, and says how', () => {
    expect(carrierOfImsi('410060000000002').id).toBe('telenor')   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    expect(carrierOfNumber('923000001000').id).toBe('jazz')
    expect(carrierOfNumber('923301234567').id).toBe('ufone')   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    expect(carrierOfNumber('923101234567').id).toBe('zong')   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    expect(carrierOfNumber('786').id).toBe('other')
    expect(carrierFor({ number: '923000001000', imsi: '410030000000009' })).toMatchObject({ id: 'ufone' })   // a ported number: the SIM wins   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    expect(carrierFor({ number: '923000001000' }).how).toMatch(/ported number may differ/)
  })
  it('writes a number the way a person says it', () => {
    expect(formatMobile('923001234567')).toBe('0300 1234567')
    expect(formatMobile('786')).toBe('786')
    expect(normaliseNumber('0300 1234567')).toBe('923001234567')
    expect(normaliseNumber('+92 300 1234567')).toBe('923001234567')
  })
})

describe('the connections model', () => {
  const model = buildModel(response)
  it('has the subscribers, their contacts, services and hubs, with carriers', () => {
    const byId = Object.fromEntries(model.nodes.map(n => [n.id, n]))
    expect(byId['923000001000']).toMatchObject({ kind: 'subject', carrier: 'jazz', label: '0300 0001000' })
    expect(byId['923400001001']).toMatchObject({ kind: 'subject', carrier: 'telenor' })   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    expect(byId['786'].kind).toBe('service')
    expect(byId['923000001099'].kind).toBe('hub')
    expect(byId['923000001010']).toMatchObject({ kind: 'contact', events: 55 })
    expect(model.edges.filter(e => e.kind === 'observed')).toHaveLength(8)
  })
  it('infers shared-contact links between subscribers, never through a service or a hub', () => {
    const inferred = inferSharedContacts(model)
    const pair = (a, b) => inferred.find(e => (e.source === a && e.target === b) || (e.source === b && e.target === a))
    expect(pair('923000001000', '923400001001').count).toBe(2)   // 010 and 011   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    expect(pair('923400001001', '923300001002').count).toBe(1)   // 011 only: the hub is left out   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    expect(pair('923000001000', '923300001002').count).toBe(1)   // real-data-ok: an invented number in a carrier's range, for the carrier tests
  })
  it('lists the contacts two numbers have in common, strongest first', () => {
    expect(sharedContacts(model, '923000001000', '923400001001').map(c => c.id)).toEqual(['923000001010', '923000001011'])   // real-data-ok: an invented number in a carrier's range, for the carrier tests
  })
  it('finds the shortest connection and does not pass through a hub or a service', () => {
    expect(shortestPath(model, '923000001000', '923300001002')).toEqual(['923000001000', '923300001002'])   // the inferred link is the shortest   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    const noInferred = { ...model, edges: model.edges.filter(e => e.kind === 'observed') }
    expect(shortestPath(noInferred, '923000001000', '923300001002')).toEqual(['923000001000', '923000001011', '923300001002'])   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    expect(shortestPath(noInferred, '923000001000', '923300001002', 1)).toBeNull()   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    const hubOnly = buildModel({ subjects: response.subjects.slice(1), edges: [edge('923400001001', '923000001099', 4, { is_hub: true }), edge('923300001002', '923000001099', 3, { is_hub: true })] })   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    expect(shortestPath({ ...hubOnly, edges: hubOnly.edges.filter(e => e.kind === 'observed') }, '923400001001', '923300001002')).toBeNull()   // real-data-ok: an invented number in a carrier's range, for the carrier tests
  })
})

describe('filters', () => {
  const model = buildModel(response)
  it('shows everything when nothing is chosen, and the neighbourhood of a target', () => {
    const all = filterModel(model, emptyFilters())
    expect(all.nodes.map(n => n.id).sort()).toEqual(model.nodes.filter(n => n.kind !== 'service').map(n => n.id).sort())   // only the services are left out by default
    const f = emptyFilters(); f.targets = new Set(['923000001000'])
    const shown = filterModel(model, f)
    expect(shown.nodes.filter(n => n.kind === 'subject').map(n => n.id)).toEqual(['923000001000'])
    expect(shown.nodes.map(n => n.id).sort()).toEqual(['923000001000', '923000001010', '923000001011'].sort())   // the service is hidden by default
    expect(summarise(model, shown).shown.subjects).toBe(1)
  })
  it('filters by carrier: the SIM carrier of the subjects and the range carrier of the others', () => {
    const f = emptyFilters(); f.carriers = new Set(['telenor'])
    const shown = filterModel(model, f)
    expect(shown.nodes.filter(n => n.kind === 'subject').map(n => n.id)).toEqual(['923400001001'])   // real-data-ok: an invented number in a carrier's range, for the carrier tests
    expect(shown.edges.every(e => e.kind !== 'observed' || shown.nodes.find(n => n.id === e.target).kind === 'subject' || shown.nodes.find(n => n.id === e.target).carrier === 'telenor')).toBe(true)
  })
  it('filters by period and hides the inferred links for a period', () => {
    const f = emptyFilters(); f.from = '2026-02-25'
    const shown = filterModel(model, f)
    expect(shown.edges.filter(e => e.kind === 'observed').map(e => e.target)).toEqual(['923000001010'])
    expect(shown.edges.some(e => e.kind === 'inferred')).toBe(false)
  })
  it('shows services only when asked, and keeps links with at least a number of events', () => {
    const f = emptyFilters(); f.show.services = true
    expect(filterModel(model, f).nodes.some(n => n.id === '786')).toBe(true)
    const g = emptyFilters(); g.minEvents = 20
    expect(filterModel(model, g).edges.filter(e => e.kind === 'observed').every(e => e.count >= 20)).toBe(true)
  })
  it('round-trips through the address', () => {
    const f = emptyFilters()
    f.targets = new Set(['923000001000']); f.carriers = new Set(['jazz', 'zong']); f.from = '2026-02-01'; f.to = '2026-02-28'; f.minEvents = 5; f.show.hubs = false; f.show.services = true
    const search = filtersToSearch(f, '?question=keep')
    expect(search).toContain('question=keep')
    const back = filtersFromSearch(search)
    expect([...back.targets]).toEqual(['923000001000'])
    expect([...back.carriers]).toEqual(['jazz', 'zong'])
    expect(back).toMatchObject({ from: '2026-02-01', to: '2026-02-28', minEvents: 5 })
    expect(back.show).toMatchObject({ hubs: false, services: true, contacts: true })
    expect(filtersFromSearch('?t=0300%201000001').targets.size).toBe(1)
    expect(filtersFromSearch('?from=junk&min=-3').from).toBe('')
  })
})

describe('a graph from the rows of an answer', () => {
  const rows = [{ src: '923000001000', dst: '923000001010' }, { src: '923000001010', dst: '923000001020' }, { src: '923000001000', dst: '923000001010' }, { src: '923000001020', dst: '923000001020' }, { src: null, dst: 'x' }]
  it('draws each distinct link once and the starting number as a subject', () => {
    const m = modelFromRows(rows)
    expect(m.edges).toHaveLength(2)
    expect(m.nodes.map(n => n.id)).toEqual(['923000001000', '923000001010', '923000001020'])
    expect(m.subjects).toEqual(['923000001000'])
  })
  it('uses a weight column when it is there', () => {
    expect(modelFromRows([{ src: 'a', dst: 'b', w: 7 }], { weight: 'w' }).edges[0].count).toBe(7)
  })
})

describe('layout and overview', () => {
  it('lays a graph out the same way every time, inside the box', () => {
    const model = buildModel(response)
    const a = layoutGraph(model.nodes, model.edges), b = layoutGraph(model.nodes, model.edges)
    expect([...a.entries()]).toEqual([...b.entries()])
    for (const p of a.values()) { expect(Math.abs(p.x)).toBeLessThanOrEqual(451); expect(Math.abs(p.y)).toBeLessThanOrEqual(351) }
  })
  it('groups the contacts of a big graph by community and opens a group on request', () => {
    const nodes = [{ id: 's', kind: 'subject' }]; const edges = []
    for (const g of ['a', 'b']) for (let i = 0; i < 4; i++) { nodes.push({ id: `${g}${i}`, kind: 'contact' }); for (let j = 0; j < i; j++) edges.push({ source: `${g}${i}`, target: `${g}${j}`, kind: 'observed', count: 5 }) }
    edges.push({ source: 's', target: 'a0', kind: 'observed', count: 1 })
    const label = communities(nodes, edges)
    const collapsed = collapse(nodes, edges, label)
    expect(collapsed.groups.length).toBe(2)
    expect(collapsed.nodes.some(n => n.kind === 'group')).toBe(true)
    const opened = collapse(nodes, edges, label, { expanded: new Set(collapsed.groups.map(g => g.id)) })
    expect(opened.groups.length).toBe(0)
  })
})
