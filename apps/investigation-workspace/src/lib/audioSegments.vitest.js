import { describe, expect, it } from 'vitest'
import { audioSegments, LOW_RESOURCE_LANGUAGE, MACHINE_TRANSCRIPT, SEARCH_LEAD, speechNote, worstStatus } from './audioSegments.js'

function artifact(id, start, end, observation, type = 'forensics.audio-timestamp-segment/v1') {
  return { artifact_id: id, artifact_type: type, citation_locator: { source_file: 'a.wav', start_seconds: start, end_seconds: end }, metadata: { observation } }
}
const route = { status: SEARCH_LEAD, measured: { word_error: 0.303, word_recall: 0.649, tier: 'public' } }

describe('audioSegments', () => {
  it('reads one artifact per segment, in time order, with language, status and the measured figures', () => {
    const detail = { derived_artifacts: [
      artifact('b', 33.13, 38.96, { text: 'second', language: 'ps', reliability_state: SEARCH_LEAD, model: 'omni', route }),
      artifact('a', 0, 33.13, { text: 'first', language: 'ps', reliability_state: SEARCH_LEAD, model: 'omni', route }),
      artifact('t', 0, null, { technical: true }, 'forensics.audio-observation/v1'),
    ] }
    const result = audioSegments(detail)
    expect(result.segments.map(segment => segment.text)).toEqual(['first', 'second'])
    expect(result.segments[0]).toMatchObject({ start: 0, end: 33.13, language: 'ps', status: SEARCH_LEAD })
    expect(result).toMatchObject({ status: SEARCH_LEAD, routed: true, language: 'ps', complete: true })
    expect(result.measured.word_recall).toBe(0.649)
  })

  it('shows an identical segment once and skips a segment without text or start', () => {
    const same = { text: 'same', language: 'ur' }
    const detail = { derived_artifacts: [artifact('1', 0, 5, same), artifact('2', 0, 5, same), artifact('3', 6, 7, { text: '' }), { artifact_type: 'forensics.audio-timestamp-segment/v1', metadata: { observation: { text: 'no time' } }, citation_locator: {} }] }
    expect(audioSegments(detail).segments).toHaveLength(1)
  })

  it('a Whisper reading has no reliability state and is a machine transcript', () => {
    const result = audioSegments({ derived_artifacts: [artifact('1', 0, 8.88, { text: 'اردو', language: 'ur', model: 'faster-whisper-turbo' })] })
    expect(result).toMatchObject({ status: MACHINE_TRANSCRIPT, routed: false })
  })

  it('the status of a recording is the least trustworthy status of its segments', () => {
    expect(worstStatus([SEARCH_LEAD, MACHINE_TRANSCRIPT])).toBe(SEARCH_LEAD)
    expect(worstStatus([SEARCH_LEAD, LOW_RESOURCE_LANGUAGE, null])).toBe(LOW_RESOURCE_LANGUAGE)
    expect(worstStatus([])).toBe(MACHINE_TRANSCRIPT)
  })

  it('says where a reading stopped when the time budget ended it', () => {
    const detail = { derived_artifacts: [artifact('1', 0, 38, { text: 'x', language: 'ps', reliability_state: SEARCH_LEAD, reading_complete: false })] }
    const result = audioSegments(detail)
    expect(result).toMatchObject({ complete: false, readUntil: 38 })
    expect(speechNote(result.status, null, result).incomplete).toBe('Reading stopped at 0:38; the rest of the recording is not read.')
  })

  it('still reads a list of cues inside one artifact', () => {
    const detail = { derived_artifacts: [{ artifact_id: 'x', artifact_type: 'transcript', metadata: { segments: [{ start: 1, end: 2, text: 'cue one' }] } }] }
    expect(audioSegments(detail).segments).toMatchObject([{ start: 1, text: 'cue one', status: null }])
  })
})

describe('speechNote', () => {
  it('a search lead says it is not a transcript and carries only the figures that were measured', () => {
    const note = speechNote(SEARCH_LEAD, route.measured)
    expect(note).toMatchObject({ label: 'Search lead', tone: 'caution' })
    expect(note.facts).toEqual(['About 3 words in 10 are wrong', '65% of spoken words are found', 'Measured on public speech'])
    expect(speechNote(SEARCH_LEAD, null).facts).toEqual([])
  })

  it('a low-resource reading is a pointer to listen', () => {
    const note = speechNote(LOW_RESOURCE_LANGUAGE, { word_error: 0.67, word_recall: 0.361 })
    expect(note).toMatchObject({ label: 'Pointer to listen', tone: 'danger' })
    expect(note.facts[0]).toBe('About 7 words in 10 are wrong')
  })

  it('anything else is a machine transcript to check', () => {
    expect(speechNote(null, null)).toMatchObject({ label: 'Machine transcript', tone: 'plain', incomplete: null })
  })
})
