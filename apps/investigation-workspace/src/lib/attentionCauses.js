import { reviewKind } from './dashboardCharts.js'

// Needs review, grouped by cause. A repeated cause ("the file has no header row" on three sources) is one finding, not
// three rows, so the analyst can fix the cause once. Named items are only the most recent each case reports, while the
// per-case counts are complete, so each kind is reconciled to its complete total: missing-copy sources are all one cause,
// so the remainder joins that group; failed sources with no listed cause form their own "cause not listed" group.
const GENERIC = { failed: 'Did not finish processing', missing: 'Retained copy is missing' }
const NOT_LISTED = 'Failed, cause not listed'

function clean(text) {
  return String(text || '').replace(/\s+/g, ' ').replace(/[.\s]+$/, '').trim()
}

export function causeOf(item) {
  const kind = reviewKind(item)
  const label = clean(item.detail) || GENERIC[kind]
  return { kind, label, key: `${kind}|${label.toLowerCase()}` }
}

export function attentionCauses(items, byCase) {
  const groups = new Map()
  for (const item of items) {
    const cause = causeOf(item)
    const group = groups.get(cause.key) || { key: cause.key, kind: cause.kind, label: cause.label, items: [], extra: 0, caseIds: new Set() }
    group.items.push(item)
    group.caseIds.add(item.caseId)
    groups.set(cause.key, group)
  }
  for (const kind of ['failed', 'missing']) {
    const complete = byCase.reduce((sum, entry) => sum + entry[kind], 0)
    const named = items.filter(item => reviewKind(item) === kind).length
    const remainder = Math.max(0, complete - named)
    if (!remainder) continue
    const label = kind === 'missing' ? GENERIC.missing : NOT_LISTED
    const key = `${kind}|${label.toLowerCase()}`
    const group = groups.get(key) || { key, kind, label, items: [], extra: 0, caseIds: new Set() }
    group.extra += remainder
    for (const entry of byCase) if (entry[kind] > 0) group.caseIds.add(entry.caseId)
    groups.set(key, group)
  }
  return [...groups.values()]
    .map(group => ({ ...group, caseIds: [...group.caseIds].sort(), count: group.items.length + group.extra }))
    .sort((left, right) => right.count - left.count || (left.kind === right.kind ? left.label.localeCompare(right.label) : left.kind === 'failed' ? -1 : 1))
}
