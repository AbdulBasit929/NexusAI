import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { agentsApi } from '../utils/api'
import { useAnalystPortal } from './AnalystPortalLayout'
import {
  activityFilterOptions,
  activityCell,
  activityFindingGroups,
  activityItemPresentation,
  activityResultState,
  groupActivityItems,
} from './analystActivityPresentation.js'
import { familyIcon } from './analystPresentation'
import ActivityResultTable from './ActivityResultTable.js'

function askPath(portal, view) {
  const params = new URLSearchParams({ prompt: view.question })
  if (view.primarySource?.evidenceId) params.set('evidence', view.primarySource.evidenceId)
  return portal.path(`/analyst?${params.toString()}`)
}

function sourcePath(portal, source) {
  return portal.path(`/analyst?panel=evidence&${source.navigation.toString()}`)
}

function activityTime(value) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return 'Time not reported'
  return date.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
}

function timelineRows(presentation) {
  return (Array.isArray(presentation?.visualizations) ? presentation.visualizations : [])
    .filter(item => item?.type === 'timeline')
    .flatMap(item => Array.isArray(item?.spec?.rows) ? item.spec.rows : [])
    .slice(0, 20)
}

function timelineMoment(row) {
  return row?.timestamp || row?.observed_at || row?.event_time || row?.source_time || row?.timestamp_seconds || 'Time not reported'
}

function timelineFinding(row) {
  return row?.label || row?.title || row?.event || row?.description || row?.plate || row?.normalized_plate || 'Evidence observation'
}

export default function AnalystHistory() {
  const portal = useAnalystPortal()
  const location = useLocation()
  const navigate = useNavigate()
  const [state, setState] = useState({ status: 'loading', items: [], error: '', cursor: '', hasMore: false })
  const [query, setQuery] = useState('')
  const [resultFilter, setResultFilter] = useState('all')
  const selectedId = new URLSearchParams(location.search).get('analysis') || ''
  const selected = useMemo(() => state.items.find(item => item.analysis_id === selectedId) || null, [selectedId, state.items])
  const scope = portal.activeWorkspace
  const catalogItems = useMemo(() => portal.dataState.evidence?.items || [], [portal.dataState.evidence?.items])

  const loadHistory = useCallback(async ({ append = false, cursor = '' } = {}) => {
    if (!scope?.caseId || !scope?.collectionId) return
    setState(previous => ({ ...previous, status: 'loading', error: '' }))
    try {
      const result = await agentsApi.analysisHistory('Forensic_Records_Analyst', undefined, {
        caseId: scope.caseId,
        collectionId: scope.collectionId,
      }, { limit: 25, cursor })
      setState(previous => ({
        status: 'ready',
        items: append ? [...previous.items, ...(result?.items || [])] : (result?.items || []),
        error: '',
        cursor: result?.next_cursor || '',
        hasMore: Boolean(result?.has_more),
      }))
    } catch (error) {
      setState(previous => ({ ...previous, status: 'error', error: error.message }))
    }
  }, [scope?.caseId, scope?.collectionId])

  useEffect(() => { loadHistory() }, [loadHistory])

  const itemViews = useMemo(() => state.items.map(item => activityItemPresentation(item, catalogItems)), [catalogItems, state.items])
  const filters = useMemo(() => activityFilterOptions(state.items), [state.items])
  const filteredViews = useMemo(() => itemViews.filter(view => {
    if (query.trim() && !view.searchText.includes(query.trim().toLowerCase())) return false
    return resultFilter === 'all' || view.result.filter === resultFilter
  }), [itemViews, query, resultFilter])
  const groups = useMemo(() => groupActivityItems(filteredViews.map(view => view.item)), [filteredViews])
  const viewsById = useMemo(() => new Map(filteredViews.map(view => [view.item.analysis_id, view])), [filteredViews])
  const selectedView = useMemo(() => selected ? activityItemPresentation(selected, catalogItems) : null, [catalogItems, selected])
  const selectedPresentation = selectedView?.presentation || {}
  const selectedFindingGroups = activityFindingGroups(selectedPresentation)
  const selectedTimeline = timelineRows(selectedPresentation)

  useEffect(() => {
    if (!filters.some(filter => filter.id === resultFilter)) setResultFilter('all')
  }, [filters, resultFilter])

  return (
    <div className="analyst-page analyst-activity-page">
      <section className="analyst-activity-hero">
        <div><span className="analyst-eyebrow">Investigation journal</span><h1>Activity</h1><p>Questions, findings, and evidence consulted in this workspace.</p></div>
        <Link className="analyst-primary-action" to={portal.path('/analyst')}><i className="fas fa-magnifying-glass" aria-hidden="true" /> Ask a question</Link>
      </section>

      <section className="analyst-activity-controls" aria-label="Search and filter loaded activity">
        <label className="analyst-search-field" aria-describedby="activity-search-boundary"><i className="fas fa-magnifying-glass" aria-hidden="true" /><span className="sr-only">Search loaded activity</span><input value={query} onChange={event => setQuery(event.target.value)} placeholder="Search loaded activity" /></label>
        <span id="activity-search-boundary" className="analyst-activity-search-boundary">Searches the {state.items.length.toLocaleString()} currently loaded {state.items.length === 1 ? 'item' : 'items'}.</span>
        <div className="analyst-activity-filters" role="group" aria-label="Filter activity by result">
          {filters.map(filter => <button key={filter.id} type="button" aria-pressed={resultFilter === filter.id} className={resultFilter === filter.id ? 'active' : ''} onClick={() => setResultFilter(filter.id)}>{filter.label}<span>{filter.count}</span></button>)}
        </div>
      </section>

      {state.status === 'loading' && !state.items.length && <div className="analyst-activity-loading" role="status"><i className="fas fa-circle-notch fa-spin" aria-hidden="true" /><span><strong>Loading activity…</strong><small>Opening the investigation journal.</small></span></div>}
      {state.status === 'error' && <div className="analyst-activity-error" role="alert"><i className="fas fa-triangle-exclamation" aria-hidden="true" /><span><strong>Activity could not be loaded.</strong><small>Your retained investigation record was not changed.</small><details><summary>Technical details</summary><code>{state.error}</code></details></span></div>}

      <div className="analyst-activity-layout" data-selected={Boolean(selectedView)}>
        <section className="analyst-activity-journal" aria-label="Investigation activity" aria-busy={state.status === 'loading'}>
          {groups.map(group => <section className="analyst-activity-day" key={group.label} aria-labelledby={`activity-day-${group.label.replace(/\W+/g, '-').toLowerCase()}`}>
            <h2 id={`activity-day-${group.label.replace(/\W+/g, '-').toLowerCase()}`}>{group.label}</h2>
            <ol>{group.items.map(item => {
              const view = viewsById.get(item.analysis_id) || activityItemPresentation(item, catalogItems)
              const active = selectedId === item.analysis_id
              return <li key={item.analysis_id}><article className="analyst-activity-item" data-active={active} data-tone={view.result.tone}>
                <time dateTime={item.created_at}>{activityTime(item.created_at)}</time>
                <span className="analyst-activity-item__marker" aria-hidden="true"><i className={`fas ${view.primarySource?.icon || familyIcon(view.presentation.family_id || 'analysis')}`} /></span>
                <Link className="analyst-activity-item__reopen" aria-current={active ? 'true' : undefined} to={portal.path(`/analyst?panel=history&analysis=${encodeURIComponent(item.analysis_id)}`)}>
                  <span className="analyst-activity-item__question"><bdi dir="auto">{view.question}</bdi></span>
                  <span className="analyst-activity-state" data-tone={view.result.tone}><i className={`fas ${view.result.icon}`} aria-hidden="true" /> {view.result.label}</span>
                  <span className="analyst-activity-item__summary"><bdi dir="auto">{view.summary}</bdi></span>
                  {view.primarySource && <span className="analyst-activity-item__source"><i className={`fas ${view.primarySource.icon}`} aria-hidden="true" /><span><small>{view.primarySource.type}</small><strong><bdi dir="auto">{view.primarySource.label}</bdi></strong>{view.primarySource.detail && <em>{view.primarySource.detail}</em>}</span>{view.sources.length > 1 && <b>+{view.sources.length - 1}</b>}</span>}
                  <span className="analyst-activity-item__action">View stored result <i className="fas fa-arrow-right" aria-hidden="true" /></span>
                </Link>
                <Link className="analyst-activity-item__continue" to={askPath(portal, view)} aria-label={`Use again in Ask: ${view.question}`}><i className="fas fa-arrow-rotate-left" aria-hidden="true" /> Use again</Link>
              </article></li>
            })}</ol>
          </section>)}

          {state.status === 'ready' && !filteredViews.length && <div className="analyst-empty analyst-activity-empty"><i className={`fas ${state.items.length ? 'fa-magnifying-glass' : 'fa-clock-rotate-left'}`} /><h2>{state.items.length ? 'No activity matches this view' : 'No activity yet'}</h2><p>{state.items.length ? 'Try another search or result filter. Only the currently loaded activity page is searched.' : 'Questions you ask and investigation results will appear here.'}</p>{!state.items.length && <Link className="analyst-primary-action" to={portal.path('/analyst')}>Ask a question</Link>}</div>}
          {state.hasMore && <button className="analyst-load-more" type="button" onClick={() => loadHistory({ append: true, cursor: state.cursor })} disabled={state.status === 'loading'}>{state.status === 'loading' ? 'Loading more activity…' : 'Load more activity'}</button>}
        </section>

        {selectedView && <aside className="analyst-activity-detail" aria-label="Stored investigation result">
          <header>
            <div><span className="analyst-eyebrow">Stored result</span><h2><bdi dir="auto">{selectedView.question}</bdi></h2></div>
            <button type="button" onClick={() => navigate(portal.path('/analyst?panel=history'))} aria-label="Close stored result"><i className="fas fa-xmark" aria-hidden="true" /></button>
          </header>
          <div className="analyst-activity-detail__status"><span className="analyst-activity-state" data-tone={selectedView.result.tone}><i className={`fas ${selectedView.result.icon}`} aria-hidden="true" /> {selectedView.result.label}</span><time dateTime={selected.created_at}>{new Date(selected.created_at).toLocaleString()}</time></div>

          <section className="analyst-activity-answer" aria-labelledby="activity-answer-title"><span>Answer</span><h3 id="activity-answer-title"><bdi dir="auto">{selectedView.answer || selectedView.result.description || 'No retained answer text was reported.'}</bdi></h3></section>

          {selectedPresentation.metrics?.length > 0 && <dl className="analyst-activity-metrics">{selectedPresentation.metrics.map((metric, index) => <div key={`${metric.label}-${index}`}><dt>{metric.label}</dt><dd>{activityCell(metric.value)}</dd></div>)}</dl>}
          {selectedFindingGroups.visible.length > 0 && <section className="analyst-activity-detail__section"><h3>Key findings</h3><ul className="analyst-activity-findings">{selectedFindingGroups.visible.map((finding, index) => <li key={index}><span>{String(finding.claim_type || finding.kind || 'Finding').replaceAll('_', ' ')}</span><bdi dir="auto">{activityCell(finding)}</bdi></li>)}</ul></section>}
          <ActivityResultTable table={selectedPresentation.table} operation={selectedPresentation.operation_id} />
          {selectedTimeline.length > 0 && <section className="analyst-activity-detail__section"><h3>Timeline</h3><ol className="analyst-activity-timeline">{selectedTimeline.map((row, index) => <li key={index}><time><bdi dir="auto">{activityCell(timelineMoment(row))}</bdi></time><bdi dir="auto">{activityCell(timelineFinding(row))}</bdi></li>)}</ol></section>}

          {selectedView.sources.length > 0 && <section className="analyst-activity-detail__section"><h3>Evidence consulted</h3><div className="analyst-activity-sources">{selectedView.sources.map((source, index) => source.evidenceId
            ? <Link className="analyst-activity-source" to={sourcePath(portal, source)} key={`${source.evidenceId}-${index}`} aria-label={`Open evidence source ${source.label}`}><i className={`fas ${source.icon}`} aria-hidden="true" /><span><small>{source.type}</small><strong><bdi dir="auto">{source.label}</bdi></strong>{source.detail && <em>{source.detail}</em>}</span><i className="fas fa-arrow-up-right-from-square" aria-hidden="true" /></Link>
            : <div className="analyst-activity-source" key={index}><i className={`fas ${source.icon}`} aria-hidden="true" /><span><small>{source.type}</small><strong><bdi dir="auto">{source.label}</bdi></strong>{source.detail && <em>{source.detail}</em>}</span></div>)}</div></section>}

          {selectedPresentation.limitations?.length > 0 && <details className="analyst-activity-disclosure"><summary>Limitations</summary><ul>{selectedPresentation.limitations.map((limitation, index) => <li key={index}>{limitation}</li>)}</ul></details>}

          <div className="analyst-activity-detail__actions"><Link className="analyst-primary-action" to={askPath(portal, selectedView)}><i className="fas fa-arrow-rotate-left" aria-hidden="true" /> Use question again</Link></div>
          <details className="analyst-technical-details"><summary><span><strong>Technical details</strong><small>Identifiers and execution metadata</small></span><i className="fas fa-chevron-down" aria-hidden="true" /></summary><dl>
            <div><dt>History record</dt><dd><bdi dir="ltr">{selected.analysis_id}</bdi></dd></div>
            <div><dt>Result state</dt><dd>{activityResultState(selected).id}</dd></div>
            <div><dt>Operation</dt><dd>{selectedPresentation.operation_id || 'Not reported'}</dd></div>
            <div><dt>Execution authority</dt><dd>{selected.execution_authority || selectedPresentation.execution_authority || 'Not reported'}</dd></div>
            {selectedFindingGroups.technical.length > 0 && <div><dt>Execution notes</dt><dd>{selectedFindingGroups.technical.map(finding => activityCell(finding)).join(' · ')}</dd></div>}
            {selectedView.sources.some(source => source.evidenceId) && <div><dt>Evidence identifiers</dt><dd>{selectedView.sources.filter(source => source.evidenceId).map(source => source.evidenceId).join(', ')}</dd></div>}
          </dl></details>
        </aside>}
      </div>
    </div>
  )
}
