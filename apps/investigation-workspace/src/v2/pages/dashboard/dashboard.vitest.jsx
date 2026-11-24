import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import DashboardPage from './DashboardPage.jsx'
import { TimelinePanel, exceptionRows, gaugePath } from './insights.jsx'
import { resetDashboardReads } from './data.js'
import { MIN_WINDOW, alignTimelines, answerQuality, caseStateMix, caseStates, panWindow, viewWindow, windowTicks, zoomWindow, durationHistogram, caseTimelines, dailyQuestions, dataNotes, declineText, formatDuration, flowLayout, flowModel, heatShare, modalityMix, spreadLabels, needsYou, niceCeiling, qualityMap, recordKinds, setAside, setAsideCauses, timeBasis, timeToReady } from './model.js'
import { cleanRate, latestItems, resumeEntries, statusCounts } from './sections.jsx'
import { BarStrip, DotRow, Histogram, RangeStrip, SegBar } from '../../ui/micro.jsx'
import { CountUp } from '../../ui/CountUp.jsx'
import { Sparkline, dailySeries } from '../../ui/Sparkline.jsx'
import { shift } from '../../../lib/caseActivity.js'
import { resetAttentionCache } from '../../../lib/workspaceAttention.js'
import { resetCaseRecordsForTest } from '../../../lib/useCaseRecords.js'
import { clearWorkspaceStateForTests } from '../../../lib/workspaceState.js'

const rowsOf = states => states.map((status, index) => ({ caseId: `c${index}`, status, index }))

describe('statusCounts', () => {
  it('counts the states in the order that matters and leaves out the ones nobody is in', () => {
    const counts = statusCounts(rowsOf(['attention', 'attention', 'processing', 'complete', 'complete', 'complete', 'loading']))
    expect(counts.map(item => [item.id, item.count])).toEqual([['attention', 2], ['processing', 1], ['complete', 3]])
    expect(statusCounts(rowsOf(['unavailable'])).map(item => item.label)).toEqual(['not reporting'])
    expect(statusCounts([])).toEqual([])
  })
})

describe('cleanRate', () => {
  it('is the share of rows that were accepted, and nothing before any row was read', () => {
    expect(cleanRate({ acceptedRows: 90, duplicateRows: 5, rejectedRows: 5 })).toBe(0.9)
    expect(cleanRate({ acceptedRows: 0, duplicateRows: 0, rejectedRows: 0 })).toBeNull()
  })
})

describe('latestItems', () => {
  const rows = [
    { caseId: 'a', state: { data: { recent_evidence: [{ source_file: 'old.csv', evidence_id: 'e1', processing_status: 'completed', updated_at: '2026-10-01T10:00:00Z' }, { source_file: 'new.csv', evidence_id: 'e2', processing_status: 'failed', updated_at: '2026-11-02T10:00:00Z' }, { source_file: 'undated.csv', evidence_id: 'e3', processing_status: 'completed' }] } } },
    { caseId: 'b', state: { data: { recent_evidence: [{ source_file: 'old.csv', evidence_id: 'e1', processing_status: 'completed', updated_at: '2026-10-15T10:00:00Z' }], recent_jobs: [{ source_file: 'old.csv', evidence_id: 'e1', status: 'completed', completed_at: '2026-10-20T10:00:00Z' }] } } },
  ]
  it('puts the newest first, keeps one entry per file and case, and leaves out what has no date', () => {
    const items = latestItems(rows)
    expect(items.map(item => `${item.caseId}:${item.label}`)).toEqual(['a:new.csv', 'b:old.csv', 'a:old.csv'])
    expect(items.find(item => item.caseId === 'b').date.toISOString()).toBe('2026-10-20T10:00:00.000Z')
    expect(items[0].status).toBe('failed')
  })
  it('stops at the limit', () => {
    expect(latestItems(rows, 2)).toHaveLength(2)
  })
})

describe('resumeEntries', () => {
  it('takes the last question of each case, newest first, up to three', () => {
    const entries = [{ caseId: 'a', id: '1' }, { caseId: 'a', id: '2' }, { caseId: 'b', id: '3' }, { caseId: 'c', id: '4' }, { caseId: 'd', id: '5' }]
    expect(resumeEntries(entries).map(entry => entry.id)).toEqual(['1', '3', '4'])
    expect(resumeEntries([])).toEqual([])
  })
})

describe('formatDuration', () => {
  it('says seconds, then minutes with their seconds, then hours', () => {
    expect([0.4, 36, 215, 60, 3600, 3900, NaN].map(formatDuration)).toEqual(['<1 s', '36 s', '3 min 35 s', '1 min', '1 h', '1 h 5 min', '—'])
  })
})

describe('caseTimelines', () => {
  const result = (caseId, days, truncated = false) => ({ caseId, days: days.map(([date, total]) => ({ date, total, byFamily: {} })), truncated })
  it('draws the events of each month, with a gap month as a zero, and never a day dated after today', () => {
    const [line] = caseTimelines([result('a', [['2025-01-10', 5], ['2025-01-20', 1], ['2025-04-02', 3], ['2065-01-24', 7]])], '2026-11-04')
    expect(line.series.map(point => [point.from, point.total])).toEqual([['2025-01', 6], ['2025-02', 0], ['2025-03', 0], ['2025-04', 3]])
    expect(line.total).toBe(9)
    expect([line.first, line.last]).toEqual(['2025-01-10', '2025-04-02'])
    expect([line.futureTotal, line.futureLast]).toEqual([7, '2065-01-24'])
    expect(line.partial).toBe(false)
  })
  it('carries the service’s cap through as "partial" and drops a case with nothing dated', () => {
    const lines = caseTimelines([result('a', [['2025-01-10', 5]], true), result('b', [])], '2026-11-04')
    expect(lines.map(line => [line.caseId, line.partial])).toEqual([['a', true]])
  })
  it('groups a very long span into equal runs of months without changing the total', () => {
    const [line] = caseTimelines([result('a', [['1990-01-05', 3], ['2025-12-01', 4]])], '2026-11-04')
    expect(line.series.length).toBeLessThanOrEqual(96)
    expect(line.series.reduce((sum, point) => sum + point.total, 0)).toBe(7)
    expect(line.series[0].from).toBe('1990-01')
    expect(line.series.at(-1).to).toBe('2025-12')
  })
})

describe('answerQuality', () => {
  const health = (id, totals, latency, declines = []) => ({ collection_id: id, available: true, totals, latency_ms: latency, declines })
  it('adds the counts exactly, names the case whose times are shown, and merges decline reasons', () => {
    const quality = answerQuality([
      health('a', { questions: 100, answered: 90, declined: 6, abstained: 4 }, { p50: 40000, p90: 80000, max: 120000 }, [{ reason: 'the model judged that the views cannot answer the question', count: 5 }]),
      health('b', { questions: 10, answered: 10, declined: 0, abstained: 0 }, { p50: 1000, p90: 2000, max: 3000 }, [{ reason: 'the model judged that the views cannot answer the question', count: 1 }, { reason: 'something unusual happened here', count: 2 }]),
      { collection_id: 'c', available: false },
    ])
    expect([quality.questions, quality.answered, quality.declined, quality.abstained, quality.reported]).toEqual([110, 100, 6, 4, 2])
    expect(quality.rate).toBeCloseTo(100 / 110)
    expect(quality.latency).toEqual({ caseId: 'a', p50: 40, p90: 80, max: 120 })
    expect(quality.reasons.map(reason => [reason.text, reason.count])).toEqual([['The case’s records cannot answer that kind of question', 6], ['Something unusual happened here', 2]])
  })
  it('has no rate and no times when nothing was asked', () => {
    const quality = answerQuality([health('a', { questions: 0, answered: 0, declined: 0, abstained: 0 }, { p50: null, p90: null, max: null })])
    expect(quality.rate).toBeNull()
    expect(quality.latency).toBeNull()
    expect(declineText('')).toBe('No reason recorded')
  })
})

describe('modalityMix and recordKinds', () => {
  const report = (id, evidence, extra = {}) => ({ collection_id: id, available: true, evidence, ...extra })
  it('adds the files of each kind, keeps a colour slot for each kind, and folds the unknown into "Other files"', () => {
    const mix = modalityMix([
      report('a', [{ modality: 'structured_records', files: 9, failed: 0 }, { modality: 'image', files: 23, failed: 1 }, { modality: 'tabular', files: 2, failed: 0 }]),
      report('b', [{ modality: 'structured_records', files: 12, failed: 1 }, { modality: 'document', files: 4, failed: 0 }, { modality: 'tabular', files: 1, failed: 0 }]),
    ])
    expect(mix.total).toBe(51)
    expect(mix.kinds.map(kind => [kind.id, kind.files, kind.slot])).toEqual([['image', 23, 3], ['structured_records', 21, 1], ['document', 4, 2], ['other', 3, 0]])
    expect(mix.kinds.find(kind => kind.id === 'image').failed).toBe(1)
    expect(mix.kinds.find(kind => kind.id === 'structured_records').caseCount).toBe(2)
  })
  it('keeps which cases hold each kind, the largest first, with the files that did not finish', () => {
    const mix = modalityMix([
      report('a', [{ modality: 'image', files: 23, failed: 1 }, { modality: 'tabular', files: 2, failed: 0 }]),
      report('b', [{ modality: 'image', files: 30, failed: 0 }, { modality: 'image', files: 5, failed: 2 }]),
      report('c', [{ modality: 'document', files: 0, failed: 0 }]),
    ])
    expect(mix.kinds.find(kind => kind.id === 'image').byCase).toEqual([{ caseId: 'b', files: 35, failed: 2 }, { caseId: 'a', files: 23, failed: 1 }])
    expect(mix.kinds.find(kind => kind.id === 'other').byCase).toEqual([{ caseId: 'a', files: 2, failed: 0 }])
    expect(mix.kinds.find(kind => kind.id === 'document')).toBeUndefined()
  })
  it('types the records as calls, internet sessions and app usage with repeats left out, and keeps the rest as "Other records"', () => {
    const rows = [{ caseId: 'a', summary: { acceptedRows: 1000 } }, { caseId: 'b', summary: { acceptedRows: 50 } }, { caseId: 'c', summary: { acceptedRows: 7 } }]
    const kinds = recordKinds([
      report('a', [], { calls: { typed: 600, repeats: 10, effective: 590 }, sessions: { typed: 200, repeats: 20 }, usage: { typed: 50 } }),
      report('b', [], { calls: { typed: 10, repeats: 0 }, sessions: { typed: 0 }, usage: { typed: 0 } }),
    ], rows)
    expect(kinds.kinds.map(kind => [kind.id, kind.count])).toEqual([['calls', 600], ['sessions', 180], ['usage', 50], ['other', 220]])
    expect(kinds.reported).toBe(2)
  })
})

describe('dataNotes', () => {
  it('groups what the service says to read by its code, across cases, the most common first', () => {
    const notes = dataNotes([
      { collection_id: 'a', findings: [{ code: 'no_volume', text: 'x' }, { code: 'repeats', text: 'y' }] },
      { collection_id: 'b', findings: [{ code: 'no_volume', text: 'x' }, { code: 'no_volume', text: 'x' }] },
      { collection_id: 'c', available: false, findings: [{ code: 'no_volume', text: 'x' }] },
    ])
    expect(notes.map(note => [note.code, note.caseIds])).toEqual([['no_volume', ['a', 'b']], ['repeats', ['a']]])
    expect(notes[0].title).toBe('Internet records state no byte volumes')
  })
})

describe('setAside', () => {
  it('ranks the cases by the share of their rows that were not kept', () => {
    const data = setAside([
      { caseId: 'a', summary: { acceptedRows: 990, duplicateRows: 5, rejectedRows: 5 } },
      { caseId: 'b', summary: { acceptedRows: 80, duplicateRows: 0, rejectedRows: 20 } },
      { caseId: 'c', summary: { acceptedRows: 0, duplicateRows: 0, rejectedRows: 0 } },
      { caseId: 'd' },
    ])
    expect(data.items.map(item => item.caseId)).toEqual(['b', 'a'])
    expect(data.read).toBe(1100)
    expect(data.setAside).toBe(30)
    expect(data.items[0].share).toBeCloseTo(0.2)
  })
})

describe('timeToReady', () => {
  const job = (queued, completed, status = 'completed') => ({ status, queued_at: queued, completed_at: completed })
  it('measures from upload to ready for the finished files, per case and overall', () => {
    const data = timeToReady([
      { caseId: 'a', state: { data: { recent_jobs: [job('2026-10-30T09:00:00Z', '2026-10-30T09:00:10Z'), job('2026-10-30T09:00:00Z', '2026-10-30T09:00:30Z'), job('2026-10-30T09:00:00Z', null, 'dead_letter'), job('2026-10-30T09:01:00Z', '2026-10-30T09:00:00Z')] } } },
      { caseId: 'b', state: { data: { recent_jobs: [job('2026-10-30T09:00:00Z', '2026-10-30T09:03:20Z')] } } },
      { caseId: 'c', state: { data: { recent_jobs: [] } } },
    ])
    expect(data.cases.map(item => [item.caseId, item.count, item.min, item.median, item.max])).toEqual([['a', 2, 10, 20, 30], ['b', 1, 200, 200, 200]])
    expect(data.count).toBe(3)
    expect(data.median).toBe(30)
    expect(data.slowest).toBe(200)
    expect(data.waitMedian).toBe(null)
  })
  it('rounds an axis up to a clean number', () => {
    expect([0, 0.3, 1, 37, 215, 999, 1001].map(niceCeiling)).toEqual([1, 0.5, 1, 50, 500, 1000, 2000])
  })
})

describe('needsYou', () => {
  const now = Date.parse('2026-11-05T12:00:00Z')
  const row = (caseId, data) => ({ caseId, state: { data }, summary: { failed: 0, missingAssets: 0 } })
  it('sorts by decision: what blocks first, then what can wait, then what can be cleared, one primary action on every item', () => {
    const rows = [
      { ...row('a', {
        recent_evidence: [{ evidence_id: 'e1', source_file: 'bad.csv', processing_status: 'failed' }, { evidence_id: 'e2', source_file: 'slow.pdf', processing_status: 'processing', updated_at: '2026-11-05T10:00:00Z' }, { evidence_id: 'e3', source_file: 'ok.csv', processing_status: 'completed', warnings: ['record schema did not match a specialized forensic adapter'] }],
        recent_jobs: [{ job_id: 'j1', evidence_id: 'e9', source_file: 'dead.csv', status: 'dead_letter', attempt_count: 5, max_attempts: 5 }],
      }), summary: { failed: 3, missingAssets: 0 } },
    ]
    const result = needsYou({ rows, notes: [{ code: 'no_volume', title: 'Internet records state no byte volumes', caseIds: ['a', 'b'], text: 'x' }], history: [{ caseId: 'a', id: 'q1', query: 'Which one?', state: 'clarify', updatedAt: '2026-11-04T10:00:00Z' }, { caseId: 'a', id: 'q0', query: 'Old', state: 'clarify', updatedAt: '2026-09-01T10:00:00Z' }], drafts: [{ caseId: 'a', text: 'Who called' }], now })
    expect(result.groups.map(group => group.id)).toEqual(['failed', 'dead', 'stalled', 'choice', 'plain', 'notes', 'draft'])
    expect(result.attention).toBe(3 + 1 + 1 + 1)
    expect(result.groups[0].count).toBe(3)
    expect(result.groups[0].items[0]).toMatchObject({ title: 'bad.csv', action: 'Review', to: '/cases/a/evidence/e1' })
    expect(result.groups.find(group => group.id === 'dead').items[0].detail).toBe('5 of 5 tries')
    expect(result.groups.find(group => group.id === 'stalled').items[0].detail).toBe('2 h so far')
    expect(result.groups.find(group => group.id === 'choice').items).toHaveLength(1)
    expect(result.groups.find(group => group.id === 'notes').items[0].detail).toBe('2 cases')
    expect(result.groups.every(group => group.items.every(item => item.action && item.to))).toBe(true)
  })
  it('says nothing needs you when nothing does', () => {
    expect(needsYou({ rows: [row('a', { recent_evidence: [], recent_jobs: [] })] })).toEqual({ groups: [], attention: 0 })
  })
})

describe('setAsideCauses', () => {
  it('adds the rejected rows and the exact repeats of the cases, and what was kept', () => {
    const aside = setAside([
      { caseId: 'a', summary: { acceptedRows: 940, duplicateRows: 10, rejectedRows: 50 } },
      { caseId: 'b', summary: { acceptedRows: 480, duplicateRows: 20, rejectedRows: 0 } },
    ])
    expect(setAsideCauses(aside)).toEqual({ read: 1500, rejected: 50, repeats: 30, kept: 1420 })
    expect(setAsideCauses({ items: [], read: 0 })).toEqual({ read: 0, rejected: 0, repeats: 0, kept: 0 })
    expect(setAsideCauses(undefined)).toEqual({ read: 0, rejected: 0, repeats: 0, kept: 0 })
  })
})

describe('qualityMap', () => {
  const aside = setAside([
    { caseId: 'big', summary: { acceptedRows: 54000, duplicateRows: 4, rejectedRows: 1661 } },
    { caseId: 'mid', summary: { acceptedRows: 12900, duplicateRows: 301, rejectedRows: 7 } },
    { caseId: 'clean', summary: { acceptedRows: 204, duplicateRows: 0, rejectedRows: 0 } },
  ])
  it('puts a case across by its rows on a log scale and up by its share set aside, and the area of its bubble is its rows set aside', () => {
    const map = qualityMap(aside, { width: 320, height: 160 })
    const by = id => map.points.find(point => point.caseId === id)
    expect(by('big').x).toBeGreaterThan(by('mid').x)
    expect(by('mid').x).toBeGreaterThan(by('clean').x)
    expect(by('big').y).toBeLessThan(by('mid').y)
    expect(by('clean').y).toBeCloseTo(map.plot.bottom, 6)
    expect((by('mid').r / by('big').r) ** 2).toBeCloseTo(308 / 1665, 2)
    expect(by('clean').r).toBe(2.5)
  })
  it('starts the axes at a decade and at zero, and names every decade it draws', () => {
    const map = qualityMap(aside, { width: 320, height: 160 })
    expect(map.xTicks.map(tick => tick.label)).toEqual(['100', '1k', '10k', '100k'])
    expect(map.yTicks[0]).toMatchObject({ value: 0, label: '0%' })
    expect(map.yMax).toBeGreaterThanOrEqual(3.1)
  })
  it('draws nothing for no rows or a box too small to say anything', () => {
    expect(qualityMap({ items: [] }, { width: 320, height: 160 })).toBeNull()
    expect(qualityMap(aside, { width: 40, height: 160 })).toBeNull()
  })
})

describe('timeBasis, dailyQuestions and heatShare', () => {
  it('adds the rows of the typed layouts by the basis of their time, the largest first', () => {
    const basis = timeBasis([{ calls: { layouts: [{ time_basis: 'confirmed_by_analyst', rows: 900 }, { time_basis: 'explicit_offset', rows: 100 }] }, sessions: { layouts: [{ time_basis: 'confirmed_by_analyst', rows: 50 }] } }, { available: false }])
    expect(basis.parts.map(part => [part.label, part.rows])).toEqual([['Confirmed by an analyst', 950], ['Stated in the file', 100]])
    expect(basis.total).toBe(1050)
  })
  it('takes the period k days back as the window of k days minus the window of k-1 days, the newest last', () => {
    const windows = [10, 10, 14, 14, 14, 20, 21].map(questions => ({ questions, answered: questions, declined: 0, abstained: 0 }))
    const days = dailyQuestions([{ windows }, { windows: [] }])
    expect(days.map(day => day.questions)).toEqual([1, 6, 0, 0, 4, 0, 10])
    expect(days.map(day => day.ago)).toEqual([6, 5, 4, 3, 2, 1, 0])
  })
  it('spreads cell shades on a log scale: none is none, the largest is the darkest', () => {
    expect(heatShare(0, 100)).toBe(0)
    expect(heatShare(100, 100)).toBe(48)
    expect(heatShare(1, 1000000)).toBeLessThan(heatShare(1000, 1000000))
  })
})

describe('caseStates', () => {
  it('counts open, on hold and closed, and treats a case with no record as open', () => {
    expect(caseStates({ a: { status: 'closed' }, b: { status: 'on_hold' }, c: { status: 'open' } }, ['a', 'b', 'c', 'd'])).toEqual({ open: 2, on_hold: 1, closed: 1, other: 0 })
  })
})

describe('caseStateMix', () => {
  it('gives one entry for each case, those that need a look after those that are ready, and counts a case still being read with the unread', () => {
    const rows = [{ caseId: 'a', status: 'complete' }, { caseId: 'b', status: 'attention' }, { caseId: 'c', status: 'loading' }, { caseId: 'd', status: 'processing' }, { caseId: 'e', status: 'complete' }]
    expect(caseStateMix(rows).map(item => `${item.caseId}:${item.state}`)).toEqual(['a:ready', 'e:ready', 'd:processing', 'b:review', 'c:other'])
    expect(caseStateMix(undefined)).toEqual([])
  })
})

describe('the small pictures', () => {
  it('draws the parts of a whole in proportion and leaves out a part with nothing', () => {
    const { container } = render(<SegBar label="Split" parts={[{ id: 'a', value: 3, colour: 'red' }, { id: 'b', value: 0, colour: 'blue' }, { id: 'c', value: 1, colour: 'green' }]} />)
    const parts = [...container.querySelectorAll('.nx-seg__part')]
    expect(parts.map(part => part.style.flexGrow)).toEqual(['3', '1'])
    expect(screen.getByRole('img', { name: 'Split' })).toBeTruthy()
  })
  it('scales the bars to the largest, and shows a period with nothing as a gap', () => {
    const { container } = render(<BarStrip label="Months" values={[{ key: 'a', value: 10, title: 'A' }, { key: 'b', value: 5, title: 'B' }, { key: 'c', value: 0, title: 'C' }]} />)
    const bars = [...container.querySelectorAll('.nx-bars__bar')]
    expect(bars.map(bar => bar.style.getPropertyValue('--h'))).toEqual(['100%', '50%', ''])
    expect(bars[2].className).toMatch(/is-empty/)
    expect(bars[0].getAttribute('title')).toBe('A')
  })
  it('places a range on a shared axis', () => {
    const { container } = render(<RangeStrip label="Range" axisMax={100} stats={{ min: 10, p25: 20, median: 40, p75: 60, max: 80 }} />)
    const strip = container.querySelector('.nx-range')
    expect(['--min', '--max', '--p25', '--p75', '--median'].map(name => strip.style.getPropertyValue(name))).toEqual(['10%', '20%', '20%', '40%', '40%'])
  })
  it('draws the last N calendar days ending on the last one, a day with no events as a zero', () => {
    const days = [{ date: '2026-11-01', total: 3 }, { date: '2026-11-03', total: 5 }]
    expect(dailySeries(days, '2026-11-03', 4, shift)).toEqual([0, 3, 0, 5])
    expect(dailySeries(days, null, 4, shift)).toEqual([])
  })
  it('draws a line with an end dot and a name, and draws nothing from a single point', () => {
    const { container, rerender } = render(<Sparkline values={[1, 3, 2, 5]} label="Events per day" />)
    expect(screen.getByRole('img', { name: 'Events per day' })).toBeTruthy()
    expect(container.querySelectorAll('path')).toHaveLength(2)
    expect(container.querySelectorAll('circle')).toHaveLength(1)
    rerender(<Sparkline values={[4]} label="x" />)
    expect(container.querySelector('svg')).toBeNull()
  })
  it('draws an all-zero series flat on the baseline, never as a trend', () => {
    const { container } = render(<Sparkline values={[0, 0, 0, 0]} width={40} height={20} label="flat" />)
    expect(container.querySelector('.nx-spark__line').getAttribute('d')).toMatch(/^M0\.0 17\.0 L11\.7 17\.0/)
  })
})

// ---------- the page, against a service that answers each read ----------
const JOBS = [
  { status: 'completed', queued_at: '2026-10-30T09:00:00Z', started_at: '2026-10-30T09:00:30Z', completed_at: '2026-10-30T09:00:31Z', source_file: 'a.csv', evidence_id: 'j1' },
  { status: 'completed', queued_at: '2026-10-30T09:00:00Z', started_at: '2026-10-30T09:00:50Z', completed_at: '2026-10-30T09:00:51Z', source_file: 'b.csv', evidence_id: 'j2' },
]
const STATUS = {
  summary: { evidence_total: 4, evidence_completed: 2, evidence_in_flight: 1, evidence_failed: 1, accepted_rows: 120, duplicate_rows: 4, rejected_rows: 6 },
  record_families: [{ record_type: 'generic', accepted_rows: 120 }],
  recent_evidence: [{ processing_status: 'failed', source_file: 'broken.csv', evidence_id: 'e1' }],
  recent_jobs: JOBS,
}
const HEALTH = { available: true, totals: { questions: 10, answered: 9, declined: 1, abstained: 0 }, latency_ms: { p50: 40000, p90: 80000, max: 90000 }, declines: [{ reason: 'the model judged that the views cannot answer the question', count: 1 }] }
const TYPING = { available: true, calls: { typed: 50, repeats: 0 }, sessions: { typed: 20, repeats: 0 }, usage: { typed: 0 }, evidence: [{ modality: 'structured_records', files: 3, failed: 0 }, { modality: 'image', files: 1, failed: 1 }], findings: [{ code: 'no_volume', text: 'The internet records state no byte count.' }] }
const ACTIVITY = { records: { activity_by_day: [{ activity_date: '2024-01-05T00:00:00Z', event_count: 4, record_type: 'cdr' }, { activity_date: '2024-03-10T00:00:00Z', event_count: 6, record_type: 'cdr' }] } }

function serve() {
  return vi.fn(async (input, init) => {
    const path = new URL(String(input)).pathname.replace(/^\/api/, '')
    const body = path === '/collections/status' ? STATUS : path === '/analysis/engine-health' ? HEALTH : path === '/analysis/typing-report' ? TYPING : path === '/cases' ? { cases: [] } : path === '/query/hybrid' && init?.method === 'POST' ? ACTIVITY : {}
    return { ok: true, status: 200, json: async () => body, text: async () => JSON.stringify(body) }
  })
}

describe('the dashboard page', () => {
  beforeEach(() => { localStorage.clear(); clearWorkspaceStateForTests(); resetDashboardReads(); localStorage.setItem('nexusai.viewer.motion', 'reduce') })
  afterEach(() => { cleanup(); resetAttentionCache(); resetCaseRecordsForTest(); resetDashboardReads(); vi.restoreAllMocks(); delete window.__INVESTIGATION_WORKSPACE_CONFIG__ })

  it('fills the figures, the panels and the cases table from the reads, each as it arrives', async () => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a', 'case-b'] }
    globalThis.fetch = serve()
    render(<MemoryRouter initialEntries={['/']}><DashboardPage /></MemoryRouter>)

    const states = await screen.findByRole('list', { name: 'Cases by state' })
    expect(states.textContent).toMatch(/2 need review/)
    const figures = screen.getByRole('region', { name: 'Figures' })
    expect(figures.textContent).toMatch(/Cases\s*2/)
    expect(figures.textContent).toMatch(/Files ready.*4\s*\/\s*8/)
    expect(figures.textContent).toMatch(/Records\s*240/)
    expect(figures.textContent).toMatch(/Needs you.*2/)

    // the files that did not finish come from the status; the notes to read come from the typing report, which arrives later
    const needs = screen.getByRole('region', { name: 'Needs you' })
    expect(within(needs).getByText('Did not finish')).toBeTruthy()
    expect(within(needs).getByText('Did not finish').closest('a').getAttribute('title')).toMatch(/broken.csv/)
    expect(await within(needs).findByText('Read before trusting answers')).toBeTruthy()

    // the engine's answers for the week: 9 of 10 per case, twice
    expect(await within(figures).findByText('90%')).toBeTruthy()
    const answers = screen.getByRole('region', { name: 'Answers' })
    expect(within(answers).getByText('Answered').parentElement.textContent).toMatch(/Answered\s*18/)
    fireEvent.click(within(answers).getByRole('button', { name: /as a table/i }))
    expect(within(screen.getByRole('dialog')).getByText(/The case’s records cannot answer that kind of question \(2\)/)).toBeTruthy()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()

    // what the evidence holds, from the typing report
    const types = await screen.findByRole('region', { name: 'Evidence by type' })
    expect((await within(types).findAllByText('Images')).length).toBeGreaterThan(0)
    expect(types.textContent).toMatch(/Records\s*6/)
    const kinds = screen.getByRole('region', { name: 'Records by kind' })
    expect((await within(kinds).findAllByText('Calls')).length).toBeGreaterThan(0)

    // the pipeline, from the status of each case
    const aside = screen.getByRole('region', { name: 'Data quality' })
    expect(aside.textContent).toMatch(/7\.7%/)
    const ready = screen.getByRole('region', { name: 'Time to ready' })
    expect(ready.textContent).toMatch(/Median 41 s/)

    // the timeline, one row for each case
    const timeline = await screen.findByRole('region', { name: 'Evidence timeline' })
    expect(await within(timeline).findAllByRole('listitem')).toHaveLength(2)
    expect(within(timeline).getAllByRole('img').map(lane => lane.getAttribute('aria-label')).join(' | ')).toMatch(/from Jan 2024 to Mar 2024/)

    const cases = screen.getByRole('region', { name: 'Cases' })
    expect(within(cases).getAllByRole('row')).toHaveLength(3)
    expect(within(cases).getAllByText('Needs review').length).toBeGreaterThan(0)
  })

  it('says there is no case yet, and offers the one next step', async () => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: [] }
    globalThis.fetch = serve()
    render(<MemoryRouter initialEntries={['/']}><DashboardPage /></MemoryRouter>)
    const start = await screen.findByRole('region', { name: 'Start your first case' })
    expect(within(start).getAllByRole('listitem').map(item => item.querySelector('strong').textContent)).toEqual(['Create a case', 'Add evidence', 'Ask a question'])
    expect(within(start).getByRole('link', { name: /New case/ }).getAttribute('href')).toBe('/cases/new')
    expect(screen.queryByRole('region', { name: 'Figures' })).toBeNull()
  })

  it('says once that the service did not answer, instead of drawing a page of broken panels', async () => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a', 'case-b'] }
    globalThis.fetch = vi.fn(async () => ({ ok: false, status: 503, headers: { get: () => null }, json: async () => ({}), text: async () => '' }))
    render(<MemoryRouter initialEntries={['/']}><DashboardPage /></MemoryRouter>)
    const down = await screen.findByRole('alert')
    expect(down.textContent).toMatch(/The workspace could not read your cases./)
    expect(within(down).getByRole('button', { name: /Try again/ })).toBeTruthy()
    expect(screen.queryByRole('region', { name: 'Figures' })).toBeNull()
    expect(screen.queryByRole('region', { name: 'Evidence timeline' })).toBeNull()
  })

  it('gives every chart its table twin', async () => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a', 'case-b'] }
    globalThis.fetch = serve()
    render(<MemoryRouter initialEntries={['/']}><DashboardPage /></MemoryRouter>)
    for (const name of ['Evidence timeline', 'Evidence by type', 'Records by kind', 'Answers', 'Data quality', 'Time to ready']) {
      const region = await screen.findByRole('region', { name })
      await waitFor(() => expect(within(region).getByRole('button', { name: /as a table/i })).toBeTruthy())
    }
    const aside = screen.getByRole('region', { name: 'Data quality' })
    fireEvent.click(within(aside).getByRole('button', { name: /as a table/i }))
    expect(within(screen.getByRole('dialog')).getByRole('table', { name: /Data quality for each case/ })).toBeTruthy()
    expect(within(aside).queryByRole('table')).toBeNull()
  })

  it('offers the way back into the last question you asked', async () => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['case-a'] }
    localStorage.setItem('nexusai.viewer.questions.case-a', JSON.stringify([{ id: 'q1', query: 'Who called the target at night?', label: '', answer: '', state: 'answered', pinned: false, updatedAt: '2026-11-03T08:00:00Z' }]))
    globalThis.fetch = serve()
    render(<MemoryRouter initialEntries={['/']}><DashboardPage /></MemoryRouter>)
    const band = await screen.findByRole('region', { name: 'Pick up' })
    const link = within(band).getByRole('link', { name: /Who called the target at night?/ })
    expect(link.getAttribute('href')).toBe('/cases/case-a/investigate?question=Who%20called%20the%20target%20at%20night%3F')
    expect(link.textContent).toMatch(/Who called the target at night\?/)
  })
})

describe('the indicator pictures', () => {
  afterEach(() => { cleanup(); localStorage.clear() })
  const job = (queued, done) => ({ status: 'completed', queued_at: queued, completed_at: done })

  it('bins the time to ready on fixed edges and marks the bin of the median', () => {
    const rows = [{ state: { data: { recent_jobs: [job('2026-11-01T10:00:00Z', '2026-11-01T10:00:03Z'), job('2026-11-01T10:00:00Z', '2026-11-01T10:00:40Z'), job('2026-11-01T10:00:00Z', '2026-11-01T10:00:50Z'), job('2026-11-01T10:00:00Z', '2026-11-01T10:10:00Z'), { status: 'failed', queued_at: '2026-11-01T10:00:00Z', completed_at: '2026-11-01T10:00:10Z' }] } } }]
    const result = durationHistogram(rows)
    expect(result.count).toBe(4)
    expect(result.bins.map(bin => bin.count)).toEqual([1, 0, 0, 2, 0, 0, 1])
    expect(result.bins.filter(bin => bin.mark).map(bin => bin.label)).toEqual(['30 to 60 s'])
    expect(result.median).toBe(45)
  })

  it('has no bins to draw when no job finished', () => {
    expect(durationHistogram([{ state: { data: { recent_jobs: [] } } }, { state: null }]).count).toBe(0)
  })

  it('draws one dot for each thing and says how many it left out', () => {
    render(<DotRow max={3} label="5 things" dots={Array.from({ length: 5 }, (_, index) => ({ id: index, colour: 'red', title: 'x' }))} />)
    const picture = screen.getByRole('img', { name: '5 things' })
    expect(picture.querySelectorAll('i')).toHaveLength(3)
    expect(picture.textContent).toBe('+2')
  })

  it('sizes histogram bins on one scale and keeps empty bins quiet', () => {
    render(<Histogram label="Files" bins={[{ key: 'a', count: 4, mark: true, title: 'a' }, { key: 'b', count: 2, title: 'b' }, { key: 'c', count: 0, title: 'c' }]} />)
    const bars = screen.getByRole('img', { name: 'Files' }).querySelectorAll('i')
    expect(bars[0].style.getPropertyValue('--h')).toBe('100%')
    expect(bars[1].style.getPropertyValue('--h')).toBe('50%')
    expect(bars[0].className).toBe('is-mark')
    expect(bars[2].className).toBe('is-empty')
  })

  it('shows the real figure at once when less motion is asked for, and ends on it when not', async () => {
    localStorage.setItem('nexusai.viewer.motion', 'reduce')
    const { unmount } = render(<CountUp value={1234} format={value => value.toLocaleString('en-US')} />)
    expect(screen.getByText('1,234')).toBeTruthy()
    unmount()
    localStorage.clear()
    render(<CountUp value={1234} duration={30} format={value => value.toLocaleString('en-US')} />)
    await waitFor(() => expect(screen.getByText('1,234')).toBeTruthy())
  })
})

describe('alignTimelines', () => {
  const result = (caseId, days, truncated = false) => ({ caseId, days: days.map(([date, total]) => ({ date, total, byFamily: {} })), truncated })
  const lines = (...results) => caseTimelines(results, '2025-04-15')

  it('puts every case on one calendar that runs to the current month, so the columns line up', () => {
    const axis = alignTimelines(lines(result('a', [['2024-01-10', 5], ['2024-03-02', 2]]), result('b', [['2025-02-01', 4]])), '2025-04-15')
    expect(axis.groups).toHaveLength(16)
    expect([axis.groups[0].from, axis.groups.at(-1).to]).toEqual(['2024-01', '2025-04'])
    const [a, b] = axis.lanes
    expect(a.cells.map(cell => cell.state).join(' ')).toBe(['in', 'in', 'in', ...Array(13).fill('after')].join(' '))
    expect(a.cells.slice(0, 3).map(cell => cell.total)).toEqual([5, 0, 2])
    expect(b.cells[0].state).toBe('before')
    expect(b.cells[13]).toMatchObject({ from: '2025-02', total: 4, state: 'in' })
    expect(axis.ticks.map(tick => [tick.index, tick.year])).toEqual([[0, '2024'], [12, '2025']])
  })

  it('says "not read", not "empty", after the last row the service returned for a capped case', () => {
    const axis = alignTimelines(lines(result('a', [['2024-01-10', 5]], false), result('b', [['2024-01-10', 5], ['2024-02-02', 1]], true)), '2025-04-15')
    const [complete, capped] = axis.lanes
    expect(complete.cells.at(-1).state).toBe('after')
    expect(complete.unreadFrom).toBeNull()
    expect(capped.cells.at(-1).state).toBe('unread')
    expect(capped.unreadFrom).toBe(2)
  })

  it('groups a long axis in equal runs shared by every lane, and keeps every total', () => {
    const axis = alignTimelines(lines(result('a', [['2010-01-05', 3], ['2024-06-01', 4]]), result('b', [['2020-01-05', 9]])), '2026-11-04')
    expect(axis.groupSize).toBeGreaterThan(1)
    expect(axis.groups.length).toBeLessThanOrEqual(96)
    expect(axis.lanes.map(lane => lane.cells.reduce((sum, cell) => sum + cell.total, 0))).toEqual([7, 9])
    expect(axis.lanes.every(lane => lane.cells.length === axis.groups.length)).toBe(true)
    expect(axis.ticks.length).toBeLessThanOrEqual(7)
  })

  it('draws nothing when no case has a dated month, and keeps a case that has only future dates as an empty lane', () => {
    expect(alignTimelines([], '2025-04-15')).toBeNull()
    const future = caseTimelines([result('a', [['2090-01-01', 3]]), result('b', [['2024-05-01', 2]])], '2025-04-15')
    const axis = alignTimelines(future, '2025-04-15')
    expect(axis.lanes.map(lane => lane.caseId)).toEqual(['a', 'b'])
    expect(axis.lanes[0].cells.every(cell => cell.state === 'before' && cell.total === 0)).toBe(true)
  })
})

describe('the timeline panel', () => {
  afterEach(() => { cleanup(); localStorage.clear() })
  const result = (caseId, days, truncated = false) => ({ caseId, days: days.map(([date, total]) => ({ date, total, byFamily: {} })), truncated })
  const panel = () => {
    const timelines = caseTimelines([result('case-a', [['2024-01-10', 5], ['2024-03-02', 2]]), result('case-b', [['2024-01-10', 4]], true)], '2025-04-15')
    return render(<MemoryRouter><TimelinePanel timelines={timelines} loading={false} failures={[]} today="2025-04-15" /></MemoryRouter>)
  }

  it('reads one month with the arrow keys: the title names it and each lane gives that month’s figure', () => {
    panel()
    const plot = screen.getByRole('group', { name: /Events per month in each case/ })
    expect(plot.tabIndex).toBe(0)
    const status = document.querySelector('[data-readout]')
    expect(status.textContent).toBe('')
    plot.focus()
    fireEvent.keyDown(plot, { key: 'Home' })
    expect(status.textContent).toMatch(/^Jan 2024: /)
    expect(status.textContent).toMatch(/5/)
    fireEvent.keyDown(plot, { key: 'End' })
    expect(status.textContent).toMatch(/^Apr 2025: /)
    expect(status.textContent).toMatch(/not read/)
    expect(screen.getByText('Apr 2025', { selector: '.nx-panel__sub' })).toBeTruthy()
    fireEvent.keyDown(plot, { key: 'ArrowLeft' })
    expect(status.textContent).toMatch(/^Mar 2025: /)
    fireEvent.keyDown(plot, { key: 'Escape' })
    expect(status.textContent).toBe('')
    expect(screen.getByText('Events per month · drag to zoom')).toBeTruthy()
  })

  it('shows each lane’s total at rest, with a plus where the read was only the earliest rows', () => {
    panel()
    const lanes = screen.getAllByRole('listitem')
    expect(lanes[0].querySelector('.nx-tl__count').textContent).toBe('7')
    expect(lanes[1].querySelector('a').getAttribute('title')).toMatch(/at most 100 rows/)
    expect(lanes[0].querySelector('a').getAttribute('title')).not.toMatch(/at most 100 rows/)
    expect(lanes[1].querySelector('.nx-tl__count').textContent).toBe('4+')
  })

  it('offers the two scales and says which is on', () => {
    panel()
    const group = screen.getByRole('group', { name: 'Scale of the lanes' })
    const [own, shared] = within(group).getAllByRole('button')
    expect([own.getAttribute('aria-pressed'), shared.getAttribute('aria-pressed')]).toEqual(['true', 'false'])
    fireEvent.click(shared)
    expect(shared.getAttribute('aria-pressed')).toBe('true')
  })
})

describe('gaugePath', () => {
  it('runs along the half-circle from the left end to the right end, and a part of it ends where it should', () => {
    expect(gaugePath(0, 1)).toBe('M24.00 92.00 A76 76 0 0 1 176.00 92.00')
    expect(gaugePath(0, 0.5)).toBe('M24.00 92.00 A76 76 0 0 1 100.00 16.00')
    expect(gaugePath(0.5, 1)).toBe('M100.00 16.00 A76 76 0 0 1 176.00 92.00')
  })
})

describe('the flow of records', () => {
  const records = {
    kinds: [{ id: 'calls', count: 100 }, { id: 'sessions', count: 60 }, { id: 'usage', count: 0 }, { id: 'other', count: 40 }],
    byCase: [
      { caseId: 'a', values: { calls: 60, sessions: 30, usage: 0, other: 10 } },
      { caseId: 'b', values: { calls: 30, sessions: 20, usage: 0, other: 20 } },
      { caseId: 'c', values: { calls: 10, sessions: 10, usage: 0, other: 10 } },
    ],
  }
  it('keeps the kinds that have records, the largest first, and the cases the largest first', () => {
    const model = flowModel(records)
    expect(model.kinds.map(kind => kind.id)).toEqual(['calls', 'sessions', 'other'])
    expect(model.cases.map(item => item.caseId)).toEqual(['a', 'b', 'c'])
    expect(model.total).toBe(200)
  })
  it('folds the cases beyond the limit into one node, so every record is still drawn', () => {
    const model = flowModel(records, 2)
    expect(model.cases.map(item => item.label ?? item.caseId)).toEqual(['a', 'b', '1 more case'])
    expect(model.cases[2].values).toEqual({ calls: 10, sessions: 10, other: 10 })
    expect(model.total).toBe(200)
  })
  it('places the nodes and ribbons exactly: one scale for both sides, a ribbon as thick at both ends, the sums of the ribbons the totals', () => {
    const model = flowModel(records)
    const layout = flowLayout(model, 0.04)
    const thickness = layout.ribbons.map(ribbon => [ribbon.a1 - ribbon.a0, ribbon.b1 - ribbon.b0])
    for (const [left, right] of thickness) expect(left).toBeCloseTo(right, 10)
    expect(layout.ribbons.reduce((sum, ribbon) => sum + ribbon.value, 0)).toBe(200)
    const scale = layout.left[0].h / model.cases[0].total
    expect(layout.right[0].h).toBeCloseTo(model.kinds[0].count * scale, 10)
    const span = nodes => nodes.at(-1).y + nodes.at(-1).h - nodes[0].y
    expect(span(layout.left)).toBeCloseTo(layout.left.reduce((sum, node) => sum + node.h, 0) + 0.04 * 2, 10)
    expect(layout.left[0].y).toBeGreaterThanOrEqual(0)
    expect(layout.left.at(-1).y + layout.left.at(-1).h).toBeLessThanOrEqual(1.0000001)
  })
  it('stacks the ribbons of a case inside its node and the ribbons of a kind inside its node', () => {
    const layout = flowLayout(flowModel(records))
    for (const ribbon of layout.ribbons) {
      expect(ribbon.a0).toBeGreaterThanOrEqual(layout.left[ribbon.caseIndex].y - 1e-9)
      expect(ribbon.a1).toBeLessThanOrEqual(layout.left[ribbon.caseIndex].y + layout.left[ribbon.caseIndex].h + 1e-9)
      expect(ribbon.b0).toBeGreaterThanOrEqual(layout.right[ribbon.kindIndex].y - 1e-9)
      expect(ribbon.b1).toBeLessThanOrEqual(layout.right[ribbon.kindIndex].y + layout.right[ribbon.kindIndex].h + 1e-9)
    }
  })
  it('keeps labels apart without moving them off their node more than needed, and draws nothing for no records', () => {
    const near = spreadLabels([{ y: 0, h: 0.02 }, { y: 0.03, h: 0.02 }, { y: 0.06, h: 0.02 }], 0.075)
    ;[0.01, 0.085, 0.16].forEach((value, index) => expect(near[index]).toBeCloseTo(value, 10))
    const wide = spreadLabels([{ y: 0, h: 0.4 }, { y: 0.45, h: 0.5 }], 0.075)
    ;[0.2, 0.7].forEach((value, index) => expect(wide[index]).toBeCloseTo(value, 10))
    expect(flowLayout(flowModel({ kinds: [], byCase: [] })).ribbons).toEqual([])
    const tall = spreadLabels([{ y: 0, h: 0.02 }, { y: 0.03, h: 0.02 }], 0, [0.2, 0.1])
    expect(tall[0]).toBeCloseTo(0.01, 10)
    expect(tall[1]).toBeCloseTo(0.17, 10)
    const low = spreadLabels([{ y: 0.9, h: 0.05 }, { y: 0.95, h: 0.05 }], 0, [0.2, 0.2])
    expect(low[0]).toBeCloseTo(0.69, 10)
    expect(low[1]).toBeCloseTo(0.9, 10)
  })
})

describe('exceptionRows', () => {
  const quality = (over = {}) => ({ questions: 153, answered: 128, declined: 20, abstained: 5, reasons: [{ reason: 'a', text: 'Cannot answer', count: 11 }, { reason: 'b', text: 'No view fits', count: 9 }], ...over })
  it('gives one row for each of the three most common reasons, then the questions the engine held back', () => {
    expect(exceptionRows(quality()).map(row => `${row.id}:${row.outcome}:${row.count}`)).toEqual(['r0:declined:11', 'r1:declined:9', 'abstained:abstained:5'])
  })
  it('keeps the declines that no reason covers as their own row, and says so when no reason was recorded', () => {
    expect(exceptionRows(quality({ declined: 25 })).find(row => row.id === 'rx')).toMatchObject({ count: 5, label: 'Other reasons' })
    expect(exceptionRows(quality({ reasons: [] })).find(row => row.id === 'rx')).toMatchObject({ count: 20, label: 'No reason recorded' })
  })
  it('has no rows when everything was answered', () => {
    expect(exceptionRows(quality({ declined: 0, abstained: 0, reasons: [] }))).toEqual([])
  })
  it('lights the part of the bar of what a reason belongs to, and a reason with many questions is a bar, not marks', () => {
    expect(exceptionRows(quality({ declined: 40, reasons: [{ reason: 'a', text: 'Cannot answer', count: 40 }] }))[0]).toMatchObject({ id: 'r0', outcome: 'declined', count: 40 })
  })
})

describe('the view of the timeline', () => {
  it('keeps a window inside the calendar, never narrower than the minimum, and treats everything as no window', () => {
    expect(viewWindow(60, null)).toEqual([0, 60])
    expect(viewWindow(60, [10, 30])).toEqual([10, 30])
    expect(viewWindow(60, [10, 11])).toEqual([0, 60])
    expect(viewWindow(60, [-5, 999])).toEqual([0, 60])
    expect(MIN_WINDOW).toBe(3)
  })
  it('zooms in around a group and out again, and gives back no window when everything would show', () => {
    expect(zoomWindow(60, null, 0.5, 30)).toEqual([15, 45])
    expect(zoomWindow(60, [15, 45], 0.5, 30)).toEqual([23, 38])
    expect(zoomWindow(60, [15, 45], 2, 30)).toBeNull()
    expect(zoomWindow(60, [40, 60], 0.5, 59)).toEqual([50, 60])
  })
  it('moves the view without leaving the calendar, and does nothing when the whole calendar shows', () => {
    expect(panWindow(60, [10, 30], 5)).toEqual([15, 35])
    expect(panWindow(60, [50, 60], 9)).toEqual([50, 60])
    expect(panWindow(60, [2, 22], -9)).toEqual([0, 20])
    expect(panWindow(60, null, 3)).toBeNull()
  })
  it('puts a tick at each year start when the view is long and at each quarter when it is short', () => {
    const months = (start, length) => Array.from({ length }, (_, index) => { const total = start + index; const year = 2024 + Math.floor(total / 12); return { from: `${year}-${String((total % 12) + 1).padStart(2, '0')}`, to: `${year}-${String((total % 12) + 1).padStart(2, '0')}` } })
    expect(windowTicks(months(0, 40)).map(tick => tick.label)).toEqual(['2024', '2025', '2026', '2027'])
    expect(windowTicks(months(2, 10)).map(tick => tick.label)).toEqual(['Mar ’24', 'Apr', 'Jul', 'Oct'])
    expect(windowTicks([])).toEqual([])
  })
})

describe('the table dialog', () => {
  afterEach(() => { cleanup(); localStorage.clear() })
  const result = (caseId, days, truncated = false) => ({ caseId, days: days.map(([date, total]) => ({ date, total, byFamily: {} })), truncated })
  const open = () => {
    const timelines = caseTimelines([result('case-a', [['2024-01-10', 5], ['2024-03-02', 2]])], '2025-04-15')
    render(<MemoryRouter><TimelinePanel timelines={timelines} loading={false} failures={[]} today="2025-04-15" /></MemoryRouter>)
    const button = screen.getByRole('button', { name: /as a table/i })
    button.focus()
    fireEvent.click(button)
    return button
  }

  it('opens over the page, so the panel keeps its size, and names the table', () => {
    const button = open()
    const dialog = screen.getByRole('dialog')
    expect(dialog.getAttribute('aria-modal')).toBe('true')
    expect(within(dialog).getByRole('heading', { name: 'Events per case by month range' })).toBeTruthy()
    expect(within(dialog).getByRole('table')).toBeTruthy()
    expect(button.getAttribute('aria-haspopup')).toBe('dialog')
    expect(within(screen.getByRole('region', { name: 'Evidence timeline' })).queryByRole('table')).toBeNull()
  })

  it('moves focus in, keeps Tab inside, and returns focus to the button on Escape', () => {
    const button = open()
    const dialog = screen.getByRole('dialog')
    expect(document.activeElement).toBe(dialog)
    const close = within(dialog).getByRole('button', { name: 'Close the table' })
    const focusable = [...dialog.querySelectorAll('button, a[href], input, select, textarea, summary, [tabindex]:not([tabindex="-1"])')].filter(item => !item.disabled)
    focusable.at(-1).focus()
    fireEvent.keyDown(dialog, { key: 'Tab' })
    expect(document.activeElement).toBe(focusable[0])
    focusable[0].focus()
    fireEvent.keyDown(dialog, { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(focusable.at(-1))
    fireEvent.keyDown(dialog, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(document.activeElement).toBe(button)
    expect(close).toBeTruthy()
  })

  it('closes from its button and from a click outside it', () => {
    const button = open()
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Close the table' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(document.activeElement).toBe(button)
    fireEvent.click(button)
    fireEvent.mouseDown(screen.getByRole('dialog').parentElement)
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
