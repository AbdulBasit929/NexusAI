import { render, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { InvestigationResult } from '../components/InvestigationResult.jsx'
import { EvidencePanel } from '../pages/investigate/EvidencePanel.jsx'
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

// What the analyst is shown, not what the raw response contains. (The check above stringifies the whole presentation,
// which includes the raw response, so it passes even when the reason is never displayed.)
describe('what the lane tells the analyst', () => {
  const withSentence = (response, sentence) => {
    const copy = structuredClone(response)
    if (copy.answer) copy.answer.clarification = sentence
    copy.enterprise.clarification = sentence
    copy.enterprise.executive_answer = sentence
    return copy
  }

  it('abstain: has the empty result and citation shapes that every consumer of a presentation reads', () => {
    // The case Ask page reads `presentation.citations.groups` for every turn. A clarification without it blanked the page.
    const presentation = presentInvestigationResponse(abstain, { caseId: 'records-demo' })
    expect(presentation.citations).toMatchObject({ items: [], groups: [], totalOpenable: 0 })
    expect(presentation.result).toMatchObject({ columns: [], rows: [] })
    expect(presentation.scope).toEqual([])
    expect(presentation.limitations).toEqual([])
  })

  it('a failed question has the same empty shapes', () => {
    const presentation = presentInvestigationResponse(null, { caseId: 'records-demo', error: Object.assign(new Error('x'), { reference: 'HTTP-500' }) })
    expect(presentation.state).toBe('failed')
    expect(presentation.citations.groups).toEqual([])
    expect(presentation.errorReference).toBe('HTTP-500')
  })

  it('abstain: the clarification itself states the reason', () => {
    const presentation = presentInvestigationResponse(abstain, { caseId: 'records-demo' })
    expect(presentation.clarification.question).toMatch(/did not use the field the question names \(position\)/)
  })

  it('a reason that names evidence a model produced is still stated in full', () => {
    const sentence = 'I did not run this question, because I could not apply everything it asks: the query filtered source_modality on "video_embedded_audio_transcript_segment", which the question does not ask for. The fields of Audio transcript segments (unreviewed model observations) are: source_modality.'
    const presentation = presentInvestigationResponse(withSentence(abstain, sentence), { caseId: 'records-media' })
    expect(presentation.clarification.question).toBe(sentence)
  })

  it('an answer that uses a word the older paths filter is still stated', () => {
    const response = structuredClone(answered)
    response.enterprise.executive_answer = 'Face model: buffalo_l.'
    expect(presentInvestigationResponse(response, { caseId: 'records-media' }).answer).toBe('Face model: buffalo_l.')
  })

  it('the older paths keep the vocabulary filter', () => {
    const response = structuredClone(answered)
    response.policy = ''
    response.enterprise.executive_answer = 'The route model says 5.'
    expect(presentInvestigationResponse(response, { caseId: 'records-demo' }).answer).toBe('No verified answer is available.')
  })

  it('an answered lane response is an answer even when the image that produced it said intent "semantic"', () => {
    // Images built before the fix sent `semantic` (the intent of an analysis the case cannot do); every lane answer then
    // read "Analysis unavailable" above a correct number.
    const stale = structuredClone(answered)
    stale.intent = 'semantic'
    expect(presentInvestigationResponse(stale, { caseId: 'records-demo' }).state).toBe('answered')
  })

  it('a response that is not from the lane and says "semantic" is still unsupported', () => {
    const other = structuredClone(answered)
    other.intent = 'semantic'
    other.policy = ''
    expect(presentInvestigationResponse(other, { caseId: 'records-demo' }).state).toBe('unsupported')
  })
})

// The lane's answer is a number the database computed, and its citation is the query that computed it. The response
// carries it in enterprise.derivation; the method disclosure shows it, with the conditions the server found in it.
describe('the lane shows its work', () => {
  const withChecks = checks => {
    const copy = structuredClone(answered)
    copy.enterprise.derivation.conditions_checked = checks
    return copy
  }
  const open = async (container, name) => {
    const user = (await import('@testing-library/user-event')).default.setup()
    await user.click(within(container).getByRole('button', { name }))
  }

  it('an answer carries the query and what was checked, and says how it was derived', () => {
    const checks = ['identifier 923001110001', 'range: 2026-04-02 to 2026-04-03, both days in full']
    const presentation = presentInvestigationResponse(withChecks(checks), { caseId: 'records-demo' })
    expect(presentation.method).toEqual({ sql: 'SELECT count(*) AS number_of_records FROM v_cdr', checked: checks })
    expect(presentation.derivation).toMatch(/read-only query/)
    expect(presentation.derivation).toMatch(/computed them/)
    expect(presentation.derivation).not.toMatch(/Reviewed matching evidence/)
  })

  it('an abstention has no query to show, and an answer from another path is never given one', () => {
    expect(presentInvestigationResponse(abstain, { caseId: 'records-demo' }).method).toBeNull()
    const other = structuredClone(answered)
    other.policy = ''
    expect(presentInvestigationResponse(other, { caseId: 'records-demo' }).method).toBeNull()
    expect(presentInvestigationResponse(null, { caseId: 'records-demo', error: new Error('x') }).method).toBeNull()
  })

  it('an answer without a query shows none', () => {
    const copy = structuredClone(answered)
    copy.enterprise.derivation = { views: ['v_cdr'] }
    expect(presentInvestigationResponse(copy, { caseId: 'records-demo' }).method).toBeNull()
  })

  it('shows the query and the checks in the method disclosure of the result, left to right', async () => {
    const presentation = presentInvestigationResponse(withChecks(['identifier 923001110001']), { caseId: 'records-demo' })
    const { container } = render(<InvestigationResult presentation={presentation} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} />)
    expect(container.querySelector('.method-details')).toBeNull()
    await open(container, 'Read the analysis method')
    const sql = container.querySelector('.method-details__sql')
    expect(sql.textContent).toBe('SELECT count(*) AS number_of_records FROM v_cdr')
    expect(sql.getAttribute('dir')).toBe('ltr')
    expect([...container.querySelectorAll('.method-details__checked li')].map(li => li.textContent)).toEqual(['identifier 923001110001'])
  })

  it('shows them in the evidence panel too, and nothing for an answer that has no query', async () => {
    const presentation = presentInvestigationResponse(withChecks(['call_type = VOLTE']), { caseId: 'records-demo' })
    const { container } = render(<EvidencePanel turn={{ id: 't', query: 'How many VoLTE calls?', presentation }} onClose={() => {}} />)
    await open(container, 'Read the analysis method')
    expect(container.querySelector('.method-details__sql').textContent).toContain('SELECT count(*)')
    expect(container.querySelector('.method-details__checked').textContent).toContain('call_type = VOLTE')

    const none = structuredClone(answered)
    none.policy = ''
    const older = render(<EvidencePanel turn={{ id: 'u', query: 'q', presentation: presentInvestigationResponse(none, { caseId: 'records-demo' }) }} onClose={() => {}} />)
    await open(older.container, 'Read the analysis method')
    expect(older.container.querySelector('.method-details')).toBeNull()
  })

  it('shows no list of checks when none was needed', async () => {
    const presentation = presentInvestigationResponse(withChecks([]), { caseId: 'records-demo' })
    const { container } = render(<InvestigationResult presentation={presentation} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} />)
    await open(container, 'Read the analysis method')
    expect(container.querySelector('.method-details__sql')).toBeTruthy()
    expect(container.querySelector('.method-details__checked')).toBeNull()
  })
})
