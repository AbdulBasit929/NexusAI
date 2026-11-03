import { useCallback, useRef, useState } from 'react'
import { askCase } from '../../../lib/apiClient.js'
import { presentInvestigationResponse } from '../../../lib/investigationPresentation.js'
import { recordSessionActivity } from '../../../lib/sessionActivity.js'
import { recordQuestionHistory } from '../../../lib/workspaceState.js'

let sequence = 0
const nextId = () => { sequence += 1; return `turn-${sequence}` }

// The conversation of one case, held in the page: a question, what came back, and the question that answer suggests. One request runs at a time
// (the language model on this laptop serves one), and starting another stops the one that is running. The thread does not cancel when the analyst
// opens the evidence beside it. When the conversation service exists the same shape is read from it; nothing here depends on it.
export function useAskThread(caseId) {
  const [turns, setTurns] = useState([])
  const controller = useRef(null)
  const busy = turns.some(turn => turn.busy)

  const update = useCallback((id, patch) => setTurns(items => items.map(turn => (turn.id === id ? { ...turn, ...patch } : turn))), [])

  const run = useCallback(async (id, query, scope) => {
    controller.current?.abort()
    controller.current = new AbortController()
    try {
      const response = await askCase({ caseId, query, recordType: scope?.recordType, signal: controller.current.signal })
      // the question travels with its answer: the picture of an answer (the workbench) reads the numbers of the question the engine's own checks tied the query to
      const presentation = { ...presentInvestigationResponse(response, { caseId }), question: query }
      update(id, { presentation, busy: false, finishedAt: Date.now() })
      recordSessionActivity({ caseId, query, presentation })
      recordQuestionHistory(caseId, query, presentation)
    } catch (error) {
      if (error.name === 'AbortError') { update(id, { busy: false, aborted: true }); return }
      const presentation = presentInvestigationResponse(null, { caseId, error })
      update(id, { presentation, busy: false, finishedAt: Date.now() })
      recordSessionActivity({ caseId, query, presentation })
      recordQuestionHistory(caseId, query, presentation)
    }
  }, [caseId, update])

  const ask = useCallback((query, { scope, parentId = null, label = '' } = {}) => {
    const id = nextId()
    setTurns(items => [...items, { id, query, label, parentId, scope, presentation: null, busy: true, startedAt: Date.now() }])
    return run(id, query, scope)
  }, [run])

  const retry = useCallback(turn => {
    update(turn.id, { busy: true, presentation: null, aborted: false, startedAt: Date.now() })
    return run(turn.id, turn.query, turn.scope)
  }, [run, update])

  const stop = useCallback(() => controller.current?.abort(), [])
  const reset = useCallback(() => { controller.current?.abort(); setTurns([]) }, [])

  return { turns, busy, ask, retry, stop, reset }
}
