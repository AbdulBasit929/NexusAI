import { familyLabel } from './analystPresentation.js'
import { groupMediaArtifacts } from './analystMediaPresentation.js'
import { resolveEvidenceModality } from '../utils/evidenceModality.js'

export function evidenceSourceType(item) {
  return String(item?.detected_type || item?.record_type || item?.modality || 'source').toLowerCase()
}

export function evidenceTypePresentation(item) {
  const type = evidenceSourceType(item)
  const resolved = resolveEvidenceModality(item)
  if (type.includes('anpr')) return { category: 'anpr', label: 'Vehicle evidence', shortLabel: 'ANPR', icon: 'fa-car-side' }
  if (resolved.kind === 'structured') return { ...resolved, label: familyLabel(type), shortLabel: familyLabel(type) }
  if (resolved.kind === 'document' && type === 'pdf') return { ...resolved, shortLabel: 'PDF' }
  return resolved
}

function reportedCount(item) {
  const direct = item?.accepted_rows
  const summary = item?.metadata?.structured_summary?.canonical_rows
  for (const value of [direct, summary]) {
    if (value === null || value === '' || value === undefined) continue
    const count = Number(value)
    if (Number.isFinite(count) && count >= 0) return count
  }
  return null
}

function countLabel(count, singular, plural) {
  return `${count.toLocaleString()} ${count === 1 ? singular : plural}`
}

export function evidenceFindingPreview(item, status, capability, artifacts) {
  if (status?.id !== 'ready') return status?.description || 'Readiness has not been established.'
  const count = reportedCount(item)
  const { category } = evidenceTypePresentation(item)
  // accepted_rows includes technical audio observations, not just speech.
  if (category === 'audio') {
    if (!Array.isArray(artifacts)) return 'Audio source ready — open to review transcript availability'
    return groupMediaArtifacts(artifacts).audio.length > 0 ? 'Machine transcript ready to review' : 'No transcript was recorded'
  }
  if (count !== null) {
    if (category === 'images' || category === 'anpr') return count > 0 ? 'Image findings ready to review' : 'No image findings were recorded'
    if (category === 'video') return count > 0 ? 'Timeline findings ready to review' : 'No timeline findings were recorded'
    if (category === 'documents') return `${countLabel(count, 'extracted passage', 'extracted passages')} available`
    return `${countLabel(count, 'verified record', 'verified records')} available for analysis`
  }
  if (category === 'structured') return 'Verified record count is not available'
  if (capability?.availability === 'queryable') return 'Verified analysis is available'
  if (capability?.availability === 'limited') return 'Limited analysis is available'
  if (capability?.availability === 'semantic_only') return 'Searchable text is available'
  return status?.description || 'Source is ready'
}

export function visibleStatusFilters(counts) {
  const filters = [['all', 'All sources'], ['ready', 'Ready']]
  if (Number(counts.processing) > 0) filters.push(['processing', 'Processing'])
  if (Number(counts.attention) > 0) filters.push(['attention', 'Needs attention'])
  if (Number(counts.failed) > 0) filters.push(['failed', 'Failed'])
  return filters
}

export function evidenceFilterSummary(count, hasNext) {
  return `${count.toLocaleString()} matching sources${hasNext ? ' in loaded sources' : ''}`
}

export function detailDisclosureModel({ maturity, accounting, operations = [], problems = [] }) {
  const qualityCounts = maturity?.quality?.counts || {}
  const qualityReported = Boolean(maturity?.quality?.state)
    || Object.keys(qualityCounts).length > 0
    || [accounting?.acceptedRows, accounting?.inputRows, accounting?.duplicateRows, accounting?.rejectedRows].some(value => value !== null && value !== undefined)
  return {
    fields: (maturity?.fields || []).length > 0,
    additionalFields: (maturity?.mapping?.recognized_extension_fields || []).length > 0 || (maturity?.mapping?.unmapped_fields || []).length > 0,
    quality: qualityReported,
    capabilities: (maturity?.capabilities || []).length > 0 || operations.length > 0 || Boolean(maturity?.time?.contract_version),
    notes: problems.length > 0,
  }
}
