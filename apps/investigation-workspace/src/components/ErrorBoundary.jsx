import { Component } from 'react'
import { useLocation } from 'react-router-dom'
import { RouteState } from './AnalystComponents.jsx'

// An error while rendering a page must not take the whole workspace with it. React unmounts the entire tree when nothing
// catches the error, and the analyst is left with a blank page that says nothing (docs/ux/NEXUSAI_PRODUCT_UX.md: "Never a
// blank page"). The error is still written to the console, with a fixed prefix, for whoever opens the developer tools.
export function RenderFailure() {
  return (
    <main id="workspace-main" className="state-page" tabIndex={-1}>
      <RouteState
        state="error"
        label="This page could not be displayed"
        description="Your case evidence was not changed. Reload the page to continue."
        reference="UI-RENDER"
      >
        <button type="button" onClick={() => globalThis.location.reload()}>Reload this page</button>
        <a href="/">Back to the dashboard</a>
      </RouteState>
    </main>
  )
}

// `resetKey` clears a caught error when it changes, so moving to another page works without a reload.
export class ErrorBoundary extends Component {
  state = { error: null }

  static getDerivedStateFromError(error) {
    return { error }
  }

  componentDidCatch(error, info) {
    console.error('[workspace] a page failed to render', error, info?.componentStack)
  }

  componentDidUpdate(previous) {
    if (this.state.error && previous.resetKey !== this.props.resetKey) this.setState({ error: null })
  }

  render() {
    if (!this.state.error) return this.props.children
    return this.props.fallback ?? <RenderFailure />
  }
}

// The boundary for the routes: a page that fails is replaced by the message, and the next page the analyst opens renders.
export function RouteErrorBoundary({ children }) {
  const { pathname } = useLocation()
  return <ErrorBoundary resetKey={pathname}>{children}</ErrorBoundary>
}
