import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { CircleAlert } from 'lucide-react'
import { AppShell } from '../components/CaseShell.jsx'
import { EmptyState } from '../components/AnalystComponents.jsx'
import { ActivityChart } from './dashboard/ActivityChart.jsx'
import { AttentionCauses } from './dashboard/AttentionCauses.jsx'
import { CasesTable } from './dashboard/CasesTable.jsx'
import { ContinueCards } from './dashboard/ContinueCards.jsx'
import { DashboardHeader } from './dashboard/DashboardHeader.jsx'
import { EvidenceMap } from './dashboard/EvidenceMap.jsx'
import { KpiTiles } from './dashboard/KpiTiles.jsx'
import { PipelineFlow } from './dashboard/PipelineFlow.jsx'
import { configuredCaseIds, getQueryCapabilities } from '../lib/apiClient.js'
import { caseSparks, familyOrder } from '../lib/caseActivity.js'
import { attentionItems, familyRows, reviewByCase } from '../lib/dashboardCharts.js'
import { curatedQuestions, latestEvidenceActivity, parseTimestamp } from '../lib/dashboardCases.js'
import { useCaseActivity } from '../lib/useCaseActivity.js'
import { formatNumber } from '../lib/format.js'
import { curatedFamilyLabel } from '../lib/semanticCatalog.js'
import { summariseCase, useConfiguredCaseOverviews } from '../lib/useCaseOverview.js'
import { useQuestionHistoryAcross } from '../lib/workspaceState.js'

export { curatedQuestions, dashboardNextAction } from '../lib/dashboardCases.js'

export function aggregateFamilyRows(rows) {
  const totals = new Map()
  for (const row of rows) {
    const seen = new Set()
    for (const family of row.summary?.families || []) {
      const id = String(family.record_type || '').trim()
      const value = Number(family.accepted_rows || 0)
      if (!id || !Number.isFinite(value) || value <= 0) continue
      const current = totals.get(id) || { id, value: 0, caseCount: 0 }
      current.value += value
      if (!seen.has(id)) {
        current.caseCount += 1
        seen.add(id)
      }
      totals.set(id, current)
    }
  }
  return [...totals.values()].sort((left, right) => right.value - left.value || left.id.localeCompare(right.id))
}

export function dashboardRowState(state) {
  if (state?.error) return 'unavailable'
  if (!state?.data) return 'loading'
  return summariseCase(state.data).processingState
}

// Workspace-level figures, each with the rows behind it. Only reported cases contribute, and the
// count of cases that did not report is carried so the page can say so instead of under-counting silently.
export function dashboardKpis(rows) {
  const reported = rows.filter(row => row.summary)
  const sum = key => reported.reduce((total, row) => total + (row.summary[key] || 0), 0)
  return {
    cases: rows.length,
    reported: reported.length,
    unreported: rows.length - reported.length,
    sources: sum('total'),
    ready: sum('ready'),
    processing: sum('inFlight'),
    failed: sum('failed'),
    gaps: sum('missingAssets'),
    review: sum('failed') + sum('missingAssets'),
    reviewCases: reported.filter(row => row.summary.failed + row.summary.missingAssets > 0).length,
    acceptedRows: sum('acceptedRows'),
    duplicateRows: sum('duplicateRows'),
    rejectedRows: sum('rejectedRows'),
  }
}

// The command dashboard. Zones, top to bottom: header, four metric cards, then the hero (activity and what needs
// review), then the evidence pair (what it is made of, where it ends up), then the cases table and the two ways back into
// the work. Every figure is a real count from the service; anything it cannot supply says so. See
// docs/work/UI_PAGE_SPECS/dashboard-architecture.md.
export default function DashboardPage() {
  const cases = configuredCaseIds()
  const recent = useQuestionHistoryAcross(cases).slice(0, 4)
  const { states, reload } = useConfiguredCaseOverviews(cases)
  const [selectedFamily, setSelectedFamily] = useState('')
  const [suggestions, setSuggestions] = useState({ caseId: '', loading: false, items: [] })

  const rows = useMemo(() => cases.map((caseId, index) => {
    const state = states[caseId]
    return { caseId, index, state, status: dashboardRowState(state), summary: state?.data ? summariseCase(state.data) : null, activity: state?.data ? latestEvidenceActivity(state.data) : null }
  }), [cases, states])
  const kpis = useMemo(() => dashboardKpis(rows), [rows])
  const loading = rows.some(row => row.status === 'loading')
  const unreadable = rows.filter(row => row.status === 'unavailable').length
  const families = useMemo(() => familyRows(aggregateFamilyRows(rows)), [rows])
  const attention = useMemo(() => attentionItems(rows), [rows])
  const reviewCases = useMemo(() => reviewByCase(rows), [rows])
  const isRefreshing = rows.some(row => row.state?.refreshing)
  const hasProcessing = rows.some(row => row.status === 'processing')
  // The age of the figures is the age of the oldest read among the cases shown; the newest would hide a case whose refresh failed.
  const lastUpdated = rows.filter(row => row.summary).map(row => parseTimestamp(row.state?.receivedAt)).filter(Boolean).sort((left, right) => left - right)[0] || null

  // Scope lives in the URL so a view can be shared and survives reload; absent means every case. An unknown case is ignored.
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedScope = searchParams.get('scope') || ''
  const scope = cases.includes(requestedScope) ? requestedScope : ''
  const setScope = useCallback(next => setSearchParams(current => { const params = new URLSearchParams(current); if (next) params.set('scope', next); else params.delete('scope'); return params }, { replace: true }), [setSearchParams])
  const [activityToken, setActivityToken] = useState(0)
  const activity = useCaseActivity(scope ? [scope] : cases.slice(0, 8), { refreshToken: activityToken })
  const refreshAll = useCallback(() => { reload(); setActivityToken(token => token + 1) }, [reload])
  const order = useMemo(() => familyOrder(families.map(family => family.id), activity.activity.families.map(family => family.id)), [families, activity.activity.families])
  const sparks = useMemo(() => caseSparks(activity.activity), [activity.activity])
  const familyCases = useMemo(() => (selectedFamily ? rows.filter(row => row.summary?.families.some(family => family.record_type === selectedFamily && Number(family.accepted_rows || 0) > 0)) : rows), [rows, selectedFamily])

  // Suggested questions come from one case: the one in scope, else the first with ready evidence.
  const suggestionCase = scope || rows.find(row => row.status === 'complete')?.caseId || ''
  useEffect(() => {
    if (!suggestionCase) {
      setSuggestions({ caseId: '', loading: false, items: [] })
      return undefined
    }
    const controller = new AbortController()
    setSuggestions({ caseId: suggestionCase, loading: true, items: [] })
    getQueryCapabilities({ caseId: suggestionCase, signal: controller.signal })
      .then(data => setSuggestions({ caseId: suggestionCase, loading: false, items: curatedQuestions(data) }))
      .catch(error => { if (error.name !== 'AbortError') setSuggestions({ caseId: suggestionCase, loading: false, items: [] }) })
    return () => controller.abort()
  }, [suggestionCase])

  useEffect(() => {
    if (!hasProcessing) return undefined
    const refreshWhenVisible = () => { if (globalThis.document?.visibilityState === 'visible') reload() }
    const timer = globalThis.setInterval(refreshWhenVisible, 15_000)
    globalThis.document?.addEventListener('visibilitychange', refreshWhenVisible)
    return () => {
      globalThis.clearInterval(timer)
      globalThis.document?.removeEventListener('visibilitychange', refreshWhenVisible)
    }
  }, [hasProcessing, reload])

  return (
    <AppShell identityLed>
      <main id="workspace-main" className="catalog-page dashboard-page dashboard-command" tabIndex={-1}>
        <DashboardHeader kpis={kpis} loading={loading} lastUpdated={lastUpdated} processing={hasProcessing} refreshing={isRefreshing} onRefresh={refreshAll} />

        {cases.length ? (
          <>
            <KpiTiles kpis={kpis} loading={loading} />
            {unreadable > 0 ? <p className="dash-notice" role="status"><CircleAlert aria-hidden="true" />{unreadable === cases.length ? 'No case could report status. Check that the records API is running.' : `${formatNumber(unreadable)} of ${formatNumber(cases.length)} ${unreadable === 1 ? 'case' : 'cases'} could not report status, so totals above cover the rest.`} <button type="button" onClick={reload}>Try again</button></p> : null}

            <div className="dash-grid dash-grid--hero">
              <ActivityChart activity={activity.activity} status={activity.status} failures={activity.failures} order={order} cases={cases} scope={scope} onScope={setScope} onRetry={() => setActivityToken(token => token + 1)} />
              <AttentionCauses items={attention} byCase={reviewCases} kpis={kpis} loading={loading} />
            </div>

            <div className="dash-grid dash-grid--even">
              <EvidenceMap families={families} order={order} selectedId={selectedFamily} onSelect={setSelectedFamily} kpis={kpis} loading={loading} />
              <PipelineFlow kpis={kpis} loading={loading} />
            </div>

            <CasesTable rows={familyCases} sparks={sparks} familyLabel={selectedFamily ? curatedFamilyLabel(selectedFamily) : ''} onClearFamily={() => setSelectedFamily('')} loading={loading} />

            <ContinueCards recent={recent} suggestions={suggestions.items} suggestionCase={suggestions.caseId} suggestionsLoading={suggestions.loading} />

            <p className="dash-scope"><CircleAlert aria-hidden="true" />Covers the {formatNumber(cases.length)} case{cases.length === 1 ? '' : 's'} configured for this workspace. Assignment, severity and investigative priority are not available. <Link to="/cases">Open all cases</Link></p>
          </>
        ) : (
          <EmptyState kind="not-processed" label="No cases are configured" description="Add the first evidence file to establish a collection-backed case."><Link to="/cases/new">Add evidence</Link></EmptyState>
        )}
      </main>
    </AppShell>
  )
}
