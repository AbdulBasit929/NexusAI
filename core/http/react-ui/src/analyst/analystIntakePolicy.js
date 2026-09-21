export function governedASRLanguage(value) {
  return String(value || '').trim().toLowerCase() === 'ur' ? 'ur' : ''
}

export function asrLanguageUploadMetadata(value) {
  const language = governedASRLanguage(value)
  return language ? { asr_language: language } : {}
}

export function intakeCapabilitySummary(capabilities, limit = 18) {
  const formats = new Set()
  for (const family of Array.isArray(capabilities?.families) ? capabilities.families : []) {
    if (String(family?.support_level || '').toLowerCase() === 'planned') continue
    for (const format of Array.isArray(family?.formats) ? family.formats : []) {
      const normalized = String(format || '').trim().toLowerCase()
      if (normalized && !/unknown|unsupported|mixed export|binary dump/.test(normalized)) formats.add(normalized)
    }
  }
  const values = [...formats].sort((left, right) => left.localeCompare(right))
  if (!values.length) return 'Supported formats are reported by the server after this workspace finishes loading.'
  const visible = values.slice(0, Math.max(1, limit)).map(value => value.toUpperCase())
  const remainder = Math.max(0, values.length - visible.length)
  return `Server-declared formats: ${visible.join(', ')}${remainder ? `, and ${remainder} more` : ''}. Processing depth varies by evidence family.`
}
