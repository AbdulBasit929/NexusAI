import { useEffect, useMemo, useState } from 'react'
import { runCaseTemplate } from './apiClient.js'
import { mergeActivity, parseActivity } from './caseActivity.js'

// Reads whole-case activity for the given cases in parallel and merges it. A case that cannot be read is named and
// left out, never drawn as zero. Refetches when the case list or `refreshToken` changes; in-flight reads are aborted.
export function useCaseActivity(caseIds, { refreshToken = 0, fetchImpl } = {}) {
  const key = caseIds.join('|')
  const [state, setState] = useState({ status: 'loading', results: [], failed: [] })

  useEffect(() => {
    if (!caseIds.length) {
      setState({ status: 'ready', results: [], failed: [] })
      return undefined
    }
    const controller = new AbortController()
    setState(current => ({ ...current, status: current.results.length ? 'refreshing' : 'loading' }))
    Promise.allSettled(caseIds.map(caseId => runCaseTemplate({ caseId, template: 'activity_by_day', signal: controller.signal, fetchImpl }).then(response => parseActivity(response, caseId))))
      .then(settled => {
        if (controller.signal.aborted) return
        const results = []
        const failed = []
        settled.forEach((outcome, index) => {
          if (outcome.status === 'fulfilled' && outcome.value.available) results.push(outcome.value)
          else failed.push(caseIds[index])
        })
        setState({ status: 'ready', results, failed })
      })
    return () => controller.abort()
    // The joined key stands in for the array so an equal list does not refetch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, refreshToken, fetchImpl])

  const activity = useMemo(() => mergeActivity(state.results), [state.results])
  return { status: state.status, activity, failed: state.failed, readCases: state.results.map(result => result.caseId) }
}
