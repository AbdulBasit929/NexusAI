import { describe, expect, it } from 'vitest'
import { caseNameRules, existingCase, suggestCaseName } from './caseName.js'

describe('caseNameRules', () => {
  const met = value => Object.fromEntries(caseNameRules(value).map(rule => [rule.id, rule.met]))

  it('reports each rule separately as the analyst types', () => {
    expect(met('operation-falcon-2026')).toEqual({ chars: true, length: true, edges: true })
    expect(met('Operation Falcon')).toEqual({ chars: false, length: true, edges: false })
    expect(met('abc')).toEqual({ chars: true, length: false, edges: true })
    expect(met('-abcd-')).toEqual({ chars: true, length: true, edges: false })
    expect(met('')).toEqual({ chars: false, length: false, edges: false })
    expect(met('a'.repeat(65)).length).toBe(false)
  })
})

describe('suggestCaseName', () => {
  it('offers a valid version of what was typed, without changing anything itself', () => {
    expect(suggestCaseName('Operation Falcon 2026')).toBe('operation-falcon-2026')
    expect(suggestCaseName('  Café  Raid!! ')).toBe('cafe-raid')
    expect(suggestCaseName('CASE_42/alpha')).toBe('case-42-alpha')
  })

  it('offers nothing when the text is already valid or nothing usable is left', () => {
    expect(suggestCaseName('operation-falcon')).toBeNull()
    expect(suggestCaseName('!!')).toBeNull()
    expect(suggestCaseName('ab')).toBeNull()
    expect(suggestCaseName('')).toBeNull()
  })

  it('never suggests something longer than the limit or ending in a hyphen', () => {
    const suggestion = suggestCaseName(`${'word '.repeat(30)}`)
    expect(suggestion.length).toBeLessThanOrEqual(64)
    expect(suggestion.endsWith('-')).toBe(false)
  })
})

describe('existingCase', () => {
  it('finds an identifier that already belongs to a case, exactly', () => {
    expect(existingCase('alpha', ['alpha', 'bravo'])).toBe('alpha')
    expect(existingCase('alph', ['alpha'])).toBeNull()
  })
})
