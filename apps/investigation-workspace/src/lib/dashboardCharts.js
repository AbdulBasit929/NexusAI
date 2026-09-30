import { formatNumber } from './format.js'
import { curatedFamilyLabel } from './semanticCatalog.js'

// Pure data mappers and ECharts option builders for the Dashboard. Every number comes from the
// collection-status summaries; nothing is estimated, and a case whose status was not reported is
// left out rather than drawn as zero.

const READINESS_ORDER = [
  { id: 'ready', label: 'Ready', status: 'completed' },
  { id: 'processing', label: 'Processing', status: 'processing' },
  { id: 'failed', label: 'Failed', status: 'failed' },
  { id: 'other', label: 'Not yet counted', status: null },
]

export function readinessSegments(summary) {
  const ready = Math.max(0, Number(summary.ready) || 0)
  const processing = Math.max(0, Number(summary.inFlight) || 0)
  const failed = Math.max(0, Number(summary.failed) || 0)
  const other = Math.max(0, (Number(summary.total) || 0) - ready - processing - failed)
  return { ready, processing, failed, other }
}

export function readinessRows(rows) {
  return rows
    .filter(row => row.summary && row.summary.total > 0)
    .map(row => {
      const counts = readinessSegments(row.summary)
      const total = counts.ready + counts.processing + counts.failed + counts.other
      return { caseId: row.caseId, total, counts }
    })
    // Worst first: the case that most needs the analyst leads.
    .sort((left, right) => right.counts.failed - left.counts.failed || right.counts.processing - left.counts.processing || left.caseId.localeCompare(right.caseId))
}

export function evidenceLink(caseId, segmentId) {
  const status = READINESS_ORDER.find(item => item.id === segmentId)?.status
  return `/cases/${encodeURIComponent(caseId)}/evidence${status ? `?status=${status}` : ''}`
}

export function familyRows(aggregated) {
  const total = aggregated.reduce((sum, item) => sum + item.value, 0)
  return aggregated.map(item => ({ ...item, label: curatedFamilyLabel(item.id), share: total ? item.value / total : 0 }))
}

export function familyOption(items, theme) {
  return {
    aria: { enabled: false },
    animation: false,
    grid: { left: 8, right: 56, top: 4, bottom: 4, containLabel: true },
    tooltip: {
      trigger: 'item',
      confine: true,
      backgroundColor: theme.card,
      borderColor: theme.line,
      textStyle: { color: theme.text },
      formatter: params => `<strong>${params.name}</strong><br/>${formatNumber(params.value)} accepted rows across ${formatNumber(params.data.caseCount)} ${params.data.caseCount === 1 ? 'case' : 'cases'}<br/><em>Select to show only cases with this family.</em>`,
    },
    xAxis: { type: 'value', show: false },
    yAxis: { type: 'category', inverse: true, data: items.map(item => item.label), axisTick: { show: false }, axisLine: { show: false }, axisLabel: { color: theme.text, width: 150, overflow: 'truncate', fontSize: 12 } },
    series: [{
      type: 'bar',
      barWidth: 16,
      barMinHeight: 3,
      itemStyle: { color: theme.data[0], borderRadius: [0, 3, 3, 0] },
      label: { show: true, position: 'right', color: theme.text, fontSize: 12, fontWeight: 600, formatter: params => formatNumber(params.value) },
      data: items.map(item => ({ value: item.value, id: item.id, caseCount: item.caseCount })),
    }],
  }
}

function share(part, whole) {
  if (!whole || !part) return 0
  return part / whole
}

export function formatRate(fraction) {
  if (!fraction) return '0%'
  if (fraction < 0.001) return '<0.1%'
  return `${new Intl.NumberFormat('en-GB', { maximumFractionDigits: 1 }).format(fraction * 100)}%`
}

export function ingestionRows(rows) {
  return rows.filter(row => row.summary).map(row => {
    const { acceptedRows, duplicateRows, rejectedRows } = row.summary
    const total = acceptedRows + duplicateRows + rejectedRows
    return { caseId: row.caseId, accepted: acceptedRows, duplicate: duplicateRows, rejected: rejectedRows, total, duplicateShare: share(duplicateRows, total), rejectedShare: share(rejectedRows, total) }
  }).filter(row => row.total > 0)
}

// The two kinds of "needs review" the status reports. Both count against the per-case summary, which is complete;
// the named items below are only the most recent each case lists, so the two are never mixed up.
export function reviewKind(item) {
  return item.kind === 'Failed' ? 'failed' : 'missing'
}

// Where the problems are: one row per case with something to review, ranked worst first. Counts come from the
// summary (complete), so a bar is exact even when the case names only some of its sources.
export function reviewByCase(rows) {
  return rows
    .filter(row => row.summary)
    .map(row => {
      const failed = Math.max(0, Number(row.summary.failed) || 0)
      const missing = Math.max(0, Number(row.summary.missingAssets) || 0)
      return { caseId: row.caseId, failed, missing, total: failed + missing }
    })
    .filter(entry => entry.total > 0)
    .sort((left, right) => right.total - left.total || right.failed - left.failed || left.caseId.localeCompare(right.caseId))
}

export function itemMatchesReviewFilter(item, filter) {
  return (!filter.kind || reviewKind(item) === filter.kind) && (!filter.caseId || item.caseId === filter.caseId)
}

// How many items the current filter stands for, from the complete counts, so the list can say honestly when it names fewer.
export function reviewFilterCount(byCase, filter) {
  return byCase
    .filter(entry => !filter.caseId || entry.caseId === filter.caseId)
    .reduce((total, entry) => total + (filter.kind === 'failed' ? entry.failed : filter.kind === 'missing' ? entry.missing : entry.total), 0)
}

export function attentionItems(rows) {
  const items = []
  for (const row of rows) {
    const data = row.state?.data
    if (!data) continue
    const seen = new Map()
    // The same failed source can appear as evidence and as a job: keep one row and keep the job's reason.
    const push = item => {
      const key = `${item.evidenceId || item.label}|${item.kind}`
      const existing = seen.get(key)
      if (existing) { if (!existing.detail && item.detail) existing.detail = item.detail; return }
      const entry = { caseId: row.caseId, ...item }
      seen.set(key, entry)
      items.push(entry)
    }
    for (const item of data.recent_evidence || []) if (String(item.processing_status).toLowerCase() === 'failed') push({ kind: 'Failed', label: item.source_file || 'Evidence item', evidenceId: item.evidence_id, detail: '' })
    for (const item of data.recent_jobs || []) if (String(item.status).toLowerCase() === 'failed') push({ kind: 'Failed', label: item.source_file || 'Evidence item', evidenceId: item.evidence_id, detail: item.error_message || '' })
    for (const item of data.missing_kb_assets || []) {
      const label = typeof item === 'string' ? item : item.source_file || item.evidence_id || 'Evidence item'
      push({ kind: 'Retained copy missing', label, evidenceId: typeof item === 'object' ? item.evidence_id : undefined, detail: '' })
    }
  }
  // Failures before missing copies; otherwise the order the cases reported them in.
  const order = { Failed: 0 }
  return items.map((item, index) => ({ item, index })).sort((left, right) => (order[left.item.kind] ?? 1) - (order[right.item.kind] ?? 1) || left.index - right.index).map(entry => entry.item)
}
