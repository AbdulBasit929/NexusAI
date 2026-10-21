import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { CaseDataPanel } from './CaseDataPanel.jsx'
import { getEngineHealth, getTypingReport } from '../../lib/apiClient.js'

vi.mock('../../lib/apiClient.js', async importOriginal => ({ ...(await importOriginal()), getEngineHealth: vi.fn(), getTypingReport: vi.fn() }))

describe('CaseDataPanel', () => {
  beforeEach(() => {
    getEngineHealth.mockReset().mockResolvedValue({ available: true, totals: { questions: 4, answered: 4, abstained: 0, declined: 0 }, latency_ms: { p50: 20000, p90: 30000 }, repairs: 0, declines: [] })
    getTypingReport.mockReset().mockResolvedValue({ available: true, files: { counted: 2, typed: 2 }, calls: { typed: 10, repeats: 0, effective: 10, event_types: [{ name: 'voice', count: 10 }] }, sessions: { typed: 0 }, usage: { typed: 0 }, evidence: [], findings: [] })
  })

  it('reads nothing until it is opened, then shows the data card and the engine card', async () => {
    const { container } = render(<CaseDataPanel caseId="case-a" />)
    expect(screen.getByText('About the data here and how the engine has done')).toBeInTheDocument()
    expect(getTypingReport).not.toHaveBeenCalled()
    expect(getEngineHealth).not.toHaveBeenCalled()
    const details = container.querySelector('details')
    details.open = true
    fireEvent(details, new Event('toggle'))
    expect(await screen.findByRole('region', { name: 'How the data of this case was read' })).toBeInTheDocument()
    expect(await screen.findByRole('region', { name: /How the Ask engine has done here/ })).toBeInTheDocument()
    expect(getTypingReport).toHaveBeenCalledTimes(1)
    expect(getEngineHealth).toHaveBeenCalledTimes(1)
  })
})
