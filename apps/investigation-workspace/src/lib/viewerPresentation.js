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

function artifactLabel(type) {
  const known = {
    transcript: 'Transcript', ocr: 'OCR text', ocr_region: 'OCR text region', anpr: 'Plate observation',
    plate_observation: 'Plate observation', frame: 'Extracted frame', thumbnail: 'Preview image',
  }
  return known[String(type || '').toLocaleLowerCase()] || String(type || 'Derived artifact')
}

function artifactMetadata(artifact) { return object(artifact?.metadata) }

function recordedText(value) {
  if (value == null || typeof value === 'object') return ''
  return String(value).trim()
}

function artifactProducer(artifact, runs) {
  const metadata = artifactMetadata(artifact)
  const observation = object(metadata.observation)
  const run = runs.find(item => recordedText(item?.run_id) === recordedText(artifact?.run_id)) || {}
  const candidates = [
    [run.model_id, run.model_revision],
    [metadata.model_id, metadata.model_revision ?? metadata.model_version],
    [observation.model_id, observation.model_revision ?? observation.model_version],
    [metadata.backend_id ?? metadata.backend, metadata.backend_revision ?? metadata.backend_version],
    [observation.backend_id ?? observation.backend, observation.backend_revision ?? observation.backend_version],
    [run.adapter_id, run.adapter_revision],
    [metadata.producer, metadata.producer_revision ?? metadata.producer_version],
    [observation.producer, observation.producer_revision ?? observation.producer_version],
    [metadata.processor, metadata.processor_revision ?? metadata.processor_version],
    [observation.processor, observation.processor_revision ?? observation.processor_version],
    [run.pipeline_id, run.pipeline_revision],
  ]
  const recorded = candidates.find(([producer]) => recordedText(producer))
  const producer = recorded ? recordedText(recorded[0]) : ''
  const producerVersion = recordedText(recorded?.[1]) || recordedText(candidates.find(([candidate, version]) => (
    recordedText(candidate) === producer && recordedText(version)
  ))?.[1])
  return {
    producer: producer || 'Producer not recorded',
    producerVersion: producerVersion || 'Version not recorded',
  }
}

export function documentPages(detail) {
  const metadata = object(detail?.item?.metadata)
  const fromItem = metadata.pages || metadata.document_pages
  const artifactPages = (detail?.derived_artifacts || []).flatMap(artifact => artifactMetadata(artifact).pages || [])
  const source = Array.isArray(fromItem) && fromItem.length ? fromItem : artifactPages
  if (source.length) return source.map((page, index) => ({
    number: finite(page?.page ?? page?.page_number) || index + 1,
    text: String(page?.text || page?.content || page?.source_text || ''),
  }))
  const text = metadata.source_text || metadata.extracted_text || metadata.text
    || (detail?.derived_artifacts || []).map(artifact => artifactMetadata(artifact).extracted_text || artifactMetadata(artifact).text).find(Boolean)
  return text ? String(text).split('\f').map((value, index) => ({ number: index + 1, text: value })) : []
}

function bboxFrom(value) {
  const box = value?.bbox || value?.box || value?.region || value
  if (Array.isArray(box) && box.length >= 4) return box.slice(0, 4).map(finite)
  if (box && typeof box === 'object') return [finite(box.x ?? box.left), finite(box.y ?? box.top), finite(box.width ?? ((box.right ?? 0) - (box.left ?? 0))), finite(box.height ?? ((box.bottom ?? 0) - (box.top ?? 0)))]
  return null
}

export function imageRegions(detail, citedBBox) {
  const values = []
  for (const artifact of detail?.derived_artifacts || []) {
    const metadata = artifactMetadata(artifact)
    const groups = [metadata.regions, metadata.ocr_regions, metadata.anpr_regions, metadata.observations, metadata.detections]
    const regions = groups.find(Array.isArray) || (artifact?.bbox ? [artifact] : [])
    for (const region of regions) {
      const bbox = bboxFrom(region)
      if (!bbox?.every(value => value !== null)) continue
      values.push({
        id: String(region.id || artifact.artifact_id || `region-${values.length + 1}`),
        bbox,
        label: String(region.label || region.text || region.plate || artifactLabel(artifact.artifact_type)),
        confidence: finite(region.confidence ?? artifact.confidence),
        cited: false,
      })
    }
  }
  if (citedBBox) {
    const bbox = bboxFrom(citedBBox)
    if (bbox?.every(value => value !== null)) values.unshift({ id: 'cited-region', bbox, label: 'Cited region', confidence: null, cited: true })
  }
  return values
}

function timedItems(metadata) {
  return [metadata.segments, metadata.cues, metadata.turns, metadata.utterances, metadata.events].find(Array.isArray) || []
}

export function transcriptCues(detail) {
  const result = []
  for (const artifact of detail?.derived_artifacts || []) {
    const metadata = artifactMetadata(artifact)
    for (const cue of timedItems(metadata)) {
      const start = finite(cue.t_start ?? cue.start ?? cue.start_seconds ?? cue.timestamp_seconds)
      const text = String(cue.text || cue.transcript || cue.content || '').trim()
      if (start === null || !text) continue
      result.push({ id: String(cue.id || `${artifact.artifact_id || 'cue'}-${result.length}`), start, end: finite(cue.t_end ?? cue.end ?? cue.end_seconds), text, speaker: String(cue.speaker || cue.speaker_label || '') })
    }
  }
  return result.sort((left, right) => left.start - right.start)
}

export function transcriptText(detail) {
  const candidates = [object(detail?.item?.metadata), ...(detail?.derived_artifacts || []).map(artifactMetadata)]
  return String(candidates.map(item => item.transcript || item.source_text || item.extracted_text || item.text || '').find(Boolean) || '')
}

export function videoEvents(detail, citedFrame) {
  const result = []
  for (const artifact of detail?.derived_artifacts || []) {
    const metadata = artifactMetadata(artifact)
    const values = timedItems(metadata).length ? timedItems(metadata) : [artifact]
    for (const event of values) {
      const time = finite(event.frame_ts ?? event.frame_timestamp ?? event.t_start ?? event.start_seconds ?? event.timestamp_seconds ?? metadata.frame_ts ?? metadata.t_start)
      if (time === null) continue
      result.push({ id: String(event.id || artifact.artifact_id || `event-${result.length + 1}`), time, label: String(event.label || event.text || event.plate || artifactLabel(artifact.artifact_type)), confidence: finite(event.confidence ?? artifact.confidence), cited: false })
    }
  }
  if (finite(citedFrame) !== null) result.unshift({ id: 'cited-frame', time: finite(citedFrame), label: 'Cited frame', confidence: null, cited: true })
  return result.sort((left, right) => left.time - right.time)
}

export function lineage(detail, versionFromUrl = '') {
  const item = detail?.item || {}
  const runs = Array.isArray(detail?.processing_runs) ? detail.processing_runs : []
  return {
    source: String(item.original_filename || item.source_file || item.evidence_id || 'Source file not reported'),
    version: String(versionFromUrl || item.version_id || item.evidence_version_id || 'Not reported'),
    artifacts: (detail?.derived_artifacts || []).map((artifact, index) => ({
      id: String(artifact.artifact_id || `${artifact.artifact_type || 'artifact'}-${index}`),
      label: artifactLabel(artifact.artifact_type),
      confidence: finite(artifact.confidence),
      ...artifactProducer(artifact, runs),
    })),
  }
}

export const viewerPresentationInternals = { bboxFrom, artifactLabel }
