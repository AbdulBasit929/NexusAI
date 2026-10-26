// The Ask engine's answer, read as marks for the workbench (docs/ux/workbench/RESEARCH_NOTE.md, rule "every mark opens its evidence"). The engine returns what it was asked as rows of an analyst view
// (api/forensic_records/ask_engine_response.go: the sentence built by code, the steps in plain words, the limitations in fixed words, the rows). This module turns those rows into the things the
// workbench draws: a link between two numbers, a dot on the timeline, a cell site on the map, a track between two sites, each with the facts of its own row, and nothing it cannot read from the
// row. A result it cannot place (a count, a ranked list of dates) is shown as the sentence and the table, never as an invented picture.
//
// What a mark is made of comes from the VIEW the engine read and the columns it returned. The model often selects only some columns (it may leave out `a` and `b` when the question names the two numbers),
// so a missing number is taken from the numbers of the QUESTION, which the engine's own checks have shown are the numbers the query is about.
import { presentInvestigationResponse } from '../investigationPresentation.js'
import { humanizeKey } from '../format.js'
import { clockOf, createCaseIndex, dateOf, edgeKey, looksLikeNumber, minutesBetween } from './caseIndex.js'

// How strong what a view shows is (the UX contract, section 4): observed is in the records, inferred is derived from them by a stated rule, a candidate is a lead that proves nothing.
export const STRENGTH = {
  observed: { label: 'Observed', note: 'in the records' },
  inferred: { label: 'Inferred', note: 'derived from the records by a stated rule' },
  candidate: { label: 'Candidate', note: 'a lead, not a finding' },
}
const ORDER = ['candidate', 'inferred', 'observed']
const VIEW_STRENGTH = {
  v_meeting_candidates: 'candidate', v_successor_candidates: 'candidate',
  v_colocation_windows: 'inferred', v_contact_links: 'inferred', v_connection_edges: 'inferred', v_weekly_anchors: 'inferred', v_anchor_runs: 'inferred', v_movement_anchors: 'inferred', v_anchor_changes: 'inferred', v_anchor_deviations: 'inferred',
}
// the weakest strength among the views an answer read
export function strengthOfViews(views = []) {
  let rank = ORDER.length - 1
  for (const view of views) { const s = VIEW_STRENGTH[view]; if (s) rank = Math.min(rank, ORDER.indexOf(s)) }
  return ORDER[rank]
}

// the kind of answer, by the most specific analysis view it read
const KIND_OF_VIEW = [
  ['v_meeting_candidates', 'meeting'], ['v_colocation_windows', 'window'], ['v_successor_candidates', 'successor'], ['v_silent_numbers', 'silent'], ['v_anchor_changes', 'move'], ['v_speed_alerts', 'alert'],
  ['v_rhythm_changes', 'rhythm'], ['v_movement_anchors', 'anchor'], ['v_anchor_runs', 'anchor'], ['v_anchor_deviations', 'anchor'], ['v_weekly_anchors', 'anchor'],
  ['v_connection_edges', 'path'], ['v_contact_links', 'path'], ['v_contact_pairs', 'contact'], ['v_cell_visits', 'site'], ['v_cells', 'site'],
]
export const LAYER_OF_KIND = { window: 'windows', meeting: 'meetings', successor: 'successors', silent: 'successors', move: 'moves', anchor: 'moves', alert: 'alerts', rhythm: 'rhythm' }
export const LAYER_LABELS = { contacts: 'Contacts', windows: 'Same place', meetings: 'Possible meetings', successors: 'Candidate successors', moves: 'Places by night and day', alerts: 'Impossible speed', rhythm: 'Change of rhythm' }
export const LAYER_DOT = { contacts: 'c', windows: '2', meetings: '3', successors: '1', moves: '1', alerts: 'f', rhythm: '4' }

const FOCUS_NUMBER = /\d{10,13}/g
export const numbersOfQuestion = question => [...new Set(String(question ?? '').match(FOCUS_NUMBER) || [])]

const present = value => value !== null && value !== undefined && value !== ''
const num = (value, places = 4) => {
  const n = Number(value)
  if (!Number.isFinite(n)) return String(value)
  return Number.isInteger(n) ? n.toLocaleString() : String(Number(n.toFixed(places)))
}
const TIMEY = /^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}/

// a cell of the answer's table, as the analyst reads it
export function displayCell(value) {
  if (value === null || value === undefined || value === '') return '—'
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  if (typeof value === 'number') return num(value)
  const text = String(value)
  return TIMEY.test(text) ? clockOf(text) : text
}

function facts(list) { return list.filter(([, v]) => present(v)) }

// ---- one row of each kind ---------------------------------------------------------------------------------------------------------------------------------------------------------
const siteLabel = (index, cell, row) => row?.site_name || index.siteOf(cell)?.name || String(cell ?? '').split(':').pop() || ''
const withUnit = (value, unit) => (present(value) ? `${num(value)} ${unit}` : '')

function windowMark(row, i, ctx, kind) {
  const [fa, fb] = ctx.focus
  const a = row.a || fa; const b = row.b || fb
  const start = row.start_utc; const end = row.end_utc
  const length = minutesBetween(start, end)
  const meeting = kind === 'meeting'
  return {
    id: `${kind}:${i}`, kind, row: i, numbers: [a, b].filter(Boolean), time: start, end, day: ctx.index.dayIndex(start), cells: row.cell_key ? [row.cell_key] : [],
    title: meeting ? 'A possible meeting' : 'Together at one cell site',
    subtitle: `${clockOf(start)}${present(end) ? ` to ${clockOf(end).slice(11)}` : ''}`,
    label: meeting ? `possible meeting ${clockOf(start)}` : `${clockOf(start)} at one cell site`,
    facts: facts([['Start', clockOf(start)], ['End', clockOf(end)], ['Length', length == null ? '' : `${length} min`], ['Cell site', siteLabel(ctx.index, row.cell_key, row)], ['Pairs of events in it', present(row.pairs) ? num(row.pairs) : ''],
      ...(meeting ? [['Last contact before it', clockOf(row.last_contact_utc)], ['Gap to that contact', withUnit(row.gap_minutes, 'min')], ['Contacts in the gap', present(row.contacts) ? num(row.contacts) : ''], ['Score', present(row.score) ? num(row.score, 2) : '']] : [])]),
  }
}
function successorMark(row, i, ctx) {
  const dayOfNew = ctx.index.dayIndex(row.new_first_event_utc)
  const old = row.old_number || ctx.focus[0]
  return {
    id: `successor:${i}`, kind: 'successor', row: i, numbers: [old, row.new_number].filter(Boolean), time: row.new_first_event_utc, day: dayOfNew, cells: [],
    title: 'A candidate successor', subtitle: `${labelOfNumber(old)} to ${labelOfNumber(row.new_number)}`,
    label: `${labelOfNumber(row.new_number)} may have taken over, score ${num(row.score, 2)}`,
    facts: facts([['Score', present(row.score) ? num(row.score, 4) : ''], ['Contact similarity', present(row.contact_cosine) ? num(row.contact_cosine, 4) : ''], ['Shared contacts', present(row.shared_contacts) ? num(row.shared_contacts) : ''],
      ['Shared places', present(row.cell_jaccard) ? num(row.cell_jaccard, 4) : ''], ['Shared handset', present(row.shared_device) ? (row.shared_device ? 'yes' : 'no') : ''], ['Old number last seen', clockOf(row.old_last_event_utc)], ['New number first seen', clockOf(row.new_first_event_utc)],
      ['Silence before it started', withUnit(row.silence_days, 'days')]]),
  }
}
// the number a single-number row is about: its own column, or else the one number the question names
const ownerOf = (row, ctx) => row.number || ctx.focus[0]
function silentMark(row, i, ctx) {
  const who = ownerOf(row, ctx)
  return {
    id: `silent:${i}`, kind: 'silent', row: i, numbers: [who].filter(Boolean), time: row.last_event_utc, day: ctx.index.dayIndex(row.last_event_utc), cells: [],
    title: 'A number that went silent', subtitle: `${labelOfNumber(who)} last seen ${dateOf(row.last_event_utc)}`, label: `${labelOfNumber(who)} last seen ${clockOf(row.last_event_utc)}`,
    facts: facts([['First event', clockOf(row.first_event_utc)], ['Last event', clockOf(row.last_event_utc)], ['Events', present(row.events) ? num(row.events) : ''], ['Silent for', withUnit(row.days_silent, 'days')]]),
  }
}
function moveMark(row, i, ctx) {
  const cells = [row.from_cell_key, row.to_cell_key].filter(Boolean)
  const who = ownerOf(row, ctx)
  return {
    id: `move:${i}`, kind: 'move', row: i, numbers: [who].filter(Boolean), time: row.first_week_start, day: ctx.index.dayIndex(row.first_week_start), cells, track: cells.length === 2 ? cells : null,
    title: 'A change of place', subtitle: `${labelOfNumber(who)}, ${row.slot ? `at ${row.slot}` : 'new place'} from the week of ${dateOf(row.first_week_start)}`,
    label: `new ${row.slot || ''} place from ${dateOf(row.first_week_start)}`.replace(/\s+/g, ' '),
    facts: facts([['Part of the day', row.slot], ['New place from the week of', dateOf(row.first_week_start)], ['Distance', withUnit(row.distance_km, 'km')], ['Before', row.from_cell_key], ['After', row.to_cell_key], ['Weeks it must hold', present(row.hold_weeks) ? num(row.hold_weeks) : '']]),
  }
}
function alertMark(row, i, ctx) {
  const cells = [row.from_cell_key, row.to_cell_key].filter(Boolean)
  const who = ownerOf(row, ctx)
  return {
    id: `alert:${i}`, kind: 'alert', row: i, numbers: [who].filter(Boolean), time: row.from_time_utc, end: row.to_time_utc, day: ctx.index.dayIndex(row.from_time_utc), cells, track: cells.length === 2 ? cells : null,
    title: `${num(row.speed_kmh, 1)} km/h between two events`, subtitle: `${labelOfNumber(who)}, ${clockOf(row.from_time_utc)}`, label: `${num(row.speed_kmh, 0)} km/h at ${clockOf(row.from_time_utc)}`,
    facts: facts([['Distance', withUnit(row.distance_km, 'km')], ['Gap', withUnit(row.gap_s, 's')], ['Speed', withUnit(row.speed_kmh, 'km/h')], ['Lowest speed listed', withUnit(row.limit_kmh, 'km/h')], ['From', clockOf(row.from_time_utc)], ['To', clockOf(row.to_time_utc)], ['From cell site', row.from_cell_key], ['To cell site', row.to_cell_key]]),
  }
}
function rhythmMark(row, i, ctx) {
  const who = ownerOf(row, ctx)
  return {
    id: `rhythm:${i}`, kind: 'rhythm', row: i, numbers: [who].filter(Boolean), time: row.first_day, day: ctx.index.dayIndex(row.first_day), cells: [],
    title: `A change from ${dateOf(row.first_day)}`, subtitle: `${labelOfNumber(who)}: ${row.direction === 'down' ? 'down' : 'up'} by a factor of ${num(row.ratio, 2)}`, label: `rhythm changes ${dateOf(row.first_day)}`,
    facts: facts([['Average before', withUnit(row.mean_before, 'a day')], ['Average after', withUnit(row.mean_after, 'a day')], ['Ratio', present(row.ratio) ? num(row.ratio, 2) : ''], ['Welch t', present(row.t) ? num(row.t, 2) : ''], ['Days in the series', present(row.days) ? num(row.days) : ''], ['Least ratio asked', present(row.ratio_percent) ? `${num(row.ratio_percent)} percent` : '']]),
    markLineDay: ctx.index.dayIndex(row.first_day),
  }
}
function anchorMark(row, i, ctx) {
  const when = row.first_week_start || row.week_start
  const cell = row.cell_key
  const who = ownerOf(row, ctx)
  return {
    id: `anchor:${i}`, kind: 'anchor', row: i, numbers: [who].filter(Boolean), time: when, day: when ? ctx.index.dayIndex(when) : null, cells: cell ? [cell] : [],
    title: `Place by ${row.slot || 'time of day'}`, subtitle: `${labelOfNumber(who)}${when ? `, from the week of ${dateOf(when)}` : ''}`, label: `${row.slot || ''} place ${siteLabel(ctx.index, cell, row)}`.trim(),
    facts: facts([['Part of the day', row.slot], ['Cell site', siteLabel(ctx.index, cell, row)], ['From the week of', when ? dateOf(when) : ''], ['Weeks', present(row.weeks) ? num(row.weeks) : ''], ['Events at it', present(row.events_at_anchor) ? num(row.events_at_anchor) : ''], ['Weeks it must hold', present(row.hold_weeks) ? num(row.hold_weeks) : '']]),
  }
}
const labelOfNumber = number => (looksLikeNumber(number) ? `…${String(number).replace(/\D/g, '').slice(-3)}` : String(number ?? ''))

const BUILD = { window: (r, i, c) => windowMark(r, i, c, 'window'), meeting: (r, i, c) => windowMark(r, i, c, 'meeting'), successor: successorMark, silent: silentMark, move: moveMark, alert: alertMark, rhythm: rhythmMark, anchor: anchorMark }

// ---- the engine's limitations, sorted into what they are ---------------------------------------------------------------------------------------------------------------------------
const PARAM_NOTE = /^(.+?) used: (.+?) \((read from your question|the default[^)]*)\)\.?$/
const REPEATS = /^([\d,]+) rows of this case's call and internet records are exact repeats/
const CLOCK = /^Times are read and shown on the case clock \((.+)\)\.?$/
const MODEL_NOTE = /^Computed by a model-written query/
const TRUNCATED = /^Only the first [\d,]+ rows are shown/

export function sortLimitations(limitations = []) {
  const out = { values: [], caveats: [], repeats: null, clock: '', truncated: '' }
  for (const raw of limitations) {
    const text = String(raw ?? '').trim()
    if (!text || MODEL_NOTE.test(text)) continue
    let m
    if ((m = PARAM_NOTE.exec(text))) { out.values.push({ label: m[1], value: m[2], source: m[3].startsWith('the default') ? 'the default' : 'from your question' }); continue }
    if ((m = REPEATS.exec(text))) { out.repeats = Number(m[1].replace(/,/g, '')); continue }
    if ((m = CLOCK.exec(text))) { out.clock = m[1]; continue }
    if (TRUNCATED.test(text)) { out.truncated = text; continue }
    out.caveats.push(text)
  }
  return out
}

// a clarification, an abstention or a "too broad": the engine's own words, and which of the designed states it is
function stateOfText(text) {
  if (/^This question is too broad/i.test(text)) return 'broad'
  if (/^I did not give an answer/i.test(text)) return 'abstained'
  return 'clarify'
}

// ---- the whole answer -----------------------------------------------------------------------------------------------------------------------------------------------------------------
// An index of nothing: what an answer is read against before the case's own reads have arrived (days and positions are then unknown, and the marks carry no day)
const EMPTY_INDEX = createCaseIndex({})

// Whether an answer of the engine can be drawn at all (a mark, a link the answer names, a cell site), from the answer alone: the Ask page asks this before it reads the case for the picture.
export function isDrawable(response, question = '') {
  if (response?.policy !== 'ask_engine_v2') return false
  const a = readAnswer(response, { index: EMPTY_INDEX, question })
  return ['answered', 'partial'].includes(a.state) && (a.marks.length > 0 || a.emph.length > 0 || a.sites.length > 0)
}

export function readAnswer(response, { index = EMPTY_INDEX, question = '', caseId = '', error = null } = {}) {
  const presentation = presentInvestigationResponse(response, { caseId, error })
  const enterprise = response?.enterprise || {}
  const base = { question, presentation, state: presentation.state, sentence: presentation.answer || '', marks: [], links: [], emph: [], numbers: numbersOfQuestion(question), answerNumbers: [], sites: [], tracks: [], layers: [], rows: [], columns: [], table: { columns: [], rows: [], keys: [] }, views: [], steps: [], checks: [], values: [], caveats: [], repeats: null, clock: '', truncated: '', strength: 'observed', kind: null, shape: '' }
  if (presentation.state === 'clarify' || presentation.state === 'unsupported' || presentation.state === 'failed') {
    const text = String(response?.answer?.clarification || presentation.clarification?.question || presentation.answer || '').trim()
    const steps = (enterprise.derivation?.steps || []).map(String)
    const stateId = presentation.state === 'clarify' ? stateOfText(text) : presentation.state
    return { ...base, state: stateId, sentence: text, steps, caveats: (enterprise.limitations || []).map(String), clarification: presentation.clarification || null, reference: presentation.errorReference || '' }
  }
  const derivation = enterprise.derivation || {}
  const views = (derivation.views || enterprise.provenance?.[0]?.views || []).map(String)
  const gridRows = Array.isArray(enterprise.data_grid?.rows) ? enterprise.data_grid.rows : []
  const sqlColumns = (response?.answer?.ask_engine_v2?.columns || []).map(String)
  const gridColumns = (enterprise.data_grid?.columns || []).map(c => c.key)
  const columns = sqlColumns.length ? sqlColumns : gridColumns
  const labels = Object.fromEntries((presentation.result?.columns || []).map(c => [c.key, c.label]))
  const sorted = sortLimitations(enterprise.limitations)
  const kind = (KIND_OF_VIEW.find(([view]) => views.includes(view)) || [])[1] || null
  const ctx = { index, focus: base.numbers, kind }

  const marks = []
  const links = []
  const answerNumbers = new Set()
  const note = n => { if (looksLikeNumber(n)) answerNumbers.add(String(n)) }
  gridRows.forEach((row, i) => {
    if (BUILD[kind]) {
      const mark = BUILD[kind](row, i, ctx)
      marks.push(mark)
      mark.numbers.forEach(note)
    } else {
      for (const key of ['number', 'src', 'dst', 'a', 'b', 'label', 'old_number', 'new_number']) note(row[key])
    }
  })
  base.numbers.forEach(note)

  // links between numbers: one for each pair (and kind) the marks join, with the marks behind it; and the rows of a contact or path answer, which are the links themselves
  const pairLinks = new Map()
  for (const mark of marks) {
    const link = { window: 'colocated', meeting: 'meeting', successor: 'successor' }[mark.kind]
    if (!link || mark.numbers.length < 2) continue
    const key = link === 'successor' ? `s:${mark.numbers[0]}|${mark.numbers[1]}` : `${link === 'colocated' ? 'w' : 'm'}:${edgeKey(mark.numbers[0], mark.numbers[1])}`
    if (!pairLinks.has(key)) pairLinks.set(key, { id: key, source: mark.numbers[0], target: mark.numbers[1], kind: link, events: 0, markIds: [] })
    const l = pairLinks.get(key); l.events += 1; l.markIds.push(mark.id)
  }
  links.push(...pairLinks.values())
  const emph = new Set()
  if (kind === 'path' || kind === 'contact') {
    for (const row of gridRows) {
      const source = row.src ?? row.a; const target = row.dst ?? row.b
      if (looksLikeNumber(source) && looksLikeNumber(target) && source !== target) emph.add(edgeKey(String(source), String(target)))
      else if (looksLikeNumber(row.label) && base.numbers.length === 1) emph.add(edgeKey(base.numbers[0], String(row.label)))
    }
  }

  // sites and tracks the marks name
  const cells = new Set()
  for (const mark of marks) mark.cells.forEach(c => cells.add(c))
  for (const row of gridRows) if (!BUILD[kind] && row.cell_key) cells.add(row.cell_key)
  const tracks = marks.filter(m => m.track).map(m => ({ id: m.id, cells: m.track }))

  const layers = [...new Set(marks.map(m => LAYER_OF_KIND[m.kind]).filter(Boolean))]
  const table = {
    keys: columns,
    columns: columns.map(key => labels[key] || humanizeKey(key)),
    rows: gridRows.map(row => columns.map(key => displayCell(row[key]))),
  }
  const zero = gridRows.length === 0
  return {
    ...base, state: zero ? 'zero-result' : presentation.state === 'partial' ? 'partial' : 'answered', sentence: presentation.answer || enterprise.executive_answer || '',
    shape: String(enterprise.result_shape || response?.answer?.ask_engine_v2?.shape || ''), views, kind, sql: String(derivation.sql || ''), steps: (derivation.steps || []).map(String), checks: (derivation.conditions_checked || []).map(String),
    strength: strengthOfViews(views), values: sorted.values, caveats: sorted.caveats, repeats: sorted.repeats, clock: sorted.clock, truncated: sorted.truncated,
    rows: gridRows, columns, table, marks, links, emph: [...emph], layers, answerNumbers: [...answerNumbers], sites: [...cells], tracks,
    rowCount: Number(enterprise.row_count ?? gridRows.length),
    latencyMs: Number(response?.telemetry?.total_latency_ms || 0),
  }
}
