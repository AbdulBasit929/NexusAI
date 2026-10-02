// Small, pure helpers behind the evidence viewers: time text, find-in-document, zoom limits, waveform peaks and the key/value
// card for a structured record. Nothing here reads the DOM, so each is tested on its own.

// 65 seconds is "1:05", 3723 seconds is "1:02:03". Negative or non-finite input reads as 0:00.
export function formatClock(seconds) {
  const total = Number.isFinite(Number(seconds)) && Number(seconds) > 0 ? Math.floor(Number(seconds)) : 0
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const rest = String(total % 60).padStart(2, '0')
  return hours ? `${hours}:${String(minutes).padStart(2, '0')}:${rest}` : `${minutes}:${rest}`
}

const MAX_MATCHES = 500

function escapeRegex(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

// Every place the query occurs in the text, case-insensitively, as [start, end) offsets. An empty query has no matches, and
// a runaway query is capped so a one-letter search on a long page stays fast.
export function findMatches(text, query) {
  const needle = String(query ?? '').trim()
  if (!needle || !text) return []
  const pattern = new RegExp(escapeRegex(needle), 'giu')
  const matches = []
  for (const found of String(text).matchAll(pattern)) {
    matches.push({ start: found.index, end: found.index + found[0].length })
    if (matches.length >= MAX_MATCHES) break
  }
  return matches
}

// The text cut into runs: plain text, and each match tagged with its index among the page's matches.
export function splitByMatches(text, matches) {
  const parts = []
  let cursor = 0
  matches.forEach((match, index) => {
    if (match.start > cursor) parts.push({ text: text.slice(cursor, match.start), match: false })
    parts.push({ text: text.slice(match.start, match.end), match: true, index })
    cursor = match.end
  })
  if (cursor < text.length) parts.push({ text: text.slice(cursor), match: false })
  return parts
}

// How many times the query occurs on each page, keyed by page number, so the page list can show where to look.
export function pageMatchCounts(pages, query) {
  const counts = new Map()
  for (const page of pages) counts.set(page.number, findMatches(page.text, query).length)
  return counts
}

export const ZOOM_MIN = 0.1
export const ZOOM_MAX = 8

export function clampZoom(value) {
  const number = Number(value)
  if (!Number.isFinite(number)) return 1
  return Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, number))
}

// The scale at which an image just fits its frame, never enlarging a small image past its own size.
export function fitScale(frame, natural) {
  if (!frame?.width || !frame?.height || !natural?.width || !natural?.height) return 1
  return Math.min(1, frame.width / natural.width, frame.height / natural.height)
}

// A waveform as `buckets` peaks between 0 and 1: the loudest sample in each slice of the channel.
export function peaksFrom(samples, buckets) {
  const count = Math.max(1, Math.floor(buckets))
  if (!samples?.length) return Array.from({ length: count }, () => 0)
  const size = samples.length / count
  const raw = Array.from({ length: count }, (_, index) => {
    let peak = 0
    const from = Math.floor(index * size)
    const to = Math.min(samples.length, Math.max(from + 1, Math.floor((index + 1) * size)))
    for (let position = from; position < to; position += 1) peak = Math.max(peak, Math.abs(samples[position]))
    return peak
  })
  const loudest = Math.max(...raw, 1e-9)
  return raw.map(value => value / loudest)
}

// A record as label/value pairs for the inspector, in column order, leaving out fields with nothing in them.
export function recordFields(row, columns) {
  return columns
    .filter(column => !String(column.key).startsWith('__'))
    .map(column => ({ key: column.key, label: column.label || column.key, value: row?.[column.key] }))
    .filter(field => field.value !== null && field.value !== undefined && field.value !== '')
}

// A region box as fractions of the image (x, y, width, height), whether the service sent fractions or pixels.
export function normalizeBox(box, size) {
  if (!Array.isArray(box) || box.length < 4 || box.some(value => !Number.isFinite(Number(value)))) return null
  const [x, y, width, height] = box.map(Number)
  if (Math.max(x, y, width, height) <= 1) return [x, y, width, height]
  if (!size?.width || !size?.height) return null
  return [x / size.width, y / size.height, width / size.width, height / size.height]
}

// The zoom and offset that bring one region to the middle of the frame, with room around it. Offsets are in frame pixels
// from the frame's centre, so the caller applies them to an image that is centred at zoom 1.
export function focusRegion(box, natural, frame) {
  const normalized = normalizeBox(box, natural)
  if (!normalized || !natural?.width || !frame?.width) return null
  const [x, y, width, height] = normalized
  const zoom = clampZoom(Math.min(frame.width / (Math.max(width, 0.02) * natural.width), frame.height / (Math.max(height, 0.02) * natural.height)) * 0.55)
  return { zoom, x: -((x + width / 2) - 0.5) * natural.width * zoom, y: -((y + height / 2) - 0.5) * natural.height * zoom }
}

// The cue that is speaking at `time`, or -1: the last cue that started, if it has not ended.
export function activeCueIndex(cues, time) {
  let found = -1
  cues.forEach((cue, index) => { if (time >= cue.start && (cue.end == null || time < cue.end)) found = index })
  return found
}

// Brings one element into view by scrolling its own scroll container only. `scrollIntoView` would also scroll the page and
// every ancestor, which jumps the whole screen when a viewer opens.
export function scrollWithin(container, element, align = 'nearest', smooth = false) {
  if (!container || !element) return
  const box = container.getBoundingClientRect()
  const target = element.getBoundingClientRect()
  const top = target.top - box.top + container.scrollTop
  let next = null
  if (align === 'center') next = top - (container.clientHeight - target.height) / 2
  else if (target.top < box.top) next = top
  else if (target.bottom > box.bottom) next = top - container.clientHeight + target.height
  if (next === null) return
  const options = { top: Math.max(0, next), behavior: smooth ? 'smooth' : 'auto' }
  if (container.scrollTo) container.scrollTo(options); else container.scrollTop = options.top
}
