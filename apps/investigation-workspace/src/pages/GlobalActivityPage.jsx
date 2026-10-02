import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { CircleAlert, Download, RotateCcw, Search } from 'lucide-react'
import { EmptyState, RouteState } from '../components/AnalystComponents.jsx'
import { AppShell } from '../components/CaseShell.jsx'
import { configuredCaseIds } from '../lib/apiClient.js'
import { activitiesCsv, asTimestamp, buildWorkspaceActivities, caseSummaries, countOutcomes, dayCounts, groupByDay, needsLook, reviewedKey } from '../lib/activityFeed.js'
import { useReviewed } from '../lib/activityReviewed.js'
import { relativeAge } from '../lib/dashboardCases.js'
import { formatNumber } from '../lib/format.js'
import { useAllSessionActivity } from '../lib/sessionActivity.js'
import { useConfiguredCaseOverviews } from '../lib/useCaseOverview.js'
import { useQuestionHistoryAcross } from '../lib/workspaceState.js'
import { CoverageCard, Detail, Feed, OutcomeCard, RhythmCard } from './activity/ActivityParts.jsx'
import { WorkspaceRail } from './activity/WorkspaceRail.jsx'

function download(name, text) {
  const url = URL.createObjectURL(new Blob([text], { type: 'text/csv' }))
  const link = globalThis.document.createElement('a')
  link.href = url; link.download = name
  globalThis.document.body.append(link); link.click(); link.remove()
  globalThis.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

// The workspace inbox and journal: what has happened across every case, with the things that need a look first. A rail
// holds the views (Needs attention, All, Questions, Evidence) and the cases; the journal groups entries by day; a reading
// pane shows the chosen one and lets the analyst mark it reviewed (kept in this browser, like a mail client's done). J and K
// move, E marks reviewed. It is a working record from this browser and each case's recent status, not an audit history.
export default function GlobalActivityPage() {
  const caseIds = configuredCaseIds()
  const sessionActivities = useAllSessionActivity()
  const questionHistory = useQuestionHistoryAcross(caseIds)
  const { states, reload } = useConfiguredCaseOverviews(caseIds)
  const { reviewed, toggle } = useReviewed()
  const [viewChoice, setView] = useState(null)
  const [caseFilter, setCaseFilter] = useState('')
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState('')

  const overviews = useMemo(() => Object.fromEntries(caseIds.map(id => [id, states[id]?.data || null])), [states]) // eslint-disable-line react-hooks/exhaustive-deps
  const items = useMemo(() => buildWorkspaceActivities({ sessionActivities: sessionActivities.filter(item => caseIds.includes(item.caseId)), questionHistory, overviews }), [sessionActivities, questionHistory, overviews]) // eslint-disable-line react-hooks/exhaustive-deps
  const open = items.filter(item => needsLook(item) && !reviewed.has(reviewedKey(item)))
  const view = viewChoice ?? 'all'
  // The first time entries arrive, open on the queue when something needs a look; after that the choice is the analyst's.
  useEffect(() => { if (viewChoice === null && items.length) setView(open.length ? 'attention' : 'all') }, [viewChoice, items.length]) // eslint-disable-line react-hooks/exhaustive-deps
  const unreadable = caseIds.filter(id => states[id]?.error)
  const loading = caseIds.some(id => states[id]?.loading) && !items.length

  const scoped = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase()
    return items.filter(item => {
      const text = [item.title, item.answer, item.evidenceId, item.family, item.caseId].filter(Boolean).join(' ').toLocaleLowerCase()
      return (!caseFilter || item.caseId === caseFilter) && (!needle || text.includes(needle))
    })
  }, [items, caseFilter, query])
  const visible = useMemo(() => {
    if (view === 'attention') return scoped.filter(item => needsLook(item) && !reviewed.has(reviewedKey(item)))
    if (view === 'questions') return scoped.filter(item => item.sourceKind === 'questions')
    if (view === 'evidence') return scoped.filter(item => item.sourceKind === 'evidence')
    return scoped
  }, [scoped, view, reviewed])
  const groups = useMemo(() => groupByDay(visible), [visible])
  const current = visible.find(item => reviewedKey(item) === selected) || visible[0]
  const counts = { attention: open.length, all: items.length, questions: items.filter(item => item.sourceKind === 'questions').length, evidence: items.filter(item => item.sourceKind === 'evidence').length }
  const cases = useMemo(() => caseSummaries(items, caseIds, reviewed), [items, reviewed]) // eslint-disable-line react-hooks/exhaustive-deps
  const days = useMemo(() => dayCounts(items), [items])
  const outcomes = useMemo(() => countOutcomes(scoped), [scoped])
  const latestStamp = Math.max(0, ...items.map(item => asTimestamp(item.recordedAt)))
  const hasFilters = Boolean(query || caseFilter)

  function move(step) {
    const index = Math.max(0, visible.findIndex(item => reviewedKey(item) === reviewedKey(current || {})))
    const next = visible[index + step]
    if (next) setSelected(reviewedKey(next))
  }
  function onKey(event) {
    if (['INPUT', 'TEXTAREA', 'SELECT'].includes(event.target.tagName) || event.metaKey || event.ctrlKey || event.altKey) return
    const key = event.key.toLowerCase()
    if (key === 'j') { event.preventDefault(); move(1) }
    else if (key === 'k') { event.preventDefault(); move(-1) }
    else if (key === 'e' && current && needsLook(current)) { event.preventDefault(); toggle(reviewedKey(current)); move(1) }
  }
  const clear = () => { setQuery(''); setCaseFilter('') }

  return (
    <AppShell>
      <main id="workspace-main" className="activity-page global-activity-page ac" tabIndex={-1} onKeyDown={onKey}>
        <header className="ac-head">
          <div>
            <h1>Activity</h1>
            <p>What has happened across your cases, with the things that need a look first. Everything opens in its own case.</p>
          </div>
          <dl className="ac-facts-row" aria-label="Activity summary">
            <div><dt>Cases</dt><dd>{formatNumber(caseIds.length)}</dd></div>
            <div><dt>Entries</dt><dd>{formatNumber(items.length)}</dd></div>
            <div><dt>Need a look</dt><dd>{formatNumber(open.length)}</dd></div>
            <div><dt>Latest</dt><dd>{latestStamp ? relativeAge(new Date(latestStamp)) : '—'}</dd></div>
          </dl>
        </header>

        {caseIds.length === 0 ? (
          <>
            <EmptyState kind="not-processed" label="No case working records are available" description="Add the first evidence file to establish a collection-backed case workspace.">
              <Link to="/cases/new">Add evidence</Link>
            </EmptyState>
            <div className="ac-below ac-below--one"><CoverageCard /></div>
          </>
        ) : null}
        {loading ? <RouteState state="loading" label="Reading each case" description="Recent evidence status is being read for every case." /> : null}
        {unreadable.length ? <p className="ac-notice" role="status"><CircleAlert aria-hidden="true" />{unreadable.length === caseIds.length ? 'No case could report its recent evidence status.' : `${formatNumber(unreadable.length)} of ${formatNumber(caseIds.length)} cases could not report their recent evidence status, so only their questions appear.`}<button type="button" onClick={reload}>Try again</button></p> : null}

        {caseIds.length > 0 && !loading ? (
          <>
            <div className="ac-bar" role="toolbar" aria-label="Find activity">
              <label className="ac-search"><Search aria-hidden="true" /><span className="visually-hidden">Search activity</span><input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Question, filename, case or identifier" /></label>
              <span className="ac-keys" aria-hidden="true"><kbd>J</kbd><kbd>K</kbd> move <kbd>E</kbd> mark reviewed</span>
              <span className="ac-bar__space" />
              {hasFilters ? <button type="button" className="ac-btn" onClick={clear}><RotateCcw aria-hidden="true" />Clear filters</button> : null}
              <button type="button" className="ac-btn" onClick={() => download('workspace-activity.csv', activitiesCsv(visible))}><Download aria-hidden="true" />Export CSV</button>
            </div>

            <div className="ac-main ac-main--ws">
              <WorkspaceRail view={view} onView={setView} counts={counts} cases={cases} caseFilter={caseFilter} onCase={setCaseFilter} />
              {visible.length ? (
                <>
                  <Feed groups={groups} selectedId={current ? reviewedKey(current) : ''} onSelect={setSelected} total={items.length} shown={visible.length} showCase reviewed={reviewed} idOf={reviewedKey} />
                  {current ? <Detail item={current} caseId={current.caseId} showCase isReviewed={reviewed.has(reviewedKey(current))} onReview={() => toggle(reviewedKey(current))} /> : null}
                </>
              ) : (
                <section className="ac-feed ac-feed--empty ac-span2" aria-live="polite">
                  <h2>{view === 'attention' && !hasFilters ? 'You are all caught up' : 'Nothing matches'}</h2>
                  <p>{view === 'attention' && !hasFilters ? 'No failed, limited, waiting or missing item is left to review in the entries the service reports.' : 'No entry fits this view and these filters. The record was not widened.'}</p>
                  {view === 'attention' ? <button type="button" className="ac-btn" onClick={() => setView('all')}>See all activity</button> : <button type="button" className="ac-btn" onClick={() => { clear(); setView('all') }}>Show all activity</button>}
                </section>
              )}
            </div>

            <div className="ac-below">
              <OutcomeCard counts={outcomes} total={scoped.length} />
              <RhythmCard days={days} />
              <CoverageCard />
            </div>
          </>
        ) : null}
      </main>
    </AppShell>
  )
}
