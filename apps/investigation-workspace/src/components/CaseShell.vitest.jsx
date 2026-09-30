import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { AppShell, railBadges } from './CaseShell.jsx'
import { resetAttentionCache, sidebarCases } from '../lib/workspaceAttention.js'
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
    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeTruthy()
  })

  it('offers a real global Investigate action in the header and the sidebar', () => {
    open(['case-a'], { summary: { evidence_total: 1, evidence_completed: 1 } })
    expect(screen.getByRole('link', { name: 'Ask a question' }).getAttribute('href')).toBe('/investigate')
    expect(screen.getByRole('link', { name: 'Investigate' }).getAttribute('href')).toBe('/investigate')
  })

  it('shows the live processing chip only when a case is actually processing', async () => {
    open(['case-a'], { summary: { evidence_total: 4, evidence_completed: 2, evidence_in_flight: 2, evidence_failed: 0 } })
    expect(await screen.findByRole('link', { name: '1 case is processing evidence' })).toBeTruthy()
  })

  it('shows no chip when nothing is processing', async () => {
    open(['case-b'], { summary: { evidence_total: 4, evidence_completed: 4, evidence_in_flight: 0, evidence_failed: 0 } })
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled())
    expect(screen.queryByRole('link', { name: /processing evidence/ })).toBeNull()
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
    expect(links).toEqual(['overview', 'evidence', 'investigate', 'timeline', 'activity'].map(section => `/cases/case-a/${section}`))
    expect(bar.querySelector('[aria-current="page"]').getAttribute('href')).toBe('/cases/case-a/evidence')
  })

  it('shows one case-level breadcrumb and keeps case sections out of the sidebar', () => {
    openCase('/cases/case-a/timeline', failing)
    const crumbs = screen.getByRole('navigation', { name: 'Breadcrumb' })
    expect([...crumbs.querySelectorAll('li')].map(item => item.textContent)).toEqual(['Cases', 'case-a'])
    const primary = screen.getByRole('navigation', { name: 'Primary' })
    for (const section of ['evidence', 'investigate', 'timeline', 'activity']) expect(primary.querySelector('a[href="/cases/case-a/' + section + '"]')).toBeNull()
  })

  it('adds the failed-source count to Evidence only after the status has loaded', async () => {
    openCase('/cases/case-a/overview', failing)
    expect(screen.getByRole('link', { name: 'Evidence' })).toBeTruthy()
    expect(await screen.findByRole('link', { name: 'Evidence, 1 failed source' })).toBeTruthy()
  })

  it('offers case switching only when more than one case exists', () => {
    openCase('/cases/case-a/overview', failing)
    expect(screen.getByRole('button', { name: /Switch case/ })).toBeTruthy()
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

  it('orders the active case first, then cases that need review, and caps the list', () => {
    const entries = ['a', 'b', 'c', 'd'].map((caseId, index) => ({ caseId, summary: { processingState: ['complete', 'attention', 'complete', null][index] } }))
    entries[3].summary = null
    expect(sidebarCases(entries, 'c', 3).map(item => `${item.caseId}:${item.state}`)).toEqual(['c:complete', 'b:attention', 'a:complete'])
    expect(sidebarCases(entries, '', 4).at(-1)).toMatchObject({ caseId: 'd', state: 'unreported' })
  })

  it('folds every case under one Cases category with its count, a new-case action and text status', async () => {
    openRail('/cases/case-a/overview', ['case-a', 'case-b'], id => (id === 'case-a' ? ok : bad))
    const primary = screen.getByRole('navigation', { name: 'Primary' })
    expect(within(primary).getAllByRole('link', { name: /^Cases/ })).toHaveLength(1)
    expect(within(primary).getByLabelText('2 cases')).toBeTruthy()
    expect(within(primary).getByRole('link', { name: 'New case' }).getAttribute('href')).toBe('/cases/new')
    const list = within(primary).getByRole('list', { name: 'Cases' })
    await waitFor(() => expect(list.textContent).toContain('Needs review'))
    expect(list.textContent).toContain('1 failed source')
    expect(list.querySelector('[aria-current="true"]').getAttribute('href')).toBe('/cases/case-a/overview')
    expect(list.querySelector('a[href="/cases/case-b/overview"]')).toBeTruthy()
  })

  it('folds and unfolds the case list and remembers the choice', async () => {
    openRail('/', ['case-a'], () => ok)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Hide cases' }))
    expect(screen.queryByRole('list', { name: 'Cases' })).toBeNull()
    expect(localStorage.getItem('nexusai.viewer.rail-cases-open')).toBe('false')
    await user.click(screen.getByRole('button', { name: 'Show cases' }))
    expect(screen.getByRole('list', { name: 'Cases' })).toBeTruthy()
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
    expect(screen.queryByRole('list', { name: 'Cases' })).toBeNull()
    expect(screen.getByRole('link', { name: 'Cases' })).toBeTruthy()
  })
})

describe('sidebar badges', () => {
  it('names the count in the accessible label and stays absent at zero', () => {
    expect(railBadges({ needReview: 2, processing: 0 }, '', null)).toEqual({ '/': { count: 2, tone: 'attention', label: '2 cases need review' } })
    expect(railBadges({ needReview: 0, processing: 1 }, 'a', { data: { summary: { evidence_failed: 3 } } })).toEqual({
      '/cases': { count: 1, tone: 'processing', label: '1 case is processing' },
      '/cases/a/evidence': { count: 3, tone: 'attention', label: '3 failed sources' },
    })
    expect(railBadges({ needReview: 0, processing: 0 }, '', null)).toEqual({})
  })
})
