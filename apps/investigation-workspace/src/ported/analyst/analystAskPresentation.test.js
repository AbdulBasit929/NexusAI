import test from 'node:test'
import assert from 'node:assert/strict'

import {
  analystAskScope,
  analystCitation,
  analystResultState,
  analystSourceNavigationContext,
  analystSourceNavigationParams,
  analystScopedConversationContext,
  analystShouldInheritConversationContext,
  analystSourceTimeRange,
} from './analystAskPresentation.js'

test('inherits forensic context only for anaphoric follow-ups in the same portal scope', () => {
  const messages = [{
    id: 'answer-1', sender: 'agent', metadata: {
      request_scope: 'case-a::collection-a::evidence-a',
      analysis_id: 'analysis-1',
      presentation: { context: { target: '923001110001', template: 'frequent_contacts' } },
    },
  }]
  assert.equal(analystShouldInheritConversationContext('Only outgoing.'), true)
	assert.equal(analystShouldInheritConversationContext('When did they say that?'), true)
	assert.equal(analystShouldInheritConversationContext('انہوں نے یہ کب کہا؟'), true)
  assert.equal(analystShouldInheritConversationContext('Find exact "923009998887".'), false)
  assert.equal(
    analystScopedConversationContext(messages, 'conversation-1', 'case-a::collection-a::evidence-a', 'Only outgoing.').target,
    '923001110001',
  )
  assert.equal(analystScopedConversationContext(messages, 'conversation-1', 'case-a::collection-a::evidence-b', 'Only outgoing.'), null)
  assert.equal(analystScopedConversationContext(messages, 'conversation-1', 'case-a::collection-a::evidence-a', 'Where was plate MN1367 seen?'), null)
})

test('keeps every governed result state distinct in analyst language', () => {
  const expected = {
    complete_zero_results: 'Processing completed. No detections were found.',
    no_match_for_filter: 'No results matched this filter.',
    processing: 'Processing is still in progress.',
    not_processed: 'This analysis has not been run yet.',
    failed: 'Processing failed.',
    unavailable: 'This capability is not currently available.',
    unauthorized: 'You do not have access to this result.',
  }
  for (const [state, description] of Object.entries(expected)) {
    assert.equal(analystResultState(state).description, description)
  }
  assert.notEqual(analystResultState('complete_zero_results').title, analystResultState('no_match_for_filter').title)
})

test('turns source citations into human labels without making UUIDs or URIs primary', () => {
  assert.deepEqual(analystCitation({ label: 'calls.csv', locator: { row_number: 42 } }), { label: 'calls.csv', detail: 'row 42' })
  assert.deepEqual(analystCitation({ source_file: 'retained/video.mp4', citation_locator: { timestamp_seconds: 20 } }), { label: 'video.mp4', detail: '20 seconds in source' })
  assert.deepEqual(analystCitation({ source_file: 'brief.pdf', locator: { page: 1, passage: 1 } }), { label: 'brief.pdf', detail: 'page 1, passage 1' })
  assert.deepEqual(analystCitation({ label: '50057921-4f1f-4ab8-bab0-47bcdc957822', detail: 'nexusai://evidence/value' }, 1), { label: 'Evidence source 2', detail: 'Source-level reference' })
})

test('reports source-time bounds as media seconds rather than calendar filters', () => {
  assert.deepEqual(analystSourceTimeRange({ execution_plan: { parameters: { start_seconds: 10, end_seconds: 30 } } }), { start: 10, end: 30, label: '10 seconds to 30 seconds' })
  assert.deepEqual(analystSourceTimeRange({ trace: { parameters: { source_start_seconds: 65.5 } } }), { start: 65.5, end: null, label: 'from 01:05' })
})

test('makes evidence context explicit while retaining a human workspace label', () => {
  assert.deepEqual(
    analystAskScope(
      { displayName: 'Unified review' },
      { evidence_id: 'e-1', current_version_id: 'v-2', source_file: 'recording.wav', modality: 'audio' },
      { derived_artifacts: [
        { artifact_type: 'forensics.audio-timestamp-segment/v1', processing_status: 'completed', version_id: 'v-2' },
        { artifact_type: 'forensics.audio-roman-urdu-segment/v1', processing_status: 'completed', version_id: 'v-2' },
        { artifact_type: 'forensics.audio-identifiers/v1', processing_status: 'failed', version_id: 'v-2' },
      ] },
    ),
    {
      kind: 'evidence', label: 'recording.wav', detail: 'Unified review', evidenceId: 'e-1', evidenceVersionId: 'v-2', sourceFamily: 'audio',
      availableResultFamilies: ['forensics.audio-timestamp-segment/v1', 'forensics.audio-roman-urdu-segment/v1'],
    },
  )
  assert.deepEqual(analystAskScope({ displayName: 'Unified review' }), { kind: 'workspace', label: 'Unified review', detail: 'Entire workspace' })
})

test('preserves only supported citation locators in source navigation', () => {
  const matrix = [
    [{ evidence_id: 'structured-1', locator: { row_number: 42 } }, 'evidence=structured-1&row=42'],
    [{ evidence_id: 'document-1', locator: { page: 3 } }, 'evidence=document-1&page=3'],
    [{ evidence_id: 'image-1', artifact_id: 'ocr-7' }, 'evidence=image-1&finding=ocr-7'],
    [{ evidence_id: 'anpr-1', citation_locator: { artifact_id: 'plate-4' } }, 'evidence=anpr-1&finding=plate-4'],
    [{ evidence_id: 'audio-1', locator: { start_seconds: 8 } }, 'evidence=audio-1&source_time=8'],
    [{ evidence_id: 'video-1', locator: { timestamp_seconds: 20 } }, 'evidence=video-1&source_time=20'],
  ]
  for (const [citation, expected] of matrix) {
    assert.equal(analystSourceNavigationParams(citation, citation.evidence_id).toString(), expected)
  }
  assert.equal(analystSourceNavigationParams({ detail: 'page 8' }, 'document-2').toString(), 'evidence=document-2')
})

test('turns supported source navigation parameters into truthful context', () => {
  assert.deepEqual(analystSourceNavigationContext('?source_time=8&page=3&row=42&finding=plate-4'), [
    { kind: 'time', label: 'Source time', value: '8 seconds' },
    { kind: 'page', label: 'Document location', value: 'Page 3' },
    { kind: 'row', label: 'Structured reference', value: 'Row 42' },
    { kind: 'finding', label: 'Finding reference', value: 'plate-4' },
  ])
})

test('unknown result states cannot imply successful findings', () => {
  assert.equal(analystResultState('unexpected_state').label, 'Result needs review')
})
