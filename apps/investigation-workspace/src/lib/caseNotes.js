import { useCallback, useSyncExternalStore } from 'react'

const notesCache = new Map()
const listeners = new Set()
let version = 0

function noteStorageKey(caseId) {
  return `nexusai.casenotes.${caseId}`
}

function readStoredNotes(caseId) {
  if (!caseId) return []
  try {
    const raw = globalThis.localStorage?.getItem(noteStorageKey(caseId))
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function persistNotes(caseId, notes) {
  notesCache.set(caseId, notes)
  try {
    globalThis.localStorage?.setItem(noteStorageKey(caseId), JSON.stringify(notes))
  } catch {
    // In-memory cache continues authoritative if localStorage is disabled or throws
  }
  version += 1
  for (const listener of listeners) listener()
}

function subscribe(listener) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

function getSnapshot(caseId) {
  if (!notesCache.has(caseId)) {
    notesCache.set(caseId, readStoredNotes(caseId))
  }
  return notesCache.get(caseId)
}

export function useCaseNotes(caseId) {
  const getSubSnapshot = useCallback(() => getSnapshot(caseId), [caseId])
  return useSyncExternalStore(subscribe, getSubSnapshot, getSubSnapshot)
}

export function addCaseNote(caseId, { text, category = 'hypothesis' }) {
  if (!caseId || !text || !text.trim()) return null
  const current = getSnapshot(caseId)
  const id = `note-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
  const newNote = {
    id,
    caseId,
    text: text.trim(),
    category,
    createdAt: new Date().toISOString(),
  }
  const next = [newNote, ...current].slice(0, 50)
  persistNotes(caseId, next)
  return newNote
}

export function deleteCaseNote(caseId, noteId) {
  if (!caseId || !noteId) return
  const current = getSnapshot(caseId)
  const next = current.filter(item => item.id !== noteId)
  persistNotes(caseId, next)
}

export function clearCaseNotesForTests() {
  notesCache.clear()
  version += 1
  for (const listener of listeners) listener()
}
