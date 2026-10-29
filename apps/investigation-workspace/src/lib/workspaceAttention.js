import { useEffect, useState } from 'react'
import { configuredCaseIds, getCaseOverview } from './apiClient.js'
import { summariseCase } from './useCaseOverview.js'

// The truthful Activity disclosure reads the same collection-status responses
// as the Dashboard. One shared, briefly cached fetch keeps every route from
// re-requesting every case only to report active processing.
const TTL_MS = 30_000
const cache = new Map()

function overviewFor(caseId) {
  const hit = cache.get(caseId)
  if (hit && Date.now() - hit.at < TTL_MS) return hit.promise
  const promise = getCaseOverview({ caseId }).then(data => summariseCase(data))
  cache.set(caseId, { at: Date.now(), promise })
  promise.catch(() => cache.delete(caseId))
  return promise
}

// Tests (and any future explicit refresh) start from a clean cache.
export function resetAttentionCache() { cache.clear() }

export function countAttention(summaries) {
  const known = summaries.filter(Boolean)
  return {
    reported: known.length,
    needReview: known.filter(item => item.processingState === 'attention').length,
    processing: known.filter(item => item.processingState === 'processing').length,
  }
}

export function useWorkspaceAttention() {
  const [state, setState] = useState({ reported: 0, needReview: 0, processing: 0, cases: [], settled: false })
  const key = configuredCaseIds().join('\u001f')
  useEffect(() => {
    let active = true
    const ids = configuredCaseIds()
    Promise.all(ids.map(caseId => overviewFor(caseId).catch(() => null)))
      .then(summaries => { if (active) setState({ ...countAttention(summaries), settled: true, cases: ids.map((caseId, index) => ({ caseId, summary: summaries[index] })) }) })
    return () => { active = false }
  }, [key])
  return state
}
