import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import ComponentsPage from './ComponentsPage.jsx'

describe('ComponentsPage', () => {
  it('records the owner-selected Case atlas direction while keeping comparisons available', () => {
    render(<ComponentsPage />)
    expect(screen.getByRole('heading', { name: 'Choose how the instrument holds its work' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Preview A' })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: 'Preview B' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Preview C' })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByText('Direction B selected')).toBeInTheDocument()
  })

  it('compares direction, theme, viewport and scope using one stable preview', async () => {
    const user = userEvent.setup()
    const { container } = render(<ComponentsPage />)
    await user.click(screen.getByRole('button', { name: 'Preview B' }))
    await user.click(within(screen.getByRole('group', { name: 'Theme' })).getByRole('button', { name: 'Dark' }))
    await user.click(within(screen.getByRole('group', { name: 'Preview width' })).getByRole('button', { name: '390 px' }))
    await user.click(within(screen.getByRole('group', { name: 'Scope' })).getByRole('button', { name: 'Global workspace' }))
    expect(screen.getAllByRole('heading', { name: 'Case atlas' })).toHaveLength(2)
    expect(container.querySelector('.nx-shell-preview')).toHaveAttribute('data-direction', 'b')
    expect(container.querySelector('.nx-shell-preview')).toHaveAttribute('data-theme-preview', 'dark')
    expect(container.querySelector('.nx-shell-frame')).toHaveAttribute('data-preview-width', '390')
    expect(screen.queryByLabelText('Case context preview')).toBeNull()
  })

  it('labels the synthetic data and records the non-negotiable behavior', () => {
    render(<ComponentsPage />)
    expect(screen.getAllByText('Generated demonstration').length).toBeGreaterThan(1)
    expect(screen.getByRole('heading', { name: 'These are not owner decisions' })).toBeInTheDocument()
    expect(screen.getByText(/Remembered expanded\/collapsed state/)).toBeInTheDocument()
    expect(screen.getByText(/Ctrl\/Cmd\+K opens/)).toBeInTheDocument()
  })
})
