import { createContext, useContext } from 'react'
import { Link, useParams } from 'react-router-dom'
import { AppShell } from '../components/CaseShell.jsx'
import { RouteState } from '../components/AnalystComponents.jsx'

const CapabilityContext = createContext(null)

export function CapabilityProvider({ capabilities, children }) {
  return <CapabilityContext.Provider value={capabilities}>{children}</CapabilityContext.Provider>
}

export function CapabilityGate({ capability, children }) {
  const { id = '' } = useParams()
  const provided = useContext(CapabilityContext)
  const runtime = globalThis.window?.__INVESTIGATION_WORKSPACE_CONFIG__?.capabilities
  const capabilities = provided ?? runtime
  if (capabilities && capabilities[capability] === false) {
    return (
      <AppShell caseId={id}>
        <main id="workspace-main" className="state-page" tabIndex={-1}>
          <RouteState state="blocked" label="Questions are not available for this case" description="Your access does not include this case’s investigation results." reference="ACCESS-CAPABILITY">
            <Link to={`/cases/${encodeURIComponent(id)}/overview`}>Back to case overview</Link>
          </RouteState>
        </main>
      </AppShell>
    )
  }
  return children
}
