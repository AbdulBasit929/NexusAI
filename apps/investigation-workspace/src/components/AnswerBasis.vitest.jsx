import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { AnswerEvidence, AnswerReading, recordHref } from './AnswerBasis.jsx'

const columns = ['When', 'Kind', 'Direction', 'Subscriber', 'Other party', 'Duration (seconds)']
const located = { key: '1', evidence_id: 'e1', version_id: 'v1', source_file: 'export-P-000.xlsx', row_number: 120, row_hash: 'h1', cells: ['2026-03-31 14:46:23 PKT', 'voice', 'incoming', '923000001009', '923000001044', '1800'] }
const unplaced = { key: '2', evidence_id: 'e2', version_id: 'v2', source_file: 'export-P-001.xlsx', row_number: 0, cells: ['2026-02-02 11:06:33 PKT', 'voice', 'outgoing', '923000001000', '923000001015', '78'] }

const evidence = {
  state: 'itemised',
  kind: 'aggregate',
  total_rows: 3871,
  reconciled: 'yes',
  check: 'Recomputed from the 3,871 matching rows in 2 files: max(duration_s) = 1,800.',
  scope: 'the record with the largest duration (seconds) first, then others it was computed from',
  columns,
  files: [{ evidence_id: 'e1', source_file: 'export-P-000.xlsx', rows: 2000 }, { evidence_id: 'e2', source_file: 'export-P-001.xlsx', rows: 1871 }],
  rows: [located, unplaced],
}

const renderInRouter = element => render(<MemoryRouter>{element}</MemoryRouter>)

describe('recordHref', () => {
  it('opens a record in its file at its row, only when the file, the version, the row and its hash are all known', () => {
    expect(recordHref('orion', located)).toBe('/cases/orion/evidence/e1?version=v1&row=120&row_hash=h1')
    expect(recordHref('orion', unplaced)).toBe('')
    expect(recordHref('orion', { ...located, row_hash: '' })).toBe('')
    expect(recordHref('orion', undefined)).toBe('')
  })
})

describe('AnswerEvidence', () => {
  it('says how many records in how many files stand behind the figure, and whether it was recomputed from them', () => {
    renderInRouter(<AnswerEvidence evidence={evidence} caseId="orion" />)
    expect(screen.getByText(/Behind this answer: 3,871 records in 2 files/)).toBeInTheDocument()
    expect(screen.getByText('Recomputed from the records: matches')).toBeInTheDocument()
    expect(screen.getByText(/max\(duration_s\) = 1,800/)).toBeInTheDocument()
  })

  it('lists the files and the records under plain headings, opens a record it could place, and says plainly when it could not place one', () => {
    renderInRouter(<AnswerEvidence evidence={evidence} caseId="orion" />)
    const files = screen.getByRole('table', { name: 'Source files behind this answer' })
    expect(within(files).getByText('export-P-001.xlsx')).toBeInTheDocument()
    expect(within(files).getByText('1,871')).toBeInTheDocument()
    const rows = screen.getByRole('table', { name: 'Records behind this answer' })
    for (const heading of [...columns, 'In your file']) expect(within(rows).getByRole('columnheader', { name: heading })).toBeInTheDocument()
    expect(within(rows).getByText('923000001044')).toBeInTheDocument()
    expect(within(rows).getByText(/export-P-000\.xlsx · row 120/)).toBeInTheDocument()
    expect(within(rows).getByRole('link', { name: 'Open in file' })).toHaveAttribute('href', '/cases/orion/evidence/e1?version=v1&row=120&row_hash=h1')
    expect(within(rows).getAllByRole('link')).toHaveLength(1)
    expect(within(rows).getByTitle(/could not be found/)).toBeInTheDocument()
    expect(screen.getByText(/Listed: the record with the largest duration \(seconds\) first/)).toBeInTheDocument()
    // nothing an engineer would read: no identifier, no hash, no column of the system
    expect(rows).not.toHaveTextContent(/duration_s|event_type|evidence_id|hash|h1/i)
  })

  it('warns when the records do not give the figure, and says when the figure was not recomputed', () => {
    const { rerender } = renderInRouter(<AnswerEvidence evidence={{ ...evidence, reconciled: 'no' }} caseId="orion" />)
    expect(screen.getByText('Recomputed from the records: does not match')).toBeInTheDocument()
    rerender(<MemoryRouter><AnswerEvidence evidence={{ ...evidence, reconciled: 'not_checked' }} caseId="orion" /></MemoryRouter>)
    expect(screen.getByText('Not recomputed from the records')).toBeInTheDocument()
  })

  it('says when the records are not listed, and why, and shows nothing when the answer has no evidence block', () => {
    const { container, rerender } = renderInRouter(<AnswerEvidence evidence={{ state: 'not_itemised', reason: 'the answer follows contacts through several steps, so no single set of records stands behind it' }} caseId="orion" />)
    expect(screen.getByText(/records behind this answer are not listed/)).toBeInTheDocument()
    expect(screen.getByText(/The answer follows contacts through several steps/)).toBeInTheDocument()
    rerender(<MemoryRouter><AnswerEvidence caseId="orion" /></MemoryRouter>)
    expect(container).toBeEmptyDOMElement()
  })
})

describe('AnswerEvidence for the lines of a summary', () => {
  const lines = {
    state: 'itemised', kind: 'lines', reconciled: 'yes', lines_total: 3, lines_token: 'tok',
    check: 'Each line\'s own numbers were recomputed from its records: 2 of 2 agree.',
    say: 'the calls and messages of the first number to or from the second, in its own records',
    columns, files: [{ evidence_id: 'e1', source_file: 'export-P-000.xlsx', rows: 150 }], total_rows: 150, rows: [located],
    lines: [
      { index: 1, label: '923000001005 → 923000001036', records: 150, files: 1, reconciled: 'yes', check: 'Recomputed from its records: events = 150; first_seen = 2026-02-02 00:29:05.' },
      { index: 2, label: '923000001005 → 923000001093', records: 142, files: 2, reconciled: 'yes' },
      { index: 3, label: '923000001005 → 923000001074', records: 102, files: 1, reconciled: 'not_checked' },
    ],
  }

  it('lists the lines with how many records each stands for, and the records of the first line', () => {
    renderInRouter(<AnswerEvidence evidence={lines} caseId="orion" fetchLine={vi.fn()} />)
    expect(screen.getByText(/Behind this answer: 3 lines, each standing for records/)).toBeInTheDocument()
    const table = screen.getByRole('table', { name: 'Lines of this answer' })
    expect(within(table).getByText('923000001005 → 923000001093')).toBeInTheDocument()
    expect(within(table).getByText('142')).toBeInTheDocument()
    expect(within(table).getAllByText('matches')).toHaveLength(2)
    expect(screen.getByText(/Recomputed from its records: events = 150/)).toBeInTheDocument()
    expect(within(screen.getByRole('table', { name: 'Records behind this answer' })).getByText('923000001044')).toBeInTheDocument()
    expect(screen.getByText(/The calls and messages of the first number to or from the second/)).toBeInTheDocument()
  })

  it('opens the records of another line on request, and says when they could not be listed', async () => {
    const fetchLine = vi.fn().mockResolvedValue({ line: 2, label: '923000001005 → 923000001093', evidence: { say: 'the calls and messages of the first number to or from the second, in its own records', columns, total_rows: 142, files: [{ evidence_id: 'e9', source_file: 'export-P-009.xlsx', rows: 142 }], rows: [{ ...located, key: '9', source_file: 'export-P-009.xlsx', cells: ['2026-03-01 10:00:00 PKT', 'voice', 'outgoing', '923000001005', '923000001093', '44'] }] } })
    renderInRouter(<AnswerEvidence evidence={lines} caseId="orion" fetchLine={fetchLine} />)
    const table = screen.getByRole('table', { name: 'Lines of this answer' })
    fireEvent.click(within(table).getAllByRole('button', { name: 'Show records' })[0])
    expect(await screen.findByText(/Records behind line 2: 923000001005 → 923000001093/)).toBeInTheDocument()
    expect(await screen.findByText('export-P-009.xlsx')).toBeInTheDocument()
    expect(fetchLine).toHaveBeenCalledTimes(1)
    expect(fetchLine.mock.calls[0][0]).toMatchObject({ caseId: 'orion', token: 'tok', line: 2 })
  })

  it('says plainly when the records of a line could not be listed, and keeps the rest of the answer', async () => {
    const failing = vi.fn().mockRejectedValue(new Error('HTTP 404'))
    renderInRouter(<AnswerEvidence evidence={lines} caseId="orion" fetchLine={failing} />)
    fireEvent.click(within(screen.getByRole('table', { name: 'Lines of this answer' })).getAllByRole('button', { name: 'Show records' })[1])
    expect(await screen.findByText(/records of this line could not be listed/)).toBeInTheDocument()
    expect(screen.getByRole('table', { name: 'Lines of this answer' })).toBeInTheDocument()
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
