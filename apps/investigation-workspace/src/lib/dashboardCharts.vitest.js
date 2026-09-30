import { describe, expect, it } from 'vitest'
import { attentionItems, evidenceLink, familyOption, familyRows, formatRate, ingestionRows, readinessRows, readinessSegments } from './dashboardCharts.js'

const theme = { text: '#000', muted: '#333', line: '#ccc', card: '#fff', ready: '#0a0', processing: '#a80', failed: '#a00', withheld: '#555', data: ['#00f', '#f00', '#0f0', '#f0f', '#ff0', '#888'] }
const summary = (over = {}) => ({ total: 43, ready: 42, inFlight: 0, failed: 1, acceptedRows: 10168, duplicateRows: 1, rejectedRows: 0, ...over })

describe('readiness by case', () => {
  it('splits a case into exclusive parts that add back to its total', () => {
    expect(readinessSegments(summary())).toEqual({ ready: 42, processing: 0, failed: 1, other: 0 })
    expect(readinessSegments(summary({ total: 12, ready: 8, inFlight: 1, failed: 1 }))).toEqual({ ready: 8, processing: 1, failed: 1, other: 2 })
  })

  it('leaves out cases with no reported status or no evidence instead of drawing zeros', () => {
    const rows = readinessRows([{ caseId: 'a', summary: summary() }, { caseId: 'b', summary: null }, { caseId: 'c', summary: summary({ total: 0, ready: 0, failed: 0 }) }])
    expect(rows.map(row => row.caseId)).toEqual(['a'])
    expect(rows[0].total).toBe(43)
  })

  it('orders cases worst first: most failed, then most processing, then by name', () => {
    const rows = readinessRows([
      { caseId: 'b', summary: summary({ failed: 0, ready: 43 }) },
      { caseId: 'a', summary: summary({ failed: 3, ready: 40 }) },
      { caseId: 'c', summary: summary({ failed: 0, ready: 40, inFlight: 3 }) },
    ])
    expect(rows.map(row => row.caseId)).toEqual(['a', 'c', 'b'])
  })

  it('links each segment to the evidence list state that shows those sources', () => {
    expect(evidenceLink('case/a', 'failed')).toBe('/cases/case%2Fa/evidence?status=failed')
    expect(evidenceLink('case/a', 'ready')).toBe('/cases/case%2Fa/evidence?status=completed')
    expect(evidenceLink('a', 'other')).toBe('/cases/a/evidence')
  })
})

describe('structured families', () => {
  it('ranks by accepted rows, keeps case coverage and labels curated names', () => {
    const items = familyRows([{ id: 'cdr', value: 30, caseCount: 2 }, { id: 'anpr', value: 10, caseCount: 1 }])
    expect(items.map(item => item.share)).toEqual([0.75, 0.25])
    const option = familyOption(items, theme)
    expect(option.series[0].data).toEqual([{ value: 30, id: 'cdr', caseCount: 2 }, { value: 10, id: 'anpr', caseCount: 1 }])
    expect(option.yAxis.data).toHaveLength(2)
  })
})

describe('ingestion accounting', () => {
  it('reports exact rows and honest rates, never rounding a real loss to zero', () => {
    const [row] = ingestionRows([{ caseId: 'a', summary: summary({ acceptedRows: 23080, duplicateRows: 302, rejectedRows: 7 }) }])
    expect(row).toMatchObject({ accepted: 23080, duplicate: 302, rejected: 7, total: 23389 })
    expect(formatRate(row.rejectedShare)).toBe('<0.1%')
    expect(formatRate(row.duplicateShare)).toBe('1.3%')
    expect(formatRate(0)).toBe('0%')
  })

  it('omits cases that ingested no rows', () => {
    expect(ingestionRows([{ caseId: 'a', summary: summary({ acceptedRows: 0, duplicateRows: 0, rejectedRows: 0 }) }])).toEqual([])
  })
})

describe('needs-attention list', () => {
  it('joins a failed source seen as evidence and as a job into one row with the reason', () => {
    const items = attentionItems([{ caseId: 'a', state: { data: {
      recent_evidence: [{ evidence_id: 'e1', source_file: 'x.csv', processing_status: 'failed' }],
      recent_jobs: [{ evidence_id: 'e1', source_file: 'x.csv', status: 'failed', error_message: 'Could not be read.' }],
      missing_kb_assets: [{ evidence_id: 'e2', source_file: 'y.pdf' }],
    } } }, { caseId: 'b', state: {} }])
    expect(items).toEqual([
      { caseId: 'a', kind: 'Failed', label: 'x.csv', evidenceId: 'e1', detail: 'Could not be read.' },
      { caseId: 'a', kind: 'Retained copy missing', label: 'y.pdf', evidenceId: 'e2', detail: '' },
    ])
  })

  it('lists failures before missing copies even when a missing copy was reported first', () => {
    const items = attentionItems([{ caseId: 'a', state: { data: { missing_kb_assets: ['gap.pdf'], recent_evidence: [{ evidence_id: 'e9', source_file: 'bad.csv', processing_status: 'failed' }] } } }])
    expect(items.map(item => item.kind)).toEqual(['Failed', 'Retained copy missing'])
  })
})
