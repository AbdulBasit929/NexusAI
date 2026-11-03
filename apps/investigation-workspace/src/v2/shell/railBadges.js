// Counts the sidebar and the activity menu can show, from the same collection-status responses the Dashboard reads. Kept as a pure function
// so it is tested without drawing anything.
export function railBadges(attention, caseId, caseState) {
  const badges = {}
  if (attention.needReview > 0) badges['/'] = { count: attention.needReview, tone: 'attention', label: `${attention.needReview} ${attention.needReview === 1 ? 'case needs' : 'cases need'} review` }
  if (attention.processing > 0) badges['/cases'] = { count: attention.processing, tone: 'processing', label: `${attention.processing} ${attention.processing === 1 ? 'case is' : 'cases are'} processing` }
  const failed = Number(caseState?.data?.summary?.evidence_failed || 0)
  if (caseId && failed > 0) badges[`/cases/${encodeURIComponent(caseId)}/evidence`] = { count: failed, tone: 'attention', label: `${failed} failed ${failed === 1 ? 'source' : 'sources'}` }
  return badges
}
