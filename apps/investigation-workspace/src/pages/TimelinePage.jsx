import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ChevronLeft, ChevronRight, CircleAlert, MessageSquareText, MoonStar, PieChart, TrendingUp } from 'lucide-react'
import { CaseShell } from '../components/CaseShell.jsx'
import { SkeletonRows } from '../components/Skeleton.jsx'
import { getQueryCapabilities } from '../lib/apiClient.js'
import { activityHighlights, bucketActivity, bucketQuestion, describeBucket, familyColour } from '../lib/caseActivity.js'
import { formatNumber } from '../lib/format.js'
import { curatedFamilyLabel } from '../lib/semanticCatalog.js'
import { dateOfStamp, laneHeight, laneOrder, timelineOption } from '../lib/timelineChart.js'
import { timelineInsights, windowSummary } from '../lib/timelineInsights.js'
import { againstTypical, dayBreakdown, familyOrder, familyScope, peakDays, typicalDay } from '../lib/timelineModel.js'
import { activityFailureText, useCaseActivity } from '../lib/useCaseActivity.js'

// Code-split with the other charts: ECharts is fetched only when the timeline is shown.
const EChart = lazy(() => import('../components/charts/EChart.jsx'))

const familyLabels = {
  cdr: 'CDR', ipdr: 'IPDR', anpr: 'ANPR', subscriber: 'Subscriber', tower_location: 'Tower',
  transaction: 'Financial', access_log: 'Access log', document: 'Documents', image: 'Images', audio: 'Audio', video: 'Video',
}

const scopeByCapability = {
  cdr: 'cdr', ipdr: 'ipdr', anpr: 'anpr', subscriber_identity: 'subscriber', subscriber: 'subscriber',
  tower_location: 'tower', tower: 'tower', logs_access_security: 'access_log', access_log: 'access_log',
  financial: 'financial', transaction: 'financial', document: 'document', images: 'image', image: 'image',
  audio_and_stt: 'audio', audio: 'audio', video: 'video',
}

function safeFamilyLabel(value) {
  if (!value) return 'Not reported'
  return familyLabels[String(value).toLowerCase()] || String(value)
}

function scopeFor(family) {
  const recordType = Array.isArray(family?.record_types) ? family.record_types[0] : ''
  return scopeByCapability[family?.id] || scopeByCapability[recordType] || 'all'
}

export function timelineQuestions(payload) {
  const seen = new Set()
  return (Array.isArray(payload?.families) ? payload.families : [])
    .filter(family => ['queryable', 'semantic_only', 'limited'].includes(family?.availability))
    .flatMap(family => (Array.isArray(family?.suggested_queries) ? family.suggested_queries : []).map(question => ({
      question: String(question),
      family: family.label || safeFamilyLabel(family.id),
      scope: scopeFor(family),
    })))
    .filter(item => /timeline|history|sequence|by hour|over time|time order/i.test(item.question))
    .filter(item => {
      const key = `${item.scope}:${item.question.toLocaleLowerCase()}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
    .slice(0, 6)
}

const RANGES = [{ id: 'all', label: 'All' }, { id: '90', label: '90 days' }, { id: '30', label: '30 days' }, { id: '7', label: '7 days' }]
const WEEKLY_FROM_DAYS = 150

function investigateLink(caseId, bucket, family = null) {
  const query = new URLSearchParams({ question: family ? `Show ${curatedFamilyLabel(family)} activity on ${bucket.date}${bucket.end && bucket.end !== bucket.date ? ` to ${bucket.end}` : ''}` : bucketQuestion(bucket) })
  const scope = family ? familyScope(family) : ''
  if (scope) query.set('scope', scope)
  return `/cases/${encodeURIComponent(caseId)}/investigate?${query}`
}

const INSIGHT_ICON = { spike: TrendingUp, gap: MoonStar, mix: PieChart }
const dot = colour => ({ '--fam': colour })

function SelectedDay({ caseId, bucket, typical, order, palette, position, count, onStep, questions }) {
  const rows = dayBreakdown(bucket, order)
  const week = bucket.end && bucket.end !== bucket.date
  const compare = week ? '' : againstTypical(bucket.total, typical)
  return (
    <section className="tl-read" aria-label={week ? 'Selected week' : 'Selected day'}>
      <div className="tl-read__when">
        <p className="tl-read__kicker">{week ? 'Selected week' : 'Selected day'}</p>
        <h2>{describeBucket(bucket)}</h2>
        <p className="tl-read__total"><b>{formatNumber(bucket.total)}</b> {bucket.total === 1 ? 'event' : 'events'}</p>
        {compare ? <p className="tl-read__compare">{compare}</p> : null}
        <div className="tl-read__step" role="group" aria-label={`Move between ${week ? 'weeks' : 'days'}`}>
          <button type="button" onClick={() => onStep(-1)} disabled={position <= 0}><ChevronLeft aria-hidden="true" />Earlier</button>
          <button type="button" onClick={() => onStep(1)} disabled={position >= count - 1}>Later<ChevronRight aria-hidden="true" /></button>
        </div>
      </div>
      <div className="tl-read__mix">
        <p className="tl-read__kicker">What happened</p>
        <div className="tl-stack" role="img" aria-label={rows.map(row => `${curatedFamilyLabel(row.id)} ${formatNumber(row.count)}`).join(', ')}>
          {rows.map(row => <i key={row.id} style={{ ...dot(familyColour(row.id, order, palette)), flexGrow: row.count }} />)}
        </div>
        <ul>
          {rows.map(row => (
            <li key={row.id} style={dot(familyColour(row.id, order, palette))}>
              <i aria-hidden="true" /><span>{curatedFamilyLabel(row.id)}</span><b>{formatNumber(row.count)}</b><small>{row.share < 1 ? '<1%' : `${row.share}%`}</small>
              <Link to={investigateLink(caseId, bucket, row.id)} aria-label={`Open ${curatedFamilyLabel(row.id)} events in Investigate`}>Open</Link>
            </li>
          ))}
        </ul>
      </div>
      <div className="tl-read__next">
        <p className="tl-read__kicker">Go further</p>
        <Link className="tl-btn tl-btn--primary" to={investigateLink(caseId, bucket)}><MessageSquareText aria-hidden="true" />Investigate {week ? 'this week' : 'this day'}</Link>
        {questions.length ? <ul>{questions.slice(0, 3).map(item => <li key={`${item.scope}-${item.question}`}><Link to={`/cases/${encodeURIComponent(caseId)}/investigate?${new URLSearchParams({ question: item.question, ...(item.scope !== 'all' ? { scope: item.scope } : {}) })}`}>{item.question}</Link><small>{item.family}</small></li>)}</ul> : null}
      </div>
    </section>
  )
}

// The case chronology as an explorer: one lane per record family on a real time axis, a bubble on every day that has
// events, zoomable with an overview slider. Counts come from the case's real record dates (the service's day-and-family
// summary), so a day is as exact as the date on the record and no more: events and rows open in Investigate, and upload or
// processing dates are never used. Spans over about five months are drawn by week so the bubbles stay readable.
export default function TimelinePage() {
  const { id: caseId = '' } = useParams()
  const [token, setToken] = useState(0)
  const caseIds = useMemo(() => [caseId], [caseId])
  const { status, activity, failures } = useCaseActivity(caseIds, { refreshToken: token })
  const [capabilities, setCapabilities] = useState({ loading: true })
  const [range, setRange] = useState('all')
  const [selected, setSelected] = useState('')
  const [palette, setPalette] = useState(['#0072b2', '#c2410c', '#00795f', '#7e3fb2', '#8a6100', '#475569'])
  const [windowRange, setWindowRange] = useState(null)
  const stage = useRef(null)

  useEffect(() => {
    const controller = new AbortController()
    getQueryCapabilities({ caseId, signal: controller.signal })
      .then(data => setCapabilities({ data, loading: false }))
      .catch(error => error.name !== 'AbortError' && setCapabilities({ error, loading: false }))
    return () => controller.abort()
  }, [caseId])
  useEffect(() => {
    import('../components/charts/chartTheme.js').then(({ chartTheme }) => setPalette(chartTheme().data))
  }, [])

  const questions = useMemo(() => timelineQuestions(capabilities.data), [capabilities.data])
  const weekly = activity.available && activity.days.length >= WEEKLY_FROM_DAYS
  const view = useMemo(() => (weekly ? bucketActivity(activity, 'week') : activity), [activity, weekly])
  const order = useMemo(() => familyOrder(activity.families.map(family => family.id)), [activity.families])
  const typical = useMemo(() => typicalDay(activity), [activity])
  const peaks = useMemo(() => (activity.available ? peakDays(view, 5) : []), [activity.available, view])
  // Until a day is chosen, the busiest one is read out: the place an analyst usually starts.
  const index = Math.max(0, view.days.findIndex(day => day.date === (selected || peaks[0]?.date)))
  const bucket = view.days[index]
  const highlights = activity.available ? activityHighlights(activity) : null
  const lanes = laneOrder(activity)
  const insights = useMemo(() => timelineInsights(activity), [activity])
  const windowed = useMemo(() => (activity.available && windowRange ? windowSummary(view, windowRange.from, windowRange.to) : null), [activity.available, view, windowRange])
  useEffect(() => { setWindowRange(null) }, [range, view])

  const chart = useMemo(() => ({
    buildOption: theme => timelineOption(view, theme, order, { range, selected: bucket?.date || '' }),
    height: laneHeight(lanes.length),
    label: `Timeline: ${lanes.length} record families by ${weekly ? 'week' : 'day'}, ${activity.first} to ${activity.last}. ${activity.families.map(family => `${curatedFamilyLabel(family.id)}: ${formatNumber(family.total)}`).join('. ')}`,
    onSelect: params => {
      const stamp = params?.value?.[0]
      const date = Number.isFinite(stamp) ? dateOfStamp(stamp) : ''
      if (date && view.days.some(day => day.date === date)) setSelected(date)
    },
  }), [view, order, range, bucket?.date, lanes.length, weekly, activity])

  function step(direction) { const next = view.days[index + direction]; if (next) setSelected(next.date) }
  function onKey(event) {
    if (event.key === 'ArrowLeft') { event.preventDefault(); step(-1) }
    else if (event.key === 'ArrowRight') { event.preventDefault(); step(1) }
  }

  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="timeline-page tl" tabIndex={-1}>
        <header className="tl-head">
          <div>
            <h1>Timeline</h1>
            <p className="tl-head__sub">When the records in this case happened, from the dates on the records.</p>
          </div>
          {activity.available && activity.total ? (
            <dl className="tl-facts" aria-label="Timeline summary">
              <div><dt>Events</dt><dd>{formatNumber(activity.total)}</dd></div>
              <div><dt>Span</dt><dd>{activity.first} <i>to</i> {activity.last}</dd></div>
              <div><dt>Active days</dt><dd>{formatNumber(activity.days.length)}</dd></div>
              {highlights ? <div><dt>Busiest day</dt><dd>{highlights.busiest.date} <small>{formatNumber(highlights.busiest.total)}</small></dd></div> : null}
            </dl>
          ) : null}
        </header>

        {status === 'loading' && !activity.available ? <SkeletonRows rows={6} label="Reading the timeline" /> : null}
        {!activity.available && status !== 'loading' ? (
          <p className="tl-empty" role="status"><CircleAlert aria-hidden="true" /><span>{failures.length ? 'The timeline could not be read for this case.' : 'No dated records to place on a timeline.'}{failures[0] ? <small>{activityFailureText(failures[0].reason)}</small> : null}</span>{failures.length ? <button type="button" className="tl-btn" onClick={() => setToken(value => value + 1)}>Try again</button> : null}</p>
        ) : null}
        {activity.available && !activity.total ? <p className="tl-empty">No dated records have been ingested yet. <Link to={`/cases/${encodeURIComponent(caseId)}/evidence`}>Review evidence</Link></p> : null}

        {activity.available && activity.total ? (
          <>
            {insights.length ? (
              <section className="tl-insights" aria-labelledby="tl-insights-title">
                <h2 id="tl-insights-title">What stands out</h2>
                <ul>
                  {insights.map(item => {
                    const Icon = INSIGHT_ICON[item.id]
                    const body = <><span className="tl-insight__icon" aria-hidden="true"><Icon /></span><span><b>{item.title}</b><small>{item.detail}</small></span></>
                    return <li key={item.id} className={`tl-insight tl-insight--${item.tone}`}>{item.date && view.days.some(day => day.date === item.date) ? <button type="button" onClick={() => setSelected(item.date)} aria-label={`${item.title}. ${item.detail} Read this day.`}>{body}</button> : <div>{body}</div>}</li>
                  })}
                </ul>
                <p>Plain arithmetic on the reported counts: days at least 3× the typical day, gaps of a week or more, and a family above half of all events.</p>
              </section>
            ) : null}

            <section className="tl-stage" aria-label="Timeline explorer">
              <header>
                <div className="tl-stage__title"><h2>Events by record family</h2><p>Each bubble is a {weekly ? 'week' : 'day'}; bigger means more events. Scroll to zoom, drag the slider below to move.</p></div>
                <div className="seg" role="group" aria-label="Time range">{RANGES.map(item => <button key={item.id} type="button" aria-pressed={range === item.id} onClick={() => setRange(item.id)}>{item.label}</button>)}</div>
              </header>
              <div ref={stage} className="tl-stage__plot" tabIndex={0} role="group" aria-label="Timeline chart. Left and right arrow keys move between days." onKeyDown={onKey}>
                <Suspense fallback={<p className="chart-card__loading" role="status">Loading chart…</p>}>
                  <EChart buildOption={chart.buildOption} height={chart.height} label={chart.label} onSelect={chart.onSelect} onZoom={setWindowRange} />
                </Suspense>
              </div>
              {windowed && windowed.days ? (
                <div className="tl-window" role="status" aria-live="polite">
                  <span>In view</span>
                  <b>{windowed.first}{windowed.last !== windowed.first ? ` to ${windowed.last}` : ''}</b>
                  <span>{formatNumber(windowed.total)} events · {formatNumber(windowed.days)} {windowed.days === 1 ? (weekly ? 'week' : 'day') : (weekly ? 'weeks' : 'days')}</span>
                  <i className="tl-stack" aria-hidden="true">{Object.entries(windowed.byFamily).sort((left, right) => right[1] - left[1]).map(([id, count]) => <em key={id} style={{ ...dot(familyColour(id, order, palette)), flexGrow: count }} />)}</i>
                </div>
              ) : null}
              <div className="tl-peaks"><span>Busiest {weekly ? 'weeks' : 'days'}</span>
                {peaks.map(day => <button key={day.date} type="button" aria-pressed={bucket?.date === day.date} onClick={() => setSelected(day.date)}>{day.date}<b>{formatNumber(day.total)}</b></button>)}
              </div>
            </section>

            {bucket ? <SelectedDay caseId={caseId} bucket={bucket} typical={typical} order={order} palette={palette} position={index} count={view.days.length} onStep={step} questions={questions} /> : null}

            <details className="tl-table">
              <summary>Exact values by {weekly ? 'week' : 'day'}</summary>
              <div role="region" aria-label="Exact values" tabIndex={0}>
                <table>
                  <thead><tr><th scope="col">{weekly ? 'Week of' : 'Day'}</th><th scope="col" className="is-numeric">Total</th>{lanes.map(id => <th key={id} scope="col" className="is-numeric">{curatedFamilyLabel(id)}</th>)}</tr></thead>
                  <tbody>{view.days.map(day => <tr key={day.date}><th scope="row"><button type="button" onClick={() => setSelected(day.date)}>{day.date}</button></th><td className="is-numeric">{formatNumber(day.total)}</td>{lanes.map(id => <td key={id} className="is-numeric">{day.byFamily[id] ? formatNumber(day.byFamily[id]) : '—'}</td>)}</tr>)}</tbody>
                </table>
              </div>
            </details>
            <p className="tl-note">Counts are per calendar day (UTC) from each record&rsquo;s own date; upload and processing dates are never used. Open a day in Investigate to see its individual events and rows.{activity.truncated ? ' Partial: the service returns at most 100 day-and-family groups, oldest first.' : ''}</p>
          </>
        ) : null}
      </main>
    </CaseShell>
  )
}
