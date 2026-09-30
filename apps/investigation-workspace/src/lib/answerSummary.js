import { formatNumber } from './format.js'

// The four facts an analyst checks before trusting an answer, read straight from the presentation: how many source files
// it rests on, how many rows stand behind it, how strong that evidence is called, and what scope it was asked in. An
// answer with no openable source says so plainly instead of leaving the slot out. Every value is a count or label the
// service reported; nothing is estimated.
export function answerSummary(presentation) {
  const groups = presentation?.citations?.groups || []
  const items = []
  if (groups.length) {
    const openable = presentation.citations.totalOpenable || presentation.citations.items.length
    items.push({ id: 'sources', label: 'Sources', value: `${formatNumber(groups.length)} ${groups.length === 1 ? 'file' : 'files'}`, detail: `${formatNumber(openable)} openable ${openable === 1 ? 'location' : 'locations'}` })
  } else {
    items.push({ id: 'sources', label: 'Sources', value: 'None openable', detail: 'No source location came with this answer', tone: 'warn' })
  }
  const contributing = presentation?.citations?.totalContributing
  const result = presentation?.result
  if (contributing) items.push({ id: 'rows', label: 'Rows behind it', value: formatNumber(contributing), detail: '' })
  else if (result?.rows?.length) items.push({ id: 'rows', label: 'Rows shown', value: formatNumber(result.availableRows), detail: result.truncated ? `of ${formatNumber(result.totalRows)}` : 'all of them' })
  if (groups.length) {
    const counts = new Map()
    for (const group of groups) counts.set(group.strength?.label || 'Strength unavailable', (counts.get(group.strength?.label || 'Strength unavailable') || 0) + 1)
    const [top, rest] = [[...counts.entries()].sort((left, right) => right[1] - left[1])[0], counts.size - 1]
    items.push({ id: 'strength', label: 'Evidence', value: top[0], detail: rest > 0 ? `and ${formatNumber(rest)} other ${rest === 1 ? 'kind' : 'kinds'}` : `${formatNumber(top[1])} of ${formatNumber(groups.length)} ${groups.length === 1 ? 'file' : 'files'}` })
  }
  const scope = (presentation?.scope || []).slice(0, 2)
  if (scope.length) items.push({ id: 'scope', label: 'Asked in', value: scope.map(chip => chip.value).join(' · '), detail: scope.map(chip => chip.label).join(' · ') })
  return items
}

// A file's share of the rows behind the answer, floored so a part is never shown as the whole. Null when the service did
// not report how many rows each file contributed.
export function contributionShare(group) {
  if (!group?.contributingCount || !group?.totalContributing) return null
  return Math.min(100, Math.floor((group.contributingCount / group.totalContributing) * 100))
}
