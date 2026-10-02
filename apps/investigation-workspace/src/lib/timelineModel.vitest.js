import { describe, expect, it } from 'vitest'
import { againstTypical, dayBreakdown, familyScope, groupByMonth, typicalDay, visibleDays } from './timelineModel.js'

const activity = {
  first: '2026-01-30', last: '2026-02-02',
  days: [
    { date: '2026-01-30', total: 10, byFamily: { cdr: 6, anpr: 4 } },
    { date: '2026-02-01', total: 30, byFamily: { cdr: 30 } },
    { date: '2026-02-02', total: 2, byFamily: { anpr: 2 } },
  ],
}

describe('timeline model', () => {
  it('lists days newest first and narrows by family so totals add up', () => {
    expect(visibleDays(activity).map(day => day.date)).toEqual(['2026-02-02', '2026-02-01', '2026-01-30'])
    const cdr = visibleDays(activity, { families: ['cdr'], newestFirst: false })
    expect(cdr.map(day => [day.date, day.shown])).toEqual([['2026-01-30', 6], ['2026-02-01', 30]])
  })

  it('applies the range from the last day read', () => {
    expect(visibleDays(activity, { range: '2' }).map(day => day.date)).toEqual(['2026-02-02', '2026-02-01'])
  })

  it('groups days under months with their own totals', () => {
    const groups = groupByMonth(visibleDays(activity, { newestFirst: false }))
    expect(groups.map(group => [group.label, group.total, group.days.length])).toEqual([['January 2026', 10, 1], ['February 2026', 32, 2]])
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
