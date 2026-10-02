import { useSyncExternalStore } from 'react'

let entries = []
const listeners = new Set()

function emit() {
  for (const listener of listeners) listener()
}

function subscribe(listener) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

function snapshot() {
  return entries
}

function familyFor(response) {
  return String(response?.query_plan?.applied_filters?.record_type
    || response?.enterprise?.fact_packet?.rows?.[0]?.record_type
    || 'all evidence')
}

function traceFor(response) {
  const plan = response?.planner?.semantic_operation_planner?.dynamic_plan
  return {
    requestReference: String(response?.telemetry?.request_id || response?.request_id || ''),
    planState: String(response?.planner?.semantic_operation_planner?.state || ''),
    groupedBy: Array.isArray(plan?.group_fields) ? plan.group_fields : [],
    measuredBy: Array.isArray(plan?.measures)
      ? plan.measures.map(measure => [measure?.op, measure?.field_id].filter(Boolean).join(' ')).filter(Boolean)
      : [],
  }
}

export function recordSessionActivity({ caseId, query, presentation }) {
  if (!query || !presentation) return
  const response = presentation.original || null
  const clarification = presentation.clarification
  const family = familyFor(response)
  const scope = [
    { label: 'Case', value: caseId },
    { label: 'Evidence family', value: ['cdr', 'ipdr', 'anpr'].includes(family.toLowerCase()) ? family.toUpperCase() : family },
    ...(Array.isArray(presentation.scope) ? presentation.scope : []),
  ]
  const item = {
    id: globalThis.crypto?.randomUUID?.() || `${Date.now()}-${entries.length}`,
    caseId,
    kind: 'analysis',
    query,
    state: presentation.state,
    family,
    title: query,
    answer: clarification
      ? `${clarification.title}. ${clarification.question}`
      : presentation.answer,
    answerWithheld: Boolean(clarification) || ['unsupported', 'failed', 'processing'].includes(presentation.state),
    scope,
    trace: traceFor(response),
    recordedAt: new Date().toISOString(),
  }
  entries = [item, ...entries].slice(0, 100)
  emit()
}

export function useSessionActivity(caseId) {
  const all = useSyncExternalStore(subscribe, snapshot, snapshot)
  return all.filter(item => item.caseId === caseId)
}

export function clearSessionActivityForTests() {
  entries = []
  emit()
}

// Every question asked in this tab, across cases (the workspace-level activity page).
export function useAllSessionActivity() {
  return useSyncExternalStore(subscribe, snapshot, snapshot)
}
