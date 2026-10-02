import { describe, expect, it } from 'vitest'
import { againstTypical, dayBreakdown, familyScope, peakDays, typicalDay } from './timelineModel.js'

const activity = {
  first: '2026-01-30', last: '2026-02-02',
  days: [
    { date: '2026-01-30', total: 10, byFamily: { cdr: 6, anpr: 4 } },
    { date: '2026-02-01', total: 30, byFamily: { cdr: 30 } },
    { date: '2026-02-02', total: 2, byFamily: { anpr: 2 } },
  ],
}

describe('timeline model', () => {
  it('ranks the busiest days', () => {
    expect(peakDays(activity, 2).map(day => day.date)).toEqual(['2026-02-01', '2026-01-30'])
  })

  it('compares a day with the typical day in words', () => {
    expect(typicalDay(activity)).toBe(14)
    expect(againstTypical(30, 14)).toBe('2.1× a typical day')
    expect(againstTypical(2, 14)).toBe('Quieter than a typical day')
    expect(againstTypical(14, 14)).toBe('About a typical day')
    expect(againstTypical(0, 14)).toBe('')
  })

  it('breaks a day into families with floored shares, and maps known scopes', () => {
    expect(dayBreakdown(activity.days[0], ['anpr', 'cdr'])).toEqual([{ id: 'cdr', count: 6, share: 60, index: 1 }, { id: 'anpr', count: 4, share: 40, index: 0 }])
    expect(familyScope('transaction')).toBe('financial')
    expect(familyScope('mystery')).toBe('')
  })
})
