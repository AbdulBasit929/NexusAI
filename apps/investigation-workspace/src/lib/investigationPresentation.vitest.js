import { describe, expect, test } from 'vitest'
import { createSemanticCatalog, semanticCatalog } from './semanticCatalog.js'
import { hasOpenableLocator, normalizeCitations } from './citations.js'
import { presentInvestigationResponse } from './investigationPresentation.js'
import { goldenFixture, loadClarificationFixtures, loadGoldenFixtures, loadRefreshedFixtures, refreshedFixture } from '../test/goldenFixtures.js'

const forbidden = /\b(?:llm|model|backend|agent|operation(?:_id)?|template|route|sql|planner|token|embedding|processor|fact packet|source-native|typed algebra)\b/i

describe('semantic display names', () => {
  test('loads the curated YAML rather than inventing labels', () => {
    expect(semanticCatalog.byId.get('cdr.call_type').display_name).toBe('Call type')
    expect(semanticCatalog.displayForValue('cdr.call_type', 'VOICE')).toBe('Voice call')
    expect(semanticCatalog.displayForColumn('CALL_TYPE', 'cdr', null)).toMatchObject({ label: 'Call type', fallback: false })
  })

  test('keeps a visible raw fallback when a label is absent', () => {
    const catalog = createSemanticCatalog({ one: { record_type: 'test', fields: [], metrics: [] } })
    expect(catalog.displayForColumn('provider_field_x', 'test', null)).toEqual({
      id: '', label: 'provider_field_x', description: '', fallback: true,
    })
  })
})

describe('legacy golden response baseline', () => {
  const fixtures = loadGoldenFixtures()

  test('covers all 62 retained legacy responses without exposing internal column headers', () => {
    expect(fixtures).toHaveLength(62)
    const states = {}
    for (const { name, response } of fixtures) {
      const view = presentInvestigationResponse(response, { caseId: response.collection_id })
      states[view.state] = (states[view.state] || 0) + 1
      expect(['answered', 'clarify', 'zero-result', 'unsupported', 'partial', 'processing', 'failed'], name).toContain(view.state)
      if (view.state === 'clarify') {
        expect(view.clarification.originalQuestion, name).not.toBe('')
        expect(view.clarification.degraded, name).toBe(true)
        continue
      }
      const analystText = [view.title, view.answer, view.derivation, ...view.limitations,
        ...view.followUps.flatMap(item => [item.label, item.reason])].filter(Boolean).join(' ')
      expect(analystText, name).not.toMatch(forbidden)
      expect(view.result.columns.map(column => column.label), name).not.toContain('M1')
      expect(view.result.columns.map(column => column.label), name).not.toContain('Metadata')
      expect(view.result.columns.map(column => column.label), name).not.toContain('Section')
    }
    expect(states.clarify).toBe(13)
    expect(states.unsupported).toBe(3)
    expect(states['zero-result']).toBeGreaterThanOrEqual(4)
  })

  test('treats a zero aggregate as a successful zero result', () => {
    const view = presentInvestigationResponse(goldenFixture('NEG-02.json'), { caseId: 'nexusai-forensic-demo' })
    expect(view.state).toBe('zero-result')
    expect(view.answer).toContain('no ANPR sightings')
    expect(view.scope.some(item => item.value === 'ZZZ-0000')).toBe(true)
  })

  test('does not collapse an unsupported request into clarification', () => {
    const view = presentInvestigationResponse(goldenFixture('CASE-03.json'), { caseId: 'nexusai-forensic-demo' })
    expect(view.state).toBe('unsupported')
    expect(view.answer).toContain('outside the evidence analysis')
  })

  test('only emits citations with exact openable locators', () => {
    let rendered = 0
    for (const { response } of fixtures) {
      const view = presentInvestigationResponse(response, { caseId: response.collection_id })
      for (const citation of view.citations?.items || []) {
        rendered += 1
        expect(citation.href).toContain('evidence')
        expect(citation.href).toMatch(/(?:row_hash|char_span|passage|source_time|frame|bbox)=/)
      }
    }
    expect(rendered).toBeGreaterThan(0)
  })
})

describe('clarification contract compatibility', () => {
  const fixtures = loadClarificationFixtures()

  test('labels both retained legacy fixture sets explicitly', () => {
    expect(loadGoldenFixtures().every(item => item.fixtureSet === 'golden-v1-legacy-20260923')).toBe(true)
    expect(fixtures).toHaveLength(8)
    expect(fixtures.every(item => item.fixtureSet === 'clarification-v2-legacy-20260923')).toBe(true)
  })

  test('preserves every v2 label and query verbatim', () => {
    let optionCount = 0
    for (const { name, response } of fixtures) {
      const view = presentInvestigationResponse(response, { caseId: response.collection_id })
      expect(view.state, name).toBe('clarify')
      expect(view.clarification.contractVersion, name).toBe('forensics.clarification-request/v2')
      expect(view.clarification.degraded, name).toBe(false)
      expect(view.clarification.options, name).toHaveLength(response.clarification.options.length)
      response.clarification.options.forEach((source, index) => {
        expect(view.clarification.options[index].label, name).toBe(source.label)
        expect(view.clarification.options[index].query, name).toBe(source.query)
      })
      optionCount += view.clarification.options.length
    }
    expect(optionCount).toBe(31)
  })

  test('keeps v1 string options visible but inert', () => {
    const view = presentInvestigationResponse({
      intent: 'clarification',
      query_understanding: { original_question: 'Original question' },
      clarification: {
        contract_version: 'forensics.clarification-request/v1',
        reason_code: 'missing_required_parameter',
        options: ['Call duration'],
      },
    })
    expect(view.clarification.options).toEqual([expect.objectContaining({ label: 'Call duration', query: '', legacy: true })])
    expect(view.clarification.degraded).toBe(true)
  })

  test.each([
    ['missing_required_parameter', 'I can answer this with one more detail'],
    ['no_verified_plan', 'I stopped before giving an unverified answer'],
    ['cross_check_disagreement', 'The available evidence does not support one verified result'],
  ])('uses distinct analyst language for %s', (reason, title) => {
    const view = presentInvestigationResponse({
      intent: 'clarification',
      query_understanding: { original_question: 'Original question' },
      clarification: { reason_code: reason, options: [] },
    })
    expect(view.clarification.title).toBe(title)
  })
})

describe('refreshed live responses', () => {
  test('loads the separately labelled current capture and renders every successful response', () => {
    const fixtures = loadRefreshedFixtures()
    expect(fixtures).toHaveLength(62)
    expect(fixtures.every(item => item.fixtureSet === 'live-20260924-refreshed')).toBe(true)
    for (const { name, response } of fixtures) {
      expect(() => presentInvestigationResponse(response, { caseId: response.collection_id }), name).not.toThrow()
    }
  })

  test('uses the same analyst vocabulary in the live CDR answer and table', () => {
    const view = presentInvestigationResponse(refreshedFixture('CDR-04.json'), { caseId: 'nexusai-forensic-demo' })
    const values = view.result.rows.map(row => row.call_type)
    expect(view.answer).toContain('Data session')
    expect(values).toContain('Data session')
    expect(view.answer).not.toContain('GPRS')
  })
})

describe('state contract beyond the current fixtures', () => {
  test.each([
    ['processing', { enterprise: { result_state: 'processing', executive_answer: 'Evidence is still processing.', data_grid: {} } }],
    ['failed', null, Object.assign(new Error('private stack detail'), { reference: 'QRY-4821' })],
    ['partial', { enterprise: { result_state: 'results_present', executive_answer: 'A bounded result is available.', proof_state: { rows: 'bounded' }, data_grid: {} } }],
  ])('implements %s', (expected, response, error) => {
    const view = presentInvestigationResponse(response, { caseId: 'case', error })
    expect(view.state).toBe(expected)
    expect(view.answer).not.toContain('private stack detail')
  })
})

test('renders a sourced document answer whose filename contains NexusAI and whose locator is page plus passage', () => {
  const sentence = 'nexusai-multimodal-acceptance-brief.pdf contains 1 cited passage matching "contact number 03001234567".'
  const citation = {
    citation_id: 'C1',
    evidence_id: 'document-1',
    version_id: 'version-1',
    source_file: 'nexusai-multimodal-acceptance-brief.pdf',
    locator: { page: 1, passage: 1, evidence_id: 'document-1', source_file: 'nexusai-multimodal-acceptance-brief.pdf' },
    proof_role: 'representative_evidence',
  }
  const view = presentInvestigationResponse({
    collection_id: 'nexusai-multimodal-product-acceptance',
    enterprise: {
      status: 'answered_with_limitations',
      result_state: 'results_present',
      executive_answer: sentence,
      narrative: { direct_answer: sentence },
      fact_packet: { citations: [citation], facts: [{ kind: 'metric', value: '03001234567' }] },
      data_grid: { columns: [], rows: [] },
    },
  }, { caseId: 'nexusai-multimodal-product-acceptance' })

  expect(view.state).toBe('answered')
  expect(view.answer).toBe(sentence)
  expect(view.answer).not.toBe('No verified answer is available.')
  expect(view.citations.items).toHaveLength(1)
  expect(view.citations.items[0]).toMatchObject({ label: 'nexusai-multimodal-acceptance-brief.pdf', detail: 'page 1, passage 1' })
  expect(view.citations.items[0].href).toContain('page=1')
  expect(view.citations.items[0].href).toContain('passage=1')
  expect(view.citations.items[0].strength.label).toBe('Source type not reported')
  expect(view.citations.groups[0].totalContributing).toBeNull()
})

test('uses source truth state instead of guessing provenance from artifact type', () => {
  const citation = {
    citation_id: 'C1',
    evidence_id: 'e-1',
    version_id: 'v-1',
    artifact_id: 'a-1',
    artifact_type: 'forensics.audio-timestamp-segment/v1',
    confidence: 0.91,
    locator: { timestamp_seconds: 4.2 },
  }
  const response = { enterprise: { fact_packet: { citations: [citation] } } }
  expect(normalizeCitations(response, 'case-1').items[0].strength).toMatchObject({
    id: 'medium',
    label: 'Source type not reported',
    confidence: null,
  })

  citation.source_truth_state = 'derived_model_observation'
  expect(normalizeCitations(response, 'case-1').items[0].strength).toMatchObject({
    id: 'strong',
    label: 'High-confidence observation',
    confidence: 0.91,
  })
})

test('locator validation rejects decorative source references', () => {
  expect(hasOpenableLocator({ evidence_id: 'e', version_id: 'v', source_file: 'calls.csv' })).toBe(false)
  expect(hasOpenableLocator({ evidence_id: 'e', version_id: 'v', locator: { page: 1 } })).toBe(false)
  expect(hasOpenableLocator({ evidence_id: 'e', version_id: 'v', locator: { page: 1, passage: 1 } })).toBe(true)
  expect(hasOpenableLocator({ evidence_id: 'e', version_id: 'v', locator: { source_row: 9, source_hash: 'abc' } })).toBe(true)
})
