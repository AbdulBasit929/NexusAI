import { useCallback, useState } from 'react'

// Entries the analyst has marked reviewed, kept in this browser only (like a mail client's "done"): it clears an item from
// the "Needs attention" view without changing anything in the case or the service.
const KEY = 'nexusai.viewer.activity.reviewed'
const MAX = 500

export function readReviewed() {
  try {
    const value = JSON.parse(globalThis.localStorage?.getItem(KEY) || '[]')
    return new Set(Array.isArray(value) ? value.filter(item => typeof item === 'string').slice(-MAX) : [])
  } catch { return new Set() }
}

export function useReviewed() {
  const [reviewed, setReviewed] = useState(readReviewed)
  const toggle = useCallback(key => setReviewed(current => {
    const next = new Set(current)
    if (next.has(key)) next.delete(key); else next.add(key)
    try { globalThis.localStorage?.setItem(KEY, JSON.stringify([...next].slice(-MAX))) } catch { /* kept for this tab */ }
    return next
  }), [])
  return { reviewed, toggle }
}
