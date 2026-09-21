import assert from 'node:assert/strict'
import test from 'node:test'
import {
  audioDetailPresentation,
  artifactBounds,
  artifactTimestamp,
  faceSimilarityEligible,
  formatTimestamp,
  groupMediaArtifacts,
  mediaQuickActions,
  mediaArtifactTypes,
  sourceProcessingState,
  sourceMediaKind,
  romanUrduBelongsToTranscript,
} from './analystMediaPresentation.js'

const artifact = (type, observation = {}, extra = {}) => ({
  artifact_id: `${type}-id`,
  artifact_type: type,
  processing_status: 'completed',
  metadata: { observation },
  ...extra,
})

test('recognizes the retained ANPR v1 contract and preserves real confidence', () => {
  const plate = artifact(mediaArtifactTypes.anpr, { normalized_plate_text: 'MN1367', ocr_confidence: 0.9998, bbox: { x: 1, y: 2, width: 3, height: 4 } })
  const groups = groupMediaArtifacts([plate])
  assert.equal(groups.plates.length, 1)
  assert.equal(groups.plates[0].metadata.observation.normalized_plate_text, 'MN1367')
  assert.deepEqual(artifactBounds(plate), { x: 1, y: 2, width: 3, height: 4 })
})

test('filters only blank OCR and retains low-confidence review output', () => {
  const blank = artifact(mediaArtifactTypes.ocr, { raw_text: '   ', confidence: 0.99 })
  const low = artifact(mediaArtifactTypes.ocr, { raw_text: '—', confidence: 0 })
  assert.deepEqual(groupMediaArtifacts([blank, low]).ocr, [low])
})

test('distinguishes completed zero results from not run and failure', () => {
  assert.equal(sourceProcessingState({ processing_status: 'completed' }, [], []).id, 'COMPLETE_ZERO_RESULTS')
  assert.equal(sourceProcessingState({}, [], []).id, 'NOT_RUN')
  assert.equal(sourceProcessingState({ processing_status: 'completed' }, [{ status: 'dead_letter' }], []).id, 'FAILED')
})

test('requires a real embedding before offering face similarity', () => {
  const missing = artifact(mediaArtifactTypes.face, { embedding_status: 'unavailable', embedding_dimension: 0, embedding: [] })
  const ready = artifact(mediaArtifactTypes.face, { embedding_status: 'available', embedding_dimension: 2, embedding: [0.1, 0.2] })
  assert.equal(faceSimilarityEligible(missing), false)
  assert.equal(faceSimilarityEligible(ready), true)
})

test('reads frame timestamp from the governed locator', () => {
  const frame = artifact(mediaArtifactTypes.ocr, { raw_text: 'اسلام آباد' }, { citation_locator: { timestamp_seconds: 5 } })
  assert.equal(artifactTimestamp(frame), 5)
})

test('keeps raw video plate sightings while exposing bounded UI groups', () => {
  const raw = artifact(mediaArtifactTypes.anpr, { normalized_plate_text: 'MW51VSU' }, {
    metadata: { observation_id: 'plate-1', observation: { normalized_plate_text: 'MW51VSU' } },
    citation_locator: { timestamp_seconds: 20 },
  })
  const grouped = artifact(mediaArtifactTypes.videoAnprGroup, {
    normalized_plate_text: 'MW51VSU', sightings_count: 2, first_seen_seconds: 20,
    last_seen_seconds: 21, best_observation_id: 'plate-1',
  })
  const groups = groupMediaArtifacts([raw, grouped])
  assert.equal(groups.plates.length, 1)
  assert.equal(groups.plateGroups.length, 1)
  assert.equal(artifactTimestamp(grouped), 20)
})

test('offers one evidence-scoped grouped-video ANPR action even for completed zero results', () => {
  const actions = mediaQuickActions({ modality: 'video', evidence_id: '50057921-4f1f-4ab8-bab0-47bcdc957822' }, groupMediaArtifacts([]))
  assert.equal(actions.length, 1)
  assert.equal(actions[0].id, 'video-anpr:50057921-4f1f-4ab8-bab0-47bcdc957822')
  assert.match(actions[0].prompt, /completed zero result truthfully/)
})

test('formats sub-minute media timestamps with two-digit seconds', () => {
  assert.equal(formatTimestamp(6.24), '0:06.2')
  assert.equal(formatTimestamp(65), '1:05')
})

test('missing and invalid media times are not presented as zero', () => {
  for (const value of [null, undefined, '', ' ', false, true, -1, NaN, Infinity, 'invalid']) {
    assert.equal(artifactTimestamp({ citation_locator: { start_seconds: value } }), null)
    assert.equal(formatTimestamp(value), 'Time unavailable')
  }
  assert.equal(artifactTimestamp({ citation_locator: { start_seconds: 0 } }), 0)
  assert.equal(formatTimestamp(0), '0:00')
})

test('Roman Urdu pairs by parent observation, never equal or absent time', () => {
  const first = artifact(mediaArtifactTypes.audioSegment, { start_seconds: null }, { artifact_id: 'first' })
  const second = artifact(mediaArtifactTypes.audioSegment, { start_seconds: null }, { artifact_id: 'second' })
  const roman = artifact(mediaArtifactTypes.romanUrdu, { parent_observation_id: 'second', start_seconds: null })
  assert.equal(romanUrduBelongsToTranscript(roman, first), false)
  assert.equal(romanUrduBelongsToTranscript(roman, second), true)
  assert.equal(romanUrduBelongsToTranscript(artifact(mediaArtifactTypes.romanUrdu, { start_seconds: 0 }), artifact(mediaArtifactTypes.audioSegment, { start_seconds: 0 })), false)
})

test('derives audio presentation from governed metadata before file extension fallback', () => {
  assert.equal(sourceMediaKind({ modality: 'audio', original_filename: 'misleading.bin' }), 'audio')
  assert.equal(sourceMediaKind({ mime_type: 'audio/wav', original_filename: 'unknown.bin' }), 'audio')
  assert.equal(sourceMediaKind({ original_filename: 'fallback.wav' }), 'audio')
  assert.equal(sourceMediaKind({ modality: 'document', original_filename: 'notes.txt' }), '')
})

test('reports audio readiness from completed transcript artifacts', () => {
  const technical = artifact(mediaArtifactTypes.audioTechnical, {
    container_probe: {
      format: { duration: '6.24', format_name: 'wav' },
      streams: [{ codec_type: 'audio', codec_name: 'pcm_s16le' }],
    },
  })
  const transcript = artifact(mediaArtifactTypes.audioSegment, {
    text: 'اسلام آباد', requested_language: 'ur', model: 'faster-whisper-small-ur',
  })
  const facts = audioDetailPresentation({ modality: 'audio', processing_status: 'completed' }, [technical, transcript])
  assert.equal(facts.durationSeconds, 6.24)
  assert.equal(facts.format, 'pcm_s16le')
  assert.equal(facts.language, 'ur')
  assert.equal(facts.model, 'faster-whisper-small-ur')
  assert.equal(facts.transcriptSegmentCount, 1)
  assert.equal(facts.asrStatus, 'Transcript available — analyst review required')
})

test('audio duration uses deterministic milliseconds and never coerces unknown to zero', () => {
  assert.equal(audioDetailPresentation({ media_metadata: { duration_ms: 6540 } }).durationSeconds, 6.54)
  for (const value of [null, undefined, '', ' ', false, true, -1, Infinity]) {
    assert.equal(audioDetailPresentation({ duration_seconds: value }).durationSeconds, null)
  }
  assert.equal(audioDetailPresentation({ duration_seconds: 0 }).durationSeconds, 0)
})
