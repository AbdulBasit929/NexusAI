// The link-analysis workbench's index of a case (docs/ux/workbench/RESEARCH_NOTE.md). Pure functions over the two LIVE reads of the case, GET /analysis/connections and GET /analysis/places
// (api/forensic_records/analysis_views.go): who is in contact with whom, which cell sites were used and when. No network, no DOM, no model. Everything the workbench draws before a question is
// asked, and everything it adds a mark to after, comes from here, so the picture and the numbers beside it cannot disagree.
//
// Times: the API writes every time as RFC 3339 on the case clock (for example 2026-02-02T03:37:46+05:00). A day, here, is the calendar day ON THE CASE CLOCK, read from the text itself, so a
// browser in another zone shows the same day the records say.
import { carrierFor, formatMobile } from '../carriers.js'

const DAY = 86400000
const digits = value => String(value ?? '').replace(/\D/g, '')
export const looksLikeNumber = value => digits(value).length >= 10 && digits(value).length <= 13 && /^[+\d][\d\s()+-]*$/.test(String(value ?? '').trim())

// The calendar date and the clock of a time text, as the text states them ("2026-02-09T08:36:08+05:00", "2026-02-09 08:36:08 PKT"). null when it is not a time.
const TIME = /^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2}))?)?/
export function timeParts(text) {
  const m = TIME.exec(String(text ?? '').trim())
  if (!m) return null
  return { date: `${m[1]}-${m[2]}-${m[3]}`, hh: m[4] ?? '00', mm: m[5] ?? '00', ss: m[6] ?? '00', ms: Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]), Number(m[4] ?? 0), Number(m[5] ?? 0), Number(m[6] ?? 0)) }
}
export const dateOf = text => timeParts(text)?.date ?? ''
// "2026-02-09 08:36" for a time text, the way a table shows it
export function clockOf(text) {
  const t = timeParts(text)
  return t ? `${t.date} ${t.hh}:${t.mm}` : String(text ?? '')
}
export const minutesBetween = (from, to) => {
  const a = timeParts(from); const b = timeParts(to)
  return a && b ? Math.max(0, Math.round((b.ms - a.ms) / 60000)) : null
}
const dayNumber = date => Math.round(Date.parse(`${date}T00:00:00Z`) / DAY)

export function shortNumber(number) {
  const d = digits(number)
  return d.length >= 7 ? `…${d.slice(-3)}` : String(number)
}
export const labelOf = number => (looksLikeNumber(number) ? formatMobile(number) : String(number))

const edgeKey = (a, b) => (a < b ? `${a}|${b}` : `${b}|${a}`)
export { edgeKey }

export function createCaseIndex({ connections, places } = {}) {
  const subjectsList = connections?.subjects || []
  const subjects = new Map(subjectsList.map(s => [s.number, s]))
  const edges = (connections?.edges || []).filter(e => e.source !== e.target)
  const byPair = new Map(edges.map(e => [edgeKey(e.source, e.target), e]))
  const neighbours = new Map()
  for (const e of edges) {
    for (const [a, b] of [[e.source, e.target], [e.target, e.source]]) {
      if (!neighbours.has(a)) neighbours.set(a, [])
      neighbours.get(a).push({ number: b, edge: e })
    }
  }
  const sites = new Map((places?.sites || []).map(s => [s.cell_key, s]))
  const visits = places?.visits || []
  const visitsBySubject = new Map()
  for (const v of visits) {
    if (!visitsBySubject.has(v.subject)) visitsBySubject.set(v.subject, [])
    visitsBySubject.get(v.subject).push(v)
  }

  // The case clock: the zone's name from the read, and the offset the records themselves carry
  const offset = (() => {
    for (const text of [subjectsList[0]?.first, visits[0]?.hour]) {
      const m = /(Z|[+-]\d{2}:\d{2})$/.exec(String(text ?? ''))
      if (m) return m[1] === 'Z' ? '+00:00' : m[1]
    }
    return ''
  })()

  // The days the case covers: from the first date any subscriber or any visit states to the last
  const dates = []
  for (const s of subjectsList) { if (s.first) dates.push(dateOf(s.first)); if (s.last) dates.push(dateOf(s.last)) }
  for (const v of visits) dates.push(dateOf(v.hour))
  const real = dates.filter(Boolean).sort()
  const startDate = real[0] || ''
  const endDate = real[real.length - 1] || ''
  const days = startDate ? dayNumber(endDate) - dayNumber(startDate) + 1 : 0
  const dayIndex = text => { const d = dateOf(text); return d && startDate ? dayNumber(d) - dayNumber(startDate) : null }
  const dayLabel = index => (startDate ? new Date((dayNumber(startDate) + index) * DAY).toISOString().slice(0, 10) : '')

  // events a day of one number at a cell site, from the hourly visits (the only per-day record the live reads hold; counts events that carry a cell)
  const lanes = new Map()
  const laneOf = number => {
    if (!lanes.has(number)) {
      const lane = new Array(Math.max(days, 0)).fill(0)
      for (const v of visitsBySubject.get(number) || []) { const i = dayIndex(v.hour); if (i != null && i >= 0 && i < lane.length) lane[i] += v.events }
      lanes.set(number, lane)
    }
    return lanes.get(number)
  }

  const isSubject = number => subjects.has(number)
  const edgeBetween = (a, b) => byPair.get(edgeKey(a, b)) || null
  // what a number is for the picture: a subscriber of the case, or a number the subscribers contact (an ordinary contact, a hub everyone contacts, or a service)
  const kindOf = number => {
    if (subjects.has(number)) return 'subject'
    const links = (neighbours.get(number) || []).map(n => n.edge)
    if (!looksLikeNumber(number) || links.some(e => e.is_service)) return 'service'
    if (links.some(e => e.is_hub)) return 'hub'
    return 'contact'
  }
  const excludedKind = number => { const k = kindOf(number); return k === 'service' || k === 'hub' }

  function numberInfo(number) {
    const own = subjects.get(number)
    if (own) return { number, kind: 'subject', events: own.events, contacts: own.contacts, first: own.first || '', last: own.last || '', imsi: own.imsi || '', imei: own.imei || '', carrier: carrierFor({ number, imsi: own.imsi }) }
    const all = (neighbours.get(number) || []).map(n => n.edge)
    const firsts = all.map(e => e.first).filter(Boolean).sort()
    const lasts = all.map(e => e.last).filter(Boolean).sort()
    return { number, kind: kindOf(number), events: all.reduce((s, e) => s + e.events, 0), contacts: all.length, first: firsts[0] || '', last: lasts[lasts.length - 1] || '', imsi: '', imei: '', carrier: carrierFor({ number }) }
  }

  // The contacts two numbers have in common, services and numbers that everyone contacts left out (they are listed as excluded, never hidden).
  function sharedContacts(a, b) {
    const mine = new Map((neighbours.get(a) || []).filter(n => !n.edge.is_service && !n.edge.is_hub).map(n => [n.number, n.edge]))
    const out = []
    for (const n of neighbours.get(b) || []) {
      if (n.edge.is_service || n.edge.is_hub || n.number === a || n.number === b) continue
      if (mine.has(n.number)) out.push({ number: n.number, first: mine.get(n.number).events, second: n.edge.events })
    }
    const seen = new Set()
    return out.filter(o => (seen.has(o.number) ? false : seen.add(o.number))).sort((x, y) => y.first + y.second - x.first - x.second || (x.number < y.number ? -1 : 1))
  }
  function excludedFor(numbers) {
    const out = new Map()
    for (const number of numbers) for (const n of neighbours.get(number) || []) if (n.edge.is_service || n.edge.is_hub) out.set(n.number, n.edge.is_service ? 'service' : 'hub')
    return [...out].map(([number, why]) => ({ number, why }))
  }
  // The strongest neighbours of a number (people, not services or hubs), by events, at most `limit`.
  function strongest(number, limit = 4) {
    return (neighbours.get(number) || []).filter(n => !n.edge.is_service && !n.edge.is_hub).sort((x, y) => y.edge.events - x.edge.events || (x.number < y.number ? -1 : 1)).slice(0, limit).map(n => n.number)
  }

  const siteOf = cell => sites.get(cell) || null
  const siteVisitors = cell => {
    const by = new Map()
    for (const v of visits) {
      if (v.cell_key !== cell) continue
      const row = by.get(v.subject) || { subject: v.subject, events: 0, first: v.hour, last: v.hour }
      row.events += v.events
      if (v.hour < row.first) row.first = v.hour
      if (v.hour > row.last) row.last = v.hour
      by.set(v.subject, row)
    }
    return [...by.values()].sort((x, y) => y.events - x.events)
  }
  // the cell sites a set of subscribers used, with their events (inside the days [from, to] when a period is given)
  function sitesUsedBy(numbers, range = null) {
    const used = new Map()
    for (const n of numbers) {
      for (const v of visitsBySubject.get(n) || []) {
        if (range) { const i = dayIndex(v.hour); if (i == null || i < range[0] || i > range[1]) continue }
        used.set(v.cell_key, (used.get(v.cell_key) || 0) + v.events)
      }
    }
    return used
  }

  // A number typed in any common form, or part of one, against the numbers this case knows (subscribers first)
  function find(query, limit = 8) {
    const q = digits(query)
    if (q.length < 3) return []
    const known = [...subjects.keys(), ...[...neighbours.keys()].filter(n => !subjects.has(n))]
    return known.filter(n => digits(n).includes(q)).slice(0, limit)
  }

  const subjectsByEvents = [...subjectsList].sort((a, b) => b.events - a.events || (a.number < b.number ? -1 : 1))

  return {
    subjects: subjectsList, subjectsByEvents, edges, totals: connections?.totals || {}, truncated: !!connections?.truncated, perSubject: connections?.per_subject || 0,
    sites: [...sites.values()], visits, placesTotals: places?.totals || {}, placesTruncated: !!places?.truncated,
    zone: connections?.time_zone || places?.time_zone || '', offset, startDate, endDate, days,
    connectionCaveats: connections?.caveats || [], placeCaveats: places?.caveats || [],
    hasPlaces: !!places?.available && sites.size > 0,
    isSubject, edgeBetween, kindOf, excludedKind, numberInfo, sharedContacts, excludedFor, strongest, siteOf, siteVisitors, sitesUsedBy, laneOf, dayIndex, dayLabel, find,
  }
}
