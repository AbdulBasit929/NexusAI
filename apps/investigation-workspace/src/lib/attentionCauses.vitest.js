import { describe, expect, it } from 'vitest'
import { attentionCauses, causeOf } from './attentionCauses.js'

const failed = (caseId, label, detail = '') => ({ caseId, kind: 'Failed', label, detail, evidenceId: `${caseId}-${label}` })
const gap = (caseId, label) => ({ caseId, kind: 'Retained copy missing', label, detail: '', evidenceId: `${caseId}-${label}` })

describe('causeOf', () => {
  it('uses the reported reason, ignores case, spacing and a trailing full stop, and falls back to a plain label', () => {
    expect(causeOf(failed('a', 'x', 'The file has  no header row.')).label).toBe('The file has no header row')
    expect(causeOf(failed('a', 'x', 'The FILE has no header row')).key).toBe(causeOf(failed('a', 'y', 'the file has no header row.')).key)
    expect(causeOf(failed('a', 'x')).label).toBe('Did not finish processing')
    expect(causeOf(gap('a', 'x')).label).toBe('Retained copy is missing')
  })
})

describe('attentionCauses', () => {
  it('groups sources that share a cause into one finding, biggest first', () => {
    const items = [failed('a', 'one.csv', 'No header row'), failed('a', 'two.csv', 'No header row'), failed('b', 'three.csv', 'Bad encoding'), gap('a', 'scan.pdf')]
    const byCase = [{ caseId: 'a', failed: 2, missing: 1, total: 3 }, { caseId: 'b', failed: 1, missing: 0, total: 1 }]
    const causes = attentionCauses(items, byCase)
    expect(causes.map(cause => [cause.label, cause.count])).toEqual([['No header row', 2], ['Bad encoding', 1], ['Retained copy is missing', 1]])
    expect(causes[0]).toMatchObject({ kind: 'failed', caseIds: ['a'], extra: 0 })
    expect(causes[0].items.map(item => item.label)).toEqual(['one.csv', 'two.csv'])
  })

  it('reconciles named items to the complete counts: unnamed failures get their own group', () => {
    const items = [failed('a', 'one.csv', 'No header row')]
    const causes = attentionCauses(items, [{ caseId: 'a', failed: 4, missing: 0, total: 4 }])
    expect(causes.map(cause => [cause.label, cause.count, cause.extra])).toEqual([['Failed, cause not listed', 3, 3], ['No header row', 1, 0]])
    expect(causes.reduce((sum, cause) => sum + cause.count, 0)).toBe(4)
  })

  it('adds unnamed missing-copy sources to the one missing-copy cause, and creates it when none is named', () => {
    const withNamed = attentionCauses([gap('a', 'x.pdf')], [{ caseId: 'a', failed: 0, missing: 5, total: 5 }])
    expect(withNamed).toHaveLength(1)
    expect(withNamed[0]).toMatchObject({ label: 'Retained copy is missing', count: 5, extra: 4 })
    const none = attentionCauses([], [{ caseId: 'a', failed: 0, missing: 2, total: 2 }, { caseId: 'b', failed: 0, missing: 1, total: 1 }])
    expect(none[0]).toMatchObject({ count: 3, items: [], caseIds: ['a', 'b'] })
  })

  it('returns nothing when nothing needs review', () => {
    expect(attentionCauses([], [])).toEqual([])
    expect(attentionCauses([], [{ caseId: 'a', failed: 0, missing: 0, total: 0 }])).toEqual([])
  })
})
