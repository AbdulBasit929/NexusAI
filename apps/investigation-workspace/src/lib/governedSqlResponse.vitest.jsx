import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { InvestigationResult } from '../components/InvestigationResult.jsx'
import answered from './__fixtures__/lane_answered.json'
import breakdown from './__fixtures__/lane_breakdown.json'
import absence from './__fixtures__/lane_absence.json'
import abstain from './__fixtures__/lane_abstain.json'
import list from './__fixtures__/lane_list.json'
import { presentInvestigationResponse } from './investigationPresentation'

// Responses of the governed SQL lane (api/forensic_records/governed_sql_lane.go), as the API sends them.
describe('governed SQL lane responses in the workspace', () => {
  for (const [name, response] of Object.entries({ answered, breakdown, absence, list })) {
    it(`${name}: is presented as an answer, not as "unavailable"`, () => {
      const presentation = presentInvestigationResponse(response, { caseId: 'records-demo' })
      expect(['answered', 'partial', 'zero-result']).toContain(presentation.state)
    })
  }
  it('abstain: asks the analyst for a change and carries the reason', () => {
    const presentation = presentInvestigationResponse(abstain, { caseId: 'records-demo' })
    expect(presentation.state).toBe('clarify')
    expect(JSON.stringify(presentation)).toMatch(/could not apply|did not run/i)
  })

  for (const [name, response] of Object.entries({ answered, breakdown, absence, list, abstain })) {
    it(`${name}: renders without throwing`, () => {
      const presentation = presentInvestigationResponse(response, { caseId: 'records-demo' })
      const { container } = render(<InvestigationResult presentation={presentation} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} />)
      expect(container.textContent.length).toBeGreaterThan(10)
    })
  }
})
