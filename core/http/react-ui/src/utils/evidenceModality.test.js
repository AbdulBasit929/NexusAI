import assert from 'node:assert/strict'
import test from 'node:test'

import { resolveEvidenceModality } from './evidenceModality.js'

test('uses authoritative modality before misleading filenames and MIME', () => {
  assert.equal(resolveEvidenceModality({ modality: 'audio', mime_type: 'application/octet-stream', original_filename: 'table.csv' }).kind, 'audio')
  assert.equal(resolveEvidenceModality({ modality: 'video', mime_type: 'image/png', original_filename: 'still.png' }).kind, 'video')
})

test('covers the shared evidence icon taxonomy', () => {
  const fixtures = [
    [{ detected_type: 'cdr' }, 'structured', 'fa-table-list'],
    [{ detected_type: 'pdf' }, 'document', 'fa-file-lines'],
    [{ detected_type: 'text' }, 'text', 'fa-file-lines'],
    [{ mime_type: 'image/jpeg' }, 'image', 'fa-image'],
    [{ mime_type: 'audio/wav' }, 'audio', 'fa-waveform-lines'],
    [{ source_file: 'camera.mkv' }, 'video', 'fa-film'],
    [{ source_file: 'evidence.zip' }, 'archive', 'fa-file-zipper'],
    [{ source_file: 'opaque.bin' }, 'unknown', 'fa-file-circle-question'],
  ]
  for (const [item, kind, icon] of fixtures) {
    assert.equal(resolveEvidenceModality(item).kind, kind)
    assert.equal(resolveEvidenceModality(item).icon, icon)
  }
})

test('keeps ANPR image evidence in the image base modality', () => {
  assert.equal(resolveEvidenceModality({ detected_type: 'anpr_vehicle_sightings' }).kind, 'image')
})
