import { afterEach, beforeAll, describe, expect, test, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import InvestigatePage, { investigationStarters } from './InvestigatePage.jsx'
import abstain from '../lib/__fixtures__/lane_abstain.json'
import { resetAttentionCache } from '../lib/workspaceAttention.js'
import { clearWorkspaceStateForTests } from '../lib/workspaceState.js'

describe('investigation starter questions', () => {
  const capabilities = {
    families: [
      { id: 'cdr', label: 'CDR', availability: 'queryable', record_types: ['cdr'], suggested_queries: ['Count calls', 'Show call chronology'] },
      { id: 'anpr', label: 'ANPR', availability: 'limited', record_types: ['anpr'], suggested_queries: ['Count distinct plates'] },
      { id: 'document', label: 'Documents', availability: 'no_data', suggested_queries: ['Summarise documents'] },
      { id: 'internal_key', availability: 'queryable', suggested_queries: ['Retain raw family label'] },
    ],
  }

  test('uses only curated suggestions from available families', () => {
    expect(investigationStarters(capabilities)).toEqual([
      { query: 'Count calls', family: 'CDR', scope: 'cdr' },
      { query: 'Show call chronology', family: 'CDR', scope: 'cdr' },
      { query: 'Count distinct plates', family: 'ANPR', scope: 'anpr' },
      { query: 'Retain raw family label', family: 'internal_key', scope: 'all' },
    ])
  })

  test('filters suggestions to the exact selected scope and invents no fallback', () => {
    expect(investigationStarters(capabilities, 'anpr')).toEqual([
      { query: 'Count distinct plates', family: 'ANPR', scope: 'anpr' },
    ])
    expect(investigationStarters(null)).toEqual([])
  })
})

// The case Ask page, driven the way an analyst drives it: type, submit, wait for the response.
// The governed SQL lane answers a question it cannot verify with an ABSTENTION (intent `clarification`) after the
// model's attempts, which is the slow answer an analyst waits for. That response has no citations, and the thread
// used to read `presentation.citations.groups` for every turn: the page threw while rendering and went blank.
describe('the case Ask page when the governed SQL lane abstains', () => {
  const json = body => ({ ok: true, status: 200, headers: { get: () => null }, json: async () => body, text: async () => JSON.stringify(body) })
  const box = () => screen.getByRole('textbox', { name: /ask a question about this case/i })

  beforeAll(() => {
    // jsdom has no layout: the page scrolls the new answer into view.
    Element.prototype.scrollIntoView ??= () => {}
    Element.prototype.scrollTo ??= () => {}
  })

  afterEach(() => {
    resetAttentionCache()
    clearWorkspaceStateForTests()
    vi.restoreAllMocks()
    delete window.__INVESTIGATION_WORKSPACE_CONFIG__
  })

  function withReason(response, sentence) {
    const copy = structuredClone(response)
    copy.answer.clarification = sentence
    copy.enterprise.clarification = sentence
    copy.enterprise.executive_answer = sentence
    copy.enterprise.terminal_answer.clarification = sentence
    return copy
  }

  async function askAndWait(hybridResponse, question) {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: ['records-demo'] }
    globalThis.fetch = vi.fn(async url => {
      const path = new URL(String(url)).pathname
      if (path.endsWith('/query/hybrid')) {
        await new Promise(resolve => setTimeout(resolve, 20))
        return json(hybridResponse)
      }
      if (path.endsWith('/collections/status')) return json({ summary: { evidence_total: 12, evidence_completed: 12, evidence_in_flight: 0, evidence_failed: 0 }, record_families: [] })
      return json({})
    })
    render(<MemoryRouter initialEntries={['/cases/records-demo/investigate']}><Routes><Route path="/cases/:id/investigate" element={<InvestigatePage />} /></Routes></MemoryRouter>)
    const user = userEvent.setup()
    await user.type(await screen.findByRole('textbox', { name: /ask a question about this case/i }), question)
    await user.click(screen.getByRole('button', { name: 'Ask' }))
  }

  test('shows the lane\'s reason and leaves the page usable', async () => {
    await askAndWait(abstain, 'What is the smallest position of the towers?')
    expect(await screen.findByText(/did not use the field the question names \(position\)/)).toBeTruthy()
    // The conversation and the composer are still there, so the analyst can rephrase.
    expect(screen.getByText('What is the smallest position of the towers?')).toBeTruthy()
    expect(box()).toBeTruthy()
  })

  test('states the reason in full when it names evidence that a model produced', async () => {
    // Exactly what the live lane said for "Were any of the transcribed audio segments taken from video?" on the media case:
    // the evidence's own display name contains the word "model", which must not turn the reason into a generic message.
    const reason = 'I did not run this question, because I could not apply everything it asks: the query filtered source_modality on "video_embedded_audio_transcript_segment", which the question does not ask for. Rephrase it with the exact field and value; the fields of Audio transcript segments (unreviewed model observations) are: detected_language, end_seconds, source_modality.'
    await askAndWait(withReason(abstain, reason), 'Were any of the transcribed audio segments taken from video?')
    expect(await screen.findByText(/the query filtered source_modality/)).toBeTruthy()
    expect(screen.getByText(/unreviewed model observations/)).toBeTruthy()
  })
})
