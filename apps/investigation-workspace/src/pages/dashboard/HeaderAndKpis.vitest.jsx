import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { DashboardHeader, ageText, briefing, freshnessText, isStale, updatedText } from './DashboardHeader.jsx'
import { KpiTiles, kpiTiles } from './KpiTiles.jsx'

const kpis = { cases: 2, reported: 2, unreported: 0, sources: 55, ready: 53, processing: 0, failed: 2, gaps: 1, review: 3, reviewCases: 1, acceptedRows: 23080 }
const show = ui => render(<MemoryRouter>{ui}</MemoryRouter>)

describe('D1 briefing header', () => {
  const at = new Date('2026-09-30T16:45:00Z')

  it('says how old the figures are in words, floored so age is never understated', () => {
    expect(ageText(at, new Date('2026-09-30T16:45:20Z'))).toBe('just now')
    expect(ageText(at, new Date('2026-09-30T16:48:30Z'))).toBe('3 min ago')
    expect(ageText(at, new Date('2026-09-30T19:46:00Z'))).toBe('3 h ago')
    expect(ageText(at, new Date('2026-10-02T16:45:00Z'))).toBe('over a day ago')
    expect(ageText(null, at)).toBe('')
    expect(updatedText(null)).toBe('Reading…')
  })

  it('treats figures older than five minutes as stale, never before', () => {
    expect(isStale(at, new Date('2026-09-30T16:49:59Z'))).toBe(false)
    expect(isStale(at, new Date('2026-09-30T16:50:01Z'))).toBe(true)
    expect(isStale(null, at)).toBe(false)
  })

  it('pairs time and age, warns in words when stale, and separates "reading" from "no successful read"', () => {
    expect(freshnessText({ lastUpdated: at, loading: false, now: new Date('2026-09-30T16:46:00Z') })).toBe('Updated 16:45 · 1 min ago')
    expect(freshnessText({ lastUpdated: at, loading: false, now: new Date('2026-09-30T17:15:00Z') })).toBe('Updated 16:45 · 30 min ago, may be out of date')
    expect(freshnessText({ lastUpdated: null, loading: true, now: at })).toBe('Reading…')
    expect(freshnessText({ lastUpdated: null, loading: false, now: at })).toBe('No successful read yet')
  })

  it('leads with what needs the analyst, with units, and only states what the data says', () => {
    const attention = briefing(kpis, { loading: false })
    expect(attention.tone).toBe('attention')
    expect(attention.sentences).toEqual(['3 sources need review in 1 of 2 cases.', '53 of 55 are ready to search.'])
    expect(attention.action).toEqual({ label: 'Review them', href: '#dashboard-attention' })
    expect(briefing({ ...kpis, review: 1, gaps: 0, failed: 1, reviewCases: 1 }, { loading: false }).sentences[0]).toBe('1 source needs review in 1 of 2 cases.')
  })

  it('reads well when nothing needs review, when work is in flight, and when there is no evidence', () => {
    const clear = { ...kpis, review: 0, failed: 0, gaps: 0, reviewCases: 0, ready: 55 }
    expect(briefing(clear, { loading: false })).toMatchObject({ tone: 'ok', sentences: ['All 55 sources are ready to search.'], action: null })
    expect(briefing({ ...clear, ready: 50, processing: 5 }, { loading: false })).toMatchObject({ tone: 'processing', sentences: ['5 sources are still processing.', '50 of 55 are ready to search.'] })
    expect(briefing({ ...clear, sources: 0, ready: 0 }, { loading: false }).sentences).toEqual(['No evidence has been added yet.'])
  })

  it('never claims a state it does not know: loading, unreadable and partial coverage are stated plainly', () => {
    expect(briefing(kpis, { loading: true })).toMatchObject({ tone: 'neutral', sentences: ['Reading case status…'] })
    expect(briefing({ ...kpis, reported: 0, unreported: 2 }, { loading: false })).toMatchObject({ tone: 'caution', sentences: ['Case status could not be read, so no figures are shown.'] })
    const partial = briefing({ ...kpis, cases: 4, reported: 3, unreported: 1 }, { loading: false })
    expect(partial.sentences.at(-1)).toBe('Showing 3 of 4 cases; 1 could not be read.')
  })

  it('has one h1, a compact Refresh that reports progress, and Add evidence', async () => {
    const onRefresh = vi.fn()
    const { rerender } = show(<DashboardHeader kpis={kpis} loading={false} lastUpdated={new Date()} processing={false} refreshing={false} onRefresh={onRefresh} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Dashboard' })).toBeTruthy()
    expect(screen.getByText('3 sources need review in 1 of 2 cases.')).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Review them' }).getAttribute('href')).toBe('#dashboard-attention')
    expect(screen.getByRole('link', { name: 'Add evidence' }).getAttribute('href')).toBe('/cases/new')
    await userEvent.setup().click(screen.getByRole('button', { name: 'Refresh' }))
    expect(onRefresh).toHaveBeenCalledOnce()
    expect(screen.getByRole('status').textContent).toMatch(/^Case status updated at /)
    rerender(<MemoryRouter><DashboardHeader kpis={kpis} loading={false} lastUpdated={null} processing={false} refreshing onRefresh={onRefresh} /></MemoryRouter>)
    expect(screen.getByRole('button', { name: 'Refreshing…' }).hasAttribute('disabled')).toBe(true)
    expect(screen.getByRole('status').textContent).toBe('Refreshing case status')
  })

  it('mentions polling only while it is real', () => {
    show(<DashboardHeader kpis={kpis} loading={false} lastUpdated={new Date()} processing refreshing={false} onRefresh={() => {}} />)
    expect(screen.getByText('Refreshes every 15 s while evidence is processing')).toBeTruthy()
  })
})

describe('D2 summary strip', () => {
  it('orders four figures by priority, with the review breakdown, and links each to its section', () => {
    const tiles = kpiTiles(kpis)
    expect(tiles.map(tile => tile.label)).toEqual(['Sources to review', 'Ready to search', 'Processing', 'Structured rows'])
    expect(tiles.map(tile => tile.href)).toEqual(['#dashboard-attention', '#dashboard-readiness', '#dashboard-readiness', '#dashboard-families'])
    expect(tiles[0]).toMatchObject({ value: 3, detail: '2 failed · 1 missing retained copy', tone: 'attention' })
    expect(tiles[1]).toMatchObject({ value: 53, valueSuffix: 'of 55', percent: '96%', bar: { total: 55, ready: 53, processing: 0, failed: 2 } })
  })

  it('floors the ready percentage so a real failure is never rounded away', () => {
    expect(kpiTiles({ ...kpis, sources: 1000, ready: 999, failed: 1 })[1].percent).toBe('99%')
    expect(kpiTiles({ ...kpis, sources: 55, ready: 55, failed: 0 })[1].percent).toBe('100%')
  })

  it('tones the first cell only when there is something to review, and quiets Processing at zero', () => {
    expect(kpiTiles({ ...kpis, review: 0, failed: 0, gaps: 0 })[0]).toMatchObject({ tone: undefined, detail: 'Nothing to review' })
    expect(kpiTiles(kpis)[2].quiet).toBe(true)
    expect(kpiTiles({ ...kpis, processing: 2 })[2].quiet).toBe(false)
  })

  it('renders one strip, not four cards, with exact figures and a bar with a text equivalent', () => {
    const { container } = render(<KpiTiles kpis={kpis} loading={false} />)
    const list = screen.getByRole('list', { name: 'Workspace totals' })
    expect(within(list).getAllByRole('listitem')).toHaveLength(4)
    expect(container.querySelectorAll('.dash-kpi__icon')).toHaveLength(0)
    expect(container.textContent).toContain('23,080')
    expect(container.textContent).toContain('53 of 55')
    expect(container.textContent).toContain('2 failed · 1 missing retained copy')
    expect(screen.getByRole('img', { name: 'Evidence readiness across all cases: ready 53, processing 0, failed 2, other 0' })).toBeTruthy()
  })

  it('shows an ellipsis while unknown, an em dash when nothing reported, and never a measured zero', () => {
    const { container, rerender } = render(<KpiTiles kpis={{ ...kpis, review: 0, ready: 0, sources: 0 }} loading />)
    for (const value of container.querySelectorAll('.dash-kpi strong')) expect(value.textContent).toBe('…')
    expect(screen.getByRole('list', { name: 'Workspace totals' }).getAttribute('aria-busy')).toBe('true')
    rerender(<KpiTiles kpis={{ ...kpis, reported: 0, unreported: 2, review: 0, ready: 0, sources: 0, processing: 0, acceptedRows: 0 }} loading={false} />)
    for (const value of container.querySelectorAll('.dash-kpi strong')) expect(value.textContent).toBe('—')
    expect(container.textContent).toContain('Status unavailable')
    expect(screen.queryByRole('img')).toBeNull()
  })

  it('names partial coverage on each cell and says "No sources added yet" instead of an empty bar', () => {
    const partial = render(<KpiTiles kpis={{ ...kpis, cases: 4, reported: 3, unreported: 1 }} loading={false} />)
    expect(partial.container.textContent).toContain('Across 3 of 4 cases')
    partial.unmount()
    const empty = render(<KpiTiles kpis={{ ...kpis, review: 0, failed: 0, gaps: 0, ready: 0, sources: 0, processing: 0, acceptedRows: 0 }} loading={false} />)
    expect(empty.container.textContent).toContain('No sources added yet')
    expect(screen.queryByRole('img')).toBeNull()
  })
})
