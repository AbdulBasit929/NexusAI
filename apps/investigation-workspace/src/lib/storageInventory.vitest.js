import { afterEach, describe, expect, it } from 'vitest'
import { clearStorageKeys, storageExport, storageInventory } from './workspaceState.js'

afterEach(() => localStorage.clear())

describe('browser storage inventory', () => {
  it('counts what is kept by kind, sizes it, and clears one kind without touching the others', () => {
    localStorage.setItem('nexusai.viewer.questions.a', JSON.stringify([{ id: 1 }, { id: 2 }]))
    localStorage.setItem('nexusai.viewer.draft.a', 'half a question')
    localStorage.setItem('nexusai.viewer.timeline.pins.a', JSON.stringify([{ date: '2026-02-02', note: '' }]))
    localStorage.setItem('nexusai.viewer.activity.reviewed', JSON.stringify(['a|e1', 'a|e2', 'b|e3']))
    localStorage.setItem('nexusai.viewer.theme', 'dark')
    localStorage.setItem('nexusai.viewer.rail', 'wide')
    localStorage.setItem('unrelated.key', 'x')
    const inventory = storageInventory()
    const by = Object.fromEntries(inventory.rows.map(row => [row.id, row.items]))
    expect(by).toEqual({ questions: 2, drafts: 1, pins: 1, reviewed: 3, appearance: 1, other: 1 })
    expect(inventory.bytes).toBeGreaterThan(50)
    clearStorageKeys(inventory.rows.find(row => row.id === 'pins').keys)
    expect(localStorage.getItem('nexusai.viewer.timeline.pins.a')).toBeNull()
    expect(localStorage.getItem('nexusai.viewer.theme')).toBe('dark')
    clearStorageKeys(['unrelated.key'])
    expect(localStorage.getItem('unrelated.key')).toBe('x')
    const exported = JSON.parse(storageExport())
    expect(Object.keys(exported.data)).toContain('nexusai.viewer.questions.a')
    expect(Object.keys(exported.data)).not.toContain('unrelated.key')
  })
})
