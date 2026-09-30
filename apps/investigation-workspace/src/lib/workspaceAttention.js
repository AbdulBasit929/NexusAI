import { useEffect, useState } from 'react'
import { configuredCaseIds, getCaseOverview } from './apiClient.js'
import { summariseCase } from './useCaseOverview.js'

// Live sidebar badges come from the same collection-status responses the
// Dashboard reads. One shared, briefly cached fetch keeps every route from
// re-requesting every case just to draw a badge.
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

const WEIGHT = { attention: 0, processing: 1, 'not-processed': 2, complete: 3, loading: 4, unreported: 5 }

// Every configured case is listed from the first paint. Until its status arrives it is "loading"; if the
// status call failed it is "unreported". Neither is ever shown as healthy.
export function withPending(caseIds, loaded, settled) {
  return caseIds.map(caseId => loaded.find(entry => entry.caseId === caseId) || { caseId, summary: null, pending: !settled })
}

// The sidebar's case list: the active case first for context, then cases that need the analyst, then the
// rest in configured order.
export function sidebarCases(entries, activeCaseId, limit = 6) {
  return entries
    .map((entry, index) => ({ caseId: entry.caseId, state: entry.summary?.processingState || (entry.pending ? 'loading' : 'unreported'), index }))
    .sort((left, right) => (right.caseId === activeCaseId) - (left.caseId === activeCaseId) || WEIGHT[left.state] - WEIGHT[right.state] || left.index - right.index)
    .slice(0, limit)
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
