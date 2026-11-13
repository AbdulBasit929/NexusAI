import { describe, expect, it } from 'vitest'
import { composition, dayLabel, displayName, failureReason, formatDuration, groupByDay, itemFacts, kindFromLegacyFamily, kindOf, statusCounts, statusOf } from './evidenceModel.js'

describe('the file kinds', () => {
  it('names the kinds of the service and gives an unknown one a quiet tone, never an error', () => {
    expect(kindOf('audio').label).toBe('Audio')
    expect(kindOf('STRUCTURED_RECORDS').one).toBe('Records')
    expect(kindOf('hologram')).toMatchObject({ id: 'hologram', label: 'Other', tone: 'muted' })
    expect(kindOf(undefined).label).toBe('Other')
  })

  it('keeps old links that said family=audio working, and nothing else', () => {
    expect(['audio', 'image', 'video', 'document'].map(kindFromLegacyFamily)).toEqual(['audio', 'image', 'video', 'document'])
    expect(kindFromLegacyFamily('cdr')).toBe('')
    expect(kindFromLegacyFamily(null)).toBe('')
  })
})

describe('the composition bar', () => {
  it('lists the kinds the case holds, biggest first, with their shares of all files', () => {
    const parts = composition({ audio: 10, document: 4, image: 37, structured_records: 42, tabular: 3, text: 4, video: 5, empty: 0 })
    expect(parts.map(part => [part.id, part.count])).toEqual([['structured_records', 42], ['image', 37], ['audio', 10], ['video', 5], ['document', 4], ['text', 4], ['tabular', 3]])
    expect(parts.reduce((sum, part) => sum + part.share, 0)).toBeCloseTo(1, 10)
    expect(parts[0].share).toBeCloseTo(42 / 105, 10)
  })

  it('breaks a tie by the order of the kinds, and an empty or missing count draws nothing', () => {
    expect(composition({ text: 4, document: 4 }).map(part => part.id)).toEqual(['document', 'text'])
    expect(composition(null)).toEqual([])
    expect(composition({ audio: 0 })).toEqual([])
  })
})

describe('what a row says about a file', () => {
  it('gives a recording its length and format, from the service\'s own reading of the file', () => {
    expect(itemFacts({ modality: 'audio', media_metadata: { duration_ms: 7789, channels: 1, sample_rate_hz: 16000 }, accepted_rows: 2 })).toEqual({ main: '0:08', note: 'mono 16 kHz' })
    expect(itemFacts({ modality: 'audio', media_metadata: { duration_ms: 3_725_000, channels: 2, sample_rate_hz: 44100 } })).toEqual({ main: '1:02:05', note: 'stereo 44.1 kHz' })
  })

  it('gives a video its length and whether it has a sound track', () => {
    expect(itemFacts({ modality: 'video', media_metadata: { duration_ms: 60010, audio_track_count: 1 } })).toEqual({ main: '1:00', note: 'with sound' })
    expect(itemFacts({ modality: 'video', media_metadata: { duration_ms: 5000, audio_track_count: 0 } })).toEqual({ main: '0:05', note: 'no sound' })
    expect(itemFacts({ modality: 'video', media_metadata: { duration_ms: 5000 } })).toEqual({ main: '0:05', note: null })
  })

  it('gives a picture its size in pixels and a table its rows, with the rows that were not accepted', () => {
    expect(itemFacts({ modality: 'image', media_metadata: { width_pixels: 1920, height_pixels: 1080, format: 'jpeg' } })).toEqual({ main: '1,920 × 1,080', note: 'JPEG' })
    expect(itemFacts({ modality: 'structured_records', accepted_rows: 181, total_rows: 181 })).toEqual({ main: '181 rows', note: null })
    expect(itemFacts({ modality: 'structured_records', accepted_rows: 181, total_rows: 190 })).toEqual({ main: '181 rows', note: 'of 190' })
    expect(itemFacts({ modality: 'tabular', accepted_rows: 1 })).toEqual({ main: '1 row', note: null })
    expect(itemFacts({ modality: 'document', accepted_rows: 4 })).toEqual({ main: '4 passages', note: null })
  })

  it('never shows the artifact count of a recording, a picture or a video as if it were rows', () => {
    expect(itemFacts({ modality: 'audio', accepted_rows: 2, media_metadata: {} })).toBeNull()
    expect(itemFacts({ modality: 'image', accepted_rows: 5 })).toBeNull()
    expect(itemFacts({ modality: 'video', accepted_rows: 1152 })).toBeNull()
  })

  it('says nothing rather than guess when the service reported nothing', () => {
    expect(itemFacts({ modality: 'structured_records' })).toBeNull()
    expect(itemFacts(null)).toBeNull()
  })
})

describe('durations, statuses and days', () => {
  it('formats a length as minutes and seconds, with hours only when needed, and refuses a number that is not a time', () => {
    expect(formatDuration(0)).toBe('0:00')
    expect(formatDuration(59_600)).toBe('1:00')
    expect(formatDuration(3_600_000)).toBe('1:00:00')
    expect(formatDuration(undefined)).toBeNull()
    expect(formatDuration(-5)).toBeNull()
  })

  it('maps the service\'s processing states to three honest words and keeps an unknown one visible', () => {
    expect(statusOf({ processing_status: 'completed' })).toEqual({ key: 'ready', label: 'Ready' })
    expect(statusOf({ processing_status: 'queued' })).toEqual({ key: 'processing', label: 'Queued' })
    expect(statusOf({ processing_status: 'running' })).toEqual({ key: 'processing', label: 'Processing' })
    expect(statusOf({ processing_status: 'failed' }).key).toBe('failed')
    expect(statusOf({ processing_status: 'dead_letter' }).key).toBe('failed')
    expect(statusOf({ processing_status: 'half_done' })).toEqual({ key: 'unknown', label: 'half done' })
    expect(statusOf({})).toEqual({ key: 'unknown', label: 'Not reported' })
  })

  it('counts the case for the status chips: queued and processing are one chip', () => {
    expect(statusCounts({ evidence_total: 105, completed: 100, processing: 3, queued: 1, failed: 1 })).toEqual({ all: 105, ready: 100, processing: 4, failed: 1 })
    expect(statusCounts({ evidence_total: 105, status_counts: { completed: 105 } })).toEqual({ all: 105, ready: 105, processing: 0, failed: 0 })
    expect(statusCounts(null)).toEqual({ all: 0, ready: 0, processing: 0, failed: 0 })
  })

  it('groups the service\'s newest-first list by day, naming today and yesterday', () => {
    const now = new Date('2026-11-13T10:00:00Z')
    const groups = groupByDay([
      { evidence_id: 'a', created_at: '2026-11-13T08:55:12Z' },
      { evidence_id: 'b', created_at: '2026-11-13T07:00:00Z' },
      { evidence_id: 'c', created_at: '2026-11-12T23:59:00Z' },
      { evidence_id: 'd', created_at: '2026-11-03T12:00:00Z' },
      { evidence_id: 'e' },
    ], now)
    expect(groups.map(group => [group.label, group.items.map(item => item.evidence_id)])).toEqual([['Today', ['a', 'b']], ['Yesterday', ['c']], ['3 Nov 2026', ['d']], ['Date not reported', ['e']]])
    expect(dayLabel('2026-11-13', now)).toBe('Today')
  })
})

describe('names and reasons', () => {
  it('never shows a blank name', () => {
    expect(displayName({ original_filename: 'a.wav', evidence_id: 'x' })).toBe('a.wav')
    expect(displayName({ source_file: 's.csv' })).toBe('s.csv')
    expect(displayName({ evidence_id: 'x' })).toBe('x')
    expect(displayName({})).toBe('Unnamed file')
  })

  it('takes the first reason a file failed, whatever shape the service gave it', () => {
    expect(failureReason({ errors: ['bad header'] })).toBe('bad header')
    expect(failureReason({ errors: [{ message: 'no rows' }] })).toBe('no rows')
    expect(failureReason({ errors: [] })).toBe('')
    expect(failureReason({})).toBe('')
  })
})
