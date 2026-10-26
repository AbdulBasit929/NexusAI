import { describe, expect, it } from 'vitest'
import connections from '../__fixtures__/wb/connections.json'
import places from '../__fixtures__/wb/places.json'
import connected from '../__fixtures__/wb/answers/connected.json'
import together from '../__fixtures__/wb/answers/together.json'
import meeting from '../__fixtures__/wb/answers/meeting.json'
import successor from '../__fixtures__/wb/answers/successor.json'
import move from '../__fixtures__/wb/answers/move.json'
import speed from '../__fixtures__/wb/answers/speed.json'
import rhythm from '../__fixtures__/wb/answers/rhythm.json'
import ranked from '../__fixtures__/wb/answers/ranked.json'
import unsupported from '../__fixtures__/wb/answers/unsupported.json'
import abstain from '../__fixtures__/engine_abstain.json'
import { createCaseIndex, clockOf, dateOf, looksLikeNumber, minutesBetween, shortNumber } from './caseIndex.js'
import { isDrawable, readAnswer, sortLimitations, strengthOfViews, numbersOfQuestion } from './answerMarks.js'
import { presentInvestigationResponse } from '../investigationPresentation.js'
import { graphOf, lanesOf, mapOf, periodsOf, placementNeeded, positioner, suggestionsFor, visibleMarks, workingSetOf, PLACEMENT } from './workingSet.js'
import { drawerOf, steppable } from './inspector.js'

// The fixtures are the REAL reads and answers of the live service for the SYNTHETIC development case 2 of the generator (nexusai-wave1-dev2-20261021), recorded on 2026-10-26.
// The visits are kept for three subscribers only (1003, 1007, 1011) to keep the file small; every other number is as the service returned it.
const index = createCaseIndex({ connections, places })
const Q = {
  connected: 'How are 923000001003 and 923000001007 connected?',
  together: 'When were 923000001007 and 923000001011 at the same cell within 22 minutes of each other?',
  meeting: 'Which of the times 923000001003 and 923000001007 were at the same cell within 30 minutes could have been a meeting, given a call or SMS between them in the previous 90 minutes?',
  successor: 'Which numbers may have taken over from 923000001005?',
  move: 'Did 923000001009 move house? Give the date the new place starts and how far the move was.',
  speed: 'Which consecutive events of 923000001002 imply travel faster than 200 km/h?',
  rhythm: 'Did the daily activity of 923000001006 change for good? From which day, and what were the daily averages before and after?',
  ranked: 'Who does 923000001000 contact most? Give the top five.',
}
const read = (response, key) => readAnswer(response, { index, question: Q[key], caseId: 'nexusai-wave1-dev2-20261021' })

describe('the index of a case', () => {
  it('reads the case clock and the days from the records themselves', () => {
    expect(index.offset).toBe('+05:00')
    expect(index.startDate).toBe('2026-02-02')
    expect(index.endDate).toBe('2026-04-02')
    expect(index.days).toBe(60)
    expect(index.dayIndex('2026-02-09 08:36:08 PKT')).toBe(7)
    expect(index.dayLabel(35)).toBe('2026-03-09')
    // a browser in another zone must not move a day: the date is read from the text
    expect(dateOf('2026-02-02T00:23:31+05:00')).toBe('2026-02-02')
    expect(clockOf('2026-02-09 08:36:08 PKT')).toBe('2026-02-09 08:36')
    expect(minutesBetween('2026-02-09 08:36:08 PKT', '2026-02-09 08:52:11 PKT')).toBe(16)
  })

  it('knows the subscribers, their contacts and what is left out of a picture', () => {
    expect(index.subjects).toHaveLength(12)
    expect(index.subjectsByEvents[0].number).toBe('923000001006')
    expect(index.isSubject('923000001003')).toBe(true)
    const e = index.edgeBetween('923000001003', '923000001007')
    expect(e).toBeTruthy()
    expect(index.edgeBetween('923000001007', '923000001003')).toBe(e)
    const strongest = index.strongest('923000001003', 4)
    expect(strongest).toHaveLength(4)
    expect(strongest.every(n => !index.excludedKind(n))).toBe(true)
    const excluded = index.excludedFor(['923000001003', '923000001007'])
    expect(excluded.length).toBeGreaterThan(0)
    expect(excluded.every(x => index.excludedKind(x.number))).toBe(true)
    expect(index.sharedContacts('923000001003', '923000001007').every(s => !index.excludedKind(s.number))).toBe(true)
  })

  it('counts the events of a number a day at the cell sites it used, from the hourly visits', () => {
    const lane = index.laneOf('923000001007')
    const total = places.visits.filter(v => v.subject === '923000001007').reduce((s, v) => s + v.events, 0)
    expect(lane).toHaveLength(60)
    expect(lane.reduce((a, b) => a + b, 0)).toBe(total)
    expect(index.laneOf('923000001001').every(v => v === 0)).toBe(true) // its visits are not in this trimmed fixture, and the lane says so with zeros, not with a guess
  })

  it('finds a number from a part of it', () => {
    expect(index.find('1007')).toContain('923000001007')
    expect(index.find('12')).toEqual([])
  })

  it('writes a number short and tells numbers from codes', () => {
    expect(shortNumber('923000001007')).toBe('…007')
    expect(looksLikeNumber('923000001007')).toBe(true)
    expect(looksLikeNumber('8888')).toBe(false)
    expect(numbersOfQuestion(Q.together)).toEqual(['923000001007', '923000001011'])
  })
})

describe('the engine answers, read as marks', () => {
  it('co-location: four windows, a dashed link between the pair named in the question, inferred, with the window the engine used', () => {
    const a = read(together, 'together')
    expect(a.state).toBe('answered')
    expect(a.kind).toBe('window')
    expect(a.marks).toHaveLength(4)
    expect(a.marks.map(m => m.numbers)).toEqual(Array(4).fill(['923000001007', '923000001011']))
    expect(a.marks.map(m => m.day)).toEqual([7, 9, 14, 29])
    expect(a.links).toHaveLength(1)
    expect(a.links[0]).toMatchObject({ kind: 'colocated', events: 4 })
    expect(a.links[0].markIds).toHaveLength(4)
    expect(a.strength).toBe('inferred')
    expect(a.values).toEqual([{ label: 'Co-location window', value: '22 minutes', source: 'from your question' }])
    expect(a.repeats).toBe(72)
    expect(a.caveats.some(c => /^Proximity is not association/.test(c))).toBe(true)
    expect(a.caveats.some(c => /^A cell site is a coverage area/.test(c))).toBe(true)
    expect(a.caveats.every(c => !/ used: /.test(c) && !/^Computed by a model-written query/.test(c))).toBe(true)
    expect(a.layers).toEqual(['windows'])
    expect(a.sites).toEqual(['?:551:10324679'])
    expect(a.table.columns).toContain('Start utc') // the column's name as the service gives it; the times in it are on the case clock
    expect(a.table.rows[0].join(' ')).toContain('2026-02-09 08:36')
  })

  it('meetings: two candidates, weakest strength, both rules read from the question', () => {
    const a = read(meeting, 'meeting')
    expect(a.kind).toBe('meeting')
    expect(a.marks).toHaveLength(2)
    expect(a.strength).toBe('candidate')
    expect(a.values.map(v => [v.value, v.source])).toEqual([['30 minutes', 'from your question'], ['90 minutes', 'from your question']])
    expect(a.marks[1].facts).toEqual(expect.arrayContaining([['Gap to that contact', '18 min'], ['Score', '0.9']]))
    expect(a.links[0].kind).toBe('meeting')
  })

  it('successor: the old number comes from the question when the query leaves the column out; defaults are labelled as defaults', () => {
    const a = read(successor, 'successor')
    expect(a.kind).toBe('successor')
    expect(a.marks).toHaveLength(1)
    expect(a.marks[0].numbers).toEqual(['923000001005', '923000001001'])
    expect(a.marks[0].day).toBe(index.dayIndex('2026-03-03 07:22:12 PKT'))
    expect(a.links[0]).toMatchObject({ kind: 'successor', source: '923000001005', target: '923000001001', events: 1 })
    expect(a.strength).toBe('candidate')
    expect(a.values.map(v => v.source)).toEqual(['the default', 'the default', 'the default'])
    expect(a.marks[0].facts).toEqual(expect.arrayContaining([['Score', '0.7905'], ['Shared handset', 'no']]))
  })

  it('a move, a speed alert and a change of rhythm are single-number marks with their own days, and the first two draw a track', () => {
    const m = read(move, 'move')
    expect(m.kind).toBe('move')
    expect(m.marks[0]).toMatchObject({ numbers: ['923000001009'], day: 35, cells: ['?:10286:10814', '?:10156:10444'] })
    expect(m.tracks).toHaveLength(1)
    expect(m.strength).toBe('inferred')
    expect(m.marks[0].facts).toEqual(expect.arrayContaining([['Distance', '6.29 km'], ['New place from the week of', '2026-03-09']]))

    const s = read(speed, 'speed')
    expect(s.kind).toBe('alert')
    expect(s.marks[0]).toMatchObject({ numbers: ['923000001002'], day: 6 })
    expect(s.strength).toBe('observed')
    expect(s.values).toEqual([{ label: 'Lowest speed listed', value: '200 km/h', source: 'from your question' }])
    expect(s.marks[0].facts).toEqual(expect.arrayContaining([['Speed', '727.8 km/h'], ['Gap', '11 s']]))

    const r = read(rhythm, 'rhythm')
    expect(r.kind).toBe('rhythm')
    expect(r.marks[0]).toMatchObject({ numbers: ['923000001006'], day: 30, markLineDay: 30 })
    expect(r.marks[0].facts).toEqual(expect.arrayContaining([['Average before', '8.233 a day'], ['Average after', '18.267 a day']]))
  })

  it('a ranked list and a connection are links, emphasised, and draw no mark the rows do not carry', () => {
    const k = read(ranked, 'ranked')
    expect(k.kind).toBe('contact')
    expect(k.marks).toEqual([])
    expect(k.answerNumbers).toEqual(expect.arrayContaining(['923000001000', '923000001018', '923000001102']))
    expect(k.emph).toHaveLength(5)
    const c = read(connected, 'connected')
    expect(c.kind).toBe('path')
    expect(c.emph).toEqual(['923000001003|923000001007'])
    expect(c.strength).toBe('inferred')
  })

  it('sorts the engine\'s limitations into values, care, and the notes about the records', () => {
    const sorted = sortLimitations([
      'Computed by a model-written query over this case\'s analyst views.',
      '72 rows of this case\'s call and internet records are exact repeats of another row and are left out of these counts.',
      'Times are read and shown on the case clock (PKT, UTC+05:00).',
      'Only the first 200 rows are shown; the query matched more.',
      'Weeks a new anchor must hold to count used: 2 weeks (the default, because the question names none).',
      'A cell site is a coverage area, not the position of a person.',
    ])
    expect(sorted).toEqual({ values: [{ label: 'Weeks a new anchor must hold to count', value: '2 weeks', source: 'the default' }], caveats: ['A cell site is a coverage area, not the position of a person.'], repeats: 72, clock: 'PKT, UTC+05:00', truncated: 'Only the first 200 rows are shown; the query matched more.' })
  })

  it('takes the weakest strength among the views an answer read', () => {
    expect(strengthOfViews(['v_contact_pairs'])).toBe('observed')
    expect(strengthOfViews(['v_contact_pairs', 'v_colocation_windows'])).toBe('inferred')
    expect(strengthOfViews(['v_colocation_windows', 'v_meeting_candidates'])).toBe('candidate')
  })

  it('keeps the engine\'s own words when it did not answer, and says which designed state it is', () => {
    const none = readAnswer(unsupported, { index, question: 'Which of these people is the most dangerous?' })
    expect(none.state).toBe('clarify')
    expect(none.sentence).toMatch(/^I found nothing, but I only searched one kind of evidence/)
    expect(none.marks).toEqual([])
    const refused = readAnswer(abstain, { index, question: 'x' })
    expect(['abstained', 'clarify', 'broad']).toContain(refused.state)
    const failed = readAnswer(null, { index, question: 'x', error: Object.assign(new Error('x'), { reference: 'HTTP-502' }) })
    expect(failed.state).toBe('failed')
    expect(failed.reference).toBe('HTTP-502')
  })

  it('says "no row matches" as an empty result, not as a picture', () => {
    const empty = structuredClone(together)
    empty.enterprise.data_grid.rows = []
    empty.enterprise.row_count = 0
    empty.enterprise.executive_answer = 'No row matches what you asked.'
    const a = read(empty, 'together')
    expect(a.state).toBe('zero-result')
    expect(a.marks).toEqual([])
  })
})

describe('the working set and what is drawn from it', () => {
  it('starts from the subscribers when nothing is asked, and from the question when it is', () => {
    const start = workingSetOf(index)
    expect(start.filter(n => index.isSubject(n))).toHaveLength(12)
    const joined = start.filter(n => !index.isSubject(n))
    expect(joined.length).toBeGreaterThan(0)
    expect(joined.length).toBeLessThanOrEqual(24)
    expect(joined.every(n => !index.excludedKind(n))).toBe(true)
    // each of them is a contact of at least two subscribers: that is why it is there
    const subscribers = new Set(index.subjects.map(s => s.number))
    expect(joined.every(n => new Set(index.edges.filter(e => (e.target === n && subscribers.has(e.source)) || (e.source === n && subscribers.has(e.target))).map(e => (e.source === n ? e.target : e.source))).size >= 2)).toBe(true)
    const a = read(together, 'together')
    const set = workingSetOf(index, { focus: a.numbers, answerNumbers: a.answerNumbers })
    expect(set).toEqual(expect.arrayContaining(a.numbers))
    expect(set.length).toBeLessThanOrEqual(12)
    expect(workingSetOf(index, { focus: a.numbers, expanded: [a.numbers[0]] }).length).toBeGreaterThanOrEqual(set.length)
  })

  it('never makes a service or a hub a member unless the question names it', () => {
    const set = workingSetOf(index, { focus: ['923000001003', '923000001007'] })
    expect(set.filter(n => !index.isSubject(n)).every(n => !index.excludedKind(n))).toBe(true)
  })

  it('draws the observed links among the members and the answer\'s links over them, and the brush removes what lies outside it', () => {
    const a = read(together, 'together')
    const members = workingSetOf(index, { focus: a.numbers, answerNumbers: a.answerNumbers })
    const all = graphOf(index, members, { focus: new Set(a.numbers), answer: a, layers: {}, range: null })
    expect(all.nodes.map(n => n.id).sort()).toEqual([...members].sort())
    expect(all.nodes.find(n => n.id === '923000001007')).toMatchObject({ focus: true, subject: true, label: '…007' })
    expect(all.links.filter(l => l.kind === 'colocated')).toHaveLength(1)
    const early = graphOf(index, members, { focus: new Set(a.numbers), answer: a, layers: {}, range: [0, 8] })
    expect(early.links.find(l => l.kind === 'colocated').events).toBe(1) // days 7, 9, 14 and 29: only the first lies in the first nine days
    const ninth = graphOf(index, members, { focus: new Set(a.numbers), answer: a, layers: {}, range: [0, 10] })
    expect(ninth.links.find(l => l.kind === 'colocated').events).toBe(2)
    const none = graphOf(index, members, { focus: new Set(a.numbers), answer: a, layers: {}, range: [40, 59] })
    expect(none.links.filter(l => l.kind === 'colocated')).toHaveLength(0)
    const off = graphOf(index, members, { focus: new Set(a.numbers), answer: a, layers: { windows: false }, range: null })
    expect(off.links.filter(l => l.kind === 'colocated')).toHaveLength(0)
  })

  it('filters the marks by layer and by period', () => {
    const a = read(together, 'together')
    expect(visibleMarks(a, {}, null)).toHaveLength(4)
    expect(visibleMarks(a, {}, [0, 20]).map(m => m.day)).toEqual([7, 9, 14])
    expect(visibleMarks(a, { windows: false }, null)).toEqual([])
    expect(visibleMarks(null)).toEqual([])
  })

  it('puts the generator\'s grid over a city when it is not on the basemap, with the same relative distances, and leaves real positions alone', () => {
    expect(placementNeeded(index)).toBe(true)
    const as = positioner(index, 'recorded')
    const over = positioner(index, 'city')
    const [s1, s2] = index.sites
    const [p1, p2] = [over(s1), over(s2)]
    expect(p1.lat - p2.lat).toBeCloseTo(s1.lat - s2.lat, 9)
    expect(p1.lon - p2.lon).toBeCloseTo(s1.lon - s2.lon, 9)
    expect(Math.abs(p1.lat - PLACEMENT.lat)).toBeLessThan(1)
    expect(as(s1)).toEqual({ lat: s1.lat, lon: s1.lon })
    const real = createCaseIndex({ connections, places: { ...places, sites: places.sites.map(s => ({ ...s, lat: 31.5 + (s.lat * 0.01), lon: 74.3 + (s.lon * 0.001) })) } })
    expect(placementNeeded(real)).toBe(false)
  })

  it('draws the cell sites the working set used, the sites the marks name, and the track of a move', () => {
    const a = read(move, 'move')
    const members = workingSetOf(index, { focus: a.numbers })
    const map = mapOf(index, members, { answer: a, layers: {}, range: null, position: positioner(index, 'city') })
    expect(map.sites.some(s => s.id === '?:10286:10814' && s.marked)).toBe(true)
    expect(map.sites.some(s => s.id === '?:10156:10444' && s.marked)).toBe(true)
    expect(map.track).toHaveLength(2)
    expect(map.sites.every(s => Math.abs(s.lat - PLACEMENT.lat) < 1)).toBe(true)
    const hidden = mapOf(index, members, { answer: a, layers: { moves: false }, range: null, position: positioner(index, 'city') })
    expect(hidden.track).toHaveLength(0)
  })

  it('gives each number of the question a lane, and the subscribers together a lane when nothing is asked', () => {
    const a = read(together, 'together')
    expect(lanesOf(index, { focus: a.numbers }).map(l => l.id)).toEqual(['923000001007', '923000001011'])
    const all = lanesOf(index, { focus: [], members: workingSetOf(index) })
    expect(all).toHaveLength(1)
    expect(all[0].values).toHaveLength(60)
    expect(periodsOf(index).map(p => p.range)).toEqual([null, [30, 59], [53, 59]])
  })

  it('suggests only questions of forms the engine is known to answer, with whole numbers in them', () => {
    const s = suggestionsFor(index, { focus: ['923000001003', '923000001007'] })
    expect(s.map(x => x.id)).toEqual(['connected', 'contacts', 'sites', 'together', 'meeting', 'successor', 'move', 'speed', 'rhythm'])
    expect(s.every(x => /\d{12}/.test(x.question))).toBe(true)
    expect(suggestionsFor(index, {}).length).toBeGreaterThan(5)
  })
})

describe('the evidence panel', () => {
  it('opens a number, a link and a site from the live reads', () => {
    const node = drawerOf({ type: 'node', id: '923000001003' }, { index })
    expect(node.strength).toBe('observed')
    expect(node.facts[0]).toEqual(['Events', '590'])
    expect(node.rows.length).toBe(8)
    expect(node.rowTargets[0]).toEqual({ type: 'node', id: index.strongest('923000001003', 8)[0] })
    const edge = drawerOf({ type: 'edge', a: '923000001003', b: '923000001007' }, { index })
    expect(edge.facts.map(f => f[0])).toEqual(['Events', 'Outgoing', 'Incoming', 'Call time', 'First', 'Last'])
    const site = drawerOf({ type: 'site', id: '?:551:10324679' }, { index })
    expect(site.caveats[0]).toMatch(/coverage area/)
    expect(site.rows.length).toBeGreaterThan(0)
  })

  it('opens a mark with its own facts, the engine\'s caveats and the rows of its kind, the selected one picked', () => {
    const a = read(together, 'together')
    const d = drawerOf({ type: 'mark', id: 'window:2' }, { index, answer: a, range: null })
    expect(d.strength).toBe('inferred')
    expect(d.pick).toBe(2)
    expect(d.rows).toHaveLength(4)
    expect(d.columns).toEqual(['Start', 'End', 'Length'])
    expect(d.facts[0][0]).toBe('Start')
    expect(d.caveats.length).toBeGreaterThan(1)
    expect(d.values[0].value).toBe('22 minutes')
    const inRange = drawerOf({ type: 'mark', id: 'window:2' }, { index, answer: a, range: [0, 20] })
    expect(inRange.rows).toHaveLength(3)
  })

  it('steps through the marks in the order of time', () => {
    const a = read(meeting, 'meeting')
    expect(steppable([...a.marks].reverse()).map(m => m.id)).toEqual(['meeting:0', 'meeting:1'])
  })
})

describe('which answers are drawn', () => {
  it('draws what names a mark, a link or a cell site, and nothing else', () => {
    for (const key of ['together', 'meeting', 'successor', 'move', 'speed', 'rhythm', 'ranked', 'connected']) expect(isDrawable({ together, meeting, successor, move, speed, rhythm, ranked, connected }[key], Q[key]), key).toBe(true)
    const count = structuredClone(rhythm)
    count.enterprise.data_grid = { columns: [{ header: 'Value', key: 'value' }], rows: [{ value: 3 }], count: 1 }
    count.enterprise.derivation = { ...count.enterprise.derivation, views: ['v_comm_events'] }
    count.answer.ask_engine_v2.columns = ['value']
    expect(isDrawable(count, 'How many events does 923000001006 have?')).toBe(false)
    expect(isDrawable(unsupported, 'x')).toBe(false)
    expect(isDrawable(null, 'x')).toBe(false)
  })

  it('shows every limitation the engine states, so that the value it used is never the line that is cut off', () => {
    const limitations = Array.from({ length: 6 }, (_, i) => `Limitation number ${i + 1} of the engine.`)
    const response = structuredClone(together)
    response.enterprise.limitations = limitations
    expect(presentInvestigationResponse(response, { caseId: 'c' }).limitations).toEqual(limitations)
    expect(presentInvestigationResponse(together, { caseId: 'c' }).limitations.at(-1)).toMatch(/Co-location window used: 22 minutes \(read from your question\)/)
  })
})
