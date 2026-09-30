import { formatNumber } from './format.js'
import { mondayOf, shift } from './caseActivity.js'

// What the case overview says about one case, as pure functions of its summary counts and its real activity. The overview
// is a briefing, not a portfolio: one verdict, what to do next, what the case holds and when things happened.

const plural = (count, one, many) => (count === 1 ? one : many)

// One sentence: can the analyst rely on this case, and if not, why not. Failed and missing copies come first because they
// change what an answer can be trusted for; processing only limits coverage until it finishes.
export function caseVerdict(summary) {
  if (!summary) return { tone: 'quiet', headline: 'Checking this case…', detail: '' }
  const review = (summary.failed || 0) + summary.missingAssets
  if (!summary.total) return { tone: 'quiet', headline: 'No evidence yet', detail: 'Add the first source file to start building this case.' }
  if (review > 0) return { tone: 'failed', headline: `${formatNumber(review)} ${plural(review, 'source needs', 'sources need')} review`, detail: `Answers cover the ${formatNumber(summary.ready)} ready ${plural(summary.ready, 'source', 'sources')}. Review the rest before relying on complete coverage.` }
  if (summary.inFlight > 0) return { tone: 'processing', headline: `${formatNumber(summary.inFlight)} ${plural(summary.inFlight, 'source is', 'sources are')} still processing`, detail: `Answers cover the ${formatNumber(summary.ready)} ready ${plural(summary.ready, 'source', 'sources')} until processing finishes.` }
  return { tone: 'ready', headline: 'Ready to investigate', detail: `All ${formatNumber(summary.total)} ${plural(summary.total, 'source is', 'sources are')} searchable.` }
}

// Up to four next steps, most urgent first, each a real link. The first is the one to do now; asking a question is always
// there once something is ready, so the page never ends without a way forward.
export function nextSteps(summary, caseId) {
  if (!summary) return []
  const base = `/cases/${encodeURIComponent(caseId)}`
  const steps = []
  if (!summary.total) return [{ id: 'add', tone: 'quiet', title: 'Add the first evidence file', detail: 'Nothing can be asked until a source is ingested.', cta: 'Add evidence', to: `${base}/evidence#add-evidence` }]
  if (summary.failed > 0) steps.push({ id: 'failed', tone: 'failed', title: `Review ${formatNumber(summary.failed)} failed ${plural(summary.failed, 'source', 'sources')}`, detail: 'They are not searched until they are fixed.', cta: 'Open failed sources', to: `${base}/evidence?status=failed` })
  if (summary.missingAssets > 0) steps.push({ id: 'missing', tone: 'failed', title: `${formatNumber(summary.missingAssets)} ${plural(summary.missingAssets, 'source is', 'sources are')} missing a retained copy`, detail: 'The record exists but the original file cannot be opened.', cta: 'Open evidence', to: `${base}/evidence` })
  if (summary.inFlight > 0) steps.push({ id: 'processing', tone: 'processing', title: `${formatNumber(summary.inFlight)} ${plural(summary.inFlight, 'source is', 'sources are')} processing`, detail: 'Answers will include them once they finish.', cta: 'View progress', to: `${base}/evidence` })
  if (summary.ready > 0) steps.push({ id: 'ask', tone: 'ready', title: 'Ask a question', detail: `${formatNumber(summary.ready)} ready ${plural(summary.ready, 'source', 'sources')} can be searched now.`, cta: 'Investigate', to: `${base}/investigate` })
  return steps.slice(0, 4)
}

// Ring segments for readiness: ready, processing, failed, and everything else, as floored percentages of the sources so a
// part is never drawn as the whole. The centre figure is ready over total, floored.
export function readinessRing(summary) {
  const total = summary?.total || 0
  const failed = summary?.failed || 0
  const parts = [
    { id: 'ready', label: 'Ready', value: summary?.ready || 0 },
    { id: 'processing', label: 'Processing', value: summary?.inFlight || 0 },
    { id: 'failed', label: 'Failed', value: failed },
  ]
  const other = Math.max(0, total - parts.reduce((sum, part) => sum + part.value, 0))
  if (other) parts.push({ id: 'other', label: 'Not counted yet', value: other })
  return { total, parts: parts.filter(part => part.value > 0), percent: total ? Math.floor(((summary.ready || 0) / total) * 100) : null }
}

// How many weeks the calendar shows: the span of the activity, at least 12 so it reads as a calendar and at most 26 so the
// squares stay legible. A short case does not get half a year of empty squares.
export function calendarWeeks(activity) {
  if (!activity?.first || !activity?.last) return 12
  const days = Math.round((Date.parse(`${activity.last}T00:00:00Z`) - Date.parse(`${activity.first}T00:00:00Z`)) / 86_400_000)
  return Math.min(26, Math.max(12, Math.ceil(days / 7) + 2))
}

// A calendar of activity: Monday-first weeks ending on the last day with activity, each day a level 0 to 4 of the busiest
// day. Levels are relative to this case's own busiest day and are stated as such; days after the last are left empty.
export function activityCalendar(activity, weeks = 26) {
  if (!activity?.available || !activity.last || !activity.days.length) return null
  const byDate = new Map(activity.days.map(day => [day.date, day]))
  const lastMonday = mondayOf(activity.last)
  const start = shift(lastMonday, -(weeks - 1) * 7)
  const max = Math.max(...activity.days.filter(day => day.date >= start).map(day => day.total), 1)
  const columns = []
  for (let week = 0; week < weeks; week += 1) {
    const column = []
    for (let dow = 0; dow < 7; dow += 1) {
      const date = shift(start, week * 7 + dow)
      const day = byDate.get(date)
      column.push(date > activity.last ? null : { date, total: day?.total || 0, level: day ? Math.max(1, Math.ceil((day.total / max) * 4)) : 0 })
    }
    columns.push(column)
  }
  const shown = activity.days.filter(day => day.date >= start)
  return { columns, max, start, last: activity.last, total: shown.reduce((sum, day) => sum + day.total, 0), activeDays: shown.length, earlier: activity.first < start ? activity.first : null }
}
