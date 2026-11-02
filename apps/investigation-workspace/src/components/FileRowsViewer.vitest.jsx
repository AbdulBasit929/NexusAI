import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { FileRowsViewer } from './FileRowsViewer.jsx'

const page = (overrides = {}) => ({
  contract: 'forensics.file-rows/v1',
  total: 408, first_row: 2, last_row: 412, from: 85, limit: 50,
  columns: [{ name: 'Sr #', letter: 'A' }, { name: 'A-Party', letter: 'B' }, { name: 'Duration', letter: 'H' }, { name: 'CNIC' }],
  rows: [
    { row_number: 92, cells: ['90', '923000001000', '78', 'withheld'] },
    { row_number: 93, cells: ['91', '923000001009', '1800', ''] },
    { row_number: 94, cells: ['92', '923000001010', '', ''] },
  ],
  ...overrides,
})

describe('FileRowsViewer', () => {
  it('shows the rows of the file under its own columns, marks the cited row, and says where the page sits in the file', async () => {
    const fetchRows = vi.fn().mockResolvedValue(page())
    render(<FileRowsViewer caseId="orion" evidenceId="e1" citedRow="93" fetchRows={fetchRows} />)
    const table = await screen.findByRole('table')
    for (const heading of ['Row', 'Sr #', 'A-Party', 'Duration', 'CNIC']) expect(within(table).getByRole('columnheader', { name: new RegExp(`^(?:[A-Z]+)?\\s*${heading}$`) })).toBeInTheDocument()
    expect(screen.getByText(/Rows/)).toHaveTextContent('Rows 92–94 of 408')
    const cited = within(table).getByRole('row', { current: true })
    expect(cited).toHaveTextContent('93')
    expect(cited).toHaveTextContent('Cited')
    expect(cited).toHaveTextContent('923000001009')
    expect(within(table).getAllByText('withheld')).toHaveLength(1)
    expect(fetchRows.mock.calls[0][0]).toMatchObject({ caseId: 'orion', evidenceId: 'e1', from: 85, limit: 50 })
    // no identifier, no hash, no system column
    expect(table).not.toHaveTextContent(/hash|record id|batch|raw payload/i)
  })

  it('starts at the top of the file when no row is cited', async () => {
    const fetchRows = vi.fn().mockResolvedValue(page({ rows: [{ row_number: 2, cells: ['1', 'a', '0', ''] }] }))
    render(<FileRowsViewer caseId="orion" evidenceId="e1" fetchRows={fetchRows} />)
    await screen.findByRole('table')
    expect(fetchRows.mock.calls[0][0]).toMatchObject({ from: 1 })
    expect(screen.queryByText('Cited')).not.toBeInTheDocument()
  })

  it('moves to the next rows, to a row the analyst asks for, and hides the columns that are empty on the page', async () => {
    const fetchRows = vi.fn().mockResolvedValue(page())
    render(<FileRowsViewer caseId="orion" evidenceId="e1" citedRow="93" fetchRows={fetchRows} />)
    await screen.findByRole('table')
    fireEvent.click(screen.getByRole('button', { name: 'Next rows' }))
    await waitFor(() => expect(fetchRows).toHaveBeenCalledTimes(2))
    expect(fetchRows.mock.calls[1][0]).toMatchObject({ from: 95 })
    const input = screen.getByLabelText('Go to row')
    fireEvent.change(input, { target: { value: '200' } })
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    await waitFor(() => expect(fetchRows.mock.calls.at(-1)[0]).toMatchObject({ from: 192 }))
    // a column that holds only withheld cells is not empty: it stays
    fireEvent.click(screen.getByLabelText('Hide empty columns'))
    expect(screen.getByRole('columnheader', { name: /CNIC/ })).toBeInTheDocument()
  })

  it('hides a column that is empty on the page, and keeps the others', async () => {
    const fetchRows = vi.fn().mockResolvedValue(page({ columns: [{ name: 'A-Party' }, { name: 'IMSI' }], rows: [{ row_number: 93, cells: ['923000001009', ''] }, { row_number: 94, cells: ['923000001010', ''] }] }))
    render(<FileRowsViewer caseId="orion" evidenceId="e1" citedRow="93" fetchRows={fetchRows} />)
    await screen.findByRole('table')
    expect(screen.getByRole('columnheader', { name: /IMSI/ })).toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Hide empty columns'))
    expect(screen.queryByRole('columnheader', { name: /IMSI/ })).not.toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: /A-Party/ })).toBeInTheDocument()
  })

  it('hands a row the analyst picks to the inspector under the file\'s own column names', async () => {
    const onRowSelect = vi.fn()
    render(<FileRowsViewer caseId="orion" evidenceId="e1" citedRow="93" onRowSelect={onRowSelect} fetchRows={vi.fn().mockResolvedValue(page())} />)
    const table = await screen.findByRole('table')
    fireEvent.click(within(table).getAllByRole('row')[1])
    expect(onRowSelect).toHaveBeenCalledTimes(1)
    const [values, columns] = onRowSelect.mock.calls[0]
    expect(values).toMatchObject({ __row: 92, 'Sr #': '90', 'A-Party': '923000001000', Duration: '78', CNIC: '' })
    expect(columns.map(column => column.label)).toEqual(['Row in the file', 'Sr #', 'A-Party', 'Duration', 'CNIC'])
  })

  it('calls the numbers records, not rows, when they are the typed rows\' own order (a workbook in the old XML format, a text export)', async () => {
    const fetchRows = vi.fn().mockResolvedValue(page({ row_label: 'Record', total: 264, first_row: 1, last_row: 273, rows: [{ row_number: 1, cells: ['1', '923000001000', '78', ''] }] }))
    render(<FileRowsViewer caseId="orion" evidenceId="e1" fetchRows={fetchRows} />)
    const table = await screen.findByRole('table')
    expect(within(table).getByRole('columnheader', { name: 'Record' })).toBeInTheDocument()
    expect(screen.getByText('The records of the file')).toBeInTheDocument()
    expect(screen.getByText(/records in this file, under its own headings/)).toHaveTextContent('Records 1–1 · 264 records in this file')
    expect(screen.getByLabelText('Go to record')).toBeInTheDocument()
  })

  it('shows what it was given instead when the service has no such route, so the page never goes blank', async () => {
    const fetchRows = vi.fn().mockRejectedValue(new Error('HTTP 404'))
    render(<FileRowsViewer caseId="orion" evidenceId="e1" fetchRows={fetchRows} fallback={<p>Stored records of the file</p>} />)
    expect(await screen.findByText('Stored records of the file')).toBeInTheDocument()
  })
})
