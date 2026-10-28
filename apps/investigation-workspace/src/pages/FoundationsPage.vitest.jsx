import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import FoundationsPage from './FoundationsPage.jsx'

describe('FoundationsPage', () => {
  it('renders the complete foundation proof with synthetic labels', () => {
    render(<FoundationsPage />)
    expect(screen.getByRole('heading', { name: 'A calm instrument for evidence' })).toBeInTheDocument()
    expect(screen.getByText('160 token pairs pass')).toBeInTheDocument()
    expect(screen.getAllByText('Generated demonstration')).toHaveLength(3)
    expect(screen.getByRole('heading', { name: 'Generated contrast table' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Simulation and redundant encoding' })).toBeInTheDocument()
  })

  it('switches direction, theme, density and preview width without replacing the screen set', async () => {
    const user = userEvent.setup()
    const { container } = render(<FoundationsPage />)
    await user.click(screen.getByRole('button', { name: 'B · Ink and signal' }))
    await user.click(screen.getByRole('button', { name: 'Dark' }))
    await user.click(screen.getByRole('button', { name: '390 px' }))
    await user.click(screen.getByRole('button', { name: 'compact' }))
    expect(screen.getByRole('heading', { name: 'Ink and signal' })).toBeInTheDocument()
    expect(screen.getByText(/Ink and signal · dark · 390 px · compact/)).toBeInTheDocument()
    expect(container.querySelector('.nx-foundations__system')).toHaveAttribute('data-foundation-direction', 'b')
    expect(container.querySelector('.nx-foundations__system')).toHaveAttribute('data-theme-preview', 'dark')
    expect(container.querySelector('.nx-screen-gallery')).toHaveAttribute('data-preview-width', '390')
    const captions = [...container.querySelectorAll('.nx-screen-frame figcaption')]
    expect(captions).toHaveLength(3)
    expect(captions.every(caption => caption.textContent.endsWith('390 px'))).toBe(true)
  })

  it('offers reduced motion and all three CVD simulations as explicit controls', async () => {
    const user = userEvent.setup()
    const { container } = render(<FoundationsPage />)
    await user.click(screen.getByRole('checkbox', { name: 'Emulate reduced motion' }))
    expect(container.querySelector('.nx-motion-proof')).toHaveAttribute('data-reduced')
    const simulation = screen.getByRole('group', { name: 'Simulation' })
    await user.click(within(simulation).getByRole('button', { name: 'Deuteranopia' }))
    expect(container.querySelector('.nx-cvd-proof')).toHaveAttribute('data-simulation', 'deuteranopia')
  })
})
