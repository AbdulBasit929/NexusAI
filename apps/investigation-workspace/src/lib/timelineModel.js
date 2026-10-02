import { familyOrder } from './caseActivity.js'

// Small pure helpers over the per-day record counts the service reports.

const WEEKDAY = new Intl.DateTimeFormat('en-GB', { weekday: 'short', timeZone: 'UTC' })

export function dayParts(date) {
  const value = new Date(`${date}T00:00:00Z`)
  return { day: String(value.getUTCDate()), weekday: WEEKDAY.format(value) }
}

// How a day compares with the typical day (the mean over days with activity), in plain words, or nothing when it is typical.
export function againstTypical(total, typical) {
  if (!typical || !total) return ''
  const ratio = total / typical
  if (ratio >= 2) return `${ratio >= 10 ? Math.round(ratio) : (Math.round(ratio * 10) / 10)}× a typical day`
  if (ratio <= 0.5) return 'Quieter than a typical day'
  return 'About a typical day'
}

export function typicalDay(activity) {
  return activity?.days?.length ? Math.round(activity.days.reduce((sum, day) => sum + day.total, 0) / activity.days.length) : 0
}

// A day's families largest first, each with its share of the day's events (floored, never rounded up to a false 100).
export function dayBreakdown(day, order) {
  const entries = Object.entries(day.byFamily).filter(([, count]) => count > 0).sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]))
  return entries.map(([id, count]) => ({ id, count, share: Math.floor((count / day.total) * 100), index: Math.max(0, order.indexOf(id)) }))
}

// The record scope Investigate understands for a family id, or nothing when the id is not a known scope.
const SCOPES = { cdr: 'cdr', ipdr: 'ipdr', anpr: 'anpr', subscriber: 'subscriber', tower_location: 'tower', access_log: 'access_log', transaction: 'financial', financial: 'financial', document: 'document', image: 'image', audio: 'audio', video: 'video' }
export function familyScope(id) {
  return SCOPES[String(id).toLowerCase()] || ''
}

export { familyOrder }

// The busiest buckets, largest first, for one-click jumps.
export function peakDays(activity, count = 5) {
  return [...activity.days].sort((left, right) => right.total - left.total || left.date.localeCompare(right.date)).slice(0, count)
}

// The activity limited to the chosen record families: days with none of them drop out, and every total is recomputed from
// what remains, so everything drawn from the result adds up. With no families chosen the activity is returned as it is.
export function filterActivity(activity, families) {
  if (!activity?.available || !families?.length) return activity
  const chosen = new Set(families)
  const days = activity.days.map(day => {
    const byFamily = Object.fromEntries(Object.entries(day.byFamily).filter(([id]) => chosen.has(id)))
    return { ...day, byFamily, total: Object.values(byFamily).reduce((sum, value) => sum + value, 0) }
  }).filter(day => day.total > 0)
  const kept = activity.families.filter(family => chosen.has(family.id))
  return { ...activity, days, families: kept, total: days.reduce((sum, day) => sum + day.total, 0), first: days[0]?.date || '', last: days[days.length - 1]?.date || '' }
}

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

// Events by day of the week (Monday first, UTC), with each weekday's share of the whole, floored. Real counts, read from the
// days the service reported, so it shows the rhythm of the records without any inference.
export function weekdayRhythm(activity) {
  const totals = WEEKDAYS.map(label => ({ label, total: 0, days: 0 }))
  for (const day of activity?.days || []) {
    const slot = totals[(new Date(`${day.date}T00:00:00Z`).getUTCDay() + 6) % 7]
    slot.total += day.total
    slot.days += 1
  }
  const sum = totals.reduce((acc, slot) => acc + slot.total, 0)
  return totals.map(slot => ({ ...slot, share: sum ? Math.floor((slot.total / sum) * 100) : 0 }))
}

// A short plain-text briefing of a stretch of days, for pasting into a note or message.
export function briefing({ days, families, caseId }) {
  if (!days.length) return ''
  const total = days.reduce((sum, day) => sum + day.total, 0)
  const byFamily = {}
  for (const day of days) for (const [id, count] of Object.entries(day.byFamily)) byFamily[id] = (byFamily[id] || 0) + count
  const peak = days.reduce((best, day) => (day.total > best.total ? day : best), days[0])
  const lines = [
    `Timeline briefing${caseId ? ` for ${caseId}` : ''}`,
    `${days[0].date} to ${days[days.length - 1].end || days[days.length - 1].date}: ${total.toLocaleString('en-GB')} events over ${days.length} ${days.length === 1 ? 'period' : 'periods'}.`,
    ...Object.entries(byFamily).sort((left, right) => right[1] - left[1]).map(([id, count]) => `- ${families(id)}: ${count.toLocaleString('en-GB')} (${Math.floor((count / total) * 100)}%)`),
    `Busiest: ${peak.date} with ${peak.total.toLocaleString('en-GB')} events.`,
    'Counts are per calendar day (UTC) from each record\u2019s own date.',
  ]
  return lines.join('\n')
}

// The days as CSV, one column per family, quoted where needed.
export function daysCsv(days, familyIds, label) {
  const quote = value => (/[",\n]/.test(String(value)) ? `"${String(value).replace(/"/g, '""')}"` : String(value))
  const head = ['Date', 'Total', ...familyIds.map(label)].map(quote).join(',')
  const rows = days.map(day => [day.date, day.total, ...familyIds.map(id => day.byFamily[id] || 0)].map(quote).join(','))
  return [head, ...rows].join('\n')
}
