// Known sets on the Evidence page (watch lists of pictures, hashes only; design and gates in docs/data/image-known-sets-preregistration-20261124.md).
// What the server says is a list of flags: one per picture and set, with the tier ("identical" file or "same_image" re-saved or resized). This file turns it into what the page shows.

export const KIND_LABEL = { of_interest: 'Watch list', known_harmless: 'Known harmless' }

// evidence id -> the flags of that picture
export function flagsByEvidence(response) {
  const map = new Map()
  for (const flag of response?.flags || []) {
    if (!flag?.evidence_id) continue
    const list = map.get(flag.evidence_id) || []
    list.push(flag)
    map.set(flag.evidence_id, list)
  }
  return map
}

// What a card says about its flags: a watch list always wins over "known harmless" (a picture in both is something to look at), and every set name is kept for the tooltip.
export function flagSummary(flags) {
  if (!flags?.length) return null
  const watch = flags.filter(flag => flag.set_kind === 'of_interest')
  const harmless = flags.filter(flag => flag.set_kind === 'known_harmless')
  const lead = watch.length ? watch : harmless
  const tone = watch.length ? 'watch' : 'harmless'
  const names = [...new Set(lead.map(flag => flag.set_name))]
  const exact = lead.some(flag => flag.tier === 'identical')
  const how = exact ? 'the same file' : 'the same picture, saved or resized'
  const all = [...new Set(flags.map(flag => flag.set_name))]
  return {
    tone,
    label: names.length === 1 ? names[0] : `${names[0]} +${names.length - 1}`,
    kindLabel: KIND_LABEL[lead[0].set_kind] || 'Known set',
    title: `${KIND_LABEL[lead[0].set_kind] || 'Known set'}: ${all.join(', ')} (${how})`,
  }
}

// The pictures of the page minus the ones that only known-harmless sets flagged, when the viewer asked to hide them. A picture on any watch list is never hidden.
export function applyHide(items, byEvidence, hide) {
  if (!hide) return { visible: items, hidden: [] }
  const visible = []
  const hidden = []
  for (const item of items) {
    const flags = byEvidence.get(item.evidence_id)
    const onlyHarmless = flags?.length && flags.every(flag => flag.set_kind === 'known_harmless')
    ;(onlyHarmless ? hidden : visible).push(item)
  }
  return { visible, hidden }
}

// The pictures of the list that any active set flags: the "Flagged" chip, the analyst's way to see the hits first. It works on the page that is listed.
export function onlyFlagged(items, byEvidence) {
  return items.filter(item => byEvidence.get(item.evidence_id)?.length)
}

// How many pictures of the list a hide would remove (for the label of the switch).
export function harmlessCount(items, byEvidence) {
  return applyHide(items, byEvidence, true).hidden.length
}

export function toggled(selection, id) {
  const next = new Set(selection)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  return next
}

// A short sentence for the result of adding hashes.
export function addedSentence(result) {
  const parts = []
  const added = Number(result?.added || 0)
  parts.push(`${added} ${added === 1 ? 'hash' : 'hashes'} added`)
  if (result?.already_in_set) parts.push(`${result.already_in_set} already in the set`)
  if (result?.rejected_count) parts.push(`${result.rejected_count} ${result.rejected_count === 1 ? 'row' : 'rows'} could not be used`)
  if (result?.evidence_not_found) parts.push(`${result.evidence_not_found} ${result.evidence_not_found === 1 ? 'file is' : 'files are'} not a picture of this case`)
  return `${parts.join(', ')}.`
}
