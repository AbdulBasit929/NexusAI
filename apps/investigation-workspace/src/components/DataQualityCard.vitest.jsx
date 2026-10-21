import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { DataQualityCard, eventLabel, modalityLabel } from './DataQualityCard.jsx'
import { getTypingReport } from '../lib/apiClient.js'

vi.mock('../lib/apiClient.js', async importOriginal => ({ ...(await importOriginal()), getTypingReport: vi.fn() }))

const report = {
  available: true,
  files: { counted: 6, typed: 6 },
  calls: {
    typed: 8941, repeats: 2702, effective: 6239,
    event_types: [{ name: 'data', count: 3460 }, { name: 'voice', count: 1671 }, { name: 'sms', count: 1108 }],
  },
  sessions: { typed: 2500 },
  usage: { typed: 0 },
  evidence: [{ modality: 'structured_records', files: 11, failed: 0 }, { modality: 'image', files: 23, failed: 1 }],
  findings: [
    { code: 'repeats', text: '30% of the call rows (2702) are exact repeats of another row and are left out of counts.' },
    { code: 'no_volume', text: 'The internet records read with ipdr-seed-s8 v1 state no byte count per direction.' },
  ],
}

describe('DataQualityCard', () => {
  beforeEach(() => { getTypingReport.mockReset() })
  afterEach(() => { vi.restoreAllMocks() })

  it('says how many files are typed, how many rows counts leave out, what kinds of row there are, and what to read before trusting an answer', async () => {
    getTypingReport.mockResolvedValue(report)
    render(<DataQualityCard caseId="case-a" />)
    const card = await screen.findByRole('region', { name: 'How the data of this case was read' })
    expect(within(card).getByText(/6,239/)).toBeInTheDocument()
    expect(within(card).getByText(/2,702 repeats left out/)).toBeInTheDocument()
    expect(within(card).getByText('Record files in the typed records')).toBeInTheDocument()
    const kinds = within(card).getByRole('list', { name: 'Kinds of call row' })
    expect(within(kinds).getByText('Voice calls')).toBeInTheDocument()
    expect(within(kinds).getByText('Text messages')).toBeInTheDocument()
    expect(within(card).getByText(/23 images \(1 failed\)/)).toBeInTheDocument()
    const note = within(card).getByRole('note')
    expect(within(note).getByRole('heading', { name: 'Read before trusting an answer' })).toBeInTheDocument()
    expect(within(note).getAllByRole('listitem')).toHaveLength(2)
    expect(getTypingReport).toHaveBeenCalledWith(expect.objectContaining({ caseId: 'case-a' }))
  })

  it('says nothing needs a second look when there are no findings', async () => {
    getTypingReport.mockResolvedValue({ ...report, findings: [] })
    render(<DataQualityCard caseId="case-a" />)
    expect(await screen.findByText('Nothing here needs a second look.')).toBeInTheDocument()
  })

  it('shows nothing for a case with nothing typed, an unavailable read or a failed read', async () => {
    getTypingReport.mockResolvedValueOnce({ available: true, calls: { typed: 0 }, sessions: { typed: 0 }, usage: { typed: 0 }, evidence: [] })
    const first = render(<DataQualityCard caseId="case-a" />)
    await vi.waitFor(() => expect(getTypingReport).toHaveBeenCalledTimes(1))
    expect(first.container).toBeEmptyDOMElement()
    first.unmount()
    getTypingReport.mockResolvedValueOnce({ available: false })
    const second = render(<DataQualityCard caseId="case-a" />)
    await vi.waitFor(() => expect(getTypingReport).toHaveBeenCalledTimes(2))
    expect(second.container).toBeEmptyDOMElement()
    second.unmount()
    getTypingReport.mockRejectedValueOnce(Object.assign(new Error('x'), { status: 404 }))
    const third = render(<DataQualityCard caseId="case-a" />)
    await vi.waitFor(() => expect(getTypingReport).toHaveBeenCalledTimes(3))
    expect(third.container).toBeEmptyDOMElement()
  })

  it('names kinds in plain words and keeps an unknown one as it is written', () => {
    expect(eventLabel('voice')).toBe('Voice calls')
    expect(eventLabel('something_new')).toBe('something_new')
    expect(modalityLabel('structured_records')).toBe('Record files')
    expect(modalityLabel('new_kind')).toBe('new kind')
  })
})
