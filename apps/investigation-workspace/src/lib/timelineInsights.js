import { shift } from './caseActivity.js'
import { formatNumber } from './format.js'
import { curatedFamilyLabel } from './semanticCatalog.js'
import { typicalDay } from './timelineModel.js'

// What stands out in the case's days, each with the reasoning in plain words so an analyst can check it against the chart.
// Only three things are ever claimed, all arithmetic on the reported counts: days far above the typical day, the longest
// stretch with no dated records, and a record family that makes up most of the events. Nothing is predicted or inferred.
const SPIKE_TIMES = 3
const GAP_DAYS = 7
const DOMINANT_SHARE = 0.5

const days = (from, to) => Math.round((Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / 86_400_000)

export function timelineInsights(activity) {
  if (!activity?.available || !activity.days?.length || !activity.total) return []
  const typical = typicalDay(activity)
  const found = []

  const spikes = typical ? activity.days.filter(day => day.total >= typical * SPIKE_TIMES).sort((left, right) => right.total - left.total) : []
  if (spikes.length) {
    const top = spikes[0]
    found.push({
      id: 'spike',
      tone: 'alert',
      title: spikes.length === 1 ? 'One day far above the rest' : `${formatNumber(spikes.length)} days far above the rest`,
      detail: `${top.date} has ${formatNumber(top.total)} events, ${Math.round((top.total / typical) * 10) / 10}× the typical day (${formatNumber(typical)}).`,
      date: top.date,
    })
  }

  let gap = null
  for (let index = 1; index < activity.days.length; index += 1) {
    const length = days(activity.days[index - 1].date, activity.days[index].date) - 1
    if (length >= GAP_DAYS && (!gap || length > gap.length)) gap = { length, from: shift(activity.days[index - 1].date, 1), to: shift(activity.days[index].date, -1), after: activity.days[index].date }
  }
  if (gap) {
    found.push({ id: 'gap', tone: 'quiet', title: 'A quiet stretch', detail: `No dated records for ${formatNumber(gap.length)} days, ${gap.from} to ${gap.to}.`, date: gap.after })
  }

  const top = activity.families[0]
  if (top && activity.families.length > 1 && top.total / activity.total > DOMINANT_SHARE) {
    found.push({ id: 'mix', tone: 'info', title: `${curatedFamilyLabel(top.id)} lead`, detail: `${Math.floor((top.total / activity.total) * 100)}% of all events (${formatNumber(top.total)} of ${formatNumber(activity.total)}).` })
  }
  return found
}

// The events inside a visible window of the chart (inclusive stamps in ms), in total and by family.
export function windowSummary(activity, from, to) {
  const inside = activity.days.filter(day => {
    const start = Date.parse(`${day.date}T00:00:00Z`)
    const end = Date.parse(`${day.end || day.date}T00:00:00Z`)
    return end >= from && start <= to
  })
  const byFamily = {}
  let total = 0
  for (const day of inside) {
    total += day.total
    for (const [id, count] of Object.entries(day.byFamily)) byFamily[id] = (byFamily[id] || 0) + count
  }
  return { total, days: inside.length, first: inside[0]?.date || '', last: inside[inside.length - 1]?.date || '', byFamily }
}
