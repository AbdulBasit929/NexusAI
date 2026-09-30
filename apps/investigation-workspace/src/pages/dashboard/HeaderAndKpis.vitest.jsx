import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, render, renderHook, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { DashboardHeader, ageText, briefing, freshnessText, isStale, updatedText } from './DashboardHeader.jsx'
import { KpiTiles, kpiTiles, useCountUp } from './KpiTiles.jsx'

const kpis = { cases: 2, reported: 2, unreported: 0, sources: 55, ready: 53, processing: 0, failed: 2, gaps: 1, review: 3, reviewCases: 1, acceptedRows: 23080 }
const show = ui => render(<MemoryRouter>{ui}</MemoryRouter>)
const numbers = container => [...container.querySelectorAll('.dash-kpi__num')].map(node => node.textContent)

afterEach(() => vi.unstubAllGlobals())

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

  it('keeps freshness to a few words, warns in words when stale, and separates "reading" from "no successful read"', () => {
    expect(freshnessText({ lastUpdated: at, loading: false, now: new Date('2026-09-30T16:46:00Z') })).toBe('Updated 1 min ago')
    expect(freshnessText({ lastUpdated: at, loading: false, now: new Date('2026-09-30T17:15:00Z') })).toBe('Updated 30 min ago · out of date')
    expect(freshnessText({ lastUpdated: null, loading: true, now: at })).toBe('Reading…')
    expect(freshnessText({ lastUpdated: null, loading: false, now: at })).toBe('No successful read yet')
  })

  it('leads with what needs the analyst in a few words and only states what the data says', () => {
    const attention = briefing(kpis, { loading: false })
    expect(attention.tone).toBe('attention')
    expect(attention.sentences).toEqual(['3 sources need review', '53 of 55 ready'])
    expect(attention.action).toEqual({ label: 'Review', href: '#dashboard-attention' })
    expect(briefing({ ...kpis, review: 1 }, { loading: false }).sentences[0]).toBe('1 source needs review')
  })

  it('reads well when nothing needs review, when work is in flight, and when there is no evidence', () => {
    const clear = { ...kpis, review: 0, failed: 0, gaps: 0, reviewCases: 0, ready: 55 }
    expect(briefing(clear, { loading: false })).toMatchObject({ tone: 'ok', sentences: ['All 55 sources ready'], action: null })
    expect(briefing({ ...clear, ready: 50, processing: 5 }, { loading: false })).toMatchObject({ tone: 'processing', sentences: ['5 sources processing', '50 of 55 ready'] })
    expect(briefing({ ...clear, sources: 0, ready: 0 }, { loading: false }).sentences).toEqual(['No evidence yet'])
  })

  it('never claims a state it does not know: loading, unreadable and partial coverage are stated plainly', () => {
    expect(briefing(kpis, { loading: true })).toMatchObject({ tone: 'neutral', sentences: ['Reading status…'] })
    expect(briefing({ ...kpis, reported: 0, unreported: 2 }, { loading: false })).toMatchObject({ tone: 'caution', sentences: ['Status unavailable'] })
    expect(briefing({ ...kpis, cases: 4, reported: 3, unreported: 1 }, { loading: false }).sentences.at(-1)).toBe('3 of 4 cases reporting')
  })

  it('has one h1, one short line, a compact Refresh that reports progress, and Add evidence', async () => {
    const onRefresh = vi.fn()
    const { rerender } = show(<DashboardHeader kpis={kpis} loading={false} lastUpdated={new Date()} processing={false} refreshing={false} onRefresh={onRefresh} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Dashboard' })).toBeTruthy()
    expect(screen.getByText('3 sources need review')).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Review' }).getAttribute('href')).toBe('#dashboard-attention')
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
    expect(screen.getByText('Refreshing every 15 s')).toBeTruthy()
  })
})

describe('D2 metric cards', () => {
  it('orders four cards by priority, marks the first as the hero, and links each to its section', () => {
    const tiles = kpiTiles(kpis)
    expect(tiles.map(tile => tile.label)).toEqual(['Needs review', 'Ready', 'Processing', 'Structured rows'])
    expect(tiles.map(tile => tile.href)).toEqual(['#dashboard-attention', '#dashboard-cases', '#dashboard-cases', '#dashboard-families'])
    expect(tiles.filter(tile => tile.hero).map(tile => tile.id)).toEqual(['review'])
    expect(tiles[0]).toMatchObject({ value: 3, line: '2 failed · 1 missing copy', tone: 'failed' })
    expect(tiles[1]).toMatchObject({ value: 53, valueSuffix: 'of 55', percent: '96%', tone: 'ready', bar: { total: 55, ready: 53, processing: 0, failed: 2 } })
  })

  it('gives each metric its own tone, and lets tone follow the data', () => {
    expect(kpiTiles({ ...kpis, review: 0, failed: 0, gaps: 0 })[0]).toMatchObject({ tone: 'ready', line: 'All clear' })
    expect(kpiTiles(kpis)[2]).toMatchObject({ tone: 'quiet', line: 'Idle', live: false })
    expect(kpiTiles({ ...kpis, processing: 2 })[2]).toMatchObject({ tone: 'processing', line: 'In progress', live: true })
    expect(kpiTiles(kpis)[3].tone).toBe('data')
  })

  it('floors the ready percentage so a real failure is never rounded away', () => {
    expect(kpiTiles({ ...kpis, sources: 1000, ready: 999, failed: 1 })[1].percent).toBe('99%')
    expect(kpiTiles({ ...kpis, sources: 55, ready: 55, failed: 0 })[1].percent).toBe('100%')
  })

  it('renders four separate cards with little text: a label, a figure and at most one short line', () => {
    const { container } = show(<KpiTiles kpis={kpis} loading={false} animate={false} />)
    const list = screen.getByRole('list', { name: 'Workspace totals' })
    expect(within(list).getAllByRole('listitem')).toHaveLength(4)
    expect(numbers(container)).toEqual(['3', '53', '0', '23,080'])
    expect(container.textContent).toContain('2 failed · 1 missing copy')
    expect(container.textContent).toContain('of 55')
    expect(container.textContent).toContain('96%')
    expect(container.querySelectorAll('.dash-kpi small')).toHaveLength(3)
    expect(screen.getByRole('img', { name: 'Evidence readiness across all cases: ready 53, processing 0, failed 2, other 0' })).toBeTruthy()
    // Long definitions live in the tooltip, not as body copy.
    expect(screen.getByRole('link', { name: /Needs review/ }).getAttribute('title')).toBe('2 failed sources and 1 completed jobs missing their retained copy')
  })

  it('exposes the final figure to assistive technology while the animated digits are hidden from it', () => {
    const { container } = show(<KpiTiles kpis={kpis} loading={false} animate={false} />)
    for (const digits of container.querySelectorAll('.dash-kpi__num')) expect(digits.getAttribute('aria-hidden')).toBe('true')
    expect(screen.getByRole('link', { name: /Needs review.*3/ })).toBeTruthy()
    expect(screen.getByRole('link', { name: /Structured rows.*23,080/ })).toBeTruthy()
  })

  it('shows an ellipsis while unknown, an em dash when nothing reported, and never a measured zero', () => {
    const { container, rerender } = show(<KpiTiles kpis={{ ...kpis, review: 0, ready: 0, sources: 0 }} loading animate={false} />)
    expect(numbers(container)).toEqual(['…', '…', '…', '…'])
    expect(screen.getByRole('list', { name: 'Workspace totals' }).getAttribute('aria-busy')).toBe('true')
    rerender(<MemoryRouter><KpiTiles kpis={{ ...kpis, reported: 0, unreported: 2, review: 0, ready: 0, sources: 0, processing: 0, acceptedRows: 0 }} loading={false} animate={false} /></MemoryRouter>)
    expect(numbers(container)).toEqual(['—', '—', '—', '—'])
    expect(container.textContent).toContain('Unavailable')
    expect(screen.queryByRole('img')).toBeNull()
  })

  it('names partial coverage on each card and says "No sources yet" instead of an empty bar', () => {
    const partial = show(<KpiTiles kpis={{ ...kpis, cases: 4, reported: 3, unreported: 1 }} loading={false} animate={false} />)
    expect(partial.container.textContent).toContain('3 of 4 cases')
    partial.unmount()
    const empty = show(<KpiTiles kpis={{ ...kpis, review: 0, failed: 0, gaps: 0, ready: 0, sources: 0, processing: 0, acceptedRows: 0 }} loading={false} animate={false} />)
    expect(empty.container.textContent).toContain('No sources yet')
    expect(screen.queryByRole('img')).toBeNull()
  })

  it('lands a card click on its section: scrolls, moves focus there and flashes it once', async () => {
    const scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView
    show(<><KpiTiles kpis={kpis} loading={false} animate={false} /><div id="dashboard-attention">Section</div></>)
    await userEvent.setup().click(screen.getByRole('link', { name: /Needs review/ }))
    const target = document.getElementById('dashboard-attention')
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' })
    expect(document.activeElement).toBe(target)
    expect(target.classList.contains('dash-flash')).toBe(true)
  })

  it('jumps instantly under reduced motion', async () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    const scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView
    show(<><KpiTiles kpis={kpis} loading={false} animate={false} /><div id="dashboard-cases">Section</div></>)
    await userEvent.setup().click(screen.getByRole('link', { name: /Ready/ }))
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'auto', block: 'start' })
  })
})

describe('count-up', () => {
  function withClock() {
    let now = 0
    const frames = []
    vi.stubGlobal('requestAnimationFrame', callback => { frames.push(callback); return frames.length })
    vi.stubGlobal('cancelAnimationFrame', () => {})
    vi.spyOn(globalThis.performance, 'now').mockImplementation(() => now)
    return { advanceTo: time => { now = time; act(() => { const pending = frames.splice(0); for (const frame of pending) frame(time) }) } }
  }

  it('counts up with ease-out and lands exactly on the target', () => {
    const clock = withClock()
    const { result } = renderHook(() => useCountUp(100))
    expect(result.current).toBe(0)
    clock.advanceTo(350)
    expect(result.current).toBeGreaterThan(50)
    expect(result.current).toBeLessThan(100)
    clock.advanceTo(800)
    expect(result.current).toBe(100)
  })

  it('writes the final value at once under reduced motion or when animation is off', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    expect(renderHook(() => useCountUp(42)).result.current).toBe(42)
    vi.unstubAllGlobals()
    expect(renderHook(() => useCountUp(42, { animate: false })).result.current).toBe(42)
  })
})
