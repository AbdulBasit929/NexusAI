import { describe, expect, it } from 'vitest'
import { answerSummary, contributionShare } from './answerSummary.js'

const strong = { id: 'strong', label: 'Source record' }
const base = over => ({
  citations: { groups: [{ strength: strong, contributingCount: 30, totalContributing: 100 }, { strength: strong, contributingCount: 70, totalContributing: 100 }], items: [{}, {}, {}], totalOpenable: 3, totalContributing: 100 },
  result: { rows: [{}], availableRows: 1, totalRows: 1, truncated: false },
  scope: [{ label: 'Evidence', value: 'Call detail records' }, { label: 'Target', value: '0300' }, { label: 'From', value: '2026-01-01' }],
  ...over,
})

describe('answerSummary', () => {
  it('states sources, rows behind the answer, evidence kind and scope from what was reported', () => {
    const items = answerSummary(base())
    expect(items.map(item => [item.id, item.value])).toEqual([['sources', '2 files'], ['rows', '100'], ['strength', 'Source record'], ['scope', 'Call detail records · 0300']])
    expect(items[0].detail).toBe('3 openable locations')
    expect(items[2].detail).toBe('2 of 2 files')
  })

  it('says plainly when nothing openable came with the answer, and falls back to the rows shown', () => {
    const items = answerSummary(base({ citations: { groups: [], items: [], totalContributing: 0 }, scope: [] }))
    expect(items[0]).toMatchObject({ id: 'sources', value: 'None openable', tone: 'warn' })
    expect(items[1]).toMatchObject({ id: 'rows', label: 'Rows shown', value: '1', detail: 'all of them' })
    expect(items.some(item => item.id === 'strength' || item.id === 'scope')).toBe(false)
  })

  it('names mixed evidence kinds and a truncated result without overstating', () => {
    const items = answerSummary(base({
      citations: { groups: [{ strength: strong }, { strength: strong }, { strength: { id: 'medium', label: 'Model observation' } }], items: [{}], totalOpenable: 0, totalContributing: 0 },
      result: { rows: [{}], availableRows: 50, totalRows: 900, truncated: true },
    }))
    expect(items.find(item => item.id === 'strength')).toMatchObject({ value: 'Source record', detail: 'and 1 other kind' })
    expect(items.find(item => item.id === 'rows')).toMatchObject({ value: '50', detail: 'of 900' })
    expect(items[0].detail).toBe('1 openable location')
  })
})

describe('contributionShare', () => {
  it('floors a file’s share so a part is never shown as the whole, and is null when unreported', () => {
    expect(contributionShare({ contributingCount: 999, totalContributing: 1000 })).toBe(99)
    expect(contributionShare({ contributingCount: 30, totalContributing: 100 })).toBe(30)
    expect(contributionShare({ contributingCount: null, totalContributing: 100 })).toBeNull()
    expect(contributionShare({})).toBeNull()
  })
})
