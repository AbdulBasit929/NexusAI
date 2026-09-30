const stateCatalog = {
  results_present: { label: 'Results found', title: 'Findings are ready', description: '', tone: 'complete', icon: 'fa-circle-check' },
  complete_zero_results: { label: 'Processing complete', title: 'No detections found', description: 'Processing completed. No detections were found.', tone: 'empty', icon: 'fa-circle-check' },
  no_match_for_filter: { label: 'Filter complete', title: 'No matching results', description: 'No results matched this filter.', tone: 'empty', icon: 'fa-filter-circle-xmark' },
  processing: { label: 'Processing', title: 'Analysis in progress', description: 'Processing is still in progress.', tone: 'processing', icon: 'fa-spinner' },
  not_processed: { label: 'Not processed', title: 'Analysis not yet run', description: 'This analysis has not been run yet.', tone: 'processing', icon: 'fa-clock' },
  failed: { label: 'Processing failed', title: 'Analysis could not be completed', description: 'Processing failed.', tone: 'failed', icon: 'fa-triangle-exclamation' },
  unavailable: { label: 'Unavailable', title: 'Capability unavailable', description: 'This capability is not currently available.', tone: 'unavailable', icon: 'fa-circle-minus' },
  unauthorized: { label: 'Access restricted', title: 'Result unavailable', description: 'You do not have access to this result.', tone: 'failed', icon: 'fa-lock' },
  invalid_request: { label: 'Question needs detail', title: 'One detail is needed', description: 'Add the missing or invalid detail, then try again.', tone: 'input', icon: 'fa-circle-question' },
}

const aliases = {
  answered: 'results_present',
  answered_with_limitations: 'results_present',
  partial_analysis: 'results_present',
  no_results: 'no_match_for_filter',
  needs_input: 'invalid_request',
  unsupported: 'unavailable',
  capability_unavailable: 'unavailable',
  data_unavailable: 'not_processed',
  processing_incomplete: 'processing',
  execution_failed: 'failed',
}

function finiteValue(value) {
  if (value === null || value === undefined || value === '') return null
  const number = Number(value)
  return Number.isFinite(number) ? number : null
}

function leafName(value) {
  return String(value || '').split(/[\\/]/).pop().trim()
}

function looksTechnical(value) {
  const text = String(value || '').trim()
  return !text || /^[0-9a-f]{8}-[0-9a-f-]{27}$/i.test(text) || /^(?:nexusai|forensic):\/\//i.test(text)
}

export function analystResultState(value) {
  const requested = String(value || 'results_present').trim().toLowerCase()
  const id = aliases[requested] || requested
  return { id, ...(stateCatalog[id] || { label: 'Result needs review', title: 'Result state not reported', description: 'Review the available evidence before drawing a conclusion.', tone: 'unavailable', icon: 'fa-circle-question' }) }
}

const explicitForensicTarget = /["'“”‘’]|\b(?:find|search|lookup|where\s+was|show\s+frequent|contains?|exact|evidence\s+[0-9a-f-]{8,}|plate\s+[a-z0-9-]+|\d{7,})\b/i
const forensicFollowUp = /(?:\b(?:after\s+that|before\s+that|what\s+about|how\s+about|only\s+(?:incoming|outgoing)|same\s+(?:source|target|period)|show\s+(?:its|that|the)\s+source|when\s+(?:was\s+that|did\s+(?:they|he|she)\s+say\s+(?:that|it))|where\s+was\s+that|those\s+results?|them|it)\b|(?:انہوں\s+نے|اس\s+نے).*?(?:یہ|وہ).*?کب.*?کہا)/i

// Prior answer context is an analyst convenience, never an authority source.
// Keep it only for an explicit anaphoric follow-up in the identical portal scope;
// a new quoted term, identifier, plate, evidence ID, or lookup must plan afresh.
export function analystShouldInheritConversationContext(question) {
  const value = String(question || '').trim()
  return Boolean(value) && !explicitForensicTarget.test(value) && forensicFollowUp.test(value)
}

export function analystScopedConversationContext(messages, conversationId, requestScope, question) {
  if (!analystShouldInheritConversationContext(question) || !requestScope) return null
  for (let index = (messages || []).length - 1; index >= 0; index -= 1) {
    const message = messages[index]
    if (message?.sender !== 'agent' || message?.metadata?.request_scope !== requestScope) continue
    const context = message?.metadata?.presentation?.context
    if (context && typeof context === 'object' && (context.target || context.template)) {
      return {
        ...context,
        conversation_id: context.conversation_id || String(conversationId || ''),
        analysis_id: context.analysis_id || String(message?.metadata?.analysis_id || ''),
        turn_id: context.turn_id || String(message?.message_id || message?.id || ''),
      }
    }
  }
  return null
}

export function analystCitation(citation, index = 0) {
  const item = citation && typeof citation === 'object' ? citation : { label: citation }
  const rawLabel = leafName(item.source_file || item.locator?.source_file || item.label || item.source_entry)
  const label = looksTechnical(rawLabel) ? `Evidence source ${index + 1}` : rawLabel
  const locator = item.locator || item.citation_locator || {}
  const timestamp = finiteValue(locator.timestamp_seconds ?? locator.start_seconds ?? item.timestamp_seconds)
  const page = finiteValue(locator.page ?? locator.page_number ?? item.page ?? item.page_number)
  const passage = finiteValue(locator.passage ?? locator.passage_number ?? item.passage ?? item.passage_number)
  const row = locator.row ?? locator.row_number ?? item.row ?? item.row_number
  const explicitDetail = typeof item.detail === 'string' ? item.detail.trim() : ''
  const detail = timestamp !== null
    ? `${formatSourceSeconds(timestamp)} in source`
    : page !== null
      ? `page ${page}${passage !== null ? `, passage ${passage}` : ''}`
      : row !== null && row !== undefined && row !== ''
        ? `row ${row}`
        : looksTechnical(explicitDetail) ? 'Source-level reference' : explicitDetail
  return { label, detail }
}

export function analystSourceNavigationParams(citation, evidenceId) {
  const item = citation && typeof citation === 'object' ? citation : {}
  const locator = item.locator && typeof item.locator === 'object'
    ? item.locator
    : item.citation_locator && typeof item.citation_locator === 'object'
      ? item.citation_locator
      : {}
  const params = new URLSearchParams()
  if (evidenceId) params.set('evidence', evidenceId)

  const sourceTime = finiteValue(locator.timestamp_seconds ?? locator.start_seconds ?? item.timestamp_seconds ?? item.start_seconds)
  const page = finiteValue(locator.page ?? locator.page_number ?? item.page ?? item.page_number)
  const row = locator.row ?? locator.row_number ?? item.row ?? item.row_number
  const finding = locator.finding_id ?? locator.artifact_id ?? item.finding_id ?? item.artifact_id

  if (sourceTime !== null) params.set('source_time', String(sourceTime))
  if (page !== null) params.set('page', String(page))
  if (row !== null && row !== undefined && row !== '') params.set('row', String(row))
  if (finding !== null && finding !== undefined && finding !== '') params.set('finding', String(finding))
  return params
}

export function analystSourceNavigationContext(search) {
  const params = search instanceof URLSearchParams ? search : new URLSearchParams(search || '')
  const sourceTime = finiteValue(params.get('source_time'))
  const page = finiteValue(params.get('page'))
  const row = String(params.get('row') || '').trim()
  const finding = String(params.get('finding') || '').trim()
  const items = []
  if (sourceTime !== null) items.push({ kind: 'time', label: 'Source time', value: formatSourceSeconds(sourceTime) })
  if (page !== null) items.push({ kind: 'page', label: 'Document location', value: `Page ${page}` })
  if (row) items.push({ kind: 'row', label: 'Structured reference', value: `Row ${row}` })
  if (finding) items.push({ kind: 'finding', label: 'Finding reference', value: finding })
  return items
}

export function formatSourceSeconds(value) {
  const number = finiteValue(value)
  if (number === null) return ''
  const minutes = Math.floor(number / 60)
  const seconds = number - minutes * 60
  if (minutes === 0) return `${Number.isInteger(seconds) ? seconds : seconds.toFixed(2)} seconds`
  return `${String(minutes).padStart(2, '0')}:${String(Math.floor(seconds)).padStart(2, '0')}`
}

export function analystSourceTimeRange(view) {
  const candidates = [
    view?.parameters,
    view?.execution_plan?.parameters,
    view?.trace?.parameters,
    view?.trace?.execution_plan?.parameters,
    view?.fact_packet?.time_window,
  ].filter(Boolean)
  for (const parameters of candidates) {
    const start = finiteValue(parameters.start_seconds ?? parameters.source_start_seconds)
    const end = finiteValue(parameters.end_seconds ?? parameters.source_end_seconds)
    if (start === null && end === null) continue
    const label = start !== null && end !== null
      ? `${formatSourceSeconds(start)} to ${formatSourceSeconds(end)}`
      : start !== null ? `from ${formatSourceSeconds(start)}` : `through ${formatSourceSeconds(end)}`
    return { start, end, label }
  }
  return null
}

export function analystAskScope(workspace, evidence, detail) {
  const workspaceLabel = String(workspace?.displayName || workspace?.display_name || 'Current workspace').trim()
  if (!evidence) return { kind: 'workspace', label: workspaceLabel, detail: 'Entire workspace' }
  const source = leafName(evidence.original_filename || evidence.source_file || detail?.item?.original_filename || detail?.item?.source_file)
  const artifacts = Array.isArray(detail?.derived_artifacts) ? detail.derived_artifacts : []
  const resultFamilies = [...new Set(artifacts
    .filter(item => String(item?.processing_status || 'completed').toLowerCase() === 'completed')
    .map(item => String(item?.artifact_type || '').trim())
    .filter(Boolean))]
  const versionId = String(evidence.current_version_id || detail?.item?.current_version_id || artifacts[0]?.version_id || '').trim()
  return {
    kind: 'evidence',
    label: source && !looksTechnical(source) ? source : 'Current evidence',
    detail: workspaceLabel,
    evidenceId: evidence.evidence_id || '',
    evidenceVersionId: versionId,
    sourceFamily: String(evidence.modality || evidence.detected_type || detail?.item?.modality || '').trim(),
    availableResultFamilies: resultFamilies,
  }
}
