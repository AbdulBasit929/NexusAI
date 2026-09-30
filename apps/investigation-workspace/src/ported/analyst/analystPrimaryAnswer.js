const technicalFindingPattern = /\b(planner|capability|operation|semantic|template|execution authority|proof role|row proof|citation proof|processor|model status|canonical|normalized|hash|provenance|structured records row count)\b/i
const internalFieldPattern = /(?:^|_)(?:normalized|canonical|hash|version|evidence_id|row_number|record_id|source_locator|proof_role|processor|model)(?:_|$)/i

function text(value) {
  if (value == null) return ''
  if (typeof value === 'object') return String(value.text || value.label || value.value || value.summary || '')
  return String(value)
}

function clean(value) {
  return text(value).replace(/\*\*/g, '').replace(/^#{1,6}\s+/gm, '').replace(/\s+/g, ' ').trim()
}

function metricValue(view, label) {
  const metric = (Array.isArray(view?.metrics) ? view.metrics : []).find(item => String(item?.label || item?.name || '').trim().toLowerCase() === label)
  return metric?.value ?? metric?.count ?? metric?.total ?? ''
}

function firstRow(view) {
  const rows = Array.isArray(view?.table?.rows) ? view.table.rows : []
  return rows.length === 1 && rows[0] && typeof rows[0] === 'object' && !Array.isArray(rows[0]) ? rows[0] : null
}

function firstValue(row, fields) {
  return fields.map(field => row?.[field]).find(value => value != null && value !== '')
}

function resultCount(view) {
  const rows = Array.isArray(view?.table?.rows) ? view.table.rows : []
  const reported = Number(view?.table?.count ?? view?.row_count)
  return Number.isFinite(reported) ? reported : rows.length
}

export function analystNaturalAnswer(view = {}) {
  const state = String(view.result_state || view.status || '').toLowerCase()
  if (state === 'complete_zero_results') return 'Processing completed. No detections were found.'
  if (['no_match_for_filter', 'no_results'].includes(state)) return 'No results matched this filter.'

  const answer = clean(view.clarification || view.executive_answer || 'The analysis is complete.')
  const normalizedMention = answer.match(/^An exact normalized mention of\s+(["“].+?["”])\s+was found in this recording\.?$/iu)
  if (normalizedMention) return `The phrase ${normalizedMention[1]} was found in this recording.`

  const operation = String(view.operation_id || view.operation || '').toLowerCase()
  if (operation.includes('anpr')) {
    const row = firstRow(view) || {}
    const target = metricValue(view, 'target') || firstValue(row, ['plate', 'plate_text', 'raw_plate_text', 'registration'])
    const count = resultCount(view)
    if (count > 0 && target) return `${count.toLocaleString()} sighting${count === 1 ? '' : 's'} of ${target} ${count === 1 ? 'was' : 'were'} found.`
  }

  if (/^(Retrieved|Computed)\b/i.test(answer) && resultCount(view) > 0) {
    const count = resultCount(view)
    return `${count.toLocaleString()} result${count === 1 ? '' : 's'} ${count === 1 ? 'was' : 'were'} found.`
  }
  return answer
}

function formatMoment(value) {
  const raw = text(value)
  const date = new Date(raw)
  if (!raw || !Number.isFinite(date.getTime())) return raw
  const calendar = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' }).format(date)
  return `${calendar} · ${date.toISOString().slice(11, 19)} UTC`
}

function seconds(value) {
  const number = Number(value)
  return Number.isFinite(number) ? `${number.toLocaleString()} seconds` : text(value)
}

export function analystSingleResultFacts(view = {}) {
  const row = firstRow(view)
  if (!row) return []
  const operation = String(view.operation_id || view.operation || '').toLowerCase()
  const facts = []
  const observed = firstValue(row, ['observed_at', 'captured_at', 'event_time', 'timestamp', 'call_start_ts'])
  const sourceSeconds = firstValue(row, ['source_seconds', 'timestamp_seconds', 'start_seconds'])
  if (observed) facts.push({ label: 'Observed', value: formatMoment(observed), direction: 'ltr' })
  else if (sourceSeconds != null) facts.push({ label: 'Observed', value: seconds(sourceSeconds), direction: 'ltr' })

  if (operation.includes('anpr')) {
    facts.push({ label: 'Camera', value: text(firstValue(row, ['camera_id', 'camera'])) || 'Not available', direction: 'auto' })
    facts.push({ label: 'Location', value: text(firstValue(row, ['location', 'site_name', 'camera_location'])) || 'Not available', direction: 'auto' })
    return facts
  }
  if (operation.includes('transcript') || operation.includes('text_search')) return facts

  const excluded = new Set(['source_file', 'source_seconds', 'timestamp_seconds', 'start_seconds', 'end_seconds', 'observed_at', 'captured_at', 'event_time', 'timestamp', 'call_start_ts'])
  for (const [field, value] of Object.entries(row)) {
    if (facts.length >= 3 || excluded.has(field) || internalFieldPattern.test(field) || value == null || value === '') continue
    if (typeof value === 'object') continue
    facts.push({ label: field.replaceAll('_', ' ').replace(/\b\w/g, letter => letter.toUpperCase()), value: text(value), direction: 'auto' })
  }
  return facts
}

export function analystUsefulFindings(view = {}, answer = analystNaturalAnswer(view)) {
  const seen = new Set([clean(answer).toLocaleLowerCase()])
  return (Array.isArray(view.findings) ? view.findings : []).flatMap(finding => {
    const value = clean(finding)
    const kind = String(finding?.claim_type || finding?.kind || '')
    const key = value.toLocaleLowerCase()
    if (!value || technicalFindingPattern.test(value) || /^(metric|semantic|planner|operation)$/i.test(kind) || seen.has(key)) return []
    seen.add(key)
    return [value]
  }).slice(0, 3)
}

export function analystUsefulFollowUps(actions = []) {
  const systemAction = /^(show source rows|review source rows|check relationships|audit source files|check analysis readiness|show entity coverage|expand date bounds|broaden target search|build timeline)$/i
  const seen = new Set()
  return actions.flatMap(action => {
    const label = clean(action)
    const query = clean(action?.query || label)
    const key = label.toLocaleLowerCase()
    if (!label || !query || systemAction.test(label) || seen.has(key)) return []
    seen.add(key)
    return [{ label, query }]
  }).slice(0, 2)
}

export function analystAuthoritativeEntityChoices(history = []) {
  const entityAnalysis = history.find(item => {
    const view = item?.metadata?.presentation
    return String(view?.operation_id || view?.operation || '').toLowerCase().includes('entity_activity')
      && Array.isArray(view?.table?.rows)
  })
  const rows = entityAnalysis?.metadata?.presentation?.table?.rows || []
  const seen = new Set()
  return rows.flatMap(row => {
    const type = String(row?.entity_type || '').toLowerCase()
    const value = text(row?.entity_value || row?.msisdn || row?.subscriber_reference).trim()
    if (!['phone', 'subscriber', 'msisdn'].includes(type) || !value || seen.has(value)) return []
    seen.add(value)
    return [{ label: value, query: `show frequent contacts for ${value}`, type }]
  }).slice(0, 5)
}

export function analystPrimaryFieldIsInternal(field) {
  return internalFieldPattern.test(String(field || ''))
}
