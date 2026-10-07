import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { CapabilityGate } from './shell/CapabilityGate.jsx'
import { RouteErrorBoundary } from './components/ErrorBoundary.jsx'
import { ShellSkeleton } from './components/Skeleton.jsx'

const routeLoaders = {
  dashboard: () => import('./pages/DashboardPage.jsx'),
  globalInvestigate: () => import('./pages/GlobalInvestigatePage.jsx'),
  cases: () => import('./pages/CasesPage.jsx'),
  newCase: () => import('./pages/NewCasePage.jsx'),
  overview: () => import('./pages/CaseOverviewPage.jsx'),
  evidence: () => import('./pages/EvidenceListPage.jsx'),
  evidenceDetail: () => import('./pages/EvidenceDetailPage.jsx'),
  investigate: () => import('./pages/InvestigatePage.jsx'),
  timeline: () => import('./pages/TimelinePage.jsx'),
  activity: () => import('./pages/ActivityPage.jsx'),
  globalActivity: () => import('./pages/GlobalActivityPage.jsx'),
  settings: () => import('./pages/SettingsPage.jsx'),
  typeProof: () => import('./pages/TypeProofPage.jsx'),
  routeStates: () => import('./pages/RouteStatesPage.jsx'),
}
const DashboardPage = lazy(routeLoaders.dashboard)
const GlobalInvestigatePage = lazy(routeLoaders.globalInvestigate)
const CasesPage = lazy(routeLoaders.cases)
const NewCasePage = lazy(routeLoaders.newCase)
const CaseOverviewPage = lazy(routeLoaders.overview)
const EvidenceListPage = lazy(routeLoaders.evidence)
const EvidenceDetailPage = lazy(routeLoaders.evidenceDetail)
const InvestigatePage = lazy(routeLoaders.investigate)
const TimelinePage = lazy(routeLoaders.timeline)
const ActivityPage = lazy(routeLoaders.activity)
const GlobalActivityPage = lazy(routeLoaders.globalActivity)
const SettingsPage = lazy(routeLoaders.settings)
const TypeProofPage = lazy(routeLoaders.typeProof)
const RouteStatesPage = lazy(routeLoaders.routeStates)

export function preloadRoute(name) {
  return routeLoaders[name]?.()
}

export function WorkspaceRouter() {
  function focusMain(event) {
    event.preventDefault()
    const main = globalThis.document?.getElementById('workspace-main')
    main?.scrollIntoView({ block: 'start' })
    main?.focus({ preventScroll: true })
  }
  return (
    <BrowserRouter>
      <a className="skip-link" href="#workspace-main" onClick={focusMain}>Skip to investigation</a>
      {/* Outside Suspense, so a page chunk that fails to load is caught too. */}
      <RouteErrorBoundary>
      <Suspense fallback={<ShellSkeleton />}>
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/investigate" element={<GlobalInvestigatePage />} />
          <Route path="/cases" element={<CasesPage />} />
          <Route path="/activity" element={<GlobalActivityPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          {/* Declared before the :id routes so "new" is never parsed as a case id. */}
          <Route path="/cases/new" element={<NewCasePage />} />
          <Route path="/cases/:id/overview" element={<CaseOverviewPage />} />
          <Route path="/cases/:id/evidence" element={<EvidenceListPage />} />
          <Route path="/cases/:id/evidence/:eid" element={<EvidenceDetailPage />} />
          <Route path="/cases/:id/investigate" element={(
            <CapabilityGate capability="hybrid_query">
              <InvestigatePage />
            </CapabilityGate>
          )} />
          <Route path="/cases/:id/timeline" element={<TimelinePage />} />
          <Route path="/cases/:id/activity" element={<ActivityPage />} />
          <Route path="/design/type-proof" element={<TypeProofPage />} />
          <Route path="/design/route-states" element={<RouteStatesPage />} />
          <Route path="*" element={<Navigate replace to="/" />} />
        </Routes>
      </Suspense>
      </RouteErrorBoundary>
    </BrowserRouter>
  )
}
