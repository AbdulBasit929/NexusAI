import { describe, expect, test } from 'vitest'
import { investigationStarters } from './InvestigatePage.jsx'

describe('investigation starter questions', () => {
  const capabilities = {
    families: [
      { id: 'cdr', label: 'CDR', availability: 'queryable', record_types: ['cdr'], suggested_queries: ['Count calls', 'Show call chronology'] },
      { id: 'anpr', label: 'ANPR', availability: 'limited', record_types: ['anpr'], suggested_queries: ['Count distinct plates'] },
      { id: 'document', label: 'Documents', availability: 'no_data', suggested_queries: ['Summarise documents'] },
      { id: 'internal_key', availability: 'queryable', suggested_queries: ['Retain raw family label'] },
    ],
  }

  test('uses only curated suggestions from available families', () => {
    expect(investigationStarters(capabilities)).toEqual([
      { query: 'Count calls', family: 'CDR', scope: 'cdr' },
      { query: 'Show call chronology', family: 'CDR', scope: 'cdr' },
      { query: 'Count distinct plates', family: 'ANPR', scope: 'anpr' },
      { query: 'Retain raw family label', family: 'internal_key', scope: 'all' },
    ])
  })

  test('filters suggestions to the exact selected scope and invents no fallback', () => {
    expect(investigationStarters(capabilities, 'anpr')).toEqual([
      { query: 'Count distinct plates', family: 'ANPR', scope: 'anpr' },
    ])
    expect(investigationStarters(null)).toEqual([])
  })
})
