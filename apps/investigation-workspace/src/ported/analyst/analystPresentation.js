export function numberValue(value) {
  const number = Number(value)
  return Number.isFinite(number) ? number : 0
}

export function friendlyWorkspaceName(workspace) {
  const raw = String(workspace?.displayName || workspace?.caseId || 'Workspace').trim()
  if (!raw || raw.toLowerCase() !== String(workspace?.caseId || '').toLowerCase()) return raw
  const words = raw.split(/[-_\s]+/).filter(word => word && word.toLowerCase() !== 'nexusai')
  const label = words.map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ')
  return label || 'Investigation workspace'
}

export function friendlySourceName(item) {
  const raw = String(item?.original_filename || item?.source_file || item?.evidence_id || 'Untitled source')
  const leaf = raw.split(/[\\/]/).pop() || raw
  const dot = leaf.lastIndexOf('.')
  const base = dot > 0 ? leaf.slice(0, dot) : leaf
  const extension = dot > 0 ? leaf.slice(dot).toLowerCase() : ''
  const words = base.replace(/[_-]+/g, ' ').replace(/\s+/g, ' ').trim()
  const title = words ? words.charAt(0).toUpperCase() + words.slice(1) : 'Untitled source'
  return `${title}${extension}`
}

function authoritativeCount(sources, directField, summaryField = directField) {
  for (const source of sources) {
    if (!source || !Object.prototype.hasOwnProperty.call(source, directField)) continue
    const value = source[directField]
    if (value === null || value === '') continue
    const number = Number(value)
    if (Number.isFinite(number) && number >= 0) return number
  }
  for (const source of sources) {
    const summary = source?.metadata?.structured_summary
    if (!summary || !Object.prototype.hasOwnProperty.call(summary, summaryField)) continue
    const value = summary[summaryField]
    if (value === null || value === '') continue
    const number = Number(value)
    if (Number.isFinite(number) && number >= 0) return number
  }
  return null
}

export function sourceDetailPresentation(detailItem, selectedItem, catalogItems = []) {
  const evidenceId = detailItem?.evidence_id || selectedItem?.evidence_id || ''
  const catalogItem = catalogItems.find(item => item?.evidence_id === evidenceId)
  const selectedForEvidence = selectedItem?.evidence_id === evidenceId ? selectedItem : null
  const sources = [catalogItem, detailItem, selectedForEvidence].filter(Boolean)

  return {
    item: { ...(selectedForEvidence || {}), ...(catalogItem || {}), ...(detailItem || {}) },
    accounting: {
      acceptedRows: authoritativeCount(sources, 'accepted_rows', 'canonical_rows'),
      inputRows: authoritativeCount(sources, 'total_rows', 'total_rows'),
      duplicateRows: authoritativeCount(sources, 'duplicate_rows', 'duplicate_rows'),
      rejectedRows: authoritativeCount(sources, 'rejected_rows', 'rejected_rows'),
    },
  }
}

export function citationEvidenceId(citation, catalogItems = []) {
  if (citation?.evidence_id) return citation.evidence_id
  const sourceName = String(citation?.source_file || citation?.label || '').split(/[\\/]/).pop().trim().toLowerCase()
  if (!sourceName) return ''
  const matches = catalogItems.filter(item => [item?.original_filename, item?.source_file]
    .some(value => String(value || '').split(/[\\/]/).pop().trim().toLowerCase() === sourceName))
  return matches.length === 1 ? matches[0].evidence_id || '' : ''
}

export function sourceProductState(item) {
  const internal = String(item?.processing_status || item?.ingest_status || item?.status || '').trim().toLowerCase()
  const route = String(item?.processing_route || '').trim().toLowerCase()
  const qualityState = String(item?.metadata?.structured_maturity?.quality?.state || item?.structured_maturity?.quality?.state || '').toLowerCase()
  if (['failed', 'dead_letter', 'error'].includes(internal)) {
    return { id: 'failed', label: 'Failed', tone: 'failed', description: 'Processing did not complete. The original source remains preserved.' }
  }
  if (['queued', 'running', 'processing', 'publication_pending', 'indexing'].includes(internal)) {
    return { id: 'processing', label: 'Processing', tone: 'processing', description: internal === 'queued' || internal === 'publication_pending' ? 'Waiting for governed processing.' : 'The workspace is preparing this source.' }
  }
  if (qualityState === 'failed') {
    return { id: 'failed', label: 'Failed', tone: 'failed', description: 'No canonical structured records were accepted. The original source remains preserved.' }
  }
  if (qualityState === 'needs_review') {
    return { id: 'attention', label: 'Needs review', tone: 'attention', description: 'Source ambiguity or invalid mappings must be resolved before dependent analysis is available.' }
  }
  if (['completed', 'ready', 'succeeded', 'queryable'].includes(internal)) {
    if (qualityState === 'ready_with_warnings') return { id: 'ready', label: 'Ready with warnings', tone: 'attention', description: 'Accepted records are available; review preserved or rejected fields before relying on every capability.' }
    return { id: 'ready', label: 'Ready', tone: 'ready', description: 'At least one intended analysis capability is available.' }
  }
  if (['manual_review', 'needs_review', 'review', 'unsupported', 'malformed'].includes(internal)
    || internal === 'registered'
    || route.includes('pending')
    || route.includes('manual_review')) {
    return { id: 'attention', label: 'Needs attention', tone: 'attention', description: route.includes('pending') ? 'The source is preserved, but its required analysis capability is not operational yet.' : 'This source needs analyst help or an additional accepted processing capability.' }
  }
  return { id: 'attention', label: 'Needs attention', tone: 'attention', description: 'The source is preserved, but readiness has not been established.' }
}

export function sourceMaturityPresentation(item) {
  const maturity = item?.metadata?.structured_maturity || item?.structured_maturity || {}
  const mapping = maturity?.schema_profile || maturity?.mapping || {}
  const quality = maturity?.quality || {}
  const time = maturity?.time_policy || {}
  const fields = Array.isArray(mapping?.source_columns) ? mapping.source_columns.map(field => ({
    source: field.source_field,
    canonical: field.canonical_field || field.candidate_canonical_fields?.join(', ') || 'Preserved only',
    state: field.mapping_state || 'unmapped',
    status: ({ canonical: 'Mapped', mapped: 'Mapped', recognized_extension: 'Preserved', preserved: 'Preserved', invalid: 'Invalid' }[field.mapping_state] || 'Needs review'),
    type: field.semantic_type || field.inferred_type || 'source_text',
    pattern: Array.isArray(field.sample_patterns) ? field.sample_patterns.join(', ') : (field.sample_pattern || field.pattern || 'Not reported'),
  })) : []
  const capabilities = Array.isArray(quality?.capability_readiness) ? quality.capability_readiness : []
  return { maturity, mapping, quality, time, fields, capabilities }
}

export function intakeResponseState(response) {
  const registration = response?.evidence_registration || response?.records_ingest || response || {}
  const status = String(response?.evidence_status || response?.records_status || registration?.status || '').toLowerCase()
  const evidenceId = registration?.evidence_id || registration?.existing?.evidence_id || response?.evidence_id || ''
  if (status === 'duplicate') {
    return { id: 'duplicate', label: 'Already added', tone: 'duplicate', evidenceId, message: registration?.message || 'The same content already exists in this workspace.' }
  }
  if (['failed', 'skipped', 'error'].includes(status)) {
    return { id: 'failed', label: 'Failed', tone: 'failed', evidenceId, message: response?.records_warning || registration?.message || 'The workspace could not register this source.' }
  }
  if (['queued', 'publication_pending', 'running', 'processing'].includes(status)) {
    return { id: 'processing', label: 'Processing', tone: 'processing', evidenceId, message: status === 'publication_pending' ? 'Registered safely; waiting for the processing queue.' : 'Registered safely and queued for processing.' }
  }
  const product = sourceProductState({ ...registration, processing_status: registration?.processing_status || status })
  return { ...product, evidenceId, message: registration?.warnings?.[0] || product.description }
}

export function capabilityForSource(item, capabilities) {
  const type = String(item?.detected_type || item?.record_type || item?.modality || '').toLowerCase()
  return (Array.isArray(capabilities?.families) ? capabilities.families : []).find(family =>
    (family.record_types || []).some(value => String(value).toLowerCase() === type)
    || (family.evidence_types || []).some(value => String(value).toLowerCase() === type)
    || (family.evidence_modalities || []).some(value => String(value).toLowerCase() === type))
}

export function familyLabel(value) {
  const labels = {
    access_log: 'Access activity', anpr: 'Vehicle sightings', anpr_vehicle_sightings: 'Vehicle sightings',
    case_cross_family: 'Cross-source analysis', cdr: 'Communications', communications_cdr: 'Communications',
    financial_transactions: 'Transactions', ipdr: 'Network activity', ipdr_network_sessions: 'Network activity',
    logs_access_security: 'Access activity', subscriber: 'Subscriber and device', subscriber_identity: 'Subscriber and device',
    tower: 'Location reference', tower_location: 'Location reference', transaction: 'Transactions',
  }
  return labels[value] || String(value || 'Analysis').replaceAll('_', ' ').replace(/\b\w/g, letter => letter.toUpperCase())
}

export function queryableFamilies(capabilities) {
  return (Array.isArray(capabilities?.families) ? capabilities.families : [])
    .filter(family => ['queryable', 'semantic_only', 'limited'].includes(family.availability) && Array.isArray(family.suggested_queries) && family.suggested_queries.length > 0)
    .sort((left, right) => numberValue(right.indexed_records) - numberValue(left.indexed_records))
}

export function capabilitySuggestions(capabilities, limit = 6) {
  const suggestions = []
  const seen = new Set()
  for (const family of queryableFamilies(capabilities)) {
    for (const query of family.suggested_queries || []) {
      const normalized = String(query || '').trim()
      const key = normalized.toLowerCase()
      if (!normalized || seen.has(key)) continue
      seen.add(key)
      suggestions.push({
        query: normalized,
        label: normalized.charAt(0).toUpperCase() + normalized.slice(1).replace(/[?.!]*$/, '?'),
        description: family.label || familyLabel(family.id),
        family: family.id,
        icon: familyIcon(family.id),
      })
      if (suggestions.length >= limit) return suggestions
    }
  }
  return suggestions
}

export function familyIcon(value) {
  const id = String(value || '').toLowerCase()
  if (id.includes('cdr') || id.includes('communication')) return 'fa-phone-volume'
  if (id.includes('ipdr') || id.includes('network')) return 'fa-network-wired'
  if (id.includes('anpr') || id.includes('vehicle')) return 'fa-car-side'
  if (id.includes('subscriber') || id.includes('identity')) return 'fa-id-card'
  if (id.includes('tower') || id.includes('location')) return 'fa-location-dot'
  if (id.includes('access') || id.includes('security')) return 'fa-shield-halved'
  if (id.includes('transaction') || id.includes('financial')) return 'fa-money-bill-transfer'
  if (id.includes('document') || id.includes('text')) return 'fa-file-lines'
  if (id.includes('image') || id.includes('ocr')) return 'fa-image'
  if (id.includes('face')) return 'fa-user-shield'
  if (id.includes('audio') || id.includes('speech') || id.includes('transcript')) return 'fa-waveform-lines'
  if (id.includes('video')) return 'fa-film'
  if (id.includes('cross')) return 'fa-diagram-project'
  return 'fa-magnifying-glass-chart'
}
