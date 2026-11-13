// What the Evidence list shows about each file, as pure functions (tested). Everything here is a field the evidence service returned: nothing is estimated.
//
// The file kinds are the service's own `modality` values. Seven kinds are more than the six data colours of the design system, so the seventh (spreadsheets, the smallest) takes the muted tone.

export const KINDS = [
  { id: 'structured_records', label: 'Records', one: 'Records', tone: '1' },
  { id: 'image', label: 'Images', one: 'Image', tone: '3' },
  { id: 'audio', label: 'Audio', one: 'Audio', tone: '4' },
  { id: 'video', label: 'Video', one: 'Video', tone: '2' },
  { id: 'document', label: 'Documents', one: 'Document', tone: '5' },
  { id: 'text', label: 'Text', one: 'Text', tone: '6' },
  { id: 'tabular', label: 'Spreadsheets', one: 'Spreadsheet', tone: 'muted' },
]

const BY_ID = Object.fromEntries(KINDS.map(kind => [kind.id, kind]))

export function kindOf(modality) {
  const id = String(modality || '').toLowerCase()
  return BY_ID[id] || { id, label: 'Other', one: 'Other', tone: 'muted' }
}

// Links written before the kinds existed carried `family=audio|image|video|document`: they still land on the same files.
export function kindFromLegacyFamily(family) {
  const id = String(family || '').toLowerCase()
  return ['audio', 'image', 'video', 'document'].includes(id) ? id : ''
}

// The bar: one segment for every kind the case holds, the biggest first (the order of KINDS breaks a tie), each with its share of all files.
export function composition(modalityCounts) {
  const counts = modalityCounts && typeof modalityCounts === 'object' ? modalityCounts : {}
  const entries = Object.entries(counts).map(([id, value]) => [id, Number(value) || 0]).filter(([, value]) => value > 0)
  const total = entries.reduce((sum, [, value]) => sum + value, 0)
  const order = id => { const at = KINDS.findIndex(kind => kind.id === id); return at < 0 ? KINDS.length : at }
  return entries
    .map(([id, count]) => ({ ...kindOf(id), id, count, share: total ? count / total : 0 }))
    .sort((a, b) => b.count - a.count || order(a.id) - order(b.id))
}

export function formatDuration(ms) {
  const value = Number(ms)
  if (ms == null || !Number.isFinite(value) || value < 0) return null
  const total = Math.round(value / 1000)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const seconds = total % 60
  const two = value => String(value).padStart(2, '0')
  return hours ? `${hours}:${two(minutes)}:${two(seconds)}` : `${minutes}:${two(seconds)}`
}

const whole = value => (Number.isFinite(Number(value)) ? Number(value) : null)
const plural = (count, word, many = `${word}s`) => `${count.toLocaleString('en-GB')} ${count === 1 ? word : many}`

// The one thing worth knowing about a file at a glance, from the service's own reading of it: how long a recording is, how big a picture, how many rows a table has.
// Media report their length or size; their "accepted rows" count derived artifacts, not anything an analyst would call a row, so it is not shown for them.
export function itemFacts(item) {
  const media = item?.media_metadata || {}
  const kind = kindOf(item?.modality).id
  if (kind === 'audio') {
    const length = formatDuration(media.duration_ms)
    const channels = whole(media.channels)
    const rate = whole(media.sample_rate_hz)
    const detail = [channels === 1 ? 'mono' : channels === 2 ? 'stereo' : null, rate ? `${(rate / 1000).toLocaleString('en-GB', { maximumFractionDigits: 1 })} kHz` : null].filter(Boolean).join(' ')
    return length ? { main: length, note: detail || null } : null
  }
  if (kind === 'video') {
    const length = formatDuration(media.duration_ms)
    const sound = whole(media.audio_track_count)
    return length ? { main: length, note: sound === null ? null : sound > 0 ? 'with sound' : 'no sound' } : null
  }
  if (kind === 'image') {
    const width = whole(media.width_pixels)
    const height = whole(media.height_pixels)
    return width && height ? { main: `${width.toLocaleString('en-GB')} × ${height.toLocaleString('en-GB')}`, note: media.format ? String(media.format).toUpperCase() : null } : null
  }
  const accepted = whole(item?.accepted_rows)
  const total = whole(item?.total_rows)
  if (accepted === null) return null
  if (kind === 'document' || kind === 'text') return { main: plural(accepted, 'passage'), note: null }
  const of = total !== null && total > accepted ? `of ${total.toLocaleString('en-GB')}` : null
  return { main: plural(accepted, 'row'), note: of }
}

export function statusOf(item) {
  const raw = String(item?.processing_status || '').toLowerCase()
  if (raw === 'failed' || raw === 'dead_letter') return { key: 'failed', label: 'Failed' }
  if (['registered', 'queued', 'processing', 'running'].includes(raw)) return { key: 'processing', label: raw === 'queued' || raw === 'registered' ? 'Queued' : 'Processing' }
  if (raw === 'completed') return { key: 'ready', label: 'Ready' }
  return { key: 'unknown', label: raw ? raw.replaceAll('_', ' ') : 'Not reported' }
}

// The case-wide counts for the status chips, from the service's summary (queued and processing are one chip: both mean "not ready yet").
export function statusCounts(summary) {
  const s = summary || {}
  const ready = Number(s.completed ?? s.status_counts?.completed ?? 0)
  const failed = Number(s.failed ?? s.status_counts?.failed ?? 0)
  const processing = Number(s.processing ?? 0) + Number(s.queued ?? 0)
  return { all: Number(s.evidence_total ?? ready + failed + processing), ready, processing, failed }
}

const pad = value => String(value).padStart(2, '0')
export function dayKey(iso) {
  const date = new Date(iso)
  if (!iso || Number.isNaN(date.getTime())) return ''
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`
}

export function dayLabel(key, now = new Date()) {
  if (!key) return 'Date not reported'
  const today = dayKey(now.toISOString())
  const yesterday = dayKey(new Date(now.getTime() - 86_400_000).toISOString())
  if (key === today) return 'Today'
  if (key === yesterday) return 'Yesterday'
  const date = new Date(`${key}T00:00:00Z`)
  return new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' }).format(date)
}

// The list keeps the service's order (newest first); a new group starts when the day changes.
export function groupByDay(items, now = new Date()) {
  const groups = []
  for (const item of items || []) {
    const key = dayKey(item.created_at)
    const last = groups.at(-1)
    if (last && last.key === key) last.items.push(item)
    else groups.push({ key, label: dayLabel(key, now), items: [item] })
  }
  return groups
}

export const shortId = id => String(id || '').slice(0, 8)

// A file name that fails to load never shows as a blank cell.
export function displayName(item) {
  return item?.original_filename || item?.source_file || item?.evidence_id || 'Unnamed file'
}

// Why a file failed, in the service's words, for the title of its status.
export function failureReason(item) {
  const first = (item?.errors || [])[0]
  if (!first) return ''
  if (typeof first === 'string') return first
  return String(first.message || first.error || first.code || '')
}
