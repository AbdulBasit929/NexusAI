import { Link } from 'react-router-dom'
import { ArrowRight, CircleAlert, CircleCheckBig, Clock3, History, MessageSquareText, Plus, RefreshCw, ServerOff } from 'lucide-react'
import { formatNumber } from '../../../lib/format.js'
import { formatRate } from '../../../lib/dashboardCharts.js'
import { relativeAge } from '../../../lib/dashboardCases.js'
import { CaseLabel } from '../../shell/CaseLabel.jsx'
import { Calm, Panel, PanelSkeleton } from '../../ui/Panel.jsx'
import { SegBar } from '../../ui/micro.jsx'
import { Sparkline } from '../../ui/Sparkline.jsx'
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

export function ResumeStrip({ entries }) {
  if (!entries?.length) return null
  return (
    <section className="nx-resume-band" aria-label="Pick up where you left off">
      <ul className="nx-resume-list">
        {entries.map(entry => (
          <li key={`${entry.caseId}-${entry.id}`}>
            <Link className="nx-resume" to={`/cases/${encodeURIComponent(entry.caseId)}/investigate?question=${encodeURIComponent(entry.query)}`}>
              <History className="nx-resume__icon" aria-hidden="true" />
              <span className="nx-resume__text">
                <span className="nx-resume__label">{entries.length === 1 || entry === entries[0] ? 'Pick up where you left off' : 'Also recent'}</span>
                <span className="nx-resume__question" dir="auto">{entry.label || entry.query}</span>
                <span className="nx-resume__meta"><CaseLabel caseId={entry.caseId} />{entry.updatedAt ? <> · <time dateTime={entry.updatedAt} title={formatMoment(new Date(entry.updatedAt))}>{relativeAge(new Date(entry.updatedAt))}</time></> : null}</span>
              </span>
              <ArrowRight className="nx-resume__go" aria-hidden="true" />
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}

// ---------- five figures, each with the one picture that explains it ----------
// A tile: its name, the figure, one small picture, one short line. Proportional figures (not tabular) because the figure stands alone; tabular digits are for columns.
function Tile({ label, figure, of, to, alert, caption, children }) {
  return (
    <Link className={`nx-tile${alert ? ' nx-tile--alert' : ''}`} to={to}>
      <span className="nx-tile__label">{label}</span>
      <span className="nx-tile__figure">{figure}{of ? <span className="nx-tile__of"> / {of}</span> : null}</span>
      <span className="nx-tile__visual">{children}</span>
      <span className="nx-tile__caption">{caption || ' '}</span>
    </Link>
  )
}

export function cleanRate(kpis) {
  const total = kpis.acceptedRows + kpis.duplicateRows + kpis.rejectedRows
  return total ? kpis.acceptedRows / total : null
}

const plural = (count, one, many = `${one}s`) => `${formatNumber(count)} ${count === 1 ? one : many}`

export function MetricStrip({ kpis, rows, states, quality, daily, loading }) {
  const wait = loading && !kpis.reported
  const v = number => (wait ? '—' : formatNumber(number))
  const mix = [
    { id: 'ok', value: rows.filter(row => row.status === 'complete').length, colour: 'var(--nx-ok)', title: 'Ready' },
    { id: 'warn', value: rows.filter(row => row.status === 'processing').length, colour: 'var(--nx-warn)', title: 'Processing' },
    { id: 'bad', value: rows.filter(row => row.status === 'attention').length, colour: 'var(--nx-danger)', title: 'Needs review' },
    { id: 'other', value: rows.filter(row => !['complete', 'processing', 'attention'].includes(row.status)).length, colour: 'var(--nx-border-strong)', title: 'Not reported yet' },
  ]
  const caseCaption = [states?.open ? `${formatNumber(states.open)} open` : '', states?.on_hold ? `${formatNumber(states.on_hold)} on hold` : '', states?.closed ? `${formatNumber(states.closed)} closed` : ''].filter(Boolean).join(' · ')
  const fileCaption = kpis.failed || kpis.processing ? [kpis.processing ? `${formatNumber(kpis.processing)} processing` : '', kpis.failed ? `${formatNumber(kpis.failed)} failed` : ''].filter(Boolean).join(' · ') : (kpis.sources ? 'All files read' : '')
  const rate = cleanRate(kpis)
  const reviewCase = rows.find(row => row.summary && (row.summary.failed || 0) + (row.summary.missingAssets || 0) > 0)
  const asked = quality.questions > 0
  return (
    <section className="nx-tiles" aria-label="Figures">
      <Tile label="Cases" figure={v(kpis.cases)} to="/cases" caption={caseCaption}><SegBar parts={mix} label="Cases by state" /></Tile>
      <Tile label="Files ready" figure={v(kpis.ready)} of={wait ? null : formatNumber(kpis.sources)} to="/cases" caption={wait ? '' : fileCaption}>
        <SegBar parts={[{ id: 'ready', value: kpis.ready, colour: 'var(--nx-ok)', title: 'Ready' }, { id: 'busy', value: kpis.processing, colour: 'var(--nx-warn)', title: 'Processing' }, { id: 'failed', value: kpis.failed, colour: 'var(--nx-danger)', title: 'Failed' }, { id: 'rest', value: Math.max(0, kpis.sources - kpis.ready - kpis.processing - kpis.failed), colour: 'var(--nx-border-strong)', title: 'Other' }]} label={`${formatNumber(kpis.ready)} of ${formatNumber(kpis.sources)} files are ready`} />
      </Tile>
      <Tile label="Records" figure={v(kpis.acceptedRows)} to="/cases" caption={rate === null ? '' : `${formatRate(rate)} of rows accepted`}>
        <SegBar parts={[{ id: 'kept', value: kpis.acceptedRows, colour: 'var(--nx-accent)', title: 'Accepted' }, { id: 'repeat', value: kpis.duplicateRows, colour: 'var(--nx-border-strong)', title: 'Repeats' }, { id: 'rejected', value: kpis.rejectedRows, colour: 'var(--nx-border)', title: 'Rejected' }]} label="Rows accepted, repeated and rejected" />
      </Tile>
      <Tile label="Answered · 7 days" figure={asked ? formatRate(quality.rate) : '—'} to="/investigate" caption={asked ? `${plural(quality.questions, 'question')}${quality.latency ? ` · median ${formatDuration(quality.latency.p50)}` : ''}` : (quality.reported ? 'No questions this week' : '')}>
        {asked ? (
          <>
            <span className="nx-tile__spark" dir="ltr" aria-hidden="true">{daily && daily.length === 7 ? <Sparkline values={daily.map(day => day.questions)} width={150} height={26} label="Questions per day" /> : <SegBar parts={[{ id: 'answered', value: quality.answered, colour: 'var(--nx-accent)', title: 'Answered' }, { id: 'declined', value: quality.declined, colour: 'var(--nx-border-strong)', title: 'Declined' }, { id: 'abstained', value: quality.abstained, colour: 'var(--nx-border)', title: 'Abstained' }]} label="" />}</span>
            <span className="nx-visually-hidden">{formatNumber(quality.answered)} answered, {formatNumber(quality.declined)} declined, {formatNumber(quality.abstained)} abstained{daily && daily.length === 7 ? `; per day, oldest first: ${daily.map(day => formatNumber(day.questions)).join(', ')}` : ''}.</span>
          </>
        ) : null}
      </Tile>
      <Tile label="Needs review" figure={v(kpis.review)} alert={kpis.review > 0} to={reviewCase ? `/cases/${encodeURIComponent(reviewCase.caseId)}/evidence` : '/cases'} caption={kpis.review ? `in ${plural(kpis.reviewCases, 'case')}` : (wait ? '' : 'Nothing to review')} />
    </section>
  )
}

// ---------- what needs you: sorted by decision, not by time ----------
// Three tiers: what needs you now (it blocks your work), what can wait (worth knowing), what you can clear right away (a question you started). Inside a tier the order is the rule printed under the list.
// Every row has one primary action, and the whole row is that action.
const TIERS = [
  { id: 'now', label: 'Needs you now', test: group => group.attention },
  { id: 'wait', label: 'Can wait', test: group => !group.attention && group.id !== 'draft' },
  { id: 'clear', label: 'Clear right away', test: group => group.id === 'draft' },
]
const PER_GROUP = 2

function ActionRow({ item, group }) {
  return (
    <li>
      <Link className="nx-act__row" to={item.to} title={item.hint || undefined}>
        <span className="nx-act__text">
          <span className={`nx-act__title${group.id === 'draft' || group.id === 'choice' ? ' nx-act__title--clamp' : ''}`} dir="auto">{item.title}</span>
          <span className="nx-act__meta">{item.caseId ? <CaseLabel caseId={item.caseId} /> : null}{item.caseId && item.detail ? ' · ' : ''}{item.detail}</span>
        </span>
        <span className="nx-act__go">{item.action}<ArrowRight aria-hidden="true" /></span>
      </Link>
    </li>
  )
}

export function NeedsPanel({ needs, loading, notesLoading }) {
  const tiers = TIERS.map(tier => ({ ...tier, groups: needs.groups.filter(tier.test) })).filter(tier => tier.groups.length)
  return (
    <Panel id="dash-attention" title="Needs you" className="nx-panel--needs" action={needs.attention ? <span className="nx-badge nx-badge--danger" title="Things that block your work">{formatNumber(needs.attention)}</span> : null}>
      {loading && !tiers.length ? <PanelSkeleton /> : null}
      {tiers.map(tier => (
        <div key={tier.id} className="nx-tier" role="group" aria-label={tier.label}>
          <p className="nx-overline">{tier.label}</p>
          {tier.groups.map(group => (
            <section key={group.id} className="nx-cause" aria-label={group.label}>
              <h3 className="nx-cause__head"><span className={`nx-dot nx-dot--${group.tone === 'muted' ? '' : group.tone}`} aria-hidden="true" />{group.label}<span className="nx-cause__count nx-tabular">{formatNumber(group.count)}</span></h3>
              <ul className="nx-act">{group.items.slice(0, PER_GROUP).map(item => <ActionRow key={item.id} item={item} group={group} />)}</ul>
              {group.count > Math.min(group.items.length, PER_GROUP) ? <p className="nx-cause__more">{formatNumber(group.count - Math.min(group.items.length, PER_GROUP))} more{group.items.length < group.count ? ' (not all are named by the service)' : ''}</p> : null}
            </section>
          ))}
        </div>
      ))}
      {notesLoading && !needs.groups.some(group => group.id === 'notes') ? <div className="nx-tier"><p className="nx-overline">Can wait</p><PanelSkeleton rows={2} /></div> : null}
      {!tiers.length && !loading && !notesLoading ? <Calm icon={CircleCheckBig} tone="ok"><strong>All clear</strong></Calm> : null}
      <p className="nx-rule">Ordered by what blocks your work first. <Link to="/activity">All activity</Link></p>
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
