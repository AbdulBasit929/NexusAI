import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { clearCaseSessionScope, clearWorkspaceStateForTests, recordQuestionHistory, renameQuestionHistory, toggleQuestionPin, useQuestionDraft, useQuestionHistory } from './workspaceState.js'

afterEach(() => { vi.restoreAllMocks(); globalThis.localStorage.clear(); globalThis.sessionStorage.clear(); clearWorkspaceStateForTests() })

describe('viewer workspace state', () => {
  it('persists a draft and question pins per case', () => {
    const draft = renderHook(() => useQuestionDraft('case-a'))
    act(() => draft.result.current[1]('Who called this number?'))
    expect(draft.result.current[0]).toBe('Who called this number?')
    expect(globalThis.localStorage.getItem('nexusai.viewer.draft.case-a')).toBe('Who called this number?')
    const history = renderHook(() => useQuestionHistory('case-a'))
    act(() => recordQuestionHistory('case-a', 'Who called this number?', { state: 'answered', answer: 'Three calls.' }))
    act(() => toggleQuestionPin('case-a', history.result.current[0].id))
    expect(history.result.current[0].pinned).toBe(true)
    act(() => renameQuestionHistory('case-a', history.result.current[0].id, 'Priority caller pattern'))
    expect(history.result.current[0].label).toBe('Priority caller pattern')
    expect(history.result.current[0].query).toBe('Who called this number?')
    expect(JSON.parse(globalThis.localStorage.getItem('nexusai.viewer.questions.case-a'))[0].label).toBe('Priority caller pattern')
  })

  it('continues in memory when storage throws', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked') })
    const draft = renderHook(() => useQuestionDraft('case-storage-error'))
    act(() => draft.result.current[1]('Draft remains usable'))
    expect(draft.result.current[0]).toBe('Draft remains usable')
    const history = renderHook(() => useQuestionHistory('case-storage-error'))
    act(() => recordQuestionHistory('case-storage-error', 'Question', { state: 'clarify', clarification: { title: 'One detail needed' } }))
    expect(history.result.current).toHaveLength(1)
  })

  it('clears only case-scoped session state before a case switch', () => {
    globalThis.sessionStorage.setItem('nexusai.case.case-a.scope.evidence', 'cdr')
    globalThis.sessionStorage.setItem('unrelated.preference', 'keep')
    clearCaseSessionScope()
    expect(globalThis.sessionStorage.getItem('nexusai.case.case-a.scope.evidence')).toBeNull()
    expect(globalThis.sessionStorage.getItem('unrelated.preference')).toBe('keep')
  })
})
