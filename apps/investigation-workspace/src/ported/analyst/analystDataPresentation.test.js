import assert from 'node:assert/strict'
import test from 'node:test'
import { detailDisclosureModel, evidenceFilterSummary, evidenceFindingPreview, evidenceTypePresentation, visibleStatusFilters } from './analystDataPresentation.js'

const ready = { id: 'ready', description: 'Ready' }

test('filter counts disclose bounded loaded-page scope until pagination is exhausted', () => {
  assert.equal(evidenceFilterSummary(0, true), '0 matching sources in loaded sources')
  assert.equal(evidenceFilterSummary(3, true), '3 matching sources in loaded sources')
  assert.equal(evidenceFilterSummary(0, false), '0 matching sources')
  assert.equal(evidenceFilterSummary(3, false), '3 matching sources')
})

test('recognizes structured, document, image, audio, video and ANPR evidence', () => {
  assert.equal(evidenceTypePresentation({ detected_type: 'cdr' }).category, 'structured')
  assert.equal(evidenceTypePresentation({ detected_type: 'pdf' }).category, 'documents')
  assert.equal(evidenceTypePresentation({ modality: 'image' }).category, 'images')
  assert.equal(evidenceTypePresentation({ mime_type: 'audio/wav' }).category, 'audio')
  assert.equal(evidenceTypePresentation({ mime_type: 'video/mp4' }).category, 'video')
  assert.equal(evidenceTypePresentation({ detected_type: 'anpr_vehicle_sightings' }).category, 'anpr')
})

test('uses modality-aware finding previews without inventing missing counts', () => {
  assert.equal(evidenceFindingPreview({ modality: 'image', accepted_rows: 2 }, ready), 'Image findings ready to review')
  assert.equal(evidenceFindingPreview({ modality: 'audio', accepted_rows: 1 }, ready), 'Audio source ready — open to review transcript availability')
  assert.equal(evidenceFindingPreview({ modality: 'video', accepted_rows: 0 }, ready), 'No timeline findings were recorded')
  assert.equal(evidenceFindingPreview({ detected_type: 'pdf', accepted_rows: 3 }, ready), '3 extracted passages available')
  assert.equal(evidenceFindingPreview({ detected_type: 'cdr', accepted_rows: 4 }, ready), '4 verified records available for analysis')
  assert.equal(evidenceFindingPreview({ detected_type: 'cdr' }, ready, { availability: 'queryable' }), 'Verified record count is not available')
})

test('technical audio observations and failed transcripts never prove available speech', () => {
  const item = { modality: 'audio', accepted_rows: 3 }
  const technical = { artifact_type: 'forensics.audio-observation/v1', processing_status: 'completed' }
  const transcript = { artifact_type: 'forensics.audio-timestamp-segment/v1', processing_status: 'completed' }
  assert.equal(evidenceFindingPreview(item, ready, null, [technical]), 'No transcript was recorded')
  assert.equal(evidenceFindingPreview(item, ready, null, [technical, { ...transcript, processing_status: 'failed' }]), 'No transcript was recorded')
  assert.equal(evidenceFindingPreview(item, ready, null, [technical, transcript]), 'Machine transcript ready to review')
})

test('suppresses irrelevant zero-count status filters', () => {
  assert.deepEqual(visibleStatusFilters({ all: 24, ready: 24, processing: 0, attention: 0, failed: 0 }).map(([id]) => id), ['all', 'ready'])
  assert.deepEqual(visibleStatusFilters({ all: 4, ready: 1, processing: 1, attention: 1, failed: 1 }).map(([id]) => id), ['all', 'ready', 'processing', 'attention', 'failed'])
})

test('suppresses empty detail sections while preserving reported zero accounting', () => {
  assert.deepEqual(detailDisclosureModel({ maturity: { fields: [], mapping: {}, quality: {}, capabilities: [], time: {} }, accounting: { acceptedRows: null, inputRows: null, duplicateRows: null, rejectedRows: null } }), { fields: false, additionalFields: false, quality: false, capabilities: false, notes: false })
  assert.equal(detailDisclosureModel({ maturity: { fields: [], mapping: {}, quality: {}, capabilities: [], time: {} }, accounting: { acceptedRows: 0, inputRows: 0, duplicateRows: 0, rejectedRows: 0 } }).quality, true)
})
