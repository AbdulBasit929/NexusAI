import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { SourceRowCard } from './SourceRowCard.jsx'

const found = {
  state: 'found',
  source_file: 'export-P-000-call-c1-company.xlsx',
  row_number: 93,
  kind: 'call or message',
  cells: [{ name: 'MSISDN', value: '923000001009' }, { name: 'STRT_TM', value: '31/03/2026 14:46:23' }, { name: 'BNUMBER', value: '03000001044' }, { name: 'SECS', value: '1800' }],
}

describe('SourceRowCard', () => {
  it('shows the row under the headings of the file, with the file and the row in words, and nothing technical', async () => {
    const fetchRow = vi.fn().mockResolvedValue(found)
    render(<SourceRowCard caseId="orion" evidenceId="e1" row="93" rowHash="abc" fetchRow={fetchRow} />)
    const card = await screen.findByRole('region', { name: 'This row in your file' })
    expect(card).toHaveTextContent('export-P-000-call-c1-company.xlsx · row 93 · call or message')
    expect(within(card).getByText('STRT_TM')).toBeInTheDocument()
    expect(card).toHaveTextContent(/MSISDN\s*923000001009/)
    expect(card).toHaveTextContent(/STRT_TM\s*31\/03\/2026 14:46:23/)
    expect(card).toHaveTextContent(/SECS\s*1800/)
    expect(card).not.toHaveTextContent(/hash|record id|batch|raw payload/i)
    expect(fetchRow.mock.calls[0][0]).toMatchObject({ caseId: 'orion', evidenceId: 'e1', row: '93', rowHash: 'abc' })
  })

  it('says why a row cannot be shown by column name, and that the stored records are below', async () => {
    const fetchRow = vi.fn().mockResolvedValue({ state: 'not_found', reason: 'this row is not one of the file\'s records (a title or a heading row, or a row the system did not read)' })
    render(<SourceRowCard caseId="orion" evidenceId="e1" row="2" fetchRow={fetchRow} />)
    expect(await screen.findByText(/cannot be shown by column name: this row is not one of the file's records/)).toBeInTheDocument()
    expect(screen.getByText(/rows of the file are listed below/)).toBeInTheDocument()
  })

  it('names the column of the workbook beside each heading when the file is a spreadsheet, and keeps the cells in the order given', async () => {
    const fetchRow = vi.fn().mockResolvedValue({ ...found, cells: [{ name: 'Sr #', value: '5', cell: 'A93' }, { name: 'IMEI', value: '000000000010009', cell: 'H93' }] })
    render(<SourceRowCard caseId="orion" evidenceId="e1" row="93" fetchRow={fetchRow} />)
    const card = await screen.findByRole('region', { name: 'This row in your file' })
    expect(within(card).getByTitle('Cell H93 of the workbook')).toHaveTextContent('H')
    expect(within(card).getByTitle('Cell A93 of the workbook')).toHaveTextContent('A')
    expect(card.textContent.indexOf('Sr #')).toBeLessThan(card.textContent.indexOf('IMEI'))
  })

  it('treats a failed read as a row that cannot be shown, never as an error that hides the page', async () => {
    const fetchRow = vi.fn().mockRejectedValue(new Error('HTTP 500'))
    render(<SourceRowCard caseId="orion" evidenceId="e1" row="93" fetchRow={fetchRow} />)
    expect(await screen.findByText(/cannot be shown by column name: the row could not be read/)).toBeInTheDocument()
  })

  it('shows nothing when the link names no row', () => {
    const fetchRow = vi.fn()
    const { container } = render(<SourceRowCard caseId="orion" evidenceId="e1" fetchRow={fetchRow} />)
    expect(container).toBeEmptyDOMElement()
    expect(fetchRow).not.toHaveBeenCalled()
  })
})
