import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { addCaseNote, clearCaseNotesForTests, deleteCaseNote, useCaseNotes } from './caseNotes.js'

afterEach(() => {
  globalThis.localStorage?.clear()
  clearCaseNotesForTests()
})

describe('caseNotes store', () => {
  it('persists and updates case notes per caseId', () => {
    const { result } = renderHook(() => useCaseNotes('case-alpha'))
    expect(result.current).toEqual([])

    let note
    act(() => {
      note = addCaseNote('case-alpha', { text: 'Key suspect switched SIM at tower 402', category: 'lead' })
    })

    expect(note).toBeTruthy()
    expect(result.current).toHaveLength(1)
    expect(result.current[0].text).toBe('Key suspect switched SIM at tower 402')
    expect(result.current[0].category).toBe('lead')

    // Delete note
    act(() => {
      deleteCaseNote('case-alpha', note.id)
    })
    expect(result.current).toHaveLength(0)
  })

  it('keeps case notes isolated across different cases', () => {
    const hookA = renderHook(() => useCaseNotes('case-1'))
    const hookB = renderHook(() => useCaseNotes('case-2'))

    act(() => {
      addCaseNote('case-1', { text: 'Note for Case 1' })
      addCaseNote('case-2', { text: 'Note for Case 2' })
    })

    expect(hookA.result.current).toHaveLength(1)
    expect(hookA.result.current[0].text).toBe('Note for Case 1')

    expect(hookB.result.current).toHaveLength(1)
    expect(hookB.result.current[0].text).toBe('Note for Case 2')
  })
})
