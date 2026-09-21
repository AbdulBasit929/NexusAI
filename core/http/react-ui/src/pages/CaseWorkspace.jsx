import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, Navigate, useLocation, useOutletContext, useParams } from 'react-router-dom'
import { recordsApi } from '../utils/api'
import RouteFallback from '../components/RouteFallback'
import SharedEmptyState from '../components/EmptyState'
import NexusLoadingState from '../components/NexusLoadingState'
import { useActiveCase } from '../contexts/ActiveCaseContext'
import CaseWorkspaceChrome from '../components/case-workspace/CaseWorkspaceChrome'
import { CASE_WORKSPACE_COMPATIBILITY, CASE_WORKSPACE_MODULES } from '../components/case-workspace/caseWorkspaceModules'
import EvidenceWorkspace from '../components/evidence-workspace/EvidenceWorkspace'
import { resolveEvidenceModality } from '../utils/evidenceModality'

const RecordsIntelligence = lazy(() => import('./RecordsIntelligence'))

function countValue(value) {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function resourceCount(manifest, key) {
  const item = manifest?.resources?.find(resource => resource.resource === key)
  return item?.count == null ? '—' : countValue(item.count).toLocaleString()
}

function humanize(value) {
  if (value == null || value === '') return 'Not reported'
  return String(value).replaceAll('_', ' ').replace(/\b\w/g, letter => letter.toUpperCase())
}

function collectionGroup(item) {
  if (item.visibility !== 'system') return 'Active investigations'
  if (item.environment === 'demo') return 'Legacy and demo collections'
  return 'Retained validation and system collections'
}

function analysisPosture(status, manifest, loading = false) {
  if (loading) return { id: 'loading', label: 'Verifying case state', detail: 'Loading governed evidence, capability, and lineage data.', tone: 'neutral' }
  const rows = countValue(status?.summary?.accepted_rows)
  const kb = countValue(status?.summary?.kb_assets_total || resourceCount(manifest, 'kb_entries'))
  const evidence = countValue(status?.summary?.evidence_total)
  if (rows > 0 && kb > 0) return { id: 'hybrid', label: 'Hybrid ready', detail: 'Exact records and cited knowledge are available.', tone: 'good' }
  if (rows > 0) return { id: 'records', label: 'Exact records ready', detail: 'Deterministic records operations are available.', tone: 'good' }
  if (kb > 0) return { id: 'knowledge', label: 'Knowledge-only', detail: 'Cited KB analysis is available; exact record operations remain disabled until normalization.', tone: 'warning' }
  if (evidence > 0) return { id: 'processing', label: 'Evidence registered', detail: 'Evidence exists but no queryable records or KB assets are ready yet.', tone: 'warning' }
  return { id: 'empty', label: 'Inventory only', detail: 'This accessible collection has no queryable evidence yet.', tone: 'neutral' }
}

function Kpi({ label, value, detail, tone = '', icon = '' }) {
  return (
    <div className={`case-kpi ${tone ? `case-kpi--${tone}` : ''}`}>
      <span>{icon && <i className={`fas ${icon}`} aria-hidden="true" />} {label}</span>
      <strong>{value}</strong>
      {detail && <small>{detail}</small>}
    </div>
  )
}

function EmptyState({ icon, title, children, action }) {
  return (
    <div className="case-empty">
      <i className={`fas ${icon}`} aria-hidden="true" />
      <h2>{title}</h2>
      <p>{children}</p>
      {action}
    </div>
  )
}

function OverviewPage({ caseId, status, capabilities, manifest, loading }) {
  const summary = status?.summary || {}
  const families = Array.isArray(capabilities?.families)
    ? capabilities.families.filter(family => family.available || countValue(family.indexed_records) > 0)
    : (status?.record_families || [])
  const warnings = [
    ...(manifest?.case_metadata?.warnings || []),
    ...(countValue(summary.failed_jobs) > 0 ? [`${summary.failed_jobs} failed or dead-letter ingest job(s) require review.`] : []),
    ...(countValue(summary.completed_jobs_missing_kb_asset) > 0 ? [`${summary.completed_jobs_missing_kb_asset} completed job(s) have no linked KB asset.`] : []),
  ]
  const posture = analysisPosture(status, manifest, loading)
  const displayCount = value => loading ? '—' : countValue(value).toLocaleString()
  return (
    <div className="case-page-stack">
      <section className={`case-readiness case-readiness--${posture.tone}`}>
        <div><span className="case-eyebrow">Collection readiness</span><h2>{posture.label}</h2><p>{posture.detail}</p></div>
        <div className="case-quick-actions"><Link className="btn btn-primary" to={`/app/cases/${encodeURIComponent(caseId)}/ask`}><i className="fas fa-magnifying-glass-chart" /> Ask NexusAI</Link><Link className="btn btn-secondary" to={`/app/cases/${encodeURIComponent(caseId)}/evidence`}><i className="fas fa-box-archive" /> Review evidence</Link></div>
      </section>
      <div className="case-kpi-grid" aria-busy={loading}>
        <Kpi icon="fa-fingerprint" label="Registered evidence" value={displayCount(summary.evidence_total)} detail={loading ? 'Checking evidence registry' : `${countValue(summary.evidence_in_flight)} processing`} />
        <Kpi icon="fa-table-list" label="Queryable rows" value={displayCount(summary.accepted_rows)} detail={loading ? 'Checking deterministic stores' : `${countValue(summary.rejected_rows)} rejected`} tone={!loading && countValue(summary.rejected_rows) ? 'warning' : !loading ? 'good' : ''} />
        <Kpi icon="fa-book" label="Knowledge assets" value={displayCount(summary.kb_assets_total)} detail={loading ? 'Checking cited knowledge' : `${countValue(summary.completed_jobs_missing_kb_asset)} missing links`} />
        <Kpi icon="fa-layer-group" label="Evidence families" value={loading ? '—' : families.length.toLocaleString()} detail={loading ? 'Checking capability coverage' : 'Live capability coverage'} />
      </div>

      {warnings.length > 0 && (
        <section className="case-panel case-panel--warning" aria-labelledby="case-warning-title">
          <h2 id="case-warning-title"><i className="fas fa-triangle-exclamation" /> Review notes</h2>
          <ul>{warnings.map((warning, index) => <li key={index}>{warning}</li>)}</ul>
        </section>
      )}

      <div className="case-two-column">
        <section className="case-panel" aria-labelledby="family-coverage-title">
          <div className="case-panel-heading">
            <div><span className="case-eyebrow">Coverage</span><h2 id="family-coverage-title">Queryable evidence families</h2></div>
            <Link className="btn btn-secondary btn-sm" to={`/app/cases/${encodeURIComponent(caseId)}/ask`}>Open analysis</Link>
          </div>
          {loading ? <p className="case-muted" role="status">Verifying queryable family coverage…</p> : families.length ? (
            <div className="case-family-grid">
              {families.map(family => (
                <div className="case-family-card" key={family.id || family.record_type}>
                  <strong>{family.label || family.record_type}</strong>
                  <span>{family.support_level || `${countValue(family.total_rows).toLocaleString()} rows`}</span>
                  <small>{family.availability_reason || family.adapter || 'Deterministic records route'}</small>
                </div>
              ))}
            </div>
          ) : <p className="case-muted">No queryable family coverage is reported for this case yet.</p>}
        </section>

        <section className="case-panel" aria-labelledby="manifest-summary-title">
          <span className="case-eyebrow">Reconciliation</span>
          <h2 id="manifest-summary-title">Read-only manifest</h2>
          <div className="case-manifest-summary">
            <div><span>KB entries</span><strong>{loading ? '—' : resourceCount(manifest, 'kb_entries')}</strong></div>
            <div><span>Canonical rows</span><strong>{loading ? '—' : resourceCount(manifest, 'canonical_records')}</strong></div>
            <div><span>Agent bindings</span><strong>{loading ? '—' : resourceCount(manifest, 'agents')}</strong></div>
            <div><span>Deletions</span><strong>{loading ? '—' : manifest?.deletion_count ?? 0}</strong></div>
          </div>
          <Link className="case-text-link" to={`/app/cases/${encodeURIComponent(caseId)}/admin`}>Review governance and all resource counts <i className="fas fa-arrow-right" /></Link>
        </section>
      </div>
    </div>
  )
}

function RelationshipsPage({ caseId, capabilities }) {
  const queryable = countValue(capabilities?.summary?.queryable) || (capabilities?.families || []).filter(family => family.available).length
  const prompts = [
    ['Relationship network', 'show relationship network', 'fa-share-nodes', 'Typed entity-to-entity edges with source-row citations.'],
    ['Cross-family correlation', 'correlate an identifier across record families', 'fa-link', 'Compare the same normalized identifier across available families.'],
    ['Frequent contacts', 'show frequent contacts for an exact identifier', 'fa-address-book', 'Rank interactions from deterministic communication records.'],
    ['Shared locations', 'compare shared verified locations for exact identifiers', 'fa-location-crosshairs', 'Compare only normalized rows containing supported location fields.'],
  ]
  return (
    <div className="case-page-stack">
      <section className="case-panel">
        <div className="case-panel-heading"><div><span className="case-eyebrow">Evidence exploration</span><h2>Relationships, timelines, and movement</h2></div><span className={`case-status ${queryable ? 'case-status--completed' : ''}`}>{queryable ? `${queryable} queryable families` : 'Awaiting queryable data'}</span></div>
        <p className="case-muted">Choose a governed workflow. NexusAI opens deterministic analysis and renders an edge only when typed source data and source-row citations support it.</p>
        <div className="case-action-grid">
          {prompts.map(([label, prompt, icon, detail]) => <Link key={label} className="case-action-card" to={`/app/cases/${encodeURIComponent(caseId)}/ask?prompt=${encodeURIComponent(prompt)}`}><span className="case-action-icon"><i className={`fas ${icon}`} /></span><span><strong>{label}</strong><small>{detail}</small></span><i className="fas fa-arrow-right" /></Link>)}
        </div>
      </section>
      <div className="case-assurance"><i className="fas fa-shield-halved" /><span><strong>Evidence-safe rendering</strong><small>No edge, event, route, or location is inferred without a typed result and cited source rows.</small></span></div>
    </div>
  )
}

function TimelinePage({ caseId, capabilities }) {
  const queryable = countValue(capabilities?.summary?.queryable) || (capabilities?.families || []).filter(family => family.available).length
  const workflows = [
    ['Entity chronology', 'build an evidence-backed entity timeline', 'fa-timeline', 'Order exact observations for one identifier with source references.'],
    ['Temporal activity', 'show temporal activity by hour and day', 'fa-clock', 'Compute activity distributions from normalized event timestamps.'],
    ['Movement sequence', 'show verified movement sequence', 'fa-route', 'Sequence only records containing supported locations or coordinates.'],
    ['ANPR sightings', 'show chronological ANPR sightings', 'fa-car-side', 'Review plate observations ordered by exact recorded time.'],
  ]
  return (
    <div className="case-page-stack">
      <section className="case-panel case-workflow-hero">
        <div className="case-panel-heading"><div><span className="case-eyebrow">Temporal intelligence</span><h2>Chronology grounded in recorded events</h2></div><span className={`case-status ${queryable ? 'case-status--completed' : ''}`}>{queryable ? `${queryable} queryable families` : 'Inventory only'}</span></div>
        <p className="case-muted">Time order, duration, and movement remain deterministic. NexusAI does not interpolate missing events, locations, or routes.</p>
        {queryable ? <div className="case-action-grid">{workflows.map(([label, prompt, icon, detail]) => <Link key={label} className="case-action-card" to={`/app/cases/${encodeURIComponent(caseId)}/ask?prompt=${encodeURIComponent(prompt)}`}><span className="case-action-icon"><i className={`fas ${icon}`} /></span><span><strong>{label}</strong><small>{detail}</small></span><i className="fas fa-arrow-right" /></Link>)}</div> : <EmptyState icon="fa-clock-rotate-left" title="No queryable timeline yet">Register and normalize timestamped evidence before temporal workflows become available.</EmptyState>}
      </section>
      <div className="case-assurance"><i className="fas fa-shield-halved" /><span><strong>No inferred chronology</strong><small>Every event uses a stored timestamp and retains its source evidence locator. Missing intervals remain visibly missing.</small></span></div>
    </div>
  )
}

function MediaPage({ evidence, loading }) {
  const items = Array.isArray(evidence?.items) ? evidence.items : []
  const definitions = [
    ['image', 'Images', ['image', 'photo', 'ocr']],
    ['audio', 'Audio', ['audio', 'speech', 'transcript']],
    ['video', 'Video', ['video', 'cctv']],
  ]
  const groups = definitions.map(([id, label, terms]) => {
    const matches = items.filter(item => terms.some(term => `${item.modality || ''} ${item.detected_type || ''}`.toLowerCase().includes(term)))
    return { id, label, icon: resolveEvidenceModality({ modality: id }).icon, matches }
  })
  const total = groups.reduce((sum, group) => sum + group.matches.length, 0)
  return (
    <div className="case-page-stack">
      <section className="case-panel">
        <div className="case-panel-heading"><div><span className="case-eyebrow">Media evidence</span><h2>Registered media inventory</h2></div><span className="case-status">Capability aware</span></div>
        <p className="case-muted">This module reports what is registered in the active case. Bounded ANPR, face candidates, semantic image vectors, general OCR, Urdu ASR, Roman Urdu derivatives, video sampling, and native TXT/PDF/DOCX extraction are active at their published LIMITED maturity; each result appears only when a source-bound artifact was actually recorded.</p>
        {loading ? <p className="case-muted" role="status">Verifying registered media evidence…</p> : (
          <div className="case-media-grid">
            {groups.map(group => <article className="case-media-card" key={group.id}>
              <span className="case-media-icon"><i className={`fas ${group.icon}`} /></span>
              <div><span>{group.label}</span><strong>{group.matches.length.toLocaleString()}</strong><small>{group.matches.length ? 'Registered evidence · open Evidence/Data for recorded source-bound outputs' : 'No registered evidence in this case'}</small></div>
              <span className={`case-status ${group.matches.length ? 'case-status--processing' : ''}`}>{group.matches.length ? 'Review sources' : 'Not present'}</span>
            </article>)}
          </div>
        )}
        {!loading && total === 0 && <EmptyState icon="fa-photo-film" title="No registered media evidence">Image, audio, and video evidence will appear here after case-scoped registration. This R4 module does not claim unvalidated analysis capability.</EmptyState>}
      </section>
      <div className="case-assurance"><i className="fas fa-circle-info" /><span><strong>Truthful capability boundary</strong><small>Registration proves inventory and provenance. Only persisted, cited derived artifacts establish that a specific source was processed; every model result remains a review-required observation.</small></span></div>
    </div>
  )
}

function AskPage() {
  return (
    <div className="case-page-stack">
      <div className="case-assurance"><i className="fas fa-database" /><span><strong>Deterministic facts first</strong><small>Exact counts, timestamps, durations, identities, locations, and relationships come from governed operations. Narrative synthesis remains bounded to returned evidence.</small></span></div>
      <Suspense fallback={<RouteFallback />}><RecordsIntelligence embedded /></Suspense>
    </div>
  )
}

function JobsPage({ manifest, status }) {
  const statuses = Array.isArray(manifest?.job_statuses) ? manifest.job_statuses : []
  const summary = status?.summary || {}
  return (
    <div className="case-page-stack">
      <div className="case-kpi-grid"><Kpi label="Completed" value={countValue(summary.completed_jobs).toLocaleString()} icon="fa-circle-check" tone="good" /><Kpi label="In progress" value={countValue(summary.evidence_in_flight).toLocaleString()} icon="fa-spinner" /><Kpi label="Failed" value={countValue(summary.failed_jobs).toLocaleString()} icon="fa-triangle-exclamation" tone={countValue(summary.failed_jobs) ? 'warning' : ''} /><Kpi label="Rejected rows" value={countValue(summary.rejected_rows).toLocaleString()} icon="fa-ban" /></div>
      <section className="case-panel">
        <div className="case-panel-heading"><div><span className="case-eyebrow">Processing history</span><h2>Ingest and reprocessing activity</h2></div><span className="case-status">Read only</span></div>
        {statuses.length ? <div className="case-job-grid">{statuses.map(item => <div className="case-job-card" key={item.status}><span className={`case-status case-status--${item.status}`}>{humanize(item.status)}</span><strong>{countValue(item.count).toLocaleString()}</strong><small>Last activity {item.last_activity_at ? new Date(item.last_activity_at).toLocaleString() : 'Not reported'}</small></div>)}</div> : <EmptyState icon="fa-list-check" title="No processing history">This collection has no sidecar ingest or reprocessing jobs.</EmptyState>}
        <div className="case-assurance"><i className="fas fa-clock-rotate-left" /><span><strong>Non-destructive operations</strong><small>Retries preserve prior evidence and job history. Dead-letter replay remains operator-controlled.</small></span></div>
      </section>
    </div>
  )
}

function ReportsPage({ caseId, collectionId, addToast }) {
  const [target, setTarget] = useState('')
  const [includeEvidence, setIncludeEvidence] = useState(true)
  const [generating, setGenerating] = useState(false)
  const [report, setReport] = useState(null)
  const [error, setError] = useState('')
  const requestVersionRef = useRef(0)

  useEffect(() => () => { requestVersionRef.current += 1 }, [])

  const generate = async event => {
    event.preventDefault()
    const requestVersion = ++requestVersionRef.current
    setGenerating(true)
    setError('')
    setReport(null)
    try {
      const data = await recordsApi.forensicCaseReport(
        { caseId, collectionId },
        { target: target.trim(), include_evidence: includeEvidence },
      )
      if (requestVersion === requestVersionRef.current) setReport(data)
    } catch (requestError) {
      if (requestVersion === requestVersionRef.current) setError(requestError.message || 'Report generation failed.')
    } finally {
      if (requestVersion === requestVersionRef.current) setGenerating(false)
    }
  }

  const download = () => {
    if (!report?.markdown) return
    const blob = new Blob([report.markdown], { type: 'text/markdown;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `${caseId}-forensic-report.md`
    anchor.click()
    URL.revokeObjectURL(url)
    addToast('Report exported from this browser session.', 'success')
  }

  return (
    <div className="case-page-stack">
      <section className="case-panel">
        <div className="case-panel-heading">
          <div><span className="case-eyebrow">On-demand reporting</span><h2>Deterministic forensic intelligence report</h2></div>
          <span className="case-status">Not retained</span>
        </div>
        <p className="case-muted">Generate a point-in-time Markdown report from computed aggregates and optional retrieved evidence previews. NexusAI does not save this output as a case artifact.</p>
        <div className="case-report-scope" aria-label="Locked report scope">
          <div><span>Authorized case</span><strong>{caseId}</strong></div>
          <i className="fas fa-link" aria-hidden="true" />
          <div><span>Bound collection</span><strong>{collectionId}</strong></div>
          <span className="case-status case-status--completed"><i className="fas fa-lock" /> Scope locked</span>
        </div>
        <form className="case-report-form" onSubmit={generate}>
          <label><span>Target or subject <small>Optional</small></span><input className="input" value={target} onChange={event => setTarget(event.target.value)} placeholder="e.g. subscriber, device, vehicle, or entity identifier" /></label>
          <label className="case-report-check"><input type="checkbox" checked={includeEvidence} onChange={event => setIncludeEvidence(event.target.checked)} /><span><strong>Include retrieved evidence previews</strong><small>May add source-grounded excerpts and warnings when the knowledge service is unavailable.</small></span></label>
          <button className="btn btn-primary" type="submit" disabled={generating}>{generating ? <><i className="fas fa-spinner fa-spin" /> Generating verified report…</> : <><i className="fas fa-file-circle-check" /> Generate report</>}</button>
        </form>
      </section>

      {error && <div className="alert alert-error" role="alert">Report generation failed: {error}</div>}
      {report && (
        <section className="case-panel" aria-labelledby="report-preview-title">
          <div className="case-panel-heading">
            <div><span className="case-eyebrow">Session result</span><h2 id="report-preview-title">Report preview</h2><p className="case-muted">Generated {report.generated_at ? new Date(report.generated_at).toLocaleString() : 'now'} · {Object.keys(report.sections || {}).length} analytical sections</p></div>
            <button className="btn btn-secondary" type="button" onClick={download}><i className="fas fa-download" /> Export Markdown</button>
          </div>
          {Array.isArray(report.warnings) && report.warnings.length > 0 && <div className="alert alert-warning" role="status">{report.warnings.join(' · ')}</div>}
          <pre className="case-report-preview">{report.markdown || 'The report service returned no Markdown content.'}</pre>
        </section>
      )}
      <div className="case-assurance"><i className="fas fa-shield-halved" /><span><strong>Case-scoped compatibility boundary</strong><small>The active case and collection identifiers are supplied separately by the authorized registry. The server rejects either identifier when it conflicts with the URL case.</small></span></div>
    </div>
  )
}

function SettingsPage({ caseInfo, manifest }) {
  const resources = Array.isArray(manifest?.resources) ? manifest.resources : []
  return (
    <div className="case-page-stack">
      <section className="case-panel">
        <span className="case-eyebrow">Case governance</span><h2>Metadata and retention posture</h2>
        <dl className="case-definition-grid">
          <div><dt>Purpose</dt><dd>{humanize(caseInfo?.purpose)}</dd></div><div><dt>Environment</dt><dd>{humanize(caseInfo?.environment)}</dd></div>
          <div><dt>Status</dt><dd>{humanize(caseInfo?.case_status)}</dd></div><div><dt>Catalog group</dt><dd>{humanize(caseInfo?.visibility)}</dd></div>
          <div><dt>Classification</dt><dd>{humanize(caseInfo?.security_classification)}</dd></div><div><dt>Retention</dt><dd>{humanize(caseInfo?.retention_class)}</dd></div>
          <div><dt>Disposition</dt><dd>{humanize(caseInfo?.cleanup_disposition)}</dd></div><div><dt>Permanent deletions</dt><dd>{manifest?.deletion_count ?? 0}</dd></div>
        </dl>
      </section>
      <section className="case-panel">
        <span className="case-eyebrow">Resource accounting</span><h2>Case inventory across NexusAI services</h2>
        <div className="case-table-wrap" tabIndex="0"><table className="case-table"><thead><tr><th>Resource</th><th>Count</th><th>State</th><th>System of record</th><th>Operator note</th></tr></thead><tbody>{resources.map(item => <tr key={item.resource}><td><strong>{humanize(item.resource)}</strong></td><td>{item.count == null ? 'Not exposed' : countValue(item.count).toLocaleString()}</td><td><span className="case-status">{humanize(item.status)}</span></td><td><code>{item.source}</code></td><td>{item.note || 'No exception reported'}</td></tr>)}</tbody></table></div>
      </section>
      <details className="case-panel case-advanced">
        <summary><span><i className="fas fa-screwdriver-wrench" /> Advanced case controls</span><small>Adapter/model policies, integrations, and retention actions</small></summary>
        <div className="case-advanced-body"><p>Phase 5 exposes these controls as an explicit advanced surface. Merge, archive, retention changes, and permanent deletion are intentionally unavailable until operator policy and approval workflows are implemented.</p><button className="btn btn-secondary" disabled>Destructive cleanup requires approval</button></div>
      </details>
    </div>
  )
}

function AdminPage({ caseInfo, manifest, status }) {
  return (
    <div className="case-page-stack">
      <section className="case-admin-intro">
        <div><span className="case-eyebrow">Governed administration</span><h2>Processing, policy, and resource posture</h2><p>Read-only operational context is available here. Destructive or retention-changing controls remain disabled until explicit policy and approval workflows exist.</p></div>
        <span className="case-status"><i className="fas fa-lock" /> Protected controls</span>
      </section>
      <JobsPage manifest={manifest} status={status} />
      <SettingsPage caseInfo={caseInfo} manifest={manifest} />
    </div>
  )
}

export default function CaseWorkspace() {
  const { section = 'overview' } = useParams()
  const location = useLocation()
  const requestedEvidenceId = new URLSearchParams(location.search).get('evidence') || ''
  const { addToast } = useOutletContext()
  const { activeCase, caseOptions, state: activeCaseState, error: activeCaseError, setActiveCase, refreshActiveCase } = useActiveCase()
  const caseId = activeCase?.caseId || ''
  const caseIdRef = useRef(caseId)
  const [caseInfo, setCaseInfo] = useState(null)
  const [status, setStatus] = useState(null)
  const [capabilities, setCapabilities] = useState(null)
  const [manifest, setManifest] = useState(null)
  const [evidence, setEvidence] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const resolvedSection = CASE_WORKSPACE_COMPATIBILITY[section] || section
  const currentModule = CASE_WORKSPACE_MODULES.find(module => module.id === resolvedSection)

  useEffect(() => { caseIdRef.current = caseId }, [caseId])

  const loadEvidence = useCallback(async (params = {}) => {
    if (!caseId) return
    const requestCaseId = caseId
    const data = await recordsApi.forensicCaseEvidence(caseId, { limit: 25, offset: 0, ...params })
    if (caseIdRef.current === requestCaseId) setEvidence(data)
    return data
  }, [caseId])

  const loadStatus = useCallback(async () => {
    if (!caseId) return
    const requestCaseId = caseId
    const data = await recordsApi.forensicStatus({ collection_id: caseId, limit: 20 })
    if (caseIdRef.current === requestCaseId) setStatus(data)
    return data
  }, [caseId])

  useEffect(() => {
    if (activeCaseState !== 'ready' || !caseId) return
    let active = true
    setLoading(true)
    setError('')
    setCaseInfo(null)
    setStatus(null)
    setCapabilities(null)
    setManifest(null)
    setEvidence(null)
    Promise.all([
      recordsApi.forensicCase(caseId),
      recordsApi.forensicStatus({ collection_id: caseId, limit: 20 }),
      recordsApi.forensicCapabilities({ collection_id: caseId }),
      recordsApi.forensicCaseManifest(caseId),
      recordsApi.forensicCaseEvidence(caseId, { limit: 25, offset: 0 }),
    ]).then(([info, statusData, capabilityData, manifestData, evidenceData]) => {
      if (!active) return
      setCaseInfo(info); setStatus(statusData); setCapabilities(capabilityData); setManifest(manifestData); setEvidence(evidenceData)
    }).catch(requestError => { if (active) setError(requestError.message) }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [activeCaseState, caseId])

  const caseGroups = useMemo(() => caseOptions.reduce((groups, item) => {
    const group = collectionGroup(item)
    groups[group] = [...(groups[group] || []), item]
    return groups
  }, {}), [caseOptions])
  const posture = analysisPosture(status, manifest, loading)
  const switchCase = value => {
    if (value === caseId) return
    setActiveCase(value, resolvedSection)
  }

  if (activeCaseState === 'loading' || activeCaseState === 'idle') {
    return <NexusLoadingState label="Verifying authorized case context…" />
  }
  if (activeCaseState === 'error' || activeCaseState === 'inaccessible' || !activeCase) {
    return (
      <div className="page page--narrow">
        <SharedEmptyState
          state="error"
          eyebrow="Case context"
          title="Case workspace unavailable"
          headingLevel={1}
          body={activeCaseError || 'NexusAI could not resolve an authorized case.'}
          actions={<button type="button" className="btn btn-primary" onClick={() => refreshActiveCase()}>Retry case registry</button>}
        />
      </div>
    )
  }
  if (CASE_WORKSPACE_COMPATIBILITY[section]) return <Navigate to={`/app/cases/${encodeURIComponent(caseId)}/${resolvedSection}${location.search}`} replace />
  if (!currentModule) return <Navigate to={`/app/cases/${encodeURIComponent(caseId)}/overview`} replace />

  return (
    <>
      <style>{`
        .case-page-stack { display: flex; flex-direction: column; gap: var(--spacing-md); min-width: 0; }.case-readiness { display:flex; justify-content:space-between; align-items:center; gap:var(--spacing-lg); border:1px solid var(--color-border); border-left:4px solid var(--color-primary); border-radius:var(--radius-md); padding:var(--spacing-md) var(--spacing-lg); background:linear-gradient(120deg,color-mix(in srgb,var(--color-primary) 7%,var(--color-bg-primary)),var(--color-bg-primary)); }.case-readiness--good{border-left-color:#16a34a}.case-readiness--warning{border-left-color:#d97706}.case-readiness h2{margin:3px 0;font-size:1.05rem}.case-readiness p{margin:0;color:var(--color-text-muted);font-size:.82rem}.case-quick-actions{display:flex;gap:var(--spacing-xs);flex-wrap:wrap}.case-kpi-grid { display: grid; grid-template-columns: repeat(4,minmax(0,1fr)); gap: var(--spacing-sm); }.case-kpi { border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-bg-primary); padding: var(--spacing-md); min-width: 0; }.case-kpi span,.case-kpi small { display: block; color: var(--color-text-muted); font-size: .76rem; }.case-kpi span i{width:18px;color:var(--color-primary)}.case-kpi strong { display:block; margin:5px 0; font-size:1.55rem; overflow-wrap:anywhere; }.case-kpi--warning { border-color: rgba(245,158,11,.45); }.case-kpi--good { border-color: rgba(34,197,94,.35); }
        .case-panel { min-width:0; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-bg-primary); padding:var(--spacing-lg); }.case-panel h2 { margin:3px 0 var(--spacing-sm); font-size:1rem; }.case-panel--warning { border-color:rgba(245,158,11,.42); background:rgba(245,158,11,.06); }.case-panel--warning ul { margin-bottom:0; }.case-panel-heading { display:flex; justify-content:space-between; gap:var(--spacing-sm); align-items:start; flex-wrap:wrap; }.case-eyebrow { color:var(--color-primary); font-size:.7rem; font-weight:800; text-transform:uppercase; letter-spacing:.08em; }.case-muted { color:var(--color-text-muted); line-height:1.5; }.case-two-column { display:grid; grid-template-columns:minmax(0,1.4fr) minmax(260px,.6fr); gap:var(--spacing-lg); align-items:start; }
        .case-family-grid,.case-action-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(210px,1fr)); gap:var(--spacing-sm); }.case-family-card { border:1px solid var(--color-border); border-radius:var(--radius-sm); padding:var(--spacing-sm); background:var(--color-bg-secondary); }.case-family-card strong,.case-family-card span,.case-family-card small { display:block; overflow-wrap:anywhere; }.case-family-card span,.case-family-card small { margin-top:4px; color:var(--color-text-muted); font-size:.76rem; }.case-manifest-summary { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--spacing-sm); margin:var(--spacing-md) 0; }.case-manifest-summary div { padding:var(--spacing-sm); background:var(--color-bg-secondary); border-radius:var(--radius-sm); }.case-manifest-summary span,.case-manifest-summary strong { display:block; }.case-manifest-summary span { color:var(--color-text-muted); font-size:.72rem; }.case-text-link { color:var(--color-primary); text-decoration:none; font-weight:700; font-size:.82rem; }
        .case-table-wrap { max-width:100%; overflow:auto; border:1px solid var(--color-border); border-radius:var(--radius-sm); }.case-table { width:100%; border-collapse:collapse; font-size:.79rem; }.case-table th,.case-table td { padding:9px 10px; border-bottom:1px solid var(--color-border); text-align:left; vertical-align:top; }.case-table th { background:var(--color-bg-secondary); white-space:nowrap; }.case-table td strong,.case-table td small { display:block; }.case-table td small { color:var(--color-text-muted); margin-top:3px; }.case-table code { white-space:normal; overflow-wrap:anywhere; }.case-status { display:inline-flex; border:1px solid var(--color-border); border-radius:999px; padding:3px 8px; font-size:.7rem; font-weight:800; }.case-status--completed { color:#15803d; border-color:rgba(34,197,94,.4); }.case-status--failed,.case-status--dead_letter { color:#b91c1c; border-color:rgba(239,68,68,.4); }.case-status--processing,.case-status--running,.case-status--queued { color:#1d4ed8; border-color:rgba(59,130,246,.4); }
        .case-empty { min-height:150px; display:flex; flex-direction:column; align-items:center; justify-content:center; text-align:center; border:1px dashed var(--color-border); border-radius:var(--radius-md); padding:var(--spacing-lg); color:var(--color-text-muted); }.case-empty>i { font-size:1.5rem; color:var(--color-primary); }.case-empty h2 { color:var(--color-text-primary); margin:var(--spacing-sm) 0 0; }.case-empty p { max-width:620px; }.case-action-card { display:grid; grid-template-columns:auto minmax(0,1fr) auto; gap:var(--spacing-sm); align-items:center; border:1px solid var(--color-border); border-radius:var(--radius-md); padding:var(--spacing-md); color:var(--color-text-primary); text-decoration:none; background:var(--color-bg-primary); transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}.case-action-card:hover { border-color:var(--color-primary); transform:translateY(-1px); box-shadow:var(--shadow-sm) }.case-action-card span,.case-action-card strong,.case-action-card small { display:block; min-width:0; }.case-action-card small { color:var(--color-text-muted); margin-top:5px;line-height:1.4}.case-action-icon{width:36px;height:36px;border-radius:10px;display:grid!important;place-items:center;background:color-mix(in srgb,var(--color-primary) 10%,var(--color-bg-secondary));color:var(--color-primary)}
        .case-job-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(180px,1fr)); gap:var(--spacing-sm); margin:var(--spacing-md) 0; }.case-job-card { border:1px solid var(--color-border); border-radius:var(--radius-sm); padding:var(--spacing-md); }.case-job-card strong,.case-job-card small { display:block; margin-top:8px; }.case-job-card strong { font-size:1.4rem; }.case-job-card small { color:var(--color-text-muted); }.case-definition-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:0 var(--spacing-lg); margin:0; }.case-definition-grid div { display:grid; grid-template-columns:minmax(120px,.4fr) minmax(0,1fr); gap:var(--spacing-sm); padding:9px 0; border-bottom:1px solid var(--color-border); }.case-definition-grid dt { color:var(--color-text-muted); }.case-definition-grid dd { margin:0; font-weight:650; overflow-wrap:anywhere; }.case-advanced summary { cursor:pointer; display:flex; justify-content:space-between; gap:var(--spacing-sm); }.case-advanced summary span,.case-advanced summary small { display:block; }.case-advanced summary small { color:var(--color-text-muted); }.case-advanced-body { padding-top:var(--spacing-md); border-top:1px solid var(--color-border); margin-top:var(--spacing-md); }.case-assurance{display:flex;align-items:flex-start;gap:var(--spacing-sm);padding:var(--spacing-md);border:1px solid var(--color-border);border-radius:var(--radius-md);background:var(--color-bg-secondary)}.case-assurance>i{color:var(--color-primary);margin-top:2px}.case-assurance span,.case-assurance strong,.case-assurance small{display:block}.case-assurance small{color:var(--color-text-muted);margin-top:3px}
        .case-report-scope{display:grid;grid-template-columns:minmax(0,1fr) auto minmax(0,1fr) auto;gap:var(--spacing-sm);align-items:center;margin:var(--spacing-md) 0;padding:var(--spacing-md);border:1px solid var(--color-border);border-radius:var(--radius-md);background:var(--color-bg-secondary)}.case-report-scope div span,.case-report-scope div strong{display:block;overflow-wrap:anywhere}.case-report-scope div span{color:var(--color-text-muted);font-size:.72rem}.case-report-form{display:grid;grid-template-columns:minmax(260px,1fr) minmax(280px,.85fr) auto;gap:var(--spacing-md);align-items:end}.case-report-form>label>span{display:block;margin-bottom:5px;font-size:.78rem;font-weight:700}.case-report-form>label>span small{color:var(--color-text-muted);font-weight:500}.case-report-check{display:flex;gap:var(--spacing-sm);align-items:flex-start}.case-report-check input{margin-top:4px}.case-report-check span,.case-report-check strong,.case-report-check small{display:block}.case-report-check small{color:var(--color-text-muted);font-weight:400;line-height:1.35;margin-top:3px}.case-report-preview{max-height:620px;overflow:auto;white-space:pre-wrap;word-break:break-word;margin:var(--spacing-md) 0 0;padding:var(--spacing-lg);border:1px solid var(--color-border);border-radius:var(--radius-md);background:var(--color-bg-secondary);font:500 .8rem/1.65 var(--font-mono)}
        .case-media-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:var(--spacing-sm);margin:var(--spacing-lg) 0}.case-media-card{display:grid;grid-template-columns:auto minmax(0,1fr);gap:var(--spacing-sm);align-items:start;padding:var(--spacing-md);border:1px solid var(--color-border);border-radius:var(--radius-md);background:linear-gradient(145deg,var(--color-bg-primary),var(--color-bg-secondary))}.case-media-icon{display:grid;width:42px;height:42px;place-items:center;border-radius:12px;color:var(--color-primary);background:color-mix(in srgb,var(--color-primary) 10%,var(--color-bg-primary))}.case-media-card div span,.case-media-card div strong,.case-media-card div small{display:block}.case-media-card div span{font-size:.72rem;color:var(--color-text-muted)}.case-media-card div strong{margin:3px 0;font-size:1.35rem}.case-media-card div small{font-size:.68rem;line-height:1.4;color:var(--color-text-muted)}.case-media-card>.case-status{grid-column:1/-1}.case-admin-intro{display:flex;justify-content:space-between;align-items:center;gap:var(--spacing-lg);padding:var(--spacing-lg);border:1px solid color-mix(in srgb,var(--color-primary) 20%,var(--color-border));border-radius:var(--radius-md);background:linear-gradient(120deg,color-mix(in srgb,var(--color-primary) 7%,var(--color-bg-primary)),var(--color-bg-primary))}.case-admin-intro h2{margin:3px 0;font-size:1.05rem}.case-admin-intro p{max-width:780px;margin:0;color:var(--color-text-muted);font-size:.78rem;line-height:1.5}
        .case-error { margin-bottom:var(--spacing-lg); }
        @media (max-width:1000px){.case-kpi-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.case-two-column,.case-media-grid{grid-template-columns:1fr}.case-report-form{grid-template-columns:1fr}.case-readiness{align-items:flex-start;flex-direction:column}}
        @media (max-width:620px){.case-kpi-grid,.case-definition-grid,.case-report-scope{grid-template-columns:1fr}.case-report-scope>i{display:none}.case-panel{padding:var(--spacing-md)}.case-manifest-summary{grid-template-columns:1fr}.case-table{min-width:720px}.case-admin-intro{align-items:flex-start;flex-direction:column;padding:var(--spacing-md)}}
      `}</style>

      <CaseWorkspaceChrome caseId={caseId} collectionId={activeCase.collectionId} caseInfo={caseInfo} caseGroups={caseGroups} caseOptions={caseOptions} currentModule={currentModule} posture={posture} processingCount={countValue(status?.summary?.evidence_in_flight)} onSwitchCase={switchCase}>
        {error && <div className="alert alert-error case-error" role="alert">Case workspace could not load: {error}</div>}
        {resolvedSection === 'overview' && <OverviewPage caseId={caseId} status={status} capabilities={capabilities} manifest={manifest} loading={loading} />}
        {resolvedSection === 'ask' && <AskPage />}
        {resolvedSection === 'evidence' && <EvidenceWorkspace caseId={caseId} evidence={evidence} status={status} loading={loading} onRefresh={loadEvidence} onRefreshStatus={loadStatus} addToast={addToast} initialEvidenceId={requestedEvidenceId} />}
        {resolvedSection === 'relationships' && <RelationshipsPage caseId={caseId} capabilities={capabilities} />}
        {resolvedSection === 'timeline' && <TimelinePage caseId={caseId} capabilities={capabilities} />}
        {resolvedSection === 'media' && <MediaPage evidence={evidence} loading={loading} />}
        {resolvedSection === 'reports' && <ReportsPage key={caseId} caseId={caseId} collectionId={activeCase.collectionId} addToast={addToast} />}
        {resolvedSection === 'admin' && <AdminPage caseInfo={caseInfo} manifest={manifest} status={status} />}
      </CaseWorkspaceChrome>
    </>
  )
}
