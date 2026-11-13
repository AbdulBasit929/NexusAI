// The plates and the text the service read from a photo or a video, taken from the evidence detail.
//
// What is stored: a plate reading of a photo is a derived artifact of type forensics.anpr-observation/v1 whose box is in citation_locator.bbox
// (original pixels) and whose plate, reading confidence and format flags are in metadata.observation; a plate reading of one video frame is the
// same type with a frame number and a time in its locator; a plate seen across several frames of a video is a forensics.video-anpr-plate-group/v1
// artifact (first and last second, number of sightings, the ids of its frame readings); text in a photo is forensics.image-ocr-observation/v1.
// The viewers used to look for a list of regions inside one artifact and so showed none of them.

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

function observationOf(artifact) { return object(object(artifact?.metadata).observation) }

function boxOf(locator) {
  const box = locator?.bbox
  if (!box || typeof box !== 'object') return null
  const values = Array.isArray(box) ? box.slice(0, 4).map(finite) : [finite(box.x), finite(box.y), finite(box.width), finite(box.height)]
  return values.length === 4 && values.every(value => value !== null) && values[2] > 0 && values[3] > 0 ? values : null
}

const KNOWN_FLAGS = {
  format_unknown: 'Not a known plate format',
  low_confidence: 'Low confidence',
  short_read: 'Fewer characters than a plate has',
  ambiguous_characters: 'Look-alike characters',
}

export function plateFlagLabel(flag) {
  const key = text(flag)
  if (KNOWN_FLAGS[key]) return KNOWN_FLAGS[key]
  const words = key.replace(/_/g, ' ').trim()
  return words ? words.charAt(0).toUpperCase() + words.slice(1) : ''
}

function flagsOf(observation) {
  const raw = Array.isArray(observation.plate_format_flags) ? observation.plate_format_flags : []
  return raw.map(plateFlagLabel).filter(Boolean)
}

// { photo: [plate readings of a photo], groups: [plates seen in a video, by first second], reads: [the frame readings of a video, by time] }
export function plateReadings(detail) {
  const photo = []
  const reads = []
  const groups = []
  for (const artifact of detail?.derived_artifacts || []) {
    const type = text(artifact?.artifact_type)
    const locator = object(artifact?.citation_locator)
    const observation = observationOf(artifact)
    const id = text(artifact?.artifact_id) || `reading-${photo.length + reads.length + groups.length}`
    if (/video-anpr-plate-group/.test(type)) {
      const plate = text(observation.normalized_plate_text || observation.plate)
      const first = finite(locator.first_seen_seconds ?? observation.first_seen_seconds)
      if (!plate || first === null) continue
      groups.push({
        id, plate, first, last: finite(locator.last_seen_seconds ?? observation.last_seen_seconds) ?? first,
        count: finite(observation.sightings_count), box: boxOf(locator), confidence: finite(artifact.confidence ?? observation.best_confidence ?? observation.confidence),
        memberIds: Array.isArray(observation.all_observation_ids) ? observation.all_observation_ids.map(String) : [],
      })
    } else if (/anpr-observation/.test(type)) {
      const plate = text(observation.normalized_plate_text)
      if (!plate) continue
      const time = finite(locator.timestamp_seconds ?? locator.timestamp ?? observation.timestamp_seconds)
      const frame = finite(locator.frame_number)
      const reading = { id, plate, raw: text(observation.raw_plate_text), confidence: finite(artifact.confidence ?? observation.ocr_confidence), box: boxOf(locator), flags: flagsOf(observation), format: text(observation.plate_format) || null }
      if (time !== null || frame !== null) reads.push({ ...reading, time, frame })
      else photo.push(reading)
    }
  }
  reads.sort((a, b) => (a.time ?? Infinity) - (b.time ?? Infinity))
  for (const group of groups) {
    // the group names its readings by observation id; the stored artifact of a reading of a SECOND COPY of a file has a scoped id of its own, so when no id matches the readings of the same plate inside the group's span are its readings
    let members = reads.filter(read => group.memberIds.includes(read.id))
    if (!members.length) members = reads.filter(read => read.plate === group.plate && (read.time ?? Infinity) >= group.first - 0.1 && (read.time ?? -Infinity) <= group.last + 0.1)
    group.reads = members
    if (group.count === null) group.count = members.length || null
    if (group.confidence === null && members.length) group.confidence = Math.max(...members.map(read => read.confidence ?? 0))
  }
  groups.sort((a, b) => a.first - b.first)
  photo.sort((a, b) => (b.confidence ?? 0) - (a.confidence ?? 0))
  return { photo, groups, reads }
}

// Text read inside a photo (not inside a video frame): [{ id, text, confidence, box }].
export function pictureText(detail) {
  const result = []
  for (const artifact of detail?.derived_artifacts || []) {
    if (!/image-ocr-observation/.test(text(artifact?.artifact_type))) continue
    const locator = object(artifact?.citation_locator)
    if (finite(locator.timestamp_seconds) !== null || finite(locator.frame_number) !== null) continue
    const observation = observationOf(artifact)
    const content = text(observation.raw_text || observation.text)
    const box = boxOf(locator)
    if (content && box) result.push({ id: text(artifact.artifact_id) || `text-${result.length}`, text: content, confidence: finite(artifact.confidence ?? observation.confidence), box })
  }
  return result
}

// Text read inside a frame of a video: [{ id, text, time, confidence, box }], by time.
export function videoText(detail) {
  const result = []
  for (const artifact of detail?.derived_artifacts || []) {
    if (!/image-ocr-observation/.test(text(artifact?.artifact_type))) continue
    const locator = object(artifact?.citation_locator)
    const time = finite(locator.timestamp_seconds ?? locator.timestamp)
    if (time === null) continue
    const observation = observationOf(artifact)
    const content = text(observation.raw_text || observation.text)
    const box = boxOf(locator)
    if (content && box) result.push({ id: text(artifact.artifact_id) || `frame-text-${result.length}`, text: content, time, confidence: finite(artifact.confidence ?? observation.confidence), box })
  }
  return result.sort((a, b) => a.time - b.time)
}

// The characters of a reading with the ones that differ from the plate the group settled on marked, so that a look-alike (O and 0, I and T) is seen at once.
// Readings of another length are shown whole: there is no honest position to compare.
export function plateDiff(reading, plate) {
  const characters = Array.from(text(reading))
  const reference = Array.from(text(plate))
  const same = characters.length === reference.length
  return characters.map((character, index) => ({ character, differs: same && character !== reference[index] }))
}

// The reads of a video that are on the screen at `time`: for each plate the reading nearest in time, if it is within `window` seconds.
export function readsAt(reads, time, window = 0.6, keyOf = read => read.plate) {
  const best = new Map()
  for (const read of reads) {
    if (read.time === null || !read.box) continue
    const distance = Math.abs(read.time - time)
    if (distance > window) continue
    const key = keyOf(read)
    const kept = best.get(key)
    if (!kept || distance < kept.distance) best.set(key, { read, distance })
  }
  return [...best.values()].map(item => item.read)
}

// Where the picture of a video is drawn inside its element when the element is wider or taller than the picture (object-fit: contain), in the element's pixels.
export function displayedRect(elementWidth, elementHeight, videoWidth, videoHeight) {
  if (!(elementWidth > 0 && elementHeight > 0 && videoWidth > 0 && videoHeight > 0)) return null
  const scale = Math.min(elementWidth / videoWidth, elementHeight / videoHeight)
  const width = videoWidth * scale
  const height = videoHeight * scale
  return { left: (elementWidth - width) / 2, top: (elementHeight - height) / 2, width, height }
}
