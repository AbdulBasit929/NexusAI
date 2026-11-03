import { describe, expect, it } from 'vitest'
import { humanizeCaseId } from './caseLabel.js'

describe('humanizeCaseId', () => {
  it('reads an identifier as a title and a date', () => {
    expect(humanizeCaseId('orion-lahore-reference-20261030')).toEqual({ title: 'Orion Lahore Reference', date: '30 Oct 2026' })
  })
  it('keeps words that start with a digit and leaves an identifier with no date alone', () => {
    expect(humanizeCaseId('nexusai-wave1-dev2-20261021')).toEqual({ title: 'Nexusai Wave1 Dev2', date: '21 Oct 2026' })
    expect(humanizeCaseId('forensic-demo')).toEqual({ title: 'Forensic Demo', date: '' })
  })
  it('does not read a number that is not a date as one', () => {
    expect(humanizeCaseId('case-20261340')).toEqual({ title: 'Case 20261340', date: '' })
  })
  it('returns nothing for nothing', () => {
    expect(humanizeCaseId('')).toEqual({ title: '', date: '' })
  })
})
