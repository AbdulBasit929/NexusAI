import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { EmptyState, LanguageText, RouteState } from '../components/AnalystComponents.jsx'
import { AppShell } from '../components/CaseShell.jsx'
import { ProportionBar, StackedBar } from '../components/DataVisualizations.jsx'
import { PageHeader } from '../components/PageHeader.jsx'
import { configuredCaseIds } from '../lib/apiClient.js'
import { formatNumber } from '../lib/format.js'
import { curatedFamilyLabel } from '../lib/semanticCatalog.js'
import { summariseCase, useConfiguredCaseOverviews } from '../lib/useCaseOverview.js'

const FILTERS = [
  ['all', 'All'],
  ['attention', 'Review'],
  ['processing', 'Processing'],
  ['complete', 'Ready'],
  ['not-processed', 'No evidence'],
  ['unavailable', 'Unavailable'],
]

const STATE_LABEL = {
  complete: 'Ready',
  processing: 'Processing',
  attention: 'Needs review',
  'not-processed': 'No evidence',
  forbidden: 'Access restricted',
  unavailable: 'Unavailable',
  loading: 'Checking',
}

function StateIcon({ state }) {
  if (state === 'complete') return <svg viewBox="0 0 20 20" aria-hidden="true"><path d="m5 10.4 3.1 3.1L15.4 6" /></svg>
  if (state === 'attention' || state === 'forbidden' || state === 'unavailable') return <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M10 3 17 16H3L10 3Z" /><path d="M10 7.5v4M10 14.2h.01" /></svg>
  if (state === 'processing' || state === 'loading') return <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M10 3a7 7 0 1 1-6.4 4.2" /><path d="M3 3v4.3h4.3" /></svg>
  return <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M3.5 6.5h13v10h-13zM6 6.5V4h5l2 2.5" /></svg>
}

function SearchIcon() {
  return <svg viewBox="0 0 20 20" aria-hidden="true"><circle cx="8.5" cy="8.5" r="5" /><path d="m12.3 12.3 4.2 4.2" /></svg>
}

function RefreshIcon() {
  return <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M15.7 7A6.2 6.2 0 1 0 16 12" /><path d="M16 3v4h-4" /></svg>
}

function ArrowIcon() {
  return <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M4 10h11M11 6l4 4-4 4" /></svg>
}

function rowState(state) {
  if (state?.error?.status === 403) return 'forbidden'
  if (state?.error) return 'unavailable'
  if (!state?.data) return 'loading'
  return summariseCase(state.data).processingState
}

function nextAction(summary, caseId) {
  const encoded = encodeURIComponent(caseId)
  if (summary.processingState === 'attention') return { label: 'Review evidence', to: `/cases/${encoded}/evidence` }
  if (summary.processingState === 'processing') return { label: 'View processing', to: `/cases/${encoded}/evidence` }
  if (summary.processingState === 'complete') return { label: 'Investigate', to: `/cases/${encoded}/investigate` }
  return { label: 'Add evidence', to: `/cases/${encoded}/evidence` }
}

function parseTimestamp(value) {
  const date = value ? new Date(value) : null
  return date && !Number.isNaN(date.getTime()) ? date : null
}

function latestEvidenceActivity(data) {
  const candidates = [
    ...(data?.recent_evidence || []).map(item => ({ source: item.source_file, date: parseTimestamp(item.updated_at || item.created_at) })),
    ...(data?.recent_jobs || []).map(item => ({ source: item.source_file, date: parseTimestamp(item.completed_at || item.started_at || item.queued_at) })),
  ].filter(item => item.date)
  candidates.sort((left, right) => right.date.getTime() - left.date.getTime())
  return candidates[0] || null
}

function formatTimestamp(date) {
  return `${new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC' }).format(date)} UTC`
}

function sortableValue(row, key) {
  if (key === 'caseId') return row.caseId
  if (key === 'activity') return row.activity?.date.getTime() ?? null
  if (key === 'evidence') return row.summary?.total ?? null
  return row.summary?.acceptedRows ?? null
}

function compareRows(left, right, sort) {
  const leftValue = sortableValue(left, sort.key)
  const rightValue = sortableValue(right, sort.key)
  if (leftValue === null && rightValue !== null) return 1
  if (rightValue === null && leftValue !== null) return -1
  const comparison = typeof leftValue === 'string'
    ? leftValue.localeCompare(rightValue, undefined, { numeric: true, sensitivity: 'base' })
    : leftValue - rightValue
  return (sort.direction === 'ascending' ? comparison : -comparison) || left.index - right.index
}

function rowMatchesFilter(row, filter) {
  if (filter === 'all') return true
  if (filter === 'unavailable') return row.status === 'unavailable' || row.status === 'forbidden'
  return row.status === filter
}

function SortButton({ column, label, sort, onSort }) {
  const active = sort.key === column
  return (
    <button type="button" className="case-directory__sort-button" onClick={() => onSort(column)}>
      <span>{label}</span>
      <span aria-hidden="true" className="case-directory__sort-icon">{active ? (sort.direction === 'ascending' ? '↑' : '↓') : '↕'}</span>
      <span className="visually-hidden">{active ? `, sorted ${sort.direction}` : ', not sorted'}</span>
    </button>
  )
}

function moveBetweenCaseRows(event) {
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
  const links = [...event.currentTarget.closest('tbody').querySelectorAll('.case-directory__case-link')]
  const index = links.indexOf(event.currentTarget)
  const target = event.key === 'Home'
    ? links[0]
    : event.key === 'End'
      ? links.at(-1)
      : links[index + (event.key === 'ArrowDown' ? 1 : -1)]
  if (!target) return
  event.preventDefault()
  target.focus()
}

function CaseDirectoryRow({ row }) {
  const { caseId, state, status, summary, activity } = row
  const action = summary ? nextAction(summary, caseId) : null
  const families = (summary?.families || [])
    .map(family => ({ ...family, value: Number(family.accepted_rows || 0) }))
    .filter(family => family.value > 0)
  const visibleFamilies = families.slice(0, 2)
  const familyOverflow = Math.max(0, families.length - visibleFamilies.length)
  const overview = `/cases/${encodeURIComponent(caseId)}/overview`

  return (
    <tr className={`case-directory__row case-directory__row--${status}`}>
      <th scope="row" data-label="Case">
        {summary ? (
          <Link className="case-directory__case-link" to={overview} onKeyDown={moveBetweenCaseRows} aria-keyshortcuts="ArrowUp ArrowDown Home End">
            <LanguageText as="bdi" identifier>{caseId}</LanguageText>
          </Link>
        ) : <LanguageText as="bdi" className="case-directory__case-id" identifier>{caseId}</LanguageText>}
        <span className={`case-directory__state case-directory__state--${status}`}>
          <StateIcon state={status} /><span>{STATE_LABEL[status]}</span>
        </span>
        {state?.refreshing ? <small role="status">Refreshing…</small> : null}
      </th>
      <td data-label="Evidence">
        {state?.loading ? <span className="case-directory__not-reported" role="status">Reading status…</span> : null}
        {state?.error ? <span className="case-directory__not-reported">{status === 'forbidden' ? 'Outside your access scope.' : 'Status could not be read.'}</span> : null}
        {summary ? (
          <div className="case-directory__readiness">
            <div><strong>{formatNumber(summary.total)}</strong><span> source{summary.total === 1 ? '' : 's'}</span></div>
            <ProportionBar total={summary.total} ready={summary.ready} processing={summary.inFlight} failed={summary.failed || 0} label={`${caseId} evidence readiness`} />
            <span>{formatNumber(summary.ready)} ready · {formatNumber(summary.inFlight)} active{summary.failed === null ? ' · failures not reported' : ` · ${formatNumber(summary.failed)} failed`}</span>
          </div>
        ) : null}
      </td>
      <td data-label="Evidence families">
        {summary ? (
          <div className="case-directory__coverage">
            <div><strong>{formatNumber(summary.acceptedRows)}</strong><span> accepted rows</span></div>
            {families.length ? (
              <>
                <StackedBar items={families.map(family => ({ id: family.record_type, label: curatedFamilyLabel(family.record_type), value: family.value }))} label={`${caseId} accepted rows by evidence family`} />
                <ul aria-label="Evidence families">
                  {visibleFamilies.map(family => <li key={family.record_type}><span>{curatedFamilyLabel(family.record_type)}</span><strong>{formatNumber(family.value)}</strong></li>)}
                  {familyOverflow ? <li><span>More</span><strong>+{familyOverflow}</strong></li> : null}
                </ul>
              </>
            ) : <span className="case-directory__not-reported">No structured family reported</span>}
          </div>
        ) : <span className="case-directory__not-reported">Not available</span>}
      </td>
      <td data-label="Evidence activity">
        {activity ? (
          <div className="case-directory__activity">
            <time dateTime={activity.date.toISOString()}>{formatTimestamp(activity.date)}</time>
            {activity.source ? <LanguageText as="span">{activity.source}</LanguageText> : null}
          </div>
        ) : <span className="case-directory__not-reported">Not reported</span>}
      </td>
      <td data-label="Next action">
        <div className="case-directory__actions">
          {action ? <Link className="case-directory__primary" to={action.to}>{action.label}<ArrowIcon /></Link> : <span className="case-directory__not-reported">Unavailable</span>}
        </div>
      </td>
    </tr>
  )
}

export default function CasesPage() {
  const cases = configuredCaseIds()
  const { states, reload } = useConfiguredCaseOverviews(cases)
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState('all')
  const [sort, setSort] = useState({ key: 'activity', direction: 'descending' })

  const rows = useMemo(() => cases.map((caseId, index) => {
    const state = states[caseId]
    return {
      caseId,
      index,
      state,
      status: rowState(state),
      summary: state?.data ? summariseCase(state.data) : null,
      activity: state?.data ? latestEvidenceActivity(state.data) : null,
    }
  }), [cases, states])
  const counts = useMemo(() => Object.fromEntries(FILTERS.map(([id]) => [id, rows.filter(row => rowMatchesFilter(row, id)).length])), [rows])
  const visible = useMemo(() => rows
    .filter(row => rowMatchesFilter(row, filter))
    .filter(row => row.caseId.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()))
    .sort((left, right) => compareRows(left, right, sort)), [filter, query, rows, sort])
  const failures = rows.filter(row => row.state?.error)
  const resolved = rows.filter(row => row.state?.data)
  const isRefreshing = rows.some(row => row.state?.loading || row.state?.refreshing)
  const hasProcessing = rows.some(row => row.status === 'processing')

  useEffect(() => {
    if (!hasProcessing) return undefined
    const timer = globalThis.setTimeout(reload, 15_000)
    return () => globalThis.clearTimeout(timer)
  }, [hasProcessing, reload, states])

  function clearView() {
    setQuery('')
    setFilter('all')
  }

  function changeSort(key) {
    setSort(current => current.key === key
      ? { key, direction: current.direction === 'ascending' ? 'descending' : 'ascending' }
      : { key, direction: key === 'caseId' ? 'ascending' : 'descending' })
  }

  return (
    <AppShell>
      <main id="workspace-main" className="catalog-page cases-page" tabIndex={-1}>
        <PageHeader eyebrow="Workspace" title="Cases" description="Find a collection-backed case and continue with its evidence." breadcrumbs={[{ label: 'Workspace', to: '/' }, { label: 'Cases' }]} actions={<Link className="page-header__cta" to="/cases/new">Add evidence</Link>} />

        {cases.length ? (
          <section className="case-directory" aria-labelledby="case-directory-title">
            <header className="case-directory__header">
              <div><p className="eyebrow">Configured collections</p><h2 id="case-directory-title">Case directory</h2></div>
              <div className="case-directory__header-actions">
                <span aria-live="polite"><strong>{formatNumber(visible.length)}</strong> of {formatNumber(cases.length)}</span>
                <button type="button" className="case-directory__refresh" onClick={reload} disabled={isRefreshing}><RefreshIcon /><span>{isRefreshing ? 'Checking…' : 'Refresh'}</span></button>
              </div>
            </header>

            {failures.length ? (
              <RouteState
                state={resolved.length ? 'partial' : failures.every(row => row.state.error.status === 403) ? 'forbidden' : 'unavailable'}
                label={resolved.length ? 'Some statuses are unavailable' : failures.every(row => row.state.error.status === 403) ? 'Statuses are outside your access scope' : 'Statuses could not be read'}
                description="Unavailable collections remain listed and are not counted as zero."
                failedScope={`${failures.length} of ${cases.length} collection${failures.length === 1 ? '' : 's'}`}
              />
            ) : null}

            <div className="case-directory__toolbar">
              <form role="search" onSubmit={event => event.preventDefault()}>
                <label className="visually-hidden" htmlFor="case-directory-search">Search cases</label>
                <span className="case-directory__search-field"><SearchIcon /><input id="case-directory-search" type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Search case ID" /></span>
              </form>
              <div className="case-directory__filters" role="group" aria-label="Filter cases by evidence readiness">
                {FILTERS.map(([id, label]) => <button key={id} type="button" aria-pressed={filter === id} onClick={() => setFilter(id)}><span>{label}</span><strong>{formatNumber(counts[id])}</strong></button>)}
              </div>
              {(query || filter !== 'all') ? <button type="button" className="case-directory__clear" onClick={clearView}>Clear</button> : null}
            </div>

            {visible.length ? (
              <div className="case-directory__table-wrap" tabIndex={0} aria-label="Scrollable case directory">
                <div className="case-directory__mobile-sort" aria-label="Sort cases">
                  <span>Sort</span>
                  <SortButton column="activity" label="Activity" sort={sort} onSort={changeSort} />
                  <SortButton column="caseId" label="Case" sort={sort} onSort={changeSort} />
                  <SortButton column="evidence" label="Evidence" sort={sort} onSort={changeSort} />
                  <SortButton column="rows" label="Rows" sort={sort} onSort={changeSort} />
                </div>
                <table className="case-directory__table">
                  <caption className="visually-hidden">{formatNumber(visible.length)} of {formatNumber(cases.length)} configured collections. Evidence activity is lifecycle metadata, not source-event time.</caption>
                  <thead><tr>
                    <th scope="col" aria-sort={sort.key === 'caseId' ? sort.direction : undefined}><SortButton column="caseId" label="Case" sort={sort} onSort={changeSort} /></th>
                    <th scope="col" aria-sort={sort.key === 'evidence' ? sort.direction : undefined}><SortButton column="evidence" label="Evidence" sort={sort} onSort={changeSort} /></th>
                    <th scope="col" aria-sort={sort.key === 'rows' ? sort.direction : undefined}><SortButton column="rows" label="Evidence families" sort={sort} onSort={changeSort} /></th>
                    <th scope="col" aria-sort={sort.key === 'activity' ? sort.direction : undefined}><SortButton column="activity" label="Evidence activity" sort={sort} onSort={changeSort} /></th>
                    <th scope="col"><span className="visually-hidden">Next action</span></th>
                  </tr></thead>
                  <tbody>{visible.map(row => <CaseDirectoryRow key={row.caseId} row={row} />)}</tbody>
                </table>
                <p className="visually-hidden" aria-live="polite">Sorted by {sort.key === 'caseId' ? 'case ID' : sort.key === 'evidence' ? 'evidence count' : sort.key === 'rows' ? 'accepted rows' : 'latest evidence activity'}, {sort.direction}.</p>
              </div>
            ) : <EmptyState kind="no-match" label="No cases match" description="Try another search or readiness filter."><button type="button" onClick={clearView}>Clear view</button></EmptyState>}

            <details className="case-directory__boundary"><summary>Directory scope</summary><p>Names, owners, classification and priority are absent because the service does not expose them.</p></details>
          </section>
        ) : <EmptyState kind="not-processed" label="No collections are configured" description="Add the first evidence file to establish a case workspace."><Link to="/cases/new">Add evidence</Link></EmptyState>}
      </main>
    </AppShell>
  )
}
