import { useCallback, useEffect, useState } from 'react'
import { getCaseOverview } from './apiClient.js'

// The collection-status fetch, extracted so more than one page can show real
// case readiness instead of a bare list of identifiers.
//
// Every consumer gets the same three states and the same derived summary, so a
// card on the dashboard and the case overview page can never disagree about
// what "ready" means.
export function useCaseOverview(caseId) {
  const [state, setState] = useState({ loading: true })
  useEffect(() => {
    if (!caseId) {
      setState({ loading: false })
      return undefined
    }
    const controller = new AbortController()
    setState({ loading: true })
    getCaseOverview({ caseId, signal: controller.signal })
      .then(data => setState({ data, loading: false }))
      .catch(error => error.name !== 'AbortError' && setState({ error, loading: false }))
    return () => controller.abort()
  }, [caseId])
  return state
}

// Load the configured collection directory as one abortable unit. Dashboard
// and Cases intentionally present different tasks, but the evidence readiness
// behind both views must come from the same request lifecycle.
export function useConfiguredCaseOverviews(caseIds) {
  const key = caseIds.join('\u001f')
  const [states, setStates] = useState({})
  const [generation, setGeneration] = useState(0)
  const reload = useCallback(() => setGeneration(value => value + 1), [])

  useEffect(() => {
    const controllers = new Map()
    setStates(current => Object.fromEntries(caseIds.map(caseId => [caseId, {
      ...current[caseId],
      loading: !current[caseId]?.data,
      refreshing: Boolean(current[caseId]?.data),
      error: null,
    }])))

    for (const caseId of caseIds) {
      const controller = new AbortController()
      controllers.set(caseId, controller)
      getCaseOverview({ caseId, signal: controller.signal })
        .then(data => setStates(current => ({ ...current, [caseId]: { data, loading: false, refreshing: false, receivedAt: new Date().toISOString() } })))
        .catch(error => {
          if (error.name === 'AbortError') return
          setStates(current => ({ ...current, [caseId]: { ...current[caseId], error, loading: false, refreshing: false } }))
        })
    }

    return () => { for (const controller of controllers.values()) controller.abort() }
  }, [key, generation]) // eslint-disable-line react-hooks/exhaustive-deps

  return { states, reload }
}

// A single derived shape, so "ready", "processing" and "families" mean exactly
// one thing across the product.
export function summariseCase(data) {
  const summary = data?.summary || {}
  const families = data?.record_families || []
  const total = Number(summary.evidence_total || 0)
  const ready = Number(summary.evidence_completed || 0)
  const inFlight = Number(summary.evidence_in_flight || 0)
  const failed = Object.prototype.hasOwnProperty.call(summary, 'evidence_failed') ? Number(summary.evidence_failed || 0) : null
  const missingAssets = Number(summary.completed_jobs_missing_kb_asset || 0)
  return {
    total,
    ready,
    inFlight,
    failed,
    missingAssets,
    rejectedRows: Number(summary.rejected_rows || 0),
    duplicateRows: Number(summary.duplicate_rows || 0),
    jobs: Number(summary.jobs_total || 0),
    acceptedRows: Number(summary.accepted_rows || 0),
    families,
    // `not-processed` is distinct from `processing`: nothing has been ingested
    // at all, which is a different thing to say to an analyst than "not ready
    // yet".
    processingState: failed > 0 || missingAssets > 0 ? 'attention' : inFlight > 0 ? 'processing' : total > 0 ? 'complete' : 'not-processed',
  }
}
