import { analystCitation } from '../ported/analyst/analystAskPresentation.js'

function finite(value) {
  if (value === '' || value == null) return null
  const number = Number(value)
  return Number.isFinite(number) ? number : null
}

function locatorFor(citation) {
  const source = citation?.locator || citation?.citation_locator || {}
  return {
    rowNumber: finite(source.row_number ?? source.source_row ?? source.row ?? citation?.row_number ?? citation?.source_row),
    rowHash: String(source.row_hash ?? source.source_hash ?? citation?.row_hash ?? citation?.source_hash ?? ''),
    page: finite(source.page ?? source.page_number ?? citation?.page ?? citation?.page_number),
    passage: finite(source.passage ?? source.passage_number ?? citation?.passage ?? citation?.passage_number),
    charSpan: source.char_span ?? citation?.char_span ?? null,
    startSeconds: finite(source.t_start ?? source.start_seconds ?? source.timestamp_seconds ?? citation?.t_start ?? citation?.start_seconds),
    endSeconds: finite(source.t_end ?? source.end_seconds ?? citation?.t_end ?? citation?.end_seconds),
    frameSeconds: finite(source.frame_ts ?? source.frame_timestamp ?? citation?.frame_ts),
    bbox: source.bbox ?? citation?.bbox ?? null,
    artifactId: source.artifact_id ?? citation?.artifact_id ?? '',
  }
}

export function hasOpenableLocator(citation) {
  const locator = locatorFor(citation)
  const exactLocation = (locator.rowNumber !== null && Boolean(locator.rowHash))
    || (locator.page !== null && (locator.passage !== null || Boolean(locator.charSpan)))
    || locator.startSeconds !== null
    || locator.frameSeconds !== null
    || Boolean(locator.bbox)
  return Boolean(citation?.evidence_id && citation?.version_id && exactLocation)
}

function sourceName(citation) {
  const value = citation?.source_file || citation?.locator?.source_file || citation?.citation_locator?.source_file || ''
  return String(value).split(/[\\/]/).pop()
}

function factRowCandidates(packet) {
  return (packet?.rows || []).flatMap(row => {
    const claimValues = Object.entries(row || {})
      .filter(([key, value]) => !['metadata', 'section'].includes(key) && ['string', 'number'].includes(typeof value))
      .map(([, value]) => String(value))
    const count = row?.metadata?.contributing_row_count
    if (count != null) claimValues.push(String(count))
    return (row?.metadata?.source_rows || []).slice(0, 1).map(source => ({
      ...source,
      source_row: source.row_number,
      source_hash: source.row_hash,
      proof_role: 'representative_evidence',
      __claimValues: claimValues,
    }))
  })
}

function positiveInteger(value) {
  const number = Number(value)
  return Number.isInteger(number) && number > 0 ? number : null
}

function contributionModel(response) {
  const packet = response?.enterprise?.fact_packet || {}
  const lineages = response?.enterprise?.contribution_lineage
    || response?.enterprise?.provenance?.filter(item => item?.contract_version === 'forensics.contribution-lineage/v1')
    || []
  const sources = new Map()
  let total = null
  for (const lineage of Array.isArray(lineages) ? lineages : [lineages]) {
    total = positiveInteger(lineage?.contribution_count) || total
    for (const source of lineage?.sources || lineage?.contribution_sources || []) {
      const name = sourceName(source)
      const count = positiveInteger(source?.contribution_count)
      if (name && count) sources.set(name, Math.max(sources.get(name) || 0, count))
    }
  }
  if (!total) {
    const rowCounts = (packet.rows || []).map(row => positiveInteger(row?.metadata?.contributing_row_count)).filter(Boolean)
    total = rowCounts.length ? rowCounts.reduce((sum, count) => sum + count, 0) : null
  }
  return { total, sources }
}

function strengthFor(citation) {
  const confidence = finite(citation?.confidence)
  const truthState = String(
    citation?.source_truth_state
      || citation?.locator?.source_truth_state
      || citation?.citation_locator?.source_truth_state
      || '',
  ).trim().toLocaleLowerCase()
  if (truthState === 'ingested_source_record') return { id: 'strong', label: 'Source record', confidence: null }
  if (truthState !== 'derived_model_observation') {
    // Artifact type and confidence cannot establish whether the cited value is
    // source-native or model-derived. Keep the badge explicit until the API says.
    return { id: 'medium', label: 'Source type not reported', confidence: null }
  }
  if (confidence === null) return { id: 'medium', label: 'Model observation', confidence: null }
  if (confidence >= 0.8) return { id: 'strong', label: 'High-confidence observation', confidence }
  if (confidence >= 0.5) return { id: 'medium', label: 'Review observation', confidence }
  return { id: 'weak', label: 'Low-confidence observation', confidence }
}

export function citationHref(citation, caseId, locator = locatorFor(citation)) {
  const params = new URLSearchParams({ version: String(citation.version_id) })
  if (locator.rowNumber !== null) params.set('row', String(locator.rowNumber))
  if (locator.rowHash) params.set('row_hash', locator.rowHash)
  if (locator.page !== null) params.set('page', String(locator.page))
  if (locator.passage !== null) params.set('passage', String(locator.passage))
  if (locator.charSpan) params.set('char_span', JSON.stringify(locator.charSpan))
  if (locator.startSeconds !== null) params.set('source_time', String(locator.startSeconds))
  if (locator.endSeconds !== null) params.set('source_end', String(locator.endSeconds))
  if (locator.frameSeconds !== null) params.set('frame', String(locator.frameSeconds))
  if (locator.bbox) params.set('bbox', JSON.stringify(locator.bbox))
  if (locator.artifactId) params.set('finding', String(locator.artifactId))
  return `/cases/${encodeURIComponent(caseId)}/evidence/${encodeURIComponent(citation.evidence_id)}?${params}`
}

function fingerprint(citation, locator) {
  return [citation.evidence_id, citation.version_id, locator.rowNumber, locator.rowHash, locator.page, locator.passage,
    JSON.stringify(locator.charSpan), locator.startSeconds, locator.frameSeconds, JSON.stringify(locator.bbox)].join('|')
}

export function normalizeCitations(response, caseId, limit = 24) {
  const packet = response?.enterprise?.fact_packet || {}
  const candidates = [...factRowCandidates(packet), ...(packet.citations || []), ...(response?.enterprise?.provenance || [])]
  const seen = new Set()
  const valid = []
  for (const citation of candidates) {
    if (!hasOpenableLocator(citation)) continue
    const locator = locatorFor(citation)
    const key = fingerprint(citation, locator)
    if (seen.has(key)) continue
    seen.add(key)
    const label = analystCitation({ ...citation, source_file: sourceName(citation), locator: {
      row_number: locator.rowNumber,
      page: locator.page,
      passage: locator.passage,
      timestamp_seconds: locator.startSeconds,
    } }, valid.length)
    valid.push({
      id: citation.citation_id || `source-${valid.length + 1}`,
      label: label.label,
      detail: label.detail,
      href: citationHref(citation, caseId, locator),
      preview: String(citation.preview || citation.source_entry || '').slice(0, 200),
      timestamp: String(citation.timestamp || citation.locator?.timestamp || ''),
      strength: strengthFor(citation),
      proofRole: citation.proof_role || 'source lineage',
      locator,
      claimValues: citation.__claimValues || [],
    })
    if (valid.length >= limit) break
  }
  const contribution = contributionModel(response)
  const groupsBySource = new Map()
  for (const item of valid) {
    const key = item.label || item.href
    const group = groupsBySource.get(key) || {
      id: `source-group-${groupsBySource.size + 1}`,
      label: item.label,
      href: item.href,
      detail: item.detail,
      preview: item.preview,
      strength: item.strength,
      proofRole: item.proofRole,
      locator: item.locator,
      samples: [],
      contributingCount: contribution.sources.get(item.label) || null,
      totalContributing: contribution.total,
    }
    group.samples.push(item)
    groupsBySource.set(key, group)
  }
  const groups = [...groupsBySource.values()]
  groups.forEach((group, index) => { group.sourceNumber = index + 1 })
  for (const item of valid) item.sourceNumber = (groups.findIndex(group => group.label === item.label) + 1) || 1
  return {
    items: valid,
    groups,
    markerItems: groups.slice(0, 3),
    totalOpenable: seen.size,
    totalContributing: contribution.total,
    truncated: candidates.length > valid.length,
  }
}

function normalizedClaimValue(value) {
  return String(value ?? '').replaceAll(',', '').trim().toLocaleLowerCase()
}

export function claimSegments(answer, response, citations) {
  const token = /(\b\d{1,3}(?:,\d{3})+(?:\.\d+)?\b|\b\d{4}-\d{2}-\d{2}(?:T[^\s,;]+)?\b|\b[A-Fa-f0-9]{8}-[A-Fa-f0-9-]{12,}\b|\b\+?\d[\d -]{7,}\d\b|\b\d+(?:\.\d+)?\b)/g
  const pieces = String(answer || '').split(token)
  const rows = response?.enterprise?.fact_packet?.rows || response?.enterprise?.data_grid?.rows || []
  const numericValues = rows.flatMap(row => Object.entries(row || {}).filter(([key, value]) => key !== 'metadata' && typeof value === 'number').map(([, value]) => Number(value)))
  const totals = new Set([
    numericValues.reduce((sum, value) => sum + value, 0),
    rows.length,
  ].map(String))
  const fallback = citations.items?.[0] || citations.groups?.[0]
  return pieces.map((piece, index) => {
    if (index % 2 === 0) return { text: piece, citations: [] }
    const target = normalizedClaimValue(piece)
    const exact = citations.items?.find(item => item.claimValues?.some(value => normalizedClaimValue(value) === target))
    const derived = totals.has(target) ? fallback : null
    return { text: piece, citations: [exact || derived || fallback].filter(Boolean), claim: true }
  })
}
