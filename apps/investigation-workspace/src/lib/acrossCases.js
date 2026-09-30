import { formatNumber } from './format.js'

// One question, every case. The question service answers one case at a time, so the browser asks each eligible case
// (a few at once) and gathers the outcomes. Nothing is merged into a single invented answer: each case keeps its own
// verified result, and the summary says exactly which cases answered, which found nothing, which needed a choice and
// which could not be searched. A server-side multi-case scope is BACKEND_REQUESTS row 11.
export const CONCURRENCY = 3

// Best result first. A case that answered outranks one that needs a choice, which outranks a clean "no matches".
const ORDER = ['answered', 'partial', 'clarify', 'processing', 'zero-result', 'unsupported', 'failed']

export const OUTCOME_LABEL = {
  answered: 'Answered',
  partial: 'Answered in part',
  clarify: 'Needs a choice',
  processing: 'Still processing',
  'zero-result': 'No matches',
  unsupported: 'Not available',
  failed: 'Could not search',
}

// Which cases to ask. A case with no evidence has nothing to search, so it is skipped and named, never silently dropped.
// `chosen` is an optional narrowing; empty means every case.
export function planSearch(rows, chosen = []) {
  const picked = chosen.length ? new Set(chosen) : null
  const search = []
  const skipped = []
  for (const row of rows) {
    if (picked && !picked.has(row.caseId)) skipped.push({ caseId: row.caseId, reason: 'not_chosen' })
    else if (row.summary && row.summary.total === 0) skipped.push({ caseId: row.caseId, reason: 'no_evidence' })
    else search.push(row.caseId)
  }
  const ready = new Map(rows.map(row => [row.caseId, row.summary?.ready || 0]))
  search.sort((left, right) => (ready.get(right) || 0) - (ready.get(left) || 0))
  return { search, skipped }
}

export function sortOutcomes(outcomes) {
  return [...outcomes].sort((left, right) => ORDER.indexOf(left.presentation.state) - ORDER.indexOf(right.presentation.state) || left.caseId.localeCompare(right.caseId))
}

export function tallyOutcomes(outcomes) {
  const counts = Object.fromEntries(ORDER.map(state => [state, 0]))
  for (const outcome of outcomes) counts[outcome.presentation.state] = (counts[outcome.presentation.state] || 0) + 1
  return { counts, found: counts.answered + counts.partial }
}

// One sentence for the top of the results: how many cases had something, out of how many were searched.
export function outcomeHeadline(tally, searched) {
  if (!searched) return 'No case could be searched.'
  if (tally.found === 0) return tally.counts.clarify ? `No answer yet: ${formatNumber(tally.counts.clarify)} ${tally.counts.clarify === 1 ? 'case needs' : 'cases need'} a choice from you.` : `Nothing found in ${searched === 1 ? 'the case' : `any of the ${formatNumber(searched)} cases`} searched.`
  return `Found in ${formatNumber(tally.found)} of ${formatNumber(searched)} ${searched === 1 ? 'case' : 'cases'}.`
}

// Ask every case, `concurrency` at a time. `ask(caseId)` resolves to a response or rejects; `present` turns either into a
// presentation. Outcomes are reported as they arrive so the page can show progress, and aborting stops new requests and
// resolves with what has arrived. A failed case is an outcome, not an exception: the others still answer.
export async function askAcrossCases({ caseIds, ask, present, concurrency = CONCURRENCY, signal, onOutcome }) {
  const outcomes = []
  let next = 0
  async function worker() {
    while (next < caseIds.length && !signal?.aborted) {
      const caseId = caseIds[next]
      next += 1
      let outcome
      try {
        outcome = { caseId, presentation: present(await ask(caseId), caseId, null) }
      } catch (error) {
        if (error?.name === 'AbortError') return
        outcome = { caseId, presentation: present(null, caseId, error) }
      }
      outcomes.push(outcome)
      onOutcome?.(outcome, outcomes.length)
    }
  }
  await Promise.all(Array.from({ length: Math.min(concurrency, caseIds.length) }, worker))
  return outcomes
}
