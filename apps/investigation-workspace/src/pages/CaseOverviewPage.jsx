import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { CaseShell } from '../components/CaseShell.jsx'
import { getCaseOverview } from '../lib/apiClient.js'
import { formatNumber } from '../lib/format.js'
import { RouteState } from '../components/AnalystComponents.jsx'
import { curatedFamilyLabel } from '../lib/semanticCatalog.js'
import { PageHeader } from '../components/PageHeader.jsx'

export default function CaseOverviewPage() {
  const { id: caseId = '' } = useParams()
  const [state, setState] = useState({ loading: true })
  useEffect(() => {
    const controller = new AbortController()
    getCaseOverview({ caseId, signal: controller.signal })
      .then(data => setState({ data, loading: false }))
      .catch(error => error.name !== 'AbortError' && setState({ error, loading: false }))
    return () => controller.abort()
  }, [caseId])
  const summary = state.data?.summary || {}
  const families = state.data?.record_families || []
  const total = Number(summary.evidence_total || 0)
  const ready = Number(summary.evidence_completed || 0)
  const processing = Number(summary.evidence_in_flight || 0)
  const failed = Number(summary.evidence_failed || 0)
  const notProcessed = Math.max(0, total - ready - processing - failed)
  const quality = { accepted: Number(summary.accepted_rows || 0), rejected: Number(summary.rejected_rows || 0), duplicates: Number(summary.duplicate_rows || 0) }
  const qualityNotes = [
    quality.rejected > 0 ? `${formatNumber(quality.rejected)} structured rows were rejected across the collection.` : '',
    quality.duplicates > 0 ? `${formatNumber(quality.duplicates)} duplicate structured rows were excluded across the collection.` : '',
    failed > 0 ? `${formatNumber(failed)} evidence item${failed === 1 ? '' : 's'} failed processing and ${failed === 1 ? 'requires' : 'require'} review in the Evidence catalog.` : '',
    processing > 0 ? `${formatNumber(processing)} evidence item${processing === 1 ? '' : 's'} ${processing === 1 ? 'is' : 'are'} still processing and excluded from complete analysis.` : '',
    Number(summary.completed_jobs_missing_kb_asset || 0) > 0 ? `${formatNumber(summary.completed_jobs_missing_kb_asset)} completed ingest job${Number(summary.completed_jobs_missing_kb_asset) === 1 ? '' : 's'} ${Number(summary.completed_jobs_missing_kb_asset) === 1 ? 'has' : 'have'} no retained knowledge-base asset.` : '',
  ].filter(Boolean)
  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="catalog-page" tabIndex={-1}>
        <PageHeader
          eyebrow="Case overview"
          title="Case overview"
          description="Collection-backed evidence status and readiness."
          meta={state.data ? [
            { label: 'Case', value: caseId, identifier: true },
            { label: 'Evidence', value: formatNumber(summary.evidence_total) },
            { label: 'Structured rows', value: formatNumber(summary.accepted_rows) },
          ] : []}
          actions={<><Link className="page-header__cta" to={`/cases/${encodeURIComponent(caseId)}/investigate`}>Ask about this case</Link><Link className="page-header__secondary" to={`/cases/${encodeURIComponent(caseId)}/evidence#add-evidence`}>Add evidence</Link></>}
        />
        {state.loading && <RouteState state="loading" label="Loading case status" />}
        {state.error && <RouteState state={state.error.status === 403 ? 'forbidden' : 'error'} label={state.error.status === 403 ? 'Case overview is forbidden' : 'Case overview could not be loaded'} reference={state.error.reference || 'OVERVIEW'} />}
        {state.data && (
          <>
            <section className="readiness-panel" aria-labelledby="readiness-title">
              <div className="readiness-panel__total"><span id="readiness-title">Evidence readiness</span><strong>{formatNumber(total)}</strong><small>evidence files reported by the collection status</small></div>
              <dl className="readiness-panel__breakdown">
                <div><dt>Ready</dt><dd>{formatNumber(ready)}</dd><small>Available for analysis</small></div>
                <div><dt>Processing</dt><dd>{formatNumber(processing)}</dd><small>Excluded until ready</small></div>
                <div><dt>Failed</dt><dd>{formatNumber(failed)}</dd><small>Requires evidence review</small></div>
                <div><dt>Not processed</dt><dd>{formatNumber(notProcessed)}</dd><small>No completed processing state</small></div>
              </dl>
            </section>
            <section className="quality-notes" aria-labelledby="quality-notes-title"><h2 id="quality-notes-title">Data-quality notes</h2>{qualityNotes.length ? <ul>{qualityNotes.map(note => <li key={note}>{note}</li>)}</ul> : <p>No data-quality exceptions were reported by the collection-status response.</p>}</section>
            <section className="catalog-section structured-composition">
              <div className="section-title-row"><div><h2>Structured records</h2><p><strong>{formatNumber(quality.accepted)}</strong> accepted rows across the reported record families</p></div><Link to={`/cases/${encodeURIComponent(caseId)}/evidence`}>Browse evidence files</Link></div>
              {families.length ? <ul className="family-summary">{families.map(family => <li key={family.record_type}><strong>{curatedFamilyLabel(family.record_type)}</strong><span>{formatNumber(family.accepted_rows)} accepted rows</span></li>)}</ul> : <p>No structured evidence families were reported.</p>}
            </section>
          </>
        )}
      </main>
    </CaseShell>
  )
}
