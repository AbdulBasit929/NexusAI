import { attentionItems, reviewByCase } from '../../../lib/dashboardCharts.js'
import { relativeAge } from '../../../lib/dashboardCases.js'
import { curatedFamilyLabel } from '../../../lib/semanticCatalog.js'

// What the dashboard draws, as pure functions of what the services reported. Nothing here fetches. A number is a count a service gave, a share of two such counts,
// or a median of dated jobs; a case that could not be read is left out and counted as unread, never as zero.

const formatNumber = value => new Intl.NumberFormat('en-US').format(Number(value) || 0)

export const familyName = id => (id === 'generic' ? 'Other records' : curatedFamilyLabel(id))

const isoToday = () => new Date().toISOString().slice(0, 10)
const number = value => (Number.isFinite(Number(value)) ? Number(value) : 0)
const sum = (items, pick) => items.reduce((total, item) => total + number(pick(item)), 0)

// ---------- time ----------
export function formatDuration(seconds) {
  if (!Number.isFinite(seconds)) return '—'
  const whole = Math.round(seconds)
  if (whole < 1) return '<1 s'
  if (whole < 60) return `${whole} s`
  const minutes = Math.floor(whole / 60)
  const rest = whole % 60
  if (minutes < 60) return rest ? `${minutes} min ${rest} s` : `${minutes} min`
  const hours = Math.floor(minutes / 60)
  return minutes % 60 ? `${hours} h ${minutes % 60} min` : `${hours} h`
}

const quantile = (sorted, fraction) => {
  if (!sorted.length) return null
  const position = (sorted.length - 1) * fraction
  const low = Math.floor(position)
  const high = Math.ceil(position)
  return sorted[low] + (sorted[high] - sorted[low]) * (position - low)
}

// ---------- the evidence timeline: one row for each case, the events of each month ----------
// The service returns at most 100 rows (a day and a family each), oldest first, so a case with more days is read only as far as those rows go and is marked `partial`.
// A day dated after today is a fault in the source's clock, so it is counted and named but never drawn.
const MAX_BARS = 96

export function caseTimelines(results, today = isoToday()) {
  return (results || []).map(result => {
    const past = result.days.filter(day => day.date <= today)
    const future = result.days.filter(day => day.date > today)
    const months = new Map()
    for (const day of past) months.set(day.date.slice(0, 7), (months.get(day.date.slice(0, 7)) || 0) + day.total)
    const keys = [...months.keys()].sort()
    let series = []
    if (keys.length) {
      let [year, month] = keys[0].split('-').map(Number)
      const [lastYear, lastMonth] = keys.at(-1).split('-').map(Number)
      while (year < lastYear || (year === lastYear && month <= lastMonth)) {
        const key = `${year}-${String(month).padStart(2, '0')}`
        series.push({ from: key, to: key, total: months.get(key) || 0 })
        month += 1
        if (month > 12) { month = 1; year += 1 }
      }
    }
    const monthly = series.map(point => ({ key: point.from, total: point.total }))
    // a very long span is drawn in equal groups of months so a bar never gets thinner than a pixel; the totals do not change
    if (series.length > MAX_BARS) {
      const size = Math.ceil(series.length / MAX_BARS)
      const grouped = []
      for (let index = 0; index < series.length; index += size) {
        const part = series.slice(index, index + size)
        grouped.push({ from: part[0].from, to: part.at(-1).to, total: sum(part, item => item.total) })
      }
      series = grouped
    }
    return {
      caseId: result.caseId,
      series,
      monthly,
      total: sum(past, day => day.total),
      first: past[0]?.date || null,
      last: past.at(-1)?.date || null,
      partial: Boolean(result.truncated),
      futureTotal: sum(future, day => day.total),
      futureLast: future.at(-1)?.date || null,
    }
  }).filter(item => item.series.length || item.futureTotal)
}

// The same cases on ONE calendar, so the rows can be read against each other: the axis runs from the earliest month any case has to the current month, in equal groups when it is long (the groups are
// the same for every row, so the columns line up). A cell is `in` the case's own span, `before` it, or `after` it; after the last month a case that was read only in part (`partial`) is `unread`, which
// is not the same as empty: the service stopped returning rows, it did not say there were none. `ticks` are the year starts (thinned to at most seven), `groupSize` is the months in a cell.
export function alignTimelines(timelines, today = isoToday()) {
  const lanes = timelines || []
  const dated = lanes.filter(item => item.monthly?.length)
  if (!dated.length) return null
  const first = dated.map(item => item.monthly[0].key).sort()[0]
  const todayKey = today.slice(0, 7)
  const last = [todayKey, ...dated.map(item => item.monthly.at(-1).key)].sort().at(-1)
  const keys = []
  let [year, month] = first.split('-').map(Number)
  const [lastYear, lastMonth] = last.split('-').map(Number)
  while (year < lastYear || (year === lastYear && month <= lastMonth)) {
    keys.push(`${year}-${String(month).padStart(2, '0')}`)
    month += 1
    if (month > 12) { month = 1; year += 1 }
  }
  const groupSize = Math.max(1, Math.ceil(keys.length / MAX_BARS))
  const groups = []
  for (let index = 0; index < keys.length; index += groupSize) {
    const part = keys.slice(index, index + groupSize)
    groups.push({ from: part[0], to: part.at(-1) })
  }
  const rows = lanes.map(item => {
    if (!item.monthly?.length) return { ...item, cells: groups.map(group => ({ ...group, total: 0, state: 'before' })), unreadFrom: null } // only dates in the future: named by the row, nothing to draw
    const byMonth = new Map(item.monthly.map(point => [point.key, point.total]))
    const start = item.monthly[0].key
    const end = item.monthly.at(-1).key
    let unreadFrom = null
    const cells = groups.map((group, index) => {
      if (group.to < start) return { ...group, total: 0, state: 'before' }
      if (group.from > end) {
        if (item.partial && unreadFrom === null) unreadFrom = index
        return { ...group, total: 0, state: item.partial ? 'unread' : 'after' }
      }
      return { ...group, total: keys.slice(keys.indexOf(group.from), keys.indexOf(group.to) + 1).reduce((total, key) => total + (byMonth.get(key) || 0), 0), state: 'in' }
    })
    return { ...item, cells, unreadFrom }
  })
  const yearOf = group => group.from.slice(0, 4)
  const starts = groups.map((group, index) => ({ index, year: yearOf(group) })).filter((tick, index) => index === 0 || tick.year !== yearOf(groups[index - 1]))
  const step = Math.max(1, Math.ceil(starts.length / 7))
  return { lanes: rows, groups, groupSize, ticks: starts.filter((tick, position) => position % step === 0) }
}

// ---------- the view of the timeline: a window over the groups of the shared calendar ----------
// A window is [from, to) in group indexes, or null for the whole calendar. It is never narrower than MIN_WINDOW groups (a drag of one cell is a click, not a zoom) and never leaves the calendar.
export const MIN_WINDOW = 3

export function viewWindow(count, win) {
  if (!win || !count) return [0, Math.max(0, count)]
  const from = Math.max(0, Math.min(count - 1, Math.floor(win[0])))
  const to = Math.max(from + 1, Math.min(count, Math.ceil(win[1])))
  if (to - from < Math.min(MIN_WINDOW, count)) return [0, count]
  return from === 0 && to === count ? [0, count] : [from, to]
}

// Zoom in (factor < 1) or out (factor > 1) around a group, keeping that group at the same place in the view; the result is the whole calendar (null) when it would show everything.
export function zoomWindow(count, win, factor, around) {
  const [from, to] = viewWindow(count, win)
  const span = to - from
  const next = Math.max(Math.min(MIN_WINDOW, count), Math.min(count, Math.round(span * factor)))
  if (next >= count) return null
  const anchor = Number.isFinite(around) ? Math.min(to - 1, Math.max(from, around)) : from + span / 2
  const share = span ? (anchor - from) / span : 0.5
  const start = Math.max(0, Math.min(count - next, Math.round(anchor - share * next)))
  return [start, start + next]
}

// Move the view by `steps` groups, staying inside the calendar.
export function panWindow(count, win, steps) {
  const [from, to] = viewWindow(count, win)
  if (from === 0 && to === count) return null
  const span = to - from
  const start = Math.max(0, Math.min(count - span, from + steps))
  return [start, start + span]
}

// Tick marks for the groups in view: the starts of years when the view is long, the starts of quarters when it is short. Each is { index (within the view), label }.
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
export function windowTicks(groups) {
  const list = groups || []
  if (!list.length) return []
  const years = list.map((group, index) => ({ index, year: group.from.slice(0, 4), month: Number(group.from.slice(5, 7)) })).filter((tick, index) => index === 0 || tick.year !== list[index - 1].from.slice(0, 4))
  if (list.length > 30 || years.length >= 3) {
    const step = Math.max(1, Math.ceil(years.length / 7))
    return years.filter((tick, position) => position % step === 0).map(tick => ({ index: tick.index, label: tick.year }))
  }
  const quarters = list.map((group, index) => ({ index, year: group.from.slice(2, 4), month: Number(group.from.slice(5, 7)) })).filter(tick => (tick.month - 1) % 3 === 0 || tick.index === 0)
  const step = Math.max(1, Math.ceil(quarters.length / 6))
  return quarters.filter((tick, position) => position % step === 0).map(tick => ({ index: tick.index, label: tick.month === 1 ? `${MONTHS[0]} ’${tick.year}` : `${MONTHS[tick.month - 1]}${tick.index === 0 ? ` ’${tick.year}` : ''}` }))
}

// ---------- answers: what the engine did with the questions of the last days ----------
const DECLINE_TEXT = [
  [/cannot answer|can not answer|no view/i, 'The case’s records cannot answer that kind of question'],
  [/ambiguous|clarif/i, 'The question was too open and needed narrowing'],
  [/not found|no (such|matching)/i, 'Nothing matching was found'],
  [/permission|forbidden|not allowed/i, 'The question asks for something the case may not show'],
]

export function declineText(reason) {
  const text = String(reason || '').trim()
  if (!text) return 'No reason recorded'
  const known = DECLINE_TEXT.find(([pattern]) => pattern.test(text))
  if (known) return known[1]
  const clipped = text.length > 90 ? `${text.slice(0, 87)}…` : text
  return clipped.charAt(0).toUpperCase() + clipped.slice(1)
}

// `items` are engine-health reads, one for each case. Percentiles cannot be merged across cases without the questions behind them, so the times are those of the
// case with the most questions and the case is named; the counts are exact sums.
export function answerQuality(items) {
  const read = (items || []).filter(item => item && item.available !== false && item.totals)
  const total = key => sum(read, item => item.totals[key])
  const questions = total('questions')
  const answered = total('answered')
  const busiest = read.filter(item => number(item.totals.questions) > 0).sort((left, right) => number(right.totals.questions) - number(left.totals.questions))[0]
  const reasons = new Map()
  for (const item of read) for (const decline of item.declines || []) reasons.set(decline.reason, (reasons.get(decline.reason) || 0) + number(decline.count))
  return {
    reported: read.length,
    questions,
    answered,
    declined: total('declined'),
    abstained: total('abstained'),
    rate: questions ? answered / questions : null,
    latency: busiest?.latency_ms?.p50 != null ? { caseId: busiest.collection_id ?? busiest.caseId, p50: busiest.latency_ms.p50 / 1000, p90: busiest.latency_ms.p90 != null ? busiest.latency_ms.p90 / 1000 : null, max: busiest.latency_ms.max != null ? busiest.latency_ms.max / 1000 : null } : null,
    reasons: [...reasons.entries()].map(([reason, count]) => ({ reason, text: declineText(reason), count })).sort((left, right) => right.count - left.count || left.text.localeCompare(right.text)),
  }
}

// ---------- what the evidence holds ----------
// A colour belongs to a kind of evidence for good (never to its rank), so a kind keeps its colour when another kind comes or goes. Six kinds have one; the rest are "Other files".
export const MODALITIES = [
  { id: 'structured_records', label: 'Records', slot: 1 },
  { id: 'document', label: 'Documents', slot: 2 },
  { id: 'image', label: 'Images', slot: 3 },
  { id: 'audio', label: 'Audio', slot: 4 },
  { id: 'video', label: 'Video', slot: 5 },
  { id: 'text', label: 'Text files', slot: 6 },
]

export function modalityMix(reports) {
  const read = (reports || []).filter(report => report && report.available !== false)
  const named = new Map(MODALITIES.map(kind => [kind.id, { ...kind, files: 0, failed: 0, caseCount: 0, byCase: [] }]))
  const other = { id: 'other', label: 'Other files', slot: 0, files: 0, failed: 0, caseCount: 0, byCase: [] }
  for (const report of read) {
    const parts = new Map()
    for (const entry of report.evidence || []) {
      const target = named.get(entry.modality) || other
      target.files += number(entry.files)
      target.failed += number(entry.failed)
      let part = parts.get(target.id)
      if (!part && number(entry.files) > 0) {
        part = { caseId: report.collection_id, files: 0, failed: 0 }
        parts.set(target.id, part)
        target.byCase.push(part)
        target.caseCount += 1
      }
      if (part) { part.files += number(entry.files); part.failed += number(entry.failed) }
    }
  }
  const kinds = [...named.values(), other].filter(kind => kind.files > 0).sort((left, right) => right.files - left.files || left.slot - right.slot)
  for (const kind of kinds) kind.byCase.sort((left, right) => right.files - left.files || String(left.caseId).localeCompare(String(right.caseId)))
  return { reported: read.length, total: sum(kinds, kind => kind.files), kinds }
}

// Calls, internet sessions and app usage are the typed records of the typing report, with exact repeats left out as the report does; what is left of the accepted rows is "Other records".
export function recordKinds(reports, rows) {
  const reportsByCase = new Map((reports || []).filter(report => report && report.available !== false).map(report => [report.collection_id, report]))
  const kinds = [{ id: 'calls', label: 'Calls', count: 0 }, { id: 'sessions', label: 'Internet sessions', count: 0 }, { id: 'usage', label: 'App usage', count: 0 }, { id: 'other', label: 'Other records', count: 0 }]
  const effective = part => Math.max(0, number(part?.effective ?? number(part?.typed) - number(part?.repeats)))
  const byCase = []
  for (const row of rows || []) {
    const report = reportsByCase.get(row.caseId)
    if (!report || !row.summary) continue
    const calls = effective(report.calls)
    const sessions = effective(report.sessions)
    const usage = effective(report.usage)
    kinds[0].count += calls
    kinds[1].count += sessions
    kinds[2].count += usage
    const other = Math.max(0, row.summary.acceptedRows - calls - sessions - usage)
    kinds[3].count += other
    byCase.push({ caseId: row.caseId, values: { calls, sessions, usage, other } })
  }
  const total = entry => entry.values.calls + entry.values.sessions + entry.values.usage + entry.values.other
  return { reported: reportsByCase.size, total: sum(kinds, kind => kind.count), kinds: kinds.filter(kind => kind.count > 0), byCase: byCase.sort((left, right) => total(right) - total(left)) }
}

// ---------- notes the service says to read before trusting an answer ----------
const NOTE_TITLE = {
  no_volume: 'Internet records state no byte volumes',
  no_public_address: 'Internet records state no public address',
  repeats: 'Exact repeats are left out of counts',
}

export function dataNotes(reports) {
  const groups = new Map()
  for (const report of reports || []) {
    if (!report || report.available === false) continue
    for (const finding of report.findings || []) {
      const group = groups.get(finding.code) || { code: finding.code, title: NOTE_TITLE[finding.code] || String(finding.code || 'Note').replace(/_/g, ' ').replace(/^./, char => char.toUpperCase()), text: finding.text, caseIds: [] }
      if (!group.caseIds.includes(report.collection_id)) group.caseIds.push(report.collection_id)
      groups.set(finding.code, group)
    }
  }
  return [...groups.values()].sort((left, right) => right.caseIds.length - left.caseIds.length || left.title.localeCompare(right.title))
}

// ---------- the pipeline ----------
// Rows read that were not kept: exact repeats and rows the reader could not accept. Ranked by share, so the case that needs a look is the first line.
export function setAside(rows) {
  const items = (rows || []).filter(row => row.summary).map(row => {
    const { acceptedRows, duplicateRows, rejectedRows } = row.summary
    const read = acceptedRows + duplicateRows + rejectedRows
    return { caseId: row.caseId, read, duplicates: duplicateRows, rejected: rejectedRows, share: read ? (duplicateRows + rejectedRows) / read : 0 }
  }).filter(item => item.read > 0)
  const read = sum(items, item => item.read)
  const kept = sum(items, item => item.duplicates + item.rejected)
  return { items: items.sort((left, right) => right.share - left.share || right.rejected - left.rejected), read, setAside: kept, share: read ? kept / read : 0 }
}

// How long files take from upload to ready, from the finished jobs each case lists (its latest 20 at most). Seconds, whole.
export function timeToReady(rows) {
  const stamp = value => { const time = value ? Date.parse(value) : NaN; return Number.isFinite(time) ? time : null }
  const cases = []
  const all = []
  const waits = []
  const processing = []
  for (const row of rows || []) {
    const finished = (row.state?.data?.recent_jobs || []).filter(job => String(job.status || '').toLowerCase() === 'completed')
    for (const job of finished) {
      const queued = stamp(job.queued_at); const started = stamp(job.started_at); const done = stamp(job.completed_at)
      if (queued !== null && started !== null && started >= queued) waits.push((started - queued) / 1000)
      if (started !== null && done !== null && done >= started) processing.push((done - started) / 1000)
    }
    const seconds = finished
      .map(job => { const from = stamp(job.queued_at); const to = stamp(job.completed_at); return from !== null && to !== null && to >= from ? (to - from) / 1000 : null })
      .filter(value => value !== null)
      .sort((left, right) => left - right)
    if (!seconds.length) continue
    all.push(...seconds)
    cases.push({ caseId: row.caseId, count: seconds.length, min: seconds[0], p25: quantile(seconds, 0.25), median: quantile(seconds, 0.5), p75: quantile(seconds, 0.75), max: seconds.at(-1) })
  }
  all.sort((left, right) => left - right)
  waits.sort((left, right) => left - right)
  processing.sort((left, right) => left - right)
  return { cases, count: all.length, median: quantile(all, 0.5), slowest: all.at(-1) ?? null, waitMedian: quantile(waits, 0.5), processMedian: quantile(processing, 0.5) }
}

// A round upper end for a shared axis of seconds: 1, 2, 5 or 10 times a power of ten.
export function niceCeiling(value) {
  if (!(value > 0)) return 1
  const power = 10 ** Math.floor(Math.log10(value))
  const fraction = value / power
  return (fraction <= 1 ? 1 : fraction <= 2 ? 2 : fraction <= 5 ? 5 : 10) * power
}

// ---------- the cases ----------
export function caseStates(records, caseIds) {
  const counts = { open: 0, on_hold: 0, closed: 0, other: 0 }
  for (const caseId of caseIds || []) {
    const status = String(records?.[caseId]?.status || 'open').toLowerCase().replace(/[\s-]+/g, '_')
    if (status in counts) counts[status] += 1
    else counts.other += 1
  }
  return counts
}

// One entry for each case, in the order the row of marks is read: ready, still processing, needs review, then the cases that have not reported. A case that is still being read counts with the latter.
const MARK_ORDER = ['ready', 'processing', 'review', 'other']
export function caseStateMix(rows) {
  const stateOf = row => (row.status === 'complete' ? 'ready' : row.status === 'processing' ? 'processing' : row.status === 'attention' ? 'review' : 'other')
  return (rows || []).map(row => ({ caseId: row.caseId, state: stateOf(row) })).sort((left, right) => MARK_ORDER.indexOf(left.state) - MARK_ORDER.indexOf(right.state))
}

// The cases that hold the most accepted rows, largest first: where the volume is.
export function largestCases(rows, limit = 4) {
  return (rows || []).filter(row => row.summary && row.summary.acceptedRows > 0).map(row => ({ caseId: row.caseId, records: row.summary.acceptedRows }))
    .sort((left, right) => right.records - left.records).slice(0, limit)
}

// A moment as the case's own clock says it, with the zone: "5 Mar 2026, 14:32 PKT" (directive 10.4). Relative times carry this as their title.
const PKT = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false, timeZone: 'Asia/Karachi' })
export const formatMoment = date => (date && !Number.isNaN(date.getTime()) ? `${PKT.format(date)} PKT` : '')

// Questions in each of the last seven 24-hour periods (the newest last), from the engine's own audit counts. The service counts a window ending now, so the window of k days minus the window
// of k-1 days is the k-th period back; the counts are exact, the periods are rolling (not calendar days). `windows[k-1]` holds the totals of the last k days.
export function dailyQuestions(items) {
  const days = Array.from({ length: 7 }, (_, index) => ({ ago: 6 - index, questions: 0, answered: 0, declined: 0, abstained: 0 }))
  for (const item of items || []) {
    const windows = item?.windows
    if (!Array.isArray(windows) || windows.length < 7) continue
    for (let k = 1; k <= 7; k += 1) {
      const now = windows[k - 1] || {}
      const before = k > 1 ? windows[k - 2] || {} : {}
      for (const key of ['questions', 'answered', 'declined', 'abstained']) days[7 - k][key] += Math.max(0, number(now[key]) - number(before[key]))
    }
  }
  return days
}

// The shade of a heat cell from its count: 0 is empty, the rest are spread on a log scale so one huge cell does not turn every other cell blank. Returns a percentage of the accent to mix in.
export function heatShare(value, max) {
  if (!(value > 0) || !(max > 0)) return 0
  return Math.round(10 + 38 * (Math.log10(1 + value) / Math.log10(1 + max)))
}

// ---------- what needs the analyst ----------
// Causes in the order they matter. The first five need an action; the rest are worth knowing. Every item is read from what the services already report (a case's latest evidence and
// jobs, the typing report, the questions and drafts kept in this browser); the counts of the first two come from the complete per-case totals, not from the named items.
const ACTIVE = new Set(['registered', 'queued', 'processing', 'running'])
const STALLED_AFTER_MS = 30 * 60 * 1000
const CAUSES = [
  { id: 'failed', label: 'Did not finish', tone: 'danger', action: 'Review', attention: true },
  { id: 'dead', label: 'Gave up after several tries', tone: 'danger', action: 'Review', attention: true },
  { id: 'missing', label: 'Retained copy missing', tone: 'danger', action: 'Review', attention: true },
  { id: 'stalled', label: 'Still processing', tone: 'warn', action: 'Open', attention: true },
  { id: 'choice', label: 'A question needs your choice', tone: 'accent', action: 'Answer', attention: true },
  { id: 'plain', label: 'Read as a plain table', tone: 'muted', action: 'Open' },
  { id: 'notes', label: 'Read before trusting answers', tone: 'muted', action: 'Read' },
  { id: 'draft', label: 'Unsent question', tone: 'muted', action: 'Continue' },
]

export function needsYou({ rows, notes = [], history = [], drafts = [], now = Date.now() }) {
  const buckets = new Map(CAUSES.map(cause => [cause.id, { ...cause, count: 0, items: [] }]))
  const add = (id, item, count = 1) => { const bucket = buckets.get(id); bucket.items.push({ action: bucket.action, ...item, id: id + '-' + item.id }); bucket.count += count }
  const evidencePath = (caseId, evidenceId) => '/cases/' + encodeURIComponent(caseId) + '/evidence' + (evidenceId ? '/' + encodeURIComponent(evidenceId) : '')

  for (const item of attentionItems(rows)) add(item.kind === 'Failed' ? 'failed' : 'missing', { id: item.caseId + '-' + (item.evidenceId || item.label), title: item.label, caseId: item.caseId, to: evidencePath(item.caseId, item.evidenceId) }, 0)
  // the named items are only the latest each case lists; the counts are the complete totals
  const totals = reviewByCase(rows)
  buckets.get('failed').count = totals.reduce((sum, entry) => sum + entry.failed, 0)
  buckets.get('missing').count = totals.reduce((sum, entry) => sum + entry.missing, 0)

  for (const row of rows) {
    const data = row.state?.data
    if (!data) continue
    for (const job of data.recent_jobs || []) {
      if (String(job.status || '').toLowerCase() !== 'dead_letter') continue
      add('dead', { id: row.caseId + '-' + (job.job_id || job.evidence_id), title: job.source_file || 'Evidence item', caseId: row.caseId, detail: job.attempt_count ? job.attempt_count + ' of ' + (job.max_attempts || '?') + ' tries' : '', to: evidencePath(row.caseId, job.evidence_id) })
    }
    let plain = 0
    for (const item of data.recent_evidence || []) {
      const status = String(item.processing_status || '').toLowerCase()
      const updated = item.updated_at ? Date.parse(item.updated_at) : NaN
      if (ACTIVE.has(status) && Number.isFinite(updated) && now - updated > STALLED_AFTER_MS) add('stalled', { id: row.caseId + '-' + item.evidence_id, title: item.source_file || 'Evidence item', caseId: row.caseId, detail: relativeAge(new Date(updated), now).replace(' ago', '') + ' so far', to: evidencePath(row.caseId, item.evidence_id) })
      if (status === 'completed' && (item.warnings || []).some(warning => /did not match a specialized/i.test(String(warning)))) plain += 1
    }
    if (plain) add('plain', { id: row.caseId, title: formatNumber(plain) + ' recent ' + (plain === 1 ? 'file was' : 'files were') + ' read as plain tables', caseId: row.caseId, detail: 'questions about them use the older path', to: evidencePath(row.caseId) })
  }
  const cutoff = now - 14 * 24 * 60 * 60 * 1000
  for (const entry of history) {
    if (entry.state === 'clarify' && Date.parse(entry.updatedAt) > cutoff) add('choice', { id: entry.caseId + '-' + entry.id, title: entry.label || entry.query, caseId: entry.caseId, to: '/cases/' + encodeURIComponent(entry.caseId) + '/investigate?question=' + encodeURIComponent(entry.query) })
  }
  for (const note of notes) add('notes', { id: note.code, title: note.title, caseIds: note.caseIds, detail: note.caseIds.length === 1 ? '' : formatNumber(note.caseIds.length) + ' cases', hint: note.text, caseId: note.caseIds.length === 1 ? note.caseIds[0] : '', to: '/cases/' + encodeURIComponent(note.caseIds[0]) + '/overview' })
  for (const draft of drafts) add('draft', { id: draft.caseId, title: draft.text, caseId: draft.caseId, to: '/cases/' + encodeURIComponent(draft.caseId) + '/investigate' })

  const groups = [...buckets.values()].filter(group => group.count > 0 || group.items.length > 0).map(group => ({ ...group, count: Math.max(group.count, group.items.length) }))
  return { groups, attention: groups.filter(group => group.attention).reduce((sum, group) => sum + group.count, 0) }
}

// ---------- how the times of the records are known ----------
// Rows of the typed records by the basis of their time: stated in the file, declared by the source, or confirmed by an analyst. What an analyst checks before they trust an hour.
const BASIS_LABEL = { confirmed_by_analyst: 'Confirmed by an analyst', explicit_offset: 'Stated in the file', declared: 'Declared by the source', assumed: 'Assumed' }
export function timeBasis(reports) {
  const rows = new Map()
  for (const report of reports || []) {
    if (!report || report.available === false) continue
    for (const layout of [...(report.calls?.layouts || []), ...(report.sessions?.layouts || [])]) {
      const id = layout.time_basis || 'unknown'
      rows.set(id, (rows.get(id) || 0) + number(layout.rows))
    }
  }
  const parts = [...rows.entries()].map(([id, count]) => ({ id, label: BASIS_LABEL[id] || String(id).replace(/_/g, ' ').replace(/^./, char => char.toUpperCase()), rows: count })).filter(part => part.rows > 0).sort((left, right) => right.rows - left.rows)
  return { total: sum(parts, part => part.rows), parts }
}

// Time from upload to ready for every finished job each case lists, in seven bins (the edges are fixed so two reads of the workspace can be compared); the bin that holds the median is marked.
const BIN_EDGES = [0, 5, 15, 30, 60, 120, 300, Infinity]
const BIN_LABELS = ['under 5 s', '5 to 15 s', '15 to 30 s', '30 to 60 s', '1 to 2 min', '2 to 5 min', '5 min or more']
export function durationHistogram(rows) {
  const stamp = value => { const time = value ? Date.parse(value) : NaN; return Number.isFinite(time) ? time : null }
  const seconds = []
  for (const row of rows || []) {
    for (const job of row.state?.data?.recent_jobs || []) {
      if (String(job.status || '').toLowerCase() !== 'completed') continue
      const from = stamp(job.queued_at); const to = stamp(job.completed_at)
      if (from !== null && to !== null && to >= from) seconds.push((to - from) / 1000)
    }
  }
  seconds.sort((left, right) => left - right)
  const median = quantile(seconds, 0.5)
  const bins = BIN_LABELS.map((label, index) => ({ key: label, label, count: seconds.filter(value => value >= BIN_EDGES[index] && value < BIN_EDGES[index + 1]).length, mark: median !== null && median >= BIN_EDGES[index] && median < BIN_EDGES[index + 1] }))
  return { bins, count: seconds.length, median }
}
