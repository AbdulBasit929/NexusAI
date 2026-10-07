import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Link, MemoryRouter, Route, Routes } from 'react-router-dom'
import { ErrorBoundary, RouteErrorBoundary } from './ErrorBoundary.jsx'

function Broken() {
  throw new Error('render failed on purpose')
}

afterEach(() => vi.restoreAllMocks())

describe('ErrorBoundary', () => {
  it('renders its children when nothing fails', () => {
    render(<ErrorBoundary><p>a page</p></ErrorBoundary>)
    expect(screen.getByText('a page')).toBeTruthy()
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('replaces a page that throws with a message the analyst can act on, never a blank page', () => {
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})
    render(<><p>outside the page</p><ErrorBoundary><Broken /></ErrorBoundary></>)
    const alert = screen.getByRole('alert')
    expect(alert.textContent).toMatch(/This page could not be displayed/)
    expect(alert.textContent).toMatch(/Your case evidence was not changed/)
    expect(screen.getByRole('button', { name: 'Reload this page' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Back to the dashboard' }).getAttribute('href')).toBe('/')
    expect(screen.getByText('outside the page')).toBeTruthy()
    // Still in the console for whoever opens the developer tools.
    expect(logged.mock.calls.some(call => String(call[0]).startsWith('[workspace]'))).toBe(true)
  })

  it('shows a fallback of the caller\'s choosing', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(<ErrorBoundary fallback={<p>this answer could not be shown</p>}><Broken /></ErrorBoundary>)
    expect(screen.getByText('this answer could not be shown')).toBeTruthy()
  })
})

describe('RouteErrorBoundary', () => {
  it('lets the next page render without a reload', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(
      <MemoryRouter initialEntries={['/broken']}>
        <Link to="/fine">open the other page</Link>
        <RouteErrorBoundary>
          <Routes>
            <Route path="/broken" element={<Broken />} />
            <Route path="/fine" element={<p>the other page</p>} />
          </Routes>
        </RouteErrorBoundary>
      </MemoryRouter>,
    )
    expect(screen.getByRole('alert')).toBeTruthy()
    await userEvent.setup().click(screen.getByRole('link', { name: 'open the other page' }))
    expect(screen.getByText('the other page')).toBeTruthy()
    expect(screen.queryByRole('alert')).toBeNull()
  })
})
