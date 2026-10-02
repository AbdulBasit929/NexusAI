import { describe, expect, it } from 'vitest'
import { againstTypical, briefing, dayBreakdown, daysCsv, familyScope, filterActivity, peakDays, typicalDay, weekdayRhythm } from './timelineModel.js'

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

  it('narrows to chosen families and keeps every total consistent', () => {
    const full = { available: true, first: '2026-01-30', last: '2026-02-02', total: 42, families: [{ id: 'cdr', total: 36 }, { id: 'anpr', total: 6 }], days: activity.days }
    const only = filterActivity(full, ['anpr'])
    expect(only.days.map(day => [day.date, day.total])).toEqual([['2026-01-30', 4], ['2026-02-02', 2]])
    expect(only.total).toBe(6)
    expect(only.families.map(family => family.id)).toEqual(['anpr'])
    expect(filterActivity(full, [])).toBe(full)
  })

  it('reads the weekly rhythm Monday first from real days', () => {
    const rhythm = weekdayRhythm({ days: [{ date: '2026-02-02', total: 30 }, { date: '2026-02-03', total: 10 }] })
    expect(rhythm.map(slot => slot.label)).toEqual(['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'])
    expect(rhythm[0]).toMatchObject({ total: 30, days: 1, share: 75 })
    expect(rhythm[1].share).toBe(25)
  })

  it('writes a briefing and a CSV of the days', () => {
    const text = briefing({ days: activity.days, families: id => id.toUpperCase(), caseId: 'c1' })
    expect(text).toContain('Timeline briefing for c1')
    expect(text).toContain('42 events over 3 periods')
    expect(text).toContain('Busiest: 2026-02-01 with 30 events.')
    expect(daysCsv(activity.days, ['cdr', 'anpr'], id => (id === 'cdr' ? 'Calls, all' : id))).toBe('Date,Total,"Calls, all",anpr\n2026-01-30,10,6,4\n2026-02-01,30,30,0\n2026-02-02,2,0,2')
  })
})
