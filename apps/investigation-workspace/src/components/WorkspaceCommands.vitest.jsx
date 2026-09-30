import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { WorkspaceCommands } from './WorkspaceCommands.jsx'
import { clearWorkspaceStateForTests } from '../lib/workspaceState.js'

const question = (id, query, over = {}) => ({ id, query, label: '', state: 'answered', pinned: false, updatedAt: new Date(Date.now() - 12 * 60_000).toISOString(), ...over })

function open(caseId = '') {
  window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a', 'case-b'] }
  localStorage.setItem('nexusai.viewer.questions.case-a', JSON.stringify([question('q1', 'Who called most often?', { pinned: true })]))
  localStorage.setItem('nexusai.viewer.questions.case-b', JSON.stringify([question('q2', 'List all vehicles')]))
  globalThis.fetch = async () => ({ ok: true, status: 200, json: async () => ({ items: [] }), text: async () => '{"items":[]}' })
  return render(<MemoryRouter><WorkspaceCommands caseId={caseId} showTrigger /></MemoryRouter>)
}

afterEach(() => {
  cleanup()
  localStorage.clear()
  clearWorkspaceStateForTests()
  delete window.__INVESTIGATION_WORKSPACE_CONFIG__
})

describe('command palette', () => {
  it('opens on an empty query as "pick up where you left off": pinned and recent questions from every case', async () => {
    open()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Quick find' }))
    const results = screen.getByRole('listbox', { name: 'Quick find results' })
    const pinned = within(results).getByRole('group', { name: 'Pinned questions' })
    expect(pinned.textContent).toContain('Who called most often?')
    expect(pinned.textContent).toContain('case-a')
    const recent = within(results).getByRole('group', { name: 'Recent questions' })
    expect(recent.textContent).toContain('List all vehicles')
    expect(recent.textContent).toContain('case-b')
    expect(within(results).getByRole('group', { name: 'Routes' }).textContent).toContain('Investigate')
  })

  it('draws no empty groups and states the scope of saved questions', async () => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a'] }
    globalThis.fetch = async () => ({ ok: true, status: 200, json: async () => ({ items: [] }), text: async () => '{"items":[]}' })
    render(<MemoryRouter><WorkspaceCommands showTrigger /></MemoryRouter>)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Quick find' }))
    expect(screen.queryByRole('group', { name: 'Pinned questions' })).toBeNull()
    expect(screen.queryByRole('group', { name: 'Recent questions' })).toBeNull()
    expect(screen.getByRole('group', { name: 'Cases' })).toBeTruthy()
  })

  it('finds a saved question by typing part of it and never widens beyond the saved scope', async () => {
    open()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Quick find' }))
    await user.type(screen.getByRole('combobox'), 'vehicles')
    const results = screen.getByRole('listbox', { name: 'Quick find results' })
    expect(within(results).getByText('List all vehicles')).toBeTruthy()
    expect(within(results).queryByText('Who called most often?')).toBeNull()
    await user.clear(screen.getByRole('combobox'))
    await user.type(screen.getByRole('combobox'), 'qqqq')
    expect(screen.getByText(/The scope was not widened/)).toBeTruthy()
  })
})
