import { AppShell } from '../components/CaseShell.jsx'
import { RouteState } from '../components/AnalystComponents.jsx'
import { PageHeader } from '../components/PageHeader.jsx'

const routes = ['Dashboard', 'Cases', 'Overview', 'Evidence', 'Investigate', 'Timeline', 'Activity', 'Settings']
const states = ['loading', 'ready', 'empty', 'partial', 'error', 'forbidden', 'unavailable']

export default function RouteStatesPage() {
  return <AppShell><main id="workspace-main" className="catalog-page route-state-proof" tabIndex={-1}><PageHeader eyebrow="Review fixture" title="Route-state coverage" description="All seven required states use an icon, label, and description. Partial retains its completed artifact and names the failed scope." breadcrumbs={[{ label: 'Workspace', to: '/' }, { label: 'Route-state coverage' }]} />{routes.map(route => <section key={route}><h2>{route}</h2><div className="route-state-grid">{states.map(state => <RouteState key={state} state={state} failedScope={state === 'partial' ? 'Audio evidence' : undefined} reference={state === 'error' ? 'FIXTURE-ERROR' : undefined} />)}</div></section>)}</main></AppShell>
}
