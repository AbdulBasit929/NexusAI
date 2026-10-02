import { lazy, Suspense, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { CircleAlert, ClipboardCopy, Download, RotateCcw, Search } from 'lucide-react'
import { CaseShell } from '../components/CaseShell.jsx'
import { SkeletonRows } from '../components/Skeleton.jsx'
import { getQueryCapabilities } from '../lib/apiClient.js'
import { activityHighlights, bucketActivity, bucketQuestion } from '../lib/caseActivity.js'
import { formatNumber } from '../lib/format.js'
import { curatedFamilyLabel } from '../lib/semanticCatalog.js'
import { dateOfStamp, laneOrder, workbenchHeight, workbenchOption } from '../lib/timelineChart.js'
import { timelineInsights, windowSummary } from '../lib/timelineInsights.js'
import { briefing, daysCsv, familyOrder, familyScope, filterActivity, peakDays, typicalDay } from '../lib/timelineModel.js'
import { usePins } from '../lib/timelineAnnotations.js'
import { activityFailureText, useCaseActivity } from '../lib/useCaseActivity.js'
import { FamilyStrip, InsightsCard, RhythmPanel, SavedPanel } from './timeline/FamilyRail.jsx'
import { Inspector } from './timeline/Inspector.jsx'

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

function investigateLink(caseId) {
  return (bucket, family = null) => {
    const query = new URLSearchParams({ question: family ? `Show ${curatedFamilyLabel(family)} activity on ${bucket.date}${bucket.end && bucket.end !== bucket.date ? ` to ${bucket.end}` : ''}` : bucketQuestion(bucket) })
    const scope = family ? familyScope(family) : ''
    if (scope) query.set('scope', scope)
    return `/cases/${encodeURIComponent(caseId)}/investigate?${query}`
  }
}

function download(name, text, type) {
  const url = URL.createObjectURL(new Blob([text], { type }))
  const link = globalThis.document.createElement('a')
  link.href = url; link.download = name
  globalThis.document.body.append(link); link.click(); link.remove()
  globalThis.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

// The case timeline as a workbench: a rail of record-family filters, the weekly rhythm and pinned days on the left; in the
// middle one chart with two views on a shared time axis (stacked volume above, a lane per family below) and a zoom whose
// window is summarised live; and an inspector on the right for the chosen day. It is built only on the real day-and-family
// counts (the service's `activity_by_day`), so a day is as exact as the date on the record: upload and processing dates are
// never used, and events inside a day open in Investigate until the service supplies them.
export default function TimelinePage() {
  const { id: caseId = '' } = useParams()
  const [token, setToken] = useState(0)
  const caseIds = useMemo(() => [caseId], [caseId])
  const { status, activity: all, failures } = useCaseActivity(caseIds, { refreshToken: token })
  const [capabilities, setCapabilities] = useState({ loading: true })
  const [range, setRange] = useState('all')
  const [chosen, setChosen] = useState([])
  const [selected, setSelected] = useState('')
  const [jump, setJump] = useState('')
  const [windowRange, setWindowRange] = useState(null)
  const [copied, setCopied] = useState(false)
  const [palette, setPalette] = useState(['#0072b2', '#c2410c', '#00795f', '#7e3fb2', '#8a6100', '#475569'])
  const { pins, toggle, setNote } = usePins(caseId)

  useEffect(() => {
    const controller = new AbortController()
    getQueryCapabilities({ caseId, signal: controller.signal })
      .then(data => setCapabilities({ data, loading: false }))
      .catch(error => error.name !== 'AbortError' && setCapabilities({ error, loading: false }))
    return () => controller.abort()
  }, [caseId])
  useEffect(() => { import('../components/charts/chartTheme.js').then(({ chartTheme }) => setPalette(chartTheme().data)) }, [])

  const questions = useMemo(() => timelineQuestions(capabilities.data), [capabilities.data])
  const activity = useMemo(() => filterActivity(all, chosen), [all, chosen])
  const weekly = activity.available && activity.days.length >= WEEKLY_FROM_DAYS
  const view = useMemo(() => (weekly ? bucketActivity(activity, 'week') : activity), [activity, weekly])
  const order = useMemo(() => familyOrder(all.families.map(family => family.id)), [all.families])
  const typical = useMemo(() => typicalDay(activity), [activity])
  const peaks = useMemo(() => (activity.available ? peakDays(view, 4) : []), [activity.available, view])
  const insights = useMemo(() => timelineInsights(activity), [activity])
  const windowed = useMemo(() => (activity.available && windowRange ? windowSummary(view, windowRange.from, windowRange.to) : null), [activity.available, view, windowRange])
  const lanes = laneOrder(activity)
  const shown = windowed && windowed.days ? view.days.filter(day => Date.parse(`${day.end || day.date}T00:00:00Z`) >= windowRange.from && Date.parse(`${day.date}T00:00:00Z`) <= windowRange.to) : view.days
  // Until a day is chosen, the busiest one is read out: the place an analyst usually starts.
  const index = Math.max(0, view.days.findIndex(day => day.date === (selected || peaks[0]?.date)))
  const bucket = view.days[index]
  const highlights = all.available ? activityHighlights(all) : null
  const link = useMemo(() => investigateLink(caseId), [caseId])
  const pin = pins.find(item => item.date === bucket?.date)

  useEffect(() => { setWindowRange(null) }, [range, view])

  const chart = useMemo(() => ({
    buildOption: theme => workbenchOption(view, theme, order, { range, selected: bucket?.date || '' }),
    height: workbenchHeight(lanes.length),
    label: `Timeline of ${lanes.length} record families by ${weekly ? 'week' : 'day'}, ${activity.first} to ${activity.last}: stacked volume above and one lane per family below. ${activity.families.map(family => `${curatedFamilyLabel(family.id)}: ${formatNumber(family.total)}`).join('. ')}`,
    onSelect: params => {
      const stamp = params?.value?.[0]
      const date = Number.isFinite(stamp) ? dateOfStamp(stamp) : ''
      if (date && view.days.some(day => day.date === date)) setSelected(date)
    },
  }), [view, order, range, bucket?.date, lanes.length, weekly, activity])

  function step(direction) { const next = view.days[index + direction]; if (next) setSelected(next.date) }
  function goTo(event) {
    event.preventDefault()
    const text = jump.trim()
    const hit = view.days.find(day => day.date === text) || view.days.find(day => day.date.startsWith(text))
    if (hit) setSelected(hit.date)
  }
  function onKey(event) {
    if (['INPUT', 'TEXTAREA', 'SELECT'].includes(event.target.tagName)) return
    if (event.key === 'ArrowLeft') { event.preventDefault(); step(-1) }
    else if (event.key === 'ArrowRight') { event.preventDefault(); step(1) }
    else if (event.key.toLowerCase() === 'p' && bucket) toggle(bucket.date)
  }
  function toggleFamily(id) { setChosen(value => (value.includes(id) ? value.filter(item => item !== id) : [...value, id])) }
  async function copyBrief() {
    try { await globalThis.navigator.clipboard.writeText(briefing({ days: shown, families: curatedFamilyLabel, caseId })); setCopied(true); globalThis.setTimeout(() => setCopied(false), 1600) } catch { setCopied(false) }
  }
  function exportCsv() { download(`timeline-${caseId}.csv`, daysCsv(shown, lanes, curatedFamilyLabel), 'text/csv') }
  function reset() { setChosen([]); setRange('all'); setSelected(''); setJump('') }

  const ready = activity.available && all.total > 0
  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="timeline-page tw" tabIndex={-1} onKeyDown={onKey}>
        <header className="tw-head">
          <div>
            <h1>Timeline</h1>
            <p>When the records in this case happened, from the dates on the records.</p>
          </div>
          {all.available && all.total ? (
            <dl className="tw-facts" aria-label="Timeline summary">
              <div><dt>Events</dt><dd>{formatNumber(all.total)}</dd></div>
              <div><dt>Span</dt><dd>{all.first} <i>to</i> {all.last}</dd></div>
              <div><dt>Active days</dt><dd>{formatNumber(all.days.length)}</dd></div>
              {highlights ? <div><dt>Busiest day</dt><dd>{highlights.busiest.date} <small>{formatNumber(highlights.busiest.total)}</small></dd></div> : null}
            </dl>
          ) : null}
        </header>

        {status === 'loading' && !all.available ? <SkeletonRows rows={6} label="Reading the timeline" /> : null}
        {!all.available && status !== 'loading' ? (
          <p className="tw-empty" role="status"><CircleAlert aria-hidden="true" /><span>{failures.length ? 'The timeline could not be read for this case.' : 'No dated records to place on a timeline.'}{failures[0] ? <small>{activityFailureText(failures[0].reason)}</small> : null}</span>{failures.length ? <button type="button" className="tw-btn" onClick={() => setToken(value => value + 1)}>Try again</button> : null}</p>
        ) : null}
        {all.available && !all.total ? <p className="tw-empty">No dated records have been ingested yet. <Link to={`/cases/${encodeURIComponent(caseId)}/evidence`}>Review evidence</Link></p> : null}

        {ready ? (
          <>
            <div className="tw-bar" role="toolbar" aria-label="Timeline controls">
              <div className="seg" role="group" aria-label="Time range">{RANGES.map(item => <button key={item.id} type="button" aria-pressed={range === item.id} onClick={() => setRange(item.id)}>{item.label}</button>)}</div>
              <form className="tw-jump" onSubmit={goTo}><Search aria-hidden="true" /><label><span className="visually-hidden">Go to a date</span><input value={jump} onChange={event => setJump(event.target.value)} placeholder="Go to 2026-03" inputMode="numeric" /></label></form>
              <span className="tw-bar__space" />
              {(chosen.length || range !== 'all') ? <button type="button" className="tw-btn" onClick={reset}><RotateCcw aria-hidden="true" />Reset</button> : null}
              <button type="button" className="tw-btn" onClick={copyBrief}><ClipboardCopy aria-hidden="true" />{copied ? 'Copied' : 'Copy briefing'}</button>
              <button type="button" className="tw-btn" onClick={exportCsv}><Download aria-hidden="true" />Export CSV</button>
            </div>

            <FamilyStrip all={all.families} days={all.days} order={order} palette={palette} chosen={chosen} onToggle={toggleFamily} onClear={() => setChosen([])} totals={all.total} />

            <div className="tw-main">
              <section className="tw-stage" aria-label="Timeline explorer">
                <header>
                  <h2>Volume and activity by record family</h2>
                  <p>Each {weekly ? 'week' : 'day'} on one time axis. Scroll to zoom, drag the slider to move, select a bar or bubble to read it.</p>
                </header>
                {activity.days.length ? (
                  <div className="tw-stage__plot" role="group" aria-label="Timeline chart. Left and right arrow keys move between days, P pins the day." tabIndex={0}>
                    <Suspense fallback={<p className="chart-card__loading" role="status">Loading chart…</p>}>
                      <EChart buildOption={chart.buildOption} height={chart.height} label={chart.label} onSelect={chart.onSelect} onZoom={setWindowRange} />
                    </Suspense>
                  </div>
                ) : <p className="tw-empty">No days match the chosen families.</p>}
                <footer>
                  <p className="tw-window" role="status" aria-live="polite">
                    <span>In view</span>
                    {windowed && windowed.days ? <><b>{windowed.first}{windowed.last !== windowed.first ? ` to ${windowed.last}` : ''}</b><span>{formatNumber(windowed.total)} events · {formatNumber(windowed.days)} {weekly ? (windowed.days === 1 ? 'week' : 'weeks') : (windowed.days === 1 ? 'day' : 'days')}</span></> : <><b>{activity.first} to {activity.last}</b><span>{formatNumber(activity.total)} events · {formatNumber(activity.days.length)} {weekly ? 'weeks' : 'days'}</span></>}
                  </p>
                  <div className="tw-peaks"><span>Busiest {weekly ? 'weeks' : 'days'}</span>
                    {peaks.map(day => <button key={day.date} type="button" aria-pressed={bucket?.date === day.date} onClick={() => setSelected(day.date)}>{day.date}<b>{formatNumber(day.total)}</b></button>)}
                  </div>
                </footer>
              </section>
              {bucket ? <Inspector caseId={caseId} bucket={bucket} typical={typical} order={order} palette={palette} position={index} count={view.days.length} onStep={step} neighbours={view.days.slice(Math.max(0, index - 10), index + 11)} onSelect={setSelected} pinned={Boolean(pin)} note={pin?.note || ''} onPin={() => toggle(bucket.date)} onNote={text => setNote(bucket.date, text)} link={link} /> : <aside className="tw-inspector"><p className="tw-empty">Nothing to read for these families.</p></aside>}
            </div>

            <div className="tw-below">
              <InsightsCard insights={insights} questions={questions} caseId={caseId} canOpen={date => view.days.some(day => day.date === date)} onSelect={setSelected} />
              <RhythmPanel days={activity.days} />
              <SavedPanel pins={pins} onOpenPin={setSelected} onRemovePin={toggle} />
            </div>

            <details className="tw-table">
              <summary>Exact values by {weekly ? 'week' : 'day'}</summary>
              <div role="region" aria-label="Exact values" tabIndex={0}>
                <table>
                  <thead><tr><th scope="col">{weekly ? 'Week of' : 'Day'}</th><th scope="col" className="is-numeric">Total</th>{lanes.map(id => <th key={id} scope="col" className="is-numeric">{curatedFamilyLabel(id)}</th>)}</tr></thead>
                  <tbody>{view.days.map(day => <tr key={day.date}><th scope="row"><button type="button" onClick={() => setSelected(day.date)}>{day.date}</button></th><td className="is-numeric">{formatNumber(day.total)}</td>{lanes.map(id => <td key={id} className="is-numeric">{day.byFamily[id] ? formatNumber(day.byFamily[id]) : '—'}</td>)}</tr>)}</tbody>
                </table>
              </div>
            </details>
            <p className="tw-note">Counts are per calendar day (UTC) from each record&rsquo;s own date; upload and processing dates are never used.{activity.truncated ? ' Partial: the service returns at most 100 day-and-family groups, oldest first.' : ''}</p>
          </>
        ) : null}
      </main>
    </CaseShell>
  )
}
