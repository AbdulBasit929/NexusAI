import { resolveEvidenceModality } from '../utils/evidenceModality.js'

export const mediaArtifactTypes = Object.freeze({
  anpr: 'forensics.anpr-observation/v1',
  videoAnprGroup: 'forensics.video-anpr-plate-group/v1',
  face: 'forensics.face-observation/v1',
  ocr: 'forensics.image-ocr-observation/v1',
  imageEmbedding: 'forensics.image-embedding-observation/v1',
  audioTechnical: 'forensics.audio-observation/v1',
  audioSegment: 'forensics.audio-timestamp-segment/v1',
  romanUrdu: 'forensics.audio-roman-urdu-segment/v1',
})

export function artifactObservation(artifact) {
  return artifact?.metadata?.observation || artifact?.metadata || {}
}

export function sourceMediaKind(item = {}) {
  const kind = resolveEvidenceModality(item).kind
  return ['image', 'audio', 'video'].includes(kind) ? kind : ''
}

export function artifactBounds(artifact) {
  const observation = artifactObservation(artifact)
  return observation.bbox
    || artifact?.citation_locator?.bbox
    || artifact?.metadata?.candidate?.bounds
    || artifact?.metadata?.crop?.bounds
    || artifact?.metadata?.face_crop?.bounds
    || null
}

export function artifactTimestamp(artifact) {
  const observation = artifactObservation(artifact)
  const value = observation.frame_timestamp_seconds
    ?? artifact?.citation_locator?.timestamp_seconds
    ?? observation.first_seen_seconds
    ?? artifact?.citation_locator?.first_seen_seconds
    ?? observation.start_seconds
    ?? artifact?.citation_locator?.start_seconds
  if (value === null || value === undefined || typeof value === 'boolean' || String(value).trim() === '') return null
  const seconds = Number(value)
  return Number.isFinite(seconds) && seconds >= 0 ? seconds : null
}

export function romanUrduBelongsToTranscript(roman, transcript) {
  const parent = artifactObservation(roman).parent_observation_id
  const source = transcript?.metadata?.observation_id || transcript?.artifact_id
  // Equal (or missing) timestamps do not establish derivative lineage.
  return Boolean(parent && source && parent === source)
}

export function meaningfulOCRArtifacts(artifacts = []) {
  return artifacts.filter(artifact => {
    if (artifact?.artifact_type !== mediaArtifactTypes.ocr) return false
    // Whitespace-only output is not an observation. Low-confidence, short, or
    // unusual-script model output remains visible and explicitly review-bound.
    return String(artifactObservation(artifact).raw_text || '').trim().length > 0
  })
}

export function groupMediaArtifacts(artifacts = []) {
  const completed = artifacts.filter(artifact => !artifact?.processing_status || artifact.processing_status === 'completed')
  return {
    plates: completed.filter(artifact => artifact.artifact_type === mediaArtifactTypes.anpr),
    plateGroups: completed.filter(artifact => artifact.artifact_type === mediaArtifactTypes.videoAnprGroup),
    faces: completed.filter(artifact => artifact.artifact_type === mediaArtifactTypes.face),
    ocr: meaningfulOCRArtifacts(completed),
    imageEmbeddings: completed.filter(artifact => artifact.artifact_type === mediaArtifactTypes.imageEmbedding),
    audio: completed.filter(artifact => artifact.artifact_type === mediaArtifactTypes.audioSegment),
    romanUrdu: completed.filter(artifact => artifact.artifact_type === mediaArtifactTypes.romanUrdu),
  }
}

export function audioDetailPresentation(item = {}, artifacts = []) {
  const groups = groupMediaArtifacts(artifacts)
  const technical = artifacts.find(artifact => artifact?.artifact_type === mediaArtifactTypes.audioTechnical)
  const observation = artifactObservation(technical)
  const probe = observation.container_probe || item.media_metadata?.container_probe || item.media_metadata || {}
  const format = probe.format || {}
  const stream = Array.isArray(probe.streams) ? probe.streams.find(entry => entry?.codec_type === 'audio') || {} : {}
  const validSeconds = value => {
    if (value === null || value === undefined || typeof value === 'boolean' || String(value).trim() === '') return null
    const number = Number(value)
    return Number.isFinite(number) && number >= 0 ? number : null
  }
  const milliseconds = validSeconds(item.media_metadata?.duration_ms)
  const duration = validSeconds(format.duration ?? probe.duration_seconds ?? item.duration_seconds)
    ?? (milliseconds === null ? null : milliseconds / 1000)
  const firstSegment = groups.audio.length ? artifactObservation(groups.audio[0]) : {}
  const language = firstSegment.requested_language || firstSegment.language || firstSegment.detected_language || item.language || item.metadata?.language || ''
  const model = firstSegment.model || firstSegment.processor || ''
  const formatLabel = String(stream.codec_long_name || stream.codec_name || format.format_long_name || format.format_name || item.mime_type || '').trim()
  return {
    durationSeconds: duration,
    format: formatLabel,
    language: String(language || '').trim(),
    model: String(model || '').trim(),
    transcriptSegmentCount: groups.audio.length,
    romanUrduSegmentCount: groups.romanUrdu.length,
    asrStatus: groups.audio.length > 0
      ? 'Transcript available — analyst review required'
      : String(item.processing_status || '').toLowerCase() === 'completed'
        ? 'Processing complete — no transcript recorded'
        : 'Transcript not available',
  }
}

export function sourceProcessingState(item, jobs = [], artifacts = []) {
  const status = String(item?.processing_status || '').toLowerCase()
  const latest = jobs[0] || {}
  const latestStatus = String(latest.status || '').toLowerCase()
  if (['failed', 'dead_letter'].includes(status) || ['failed', 'dead_letter'].includes(latestStatus)) {
    return { id: 'FAILED', label: 'Processing failed', tone: 'danger' }
  }
  if (['processing', 'queued', 'retrying', 'registered'].includes(status) || ['processing', 'queued', 'retrying'].includes(latestStatus)) {
    return { id: 'PROCESSING', label: 'Processing in progress', tone: 'warning' }
  }
  if (status === 'completed') {
    return artifacts.length > 0
      ? { id: 'COMPLETE_RESULTS', label: 'Processing complete — results available', tone: 'success' }
      : { id: 'COMPLETE_ZERO_RESULTS', label: 'Processing complete — no derived observations', tone: 'neutral' }
  }
  if (!status) return { id: 'NOT_RUN', label: 'Processing state not reported', tone: 'neutral' }
  return { id: 'UNAVAILABLE', label: 'Processing unavailable', tone: 'neutral' }
}

export function faceSimilarityEligible(artifact) {
  if (artifact?.artifact_type !== mediaArtifactTypes.face) return false
  const observation = artifactObservation(artifact)
  return String(observation.embedding_status || '').toLowerCase() !== 'unavailable'
    && Number(observation.embedding_dimension || 0) > 0
    && Array.isArray(observation.embedding)
    && observation.embedding.length > 0
}

export function mediaQuickActions(item, groups) {
  const filename = String(item?.original_filename || item?.source_file || 'this source').split(/[\\/]/).pop()
  const actions = []
  const seenPlates = new Set()
  if (sourceMediaKind(item) === 'video' && item?.evidence_id) {
    actions.push({
      id: `video-anpr:${item.evidence_id}`,
      icon: 'fa-timeline',
      label: 'Show grouped video plates',
      prompt: `Show grouped video ANPR observations for retained evidence ${item.evidence_id}. Use exact source timestamps, cite every returned group, and report a completed zero result truthfully.`,
    })
  }
  for (const artifact of groups.plates) {
    const observation = artifactObservation(artifact)
    const plate = String(observation.normalized_plate_text || observation.raw_plate_text || '').trim()
    if (!plate || seenPlates.has(plate)) continue
    seenPlates.add(plate)
    actions.push({
      id: `plate:${plate}`,
      icon: 'fa-car-side',
      label: `Find ${plate}`,
      prompt: `Find exact ANPR sightings for ${plate} in this workspace and cite every matching source.`,
    })
  }
  const firstOCR = groups.ocr.find(artifact => String(artifactObservation(artifact).raw_text || '').trim().length >= 2)
  if (firstOCR) {
    const text = String(artifactObservation(firstOCR).raw_text).trim()
    actions.push({
      id: `ocr:${firstOCR.artifact_id}`,
      icon: 'fa-font',
      label: 'Find this recognized text',
      prompt: `Find the exact recognized text ${JSON.stringify(text)} in retained derived observations, focused on ${filename}, and cite the matching artifact and source.`,
    })
  }
  const firstAudio = groups.audio[0]
  if (firstAudio) {
    const text = String(artifactObservation(firstAudio).text || '').trim()
    if (text) actions.push({
      id: `transcript:${firstAudio.artifact_id}`,
      icon: 'fa-waveform-lines',
      label: 'Find this transcript phrase',
      prompt: `Find the transcript phrase ${JSON.stringify(text)} in ${filename} and cite the matching time range and retained source.`,
    })
  }
  return actions
}

export function formatTimestamp(seconds) {
  if (seconds === null || seconds === undefined || typeof seconds === 'boolean' || String(seconds).trim() === '') return 'Time unavailable'
  const value = Number(seconds)
  if (!Number.isFinite(value) || value < 0) return 'Time unavailable'
  const minutes = Math.floor(value / 60)
  const remainder = value - (minutes * 60)
  const [wholeSeconds, fraction = ''] = remainder.toFixed(remainder % 1 ? 1 : 0).split('.')
  return `${minutes}:${wholeSeconds.padStart(2, '0')}${fraction ? `.${fraction}` : ''}`
}
