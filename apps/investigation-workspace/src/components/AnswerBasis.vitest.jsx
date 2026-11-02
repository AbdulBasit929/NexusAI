import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { AnswerEvidence, AnswerReading } from './AnswerBasis.jsx'

const evidence = {
  state: 'itemised',
  total_rows: 3871,
  reconciled: 'yes',
  check: 'Recomputed from the 3,871 matching rows in 2 files: max(duration_s) = 1,800.',
  scope: 'the record with the largest duration (seconds) first, then others it was computed from',
  columns: ['When', 'Kind', 'Direction', 'Subscriber', 'Other party', 'Duration (seconds)'],
  files: [{ evidence_id: 'e1', source_file: 'export-P-000.xlsx', rows: 2000 }, { evidence_id: 'e2', source_file: 'export-P-001.xlsx', rows: 1871 }],
  rows: [
    { key: '1', evidence_id: 'e1', source_file: 'export-P-000.xlsx', row_number: 120, cells: ['2026-03-31 14:46:23 PKT', 'voice', 'incoming', '923000001009', '923000001044', '1800'] },
    { key: '2', evidence_id: 'e2', source_file: 'export-P-001.xlsx', row_number: 0, cells: ['2026-02-02 11:06:33 PKT', 'voice', 'outgoing', '923000001000', '923000001015', '78'] },
  ],
}
const citations = { items: [{ locator: { rowNumber: 120 }, href: '/cases/orion/evidence/e1?version=v1&row=120' }] }

const renderInRouter = element => render(<MemoryRouter>{element}</MemoryRouter>)

describe('AnswerEvidence', () => {
  it('says how many rows in how many files stand behind the figure, and whether it was recomputed from them', () => {
    renderInRouter(<AnswerEvidence evidence={evidence} citations={citations} />)
    expect(screen.getByText(/Behind this answer: 3,871 rows in 2 files/)).toBeInTheDocument()
    expect(screen.getByText('Recomputed from the rows: matches')).toBeInTheDocument()
    expect(screen.getByText(/max\(duration_s\) = 1,800/)).toBeInTheDocument()
  })

  it('lists the files and the records under plain headings, opens a row it could place, and says plainly when it could not place one', () => {
    renderInRouter(<AnswerEvidence evidence={evidence} citations={citations} />)
    const files = screen.getByRole('table', { name: 'Source files behind this answer' })
    expect(within(files).getByText('export-P-001.xlsx')).toBeInTheDocument()
    expect(within(files).getByText('1,871')).toBeInTheDocument()
    const rows = screen.getByRole('table', { name: 'Records behind this answer' })
    for (const heading of ['When', 'Kind', 'Direction', 'Subscriber', 'Other party', 'Duration (seconds)', 'In your file']) expect(within(rows).getByRole('columnheader', { name: heading })).toBeInTheDocument()
    expect(within(rows).getByText('923000001044')).toBeInTheDocument()
    expect(within(rows).getByText('1800')).toBeInTheDocument()
    expect(within(rows).getByText(/export-P-000\.xlsx · row 120/)).toBeInTheDocument()
    expect(within(rows).getByRole('link', { name: 'Open in file' })).toHaveAttribute('href', '/cases/orion/evidence/e1?version=v1&row=120')
    expect(within(rows).getAllByRole('link')).toHaveLength(1)
    expect(within(rows).getByTitle(/could not be found/)).toBeInTheDocument()
    expect(screen.getByText(/Listed: the record with the largest duration \(seconds\) first/)).toBeInTheDocument()
    // nothing an engineer would read: no identifier, no hash, no column of the system
    expect(rows).not.toHaveTextContent(/duration_s|event_type|evidence_id|hash/i)
  })

  it('warns when the rows do not give the figure, and says when the figure was not recomputed', () => {
    const { rerender } = renderInRouter(<AnswerEvidence evidence={{ ...evidence, reconciled: 'no' }} citations={citations} />)
    expect(screen.getByText('Recomputed from the rows: does not match')).toBeInTheDocument()
    rerender(<MemoryRouter><AnswerEvidence evidence={{ ...evidence, reconciled: 'not_checked' }} citations={citations} /></MemoryRouter>)
    expect(screen.getByText('Not recomputed from the rows')).toBeInTheDocument()
  })

  it('says when the records are not listed, and why, and shows nothing when the answer has no evidence block', () => {
    const { container, rerender } = renderInRouter(<AnswerEvidence evidence={{ state: 'not_itemised', reason: 'the answer reads v_contact_pairs, which summarises many records' }} />)
    expect(screen.getByText(/records behind this answer are not listed/)).toBeInTheDocument()
    expect(screen.getByText(/The answer reads v_contact_pairs/)).toBeInTheDocument()
    rerender(<MemoryRouter><AnswerEvidence /></MemoryRouter>)
    expect(container).toBeEmptyDOMElement()
  })
})

describe('AnswerReading', () => {
  it('asks for the reading once the figure is shown, says it is being written, then shows it with its disclosure', async () => {
    const fetchReading = vi.fn().mockResolvedValue({ state: 'ready', text: 'The longest voice call lasted 1,800 seconds.', disclosure: 'Written by the local language model from the rows of this result only. Check it against the rows.' })
    render(<AnswerReading caseId="orion" reading={{ state: 'pending', token: 'abc123' }} fetchReading={fetchReading} />)
    expect(screen.getByRole('status')).toHaveTextContent('Reading the result…')
    expect(await screen.findByText('The longest voice call lasted 1,800 seconds.')).toBeInTheDocument()
    expect(screen.getByText(/Check it against the rows/)).toBeInTheDocument()
    expect(fetchReading).toHaveBeenCalledTimes(1)
    expect(fetchReading.mock.calls[0][0]).toMatchObject({ caseId: 'orion', token: 'abc123' })
  })

  it('says why no reading is shown, instead of showing one that was not checked', async () => {
    const fetchReading = vi.fn().mockResolvedValue({ state: 'unavailable', reason: 'a reading was written but did not pass the check against the rows (the number 4,000 does not occur), so none is shown' })
    render(<AnswerReading caseId="orion" reading={{ state: 'pending', token: 'abc123' }} fetchReading={fetchReading} />)
    expect(await screen.findByText(/No reading is shown: a reading was written but did not pass the check/)).toBeInTheDocument()
  })

  it('treats a failed fetch as no reading, never as an error that hides the answer', async () => {
    const fetchReading = vi.fn().mockRejectedValue(new Error('HTTP 404'))
    render(<AnswerReading caseId="orion" reading={{ state: 'pending', token: 'abc123' }} fetchReading={fetchReading} />)
    expect(await screen.findByText(/No reading is shown: it could not be fetched; ask the question again/)).toBeInTheDocument()
  })

  it('shows nothing for an answer that has no reading', () => {
    const fetchReading = vi.fn()
    const { container } = render(<AnswerReading caseId="orion" fetchReading={fetchReading} />)
    expect(container).toBeEmptyDOMElement()
    expect(fetchReading).not.toHaveBeenCalled()
  })
})
