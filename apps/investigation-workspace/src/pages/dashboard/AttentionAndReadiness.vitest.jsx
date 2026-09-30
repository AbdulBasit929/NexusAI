import { describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { AttentionCauses, attentionReason, reviewTarget, unnamedFailures } from './AttentionCauses.jsx'
import { ReadinessBars } from './ReadinessBars.jsx'
import { attentionItems, evidenceLink, itemMatchesReviewFilter, readinessRows, reviewByCase, reviewFilterCount, reviewKind } from '../../lib/dashboardCharts.js'

const show = ui => render(<MemoryRouter>{ui}</MemoryRouter>)

const statusData = {
  recent_evidence: [
    { evidence_id: 'e1', source_file: 'bad.csv', processing_status: 'failed' },
    { evidence_id: 'e2', source_file: 'fine.csv', processing_status: 'completed' },
  ],
  recent_jobs: [{ evidence_id: 'e1', source_file: 'bad.csv', status: 'failed', error_message: 'The source could not be parsed.' }],
  missing_kb_assets: [{ evidence_id: 'e3', source_file: 'gap.pdf' }],
}

const rows = [
  { caseId: 'alpha', summary: { failed: 3, missingAssets: 1 }, state: { data: statusData } },
  { caseId: 'bravo', summary: { failed: 0, missingAssets: 2 }, state: { data: { missing_kb_assets: [{ evidence_id: 'b1', source_file: 'b1.pdf' }, { evidence_id: 'b2', source_file: 'b2.pdf' }] } } },
  { caseId: 'clean', summary: { failed: 0, missingAssets: 0 }, state: { data: {} } },
  { caseId: 'unreported', summary: null, state: null },
]

describe('Needs review data', () => {
  it('ranks cases worst first from the complete summary counts and leaves out clean and unreported cases', () => {
    expect(reviewByCase(rows)).toEqual([
      { caseId: 'alpha', failed: 3, missing: 1, total: 4 },
      { caseId: 'bravo', failed: 0, missing: 2, total: 2 },
    ])
  })

  it('maps a named item to a kind, and a filter to the count it stands for', () => {
    expect(reviewKind({ kind: 'Failed' })).toBe('failed')
    expect(reviewKind({ kind: 'Retained copy missing' })).toBe('missing')
    const byCase = reviewByCase(rows)
    expect(reviewFilterCount(byCase, { caseId: null, kind: null })).toBe(6)
    expect(reviewFilterCount(byCase, { caseId: null, kind: 'failed' })).toBe(3)
    expect(reviewFilterCount(byCase, { caseId: 'bravo', kind: 'missing' })).toBe(2)
    expect(itemMatchesReviewFilter({ kind: 'Failed', caseId: 'alpha' }, { caseId: 'bravo', kind: null })).toBe(false)
    expect(itemMatchesReviewFilter({ kind: 'Failed', caseId: 'alpha' }, { caseId: 'alpha', kind: 'failed' })).toBe(true)
  })

  it('merges a failed source seen as evidence and as a job, keeps the job reason, and puts failures first', () => {
    const items = attentionItems([{ caseId: 'a', state: { data: { ...statusData, missing_kb_assets: [{ evidence_id: 'e0', source_file: 'first-gap.pdf' }] } } }])
    expect(items.map(item => `${item.kind}:${item.label}`)).toEqual(['Failed:bad.csv', 'Retained copy missing:first-gap.pdf'])
    expect(items[0].detail).toBe('The source could not be parsed.')
  })

  it('names the reason honestly when the service gave none, and links each row to the real source', () => {
    expect(attentionReason({ kind: 'Failed', detail: '' })).toBe('Did not finish processing.')
    expect(attentionReason({ kind: 'Retained copy missing', detail: '' })).toBe('Retained copy is missing.')
    expect(attentionReason({ kind: 'Failed', detail: 'Bad header' })).toBe('Bad header')
    expect(reviewTarget({ caseId: 'case/x', evidenceId: 'e 1', kind: 'Failed' })).toBe('/cases/case%2Fx/evidence/e%201')
    expect(reviewTarget({ caseId: 'a', kind: 'Failed' })).toBe('/cases/a/evidence?status=failed')
    expect(reviewTarget({ caseId: 'a', kind: 'Retained copy missing' })).toBe('/cases/a/evidence')
  })

  it('finds failed sources a case counts but does not name', () => {
    expect(unnamedFailures(reviewByCase(rows), attentionItems(rows))).toEqual([{ caseId: 'alpha', unnamed: 2, failed: 3 }])
  })
})

describe('Needs review by cause', () => {
  const byCase = reviewByCase(rows)
  const items = attentionItems(rows)
  const kpis = { cases: 4, reported: 3, sources: 20, ready: 14, failed: 3, gaps: 3, review: 6 }
  const card = () => show(<AttentionCauses items={items} byCase={byCase} kpis={kpis} loading={false} />)

  it('summarises how many distinct causes there are across how many sources', () => {
    card()
    expect(screen.getByRole('heading', { name: 'What needs review?' })).toBeTruthy()
    expect(screen.getByText('3 causes across 6 sources')).toBeTruthy()
    expect(screen.getByText('sources need review')).toBeTruthy()
    expect(document.querySelector('.tri-total').textContent).toBe('6')
    expect(screen.getByText(/3 failed · 3 missing a copy/)).toBeTruthy()
  })

  it('draws one bar segment per cause, sized by its count, and lights the matching row on hover', async () => {
    card()
    const segments = [...document.querySelectorAll('.tri-seg')]
    expect(segments.map(segment => segment.style.flexGrow)).toEqual(['3', '2', '1'])
    await userEvent.setup().hover(segments[1])
    expect(document.querySelectorAll('.tri-row.is-hot')).toHaveLength(1)
    expect(document.querySelector('.tri-row.is-hot .tri-label').textContent).toBe('Failed, cause not listed')
  })

  it('lists causes biggest first with exact counts, and opens the biggest one so its sources show at once', () => {
    card()
    const heads = [...document.querySelectorAll('.tri-head')]
    expect(heads.map(head => `${head.querySelector('.tri-label').textContent}|${head.querySelector('.tri-count').textContent}`)).toEqual(['Retained copy is missing|3', 'Failed, cause not listed|2', 'The source could not be parsed|1'])
    expect(heads[0].getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByRole('link', { name: 'Review gap.pdf' }).getAttribute('href')).toBe('/cases/alpha/evidence/e3')
    expect(screen.getByRole('link', { name: 'Review b1.pdf' })).toBeTruthy()
  })

  it('opens another cause in place and closes the first, and lets an open cause be closed', async () => {
    card()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /The source could not be parsed/ }))
    expect(screen.getByRole('link', { name: 'Review bad.csv' }).getAttribute('href')).toBe('/cases/alpha/evidence/e1')
    expect(screen.queryByRole('link', { name: 'Review gap.pdf' })).toBeNull()
    await user.click(screen.getByRole('button', { name: /The source could not be parsed/ }))
    expect(screen.queryByRole('link', { name: 'Review bad.csv' })).toBeNull()
  })

  it('says honestly when failures are counted but not named, and links to the full failed list', async () => {
    card()
    await userEvent.setup().click(screen.getByRole('button', { name: /Failed, cause not listed/ }))
    expect(screen.getByText('2 not named here.')).toBeTruthy()
    expect(screen.getByRole('link', { name: /All 3 failed in alpha/ }).getAttribute('href')).toBe('/cases/alpha/evidence?status=failed')
  })

  it('shows a positive empty state, a neutral unavailable state, and a loading skeleton', () => {
    const { rerender } = show(<AttentionCauses items={[]} byCase={[]} kpis={{ cases: 2, reported: 2, sources: 55, ready: 55 }} loading={false} />)
    expect(screen.getByText('Nothing needs review')).toBeTruthy()
    expect(screen.getByText('55 of 55 sources are ready.')).toBeTruthy()
    rerender(<MemoryRouter><AttentionCauses items={[]} byCase={[]} kpis={{ cases: 2, reported: 0, sources: 0, ready: 0 }} loading={false} /></MemoryRouter>)
    expect(screen.getByText('Status unavailable')).toBeTruthy()
    expect(screen.queryByText('Nothing needs review')).toBeNull()
    rerender(<MemoryRouter><AttentionCauses items={[]} byCase={[]} kpis={{ cases: 2, reported: 0, sources: 0, ready: 0 }} loading /></MemoryRouter>)
    expect(screen.queryByText('Nothing needs review')).toBeNull()
    expect(screen.queryByText('Status unavailable')).toBeNull()
  })
})

describe('D4 readiness by case', () => {
  const readiness = readinessRows([
    { caseId: 'clean', summary: { total: 43, ready: 43, inFlight: 0, failed: 0 } },
    { caseId: 'messy', summary: { total: 12, ready: 8, inFlight: 1, failed: 2 } },
    { caseId: 'empty', summary: { total: 0, ready: 0, inFlight: 0, failed: 0 } },
    { caseId: 'unreported', summary: null },
  ])
  const kpis = { ready: 51, sources: 55 }

  it('orders the worst case first, drops cases with no sources and never zero-shapes an unreported case', () => {
    expect(readiness.map(row => row.caseId)).toEqual(['messy', 'clean'])
    expect(readiness[0].counts).toEqual({ ready: 8, processing: 1, failed: 2, other: 1 })
  })

  it('makes every segment a real link with an exact accessible count, and prints counts only in wide segments', () => {
    show(<ReadinessBars rows={readiness} kpis={kpis} loading={false} />)
    const messy = screen.getByRole('group', { name: 'Evidence readiness of messy' })
    const ready = within(messy).getByRole('link', { name: /8 ready of 12 sources in messy/ })
    expect(ready.getAttribute('href')).toBe(evidenceLink('messy', 'ready'))
    expect(ready.getAttribute('href')).toBe('/cases/messy/evidence?status=completed')
    expect(within(messy).getByRole('link', { name: /2 failed of 12/ }).getAttribute('href')).toBe('/cases/messy/evidence?status=failed')
    expect(within(messy).getByRole('link', { name: /1 processing of 12/ }).textContent).toBe('')
    expect(screen.getByText('51 of 55 sources are ready across 2 cases.')).toBeTruthy()
  })

  it('offers the same numbers as a table with links, and honest empty and loading states', async () => {
    const { rerender } = show(<ReadinessBars rows={readiness} kpis={kpis} loading={false} />)
    await userEvent.setup().click(screen.getByRole('button', { name: /Table/ }))
    const table = screen.getByRole('table')
    expect(within(table).getByRole('link', { name: '43' }).getAttribute('href')).toBe('/cases/clean/evidence?status=completed')
    expect(within(table).getAllByText('0', { selector: 'td' })).toHaveLength(2)
    rerender(<MemoryRouter><ReadinessBars rows={[]} kpis={kpis} loading={false} /></MemoryRouter>)
    expect(screen.getByText('No case has reported evidence yet.')).toBeTruthy()
  })
})
