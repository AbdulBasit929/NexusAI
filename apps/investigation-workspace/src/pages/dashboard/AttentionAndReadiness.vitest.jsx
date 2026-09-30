import { describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { AttentionQueue, attentionReason, caseFailures, reviewTarget } from './AttentionQueue.jsx'
import { ReadinessBars } from './ReadinessBars.jsx'
import { attentionItems, evidenceLink, readinessRows } from '../../lib/dashboardCharts.js'

const show = ui => render(<MemoryRouter>{ui}</MemoryRouter>)

const statusData = {
  recent_evidence: [
    { evidence_id: 'e1', source_file: 'bad.csv', processing_status: 'failed' },
    { evidence_id: 'e2', source_file: 'fine.csv', processing_status: 'completed' },
  ],
  recent_jobs: [{ evidence_id: 'e1', source_file: 'bad.csv', status: 'failed', error_message: 'The source could not be parsed.' }],
  missing_kb_assets: [{ evidence_id: 'e3', source_file: 'gap.pdf' }],
}

describe('D3 needs review', () => {
  it('merges a failed source seen as evidence and as a job, keeps the job reason, and puts failures first', () => {
    const items = attentionItems([
      { caseId: 'a', state: { data: { ...statusData, missing_kb_assets: [{ evidence_id: 'e0', source_file: 'first-gap.pdf' }] } } },
    ])
    expect(items.map(item => `${item.kind}:${item.label}`)).toEqual(['Failed:bad.csv', 'Retained copy missing:first-gap.pdf'])
    expect(items[0].detail).toBe('The source could not be parsed.')
  })

  it('names the reason honestly when the service gave none and links each row to the real source', () => {
    expect(attentionReason({ kind: 'Failed', detail: '' })).toBe('Processing did not complete.')
    expect(attentionReason({ kind: 'Retained copy missing', detail: '' })).toMatch(/retained copy .* is missing/)
    expect(attentionReason({ kind: 'Failed', detail: 'Bad header' })).toBe('Bad header')
    expect(reviewTarget({ caseId: 'case/x', evidenceId: 'e 1', kind: 'Failed' })).toBe('/cases/case%2Fx/evidence/e%201')
    expect(reviewTarget({ caseId: 'a', kind: 'Failed' })).toBe('/cases/a/evidence?status=failed')
    expect(reviewTarget({ caseId: 'a', kind: 'Retained copy missing' })).toBe('/cases/a/evidence')
  })

  it('lists only cases that have failures for the completeness links', () => {
    expect(caseFailures([{ caseId: 'a', summary: { failed: 2 } }, { caseId: 'b', summary: { failed: 0 } }, { caseId: 'c', summary: null }])).toEqual([{ caseId: 'a', failed: 2 }])
  })

  it('says when the named list is shorter than the reported failures and links to all of them', () => {
    const items = attentionItems([{ caseId: 'a', state: { data: statusData } }])
    show(<AttentionQueue items={items} kpis={{ failed: 5 }} failures={[{ caseId: 'a', failed: 5 }]} loading={false} />)
    expect(screen.getByText(/5 failed sources are reported in total; 1 is named here/)).toBeTruthy()
    expect(screen.getByRole('link', { name: /All 5 failed in a/ }).getAttribute('href')).toBe('/cases/a/evidence?status=failed')
    expect(screen.getByRole('link', { name: 'Review bad.csv' }).getAttribute('href')).toBe('/cases/a/evidence/e1')
  })

  it('shows a plain positive empty state, a loading skeleton, and collapses long lists', async () => {
    const { rerender } = show(<AttentionQueue items={[]} kpis={{ failed: 0 }} failures={[]} loading={false} />)
    expect(screen.getByText(/No failed sources or missing retained copies/)).toBeTruthy()
    rerender(<MemoryRouter><AttentionQueue items={[]} kpis={{ failed: 0 }} failures={[]} loading /></MemoryRouter>)
    expect(screen.queryByText(/No failed sources/)).toBeNull()
    const many = Array.from({ length: 9 }, (_, index) => ({ caseId: 'a', kind: 'Failed', label: `f${index}.csv`, evidenceId: `e${index}`, detail: '' }))
    rerender(<MemoryRouter><AttentionQueue items={many} kpis={{ failed: 9 }} failures={[]} loading={false} /></MemoryRouter>)
    expect(screen.getAllByRole('listitem')).toHaveLength(6)
    await userEvent.setup().click(screen.getByRole('button', { name: 'Show all 9' }))
    expect(screen.getAllByRole('listitem')).toHaveLength(9)
  })
})

describe('D4 readiness by case', () => {
  const rows = readinessRows([
    { caseId: 'clean', summary: { total: 43, ready: 43, inFlight: 0, failed: 0 } },
    { caseId: 'messy', summary: { total: 12, ready: 8, inFlight: 1, failed: 2 } },
    { caseId: 'empty', summary: { total: 0, ready: 0, inFlight: 0, failed: 0 } },
    { caseId: 'unreported', summary: null },
  ])
  const kpis = { ready: 51, sources: 55 }

  it('orders the worst case first, drops cases with no sources and never zero-shapes an unreported case', () => {
    expect(rows.map(row => row.caseId)).toEqual(['messy', 'clean'])
    expect(rows[0].counts).toEqual({ ready: 8, processing: 1, failed: 2, other: 1 })
  })

  it('makes every segment a real link with an exact accessible count, and prints counts only in wide segments', () => {
    show(<ReadinessBars rows={rows} kpis={kpis} loading={false} />)
    const messy = screen.getByRole('group', { name: 'Evidence readiness of messy' })
    const ready = within(messy).getByRole('link', { name: /8 ready of 12 sources in messy/ })
    expect(ready.getAttribute('href')).toBe(evidenceLink('messy', 'ready'))
    expect(ready.getAttribute('href')).toBe('/cases/messy/evidence?status=completed')
    expect(within(messy).getByRole('link', { name: /2 failed of 12/ }).getAttribute('href')).toBe('/cases/messy/evidence?status=failed')
    // 1 of 12 is under 10%, so no printed number, but the link and its label still exist.
    expect(within(messy).getByRole('link', { name: /1 processing of 12/ }).textContent).toBe('')
    expect(screen.getByText('51 of 55 sources are ready across 2 cases.')).toBeTruthy()
  })

  it('offers the same numbers as a table with links, and honest empty and loading states', async () => {
    const { rerender } = show(<ReadinessBars rows={rows} kpis={kpis} loading={false} />)
    await userEvent.setup().click(screen.getByRole('button', { name: /Table/ }))
    const table = screen.getByRole('table')
    expect(within(table).getByRole('link', { name: '43' }).getAttribute('href')).toBe('/cases/clean/evidence?status=completed')
    // A count of zero is plain text, not a link to an empty list: the clean case has zero processing and zero failed.
    expect(within(table).getAllByText('0', { selector: 'td' })).toHaveLength(2)
    rerender(<MemoryRouter><ReadinessBars rows={[]} kpis={kpis} loading={false} /></MemoryRouter>)
    expect(screen.getByText('No case has reported evidence yet.')).toBeTruthy()
  })
})
