import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import { EmptyState, FindingCard, LanguageText, ProcessingBadge, ResultStateBanner } from './AnalystComponents.jsx'

describe('analyst design-system components', () => {
  test('LanguageText keeps Urdu text auto-directed and isolates inherent LTR identifiers', () => {
    render(<LanguageText>کال 03001234567 at 1035</LanguageText>)
    const text = screen.getByText(/کال/)
    expect(text).toHaveAttribute('dir', 'auto')
    expect(text).toHaveAttribute('lang', 'ur')
    expect(text.textContent).toBe('کال 03001234567 at 1035')
    expect(text.querySelector('bdi')).toHaveAttribute('dir', 'ltr')
    expect(text.querySelector('bdi').textContent).toBe('03001234567')
  })

  test('EmptyState keeps four distinct truths', () => {
    const { rerender } = render(<EmptyState kind="complete-zero" />)
    expect(screen.getByRole('heading', { name: /zero findings/i })).toBeInTheDocument()
    rerender(<EmptyState kind="no-match" />)
    expect(screen.getByRole('heading', { name: /no match/i })).toBeInTheDocument()
    rerender(<EmptyState kind="unavailable" />)
    expect(screen.getByRole('heading', { name: /unavailable/i })).toBeInTheDocument()
    rerender(<EmptyState kind="not-processed" />)
    expect(screen.getByRole('heading', { name: /not processed/i })).toBeInTheDocument()
  })

  test('state, processing, and finding components expose text semantics as well as colour', () => {
    render(<><ResultStateBanner state="processing" /><ProcessingBadge state="not-processed" /><FindingCard classification="candidate" citationCount={2}>Possible overlap</FindingCard></>)
    expect(screen.getByText('Evidence is being processed')).toBeInTheDocument()
    expect(screen.getByText('Not processed')).toBeInTheDocument()
    expect(screen.getByText('Candidate observation')).toBeInTheDocument()
    expect(screen.getByText('2 sources')).toBeInTheDocument()
  })
})
