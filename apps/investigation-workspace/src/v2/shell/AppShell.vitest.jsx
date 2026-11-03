import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { AppShell, railBadges } from './AppShell.jsx'
import { trailFor } from './nav.js'
import { resetAttentionCache } from '../../lib/workspaceAttention.js'
import { resetCaseRecordsForTest } from '../../lib/useCaseRecords.js'

function open(path, { caseId = '', caseIds = ['case-a', 'case-b'], status } = {}) {
  window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds }
  const body = status || { summary: { evidence_total: 12, evidence_completed: 12, evidence_in_flight: 0, evidence_failed: 0 } }
  globalThis.fetch = vi.fn(async () => ({ ok: true, status: 200, json: async () => body, text: async () => JSON.stringify(body) }))
  return render(<MemoryRouter initialEntries={[path]}><AppShell caseId={caseId}><main>Page</main></AppShell></MemoryRouter>)
}

beforeEach(() => { localStorage.clear(); document.documentElement.removeAttribute('data-theme'); document.documentElement.removeAttribute('data-density') })
afterEach(() => {
  cleanup()
  resetAttentionCache()
  resetCaseRecordsForTest()
  vi.restoreAllMocks()
  delete window.__INVESTIGATION_WORKSPACE_CONFIG__
})

describe('landmarks and the workspace group', () => {
  it('has one banner, a labelled workspace navigation and a breadcrumb', () => {
    open('/')
    expect(screen.getAllByRole('banner')).toHaveLength(1)
    expect(screen.getByRole('navigation', { name: 'Workspace sections' })).toBeTruthy()
    expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toBeTruthy()
  })

  it('links the global places and marks the current one', () => {
    open('/investigate')
    const nav = screen.getByRole('navigation', { name: 'Workspace sections' })
    const links = Object.fromEntries([...nav.querySelectorAll('a')].map(link => [link.textContent.replace(/\d+$/, '').trim(), link.getAttribute('href')]))
    expect(links).toEqual({ Dashboard: '/', 'Ask across cases': '/investigate', Cases: '/cases', Activity: '/activity' })
    expect(nav.querySelector('[aria-current="page"]').getAttribute('href')).toBe('/investigate')
  })
})

describe('inside a case', () => {
  it('lists the case sections in order and marks the current one', () => {
    open('/cases/case-a/evidence/e1', { caseId: 'case-a' })
    const nav = screen.getByRole('navigation', { name: 'This case' })
    const hrefs = [...nav.querySelectorAll('.nx-nav a')].map(link => link.getAttribute('href'))
    expect(hrefs).toEqual(['overview', 'evidence', 'investigate', 'connections', 'timeline', 'activity'].map(section => `/cases/case-a/${section}`))
    expect(nav.querySelector('[aria-current="page"]').getAttribute('href')).toBe('/cases/case-a/evidence')
  })

  it('reads an identifier as a title in the breadcrumb and keeps the identifier as its title attribute', () => {
    open('/cases/orion-lahore-reference-20261030/timeline', { caseId: 'orion-lahore-reference-20261030', caseIds: ['orion-lahore-reference-20261030'] })
    const crumbs = screen.getByRole('navigation', { name: 'Breadcrumb' })
    expect([...crumbs.querySelectorAll('li')].map(item => item.textContent)).toEqual(['Cases', 'Orion Lahore Reference', 'Timeline'])
    expect(within(crumbs).getByText('Orion Lahore Reference').closest('[title]').getAttribute('title')).toBe('orion-lahore-reference-20261030')
  })

  it('shows the failed-source count on Evidence only once the status has loaded', async () => {
    open('/cases/case-a/overview', { caseId: 'case-a', status: { summary: { evidence_total: 12, evidence_completed: 11, evidence_in_flight: 0, evidence_failed: 1 } } })
    const evidence = screen.getByRole('link', { name: 'Evidence' })
    expect(evidence.querySelector('.nx-nav-item__count')).toBeNull()
    await waitFor(() => expect(screen.getByRole('link', { name: /Evidence/ }).querySelector('.nx-nav-item__count')?.textContent).toBe('1'))
  })

  it('says what the case status service reported, never healthy before it has spoken', async () => {
    open('/cases/case-a/overview', { caseId: 'case-a' })
    expect(screen.getByText('Checking status…')).toBeTruthy()
    expect(await screen.findByText('12 of 12 sources ready')).toBeTruthy()
  })
})

describe('the rail, search and appearance', () => {
  it('collapses to icons, remembers the choice, and keeps every link named', async () => {
    open('/')
    const app = document.querySelector('.nx-app')
    expect(app.getAttribute('data-rail')).toBe('expanded')
    await userEvent.click(screen.getByRole('button', { name: 'Collapse the navigation' }))
    expect(app.getAttribute('data-rail')).toBe('collapsed')
    expect(localStorage.getItem('nexusai.viewer.rail.default')).toBe('collapsed')
    expect(screen.getByRole('link', { name: 'Dashboard' })).toBeTruthy()
  })

  it('opens the search with Ctrl+K and closes it with Escape', async () => {
    open('/')
    await userEvent.keyboard('{Control>}k{/Control}')
    const dialog = await screen.findByRole('dialog', { name: 'Search or jump to' })
    expect(within(dialog).getByRole('combobox')).toHaveFocus()
    expect(within(dialog).getByText('Go to')).toBeTruthy()
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Search or jump to' })).toBeNull())
  })

  it('sets the theme and the density on the document and remembers them', async () => {
    open('/')
    await userEvent.click(screen.getByRole('button', { name: 'Appearance' }))
    await userEvent.click(screen.getByRole('menuitemradio', { name: 'Dark' }))
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    expect(localStorage.getItem('nexusai.viewer.theme')).toBe('dark')
    await userEvent.click(screen.getByRole('menuitemradio', { name: 'Compact' }))
    expect(document.documentElement.getAttribute('data-density')).toBe('compact')
    await userEvent.click(screen.getByRole('menuitemradio', { name: 'High contrast' }))
    expect(document.documentElement.getAttribute('data-theme')).toBe('contrast')
  })

  it('says when something is running or needs review, in the activity menu', async () => {
    open('/', { status: { summary: { evidence_total: 4, evidence_completed: 2, evidence_in_flight: 2, evidence_failed: 0 } } })
    const button = await screen.findByRole('button', { name: /Activity: 2 need attention or are running/ })
    await userEvent.click(button)
    expect(await screen.findByText(/cases are processing evidence/)).toBeTruthy()
  })
})

describe('the trail and the badges', () => {
  it('builds the trail from the address', () => {
    expect(trailFor('/')).toEqual([{ label: 'Dashboard', to: '/' }])
    expect(trailFor('/cases/new')).toEqual([{ label: 'Cases', to: '/cases' }, { label: 'New case' }])
    expect(trailFor('/cases/a%20b/evidence/e1').map(item => item.label || item.caseId)).toEqual(['Cases', 'a b', 'Evidence', 'File'])
  })
  it('counts only what the service reported', () => {
    expect(railBadges({ needReview: 0, processing: 0 }, 'a', null)).toEqual({})
    expect(railBadges({ needReview: 2, processing: 1 }, 'a', { data: { summary: { evidence_failed: 3 } } })['/cases/a/evidence'].count).toBe(3)
  })
})
