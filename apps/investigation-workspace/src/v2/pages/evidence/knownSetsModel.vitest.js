import { describe, expect, test } from 'vitest'
import { addedSentence, applyHide, flagsByEvidence, flagSummary, harmlessCount, onlyFlagged, toggled } from './knownSetsModel.js'

const flag = (evidence_id, set_kind, set_name, tier = 'same_image') => ({ evidence_id, set_kind, set_name, tier, distance: 0 })
const response = {
  flags: [
    flag('a', 'of_interest', 'Case 12 watch', 'identical'),
    flag('b', 'known_harmless', 'Stock photos'),
    flag('c', 'known_harmless', 'Stock photos'),
    flag('c', 'of_interest', 'Case 12 watch'),
    flag('d', 'known_harmless', 'Logos'),
    flag('d', 'known_harmless', 'Stock photos'),
  ],
}
const items = ['a', 'b', 'c', 'd', 'e'].map(evidence_id => ({ evidence_id }))

describe('flags of a page', () => {
  test('are grouped by picture', () => {
    const map = flagsByEvidence(response)
    expect([...map.keys()]).toEqual(['a', 'b', 'c', 'd'])
    expect(map.get('c')).toHaveLength(2)
    expect(flagsByEvidence(null).size).toBe(0)
  })

  test('a card says watch list when any set is a watch list, and names the set and how it matched', () => {
    const map = flagsByEvidence(response)
    expect(flagSummary(map.get('a'))).toMatchObject({ tone: 'watch', label: 'Case 12 watch', kindLabel: 'Watch list' })
    expect(flagSummary(map.get('a')).title).toContain('the same file')
    expect(flagSummary(map.get('c'))).toMatchObject({ tone: 'watch' })
    expect(flagSummary(map.get('b')).title).toContain('the same picture, saved or resized')
    expect(flagSummary(map.get('d'))).toMatchObject({ tone: 'harmless', label: 'Logos +1', kindLabel: 'Known harmless' })
    expect(flagSummary(undefined)).toBeNull()
    expect(flagSummary([])).toBeNull()
  })

  test('several watch lists show the first name and how many more', () => {
    const summary = flagSummary([flag('x', 'of_interest', 'One'), flag('x', 'of_interest', 'Two'), flag('x', 'of_interest', 'Two')])
    expect(summary.label).toBe('One +1')
    expect(summary.title).toContain('One, Two')
  })
})

describe('hiding known harmless pictures', () => {
  const map = flagsByEvidence(response)

  test('shows everything when the viewer did not ask to hide', () => {
    const { visible, hidden } = applyHide(items, map, false)
    expect(visible).toHaveLength(5)
    expect(hidden).toHaveLength(0)
  })

  test('hides a picture that only harmless sets flagged, never one on a watch list, never an unflagged one', () => {
    const { visible, hidden } = applyHide(items, map, true)
    expect(hidden.map(item => item.evidence_id)).toEqual(['b', 'd'])
    expect(visible.map(item => item.evidence_id)).toEqual(['a', 'c', 'e'])
    expect(harmlessCount(items, map)).toBe(2)
  })
})

test('the Flagged chip keeps only the pictures that any set flags', () => {
  const map = flagsByEvidence(response)
  expect(onlyFlagged(items, map).map(item => item.evidence_id)).toEqual(['a', 'b', 'c', 'd'])
  expect(onlyFlagged(items, new Map())).toEqual([])
})

test('selection toggles without changing the old set', () => {
  const one = new Set(['a'])
  const two = toggled(one, 'b')
  expect([...two]).toEqual(['a', 'b'])
  expect([...one]).toEqual(['a'])
  expect([...toggled(two, 'a')]).toEqual(['b'])
})

test('the result of adding hashes reads as one sentence', () => {
  expect(addedSentence({ added: 1 })).toBe('1 hash added.')
  expect(addedSentence({ added: 3, already_in_set: 2, rejected_count: 1, evidence_not_found: 4 })).toBe('3 hashes added, 2 already in the set, 1 row could not be used, 4 files are not a picture of this case.')
  expect(addedSentence({})).toBe('0 hashes added.')
})
