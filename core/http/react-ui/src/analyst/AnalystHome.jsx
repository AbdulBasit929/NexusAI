import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { agentsApi } from '../utils/api'
import { useAnalystPortal } from './AnalystPortalLayout'
import { activityItemPresentation } from './analystActivityPresentation'
import { evidenceFindingPreview, evidenceTypePresentation } from './analystDataPresentation'
import { homeActivityPreviewText, homeEvidenceSummary, homeOrientation } from './analystHomePresentation'
import { capabilityForSource, capabilitySuggestions, friendlySourceName, friendlyWorkspaceName, sourceProductState } from './analystPresentation'

function formatActivityTime(value) {
  const date = new Date(value)
  return Number.isFinite(date.getTime())
    ? date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
    : 'Time not reported'
}

export default function AnalystHome() {
  const portal = useAnalystPortal()
  const { activeWorkspace, dataState } = portal
  const [recent, setRecent] = useState([])
  const evidenceSummary = useMemo(
    () => homeEvidenceSummary(dataState.statusData, dataState.evidence),
    [dataState.evidence, dataState.statusData],
  )
  const orientation = homeOrientation(evidenceSummary)
  const suggestions = useMemo(() => capabilitySuggestions(dataState.capabilities, 1), [dataState.capabilities])
  const recentEvidence = useMemo(
    () => (Array.isArray(dataState.evidence?.items) ? dataState.evidence.items : []).slice(0, 4),
    [dataState.evidence],
  )
  const recentViews = useMemo(
    () => recent.map(item => activityItemPresentation(item, dataState.evidence?.items || [])),
    [dataState.evidence?.items, recent],
  )

  useEffect(() => {
    if (!activeWorkspace?.caseId || !activeWorkspace?.collectionId) return
    let active = true
    agentsApi.analysisHistory('Forensic_Records_Analyst', undefined, {
      caseId: activeWorkspace.caseId,
      collectionId: activeWorkspace.collectionId,
    }, { limit: 4 }).then(result => {
      if (active) setRecent(Array.isArray(result?.items) ? result.items : [])
    }).catch(() => { if (active) setRecent([]) })
    return () => { active = false }
  }, [activeWorkspace?.caseId, activeWorkspace?.collectionId])

  return (
    <div className="analyst-page analyst-home-page">
      <header className="analyst-workspace-intro">
        <div>
          <span className="analyst-eyebrow">Investigation workspace</span>
          <h1><bdi dir="auto">{friendlyWorkspaceName(activeWorkspace)}</bdi></h1>
        </div>
        <p className="analyst-workspace-readiness" data-tone={orientation.tone}>
          <i className="fas fa-circle" aria-hidden="true" /> {orientation.label}
        </p>
      </header>

      {dataState.status === 'error' && <div className="analyst-inline-error" role="alert">Workspace data could not be refreshed: {dataState.error}</div>}

      <section className="analyst-home-start" data-state={orientation.tone} aria-labelledby="analyst-home-start-title">
        <span className="analyst-home-start__icon" aria-hidden="true"><i className={`fas ${evidenceSummary.isEmpty ? 'fa-folder-plus' : evidenceSummary.ready ? 'fa-magnifying-glass' : 'fa-arrows-rotate'}`} /></span>
        <div className="analyst-home-start__copy">
          <span className="analyst-eyebrow">Start here</span>
          <h2 id="analyst-home-start-title">{orientation.title}</h2>
          <p>{orientation.description}</p>
          {evidenceSummary.ready > 0 && suggestions[0] && <small>Try asking: <q><bdi dir="auto">{suggestions[0].query}</bdi></q></small>}
        </div>
        <div className="analyst-home-start__actions">
          {orientation.action === 'add' && <Link className="analyst-primary-action" to={portal.path('/analyst/data?add=true')}><i className="fas fa-plus" /> Add data</Link>}
          {orientation.action === 'ask' && <Link className="analyst-primary-action" to={portal.path('/analyst/ask')}><i className="fas fa-arrow-right" /> Open Ask</Link>}
          {orientation.action === 'review' && <Link className="analyst-primary-action" to={portal.path('/analyst/data')}><i className="fas fa-folder-open" /> Review data</Link>}
          {orientation.action === 'ask' && <Link className="analyst-secondary-action" to={portal.path('/analyst/data')}><i className="fas fa-folder-open" /> Review data</Link>}
        </div>
      </section>

      {(evidenceSummary.processing > 0 || evidenceSummary.attention > 0) && (
        <Link className="analyst-attention-strip" to={portal.path('/analyst/data')}>
          <i className={`fas ${evidenceSummary.attention ? 'fa-triangle-exclamation' : 'fa-arrows-rotate'}`} aria-hidden="true" />
          <span>
            <strong>{evidenceSummary.attention
              ? `${evidenceSummary.attention} source${evidenceSummary.attention === 1 ? '' : 's'} need attention`
              : `${evidenceSummary.processing} source${evidenceSummary.processing === 1 ? '' : 's'} processing`}</strong>
            <small>Open Data to review the affected evidence and available next steps.</small>
          </span>
          <i className="fas fa-arrow-right" aria-hidden="true" />
        </Link>
      )}

      <div className="analyst-home-overview">
        <section className="analyst-home-section analyst-home-data" aria-labelledby="analyst-home-data-title">
          <div className="analyst-section-heading">
            <div><span className="analyst-eyebrow">Data</span><h2 id="analyst-home-data-title">Evidence overview</h2></div>
            <Link to={portal.path('/analyst/data')}>View all <i className="fas fa-arrow-right" aria-hidden="true" /></Link>
          </div>

          <dl className="analyst-home-states" aria-label="Evidence readiness">
            <div data-state="ready"><dt>Ready</dt><dd>{evidenceSummary.ready}</dd></div>
            <div data-state="processing"><dt>Processing</dt><dd>{evidenceSummary.processing}</dd></div>
            <div data-state="attention"><dt>Needs attention</dt><dd>{evidenceSummary.attention}</dd></div>
          </dl>

          <div className="analyst-home-evidence">
            {recentEvidence.map(item => {
              const status = sourceProductState(item)
              const type = evidenceTypePresentation(item)
              const capability = capabilityForSource(item, dataState.capabilities)
              return (
                <Link key={item.evidence_id} to={portal.path(`/analyst/data?evidence=${encodeURIComponent(item.evidence_id)}`)}>
                  <span className="analyst-evidence-row__icon" data-kind={type.category} aria-hidden="true"><i className={`fas ${type.icon}`} /></span>
                  <span className="analyst-home-evidence__copy">
                    <small>{type.label}</small>
                    <strong><bdi dir="auto">{friendlySourceName(item)}</bdi></strong>
                    <span>{evidenceFindingPreview(item, status, capability)}</span>
                  </span>
                  <span className="analyst-simple-status" data-tone={status.tone}>{status.label}</span>
                </Link>
              )
            })}
            {!recentEvidence.length && (
              <div className="analyst-home-empty">
                <i className="fas fa-folder-open" aria-hidden="true" />
                <div><strong>This workspace has no evidence yet</strong><span>Add a retained source to begin reviewing and investigating it.</span></div>
                <Link to={portal.path('/analyst/data?add=true')}>Add data</Link>
              </div>
            )}
          </div>
        </section>

        <section className="analyst-home-section analyst-home-activity" aria-labelledby="analyst-home-activity-title">
          <div className="analyst-section-heading">
            <div><span className="analyst-eyebrow">Activity</span><h2 id="analyst-home-activity-title">Recent analysis</h2></div>
            <Link to={portal.path('/analyst/activity')}>View all <i className="fas fa-arrow-right" aria-hidden="true" /></Link>
          </div>
          <ol>
            {recentViews.slice(0, 3).map(view => (
              <li key={view.item.analysis_id}>
                <Link to={portal.path(`/analyst/activity?analysis=${encodeURIComponent(view.item.analysis_id)}`)}>
                  <span className="analyst-home-activity__state" data-tone={view.result.tone}><i className={`fas ${view.result.icon}`} aria-hidden="true" /> {view.result.label}</span>
                  <strong><bdi dir="auto">{homeActivityPreviewText(view.question)}</bdi></strong>
                  <span><bdi dir="auto">{homeActivityPreviewText(view.summary, view.question)}</bdi></span>
                  <small>{formatActivityTime(view.item.created_at)}{view.primarySource ? ` · ${view.primarySource.label}` : ''}</small>
                </Link>
              </li>
            ))}
          </ol>
          {!recentViews.length && (
            <div className="analyst-home-empty analyst-home-empty--activity">
              <i className="fas fa-clock-rotate-left" aria-hidden="true" />
              <div><strong>No analysis activity yet</strong><span>Questions and their source-backed results will appear here.</span></div>
              {evidenceSummary.ready > 0 && <Link to={portal.path('/analyst/ask')}>Ask a question</Link>}
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
