import { createContext, useContext } from 'react'

const CapabilityContext = createContext(null)

export function CapabilityProvider({ capabilities, children }) {
  return <CapabilityContext.Provider value={capabilities}>{children}</CapabilityContext.Provider>
}

export function CapabilityGate({ capability, children }) {
  const provided = useContext(CapabilityContext)
  const runtime = globalThis.window?.__INVESTIGATION_WORKSPACE_CONFIG__?.capabilities
  const capabilities = provided ?? runtime
  if (capabilities && capabilities[capability] === false) {
    return (
      <main id="workspace-main" className="state-page" tabIndex={-1}>
        <p className="eyebrow">Unavailable</p>
        <h1>Questions are not available for this case</h1>
        <p>Your access does not include this case’s investigation results.</p>
      </main>
    )
  }
  return children
}
