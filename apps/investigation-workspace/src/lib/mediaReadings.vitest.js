import { describe, expect, it } from 'vitest'
import { displayedRect, pictureText, plateFlagLabel, plateReadings, readsAt } from './mediaReadings.js'

const bbox = { x: 504, y: 492, width: 161, height: 58 }
function artifact(id, type, locator, observation, confidence = null) {
  return { artifact_id: id, artifact_type: type, citation_locator: locator, confidence, metadata: { observation } }
}

describe('plateReadings', () => {
  it('reads a plate of a photo with its box, confidence and flags', () => {
    const detail = { derived_artifacts: [artifact('p1', 'forensics.anpr-observation/v1', { bbox }, { normalized_plate_text: 'SYNI2905', raw_plate_text: 'SYNI 2905', ocr_confidence: 0.98, plate_format_flags: ['format_unknown'] }, 0.9795)] }
    const { photo, groups, reads } = plateReadings(detail)
    expect(groups).toEqual([])
    expect(reads).toEqual([])
    expect(photo).toEqual([{ id: 'p1', plate: 'SYNI2905', raw: 'SYNI 2905', confidence: 0.9795, box: [504, 492, 161, 58], flags: ['Not a known plate format'], format: null }])
  })

  it('reads the frame readings of a video and the plate groups, and ties a group to its own readings', () => {
    const detail = { derived_artifacts: [
      artifact('r2', 'forensics.anpr-observation/v1', { bbox, frame_number: 120, timestamp_seconds: 4.0 }, { normalized_plate_text: 'AB1234', ocr_confidence: 0.7 }, 0.7),
      artifact('r1', 'forensics.anpr-observation/v1', { bbox, frame_number: 60, timestamp_seconds: 2.0 }, { normalized_plate_text: 'AB1234', ocr_confidence: 0.9 }, 0.9),
      artifact('other', 'forensics.anpr-observation/v1', { bbox, frame_number: 61, timestamp_seconds: 2.1 }, { normalized_plate_text: 'ZZ9999' }, 0.5),
      artifact('g1', 'forensics.video-anpr-plate-group/v1', { bbox, first_seen_seconds: 2.0, last_seen_seconds: 4.0 }, { normalized_plate_text: 'AB1234', sightings_count: 2, all_observation_ids: ['r1', 'r2'] }),
    ] }
    const { photo, groups, reads } = plateReadings(detail)
    expect(photo).toEqual([])
    expect(reads.map(read => read.id)).toEqual(['r1', 'other', 'r2'])
    expect(groups).toHaveLength(1)
    expect(groups[0]).toMatchObject({ plate: 'AB1234', first: 2, last: 4, count: 2, confidence: 0.9 })
    expect(groups[0].reads.map(read => read.id)).toEqual(['r1', 'r2'])
  })

  it('ties a group to the readings of the same plate inside its span when the ids do not match (a second copy of a file stores readings under ids of its own)', () => {
    const detail = { derived_artifacts: [
      artifact('scoped-1', 'forensics.anpr-observation/v1', { bbox, frame_number: 60, timestamp_seconds: 2.0 }, { normalized_plate_text: 'AB1234' }, 0.8),
      artifact('scoped-2', 'forensics.anpr-observation/v1', { bbox, frame_number: 90, timestamp_seconds: 3.0 }, { normalized_plate_text: 'AB1234' }, 0.95),
      artifact('late', 'forensics.anpr-observation/v1', { bbox, frame_number: 300, timestamp_seconds: 10.0 }, { normalized_plate_text: 'AB1234' }, 0.99),
      artifact('g', 'forensics.video-anpr-plate-group/v1', { bbox, first_seen_seconds: 2.0, last_seen_seconds: 3.0 }, { normalized_plate_text: 'AB1234', sightings_count: 24, all_observation_ids: ['not-a-stored-id'] }),
    ] }
    const [group] = plateReadings(detail).groups
    expect(group.reads.map(read => read.id)).toEqual(['scoped-1', 'scoped-2'])
    expect(group).toMatchObject({ count: 24, confidence: 0.95 })
  })

  it('skips a reading without a plate, a box that is not a box and a group without a first second', () => {
    const detail = { derived_artifacts: [
      artifact('a', 'forensics.anpr-observation/v1', { bbox }, { normalized_plate_text: '' }),
      artifact('b', 'forensics.anpr-observation/v1', { bbox: { x: 1, y: 2 } }, { normalized_plate_text: 'XY12' }),
      artifact('c', 'forensics.video-anpr-plate-group/v1', {}, { normalized_plate_text: 'XY12' }),
    ] }
    const { photo, groups } = plateReadings(detail)
    expect(photo.map(p => [p.plate, p.box])).toEqual([['XY12', null]])
    expect(groups).toEqual([])
  })

  it('humanizes a flag it does not know', () => {
    expect(plateFlagLabel('format_unknown')).toBe('Not a known plate format')
    expect(plateFlagLabel('odd_spacing')).toBe('Odd spacing')
    expect(plateFlagLabel('')).toBe('')
  })
})

describe('pictureText', () => {
  it('lists text read in a photo with its box and skips text read in a video frame', () => {
    const detail = { derived_artifacts: [
      artifact('t1', 'forensics.image-ocr-observation/v1', { bbox: { x: 10, y: 20, width: 40, height: 24 } }, { raw_text: '1427' }, 0.9),
      artifact('t2', 'forensics.image-ocr-observation/v1', { bbox: { x: 1, y: 1, width: 5, height: 5 }, timestamp_seconds: 3 }, { raw_text: 'frame text' }, 0.9),
      artifact('t3', 'forensics.image-ocr-observation/v1', { bbox: { x: 1, y: 1, width: 5, height: 5 } }, { raw_text: '  ' }, 0.9),
    ] }
    expect(pictureText(detail)).toEqual([{ id: 't1', text: '1427', confidence: 0.9, box: [10, 20, 40, 24] }])
  })
})

describe('on the screen', () => {
  const reads = [
    { id: 'a', plate: 'AB1234', time: 2.0, box: [1, 2, 3, 4] },
    { id: 'b', plate: 'AB1234', time: 2.4, box: [1, 2, 3, 4] },
    { id: 'c', plate: 'ZZ9999', time: 5.0, box: [1, 2, 3, 4] },
    { id: 'd', plate: 'QQ0000', time: 2.2, box: null },
  ]
  it('takes, for each plate, the reading nearest in time within the window and ignores readings without a box', () => {
    expect(readsAt(reads, 2.3).map(read => read.id)).toEqual(['b'])
    expect(readsAt(reads, 9)).toEqual([])
  })

  it('finds where the picture is drawn when the element is wider than the picture', () => {
    expect(displayedRect(1000, 400, 1920, 1080)).toEqual({ left: (1000 - 1920 * (400 / 1080)) / 2, top: 0, width: 1920 * (400 / 1080), height: 400 })
    expect(displayedRect(0, 0, 10, 10)).toBeNull()
  })
})
