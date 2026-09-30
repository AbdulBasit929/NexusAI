import { describe, expect, it } from 'vitest'
import { activityCalendar, calendarWeeks, caseVerdict, nextSteps, readinessRing } from './caseDossier.js'
import { mergeActivity, parseActivity } from './caseActivity.js'

const summary = over => ({ total: 43, ready: 36, inFlight: 1, failed: 4, missingAssets: 2, ...over })

describe('caseVerdict', () => {
  it('leads with what must be reviewed, then what is still processing, then that the case is ready', () => {
    expect(caseVerdict(summary())).toMatchObject({ tone: 'failed', headline: '6 sources need review' })
    expect(caseVerdict(summary({ failed: 0, missingAssets: 1 })).headline).toBe('1 source needs review')
    expect(caseVerdict(summary({ failed: 0, missingAssets: 0 }))).toMatchObject({ tone: 'processing', headline: '1 source is still processing' })
    expect(caseVerdict(summary({ failed: 0, missingAssets: 0, inFlight: 0, ready: 43 }))).toEqual({ tone: 'ready', headline: 'Ready to investigate', detail: 'All 43 sources are searchable.' })
  })

  it('is honest about an empty case and one not read yet', () => {
    expect(caseVerdict(summary({ total: 0, ready: 0, failed: 0, missingAssets: 0, inFlight: 0 })).headline).toBe('No evidence yet')
    expect(caseVerdict(null).headline).toBe('Checking this case…')
  })
})

describe('nextSteps', () => {
  it('orders steps by urgency, each a real link, and always ends with a way to ask when something is ready', () => {
    const steps = nextSteps(summary(), 'case/a')
    expect(steps.map(step => step.id)).toEqual(['failed', 'missing', 'processing', 'ask'])
    expect(steps[0]).toMatchObject({ title: 'Review 4 failed sources', to: '/cases/case%2Fa/evidence?status=failed' })
    expect(steps.at(-1).to).toBe('/cases/case%2Fa/investigate')
  })

  it('offers only asking when all is well, only adding evidence when empty, and nothing before status is known', () => {
    expect(nextSteps(summary({ failed: 0, missingAssets: 0, inFlight: 0 }), 'a').map(step => step.id)).toEqual(['ask'])
    expect(nextSteps(summary({ total: 0, ready: 0, failed: 0, missingAssets: 0, inFlight: 0 }), 'a')[0]).toMatchObject({ id: 'add', to: '/cases/a/evidence#add-evidence' })
    expect(nextSteps(null, 'a')).toEqual([])
    expect(nextSteps(summary({ ready: 0 }), 'a').some(step => step.id === 'ask')).toBe(false)
  })
})

describe('readinessRing', () => {
  it('counts each state, names what is not counted yet, and floors the ready share', () => {
    const ring = readinessRing(summary({ total: 50, ready: 33, inFlight: 1, failed: 4 }))
    expect(ring.parts.map(part => [part.id, part.value])).toEqual([['ready', 33], ['processing', 1], ['failed', 4], ['other', 12]])
    expect(ring.percent).toBe(66)
    expect(readinessRing(summary({ total: 1000, ready: 999, inFlight: 0, failed: 0 })).percent).toBe(99)
    expect(readinessRing(null)).toEqual({ total: 0, parts: [], percent: null })
  })
})

describe('activityCalendar', () => {
  const row = (date, count) => ({ activity_date: `${date}T00:00:00Z`, record_type: 'cdr', event_count: count })
  const activity = mergeActivity([parseActivity({ records: { activity_by_day: [row('2026-03-02', 10), row('2026-03-04', 40), row('2026-03-08', 20)] } }, 'a')])

  it('lays days out in Monday-first weeks ending on the last active day, with levels relative to the busiest day', () => {
    const calendar = activityCalendar(activity, 1)
    expect(calendar.columns).toHaveLength(1)
    expect(calendar.start).toBe('2026-03-02')
    expect(calendar.columns[0].map(cell => cell.date)).toEqual(['2026-03-02', '2026-03-03', '2026-03-04', '2026-03-05', '2026-03-06', '2026-03-07', '2026-03-08'])
    expect(calendar.columns[0].map(cell => cell.level)).toEqual([1, 0, 4, 0, 0, 0, 2])
    expect(activityCalendar(activity, 2).columns[1].map(cell => cell.level)).toEqual([1, 0, 4, 0, 0, 0, 2])
    expect(activityCalendar(activity, 2).columns[0].every(cell => cell.level === 0)).toBe(true)
    expect(calendar).toMatchObject({ max: 40, total: 70, activeDays: 3, last: '2026-03-08', earlier: null })
  })

  it('says when there is earlier activity than the window shows, and returns nothing without activity', () => {
    const wide = mergeActivity([parseActivity({ records: { activity_by_day: [row('2025-01-06', 5), row('2026-03-04', 40)] } }, 'a')])
    expect(activityCalendar(wide, 4).earlier).toBe('2025-01-06')
    expect(activityCalendar({ available: false }, 4)).toBeNull()
    expect(activityCalendar(mergeActivity([parseActivity({ records: { activity_by_day: [] } }, 'a')]), 4)).toBeNull()
  })
})

describe('calendarWeeks', () => {
  it('fits the span of activity between 12 and 26 weeks', () => {
    expect(calendarWeeks({ first: '2026-03-01', last: '2026-03-02' })).toBe(12)
    expect(calendarWeeks({ first: '2026-01-01', last: '2026-05-01' })).toBe(20)
    expect(calendarWeeks({ first: '2024-01-01', last: '2026-05-01' })).toBe(26)
    expect(calendarWeeks(null)).toBe(12)
  })
})
