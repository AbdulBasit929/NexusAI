import assert from 'node:assert/strict'
import test from 'node:test'
import { homeActivityPreviewText, homeEvidenceSummary, homeOrientation } from './analystHomePresentation.js'

test('uses evidence readiness instead of structured row accounting', () => {
  const summary = homeEvidenceSummary(
    { summary: { accepted_rows: 0, evidence_completed: 3, evidence_in_flight: 1, evidence_failed: 0 } },
    { summary: { evidence_total: 4 }, items: [] },
  )
  assert.deepEqual(summary, { total: 4, ready: 3, processing: 1, attention: 0, hasEvidence: true, isEmpty: false })
  assert.equal(homeOrientation(summary).label, '3 evidence sources ready')
})

test('falls back to catalog and typed item states without inventing readiness', () => {
  const summary = homeEvidenceSummary({}, {
    summary: { evidence_total: 3, completed: 1, processing: 1, failed: 1 },
    items: [
      { processing_status: 'completed' },
      { processing_status: 'running' },
      { processing_status: 'dead_letter' },
    ],
  })
  assert.deepEqual(summary, { total: 3, ready: 1, processing: 1, attention: 1, hasEvidence: true, isEmpty: false })
})

test('presents truthful empty and processing-only workspace states', () => {
  const empty = homeEvidenceSummary({}, { summary: { evidence_total: 0 }, items: [] })
  assert.equal(homeOrientation(empty).title, 'Build the evidence base')
  assert.equal(homeOrientation(empty).action, 'add')

  const processing = homeEvidenceSummary(
    { summary: { evidence_completed: 0, evidence_in_flight: 2, evidence_failed: 0 } },
    { summary: { evidence_total: 2 }, items: [] },
  )
  assert.equal(homeOrientation(processing).title, 'Evidence is being prepared')
  assert.equal(homeOrientation(processing).action, 'review')
})

test('routes attention-only workspaces to evidence review instead of Ask', () => {
  const attention = homeEvidenceSummary(
    { summary: { evidence_completed: 0, evidence_in_flight: 0, evidence_failed: 2 } },
    { summary: { evidence_total: 2 }, items: [] },
  )
  const orientation = homeOrientation(attention)

  assert.equal(orientation.label, '2 sources need attention')
  assert.equal(orientation.title, 'Review evidence that needs attention')
  assert.equal(orientation.tone, 'attention')
  assert.equal(orientation.action, 'review')
})

test('keeps retained identifiers available in Activity while simplifying Home previews', () => {
  const UUID = 'f47ac10b-58cc-4372-a567-0e02b2c3d479'
  assert.equal(homeActivityPreviewText(`Show plate AB1234 in ${UUID}`), 'Show plate AB1234 in this evidence source')
  assert.equal(
    homeActivityPreviewText('No records matched F47AC10B or A567 for plate AB1234.', `Show plate AB1234 in ${UUID}`),
    'No records matched this evidence source or this evidence source for plate AB1234.',
  )
})
