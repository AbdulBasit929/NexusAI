import { useCallback, useSyncExternalStore } from 'react'

const EMPTY_HISTORY = []
const histories = new Map()
const drafts = new Map()
const listeners = new Set()

function historyKey(caseId) { return `nexusai.viewer.questions.${caseId}` }
function draftKey(caseId) { return `nexusai.viewer.draft.${caseId}` }

function readJSON(key, fallback) {
  try {
    const value = globalThis.localStorage?.getItem(key)
    return value ? JSON.parse(value) : fallback
  } catch { return fallback }
}

function readText(key, fallback = '') {
  try { return globalThis.localStorage?.getItem(key) ?? fallback } catch { return fallback }
}

function write(key, value) {
  try { globalThis.localStorage?.setItem(key, value) } catch { /* in-memory state remains authoritative for this tab */ }
}

// A case switch is a navigation boundary. Session-scoped filters belong to
// the old case and must be gone before the next route can issue a request.
// Durable drafts/history are keyed by case and remain safely isolated.
export function clearCaseSessionScope() {
  try {
    for (let index = globalThis.sessionStorage.length - 1; index >= 0; index -= 1) {
      const key = globalThis.sessionStorage.key(index)
      if (key?.startsWith('nexusai.case.')) globalThis.sessionStorage.removeItem(key)
    }
  } catch { /* navigation remains safe when storage is unavailable */ }
}

function historySnapshot(caseId) {
  if (!histories.has(caseId)) {
    const stored = readJSON(historyKey(caseId), [])
    histories.set(caseId, Array.isArray(stored) ? stored : [])
  }
  return histories.get(caseId)
}

let version = 0
const merged = new Map()

function emit() { version += 1; for (const listener of listeners) listener() }
function subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener) }

function saveHistory(caseId, entries) {
  histories.set(caseId, entries)
  write(historyKey(caseId), JSON.stringify(entries))
  emit()
}

export function recordQuestionHistory(caseId, query, presentation) {
  if (!caseId || !query || !presentation) return
  const existing = historySnapshot(caseId)
  const previous = existing.find(item => item.query === query)
  const entry = {
    id: previous?.id || globalThis.crypto?.randomUUID?.() || `${Date.now()}-${existing.length}`,
    query,
    label: previous?.label || '',
    answer: presentation.answer || presentation.clarification?.title || '',
    state: presentation.state,
    pinned: Boolean(previous?.pinned),
    updatedAt: new Date().toISOString(),
  }
  const next = [entry, ...existing.filter(item => item.query !== query)]
    .sort((left, right) => Number(right.pinned) - Number(left.pinned) || String(right.updatedAt).localeCompare(String(left.updatedAt)))
    .slice(0, 50)
  saveHistory(caseId, next)
}

export function toggleQuestionPin(caseId, id) {
  const next = historySnapshot(caseId).map(item => item.id === id ? { ...item, pinned: !item.pinned } : item)
    .sort((left, right) => Number(right.pinned) - Number(left.pinned) || String(right.updatedAt).localeCompare(String(left.updatedAt)))
  saveHistory(caseId, next)
}

export function renameQuestionHistory(caseId, id, label) {
  const resolved = String(label || '').trim().slice(0, 120)
  const next = historySnapshot(caseId).map(item => item.id === id ? { ...item, label: resolved } : item)
  saveHistory(caseId, next)
}

export function useQuestionHistory(caseId) {
  return useSyncExternalStore(subscribe, () => historySnapshot(caseId), () => historySnapshot(caseId))
}

export function useQuestionDraft(caseId) {
  const key = draftKey(caseId)
  const getSnapshot = useCallback(() => {
    if (!drafts.has(key)) drafts.set(key, readText(key))
    return drafts.get(key)
  }, [key])
  const value = useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
  const setValue = useCallback(next => {
    const current = drafts.has(key) ? drafts.get(key) : readText(key)
    const resolved = typeof next === 'function' ? next(current) : next
    drafts.set(key, String(resolved ?? ''))
    write(key, drafts.get(key))
    emit()
  }, [key])
  return [value, setValue]
}

// Question history across SEVERAL cases, for the dashboard's resume list.
//
// Calling `useQuestionHistory` in a loop would break the rules of hooks the
// moment the configured case list changed length. This reads every case in one
// subscription and caches the merged array against a version counter, because
// `useSyncExternalStore` requires the same reference back until something
// actually changes.
export function useQuestionHistoryAcross(caseIds) {
  const key = (caseIds || []).join(',')
  const getSnapshot = useCallback(() => {
    const cached = merged.get(key)
    if (cached && cached.version === version) return cached.value
    const value = key === ''
      ? EMPTY_HISTORY
      : key.split(',')
        .flatMap(caseId => historySnapshot(caseId).map(entry => ({ ...entry, caseId })))
        .sort((left, right) => String(right.updatedAt).localeCompare(String(left.updatedAt)))
    merged.set(key, { version, value })
    return value
  }, [key])
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
}

// Everything this workspace keeps in the browser, so Settings can report it
// honestly and offer to remove it. Nothing here has ever left this device.
export function localStorageFootprint() {
  const entries = []
  try {
    for (let index = 0; index < globalThis.localStorage.length; index += 1) {
      const key = globalThis.localStorage.key(index)
      if (!key?.startsWith('nexusai.')) continue
      entries.push({ key, bytes: (globalThis.localStorage.getItem(key) || '').length })
    }
  } catch { return { entries: [], available: false, questions: 0, drafts: 0, cases: 0, preferences: 0, bytes: 0 } }
  const caseIds = new Set()
  let questions = 0
  let drafts = 0
  let preferences = 0
  for (const entry of entries) {
    if (entry.key.startsWith('nexusai.viewer.questions.')) {
      const caseId = entry.key.slice('nexusai.viewer.questions.'.length)
      try {
        const stored = JSON.parse(globalThis.localStorage.getItem(entry.key))
        if (Array.isArray(stored) && stored.length) {
          questions += stored.length
          if (caseId) caseIds.add(caseId)
        }
      } catch { /* malformed local history contributes no invented count */ }
      continue
    }
    if (entry.key.startsWith('nexusai.viewer.draft.')) {
      const caseId = entry.key.slice('nexusai.viewer.draft.'.length)
      try {
        if ((globalThis.localStorage.getItem(entry.key) || '').trim()) {
          drafts += 1
          if (caseId) caseIds.add(caseId)
        }
      } catch { /* availability was established above; a single bad value stays uncounted */ }
      continue
    }
    preferences += 1
  }
  return {
    entries,
    available: true,
    questions,
    drafts,
    cases: caseIds.size,
    preferences,
    bytes: entries.reduce((total, entry) => total + entry.bytes, 0),
  }
}

// What this browser keeps, by kind, so the analyst can see it and remove one kind at a time. Counts are of items (questions,
// drafts, pinned days, reviewed entries) and sizes are characters stored, both read from what is actually there.
const CATEGORIES = [
  { id: 'questions', label: 'Question history', note: 'The questions you asked in each case, so you can reopen them.', prefix: 'nexusai.viewer.questions.' },
  { id: 'drafts', label: 'Draft questions', note: 'Question text you typed and did not send.', prefix: 'nexusai.viewer.draft.' },
  { id: 'pins', label: 'Pinned timeline days', note: 'Days you pinned on a case timeline, with your notes.', prefix: 'nexusai.viewer.timeline.pins.' },
  { id: 'reviewed', label: 'Reviewed activity', note: 'Entries you marked reviewed on the Activity page.', prefix: 'nexusai.viewer.activity.reviewed' },
  { id: 'appearance', label: 'Appearance', note: 'Your theme choice.', prefix: 'nexusai.viewer.theme' },
]

export function storageInventory() {
  const rows = [...CATEGORIES.map(category => ({ ...category, items: 0, bytes: 0, keys: [] })), { id: 'other', label: 'Other preferences', note: 'Navigation and view preferences.', prefix: null, items: 0, bytes: 0, keys: [] }]
  try {
    for (let index = 0; index < globalThis.localStorage.length; index += 1) {
      const key = globalThis.localStorage.key(index)
      if (!key?.startsWith('nexusai.')) continue
      const value = globalThis.localStorage.getItem(key) || ''
      const row = rows.find(entry => entry.prefix && key.startsWith(entry.prefix)) || rows[rows.length - 1]
      let items = 1
      if (['questions', 'pins', 'reviewed'].includes(row.id)) { try { const parsed = JSON.parse(value); items = Array.isArray(parsed) ? parsed.length : 0 } catch { items = 0 } }
      else if (row.id === 'drafts') items = value.trim() ? 1 : 0
      row.items += items
      row.bytes += key.length + value.length
      row.keys.push(key)
    }
  } catch { return { available: false, rows: [], bytes: 0 } }
  return { available: true, rows, bytes: rows.reduce((total, row) => total + row.bytes, 0) }
}

// Everything this browser keeps for the workspace as one JSON document, for the analyst to keep or move. It is read from
// local storage as it is; nothing is added.
export function storageExport() {
  const data = {}
  try {
    for (let index = 0; index < globalThis.localStorage.length; index += 1) {
      const key = globalThis.localStorage.key(index)
      if (!key?.startsWith('nexusai.')) continue
      const value = globalThis.localStorage.getItem(key) || ''
      try { data[key] = JSON.parse(value) } catch { data[key] = value }
    }
  } catch { return '{}' }
  return JSON.stringify({ exportedAt: new Date().toISOString(), note: 'Saved in this browser only; not part of any case record.', data }, null, 2)
}

// Removes one kind of stored data (never anything outside this workspace's own keys).
export function clearStorageKeys(keys) {
  try { for (const key of keys) if (key.startsWith('nexusai.')) globalThis.localStorage.removeItem(key) } catch { /* nothing more to remove */ }
  histories.clear()
  drafts.clear()
  merged.clear()
  emit()
}

// Removes stored question history and drafts for every case. Deliberately does
// NOT touch retained evidence -- nothing in this browser is evidence, and this
// control must never read as though it could delete any.
export function clearStoredQuestions() {
  try {
    const doomed = []
    for (let index = 0; index < globalThis.localStorage.length; index += 1) {
      const key = globalThis.localStorage.key(index)
      if (key?.startsWith('nexusai.viewer.questions.') || key?.startsWith('nexusai.viewer.draft.')) doomed.push(key)
    }
    for (const key of doomed) globalThis.localStorage.removeItem(key)
  } catch { /* the in-memory clear below still applies to this tab */ }
  histories.clear()
  drafts.clear()
  merged.clear()
  emit()
}

export function clearWorkspaceStateForTests() {
  histories.clear()
  drafts.clear()
  merged.clear()
  emit()
}
