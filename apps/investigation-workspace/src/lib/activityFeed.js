// The case's working record as one feed: questions asked in this tab, questions saved in this browser, and the recent
// evidence entries the collection status reports. Everything here is a pure function of those three sources. It is a
// working record, not an audit history (no actor, custody or export events exist in the current service).

export const STATE_LABELS = {
  answered: 'Answered',
  partial: 'Answered with limits',
  'zero-result': 'Answered · no matches',
  clarify: 'Clarification requested',
  processing: 'Processing',
  unsupported: 'Unavailable',
  failed: 'Failed',
  missing: 'Retained copy missing',
}

const FAMILY_LABELS = { cdr: 'CDR', ipdr: 'IPDR', anpr: 'ANPR', tower: 'Tower and cell', document: 'Documents', audio: 'Audio', video: 'Video', image: 'Images', 'all evidence': 'All evidence' }

export function familyLabel(value) {
  if (!value) return 'Not reported'
  return FAMILY_LABELS[String(value).toLowerCase()] || String(value)
}

export function outcomeLabel(item) {
  if (item.kind === 'evidence') {
    if (item.state === 'answered') return 'Evidence ready'
    if (item.state === 'processing') return 'Evidence processing'
    if (item.state === 'failed') return 'Evidence failed'
    if (item.state === 'missing') return 'Retained copy missing'
    return 'Evidence status unavailable'
  }
  return STATE_LABELS[item.state] || 'Outcome not reported'
}

// Outcomes grouped for filtering and counting: what went well, what needs a look, what failed, what could not be said.
export const OUTCOMES = [
  { id: 'ok', label: 'Answered', states: ['answered', 'zero-result'] },
  { id: 'attention', label: 'Needs attention', states: ['partial', 'clarify', 'processing', 'missing'] },
  { id: 'failed', label: 'Failed', states: ['failed'] },
  { id: 'unavailable', label: 'Unavailable', states: ['unsupported'] },
]

export function outcomeOf(item) {
  return OUTCOMES.find(group => group.states.includes(item.state))?.id || 'unavailable'
}

function evidenceState(status) {
  if (status === 'completed') return 'answered'
  if (['queued', 'registered', 'processing', 'running'].includes(status)) return 'processing'
  if (status === 'failed') return 'failed'
  return 'unsupported'
}

function missingCopies(data, caseId) {
  const seen = new Set((data?.recent_evidence || []).map(item => item.evidence_id))
  return (data?.missing_kb_assets || []).map(entry => (typeof entry === 'string' ? { evidence_id: entry, source_file: entry } : entry))
    .filter(entry => entry?.evidence_id && !seen.has(entry.evidence_id))
    .map(entry => ({
      id: `missing-${entry.evidence_id}`,
      caseId,
      kind: 'evidence',
      sourceKind: 'evidence',
      sourceLabel: 'Collection status',
      evidenceId: entry.evidence_id,
      family: null,
      state: 'missing',
      title: `Retained copy is missing: ${entry.source_file || entry.evidence_id}`,
      answer: 'Processing finished but the retained copy of this file cannot be opened. No analysis from it can be traced back to the source until it is restored.',
      answerWithheld: true,
      scope: [{ label: 'Evidence ID', value: entry.evidence_id }],
      recordedAt: null,
    }))
}

export function evidenceActivities(data, caseId) {
  const jobs = new Map((data?.recent_jobs || []).map(job => [job.evidence_id, job]))
  const recent = (data?.recent_evidence || []).map(evidence => {
    const job = jobs.get(evidence.evidence_id) || {}
    const state = evidenceState(evidence.processing_status)
    const reprocessed = Number(job.attempt_count) > 1
    const hasCounts = Number.isFinite(Number(job.accepted_rows))
    const counts = hasCounts
      ? `${Number(job.accepted_rows).toLocaleString()} accepted; ${Number(job.rejected_rows || 0).toLocaleString()} rejected; ${Number(job.duplicate_rows || 0).toLocaleString()} duplicates.`
      : ''
    const family = evidence.detected_type || evidence.modality || null
    return {
      id: `evidence-${evidence.evidence_id}`,
      caseId,
      kind: 'evidence',
      sourceKind: 'evidence',
      sourceLabel: 'Recent collection status',
      evidenceId: evidence.evidence_id,
      family,
      state,
      title: `${reprocessed ? 'Evidence reprocessed' : 'Evidence added'}: ${evidence.source_file || evidence.evidence_id}`,
      answer: state === 'answered' ? `Processing completed.${counts ? ` ${counts}` : ''}` : state === 'processing' ? 'Evidence processing is not complete.' : state === 'failed' ? 'Evidence processing failed. No analysis result is implied.' : 'The collection status response did not report a recognized processing state.',
      answerWithheld: state !== 'answered',
      scope: [
        { label: 'Evidence family', value: familyLabel(family) },
        { label: 'Evidence ID', value: evidence.evidence_id },
      ],
      recordedAt: evidence.updated_at || evidence.created_at || null,
    }
  })
  return [...recent, ...missingCopies(data, caseId)]
}

export function browserHistoryActivities(history, sessionActivities, caseId) {
  const active = new Set(sessionActivities.map(item => `${item.caseId ?? caseId}|${item.query}`))
  return history.filter(item => !active.has(`${item.caseId ?? caseId}|${item.query}`)).map(item => ({
    id: `history-${item.caseId ?? caseId}-${item.id}`,
    caseId: item.caseId ?? caseId,
    kind: 'analysis',
    sourceKind: 'questions',
    sourceLabel: 'Saved in this browser',
    query: item.query,
    family: null,
    state: item.state || 'unsupported',
    title: item.label || item.query,
    answer: item.answer || 'Answer text was not retained.',
    answerWithheld: ['clarify', 'unsupported', 'failed', 'processing'].includes(item.state),
    scope: [{ label: 'Analysis scope', value: 'Not retained with this saved entry' }],
    recordedAt: item.updatedAt || null,
  }))
}

export function asTimestamp(value) {
  const time = value ? new Date(value).getTime() : Number.NaN
  return Number.isFinite(time) ? time : 0
}

export function buildActivities({ sessionActivities, questionHistory, overview, caseId }) {
  const currentTab = sessionActivities.map(item => ({ ...item, sourceKind: 'questions', sourceLabel: 'Current tab' }))
  return [...currentTab, ...browserHistoryActivities(questionHistory, sessionActivities, caseId), ...evidenceActivities(overview, caseId)]
    .sort((left, right) => asTimestamp(right.recordedAt) - asTimestamp(left.recordedAt))
}

// The same feed across every case: tab and browser questions of each case plus each case's evidence entries.
export function buildWorkspaceActivities({ sessionActivities, questionHistory, overviews }) {
  const currentTab = sessionActivities.map(item => ({ ...item, sourceKind: 'questions', sourceLabel: 'Current tab' }))
  const evidence = Object.entries(overviews || {}).flatMap(([caseId, data]) => (data ? evidenceActivities(data, caseId) : []))
  return [...currentTab, ...browserHistoryActivities(questionHistory, sessionActivities, ''), ...evidence].sort((left, right) => asTimestamp(right.recordedAt) - asTimestamp(left.recordedAt))
}

// Per case: how many entries, how many still need a look, and the latest dated one.
export function caseSummaries(items, caseIds, reviewed = new Set()) {
  return caseIds.map(caseId => {
    const mine = items.filter(item => item.caseId === caseId)
    const open = mine.filter(item => ['attention', 'failed'].includes(outcomeOf(item)) && !reviewed.has(`${item.caseId}|${item.id}`))
    const stamps = mine.map(item => asTimestamp(item.recordedAt)).filter(Boolean)
    return { caseId, total: mine.length, open: open.length, latest: stamps.length ? Math.max(...stamps) : 0 }
  })
}

export const reviewedKey = item => `${item.caseId}|${item.id}`
export const needsLook = item => ['attention', 'failed'].includes(outcomeOf(item))

const DAY = new Intl.DateTimeFormat('en-GB', { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' })
const CLOCK = new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit', hour12: false, timeZone: 'UTC' })

export function dayKey(value) {
  return asTimestamp(value) ? new Date(value).toISOString().slice(0, 10) : ''
}
export function dayLabel(key) {
  return key ? DAY.format(new Date(`${key}T00:00:00Z`)) : 'Time not reported'
}
export function clockOf(value) {
  return asTimestamp(value) ? `${CLOCK.format(new Date(value))} UTC` : 'Time not reported'
}
export function exactTime(value) {
  return asTimestamp(value) ? `${DAY.format(new Date(value))}, ${CLOCK.format(new Date(value))} UTC` : 'Time not reported'
}

// Entries in the order given, cut into calendar days (UTC); entries with no time share a last group.
export function groupByDay(items) {
  const groups = []
  for (const item of items) {
    const key = dayKey(item.recordedAt)
    let group = groups.find(entry => entry.key === key)
    if (!group) { group = { key, label: dayLabel(key), items: [] }; groups.push(group) }
    group.items.push(item)
  }
  return groups.sort((left, right) => (left.key === '' ? 1 : right.key === '' ? -1 : right.key.localeCompare(left.key)))
}

export function countOutcomes(items) {
  const counts = Object.fromEntries(OUTCOMES.map(group => [group.id, 0]))
  for (const item of items) counts[outcomeOf(item)] += 1
  return counts
}

// Entries per calendar day over the last `span` days ending on the latest dated entry, oldest first. Undated entries are
// not placed on any day.
export function dayCounts(items, span = 14) {
  const stamps = items.map(item => asTimestamp(item.recordedAt)).filter(Boolean)
  if (!stamps.length) return []
  const last = new Date(Math.max(...stamps)).toISOString().slice(0, 10)
  const days = Array.from({ length: span }, (_, index) => new Date(Date.parse(`${last}T00:00:00Z`) - (span - 1 - index) * 86_400_000).toISOString().slice(0, 10))
  const counts = new Map(days.map(day => [day, 0]))
  for (const item of items) { const key = dayKey(item.recordedAt); if (counts.has(key)) counts.set(key, counts.get(key) + 1) }
  return days.map(day => ({ day, count: counts.get(day) }))
}

export function activitiesCsv(items) {
  const quote = value => (/[",\n]/.test(String(value)) ? `"${String(value).replace(/"/g, '""')}"` : String(value))
  const rows = items.map(item => [item.recordedAt || '', item.sourceLabel, outcomeLabel(item), item.title, item.answerWithheld ? '' : item.answer].map(quote).join(','))
  return ['Time (UTC),Source,Outcome,Activity,Answer', ...rows].join('\n')
}
