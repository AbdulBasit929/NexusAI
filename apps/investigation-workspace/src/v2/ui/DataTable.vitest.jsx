import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { columnKind, DataTable } from './DataTable.jsx'

const columns = [{ key: 'contact', label: 'Contact' }, { key: 'calls', label: 'Calls' }, { key: 'last', label: 'Last contact' }]
const many = Array.from({ length: 120 }, (_, index) => ({ contact: `92300000${String(1000 + index)}`, calls: String(index + 1), last: '2026-04-17' }))

describe('columnKind', () => {
  it('reads a long run of digits as an identifier, never as a quantity', () => {
    expect(columnKind({ key: 'contact' }, [{ contact: '923000001038' }, { contact: '923000001001' }])).toBe('text')
    expect(columnKind({ key: 'value' }, [{ value: '923000001038' }])).toBe('text')
  })
  it('reads counts, thousands separators and percentages as numbers', () => {
    expect(columnKind({ key: 'calls' }, [{ calls: '93' }, { calls: '1,204' }, { calls: '5%' }])).toBe('number')
  })
  it('reads dates and words as text', () => {
    expect(columnKind({ key: 'last' }, [{ last: '2026-04-17' }])).toBe('text')
  })
})

describe('DataTable', () => {
  it('says how many rows it shows and offers the rest instead of hiding them', async () => {
    render(<DataTable columns={columns} rows={many} label="Result" />)
    expect(screen.getByRole('status')).toHaveTextContent('Showing 1 to 50 of 120')
    await userEvent.click(screen.getByRole('button', { name: /show 100 more|show 70 more/i }))
    expect(screen.getByRole('status')).toHaveTextContent('120 rows')
    expect(within(screen.getByRole('region', { name: 'Result' })).getAllByRole('row')).toHaveLength(121)
  })

  it('sorts a numeric column as numbers and announces the order', async () => {
    render(<DataTable columns={columns} rows={[{ contact: 'a', calls: '9', last: 'x' }, { contact: 'b', calls: '10', last: 'y' }, { contact: 'c', calls: '2', last: 'z' }]} label="Result" />)
    await userEvent.click(screen.getByRole('button', { name: 'Calls' }))
    expect(screen.getByRole('columnheader', { name: 'Calls' })).toHaveAttribute('aria-sort', 'ascending')
    const cells = screen.getAllByRole('cell').filter((_, index) => index % 3 === 1).map(cell => cell.textContent)
    expect(cells).toEqual(['2', '9', '10'])
    await userEvent.click(screen.getByRole('button', { name: 'Calls' }))
    expect(screen.getByRole('columnheader', { name: 'Calls' })).toHaveAttribute('aria-sort', 'descending')
  })

  it('filters the rows and says how many match', async () => {
    render(<DataTable columns={columns} rows={many} label="Result" />)
    await userEvent.type(screen.getByRole('searchbox'), '1010')
    expect(screen.getByRole('status')).toHaveTextContent('1 of 120 rows match')
  })

  it('shows a dash, with a name, for a value that is missing', () => {
    render(<DataTable columns={columns} rows={[{ contact: 'x', calls: '', last: null }]} label="Result" />)
    expect(screen.getAllByLabelText('no value')).toHaveLength(2)
  })
})
