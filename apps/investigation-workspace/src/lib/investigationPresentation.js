import { analystPrimaryFieldIsInternal } from '../ported/analyst/analystPrimaryAnswer.js'
import { citationHref, claimSegments, hasOpenableLocator, normalizeCitations } from './citations.js'
import { humanizeKey } from './format.js'
import { semanticCatalog as defaultCatalog } from './semanticCatalog.js'

// Evidence names may legitimately contain the product name. Treating the
// brand as analyst-forbidden language discarded complete sourced answers when
// a retained filename began with "nexusai". Keep implementation vocabulary
// blocked without rejecting evidence text merely for naming the product.
const forbiddenAnalystLanguage = /\b(?:llm|model|backend|agent|operation(?:_id)?|template|route|sql|planner|token|embedding|processor|fact packet|source-native|typed algebra)\b/i
const hiddenColumns = new Set(['metadata', 'section', 'result_semantics', 'retrieval_score', 'source_entry'])

function array(value) {
  return Array.isArray(value) ? value : []
}

function text(value) {
  return String(value ?? '').replace(/\s+/g, ' ').trim()
}

function safeText(value) {
  const result = text(value)
  return result && !forbiddenAnalystLanguage.test(result) ? result : ''
}

// The governed SQL lane writes its sentences on the server from the query result and the curated layer; no model
// narrates them. They are the answer, or the stated reason for not answering, and must reach the analyst as written.
// The vocabulary filter above is for the older paths, whose text can leak implementation words. Applied to the lane
// it drops an answer about a "model" or a "route", and the reason an abstention gives ("unreviewed model observations").
const LANE_POLICY = 'governed_sql_lane'
function fromLane(response) {
  return response?.policy === LANE_POLICY
}
function textOf(response, value) {
  return fromLane(response) ? text(value) : safeText(value)
}

// Every consumer of a presentation (the thread, the evidence panel, the result components) reads `result`, `citations`
// and the lists without asking which state produced it. A state that carries no evidence, a failure or a clarification
// (which is what every abstention of the lane is), still has them, empty. A clarification once had none and the thread
// threw `Cannot read properties of undefined (reading 'groups')` while rendering, which blanked the whole page.
function withoutEvidence() {
  return {
    result: { columns: [], rows: [], unresolvedLabels: [] },
    citations: { items: [], groups: [], markerItems: [], totalOpenable: 0, truncated: false },
    limitations: [], followUps: [], scope: [], derivation: '',
  }
}

function sourcePlan(response) {
  return response?.planner?.semantic_operation_planner?.dynamic_plan
    || response?.query_plan?.applied_filters?.source_native
    || null
}

function recordTypeFor(response) {
  return response?.query_plan?.applied_filters?.record_type
    || response?.enterprise?.fact_packet?.rows?.[0]?.record_type
    || response?.records?.[0]?.record_type
    || ''
}

function displayValue(value, column, catalog) {
  const labelled = column.semanticId ? catalog.displayForValue(column.semanticId, value) : value
  if (labelled == null || labelled === '') return '—'
  if (typeof labelled === 'number') return labelled.toLocaleString()
  if (typeof labelled === 'boolean') return labelled ? 'Yes' : 'No'
  if (typeof labelled === 'object') return 'Available in source details'
  const raw = String(labelled)
  const date = /^\d{4}-\d{2}-\d{2}T/.test(raw) ? new Date(raw) : null
  if (date && Number.isFinite(date.getTime())) {
    return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'medium', timeZone: 'UTC' }).format(date) + ' UTC'
  }
  return raw
}

function rowCitation(row, caseId) {
  const candidate = row?.citation || row?.citation_locator || row?.locator
    || row?.metadata?.source_rows?.[0] || null
  if (!candidate) return { locator: '', href: '' }
  const citation = candidate.locator ? candidate : {
    ...candidate,
    evidence_id: candidate.evidence_id || row?.evidence_id || row?.metadata?.evidence_id,
    version_id: candidate.version_id || row?.version_id || row?.metadata?.version_id,
    locator: candidate,
  }
  return hasOpenableLocator(citation)
    ? { locator: JSON.stringify(citation.locator), href: citationHref(citation, caseId) }
    : { locator: '', href: '' }
}

function resultTable(response, catalog, caseId = '') {
  const grid = response?.enterprise?.data_grid || {}
  const rows = array(grid.rows)
  const plan = sourcePlan(response)
  const recordType = recordTypeFor(response)
  const declared = array(grid.columns)
  const keys = declared.length ? declared.map(column => column.key) : [...new Set(rows.flatMap(row => Object.keys(row || {})))]
  const columns = keys.flatMap(key => {
    if (!key || hiddenColumns.has(String(key).toLowerCase()) || analystPrimaryFieldIsInternal(key)) return []
    const display = catalog.displayForColumn(key, recordType, plan)
    const label = display.fallback && display.label === key ? humanizeKey(key) : display.label
    return [{ key, label, description: display.description, semanticId: display.id, fallback: display.fallback }]
  })
  const displayedRows = rows.map(row => {
    const citation = rowCitation(row, caseId)
    return {
      ...Object.fromEntries(columns.map(column => [column.key, displayValue(row?.[column.key], column, catalog)])),
      ...(citation.locator ? { __citationLocator: citation.locator, __citationHref: citation.href } : {}),
    }
  })
  const totals = grid.totals && typeof grid.totals === 'object'
    ? Object.fromEntries(columns.map(column => [column.key, displayValue(grid.totals[column.key], column, catalog)]))
    : null
  return {
    columns,
    rows: displayedRows,
    totals,
    totalRows: Number.isFinite(Number(grid.count)) ? Number(grid.count) : rows.length,
    availableRows: rows.length,
    truncated: Number.isFinite(Number(grid.count)) && Number(grid.count) > rows.length,
    unresolvedLabels: columns.filter(column => column.fallback).map(column => column.key),
  }
}

function isAggregateZero(response) {
  const rows = array(response?.enterprise?.data_grid?.rows)
  if (rows.length !== 1) return false
  const values = Object.entries(rows[0] || {}).filter(([key]) => /^m\d+$/i.test(key)).map(([, value]) => Number(value))
  return values.length > 0 && values.every(value => value === 0)
}

function classifiedState(response, error) {
  if (error) return 'failed'
  const enterprise = response?.enterprise || {}
  const resultState = text(enterprise.result_state || enterprise.status).toLowerCase()
  const processingState = text(enterprise.processing_state).toLowerCase()
  const plannerState = text(response?.planner?.semantic_operation_planner?.state).toUpperCase()
  // `semantic` is the intent of an analysis the case cannot do. The lane's answered responses once carried it by mistake
  // (they now say `records`); a lane response is judged by the lane's own result, whichever image produced it.
  if (response?.intent === 'semantic' && !fromLane(response)) return 'unsupported'
  if (response?.intent === 'clarification') return plannerState === 'UNSUPPORTED_REQUEST_CLASS' ? 'unsupported' : 'clarify'
  if (/fail|error/.test(resultState)) return 'failed'
  if (/processing|queued|running/.test(resultState) || /processing|queued|running/.test(processingState)) return 'processing'
  if (/unavailable|unsupported|unauthorized/.test(resultState)) return 'unsupported'
  if (/no_match|no_results|complete_zero/.test(resultState) || isAggregateZero(response)) return 'zero-result'
  const limitations = array(enterprise.limitations).map(text).join(' ')
  if (enterprise?.proof_state?.rows === 'bounded' || /\bincomplete\b/i.test(`${enterprise.executive_answer} ${limitations}`)) return 'partial'
  return 'answered'
}

function clarificationModel(response, catalog) {
  const source = response?.clarification || response?.enterprise?.clarification || {}
  const originalQuestion = text(response?.query_understanding?.original_question || response?.enterprise?.fact_packet?.question)
  const options = array(source.options).map(option => {
    if (typeof option === 'string') {
      return { label: option, description: '', query: '', fallback: false, legacy: true }
    }
    const field = catalog.fieldFor(option?.field_id || option?.id, recordTypeFor(response))
    const suppliedLabel = typeof option?.label === 'string' ? option.label : ''
    const suppliedQuery = typeof option?.query === 'string' ? option.query : ''
    return {
      label: suppliedLabel || field?.display_name || text(option?.display_name || option?.value),
      description: field?.description || text(option?.description),
      query: suppliedQuery || (typeof option?.bound_query === 'string' ? option.bound_query : '')
        || (typeof option?.rerun_query === 'string' ? option.rerun_query : ''),
      fallback: !field,
      legacy: false,
    }
  }).filter(option => option.label.trim())
  const reason = text(source.reason_code)
  const specificQuestion = textOf(response, source.question || response?.enterprise?.executive_answer)
  const title = reason === 'missing_required_parameter'
    ? 'I can answer this with one more detail'
    : reason === 'cross_check_disagreement'
      ? 'The available evidence does not support one verified result'
      : reason === 'no_verified_plan'
        ? 'I stopped before giving an unverified answer'
        : 'No result was stated without a verified analysis'
  const reasonMessage = {
    missing_required_parameter: 'A required detail was not supplied. Choose the detail you intended, and I will run that exact question.',
    no_verified_plan: 'I could not verify a safe analysis for the original question. Choose a nearby question that can be checked against this case.',
    cross_check_disagreement: 'Two checked paths did not agree, so no result was stated. Choose a narrower question to continue.',
  }[reason]
  const clickableOptions = options.filter(option => option.query)
  return {
    title,
    question: reasonMessage || specificQuestion || 'The evidence does not currently support a verified answer to this question.',
    originalQuestion,
    reason,
    contractVersion: text(source.contract_version),
    missingFields: array(source.missing_fields).map(text).filter(Boolean),
    options,
    degraded: clickableOptions.length === 0,
    hasLegacyOptions: options.some(option => option.legacy),
    executionHeld: source.execution_held !== false,
  }
}

function limitationsFor(response, state) {
  const enterprise = response?.enterprise || {}
  const result = []
  const combined = `${enterprise.executive_answer || ''} ${array(enterprise.limitations).join(' ')}`
  if (/text search is incomplete|absence.+not been established/i.test(combined)) {
    result.push('Text-search coverage is incomplete. The absence of additional matches has not been established.')
  }
  if (enterprise?.proof_state?.rows === 'bounded') result.push('Only the returned subset of matching source records is shown.')
  if (state === 'processing') result.push('This result covers only evidence that was ready when the question ran.')
  for (const item of array(enterprise.limitations)) {
    const candidate = safeText(item)
    if (!candidate || /raw full tables|guardrail|requested operation|approved query/i.test(candidate)) continue
    if (!result.includes(candidate)) result.push(candidate)
  }
  return result.slice(0, 4)
}

function derivationFor(response, catalog) {
  const plan = sourcePlan(response)
  const entity = catalog.entityFor(recordTypeFor(response))
  const groups = array(plan?.group_fields).map(id => catalog.byId.get(id)?.display_name).filter(Boolean)
  const measures = array(plan?.measures).map(measure => {
    const item = catalog.byId.get(measure.field_id)
    return item?.display_name || (measure.op === 'COUNT' ? entity?.default_measure && catalog.byId.get(entity.default_measure)?.display_name : '')
  }).filter(Boolean)
  const action = measures.length ? `Calculated ${measures.join(' and ')}` : 'Reviewed matching evidence'
  const grouping = groups.length ? `, grouped by ${groups.join(' and ')}` : ''
  const scope = entity?.display_name ? ` in ${entity.display_name}` : ' in the selected case scope'
  const generated = text(response?.planner?.semantic_operation_planner?.state) === 'semantic_ir_fallback'
    ? ' The question was translated into a checked analysis using the fields available in this case.'
    : ''
  return `${action}${grouping}${scope}.${generated}`
}

function followUpsFor(response) {
  const actions = array(response?.enterprise?.recommended_actions)
  const seen = new Set()
  return actions.flatMap(action => {
    const label = safeText(action?.label)
    const query = safeText(action?.query)
    const key = query.toLocaleLowerCase()
    if (!label || !query || seen.has(key)) return []
    seen.add(key)
    return [{ label, query, reason: safeText(action?.reason) }]
  }).slice(0, 4)
}

function scopeFor(response, catalog) {
  const applied = response?.query_plan?.applied_filters || {}
  const plan = sourcePlan(response)
  const chips = []
  const entity = catalog.entityFor(applied.record_type)
  if (entity) chips.push({ label: 'Evidence', value: entity.display_name })
  for (const filter of array(plan?.filters)) {
    const field = catalog.byId.get(filter.field_id)
    const values = array(filter.values).length ? filter.values : [filter.value]
    chips.push({
      label: field?.display_name || filter.field_id,
      value: values.map(value => catalog.displayForValue(filter.field_id, value)).join(', '),
      fallback: !field,
    })
  }
  for (const target of array(response?.query_plan?.target_identifiers)) chips.push({ label: 'Target', value: target })
  if (applied.date_from || response?.query_plan?.date_bounds?.from) chips.push({ label: 'From', value: applied.date_from || response.query_plan.date_bounds.from })
  if (applied.date_to || response?.query_plan?.date_bounds?.to) chips.push({ label: 'To', value: applied.date_to || response.query_plan.date_bounds.to })
  return chips
}

function answeredTitle(state) {
  return {
    answered: 'Answer from case evidence',
    partial: 'Answer from available evidence',
    'zero-result': 'Analysis completed with no matches',
    processing: 'Evidence is still processing',
    unsupported: 'This question is not available here',
    failed: 'The question could not be completed',
  }[state] || 'Answer from case evidence'
}

export function presentInvestigationResponse(response, { caseId = response?.collection_id || '', catalog = defaultCatalog, error = null } = {}) {
  const state = classifiedState(response, error)
  if (state === 'clarify') {
    return { state, clarification: clarificationModel(response, catalog), ...withoutEvidence(), original: response }
  }
  if (state === 'failed') {
    return {
      state,
      title: answeredTitle(state),
      answer: 'Something prevented this question from completing. Your case evidence was not changed.',
      errorReference: text(error?.reference || response?.telemetry?.request_id || 'QRY-LOCAL'),
      ...withoutEvidence(),
      original: response,
    }
  }
  const enterprise = response?.enterprise || {}
  let answer = textOf(response, enterprise.executive_answer)
  if (state === 'unsupported') {
    answer = response?.intent === 'semantic'
      ? safeText(response?.answer?.answer) || 'This question is outside the evidence analysis available for this case.'
      : 'This question is outside the evidence analysis currently available for this case.'
  }
  const citations = normalizeCitations(response, caseId)
  return {
    state,
    title: answeredTitle(state),
    answer: answer || (state === 'processing' ? 'The available evidence is still being prepared.' : 'No verified answer is available.'),
    originalQuestion: text(response?.query_understanding?.original_question || enterprise?.fact_packet?.question),
    direction: enterprise?.narrative?.direction === 'rtl' ? 'rtl' : 'ltr',
    result: resultTable(response, catalog, caseId),
    citations,
    claimSegments: claimSegments(answer || (state === 'processing' ? 'The available evidence is still being prepared.' : 'No verified answer is available.'), response, citations),
    derivation: state === 'unsupported' ? '' : derivationFor(response, catalog),
    limitations: limitationsFor(response, state),
    followUps: followUpsFor(response),
    scope: scopeFor(response, catalog),
    interpretation: safeText(enterprise?.narrative?.status === 'generated' ? enterprise.narrative.direct_answer : ''),
    original: response,
  }
}

export const presentationInternals = { classifiedState, isAggregateZero, safeText, resultTable }
