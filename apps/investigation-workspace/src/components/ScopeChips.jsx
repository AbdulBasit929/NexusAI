import { useEffect, useState } from 'react'

export const evidenceFamilies = [
  { id: 'all', label: 'All' },
  { id: 'cdr', label: 'CDR', recordType: 'cdr' },
  { id: 'ipdr', label: 'IPDR', recordType: 'ipdr' },
  { id: 'anpr', label: 'ANPR', recordType: 'anpr' },
  { id: 'subscriber', label: 'Subscriber', recordType: 'subscriber' },
  // `tower_location`, NOT `tower`: that is the record_type stored in
  // forensic.records and declared in semantic_layer/tower.yaml. With 'tower'
  // the evidence filter matched nothing, and -- worse -- the Investigate scope
  // sent record_type=tower to the question API, which answered "There are no
  // tower records in this case" about a case holding 5. A confident false
  // negative. Measured and fixed 2026-09-27.
  { id: 'tower', label: 'Tower', recordType: 'tower_location' },
  { id: 'financial', label: 'Financial', recordType: 'transaction' },
  { id: 'access_log', label: 'Access log', recordType: 'access_log' },
  { id: 'document', label: 'Documents', modality: 'document' },
  { id: 'image', label: 'Images', modality: 'image' },
  { id: 'audio', label: 'Audio', modality: 'audio' },
  { id: 'video', label: 'Video', modality: 'video' },
]

function readScope(storageKey, fallback) {
  try {
    const value = globalThis.sessionStorage?.getItem(storageKey)
    return evidenceFamilies.some(item => item.id === value) ? value : fallback
  } catch { return fallback }
}

export function useEvidenceScope(caseId, surface, fallback = 'all') {
  const storageKey = `nexusai.case.${caseId}.scope.${surface}`
  const [scope, setScope] = useState(() => readScope(storageKey, fallback))
  useEffect(() => {
    try { globalThis.sessionStorage?.setItem(storageKey, scope) } catch { /* session preference remains in memory */ }
  }, [scope, storageKey])
  return [scope, setScope]
}

export function familyDefinition(id) { return evidenceFamilies.find(item => item.id === id) || evidenceFamilies[0] }

export function evidenceMatchesFamily(evidence, familyId) {
  const family = familyDefinition(familyId)
  if (family.id === 'all') return true
  if (family.modality) return String(evidence?.modality || '').toLowerCase() === family.modality
  return String(evidence?.detected_type || '').toLowerCase() === family.recordType
}

export function ScopeChips({ value, onChange, counts = {}, label = 'Evidence family scope' }) {
  const selected = familyDefinition(value)
  return <div className="scope-filter"><p className="scope-filter__label"><span>Evidence families</span><strong>{selected.label}</strong></p><div className="filter-chips" role="group" aria-label={label}>{evidenceFamilies.map(item => <button key={item.id} type="button" aria-pressed={value === item.id} onClick={() => onChange(item.id)}>{item.label}{counts[item.id] != null ? <span>{Number(counts[item.id]).toLocaleString()}</span> : null}</button>)}</div></div>
}
