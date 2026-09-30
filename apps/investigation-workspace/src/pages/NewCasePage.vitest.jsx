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
    expect(screen.getByLabelText(/choose evidence files/i)).toBeTruthy()
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
})
