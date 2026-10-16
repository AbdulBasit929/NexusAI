// The model behind the link chart (roadmap 7.8.5): pure functions, no DOM, no network. Lifted from the visual spike (tools/visual_spike/src/filters.js, overview.js, layout.js)
// and made to read the API's /analysis/connections response and the rows of an Ask answer of the "graph" shape.
//
// What a filter means, so that nothing on screen is misread:
//   Targets   the subjects to look at; the chart shows each target and the numbers it is in contact with (its neighbourhood)
//   Carrier   keeps the subjects whose SIM is on the chosen carriers and, around them, only the other numbers whose carrier (by number range) is chosen
//   Period    keeps a link when its first and last contact overlap the period; shared-contact links are computed on the whole record, so they are hidden for a period
import { carrierFor, carrierOfNumber, formatMobile, pkMobile } from './carriers.js'

const digits = value => String(value ?? '').replace(/\D/g, '')

// a number typed or linked in any common form (0300 1234567, +92 300 1234567) as the digits the data holds (923001234567)
export function normaliseNumber(value) {
  const p = pkMobile(value)
  return p ? `92${p.prefix}${p.rest}` : digits(value)
}

const looksLikeNumber = id => !!pkMobile(id) || digits(id).length >= 10

// ---- the model ------------------------------------------------------------------------------------------------------------------------------------------------------
// response: { subjects: [{number, events, contacts, first, last, imsi, imei}], edges: [{source, target, events, outgoing, incoming, unknown, duration_s, first, last, is_service, is_hub}] }
export function buildModel(response) {
  const subjects = response?.subjects || []
  const nodes = new Map()
  for (const s of subjects) {
    const c = carrierFor({ number: s.number, imsi: s.imsi })
    nodes.set(s.number, { id: s.number, label: formatMobile(s.number), kind: 'subject', carrier: c.id, carrierHow: c.how, events: s.events, first: s.first, last: s.last, imsi: s.imsi || '', imei: s.imei || '' })
  }
  const edges = []
  for (const e of response?.edges || []) {
    if (!nodes.has(e.source)) continue
    let other = nodes.get(e.target)
    if (!other) {
      const c = carrierOfNumber(e.target)
      const number = looksLikeNumber(e.target)
      other = { id: e.target, label: number ? formatMobile(e.target) : e.target, kind: e.is_service || !number ? 'service' : e.is_hub ? 'hub' : 'contact', carrier: c.id, carrierHow: c.how, events: 0, first: e.first, last: e.last }
      nodes.set(e.target, other)
    }
    if (other.kind === 'contact' && e.is_hub) other.kind = 'hub'
    other.events += e.events
    edges.push({ id: `o${edges.length}`, source: e.source, target: e.target, kind: 'observed', count: e.events, outgoing: e.outgoing, incoming: e.incoming, unknown: e.unknown, duration: e.duration_s, first: e.first, last: e.last })
  }
  const model = { nodes: [...nodes.values()], edges, subjects: subjects.map(s => s.number), totals: response?.totals || {}, truncated: !!response?.truncated, perSubject: response?.per_subject || 0 }
  model.edges.push(...inferSharedContacts(model))
  return model
}

// Two subjects that both have the same ordinary contact (not a service, not a hub) are linked by an INFERRED edge whose count is the number they share. Not a link between the two.
export function inferSharedContacts(model) {
  const subjects = new Set(model.subjects)
  const kindOf = new Map(model.nodes.map(n => [n.id, n.kind]))
  const contactsOf = new Map()
  for (const e of model.edges) {
    if (e.kind !== 'observed' || !subjects.has(e.source)) continue
    if (kindOf.get(e.target) !== 'contact') continue
    if (!contactsOf.has(e.source)) contactsOf.set(e.source, new Set())
    contactsOf.get(e.source).add(e.target)
  }
  const ids = [...contactsOf.keys()].sort()
  const out = []
  for (let i = 0; i < ids.length; i++) {
    for (let j = i + 1; j < ids.length; j++) {
      let shared = 0
      for (const c of contactsOf.get(ids[i])) if (contactsOf.get(ids[j]).has(c)) shared++
      if (shared > 0) out.push({ id: `i${out.length}`, source: ids[i], target: ids[j], kind: 'inferred', count: shared })
    }
  }
  return out
}

export function sharedContacts(model, a, b) {
  const side = id => new Map(model.edges.filter(e => e.kind === 'observed' && (e.source === id || e.target === id)).map(e => [e.source === id ? e.target : e.source, e]))
  const x = side(a), y = side(b)
  const kindOf = new Map(model.nodes.map(n => [n.id, n.kind]))
  return [...x.keys()].filter(k => y.has(k) && kindOf.get(k) === 'contact' && k !== a && k !== b)
    .map(k => ({ id: k, first: x.get(k).count, second: y.get(k).count })).sort((p, q) => q.first + q.second - p.first - p.second || (p.id < q.id ? -1 : 1))
}

// The shortest connection over observed and inferred links, at most `maxHops` steps, never through a service or a hub (the same rule the Ask engine applies).
export function shortestPath(model, a, b, maxHops = 4) {
  if (a === b) return [a]
  const kindOf = new Map(model.nodes.map(n => [n.id, n.kind]))
  const adj = new Map()
  for (const e of model.edges) {
    if (!adj.has(e.source)) adj.set(e.source, [])
    if (!adj.has(e.target)) adj.set(e.target, [])
    adj.get(e.source).push(e.target)
    adj.get(e.target).push(e.source)
  }
  const prev = new Map([[a, null]])
  let frontier = [a]
  for (let d = 1; d <= maxHops && frontier.length; d++) {
    const next = []
    for (const id of frontier) {
      for (const n of adj.get(id) || []) {
        if (prev.has(n)) continue
        if (n !== b && (kindOf.get(n) === 'service' || kindOf.get(n) === 'hub')) continue
        prev.set(n, id)
        if (n === b) {
          const path = [b]
          for (let p = id; p !== null; p = prev.get(p)) path.unshift(p)
          return path
        }
        next.push(n)
      }
    }
    frontier = next
  }
  return null
}

// ---- filters ----------------------------------------------------------------------------------------------------------------------------------------------------------
export const emptyFilters = () => ({ targets: new Set(), carriers: new Set(), from: '', to: '', minEvents: 1, show: { observed: true, inferred: true, contacts: true, services: false, hubs: true } })
// What the analyst has CHOSEN (the Reset button and the summary line); the model always applies the show switches, whose default hides the services
export const isFiltered = f => f.targets.size > 0 || f.carriers.size > 0 || !!f.from || !!f.to || f.minEvents > 1

export function filterModel(model, f) {
  const nodeById = new Map(model.nodes.map(n => [n.id, n]))
  const subjects = model.nodes.filter(n => n.kind === 'subject')
  const core = new Set(subjects.filter(s => (!f.targets.size || f.targets.has(s.id)) && (!f.carriers.size || f.carriers.has(s.carrier))).map(s => s.id))
  const period = !!(f.from || f.to)
  const overlaps = e => (!f.from || !e.last || String(e.last).slice(0, 10) >= f.from) && (!f.to || !e.first || String(e.first).slice(0, 10) <= f.to)
  const shown = kind => (kind === 'service' ? f.show.services : kind === 'hub' ? f.show.hubs : kind === 'contact' ? f.show.contacts : true)
  const edges = model.edges.filter(e => {
    if (e.kind === 'observed') {
      if (!f.show.observed || e.count < f.minEvents) return false
      const aIn = core.has(e.source), bIn = core.has(e.target)
      if (!aIn && !bIn) return false
      const other = nodeById.get(aIn ? e.target : e.source)
      if (!other) return false
      if (other.kind === 'subject') {
        if (!(aIn && bIn)) return false
      } else if (!shown(other.kind) || (f.carriers.size && !f.carriers.has(other.carrier))) {
        return false
      }
      return !period || overlaps(e)
    }
    return f.show.inferred && !period && core.has(e.source) && core.has(e.target)
  })
  const used = new Set(core)
  for (const e of edges) { used.add(e.source); used.add(e.target) }
  const degree = new Map()
  for (const e of edges) { degree.set(e.source, (degree.get(e.source) || 0) + 1); degree.set(e.target, (degree.get(e.target) || 0) + 1) }
  const nodes = model.nodes.filter(n => used.has(n.id)).map(n => ({ ...n, degree: degree.get(n.id) || 0 }))
  return { ...model, nodes, edges, filtered: true, subjects: model.subjects.filter(id => core.has(id)) }
}

export function summarise(base, shown) {
  const count = m => ({ subjects: m.nodes.filter(n => n.kind === 'subject').length, numbers: m.nodes.length, links: m.edges.filter(e => e.kind === 'observed').length, inferred: m.edges.filter(e => e.kind === 'inferred').length })
  return { all: count(base), shown: count(shown) }
}

// ---- the address (so that a view can be shared and reopened exactly, roadmap 7.8.4 and V8) ---------------------------------------------------------------------------
export function filtersFromSearch(search) {
  const p = new URLSearchParams(search)
  const f = emptyFilters()
  f.targets = new Set((p.get('t') || '').split(',').map(normaliseNumber).filter(Boolean))
  f.carriers = new Set((p.get('c') || '').split(',').filter(Boolean))
  if (/^\d{4}-\d{2}-\d{2}$/.test(p.get('from') || '')) f.from = p.get('from')
  if (/^\d{4}-\d{2}-\d{2}$/.test(p.get('to') || '')) f.to = p.get('to')
  const min = Number(p.get('min'))
  if (Number.isInteger(min) && min > 1 && min < 10000) f.minEvents = min
  const hide = new Set((p.get('hide') || '').split(','))
  const show = new Set((p.get('show') || '').split(','))
  for (const k of ['observed', 'inferred', 'contacts', 'hubs']) if (hide.has(k)) f.show[k] = false
  if (show.has('services')) f.show.services = true
  return f
}

export function filtersToSearch(f, search = '') {
  const p = new URLSearchParams(search)
  for (const k of ['t', 'c', 'from', 'to', 'min', 'hide', 'show']) p.delete(k)
  if (f.targets.size) p.set('t', [...f.targets].join(','))
  if (f.carriers.size) p.set('c', [...f.carriers].join(','))
  if (f.from) p.set('from', f.from)
  if (f.to) p.set('to', f.to)
  if (f.minEvents > 1) p.set('min', String(f.minEvents))
  const hidden = ['observed', 'inferred', 'contacts', 'hubs'].filter(k => !f.show[k])
  if (hidden.length) p.set('hide', hidden.join(','))
  if (f.show.services) p.set('show', 'services')
  const s = p.toString()
  return s ? `?${s}` : ''
}

// ---- a graph from the rows of an answer (shape "graph": src, dst, and optionally weight) -----------------------------------------------------------------------------------
export function modelFromRows(rows, { src = 'src', dst = 'dst', weight = null } = {}) {
  const nodes = new Map()
  const edges = []
  const ensure = id => {
    const key = String(id)
    if (!nodes.has(key)) {
      const c = carrierOfNumber(key)
      nodes.set(key, { id: key, label: looksLikeNumber(key) ? formatMobile(key) : key, kind: 'contact', carrier: c.id, carrierHow: c.how, events: 0 })
    }
    return nodes.get(key)
  }
  const seen = new Set()
  for (const row of rows || []) {
    const a = row[src], b = row[dst]
    if (a == null || b == null || a === b) continue
    const key = `${a}>${b}`
    if (seen.has(key)) continue
    seen.add(key)
    const w = weight && Number.isFinite(Number(row[weight])) ? Number(row[weight]) : 1
    ensure(a).events += w
    ensure(b).events += w
    edges.push({ id: `r${edges.length}`, source: String(a), target: String(b), kind: 'observed', count: w })
  }
  // the numbers an answer starts from are drawn as subjects
  const outgoing = new Set(edges.map(e => e.source))
  const incoming = new Set(edges.map(e => e.target))
  for (const id of outgoing) if (!incoming.has(id)) nodes.get(id).kind = 'subject'
  const all = [...nodes.values()]
  return { nodes: all, edges, subjects: all.filter(n => n.kind === 'subject').map(n => n.id), totals: {}, truncated: false }
}

// ---- layout (deterministic, so that the same graph always lands in the same place) -----------------------------------------------------------------------------------------
function rng(seed) {
  let a = seed >>> 0
  return () => {
    a |= 0
    a = (a + 0x6D2B79F5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

export function layoutGraph(nodes, edges, iterations = 220) {
  const r = rng(7)
  const pos = new Map()
  const n = nodes.length
  nodes.forEach((node, i) => pos.set(node.id, { x: Math.cos(i / n * 6.283) * 300 + (r() - 0.5) * 40, y: Math.sin(i / n * 6.283) * 300 + (r() - 0.5) * 40, vx: 0, vy: 0 }))
  const k = Math.sqrt((900 * 900) / Math.max(n, 1)) * 0.45
  for (let it = 0; it < iterations; it++) {
    const cool = 1 - it / iterations
    if (n <= 1500) {
      for (let i = 0; i < n; i++) {
        for (let j = i + 1; j < n; j++) {
          const a = pos.get(nodes[i].id), b = pos.get(nodes[j].id)
          const dx = a.x - b.x, dy = a.y - b.y
          const d2 = dx * dx + dy * dy + 0.01, f = (k * k) / d2
          a.vx += dx * f; a.vy += dy * f; b.vx -= dx * f; b.vy -= dy * f
        }
      }
    }
    for (const e of edges) {
      const a = pos.get(e.source), b = pos.get(e.target)
      if (!a || !b) continue
      const dx = b.x - a.x, dy = b.y - a.y
      const d = Math.sqrt(dx * dx + dy * dy) + 0.01, f = (d * d) / k / d
      a.vx += dx * f * 0.02; a.vy += dy * f * 0.02; b.vx -= dx * f * 0.02; b.vy -= dy * f * 0.02
    }
    for (const p of pos.values()) {
      const s = Math.min(8, Math.hypot(p.vx, p.vy)) * cool
      const m = Math.hypot(p.vx, p.vy) || 1
      p.x += p.vx / m * s; p.y += p.vy / m * s; p.vx = p.vy = 0
    }
  }
  const xs = [...pos.values()].map(p => p.x), ys = [...pos.values()].map(p => p.y)
  if (!xs.length) return pos
  const x0 = Math.min(...xs), x1 = Math.max(...xs), y0 = Math.min(...ys), y1 = Math.max(...ys)
  const s = Math.min(900 / Math.max(x1 - x0, 1), 700 / Math.max(y1 - y0, 1))
  for (const p of pos.values()) { p.x = (p.x - (x0 + x1) / 2) * s; p.y = (p.y - (y0 + y1) / 2) * s }
  return pos
}

// ---- overview mode for big graphs (roadmap 7.7 rule V5): contacts grouped by community, a group opens on click ---------------------------------------------------------------------
export function communities(nodes, edges, rounds = 12) {
  const ids = nodes.map(n => n.id).sort()
  const nb = new Map(ids.map(i => [i, []]))
  for (const e of edges) {
    if (e.kind === 'observed' && nb.has(e.source) && nb.has(e.target)) { nb.get(e.source).push([e.target, e.count]); nb.get(e.target).push([e.source, e.count]) }
  }
  const label = new Map(ids.map(i => [i, i]))
  for (let r = 0; r < rounds; r++) {
    let changed = 0
    for (const id of ids) {
      const score = new Map()
      for (const [other, w] of nb.get(id)) score.set(label.get(other), (score.get(label.get(other)) || 0) + w)
      if (!score.size) continue
      let best = null, bestW = -1
      for (const [l, w] of [...score].sort((a, b) => (a[0] < b[0] ? -1 : 1))) if (w > bestW) { best = l; bestW = w }
      if (best !== label.get(id)) { label.set(id, best); changed++ }
    }
    if (!changed) break
  }
  return label
}

export function collapse(nodes, edges, label, { minGroup = 3, expanded = new Set() } = {}) {
  const members = new Map()
  for (const n of nodes) {
    if (n.kind !== 'contact') continue
    const l = label.get(n.id)
    if (!members.has(l)) members.set(l, [])
    members.get(l).push(n.id)
  }
  const groupOf = new Map()
  const groups = []
  for (const [l, ids] of [...members].sort((a, b) => (a[0] < b[0] ? -1 : 1))) {
    const gid = `grp:${l}`
    if (ids.length >= minGroup && !expanded.has(gid)) { ids.forEach(i => groupOf.set(i, gid)); groups.push({ id: gid, members: ids.slice().sort() }) }
  }
  const outNodes = nodes.filter(n => !groupOf.has(n.id)).concat(groups.map(g => ({ id: g.id, kind: 'group', label: `${g.members.length} numbers`, members: g.members, count: g.members.length, carrier: 'other' })))
  const rep = id => groupOf.get(id) ?? id
  const merged = new Map()
  const outEdges = []
  for (const e of edges) {
    const a = rep(e.source), b = rep(e.target)
    if (a === b) continue
    if (a === e.source && b === e.target) { outEdges.push(e); continue }
    if (e.kind !== 'observed') { outEdges.push({ ...e, id: `g${outEdges.length}`, source: a, target: b, group: true }); continue }
    const key = a < b ? `${a}|${b}` : `${b}|${a}`
    if (!merged.has(key)) merged.set(key, { id: `m${merged.size}`, source: a, target: b, kind: 'observed', count: 0, links: 0, group: true })
    const m = merged.get(key)
    m.count += e.count
    m.links++
  }
  return { nodes: outNodes, edges: [...outEdges, ...merged.values()], groups, groupOf }
}
