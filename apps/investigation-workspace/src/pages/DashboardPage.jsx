import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, CircleAlert, CircleCheckBig, Clock3, Folder, Search, X } from 'lucide-react'
import { AppShell } from '../components/CaseShell.jsx'
import { EmptyState, LanguageText } from '../components/AnalystComponents.jsx'
import { ChartCard } from '../components/charts/ChartCard.jsx'
import { ProportionBar } from '../components/DataVisualizations.jsx'
import { DashboardHeader } from './dashboard/DashboardHeader.jsx'
import { KpiTiles } from './dashboard/KpiTiles.jsx'
import { AttentionQueue, caseFailures } from './dashboard/AttentionQueue.jsx'
import { ReadinessBars } from './dashboard/ReadinessBars.jsx'
import { configuredCaseIds, getQueryCapabilities } from '../lib/apiClient.js'
import { attentionItems, familyOption, familyRows, formatRate, ingestionRows, readinessRows } from '../lib/dashboardCharts.js'
import { formatNumber } from '../lib/format.js'
import { curatedFamilyLabel } from '../lib/semanticCatalog.js'
import { summariseCase, useConfiguredCaseOverviews } from '../lib/useCaseOverview.js'
import { useQuestionHistoryAcross } from '../lib/workspaceState.js'

const STATE_LABEL = {
  complete: 'Ready',
  processing: 'Processing',
  attention: 'Needs review',
  'not-processed': 'No evidence',
  unavailable: 'Unavailable',
  loading: 'Checking',
}
const STATE_ICON = { complete: CircleCheckBig, processing: Clock3, attention: CircleAlert, 'not-processed': Folder, unavailable: CircleAlert, loading: Clock3 }
const SORT_WEIGHT = { attention: 0, processing: 1, 'not-processed': 2, complete: 3, unavailable: 4, loading: 5 }
const STATE_FILTERS = [['all', 'All cases'], ['attention', 'Needs review'], ['processing', 'Processing'], ['complete', 'Ready']]

function rowMatchesFilter(row, filter) {
  if (filter === 'all') return true
  if (filter.startsWith('family:')) return Boolean(row.summary?.families.some(family => family.record_type === filter.slice(7) && Number(family.accepted_rows || 0) > 0))
  return row.status === filter
}

export function aggregateFamilyRows(rows) {
  const totals = new Map()
  for (const row of rows) {
    const seen = new Set()
    for (const family of row.summary?.families || []) {
      const id = String(family.record_type || '').trim()
      const value = Number(family.accepted_rows || 0)
      if (!id || !Number.isFinite(value) || value <= 0) continue
      const current = totals.get(id) || { id, value: 0, caseCount: 0 }
      current.value += value
      if (!seen.has(id)) {
        current.caseCount += 1
        seen.add(id)
      }
      totals.set(id, current)
    }
  }
  return [...totals.values()].sort((left, right) => right.value - left.value || left.id.localeCompare(right.id))
}

export function dashboardRowState(state) {
  if (state?.error) return 'unavailable'
  if (!state?.data) return 'loading'
  return summariseCase(state.data).processingState
}

export function dashboardNextAction(summary, caseId) {
  const encoded = encodeURIComponent(caseId)
  if (summary.processingState === 'attention') return { label: 'Review evidence', to: `/cases/${encoded}/evidence` }
  if (summary.processingState === 'processing') return { label: 'View processing', to: `/cases/${encoded}/evidence` }
  if (summary.processingState === 'complete') return { label: 'Start investigating', to: `/cases/${encoded}/investigate` }
  return { label: 'Add evidence', to: `/cases/${encoded}/evidence` }
}

export function dashboardOrientation(summary) {
  const reasons = []
  if (summary.failed > 0) reasons.push(`${formatNumber(summary.failed)} evidence item${summary.failed === 1 ? '' : 's'} failed`)
  if (summary.missingAssets > 0) reasons.push(`${formatNumber(summary.missingAssets)} completed job${summary.missingAssets === 1 ? '' : 's'} missing a retained asset`)
  if (reasons.length) return `${reasons.join(' · ')}. Review before relying on complete coverage.`
  if (summary.inFlight > 0) return `${formatNumber(summary.inFlight)} evidence item${summary.inFlight === 1 ? '' : 's'} still processing and excluded from complete analysis.`
  if (summary.total === 0) return 'Add the first source file to establish this case evidence base.'
  return `${formatNumber(summary.ready)} of ${formatNumber(summary.total)} evidence sources are ready for analysis.`
}

// Workspace-level figures, each with the rows behind it. Only reported cases contribute, and the
// count of cases that did not report is carried so the page can say so instead of under-counting silently.
export function dashboardKpis(rows) {
  const reported = rows.filter(row => row.summary)
  const sum = key => reported.reduce((total, row) => total + (row.summary[key] || 0), 0)
  return {
    cases: rows.length,
    reported: reported.length,
    unreported: rows.length - reported.length,
    sources: sum('total'),
    ready: sum('ready'),
    processing: sum('inFlight'),
    failed: sum('failed'),
    gaps: sum('missingAssets'),
    review: sum('failed') + sum('missingAssets'),
    acceptedRows: sum('acceptedRows'),
  }
}

function queueCue(summary) {
  if (summary.failed > 0) return `${formatNumber(summary.failed)} failed source${summary.failed === 1 ? '' : 's'}`
  if (summary.missingAssets > 0) return `${formatNumber(summary.missingAssets)} retained asset gap${summary.missingAssets === 1 ? '' : 's'}`
  if (summary.inFlight > 0) return `${formatNumber(summary.inFlight)} source${summary.inFlight === 1 ? '' : 's'} processing`
  if (summary.total === 0) return 'No evidence added'
  return `${formatNumber(summary.ready)} of ${formatNumber(summary.total)} sources ready`
}

function actionHeading(status, caseId) {
  if (status === 'attention') return <>Review evidence in <LanguageText as="bdi" identifier>{caseId}</LanguageText></>
  if (status === 'processing') return <>Check processing in <LanguageText as="bdi" identifier>{caseId}</LanguageText></>
  if (status === 'not-processed') return <>Establish evidence for <LanguageText as="bdi" identifier>{caseId}</LanguageText></>
  if (status === 'complete') return <>Continue with <LanguageText as="bdi" identifier>{caseId}</LanguageText></>
  if (status === 'loading') return <>Checking <LanguageText as="bdi" identifier>{caseId}</LanguageText></>
  return <>Open <LanguageText as="bdi" identifier>{caseId}</LanguageText></>
}

function StateBadge({ state }) {
  const Icon = STATE_ICON[state] || Folder
  return <span className={`dashboard-state dashboard-state--${state}`}><Icon aria-hidden="true" /><span>{STATE_LABEL[state]}</span></span>
}

function parseTimestamp(value) {
  const date = value ? new Date(value) : null
  return date && !Number.isNaN(date.getTime()) ? date : null
}

function latestEvidenceActivity(data) {
  const candidates = [
    ...(data?.recent_evidence || []).map(item => ({ label: item.source_file, date: parseTimestamp(item.updated_at || item.created_at) })),
    ...(data?.recent_jobs || []).map(item => ({ label: item.source_file, date: parseTimestamp(item.completed_at || item.started_at || item.queued_at) })),
  ].filter(item => item.date)
  candidates.sort((left, right) => right.date.getTime() - left.date.getTime())
  return candidates[0] || null
}

function formatTimestamp(date) {
  return `${new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC' }).format(date)} UTC`
}

function processingLabel(value) {
  const labels = { completed: 'Completed', ready: 'Ready', failed: 'Failed', processing: 'Processing', running: 'Processing', queued: 'Queued' }
  return labels[String(value || '').toLowerCase()] || String(value || 'Status not reported')
}

function jobDuration(job) {
  const start = parseTimestamp(job?.started_at)
  const end = parseTimestamp(job?.completed_at)
  if (!start || !end || end < start) return null
  const seconds = Math.round((end.getTime() - start.getTime()) / 1000)
  if (seconds < 60) return `${formatNumber(seconds)} sec`
  const minutes = Math.floor(seconds / 60)
  return `${formatNumber(minutes)} min ${formatNumber(seconds % 60)} sec`
}

const analystForbiddenQuestion = /\b(?:model|backend|agent|operation|template|sql|embedding|token|deterministic|forensic query|record_type|limit\s*=)\b/i

export function curatedQuestions(payload) {
  const available = new Set((payload?.families || [])
    .filter(family => ['queryable', 'semantic_only', 'limited'].includes(family?.availability))
    .map(family => family.id))
  const seen = new Set()
  return (payload?.query_corpus?.entries || [])
    .filter(entry => entry?.suggested !== false && available.has(entry?.family_id))
    .map(entry => ({ query: String(entry.query || '').trim(), familyId: entry.family_id }))
    .filter(entry => !analystForbiddenQuestion.test(entry.query))
    .filter(entry => {
      const key = entry.query.toLocaleLowerCase()
      if (!key || seen.has(key)) return false
      seen.add(key)
      return true
    })
    .slice(0, 4)
}

function CaseQueueItem({ row, selected, onSelect }) {
  const { caseId, status, summary, activity, state } = row
  return (
    <li>
      <button type="button" className={`dashboard-case dashboard-case--${status}`} aria-pressed={selected} aria-controls="dashboard-case-inspector" onClick={onSelect}>
        <StateBadge state={status} />
        <LanguageText as="bdi" identifier>{caseId}</LanguageText>
        <span className="dashboard-case__cue">
          {state?.loading && !state?.data ? 'Reading evidence state…' : null}
          {state?.error ? (state.error.status === 403 ? 'Status is outside the current access scope.' : 'Status could not be read.') : null}
          {summary ? queueCue(summary) : null}
        </span>
        {summary ? <ProportionBar total={summary.total} ready={summary.ready} processing={summary.inFlight} failed={summary.failed || 0} label={`${caseId} evidence readiness`} /> : null}
        {activity ? <time dateTime={activity.date.toISOString()}>Updated {formatTimestamp(activity.date)}</time> : null}
        <span className="dashboard-case__arrow"><ArrowRight aria-hidden="true" /></span>
      </button>
    </li>
  )
}

function FamilyList({ families }) {
  const items = [...families].filter(family => Number(family.accepted_rows) > 0).sort((left, right) => right.accepted_rows - left.accepted_rows)
  const maximum = Math.max(...items.map(item => item.accepted_rows), 1)
  if (!items.length) return <p>No structured families reported.</p>
  return <ul className="dash-ranked">{items.map(item => <li key={item.record_type}><span>{curatedFamilyLabel(item.record_type)}</span><span className="dash-ranked__bar" aria-hidden="true"><i style={{ inlineSize: `${Math.max((item.accepted_rows / maximum) * 100, 1.5)}%` }} /></span><strong>{formatNumber(item.accepted_rows)}</strong></li>)}</ul>
}

function CaseInspector({ row, capabilityState }) {
  const [detailView, setDetailView] = useState('overview')
  const selectedCaseId = row?.caseId || ''
  useEffect(() => setDetailView('overview'), [selectedCaseId])
  if (!row) return null
  const { caseId, status, summary, activity, state } = row
  const overview = `/cases/${encodeURIComponent(caseId)}/overview`
  const action = summary ? dashboardNextAction(summary, caseId) : { label: 'Open case', to: overview }
  const recentJobs = (state?.data?.recent_jobs || []).slice(0, 4)
  const attention = attentionItems([row])
  const questions = curatedQuestions(capabilityState?.data)
  const tabs = [['overview', 'Overview', null], ['attention', 'Attention', attention.length], ['processing', 'Processing', recentJobs.length], ['questions', 'Questions', questions.length]]

  function moveDetailTab(event) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const current = tabs.findIndex(([id]) => id === detailView)
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : event.key === 'ArrowRight' ? (current + 1) % tabs.length : (current - 1 + tabs.length) % tabs.length
    setDetailView(tabs[next][0])
    event.currentTarget.parentElement?.querySelectorAll('[role="tab"]')?.[next]?.focus()
  }

  return (
    <section id="dashboard-case-inspector" className={`dashboard-focus dashboard-focus--${status}`} aria-labelledby="dashboard-focus-title">
      <header className="dashboard-focus__header">
        <div><p className="eyebrow">Next available action</p><h2 id="dashboard-focus-title">{actionHeading(status, caseId)}</h2></div>
        <StateBadge state={status} />
      </header>
      <p className="dashboard-focus__explanation">
        {state?.loading && !state?.data ? 'Reading the current collection status before suggesting an action.' : null}
        {state?.error ? (state.error.status === 403 ? 'The collection status is outside the current access scope. Open the case to continue within the available permissions.' : 'The collection status is temporarily unavailable. The case itself can still be opened.') : null}
        {summary ? dashboardOrientation(summary) : null}
      </p>
      <p className="dashboard-focus__basis">Readiness order · not investigative priority</p>

      {summary ? (
        <>
          <dl className="dashboard-focus__metrics" aria-label="Selected case evidence readiness">
            <div><dt>Evidence sources</dt><dd>{formatNumber(summary.total)}</dd></div>
            <div><dt>Ready</dt><dd>{formatNumber(summary.ready)}</dd></div>
            <div><dt>Processing</dt><dd>{formatNumber(summary.inFlight)}</dd></div>
            <div><dt>Failed</dt><dd>{summary.failed === null ? 'Not reported' : formatNumber(summary.failed)}</dd></div>
          </dl>
          <div className="dashboard-focus__tabs" role="tablist" aria-label="Selected case details">
            {tabs.map(([id, label, count]) => <button key={id} type="button" role="tab" id={`dashboard-tab-${id}`} tabIndex={detailView === id ? 0 : -1} aria-selected={detailView === id} aria-controls="dashboard-detail-panel" onKeyDown={moveDetailTab} onClick={() => setDetailView(id)}>{label}{count === null ? null : <strong>{formatNumber(count)}</strong>}</button>)}
          </div>

          <div id="dashboard-detail-panel" className="dashboard-focus__tab-panel" role="tabpanel" aria-labelledby={`dashboard-tab-${detailView}`}>
            {detailView === 'overview' ? (
              <div className="dashboard-focus__details">
                <section aria-labelledby="dashboard-families-title">
                  <h3 id="dashboard-families-title">Structured evidence</h3>
                  <FamilyList families={summary.families} />
                </section>
                <section aria-labelledby="dashboard-accounting-title">
                  <h3 id="dashboard-accounting-title">Ingestion</h3>
                  <dl className="dash-accounting">
                    <div><dt>Accepted</dt><dd>{formatNumber(summary.acceptedRows)}</dd></div>
                    <div><dt>Duplicate</dt><dd>{formatNumber(summary.duplicateRows)}</dd></div>
                    <div><dt>Rejected</dt><dd>{formatNumber(summary.rejectedRows)}</dd></div>
                  </dl>
                </section>
              </div>
            ) : null}

            {detailView === 'attention' ? (
              <section className="dashboard-operation dashboard-operation--attention" aria-labelledby="dashboard-attention-title">
                <header><h3 id="dashboard-attention-title">Failures and asset gaps</h3><strong>{formatNumber(attention.length)}</strong></header>
                {attention.length ? <ul>
                  {attention.slice(0, 5).map(item => <li key={`${item.evidenceId || item.label}-${item.kind}`}><span>{item.kind}</span>{item.evidenceId ? <Link to={`/cases/${encodeURIComponent(caseId)}/evidence/${encodeURIComponent(item.evidenceId)}`}><LanguageText>{item.label}</LanguageText></Link> : <LanguageText>{item.label}</LanguageText>}{item.detail ? <small><LanguageText>{item.detail}</LanguageText></small> : null}</li>)}
                </ul> : <p>No failures or asset gaps reported.</p>}
              </section>
            ) : null}

            {detailView === 'processing' ? (
              <section className="dashboard-operation" aria-labelledby="dashboard-processing-title">
                <header><h3 id="dashboard-processing-title">Latest jobs</h3><strong>{formatNumber(recentJobs.length)}</strong></header>
                {recentJobs.length ? <ol className="dashboard-job-list">{recentJobs.map((job, index) => <li key={job.job_id || job.evidence_id || `${job.source_file}-${index}`}><span className={`dashboard-job-list__marker dashboard-job-list__marker--${String(job.status || '').toLowerCase()}`} aria-hidden="true" /><div><LanguageText>{job.source_file || 'Evidence job'}</LanguageText><small>{processingLabel(job.status)}{jobDuration(job) ? ` · ${jobDuration(job)}` : ''}</small></div></li>)}</ol> : <p>No recent jobs reported.</p>}
              </section>
            ) : null}

            {detailView === 'questions' ? (
              <section className="dashboard-suggestions" aria-labelledby="dashboard-suggestions-title">
                <header><h3 id="dashboard-suggestions-title">Suggested questions</h3>{capabilityState?.loading ? <span role="status">Loading…</span> : null}</header>
                {questions.length ? <ul>{questions.map(item => <li key={item.query}><Link to={`/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(item.query)}`}><LanguageText>{item.query}</LanguageText><ArrowRight aria-hidden="true" /></Link></li>)}</ul> : capabilityState?.loading ? null : <p>No curated questions reported.</p>}
              </section>
            ) : null}
          </div>
        </>
      ) : null}

      <div className="dashboard-focus__footer">
        <div>
          <span>Latest evidence activity</span>
          {activity ? <><LanguageText>{activity.label || 'Evidence update'}</LanguageText><time dateTime={activity.date.toISOString()}>{formatTimestamp(activity.date)}</time></> : <small>No recent evidence activity reported</small>}
        </div>
        <div className="dashboard-focus__actions">
          <Link className="dashboard-focus__secondary" to={overview}>Open overview</Link>
          <Link className="dashboard-focus__primary" to={action.to}>{action.label}<ArrowRight aria-hidden="true" /></Link>
        </div>
      </div>
    </section>
  )
}

function IngestionCard({ rows }) {
  return (
    <section id="dashboard-ingestion" className="dash-card" aria-labelledby="dashboard-ingestion-heading">
      <header className="dash-card__header"><div><h2 id="dashboard-ingestion-heading">Did ingestion keep every row?</h2><p>Rows accepted, dropped as duplicates or rejected, per case. Rates are shares of all rows the case ingested.</p></div></header>
      {rows.length ? (
        <ul className="dash-ingest">{rows.map(row => (
          <li key={row.caseId}>
            <Link to={`/cases/${encodeURIComponent(row.caseId)}/overview`}><LanguageText as="bdi" identifier>{row.caseId}</LanguageText></Link>
            <dl>
              <div><dt>Accepted</dt><dd>{formatNumber(row.accepted)}</dd></div>
              <div><dt>Duplicate</dt><dd>{formatNumber(row.duplicate)}<small>{formatRate(row.duplicateShare)}</small></dd></div>
              <div className={row.rejected ? 'is-flagged' : undefined}><dt>Rejected</dt><dd>{formatNumber(row.rejected)}<small>{formatRate(row.rejectedShare)}</small></dd></div>
            </dl>
          </li>
        ))}</ul>
      ) :<p className="dash-card__empty">No structured rows have been ingested yet.</p>}
    </section>
  )
}

export default function DashboardPage() {
  const cases = configuredCaseIds()
  const recent = useQuestionHistoryAcross(cases).slice(0, 4)
  const { states, reload } = useConfiguredCaseOverviews(cases)
  const [filter, setFilter] = useState('all')
  const [query, setQuery] = useState('')
  const [selectedCaseId, setSelectedCaseId] = useState('')
  const [capabilityState, setCapabilityState] = useState({ loading: false, data: null })

  const rows = useMemo(() => cases.map((caseId, index) => {
    const state = states[caseId]
    return { caseId, index, state, status: dashboardRowState(state), summary: state?.data ? summariseCase(state.data) : null, activity: state?.data ? latestEvidenceActivity(state.data) : null }
  }), [cases, states])
  const kpis = useMemo(() => dashboardKpis(rows), [rows])
  const loading = rows.some(row => row.status === 'loading')
  const unreadable = rows.filter(row => row.status === 'unavailable').length
  const readiness = useMemo(() => readinessRows(rows), [rows])
  const families = useMemo(() => familyRows(aggregateFamilyRows(rows)), [rows])
  const ingestion = useMemo(() => ingestionRows(rows), [rows])
  const attention = useMemo(() => attentionItems(rows), [rows])
  const failures = useMemo(() => caseFailures(rows), [rows])
  const visible = useMemo(() => rows
    .filter(row => rowMatchesFilter(row, filter))
    .filter(row => row.caseId.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()))
    .sort((left, right) => SORT_WEIGHT[left.status] - SORT_WEIGHT[right.status] || left.index - right.index), [filter, query, rows])
  const selected = visible.find(row => row.caseId === selectedCaseId) || visible[0]
  const isRefreshing = rows.some(row => row.state?.refreshing)
  const hasProcessing = rows.some(row => row.status === 'processing')
  // The age of the figures is the age of the oldest read among the cases shown; the newest would hide a case whose refresh failed.
  const lastUpdated = rows.filter(row => row.summary).map(row => parseTimestamp(row.state?.receivedAt)).filter(Boolean).sort((left, right) => left - right)[0] || null

  const familyChart = useMemo(() => ({
    buildOption: theme => familyOption(families, theme),
    height: Math.max(90, families.length * 30 + 12),
    label: `Accepted rows by evidence family. ${families.map(item => `${item.label}: ${formatNumber(item.value)}`).join('. ')}`,
    onSelect: params => { if (params?.data?.id) { setFilter(`family:${params.data.id}`); setSelectedCaseId(''); globalThis.document?.getElementById('dashboard-workbench-title')?.scrollIntoView?.({ block: 'start' }) } },
  }), [families])

  useEffect(() => {
    if (!selected?.caseId) {
      setCapabilityState({ loading: false, data: null })
      return undefined
    }
    const controller = new AbortController()
    setCapabilityState({ loading: true, data: null })
    getQueryCapabilities({ caseId: selected.caseId, signal: controller.signal })
      .then(data => setCapabilityState({ loading: false, data }))
      .catch(error => { if (error.name !== 'AbortError') setCapabilityState({ loading: false, data: null, error }) })
    return () => controller.abort()
  }, [selected?.caseId])

  useEffect(() => {
    if (!hasProcessing) return undefined
    const refreshWhenVisible = () => { if (globalThis.document?.visibilityState === 'visible') reload() }
    const timer = globalThis.setInterval(refreshWhenVisible, 15_000)
    globalThis.document?.addEventListener('visibilitychange', refreshWhenVisible)
    return () => {
      globalThis.clearInterval(timer)
      globalThis.document?.removeEventListener('visibilitychange', refreshWhenVisible)
    }
  }, [hasProcessing, reload])

  const clearView = useCallback(() => { setFilter('all'); setQuery(''); setSelectedCaseId('') }, [])
  const filterLabel = filter.startsWith('family:') ? `Cases with ${curatedFamilyLabel(filter.slice(7))}` : STATE_FILTERS.find(([id]) => id === filter)?.[1]

  return (
    <AppShell identityLed>
      <main id="workspace-main" className="catalog-page dashboard-page dashboard-command" tabIndex={-1}>
        <DashboardHeader caseCount={cases.length} reporting={kpis.reported} loading={loading} lastUpdated={lastUpdated} processing={hasProcessing} refreshing={isRefreshing} onRefresh={reload} />

        {cases.length ? (
          <>
            <KpiTiles kpis={kpis} loading={loading} />
            {unreadable > 0 ? <p className="dash-notice" role="status"><CircleAlert aria-hidden="true" />{formatNumber(unreadable)} of {formatNumber(cases.length)} {unreadable === 1 ? 'case' : 'cases'} could not report status, so totals above cover the rest. <button type="button" onClick={reload}>Try again</button></p> : null}

            <div className="dash-grid dash-grid--wide-left">
              <AttentionQueue items={attention} kpis={kpis} failures={failures} loading={loading} />
              <ReadinessBars rows={readiness} kpis={kpis} loading={loading} />
            </div>

            <div className="dash-grid dash-grid--wide-left">
              <div id="dashboard-families">
                {families.length ? (
                  <ChartCard
                    title="Which kinds of records make up the evidence?"
                    description="Accepted rows by structured record family, largest first. Select a bar to show only the cases that have it."
                    chart={familyChart}
                    columns={[
                      { key: 'label', label: 'Record family' },
                      { key: 'value', label: 'Accepted rows', numeric: true, render: row => formatNumber(row.value) },
                      { key: 'caseCount', label: 'Cases', numeric: true },
                      { key: 'show', label: 'Filter', render: row => <button type="button" className="dash-link-button" aria-label={`Show only cases with ${row.label}`} onClick={() => { setFilter(`family:${row.id}`); setSelectedCaseId('') }}>Show cases</button> },
                    ]}
                    rows={families.map(item => ({ ...item, key: item.id }))}
                    footer={<span>{formatNumber(kpis.acceptedRows)} accepted rows in total. Counts cover structured records, not every file.</span>}
                  />
                ) : <section className="dash-card"><header className="dash-card__header"><div><h2>Which kinds of records make up the evidence?</h2></div></header><p className="dash-card__empty">{loading ? 'Reading record families…' : 'No structured records have been accepted yet.'}</p></section>}
              </div>
              <IngestionCard rows={ingestion} />
            </div>

            <section className="dashboard-workbench" aria-labelledby="dashboard-workbench-title">
              <header className="dashboard-workbench__toolbar">
                <div><p className="eyebrow">Case workbench</p><h2 id="dashboard-workbench-title">Cases</h2></div>
                <form className="dashboard-search" role="search" onSubmit={event => event.preventDefault()}>
                  <label className="visually-hidden" htmlFor="dashboard-case-search">Find a case</label>
                  <span className="dashboard-search__field"><Search aria-hidden="true" /><input id="dashboard-case-search" type="search" value={query} onChange={event => { setQuery(event.target.value); setSelectedCaseId('') }} placeholder="Search collection ID" /></span>
                </form>
              </header>
              <div className="dash-chips" role="group" aria-label="Filter cases">
                {STATE_FILTERS.map(([id, label]) => <button key={id} type="button" aria-pressed={filter === id} onClick={() => { setFilter(id); setSelectedCaseId('') }}>{label}</button>)}
                {filter.startsWith('family:') ? <button type="button" aria-pressed="true" onClick={clearView}>{filterLabel}<X aria-hidden="true" /><span className="visually-hidden">Clear this filter</span></button> : null}
              </div>

              {visible.length ? (
                <div className="dashboard-workbench__body">
                  <div className="dashboard-case-list">
                    <p className="dashboard-case-list__count"><strong>{formatNumber(visible.length)}</strong> of {formatNumber(cases.length)} cases <span>· readiness order</span></p>
                    <ol>{visible.map(row => <CaseQueueItem key={row.caseId} row={row} selected={selected?.caseId === row.caseId} onSelect={() => setSelectedCaseId(row.caseId)} />)}</ol>
                  </div>
                  <CaseInspector row={selected} capabilityState={capabilityState} />
                </div>
              ) : (
                <EmptyState kind="no-match" label="No cases match this view" description="The search and filter apply only to the cases in this workspace."><button type="button" onClick={clearView}>Clear view</button></EmptyState>
              )}
            </section>

            <section className="dashboard-recent" aria-labelledby="recent-work-title">
              <header><div><p className="eyebrow">This browser only</p><h2 id="recent-work-title">Recent questions</h2></div><span>{formatNumber(recent.length)} saved</span></header>
              {recent.length ? (
                <ol>{recent.map(entry => (
                  <li key={`${entry.caseId}-${entry.id}`}>
                    <Link to={`/cases/${encodeURIComponent(entry.caseId)}/investigate?question=${encodeURIComponent(entry.query)}`} title={entry.label ? entry.query : undefined}>
                      <span className="dashboard-recent__question"><LanguageText>{entry.label || entry.query}</LanguageText>{entry.pinned ? <small>Pinned</small> : null}</span>
                      {entry.label ? <small><LanguageText>{entry.query}</LanguageText></small> : null}
                      <span><bdi dir="ltr">{entry.caseId}</bdi><ArrowRight aria-hidden="true" /></span>
                    </Link>
                  </li>
                ))}</ol>
              ) : <p className="dashboard-recent__empty">Questions you ask in this browser appear here as links that reopen them. <Link to="/investigate">Ask a question</Link></p>}
            </section>

            <p className="dash-scope"><CircleAlert aria-hidden="true" />Scope: the {formatNumber(cases.length)} case{cases.length === 1 ? '' : 's'} configured for this workspace. Assignment, severity and investigative priority are not available. <Link to="/cases">Open all cases</Link></p>
          </>
        ) : (
          <EmptyState kind="not-processed" label="No cases are configured" description="Add the first evidence file to establish a collection-backed case."><Link to="/cases/new">Add evidence</Link></EmptyState>
        )}
      </main>
    </AppShell>
  )
}
