import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { EngineHealthCard, reasonLabel, seconds } from './EngineHealthCard.jsx'
import { getEngineHealth } from '../lib/apiClient.js'

vi.mock('../lib/apiClient.js', async importOriginal => ({ ...(await importOriginal()), getEngineHealth: vi.fn() }))

const health = {
  available: true,
  totals: { questions: 40, answered: 30, abstained: 1, declined: 9 },
  latency_ms: { p50: 31200, p90: 74000, max: 120000 },
  repairs: 6,
  declines: [
    { reason: 'no analyst view fits the question closely enough', count: 6 },
    { reason: 'the analyst views cannot speak for this case: the case has no typed call records', count: 3 },
  ],
}

describe('EngineHealthCard', () => {
  beforeEach(() => { getEngineHealth.mockReset() })
  afterEach(() => { vi.restoreAllMocks() })

  it('says what the engine answered, how long it took, how often it repaired, and why it declined', async () => {
    getEngineHealth.mockResolvedValue(health)
    render(<EngineHealthCard caseId="case-a" days={7} />)
    const card = await screen.findByRole('region', { name: /How the Ask engine has done here, last 7 days/ })
    expect(within(card).getByText('40')).toBeInTheDocument()
    expect(within(card).getByText('75%')).toBeInTheDocument()
    expect(within(card).getByText('31 s')).toBeInTheDocument()
    expect(within(card).getByText('74 s')).toBeInTheDocument()
    expect(within(card).getByText('0.2')).toBeInTheDocument()
    const reasons = within(card).getByRole('list', { name: 'Why the engine did not answer' })
    expect(within(reasons).getByText('No view of the records fits the question')).toBeInTheDocument()
    expect(within(reasons).getByText('The case is not fully typed yet: the older paths answered')).toBeInTheDocument()
    expect(getEngineHealth).toHaveBeenCalledWith(expect.objectContaining({ caseId: 'case-a', days: 7 }))
  })

  it('shows nothing for a case with no questions, an unavailable read or a failed read', async () => {
    getEngineHealth.mockResolvedValueOnce({ available: true, totals: { questions: 0 } })
    const first = render(<EngineHealthCard caseId="case-a" />)
    await vi.waitFor(() => expect(getEngineHealth).toHaveBeenCalledTimes(1))
    expect(first.container).toBeEmptyDOMElement()
    first.unmount()
    getEngineHealth.mockResolvedValueOnce({ available: false })
    const second = render(<EngineHealthCard caseId="case-a" />)
    await vi.waitFor(() => expect(getEngineHealth).toHaveBeenCalledTimes(2))
    expect(second.container).toBeEmptyDOMElement()
    second.unmount()
    getEngineHealth.mockRejectedValueOnce(Object.assign(new Error('x'), { status: 404 }))
    const third = render(<EngineHealthCard caseId="case-a" />)
    await vi.waitFor(() => expect(getEngineHealth).toHaveBeenCalledTimes(3))
    expect(third.container).toBeEmptyDOMElement()
  })

  it('writes times in seconds and keeps an unknown reason as it is written', () => {
    expect(seconds(1250)).toBe('1.3 s')
    expect(seconds(31200)).toBe('31 s')
    expect(seconds(null)).toBe('-')
    expect(reasonLabel('something new')).toBe('something new')
  })
})
