// The case identifier scopes every request, so it is chosen, never silently rewritten. These helpers only inform the
// choice: which rules the text already meets, what a valid version of it could be (offered, not applied), and whether the
// identifier already belongs to a case in this workspace.
export const MIN_LENGTH = 4
export const MAX_LENGTH = 64

export function caseNameRules(value) {
  return [
    { id: 'chars', label: 'Lowercase letters, numbers and hyphens only', met: value.length > 0 && /^[a-z0-9-]+$/.test(value) },
    { id: 'length', label: `${MIN_LENGTH} to ${MAX_LENGTH} characters`, met: value.length >= MIN_LENGTH && value.length <= MAX_LENGTH },
    { id: 'edges', label: 'Starts and ends with a letter or number', met: /^[a-z0-9](.*[a-z0-9])?$/.test(value) },
  ]
}

// A valid identifier close to what was typed: accents dropped, lowercase, other runs become one hyphen, ends trimmed. Null
// when nothing usable is left or the typed text already is valid, so a suggestion is only offered when it would change something.
export function suggestCaseName(value) {
  const text = String(value || '')
  const cleaned = text
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, MAX_LENGTH)
    .replace(/-+$/g, '')
  if (cleaned.length < MIN_LENGTH || cleaned === text) return null
  return cleaned
}

export function existingCase(value, caseIds) {
  return caseIds.find(id => id === value) || null
}
