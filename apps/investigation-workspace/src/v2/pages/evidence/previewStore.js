// The pictures on the faces of the evidence cards, made from the files themselves: a small copy of a photo, the real waveform of a recording, the first rows of a table, the first passage of a
// document. Nothing is drawn that was not read from the file: a kind that cannot be read cheaply (a video, a huge file) gets no picture and the card says so with its facts instead.
//
// Rules: one read at a time per kind of work is not enough, so at most four run together (a case can hold hundreds of files and the service is one machine); a card that scrolled away before its turn
// is dropped from the line; a finished picture is kept (the most recent 400) and its memory given back when it is pushed out.

import { getEvidence, getEvidenceContent } from '../../../lib/apiClient.js'
import { peaksFrom } from '../../../lib/viewerTools.js'
import { kindOf } from './evidenceModel.js'

export const THUMB_EDGE = 480
export const WAVE_BARS = 56
export const MAX_AUDIO_BYTES = 8 * 1024 * 1024
export const MAX_IMAGE_BYTES = 14 * 1024 * 1024
const CONCURRENCY = 4

// ---- pure parts (tested) ----

export function fitWithin(width, height, edge) {
  const w = Number(width)
  const h = Number(height)
  if (!(w > 0) || !(h > 0)) return { width: edge, height: edge }
  const scale = Math.min(1, edge / Math.max(w, h))
  return { width: Math.max(1, Math.round(w * scale)), height: Math.max(1, Math.round(h * scale)) }
}

const ARABIC = /[؀-ۿݐ-ݿﭐ-﷿ﹰ-﻿]/g
const LETTER = /\p{L}/gu
export function textDirection(text) {
  const letters = String(text || '').match(LETTER)?.length || 0
  const arabic = String(text || '').match(ARABIC)?.length || 0
  return letters && arabic / letters > 0.4 ? 'rtl' : 'ltr'
}

const clip = (value, size) => { const text = String(value ?? '').replace(/\s+/g, ' ').trim(); return text.length > size ? `${text.slice(0, size - 1)}…` : text }
const readable = key => String(key).replace(/^_+/, '').replace(/[_-]+/g, ' ').trim()

// The first rows of a table file as the service stored them (the file's own columns, in the file's own order): up to three columns that hold a value in the first row, and up to three rows.
export function rowsPreview(records, { maxColumns = 3, maxRows = 3 } = {}) {
  const sources = (records || []).map(record => record?.raw_payload).filter(payload => payload && typeof payload === 'object' && !Array.isArray(payload))
  if (!sources.length) return null
  const first = sources[0]
  const columns = Object.keys(first).filter(key => !key.startsWith('_') && String(first[key] ?? '').trim() !== '').slice(0, maxColumns)
  if (!columns.length) return null
  return { columns: columns.map(readable), rows: sources.slice(0, maxRows).map(payload => columns.map(key => clip(payload[key], 22))) }
}

// The first passages of a document or text file, in reading order (up to three, each cut to a line or two of a card): what the file starts with.
export function passagePreview(detail, { maxLines = 3, maxChars = 140 } = {}) {
  const passages = (detail?.derived_artifacts || []).filter(artifact => String(artifact?.artifact_type || '').includes('passage') && typeof artifact?.metadata?.text === 'string' && artifact.metadata.text.trim())
  if (!passages.length) return null
  const order = artifact => [Number(artifact.citation_locator?.paragraph ?? artifact.citation_locator?.section ?? 0), Number(artifact.citation_locator?.passage ?? 0)]
  passages.sort((a, b) => { const [pa, qa] = order(a); const [pb, qb] = order(b); return pa - pb || qa - qb })
  const lines = passages.slice(0, maxLines).map(artifact => clip(artifact.metadata.text, maxChars)).filter(Boolean)
  return lines.length ? { lines, dir: textDirection(lines.join(' ')) } : null
}

// A small, bounded cache: the most recently used keys stay; what is pushed out is given to `onEvict` (to free a picture's memory).
export class Lru {
  constructor(max, onEvict = () => {}) { this.max = max; this.onEvict = onEvict; this.map = new Map() }
  get(key) { if (!this.map.has(key)) return undefined; const value = this.map.get(key); this.map.delete(key); this.map.set(key, value); return value }
  has(key) { return this.map.has(key) }
  set(key, value) {
    if (this.map.has(key)) this.onEvict(this.map.get(key), key)
    this.map.delete(key)
    this.map.set(key, value)
    while (this.map.size > this.max) { const [oldKey, oldValue] = this.map.entries().next().value; this.map.delete(oldKey); this.onEvict(oldValue, oldKey) }
  }
  get size() { return this.map.size }
}

// A line with a limit: at most `limit` jobs run together; a job whose `skip()` says so when its turn comes is not run.
export function createQueue(limit) {
  const waiting = []
  let running = 0
  const next = () => {
    while (running < limit && waiting.length) {
      const job = waiting.shift()
      if (job.skip?.()) { job.resolve(null); continue }
      running += 1
      Promise.resolve().then(job.task).then(job.resolve, job.reject).finally(() => { running -= 1; next() })
    }
  }
  return { run: (task, skip) => new Promise((resolve, reject) => { waiting.push({ task, skip, resolve, reject }); next() }), get waiting() { return waiting.length }, get running() { return running } }
}

// ---- the work (a browser) ----

const cache = new Lru(400, value => { if (value?.url) globalThis.URL?.revokeObjectURL?.(value.url) })
const inflight = new Map()
const queue = createQueue(CONCURRENCY)
let audioContext = null

export const previewKey = (caseId, item) => `${caseId}|${item.evidence_id}|${item.sha256 || item.updated_at || ''}`
export const cachedPreview = (caseId, item) => cache.get(previewKey(caseId, item))

async function readPhoto(caseId, item, signal) {
  if (Number(item.size_bytes) > MAX_IMAGE_BYTES) return { kind: 'none' }
  const blob = await getEvidenceContent({ caseId, evidenceId: item.evidence_id, signal })
  const bitmap = await globalThis.createImageBitmap(blob)
  const { width, height } = fitWithin(bitmap.width, bitmap.height, THUMB_EDGE)
  const canvas = globalThis.document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  canvas.getContext('2d').drawImage(bitmap, 0, 0, width, height)
  bitmap.close?.()
  const small = await new Promise(resolve => canvas.toBlob(resolve, 'image/jpeg', 0.82))
  return small ? { kind: 'image', url: globalThis.URL.createObjectURL(small), width, height } : { kind: 'none' }
}

async function readWave(caseId, item, signal) {
  const Context = globalThis.AudioContext || globalThis.webkitAudioContext
  if (!Context || Number(item.size_bytes) > MAX_AUDIO_BYTES) return { kind: 'none' }
  const blob = await getEvidenceContent({ caseId, evidenceId: item.evidence_id, signal })
  audioContext = audioContext || new Context()
  const decoded = await audioContext.decodeAudioData(await blob.arrayBuffer())
  return { kind: 'wave', peaks: peaksFrom(decoded.getChannelData(0), WAVE_BARS) }
}

async function readDetail(caseId, item, signal) {
  const detail = await getEvidence({ caseId, evidenceId: item.evidence_id, limit: 3, signal })
  const kind = kindOf(item.modality).id
  if (kind === 'document' || kind === 'text') { const passage = passagePreview(detail); return passage ? { kind: 'text', ...passage } : { kind: 'none' } }
  const rows = rowsPreview(detail?.records_preview)
  return rows ? { kind: 'rows', ...rows } : { kind: 'none' }
}

export function loadPreview({ caseId, item, signal }) {
  const key = previewKey(caseId, item)
  if (cache.has(key)) return Promise.resolve(cache.get(key))
  if (inflight.has(key)) return inflight.get(key)
  const kind = kindOf(item.modality).id
  const work = kind === 'image' ? readPhoto : kind === 'audio' ? readWave : kind === 'video' ? null : readDetail
  if (!work) return Promise.resolve({ kind: 'none' })
  const promise = queue.run(async () => {
    try { return await work(caseId, item, signal) } catch (error) { if (error?.name === 'AbortError') return null; return { kind: 'none' } }
  }, () => signal?.aborted).then(result => { if (result) cache.set(key, result); return result }).finally(() => inflight.delete(key))
  inflight.set(key, promise)
  return promise
}
