import { describe, expect, it } from 'vitest'
import { ACTIVITY_CAP, activityHighlights, activityOption, activityRows, familyColour, familyOrder, mergeActivity, parseActivity } from './caseActivity.js'

const response = rows => ({ records: { activity_by_day: rows } })
const row = (date, record_type, event_count) => ({ activity_date: `${date}T00:00:00Z`, record_type, event_count })

describe('parseActivity', () => {
  it('turns day and family rows into ordered days, family totals and a span', () => {
    const activity = parseActivity(response([row('2026-03-02', 'cdr', 5), row('2026-03-01', 'cdr', 3), row('2026-03-01', 'anpr', 2)]), 'alpha')
    expect(activity.available).toBe(true)
    expect(activity.days.map(day => day.date)).toEqual(['2026-03-01', '2026-03-02'])
    expect(activity.days[0]).toMatchObject({ total: 5, byFamily: { cdr: 3, anpr: 2 } })
    expect(activity.families).toEqual([{ id: 'cdr', total: 8 }, { id: 'anpr', total: 2 }])
    expect(activity).toMatchObject({ total: 10, first: '2026-03-01', last: '2026-03-02', truncated: false })
  })

  it('accepts plain dates as well as timestamps, and ignores rows it cannot trust', () => {
    const activity = parseActivity(response([{ activity_date: '2026-03-01', record_type: 'cdr', event_count: '4' }, { activity_date: null, record_type: 'cdr', event_count: 9 }, { activity_date: '2026-03-02', record_type: '', event_count: 9 }, { activity_date: '2026-03-03', record_type: 'cdr', event_count: 0 }, row('2026-03-04', 'cdr', 'x')]), 'a')
    expect(activity.days.map(day => day.date)).toEqual(['2026-03-01'])
    expect(activity.total).toBe(4)
  })

  it('reports an unexpected response as unavailable, never as zero', () => {
    expect(parseActivity({}, 'a').available).toBe(false)
    expect(parseActivity(null, 'a').available).toBe(false)
    expect(parseActivity({ records: { activity_by_day: 'nope' } }, 'a').available).toBe(false)
  })

  it('treats a full page as possibly partial, and an empty list as a real empty case', () => {
    const full = Array.from({ length: ACTIVITY_CAP }, (_, index) => row(`2026-01-${String((index % 28) + 1).padStart(2, '0')}`, `f${index}`, 1))
    expect(parseActivity(response(full), 'a').truncated).toBe(true)
    const empty = parseActivity(response([]), 'a')
    expect(empty).toMatchObject({ available: true, total: 0, days: [], truncated: false, first: null })
  })
})

describe('mergeActivity', () => {
  const alpha = parseActivity(response([row('2026-03-01', 'cdr', 5), row('2026-03-02', 'cdr', 1)]), 'alpha')
  const bravo = parseActivity(response([row('2026-03-01', 'anpr', 7), row('2026-03-03', 'cdr', 2)]), 'bravo')

  it('sums days across cases and remembers which case holds most of each day', () => {
    const merged = mergeActivity([alpha, bravo])
    expect(merged.days.map(day => [day.date, day.total, day.topCase])).toEqual([['2026-03-01', 12, 'bravo'], ['2026-03-02', 1, 'alpha'], ['2026-03-03', 2, 'bravo']])
    expect(merged.families).toEqual([{ id: 'cdr', total: 8 }, { id: 'anpr', total: 7 }])
    expect(merged).toMatchObject({ total: 15, first: '2026-03-01', last: '2026-03-03', cases: ['alpha', 'bravo'], truncated: false })
  })

  it('leaves out a case that could not be read and is partial if any read case was capped', () => {
    expect(mergeActivity([alpha, { available: false, caseId: 'down' }]).cases).toEqual(['alpha'])
    expect(mergeActivity([{ available: false }]).available).toBe(false)
    expect(mergeActivity([{ ...alpha, truncated: true }, bravo]).truncated).toBe(true)
  })
})

describe('colour and table helpers', () => {
  it('keeps one colour per record family from the sorted list of everything in view', () => {
    const order = familyOrder(['cdr', 'anpr'], ['ipdr', 'anpr'])
    expect(order).toEqual(['anpr', 'cdr', 'ipdr'])
    const palette = ['#1', '#2', '#3']
    expect(familyColour('cdr', order, palette)).toBe('#2')
    expect(familyColour('cdr', familyOrder(['cdr', 'anpr', 'ipdr']), palette)).toBe('#2')
  })

  it('offers every day as an exact table row', () => {
    const activity = parseActivity(response([row('2026-03-01', 'cdr', 3), row('2026-03-01', 'anpr', 2)]), 'a')
    expect(activityRows(activity)).toEqual([{ key: '2026-03-01', date: '2026-03-01', total: 5, cdr: 3, anpr: 2 }])
  })
})

describe('activityHighlights', () => {
  it('names the busiest day, the most active family and a typical day from the same rows as the chart', () => {
    const activity = parseActivity(response([row('2026-03-01', 'cdr', 30), row('2026-03-02', 'cdr', 90), row('2026-03-02', 'anpr', 10), row('2026-03-03', 'cdr', 20)]), 'a')
    expect(activityHighlights(activity)).toEqual({ busiest: { date: '2026-03-02', total: 100 }, topFamily: { id: 'cdr', total: 140, share: 93 }, typical: 50, activeDays: 3 })
  })

  it('floors the share so a family is never rounded up to everything, and returns nothing for no activity', () => {
    const activity = parseActivity(response([row('2026-03-01', 'cdr', 999), row('2026-03-01', 'anpr', 1)]), 'a')
    expect(activityHighlights(activity).topFamily.share).toBe(99)
    expect(activityHighlights(parseActivity(response([]), 'a'))).toBeNull()
  })
})

describe('activityOption', () => {
  const theme = { text: '#111', muted: '#444', line: '#ccc', card: '#fff', data: ['#a', '#b', '#c'] }
  it('stacks one bar series per family in the shared colours, with an interactive legend', () => {
    const activity = parseActivity(response([row('2026-03-01', 'cdr', 3), row('2026-03-01', 'anpr', 2), row('2026-03-02', 'cdr', 1)]), 'a')
    const option = activityOption(activity, theme, familyOrder(activity.families.map(family => family.id)))
    expect(option.series).toHaveLength(2)
    expect(option.series.every(series => series.type === 'bar' && series.stack === 'activity')).toBe(true)
    expect(option.series.find(series => series.itemStyle.color === '#b').data).toEqual([3, 1])
    expect(option.xAxis.data).toEqual(['2026-03-01', '2026-03-02'])
    expect(option.legend).toBeTruthy()
    expect(option.dataZoom).toEqual([])
  })

  it('adds a zoom slider that opens on the recent stretch when there are many days', () => {
    const rows = Array.from({ length: 120 }, (_, index) => row(new Date(Date.UTC(2026, 0, 1 + index)).toISOString().slice(0, 10), 'cdr', 1))
    const activity = parseActivity(response(rows.slice(0, 99)), 'a')
    const many = { ...activity, days: Array.from({ length: 120 }, (_, index) => ({ date: rows[index].activity_date.slice(0, 10), total: 1, byFamily: { cdr: 1 }, byCase: { a: 1 } })) }
    const option = activityOption(many, theme, ['cdr'])
    expect(option.dataZoom).toHaveLength(2)
    expect(option.dataZoom[0].start).toBeGreaterThan(0)
    expect(option.dataZoom[0].end).toBe(100)
  })

  it('says exact counts and offers the drill in the tooltip', () => {
    const activity = mergeActivity([parseActivity(response([row('2026-03-01', 'cdr', 3)]), 'alpha'), parseActivity(response([row('2026-03-01', 'cdr', 9)]), 'bravo')])
    const option = activityOption(activity, theme, ['cdr'])
    const html = option.tooltip.formatter([{ value: 12, marker: '', seriesName: 'Call records', axisValueLabel: '2026-03-01', dataIndex: 0 }])
    expect(html).toContain('Total: <strong>12</strong>')
    expect(html).toContain('Most in bravo')
    expect(html).toContain('Select to open this day.')
  })
})
