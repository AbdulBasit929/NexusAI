// What the audio viewer shows for a recording, read from the evidence detail the service returns.
//
// The worker stores ONE derived artifact per spoken segment (type forensics.audio-timestamp-segment/v1): its start and end are in
// citation_locator, its text, language, model and, for the languages that have a speech route, its reliability state and the figures
// measured on public speech are in metadata.observation. (The viewer used to look for a list of cues inside a single artifact and so found
// no transcript at all for anything the worker had read.) A detail that carries a list of cues instead still works.
import { transcriptCues } from './viewerPresentation.js'

export const MACHINE_TRANSCRIPT = 'MACHINE_TRANSCRIPT'
export const SEARCH_LEAD = 'SEARCH_LEAD'
export const LOW_RESOURCE_LANGUAGE = 'LOW_RESOURCE_LANGUAGE'

// From the most to the least trustworthy; the status of a recording is the least trustworthy status of its segments.
const ORDER = [MACHINE_TRANSCRIPT, SEARCH_LEAD, LOW_RESOURCE_LANGUAGE]

function finite(value) {
  if (value == null || value === '') return null
  const number = Number(value)
  return Number.isFinite(number) ? number : null
}

function object(value) {
  if (!value) return {}
  if (typeof value === 'object') return value
  try { return JSON.parse(value) } catch { return {} }
}

function text(value) { return value == null || typeof value === 'object' ? '' : String(value).trim() }

export function normalizeStatus(value) {
  const state = text(value).toUpperCase()
  return ORDER.includes(state) ? state : MACHINE_TRANSCRIPT
}

export function worstStatus(statuses) {
  let worst = 0
  for (const status of statuses) worst = Math.max(worst, ORDER.indexOf(normalizeStatus(status)))
  return ORDER[worst]
}

function segmentOf(artifact) {
  const metadata = object(artifact?.metadata)
  const observation = object(metadata.observation)
  const locator = object(artifact?.citation_locator)
  const start = finite(locator.start_seconds ?? observation.start_seconds)
  const content = text(observation.text)
  if (start === null || !content) return null
  const route = object(observation.route)
  return {
    id: text(artifact.artifact_id) || `segment-${start}`,
    start,
    end: finite(locator.end_seconds ?? observation.end_seconds),
    text: content,
    language: text(observation.language).toLowerCase() || null,
    status: observation.reliability_state ? normalizeStatus(observation.reliability_state) : null,
    model: text(observation.model) || null,
    confidence: finite(artifact.confidence ?? observation.confidence),
    measured: route.measured && typeof route.measured === 'object' ? route.measured : null,
    complete: observation.reading_complete !== false,
    audioSeconds: finite(observation.audio_seconds_total),
  }
}

// { segments, status, language, model, measured, complete, readUntil, audioSeconds, routed }
export function audioSegments(detail) {
  const found = []
  const seen = new Set()
  for (const artifact of detail?.derived_artifacts || []) {
    if (!/audio-timestamp-segment/.test(text(artifact?.artifact_type))) continue
    const segment = segmentOf(artifact)
    if (!segment) continue
    const key = `${segment.start}|${segment.end}|${segment.text}`
    if (seen.has(key)) continue                      // a reprocessed recording keeps the earlier reading beside the new one; an identical segment is shown once
    seen.add(key)
    found.push(segment)
  }
  let segments = found.sort((a, b) => a.start - b.start)
  if (!segments.length) {
    segments = transcriptCues(detail).map(cue => ({ id: cue.id, start: cue.start, end: cue.end, text: cue.text, language: null, status: null, model: null, confidence: null, measured: null, complete: true, audioSeconds: null, speaker: cue.speaker }))
  }
  const first = segments[0] || {}
  const incomplete = segments.some(segment => !segment.complete)
  const status = worstStatus(segments.map(segment => segment.status))
  return {
    segments,
    status,
    routed: segments.some(segment => segment.status !== null),
    language: first.language || null,
    model: first.model || null,
    measured: segments.map(segment => segment.measured).find(Boolean) || null,
    complete: !incomplete,
    readUntil: incomplete ? Math.max(...segments.map(segment => segment.end ?? segment.start)) : null,
    audioSeconds: segments.map(segment => segment.audioSeconds).find(value => value !== null) ?? null,
  }
}

const percent = value => `${Math.round(Number(value) * 100)}%`

// The banner above the player: a label, one sentence, and the figures that were measured (only those that are recorded).
export function speechNote(status, measured, { complete = true, readUntil = null } = {}) {
  const wrong = measured && finite(measured.word_error) !== null ? Math.max(1, Math.min(9, Math.round(measured.word_error * 10))) : null
  const found = measured && finite(measured.word_recall) !== null ? percent(measured.word_recall) : null
  const facts = []
  if (wrong !== null) facts.push(`About ${wrong} words in 10 are wrong`)
  if (found !== null) facts.push(`${found} of spoken words are found`)
  if (measured?.tier) facts.push('Measured on public speech')
  let note
  if (status === SEARCH_LEAD) {
    note = { status, tone: 'caution', label: 'Search lead', sentence: 'Good enough to find this recording by a spoken word, not to read as a transcript. Listen before you rely on any word.', facts }
  } else if (status === LOW_RESOURCE_LANGUAGE) {
    note = { status, tone: 'danger', label: 'Pointer to listen', sentence: 'This language is read poorly. Use the text to know where to listen, not what was said.', facts }
  } else {
    note = { status: MACHINE_TRANSCRIPT, tone: 'plain', label: 'Machine transcript', sentence: 'Check it against the recording before you rely on it.', facts }
  }
  return { ...note, incomplete: complete ? null : `Reading stopped at ${clock(readUntil)}; the rest of the recording is not read.` }
}

function clock(seconds) {
  const total = Math.max(0, Math.floor(Number(seconds) || 0))
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`
}
