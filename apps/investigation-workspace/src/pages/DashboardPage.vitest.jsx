import { describe, expect, it } from 'vitest'
import { aggregateFamilyRows, curatedQuestions, dashboardKpis, dashboardNextAction, dashboardOrientation, dashboardRowState } from './DashboardPage.jsx'

describe('Dashboard decision presentation', () => {
  it('orders service failures ahead of stale retained data', () => {
    expect(dashboardRowState({ data: { summary: { evidence_total: 2, evidence_completed: 2 } }, error: new Error('offline') })).toBe('unavailable')
  })

  it('keeps no evidence distinct from active processing', () => {
    expect(dashboardRowState({ data: { summary: { evidence_total: 0, evidence_in_flight: 0 } } })).toBe('not-processed')
    expect(dashboardRowState({ data: { summary: { evidence_total: 2, evidence_in_flight: 2 } } })).toBe('processing')
  })

  it('routes each readiness state to a real next action', () => {
    expect(dashboardNextAction({ processingState: 'attention' }, 'case/a')).toEqual({ label: 'Review evidence', to: '/cases/case%2Fa/evidence' })
    expect(dashboardNextAction({ processingState: 'complete' }, 'case/a')).toEqual({ label: 'Start investigating', to: '/cases/case%2Fa/investigate' })
  })

  it('describes evidence accounting without inventing percentages', () => {
    expect(dashboardOrientation({ failed: 1, missingAssets: 0, inFlight: 0, total: 2, ready: 1 })).toBe('1 evidence item failed. Review before relying on complete coverage.')
    expect(dashboardOrientation({ failed: 0, missingAssets: 0, inFlight: 0, total: 4, ready: 4 })).toBe('4 of 4 evidence sources are ready for analysis.')
  })

  it('projects only analyst-safe questions from the server corpus', () => {
    const payload = {
      families: [{ id: 'cdr', availability: 'queryable' }],
      query_corpus: { entries: [
        { family_id: 'cdr', query: 'Who called most often?', suggested: true },
        { family_id: 'cdr', query: 'Run deterministic forensic query; template=canonical_records; limit=20', suggested: true },
      ] },
    }
    expect(curatedQuestions(payload)).toEqual([{ familyId: 'cdr', query: 'Who called most often?' }])
  })

  it('sums workspace figures only over cases that reported, and says how many did not', () => {
    const summary = (over) => ({ total: 43, ready: 42, inFlight: 0, failed: 1, missingAssets: 3, acceptedRows: 100, ...over })
    expect(dashboardKpis([{ summary: summary({}) }, { summary: summary({ total: 12, ready: 11, failed: 1, missingAssets: 0, acceptedRows: 50 }) }, { summary: null }])).toEqual({
      cases: 3, reported: 2, unreported: 1, sources: 55, ready: 53, processing: 0, failed: 2, gaps: 3, review: 5, acceptedRows: 150,
    })
  })

  it('aggregates structured families across cases without double-counting case coverage', () => {
    expect(aggregateFamilyRows([
      { summary: { families: [{ record_type: 'cdr', accepted_rows: 10 }, { record_type: 'cdr', accepted_rows: 2 }] } },
      { summary: { families: [{ record_type: 'cdr', accepted_rows: 8 }, { record_type: 'anpr', accepted_rows: 3 }] } },
    ])).toEqual([
      { id: 'cdr', value: 20, caseCount: 2 },
      { id: 'anpr', value: 3, caseCount: 1 },
    ])
  })
})
