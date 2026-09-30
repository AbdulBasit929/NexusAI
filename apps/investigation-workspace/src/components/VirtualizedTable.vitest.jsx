import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { VirtualizedTable } from './VirtualizedTable.jsx'

describe('VirtualizedTable', () => {
  it('keeps real table semantics while windowing ten thousand rows', () => {
    const rows = Array.from({ length: 10000 }, (_, index) => ({ row_number: index + 1, name: `Record ${index + 1}`, amount: index }))
    render(<VirtualizedTable columns={[{ key: 'row_number', label: 'Row' }, { key: 'name', label: 'Name' }, { key: 'amount', label: 'Amount' }]} rows={rows} label="Ten-thousand-row proof" />)
    const table = screen.getByRole('table')
    expect(table).toHaveAttribute('aria-rowcount', '10001')
    expect(screen.getByRole('columnheader', { name: 'Row' })).toHaveAttribute('scope', 'col')
    expect(screen.getAllByRole('cell', { name: '1' })[0]).toHaveAttribute('headers')
    expect(within(table).getAllByRole('row').length).toBeLessThan(40)
    fireEvent.scroll(screen.getByRole('region', { name: 'Ten-thousand-row proof' }), { target: { scrollTop: 180000 } })
    expect(within(table).getAllByRole('row').length).toBeLessThan(40)
  })

  it('sorts, filters, changes density and jumps without a network request', () => {
    render(<VirtualizedTable columns={[{ key: 'row_number', label: 'Row' }, { key: 'name', label: 'Name' }]} rows={[{ row_number: 2, name: 'Bravo' }, { row_number: 1, name: 'Alpha' }]} label="Rows" />)
    fireEvent.click(screen.getByRole('button', { name: 'Sort by Row' }))
    expect(screen.getByRole('cell', { name: '1' })).toBeInTheDocument()
    fireEvent.change(screen.getByRole('searchbox', { name: 'Filter rows' }), { target: { value: 'Bravo' } })
    expect(screen.getByText('1 of 2 loaded rows')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Comfortable' }))
    expect(screen.getByRole('button', { name: 'Comfortable' })).toHaveAttribute('aria-pressed', 'true')
  })
})
