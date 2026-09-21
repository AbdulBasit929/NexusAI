import test from 'node:test'
import assert from 'node:assert/strict'

import {
  activityDateGroup,
  activityCell,
  activityFilterOptions,
  activityFindingGroups,
  activityItemPresentation,
  activityResultState,
  activitySourcesFor,
  groupActivityItems,
} from './analystActivityPresentation.js'

test('four P1 failure and missing-answer states never look completed', () => {
  for (const status of ['error', 'failed', 'timed_out', 'cancelled', 'error: private SQL diagnostic']) {
    assert.equal(activityResultState({ ...historyItem('results_present'), status }).id, 'failed')
  }
  assert.equal(activityResultState({ status: 'completed', answer: '' }).id, 'failed')
  assert.equal(activityResultState({ status: 'needs_input', answer: 'Which source?' }).id, 'invalid_request')
  assert.equal(activityResultState(historyItem('no_match_for_filter')).id, 'no_match_for_filter')
})

test('four P1 Activity cells and paginated-source citations are analyst readable', () => {
  assert.equal(activityCell({ unknown: 'private' }), '—')
  assert.equal(activityCell({ timestamp_seconds: 2 }), '2s in source')
  assert.equal(activityCell(0.875), '0.875')
  assert.equal(activityCell('nexusai://evidence/opaque'), 'See source citation')
  const sources = activitySourcesFor(historyItem('results_present', { citations: [
    { source_file: 'candidate.jpg', evidence_id: 'candidate-id', version_id: 'version-id', proof_role: 'candidate_observation' },
    { source_file: 'query.jpg', evidence_id: 'query-id', version_id: 'query-version', proof_role: 'query_observation' },
  ] }), [])
  assert.equal(sources.length, 2)
  assert.equal(sources[0].label, 'Candidate.jpg')
  assert.equal(sources[0].evidenceId, 'candidate-id')
  assert.ok(!JSON.stringify(sources).includes('[object Object]'))
})

function historyItem(resultState, extra = {}) {
  return {
    analysis_id: `analysis-${resultState}`,
    query: extra.query || 'Show the retained findings',
    status: 'completed',
    created_at: extra.created_at || '2026-08-27T08:30:00Z',
    metadata: {
      presentation: {
        contract_version: 'forensics.agent-presentation/v1',
        result_state: resultState,
        row_count: extra.row_count,
        executive_answer: extra.answer || '',
        citations: extra.citations || [],
      },
    },
  }
}

test('keeps Activity result states and zero-result meanings distinct', () => {
  assert.equal(activityResultState(historyItem('results_present', { row_count: 1 })).label, '1 result')
  assert.equal(activityResultState(historyItem('results_present', { row_count: 12 })).label, '12 results')
  assert.equal(activityResultState(historyItem('results_present', { row_count: 0 })).label, 'Results available')
  assert.equal(activityResultState(historyItem('complete_zero_results', { row_count: 0 })).label, 'Processing completed · No detections')
  assert.equal(activityResultState(historyItem('no_match_for_filter', { row_count: 0 })).label, 'No results matched this filter')
  assert.notEqual(activityResultState(historyItem('complete_zero_results')).label, activityResultState(historyItem('no_match_for_filter')).label)
})

test('builds only useful Activity filters from the bounded loaded page', () => {
  const filters = activityFilterOptions([
    historyItem('results_present'),
    historyItem('complete_zero_results'),
    historyItem('processing'),
  ])
  assert.deepEqual(filters.map(filter => [filter.id, filter.count]), [['all', 3], ['results', 1], ['no_results', 1], ['processing', 1]])
})

test('uses human calendar grouping without altering authoritative timestamps', () => {
  const now = new Date('2026-08-27T12:00:00')
  assert.equal(activityDateGroup('2026-08-27T08:30:00', now), 'Today')
  assert.equal(activityDateGroup('2026-08-26T22:00:00', now), 'Yesterday')
  assert.equal(activityDateGroup('2026-08-23T08:30:00', now), 'August 23, 2026')
  assert.deepEqual(groupActivityItems([
    historyItem('results_present', { created_at: '2026-08-27T08:30:00' }),
    historyItem('no_match_for_filter', { created_at: '2026-08-27T07:30:00' }),
  ], now).map(group => [group.label, group.items.length]), [['Today', 2]])
})

test('uses catalog filenames and supported locators for Activity sources', () => {
  const citation = { evidence_id: 'video-random-7', source_file: 'retained/camera-clip.mp4', locator: { timestamp_seconds: 20 } }
  const source = activitySourcesFor(historyItem('results_present', { citations: [citation] }), [
    { evidence_id: 'video-random-7', original_filename: 'camera-clip.mp4', modality: 'video' },
  ])[0]
  assert.equal(source.label, 'Camera clip.mp4')
  assert.equal(source.type, 'Video evidence')
  assert.equal(source.detail, '20 seconds in source')
  assert.equal(source.navigation.toString(), 'evidence=video-random-7&source_time=20')
})

test('search presentation includes question, answer, state and human source context', () => {
  const view = activityItemPresentation(historyItem('results_present', {
    query: 'پلیٹ کہاں دیکھی گئی؟',
    answer: 'One observation was found.',
    citations: [{ evidence_id: 'image-random-2', source_file: 'gate-camera.jpg' }],
  }), [{ evidence_id: 'image-random-2', original_filename: 'gate-camera.jpg', modality: 'image' }])
  assert.match(view.searchText, /پلیٹ کہاں دیکھی گئی/)
  assert.match(view.searchText, /gate camera\.jpg/)
  assert.match(view.searchText, /results available/)
})

test('keeps investigation findings primary and moves execution notes to disclosure', () => {
  const groups = activityFindingGroups({ findings: [
    { claim_type: 'deterministic_fact', text: 'One retained observation was found.' },
    { claim_type: 'metric', text: 'Template: video_timeline' },
    { claim_type: 'metric', text: 'Planner Confidence: 0.9' },
    { claim_type: 'metric', text: 'Display Rows: 0' },
    { kind: 'execution_metadata', text: 'worker detail' },
  ] })
  assert.deepEqual(groups.visible.map(item => item.text), ['One retained observation was found.'])
  assert.deepEqual(groups.technical.map(item => item.text), ['Template: video_timeline', 'Planner Confidence: 0.9', 'Display Rows: 0', 'worker detail'])
})
