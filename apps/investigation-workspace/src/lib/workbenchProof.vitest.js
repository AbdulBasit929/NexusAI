import { describe, expect, it } from 'vitest'
import { CAVEATS, DATA, PRESETS, STATES, drawerFor, excludedFor, graphOf, mapOf, marksOf, placed, positionOf, sharedContacts, short, tableFor, windowsOf, workingSet } from './workbenchProof.js'

// The workbench direction proof reads a synthetic fixture; these tests hold it to what the generator's oracle says about development case 2 (the same numbers the demonstration of stage 3 asked for).
describe('the fixture of the workbench proof', () => {
  it('is the synthetic development case, 12 numbers over 60 days', () => {
    expect(DATA.synthetic).toBe(true)
    expect(DATA.subjects).toHaveLength(12)
    expect(DATA.days).toBe(60)
    expect(DATA.subjects.every(s => /^92300000\d{4}$/.test(s.number))).toBe(true)
  })

  it('holds the oracle\'s answers of stage 3', () => {
    expect(windowsOf('923000001007', '923000001011')).toHaveLength(4)
    expect(DATA.meetings30_90.filter(m => m.a === '923000001003' && m.b === '923000001007')).toHaveLength(2)
    expect(DATA.successors.find(s => s.old === '923000001005').candidates[0]).toMatchObject({ number: '923000001001', score: 0.7905 })
    expect(DATA.moves).toEqual([expect.objectContaining({ number: '923000001009', first_week_start: '2026-03-09', distance_km: 6.29 })])
    expect(DATA.speed200.filter(a => a.number === '923000001002')).toHaveLength(1)
    expect(DATA.rhythm).toEqual([expect.objectContaining({ number: '923000001006', first_day: '2026-03-04', mean_before: 8.233, mean_after: 18.267, direction: 'up' })])
  })
})

describe('the question presets', () => {
  it('say in a sentence what the demonstration of the stage answered', () => {
    const said = Object.fromEntries(PRESETS.map(p => [p.id, p.sentence()]))
    expect(said.together).toMatch(/^4 times, between 2026-02-09 and 2026-03-03, at 1 cell site\.$/)
    expect(said.meeting).toMatch(/^2 possible meetings \(candidates, not findings\)/)
    expect(said.successor).toMatch(/…001 may have taken over from …005 \(score 0\.79\)/)
    expect(said.move).toMatch(/week of 2026-03-09, 6\.29 km away/)
    expect(said.speed).toMatch(/727\.8 km\/h/)
    expect(said.rhythm).toMatch(/from 2026-03-04 the daily average rose from 8\.233 to 18\.267/)
  })

  it('every preset names a caveat of the engine and values that are shown as read from the question or the default', () => {
    for (const p of PRESETS) {
      expect(CAVEATS[p.caveat]).toBeTruthy()
      for (const param of p.params) expect(['from your question', 'the default']).toContain(param.source)
      expect(drawerFor(p.select, p, null)).not.toBeNull()
    }
  })
})

describe('the working set', () => {
  it('starts small: the numbers of the question and their strongest neighbours', () => {
    for (const p of PRESETS) {
      const members = workingSet(p)
      expect(members.length).toBeLessThanOrEqual(10)
      for (const n of p.focus) expect(members).toContain(n)
    }
  })

  it('grows by one hop, deliberately', () => {
    const p = PRESETS[0]
    expect(workingSet(p, [p.focus[0]]).length).toBeGreaterThanOrEqual(workingSet(p).length)
  })

  it('shows services and numbers everyone contacts as excluded, never as members', () => {
    const p = PRESETS[0]
    const excluded = excludedFor(workingSet(p))
    const members = workingSet(p)
    expect(excluded.length).toBeGreaterThan(0)
    for (const e of excluded) expect(members).not.toContain(e.number)
    expect(sharedContacts('923000001003', '923000001007').every(n => !excluded.some(e => e.number === n))).toBe(true)
  })
})

describe('the marks and the brush', () => {
  const together = PRESETS.find(p => p.id === 'together')

  it('draws the windows as marks of the timeline and as a link of the graph', () => {
    const layers = { contacts: true, windows: true }
    expect(marksOf(together, layers, null).filter(m => m.kind === 'window')).toHaveLength(4)
    const graph = graphOf(together, workingSet(together), layers, null)
    expect(graph.links.filter(l => l.kind === 'colocated')).toHaveLength(1)
  })

  it('filters every pane by the brushed period', () => {
    const layers = { contacts: true, windows: true }
    const range = [0, 20]   // 2026-02-02 to 2026-02-22: three of the four windows (02-09, 02-11 and 02-16; the fourth is 03-03)
    expect(windowsOf('923000001007', '923000001011', range)).toHaveLength(3)
    expect(drawerFor(together.select, together, range).rows.length).toBe(windowsOf('923000001007', '923000001011', range).length)
    const empty = graphOf(together, workingSet(together), layers, [58, 60])
    expect(empty.links.filter(l => l.kind === 'colocated')).toHaveLength(0)
  })

  it('puts the synthetic grid over a city and says where, with the same relative distances', () => {
    const [a, b] = ['?:10286:10814', '?:10156:10444'].map(positionOf)
    const [pa, pb] = [placed(a), placed(b)]
    expect(pa.lat - pb.lat).toBeCloseTo(a.lat - b.lat, 9)
    expect(pa.lon - pb.lon).toBeCloseTo(a.lon - b.lon, 9)
    expect(pa.lat).toBeGreaterThan(30)
    expect(mapOf(PRESETS.find(p => p.id === 'move'), { moves: true }, null).track).toHaveLength(2)
  })
})

describe('the drawer and the table twin', () => {
  it('opens the evidence of a mark with its facts, the engine\'s caveat and its rows, which are the table twin\'s', () => {
    for (const p of PRESETS) {
      const d = drawerFor(p.select, p, null)
      expect(d.facts.length).toBeGreaterThan(0)
      expect(d.columns.length).toBe(d.rows[0]?.length ?? d.columns.length)
      expect(tableFor(p, p.layers, null).rows).toEqual(d.rows)
    }
  })

  it('writes a number short and keeps the full one for the title attribute', () => {
    expect(short('923000001007')).toBe('…007')
  })

  it('has the four designed states of the contract', () => {
    for (const id of ['empty', 'broad', 'abstained', 'partial']) {
      expect(STATES[id].title).toBeTruthy()
      expect(STATES[id].actions.length).toBeGreaterThan(0)
    }
  })
})
