import { describe, expect, it } from 'vitest'
import { activitiesCsv, buildActivities, countOutcomes, dayCounts, groupByDay, outcomeOf } from './activityFeed.js'

const overview = { recent_evidence: [{ evidence_id: 'e1', source_file: 'calls.csv', processing_status: 'completed', updated_at: '2026-02-02T10:00:00Z' }, { evidence_id: 'e2', source_file: 'scan.pdf', processing_status: 'failed' }], recent_jobs: [{ evidence_id: 'e1', accepted_rows: 5, rejected_rows: 1, duplicate_rows: 0, attempt_count: 2 }] }
const session = [{ id: 's1', query: 'Busiest hour', state: 'answered', title: 'Busiest hour', answer: 'Noon', scope: [], recordedAt: '2026-02-03T09:15:00Z' }]
const history = [{ id: 'h1', query: 'Old question', state: 'clarify', updatedAt: '2026-02-02T08:00:00Z' }, { id: 'h2', query: 'Busiest hour', state: 'answered', updatedAt: '2026-02-03T09:15:00Z' }]

describe('activity feed', () => {
  const items = buildActivities({ sessionActivities: session, questionHistory: history, overview, caseId: 'c1' })

  it('merges the three sources newest first and does not repeat a question already in this tab', () => {
    expect(items.map(item => item.id)).toEqual(['s1', 'evidence-e1', 'history-c1-h1', 'evidence-e2'])
    expect(items.find(item => item.id === 'evidence-e1').title).toBe('Evidence reprocessed: calls.csv')
  })

  it('groups by UTC day with undated entries last, and counts outcomes', () => {
    const groups = groupByDay(items)
    expect(groups.map(group => [group.key, group.items.length])).toEqual([['2026-02-03', 1], ['2026-02-02', 2], ['', 1]])
    expect(groups[2].label).toBe('Time not reported')
    expect(countOutcomes(items)).toEqual({ ok: 2, attention: 1, failed: 1, unavailable: 0 })
    expect(outcomeOf({ state: 'mystery' })).toBe('unavailable')
  })

  it('places dated entries on the last days and leaves undated ones off', () => {
    const counts = dayCounts(items, 3)
    expect(counts).toEqual([{ day: '2026-02-01', count: 0 }, { day: '2026-02-02', count: 2 }, { day: '2026-02-03', count: 1 }])
    expect(dayCounts([{ recordedAt: null }])).toEqual([])
  })

  it('writes CSV without withheld answers', () => {
    const csv = activitiesCsv(items)
    expect(csv.split('\n')[0]).toBe('Time (UTC),Source,Outcome,Activity,Answer')
    expect(csv).toContain('Evidence ready')
    expect(csv.split('\n').find(line => line.includes('Old question'))).toMatch(/,$/)
  })
})

import { buildWorkspaceActivities, caseSummaries, reviewedKey } from './activityFeed.js'

describe('workspace feed', () => {
  const overviews = { a: { recent_evidence: [{ evidence_id: 'e1', source_file: 'x.csv', processing_status: 'failed', updated_at: '2026-02-02T10:00:00Z' }], missing_kb_assets: [{ evidence_id: 'm1', source_file: 'gap.pdf' }, 'e1'] }, b: null }
  const items = buildWorkspaceActivities({ sessionActivities: [{ id: 's', caseId: 'b', query: 'Q', state: 'answered', title: 'Q', answer: 'A', scope: [], recordedAt: '2026-02-03T00:00:00Z' }], questionHistory: [{ id: 'h', caseId: 'b', query: 'Q', state: 'answered', updatedAt: '2026-02-03T00:00:00Z' }, { id: 'h2', caseId: 'a', query: 'Q', state: 'clarify', updatedAt: '2026-02-01T00:00:00Z' }], overviews })

  it('merges cases, keeps the same question in two cases, and adds missing copies once', () => {
    expect(items.map(item => `${item.caseId}:${item.id}`)).toEqual(['b:s', 'a:evidence-e1', 'a:history-a-h2', 'a:missing-m1'])
    expect(items.find(item => item.id === 'missing-m1')).toMatchObject({ state: 'missing', recordedAt: null })
    expect(outcomeOf(items.find(item => item.id === 'missing-m1'))).toBe('attention')
  })

  it('summarises each case and respects reviewed entries', () => {
    expect(caseSummaries(items, ['a', 'b', 'c']).map(row => [row.caseId, row.total, row.open])).toEqual([['a', 3, 3], ['b', 1, 0], ['c', 0, 0]])
    const reviewed = new Set([reviewedKey(items.find(item => item.id === 'missing-m1'))])
    expect(caseSummaries(items, ['a'], reviewed)[0].open).toBe(2)
  })
})

import { seriesByDay } from './activityFeed.js'

describe('seriesByDay', () => {
  const rows = [{ caseId: 'a', recordedAt: '2026-02-03T01:00:00Z' }, { caseId: 'a', recordedAt: '2026-02-03T05:00:00Z' }, { caseId: 'b', recordedAt: '2026-02-01T05:00:00Z' }, { caseId: 'b', recordedAt: null }, { caseId: 'z', recordedAt: '2026-02-02T05:00:00Z' }]
  it('counts per case per day ending on the latest entry, and leaves out undated or unknown ones', () => {
    const { days, series } = seriesByDay(rows, item => item.caseId, ['a', 'b'], 3)
    expect(days).toEqual(['2026-02-01', '2026-02-02', '2026-02-03'])
    expect(series).toEqual([{ key: 'a', values: [0, 0, 2], total: 2 }, { key: 'b', values: [1, 0, 0], total: 1 }])
    expect(seriesByDay([{ recordedAt: null }], () => 'a', ['a'])).toEqual({ days: [], series: [] })
  })
})
