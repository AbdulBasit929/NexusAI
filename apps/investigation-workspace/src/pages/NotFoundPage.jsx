import { Link } from 'react-router-dom'
import { AppShell } from '../components/CaseShell.jsx'
import { RouteState } from '../components/AnalystComponents.jsx'

export default function NotFoundPage() {
  return (
    <AppShell>
      <main id="workspace-main" className="state-page" tabIndex={-1}>
        <RouteState state="empty" label="Page not found" description="This workspace address does not match an available NexusAI page." reference="UI-404">
          <Link to="/">Back to the dashboard</Link>
          <Link to="/cases">Open cases</Link>
        </RouteState>
      </main>
    </AppShell>
  )
}
