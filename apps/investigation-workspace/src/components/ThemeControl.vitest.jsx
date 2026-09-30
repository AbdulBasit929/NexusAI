import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { ThemeControl } from './ThemeControl.jsx'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  document.documentElement.removeAttribute('data-theme')
  try { localStorage.clear() } catch { /* a throwing-storage test may leave it unavailable */ }
})

describe('ThemeControl', () => {
  test('persists an explicit viewer choice and restores it on remount', () => {
    const first = render(<ThemeControl />)
    fireEvent.click(screen.getByRole('radio', { name: /Dark/ }))
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(localStorage.getItem('nexusai.viewer.theme')).toBe('dark')
    first.unmount()
    render(<ThemeControl />)
    expect(screen.getByRole('radio', { name: /Dark/ })).toBeChecked()
  })

  test('uses a real device-following default and responds to preference changes', () => {
    const listeners = new Set()
    const media = {
      matches: false,
      addEventListener: (_name, listener) => listeners.add(listener),
      removeEventListener: (_name, listener) => listeners.delete(listener),
    }
    vi.stubGlobal('matchMedia', () => media)
    render(<ThemeControl />)
    expect(screen.getByRole('radio', { name: /Use device setting/ })).toBeChecked()
    expect(document.documentElement).not.toHaveAttribute('data-theme')
    expect(localStorage.getItem('nexusai.viewer.theme')).toBeNull()
    expect(screen.getByText(/Currently light/)).toBeInTheDocument()

    act(() => {
      media.matches = true
      for (const listener of listeners) listener({ matches: true })
    })
    expect(screen.getByText(/Currently dark/)).toBeInTheDocument()
    expect(document.documentElement).not.toHaveAttribute('data-theme')
  })

  test('keeps every mounted appearance control synchronized', () => {
    render(<><ThemeControl /><ThemeControl /></>)
    fireEvent.click(screen.getAllByRole('radio', { name: /Dark/ })[0])
    for (const radio of screen.getAllByRole('radio', { name: /Dark/ })) expect(radio).toBeChecked()
  })

  test('still renders and switches when storage throws', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked') })
    render(<ThemeControl />)
    fireEvent.click(screen.getByRole('radio', { name: /Dark/ }))
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(screen.getByRole('radio', { name: /Dark/ })).toBeChecked()
  })
})
