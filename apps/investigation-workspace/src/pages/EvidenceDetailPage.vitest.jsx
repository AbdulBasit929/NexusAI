import { describe, expect, it } from 'vitest'
import { evidenceFacts } from './EvidenceDetailPage.jsx'

describe('evidenceFacts', () => {
  const item = { evidence_id: 'id-1', original_filename: 'calls.csv', modality: 'structured_records', detected_type: 'cdr', size_bytes: 2048, accepted_rows: 1200, created_at: '2026-09-27T10:00:00Z', sha256: 'abc123' }

  it('lists only what the service reported, in a fixed order, with the ID and hash copyable', () => {
    const facts = evidenceFacts(item)
    expect(facts.map(fact => fact.id)).toEqual(['id', 'name', 'family', 'size', 'rows', 'added', 'hash'])
    expect(facts.find(fact => fact.id === 'size').value).toBe('2.0 KB')
    expect(facts.find(fact => fact.id === 'rows').value).toBe('1,200')
    expect(facts.find(fact => fact.id === 'added').value).toBe('27 Sept 2026, 10:00 UTC')
    expect(facts.filter(fact => fact.copy).map(fact => fact.id)).toEqual(['id', 'hash'])
  })

  it('leaves a field out rather than filling it in, and uses whichever hash name the service used', () => {
    const facts = evidenceFacts({ evidence_id: 'id-2', content_hash: 'ff00' })
    expect(facts.map(fact => fact.id)).toEqual(['id', 'hash'])
    expect(facts.find(fact => fact.id === 'hash').value).toBe('ff00')
    expect(evidenceFacts({})).toEqual([])
  })
})
