import { familyOrder, rangeStartIndex } from './caseActivity.js'

// The case chronology as the service reports it: one entry per calendar day with the events of each record family on that
// day. Everything here is a pure function of those counts. A family filter narrows the days to the chosen families (a day
// with none of them drops out), so totals always add up to what is listed.

const MONTH = new Intl.DateTimeFormat('en-GB', { month: 'long', year: 'numeric', timeZone: 'UTC' })
const WEEKDAY = new Intl.DateTimeFormat('en-GB', { weekday: 'short', timeZone: 'UTC' })

export function dayParts(date) {
  const value = new Date(`${date}T00:00:00Z`)
  return { day: String(value.getUTCDate()), weekday: WEEKDAY.format(value), month: MONTH.format(value), monthKey: date.slice(0, 7) }
}

// Days in the range and family filter, with the filtered total, newest or oldest first.
export function visibleDays(activity, { range = 'all', families = [], newestFirst = true } = {}) {
  if (!activity?.days?.length) return []
  const start = rangeStartIndex(activity, range)
  const chosen = new Set(families)
  const days = activity.days.slice(start).map(day => {
    if (!chosen.size) return { ...day, shown: day.total, byShown: day.byFamily }
    const byShown = Object.fromEntries(Object.entries(day.byFamily).filter(([id]) => chosen.has(id)))
    return { ...day, shown: Object.values(byShown).reduce((sum, value) => sum + value, 0), byShown }
  }).filter(day => day.shown > 0)
  return newestFirst ? days.reverse() : days
}

// Days grouped under their month, each month with its own totals, in the order the days were given.
export function groupByMonth(days) {
  const groups = []
  for (const day of days) {
    const parts = dayParts(day.date)
    let group = groups[groups.length - 1]
    if (!group || group.key !== parts.monthKey) {
      group = { key: parts.monthKey, label: parts.month, days: [], total: 0 }
      groups.push(group)
    }
    group.days.push(day)
    group.total += day.shown
  }
  return groups
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
