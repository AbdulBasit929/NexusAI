import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { CaseShell } from '../components/CaseShell.jsx'
import { useSessionActivity } from '../lib/sessionActivity.js'
import { useQuestionHistory } from '../lib/workspaceState.js'
import { getCaseOverview } from '../lib/apiClient.js'
import { EmptyState, LanguageText, RouteState } from '../components/AnalystComponents.jsx'
import { SelectControl } from '../components/SelectControl.jsx'
import { PageHeader } from '../components/PageHeader.jsx'

const stateLabels = {
  answered: 'Answered',
  partial: 'Answered with limits',
  'zero-result': 'Answered · no matches',
  clarify: 'Clarification requested',
  processing: 'Processing',
  unsupported: 'Unavailable',
  failed: 'Failed',
}

const familyLabels = { cdr: 'CDR', ipdr: 'IPDR', anpr: 'ANPR', tower: 'Tower and cell', document: 'Documents', audio: 'Audio', video: 'Video', image: 'Images', 'all evidence': 'All evidence' }
const sourceLabels = { questions: 'Questions', evidence: 'Evidence status' }

function familyLabel(value) {
  if (!value) return 'Not reported'
  return familyLabels[String(value).toLowerCase()] || String(value)
}

function outcomeLabel(item) {
  if (item.kind === 'evidence') {
    if (item.state === 'answered') return 'Evidence ready'
    if (item.state === 'processing') return 'Evidence processing'
    if (item.state === 'failed') return 'Evidence failed'
    return 'Evidence status unavailable'
  }
  return stateLabels[item.state] || 'Outcome not reported'
}

function ActivityState({ item }) {
  const failed = item.state === 'failed'
  const unavailable = item.state === 'unsupported'
  const attention = ['clarify', 'processing', 'partial'].includes(item.state)
  return (
    <span className={`activity-state activity-state--${item.state}`}>
      <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
        <circle cx="10" cy="10" r="7.5" />
        {failed ? <path d="m7 7 6 6m0-6-6 6" />
          : unavailable ? <path d="m6.5 13.5 7-7" />
            : attention ? <><path d="M10 6.5v4" /><path d="M10 13.5h.01" /></>
              : <path d="m6.5 10 2.2 2.2 4.8-5" />}
      </svg>
      {outcomeLabel(item)}
    </span>
  )
}

function evidenceState(status) {
  if (status === 'completed') return 'answered'
  if (['queued', 'registered', 'processing', 'running'].includes(status)) return 'processing'
  if (status === 'failed') return 'failed'
  return 'unsupported'
}

function evidenceActivities(data, caseId) {
  const jobs = new Map((data?.recent_jobs || []).map(job => [job.evidence_id, job]))
  return (data?.recent_evidence || []).map(evidence => {
    const job = jobs.get(evidence.evidence_id) || {}
    const state = evidenceState(evidence.processing_status)
    const reprocessed = Number(job.attempt_count) > 1
    const hasCounts = Number.isFinite(Number(job.accepted_rows))
    const counts = hasCounts
      ? `${Number(job.accepted_rows).toLocaleString()} accepted; ${Number(job.rejected_rows || 0).toLocaleString()} rejected; ${Number(job.duplicate_rows || 0).toLocaleString()} duplicates.`
      : ''
    const family = evidence.detected_type || evidence.modality || null
    return {
      id: `evidence-${evidence.evidence_id}`,
      caseId,
      kind: 'evidence',
      sourceKind: 'evidence',
      sourceLabel: 'Recent collection status',
      evidenceId: evidence.evidence_id,
      family,
      state,
      title: `${reprocessed ? 'Evidence reprocessed' : 'Evidence added'}: ${evidence.source_file || evidence.evidence_id}`,
      answer: state === 'answered' ? `Processing completed.${counts ? ` ${counts}` : ''}` : state === 'processing' ? 'Evidence processing is not complete.' : state === 'failed' ? 'Evidence processing failed. No analysis result is implied.' : 'The collection status response did not report a recognized processing state.',
      answerWithheld: state !== 'answered',
      scope: [
        { label: 'Evidence family', value: familyLabel(family) },
        { label: 'Evidence ID', value: evidence.evidence_id },
      ],
      recordedAt: evidence.updated_at || evidence.created_at || null,
    }
  })
}

function browserHistoryActivities(history, sessionActivities, caseId) {
  const activeQueries = new Set(sessionActivities.map(item => item.query))
  return history.filter(item => !activeQueries.has(item.query)).map(item => ({
    id: `history-${item.id}`,
    caseId,
    kind: 'analysis',
    sourceKind: 'questions',
    sourceLabel: 'Saved in this browser',
    query: item.query,
    family: null,
    state: item.state || 'unsupported',
    title: item.label || item.query,
    answer: item.answer || 'Answer text was not retained.',
    answerWithheld: ['clarify', 'unsupported', 'failed', 'processing'].includes(item.state),
    scope: [{ label: 'Analysis scope', value: 'Not retained with this saved entry' }],
    recordedAt: item.updatedAt || null,
  }))
}

function asTimestamp(value) {
  const time = value ? new Date(value).getTime() : Number.NaN
  return Number.isFinite(time) ? time : 0
}

function displayTime(value) {
  if (!asTimestamp(value)) return 'Time not reported'
  return `${new Intl.DateTimeFormat('en-GB', {
    day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false, timeZone: 'UTC',
  }).format(new Date(value))} UTC`
}

export default function ActivityPage() {
  const { id: caseId = '' } = useParams()
  const sessionActivities = useSessionActivity(caseId)
  const questionHistory = useQuestionHistory(caseId)
  const [overview, setOverview] = useState({ loading: true })
  const [query, setQuery] = useState('')
  const [source, setSource] = useState('all')
  const [family, setFamily] = useState('all')
  const [state, setState] = useState('all')

  useEffect(() => {
    const controller = new AbortController()
    Promise.resolve().then(() => getCaseOverview({ caseId, signal: controller.signal }))
      .then(data => setOverview({ data, loading: false }))
      .catch(error => error.name !== 'AbortError' && setOverview({ error, loading: false }))
    return () => controller.abort()
  }, [caseId])

  const activities = useMemo(() => {
    const currentTab = sessionActivities.map(item => ({ ...item, sourceKind: 'questions', sourceLabel: 'Current tab' }))
    return [
      ...currentTab,
      ...browserHistoryActivities(questionHistory, sessionActivities, caseId),
      ...evidenceActivities(overview.data, caseId),
    ].sort((left, right) => asTimestamp(right.recordedAt) - asTimestamp(left.recordedAt))
  }, [sessionActivities, questionHistory, overview.data, caseId])

  const families = useMemo(() => [...new Set(activities.map(item => item.family).filter(Boolean))].sort(), [activities])
  const states = useMemo(() => [...new Set(activities.map(item => item.state).filter(Boolean))].sort(), [activities])
  const visible = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase()
    return activities.filter(item => {
      const searchable = [item.title, item.answer, item.evidenceId, item.family].filter(Boolean).join(' ').toLocaleLowerCase()
      return (!needle || searchable.includes(needle))
        && (source === 'all' || item.sourceKind === source)
        && (family === 'all' || item.family === family)
        && (state === 'all' || item.state === state)
    })
  }, [activities, query, source, family, state])
  const hasFilters = Boolean(query || source !== 'all' || family !== 'all' || state !== 'all')
  const clearFilters = () => { setQuery(''); setSource('all'); setFamily('all'); setState('all') }
  const localCount = activities.filter(item => item.sourceKind === 'questions').length

  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="activity-page" tabIndex={-1}>
        <PageHeader
          eyebrow="Case workspace"
          title="Recent activity"
          description="Resume questions saved in this browser and review the bounded recent evidence state reported for this collection."
          actions={<Link className="button-link button-link--primary" to={`/cases/${encodeURIComponent(caseId)}/investigate`}>Ask a question</Link>}
        />

        <aside className="activity-boundary" aria-labelledby="activity-boundary-title">
          <div aria-hidden="true" className="activity-boundary__icon">i</div>
          <div>
            <h2 id="activity-boundary-title">Working record, not audit history</h2>
            <p>Questions come from this browser. Evidence rows are only the recent sample returned by collection status. User, custody, team-wide and export activity are not available from the current API.</p>
          </div>
        </aside>

        {activities.length > 0 && (
          <section className="activity-workspace" aria-labelledby="activity-workspace-title">
            <header className="activity-workspace__header">
              <div>
                <p className="section-kicker">Current collection</p>
                <h2 id="activity-workspace-title">Questions and evidence status</h2>
                <p>{localCount.toLocaleString()} {localCount === 1 ? 'question' : 'questions'} available from this browser. Evidence entries remain a bounded API sample.</p>
              </div>
              <p className="activity-result-count" role="status">Showing <strong>{visible.length.toLocaleString()}</strong> of {activities.length.toLocaleString()}</p>
            </header>

            <div className="activity-toolbar" aria-label="Find and filter activity">
              <label className="activity-search">
                <span>Search activity</span>
                <input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Question, filename or identifier" />
              </label>
              <SelectControl label="Source" value={source} options={['questions', 'evidence']} onChange={setSource} optionLabel={value => sourceLabels[value]} />
              <SelectControl label="Evidence family" value={family} options={families} onChange={setFamily} optionLabel={familyLabel} />
              <SelectControl label="Outcome" value={state} options={states} onChange={setState} optionLabel={value => stateLabels[value] || String(value)} />
              {hasFilters && <button type="button" className="activity-clear" onClick={clearFilters}>Clear filters</button>}
            </div>

            {visible.length === 0 ? (
              <EmptyState kind="no-match" label="No activity matches these filters" description="The available working record was not widened. Clear the filters to return to all available entries.">
                <button type="button" onClick={clearFilters}>Clear filters</button>
              </EmptyState>
            ) : (
              <>
                <div className="activity-columns" aria-hidden="true">
                  <span>Time</span><span>Activity</span><span>Source</span><span>Outcome</span><span>Scope</span><span>Action</span>
                </div>
                <ol className="activity-list" aria-label="Activity">
                  {visible.map(item => (
                    <li key={item.id}>
                      <article className="activity-row">
                        <div className="activity-cell activity-cell--time" data-label="Time">
                          {asTimestamp(item.recordedAt)
                            ? <time dateTime={item.recordedAt}>{displayTime(item.recordedAt)}</time>
                            : <span>Time not reported</span>}
                        </div>
                        <div className="activity-cell activity-cell--summary" data-label="Activity">
                          <h3><LanguageText>{item.title}</LanguageText></h3>
                          <LanguageText as="p" className={item.answerWithheld ? 'activity-answer activity-answer--withheld' : 'activity-answer'}>{item.answer}</LanguageText>
                        </div>
                        <div className="activity-cell" data-label="Source"><span className="activity-source">{item.sourceLabel}</span></div>
                        <div className="activity-cell" data-label="Outcome"><ActivityState item={item} /></div>
                        <div className="activity-cell activity-cell--scope" data-label="Scope">
                          <dl className="activity-scope">
                            {item.scope.map((scopeItem, index) => (
                              <div key={`${scopeItem.label}-${scopeItem.value}-${index}`}><dt>{scopeItem.label}</dt><dd><LanguageText>{scopeItem.value}</LanguageText></dd></div>
                            ))}
                          </dl>
                        </div>
                        <div className="activity-cell activity-cell--action" data-label="Action">
                          {item.kind === 'analysis'
                            ? <Link to={`/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(item.query)}`}>Reopen with this question</Link>
                            : <Link to={`/cases/${encodeURIComponent(caseId)}/evidence/${encodeURIComponent(item.evidenceId)}`}>Open evidence</Link>}
                        </div>
                      </article>
                    </li>
                  ))}
                </ol>
              </>
            )}
          </section>
        )}

        {!overview.loading && !overview.error && activities.length === 0 && (
          <EmptyState kind="no-match" label="No recent activity is available" description="No saved question exists in this browser and the collection status response returned no recent evidence entries.">
            <Link to={`/cases/${encodeURIComponent(caseId)}/investigate`}>Ask a question</Link>
          </EmptyState>
        )}
        {overview.loading && <RouteState state={localCount ? 'partial' : 'loading'} label={localCount ? 'Browser questions are available' : 'Loading recent evidence status'} description={localCount ? 'The browser working record is visible while the recent collection-status sample loads.' : undefined} failedScope={localCount ? 'Recent evidence status' : undefined} />}
        {overview.error && <RouteState state={localCount ? 'partial' : overview.error.status === 403 ? 'forbidden' : 'error'} label={localCount ? 'Browser questions are available with limits' : overview.error.status === 403 ? 'Activity access is forbidden' : 'Recent evidence status could not be loaded'} description={localCount ? 'Saved browser questions remain available; recent evidence status did not load.' : undefined} failedScope={localCount ? 'Recent evidence status' : undefined} reference={overview.error.reference || 'ACTIVITY'} />}
      </main>
    </CaseShell>
  )
}
