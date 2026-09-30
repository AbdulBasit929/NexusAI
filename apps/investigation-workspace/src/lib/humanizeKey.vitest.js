import { describe, expect, it } from 'vitest'
import { humanizeKey } from './format.js'

describe('humanizeKey', () => {
  it('turns engine column names into words', () => {
    expect(humanizeKey('accepted_rows')).toBe('Accepted rows')
    expect(humanizeKey('lastObservedAt')).toBe('Last observed at')
    expect(humanizeKey('evidence-ids')).toBe('Evidence ids')
    expect(humanizeKey('  processing_states ')).toBe('Processing states')
  })

  it('leaves measure columns and empty input alone', () => {
    expect(humanizeKey('m1')).toBe('m1')
    expect(humanizeKey('')).toBe('')
    expect(humanizeKey(null)).toBe('')
  })
})
