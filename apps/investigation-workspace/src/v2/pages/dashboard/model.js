import { curatedFamilyLabel } from '../../../lib/semanticCatalog.js'

// What the dashboard draws, as pure functions of what the services reported. Nothing here fetches. A number is a count a service gave, a share of two such counts,
// or a median of dated jobs; a case that could not be read is left out and counted as unread, never as zero.

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
      total: sum(past, day => day.total),
      first: past[0]?.date || null,
      last: past.at(-1)?.date || null,
      partial: Boolean(result.truncated),
      futureTotal: sum(future, day => day.total),
      futureLast: future.at(-1)?.date || null,
    }
  }).filter(item => item.series.length || item.futureTotal)
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
  const named = new Map(MODALITIES.map(kind => [kind.id, { ...kind, files: 0, failed: 0, caseCount: 0 }]))
  const other = { id: 'other', label: 'Other files', slot: 0, files: 0, failed: 0, caseCount: 0 }
  for (const report of read) {
    const seen = new Set()
    for (const entry of report.evidence || []) {
      const target = named.get(entry.modality) || other
      target.files += number(entry.files)
      target.failed += number(entry.failed)
      if (!seen.has(target.id) && number(entry.files) > 0) { target.caseCount += 1; seen.add(target.id) }
    }
  }
  const kinds = [...named.values(), other].filter(kind => kind.files > 0).sort((left, right) => right.files - left.files || left.slot - right.slot)
  return { reported: read.length, total: sum(kinds, kind => kind.files), kinds }
}

// Calls, internet sessions and app usage are the typed records of the typing report, with exact repeats left out as the report does; what is left of the accepted rows is "Other records".
export function recordKinds(reports, rows) {
  const byCase = new Map((reports || []).filter(report => report && report.available !== false).map(report => [report.collection_id, report]))
  const kinds = [{ id: 'calls', label: 'Calls', count: 0 }, { id: 'sessions', label: 'Internet sessions', count: 0 }, { id: 'usage', label: 'App usage', count: 0 }, { id: 'other', label: 'Other records', count: 0 }]
  const effective = part => Math.max(0, number(part?.effective ?? number(part?.typed) - number(part?.repeats)))
  for (const row of rows || []) {
    const report = byCase.get(row.caseId)
    if (!report || !row.summary) continue
    const calls = effective(report.calls)
    const sessions = effective(report.sessions)
    const usage = effective(report.usage)
    kinds[0].count += calls
    kinds[1].count += sessions
    kinds[2].count += usage
    kinds[3].count += Math.max(0, row.summary.acceptedRows - calls - sessions - usage)
  }
  return { reported: byCase.size, total: sum(kinds, kind => kind.count), kinds: kinds.filter(kind => kind.count > 0) }
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
  for (const row of rows || []) {
    const seconds = (row.state?.data?.recent_jobs || [])
      .filter(job => String(job.status || '').toLowerCase() === 'completed')
      .map(job => { const from = stamp(job.queued_at); const to = stamp(job.completed_at); return from !== null && to !== null && to >= from ? (to - from) / 1000 : null })
      .filter(value => value !== null)
      .sort((left, right) => left - right)
    if (!seconds.length) continue
    all.push(...seconds)
    cases.push({ caseId: row.caseId, count: seconds.length, min: seconds[0], p25: quantile(seconds, 0.25), median: quantile(seconds, 0.5), p75: quantile(seconds, 0.75), max: seconds.at(-1) })
  }
  all.sort((left, right) => left - right)
  return { cases, count: all.length, median: quantile(all, 0.5), slowest: all.at(-1) ?? null }
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

// The cases that hold the most accepted rows, largest first: where the volume is.
export function largestCases(rows, limit = 4) {
  return (rows || []).filter(row => row.summary && row.summary.acceptedRows > 0).map(row => ({ caseId: row.caseId, records: row.summary.acceptedRows }))
    .sort((left, right) => right.records - left.records).slice(0, limit)
}

// A moment as the case's own clock says it, with the zone: "5 Mar 2026, 14:32 PKT" (directive 10.4). Relative times carry this as their title.
const PKT = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false, timeZone: 'Asia/Karachi' })
export const formatMoment = date => (date && !Number.isNaN(date.getTime()) ? `${PKT.format(date)} PKT` : '')
