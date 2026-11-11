import { describe, expect, it } from 'vitest'
import { presentInvestigationResponse } from './investigationPresentation'

// The plate search over image and video readings (api/forensic_records/plate_image_search.go, policy plate_search_only).
// Its sentences are written on the server and say "model readings"; the vocabulary filter of the older paths dropped them
// and the analyst saw "No verified answer is available" next to a file the API had found.
const answer = 'The plate SYNT9150 was read in 1 image or video file: roadside-vehicle-02.jpg. These are unreviewed model readings, not camera sightings.'
const response = {
  collection_id: 'orion-lahore-reference-20261030',
  intent: 'records',
  policy: 'plate_search_only',
  route: ['plate_image_search'],
  evidence: { result_state: 'COMPLETE_RESULTS', results: [] },
  enterprise: {
    executive_answer: answer,
    limitations: ['Plate readings are model observations and need analyst review.'],
    provenance: [{
      citation_id: 'plate-1',
      evidence_id: '11111111-1111-4111-8111-111111111111',
      version_id: '22222222-2222-4222-8222-222222222222',
      source_file: 'roadside-vehicle-02.jpg',
      source_truth_state: 'derived_model_observation',
      proof_role: 'exact plate reading',
      locator: { artifact_id: 'a1', bbox: { x: 10, y: 20, width: 90, height: 40 }, coordinate_space: 'original_image_pixels' },
    }],
  },
}

describe('plate search over image readings in the workspace', () => {
  it('shows the server sentence as written, not "no verified answer"', () => {
    const presentation = presentInvestigationResponse(response, { caseId: 'orion-lahore-reference-20261030' })
    expect(presentation.answer).toBe(answer)
    expect(presentation.state).not.toBe('failed')
  })

  it('lists the matched photo as an openable source', () => {
    const presentation = presentInvestigationResponse(response, { caseId: 'orion-lahore-reference-20261030' })
    expect(presentation.citations.totalOpenable).toBe(1)
    expect(presentation.citations.items[0].href).toContain('/evidence/11111111-1111-4111-8111-111111111111')
    expect(presentation.citations.items[0].strength.label).toBe('Model observation')
  })

  it('a no-match answer keeps its sentence too', () => {
    const none = {
      ...response,
      enterprise: { ...response.enterprise, executive_answer: 'No image or video reading matches the plate ZZZ9999 exactly. The plate reader may have misread it, so this does not show the plate is absent from the images or video.', provenance: [] },
    }
    const presentation = presentInvestigationResponse(none, { caseId: 'orion-lahore-reference-20261030' })
    expect(presentation.answer).toMatch(/does not show the plate is absent/)
  })
})
