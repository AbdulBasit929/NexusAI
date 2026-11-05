import { Link } from 'react-router-dom'
import { ArrowRight, CircleAlert, CircleCheckBig, Clock3, History, MessageSquareText, Plus, RefreshCw, ServerOff } from 'lucide-react'
import { formatNumber } from '../../../lib/format.js'
import { formatRate } from '../../../lib/dashboardCharts.js'
import { relativeAge } from '../../../lib/dashboardCases.js'
import { CaseLabel } from '../../shell/CaseLabel.jsx'
import { Calm, Panel, PanelSkeleton } from '../../ui/Panel.jsx'
import { formatDuration, formatMoment } from './model.js'

// ---------- the header: the title, the count of cases in each state, two actions ----------
export function statusCounts(rows) {
  return [
    { id: 'attention', label: 'need review', count: rows.filter(row => row.status === 'attention').length, tone: 'danger' },
    { id: 'processing', label: 'processing', count: rows.filter(row => row.status === 'processing').length, tone: 'warn' },
    { id: 'complete', label: 'ready', count: rows.filter(row => row.status === 'complete').length, tone: 'ok' },
    { id: 'unavailable', label: 'not reporting', count: rows.filter(row => row.status === 'unavailable').length, tone: 'warn' },
  ].filter(item => item.count > 0)
}

export function Hero({ rows, loading, lastUpdated, refreshing, onRefresh }) {
  const counts = statusCounts(rows)
  return (
    <header className="nx-hero">
      <div className="nx-hero__text">
        <h1>Dashboard</h1>
        <ul className="nx-pills" aria-label="Cases by state" aria-live="polite">
          {counts.length ? counts.map(item => <li key={item.id} className="nx-pill"><span className={`nx-dot nx-dot--${item.tone}`} aria-hidden="true" />{formatNumber(item.count)} {item.label}</li>) : <li className="nx-pill nx-muted">{loading ? 'Reading…' : 'No case yet'}</li>}
        </ul>
      </div>
      <div className="nx-hero__actions">
        <button type="button" className="nx-btn nx-btn--ghost nx-btn--sm" onClick={onRefresh} aria-label="Refresh the figures" title={lastUpdated ? `Updated ${relativeAge(lastUpdated)}` : 'Refresh'}><RefreshCw aria-hidden="true" className={refreshing ? 'is-spinning' : undefined} />{refreshing ? 'Refreshing' : lastUpdated ? relativeAge(lastUpdated) : 'Refresh'}</button>
        <Link className="nx-btn" to="/investigate"><MessageSquareText aria-hidden="true" />Ask across cases</Link>
        <Link className="nx-btn nx-btn--primary" to="/cases/new"><Plus aria-hidden="true" />New case</Link>
      </div>
    </header>
  )
}

// ---------- the way back into the work: the last question in each of the cases you touched most recently (up to three) ----------
export function resumeEntries(recent, limit = 3) {
  const seen = new Set()
  return recent.filter(entry => (seen.has(entry.caseId) ? false : seen.add(entry.caseId))).slice(0, limit)
}

export function ResumePanel({ entries }) {
  return (
    <Panel id="dash-resume" title="Pick up" sub={entries.length ? 'Your last questions' : null}>
      {entries.length ? (
        <>
        <ul className="nx-rows">
          {entries.map(entry => (
            <li key={`${entry.caseId}-${entry.id}`}>
              <Link className="nx-rows__item" to={`/cases/${encodeURIComponent(entry.caseId)}/investigate?question=${encodeURIComponent(entry.query)}`}>
                <History className="nx-rows__icon" aria-hidden="true" />
                <span className="nx-rows__text">
                  <span className="nx-rows__main" dir="auto">{entry.label || entry.query}</span>
                  <span className="nx-rows__meta"><CaseLabel caseId={entry.caseId} />{entry.updatedAt ? <> · <time dateTime={entry.updatedAt} title={formatMoment(new Date(entry.updatedAt))}>{relativeAge(new Date(entry.updatedAt))}</time></> : null}</span>
                </span>
                <ArrowRight className="nx-rows__go" aria-hidden="true" />
              </Link>
            </li>
          ))}
        </ul>
        <Link className="nx-rows__foot" to="/investigate"><MessageSquareText aria-hidden="true" />Ask across cases<ArrowRight aria-hidden="true" /></Link>
        </>
      ) : (
        <div className="nx-fill nx-empty-action">
          <Calm icon={History}>No question asked yet</Calm>
          <Link className="nx-btn" to="/investigate"><MessageSquareText aria-hidden="true" />Ask across cases</Link>
        </div>
      )}
    </Panel>
  )
}

// The share of the rows read that were accepted (null before any row was read).
export function cleanRate(kpis) {
  const total = kpis.acceptedRows + kpis.duplicateRows + kpis.rejectedRows
  return total ? kpis.acceptedRows / total : null
}

// ---------- what needs you: a few short lines, one for each cause ----------
// The count and the cause, nothing else; the line opens the first thing of that cause, and the things themselves are in its title (on hover and on focus). The order is what blocks your work first.
export function NeedsPanel({ needs, loading, notesLoading }) {
  const groups = needs.groups.slice(0, 6)
  return (
    <Panel id="dash-attention" title="Needs you" className="nx-panel--needs" action={needs.attention ? <span className="nx-badge nx-badge--danger" title="Things that block your work">{formatNumber(needs.attention)}</span> : null}>
      {loading && !groups.length ? <PanelSkeleton rows={3} /> : null}
      {groups.length ? (
        <ul className="nx-causes">
          {groups.map(group => (
            <li key={group.id}>
              <Link className="nx-causes__row" to={group.items[0]?.to || '/activity'} title={group.items.slice(0, 4).map(item => item.title).join('\n')}>
                <span className={`nx-dot nx-dot--${group.tone === 'muted' ? '' : group.tone}`} aria-hidden="true" />
                <span className="nx-causes__label">{group.label}</span>
                <span className="nx-causes__count nx-tabular">{formatNumber(group.count)}</span>
                <ArrowRight className="nx-causes__go" aria-hidden="true" />
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
      {notesLoading && !needs.groups.some(group => group.id === 'notes') ? <PanelSkeleton rows={1} /> : null}
      {!groups.length && !loading && !notesLoading ? <Calm icon={CircleCheckBig} tone="ok"><strong>All clear</strong></Calm> : null}
    </Panel>
  )
}

// ---------- the latest things that happened to the evidence ----------
export function latestItems(rows, limit = 6) {
  const found = new Map()
  const add = (caseId, item, status, date) => {
    if (!date) return
    const key = `${caseId}|${item.evidence_id || item.source_file}`
    const existing = found.get(key)
    if (!existing || existing.date < date) found.set(key, { caseId, label: item.source_file || 'Evidence', evidenceId: item.evidence_id, status, date })
  }
  const stamp = value => { const date = value ? new Date(value) : null; return date && !Number.isNaN(date.getTime()) ? date : null }
  for (const row of rows) {
    for (const item of row.state?.data?.recent_evidence || []) add(row.caseId, item, String(item.processing_status || '').toLowerCase(), stamp(item.updated_at || item.created_at))
    for (const item of row.state?.data?.recent_jobs || []) add(row.caseId, item, String(item.status || '').toLowerCase(), stamp(item.completed_at || item.started_at || item.queued_at))
  }
  return [...found.values()].sort((a, b) => b.date - a.date).slice(0, limit)
}

export function LatestPanel({ items, loading }) {
  return (
    <Panel id="dash-latest" title="Latest evidence">
      {loading && !items.length ? <PanelSkeleton /> : items.length ? (
        <ul className="nx-feed">
          {items.map(item => {
            const failed = item.status === 'failed' || item.status === 'dead_letter'
            const done = ['completed', 'ready', 'succeeded'].includes(item.status)
            const Icon = failed ? CircleAlert : done ? CircleCheckBig : Clock3
            return (
              <li key={`${item.caseId}-${item.evidenceId || item.label}`}>
                <Link className="nx-feed__item" to={`/cases/${encodeURIComponent(item.caseId)}/evidence${item.evidenceId ? `/${encodeURIComponent(item.evidenceId)}` : ''}`}>
                  <Icon className={`nx-feed__icon nx-feed__icon--${failed ? 'bad' : done ? 'ok' : 'busy'}`} aria-hidden="true" />
                  <span className="nx-feed__text"><span className="nx-feed__name" dir="auto">{item.label}</span><span className="nx-feed__meta"><CaseLabel caseId={item.caseId} /></span></span>
                  <time className="nx-feed__time" dateTime={item.date.toISOString()} title={formatMoment(item.date)}>{relativeAge(item.date)}</time>
                </Link>
              </li>
            )
          })}
        </ul>
      ) : <Calm icon={Clock3}>Nothing yet</Calm>}
    </Panel>
  )
}

// ---------- the two states a whole page can be in ----------
// First run: nothing to show yet, so the page says what the three steps are and offers the first one.
export function FirstRun() {
  const steps = [['Create a case', 'Name the matter you are working on.'], ['Add evidence', 'Spreadsheets, documents, images, audio.'], ['Ask a question', 'Every answer shows the records behind it.']]
  return (
    <section className="nx-start3" aria-label="Start your first case">
      <ol>
        {steps.map(([title, text], index) => <li key={title}><span className="nx-start3__n" aria-hidden="true">{index + 1}</span><div><strong>{title}</strong><p>{text}</p></div></li>)}
      </ol>
      <Link className="nx-btn nx-btn--primary" to="/cases/new"><Plus aria-hidden="true" />New case</Link>
    </section>
  )
}

// The service could not be reached for any case: one message with one action, never a page of broken panels.
export function ServiceDown({ onRetry }) {
  return (
    <section className="nx-down" role="alert">
      <ServerOff aria-hidden="true" />
      <div><strong>The workspace could not read your cases.</strong><p>The service did not answer. Check that it is running, then try again.</p></div>
      <button type="button" className="nx-btn" onClick={onRetry}><RefreshCw aria-hidden="true" />Try again</button>
    </section>
  )
}
