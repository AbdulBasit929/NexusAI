// The link-analysis workbench, direction proof (docs/ux/workbench/RESEARCH_NOTE.md). Pure functions over the SYNTHETIC fixture of development case 2: the working set of a question, the marks it
// draws, what the drawer says about a mark. No network, no model; every number shown comes from the fixture (the connections and places reads of the stored synthetic case and the answers of the
// link-analysis stage as the generator's oracle states them), so the proof is checkable and the same state gives the same picture.
import fixture from './__fixtures__/workbench_case2.json'

export const DATA = fixture
export const short = number => `…${String(number).slice(-3)}`
const day = iso => Math.round((Date.parse(`${String(iso).slice(0, 10)}T00:00:00Z`) - Date.parse(`${fixture.start}T00:00:00Z`)) / 86400000)
export const dayIndex = day
export const dayLabel = index => new Date(Date.parse(`${fixture.start}T00:00:00Z`) + index * 86400000).toISOString().slice(0, 10)
const minutes = (a, b) => Math.max(0, Math.round((Date.parse(String(b).replace(' ', 'T') + 'Z') - Date.parse(String(a).replace(' ', 'T') + 'Z')) / 60000))

const subjects = new Set(fixture.subjects.map(s => s.number))
const edgeKey = (a, b) => (a < b ? `${a}|${b}` : `${b}|${a}`)
const edges = fixture.connections.edges.filter(e => e.source !== e.target)
const byPair = new Map(edges.map(e => [edgeKey(e.source, e.target), e]))
const neighbours = new Map()
for (const e of edges) {
  for (const [a, b] of [[e.source, e.target], [e.target, e.source]]) {
    if (!neighbours.has(a)) neighbours.set(a, [])
    neighbours.get(a).push({ number: b, edge: e })
  }
}

export const isSubject = number => subjects.has(number)
export const edgeBetween = (a, b) => byPair.get(edgeKey(a, b)) || null
export const subjectInfo = number => fixture.connections.subjects.find(s => s.number === number) || { number, events: 0, contacts: 0 }

// The contacts two numbers have in common, services and numbers that everyone contacts left out (they are listed as excluded, never hidden).
export function sharedContacts(a, b) {
  const mine = new Set((neighbours.get(a) || []).filter(n => !n.edge.is_service && !n.edge.is_hub).map(n => n.number))
  const common = (neighbours.get(b) || []).filter(n => mine.has(n.number) && !n.edge.is_service && !n.edge.is_hub).map(n => n.number)
  return [...new Set(common)].filter(n => n !== a && n !== b)
}
export function excludedFor(numbers) {
  const out = new Map()
  for (const number of numbers) for (const n of neighbours.get(number) || []) if (n.edge.is_service || n.edge.is_hub) out.set(n.number, n.edge.is_service ? 'service' : 'hub')
  return [...out].map(([number, why]) => ({ number, why }))
}
// The strongest neighbours of a number (people, not services or hubs), by events, at most `limit`.
export function strongest(number, limit = 4) {
  return (neighbours.get(number) || []).filter(n => !n.edge.is_service && !n.edge.is_hub).sort((x, y) => y.edge.events - x.edge.events).slice(0, limit).map(n => n.number)
}

const inRange = (iso, range) => !range || (day(iso) >= range[0] && day(iso) <= range[1])
export const windowsOf = (a, b, range) => fixture.windows22.filter(w => ((w.a === a && w.b === b) || (w.a === b && w.b === a)) && inRange(w.start, range))
export const meetingsOf = (a, b, range) => fixture.meetings30_90.filter(w => ((w.a === a && w.b === b) || (w.a === b && w.b === a)) && inRange(w.start, range))
export const successorsOf = number => fixture.successors.find(s => s.old === number) || null
export const moveOf = number => fixture.moves.find(m => m.number === number) || null
export const alertsOf = number => fixture.speed200.filter(a => a.number === number)
export const rhythmOf = number => fixture.rhythm.find(r => r.number === number) || null
export const positionOf = cell => fixture.positions[cell] || null
// Decision D16: the generated cases sit on the generator's own grid (in the ocean); the interface may place that grid over a city, and says so. The same relative distances, over Lahore.
const HOME = (() => { const s = fixture.sites; return { lat: s.reduce((a, b) => a + b.lat, 0) / s.length, lon: s.reduce((a, b) => a + b.lon, 0) / s.length } })()
export const PLACEMENT = { city: 'Lahore', lat: 31.5204, lon: 74.3587 }
export const placed = p => (p ? { lat: PLACEMENT.lat + (p.lat - HOME.lat), lon: PLACEMENT.lon + (p.lon - HOME.lon) } : null)

// What a question puts on the canvas. Each preset is a question an analyst asked in the demonstration of the stage; its sentence is the one the engine's answer says, in plain words.
export const CAVEATS = {
  connections: 'A link is a record of contact between two numbers, not a finding about the people. Services and numbers that everyone contacts are shown as excluded.',
  colocation: 'Proximity is not association: two numbers at the same cell site within the stated window of each other were in the same coverage area, which is not evidence that they met or know each other. No chance baseline is computed here.',
  meeting: 'A candidate, not a finding: proximity and contact are not proof of a meeting.',
  successor: 'Behavioural similarity, not identity: a new number that starts when another stops and shares its contacts, places or handset may have taken over from it, or may not.',
  move: 'Cell sites, not positions of a person: an anchor is the cell site with most events of a slot in a week, a coverage area, and a move is a change of that site.',
  speed: 'Cell sites, not positions of a person: two events at distant cell sites too close in time may be a handover between towers, a fault or a record error, and are not proof that anyone travelled that fast.',
  rhythm: 'A change in the pattern of use, not an explanation: the daily count of events changed, and the records do not say why.',
}

// The strength of what an answer shows (the UX contract, section 4): observed is in the records, inferred is derived from them by a stated rule, a candidate is a lead that proves nothing.
export const STRENGTH = {
  observed: { label: 'Observed', note: 'in the records' },
  inferred: { label: 'Inferred', note: 'derived from the records by a stated rule' },
  candidate: { label: 'Candidate', note: 'a lead, not a finding' },
}
// The quick periods of the time bar, as day indexes of the case.
export const PERIODS = [
  { id: 'all', label: 'All', range: () => null },
  { id: '30', label: '30 days', range: () => [Math.max(0, fixture.days - 30), fixture.days - 1] },
  { id: '7', label: '7 days', range: () => [Math.max(0, fixture.days - 7), fixture.days - 1] },
]
export const nf = new Intl.NumberFormat('en-GB')

export const PRESETS = [
  {
    id: 'connected', strength: 'observed', derivation: ['Read: the contacts of the case (v_contact_pairs), for the two numbers named.', 'Left out: services and numbers that everyone contacts (listed as excluded).', 'Found: their direct contact and the contacts they have in common.'], label: 'How are two numbers connected?', question: 'How are 923000001003 and 923000001007 connected?', focus: ['923000001003', '923000001007'], layers: { contacts: true },
    caveat: 'connections', select: { type: 'edge', a: '923000001003', b: '923000001007' },
    sentence: () => {
      const e = edgeBetween('923000001003', '923000001007'); const shared = sharedContacts('923000001003', '923000001007')
      return e ? `${short('923000001003')} and ${short('923000001007')} are in direct contact (${e.events} events, ${e.first.slice(0, 10)} to ${e.last.slice(0, 10)}) and share ${shared.length} contact${shared.length === 1 ? '' : 's'}.`
        : `${short('923000001003')} and ${short('923000001007')} are not in direct contact; ${shared.length} contacts in common.`
    },
    params: [],
  },
  {
    id: 'together', strength: 'inferred', derivation: ['Read: v_colocation_windows, for the two numbers named.', 'Window set from your question: 22 minutes (the engine, not the model, sets it).', 'Kept only the rows where both were at the same cell site within that window.', 'Checked: one read-only SELECT over the case’s analyst views.'], label: 'When were two numbers at the same cell?', question: 'When were 923000001007 and 923000001011 at the same cell within 22 minutes of each other?', focus: ['923000001007', '923000001011'],
    layers: { contacts: true, windows: true }, caveat: 'colocation', select: { type: 'window', index: 0, a: '923000001007', b: '923000001011' },
    sentence: () => {
      const w = windowsOf('923000001007', '923000001011'); const cells = new Set(w.map(x => x.cell_key))
      return `${w.length} times, between ${w[0].start.slice(0, 10)} and ${w[w.length - 1].start.slice(0, 10)}, at ${cells.size} cell site${cells.size === 1 ? '' : 's'}.`
    },
    params: [{ label: 'Window', value: '22 minutes', source: 'from your question' }],
  },
  {
    id: 'meeting', strength: 'candidate', derivation: ['Read: v_meeting_candidates, for the two numbers named.', 'Window 30 minutes and gap 90 minutes, both set from your question.', 'Kept the windows that follow a call or SMS between them within the gap.', 'Checked: one read-only SELECT over the case’s analyst views.'], label: 'Which of those could be a meeting?', question: 'Which of the times 923000001003 and 923000001007 were at the same cell within 30 minutes could have been a meeting, given a call or SMS between them in the previous 90 minutes?',
    focus: ['923000001003', '923000001007'], layers: { contacts: true, meetings: true }, caveat: 'meeting', select: { type: 'meeting', index: 0, a: '923000001003', b: '923000001007' },
    sentence: () => { const m = meetingsOf('923000001003', '923000001007'); return `${m.length} possible meetings (candidates, not findings): ${m.map(x => x.start.slice(0, 10)).join(' and ')}.` },
    params: [{ label: 'Window', value: '30 minutes', source: 'from your question' }, { label: 'Gap to the last contact', value: '90 minutes', source: 'from your question' }],
  },
  {
    id: 'successor', strength: 'candidate', derivation: ['Read: v_successor_candidates, for the old number named.', 'Silence 7 days, start window 14 days and lowest score 35 percent: the defaults, because the question names none.', 'Score = 0.6 contact similarity + 0.2 shared places + 0.2 shared handset.', 'Checked: one read-only SELECT over the case’s analyst views.'], label: 'Did a number change hands?', question: 'Which numbers may have taken over from 923000001005?', focus: ['923000001005', '923000001001'], layers: { contacts: true, successors: true }, caveat: 'successor',
    select: { type: 'successor', old: '923000001005', new: '923000001001' },
    sentence: () => { const c = successorsOf('923000001005').candidates[0]; return `${short(c.number)} may have taken over from ${short('923000001005')} (score ${c.score.toFixed(2)}).` },
    params: [{ label: 'Silence', value: '7 days', source: 'the default' }, { label: 'Start window', value: '14 days', source: 'the default' }, { label: 'Lowest score', value: '35 percent', source: 'the default' }],
  },
  {
    id: 'move', strength: 'inferred', derivation: ['Read: v_anchor_changes, for the number named.', 'A new night place must hold 2 weeks: the default, because the question names none.', 'Weeks are blocks of 7 days from the start of the case; the night is 22:00 to 07:00.', 'Checked: one read-only SELECT over the case’s analyst views.'], label: 'Who moved house, and when?', question: 'Did 923000001009 move house? Give the date the new place starts and how far the move was.', focus: ['923000001009'], layers: { moves: true }, caveat: 'move',
    select: { type: 'move', number: '923000001009' },
    sentence: () => { const m = moveOf('923000001009'); return `Yes: the night place of ${short(m.number)} changed in the week of ${m.first_week_start}, ${m.distance_km} km away.` },
    params: [{ label: 'Weeks a new place must hold', value: '2 weeks', source: 'the default' }],
  },
  {
    id: 'speed', strength: 'observed', derivation: ['Read: v_speed_alerts, for the number named.', 'Lowest speed set from your question: 200 km/h.', 'Kept consecutive events at different cell sites whose distance over the time between them reaches that speed.', 'Checked: one read-only SELECT over the case’s analyst views.'], label: 'Which events are impossibly far apart?', question: 'Which consecutive events of 923000001002 imply travel faster than 200 km/h?', focus: ['923000001002'], layers: { alerts: true }, caveat: 'speed',
    select: { type: 'alert', number: '923000001002', index: 0 },
    sentence: () => { const a = alertsOf('923000001002')[0]; return `1 pair: ${a.distance_km} km in ${a.gap_s} s (${a.speed_kmh} km/h), on ${a.from_time.slice(0, 10)} at ${a.from_time.slice(11, 16)}.` },
    params: [{ label: 'Lowest speed', value: '200 km/h', source: 'from your question' }],
  },
  {
    id: 'rhythm', strength: 'observed', derivation: ['Read: v_rhythm_changes, for the number named.', 'Least ratio of the two daily means: 150 percent, the default; at least 10 days on each side.', 'The best change point of the daily count by Welch t, listed only when |t| is at least 5.', 'Checked: one read-only SELECT over the case’s analyst views.'], label: 'Did the activity change?', question: 'Did the daily activity of 923000001006 change for good? From which day, and what were the daily averages before and after?', focus: ['923000001006'], layers: { rhythm: true }, caveat: 'rhythm',
    select: { type: 'rhythm', number: '923000001006' },
    sentence: () => { const r = rhythmOf('923000001006'); return `Yes: from ${r.first_day} the daily average ${r.direction === 'up' ? 'rose' : 'fell'} from ${r.mean_before} to ${r.mean_after} (×${r.ratio}).` },
    params: [{ label: 'Least ratio', value: '150 percent', source: 'the default' }],
  },
]

// The working set of a preset: its focus numbers, the strongest neighbours of each (at most four, so the picture starts small), and for two numbers their shared contacts.
export function workingSet(preset, expanded = []) {
  const set = new Set(preset.focus)
  for (const number of preset.focus) for (const n of strongest(number, preset.focus.length > 1 ? 3 : 4)) set.add(n)
  if (preset.focus.length === 2) for (const n of sharedContacts(preset.focus[0], preset.focus[1]).slice(0, 4)) set.add(n)
  for (const number of expanded) for (const n of strongest(number, 3)) set.add(n)
  return [...set]
}

// The graph of a working set: nodes, observed links between members (only those active in the brushed range), and the analysis links of the layers that are on.
export function graphOf(preset, members, layers, range) {
  const nodes = members.map(number => ({ id: number, label: short(number), subject: isSubject(number), focus: preset.focus.includes(number), events: subjectInfo(number).events }))
  const links = []
  for (let i = 0; i < members.length; i++) {
    for (let j = i + 1; j < members.length; j++) {
      const e = edgeBetween(members[i], members[j])
      if (!e) continue
      if (range && (day(e.last) < range[0] || day(e.first) > range[1])) continue
      links.push({ id: `c:${edgeKey(members[i], members[j])}`, source: e.source, target: e.target, kind: 'observed', events: e.events })
    }
  }
  const has = id => members.includes(id)
  if (layers.windows || layers.meetings) {
    for (let i = 0; i < preset.focus.length; i++) for (let j = i + 1; j < preset.focus.length; j++) {
      const a = preset.focus[i]; const b = preset.focus[j]
      const wins = layers.windows ? windowsOf(a, b, range).length : 0; const meets = layers.meetings ? meetingsOf(a, b, range).length : 0
      if (wins) links.push({ id: `w:${edgeKey(a, b)}`, source: a, target: b, kind: 'colocated', events: wins })
      if (meets) links.push({ id: `m:${edgeKey(a, b)}`, source: a, target: b, kind: 'meeting', events: meets })
    }
  }
  if (layers.successors) {
    for (const s of fixture.successors) for (const c of s.candidates) if (has(s.old) && has(c.number)) links.push({ id: `s:${s.old}|${c.number}`, source: s.old, target: c.number, kind: 'successor', events: Math.round(c.score * 100) })
  }
  return { nodes, links }
}

// Days with events of one number (the lane of the timeline).
export const laneOf = number => fixture.daily[number] || []

// The marks of the map: the cell sites a number used (a dot sized by its events), and the track of a move or an alert.
export function mapOf(preset, layers, range) {
  const wanted = new Map()
  const add = (cell, label, value) => { const p = placed(positionOf(cell)); if (p && !wanted.has(cell)) wanted.set(cell, { id: cell, label, value, lat: p.lat, lon: p.lon, radius: 0 }) }
  for (const row of fixture.visits.filter(v => preset.focus.includes(v.subject))) add(row.cell_key, short(row.cell_key.split(':').pop()), row.events)
  if (layers.windows) for (const a of preset.focus) for (const b of preset.focus) if (a < b) for (const w of windowsOf(a, b, range)) add(w.cell_key, 'together', 20)
  let track = []
  if (layers.moves) { const m = moveOf(preset.focus[0]); if (m) { add(m.from_cell, 'before', 40); add(m.to_cell, 'after', 40); track = [placed(positionOf(m.from_cell)), placed(positionOf(m.to_cell))].filter(Boolean) } }
  if (layers.alerts) { const a = alertsOf(preset.focus[0])[0]; if (a) { add(a.from_cell, 'from', 40); add(a.to_cell, 'to', 40); track = [placed(positionOf(a.from_cell)), placed(positionOf(a.to_cell))].filter(Boolean) } }
  return { sites: [...wanted.values()], track }
}

// The marks of the timeline, as day indexes.
export function marksOf(preset, layers, range) {
  const out = []
  if (layers.windows) for (const a of preset.focus) for (const b of preset.focus) if (a < b) for (const w of windowsOf(a, b, range)) out.push({ kind: 'window', day: day(w.start), id: `window:${w.start}`, label: `${w.start.slice(0, 16)} at one cell` })
  if (layers.meetings) for (const a of preset.focus) for (const b of preset.focus) if (a < b) for (const w of meetingsOf(a, b, range)) out.push({ kind: 'meeting', day: day(w.start), id: `meeting:${w.start}`, label: `possible meeting ${w.start.slice(0, 16)}` })
  if (layers.moves) { const m = moveOf(preset.focus[0]); if (m) out.push({ kind: 'move', day: day(m.first_week_start), id: `move:${m.number}`, label: `new night place from ${m.first_week_start}` }) }
  if (layers.alerts) for (const a of alertsOf(preset.focus[0])) out.push({ kind: 'alert', day: day(a.from_time), id: `alert:${a.from_time}`, label: `${a.speed_kmh} km/h at ${a.from_time.slice(0, 16)}` })
  if (layers.rhythm) { const r = rhythmOf(preset.focus[0]); if (r) out.push({ kind: 'rhythm', day: day(r.first_day), id: `rhythm:${r.number}`, label: `rhythm changes ${r.first_day}` }) }
  if (layers.successors) { const s = successorsOf(preset.focus[0]); if (s) out.push({ kind: 'silent', day: day(s.old_last), id: `silent:${s.old}`, label: `${short(s.old)} last seen ${s.old_last.slice(0, 10)}` }) }
  return out.sort((x, y) => x.day - y.day)
}

// The drawer: what a selected mark is, its facts, its caveat and the rows behind it. A table row carries only values the fixture holds.
function drawerCore(selection, preset, range) {
  if (!selection) return null
  switch (selection.type) {
    case 'node': {
      const nb = strongest(selection.id, 5)
      const own = isSubject(selection.id)
      const info = own ? subjectInfo(selection.id) : (() => {
        const all = (neighbours.get(selection.id) || []).map(n => n.edge)
        return { events: all.reduce((s, e) => s + e.events, 0), contacts: all.length, first: all.map(e => e.first).sort()[0] || '', last: all.map(e => e.last).sort().slice(-1)[0] || '' }
      })()
      return { title: short(selection.id), subtitle: own ? 'A subscriber of this case' : 'A number that subscribers contact (seen only through their records)', caveat: 'connections',
        facts: [[own ? 'Events' : 'Events with subscribers', String(info.events ?? '—')], [own ? 'Contacts' : 'Subscribers who contact it', String(info.contacts ?? '—')], ['First event', (info.first || '').slice(0, 10) || '—'], ['Last event', (info.last || '').slice(0, 10) || '—']],
        rowsTitle: 'Strongest contacts', columns: ['Number', 'Events'], rows: nb.map(n => [n, String(edgeBetween(selection.id, n)?.events ?? '')]) }
    }
    case 'edge': {
      const e = edgeBetween(selection.a, selection.b)
      return { title: `${short(selection.a)} ↔ ${short(selection.b)}`, subtitle: 'Observed contact', caveat: 'connections',
        facts: e ? [['Events', String(e.events)], ['Outgoing', String(e.outgoing)], ['Incoming', String(e.incoming)], ['Call time', `${Math.round(e.duration_s / 60)} min`], ['First', e.first.slice(0, 10)], ['Last', e.last.slice(0, 10)]] : [['Events', '0']],
        rowsTitle: 'Shared contacts', columns: ['Number'], rows: sharedContacts(selection.a, selection.b).map(n => [n]) }
    }
    case 'window': {
      const rows = windowsOf(selection.a, selection.b, range)
      return { title: `${short(selection.a)} and ${short(selection.b)} at one cell`, subtitle: `${rows.length} windows within 22 minutes`, caveat: 'colocation',
        facts: [['Window used', '22 minutes (from your question)'], ['Windows', String(rows.length)], ['Cell site', [...new Set(rows.map(r => String(r.cell_key).split(':').pop()))].join(', ') || '—']],
        rowsTitle: 'The windows', columns: ['Start', 'End', 'Length'], rows: rows.map(r => [r.start.slice(0, 16), r.end.slice(11, 16), `${minutes(r.start, r.end)} min`]), pick: selection.index }
    }
    case 'meeting': {
      const rows = meetingsOf(selection.a, selection.b, range)
      return { title: `Possible meetings of ${short(selection.a)} and ${short(selection.b)}`, subtitle: `${rows.length} candidates`, caveat: 'meeting',
        facts: [['Window used', '30 minutes (from your question)'], ['Gap to the last contact', '90 minutes (from your question)'], ['Candidates', String(rows.length)]],
        rowsTitle: 'The candidates', columns: ['Start', 'End', 'Length'], rows: rows.map(r => [r.start.slice(0, 16), r.end.slice(11, 16), `${minutes(r.start, r.end)} min`]), pick: selection.index }
    }
    case 'successor': {
      const s = successorsOf(selection.old); const c = s.candidates.find(x => x.number === selection.new)
      return { title: `${short(selection.old)} → ${short(selection.new)}`, subtitle: 'A candidate successor', caveat: 'successor',
        facts: [['Score', c.score.toFixed(4)], ['Contact similarity', c.contact_cosine.toFixed(4)], ['Shared contacts', String(c.shared_contacts)], ['Shared places', c.cell_jaccard.toFixed(4)], ['Shared handset', c.shared_device ? 'yes' : 'no'], ['Old number last seen', s.old_last.slice(0, 16)], ['New number first seen', c.first_event.slice(0, 16)]],
        rowsTitle: 'How the score is made', columns: ['Part', 'Weight', 'Value'], rows: [['Contact similarity', '0.6', c.contact_cosine.toFixed(4)], ['Shared places', '0.2', c.cell_jaccard.toFixed(4)], ['Shared handset', '0.2', c.shared_device ? '1' : '0']] }
    }
    case 'move': {
      const m = moveOf(selection.number)
      return { title: `${short(m.number)} moved`, subtitle: 'A change of the night place', caveat: 'move',
        facts: [['New place from', `week of ${m.first_week_start}`], ['Distance', `${m.distance_km} km`], ['Weeks it must hold', '2 (the default)']],
        rowsTitle: 'The two places', columns: ['', 'Cell site'], rows: [['Before', m.from_cell], ['After', m.to_cell]] }
    }
    case 'alert': {
      const a = alertsOf(selection.number)[selection.index || 0]
      return { title: `${a.speed_kmh} km/h in ${a.gap_s} s`, subtitle: `${short(a.number)}, two consecutive events`, caveat: 'speed',
        facts: [['Distance', `${a.distance_km} km`], ['Gap', `${a.gap_s} s`], ['Lowest speed listed', '200 km/h (from your question)']],
        rowsTitle: 'The two events', columns: ['Time', 'Cell site'], rows: [[a.from_time.slice(0, 19), a.from_cell], [a.to_time.slice(0, 19), a.to_cell]] }
    }
    case 'rhythm': {
      const r = rhythmOf(selection.number)
      return { title: `${short(r.number)}: a change from ${r.first_day}`, subtitle: `${r.direction === 'up' ? 'Up' : 'Down'} by a factor of ${r.ratio}`, caveat: 'rhythm',
        facts: [['Average before', `${r.mean_before} a day`], ['Average after', `${r.mean_after} a day`], ['Welch t', String(r.t)], ['Days in the series', String(r.days)], ['Least ratio', '150 percent (the default)']],
        rowsTitle: 'The two sides', columns: ['', 'Average a day'], rows: [['Before', String(r.mean_before)], ['After', String(r.mean_after)]] }
    }
    case 'site': {
      const s = fixture.sites.find(x => x.cell_key === selection.id)
      const rows = fixture.visits.filter(v => v.cell_key === selection.id)
      return { title: `Cell site ${String(selection.id).split(':').pop()}`, subtitle: 'A coverage area, not the position of a person', caveat: 'move',
        facts: [['Events at this site', String(s ? s.events : '—')], ['Subscribers who used it', String(s ? s.visitors : '—')]],
        rowsTitle: 'Who used it', columns: ['Number', 'Events', 'First', 'Last'], rows: rows.map(v => [v.subject, String(v.events), v.first.slice(0, 10), v.last.slice(0, 10)]) }
    }
    default:
      return null
  }
}

const STRENGTH_OF_SELECTION = { node: 'observed', edge: 'observed', site: 'observed', alert: 'observed', rhythm: 'observed', window: 'inferred', move: 'inferred', meeting: 'candidate', successor: 'candidate' }
export function drawerFor(selection, preset, range) {
  const d = drawerCore(selection, preset, range)
  return d ? { ...d, strength: STRENGTH_OF_SELECTION[selection.type] || 'observed' } : d
}

// What the table twin lists for a preset: the same marks as the canvas, as rows.
export function tableFor(preset, layers, range) {
  const d = drawerFor(preset.select, preset, range)
  return d ? { title: d.rowsTitle, columns: d.columns, rows: d.rows } : { title: '', columns: [], rows: [] }
}

// The designed states of the workbench (the contract's state list): each says what happened, why, and what to do.
export const STATES = {
  empty: { title: 'No relationships within these limits', body: 'These two numbers have no contact in the case and no number in common within 2 hops, with services and numbers everyone contacts left out.', actions: ['Widen the period', 'Allow 3 hops', 'Show the excluded numbers'] },
  broad: { title: 'This is too broad to draw', body: '1,840 numbers fall within 2 hops of this number. A picture of that many is not an answer; start from fewer.', actions: ['Pick two numbers', 'Choose a period', 'Show the 10 strongest'] },
  abstained: { title: 'The engine did not answer', body: 'It could not show that the query it wrote asks everything you asked: the question names a night and the query did not restrict the hour.', actions: ['Rephrase with the exact number or date', 'Open the table of events instead'] },
  partial: { title: 'Part of this case is not typed yet', body: 'Call records are typed; 4 of the 33 sources (access logs) are not, so they are not in this picture. Counts here are of the typed records only.', actions: ['See what is typed', 'Continue with the typed records'] },
}
