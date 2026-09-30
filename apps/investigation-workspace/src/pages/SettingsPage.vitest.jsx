import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { clearWorkspaceStateForTests } from '../lib/workspaceState.js'
import SettingsPage from './SettingsPage.jsx'

afterEach(() => {
  vi.restoreAllMocks()
  clearWorkspaceStateForTests()
  document.documentElement.removeAttribute('data-theme')
  try { localStorage.clear() } catch { /* restored spies make the next test independent */ }
})

function open() {
  return render(<MemoryRouter><SettingsPage /></MemoryRouter>)
}

describe('SettingsPage', () => {
  test('reports useful browser-local work without inventing identity or connection facts', () => {
    localStorage.setItem('nexusai.viewer.questions.case-a', JSON.stringify([{ id: '1' }, { id: '2' }]))
    localStorage.setItem('nexusai.viewer.questions.case-b', JSON.stringify([{ id: '3' }]))
    localStorage.setItem('nexusai.viewer.draft.case-a', 'unfinished question')
    localStorage.setItem('nexusai.viewer.draft.case-c', 'another draft')
    open()

    expect(screen.getByText('Saved questions').closest('div')).toHaveTextContent('3')
    expect(screen.getByText('Unfinished drafts').closest('div')).toHaveTextContent('2')
    expect(screen.getByText('Cases with local work').closest('div')).toHaveTextContent('3')
    expect(screen.queryByText('Tenant')).not.toBeInTheDocument()
    expect(screen.queryByText('Role')).not.toBeInTheDocument()
    expect(screen.queryByText('API path')).not.toBeInTheDocument()
  })

  test('reviews the exact destructive scope and preserves preferences when clearing', async () => {
    localStorage.setItem('nexusai.viewer.questions.case-a', JSON.stringify([{ id: '1' }]))
    localStorage.setItem('nexusai.viewer.draft.case-a', 'unfinished question')
    localStorage.setItem('nexusai.viewer.theme', 'dark')
    open()

    await userEvent.click(screen.getByRole('button', { name: 'Clear question history and drafts…' }))
    expect(screen.getByRole('heading', { name: 'Clear 2 local items?' })).toBeInTheDocument()
    expect(screen.getByText(/Case evidence and server-side findings are not affected/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Clear from this browser' }))

    expect(localStorage.getItem('nexusai.viewer.questions.case-a')).toBeNull()
    expect(localStorage.getItem('nexusai.viewer.draft.case-a')).toBeNull()
    expect(localStorage.getItem('nexusai.viewer.theme')).toBe('dark')
    expect(screen.getByRole('status')).toHaveTextContent('Question history and drafts were cleared')
    expect(screen.getByRole('heading', { name: 'No local questions or drafts' })).toBeInTheDocument()
  })

  test('remains truthful and usable when browser storage is unavailable', () => {
    localStorage.setItem('nexusai.viewer.theme', 'dark')
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => { throw new Error('blocked') })
    open()
    expect(screen.getByRole('heading', { name: 'Browser storage is unavailable' })).toBeInTheDocument()
    expect(screen.getAllByRole('radio', { name: /Dark/ }).length).toBeGreaterThan(0)
  })
})
