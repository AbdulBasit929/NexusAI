import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowDown, ArrowUp, ChevronLeft, ChevronRight, CircleAlert, MessageSquareText, Search } from 'lucide-react'
import { CaseShell } from '../components/CaseShell.jsx'
import { SkeletonRows } from '../components/Skeleton.jsx'
import { getQueryCapabilities } from '../lib/apiClient.js'
import { activityHighlights, bucketQuestion, describeBucket } from '../lib/caseActivity.js'
import { formatNumber } from '../lib/format.js'
import { curatedFamilyLabel } from '../lib/semanticCatalog.js'
import { againstTypical, dayBreakdown, dayParts, familyOrder, familyScope, groupByMonth, typicalDay, visibleDays } from '../lib/timelineModel.js'
import { activityFailureText, useCaseActivity } from '../lib/useCaseActivity.js'

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

function investigateLink(caseId, item) {
  const query = new URLSearchParams({ question: item.question })
  if (item.scope !== 'all') query.set('scope', item.scope)
  return `/cases/${encodeURIComponent(caseId)}/investigate?${query}`
}

const RANGES = [{ id: 'all', label: 'All' }, { id: '90', label: '90 days' }, { id: '30', label: '30 days' }, { id: '7', label: '7 days' }]

function investigateDay(caseId, day, family = null) {
  const query = new URLSearchParams({ question: family ? `Show ${curatedFamilyLabel(family)} activity on ${day.date}` : bucketQuestion(day) })
  const scope = family ? familyScope(family) : ''
  if (scope) query.set('scope', scope)
  return `/cases/${encodeURIComponent(caseId)}/investigate?${query}`
}

const swatch = index => ({ '--fam': `var(--analyst-data-${(index % 6) + 1})` })

function DayDetail({ caseId, day, typical, order, position, count, onStep, questions }) {
  const rows = dayBreakdown(day, order)
  const compare = againstTypical(day.total, typical)
  return (
    <aside className="tl-detail" aria-label="Selected day">
      <header>
        <p className="tl-detail__kicker">{dayParts(day.date).weekday}</p>
        <h2>{describeBucket(day)}</h2>
        <p className="tl-detail__total"><b>{formatNumber(day.total)}</b> {day.total === 1 ? 'event' : 'events'}{compare ? <span> · {compare}</span> : null}</p>
      </header>
      <ul className="tl-detail__families" aria-label="Events by record family">
        {rows.map(row => (
          <li key={row.id} style={swatch(row.index)}>
            <div><i aria-hidden="true" /><span>{curatedFamilyLabel(row.id)}</span><b>{formatNumber(row.count)}</b></div>
            <span className="tl-detail__bar" aria-hidden="true"><i style={{ inlineSize: `${Math.max(2, row.share)}%` }} /></span>
            <small>{row.share < 1 ? 'Under 1%' : `${row.share}%`} of the day · <Link to={investigateDay(caseId, day, row.id)}>Open these in Investigate</Link></small>
          </li>
        ))}
      </ul>
      <div className="tl-detail__actions">
        <Link className="tl-btn tl-btn--primary" to={investigateDay(caseId, day)}><MessageSquareText aria-hidden="true" />Open this day in Investigate</Link>
        <div className="tl-detail__step" role="group" aria-label="Move between days">
          <button type="button" onClick={() => onStep(-1)} disabled={position <= 0}><ChevronLeft aria-hidden="true" />Earlier day</button>
          <button type="button" onClick={() => onStep(1)} disabled={position >= count - 1}>Later day<ChevronRight aria-hidden="true" /></button>
        </div>
      </div>
      {questions.length ? (
        <section className="tl-detail__ask" aria-labelledby="tl-ask">
          <h3 id="tl-ask">Ask about a sequence</h3>
          <ul>{questions.map(item => <li key={`${item.scope}-${item.question}`}><Link to={`/cases/${encodeURIComponent(caseId)}/investigate?${new URLSearchParams({ question: item.question, ...(item.scope !== 'all' ? { scope: item.scope } : {}) })}`}>{item.question}</Link><small>{item.family}</small></li>)}</ul>
        </section>
      ) : null}
    </aside>
  )
}

// The case chronology: when records happened, day by day, grouped by month, in the families that hold them. Counts come from
// the case's real record dates (the service's day-and-family summary), so each day is as exact as the date on the record and
// no more: individual events and rows open in Investigate, and upload or processing dates are never used. Rows are chosen
// to see a day's make-up; the family chips and range narrow everything, totals included.
export default function TimelinePage() {
  const { id: caseId = '' } = useParams()
  const [token, setToken] = useState(0)
  const caseIds = useMemo(() => [caseId], [caseId])
  const { status, activity, failures } = useCaseActivity(caseIds, { refreshToken: token })
  const [capabilities, setCapabilities] = useState({ loading: true })
  const [range, setRange] = useState('all')
  const [families, setFamilies] = useState([])
  const [newestFirst, setNewestFirst] = useState(true)
  const [selected, setSelected] = useState('')
  const [jump, setJump] = useState('')
  const list = useRef(null)

  useEffect(() => {
    const controller = new AbortController()
    getQueryCapabilities({ caseId, signal: controller.signal })
      .then(data => setCapabilities({ data, loading: false }))
      .catch(error => error.name !== 'AbortError' && setCapabilities({ error, loading: false }))
    return () => controller.abort()
  }, [caseId])

  const questions = useMemo(() => timelineQuestions(capabilities.data), [capabilities.data])
  const order = useMemo(() => familyOrder(activity.families.map(family => family.id)), [activity.families])
  const typical = useMemo(() => typicalDay(activity), [activity])
  const days = useMemo(() => visibleDays(activity, { range, families, newestFirst }), [activity, range, families, newestFirst])
  const groups = useMemo(() => groupByMonth(days), [days])
  const shownTotal = days.reduce((sum, day) => sum + day.shown, 0)
  const peak = days.reduce((best, day) => Math.max(best, day.shown), 1)
  const index = Math.max(0, days.findIndex(day => day.date === selected))
  const current = days[index]
  const highlights = activity.available ? activityHighlights(activity) : null

  function toggle(id) { setFamilies(value => (value.includes(id) ? value.filter(item => item !== id) : [...value, id])) }
  function select(date) { setSelected(date) }
  function step(direction) { const next = days[index + (newestFirst ? -direction : direction)]; if (next) { setSelected(next.date); list.current?.querySelector(`[data-date="${next.date}"]`)?.scrollIntoView?.({ block: 'nearest' }) } }
  function goTo(event) {
    event.preventDefault()
    const hit = days.find(day => day.date === jump.trim()) || days.find(day => day.date.startsWith(jump.trim()))
    if (hit) { setSelected(hit.date); list.current?.querySelector(`[data-date="${hit.date}"]`)?.scrollIntoView?.({ block: 'center' }) }
  }
  function onKey(event) {
    if (!['ArrowDown', 'ArrowUp'].includes(event.key)) return
    const buttons = [...list.current.querySelectorAll('button[data-date]')]
    const at = buttons.indexOf(globalThis.document.activeElement)
    const next = buttons[at + (event.key === 'ArrowDown' ? 1 : -1)]
    if (next) { event.preventDefault(); next.focus(); setSelected(next.dataset.date) }
  }
  // The detail follows the data: when the filter drops the chosen day, the first listed day stands in.
  const position = current ? (newestFirst ? days.length - 1 - index : index) : 0

  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="timeline-page tl" tabIndex={-1}>
        <header className="tl-head">
          <div>
            <h1>Timeline</h1>
            <p className="tl-head__sub">
              {activity.available && activity.total ? <>Day by day, from the dates on the records. <b>{formatNumber(activity.total)}</b> events · {activity.first} to {activity.last}</> : 'When the records in this case happened, day by day.'}
            </p>
          </div>
          {highlights ? <p className="tl-head__peak">Busiest day <Link to={investigateDay(caseId, { date: highlights.busiest.date })}>{highlights.busiest.date}</Link> <small>{formatNumber(highlights.busiest.total)} events</small></p> : null}
        </header>

        {status === 'loading' && !activity.available ? <SkeletonRows rows={6} label="Reading the timeline" /> : null}
        {!activity.available && status !== 'loading' ? (
          <p className="tl-empty" role="status"><CircleAlert aria-hidden="true" /><span>{failures.length ? 'The timeline could not be read for this case.' : 'No dated records to place on a timeline.'}{failures[0] ? <small>{activityFailureText(failures[0].reason)}</small> : null}</span>{failures.length ? <button type="button" className="tl-btn" onClick={() => setToken(value => value + 1)}>Try again</button> : null}</p>
        ) : null}
        {activity.available && !activity.total ? <p className="tl-empty">No dated records have been ingested yet. <Link to={`/cases/${encodeURIComponent(caseId)}/evidence`}>Review evidence</Link></p> : null}

        {activity.available && activity.total ? (
          <>
            <div className="tl-bar">
              <div className="tl-chips" role="group" aria-label="Record families">
                <button type="button" aria-pressed={families.length === 0} onClick={() => setFamilies([])}>All families</button>
                {activity.families.map(family => <button key={family.id} type="button" style={swatch(Math.max(0, order.indexOf(family.id)))} aria-pressed={families.includes(family.id)} onClick={() => toggle(family.id)}><i aria-hidden="true" />{curatedFamilyLabel(family.id)}<small>{formatNumber(family.total)}</small></button>)}
              </div>
              <div className="tl-tools">
                <div className="seg" role="group" aria-label="Time range">{RANGES.map(item => <button key={item.id} type="button" aria-pressed={range === item.id} onClick={() => setRange(item.id)}>{item.label}</button>)}</div>
                <button type="button" className="tl-btn" onClick={() => setNewestFirst(value => !value)} aria-label={newestFirst ? 'Showing newest first. Show oldest first' : 'Showing oldest first. Show newest first'}>{newestFirst ? <ArrowDown aria-hidden="true" /> : <ArrowUp aria-hidden="true" />}{newestFirst ? 'Newest first' : 'Oldest first'}</button>
                <form className="tl-jump" onSubmit={goTo}><Search aria-hidden="true" /><label><span className="visually-hidden">Go to a date</span><input value={jump} onChange={event => setJump(event.target.value)} placeholder="Go to 2026-03" inputMode="numeric" /></label></form>
              </div>
            </div>
            <p className="tl-count" role="status">{days.length ? `${formatNumber(days.length)} ${days.length === 1 ? 'day' : 'days'} · ${formatNumber(shownTotal)} events${families.length ? ` in ${families.map(curatedFamilyLabel).join(', ')}` : ''}` : 'No days match this range and these families.'}</p>

            {days.length ? (
              <div className="tl-body">
                <div className="tl-river" ref={list} onKeyDown={onKey}>
                  {groups.map(group => (
                    <section key={group.key} aria-label={group.label}>
                      <h2 className="tl-month">{group.label}<small>{formatNumber(group.total)} events · {group.days.length} active {group.days.length === 1 ? 'day' : 'days'}</small></h2>
                      <ol>
                        {group.days.map(day => {
                          const parts = dayParts(day.date)
                          const entries = Object.entries(day.byShown).filter(([, count]) => count > 0)
                          return (
                            <li key={day.date}>
                              <button type="button" data-date={day.date} aria-current={current?.date === day.date ? 'true' : undefined} onClick={() => select(day.date)} aria-label={`${describeBucket(day)}: ${formatNumber(day.shown)} events`}>
                                <span className="tl-date"><b>{parts.day}</b><small>{parts.weekday}</small></span>
                                <span className="tl-track" aria-hidden="true"><span style={{ inlineSize: `${Math.max(1.5, (day.shown / peak) * 100)}%` }}>{entries.map(([id, count]) => <i key={id} style={{ ...swatch(Math.max(0, order.indexOf(id))), flexGrow: count }} />)}</span></span>
                                <span className="tl-num">{formatNumber(day.shown)}</span>
                              </button>
                            </li>
                          )
                        })}
                      </ol>
                    </section>
                  ))}
                </div>
                {current ? <DayDetail caseId={caseId} day={current} typical={typical} order={order} position={position} count={days.length} onStep={step} questions={questions} /> : null}
              </div>
            ) : null}
            <p className="tl-note">Counts are per calendar day (UTC) from each record&rsquo;s own date; upload and processing dates are never used. Open a day in Investigate to see its individual events and rows.{activity.truncated ? ' Partial: the service returns at most 100 day-and-family groups, oldest first.' : ''}</p>
          </>
        ) : null}
      </main>
    </CaseShell>
  )
}
