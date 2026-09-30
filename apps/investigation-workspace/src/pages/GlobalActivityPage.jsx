import { Link } from 'react-router-dom'
import { EmptyState, LanguageText, RouteState } from '../components/AnalystComponents.jsx'
import { AppShell } from '../components/CaseShell.jsx'
import { PageHeader } from '../components/PageHeader.jsx'
import { configuredCaseIds } from '../lib/apiClient.js'
import { formatNumber } from '../lib/format.js'

export default function GlobalActivityPage() {
  const cases = configuredCaseIds()

  return (
    <AppShell>
      <main id="workspace-main" className="catalog-page global-activity-page" tabIndex={-1}>
        <PageHeader
          eyebrow="Workspace activity"
          title="Activity"
          description="Open a case working record. An authoritative workspace-wide activity history is not available from the current service."
          breadcrumbs={[{ label: 'Workspace', to: '/' }, { label: 'Activity' }]}
          meta={cases.length ? [{ label: 'Available working records', value: `${cases.length} configured case${cases.length === 1 ? '' : 's'}` }] : []}
        />

        <div className="global-activity__layout">
          <section className="global-activity__main" aria-labelledby="global-activity-title">
            <RouteState
              state="unavailable"
              label="Workspace-wide audit history is not available"
              description="The service does not provide authorized event rows across cases and viewers. The available total cannot show who acted, what happened or when."
            >
              <p>Use a case working record for browser-saved questions and the recent evidence sample returned for that collection.</p>
            </RouteState>

            <header className="global-activity__section-header">
              <div>
                <p className="eyebrow">Available now</p>
                <h2 id="global-activity-title">Continue within a case</h2>
                <p>Each link stays inside one configured collection. It does not widen scope across cases.</p>
              </div>
              {cases.length ? <span>{formatNumber(cases.length)}</span> : null}
            </header>

            {cases.length ? (
              <ul className="global-activity__cases" aria-label="Case working records">
                {cases.map(caseId => (
                  <li key={caseId}>
                    <div>
                      <LanguageText as="bdi" identifier>{caseId}</LanguageText>
                      <span>Browser working record · case scoped</span>
                    </div>
                    <Link to={`/cases/${encodeURIComponent(caseId)}/activity`}>Open working record</Link>
                  </li>
                ))}
              </ul>
            ) : (
              <EmptyState kind="not-processed" label="No case working records are available" description="Add the first evidence file to establish a collection-backed case workspace.">
                <Link to="/cases/new">Add evidence</Link>
              </EmptyState>
            )}
          </section>

          <aside className="global-activity__boundary" aria-labelledby="activity-boundary-title">
            <p className="eyebrow">Record boundary</p>
            <h2 id="activity-boundary-title">What is missing</h2>
            <p>A workspace audit record needs authorized event entries with enough context to verify each one.</p>
            <dl>
              <div><dt>Who</dt><dd>Viewer or service identity</dd></div>
              <div><dt>What</dt><dd>Event type and outcome</dd></div>
              <div><dt>When</dt><dd>Authoritative timestamp</dd></div>
              <div><dt>Where</dt><dd>Exact case and source</dd></div>
            </dl>
            <p className="global-activity__boundary-note">Custody events, exports and team-wide viewer activity remain absent until the service provides them with an authorization decision.</p>
            <Link to="/cases">Open case directory</Link>
          </aside>
        </div>
      </main>
    </AppShell>
  )
}
