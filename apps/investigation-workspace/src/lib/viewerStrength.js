// How strongly an observation about a source is stated: the source itself is always strong; a derived observation is only as
// strong as its recorded confidence, and one with no confidence is a candidate.
export function strength(confidence, source = false) {
  if (source) return { id: 'strong', label: 'Source evidence' }
  if (confidence == null) return { id: 'medium', label: 'Candidate observation' }
  if (confidence >= 0.8) return { id: 'strong', label: `High-confidence observation · ${Math.round(confidence * 100)}%` }
  if (confidence >= 0.5) return { id: 'medium', label: `Review observation · ${Math.round(confidence * 100)}%` }
  return { id: 'weak', label: `Low-confidence observation · ${Math.round(confidence * 100)}%` }
}
