import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { CircleAlert } from 'lucide-react'
import { Card } from '../../components/Card.jsx'
import { ChartCard } from '../../components/charts/ChartCard.jsx'
import { SelectControl } from '../../components/SelectControl.jsx'
import { SkeletonRows } from '../../components/Skeleton.jsx'
import { activityHighlights, activityOption, activityRows, bucketActivity, bucketQuestion } from '../../lib/caseActivity.js'
import { formatNumber } from '../../lib/format.js'
import { curatedFamilyLabel } from '../../lib/semanticCatalog.js'
import { activityFailureText } from '../../lib/useCaseActivity.js'

export function dayQuestion(date) {
  return `Show activity on ${date}`
}

// Where a click on a day or a week goes: the case that holds most of it, straight into Investigate with the period as
// the question. All values come from the reported activity; nothing is inferred.
export function dayTarget(activity, date, scope) {
  const day = activity.days.find(entry => entry.date === date)
  const caseId = scope || day?.topCase
  return day && caseId ? `/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(bucketQuestion(day))}` : null
}

const RANGES = [{ id: 'all', label: 'All' }, { id: '30', label: '30 days' }, { id: '7', label: '7 days' }]
const WEEKLY_FROM_DAYS = 21

function Segmented({ label, value, options, onChange }) {
  return (
    <div className="seg" role="group" aria-label={label}>
      {options.map(option => <button key={option.id} type="button" aria-pressed={value === option.id} onClick={() => onChange(option.id)}>{option.label}</button>)}
    </div>
  )
}

// D-hero. When did activity happen, and in what? Stacked bars per day by record family, real timestamps only. The
// legend toggles families, the slider zooms, the tooltip has exact counts, and a click opens that day in Investigate.
// A capped read is labelled partial; a case that could not be read is named. Rules: UI_REDESIGN_BRIEF §6 and §11.
export function ActivityChart({ activity, status, failures = [], order, cases, scope, onScope, onRetry }) {
  const navigate = useNavigate()
  const [range, setRange] = useState('all')
  const [grain, setGrain] = useState('day')
  const failed = failures.map(failure => failure.caseId)
  const canWeek = activity.available && activity.days.length >= WEEKLY_FROM_DAYS
  const view = useMemo(() => (canWeek && grain === 'week' ? bucketActivity(activity, 'week') : activity), [activity, canWeek, grain])
  const chart = useMemo(() => ({
    buildOption: theme => activityOption(view, theme, order, { range }),
    height: 300,
    fill: true,
    label: `Activity per ${view.bucket === 'week' ? 'week' : 'day'} by record family, ${activity.first} to ${activity.last}. ${activity.families.map(family => `${curatedFamilyLabel(family.id)}: ${formatNumber(family.total)}`).join('. ')}`,
    onSelect: params => {
      const target = params?.name ? dayTarget(view, params.name, scope) : null
      if (target) navigate(target)
    },
  }), [activity, view, range, order, scope, navigate])

  const scopeControl = cases.length > 1 ? <SelectControl label="Scope" value={scope || 'all'} options={cases} onChange={value => onScope(value === 'all' ? '' : value)} optionLabel={item => item} /> : null

  if (status === 'loading' && !activity.available) {
    return <div className="dash-slot"><Card className="activity-card" title="When did activity happen?" actions={scopeControl}><SkeletonRows rows={4} label="Reading activity" /></Card></div>
  }
  if (!activity.available) {
    return (
      <div className="dash-slot">
        <Card className="activity-card" title="When did activity happen?" actions={scopeControl}>
          <p className="dash-card__empty">
            <CircleAlert aria-hidden="true" />
            <span>
              {failed.length ? 'Activity could not be read for this scope.' : 'No dated records to chart.'}
              {failures[0] ? <small className="activity-card__reason">{activityFailureText(failures[0].reason)}</small> : null}
            </span>
            {failed.length ? <button type="button" className="dash-link-button" onClick={onRetry}>Try again</button> : null}
          </p>
        </Card>
      </div>
    )
  }
  if (!activity.total) {
    return <div className="dash-slot"><Card className="activity-card" title="When did activity happen?" actions={scopeControl}><p className="dash-card__empty">No dated records have been ingested yet.</p></Card></div>
  }

  const highlights = activityHighlights(activity)
  const busiestLink = highlights ? dayTarget(activity, highlights.busiest.date, scope) : null
  const columns = [
    { key: 'date', label: 'Day' },
    { key: 'total', label: 'Total', numeric: true, render: row => formatNumber(row.total) },
    ...activity.families.map(family => ({ key: family.id, label: curatedFamilyLabel(family.id), numeric: true, render: row => formatNumber(row[family.id]) })),
  ]
  return (
    <div className="dash-slot">
      <ChartCard
        className="activity-card"
        title="When did activity happen?"
        actions={scopeControl}
        toolbar={(
          <>
            <Segmented label="Time range" value={range} options={RANGES} onChange={setRange} />
            {canWeek ? <Segmented label="Group by" value={grain} options={[{ id: 'day', label: 'Days' }, { id: 'week', label: 'Weeks' }]} onChange={setGrain} /> : null}
          </>
        )}
        chart={chart}
        columns={columns}
        rows={activityRows(view)}
        footer={(
          <>
            {highlights ? (
              <ul className="activity-highlights" aria-label="Highlights">
                <li>
                  <span>Busiest day</span>
                  <b>{busiestLink ? <Link to={busiestLink}>{highlights.busiest.date}</Link> : highlights.busiest.date}</b>
                  <small>{formatNumber(highlights.busiest.total)} events</small>
                </li>
                <li>
                  <span>Most active</span>
                  <b>{curatedFamilyLabel(highlights.topFamily.id)}</b>
                  <small>{highlights.topFamily.share}% of events</small>
                </li>
                <li>
                  <span>Typical day</span>
                  <b>{formatNumber(highlights.typical)}</b>
                  <small>events, {formatNumber(highlights.activeDays)} active {highlights.activeDays === 1 ? 'day' : 'days'}</small>
                </li>
              </ul>
            ) : null}
            <span>{formatNumber(activity.total)} events · {activity.first} to {activity.last}{activity.cases.length > 1 ? ` · ${formatNumber(activity.cases.length)} cases` : ''}</span>
            {activity.truncated ? <span className="activity-card__note">Partial: the service returns at most 100 day-and-family groups, oldest first.</span> : null}
            {failed.length ? <span className="activity-card__note">Not read: {failed.join(', ')} ({activityFailureText(failures[0].reason).replace(/\.$/, '').toLowerCase()}). <button type="button" className="dash-link-button" onClick={onRetry}>Try again</button></span> : null}
          </>
        )}
      />
    </div>
  )
}
