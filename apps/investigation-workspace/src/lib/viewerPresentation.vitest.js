import { describe, expect, it } from 'vitest'
import { documentPages, imageRegions, lineage, transcriptCues, videoEvents } from './viewerPresentation.js'

const detail = {
  item: { original_filename: 'source.pdf', version_id: 'version-1', metadata: { pages: [{ page: 1, text: 'First' }, { page: 2, text: 'Second' }] } },
  processing_runs: [{ run_id: 'run-ocr', model_id: 'ocr-reader', model_revision: '2026.09' }],
  derived_artifacts: [
    { artifact_id: 'ocr-1', artifact_type: 'ocr', run_id: 'run-ocr', confidence: 0.31, metadata: { regions: [{ bbox: [0.1, 0.2, 0.3, 0.1], text: 'PK-123', confidence: 0.31 }] } },
    { artifact_id: 'transcript-1', artifact_type: 'transcript', confidence: 0.91, metadata: { segments: [{ start: 4.2, end: 7.5, speaker: 'Speaker 1', text: 'Hello' }] } },
    { artifact_id: 'event-1', artifact_type: 'anpr', confidence: 0.82, metadata: { events: [{ frame_ts: 12.5, plate: 'ABC-123' }] } },
  ],
}

describe('viewer presentation', () => {
  it('preserves paginated text and source lineage', () => {
    expect(documentPages(detail)).toEqual([{ number: 1, text: 'First' }, { number: 2, text: 'Second' }])
    expect(lineage(detail)).toMatchObject({ source: 'source.pdf', version: 'version-1' })
    expect(lineage(detail).artifacts[0]).toMatchObject({ producer: 'ocr-reader', producerVersion: '2026.09' })
    expect(lineage(detail).artifacts[1]).toMatchObject({ producer: 'Producer not recorded', producerVersion: 'Version not recorded' })
  })

  it('keeps low-confidence regions, timed speaker cues and video events distinct', () => {
    expect(imageRegions(detail)).toMatchObject([{ label: 'PK-123', confidence: 0.31 }])
    expect(transcriptCues(detail)).toEqual([{ id: 'transcript-1-0', start: 4.2, end: 7.5, speaker: 'Speaker 1', text: 'Hello' }])
    expect(videoEvents(detail)).toMatchObject([{ time: 12.5, label: 'ABC-123', confidence: 0.82 }])
  })
})
