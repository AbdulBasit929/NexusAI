import { useNavigate } from 'react-router-dom'
import { activityCalendar, calendarWeeks } from '../../lib/caseDossier.js'
import { describeBucket, shortDate } from '../../lib/caseActivity.js'
import { formatNumber } from '../../lib/format.js'
import { activityFailureText } from '../../lib/useCaseActivity.js'

const CELL = 13
const STEP = 17
const LEFT = 26
const TOP = 16
const DAYS = ['Mon', '', 'Wed', '', 'Fri', '', '']
const MONTH = new Intl.DateTimeFormat('en-GB', { month: 'short', timeZone: 'UTC' })

// When did things happen, as a calendar: one square per day, darker for busier, weeks left to right. The scale is the case's
// own busiest day (said under the calendar), the data is the case's real per-day counts, and a day with nothing is empty,
// not zero-shaded. A click on a day opens Investigate on it; the table beneath is the exact, keyboard-reachable version.
export function ActivityCalendar({ caseId, activity, status, failures = [], onRetry }) {
  const navigate = useNavigate()
  const calendar = activityCalendar(activity, calendarWeeks(activity))
  if (status === 'loading' && !activity.available) return <p className="dash-card__empty" role="status">Reading activity…</p>
  if (!activity.available) {
    return <p className="dash-card__empty">Activity could not be read for this case.{failures[0] ? <small className="activity-card__reason">{activityFailureText(failures[0].reason)}</small> : null} <button type="button" className="dash-link-button" onClick={onRetry}>Try again</button></p>
  }
  if (!calendar) return <p className="dash-card__empty">No dated records have been ingested yet.</p>
  const width = LEFT + calendar.columns.length * STEP
  const height = TOP + 7 * STEP
  const months = calendar.columns.map((column, index) => {
    const first = column.find(Boolean)
    const previous = calendar.columns[index - 1]?.find(Boolean)
    return first && (!previous || MONTH.format(new Date(`${first.date}T00:00:00Z`)) !== MONTH.format(new Date(`${previous.date}T00:00:00Z`))) ? { index, label: MONTH.format(new Date(`${first.date}T00:00:00Z`)) } : null
  }).filter(Boolean)
  const days = activity.days.filter(day => day.date >= calendar.start)
  const busiest = days.reduce((best, day) => (day.total > best.total ? day : best), days[0])
  return (
    <>
      <svg className="co-cal" style={{ maxInlineSize: width * 1.7 }} viewBox={`0 0 ${width} ${height}`} role="img" aria-label={`Activity by day, ${shortDate(calendar.start)} to ${shortDate(calendar.last)}: ${formatNumber(calendar.total)} events on ${formatNumber(calendar.activeDays)} days. Busiest day ${busiest.date}, ${formatNumber(busiest.total)} events.`}>
        {DAYS.map((label, row) => label ? <text key={row} className="co-cal__label" x="0" y={TOP + row * STEP + CELL - 2}>{label}</text> : null)}
        {months.map(month => <text key={month.index} className="co-cal__label" x={LEFT + month.index * STEP} y="10">{month.label}</text>)}
        {calendar.columns.map((column, week) => column.map((cell, row) => cell ? (
          <rect
            key={cell.date}
            className={`co-cal__cell co-cal__cell--${cell.level}${cell.level ? ' is-active' : ''}`}
            x={LEFT + week * STEP}
            y={TOP + row * STEP}
            width={CELL}
            height={CELL}
            rx="3"
            onClick={cell.level ? () => navigate(`/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(`Show activity on ${cell.date}`)}`) : undefined}
          ><title>{cell.total ? `${describeBucket(cell)}: ${formatNumber(cell.total)} events` : `${describeBucket(cell)}: no activity`}</title></rect>
        ) : null))}
      </svg>
      <div className="co-cal__foot">
        <span>{formatNumber(calendar.total)} events on {formatNumber(calendar.activeDays)} active {calendar.activeDays === 1 ? 'day' : 'days'}. Busiest {describeBucket(busiest)} ({formatNumber(busiest.total)}).</span>
        <span className="co-cal__scale" aria-label={`Darker means busier, up to ${formatNumber(calendar.max)} events in a day`}>Less{[0, 1, 2, 3, 4].map(level => <i key={level} className={`co-cal__cell--${level}`} aria-hidden="true" />)}More</span>
      </div>
      {calendar.earlier ? <p className="co-cal__note">Showing the last {calendar.columns.length} weeks; activity goes back to {describeBucket({ date: calendar.earlier })}.</p> : null}
      {activity.truncated ? <p className="co-cal__note">Partial: the service returns at most 100 day-and-family groups, oldest first.</p> : null}
      <details className="co-cal__table">
        <summary>Exact values by day</summary>
        <table>
          <thead><tr><th scope="col">Day</th><th scope="col" className="is-numeric">Events</th></tr></thead>
          <tbody>{[...days].reverse().map(day => <tr key={day.date}><th scope="row">{describeBucket(day)}</th><td className="is-numeric">{formatNumber(day.total)}</td></tr>)}</tbody>
        </table>
      </details>
    </>
  )
}
