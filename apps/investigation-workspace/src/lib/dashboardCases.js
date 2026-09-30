import { formatNumber } from './format.js'

// Pure helpers behind the dashboard's case table and question cards. Nothing here fetches; each takes what the
// page already holds.

export function parseTimestamp(value) {
  const date = value ? new Date(value) : null
  return date && !Number.isNaN(date.getTime()) ? date : null
}

// The newest thing a case's status response dates: an evidence update or a job start/finish.
export function latestEvidenceActivity(data) {
  const candidates = [
    ...(data?.recent_evidence || []).map(item => ({ label: item.source_file, date: parseTimestamp(item.updated_at || item.created_at) })),
    ...(data?.recent_jobs || []).map(item => ({ label: item.source_file, date: parseTimestamp(item.completed_at || item.started_at || item.queued_at) })),
  ].filter(item => item.date)
  candidates.sort((left, right) => right.date.getTime() - left.date.getTime())
  return candidates[0] || null
}

// "3 h ago", floored so a stale figure is never made to look fresher than it is.
export function relativeAge(date, now = Date.now()) {
  const seconds = Math.max(0, Math.floor((now - date.getTime()) / 1000))
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${formatNumber(minutes)} min ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 48) return `${formatNumber(hours)} h ago`
  const days = Math.floor(hours / 24)
  return days < 60 ? `${formatNumber(days)} d ago` : `${formatNumber(Math.floor(days / 30))} mo ago`
}

export function dashboardNextAction(summary, caseId) {
  const encoded = encodeURIComponent(caseId)
  if (summary.processingState === 'attention') return { label: 'Review evidence', to: `/cases/${encoded}/evidence` }
  if (summary.processingState === 'processing') return { label: 'View processing', to: `/cases/${encoded}/evidence` }
  if (summary.processingState === 'complete') return { label: 'Investigate', to: `/cases/${encoded}/investigate` }
  return { label: 'Add evidence', to: `/cases/${encoded}/evidence` }
}

const forbiddenQuestion = /\b(?:model|backend|agent|operation|template|sql|embedding|token|deterministic|forensic query|record_type|limit\s*=)\b/i

// Suggested questions come from the service's own corpus, limited to families the case can actually answer, and any
// that read like engine talk are dropped.
export function curatedQuestions(payload) {
  const available = new Set((payload?.families || [])
    .filter(family => ['queryable', 'semantic_only', 'limited'].includes(family?.availability))
    .map(family => family.id))
  const seen = new Set()
  return (payload?.query_corpus?.entries || [])
    .filter(entry => entry?.suggested !== false && available.has(entry?.family_id))
    .map(entry => ({ query: String(entry.query || '').trim(), familyId: entry.family_id }))
    .filter(entry => !forbiddenQuestion.test(entry.query))
    .filter(entry => {
      const key = entry.query.toLocaleLowerCase()
      if (!key || seen.has(key)) return false
      seen.add(key)
      return true
    })
    .slice(0, 4)
}
