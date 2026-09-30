import { sourceProductState } from './analystPresentation.js'

function reportedCount(...values) {
  for (const value of values) {
    if (value === null || value === undefined || value === '') continue
    const count = Number(value)
    if (Number.isFinite(count) && count >= 0) return count
  }
  return null
}

export function homeEvidenceSummary(statusData, evidence) {
  const status = statusData?.summary || {}
  const catalog = evidence?.summary || {}
  const items = Array.isArray(evidence?.items) ? evidence.items : []
  const itemStates = items.map(sourceProductState)
  const itemCount = state => itemStates.filter(item => item.id === state).length

  const ready = reportedCount(status.evidence_completed, catalog.completed, itemCount('ready')) || 0
  const processing = reportedCount(
    status.evidence_in_flight,
    reportedCount(catalog.processing, 0) + reportedCount(catalog.queued, 0),
    itemCount('processing'),
  ) || 0
  const failed = reportedCount(status.evidence_failed, catalog.failed, itemCount('failed')) || 0
  const review = reportedCount(catalog.needs_review, catalog.attention, itemCount('attention')) || 0
  const attention = failed + review
  const total = reportedCount(catalog.evidence_total, status.evidence_total, ready + processing + attention, items.length) || 0

  return {
    total,
    ready,
    processing,
    attention,
    hasEvidence: total > 0,
    isEmpty: total === 0 && processing === 0,
  }
}

export function homeOrientation(summary) {
  if (summary.isEmpty) {
    return {
      label: 'No evidence added',
      title: 'Build the evidence base',
      description: 'Add retained sources to prepare this investigation for review and analysis.',
      tone: 'empty',
      action: 'add',
    }
  }
  if (summary.ready === 0 && summary.attention > 0) {
    return {
      label: `${summary.attention.toLocaleString()} source${summary.attention === 1 ? '' : 's'} need attention`,
      title: 'Review evidence that needs attention',
      description: 'Open Data to see what could not be prepared and the available next step.',
      tone: 'attention',
      action: 'review',
    }
  }
  if (summary.ready === 0 && summary.processing > 0) {
    return {
      label: `${summary.processing.toLocaleString()} processing`,
      title: 'Evidence is being prepared',
      description: 'You can review registered sources now. Analysis becomes available as processing completes.',
      tone: 'processing',
      action: 'review',
    }
  }
  return {
    label: `${summary.ready.toLocaleString()} evidence source${summary.ready === 1 ? '' : 's'} ready`,
    title: 'Ask about this investigation',
    description: 'Ask in everyday language and review answers with their retained sources.',
    tone: summary.attention > 0 ? 'attention' : 'ready',
    action: 'ask',
  }
}

const UUID_PATTERN = /\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b/gi

function escapedPattern(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export function homeActivityPreviewText(value, protectedSource = value) {
  const protectedUUIDs = String(protectedSource || '').match(UUID_PATTERN) || []
  let preview = String(value || '').replace(UUID_PATTERN, 'this evidence source')
  for (const uuid of protectedUUIDs) {
    for (const fragment of uuid.split('-').filter(part => part.length >= 4)) {
      preview = preview.replace(new RegExp(`\\b${escapedPattern(fragment)}\\b`, 'gi'), 'this evidence source')
    }
  }
  return preview
}
