import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { CaseShell } from '../components/CaseShell.jsx'
import { RouteState } from '../components/AnalystComponents.jsx'
import { PageHeader } from '../components/PageHeader.jsx'
import { ProportionBar } from '../components/DataVisualizations.jsx'
import { ActivityChart } from './dashboard/ActivityChart.jsx'
import { AttentionCauses } from './dashboard/AttentionCauses.jsx'
import { ContinueCards } from './dashboard/ContinueCards.jsx'
import { EvidenceMap } from './dashboard/EvidenceMap.jsx'
import { KpiTiles } from './dashboard/KpiTiles.jsx'
import { PipelineFlow } from './dashboard/PipelineFlow.jsx'
import { getQueryCapabilities } from '../lib/apiClient.js'
import { familyOrder } from '../lib/caseActivity.js'
import { aggregateFamilyRows, curatedQuestions, dashboardKpis, dashboardRowState, latestEvidenceActivity } from '../lib/dashboardCases.js'
import { attentionItems, familyRows, formatRate, reviewByCase } from '../lib/dashboardCharts.js'
import { formatNumber } from '../lib/format.js'
import { summariseCase, useCaseOverview } from '../lib/useCaseOverview.js'
import { useCaseActivity } from '../lib/useCaseActivity.js'
import { useQuestionHistory } from '../lib/workspaceState.js'

const TARGETS = { ready: '#case-quality', processing: '#case-quality' }

// Data-quality notes, each a plain sentence from the collection-wide summary counts (never the bounded recent lists).
export function qualityNotes(summary, counts) {
  const missing = Number(summary.completed_jobs_missing_kb_asset || 0)
  return [
    counts.rejected > 0 ? `${formatNumber(counts.rejected)} structured rows were rejected across the collection.` : '',
    counts.duplicates > 0 ? `${formatNumber(counts.duplicates)} duplicate structured rows were excluded across the collection.` : '',
    counts.failed > 0 ? `${formatNumber(counts.failed)} evidence item${counts.failed === 1 ? '' : 's'} failed processing and ${counts.failed === 1 ? 'requires' : 'require'} review in the Evidence catalog.` : '',
    counts.processing > 0 ? `${formatNumber(counts.processing)} evidence item${counts.processing === 1 ? '' : 's'} ${counts.processing === 1 ? 'is' : 'are'} still processing and excluded from complete analysis.` : '',
    missing > 0 ? `${formatNumber(missing)} completed ingest job${missing === 1 ? '' : 's'} ${missing === 1 ? 'is' : 'are'} missing a retained evidence copy.` : '',
  ].filter(Boolean)
}

function QualityCard({ caseId, counts, notes }) {
  const read = counts.accepted + counts.duplicates + counts.rejected
  return (
    <section id="case-quality" className="cases-card" aria-labelledby="quality-notes-title">
      <header className="cases-card__header">
        <div><h2 id="quality-notes-title">Data-quality notes</h2><p>{read ? 'Did ingestion keep every row? Shares are of all rows read.' : 'No structured rows have been read yet.'}</p></div>
      </header>
      {read ? (
        <div className="cq-rows">
          <ProportionBar total={read} ready={counts.accepted} processing={counts.duplicates} failed={counts.rejected} label={`${caseId} structured rows`} />
          <dl className="cq-figures">
            <div><dt>Accepted</dt><dd>{formatNumber(counts.accepted)}</dd></div>
            <div><dt>Duplicate</dt><dd>{formatNumber(counts.duplicates)}<small>{formatRate(counts.duplicates / read)}</small></dd></div>
            <div className={counts.rejected ? 'is-flagged' : undefined}><dt>Rejected</dt><dd>{formatNumber(counts.rejected)}<small>{formatRate(counts.rejected / read)}</small></dd></div>
          </dl>
        </div>
      ) : null}
      {notes.length ? <ul className="cq-notes">{notes.map(note => <li key={note}>{note}</li>)}</ul> : <p className="dash-card__empty">No data-quality issues are reported for this case.</p>}
      <p className="cq-foot">{formatNumber(counts.accepted)} accepted rows across the reported record families. <Link to={`/cases/${encodeURIComponent(caseId)}/evidence`}>Open the evidence catalog</Link></p>
    </section>
  )
}

// The case dossier: the dashboard's own widgets, scoped to one case, so what an analyst learned on the dashboard reads the
// same here. Metrics, when activity happened, what needs review, what the evidence is made of and where it ends up, data
// quality, and the ways back into the work. Every figure is the collection's own summary count.
export default function CaseOverviewPage() {
  const { id: caseId = '' } = useParams()
  const navigate = useNavigate()
  const state = useCaseOverview(caseId)
  const [suggestions, setSuggestions] = useState({ loading: false, items: [] })
  const [activityToken, setActivityToken] = useState(0)
  const activity = useCaseActivity([caseId], { refreshToken: activityToken })
  const recent = useQuestionHistory(caseId).slice(0, 4).map(entry => ({ ...entry, caseId }))

  const row = useMemo(() => {
    const summary = state.data ? summariseCase(state.data) : null
    return { caseId, index: 0, state, status: dashboardRowState(state), summary, activity: state.data ? latestEvidenceActivity(state.data) : null }
  }, [caseId, state])
  const kpis = useMemo(() => dashboardKpis([row]), [row])
  const families = useMemo(() => familyRows(aggregateFamilyRows([row])), [row])
  const items = useMemo(() => attentionItems([row]), [row])
  const byCase = useMemo(() => reviewByCase([row]), [row])
  const order = useMemo(() => familyOrder(families.map(family => family.id), activity.activity.families.map(family => family.id)), [families, activity.activity.families])

  useEffect(() => {
    if (row.status !== 'complete' && row.status !== 'attention' && row.status !== 'processing') return undefined
    const controller = new AbortController()
    setSuggestions({ loading: true, items: [] })
    getQueryCapabilities({ caseId, signal: controller.signal })
      .then(data => setSuggestions({ loading: false, items: curatedQuestions(data) }))
      .catch(error => { if (error.name !== 'AbortError') setSuggestions({ loading: false, items: [] }) })
    return () => controller.abort()
  }, [caseId, row.status])

  const summary = state.data?.summary || {}
  const counts = {
    accepted: Number(summary.accepted_rows || 0),
    rejected: Number(summary.rejected_rows || 0),
    duplicates: Number(summary.duplicate_rows || 0),
    failed: Number(summary.evidence_failed || 0),
    processing: Number(summary.evidence_in_flight || 0),
  }
  const encoded = encodeURIComponent(caseId)

  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="catalog-page dashboard-page dashboard-command" tabIndex={-1}>
        <PageHeader
          eyebrow="Case overview"
          title="Case overview"
          description="Collection-backed evidence status and readiness."
          meta={state.data ? [
            { label: 'Case', value: caseId, identifier: true },
            { label: 'Evidence', value: formatNumber(summary.evidence_total) },
            { label: 'Structured rows', value: formatNumber(summary.accepted_rows) },
          ] : []}
          actions={<><Link className="page-header__cta" to={`/cases/${encoded}/investigate`}>Ask about this case</Link><Link className="page-header__secondary" to={`/cases/${encoded}/evidence#add-evidence`}>Add evidence</Link></>}
        />
        {state.loading && <RouteState state="loading" label="Loading case status" />}
        {state.error && <RouteState state={state.error.status === 403 ? 'forbidden' : 'error'} label={state.error.status === 403 ? 'Case overview is forbidden' : 'Case overview could not be loaded'} reference={state.error.reference} />}
        {state.data && (
          <>
            <KpiTiles kpis={kpis} loading={false} targets={TARGETS} label="Case totals" />

            <div className="dash-grid dash-grid--hero">
              <ActivityChart activity={activity.activity} status={activity.status} failures={activity.failures} order={order} cases={[caseId]} scope={caseId} onScope={() => {}} onRetry={() => setActivityToken(token => token + 1)} />
              <AttentionCauses items={items} byCase={byCase} kpis={kpis} loading={false} />
            </div>

            <div className="dash-grid dash-grid--even">
              <EvidenceMap families={families} order={order} selectedId="" onSelect={id => { if (id) navigate(`/cases/${encoded}/evidence?family=${encodeURIComponent(id)}`) }} kpis={kpis} loading={false} pickLabel="Open evidence of type" pickText="Open evidence" />
              <PipelineFlow kpis={kpis} loading={false} />
            </div>

            <QualityCard caseId={caseId} counts={counts} notes={qualityNotes(summary, counts)} />

            <ContinueCards recent={recent} suggestions={suggestions.items} suggestionCase={caseId} suggestionsLoading={suggestions.loading} />
          </>
        )}
      </main>
    </CaseShell>
  )
}
