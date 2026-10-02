import { describe, expect, it } from 'vitest'
import { mergeActivity, parseActivity } from './caseActivity.js'
import { timelineInsights, windowSummary } from './timelineInsights.js'

const build = rows => mergeActivity([parseActivity({ records: { activity_by_day: rows } }, 'c')])
const day = (date, record_type, event_count) => ({ activity_date: date, record_type, event_count })

describe('timeline insights', () => {
  it('names a spike, a quiet stretch and a leading family, with the reasoning', () => {
    const activity = build([day('2026-01-01', 'cdr', 10), day('2026-01-02', 'cdr', 10), day('2026-01-03', 'cdr', 300), day('2026-01-20', 'cdr', 10), day('2026-01-21', 'anpr', 5)])
    const found = timelineInsights(activity)
    expect(found.map(item => item.id)).toEqual(['spike', 'gap', 'mix'])
    expect(found[0].detail).toContain('2026-01-03 has 300 events')
    expect(found[1].detail).toBe('No dated records for 16 days, 2026-01-04 to 2026-01-19.')
    expect(found[2].detail).toContain('%')
  })

  it('claims nothing for an even, continuous case', () => {
    const activity = build([day('2026-01-01', 'cdr', 10), day('2026-01-02', 'anpr', 10), day('2026-01-03', 'cdr', 10), day('2026-01-04', 'anpr', 10)])
    expect(timelineInsights(activity)).toEqual([])
    expect(timelineInsights({ available: false })).toEqual([])
  })

  it('sums the days inside a window by family', () => {
    const activity = build([day('2026-01-01', 'cdr', 10), day('2026-01-02', 'cdr', 5), day('2026-01-02', 'anpr', 2), day('2026-01-05', 'cdr', 9)])
    const sum = windowSummary(activity, Date.parse('2026-01-02T00:00:00Z'), Date.parse('2026-01-04T00:00:00Z'))
    expect(sum).toMatchObject({ total: 7, days: 1, first: '2026-01-02', byFamily: { cdr: 5, anpr: 2 } })
  })
})
