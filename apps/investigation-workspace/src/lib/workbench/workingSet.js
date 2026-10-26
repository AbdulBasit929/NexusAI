// The working set of the workbench and what is drawn from it (docs/ux/workbench/RESEARCH_NOTE.md, rules 3 and 4): a small set of numbers grown deliberately, never "everything"; the links among them;
// the cell sites they used; the marks of the answer on the timeline. Pure functions over the case index and the answer, so that the same state always gives the same picture.
import { edgeKey, labelOf, looksLikeNumber, shortNumber } from './caseIndex.js'

// Decision D16: the generated cases sit on the generator's own grid, in the ocean; the interface may place that grid over a city, and says so on the map. Real positions are never moved.
export const PLACEMENT = { city: 'Lahore', lat: 31.5204, lon: 74.3587 }
// the area the offline basemap covers (Pakistan, with a margin): positions outside it are not on the basemap at all
const BASEMAP = { south: 22, north: 38, west: 59, east: 78 }
export const inBasemap = p => p && Number.isFinite(p.lat) && Number.isFinite(p.lon) && p.lat >= BASEMAP.south && p.lat <= BASEMAP.north && p.lon >= BASEMAP.west && p.lon <= BASEMAP.east
export function placementNeeded(index) {
  const positioned = index.sites.filter(s => Number.isFinite(s.lat) && Number.isFinite(s.lon))
  return positioned.length > 0 && positioned.every(s => !inBasemap(s))
}
function centre(sites) {
  const p = sites.filter(s => Number.isFinite(s.lat) && Number.isFinite(s.lon))
  return p.length ? { lat: p.reduce((a, s) => a + s.lat, 0) / p.length, lon: p.reduce((a, s) => a + s.lon, 0) / p.length } : { lat: 0, lon: 0 }
}
// positions as recorded, or the same relative distances over the city (the offsets are kept, so every distance is the generator's)
export function positioner(index, mode) {
  if (mode !== 'city') return site => ({ lat: site.lat, lon: site.lon })
  const home = centre(index.sites)
  return site => ({ lat: PLACEMENT.lat + (site.lat - home.lat), lon: PLACEMENT.lon + (site.lon - home.lon) })
}

const MAX_OVERVIEW = 40
const MAX_SHARED = 24

// The ordinary contacts that two or more of the given subscribers have (services and numbers everyone contacts left out), the most widely shared first: they are what joins subscribers who
// never contact each other, and the reason an overview is more than a ring of unconnected dots.
export function sharedAmong(index, subscribers, limit = MAX_SHARED) {
  const inside = new Set(subscribers)
  const by = new Map()
  for (const e of index.edges) {
    if (e.is_service || e.is_hub) continue
    const sub = inside.has(e.source) ? e.source : inside.has(e.target) ? e.target : null
    if (!sub) continue
    const other = sub === e.source ? e.target : e.source
    if (inside.has(other)) continue
    if (!by.has(other)) by.set(other, new Map())
    by.get(other).set(sub, (by.get(other).get(sub) || 0) + e.events)
  }
  const total = m => [...m.values()].reduce((a, b) => a + b, 0)
  return [...by].filter(([, m]) => m.size >= 2).sort((a, b) => b[1].size - a[1].size || total(b[1]) - total(a[1]) || (a[0] < b[0] ? -1 : 1)).slice(0, limit).map(([number, m]) => ({ number, subscribers: m.size, events: total(m) }))
}

// The numbers the picture starts from. With an answer: the numbers of the question and of its rows, each focus number's strongest neighbours (at most four, three for a pair), and for two
// numbers their shared contacts. Without one: the subscribers themselves (the strongest forty) and the contacts they share, because who is joined to whom is a picture that is small and true.
export function workingSetOf(index, { focus = [], answerNumbers = [], expanded = [], added = [] } = {}) {
  const set = new Set()
  const known = number => index.isSubject(number) || index.edges.some(e => e.source === number || e.target === number)
  if (!focus.length && !answerNumbers.length) {
    const top = index.subjectsByEvents.slice(0, MAX_OVERVIEW).map(s => s.number)
    top.forEach(n => set.add(n))
    sharedAmong(index, top).forEach(s => set.add(s.number))
  } else {
    for (const number of [...focus, ...answerNumbers]) if (known(number)) set.add(number)
    const near = focus.length > 1 ? 3 : 4
    for (const number of focus) for (const n of index.strongest(number, near)) set.add(n)
    if (focus.length === 2) for (const s of index.sharedContacts(focus[0], focus[1]).slice(0, 4)) set.add(s.number)
  }
  for (const number of added) if (known(number)) set.add(number)
  for (const number of expanded) for (const n of index.strongest(number, 3)) set.add(n)
  return [...set]
}

// The part of the working set that the question names, drawn larger
export const focusOf = (focus, answerNumbers) => new Set(focus.length ? focus : answerNumbers.length ? answerNumbers.slice(0, 3) : [])

// ---- the links pane ---------------------------------------------------------------------------------------------------------------------------------------------------------------------
// range: [firstDay, lastDay] of the brush, or null
export function graphOf(index, members, { focus = new Set(), answer = null, layers = {}, range = null } = {}) {
  const has = new Set(members)
  const nodes = members.map(number => {
    const info = index.numberInfo(number)
    return { id: number, label: shortNumber(number), full: labelOf(number), subject: info.kind === 'subject', focus: focus.has(number), kind: info.kind, events: info.events }
  })
  const emph = new Set(answer?.emph || [])
  const links = []
  if (layers.contacts !== false) {
    for (let i = 0; i < members.length; i++) {
      for (let j = i + 1; j < members.length; j++) {
        const e = index.edgeBetween(members[i], members[j])
        if (!e) continue
        if (range) { const first = index.dayIndex(e.first); const last = index.dayIndex(e.last); if ((last != null && last < range[0]) || (first != null && first > range[1])) continue }
        links.push({ id: `c:${edgeKey(members[i], members[j])}`, source: e.source, target: e.target, kind: 'observed', events: e.events, emph: emph.has(edgeKey(e.source, e.target)) })
      }
    }
  }
  for (const link of answer?.links || []) {
    const layer = { colocated: 'windows', meeting: 'meetings', successor: 'successors' }[link.kind]
    if (layers[layer] === false) continue
    if (!has.has(link.source) || !has.has(link.target)) continue
    const marks = (answer.marks || []).filter(m => link.markIds.includes(m.id) && (!range || m.day == null || (m.day >= range[0] && m.day <= range[1])))
    if (!marks.length) continue
    links.push({ ...link, events: link.kind === 'successor' ? Math.round((Number(answer.rows[marks[0].row]?.score) || 0) * 100) : marks.length, markIds: marks.map(m => m.id) })
  }
  return { nodes, links }
}

// ---- the marks on the timeline (and what the brush leaves of them) ---------------------------------------------------------------------------------------------------
export function visibleMarks(answer, layers = {}, range = null) {
  if (!answer) return []
  const layerOf = { window: 'windows', meeting: 'meetings', successor: 'successors', silent: 'successors', move: 'moves', anchor: 'moves', alert: 'alerts', rhythm: 'rhythm' }
  return answer.marks.filter(m => layers[layerOf[m.kind]] !== false && (!range || m.day == null || (m.day >= range[0] && m.day <= range[1])))
}

// ---- the places pane --------------------------------------------------------------------------------------------------------------------------------------------------------------
// The cell sites of the working set (its subscribers' visits, inside the brushed days) and of the marks; the tracks of a move or a speed alert.
export function mapOf(index, members, { answer = null, layers = {}, range = null, position = s => s } = {}) {
  const subscribers = members.filter(n => index.isSubject(n))
  const used = index.sitesUsedBy(subscribers, range)
  const wanted = new Map([...used].map(([cell, events]) => [cell, events]))
  const marked = new Set()
  const marks = visibleMarks(answer, layers, range)
  for (const m of marks) for (const c of m.cells) { marked.add(c); if (!wanted.has(c)) wanted.set(c, 0) }
  const sites = []
  for (const [cell, events] of wanted) {
    const site = index.siteOf(cell)
    if (!site || !Number.isFinite(site.lat) || !Number.isFinite(site.lon)) continue
    const p = position(site)
    sites.push({ id: cell, label: site.name || String(cell).split(':').pop(), value: events || 1, lat: p.lat, lon: p.lon, radius: site.radius_m || 0, marked: marked.has(cell) })
  }
  const byId = new Map(sites.map(s => [s.id, s]))
  const track = []
  for (const t of (answer?.tracks || []).filter(t => marks.some(m => m.id === t.id))) {
    const pts = t.cells.map(c => byId.get(c)).filter(Boolean)
    if (pts.length === 2) track.push(...pts.map(p => ({ lat: p.lat, lon: p.lon })))
  }
  return { sites, track, unlocated: [...wanted.keys()].filter(c => !byId.has(c)).length }
}

// ---- the lanes of the timeline ----------------------------------------------------------------------------------------------------------------------------------------------
// One lane for each number the question names (at most three); with none, one lane for all the subscribers of the working set.
export function lanesOf(index, { focus = [], members = [] } = {}) {
  const own = focus.filter(n => index.isSubject(n)).slice(0, 3)
  if (own.length) return own.map(n => ({ id: n, title: shortNumber(n), values: index.laneOf(n) }))
  const subscribers = members.filter(n => index.isSubject(n))
  const values = new Array(index.days).fill(0)
  for (const n of subscribers) index.laneOf(n).forEach((v, i) => { values[i] += v })
  return [{ id: 'all', title: subscribers.length > 1 ? 'All' : (subscribers[0] ? shortNumber(subscribers[0]) : 'All'), values }]
}

// quick periods of the time bar, as day indexes of the case
export function periodsOf(index) {
  const last = Math.max(0, index.days - 1)
  return [
    { id: 'all', label: 'All', range: null },
    { id: '30', label: '30 days', range: [Math.max(0, last - 29), last] },
    { id: '7', label: '7 days', range: [Math.max(0, last - 6), last] },
  ]
}

// ---- suggestions: questions the engine is known to answer, for the numbers on screen --------------------------------------------------------------------------------
export function suggestionsFor(index, { focus = [], selected = null } = {}) {
  const top = index.subjectsByEvents.map(s => s.number)
  const one = (selected && looksLikeNumber(selected) ? selected : focus[0]) || top[0]
  const two = focus.length > 1 ? focus[1] : (selected && selected !== one ? selected : (top.find(n => n !== one) || null))
  const out = []
  if (!one) return out
  if (two) out.push({ id: 'connected', label: 'How are two numbers connected?', question: `How are ${one} and ${two} connected?` })
  out.push({ id: 'contacts', label: 'Who do they contact most?', question: `Who does ${one} contact most? Give the top five.` })
  out.push({ id: 'sites', label: 'Which cell sites do they use most?', question: `Which cell sites did ${one} use most? Give the top five.` })
  if (two) out.push({ id: 'together', label: 'When were two numbers at the same cell?', question: `When were ${one} and ${two} at the same cell within 15 minutes of each other?` })
  if (two) out.push({ id: 'meeting', label: 'Could they have met?', question: `Which of the times ${one} and ${two} were at the same cell within 30 minutes could have been a meeting, given a call or SMS between them in the previous 90 minutes?` })
  out.push({ id: 'successor', label: 'Did a number change hands?', question: `Which numbers may have taken over from ${one}?` })
  out.push({ id: 'move', label: 'Did they move house?', question: `Did ${one} move house? Give the date the new place starts and how far the move was.` })
  out.push({ id: 'speed', label: 'Which events are impossibly far apart?', question: `Which consecutive events of ${one} imply travel faster than 200 km/h?` })
  out.push({ id: 'rhythm', label: 'Did the activity change?', question: `Did the daily activity of ${one} change for good? From which day, and what were the daily averages before and after?` })
  return out
}
