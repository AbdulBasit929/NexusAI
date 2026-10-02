import { useCallback, useState } from 'react'

// Days an analyst has pinned on a case's timeline, each with a short note. Kept in this browser only (and said so in the
// interface) until the service stores annotations for the case (docs/work/BACKEND_REQUESTS.md, row 21).
const key = caseId => `nexusai.viewer.timeline.pins.${caseId}`
const MAX_NOTE = 400

export function readPins(caseId) {
  try {
    const value = JSON.parse(globalThis.localStorage?.getItem(key(caseId)) || '[]')
    return Array.isArray(value) ? value.filter(pin => pin && /^\d{4}-\d{2}-\d{2}$/.test(pin.date)).map(pin => ({ date: pin.date, note: String(pin.note || '').slice(0, MAX_NOTE) })) : []
  } catch { return [] }
}

function writePins(caseId, pins) {
  try { globalThis.localStorage?.setItem(key(caseId), JSON.stringify(pins)) } catch { /* the in-memory list stays right for this tab */ }
}

export function withPin(pins, date) {
  return pins.some(pin => pin.date === date) ? pins.filter(pin => pin.date !== date) : [...pins, { date, note: '' }].sort((left, right) => left.date.localeCompare(right.date))
}

export function withNote(pins, date, note) {
  const text = String(note).slice(0, MAX_NOTE)
  return pins.some(pin => pin.date === date) ? pins.map(pin => (pin.date === date ? { ...pin, note: text } : pin)) : [...pins, { date, note: text }].sort((left, right) => left.date.localeCompare(right.date))
}

export function usePins(caseId) {
  const [pins, setPins] = useState(() => readPins(caseId))
  const [forCase, setForCase] = useState(caseId)
  if (forCase !== caseId) { setForCase(caseId); setPins(readPins(caseId)) }
  const update = useCallback(change => setPins(current => { const next = change(current); writePins(caseId, next); return next }), [caseId])
  return { pins, toggle: date => update(list => withPin(list, date)), setNote: (date, note) => update(list => withNote(list, date, note)) }
}
