import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { WorkspaceCommands } from './WorkspaceCommands.jsx'
import { clearWorkspaceStateForTests } from '../lib/workspaceState.js'

const question = (id, query, over = {}) => ({ id, query, label: '', state: 'answered', pinned: false, updatedAt: new Date(Date.now() - 12 * 60_000).toISOString(), ...over })

function open(caseId = '') {
  window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a', 'case-b'] }
  localStorage.setItem('nexusai.viewer.questions.case-a', JSON.stringify([question('q1', 'Who called most often?', { pinned: true })]))
  localStorage.setItem('nexusai.viewer.questions.case-b', JSON.stringify([question('q2', 'List all vehicles')]))
  globalThis.fetch = async () => ({ ok: true, status: 200, json: async () => ({ items: [] }), text: async () => '{"items":[]}' })
  function LocationProbe() { return <output aria-label="Current path">{useLocation().pathname}</output> }
  return render(<MemoryRouter><WorkspaceCommands caseId={caseId} showTrigger /><LocationProbe /></MemoryRouter>)
}

afterEach(() => {
  cleanup()
  localStorage.clear()
  clearWorkspaceStateForTests()
  delete window.__INVESTIGATION_WORKSPACE_CONFIG__
})

describe('command palette', () => {
  it('opens with Go to, Do and Ask groups and saved questions from every case', async () => {
    open()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Quick find' }))
    const results = screen.getByRole('listbox', { name: 'Quick find results' })
    const ask = within(results).getByRole('group', { name: 'Ask' })
    expect(ask.textContent).toContain('Who called most often?')
    expect(ask.textContent).toContain('case-a')
    expect(ask.textContent).toContain('List all vehicles')
    expect(ask.textContent).toContain('case-b')
    expect(within(results).getByRole('group', { name: 'Go to' }).textContent).toContain('Ask across cases')
    expect(within(results).getByRole('group', { name: 'Do' }).textContent).toContain('Create a case')
    expect(screen.getByRole('combobox')).toHaveAttribute('aria-activedescendant', 'command-option-0')
  })

  it('keeps the three bounded command groups when there is no saved history', async () => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a'] }
    globalThis.fetch = async () => ({ ok: true, status: 200, json: async () => ({ items: [] }), text: async () => '{"items":[]}' })
    render(<MemoryRouter><WorkspaceCommands showTrigger /></MemoryRouter>)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Quick find' }))
    expect(screen.getByRole('group', { name: 'Go to' })).toBeTruthy()
    expect(screen.getByRole('group', { name: 'Do' })).toBeTruthy()
    expect(screen.getByRole('group', { name: 'Ask' })).toBeTruthy()
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

  it('opens from slash and moves the active option without moving DOM focus', async () => {
    open()
    const user = userEvent.setup()
    await user.keyboard('/')
    const input = await screen.findByRole('combobox')
    expect(document.activeElement).toBe(input)
    await user.keyboard('{ArrowDown}')
    expect(document.activeElement).toBe(input)
    expect(input).toHaveAttribute('aria-activedescendant', 'command-option-1')
  })

  it('opens the single-source shortcut sheet with question mark', async () => {
    open()
    const user = userEvent.setup()
    await user.keyboard('?')
    expect(await screen.findByRole('dialog', { name: 'Keyboard shortcuts' })).toHaveTextContent('Go to Dashboard')
  })

  it('uses the documented G then letter route sequence', async () => {
    open()
    const user = userEvent.setup()
    await user.keyboard('gc')
    expect(screen.getByLabelText('Current path')).toHaveTextContent('/cases')
  })
})
