import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import NewCasePage from './NewCasePage.jsx'

// The case name IS the identifier the whole system scopes on. An analyst who
// types a name with a space and later cannot find their case has been failed
// by this screen, so the name is validated here rather than silently
// normalised into something they did not choose.

function open() {
  return render(<MemoryRouter><NewCasePage /></MemoryRouter>)
}

describe('NewCasePage', () => {
  it('does not offer intake until the case is named', () => {
    open()
    expect(screen.getByText(/enter a valid case identifier above to open evidence intake/i)).toBeTruthy()
    expect(screen.queryByLabelText(/choose evidence files/i)).toBeNull()
  })

  it('offers intake once the name is usable as an identifier', async () => {
    open()
    await userEvent.type(screen.getByLabelText('Case identifier'), 'operation-falcon-2026')
    expect(screen.queryByLabelText(/choose evidence files/i)).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: 'Continue' }))
    expect(screen.getByLabelText(/choose evidence files/i)).toBeTruthy()
  })

  it('keeps Continue off until the identifier is valid, and Enter continues from the field', async () => {
    open()
    expect(screen.getByRole('button', { name: 'Continue' }).disabled).toBe(true)
    await userEvent.type(screen.getByLabelText('Case identifier'), 'operation-falcon-2026{Enter}')
    expect(screen.getByLabelText(/choose evidence files/i)).toBeTruthy()
    await userEvent.click(screen.getByRole('button', { name: 'Change identifier' }))
    expect(screen.getByLabelText('Case identifier').value).toBe('operation-falcon-2026')
  })

  it('previews the case as not created until a file is accepted', async () => {
    open()
    expect(screen.getByText('Not created yet')).toBeTruthy()
    await userEvent.type(screen.getByLabelText('Case identifier'), 'operation-falcon-2026')
    expect(document.querySelector('.nc-preview__id').textContent).toBe('operation-falcon-2026')
    expect(screen.getByText('Not created yet')).toBeTruthy()
  })

  it('refuses a name that would not survive as an identifier, and says why', async () => {
    open()
    const field = screen.getByLabelText('Case identifier')
    await userEvent.type(field, 'Operation Falcon 2026')
    await userEvent.tab()

    const problem = await screen.findByRole('alert')
    expect(problem.textContent).toMatch(/lowercase letters, numbers and hyphens/i)
    expect(field.getAttribute('aria-invalid')).toBe('true')
    // And it must not quietly proceed as though the name were acceptable.
    expect(screen.queryByLabelText(/choose evidence files/i)).toBeNull()
  })

  it('states plainly that the case exists once evidence is added', () => {
    open()
    expect(screen.getByText(/no empty case is created before a file is accepted/i)).toBeTruthy()
  })

  it('gives a specific repair for a too-short identifier', async () => {
    open()
    const field = screen.getByLabelText('Case identifier')
    await userEvent.type(field, 'abc')
    await userEvent.tab()
    expect(await screen.findByRole('alert')).toHaveTextContent('Use at least 4 characters')
  })

  it('shows which identifier rules are met as the analyst types, before any error is raised', async () => {
    open()
    await userEvent.type(screen.getByLabelText('Case identifier'), 'Op Falcon')
    const rules = screen.getByRole('list', { name: 'Identifier rules' })
    expect(rules.textContent).toMatch(/Lowercase letters, numbers and hyphens only, not met/)
    expect(rules.textContent).toMatch(/4 to 64 characters, met/)
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('offers a valid version but never applies it without a click', async () => {
    open()
    const field = screen.getByLabelText('Case identifier')
    await userEvent.type(field, 'Operation Falcon 2026')
    expect(field.value).toBe('Operation Falcon 2026')
    await userEvent.click(screen.getByRole('button', { name: 'operation-falcon-2026' }))
    expect(field.value).toBe('operation-falcon-2026')
    expect(screen.getByRole('button', { name: 'Continue' }).disabled).toBe(false)
  })

  it('does not start a case on an identifier that already exists, and offers to add evidence to it', async () => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-alpha'] }
    open()
    await userEvent.type(screen.getByLabelText('Case identifier'), 'case-alpha')
    expect(screen.getByRole('status').textContent).toMatch(/already a case in this workspace/)
    expect(screen.getByRole('link', { name: 'Add evidence to it instead' }).getAttribute('href')).toBe('/cases/case-alpha/evidence#add-evidence')
    expect(screen.queryByLabelText(/choose evidence files/i)).toBeNull()
    delete window.__INVESTIGATION_WORKSPACE_CONFIG__
  })
})
