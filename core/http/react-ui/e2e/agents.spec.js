import { test, expect } from './coverage-fixtures.js'

// Agents feature page (src/pages/Agents.jsx).
test.describe('Agents page', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/agents', route => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ agents: [], statuses: {} }),
    }))
    await page.goto('/app/agents')
  })

  test('renders the agents list and empty state', async ({ page }) => {
    await expect(page).toHaveURL(/\/app\/agents$/)
    await expect(page.getByRole('heading', { name: 'Agents', exact: true })).toBeVisible()
    await expect(page.getByText(/No agents configured/i)).toBeVisible()
    await expect(page.getByRole('button', { name: 'Create Agent' }).first()).toBeVisible()
  })

  test('Create Agent navigates to the agent creation form', async ({ page }) => {
    const create = page.getByRole('button', { name: 'Create Agent' }).last()
    await create.scrollIntoViewIfNeeded()
    await Promise.all([
      page.waitForURL(/\/app\/agents\/new$/),
      create.click(),
    ])
    // Wait for AgentCreate.jsx to actually render, not just for the URL to
    // change. Ending the test the instant the route matched let the component
    // mount race the coverage teardown — its ~400 lines were collected only
    // when the render won, swinging total UI coverage ~1pp run-to-run.
    await expect(page.getByRole('heading', { name: 'Create Agent' })).toBeVisible()
  })
})

test.describe('Forensic analyst chat', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/agents/Forensic_Records_Analyst/config', async route => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          name: 'Forensic_Records_Analyst',
          model: 'qwen_qwen3-4b-instruct-2507',
          enable_forensic_records: true,
          forensic_collection_id: 'persisted-agent-collection-must-not-win',
          forensic_tenant_id: 'default',
        }),
      })
    })
    await page.route('**/api/v1/forensics/cases', async route => {
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
        default_case_id: 'case-alpha',
        cases: [
          { case_id: 'case-alpha', collection_id: 'collection-alpha', display_name: 'Case Alpha', selectable: true },
          { case_id: 'case-beta', collection_id: 'collection-beta', display_name: 'Case Beta', selectable: true },
        ],
      }) })
    })
    await page.route('**/api/records/forensic/capabilities**', async route => {
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
        summary: { queryable: 2 },
        families: [{ id: 'cdr', label: 'Communications CDR', available: true }, { id: 'anpr', label: 'ANPR', available: true }],
        query_corpus: {
          contract_version: 'forensics.family-query-answer-corpus/v1',
          corpus_version: '2026-08-11.r6.6',
          entries: [
            { id: 'curated-call-types', suggested: true, query: 'show the CDR call type breakdown', family_id: 'cdr', expected_template: 'call_type_breakdown' },
            { id: 'catalog-only', suggested: false, query: 'Run deterministic forensic query; template=source_records; limit=20', family_id: 'cross_family', expected_template: 'source_records' },
          ],
        },
      }) })
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/history?**', async route => {
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
        contract_version: 'forensics.case-analysis-history/v1', items: [], has_more: false,
      }) })
    })
  })

  test('shows case/model assurance and runs a suggested exact query', async ({ page }) => {
    const sentMessages = []
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', async route => {
      sentMessages.push(route.request().postDataJSON())
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ message_id: 'forensic-ui-test' }),
      })
    })

    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')

    await expect(page.getByText('Ask NexusAI', { exact: true }).first()).toBeVisible()
    await expect(page.getByLabel('Active forensic case')).toHaveValue('case-alpha')
    await page.getByText('Status & scope', { exact: true }).click()
    await expect(page.getByText('collection-alpha', { exact: true })).toBeVisible()
    await expect(page.getByText('persisted-agent-collection-must-not-win', { exact: true })).toHaveCount(0)
    await expect(page.getByText('qwen_qwen3-4b-instruct-2507', { exact: true })).toBeVisible()
    await expect(page.getByText('Verified records & cited evidence', { exact: true })).toBeVisible()

    await page.getByRole('button', { name: /Correlate an entity/ }).first().click()
    await expect.poll(() => sentMessages.length).toBe(1)
    expect(sentMessages[0]).toEqual({
      message: 'correlate 35678901234567 across record families',
      case_id: 'case-alpha',
      collection_id: 'collection-alpha',
    })
    await page.setViewportSize({ width: 390, height: 844 })
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1)
  })

  test('keeps long answers reachable with an explicit jump-to-latest recovery control', async ({ page }) => {
    await page.addInitScript(() => {
      class MockEventSource {
        constructor() { this.listeners = {}; window.__agentEventSource = this }
        addEventListener(type, callback) { (this.listeners[type] ||= []).push(callback) }
        close() {}
        emit(type, payload) { for (const callback of this.listeners[type] || []) callback({ data: JSON.stringify(payload) }) }
      }
      window.EventSource = MockEventSource
      window.__emitAgentEvent = (type, payload) => window.__agentEventSource?.emit(type, payload)
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ message_id: 'request-long-answer' }) }))
    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    await page.getByLabel('Analyst query').fill('show a long governed result')
    await page.getByRole('button', { name: 'Send message' }).click()
    await page.evaluate(() => window.__emitAgentEvent('json_message', {
      sender: 'agent', message_id: 'request-long-answer-agent', case_id: 'case-alpha', collection_id: 'collection-alpha',
      content: Array.from({ length: 90 }, (_, index) => `Evidence line ${index + 1}`).join('\n\n'), timestamp: Date.now(),
    }))
    const messages = page.getByLabel('Conversation messages')
    await expect(messages.getByText('Evidence line 90', { exact: true })).toBeVisible()
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0)
    await messages.evaluate(element => { element.scrollTop = 0; element.dispatchEvent(new Event('scroll')) })
    const jump = page.getByRole('button', { name: 'Jump to latest' })
    await expect(jump).toBeVisible()
    await jump.click()
    await expect(jump).toBeHidden()
    await expect.poll(() => messages.evaluate(element => element.scrollHeight - element.scrollTop - element.clientHeight)).toBeLessThan(90)
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0)

    await messages.evaluate(element => { element.scrollTop = 0; element.dispatchEvent(new Event('scroll')) })
    await page.getByLabel('Analyst query').fill('start a visible follow-up')
    await page.getByRole('button', { name: 'Send message' }).click()
    await expect(messages.getByText('start a visible follow-up', { exact: true })).toBeVisible()
    await expect.poll(() => messages.evaluate(element => element.scrollHeight - element.scrollTop - element.clientHeight)).toBeLessThan(90)
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0)
  })

  test('renders only curated corpus prompts and exposes typed specialist provenance accessibly', async ({ page }) => {
    const sentMessages = []
    await page.addInitScript(() => {
      class MockEventSource {
        constructor() { this.listeners = {}; window.__agentEventSource = this }
        addEventListener(type, callback) { (this.listeners[type] ||= []).push(callback) }
        close() {}
        emit(type, payload) { for (const callback of this.listeners[type] || []) callback({ data: JSON.stringify(payload) }) }
      }
      window.EventSource = MockEventSource
      window.__emitAgentEvent = (type, payload) => window.__agentEventSource?.emit(type, payload)
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', async route => {
      sentMessages.push(route.request().postDataJSON())
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ message_id: 'request-r64' }) })
    })

    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    const curatedPrompt = page.getByRole('button', { name: /Communications CDR.*call_type_breakdown/i })
    await expect(curatedPrompt).toBeVisible()
    await expect(page.getByRole('button', { name: /source_records/i })).toHaveCount(0)
    await curatedPrompt.click()
    await expect.poll(() => sentMessages.length).toBe(1)
    expect(sentMessages[0].message).toBe('show the CDR call type breakdown')
    await page.evaluate(() => window.__emitAgentEvent('json_message', {
      sender: 'agent', message_id: 'request-r64-agent', case_id: 'case-alpha', collection_id: 'collection-alpha',
      content: 'Computed from exact CDR rows.', timestamp: Date.now(),
      answer_metadata: { presentation: {
        contract_version: 'forensics.agent-presentation/v1', executive_answer: 'Call activity by type',
        operation_id: 'forensics.call_type_breakdown', family_id: 'communications_cdr',
        specialist: 'Communications_CDR_Analyst', source_access: 'records', elapsed_ms: 18,
        execution_authority: 'deterministic_records', model_status: 'not_used', tool_id: 'forensic_records.hybrid_query',
        status: 'answered_with_limitations', result_kind: 'table', model_status: 'fallback', language: 'en', text_direction: 'ltr',
        context: { target: '923001110001', template: 'frequent_contacts', direction: '' },
        findings: [{ text: 'VOICE contains 12 exact events.', claim_type: 'deterministic_fact' }],
        relationships: [{ relationship_id: 'R1', type: 'observed_association', strength: 'observed_association', source: '923001110001', target: 'VOICE', citation_ids: ['C1'] }],
        table: { title: 'CDR call types', columns: ['call_type', 'event_count', 'direction', 'duration_seconds', 'observed_at', 'source_file', 'row_number', 'evidence_id', 'version_id'], priority_columns: ['call_type', 'event_count'], column_visibility: 'progressive', rows: [{ call_type: 'VOICE', event_count: 12, direction: 'outgoing', duration_seconds: 60, observed_at: '2026-08-17T08:30:00+05:00', source_file: 'cdr.csv', row_number: 7, evidence_id: 'evidence-cdr-1', version_id: 'version-cdr-1' }] },
        citations: [{ label: 'cdr.csv', detail: '12 contributing rows · complete lineage', proof_role: 'aggregate_contribution_lineage', completeness: 'complete', source_file: 'cdr.csv', evidence_id: 'evidence-cdr-1' }], limitations: ['Call records do not establish communication content.'], proof_state: { rows: 'complete', citations: 'complete' },
      } },
    }))

    const result = page.getByRole('region', { name: 'Forensic analysis result' })
    await expect(result.getByRole('table', { name: 'CDR call types' })).toBeVisible()
    await expect(result.getByRole('columnheader', { name: 'Call Type' })).toBeVisible()
    await expect(result).toContainText('2 of 9 columns')
    await expect(result.getByRole('heading', { name: 'Key findings' })).toBeVisible()
    await expect(result.getByRole('heading', { name: 'Relationships' })).toBeVisible()
    await expect(result.getByText(/Aggregate contribution lineage/).first()).toBeVisible()
    await result.getByText('Columns', { exact: true }).click()
    await result.getByLabel('Source File').check()
    await expect(result.getByRole('columnheader', { name: 'Source File' })).toBeVisible()
    await result.getByText('Columns', { exact: true }).click()
    await expect(result.getByRole('button', { name: 'Open evidence source cdr.csv' })).toBeVisible()
    await result.getByText('How this was determined', { exact: true }).click()
    await expect(result).toContainText('Communications CDR Analyst')

    await page.evaluate(() => window.__emitAgentEvent('json_message_status', {
      status: 'completed', message_id: 'request-r64',
    }))
    await page.getByLabel('Analyst query').fill('Only outgoing.')
    await page.getByRole('button', { name: 'Send message' }).click()
    await expect.poll(() => sentMessages.length).toBe(2)
    expect(sentMessages[1].conversation_context).toEqual(expect.objectContaining({
      target: '923001110001', template: 'frequent_contacts', direction: '',
    }))
    await result.getByRole('button', { name: 'Open evidence source cdr.csv' }).click()
    await expect(page).toHaveURL(/\/app\/cases\/case-alpha\/evidence\?evidence=evidence-cdr-1$/)
  })

  test('renders Urdu results RTL while exact identifiers remain LTR at mobile width', async ({ page }) => {
    await page.addInitScript(() => {
      class MockEventSource {
        constructor() { this.listeners = {}; window.__agentEventSource = this }
        addEventListener(type, callback) { (this.listeners[type] ||= []).push(callback) }
        close() {}
        emit(type, payload) { for (const callback of this.listeners[type] || []) callback({ data: JSON.stringify(payload) }) }
      }
      window.EventSource = MockEventSource
      window.__emitAgentEvent = (type, payload) => window.__agentEventSource?.emit(type, payload)
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ message_id: 'request-urdu' }) }))
    await page.setViewportSize({ width: 390, height: 820 })
    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    await page.getByLabel('Analyst query').fill('صرف آؤٹ گوئنگ سرگرمی دکھائیں')
    await page.getByRole('button', { name: 'Send message' }).click()
    await page.evaluate(() => window.__emitAgentEvent('json_message', {
      sender: 'agent', message_id: 'request-urdu-agent', case_id: 'case-alpha', collection_id: 'collection-alpha', content: 'Exact Urdu result.', timestamp: Date.now(),
      answer_metadata: { presentation: {
        contract_version: 'forensics.agent-presentation/v1', status: 'answered', executive_answer: 'نمبر 923001110001 کے 3 آؤٹ گوئنگ واقعات ملے۔', operation_id: 'cdr.temporal_activity', family_id: 'communications_cdr', execution_authority: 'deterministic_records', model_status: 'fallback', elapsed_ms: 14, language: 'ur', text_direction: 'rtl',
        findings: [{ text: 'نمبر 923001110001 کے 3 آؤٹ گوئنگ واقعات ملے۔', claim_type: 'deterministic_fact' }],
        table: { title: 'آؤٹ گوئنگ سرگرمی', columns: ['target', 'event_count'], priority_columns: ['target', 'event_count'], rows: [{ target: '923001110001', event_count: 3 }] },
        citations: [{ label: 'cdr.csv', detail: 'row 7', proof_role: 'representative_evidence', completeness: 'representative' }], limitations: ['یہ ریکارڈ گفتگو کا متن ثابت نہیں کرتے۔'],
      } },
    }))
    const result = page.getByRole('region', { name: 'Forensic analysis result' })
    await expect(result).toHaveAttribute('dir', 'rtl')
    await expect(result.locator('bdi[dir="ltr"]').filter({ hasText: '923001110001' }).first()).toBeVisible()
    const bounds = await result.evaluate(element => ({ width: element.clientWidth, content: element.scrollWidth }))
    expect(bounds.content).toBeLessThanOrEqual(bounds.width)
  })

  test('renders synchronized telecom timeline, uncertainty-aware coordinates, and evidence details', async ({ page }) => {
    await page.addInitScript(() => {
      class MockEventSource {
        constructor() { this.listeners = {}; window.__agentEventSource = this }
        addEventListener(type, callback) { (this.listeners[type] ||= []).push(callback) }
        close() {}
        emit(type, payload) { for (const callback of this.listeners[type] || []) callback({ data: JSON.stringify(payload) }) }
      }
      window.EventSource = MockEventSource
      window.__emitAgentEvent = (type, payload) => window.__agentEventSource?.emit(type, payload)
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ message_id: 'request-r76' }) }))
    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    await page.getByLabel('Analyst query').fill('join cdr to tower PK-LHR-SYN-001')
    await page.getByRole('button', { name: 'Send message' }).click()
    const rows = [
      { cdr_observed_at: '2026-08-01T10:15:00Z', cell_site_id: 'PK-LHR-SYN-001', site_identifier: 'LHR-001', latitude: 31.5204, longitude: 74.3587, coordinate_datum: 'WGS84', uncertainty_radius_m: 125, uncertainty_class: 'provider_supplied', match_status: 'matched', cdr_source_file: 'synthetic-cdr.csv', cdr_row_number: 4 },
      { cdr_observed_at: '2026-08-01T10:25:00Z', cell_site_id: 'PK-LHR-SYN-001', site_identifier: 'LHR-001-B', latitude: 31.5212, longitude: 74.3592, coordinate_datum: 'WGS84', uncertainty_radius_m: 300, uncertainty_class: 'estimated', match_status: 'ambiguous_overlapping_references', cdr_source_file: 'synthetic-cdr.csv', cdr_row_number: 5 },
    ]
    await page.evaluate(visualRows => window.__emitAgentEvent('json_message', {
      sender: 'agent', message_id: 'request-r76-agent', case_id: 'case-alpha', collection_id: 'collection-alpha', content: 'Exact tower join.', timestamp: Date.now(),
      answer_metadata: { presentation: {
        contract_version: 'forensics.agent-presentation/v1', executive_answer: 'Two exact CDR observations were evaluated.', operation_id: 'tower.cdr_join', family_id: 'tower_location', specialist: 'Tower_Location_Reference_Analyst', source_access: 'records', execution_authority: 'deterministic_records', model_status: 'not_used', elapsed_ms: 22,
        table: { title: 'Time-valid tower join', columns: ['cdr_observed_at', 'cell_site_id', 'match_status'], rows: visualRows },
        visualizations: [
          { id: 'tower-reference-map', type: 'map', title: 'Supplied tower/site reference coordinates', citation_ids: ['citation-tower-1'], spec: { latitude_field: 'latitude', longitude_field: 'longitude', uncertainty_field: 'uncertainty_radius_m', match_status_field: 'match_status', rows: visualRows } },
          { id: 'tower-reference-timeline', type: 'timeline', title: 'Tower/site reference history', citation_ids: ['citation-tower-1'], spec: { time_fields: ['cdr_observed_at'], label_fields: ['match_status', 'site_identifier'], match_status_field: 'match_status', rows: visualRows } },
        ],
        limitations: ['No route, RF coverage, or device presence is inferred.'],
      } },
    }, visualRows), rows)

    const result = page.getByRole('region', { name: 'Forensic analysis result' })
    await expect(result.getByRole('figure', { name: 'Supplied tower/site reference coordinates' })).toBeVisible()
    await expect(result.getByRole('figure', { name: 'Tower/site reference history' })).toBeVisible()
    await expect(result.getByRole('complementary', { name: 'Selected visual evidence details' })).toContainText('125 m')
    await result.getByRole('button', { name: /Ambiguous Overlapping References/ }).click()
    await expect(result.getByRole('complementary', { name: 'Selected visual evidence details' })).toContainText('300 m')
    await expect(result).toContainText('synthetic-cdr.csv · row 5')
    await page.setViewportSize({ width: 390, height: 844 })
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1)
  })

  test('resolves the governed default through the provider instead of agent configuration', async ({ page }) => {
    await page.goto('/app/agents/Forensic_Records_Analyst/chat')

    await expect(page).toHaveURL(/\/app\/agents\/Forensic_Records_Analyst\/chat\?case=case-alpha$/)
    await expect(page.getByLabel('Active forensic case')).toHaveValue('case-alpha')
    await page.getByText('Status & scope', { exact: true }).click()
    await expect(page.getByText('collection-alpha', { exact: true })).toBeVisible()
  })

  test('preserves the governed case and analyst question when returning to Ask NexusAI', async ({ page }) => {
    const sentMessages = []
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', async route => {
      sentMessages.push(route.request().postDataJSON())
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ message_id: 'continuity-request' }) })
    })

    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha&prompt=show%20top%20contacts')
    const returnToAsk = page.getByRole('button', { name: 'Back to Ask NexusAI' })
    await expect(page.getByLabel('Analyst query')).toHaveValue('show top contacts')
    await expect(returnToAsk).toBeEnabled()

    await page.getByRole('button', { name: 'Send message' }).click()
    await expect.poll(() => sentMessages.length).toBe(1)

    await page.getByLabel('Active forensic case').selectOption('case-beta')
    await expect(page).toHaveURL(/case=case-beta/)
    await returnToAsk.click()
    await expect(page).toHaveURL(/\/app\/cases\/case-beta\/ask\?prompt=show(?:%20|\+)top(?:%20|\+)contacts$/)
  })

  test('fails closed for an inaccessible explicit case without sending agent work', async ({ page }) => {
    const sentMessages = []
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', async route => {
      sentMessages.push(route.request().postDataJSON())
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ message_id: 'must-not-send' }) })
    })

    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-inaccessible')
    await expect(page.getByRole('alert')).toContainText('Case case-inaccessible is not an authorized accessible case')
    await expect(page.getByLabel('Analyst query')).toBeDisabled()
    expect(sentMessages).toHaveLength(0)
  })

  test('isolates conversations and rejects a late SSE answer after a case switch', async ({ page }) => {
    const sentMessages = []
    await page.addInitScript(() => {
      class MockEventSource {
        constructor(url) {
          this.url = url
          this.listeners = {}
          window.__agentEventSource = this
        }
        addEventListener(type, callback) {
          this.listeners[type] = this.listeners[type] || []
          this.listeners[type].push(callback)
        }
        close() {}
        emit(type, payload) {
          for (const callback of this.listeners[type] || []) callback({ data: JSON.stringify(payload) })
        }
      }
      window.EventSource = MockEventSource
      window.__emitAgentEvent = (type, payload) => window.__agentEventSource?.emit(type, payload)
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', async route => {
      const body = route.request().postDataJSON()
      sentMessages.push(body)
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ message_id: `request-${body.case_id}` }) })
    })

    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    await page.getByLabel('Analyst query').fill('alpha-only question')
    await page.getByRole('button', { name: 'Send message' }).click()
    await expect.poll(() => sentMessages.length).toBe(1)
    expect(sentMessages[0]).toMatchObject({ case_id: 'case-alpha', collection_id: 'collection-alpha' })
    const messages = page.locator('.chat-messages')
    await expect(messages.getByText('alpha-only question', { exact: true })).toBeVisible()

    await page.getByLabel('Active forensic case').selectOption('case-beta')
    await expect(page).toHaveURL(/case=case-beta/)
    await expect(messages.getByText('alpha-only question', { exact: true })).toHaveCount(0)
    await page.evaluate(() => window.__emitAgentEvent('json_message', {
      sender: 'agent',
      message_id: 'request-case-alpha-agent',
      case_id: 'case-alpha',
      collection_id: 'collection-alpha',
      content: 'OLD CASE ANSWER MUST NOT RENDER',
      timestamp: Date.now(),
    }))
    await expect(messages.getByText('OLD CASE ANSWER MUST NOT RENDER')).toHaveCount(0)

    await page.getByLabel('Analyst query').fill('beta-only question')
    await page.getByRole('button', { name: 'Send message' }).click()
    await expect.poll(() => sentMessages.length).toBe(2)
    expect(sentMessages[1]).toMatchObject({ case_id: 'case-beta', collection_id: 'collection-beta' })
    await page.evaluate(() => window.__emitAgentEvent('json_message', {
      sender: 'agent',
      message_id: 'request-case-beta-agent',
      case_id: 'case-beta',
      collection_id: 'collection-beta',
      content: 'BETA CASE ANSWER',
      timestamp: Date.now(),
    }))
    await expect(messages.getByText('BETA CASE ANSWER')).toBeVisible()

    await page.getByLabel('Active forensic case').selectOption('case-alpha')
    await expect(messages.getByText('alpha-only question', { exact: true })).toBeVisible()
    await expect(messages.getByText('BETA CASE ANSWER')).toHaveCount(0)
    await expect(messages.getByText('OLD CASE ANSWER MUST NOT RENDER')).toHaveCount(0)
  })

  test('keeps interleaved same-case lifecycle events bound to their originating request', async ({ page }) => {
    const sentMessages = []
    await page.addInitScript(() => {
      class MockEventSource {
        constructor(url) {
          this.url = url
          this.listeners = {}
          window.__agentEventSource = this
        }
        addEventListener(type, callback) {
          this.listeners[type] = this.listeners[type] || []
          this.listeners[type].push(callback)
        }
        close() {}
        emit(type, payload) {
          for (const callback of this.listeners[type] || []) callback({ data: JSON.stringify(payload) })
        }
      }
      window.EventSource = MockEventSource
      window.__emitAgentEvent = (type, payload) => window.__agentEventSource?.emit(type, payload)
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', async route => {
      const body = route.request().postDataJSON()
      sentMessages.push(body)
      const messageId = body.message.startsWith('first') ? 'request-first' : 'request-second'
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ message_id: messageId }) })
    })

    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    const chatMessages = page.locator('.chat-messages')
    await page.getByLabel('Analyst query').fill('first concurrent question')
    await page.getByRole('button', { name: 'Send message' }).click()
    await expect.poll(() => sentMessages.length).toBe(1)

    await page.getByRole('button', { name: 'New Chat' }).click()
    await page.getByLabel('Analyst query').fill('second concurrent question')
    await page.getByRole('button', { name: 'Send message' }).click()
    await expect.poll(() => sentMessages.length).toBe(2)

    await page.evaluate(() => {
      window.__emitAgentEvent('stream_event', { message_id: 'request-first', type: 'content', content: 'FIRST STREAM MUST NOT RENDER' })
      window.__emitAgentEvent('stream_event', { message_id: 'request-second', type: 'content', content: 'SECOND STREAM IS ACTIVE' })
    })
    await expect(chatMessages.getByText('SECOND STREAM IS ACTIVE', { exact: true })).toBeVisible()
    await expect(chatMessages.getByText('FIRST STREAM MUST NOT RENDER', { exact: true })).toHaveCount(0)

    await page.evaluate(() => window.__emitAgentEvent('json_message', {
      sender: 'agent',
      message_id: 'request-first-agent',
      content: 'FIRST FINAL ANSWER',
      timestamp: Date.now(),
    }))
    await expect(chatMessages.getByText('FIRST FINAL ANSWER', { exact: true })).toHaveCount(0)
    await expect(chatMessages.getByText('SECOND STREAM IS ACTIVE', { exact: true })).toBeVisible()

    await page.evaluate(() => window.__emitAgentEvent('json_message', {
      sender: 'agent',
      message_id: 'request-second-agent',
      content: 'SECOND FINAL ANSWER',
      timestamp: Date.now(),
    }))
    await expect(chatMessages.getByText('SECOND FINAL ANSWER', { exact: true })).toBeVisible()
    await expect(chatMessages.getByText('SECOND STREAM IS ACTIVE', { exact: true })).toHaveCount(0)

    await page.locator('.chat-list-item').filter({ hasText: 'first concurrent question' }).click()
    await expect(chatMessages.getByText('FIRST FINAL ANSWER', { exact: true })).toBeVisible()
    await expect(chatMessages.getByText('SECOND FINAL ANSWER', { exact: true })).toHaveCount(0)
  })

  test('stops only the active governed request and waits for correlated cancellation', async ({ page }) => {
    const cancellations = []
    await page.addInitScript(() => {
      class MockEventSource {
        constructor() { this.listeners = {}; window.__agentEventSource = this }
        addEventListener(type, callback) { (this.listeners[type] ||= []).push(callback) }
        close() {}
        emit(type, payload) { for (const callback of this.listeners[type] || []) callback({ data: JSON.stringify(payload) }) }
      }
      window.EventSource = MockEventSource
      window.__emitAgentEvent = (type, payload) => window.__agentEventSource?.emit(type, payload)
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', route => route.fulfill({
      contentType: 'application/json', body: JSON.stringify({ message_id: 'request-stop' }),
    }))
    await page.route('**/api/agents/Forensic_Records_Analyst/chat/request-stop/cancel', async route => {
      cancellations.push(route.request().postDataJSON())
      await route.fulfill({ contentType: 'application/json', status: 202, body: JSON.stringify({ status: 'cancellation_requested' }) })
    })

    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    await page.getByLabel('Analyst query').fill('long governed analysis')
    await page.getByRole('button', { name: 'Send message' }).click()
    await page.getByRole('button', { name: 'Stop request' }).click()
    await expect.poll(() => cancellations.length).toBe(1)
    expect(cancellations[0]).toEqual({ case_id: 'case-alpha', collection_id: 'collection-alpha' })
    await expect(page.getByRole('button', { name: 'Stop request' })).toBeDisabled()

    await page.evaluate(() => window.__emitAgentEvent('json_message_status', {
      status: 'cancelled', message_id: 'request-stop',
    }))
    await expect(page.locator('.chat-messages').getByText('Request stopped by analyst.', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Send message' })).toBeVisible()
  })

  test('surfaces a correlated governed timeout without retrying the request', async ({ page }) => {
    const sentMessages = []
    await page.addInitScript(() => {
      class MockEventSource {
        constructor() { this.listeners = {}; window.__agentEventSource = this }
        addEventListener(type, callback) { (this.listeners[type] ||= []).push(callback) }
        close() {}
        emit(type, payload) { for (const callback of this.listeners[type] || []) callback({ data: JSON.stringify(payload) }) }
      }
      window.EventSource = MockEventSource
      window.__emitAgentEvent = (type, payload) => window.__agentEventSource?.emit(type, payload)
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', async route => {
      sentMessages.push(route.request().postDataJSON())
      await route.fulfill({
        contentType: 'application/json', body: JSON.stringify({ message_id: 'request-timeout' }),
      })
    })

    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    await page.getByLabel('Analyst query').fill('bounded governed analysis')
    await page.getByRole('button', { name: 'Send message' }).click()
    await expect.poll(() => sentMessages.length).toBe(1)

    await page.evaluate(() => window.__emitAgentEvent('json_message_status', {
      status: 'timed_out', message_id: 'request-timeout',
    }))
    await expect(page.locator('.chat-messages').getByText(
      'Request timed out. No automatic retry was sent.',
      { exact: true },
    )).toBeVisible()
    await expect(page.getByRole('button', { name: 'Send message' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Stop request' })).toHaveCount(0)
    expect(sentMessages).toHaveLength(1)
  })

  test('reconciles the exact governed request after SSE reconnect without replay or retry', async ({ page }) => {
    const sentMessages = []
    const statusRequests = []
    await page.addInitScript(() => {
      class MockEventSource {
        constructor() { this.listeners = {}; window.__agentEventSource = this }
        addEventListener(type, callback) { (this.listeners[type] ||= []).push(callback) }
        close() {}
        emit(type, payload) { for (const callback of this.listeners[type] || []) callback({ data: JSON.stringify(payload) }) }
        reopen() { this.onopen?.() }
      }
      window.EventSource = MockEventSource
      window.__reopenAgentEventSource = () => window.__agentEventSource?.reopen()
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/chat/request-reconnect/status**', async route => {
      statusRequests.push(route.request().url())
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
        agent_name: 'Forensic_Records_Analyst', case_id: 'case-alpha',
        message_id: 'request-reconnect', status: 'completed',
      }) })
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', async route => {
      sentMessages.push(route.request().postDataJSON())
      await route.fulfill({
        contentType: 'application/json', body: JSON.stringify({ message_id: 'request-reconnect' }),
      })
    })

    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    await page.getByLabel('Analyst query').fill('reconnect governed analysis')
    await page.getByRole('button', { name: 'Send message' }).click()
    await expect.poll(() => sentMessages.length).toBe(1)
    await page.evaluate(() => window.__reopenAgentEventSource())

    await expect.poll(() => statusRequests.length).toBe(1)
    const statusURL = new URL(statusRequests[0])
    expect(statusURL.searchParams.get('case_id')).toBe('case-alpha')
    expect(statusURL.searchParams.get('collection_id')).toBe('collection-alpha')
    await expect(page.locator('.chat-messages').getByText(
      'Request completed offline. Replay unavailable.',
      { exact: true },
    )).toBeVisible()
    await expect(page.getByRole('button', { name: 'Send message' })).toBeVisible()
    expect(sentMessages).toHaveLength(1)
  })

  test('offers one analyst-controlled retry with scoped idempotency lineage', async ({ page }) => {
    const retries = []
    await page.addInitScript(() => {
      class MockEventSource {
        constructor() { this.listeners = {}; window.__agentEventSource = this }
        addEventListener(type, callback) { (this.listeners[type] ||= []).push(callback) }
        close() {}
        emit(type, payload) { for (const callback of this.listeners[type] || []) callback({ data: JSON.stringify(payload) }) }
      }
      window.EventSource = MockEventSource
      window.__emitAgentEvent = (type, payload) => window.__agentEventSource?.emit(type, payload)
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', route => route.fulfill({
      contentType: 'application/json', body: JSON.stringify({ message_id: 'request-retry-source' }),
    }))
    await page.route('**/api/agents/Forensic_Records_Analyst/chat/request-retry-source/retry', async route => {
      retries.push(route.request().postDataJSON())
      await route.fulfill({ contentType: 'application/json', status: 202, body: JSON.stringify({
        message_id: 'request-retry-result', retry_of: 'request-retry-source',
        case_id: 'case-alpha', idempotent_replay: false,
      }) })
    })

    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    await page.getByLabel('Analyst query').fill('retry this exact governed query')
    await page.getByRole('button', { name: 'Send message' }).click()
    await page.evaluate(() => window.__emitAgentEvent('json_message_status', {
      status: 'timed_out', message_id: 'request-retry-source',
    }))
    await page.getByRole('button', { name: 'Retry once' }).click()

    await expect.poll(() => retries.length).toBe(1)
    expect(retries[0].message).toBe('retry this exact governed query')
    expect(retries[0].case_id).toBe('case-alpha')
    expect(retries[0].collection_id).toBe('collection-alpha')
    expect(retries[0].idempotency_key.length).toBeGreaterThanOrEqual(16)
    await expect(page.getByRole('button', { name: 'Retry once' })).toHaveCount(0)
  })

  test('uses retained server history as saved-analysis authority', async ({ page }) => {
    let savedRequest = null
    const retained = {
      analysis_id: 'analysis-1', contract_version: 'forensics.case-analysis-history/v1',
      agent_name: 'Forensic_Records_Analyst', case_id: 'case-alpha', collection_id: 'collection-alpha',
      message_id: 'request-history-1', query: 'retained governed query', answer: 'RETAINED GOVERNED ANSWER',
      status: 'completed', execution_authority: 'deterministic_records', model_role: 'explanation_after_exact_analysis',
      source: 'runtime', saved: false, legal_hold: false,
      created_at: '2026-08-10T08:00:00Z', updated_at: '2026-08-10T08:00:01Z', completed_at: '2026-08-10T08:00:01Z',
    }
    await page.route('**/api/agents/Forensic_Records_Analyst/history?**', route => route.fulfill({
      contentType: 'application/json', body: JSON.stringify({ contract_version: 'forensics.case-analysis-history/v1', items: [retained], has_more: false }),
    }))
    await page.route('**/api/agents/Forensic_Records_Analyst/history/analysis-1/saved', async route => {
      savedRequest = route.request().postDataJSON()
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ ...retained, saved: true, saved_at: '2026-08-10T08:01:00Z' }) })
    })

    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    await page.getByRole('button', { name: 'Show conversations and case history' }).click()
    await page.getByText('retained governed query', { exact: true }).click()
    await expect(page.getByText('RETAINED GOVERNED ANSWER', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Show conversations and case history' })).toHaveAttribute('aria-expanded', 'false')
    await expect(page.getByText(/Retained system-of-record view/)).toBeVisible()
    await page.getByRole('button', { name: 'Save', exact: true }).click()
    expect(savedRequest).toEqual({ case_id: 'case-alpha', collection_id: 'collection-alpha', saved: true, title: '' })
  })

  test('imports browser history only after confirmation and exposes guarded rollback', async ({ page }) => {
    let imported = null
    let rollbackURL = ''
    await page.route('**/api/agents/Forensic_Records_Analyst/chat', route => route.fulfill({
      contentType: 'application/json', body: JSON.stringify({ message_id: 'request-local-import' }),
    }))
    await page.route('**/api/agents/Forensic_Records_Analyst/history/import', async route => {
      imported = route.request().postDataJSON()
      await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({
        contract_version: 'forensics.case-analysis-history/v1', import_id: 'import-1', imported: 1, skipped: 0, rollback_available: true,
      }) })
    })
    await page.route('**/api/agents/Forensic_Records_Analyst/history/imports/import-1?**', async route => {
      rollbackURL = route.request().url()
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ import_id: 'import-1', deleted: 1, retained_saved: 0, retained_legal_hold: 0 }) })
    })

    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')
    await page.getByLabel('Analyst query').fill('browser query to import')
    await page.getByRole('button', { name: 'Send message' }).click()
    await page.getByRole('button', { name: 'Import local history' }).click()
    await page.getByRole('button', { name: 'Import explicitly' }).click()
    await expect.poll(() => imported).not.toBeNull()
    expect(imported.contract_version).toBe('browser-agent-history/v1')
    expect(imported.case_id).toBe('case-alpha')
    expect(imported.collection_id).toBe('collection-alpha')
    expect(imported.conversations.some(conversation => conversation.messages.some(message => message.content === 'browser query to import'))).toBe(true)
    await page.getByRole('button', { name: 'Undo latest import' }).click()
    await page.getByRole('button', { name: 'Roll back import' }).click()
    await expect.poll(() => rollbackURL).toContain('/history/imports/import-1')
    const rollback = new URL(rollbackURL)
    expect(rollback.searchParams.get('case_id')).toBe('case-alpha')
    expect(rollback.searchParams.get('collection_id')).toBe('collection-alpha')
  })

  test('opens retained case history as an accessible mobile drawer', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/app/agents/Forensic_Records_Analyst/chat?case=case-alpha')

    const historyToggle = page.getByRole('button', { name: 'Show conversations and case history', exact: true })
    await expect(historyToggle).toHaveAttribute('aria-expanded', 'false')
    await historyToggle.click()

    await expect(page.getByText('Retained case history', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Hide conversations and case history', exact: true })).toHaveAttribute('aria-expanded', 'true')
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  })
})

test('generic Agent Chat keeps its unscoped compatibility payload', async ({ page }) => {
  const sentMessages = []
  const caseRegistryRequests = []
  page.on('request', request => {
    if (new URL(request.url()).pathname === '/api/v1/forensics/cases') caseRegistryRequests.push(request.url())
  })
  await page.route('**/api/agents/General_Assistant/config', route => route.fulfill({
    contentType: 'application/json',
    body: JSON.stringify({ name: 'General_Assistant', model: 'automatic', enable_forensic_records: false }),
  }))
  await page.route('**/api/agents/General_Assistant/chat', async route => {
    sentMessages.push(route.request().postDataJSON())
    await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ message_id: 'generic-request' }) })
  })

  await page.goto('/app/agents/General_Assistant/chat')
  await expect(page.getByRole('button', { name: 'Back to Ask NexusAI' })).toHaveCount(0)
  await page.getByRole('textbox', { name: 'Message', exact: true }).fill('generic compatibility question')
  await page.getByRole('button', { name: 'Send message' }).click()
  await expect.poll(() => sentMessages.length).toBe(1)
  expect(sentMessages[0]).toEqual({ message: 'generic compatibility question' })
  expect(caseRegistryRequests).toHaveLength(0)
})
