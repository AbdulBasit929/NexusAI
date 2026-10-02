import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Download, RotateCcw, Search } from 'lucide-react'
import { CaseShell } from '../components/CaseShell.jsx'
import { useSessionActivity } from '../lib/sessionActivity.js'
import { useQuestionHistory } from '../lib/workspaceState.js'
import { getCaseOverview } from '../lib/apiClient.js'
import { EmptyState, RouteState } from '../components/AnalystComponents.jsx'
import { activitiesCsv, asTimestamp, buildActivities, countOutcomes, groupByDay, outcomeOf, seriesByDay } from '../lib/activityFeed.js'
import { relativeAge } from '../lib/dashboardCases.js'
import { formatNumber } from '../lib/format.js'
import { CoverageCard, Detail, Feed, OutcomeCard, Pulse } from './activity/ActivityParts.jsx'

const SOURCES = [{ id: 'all', label: 'All' }, { id: 'questions', label: 'Questions' }, { id: 'evidence', label: 'Evidence' }]

function download(name, text) {
  const url = URL.createObjectURL(new Blob([text], { type: 'text/csv' }))
  const link = globalThis.document.createElement('a')
  link.href = url; link.download = name
  globalThis.document.body.append(link); link.click(); link.remove()
  globalThis.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

// The case journal: one feed of questions and evidence updates grouped by day, a reading pane for the chosen entry, and
// three equal cards under it (outcomes, the last two weeks, and what this record does and does not cover). It is a working
// record built from this browser and the recent collection status, not an audit history, and says so.
export default function ActivityPage() {
  const { id: caseId = '' } = useParams()
  const sessionActivities = useSessionActivity(caseId)
  const questionHistory = useQuestionHistory(caseId)
  const [overview, setOverview] = useState({ loading: true })
  const [query, setQuery] = useState('')
  const [source, setSource] = useState('all')
  const [outcome, setOutcome] = useState('all')
  const [selected, setSelected] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    Promise.resolve().then(() => getCaseOverview({ caseId, signal: controller.signal }))
      .then(data => setOverview({ data, loading: false }))
      .catch(error => error.name !== 'AbortError' && setOverview({ error, loading: false }))
    return () => controller.abort()
  }, [caseId])

  const activities = useMemo(() => buildActivities({ sessionActivities, questionHistory, overview: overview.data, caseId }), [sessionActivities, questionHistory, overview.data, caseId])
  const matching = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase()
    return activities.filter(item => {
      const text = [item.title, item.answer, item.evidenceId, item.family].filter(Boolean).join(' ').toLocaleLowerCase()
      return (!needle || text.includes(needle)) && (source === 'all' || item.sourceKind === source)
    })
  }, [activities, query, source])
  const counts = useMemo(() => countOutcomes(matching), [matching])
  const visible = useMemo(() => (outcome === 'all' ? matching : matching.filter(item => outcomeOf(item) === outcome)), [matching, outcome])
  const groups = useMemo(() => groupByDay(visible), [visible])
  const current = visible.find(item => item.id === selected) || visible[0]
  const pulse = useMemo(() => seriesByDay(activities, item => item.sourceKind, ['questions', 'evidence']), [activities])
  const hasFilters = Boolean(query || source !== 'all' || outcome !== 'all')
  const clear = () => { setQuery(''); setSource('all'); setOutcome('all') }
  const questions = activities.filter(item => item.sourceKind === 'questions').length
  const dated = activities.map(item => asTimestamp(item.recordedAt)).filter(Boolean)
  const latest = dated.length ? new Date(Math.max(...dated)) : null

  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="activity-page ac" tabIndex={-1}>
        <header className="ac-head">
          <div>
            <h1>Recent activity</h1>
            <p>Resume questions saved in this browser and review the bounded recent evidence state reported for this collection.</p>
          </div>
        </header>

        {activities.length > 0 && (
          <>
            <Pulse stats={[{ label: 'Entries', value: formatNumber(activities.length) }, { label: 'Questions', value: formatNumber(questions) }, { label: 'Evidence updates', value: formatNumber(activities.length - questions) }, { label: 'Latest', value: latest ? relativeAge(latest) : '—' }]} data={pulse} keys={['questions', 'evidence']} labelOf={key => (key === 'questions' ? 'Questions' : 'Evidence updates')} caption="Entries per day over the last 30 days that have any" />

            <div className="ac-bar" role="toolbar" aria-label="Find and filter activity">
              <label className="ac-search"><Search aria-hidden="true" /><span className="visually-hidden">Search activity</span><input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Question, filename or identifier" /></label>
              <div className="seg" role="group" aria-label="Source">{SOURCES.map(item => <button key={item.id} type="button" aria-pressed={source === item.id} onClick={() => setSource(item.id)}>{item.label}</button>)}</div>
              <span className="ac-bar__space" />
              {hasFilters ? <button type="button" className="ac-btn" onClick={clear}><RotateCcw aria-hidden="true" />Clear filters</button> : null}
              <button type="button" className="ac-btn" onClick={() => download(`activity-${caseId}.csv`, activitiesCsv(visible))}><Download aria-hidden="true" />Export CSV</button>
            </div>

            {visible.length === 0 ? (
              <EmptyState kind="no-match" label="No activity matches these filters" description="The available working record was not widened. Clear the filters to return to all available entries.">
                <button type="button" onClick={clear}>Clear filters</button>
              </EmptyState>
            ) : (
              <div className="ac-main">
                <Feed groups={groups} selectedId={current?.id} onSelect={setSelected} total={activities.length} shown={visible.length} />
                {current ? <Detail item={current} caseId={caseId} /> : null}
              </div>
            )}

            <div className="ac-below">
              <OutcomeCard counts={counts} total={matching.length} active={outcome} onPick={setOutcome} />
              <CoverageCard />
            </div>
          </>
        )}

        {!overview.loading && !overview.error && activities.length === 0 && (
          <>
            <EmptyState kind="no-match" label="No recent activity is available" description="No saved question exists in this browser and the collection status response returned no recent evidence entries.">
              <Link to={`/cases/${encodeURIComponent(caseId)}/investigate`}>Ask a question</Link>
            </EmptyState>
            <div className="ac-below ac-below--one"><CoverageCard /></div>
          </>
        )}
        {overview.loading && <RouteState state={questions ? 'partial' : 'loading'} label={questions ? 'Browser questions are available' : 'Loading recent evidence status'} description={questions ? 'The browser working record is visible while the recent collection-status sample loads.' : undefined} failedScope={questions ? 'Recent evidence status' : undefined} />}
        {overview.error && <RouteState state={questions ? 'partial' : overview.error.status === 403 ? 'forbidden' : 'error'} label={questions ? 'Browser questions are available with limits' : overview.error.status === 403 ? 'Activity access is forbidden' : 'Recent evidence status could not be loaded'} description={questions ? 'Saved browser questions remain available; recent evidence status did not load.' : undefined} failedScope={questions ? 'Recent evidence status' : undefined} reference={overview.error.reference || 'ACTIVITY'} />}
      </main>
    </CaseShell>
  )
}
