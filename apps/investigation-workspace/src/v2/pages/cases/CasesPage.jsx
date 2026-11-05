import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { LayoutGrid, Plus, RefreshCw, Search, SearchX, Table2 } from 'lucide-react'
import { configuredCaseIds } from '../../../lib/apiClient.js'
import { dashboardNextAction, latestEvidenceActivity, relativeAge } from '../../../lib/dashboardCases.js'
import { formatNumber } from '../../../lib/format.js'
import { summariseCase, useConfiguredCaseOverviews } from '../../../lib/useCaseOverview.js'
import { useCaseRecords } from '../../../lib/useCaseRecords.js'
import { AppShell } from '../../shell/AppShell.jsx'
import { CaseLabel } from '../../shell/CaseLabel.jsx'
import { humanizeCaseId } from '../../lib/caseLabel.js'
import { familyName } from '../dashboard/model.js'
import { CasesPanel, SourcesCell, STATUS, WEIGHT } from '../dashboard/panels.jsx'

const FILTERS = [['all', 'All'], ['attention', 'Needs review'], ['processing', 'Processing'], ['complete', 'Ready'], ['not-processed', 'No evidence'], ['unavailable', 'Unavailable']]
const SORTS = [['activity', 'Latest activity'], ['name', 'Name'], ['sources', 'Sources'], ['records', 'Records'], ['status', 'Needs you first']]

export function rowState(state) {
  if (state?.error?.status === 403) return 'forbidden'
  if (state?.error) return 'unavailable'
  if (!state?.data) return 'loading'
  return summariseCase(state.data).processingState
}

const matches = (row, filter) => filter === 'all' || (filter === 'unavailable' ? row.status === 'unavailable' || row.status === 'forbidden' : row.status === filter)

// The order the analyst chose. A case whose status could not be read goes last, never first as if it were empty.
export function sortRows(rows, key, names) {
  const value = row => ({
    activity: row.activity?.date?.getTime() ?? null,
    name: (names[row.caseId] || row.caseId).toLocaleLowerCase(),
    sources: row.summary?.total ?? null,
    records: row.summary?.acceptedRows ?? null,
    status: WEIGHT[row.status] ?? 9,
  })[key]
  return rows.slice().sort((left, right) => {
    const a = value(left); const b = value(right)
    if (a === null && b !== null) return 1
    if (b === null && a !== null) return -1
    const compared = typeof a === 'string' ? a.localeCompare(b, undefined, { numeric: true }) : key === 'status' ? a - b : b - a
    return compared || left.index - right.index
  })
}

function CaseCard({ row }) {
  const { caseId, summary, status, activity, state } = row
  const meta = STATUS[status] || STATUS.loading
  const next = summary ? dashboardNextAction(summary, caseId) : null
  const review = summary ? (summary.failed || 0) + summary.missingAssets : 0
  const held = (summary?.families || []).filter(family => Number(family.accepted_rows) > 0).sort((a, b) => Number(b.accepted_rows) - Number(a.accepted_rows))
  return (
    <li className="nx-case">
      <div className="nx-case__top">
        <span className={`nx-badge${meta.tone ? ` nx-badge--${meta.tone}` : ''}`}>{meta.label}</span>
        {activity ? <time className="nx-case__time" dateTime={activity.date.toISOString()}>{relativeAge(activity.date)}</time> : null}
      </div>
      <h3 className="nx-case__title">{summary ? <Link to={`/cases/${encodeURIComponent(caseId)}/overview`}><CaseLabel caseId={caseId} /></Link> : <CaseLabel caseId={caseId} />}</h3>
      <p className="nx-case-id nx-mono" dir="ltr">{caseId}</p>
      {summary ? (
        <>
          <p className="nx-case__ready"><SourcesCell summary={summary} /><span className="nx-secondary"> sources ready</span></p>
          <dl className="nx-case__figures">
            <div><dt>Sources</dt><dd className="nx-tabular">{formatNumber(summary.total)}</dd></div>
            <div><dt>Records</dt><dd className="nx-tabular">{formatNumber(summary.acceptedRows)}</dd></div>
            <div className={review ? 'is-alert' : undefined}><dt>To review</dt><dd className="nx-tabular">{review ? formatNumber(review) : 'None'}</dd></div>
          </dl>
          <p className="nx-holds">{held.length ? held.slice(0, 3).map(family => <span key={family.record_type} className="nx-badge">{familyName(family.record_type)}</span>) : <span className="nx-muted">No structured records yet</span>}{held.length > 3 ? <span className="nx-badge">+{held.length - 3}</span> : null}</p>
        </>
      ) : <p className="nx-muted" role={state?.loading ? 'status' : undefined}>{state?.loading ? 'Reading the status…' : status === 'forbidden' ? 'Outside your access.' : 'The status could not be read.'}</p>}
      <div className="nx-case__actions">
        {next ? <Link className={`nx-btn${status === 'attention' ? '' : ' nx-btn--primary'}`} to={next.to}>{next.label}</Link> : null}
        {summary ? <Link className="nx-btn nx-btn--ghost" to={`/cases/${encodeURIComponent(caseId)}/overview`}>Overview</Link> : null}
      </div>
    </li>
  )
}

export default function CasesPage() {
  const cases = configuredCaseIds()
  const { states, reload } = useConfiguredCaseOverviews(cases)
  const { records } = useCaseRecords()
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState('all')
  const [sort, setSort] = useState('activity')
  const [view, setView] = useState(() => { try { return globalThis.localStorage?.getItem('nexusai.cases.view') === 'table' ? 'table' : 'cards' } catch { return 'cards' } })
  const chooseView = next => { setView(next); try { globalThis.localStorage?.setItem('nexusai.cases.view', next) } catch { /* the choice is just not remembered */ } }
  useEffect(() => { globalThis.document.title = 'Cases · NexusAI' }, [])

  const names = useMemo(() => Object.fromEntries(cases.map(id => [id, records[id]?.display_name || humanizeCaseId(id).title || id])), [cases, records])
  const rows = useMemo(() => cases.map((caseId, index) => {
    const state = states[caseId]
    return { caseId, index, state, status: rowState(state), summary: state?.data ? summariseCase(state.data) : null, activity: state?.data ? latestEvidenceActivity(state.data) : null }
  }), [cases, states])
  const counts = useMemo(() => Object.fromEntries(FILTERS.map(([id]) => [id, rows.filter(row => matches(row, id)).length])), [rows])
  const visible = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase()
    return sortRows(rows.filter(row => matches(row, filter)).filter(row => !needle || `${names[row.caseId]} ${row.caseId}`.toLocaleLowerCase().includes(needle)), sort, names)
  }, [rows, filter, query, sort, names])
  const refreshing = rows.some(row => row.state?.loading || row.state?.refreshing)
  const processing = rows.some(row => row.status === 'processing')
  useEffect(() => {
    if (!processing) return undefined
    const timer = globalThis.setTimeout(reload, 15_000)
    return () => globalThis.clearTimeout(timer)
  }, [processing, reload, states])

  return (
    <AppShell>
      <main id="workspace-main" className="nx-ui nx-page nx-cases" tabIndex={-1}>
        <header className="nx-page-head">
          <div className="nx-page-head__text">
            <h1>Cases</h1>
            <p>{formatNumber(cases.length)} {cases.length === 1 ? 'case' : 'cases'}: {formatNumber(counts.complete)} ready, {formatNumber(counts.attention)} {counts.attention === 1 ? 'needs' : 'need'} review, {formatNumber(counts.processing)} processing.</p>
          </div>
          <div className="nx-page-head__actions">
            <button type="button" className="nx-btn" onClick={reload} disabled={refreshing}><RefreshCw aria-hidden="true" className={refreshing ? 'is-spinning' : undefined} />{refreshing ? 'Checking…' : 'Refresh'}</button>
            <Link className="nx-btn nx-btn--primary" to="/cases/new"><Plus aria-hidden="true" />New case</Link>
          </div>
        </header>

        {cases.length ? (
          <>
            <div className="nx-cases__bar">
              <form role="search" className="nx-cases__search" onSubmit={event => event.preventDefault()}>
                <Search aria-hidden="true" /><label className="nx-visually-hidden" htmlFor="nx-case-search">Search cases</label>
                <input id="nx-case-search" type="search" className="nx-input" value={query} onChange={event => setQuery(event.target.value)} placeholder="Search by name or identifier" />
              </form>
              <div className="nx-cases__filters" role="group" aria-label="Show cases that are">
                {FILTERS.map(([id, label]) => <button key={id} type="button" className="nx-chip" aria-pressed={filter === id} onClick={() => setFilter(id)}>{label}<span className="nx-chip__count nx-tabular">{formatNumber(counts[id])}</span></button>)}
              </div>
              <label className="nx-cases__sort"><span className="nx-visually-hidden">Order of the cases</span>
                <select className="nx-select" value={sort} onChange={event => setSort(event.target.value)}>{SORTS.map(([id, label]) => <option key={id} value={id}>{label}</option>)}</select>
              </label>
              <div className="nx-segmented" role="group" aria-label="View">
                <button type="button" aria-pressed={view === 'cards'} onClick={() => chooseView('cards')}><LayoutGrid aria-hidden="true" />Cards</button>
                <button type="button" aria-pressed={view === 'table'} onClick={() => chooseView('table')}><Table2 aria-hidden="true" />Table</button>
              </div>
            </div>
            {visible.length ? (
              view === 'cards'
                ? <ul className="nx-cases__grid" aria-label={`${formatNumber(visible.length)} of ${formatNumber(cases.length)} cases`}>{visible.map(row => <CaseCard key={row.caseId} row={row} />)}</ul>
                : <CasesPanel rows={visible} limit={500} headless ordered />
            ) : (
              <div className="nx-empty nx-card"><span className="nx-empty__icon"><SearchX aria-hidden="true" /></span><h3>No case matches</h3><p>Try another word, or show every case.</p><div className="nx-empty__actions"><button type="button" className="nx-btn" onClick={() => { setQuery(''); setFilter('all') }}>Show every case</button></div></div>
            )}
            <p className="nx-dash__scope">A name, an owner, a classification and a priority are not recorded for a case yet; a case is named by the analyst who opens it.</p>
          </>
        ) : (
          <div className="nx-empty nx-card"><h3>No case is set up yet</h3><p>Create the first case and add its evidence to begin.</p><div className="nx-empty__actions"><Link className="nx-btn nx-btn--primary" to="/cases/new">New case</Link></div></div>
        )}
      </main>
    </AppShell>
  )
}
