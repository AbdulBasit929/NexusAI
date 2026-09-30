import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { ChartCard } from './ChartCard.jsx'

// The chart engine is exercised in the browser; here the frame, the toggle and the table path are the contract.
vi.mock('./EChart.jsx', () => ({ default: ({ label }) => <div role="img" aria-label={label} /> }))

const columns = [
  { key: 'name', label: 'Case' },
  { key: 'ready', label: 'Ready', numeric: true, render: row => <a href={`/x/${row.name}`}>{row.ready}</a> },
]
const rows = [{ key: 'a', name: 'case-a', ready: 42 }]
const chart = { buildOption: () => ({}), label: 'Readiness. case-a: 42 of 43 ready', height: 100, legend: [{ label: 'Ready', tone: 'ready' }] }

function show(props = {}) {
  return render(<MemoryRouter><ChartCard title="Is each case ready?" chart={chart} columns={columns} rows={rows} {...props} /></MemoryRouter>)
}

describe('ChartCard', () => {
  it('titles the chart with the question and gives it an exact text equivalent', async () => {
    show()
    expect(screen.getByRole('heading', { name: 'Is each case ready?' })).toBeTruthy()
    expect(await screen.findByRole('img', { name: 'Readiness. case-a: 42 of 43 ready' })).toBeTruthy()
    expect(screen.getByRole('list', { name: 'Legend' })).toBeTruthy()
  })

  it('offers the same values as a table whose cells are real links', async () => {
    show()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /Table/ }))
    expect(screen.getByRole('table')).toBeTruthy()
    expect(screen.getByRole('rowheader', { name: 'case-a' })).toBeTruthy()
    expect(screen.getByRole('link', { name: '42' }).getAttribute('href')).toBe('/x/case-a')
    expect(screen.getByRole('button', { name: /Table/ }).getAttribute('aria-pressed')).toBe('true')
    await user.click(screen.getByRole('button', { name: /Chart/ }))
    expect(screen.queryByRole('table')).toBeNull()
  })
})
