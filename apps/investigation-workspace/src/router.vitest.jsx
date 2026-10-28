import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Link, MemoryRouter, Route, Routes } from 'react-router-dom'
import { RouteEffects } from './router.jsx'
import NotFoundPage from './pages/NotFoundPage.jsx'
import { CapabilityGate, CapabilityProvider } from './shell/CapabilityGate.jsx'
import { resetAttentionCache } from './lib/workspaceAttention.js'

afterEach(() => {
  resetAttentionCache()
  vi.restoreAllMocks()
  delete window.__INVESTIGATION_WORKSPACE_CONFIG__
})

describe('U2 global route behavior', () => {
  it('updates the title, announces navigation and moves focus to the destination main', async () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <RouteEffects />
        <Routes>
          <Route path="/" element={<main id="workspace-main" tabIndex={-1}><Link to="/settings">Settings</Link></main>} />
          <Route path="/settings" element={<main id="workspace-main" tabIndex={-1}>Settings page</main>} />
        </Routes>
      </MemoryRouter>,
    )
    expect(document.title).toBe('Dashboard · NexusAI')
    await userEvent.click(screen.getByRole('link', { name: 'Settings' }))
    await waitFor(() => expect(document.title).toBe('Settings · NexusAI'))
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('main')))
    expect(screen.getByRole('status')).toHaveTextContent('Loaded Settings')
  })

  it('renders an actionable not-found state instead of redirecting', () => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: [] }
    render(<MemoryRouter><NotFoundPage /></MemoryRouter>)
    expect(screen.getByRole('heading', { name: 'Page not found' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Back to the dashboard' })).toHaveAttribute('href', '/')
    expect(screen.getByText('UI-404')).toBeTruthy()
  })

  it('keeps access denial inside the case shell with a safe return route', () => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a'] }
    globalThis.fetch = vi.fn(async () => ({ ok: true, status: 200, json: async () => ({ summary: { evidence_total: 1, evidence_completed: 1 } }), text: async () => '{}' }))
    render(
      <MemoryRouter initialEntries={['/cases/case-a/investigate']}>
        <CapabilityProvider capabilities={{ hybrid_query: false }}>
          <Routes><Route path="/cases/:id/investigate" element={<CapabilityGate capability="hybrid_query"><p>Secret result</p></CapabilityGate>} /></Routes>
        </CapabilityProvider>
      </MemoryRouter>,
    )
    expect(screen.getByRole('heading', { name: 'Questions are not available for this case' })).toBeTruthy()
    expect(screen.queryByText('Secret result')).toBeNull()
    expect(screen.getByRole('link', { name: 'Back to case overview' })).toHaveAttribute('href', '/cases/case-a/overview')
  })
})
