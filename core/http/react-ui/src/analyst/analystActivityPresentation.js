import { analystCitation, analystResultState, analystSourceNavigationParams } from './analystAskPresentation.js'
import { evidenceTypePresentation } from './analystDataPresentation.js'
import { citationEvidenceId, friendlySourceName } from './analystPresentation.js'

export function activityPresentationFor(item) {
  const value = item?.metadata?.presentation
  const presentation = value && typeof value === 'object' && !Array.isArray(value) ? value : {}
  return {
    ...presentation,
    operation_id: typeof presentation.operation_id === 'string' ? presentation.operation_id : '',
    metrics: (Array.isArray(presentation.metrics) ? presentation.metrics : []).filter(metric => metric && typeof metric.label === 'string'),
    limitations: (Array.isArray(presentation.limitations) ? presentation.limitations : []).filter(value => typeof value === 'string'),
    table: activityTable(presentation.table, presentation.operation_id),
  }
}

export function activityTable(value, operation = '') {
  const table = value && typeof value === 'object' && !Array.isArray(value) ? value : {}
  const rows = (Array.isArray(table.rows) ? table.rows : []).filter(row => row && typeof row === 'object' && !Array.isArray(row)).slice(0, 20)
  const keys = values => [...new Set((Array.isArray(values) ? values : []).filter(key => typeof key === 'string' && key.trim()))].slice(0, 20)
  const video = operation === 'video.anpr_grouped_timeline' || /plate readings|video.*anpr/i.test(typeof table.title === 'string' ? table.title : '')
  const malformedVideo = video && rows.some(row => !row.normalized_plate_text && !row.raw_plate_text)
  const missingRows = table.rows != null && (!Array.isArray(table.rows) || rows.length !== table.rows.slice(0, 20).length)
  let columns = keys(table.columns)
  if (!columns.length && !malformedVideo) columns = keys(rows.flatMap(row => Object.keys(row).filter(key => row[key] == null || ['string', 'number', 'boolean'].includes(typeof row[key]))))
  const unavailable = malformedVideo || missingRows || (rows.length > 0 && !columns.length)
  return { ...table, title: typeof table.title === 'string' ? table.title : 'Results', columns, priority_columns: keys(table.priority_columns), rows: unavailable ? [] : rows, unavailable }
}

// Technical payloads remain in the disclosure, never implicit object strings.
export function activityCell(value) {
  if (value == null || value === '') return '—'
  if (typeof value === 'number') return Number.isFinite(value) ? String(value) : '—'
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  if (typeof value === 'object') {
    if (Array.isArray(value)) return value.map(activityCell).filter(text => text !== '—').join(', ') || '—'
    if (value.timestamp_seconds != null) return `${value.timestamp_seconds}s in source`
    return activityCell(value.source_file || value.label || value.text || value.statement)
  }
  return /^nexusai:\/\/|^[0-9a-f]{8}-[0-9a-f-]{27}$/i.test(String(value)) ? 'See source citation' : String(value)
}

export function activityCitationsFor(item) {
  const presentation = activityPresentationFor(item)
  if (Array.isArray(presentation.citations) && presentation.citations.length) return presentation.citations
  if (Array.isArray(presentation.fact_packet?.citations) && presentation.fact_packet.citations.length) return presentation.fact_packet.citations
  return Array.isArray(presentation.trace?.raw_provenance) ? presentation.trace.raw_provenance : []
}

function authoritativeResultState(item) {
  const presentation = activityPresentationFor(item)
  const status = String(item?.status || '').toLowerCase()
  if (['error', 'failed', 'cancelled', 'timed_out'].includes(status) || status.startsWith('error:')) return 'failed'
  if (status === 'needs_input') return 'invalid_request'
  if (presentation.result_state) return presentation.result_state
  if (presentation.status) return presentation.status
  if (status === 'completed') return String(item?.answer || presentation.executive_answer || '').trim() ? 'results_present' : 'failed'
  if (['failed', 'cancelled'].includes(status)) return 'failed'
  if (['queued', 'running', 'processing'].includes(status)) return 'processing'
  return 'not_processed'
}

function authoritativeRowCount(presentation) {
  const values = [presentation?.row_count, presentation?.table?.count]
  for (const value of values) {
    if (value === null || value === undefined || value === '') continue
    const count = Number(value)
    if (Number.isFinite(count) && count >= 0) return count
  }
  if (Array.isArray(presentation?.table?.rows)) return presentation.table.rows.length
  return null
}

export function activityResultState(item) {
  const presentation = activityPresentationFor(item)
  const state = analystResultState(authoritativeResultState(item))
  const count = authoritativeRowCount(presentation)
  const labels = {
    complete_zero_results: 'Processing completed · No detections',
    no_match_for_filter: 'No results matched this filter',
    processing: 'Processing',
    not_processed: 'Not processed',
    failed: 'Processing failed',
    unavailable: 'Capability unavailable',
    unauthorized: 'Access restricted',
    invalid_request: 'Question needs detail',
  }
  const label = state.id === 'results_present'
    ? count === 1 ? '1 result' : count !== null && count > 1 ? `${count.toLocaleString()} results` : 'Results available'
    : labels[state.id] || state.label
  const filter = state.id === 'results_present'
    ? 'results'
    : ['complete_zero_results', 'no_match_for_filter', 'not_processed', 'invalid_request'].includes(state.id)
      ? 'no_results'
      : state.id === 'processing' ? 'processing' : 'failed'
  return { ...state, label, count, filter }
}

function catalogEvidence(citation, catalogItems) {
  const evidenceId = citationEvidenceId(citation, catalogItems)
  return {
    evidenceId,
    item: catalogItems.find(item => item?.evidence_id === evidenceId) || null,
  }
}

export function activitySourcesFor(item, catalogItems = []) {
  const unique = new Set()
  return activityCitationsFor(item).map((citation, index) => {
    const normalized = citation && typeof citation === 'object' ? citation : { label: citation }
    const { evidenceId, item: evidence } = catalogEvidence(normalized, catalogItems)
    const citationLabel = analystCitation(normalized, index)
    const label = evidence
      ? friendlySourceName(evidence)
      : citationLabel.label.startsWith('Evidence source ')
        ? citationLabel.label
        : friendlySourceName({ source_file: citationLabel.label })
    const typePresentation = evidence ? evidenceTypePresentation(evidence) : { label: 'Evidence source', icon: 'fa-file-circle-question' }
    const navigation = analystSourceNavigationParams(normalized, evidenceId)
    const key = `${evidenceId}|${label}|${citationLabel.detail}|${navigation.toString()}`
    if (unique.has(key)) return null
    unique.add(key)
    return { citation: normalized, evidenceId, label, type: typePresentation.label, icon: typePresentation.icon, detail: citationLabel.detail, navigation }
  }).filter(Boolean)
}

export function activityItemPresentation(item, catalogItems = []) {
  const presentation = activityPresentationFor(item)
  const sources = activitySourcesFor(item, catalogItems)
  const result = activityResultState(item)
  const question = String(item?.query || 'Investigation activity').trim()
  const answer = String(presentation.executive_answer || item?.answer || '').trim()
  const summary = answer.length > 320 || /(?:^|\n)\s*(?:#{1,6}\s|\|\s*---|<details>)/i.test(answer)
    ? result.description || 'Stored result available to review.'
    : answer || result.description || 'Open this activity to review the retained result.'
  return {
    item,
    presentation,
    question,
    answer,
    summary,
    result,
    sources,
    primarySource: sources[0] || null,
    searchText: [question, answer, result.label, ...sources.flatMap(source => [source.label, source.type, source.detail])].join(' ').toLowerCase(),
  }
}

function technicalFinding(finding) {
  const kind = String(finding?.claim_type || finding?.kind || '').trim().toLowerCase()
  const text = String(finding?.text || finding?.statement || finding || '').trim()
  return ['technical', 'execution_metadata', 'trace', 'diagnostic'].includes(kind)
    || /^(?:template|route|operation|execution(?: authority)?|model|backend|planner(?: confidence)?|specialist|source access|elapsed|display rows|contract(?: version)?):/i.test(text)
}

export function activityFindingGroups(presentation) {
  const findings = (Array.isArray(presentation?.findings) ? presentation.findings : []).filter(finding => finding != null)
  return {
    visible: findings.filter(finding => !technicalFinding(finding)),
    technical: findings.filter(technicalFinding),
  }
}

export function activityFilterOptions(items) {
  const labels = { results: 'Results', no_results: 'No results', processing: 'Processing', failed: 'Failed' }
  const counts = Object.fromEntries(Object.keys(labels).map(key => [key, 0]))
  for (const item of items) counts[activityResultState(item).filter] += 1
  return [
    { id: 'all', label: 'All', count: items.length },
    ...Object.entries(labels).filter(([id]) => counts[id] > 0).map(([id, label]) => ({ id, label, count: counts[id] })),
  ]
}

export function activityDateGroup(value, now = new Date()) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return 'Date not reported'
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const activityDay = new Date(date.getFullYear(), date.getMonth(), date.getDate())
  const calendarDays = Math.round((today.getTime() - activityDay.getTime()) / 86400000)
  if (calendarDays === 0) return 'Today'
  if (calendarDays === 1) return 'Yesterday'
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' })
}

export function groupActivityItems(items, now = new Date()) {
  const groups = []
  for (const item of items) {
    const label = activityDateGroup(item?.created_at, now)
    const existing = groups.find(group => group.label === label)
    if (existing) existing.items.push(item)
    else groups.push({ label, items: [item] })
  }
  return groups
}
