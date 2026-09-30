import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { InvestigationResult } from './InvestigationResult.jsx'
import { EvidencePanel } from '../pages/investigate/EvidencePanel.jsx'
import { presentInvestigationResponse } from '../lib/investigationPresentation.js'

const response = { enterprise: { executive_answer: 'Three contacts stand out.', result_state: 'complete', limitations: ['Only the returned subset is shown.'], data_grid: { count: 3, columns: [{ key: 'contact' }, { key: 'm1' }], rows: [{ contact: 'Ali', m1: 5 }, { contact: 'Sana', m1: 3 }, { contact: 'Bilal', m1: 2 }] } } }
const view = () => presentInvestigationResponse(response, { caseId: 'alpha' })

describe('compact answer', () => {
  it('shows the finding, a one-line summary and the result, and leaves sources and method to the evidence panel', () => {
    const { container } = render(<InvestigationResult presentation={view()} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} compact />)
    expect(screen.getByText(/Three contacts stand out/)).toBeTruthy()
    expect(container.querySelector('.answer-summary')).toBeTruthy()
    expect(container.querySelector('.result-section--result')).toBeTruthy()
    expect(container.querySelector('.result-section--citations')).toBeNull()
    expect(container.querySelector('.result-section--derivation')).toBeNull()
  })

  it('keeps limitations visible in the thread, never only in the panel', () => {
    render(<InvestigationResult presentation={view()} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} compact />)
    expect(screen.getByRole('list', { name: 'Limitations' }).textContent).toContain('Only the returned subset is shown.')
  })

  it('stays the stacked layout unless compact is asked for', () => {
    const { container } = render(<InvestigationResult presentation={view()} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} />)
    expect(container.querySelector('.result-section--citations')).toBeTruthy()
  })
})

describe('EvidencePanel', () => {
  it('shows what the answer rests on, its sources and limitations, and closes with Escape or the button', async () => {
    const onClose = vi.fn()
    render(<EvidencePanel turn={{ id: 't1', query: 'Who called most?', presentation: view() }} onClose={onClose} />)
    expect(screen.getByRole('heading', { name: 'Evidence for this answer' })).toBeTruthy()
    expect(screen.getByText('Who called most?')).toBeTruthy()
    expect(screen.getByText('No openable source locator accompanied this response.')).toBeTruthy()
    await userEvent.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(1)
    await userEvent.click(screen.getByRole('button', { name: 'Close evidence panel' }))
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  it('renders nothing without a presentation', () => {
    const { container } = render(<EvidencePanel turn={{ id: 't', query: 'q', presentation: null }} onClose={() => {}} />)
    expect(container.textContent).toBe('')
  })
})
