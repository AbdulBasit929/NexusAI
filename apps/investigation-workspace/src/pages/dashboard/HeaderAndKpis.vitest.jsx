import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { DashboardHeader, ageText, headerFacts, isStale, updatedText } from './DashboardHeader.jsx'
import { KpiTiles, kpiTiles } from './KpiTiles.jsx'

const kpis = { cases: 2, reported: 2, unreported: 0, sources: 55, ready: 53, processing: 0, failed: 2, gaps: 3, review: 5, acceptedRows: 23080 }
const show = ui => render(<MemoryRouter>{ui}</MemoryRouter>)

describe('D1 page header', () => {
  it('states scope and freshness as facts and only mentions polling while it is real', () => {
    const now = new Date('2026-09-30T16:45:00Z')
    expect(headerFacts({ caseCount: 2, lastUpdated: now, processing: false }).map(fact => fact.label)).toEqual(['Scope', 'Updated'])
    const processing = headerFacts({ caseCount: 1, lastUpdated: null, processing: true, loading: true })
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

describe('D1 freshness and coverage (research: a freshness signal is owed, stale is its own state)', () => {
  const at = new Date('2026-09-30T16:45:00Z')
  it('says how old the figures are in words', () => {
    expect(ageText(at, new Date('2026-09-30T16:45:20Z'))).toBe('just now')
    expect(ageText(at, new Date('2026-09-30T16:48:30Z'))).toBe('3 min ago')
    expect(ageText(at, new Date('2026-09-30T19:46:00Z'))).toBe('3 h ago')
    expect(ageText(at, new Date('2026-10-02T16:45:00Z'))).toBe('over a day ago')
    expect(ageText(null, at)).toBe('')
  })

  it('treats figures older than five minutes as stale, never before', () => {
    expect(isStale(at, new Date('2026-09-30T16:49:59Z'))).toBe(false)
    expect(isStale(at, new Date('2026-09-30T16:50:01Z'))).toBe(true)
    expect(isStale(null, at)).toBe(false)
  })

  it('shows the age beside the time and warns in words when stale', () => {
    const fresh = headerFacts({ caseCount: 2, reporting: 2, lastUpdated: at, processing: false, now: new Date('2026-09-30T16:46:00Z') })
    expect(fresh[1].value).toMatch(/· 1 min ago$/)
    expect(fresh[1].stale).toBeFalsy()
    const stale = headerFacts({ caseCount: 2, reporting: 2, lastUpdated: at, processing: false, now: new Date('2026-09-30T17:15:00Z') })
    expect(stale[1].value).toMatch(/· 30 min ago, may be out of date$/)
    expect(stale[1].stale).toBe(true)
  })

  it('names how many cases actually reported instead of claiming all of them', () => {
    expect(headerFacts({ caseCount: 4, reporting: 3, lastUpdated: at, processing: false, now: at })[0].value).toBe('3 of 4 cases reporting')
    expect(headerFacts({ caseCount: 1, reporting: 1, lastUpdated: at, processing: false, now: at })[0].value).toBe('1 case in this workspace')
    // While the first read is still in flight nothing has reported yet, and that is not a fault to announce.
    expect(headerFacts({ caseCount: 4, reporting: 0, lastUpdated: null, processing: false, loading: true, now: at })[0].value).toBe('4 cases in this workspace')
  })
})

describe('D2 never turns missing into zero', () => {
  it('shows an em dash and "Status unavailable" when no case could report', () => {
    const { container } = render(<KpiTiles kpis={{ ...kpis, reported: 0, unreported: 2, review: 0, ready: 0, sources: 0, processing: 0, acceptedRows: 0 }} loading={false} />)
    for (const value of container.querySelectorAll('.dash-kpi strong')) expect(value.textContent).toBe('—')
    expect(container.textContent).toContain('Status unavailable')
    expect(container.textContent).not.toMatch(/\b0\b/)
    expect(screen.queryByRole('img')).toBeNull()
  })

  it('says the totals cover only the cases that reported', () => {
    const { container } = render(<KpiTiles kpis={{ ...kpis, cases: 4, reported: 3, unreported: 1 }} loading={false} />)
    expect(container.textContent).toContain('Across 3 of 4 cases')
    expect(kpiTiles(kpis).every(tile => !tile.scope)).toBe(true)
  })

  it('says "No sources added yet" instead of drawing an empty bar when there is nothing to measure', () => {
    const { container } = render(<KpiTiles kpis={{ ...kpis, review: 0, ready: 0, sources: 0, processing: 0, failed: 0, gaps: 0, acceptedRows: 0 }} loading={false} />)
    expect(container.textContent).toContain('No sources added yet')
    expect(screen.queryByRole('img')).toBeNull()
  })
})
