import { useCallback, useEffect, useRef, useState } from 'react'
import { ConversationError, askTurn, createConversation, forkTurn, getConversation, getTurn, pinTurn, rerunTurn, sendFeedback } from '../../../lib/conversationsApi.js'
import { presentInvestigationResponse } from '../../../lib/investigationPresentation.js'
import { recordSessionActivity } from '../../../lib/sessionActivity.js'

// The conversation of one case, kept by the conversation service (the case layer): every question is a turn that is stored, can be run again against the data as it is now (with a
// comparison), forked, given feedback and pinned to the findings. This hook has the same shape as useAskThread (turns, busy, ask, retry, stop, reset) and adds what the service gives.
// A conversation is a tree; the page draws the path from the first question to the newest turn. A question runs to the end on the server even when this page is closed, so "stop" here
// means "stop waiting": the turn stays in the history and shows its answer when the conversation is opened again.
let sequence = 0
const requestId = () => { sequence += 1; return `ui-${Date.now().toString(36)}-${sequence.toString(36)}-${Math.random().toString(36).slice(2, 8)}` }
const RUNNING = new Set(['running', 'queued', 'accepted'])
const stamp = value => { const time = value ? Date.parse(value) : NaN; return Number.isFinite(time) ? time : null }

// One stored turn in the shape the Ask page draws. The answer is exactly what POST /query/hybrid returns, so the page's own presentation code reads it.
export function toUiTurn(stored, caseId) {
  const running = RUNNING.has(stored.state)
  const failed = stored.state === 'failed' || stored.state === 'interrupted'
  let presentation = null
  if (stored.answer) presentation = { ...presentInvestigationResponse(stored.answer, { caseId }), question: stored.question }
  else if (failed) {
    const error = Object.assign(new Error(stored.error?.message || (stored.state === 'interrupted' ? 'The server stopped while this question ran.' : 'The question could not be completed.')), { reference: String(stored.turn_id || '').slice(0, 8) })
    presentation = presentInvestigationResponse(null, { caseId, error })
  }
  return {
    id: stored.turn_id,
    stored: true,
    query: stored.question,
    label: '',
    parentId: null,                                   // "following your choice" is a clarification; a continuation is not one
    scope: null,
    presentation,
    busy: running,
    startedAt: stamp(stored.started_at) || stamp(stored.created_at),
    finishedAt: stamp(stored.completed_at),
    kind: stored.kind || 'ask',                       // ask, edit, rerun
    outcome: stored.outcome || '',
    reruns: stored.reruns || { count: 0, latest_changed: null },
    interpreted: stored.resolved_question || '',
    snapshotState: stored.snapshot_state || '',
    answerMissing: !stored.answer && stored.state === 'completed',   // not stored, or the answer is loaded on demand
    feedback: stored.feedback || null,
    pinnedAs: stored.pinned_as || [],
    reproduce: { engine: stored.engine_version || '', model: stored.model || '', dataVersion: stored.data_version_hash || '', query: stored.query || '', queryHash: stored.query_hash || '' },
    diff: null,
    phase: null,
    serviceMessage: stored.error?.message || '',
  }
}

// The turns from the first question to the newest one: walk up from the active turn, so a question that was edited or run again shows the newest version and not both.
export function pathOf(thread) {
  const turns = thread?.turns || []
  const byId = new Map(turns.map(turn => [turn.turn_id, turn]))
  const path = []
  let current = byId.get(thread?.active_turn_id) || turns.at(-1)
  const seen = new Set()
  while (current && !seen.has(current.turn_id)) {
    seen.add(current.turn_id)
    path.unshift(current)
    current = current.parent_turn_id ? byId.get(current.parent_turn_id) : null
  }
  return path
}

export function useConversationThread(caseId, { enabled = true, onChanged } = {}) {
  const [turns, setTurns] = useState([])
  const [threadId, setThreadId] = useState(null)
  const [thread, setThread] = useState(null)        // the conversation record: title, visibility, owner, forked_from
  const [opening, setOpening] = useState(false)
  const [problem, setProblem] = useState('')
  const controller = useRef(null)
  const current = useRef(null)
  const busy = turns.some(turn => turn.busy)

  const patch = useCallback((id, change) => setTurns(items => items.map(turn => (turn.id === id ? { ...turn, ...(typeof change === 'function' ? change(turn) : change) } : turn))), [])
  const replace = useCallback((id, next) => setTurns(items => items.map(turn => (turn.id === id ? next : turn))), [])

  // narration: only phases the service reports, with a running clock from its heartbeat
  const narrate = useCallback(id => event => {
    if (event.event === 'phase') patch(id, { serverTurnId: event.turn_id || undefined, phase: { name: event.phase, label: event.label || '', position: event.position ?? null, at: Date.now() } })
    else if (event.event === 'heartbeat') patch(id, turn => ({ phase: { ...(turn.phase || { name: 'engine', label: 'Answering' }), elapsedMs: event.elapsed_ms } }))
  }, [patch])

  const loadAnswers = useCallback(async (list, signal) => {
    for (const turn of [...list].reverse()) {
      if (signal?.aborted) return
      if (turn.presentation || !turn.answerMissing || turn.busy) continue
      try {
        const full = await getTurn({ caseId, turnId: turn.id, signal })
        if (!signal?.aborted) replace(turn.id, { ...toUiTurn(full, caseId), diff: turn.diff })
      } catch { /* the turn stays as it is; its card offers "Run again" */ }
    }
  }, [caseId, replace])

  const open = useCallback(async id => {
    controller.current?.abort()
    const gate = new AbortController()
    controller.current = gate
    setOpening(true)
    setProblem('')
    try {
      const stored = await getConversation({ caseId, threadId: id, signal: gate.signal })
      const list = pathOf(stored).map(turn => toUiTurn(turn, caseId))
      current.current = stored.thread_id
      setThreadId(stored.thread_id)
      setThread(stored)
      setTurns(list)
      setOpening(false)
      loadAnswers(list, gate.signal)
    } catch (error) {
      if (error.name === 'AbortError') return
      setOpening(false)
      setProblem(error instanceof ConversationError ? error.message : 'The conversation could not be opened.')
    }
  }, [caseId, loadAnswers])

  const ask = useCallback(async (query, { parentTurnId, editOfTurnId } = {}) => {
    if (!enabled) return undefined
    controller.current?.abort()
    const gate = new AbortController()
    controller.current = gate
    const id = `pending-${requestId()}`
    setTurns(items => [...items, { id, stored: false, query, label: '', parentId: null, scope: null, presentation: null, busy: true, startedAt: Date.now(), kind: editOfTurnId ? 'edit' : 'ask', phase: { name: 'sending', label: 'Sending the question', at: Date.now() }, reruns: { count: 0 }, pinnedAs: [] }])
    setProblem('')
    try {
      if (!current.current) {
        const created = await createConversation({ caseId, signal: gate.signal })
        current.current = created.thread_id
        setThreadId(created.thread_id)
        setThread(created)
      }
      const { turn } = await askTurn({ caseId, threadId: current.current, question: query, clientRequestId: requestId(), parentTurnId, editOfTurnId, signal: gate.signal, onEvent: narrate(id) })
      const next = toUiTurn(turn, caseId)
      replace(id, next)
      if (next.presentation) recordSessionActivity({ caseId, query, presentation: next.presentation })
      onChanged?.()
      return next
    } catch (error) {
      if (error.name === 'AbortError') { patch(id, { busy: false, aborted: true, stoppedWaiting: true }); onChanged?.(); return undefined }
      if (error.turn) { replace(id, toUiTurn(error.turn, caseId)); onChanged?.(); return undefined }
      const presentation = presentInvestigationResponse(null, { caseId, error: Object.assign(new Error(error.message), { reference: error.requestId || `HTTP-${error.status || 0}` }) })
      patch(id, { presentation, busy: false, finishedAt: Date.now(), serviceMessage: error instanceof ConversationError ? error.message : '' })
      return undefined
    }
  }, [caseId, enabled, narrate, onChanged, patch, replace])

  const retry = useCallback(turn => ask(turn.query), [ask])

  // The same question again, against the data as it is now. The new run takes the old one's place and carries the comparison.
  const rerun = useCallback(async turn => {
    if (!enabled || !turn.stored) return undefined
    controller.current?.abort()
    const gate = new AbortController()
    controller.current = gate
    patch(turn.id, { busy: true, rerunning: true, startedAt: Date.now(), phase: { name: 'sending', label: 'Asking again', at: Date.now() } })
    try {
      const { turn: stored, diff } = await rerunTurn({ caseId, turnId: turn.id, signal: gate.signal, onEvent: narrate(turn.id) })
      const next = { ...toUiTurn(stored, caseId), diff, previousTurnId: turn.id }
      replace(turn.id, next)
      onChanged?.()
      return next
    } catch (error) {
      if (error.name === 'AbortError') { patch(turn.id, { busy: false, rerunning: false, aborted: true, stoppedWaiting: true }); return undefined }
      patch(turn.id, { busy: false, rerunning: false, serviceMessage: error instanceof ConversationError ? error.message : 'The question could not be asked again.' })
      return undefined
    }
  }, [caseId, enabled, narrate, onChanged, patch, replace])

  const fork = useCallback(async (turn, title) => {
    const created = await forkTurn({ caseId, turnId: turn.id, title })
    controller.current?.abort()
    current.current = created.thread_id
    setThreadId(created.thread_id)
    setThread(created)
    setTurns([])
    onChanged?.()
    return created
  }, [caseId, onChanged])

  const feedback = useCallback(async (turn, verdict, note) => {
    const result = await sendFeedback({ caseId, turnId: turn.id, verdict, note })
    patch(turn.id, { feedback: result })
    return result
  }, [caseId, patch])

  const pin = useCallback(async (turn, details = {}) => {
    const result = await pinTurn({ caseId, turnId: turn.id, ...details })
    patch(turn.id, current => ({ pinnedAs: [...new Set([...(current.pinnedAs || []), result.finding?.finding_id].filter(Boolean))] }))
    return result
  }, [caseId, patch])

  const stop = useCallback(() => controller.current?.abort(), [])
  const reset = useCallback(() => {
    controller.current?.abort()
    current.current = null
    setThreadId(null)
    setThread(null)
    setTurns([])
    setProblem('')
  }, [])

  useEffect(() => () => controller.current?.abort(), [])

  return { turns, busy, ask, retry, stop, reset, rerun, fork, feedback, pin, open, threadId, thread, opening, problem }
}
