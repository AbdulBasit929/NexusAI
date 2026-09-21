import { test, expect } from './coverage-fixtures.js'

const caseId = 'workspace-alpha'
const evidenceId = 'evidence-cdr-1'

async function mockAnalystPortal(page) {
  await page.route('**/api/operations', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ operations: [] }) }))
  await page.route('**/api/features', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ records: true, agents: true }) }))
  await page.route('**/api/auth/status', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ authEnabled: false, staticApiKeyRequired: false, user: null }) }))
  await page.route('**/api/branding', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ instance_name: 'NexusAI' }) }))
  await page.route('**/api/v1/forensics/cases**', route => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith(`/${caseId}/manifest`)) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ case_id: caseId, collection_id: caseId, resources: [{ resource: 'canonical_records', count: 5004 }], record_families: [{ record_type: 'cdr', accepted_rows: 5004 }] }) })
    }
    if (url.pathname.endsWith(`/${caseId}/evidence/${evidenceId}`)) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ item: { evidence_id: evidenceId, case_id: caseId, source_file: 'calls.csv', detected_type: 'cdr', processing_status: 'completed', size_bytes: 1024, created_at: '2026-08-17T08:00:00Z', sha256: 'a'.repeat(64), raw_storage_ref: `forensic://source/${evidenceId}`, processing_route: 'structured_records', metadata: { structured_maturity: { schema_profile: { mapping_profile_id: 'cdr-source-mapping/v2', source_columns: [{ source_field: 'MSISDN', canonical_field: 'msisdn', mapping_state: 'canonical', semantic_type: 'string_identifier' }, { source_field: 'Provider Event Class', canonical_field: null, mapping_state: 'recognized_extension', semantic_type: 'source_text' }], recognized_extension_fields: ['Provider Event Class'], unmapped_fields: [] }, time_policy: { contract_version: 'forensics.time-policy/v1', source_timezone: 'Asia/Karachi', timezone_state: 'source_declared', date_order: 'DMY', date_order_state: 'source_declared' }, quality: { state: 'ready', counts: { accepted_records: 5004, rejected_records: 0, unmapped_fields: 0 }, capability_readiness: [{ id: 'record_inspection', label: 'Record inspection', status: 'available', reason: 'Canonical records are available.' }, { id: 'temporal_activity', label: 'Temporal activity', status: 'available', reason: 'Accepted timestamps are available.' }] } } } }, warnings: [], errors: [] }) })
    }
    if (url.pathname.endsWith(`/${caseId}/evidence`)) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [{ evidence_id: evidenceId, case_id: caseId, source_file: 'calls.csv', detected_type: 'cdr', modality: 'structured_records', processing_status: 'completed', accepted_rows: 5004, size_bytes: 1024, created_at: '2026-08-17T08:00:00Z' }], summary: { evidence_total: 1, completed: 1, processing: 0, queued: 0, failed: 0, status_counts: { completed: 1 } }, pagination: { limit: 12, offset: 0, has_next: false } }) })
    }
    if (url.pathname.endsWith(`/${caseId}`)) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ case_id: caseId, collection_id: caseId, display_name: 'Workspace Alpha', selectable: true }) })
    }
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ default_case_id: caseId, cases: [{ case_id: caseId, collection_id: caseId, display_name: 'Workspace Alpha', selectable: true }] }) })
  })
  await page.route('**/api/records/forensic/status**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ summary: { accepted_rows: 5004, evidence_completed: 1, evidence_in_flight: 0, evidence_failed: 0 }, record_families: [{ record_type: 'cdr', accepted_rows: 5004 }] }) }))
  await page.route('**/api/records/forensic/capabilities**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ summary: { families_total: 1, queryable: 1 }, families: [{ id: 'cdr', label: 'Call detail records', support_level: 'operational', availability: 'queryable', formats: ['csv', 'json', 'parquet'], indexed_records: 5004, record_types: ['cdr'], evidence_types: ['cdr'], deterministic_operations: ['exact frequent-contact ranking', 'exact duration summary'], semantic_operations: ['bounded communications-pattern explanation'], suggested_queries: ['show frequent contacts', 'show longest call duration'] }], query_corpus: { entries: [] } }) }))
  await page.route('**/api/agents/Forensic_Records_Analyst/config**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ name: 'Forensic_Records_Analyst', enable_forensic_records: true, forensic_tenant_id: 'default', forensic_collection_id: caseId, model: 'approved-explainer' }) }))
  await page.route('**/api/agents/Forensic_Records_Analyst/history**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [{ analysis_id: 'analysis-1', query: 'Who did this number contact most?', answer: 'A deterministic answer.', status: 'completed', execution_authority: 'deterministic_records', created_at: '2026-08-17T08:30:00Z', metadata: { presentation: { status: 'answered_with_limitations', executive_answer: 'Eight contacts were ranked from exact CDR events.', family_id: 'communications_cdr', operation_id: 'cdr.frequent_contacts', metrics: [{ label: 'Ranked contacts', value: 8 }], findings: [{ text: 'The exact ranking contains eight contacts.', claim_type: 'deterministic_fact' }], table: { title: 'Most frequent phone contacts', columns: ['target', 'event_count'], rows: [{ target: '923001110001', event_count: 8 }] }, citations: [{ label: 'calls.csv', detail: 'rows 1-5004' }], limitations: ['Frequency does not establish identity, ownership, or intent.'], proof_state: { citations: 'complete' } } } }], has_more: false }) }))
  await page.route('**/api/agents/Forensic_Records_Analyst/sse**', route => route.fulfill({ status: 200, contentType: 'text/event-stream', body: '' }))
}

async function seedAskResult(page, query, presentation) {
  await page.addInitScript(({ storageCase, storageQuery, storagePresentation }) => {
    const now = Date.now()
    const id = 'nx-ux1d-preview'
    localStorage.setItem(`localai_agent_chats_Forensic_Records_Analyst__case_${storageCase}`, JSON.stringify({
      contractVersion: 'browser-agent-history/v1',
      activeId: id,
      lastSaved: now,
      conversations: [{
        id,
        name: storageQuery,
        createdAt: now,
        updatedAt: now,
        messages: [
          { id: 'preview-question', sender: 'user', content: storageQuery, timestamp: now - 1000 },
          { id: 'preview-answer', sender: 'agent', content: 'Compatibility narrative.', timestamp: now, metadata: { presentation: { contract_version: 'forensics.agent-presentation/v1', ...storagePresentation } } },
        ],
      }],
    }))
  }, { storageCase: caseId, storageQuery: query, storagePresentation: presentation })
}

test('opens one canonical analyst workspace with evidence, Ask, and secondary actions', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.goto('/analyst/home')

  await expect(page).toHaveTitle('Investigation Workspace')
  await expect(page.getByRole('link', { name: 'Investigation Workspace home' })).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Current investigation' })).toHaveValue(caseId)
  await expect(page.getByRole('link', { name: 'Open evidence, 1 source' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Add data' })).toBeVisible()
  await expect(page.locator('.analyst-evidence-strip')).toHaveCount(0)
  await expect(page.getByText('Investigation context', { exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: 'Workspace menu' }).click()
  await expect(page.getByRole('menuitem', { name: 'History' })).toBeVisible()
  await expect(page.getByRole('navigation', { name: 'Investigation Workspace' })).toHaveCount(0)
  await expect(page.getByRole('heading', { name: 'Ask about your evidence' })).toBeVisible()
  await expect(page.getByRole('button', { name: /Show frequent contacts/ })).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Analyst query' })).toBeVisible()
  await expect(page.getByText(/model|agent|operation id|processor id/i)).toHaveCount(0)
  await page.getByRole('link', { name: 'Open evidence, 1 source' }).click()
  const evidenceDrawer = page.locator('.analyst-unified-drawer')
  await expect(evidenceDrawer).toBeVisible()
  expect(await evidenceDrawer.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true)
})

test('starts a new browser-local conversation without deleting governed Activity', async ({ page }) => {
  await mockAnalystPortal(page)
  await seedAskResult(page, 'Find LEH5003.', {
    status: 'answered', result_state: 'results_present', executive_answer: 'One sighting was found.',
    operation_id: 'anpr.sightings', table: { rows: [] }, findings: [], citations: [], limitations: [],
  })
  const historyMutations = []
  page.on('request', request => {
    if (request.url().includes('/history') && request.method() !== 'GET') historyMutations.push(request.method())
  })
  await page.goto(`/analyst?case=${caseId}`)
  await expect(page.getByText('Find LEH5003.', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Workspace menu' }).click()
  await page.getByRole('menuitem', { name: 'New conversation' }).click()
  await expect(page.getByRole('heading', { name: 'Ask about your evidence' })).toBeVisible()
  await expect(page.getByText('Find LEH5003.', { exact: true })).toHaveCount(0)
  await expect(page.getByRole('textbox', { name: 'Analyst query' })).toBeFocused()
  await expect.poll(() => page.evaluate(storageCase => JSON.parse(localStorage.getItem(`localai_agent_chats_Forensic_Records_Analyst__case_${storageCase}`)).conversations.length, caseId)).toBe(2)
  await page.getByRole('button', { name: 'Workspace menu' }).click()
  await page.getByRole('menuitem', { name: 'History' }).click()
  await expect(page.getByRole('dialog', { name: 'History' }).getByText('Who did this number contact most?')).toBeVisible()
  expect(historyMutations).toEqual([])
})

test('traps keyboard focus in the unified drawer and restores its trigger on Escape', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.goto(`/analyst?case=${caseId}`)

  const trigger = page.getByRole('link', { name: 'Open evidence, 1 source' })
  await trigger.click()
  const drawer = page.getByRole('dialog', { name: 'Evidence' })
  await expect(drawer).toBeVisible()
  const focusable = drawer.locator('button:not(:disabled):visible, a[href]:visible, input:not(:disabled):visible, select:not(:disabled):visible, textarea:not(:disabled):visible')
  const first = focusable.first()
  const last = focusable.last()
  await first.focus()
  await page.keyboard.press('Shift+Tab')
  await expect(last).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(first).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(drawer).toBeHidden()
  await expect(trigger).toBeFocused()
})

test('offers only authoritative retained entity choices for target clarification', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.unroute('**/api/agents/Forensic_Records_Analyst/history**')
  await page.route('**/api/agents/Forensic_Records_Analyst/history**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [{
    analysis_id: 'entity-activity', query: 'show available entities', status: 'completed', created_at: '2026-09-07T07:31:54Z', metadata: { presentation: {
      contract_version: 'forensics.agent-presentation/v1', operation_id: 'forensics.entity_activity', executive_answer: '20 results were found.',
      table: { rows: [{ entity_type: 'phone', entity_value: '923001110001' }, { entity_type: 'location', entity_value: 'Gulberg' }, { entity_type: 'phone', entity_value: '923331234567' }] },
    } },
  }], has_more: false }) }))
  await seedAskResult(page, 'show frequent contacts', {
    status: 'needs_input', result_state: 'invalid_request', clarification: 'Which number or subscriber should I analyze for frequent contacts?',
    table: { rows: [] }, findings: [], citations: [], limitations: [], next_actions: [{ label: 'Show available entities', query: 'show available entities' }],
  })
  await page.goto(`/analyst?case=${caseId}`)
  const choices = page.getByRole('region', { name: 'Available entity choices' })
  await expect(choices.getByRole('button', { name: /923001110001/ })).toBeVisible()
  await expect(choices.getByRole('button', { name: /923331234567/ })).toBeVisible()
  await expect(choices.getByText('Gulberg')).toHaveCount(0)
  await expect(page.getByText('Which number or subscriber should I analyze for frequent contacts?')).toBeVisible()
})

test('uses one spacious Add evidence empty state and the real intake dialog', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.route(`**/api/v1/forensics/cases/${caseId}/evidence**`, route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [], summary: { evidence_total: 0, completed: 0, processing: 0, queued: 0, failed: 0, status_counts: {} }, pagination: { has_next: false } }) }))
  await page.route('**/api/records/forensic/status**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ summary: { evidence_completed: 0, evidence_in_flight: 0, evidence_failed: 0 }, record_families: [] }) }))
  await page.goto(`/analyst?case=${caseId}`)

  await expect(page.getByRole('heading', { name: 'Add evidence to begin' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Ask about your evidence' })).toHaveCount(0)
  await expect(page.getByText(/Upload records, documents, images, audio or video/)).toBeVisible()
  await page.screenshot({ path: 'reports/nx-ux1g-final-acceptance/final-empty-state-1440.png', fullPage: true })
  await page.getByRole('button', { name: 'Add data' }).last().click()
  const intake = page.getByRole('dialog', { name: 'Add data' })
  await expect(intake.getByText('Drop files here')).toBeVisible()
  await expect(intake.getByText(/server—not the filename or browser MIME type/i)).toBeVisible()
  await intake.getByRole('button', { name: 'Close Add Data' }).click()
  await expect(page).toHaveURL(new RegExp(`/analyst\\?case=${caseId}$`))
})

test('opens a human-readable source from the simple Data surface', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.goto(`/analyst/data?case=${caseId}`)

  await expect(page.getByRole('dialog', { name: 'Evidence' }).getByRole('heading', { name: 'Evidence', exact: true }).first()).toBeVisible()
  await expect(page.getByRole('link', { name: 'Add data' })).toBeVisible()
  await expect(page.getByRole('button', { name: /Processing/ })).toHaveCount(0)
  await expect(page.getByRole('button', { name: /Needs attention/ })).toHaveCount(0)
  await expect(page.getByRole('button', { name: /Failed/ })).toHaveCount(0)
  await page.getByRole('textbox', { name: 'Search sources' }).fill('calls')
  const evidencePanel = page.getByRole('dialog', { name: 'Evidence' })
  await expect(evidencePanel.getByRole('button', { name: /Calls\.csv/ })).toBeVisible()
  await evidencePanel.getByRole('button', { name: /Calls\.csv/ }).click()
  await expect(page).toHaveURL(new RegExp(`evidence=${evidenceId}`))
  const drawer = page.getByRole('dialog', { name: 'Calls.csv' })
  await expect(drawer.getByText('Ready', { exact: true })).toBeVisible()
  await expect(drawer.getByText('5,004 verified records')).toBeVisible()
  await expect(drawer.getByRole('heading', { name: 'Field mapping' })).toBeHidden()
  await expect(drawer.getByText('SHA-256')).toBeHidden()
  await drawer.getByText('Details', { exact: true }).click()
  await expect(drawer.getByText('calls.csv', { exact: true })).toBeVisible()
  await expect(drawer.getByText('exact frequent-contact ranking')).toBeVisible()
  await expect(drawer.getByRole('heading', { name: 'Quality' })).toBeVisible()
  await expect(drawer.getByRole('heading', { name: 'Field mapping' })).toBeVisible()
  await expect(drawer.getByRole('columnheader')).toHaveCount(5)
  await expect(drawer.getByText('Mapped', { exact: true })).toBeVisible()
  await expect(drawer.getByText('Preserved', { exact: true })).toBeVisible()
  await expect(drawer.getByText('Provider Event Class', { exact: true }).first()).toBeVisible()
  await expect(drawer.getByText(/Asia\/Karachi · DMY/)).toBeVisible()
  await expect(drawer.getByText('Temporal activity')).toBeVisible()
  await expect(drawer.getByRole('link', { name: /show frequent contacts/i })).toHaveAttribute('href', /analyst\?evidence=evidence-cdr-1&prompt=/)
  await expect(drawer.getByText('SHA-256')).toBeHidden()
  await drawer.getByText('Technical details', { exact: true }).click()
  await expect(drawer.getByText('SHA-256')).toBeVisible()
  await page.setViewportSize({ width: 390, height: 820 })
  const drawerBounds = await drawer.evaluate(element => ({ width: element.clientWidth, content: element.scrollWidth }))
  expect(drawerBounds.content).toBeLessThanOrEqual(drawerBounds.width)
})

test('presents MIME-derived audio identity and artifact-derived transcript readiness', async ({ page }) => {
  await mockAnalystPortal(page)
  const audioID = 'audio-urdu-1'
  const audioItem = {
    evidence_id: audioID,
    case_id: caseId,
    original_filename: 'recording.bin',
    modality: '',
    detected_type: '',
    mime_type: 'audio/wav',
    processing_status: 'completed',
    created_at: '2026-08-24T08:00:00Z',
  }
  const artifacts = [
    {
      artifact_id: 'audio-technical-1',
      artifact_type: 'forensics.audio-observation/v1',
      processing_status: 'completed',
      metadata: { observation: { container_probe: { format: { duration: '6.24', format_name: 'wav' }, streams: [{ codec_type: 'audio', codec_name: 'pcm_s16le' }] } } },
    },
    {
      artifact_id: 'audio-transcript-1',
      artifact_type: 'forensics.audio-timestamp-segment/v1',
      processing_status: 'completed',
      metadata: { observation: { text: 'آڈیو متن', start_seconds: 0, end_seconds: 4.42, requested_language: 'ur', processor: 'faster-whisper-small' } },
    },
    {
      artifact_id: 'audio-roman-1',
      artifact_type: 'forensics.audio-roman-urdu-segment/v1',
      processing_status: 'completed',
      metadata: { observation: { raw_urdu_text: 'آڈیو متن', roman_urdu_text: 'audio matn', start_seconds: 0, end_seconds: 4.42 } },
    },
  ]
  await page.route('**/api/v1/forensics/cases**', route => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith(`/${caseId}/evidence/${audioID}/content`)) {
      return route.fulfill({ status: 200, contentType: 'audio/wav', body: '' })
    }
    if (url.pathname.endsWith(`/${caseId}/evidence/${audioID}`)) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ item: audioItem, derived_artifacts: artifacts, ingest_jobs: [{ status: 'completed' }], warnings: [], errors: [] }) })
    }
    if (url.pathname.endsWith(`/${caseId}/evidence`)) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [audioItem], summary: { evidence_total: 1, completed: 1, processing: 0, queued: 0, failed: 0, status_counts: { completed: 1 } }, pagination: { limit: 12, offset: 0, has_next: false } }) })
    }
    return route.fallback()
  })

  await page.goto(`/analyst/data?case=${caseId}`)
  await expect(page.getByRole('button', { name: /Recording\.bin/ })).toContainText('Audio')
  await page.getByRole('button', { name: /Recording\.bin/ }).click()
  const drawer = page.getByRole('dialog', { name: 'Recording.bin' })
  await expect(drawer.getByRole('heading', { name: 'Listen to recording' })).toBeVisible()
  await expect(drawer.getByText('0:06.2', { exact: true })).toBeVisible()
  await expect(drawer.getByText('ur', { exact: true })).toBeVisible()
  await expect(drawer.getByText('Transcript available — analyst review required')).toBeVisible()
  await expect(drawer.getByText('1 segments · 1 Roman Urdu')).toBeVisible()
  await expect(drawer.locator('audio[controls]')).toHaveCount(1)
  await expect(drawer.getByText('pcm_s16le', { exact: true })).toBeHidden()
  await drawer.getByText('Audio details', { exact: true }).click()
  await expect(drawer.getByText('pcm_s16le', { exact: true })).toBeVisible()
  await expect(drawer.getByText('faster-whisper-small')).toBeVisible()
})

test('presents retained ANPR, face, OCR, zero-result, and bounded candidate intelligence in Data', async ({ page }) => {
  await mockAnalystPortal(page)
  const plateID = 'image-plate'
  const faceID = 'image-face-a'
  const candidateID = 'image-face-pose'
  const noFaceID = 'image-no-face'
  const sources = [
    { evidence_id: plateID, original_filename: 'test_plate.jpg', modality: 'image', detected_type: 'image', processing_status: 'completed', media_metadata: { width_pixels: 913, height_pixels: 833 }, created_at: '2026-08-23T08:00:00Z' },
    { evidence_id: faceID, original_filename: 'subject-a-frontal.png', modality: 'image', detected_type: 'image', processing_status: 'completed', media_metadata: { width_pixels: 1254, height_pixels: 1254 }, created_at: '2026-08-23T08:00:00Z' },
    { evidence_id: candidateID, original_filename: 'subject-a-pose.png', modality: 'image', detected_type: 'image', processing_status: 'completed', media_metadata: { width_pixels: 1254, height_pixels: 1254 }, created_at: '2026-08-23T08:00:00Z' },
    { evidence_id: noFaceID, original_filename: 'no-face-street.png', modality: 'image', detected_type: 'image', processing_status: 'completed', media_metadata: { width_pixels: 960, height_pixels: 768 }, created_at: '2026-08-23T08:00:00Z' },
  ]
  const artifact = (id, type, observation, locator = {}) => ({ artifact_id: id, artifact_type: type, processing_status: 'completed', confidence: observation.ocr_confidence ?? observation.detection_confidence ?? observation.confidence, metadata: { observation_id: id, observation }, citation_locator: locator, citation_ref: `nexusai://evidence/${plateID}/artifacts/${id}` })
  const details = {
    [plateID]: {
      item: sources[0], ingest_jobs: [{ status: 'completed' }], warnings: [], errors: [],
      derived_artifacts: [
        artifact('plate-1', 'forensics.anpr-observation/v1', { normalized_plate_text: 'MN1367', raw_plate_text: 'MN1367', detection_confidence: 0.8897411, ocr_confidence: 0.9997855, manual_review_required: true, bbox: { x: 396, y: 568, width: 157, height: 60 } }, { bbox: { x: 396, y: 568, width: 157, height: 60 } }),
        artifact('ocr-1', 'forensics.image-ocr-observation/v1', { raw_text: 'Islamabad', confidence: 0.82, script_family: 'latin_script_candidate', bbox: { x: 30, y: 50, width: 120, height: 25 } }, { bbox: { x: 30, y: 50, width: 120, height: 25 } }),
      ],
    },
    [faceID]: {
      item: sources[1], ingest_jobs: [{ status: 'completed' }], warnings: [], errors: [],
      derived_artifacts: [
        artifact('face-1', 'forensics.face-observation/v1', { detection_confidence: 0.9517, embedding_status: 'available', embedding_dimension: 2, embedding: [0.1, 0.2], bbox: { x: 355, y: 296, width: 535, height: 694 } }, { bbox: { x: 355, y: 296, width: 535, height: 694 } }),
        artifact('embedding-1', 'forensics.image-embedding-observation/v1', { embedding_dimension: 2, embedding: [0.2, 0.3] }),
      ],
    },
    [candidateID]: { item: sources[2], ingest_jobs: [{ status: 'completed' }], warnings: [], errors: [], derived_artifacts: [] },
    [noFaceID]: { item: sources[3], ingest_jobs: [{ status: 'completed' }], warnings: [], errors: [], derived_artifacts: [] },
  }
  await page.route(`**/api/v1/forensics/cases/${caseId}/faces/similar**`, route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ distance_metric: 'cosine_similarity', results: [{ rank: 1, candidate_evidence_id: candidateID, candidate_face_observation_id: 'face-pose', similarity_score: 0.8419142, review_state: 'model_candidate', citation_ref: `nexusai://evidence/${candidateID}/artifacts/face-pose` }] }) }))
  await page.route(`**/api/v1/forensics/cases/${caseId}/images/similar**`, route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ distance_metric: 'cosine_similarity', results: [{ rank: 1, candidate_evidence_id: candidateID, similarity_score: 0.9668381, review_state: 'model_candidate', citation_ref: `nexusai://evidence/${candidateID}/artifacts/embedding-pose` }] }) }))
  await page.route(`**/api/v1/forensics/cases/${caseId}/evidence**`, route => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith('/evidence/compare')) return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ exact_duplicate: false, perceptual: { similarity: 0.78125, near_duplicate_candidate: false }, visual_similarity: { score: 0.9668381 }, limitations: ['Similarity is not identity.'] }) })
    const detailID = url.pathname.split('/').pop()
    if (details[detailID]) return route.fulfill({ contentType: 'application/json', body: JSON.stringify(details[detailID]) })
    if (url.pathname.endsWith('/evidence')) return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: sources, summary: { evidence_total: 4, completed: 4, status_counts: { completed: 4 } }, pagination: { has_next: false } }) })
    return route.fallback()
  })
  await page.goto(`/analyst/data?case=${caseId}`)

  await page.getByRole('button', { name: /Test plate.jpg/ }).click()
  let drawer = page.getByRole('dialog', { name: 'Test plate.jpg' })
  await expect(drawer.getByRole('toolbar', { name: 'Image view controls' })).toBeVisible()
  await drawer.getByRole('button', { name: 'Zoom in' }).click()
  await expect(drawer.getByRole('toolbar', { name: 'Image view controls' }).getByText('125%')).toBeVisible()
  await drawer.getByRole('button', { name: 'Fit' }).click()
  await expect(drawer.getByRole('toolbar', { name: 'Image view controls' }).getByText('100%')).toBeVisible()
  await expect(drawer.getByRole('heading', { name: /Detected plates/ })).toBeVisible()
  await expect(drawer.locator('.analyst-plate-text').getByText('MN1367', { exact: true })).toBeVisible()
  await expect(drawer.getByText('89% confidence')).toBeVisible()
  await expect(drawer.getByText('100% confidence')).toBeVisible()
  await expect(drawer.getByRole('button', { name: 'Focus MN1367' })).toBeVisible()
  await expect(drawer.getByText(/Dynamically reconstructed from authorized source pixels/)).toBeVisible()
  await expect(drawer.getByRole('link', { name: /Find MN1367/ })).toHaveAttribute('href', /analyst\?case=workspace-alpha&evidence=image-plate&prompt=/)
  await drawer.getByRole('button', { name: 'Close source details' }).click()

  await page.getByRole('button', { name: /Subject a frontal.png/ }).click()
  drawer = page.getByRole('dialog', { name: 'Subject a frontal.png' })
  await expect(drawer.getByRole('heading', { name: /Detected faces/ })).toBeVisible()
  await drawer.getByRole('button', { name: /Rank face candidates/ }).click()
  await expect(drawer.locator('.analyst-ranked-results').getByText('Subject a pose.png')).toBeVisible()
  await expect(drawer.getByText(/0\.8419/)).toBeVisible()
  await expect(drawer.locator('.analyst-ranked-results').getByText(/not identity/i)).toBeVisible()
  await drawer.getByRole('button', { name: 'Close source details' }).click()

  await page.getByRole('button', { name: /No face street.png/ }).click()
  drawer = page.getByRole('dialog', { name: 'No face street.png' })
  await expect(drawer.getByText('Processing complete — zero derived observations')).toBeVisible()
  await expect(drawer.getByRole('heading', { name: /Detected faces/ })).toHaveCount(0)
  for (const width of [390, 820, 1024, 1440]) {
    await page.setViewportSize({ width, height: 900 })
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
    expect(overflow).toBeLessThanOrEqual(0)
  }
})

test('keeps accepted-row truth bound to evidence identity and distinguishes zero from unavailable', async ({ page }) => {
  await mockAnalystPortal(page)
  const sources = [
    { evidence_id: 'source-a', source_file: 'source-a.csv', detected_type: 'cdr', processing_status: 'completed', accepted_rows: 4, total_rows: 5, metadata: { structured_summary: { duplicate_rows: 1, rejected_rows: 0 } }, created_at: '2026-08-17T08:00:00Z' },
    { evidence_id: 'source-b', source_file: 'source-b.csv', detected_type: 'cdr', processing_status: 'completed', accepted_rows: 0, total_rows: 0, created_at: '2026-08-17T08:00:00Z' },
    { evidence_id: 'source-unknown', source_file: 'source-unknown.csv', detected_type: 'cdr', processing_status: 'completed', created_at: '2026-08-17T08:00:00Z' },
  ]
  await page.route(`**/api/v1/forensics/cases/${caseId}/evidence**`, route => {
    const url = new URL(route.request().url())
    const id = url.pathname.split('/').pop()
    if (id !== 'evidence') return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ item: { evidence_id: id, source_file: `${id}.csv`, detected_type: 'cdr', processing_status: 'completed' }, warnings: [], errors: [] }) })
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: sources, summary: { evidence_total: 3, completed: 3, status_counts: { completed: 3 } }, pagination: { has_next: false } }) })
  })
  await page.goto(`/analyst/data?case=${caseId}`)

  await page.getByRole('textbox', { name: 'Search sources' }).fill('source-a')
  await page.getByRole('button', { name: /Source a\.csv/ }).click()
  let drawer = page.getByRole('dialog', { name: 'Source a.csv' })
  await expect(drawer.getByText('4 verified records')).toBeVisible()
  await drawer.getByText('Details', { exact: true }).click()
  await expect(drawer.getByText('5', { exact: true })).toBeVisible()
  await expect(drawer.locator('.analyst-detail-section').filter({ hasText: 'Processing summary' }).getByText('1', { exact: true })).toBeVisible()
  await drawer.getByRole('button', { name: 'Close source details' }).click()

  await page.getByRole('textbox', { name: 'Search sources' }).fill('source-b')
  await page.getByRole('button', { name: /Source b\.csv/ }).click()
  drawer = page.getByRole('dialog', { name: 'Source b.csv' })
  await expect(drawer.getByText('0 verified records')).toBeVisible()
  await drawer.getByRole('button', { name: 'Close source details' }).click()

  await page.getByRole('textbox', { name: 'Search sources' }).fill('source-unknown')
  await page.getByRole('button', { name: /Source unknown\.csv/ }).click()
  drawer = page.getByRole('dialog', { name: 'Source unknown.csv' })
  await expect(drawer.getByText(/verified record count is not available/i)).toBeVisible()
  await expect(drawer.getByText('0 verified records')).toHaveCount(0)
})

test('supports governed multi-file intake with per-file duplicate and failure outcomes', async ({ page }) => {
  await mockAnalystPortal(page)
  let registrations = 0
  const intakePayloads = []
  await page.route(`**/api/agents/collections/${caseId}/upload**`, route => {
    registrations += 1
    intakePayloads.push(route.request().postData() || '')
    if (registrations === 1) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ evidence_status: 'duplicate', evidence_registration: { status: 'duplicate', evidence_id: evidenceId, message: 'The same content already exists in this workspace.' } }) })
    }
    return route.fulfill({ status: 422, contentType: 'application/json', body: JSON.stringify({ error: { message: 'The server could not classify this source safely.' } }) })
  })
  await page.goto(`/analyst/data?case=${caseId}&add=true`)

  const dialog = page.getByRole('dialog', { name: 'Add data' })
  await expect(dialog).toBeVisible()
  await dialog.locator('input[type="file"]').setInputFiles([
    { name: 'calls-copy.csv', mimeType: 'text/csv', buffer: Buffer.from('a,b\n1,2') },
    { name: 'unknown.bin', mimeType: 'application/octet-stream', buffer: Buffer.from('not-classifiable') },
  ])
  await expect(dialog.getByText('calls-copy.csv')).toBeVisible()
  await expect(dialog.getByText('unknown.bin')).toBeVisible()
  await dialog.getByRole('button', { name: 'Add selected data' }).click()
  await expect(dialog.getByText('Already added')).toBeVisible()
  await expect(dialog.getByText('The server could not classify this source safely.')).toBeVisible()
  const chooseMore = dialog.getByRole('button', { name: 'Choose more files' })
  await expect(chooseMore).toBeVisible()
  await chooseMore.focus()
  await chooseMore.press('Tab')
  await expect(dialog.getByRole('button', { name: 'Close Add Data' })).toBeFocused()
  expect(registrations).toBe(2)
  expect(intakePayloads.every(payload => payload.includes('source_timezone_state') && payload.includes('unknown'))).toBeTruthy()
  expect(intakePayloads.every(payload => payload.includes('source_date_order_state') && payload.includes('unresolved'))).toBeTruthy()
  expect(intakePayloads.every(payload => !payload.includes('Asia/Karachi'))).toBeTruthy()
})

test('upload correction explains waiting and closing discards an unsubmitted selection', async ({ page }) => {
  await mockAnalystPortal(page)
  let registrations = 0
  await page.route(`**/api/agents/collections/${caseId}/upload**`, route => {
    registrations += 1
    return route.abort()
  })
  await page.goto(`/analyst/data?case=${caseId}&add=true`)
  const dialog = page.getByRole('dialog', { name: 'Add data' })
  await dialog.locator('input[type="file"]').setInputFiles({ name: 'video.mp4', mimeType: 'video/mp4', buffer: Buffer.from('synthetic') })
  await expect(dialog.getByText(/Server-declared formats: CSV, JSON, PARQUET/)).toBeVisible()
  await expect(dialog.getByText(/Waiting means selected, not uploaded/)).toBeVisible()
  await expect(dialog.getByText('Waiting', { exact: true })).toBeVisible()
  await dialog.getByRole('button', { name: 'Close Add Data' }).click()
  await expect(dialog).toBeHidden()
  await page.getByRole('dialog', { name: 'Evidence' }).getByRole('button', { name: 'Add data' }).click()
  await expect(dialog.getByText('video.mp4', { exact: true })).toHaveCount(0)
  expect(registrations).toBe(0)
})

test('upload correction distinguishes HTTP 413 from an uncertain connection failure', async ({ page }) => {
  await mockAnalystPortal(page)
  let registrations = 0
  await page.route(`**/api/agents/collections/${caseId}/upload**`, route => {
    registrations += 1
    return registrations === 1
      ? route.fulfill({ status: 413, contentType: 'application/json', body: '{}' })
      : route.abort('connectionreset')
  })
  await page.goto(`/analyst/data?case=${caseId}&add=true`)
  const dialog = page.getByRole('dialog', { name: 'Add data' })
  await dialog.locator('input[type="file"]').setInputFiles([
    { name: 'size.mp4', mimeType: 'video/mp4', buffer: Buffer.from('synthetic-size') },
    { name: 'connection.mp4', mimeType: 'video/mp4', buffer: Buffer.from('synthetic-connection') },
  ])
  await dialog.getByRole('button', { name: 'Add selected data' }).click()
  await expect(dialog.getByText(/too large \(HTTP 413\)/)).toBeVisible()
  await expect(dialog.getByText(/Check Data before retrying; registration may be uncertain/)).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Close Add Data' })).toBeEnabled()
  expect(registrations).toBe(2)
})

test('upload correction prevents closing during registration and allows closing after acceptance', async ({ page }) => {
  await mockAnalystPortal(page)
  let releaseRegistration
  const registrationGate = new Promise(resolve => { releaseRegistration = resolve })
  await page.route(`**/api/agents/collections/${caseId}/upload**`, async route => {
    await registrationGate
    await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ evidence_status: 'queued', evidence_registration: { status: 'queued', evidence_id: 'synthetic-upload' } }) })
  })
  await page.route(`**/api/v1/forensics/cases/${caseId}/evidence/synthetic-upload**`, route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ item: { processing_status: 'queued' } }) }))
  await page.goto(`/analyst/data?case=${caseId}&add=true`)
  const dialog = page.getByRole('dialog', { name: 'Add data' })
  await dialog.locator('input[type="file"]').setInputFiles({ name: 'video.mp4', mimeType: 'video/mp4', buffer: Buffer.from('synthetic') })
  await dialog.getByRole('button', { name: 'Add selected data' }).click()
  await expect(dialog.getByRole('button', { name: 'Close Add Data' })).toBeDisabled()
  await expect(dialog.getByText(/Keep this page open during upload and registration/)).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(dialog).toBeVisible()
  releaseRegistration()
  await expect(dialog.getByText('Processing', { exact: true })).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Close Add Data' })).toBeEnabled()
  await dialog.getByRole('button', { name: 'Close Add Data' }).click()
  await expect(dialog).toBeHidden()
})

test('maps authoritative readiness states and keeps failed separate from attention', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.route(`**/api/v1/forensics/cases/${caseId}/evidence**`, route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
    items: [
      { evidence_id: 'ready-1', source_file: 'ready.csv', detected_type: 'cdr', processing_status: 'completed', created_at: '2026-08-17T08:00:00Z' },
      { evidence_id: 'processing-1', source_file: 'processing.csv', detected_type: 'cdr', processing_status: 'running', created_at: '2026-08-17T08:00:00Z' },
      { evidence_id: 'attention-1', source_file: 'attention.jpg', detected_type: 'image', processing_status: 'registered', processing_route: 'image_ocr_vision_pending', created_at: '2026-08-17T08:00:00Z' },
      { evidence_id: 'failed-1', source_file: 'failed.csv', detected_type: 'cdr', processing_status: 'dead_letter', created_at: '2026-08-17T08:00:00Z' },
    ],
    summary: { evidence_total: 4, completed: 1, processing: 1, queued: 1, failed: 1, status_counts: { completed: 1, running: 1, registered: 1, dead_letter: 1 } },
    pagination: { has_next: false },
  }) }))
  await page.goto(`/analyst/data?case=${caseId}`)

  await expect(page.getByRole('button', { name: /Needs attention 1/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /Failed 1/ })).toBeVisible()
  await page.getByRole('button', { name: /Needs attention 1/ }).click()
  await expect(page.getByRole('button', { name: /Attention\.jpg/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /Failed\.csv/ })).toHaveCount(0)
  await page.getByRole('button', { name: /Failed 1/ }).click()
  await expect(page.getByRole('button', { name: /Failed\.csv/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /Attention\.jpg/ })).toHaveCount(0)
})

test('loads subsequent source pages from the server and explains approval-gated recovery', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.route(`**/api/v1/forensics/cases/${caseId}/evidence**`, route => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith('/evidence/failed-13/reprocess-plan')) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ eligibility: { eligible: true, reason: 'The latest job is terminal; an approved immutable reprocess would create a new linked generation.' }, approval: { required: true, state: 'not_granted', execution_permitted: false } }) })
    }
    if (url.pathname.endsWith('/evidence/failed-13')) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ item: { evidence_id: 'failed-13', source_file: 'failed-13.csv', detected_type: 'cdr', processing_status: 'dead_letter', created_at: '2026-08-17T08:00:00Z', errors: [{ message: 'Parser exhausted governed attempts.' }] }, warnings: [], errors: [{ message: 'Parser exhausted governed attempts.' }] }) })
    }
    const offset = Number(url.searchParams.get('offset') || 0)
    const pageItems = offset === 0
      ? Array.from({ length: 12 }, (_, index) => ({ evidence_id: `ready-${index + 1}`, source_file: `ready-${index + 1}.csv`, detected_type: 'cdr', processing_status: 'completed', created_at: '2026-08-17T08:00:00Z' }))
      : [{ evidence_id: 'failed-13', source_file: 'failed-13.csv', detected_type: 'cdr', processing_status: 'dead_letter', accepted_rows: 0, created_at: '2026-08-17T08:00:00Z' }]
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: pageItems, summary: { evidence_total: 13, completed: 12, failed: 1, status_counts: { completed: 12, dead_letter: 1 } }, pagination: { limit: 12, offset, has_next: offset === 0 } }) })
  })
  await page.goto(`/analyst/data?case=${caseId}`)

  await page.getByRole('button', { name: 'Load more sources' }).click()
  await page.getByRole('button', { name: /Failed 13\.csv/ }).click()
  const drawer = page.getByRole('dialog', { name: 'Failed 13.csv' })
  await expect(drawer.locator('#source-overview').getByText('Failed', { exact: true })).toBeVisible()
  await drawer.getByText('Details', { exact: true }).click()
  await expect(drawer.locator('.analyst-detail-section').filter({ hasText: 'Processing summary' }).getByText('0', { exact: true })).toBeVisible()
  await expect(drawer.getByText('The original source remains preserved. Failed processing is not silently retried or overwritten.')).toBeVisible()
  await expect(drawer.getByText(/Operator approval is required/)).toBeVisible()
  await expect(drawer.getByRole('button', { name: /retry/i })).toHaveCount(0)
})

test('keeps Ask automatic and hides implementation selectors', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto(`/analyst/ask?case=${caseId}`)

  await expect(page.getByRole('heading', { name: 'Ask about your evidence' })).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Current investigation' })).toHaveValue(caseId)
  await expect(page.getByRole('combobox', { name: 'Active forensic case' })).toHaveCount(0)
  await expect(page.getByText('Model role:')).not.toBeVisible()
  await expect(page.getByText('Validated en query')).toHaveCount(0)
  await expect(page.getByRole('button', { name: /Status/ })).toHaveCount(0)
  await expect(page.getByRole('button', { name: /Show frequent contacts/ })).toBeVisible()
  await page.getByRole('textbox', { name: 'Analyst query' }).fill('Who did 923001110001 contact most?')
  await expect(page.getByRole('button', { name: 'Ask question' })).toBeEnabled()
  await page.screenshot({ path: 'reports/nx-ux1d-visual-evidence/ask-initial-1440.png', fullPage: true })
})

test('preserves Data evidence identity as a human Ask scope', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.setViewportSize({ width: 1024, height: 900 })
  await page.goto(`/analyst/data?case=${caseId}&evidence=${evidenceId}`)
  const drawer = page.getByRole('dialog', { name: 'Calls.csv' })
  const askLink = drawer.getByRole('link', { name: /show frequent contacts/i })
  await expect(askLink).toHaveAttribute('href', new RegExp(`evidence=${evidenceId}`))
  await askLink.click()

  await expect(page).toHaveURL(new RegExp(`evidence=${evidenceId}`))
  await expect(page.getByText('Investigation context')).toHaveCount(0)
  await expect(page.locator('main').getByText('Workspace Alpha', { exact: true })).toHaveCount(0)
  await expect(page.getByRole('link', { name: 'Open evidence, 1 source' })).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Analyst query' })).toHaveValue(/Use calls\.csv as the focal source/)
  await page.screenshot({ path: 'reports/nx-ux1d-visual-evidence/ask-evidence-scope-1024.png', fullPage: true })
})

test('presents a one-result ANPR answer as compact facts with provenance on demand', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.setViewportSize({ width: 1440, height: 1100 })
  await seedAskResult(page, 'Show plate observations from 10 to 30 seconds.', {
    status: 'answered_with_limitations',
    result_state: 'results_present',
    executive_answer: 'Retrieved exact ANPR observations with canonical evidence, version, row, and hash provenance.',
    execution_plan: { parameters: { start_seconds: 10, end_seconds: 30 } },
    metrics: [{ label: 'Target', value: 'LEH5003' }],
    findings: [
      { text: '1 sighting of LEH5003 was found.', claim_type: 'deterministic_fact' },
      { text: 'Planner Confidence: 0.9', claim_type: 'metric' },
    ],
    table: { title: 'Plate observations', columns: ['observed_at', 'camera_id', 'location', 'normalized_target'], rows: [{ observed_at: '2026-09-07T04:44:55Z', camera_id: '', location: '', normalized_target: 'LEH-5003' }] },
    citations: [{ label: 'calls.csv', evidence_id: evidenceId, citation_locator: { row_number: 42 } }],
    limitations: ['A plate observation does not establish ownership.'],
    next_actions: [{ query: 'Show the surrounding source timeline.', text: 'Show surrounding timeline' }],
    operation_id: 'anpr.sightings',
    trace: { backend: 'internal-value' },
  })
  await page.goto(`/analyst/ask?case=${caseId}`)

  const submittedQuestion = page.locator('.chat-message-user .chat-message-content')
  await expect(submittedQuestion).toHaveCSS('background-color', 'rgb(238, 244, 255)')
  await expect(submittedQuestion).toHaveCSS('border-top-width', '1px')
  await expect(submittedQuestion).toHaveCSS('border-top-left-radius', '16px')
  const submittedGeometry = await submittedQuestion.evaluate(element => ({ width: element.getBoundingClientRect().width, lane: element.closest('.chat-messages').getBoundingClientRect().width }))
  expect(submittedGeometry.width).toBeLessThan(submittedGeometry.lane * 0.8)
  const answerSurface = page.locator('.chat-message-assistant .chat-message-bubble').first()
  await expect(answerSurface).toHaveCSS('background-color', 'rgb(255, 255, 255)')
  await expect(answerSurface).toHaveCSS('border-top-left-radius', '18px')
  await expect(page.getByRole('heading', { name: 'Answer', exact: true })).toBeVisible()
  await expect(page.getByText('1 sighting of LEH5003 was found.')).toBeVisible()
  await expect(page.getByText('7 Sept 2026 · 04:44:55 UTC')).toBeVisible()
  await expect(page.getByText('Not available', { exact: true })).toHaveCount(2)
  await expect(page.getByText('LEH-5003')).toBeHidden()
  await expect(page.getByText('Planner Confidence: 0.9')).toBeHidden()
  await expect(page.getByRole('button', { name: 'Open evidence source calls.csv' })).toBeVisible()
  await expect(page.getByText('row 42')).toBeVisible()
  await expect(page.getByText('A plate observation does not establish ownership.')).toBeHidden()
  await expect(page.getByRole('table')).toBeHidden()
  await expect(page.getByText('Technical details', { exact: true })).toBeHidden()
  await page.screenshot({ path: 'reports/nx-ux1g-final-acceptance/ask-governed-result-1440.png', fullPage: true })
  await page.setViewportSize({ width: 390, height: 844 })
  await expect(page.getByRole('textbox', { name: 'Analyst query' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390)
  await page.screenshot({ path: 'reports/nx-ux1g-final-acceptance/ask-governed-result-390.png', fullPage: true })
  await page.setViewportSize({ width: 1440, height: 1100 })
  await page.getByText('More details').click()
  await expect(page.getByText('10 seconds to 30 seconds')).toBeVisible()
  await expect(page.getByText('A plate observation does not establish ownership.')).toBeVisible()
  await expect(page.getByText('Technical details', { exact: true })).toBeVisible()
  await expect(page.getByText('anpr.sightings')).toBeHidden()
  await page.getByText('Technical details', { exact: true }).click()
  await page.getByText('Execution notes', { exact: true }).click()
  await expect(page.getByText('Planner Confidence: 0.9')).toBeVisible()
  await page.getByText('More details').click()
  await page.locator('.chat-messages').evaluate(element => { element.scrollTop = 0 })
  await page.screenshot({ path: 'reports/nx-ux1d-visual-evidence/ask-positive-citation-1440.png', fullPage: true })
  await page.getByRole('button', { name: 'Open evidence source calls.csv' }).click()
  await expect(page).toHaveURL(new RegExp(`analyst\\?.*evidence=${evidenceId}.*row=42.*panel=evidence`))
  await expect(page.getByRole('dialog', { name: 'Calls.csv' }).getByRole('region', { name: 'Citation context' })).toContainText('Row 42')
})

test('keeps complete-zero and no-match materially distinct in Ask', async ({ page }) => {
  await mockAnalystPortal(page)
  await seedAskResult(page, 'Which plates appear?', {
    status: 'answered', result_state: 'complete_zero_results', executive_answer: 'The retained video was analyzed.', findings: [], table: { columns: [], rows: [] }, citations: [], limitations: [],
  })
  await page.goto(`/analyst/ask?case=${caseId}`)
  await expect(page.getByText('Processing completed. No detections were found.')).toBeVisible()
  await expect(page.getByText('No detections found', { exact: true })).toHaveCount(0)
  await page.locator('.chat-messages').evaluate(element => { element.scrollTop = 0 })
  await page.screenshot({ path: 'reports/nx-ux1d-visual-evidence/ask-complete-zero-1024.png', fullPage: true })

  await page.evaluate(storageCase => localStorage.removeItem(`localai_agent_chats_Forensic_Records_Analyst__case_${storageCase}`), caseId)
  await page.reload()
  await seedAskResult(page, 'Find plate filter.', {
    status: 'no_results', result_state: 'no_match_for_filter', executive_answer: 'The source contains plate observations.', findings: [], table: { columns: [], rows: [] }, citations: [], limitations: [],
  })
  await page.reload()
  await expect(page.getByText('No results matched this filter.')).toBeVisible()
  await expect(page.getByText('No matching results', { exact: true })).toHaveCount(0)
  await page.locator('.chat-messages').evaluate(element => { element.scrollTop = 0 })
  await page.screenshot({ path: 'reports/nx-ux1d-visual-evidence/ask-no-match-1024.png', fullPage: true })
})

test('preserves mixed Urdu and LTR identifiers in the direction-aware Ask composer', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.setViewportSize({ width: 390, height: 820 })
  await page.goto(`/analyst/ask?case=${caseId}`)

  const query = 'ویڈیو ایویڈنس 50057921-4f1f-4ab8-bab0-47bcdc957822 کی پلیٹ MN1367 دکھائیں'
  const composer = page.getByRole('textbox', { name: 'Analyst query' })
  await expect(composer).toHaveAttribute('dir', 'auto')
  await composer.fill(query)
  await expect(composer).toHaveValue(query)
  await expect(page.getByRole('button', { name: 'Ask question' })).toBeEnabled()
  await page.screenshot({ path: 'reports/nx-ux1d-visual-evidence/ask-mixed-urdu-390.png', fullPage: true })
})

test('keeps a natural Urdu transcript phrase primary and normalized targets hidden', async ({ page }) => {
  await mockAnalystPortal(page)
  const phrase = 'مجھے معلوم نہیں آیا آپ نے محسوس کیا یا نہیں'
  const query = `آڈیو متن میں تلاش کریں "${phrase}"`
  await seedAskResult(page, query, {
    status: 'answered', result_state: 'results_present', language: 'ur', text_direction: 'rtl',
    executive_answer: `An exact normalized mention of "${phrase}" was found in this recording.`,
    operation_id: 'audio.transcript_search',
    table: { title: 'Transcript match', columns: ['start_seconds', 'end_seconds', 'raw_text', 'normalized_target'], rows: [{ start_seconds: 0, end_seconds: 10, raw_text: phrase, normalized_target: 'مجھے-معلوم-نہیں-آیا' }] },
    citations: [{ label: 'interview.wav', evidence_id: evidenceId, source_time: { start_seconds: 0, end_seconds: 10 } }],
    findings: [{ text: `The phrase "${phrase}" was found in this recording.`, claim_type: 'deterministic_fact' }],
    limitations: [],
  })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/analyst?case=${caseId}`)

  const answer = page.getByRole('region', { name: 'Answer' })
  await expect(answer).toContainText(`The phrase "${phrase}" was found in this recording.`)
  await expect(answer.getByText('0 seconds')).toBeVisible()
  await expect(answer.getByText('مجھے-معلوم-نہیں-آیا')).toBeHidden()
  await expect(answer.getByRole('button', { name: 'Open evidence source interview.wav' })).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Analyst query' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390)
  await page.screenshot({ path: 'reports/nx-ux1g-final-acceptance/final-urdu-result-390.png', fullPage: true })
})

test('walks a grounded document answer through its source locator and follow-up', async ({ page }) => {
  await mockAnalystPortal(page)
  const documentID = 'document-policy-1'
  const followUp = 'Compare this passage with the other selected policy.'
  let followUpRequest
  const documentItem = {
    evidence_id: documentID,
    case_id: caseId,
    original_filename: 'policy.pdf',
    modality: 'document',
    detected_type: 'pdf',
    processing_status: 'completed',
    size_bytes: 4096,
    created_at: '2026-09-16T08:00:00Z',
  }
  await page.route(`**/api/v1/forensics/cases/${caseId}/evidence**`, route => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith(`/${documentID}/content`)) return route.fulfill({ status: 200, contentType: 'application/pdf', body: '' })
    if (url.pathname.endsWith(`/${documentID}`)) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({
        item: documentItem,
        derived_artifacts: [{
          artifact_id: 'passage-1', artifact_type: 'forensics.document-native-text-passage/v1', processing_status: 'completed',
          citation_locator: { page: 4, paragraph: 2 },
          metadata: { observation: { passage_text: 'The retention schedule applies for seven years.' } },
        }],
        ingest_jobs: [{ status: 'completed' }], warnings: [], errors: [],
      }) })
    }
    if (url.pathname.endsWith(`/${caseId}/evidence`)) {
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [documentItem], summary: { evidence_total: 1, completed: 1, status_counts: { completed: 1 } }, pagination: { has_next: false } }) })
    }
    return route.fallback()
  })
  await page.route('**/api/agents/Forensic_Records_Analyst/chat**', async route => {
    followUpRequest = route.request().postDataJSON()
    await route.fulfill({ status: 202, contentType: 'application/json', body: JSON.stringify({ message_id: 'document-follow-up-1' }) })
  })
  await seedAskResult(page, 'Find the exact phrase "retention schedule" in the document.', {
    status: 'answered_with_limitations', result_state: 'results_present',
    executive_answer: 'The cited policy states that the retention schedule applies for seven years.',
    operation_id: 'document.search', retrieval_mode: 'exact_phrase',
    table: {
      title: 'Cited document passages', columns: ['source_file', 'source_location', 'passage'],
      rows: [{ citation_id: 'passage-1', source_file: 'policy.pdf', source_location: 'page 4 · paragraph 2', passage: 'The retention schedule applies for seven years.' }],
    },
    citations: [{ label: 'policy.pdf', evidence_id: documentID, citation_locator: { page: 4, paragraph: 2 } }],
    findings: [{ text: 'The retention schedule applies for seven years.', claim_type: 'deterministic_fact' }],
    limitations: ['Only bounded retained passages from the selected evidence were searched.'],
    next_actions: [{ text: 'Compare with another policy', query: followUp }],
    trace: { retrieval_strategy: { mode: 'exact_phrase', exact_is_semantic: false } },
  })

  await page.goto(`/analyst?case=${caseId}`)
  const answer = page.getByRole('region', { name: 'Answer' })
  await expect(answer.getByRole('heading', { name: /Cited document passages/ })).toBeVisible()
  await expect(answer.getByText('The retention schedule applies for seven years.', { exact: true }).first()).toBeVisible()
  await expect(answer.getByText('page 4 · paragraph 2').first()).toBeVisible()
  await expect(answer.getByText('Only bounded retained passages from the selected evidence were searched.')).toBeVisible()
  await expect(answer.getByText(/using exact phrase retrieval/)).toBeVisible()
  await page.locator('.chat-messages').evaluate(element => { element.scrollTop = 0 })
  await page.screenshot({ path: 'reports/nxb21-five-flows/document-grounded-answer-1440.png', fullPage: true })

  await answer.getByRole('button', { name: 'Open evidence source policy.pdf' }).click()
  await expect(page).toHaveURL(/evidence=document-policy-1.*page=4.*panel=evidence/)
  await expect(page.getByRole('dialog', { name: 'Policy.pdf' }).getByRole('region', { name: 'Citation context' })).toContainText('Page 4')
  await page.getByRole('button', { name: 'Close source details' }).click()
  await page.goto(`/analyst?case=${caseId}`)
  await page.getByRole('region', { name: 'Answer' }).getByRole('button', { name: 'Compare with another policy' }).click()
  await expect(page.getByText(followUp, { exact: true })).toBeVisible()
  await expect.poll(() => followUpRequest).toMatchObject({ message: followUp, case_id: caseId, collection_id: caseId })
})

test('keeps question, compact answer, evidence and composer within one normal viewport', async ({ page }) => {
  await mockAnalystPortal(page)
  await seedAskResult(page, 'Find LEH5003.', {
    status: 'answered', result_state: 'results_present', executive_answer: 'Retrieved exact ANPR observations with canonical evidence, version, row, and hash provenance.',
    operation_id: 'anpr.sightings', metrics: [{ label: 'Target', value: 'LEH5003' }],
    table: { title: 'Vehicle sightings', columns: ['observed_at', 'camera_id', 'location'], rows: [{ observed_at: '2026-09-07T04:44:55Z', camera_id: '', location: '' }] },
    citations: [{ label: 'DSC_1039.JPG', evidence_id: evidenceId }], findings: [], limitations: [],
  })

  for (const width of [390, 820, 1024, 1440, 1600]) {
    await page.setViewportSize({ width, height: 900 })
    await page.goto(`/analyst?case=${caseId}`)
    await expect(page.getByText('Find LEH5003.', { exact: true })).toBeVisible()
    await expect(page.getByText('1 sighting of LEH5003 was found.')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Open evidence source DSC_1039.JPG' })).toBeVisible()
    await expect(page.getByRole('textbox', { name: 'Analyst query' })).toBeVisible()
    const dimensions = await page.evaluate(() => ({ width: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth }))
    expect(dimensions.scrollWidth, `primary answer overflow at ${width}px`).toBeLessThanOrEqual(dimensions.width)
    const answerHeight = await page.getByRole('region', { name: 'Answer' }).evaluate(element => element.getBoundingClientRect().height)
    expect(answerHeight, `primary answer density at ${width}px`).toBeLessThan(620)
  }
})

test('presents governed Activity as a searchable journal and reopens the stored result', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto(`/analyst/activity?case=${caseId}`)

  await expect(page.getByRole('heading', { name: 'History', exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: /August 17, 2026|Earlier/ })).toBeVisible()
  await page.screenshot({ path: 'reports/nx-ux1e-visual-evidence/activity-journal-results-1440.png', fullPage: true })
  const reopen = page.locator('.analyst-activity-item__reopen').filter({ hasText: 'Who did this number contact most?' })
  await reopen.focus()
  await reopen.press('Enter')
  const detail = page.getByRole('complementary')
  await expect(detail.getByText('Eight contacts were ranked from exact CDR events.')).toBeVisible()
  await expect(detail.getByText('1 result', { exact: true })).toBeVisible()
  await expect(detail.getByRole('heading', { name: 'Key findings' })).toBeVisible()
  await expect(detail.getByRole('heading', { name: 'Most frequent phone contacts' })).toBeVisible()
  await expect(detail.getByText('Frequency does not establish identity, ownership, or intent.')).toBeHidden()
  await detail.getByText('Limitations', { exact: true }).click()
  await expect(detail.getByText('Frequency does not establish identity, ownership, or intent.')).toBeVisible()
  await page.screenshot({ path: 'reports/nx-ux1e-visual-evidence/activity-reopened-result-1440.png', fullPage: true })
  await page.screenshot({ path: 'reports/nx-ux1g-final-acceptance/activity-reopened-result-1440.png', fullPage: true })
  const sourceLink = detail.getByRole('link', { name: 'Open evidence source Calls.csv' })
  await expect(sourceLink).toBeEnabled()
  await page.getByRole('textbox', { name: 'Search loaded activity' }).fill('calls.csv')
  await expect(page.locator('.analyst-activity-item__reopen').filter({ hasText: 'Who did this number contact most?' })).toBeVisible()
  await page.setViewportSize({ width: 390, height: 820 })
  const sizes = await page.evaluate(() => ({ viewport: window.innerWidth, document: document.documentElement.scrollWidth }))
  expect(sizes.document).toBeLessThanOrEqual(sizes.viewport)
  await page.screenshot({ path: 'reports/nx-ux1e-visual-evidence/activity-mobile-390.png', fullPage: true })
  await page.screenshot({ path: 'reports/nx-ux1g-final-acceptance/activity-reopened-result-390.png', fullPage: true })

  await sourceLink.click()
  await expect(page.getByRole('dialog', { name: 'Calls.csv' }).getByText('5,004 verified records')).toBeVisible()
  await page.goto(`/analyst/activity?case=${caseId}&analysis=analysis-1`)
  await page.getByRole('link', { name: 'Use question again', exact: true }).click()
  await expect(page).toHaveURL(new RegExp(`/analyst\\?evidence=${evidenceId}&case=${caseId}$`))
  await expect(page.getByRole('textbox', { name: 'Analyst query' })).toHaveValue('Who did this number contact most?')
})

test('preserves complete-zero and no-match result states when Activity reopens', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.unroute('**/api/agents/Forensic_Records_Analyst/history**')
  const historyItems = [
    { analysis_id: 'analysis-zero', query: 'Which plates appear in this video?', answer: 'Completed zero.', status: 'completed', created_at: '2026-08-25T08:30:00Z', metadata: { presentation: { status: 'answered', result_state: 'complete_zero_results', processing_state: 'completed', row_count: 0, executive_answer: 'ANPR processing complete, 0 plate groups detected.', family_id: 'anpr_vehicles', operation_id: 'video.anpr_grouped_timeline', metrics: [], findings: [{ claim_type: 'deterministic_fact', text: 'The retained source was processed.' }, { claim_type: 'metric', text: 'Template: video_timeline' }], table: { columns: [], rows: [] }, citations: [], limitations: [] } } },
    { analysis_id: 'analysis-miss', query: 'Find ZZZ9999.', answer: 'No match.', status: 'completed', created_at: '2026-08-25T08:31:00Z', metadata: { presentation: { status: 'no_results', result_state: 'no_match_for_filter', row_count: 0, executive_answer: 'No matching records were found for ZZZ9999 in the selected case scope.', family_id: 'anpr_vehicles', operation_id: 'anpr.sightings', metrics: [], findings: [], table: { columns: [], rows: [] }, citations: [], limitations: [] } } },
  ]
  await page.route('**/api/agents/Forensic_Records_Analyst/history**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: historyItems, has_more: false }) }))

  for (const width of [390, 820, 1024, 1440]) {
    await page.setViewportSize({ width, height: 900 })
    await page.goto(`/analyst/activity?case=${caseId}&analysis=analysis-zero`)
    const detail = page.getByRole('complementary')
    await expect(detail.getByText('Processing completed · No detections')).toBeVisible()
    await expect(detail.getByText('ANPR processing complete, 0 plate groups detected.')).toBeVisible()
    await expect(detail.getByText('The retained source was processed.')).toBeVisible()
    await expect(detail.getByText('Template: video_timeline')).toBeHidden()
    await expect(page.getByRole('button', { name: 'No results 2' })).toBeVisible()
    await expect(page.getByRole('button', { name: /^Results/ })).toHaveCount(0)
    await expect(page.getByRole('button', { name: /^Processing \d/ })).toHaveCount(0)
    await expect(page.getByRole('button', { name: /^Failed/ })).toHaveCount(0)
    const sizes = await page.evaluate(() => ({ viewport: document.documentElement.clientWidth, content: document.documentElement.scrollWidth }))
    expect(sizes.content).toBeLessThanOrEqual(sizes.viewport)
    await detail.getByText('Technical details', { exact: true }).click()
    await expect(detail.getByText('Template: video_timeline')).toBeVisible()
    if (width === 1024) await page.screenshot({ path: 'reports/nx-ux1e-visual-evidence/activity-complete-zero-1024.png', fullPage: true })
  }

  await page.goto(`/analyst/activity?case=${caseId}&analysis=analysis-miss`)
  const detail = page.getByRole('complementary')
  await expect(detail.getByText('No results matched this filter')).toBeVisible()
  await expect(detail.getByText('No matching records were found for ZZZ9999 in the selected case scope.')).toBeVisible()
  await page.screenshot({ path: 'reports/nx-ux1e-visual-evidence/activity-no-match-1440.png', fullPage: true })
})

test('preserves supported source locators across Activity and Data navigation', async ({ page }) => {
  await mockAnalystPortal(page)
  const sources = [
    { evidence_id: 'structured-random-1', original_filename: 'calls-export.csv', detected_type: 'cdr', modality: 'structured_records', processing_status: 'completed', created_at: '2026-08-27T07:00:00Z' },
    { evidence_id: 'document-random-2', original_filename: 'report.pdf', detected_type: 'pdf', modality: 'document', processing_status: 'completed', created_at: '2026-08-27T07:00:00Z' },
    { evidence_id: 'image-random-3', original_filename: 'scene.jpg', detected_type: 'image', modality: 'image', processing_status: 'completed', created_at: '2026-08-27T07:00:00Z' },
    { evidence_id: 'anpr-random-4', original_filename: 'gate.jpg', detected_type: 'anpr', modality: 'image', processing_status: 'completed', created_at: '2026-08-27T07:00:00Z' },
    { evidence_id: 'audio-random-5', original_filename: 'interview.wav', detected_type: 'audio', modality: 'audio', processing_status: 'completed', created_at: '2026-08-27T07:00:00Z' },
    { evidence_id: 'video-random-6', original_filename: 'camera.mp4', detected_type: 'video', modality: 'video', processing_status: 'completed', created_at: '2026-08-27T07:00:00Z' },
  ]
  const citations = [
    { evidence_id: sources[0].evidence_id, source_file: sources[0].original_filename, locator: { row_number: 42 } },
    { evidence_id: sources[1].evidence_id, source_file: sources[1].original_filename, locator: { page: 3 } },
    { evidence_id: sources[2].evidence_id, source_file: sources[2].original_filename, artifact_id: 'ocr-random-7' },
    { evidence_id: sources[3].evidence_id, source_file: sources[3].original_filename, citation_locator: { artifact_id: 'plate-random-8' } },
    { evidence_id: sources[4].evidence_id, source_file: sources[4].original_filename, locator: { start_seconds: 8 } },
    { evidence_id: sources[5].evidence_id, source_file: sources[5].original_filename, locator: { timestamp_seconds: 20 } },
  ]
  const historyItem = { analysis_id: 'analysis-source-matrix', query: 'ویڈیو source-random-6 کی متعلقہ findings دکھائیں', answer: 'Six evidence references were retained.', status: 'completed', created_at: '2026-08-27T08:30:00Z', metadata: { presentation: { contract_version: 'forensics.agent-presentation/v1', result_state: 'results_present', row_count: 6, executive_answer: 'Six evidence references were retained.', family_id: 'case_cross_family', citations, findings: [], table: { columns: [], rows: [] }, limitations: [] } } }

  await page.unroute('**/api/agents/Forensic_Records_Analyst/history**')
  await page.route('**/api/agents/Forensic_Records_Analyst/history**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [historyItem], has_more: false }) }))
  await page.route(`**/api/v1/forensics/cases/${caseId}/evidence**`, route => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith('/content')) return route.fulfill({ status: 200, contentType: 'application/octet-stream', body: '' })
    const evidenceID = url.pathname.split('/').pop()
    const item = sources.find(source => source.evidence_id === evidenceID)
    if (item) return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ item, derived_artifacts: [], ingest_jobs: [{ status: 'completed' }], warnings: [], errors: [] }) })
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: sources, summary: { evidence_total: sources.length, completed: sources.length, status_counts: { completed: sources.length } }, pagination: { has_next: false } }) })
  })

  await page.goto(`/analyst/activity?case=${caseId}&analysis=analysis-source-matrix`)
  await expect(page.getByText('ویڈیو source-random-6 کی متعلقہ findings دکھائیں', { exact: true }).first()).toBeVisible()
  const detail = page.getByRole('complementary')
  const matrix = [
    ['Calls export.csv', /evidence=structured-random-1.*row=42/],
    ['Report.pdf', /evidence=document-random-2.*page=3/],
    ['Scene.jpg', /evidence=image-random-3.*finding=ocr-random-7/],
    ['Gate.jpg', /evidence=anpr-random-4.*finding=plate-random-8/],
    ['Interview.wav', /evidence=audio-random-5.*source_time=8/],
    ['Camera.mp4', /evidence=video-random-6.*source_time=20/],
  ]
  for (const [label, href] of matrix) await expect(detail.getByRole('link', { name: `Open evidence source ${label}` })).toHaveAttribute('href', href)
  await page.getByRole('textbox', { name: 'Search loaded activity' }).fill('camera.mp4')
  await expect(page.locator('.analyst-activity-item__reopen')).toHaveCount(1)
  const videoSource = detail.getByRole('link', { name: 'Open evidence source Camera.mp4' })
  await videoSource.focus()
  await videoSource.press('Enter')
  await expect(page).toHaveURL(/analyst\?panel=evidence.*evidence=video-random-6.*source_time=20/)
  const drawer = page.getByRole('dialog', { name: 'Camera.mp4' })
  await expect(drawer.getByRole('region', { name: 'Citation context' })).toContainText('Source time')
  await expect(drawer.getByRole('region', { name: 'Citation context' })).toContainText('20 seconds')
  await expect(drawer.getByText('Playback, seeking, and page movement remain analyst-controlled.')).toBeVisible()
  await page.screenshot({ path: 'reports/nx-ux1e-visual-evidence/activity-source-navigation-1024.png', fullPage: true })
})

test('distinguishes empty Activity from a load failure', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.unroute('**/api/agents/Forensic_Records_Analyst/history**')
  await page.route('**/api/agents/Forensic_Records_Analyst/history**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [], has_more: false }) }))
  await page.goto(`/analyst/activity?case=${caseId}`)
  await expect(page.getByRole('heading', { name: 'No activity yet' })).toBeVisible()
  await expect(page.getByText('Questions you ask and investigation results will appear here.')).toBeVisible()
  await expect(page.getByRole('link', { name: 'Ask a question' }).last()).toBeVisible()

  await page.unroute('**/api/agents/Forensic_Records_Analyst/history**')
  await page.route('**/api/agents/Forensic_Records_Analyst/history**', route => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: 'internal history transport detail' }) }))
  await page.reload()
  await expect(page.getByRole('alert')).toContainText('Activity could not be loaded.')
  await expect(page.getByRole('heading', { name: 'No activity yet' })).toHaveCount(0)
  await expect(page.getByText('internal history transport detail')).toBeHidden()
  await page.getByText('Technical details', { exact: true }).click()
  await expect(page.getByText(/Request failed|internal history transport detail/)).toBeVisible()
})

test('consumes inbound Ask prompts once without moving the portal page', async ({ page }) => {
  await mockAnalystPortal(page)
  const prompt = 'Show temporal CDR activity for 923001234567 on 2026-07-10.'
  await page.goto(`/analyst/ask?case=${caseId}&prompt=${encodeURIComponent(prompt)}`)

  await expect(page.getByRole('textbox', { name: 'Analyst query' })).toHaveValue(prompt)
  await expect(page).toHaveURL(new RegExp(`/analyst\\?case=${caseId}$`))
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0)

  await page.getByRole('textbox', { name: 'Analyst query' }).fill('My edited analyst question')
  await page.reload()
  await expect(page.getByRole('textbox', { name: 'Analyst query' })).not.toHaveValue(prompt)
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0)
})

test('keeps every existing portal route responsive and structurally fixed in both themes', async ({ page }) => {
  test.setTimeout(120_000)
  const errors = []
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
  page.on('pageerror', error => errors.push(error.message))
  await mockAnalystPortal(page)
  const shellGeometry = new Map()
  for (const theme of ['light', 'dark']) {
    await page.goto(`/analyst/home?case=${caseId}`)
    await page.evaluate(value => localStorage.setItem('localai-theme', value), theme)
    await page.reload()
    for (const width of [390, 820, 1024, 1440, 1600]) {
      await page.setViewportSize({ width, height: 900 })
      for (const route of ['home', 'ask', 'data', 'activity', 'history']) {
        await page.goto(`/analyst/${route}?case=${caseId}`)
        await expect(page.getByRole('link', { name: 'Investigation Workspace home' })).toBeVisible()
        await expect(page.locator('html')).toHaveAttribute('data-theme', theme)
        await expect(page.getByRole('link', { name: 'Add data' })).toBeVisible()
        await expect(page.getByRole('button', { name: 'Workspace menu' })).toBeVisible()
        const state = await page.evaluate(() => {
          const bounds = selector => {
            const rect = document.querySelector(selector).getBoundingClientRect()
            return [Math.round(rect.left), Math.round(rect.top), Math.round(rect.width), Math.round(rect.height)]
          }
          return {
            viewport: document.documentElement.clientWidth,
            content: document.documentElement.scrollWidth,
            shell: {
              brand: bounds('.analyst-brand'),
              workspace: bounds('.analyst-workspace-select'),
              add: bounds('.analyst-add-trigger'),
              menu: bounds('.analyst-account-trigger'),
            },
          }
        })
        expect(state.content, `${theme} ${route} overflow at ${width}px`).toBeLessThanOrEqual(state.viewport)
        const geometryKey = `${width}:${route}`
        if (theme === 'light') shellGeometry.set(geometryKey, state.shell)
        else expect(state.shell, `${route} navigation moved between themes at ${width}px`).toEqual(shellGeometry.get(geometryKey))
      }
    }
  }
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto(`/analyst?case=${caseId}`)
  const workingWidth = await page.locator('.chat-messages').evaluate(element => element.getBoundingClientRect().width)
  expect(workingWidth).toBeGreaterThan(1080)
  const headerAlignment = await page.evaluate(() => {
    const brand = document.querySelector('.analyst-brand').getBoundingClientRect()
    const selector = document.querySelector('.analyst-workspace-select').getBoundingClientRect()
    return Math.abs((brand.top + brand.height / 2) - (selector.top + selector.height / 2))
  })
  expect(headerAlignment).toBeLessThan(12)
  expect(errors).toEqual([])
})

test('preserves legacy History deep links through the unified history drawer', async ({ page }) => {
  await mockAnalystPortal(page)
  await page.goto(`/analyst/history?case=${caseId}&analysis=analysis-1`)

  await expect(page).toHaveURL(new RegExp(`/analyst\\?case=${caseId}&analysis=analysis-1&panel=history$`))
  await expect(page.getByRole('heading', { name: 'History', exact: true })).toBeVisible()
})

test('supports keyboard account access, theme switching, and a clean portal console', async ({ page }) => {
  const errors = []
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
  await mockAnalystPortal(page)
  await page.goto(`/analyst/home?case=${caseId}`)

  const account = page.getByRole('button', { name: 'Workspace menu' })
  await account.focus()
  await account.press('Enter')
  await expect(account).toHaveAttribute('aria-expanded', 'true')
  await expect(page.getByText('Administrator access')).toBeVisible()
  const themeButton = page.getByRole('menuitem', { name: /Light mode|Dark mode/ })
  const initialThemeAction = await themeButton.innerText()
  await themeButton.click()
  await expect(page.getByRole('menuitem', { name: initialThemeAction.includes('Light mode') ? 'Dark mode' : 'Light mode' })).toBeVisible()
  expect(errors).toEqual([])
})
