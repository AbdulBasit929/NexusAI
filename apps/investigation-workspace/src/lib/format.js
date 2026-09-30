export function formatNumber(value) {
  const number = Number(value)
  return Number.isFinite(number) ? number.toLocaleString() : '—'
}

export function formatBytes(value) {
  const bytes = Number(value)
  if (!Number.isFinite(bytes)) return '—'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 ** 2).toFixed(1)} MB`
}

export function displayName(value) {
  return String(value || 'Unknown').replaceAll('_', ' ').replace(/\b\w/g, letter => letter.toUpperCase())
}

// A column name the catalog has no label for, in words: `accepted_rows` reads "Accepted rows", `lastObservedAt` reads "Last
// observed at". Only used as a fallback, so a curated label always wins and an unknown name is never shown as engine jargon.
export function humanizeKey(key) {
  const text = String(key ?? '').trim()
  if (!text || /^m\d+$/i.test(text)) return text
  const spaced = text.replace(/([a-z0-9])([A-Z])/g, '$1 $2').replace(/[_-]+/g, ' ').replace(/\s+/g, ' ').trim().toLocaleLowerCase()
  return spaced.charAt(0).toLocaleUpperCase() + spaced.slice(1)
}
