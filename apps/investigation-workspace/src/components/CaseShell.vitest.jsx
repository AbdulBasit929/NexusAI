import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { AppShell } from './CaseShell.jsx'
import { resetAttentionCache } from '../lib/workspaceAttention.js'
import { clearWorkspaceStateForTests } from '../lib/workspaceState.js'

function open(caseIds, status) {
  window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds }
  globalThis.fetch = vi.fn(async () => ({ ok: true, status: 200, json: async () => status, text: async () => JSON.stringify(status) }))
  return render(<MemoryRouter initialEntries={['/']}><AppShell><main>Page</main></AppShell></MemoryRouter>)
}

afterEach(() => {
  resetAttentionCache()
  vi.restoreAllMocks()
  delete window.__INVESTIGATION_WORKSPACE_CONFIG__
})

describe('shell landmarks and header', () => {
  it('has one banner, one main, a labelled search and a labelled navigation', () => {
    open(['case-a'], { summary: { evidence_total: 1, evidence_completed: 1 } })
    expect(screen.getAllByRole('banner')).toHaveLength(1)
    expect(screen.getAllByRole('main')).toHaveLength(1)
    expect(screen.getByRole('search', { name: 'Workspace search' })).toBeTruthy()
    expect(screen.getByRole('navigation', { name: 'Workspace' })).toBeTruthy()
  })

  it('offers the global question route in the workspace navigation', () => {
    open(['case-a'], { summary: { evidence_total: 1, evidence_completed: 1 } })
    expect(screen.getByRole('link', { name: 'Ask across cases' }).getAttribute('href')).toBe('/investigate')
  })

  it('shows live processing only inside the truthful activity disclosure', async () => {
    open(['case-a'], { summary: { evidence_total: 4, evidence_completed: 2, evidence_in_flight: 2, evidence_failed: 0 } })
    const trigger = await screen.findByRole('button', { name: 'Activity, 1 processing' })
    await userEvent.click(trigger)
    expect(screen.getByRole('region', { name: 'Current activity' })).toHaveTextContent('Evidence processing')
  })

  it('states that no active processing was reported when nothing is processing', async () => {
    open(['case-b'], { summary: { evidence_total: 4, evidence_completed: 4, evidence_in_flight: 0, evidence_failed: 0 } })
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled())
    await userEvent.click(screen.getByRole('button', { name: 'Activity' }))
    expect(screen.getByText('No active processing reported.')).toBeTruthy()
  })

  it('shows and clears a truthful offline banner from browser connectivity state', async () => {
    open([], { summary: {} })
    Object.defineProperty(navigator, 'onLine', { configurable: true, value: false })
    window.dispatchEvent(new Event('offline'))
    expect(await screen.findByRole('status')).toHaveTextContent('You are offline')
    Object.defineProperty(navigator, 'onLine', { configurable: true, value: true })
    window.dispatchEvent(new Event('online'))
    await waitFor(() => expect(screen.queryByText(/You are offline/)).toBeNull())
  })
})

describe('case context bar', () => {
  function openCase(path, status) {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a', 'case-b'] }
    globalThis.fetch = vi.fn(async () => ({ ok: true, status: 200, json: async () => status, text: async () => JSON.stringify(status) }))
    return render(<MemoryRouter initialEntries={[path]}><AppShell caseId="case-a"><main>Page</main></AppShell></MemoryRouter>)
  }
  const failing = { summary: { evidence_total: 12, evidence_completed: 11, evidence_in_flight: 0, evidence_failed: 1 } }

  it('lists the case sections as links and marks the current one with aria-current', () => {
    openCase('/cases/case-a/evidence/e1', failing)
    const bar = screen.getByRole('navigation', { name: 'Case sections' })
    const links = [...bar.querySelectorAll('a')].map(link => link.getAttribute('href'))
    expect(links).toEqual(['overview', 'evidence', 'investigate', 'connections', 'timeline', 'activity'].map(section => `/cases/case-a/${section}`))
    expect(bar.querySelector('[aria-current="page"]').getAttribute('href')).toBe('/cases/case-a/evidence')
  })

  it('keeps case destinations in the section band without duplicating the case identity', () => {
    openCase('/cases/case-a/timeline', failing)
    const context = screen.getByRole('region', { name: 'Case context' })
    expect(within(context).queryByText('case-a')).toBeNull()
    expect(screen.queryByRole('navigation', { name: 'Case' })).toBeNull()
  })

  it('does not turn case status into navigation badges', async () => {
    openCase('/cases/case-a/overview', failing)
    const sections = screen.getByRole('navigation', { name: 'Case sections' })
    expect(within(sections).getByRole('link', { name: 'Evidence' })).toBeTruthy()
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled())
    expect(within(sections).queryByText('1')).toBeNull()
  })

  it('offers case switching only when more than one case exists', () => {
    openCase('/cases/case-a/overview', failing)
    expect(screen.getByRole('button', { name: 'Current case case-a' })).toBeTruthy()
  })

  it('keeps a single long case identifier complete without a useless switch action', () => {
    const longCaseId = 'operation-northbridge-cross-border-financial-network-2026'
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: [longCaseId] }
    globalThis.fetch = vi.fn(async () => ({ ok: true, status: 200, json: async () => failing, text: async () => JSON.stringify(failing) }))
    render(<MemoryRouter initialEntries={[`/cases/${longCaseId}/overview`]}><AppShell caseId={longCaseId}><main>Page</main></AppShell></MemoryRouter>)
    const scope = screen.getByText(longCaseId).closest('.workspace-header__scope')
    expect(scope.textContent).toContain(longCaseId)
    expect(within(scope).queryByRole('button')).toBeNull()
    expect(screen.getByRole('region', { name: 'Case context' }).textContent).not.toContain(longCaseId)
  })

  it('keeps page destinations in the case section navigation instead of duplicating quick actions', () => {
    openCase('/cases/case-a/overview', failing)
    const contextBand = screen.getByRole('region', { name: 'Case context' })
    expect(within(contextBand).getByRole('link', { name: 'Ask' }).getAttribute('href')).toBe('/cases/case-a/investigate')
    expect(within(contextBand).getByRole('link', { name: /^Evidence/ }).getAttribute('href')).toBe('/cases/case-a/evidence')
  })

  it('keeps readiness and page metrics out of the compact case band', async () => {
    openCase('/cases/case-a/overview', { summary: { evidence_total: 12, evidence_completed: 12, accepted_rows: 12912 } })
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled())
    const context = screen.getByRole('region', { name: 'Case context' })
    expect(within(context).queryByText('Evidence ready')).toBeNull()
    expect(screen.queryByTitle('Total ingested forensic records')).toBeNull()
  })

  it('cycles tab focus with horizontal arrow keys across the case tab strip', async () => {
    openCase('/cases/case-a/overview', failing)
    const user = userEvent.setup()
    const bar = screen.getByRole('navigation', { name: 'Case sections' })
    const links = bar.querySelectorAll('a')

    links[0].focus()
    expect(document.activeElement).toBe(links[0])

    await user.keyboard('{ArrowRight}')
    expect(document.activeElement).toBe(links[1])

    await user.keyboard('{ArrowLeft}')
    expect(document.activeElement).toBe(links[0])

    await user.keyboard('{End}')
    expect(document.activeElement).toBe(links[links.length - 1])

    await user.keyboard('{Home}')
    expect(document.activeElement).toBe(links[0])
  })

  it('opens case switcher when Alt+S keyboard shortcut is pressed', async () => {
    openCase('/cases/case-a/overview', failing)
    const user = userEvent.setup()
    expect(screen.queryByRole('dialog', { name: 'Switch case' })).toBeNull()

    await user.keyboard('{Alt>}s{/Alt}')
    expect(await screen.findByRole('dialog', { name: 'Switch case' })).toBeTruthy()
  })
})

describe('sidebar sections', () => {
  const question = (id, query, over = {}) => ({ id, query, label: '', state: 'answered', pinned: false, updatedAt: new Date(Date.now() - 12 * 60_000).toISOString(), ...over })
  function openRail(path, caseIds, statusFor) {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds }
    globalThis.fetch = vi.fn(async url => {
      const id = new URL(String(url), 'http://x').searchParams.get('collection_id')
      const body = statusFor(id)
      return { ok: true, status: 200, json: async () => body, text: async () => JSON.stringify(body) }
    })
    return render(<MemoryRouter initialEntries={[path]}><AppShell caseId={path.startsWith('/cases/') ? path.split('/')[2] : ''}><main>Page</main></AppShell></MemoryRouter>)
  }
  const ok = { summary: { evidence_total: 4, evidence_completed: 4, evidence_in_flight: 0, evidence_failed: 0 } }
  const bad = { summary: { evidence_total: 4, evidence_completed: 3, evidence_in_flight: 0, evidence_failed: 1 } }

  afterEach(() => { cleanup(); localStorage.clear(); clearWorkspaceStateForTests() })

  it('keeps Cases as one quiet destination without an inline list, count or action', async () => {
    openRail('/cases/case-a/overview', ['case-a', 'case-b'], id => (id === 'case-a' ? ok : bad))
    const workspace = screen.getByRole('navigation', { name: 'Workspace' })
    expect(within(workspace).getAllByRole('link', { name: 'Cases' })).toHaveLength(1)
    expect(within(workspace).getByRole('link', { name: 'Cases' }).getAttribute('href')).toBe('/cases')
    expect(within(workspace).queryByRole('list', { name: 'Cases' })).toBeNull()
    expect(within(workspace).queryByRole('link', { name: 'New case' })).toBeNull()
    expect(within(workspace).queryByText('Current case')).toBeNull()
  })

  it('keeps question history out of the sidebar and the drawer', () => {
    localStorage.setItem('nexusai.viewer.questions.case-a', JSON.stringify([question('q1', 'Who called most often?', { pinned: true }), question('q2', 'List all vehicles')]))
    openRail('/', ['case-a'], () => ok)
    expect(screen.queryByRole('region', { name: /Pinned|Recent/ })).toBeNull()
    expect(screen.queryByText('Who called most often?')).toBeNull()
    expect(screen.queryByText('List all vehicles')).toBeNull()
  })

  it('collapses and expands from a labelled control that reports its state', async () => {
    openRail('/', ['case-a'], () => ok)
    const user = userEvent.setup()
    const toggle = screen.getByRole('button', { name: 'Collapse navigation' })
    expect(toggle.getAttribute('aria-expanded')).toBe('true')
    await user.click(toggle)
    expect(screen.getByRole('button', { name: 'Expand navigation' }).getAttribute('aria-expanded')).toBe('false')
    expect(screen.getByRole('link', { name: 'Cases' })).toBeTruthy()
  })
})
