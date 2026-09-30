import { describe, expect, it } from 'vitest'
import { askAcrossCases, outcomeHeadline, planSearch, sortOutcomes, tallyOutcomes } from './acrossCases.js'

const outcome = (caseId, state) => ({ caseId, presentation: { state } })

describe('planSearch', () => {
  const rows = [
    { caseId: 'a', summary: { total: 5, ready: 2 } },
    { caseId: 'b', summary: { total: 0, ready: 0 } },
    { caseId: 'c', summary: { total: 9, ready: 9 } },
    { caseId: 'd', summary: null },
  ]

  it('searches every case with evidence, most ready sources first, and names the ones skipped', () => {
    expect(planSearch(rows)).toEqual({ search: ['c', 'a', 'd'], skipped: [{ caseId: 'b', reason: 'no_evidence' }] })
  })

  it('still asks a case whose status is not known yet, rather than assuming it is empty', () => {
    expect(planSearch(rows).search).toContain('d')
  })

  it('narrows to the chosen cases and says the rest were not selected', () => {
    expect(planSearch(rows, ['a', 'b'])).toEqual({ search: ['a'], skipped: [{ caseId: 'b', reason: 'no_evidence' }, { caseId: 'c', reason: 'not_chosen' }, { caseId: 'd', reason: 'not_chosen' }] })
  })
})

describe('outcomes', () => {
  const list = [outcome('z', 'zero-result'), outcome('b', 'clarify'), outcome('a', 'answered'), outcome('c', 'answered'), outcome('f', 'failed')]

  it('orders answered first, then a needed choice, then no matches, then failures', () => {
    expect(sortOutcomes(list).map(item => item.caseId)).toEqual(['a', 'c', 'b', 'z', 'f'])
  })

  it('counts each state and says how many cases had something', () => {
    const tally = tallyOutcomes(list)
    expect(tally.found).toBe(2)
    expect(outcomeHeadline(tally, 5)).toBe('Found in 2 of 5 cases.')
    expect(outcomeHeadline(tallyOutcomes([outcome('a', 'zero-result')]), 1)).toBe('Nothing found in the case searched.')
    expect(outcomeHeadline(tallyOutcomes([outcome('a', 'zero-result'), outcome('b', 'zero-result')]), 2)).toBe('Nothing found in any of the 2 cases searched.')
    expect(outcomeHeadline(tallyOutcomes([outcome('a', 'clarify')]), 1)).toBe('No answer yet: 1 case needs a choice from you.')
    expect(outcomeHeadline(tallyOutcomes([]), 0)).toBe('No case could be searched.')
  })
})

describe('askAcrossCases', () => {
  const present = (response, caseId, error) => ({ state: error ? 'failed' : response.state })

  it('asks every case, never more than the concurrency at once, and reports outcomes as they arrive', async () => {
    let active = 0
    let peak = 0
    const seen = []
    const result = await askAcrossCases({
      caseIds: ['a', 'b', 'c', 'd', 'e'],
      concurrency: 2,
      ask: async () => { active += 1; peak = Math.max(peak, active); await new Promise(resolve => setTimeout(resolve, 5)); active -= 1; return { state: 'answered' } },
      present,
      onOutcome: (item, count) => seen.push([item.caseId, count]),
    })
    expect(peak).toBe(2)
    expect(result).toHaveLength(5)
    expect(seen.map(entry => entry[1])).toEqual([1, 2, 3, 4, 5])
  })

  it('turns a failed case into a failed outcome and lets the others answer', async () => {
    const result = await askAcrossCases({
      caseIds: ['a', 'b'],
      ask: async caseId => { if (caseId === 'a') throw new Error('boom'); return { state: 'answered' } },
      present,
    })
    expect(result.map(item => [item.caseId, item.presentation.state]).sort()).toEqual([['a', 'failed'], ['b', 'answered']])
  })

  it('stops asking further cases once aborted and returns what arrived', async () => {
    const controller = new AbortController()
    const result = await askAcrossCases({
      caseIds: ['a', 'b', 'c', 'd'],
      concurrency: 1,
      signal: controller.signal,
      ask: async caseId => { if (caseId === 'a') controller.abort(); return { state: 'answered' } },
      present,
    })
    expect(result.map(item => item.caseId)).toEqual(['a'])
  })
})
