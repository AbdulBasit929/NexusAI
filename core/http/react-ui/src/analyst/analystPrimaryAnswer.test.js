import test from 'node:test'
import assert from 'node:assert/strict'
import {
  analystAuthoritativeEntityChoices,
  analystNaturalAnswer,
  analystPrimaryFieldIsInternal,
  analystSingleResultFacts,
  analystUsefulFindings,
  analystUsefulFollowUps,
} from './analystPrimaryAnswer.js'

test('offers only governed phone or subscriber candidates from retained entity activity', () => {
  const history = [{ metadata: { presentation: { operation_id: 'forensics.entity_activity', table: { rows: [
    { entity_type: 'phone', entity_value: '923001110001' },
    { entity_type: 'location', entity_value: 'Gulberg' },
    { entity_type: 'phone', entity_value: '923001110001' },
    { entity_type: 'subscriber', entity_value: 'SUB-9' },
  ] } } } }]
  assert.deepEqual(analystAuthoritativeEntityChoices(history), [
    { label: '923001110001', query: 'show frequent contacts for 923001110001', type: 'phone' },
    { label: 'SUB-9', query: 'show frequent contacts for SUB-9', type: 'subscriber' },
  ])
})

test('turns a technical one-result ANPR response into a compact human answer', () => {
  const view = {
    operation_id: 'anpr.sightings',
    executive_answer: 'Retrieved exact ANPR observations with canonical evidence, version, row, and hash provenance.',
    metrics: [{ label: 'Target', value: 'LEH5003' }],
    table: { rows: [{ observed_at: '2026-09-07T04:44:55Z', camera_id: '', location: '', row_hash: 'secret' }] },
  }
  assert.equal(analystNaturalAnswer(view), '1 sighting of LEH5003 was found.')
  assert.deepEqual(analystSingleResultFacts(view).map(item => [item.label, item.value]), [
    ['Observed', '7 Sept 2026 · 04:44:55 UTC'],
    ['Camera', 'Not available'],
    ['Location', 'Not available'],
  ])
})

test('keeps natural Urdu while removing normalized implementation wording', () => {
  const phrase = 'مجھے معلوم نہیں آیا آپ نے محسوس کیا یا نہیں'
  assert.equal(analystNaturalAnswer({ executive_answer: `An exact normalized mention of "${phrase}" was found in this recording.` }), `The phrase "${phrase}" was found in this recording.`)
  assert.equal(analystPrimaryFieldIsInternal('normalized_target'), true)
})

test('deduplicates findings and limits natural follow-ups', () => {
  const view = {
    executive_answer: 'One result was found.',
    findings: [
      { text: 'One result was found.', claim_type: 'deterministic_fact' },
      { text: 'Planner confidence: 0.9', claim_type: 'metric' },
      { text: 'The sighting occurred before sunrise.', claim_type: 'deterministic_fact' },
    ],
  }
  assert.deepEqual(analystUsefulFindings(view), ['The sighting occurred before sunrise.'])
  assert.deepEqual(analystUsefulFollowUps([
    { text: 'Show source rows', query: 'show source rows' },
    { text: 'Show other sightings of this plate', query: 'show other sightings' },
    { text: 'What happened around this time?', query: 'what happened around this time?' },
    { text: 'A third suggestion', query: 'third' },
  ]).map(item => item.label), ['Show other sightings of this plate', 'What happened around this time?'])
})

test('keeps complete-zero distinct from no-match', () => {
  assert.equal(analystNaturalAnswer({ result_state: 'complete_zero_results' }), 'Processing completed. No detections were found.')
  assert.equal(analystNaturalAnswer({ result_state: 'no_match_for_filter' }), 'No results matched this filter.')
})
