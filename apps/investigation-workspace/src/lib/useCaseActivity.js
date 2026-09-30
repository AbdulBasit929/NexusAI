import { useEffect, useMemo, useState } from 'react'
import { runCaseTemplate } from './apiClient.js'
import { mergeActivity, parseActivity } from './caseActivity.js'

// In words an analyst can act on: why a case's activity could not be read. No status codes without a sentence.
export function activityFailureText(reason) {
  if (reason === 'no_rows') return 'The service answered without activity rows.'
  if (reason === 403) return 'You do not have access to this case’s activity.'
  if (reason === 404) return 'This case was not found.'
  if (typeof reason === 'number') return `The service returned an error (HTTP ${reason}).`
  return 'The service could not be reached.'
}

// Reads whole-case activity for the given cases in parallel and merges it. A case that cannot be read is named with the
// reason and left out, never drawn as zero. Refetches when the case list or `refreshToken` changes; in-flight reads
// are aborted.
export function useCaseActivity(caseIds, { refreshToken = 0, fetchImpl } = {}) {
  const key = caseIds.join('|')
  const [state, setState] = useState({ status: 'loading', results: [], failures: [] })

  useEffect(() => {
    if (!caseIds.length) {
      setState({ status: 'ready', results: [], failures: [] })
      return undefined
    }
    const controller = new AbortController()
    setState(current => ({ ...current, status: current.results.length ? 'refreshing' : 'loading' }))
    Promise.allSettled(caseIds.map(caseId => runCaseTemplate({ caseId, template: 'activity_by_day', signal: controller.signal, fetchImpl }).then(response => parseActivity(response, caseId))))
      .then(settled => {
        if (controller.signal.aborted) return
        const results = []
        const failures = []
        settled.forEach((outcome, index) => {
          if (outcome.status === 'fulfilled' && outcome.value.available) results.push(outcome.value)
          else failures.push({ caseId: caseIds[index], reason: outcome.status === 'fulfilled' ? outcome.value.reason : (outcome.reason?.status ?? 'unreachable') })
        })
        setState({ status: 'ready', results, failures })
      })
    return () => controller.abort()
    // The joined key stands in for the array so an equal list does not refetch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, refreshToken, fetchImpl])

  const activity = useMemo(() => mergeActivity(state.results), [state.results])
  return { status: state.status, activity, failures: state.failures, failed: state.failures.map(failure => failure.caseId), readCases: state.results.map(result => result.caseId) }
}
