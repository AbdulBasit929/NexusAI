import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { ActivityChart, dayQuestion, dayTarget } from './ActivityChart.jsx'
import { EvidenceMap } from './EvidenceMap.jsx'
import { NeedsReview } from './NeedsReview.jsx'
import { mergeActivity, parseActivity } from '../../lib/caseActivity.js'
import { attentionItems, familyRows, reviewByCase } from '../../lib/dashboardCharts.js'

// ECharts needs a real layout engine, so the chart frame is stood in for here; its data mapping is tested in lib.
vi.mock('../../components/charts/ChartCard.jsx', () => ({
  ChartCard: ({ title, actions, columns, rows, footer }) => (
    <section aria-label={title}>
      <h2>{title}</h2>
      {actions}
      <table><thead><tr>{columns.map(column => <th key={column.key}>{column.label}</th>)}</tr></thead><tbody>{rows.map(row => <tr key={row.key}>{columns.map(column => <td key={column.key}>{column.render ? column.render(row) : row[column.key]}</td>)}</tr>)}</tbody></table>
      <footer>{footer}</footer>
    </section>
  ),
}))

const show = ui => render(<MemoryRouter>{ui}</MemoryRouter>)
const row = (date, record_type, event_count) => ({ activity_date: `${date}T00:00:00Z`, record_type, event_count })
const parsed = (rows, caseId = 'alpha') => parseActivity({ records: { activity_by_day: rows } }, caseId)

describe('Evidence map', () => {
  const families = familyRows([{ id: 'cdr', value: 22000, caseCount: 2 }, { id: 'ipdr', value: 3400, caseCount: 1 }, { id: 'anpr', value: 40, caseCount: 1 }])
  const order = ['anpr', 'cdr', 'ipdr']
  const kpis = { acceptedRows: 25440 }

  it('draws one bubble per family with an exact name, count and share, and a legend that says the same', () => {
    show(<EvidenceMap families={families} order={order} selectedId="" onSelect={() => {}} kpis={kpis} loading={false} />)
    const map = screen.getByRole('group', { name: 'Record families sized by accepted rows' })
    expect(within(map).getAllByRole('button')).toHaveLength(3)
    expect(within(map).getByRole('button', { name: /22,000 accepted rows, 86%/ })).toBeTruthy()
    expect(within(screen.getByRole('list', { name: 'Record families' })).getAllByRole('button')).toHaveLength(3)
    expect(screen.getByText(/25,440 accepted rows/)).toBeTruthy()
  })

  it('selects a family on click, clears it on a second click, and marks the selection', async () => {
    const onSelect = vi.fn()
    const { rerender } = show(<EvidenceMap families={families} order={order} selectedId="" onSelect={onSelect} kpis={kpis} loading={false} />)
    const user = userEvent.setup()
    await user.click(within(screen.getByRole('group', { name: 'Record families sized by accepted rows' })).getByRole('button', { name: /22,000/ }))
    expect(onSelect).toHaveBeenLastCalledWith('cdr')
    rerender(<MemoryRouter><EvidenceMap families={families} order={order} selectedId="cdr" onSelect={onSelect} kpis={kpis} loading={false} /></MemoryRouter>)
    const selected = within(screen.getByRole('group', { name: 'Record families sized by accepted rows' })).getByRole('button', { name: /22,000/ })
    expect(selected.getAttribute('aria-pressed')).toBe('true')
    await user.click(selected)
    expect(onSelect).toHaveBeenLastCalledWith('')
  })

  it('offers the same numbers as a table with a working filter, and honest empty and loading states', async () => {
    const onSelect = vi.fn()
    const { rerender } = show(<EvidenceMap families={families} order={order} selectedId="" onSelect={onSelect} kpis={kpis} loading={false} />)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /Table/ }))
    const table = screen.getByRole('table')
    expect(within(table).getByText('22,000')).toBeTruthy()
    await user.click(within(table).getByRole('button', { name: 'Show only cases with Internet session records' }))
    expect(onSelect).toHaveBeenLastCalledWith('ipdr')
    rerender(<MemoryRouter><EvidenceMap families={[]} order={[]} selectedId="" onSelect={onSelect} kpis={{ acceptedRows: 0 }} loading={false} /></MemoryRouter>)
    expect(screen.getByText('No structured records have been accepted yet.')).toBeTruthy()
  })
})

describe('Activity chart', () => {
  const failed = []
  const cases = ['alpha', 'bravo']
  const props = { order: ['cdr'], cases, scope: '', onScope: () => {}, onRetry: () => {} }

  it('sends a click on a day to Investigate, in the case that holds most of it', () => {
    const activity = mergeActivity([parsed([row('2026-03-01', 'cdr', 3)], 'alpha'), parsed([row('2026-03-01', 'cdr', 9)], 'bravo')])
    expect(dayTarget(activity, '2026-03-01', '')).toBe(`/cases/bravo/investigate?question=${encodeURIComponent('Show activity on 2026-03-01')}`)
    expect(dayTarget(activity, '2026-03-01', 'alpha')).toBe(`/cases/alpha/investigate?question=${encodeURIComponent('Show activity on 2026-03-01')}`)
    expect(dayTarget(activity, '2026-04-01', '')).toBeNull()
    expect(dayQuestion('2026-03-01')).toBe('Show activity on 2026-03-01')
  })

  it('states the span and total, and labels a capped read as partial', () => {
    const capped = { ...mergeActivity([parsed([row('2026-03-01', 'cdr', 3), row('2026-03-02', 'cdr', 4)])]), truncated: true }
    show(<ActivityChart activity={capped} status="ready" failed={failed} {...props} />)
    expect(screen.getByText(/7 events · 2026-03-01 to 2026-03-02/)).toBeTruthy()
    expect(screen.getByText(/Partial: the service returns at most 100 day-and-family groups/)).toBeTruthy()
    expect(screen.getByRole('table')).toBeTruthy()
  })

  it('names a case that could not be read instead of counting it as zero, and offers a retry', async () => {
    const onRetry = vi.fn()
    show(<ActivityChart activity={mergeActivity([parsed([row('2026-03-01', 'cdr', 3)])])} status="ready" failed={['bravo']} {...props} onRetry={onRetry} />)
    expect(screen.getByText(/Not read: bravo/)).toBeTruthy()
    await userEvent.setup().click(screen.getByRole('button', { name: 'Try again' }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('has a scope control only when there is more than one case', () => {
    const activity = mergeActivity([parsed([row('2026-03-01', 'cdr', 3)])])
    const { rerender } = show(<ActivityChart activity={activity} status="ready" failed={failed} {...props} />)
    expect(screen.getByText('Scope')).toBeTruthy()
    rerender(<MemoryRouter><ActivityChart activity={activity} status="ready" failed={failed} {...props} cases={['alpha']} /></MemoryRouter>)
    expect(screen.queryByText('Scope')).toBeNull()
  })

  it('shows loading, empty and unreadable as three different states', () => {
    const none = mergeActivity([])
    const { rerender } = show(<ActivityChart activity={none} status="loading" failed={[]} {...props} />)
    expect(screen.queryByText(/No dated records/)).toBeNull()
    rerender(<MemoryRouter><ActivityChart activity={none} status="ready" failed={[]} {...props} /></MemoryRouter>)
    expect(screen.getByText(/No dated records to chart/)).toBeTruthy()
    rerender(<MemoryRouter><ActivityChart activity={none} status="ready" failed={['alpha']} {...props} /></MemoryRouter>)
    expect(screen.getByText(/Activity could not be read for this scope/)).toBeTruthy()
    rerender(<MemoryRouter><ActivityChart activity={mergeActivity([parsed([])])} status="ready" failed={[]} {...props} /></MemoryRouter>)
    expect(screen.getByText('No dated records have been ingested yet.')).toBeTruthy()
  })
})

describe('Needs review rail (compact)', () => {
  const rows = [
    { caseId: 'alpha', summary: { failed: 3, missingAssets: 1 }, state: { data: { recent_evidence: Array.from({ length: 6 }, (_, index) => ({ evidence_id: `e${index}`, source_file: `f${index}.csv`, processing_status: 'failed' })), missing_kb_assets: [{ evidence_id: 'm1', source_file: 'gap.pdf' }] } } },
  ]
  const kpis = { cases: 1, reported: 1, sources: 20, ready: 14, failed: 3, gaps: 1, review: 4 }

  it('leaves out the by-case bars, shows four rows and an icon-only Review with a full name', async () => {
    show(<NeedsReview compact items={attentionItems(rows)} byCase={reviewByCase(rows)} kpis={kpis} loading={false} />)
    expect(screen.queryByRole('group', { name: /Sources to review in/ })).toBeNull()
    const links = screen.getAllByRole('link', { name: /^Review / })
    expect(links).toHaveLength(4)
    expect(links[0].getAttribute('title')).toBe('Review f0.csv')
    expect(links[0].textContent).toBe('')
    await userEvent.setup().click(screen.getByRole('button', { name: /Show all 7/ }))
    expect(screen.getAllByRole('link', { name: /^Review / })).toHaveLength(7)
  })
})
