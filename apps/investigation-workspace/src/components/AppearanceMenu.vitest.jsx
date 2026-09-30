import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { AppearanceMenu } from './AppearanceMenu.jsx'

const show = () => render(<MemoryRouter><AppearanceMenu /></MemoryRouter>)

beforeEach(() => {
  localStorage.clear()
  document.documentElement.removeAttribute('data-theme')
})
afterEach(() => cleanup())

describe('AppearanceMenu', () => {
  it('is one icon button whose name states the current appearance', () => {
    show()
    const button = screen.getByRole('button', { name: /^Appearance, system, currently (light|dark)$/ })
    expect(button.getAttribute('aria-expanded')).toBe('false')
    expect(screen.queryByRole('radio')).toBeNull()
  })

  it('opens a labelled panel with one radio group and focuses the selected option', async () => {
    show()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /^Appearance/ }))
    expect(screen.getByRole('button', { name: /^Appearance/ }).getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByRole('group', { name: 'Appearance' })).toBeTruthy()
    expect(screen.getAllByRole('radio').map(radio => radio.value)).toEqual(['system', 'light', 'dark'])
    expect(document.activeElement).toBe(screen.getByRole('radio', { name: /System/ }))
  })

  it('applies and remembers the chosen theme and reflects it in the button name', async () => {
    show()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /^Appearance/ }))
    await user.click(screen.getByRole('radio', { name: /Dark/ }))
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    expect(localStorage.getItem('nexusai.viewer.theme')).toBe('dark')
    expect(screen.getByRole('button', { name: 'Appearance, dark' })).toBeTruthy()
    expect(screen.getByRole('status').textContent).toBe('Always dark on this browser.')
    await user.click(screen.getByRole('radio', { name: /System/ }))
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false)
    expect(localStorage.getItem('nexusai.viewer.theme')).toBeNull()
  })

  it('closes with Escape and returns focus to the button', async () => {
    show()
    const user = userEvent.setup()
    const button = screen.getByRole('button', { name: /^Appearance/ })
    await user.click(button)
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('group', { name: 'Appearance' })).toBeNull()
    expect(document.activeElement).toBe(button)
  })

  it('closes on an outside click and offers the full settings page', async () => {
    render(<MemoryRouter><AppearanceMenu /><p>Outside</p></MemoryRouter>)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /^Appearance/ }))
    expect(screen.getByRole('link', { name: 'All display settings' }).getAttribute('href')).toBe('/settings')
    await user.click(screen.getByText('Outside'))
    expect(screen.queryByRole('group', { name: 'Appearance' })).toBeNull()
  })
})
