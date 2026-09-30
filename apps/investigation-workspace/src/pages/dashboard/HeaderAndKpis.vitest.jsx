import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { DashboardHeader, headerFacts, updatedText } from './DashboardHeader.jsx'
import { KpiTiles, kpiTiles } from './KpiTiles.jsx'

const kpis = { cases: 2, unreported: 0, sources: 55, ready: 53, processing: 0, failed: 2, gaps: 3, review: 5, acceptedRows: 23080 }
const show = ui => render(<MemoryRouter>{ui}</MemoryRouter>)

describe('D1 page header', () => {
  it('states scope and freshness as facts and only mentions polling while it is real', () => {
    const now = new Date('2026-09-30T16:45:00Z')
    expect(headerFacts({ caseCount: 2, lastUpdated: now, processing: false }).map(fact => fact.label)).toEqual(['Scope', 'Updated'])
    const processing = headerFacts({ caseCount: 1, lastUpdated: null, processing: true })
    expect(processing[0].value).toBe('1 case in this workspace')
    expect(processing[1].value).toBe('Reading…')
    expect(processing[2]).toEqual({ label: 'Refresh', value: 'Every 15 s while evidence is processing' })
    expect(updatedText(null)).toBe('Reading…')
  })

  it('has one h1, a Refresh that calls back and reports progress, and Add evidence', async () => {
    const onRefresh = vi.fn()
    const { rerender } = show(<DashboardHeader caseCount={2} lastUpdated={new Date('2026-09-30T16:45:00Z')} processing={false} refreshing={false} onRefresh={onRefresh} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Dashboard' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Add evidence' }).getAttribute('href')).toBe('/cases/new')
    await userEvent.setup().click(screen.getByRole('button', { name: 'Refresh' }))
    expect(onRefresh).toHaveBeenCalledOnce()
    expect(screen.getByRole('status').textContent).toMatch(/^Case status updated at /)
    rerender(<MemoryRouter><DashboardHeader caseCount={2} lastUpdated={null} processing={false} refreshing onRefresh={onRefresh} /></MemoryRouter>)
    expect(screen.getByRole('button', { name: 'Refreshing…' }).hasAttribute('disabled')).toBe(true)
    expect(screen.getByRole('status').textContent).toBe('Refreshing case status')
  })
})

describe('D2 KPI tiles', () => {
  it('orders the four figures by priority and links each to the section behind it', () => {
    const tiles = kpiTiles(kpis)
    expect(tiles.map(tile => tile.label)).toEqual(['Needs review', 'Evidence ready', 'Processing', 'Structured rows'])
    expect(tiles.map(tile => tile.href)).toEqual(['#dashboard-attention', '#dashboard-readiness', '#dashboard-readiness', '#dashboard-families'])
    expect(tiles[0].value).toBe(5)
    expect(tiles[1]).toMatchObject({ value: 53, valueSuffix: 'of 55', bar: { total: 55, ready: 53, processing: 0, failed: 2 } })
  })

  it('tones Needs review only when there is something to review', () => {
    expect(kpiTiles(kpis)[0].tone).toBe('attention')
    expect(kpiTiles({ ...kpis, review: 0 })[0].tone).toBeUndefined()
  })

  it('shows exact figures, a definition for each and a composition bar with a text equivalent', () => {
    const { container } = render(<KpiTiles kpis={kpis} loading={false} />)
    const list = screen.getByRole('list', { name: 'Workspace totals' })
    expect(within(list).getAllByRole('listitem')).toHaveLength(4)
    expect(container.textContent).toContain('23,080')
    expect(container.textContent).toContain('53 of 55')
    expect(container.textContent).toContain('Failed sources and completed jobs missing their retained copy')
    expect(screen.getByRole('img', { name: 'Evidence readiness across all cases: ready 53, processing 0, failed 2, other 0' })).toBeTruthy()
  })

  it('shows an ellipsis, never zero, while status is unknown and marks the list busy', () => {
    const { container } = render(<KpiTiles kpis={{ ...kpis, review: 0, ready: 0, sources: 0 }} loading />)
    expect(container.querySelectorAll('.dash-kpi strong')).toHaveLength(4)
    for (const value of container.querySelectorAll('.dash-kpi strong')) expect(value.textContent).toBe('…')
    expect(screen.getByRole('list', { name: 'Workspace totals' }).getAttribute('aria-busy')).toBe('true')
    expect(screen.queryByRole('img')).toBeNull()
  })
})
