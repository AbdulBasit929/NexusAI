import { test, expect } from './coverage-fixtures.js'

async function mockCaseWorkspace(page, { caseListGate, onCaseList, agentStatuses, agentRegistryFailure = false } = {}) {
  await page.route('**/api/auth/status', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ authEnabled: false, staticApiKeyRequired: false, user: null }) }))
  await page.route('**/api/features', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ agents: true, records: true, collections: true }) }))
  await page.route('**/version', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ version: 'test' }) }))
  await page.route('**/api/agents', route => agentRegistryFailure
    ? route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: 'registry unavailable' }) })
    : route.fulfill({ contentType: 'application/json', body: JSON.stringify({
      agentCount: 2,
      agents: ['Forensic_Records_Analyst', 'Communications_CDR_Analyst'],
      statuses: agentStatuses || { Forensic_Records_Analyst: true, Communications_CDR_Analyst: true },
    }) }))
  await page.route('**/api/v1/forensics/cases', async route => {
    onCaseList?.()
    if (caseListGate) await caseListGate
    await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
    contract_version: 'forensics.case-governance/v1', default_case_id: 'records-demo-verified', hidden_count: 16, deletion_count: 0,
    cases: [
      { case_id: 'records-demo-verified', collection_id: 'records-demo-verified', display_name: 'Verified Records Demo', selectable: true, purpose: 'canonical_pilot' },
      { case_id: 'records-demo', collection_id: 'records-demo', display_name: 'records-demo', selectable: true, purpose: 'legacy_demo' },
    ],
    }) })
  })
  await page.route('**/api/v1/forensics/cases/records-demo-verified', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
    case_id: 'records-demo-verified', collection_id: 'records-demo-verified', display_name: 'Verified Records Demo', purpose: 'canonical_pilot', environment: 'demo', case_status: 'active', visibility: 'analyst', security_classification: 'internal', retention_class: 'preserve_reference', cleanup_disposition: 'preserve', warnings: [],
  }) }))
  await page.route('**/api/v1/forensics/cases/records-demo-verified/manifest', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
    contract_version: 'forensics.case-governance/v1', case_id: 'records-demo-verified', collection_id: 'records-demo-verified', read_only: true, deletion_count: 0,
    resources: [
      { resource: 'kb_entries', count: 8, status: 'counted', source: 'localai_kb' },
      { resource: 'canonical_records', count: 18192, status: 'counted', source: 'forensic.records' },
      { resource: 'agents', count: 1, status: 'counted', source: 'localai_agent_pool' },
    ],
    job_statuses: [{ status: 'completed', count: 4, last_activity_at: '2026-07-30T10:00:00Z' }], case_metadata: { warnings: [] },
  }) }))
  await page.route('**/api/v1/forensics/cases/records-demo-verified/evidence**', route => {
    const offset = Number(new URL(route.request().url()).searchParams.get('offset') || 0)
    const pageItem = offset > 0
      ? { evidence_id: 'evidence-2', collection_id: 'records-demo-verified', case_id: 'records-demo-verified', source_file: 'tower.csv', sha256: 'd'.repeat(64), modality: 'structured_records', detected_type: 'tower', classifier_confidence: 0.98, processing_route: 'structured_records', processing_status: 'completed', size_bytes: 2048, created_at: '2026-07-30T10:02:00Z' }
      : { evidence_id: 'evidence-1', collection_id: 'records-demo-verified', case_id: 'records-demo-verified', source_file: 'calls.csv', sha256: 'a'.repeat(64), modality: 'structured_records', detected_type: 'cdr', classifier_confidence: 0.99, processing_route: 'structured_records', processing_status: 'completed', size_bytes: 1024, created_at: '2026-07-30T10:00:00Z' }
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({
      count: 1,
      summary: { evidence_total: 2, completed: 2, processing: 0, queued: 0, failed: 0, total_size_bytes: 3072 },
      pagination: { limit: 25, offset, returned: 1, has_next: offset === 0, has_previous: offset > 0, evidence_total: 2 },
      items: [pageItem],
    }) })
  })
  await page.route('**/api/v1/forensics/cases/records-demo-verified/evidence/evidence-1**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
    evidence_id: 'evidence-1', collection_id: 'records-demo-verified', records_preview_included: true,
    item: { evidence_id: 'evidence-1', collection_id: 'records-demo-verified', case_id: 'records-demo-verified', source_file: 'calls.csv', content_type: 'text/csv', size_bytes: 1024, sha256: 'a'.repeat(64), modality: 'structured_records', detected_type: 'cdr', classifier_confidence: 0.99, processing_route: 'structured_records', processing_status: 'completed', raw_storage_ref: 'forensic://source/evidence-1', records_batch_id: 'batch-1', kb_entry_ref: 'kb-entry-1', warnings: ['One malformed source row was rejected.'], errors: [], created_at: '2026-07-30T10:00:00Z' },
    ingest_jobs: [{ job_id: 'job-1', status: 'completed', record_type: 'cdr', total_rows: 103, accepted_rows: 100, duplicate_rows: 2, rejected_rows: 1, attempt_count: 1, max_attempts: 3, reprocess_generation: 0, completed_at: '2026-07-30T10:01:00Z' }],
    kb_assets: [{ kb_asset_id: 'asset-1', rag_status: 'ready', structured_status: 'ready' }],
    processing_runs: [{ run_id: 'run-1', run_kind: 'ingest', pipeline_id: 'records-worker', status: 'succeeded', attempt_count: 1, completed_at: '2026-07-30T10:01:00Z' }],
    processing_events: [{ event_id: 1, run_id: 'run-1', event_sequence: 1, event_type: 'processing_completed', status_from: 'running', status_to: 'succeeded', occurred_at: '2026-07-30T10:01:00Z' }],
    derived_artifacts: [{ artifact_id: 'artifact-1', run_id: 'run-1', artifact_type: 'canonical_records', media_type: 'application/json', content_sha256: 'b'.repeat(64), processing_status: 'completed', citation_ref: 'nexusai://evidence/evidence-1/artifacts/artifact-1', citation_locator: { batch_id: 'batch-1', row_start: 1, row_end: 100 } }],
    custody_events: [{ custody_event_id: 'custody-1', event_type: 'registered', actor_type: 'system', actor_id: 'forensic-api', custody_location: 'content-addressed-store', reason: 'Source registered', occurred_at: '2026-07-30T10:00:00Z', previous_event_sha256: null, event_sha256: 'c'.repeat(64) }],
    custody_integrity: { event_count: 1, broken_links: 0, chain_valid: true, first_recorded_at: '2026-07-30T10:00:00Z', last_recorded_at: '2026-07-30T10:00:00Z' },
    records_preview: [{ record_id: 'record-1', row_number: 1, source_file: 'calls.csv' }],
    entity_rollup: [{ entity_type: 'phone', observation_count: 12, unique_entities: 4 }],
  }) }))
  await page.route('**/api/v1/forensics/cases/records-demo-verified/evidence/evidence-image**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
    evidence_id: 'evidence-image', collection_id: 'records-demo-verified', records_preview_included: false,
    item: { evidence_id: 'evidence-image', collection_id: 'records-demo-verified', case_id: 'records-demo-verified', source_file: 'vehicle.jpg', content_type: 'image/jpeg', size_bytes: 204800, sha256: 'e'.repeat(64), modality: 'image', detected_type: 'image', classifier_confidence: 0.99, processing_route: 'image_ocr_vision_pending', processing_status: 'registered', raw_storage_ref: 'forensic://source/evidence-image', warnings: [], errors: [], created_at: '2026-08-12T10:00:00Z', media_metadata: { intake_contract_version: 'forensics.image-intake/v1', validation_state: 'accepted_metadata_only', downstream_decode_allowed: true, inspection_mode: 'bounded_header_only', format: 'jpeg', width_pixels: 1920, height_pixels: 1080, pixel_count: 2073600, color_model: 'ycbcr', exif_orientation_label: 'right_top' } },
    ingest_jobs: [], kb_assets: [], processing_runs: [], processing_events: [], derived_artifacts: [{ artifact_id: 'region-1', run_id: 'detect-run-1', artifact_type: 'plate_region_candidate', media_type: 'application/json', content_sha256: 'f'.repeat(64), processing_status: 'completed', confidence: 0.91, citation_ref: 'nexusai://evidence/evidence-image/artifacts/region-1', metadata: { plate_region_candidate: { contract_version: 'forensics.plate-region-candidate/v1', candidate_id: 'plate-region-1', parent_evidence_id: 'evidence-image', parent_version_id: 'version-image', parent_sha256: 'e'.repeat(64), detector_id: 'fixture-detector', detector_version: '1.0.0', coordinate_space: 'original_image_pixels', bounds: { x: 430, y: 610, width: 380, height: 120 }, confidence: 0.91, label: 'license_plate', transform_chain: ['source_pixels_unchanged'], review_state: 'model_candidate' } } }], custody_events: [], custody_integrity: { event_count: 0, broken_links: 0, chain_valid: true }, records_preview: [], entity_rollup: [],
  }) }))
  await page.route('**/api/v1/forensics/cases/records-demo-verified/evidence/evidence-1/reprocess-plan**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
    contract_version: 'forensics.evidence-reprocess-plan/v1', case_id: 'records-demo-verified', collection_id: 'records-demo-verified', evidence_id: 'evidence-1',
    current_state: { evidence_status: 'completed', latest_job_status: 'completed', reprocess_generation: 0 },
    eligibility: { eligible: true, reason: 'The latest job is terminal; an approved immutable reprocess would create a new linked generation.' },
    approval: { required: true, state: 'not_granted', execution_permitted: false, notice: 'This plan is read-only.' },
  }) }))
  await page.route('**/api/records/forensic/status**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
    summary: { evidence_total: 2, evidence_in_flight: 1, accepted_rows: 18192, rejected_rows: 0, failed_jobs: 0, kb_assets_total: 8, completed_jobs_missing_kb_asset: 0 },
    recent_jobs: [{ job_id: 'job-queued', source_file: 'waiting.csv', record_type: 'cdr', status: 'queued', attempt_count: 1, max_attempts: 3, queued_at: '2026-07-30T10:03:00Z' }],
    record_families: [{ record_type: 'cdr', total_rows: 18192 }], recent_evidence: [],
  }) }))
  await page.route('**/api/records/forensic/capabilities**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
    families: [{ id: 'cdr', label: 'Call detail records', available: true, support_level: 'operational', indexed_records: 18192, availability_reason: 'Normalized records available.' }],
  }) }))
}

test('offers only verified operational specialist handoffs from Ask NexusAI', async ({ page }) => {
  await mockCaseWorkspace(page)
  await page.goto('/app/cases/records-demo-verified/ask?prompt=show%20top%20contacts')

  const cdr = page.getByRole('link', { name: 'CDR Specialist' })
  await expect(cdr).toBeVisible()
  await expect(cdr).toHaveAttribute('href', /Communications_CDR_Analyst\/chat\?prompt=show%20top%20contacts&case=records-demo-verified/)
  await expect(page.getByRole('link', { name: 'Ask Analyst' })).toHaveAttribute('href', /Forensic_Records_Analyst\/chat\?prompt=show%20top%20contacts&case=records-demo-verified/)
})

test('does not expose a dead specialist control when the registered agent is offline', async ({ page }) => {
  await mockCaseWorkspace(page, { agentStatuses: { Forensic_Records_Analyst: true, Communications_CDR_Analyst: false } })
  await page.goto('/app/cases/records-demo-verified/ask')

  await expect(page.getByLabel('CDR Specialist unavailable')).toBeVisible()
  await expect(page.getByRole('link', { name: 'CDR Specialist' })).toHaveCount(0)
  await expect(page.getByRole('link', { name: 'Ask Analyst' })).toBeVisible()
})

test('keeps deterministic Ask available when specialist discovery fails', async ({ page }) => {
  await mockCaseWorkspace(page, { agentRegistryFailure: true })
  await page.goto('/app/cases/records-demo-verified/ask')

  await expect(page.getByRole('alert')).toContainText('Deterministic analysis remains available; specialist handoff is disabled.')
  await expect(page.getByRole('link', { name: 'Ask Analyst' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Analyze' })).toBeEnabled()
})

test('does not report zero-data conclusions before governed case state loads', async ({ page }) => {
  let releaseCaseList
  const caseListGate = new Promise(resolve => { releaseCaseList = resolve })
  await mockCaseWorkspace(page, { caseListGate })

  const navigation = page.goto('/app/cases/records-demo-verified/overview')
  await expect(page.getByText('Verifying authorized case context…', { exact: true })).toBeVisible()
  await expect(page.getByText('Inventory only', { exact: true })).toHaveCount(0)
  await expect(page.getByText('No queryable family coverage is reported for this case yet.')).toHaveCount(0)

  releaseCaseList()
  await navigation
  await expect(page.getByRole('heading', { name: 'Hybrid ready' })).toBeVisible()
  await expect(page.getByText('Call detail records')).toBeVisible()
})

for (const viewport of [
  { name: 'desktop', width: 1440, height: 1000 },
  { name: 'tablet', width: 820, height: 1000 },
  { name: 'mobile', width: 390, height: 844 },
]) {
  test(`renders the governed workspace without page-level horizontal overflow on ${viewport.name}`, async ({ page }) => {
    await page.setViewportSize(viewport)
    await mockCaseWorkspace(page)
    await page.goto('/app/cases/records-demo-verified/overview')
    await expect(page.getByRole('heading', { name: 'Verified Records Demo' })).toBeVisible()
    await expect(page.getByRole('navigation', { name: 'Case workspace sections' })).toBeVisible()
    await expect(page.getByText('Call detail records')).toBeVisible()
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow).toBeLessThanOrEqual(1)
  })
}

test('supports keyboard navigation and keeps evidence bound to the URL case', async ({ page }) => {
  await mockCaseWorkspace(page)
  await page.goto('/app/cases/records-demo-verified/overview')
  await page.getByLabel('Active collection').focus()
  await expect(page.getByLabel('Active collection')).toBeFocused()
  await page.getByRole('navigation', { name: 'Case workspace sections' }).getByRole('link', { name: 'Evidence' }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(/\/app\/cases\/records-demo-verified\/evidence$/)
  await expect(page.getByRole('heading', { name: 'Source registry and processing truth' })).toBeVisible()
  await expect(page.getByText('calls.csv')).toBeVisible()
  await expect(page.getByLabel('Active collection')).toHaveValue('records-demo-verified')
})

test('keeps the evidence command strip first, sticky, responsive, and modality-accurate', async ({ page }) => {
  test.setTimeout(90_000)
  await mockCaseWorkspace(page)
  await page.route('**/api/v1/forensics/cases/records-demo-verified/evidence**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
    count: 1,
    summary: { evidence_total: 1, completed: 1, processing: 0, queued: 0, failed: 0, total_size_bytes: 4096 },
    pagination: { limit: 25, offset: 0, returned: 1, has_next: false, has_previous: false, evidence_total: 1 },
    items: [{
      evidence_id: 'evidence-audio', collection_id: 'records-demo-verified', case_id: 'records-demo-verified',
      source_file: 'interview.wav', content_type: 'audio/wav', sha256: 'f'.repeat(64), modality: 'audio',
      detected_type: 'speech_audio', processing_route: 'audio_transcription', processing_status: 'completed',
      size_bytes: 4096, created_at: '2026-09-17T10:00:00Z',
    }],
  }) }))

  const lightGeometry = new Map()
  for (const theme of ['light', 'dark']) {
    await page.goto('/app/cases/records-demo-verified/evidence')
    await page.evaluate(value => localStorage.setItem('localai-theme', value), theme)
    await page.reload()

    for (const width of [390, 820, 1024, 1440, 1600]) {
      await page.setViewportSize({ width, height: 900 })
      await page.waitForTimeout(350)
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme)
      const audioEvidence = page.getByRole('button', { name: 'Inspect interview.wav' })
      await expect(audioEvidence.locator('.evidence-source__icon')).toBeVisible()
      await expect(audioEvidence.locator('.evidence-source__icon i')).toHaveClass(/fa-waveform-lines/)

      const geometry = await page.evaluate(() => {
        const command = document.querySelector('.evidence-command--sticky').getBoundingClientRect()
        const summary = document.querySelector('.evidence-summary').getBoundingClientRect()
        return {
          viewport: document.documentElement.clientWidth,
          content: document.documentElement.scrollWidth,
          command: [Math.round(command.left), Math.round(command.width), Math.round(command.height)],
          commandTop: Math.round(command.top),
          summaryTop: Math.round(summary.top),
        }
      })
      expect(geometry.content, `${theme} evidence overflow at ${width}px`).toBeLessThanOrEqual(geometry.viewport)
      expect(geometry.commandTop, `${theme} command strip order at ${width}px`).toBeLessThan(geometry.summaryTop)
      if (theme === 'light') lightGeometry.set(width, geometry.command)
      else expect(geometry.command, `evidence command geometry moved between themes at ${width}px`).toEqual(lightGeometry.get(width))

      await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight))
      const stickyTop = await page.locator('.evidence-command--sticky').evaluate(element => Math.round(element.getBoundingClientRect().top))
      expect(stickyTop, `${theme} sticky toolbar at ${width}px`).toBeGreaterThanOrEqual(0)
      expect(stickyTop, `${theme} sticky toolbar at ${width}px`).toBeLessThanOrEqual(16)
      await page.evaluate(() => window.scrollTo(0, 0))
    }
  }

  await page.getByLabel('Search registered evidence').fill('interview')
  await page.getByLabel('Processing state').selectOption('completed')
  await page.getByLabel('Modality').selectOption('audio')
  const chips = page.getByLabel('Active evidence filters')
  await expect(chips).toContainText('Search: interview')
  await expect(chips).toContainText('State: Completed')
  await expect(chips).toContainText('Modality: Audio')
  await chips.getByRole('button', { name: 'Clear all' }).click()
  await expect(chips).toHaveCount(0)
})

test('inspects case-scoped evidence truth, verified custody, and stable artifact citations', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  await mockCaseWorkspace(page)
  await page.goto('/app/cases/records-demo-verified/evidence')

  await expect(page.getByRole('heading', { name: 'Source registry and processing truth' })).toBeVisible()
  await page.getByRole('button', { name: 'Inspect calls.csv' }).click()

  await expect(page.getByRole('heading', { name: 'calls.csv' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Exact row accounting' })).toBeVisible()
  const accounting = page.getByRole('region', { name: 'Exact row accounting' })
  await expect(accounting).toContainText('103')
  await expect(accounting).toContainText('100')
  await expect(accounting).toContainText('2')
  await expect(accounting).toContainText('1')
  await expect(page.getByRole('heading', { name: 'Lineage and linked outputs' })).toBeVisible()
  await expect(page.getByText('forensic://source/evidence-1')).toBeVisible()
  await expect(page.getByText('One malformed source row was rejected.')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Artifacts and citations' })).toBeVisible()
  await page.getByRole('button', { name: 'Inspect citation canonical records' }).click()
  await expect(page.getByLabel('Selected artifact citation')).toContainText('nexusai://evidence/evidence-1/artifacts/artifact-1')
  await expect(page.getByLabel('Selected artifact citation')).toContainText('"row_start":1')
  await expect(page.getByRole('heading', { name: 'Canonical runs and events' })).toBeVisible()
  const custody = page.getByRole('region', { name: 'Append-only custody record' })
  await expect(custody).toContainText('Hash chain verified')
  await expect(custody).toContainText('Source registered')
  await expect(page.getByRole('region', { name: 'Reprocess approval review' })).toContainText('Eligible for approval review')
  await expect(page.getByRole('region', { name: 'Reprocess approval review' })).toContainText('Disabled here')
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1)

  await page.setViewportSize({ width: 390, height: 844 })
  await page.waitForTimeout(350)
  await expect(page.getByLabel('Selected artifact citation')).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1)
})

test('presents bounded image admission metadata without claiming OCR or detection', async ({ page }) => {
  await mockCaseWorkspace(page)
  await page.goto('/app/cases/records-demo-verified/evidence?evidence=evidence-image')

  const admission = page.getByRole('region', { name: 'Immutable image metadata' })
  await expect(admission).toBeVisible()
  await expect(admission).toContainText('Accepted Metadata Only')
  await expect(admission).toContainText('1,920 × 1,080 px')
  await expect(admission).toContainText('YCbCr')
  await expect(admission).toContainText('No OCR or detection claimed')
  await expect(admission).toContainText('EXIF free text and GPS are not surfaced')
  const regions = page.getByRole('region', { name: 'Plate-region candidates' })
  await expect(regions).toContainText('Candidates are not proof')
  await expect(regions).toContainText('plate-region-1')
  await expect(regions).toContainText('430,610 · 380×120')
  await expect(page.getByRole('button', { name: 'Inspect plate-region candidate plate-region-1' })).toBeVisible()
  await expect(regions).toContainText('append-only authorized review event')
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1)

  await page.setViewportSize({ width: 390, height: 844 })
  await page.waitForTimeout(350)
  await expect(regions).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1)
})

test('opens an exact evidence item from a citation deep link', async ({ page }) => {
  await mockCaseWorkspace(page)
  await page.goto('/app/cases/records-demo-verified/evidence?evidence=evidence-1')

  await expect(page).toHaveURL(/\/app\/cases\/records-demo-verified\/evidence\?evidence=evidence-1$/)
  await expect(page.getByRole('heading', { name: 'calls.csv' })).toBeVisible()
  await expect(page.getByText('forensic://source/evidence-1')).toBeVisible()
})

test('keeps governed reprocess review read-only and never posts a queue action', async ({ page }) => {
  const reprocessPosts = []
  await mockCaseWorkspace(page)
  page.on('request', request => {
    if (request.method() === 'POST' && request.url().includes('/reprocess')) reprocessPosts.push(request.url())
  })
  await page.goto('/app/cases/records-demo-verified/evidence')
  await page.getByRole('button', { name: 'Inspect calls.csv' }).click()
  const review = page.getByRole('region', { name: 'Reprocess approval review' })
  await expect(review).toContainText('read-only governance review')
  await expect(review).toContainText('Not Granted')
  await expect(review.getByRole('button')).toHaveCount(0)
  expect(reprocessPosts).toEqual([])
})

test('paginates evidence, exposes queue state, and retains partial intake failures', async ({ page }) => {
  let uploadCalls = 0
  await mockCaseWorkspace(page)
  await page.route('**/api/agents/collections/records-demo-verified/upload**', route => {
    uploadCalls += 1
    if (uploadCalls === 2) return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ status: 'ok', evidence_status: 'failed', records_warning: 'Forensic registration rejected by fixture' }) })
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ status: 'queued', job_id: `job-${uploadCalls}` }) })
  })
  await page.goto('/app/cases/records-demo-verified/evidence')

  await expect(page.getByText('Queue observability', { exact: true })).toBeVisible()
  await expect(page.getByLabel('Recent queue jobs')).toContainText('waiting.csv')
  await expect(page.getByText('1 returned; 1-1 of 2', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Next' }).click()
  await expect(page.getByText('Page 2; 25 per page', { exact: true })).toBeVisible()
  await expect(page.getByText('tower.csv')).toBeVisible()
  await page.getByRole('button', { name: 'Previous' }).click()
  await expect(page.getByText('calls.csv')).toBeVisible()

  await page.getByText('Evidence intake', { exact: true }).click()
  await page.locator('input[type=file]').setInputFiles([
    { name: 'alpha.csv', mimeType: 'text/csv', buffer: Buffer.from('a,b\\n1,2\\n') },
    { name: 'beta.csv', mimeType: 'text/csv', buffer: Buffer.from('invalid') },
    { name: 'gamma.csv', mimeType: 'text/csv', buffer: Buffer.from('a,b\\n3,4\\n') },
  ])
  await page.getByRole('button', { name: 'Confirm registration' }).click()
  await expect(page.getByLabel('Evidence intake results')).toContainText('Forensic registration rejected by fixture')
  await expect(page.getByLabel('Evidence intake results')).toContainText('alpha.csv')
  await expect(page.getByLabel('Evidence intake results')).toContainText('gamma.csv')
  await expect(page.getByText('Evidence intake finished: 2 registered, 1 failed', { exact: true })).toBeVisible()
  expect(uploadCalls).toBe(3)
})

test('presents the complete eight-module case desk with truthful timeline, media, and admin boundaries', async ({ page }) => {
  await mockCaseWorkspace(page)
  await page.goto('/app/cases/records-demo-verified/overview')

  const navigation = page.getByRole('navigation', { name: 'Case workspace sections' })
  for (const module of ['Overview', 'Ask', 'Evidence', 'Relationships', 'Timeline', 'Media', 'Reports', 'Admin']) {
    await expect(navigation.getByRole('link', { name: module })).toBeVisible()
  }
  await expect(navigation.getByRole('link')).toHaveCount(8)
  await expect(page.getByLabel('Authorized case scope')).toContainText('records-demo-verified')

  await navigation.getByRole('link', { name: 'Timeline' }).click()
  await expect(page).toHaveURL(/\/app\/cases\/records-demo-verified\/timeline$/)
  await expect(page.getByRole('heading', { name: 'Chronology grounded in recorded events' })).toBeVisible()
  await expect(page.getByText('No inferred chronology')).toBeVisible()

  await page.getByRole('navigation', { name: 'Case workspace sections' }).getByRole('link', { name: 'Media' }).click()
  await expect(page).toHaveURL(/\/app\/cases\/records-demo-verified\/media$/)
  await expect(page.getByRole('heading', { name: 'Registered media inventory' })).toBeVisible()
  await expect(page.getByText('Truthful capability boundary')).toBeVisible()
  await expect(page.getByText('No registered media evidence')).toBeVisible()

  await page.getByRole('navigation', { name: 'Case workspace sections' }).getByRole('link', { name: 'Admin' }).click()
  await expect(page).toHaveURL(/\/app\/cases\/records-demo-verified\/admin$/)
  await expect(page.getByRole('heading', { name: 'Processing, policy, and resource posture' })).toBeVisible()
  await page.getByText('Advanced case controls').click()
  await expect(page.getByText('Destructive cleanup requires approval')).toBeVisible()
})

test('preserves legacy case links through explicit module redirects without losing query context', async ({ page }) => {
  await mockCaseWorkspace(page)

  await page.goto('/app/cases/records-demo-verified/analyze?prompt=show%20case%20readiness')
  await expect(page).toHaveURL(/\/app\/cases\/records-demo-verified\/ask\?prompt=show%20case%20readiness$/)
  await expect(page.getByRole('heading', { name: 'Ask', exact: true })).toBeVisible()

  await page.goto('/app/cases/records-demo-verified/jobs')
  await expect(page).toHaveURL(/\/app\/cases\/records-demo-verified\/admin$/)

  await page.goto('/app/cases/records-demo-verified/settings')
  await expect(page).toHaveURL(/\/app\/cases\/records-demo-verified\/admin$/)
})

test('generates and exports an on-demand report with the authorized case scope locked', async ({ page }) => {
  await mockCaseWorkspace(page)
  let reportBody
  await page.route('**/api/v1/forensics/cases/records-demo-verified/reports', async route => {
    reportBody = route.request().postDataJSON()
    await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
      collection_id: 'records-demo-verified',
      target: 'subscriber-42',
      markdown: '## Forensic Intelligence Report\n\n- Collection ID: records-demo-verified\n- Accuracy Guardrail: deterministic sources only.',
      sections: { overview: {}, data_quality: {} },
      warnings: [],
      generated_at: '2026-08-07T10:00:00Z',
    }) })
  })

  await page.goto('/app/cases/records-demo-verified/reports')
  await expect(page.getByRole('heading', { name: 'Deterministic forensic intelligence report' })).toBeVisible()
  await expect(page.getByLabel('Locked report scope')).toContainText('records-demo-verified')
  await expect(page.getByText('Not retained', { exact: true })).toBeVisible()
  await page.getByLabel(/Target or subject/).fill('subscriber-42')
  await page.getByRole('button', { name: /Generate report/ }).click()

  await expect.poll(() => reportBody).toEqual({
    target: 'subscriber-42',
    include_evidence: true,
    case_id: 'records-demo-verified',
    collection_id: 'records-demo-verified',
  })
  await expect(page.getByRole('heading', { name: 'Report preview' })).toBeVisible()
  await expect(page.locator('.case-report-preview')).toContainText('Accuracy Guardrail')

  const downloadPromise = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Export Markdown' }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toBe('records-demo-verified-forensic-report.md')
})

test('fails closed before issuing case-specific requests for an inaccessible URL case', async ({ page }) => {
  await mockCaseWorkspace(page)
  const caseSpecificRequests = []
  page.on('request', request => {
    const url = request.url()
    if (url.includes('/api/v1/forensics/cases/case-rogue') || url.includes('collection_id=case-rogue')) {
      caseSpecificRequests.push(url)
    }
  })

  await page.goto('/app/cases/case-rogue/overview')

  const state = page.locator('[data-state="error"]')
  await expect(state).toBeVisible()
  await expect(state).toContainText('Case case-rogue is not an authorized accessible case.')
  await expect(page.getByRole('region', { name: 'Application context' })).not.toContainText('case-rogue')
  expect(caseSpecificRequests).toHaveLength(0)
})

test('resolves the records default once and carries it into the governed case route', async ({ page }) => {
  let registryRequests = 0
  await mockCaseWorkspace(page, { onCaseList: () => { registryRequests += 1 } })

  await page.goto('/app/records')

  await expect(page).toHaveURL(/\/app\/cases\/records-demo-verified\/ask$/)
  await expect(page.getByRole('region', { name: 'Application context' }).getByLabel('Case context: records-demo-verified')).toBeVisible()
  expect(registryRequests).toBe(1)
})

test('clears prior-case results while a newly selected case is loading', async ({ page }) => {
  await mockCaseWorkspace(page)
  let releaseNextCase
  const nextCaseGate = new Promise(resolve => { releaseNextCase = resolve })

  await page.route('**/api/v1/forensics/cases/records-demo', async route => {
    await nextCaseGate
    await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
      case_id: 'records-demo', collection_id: 'records-demo', display_name: 'records-demo', case_status: 'active', security_classification: 'internal',
    }) })
  })
  await page.route('**/api/v1/forensics/cases/records-demo/manifest', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
    resources: [{ resource: 'canonical_records', count: 4, status: 'counted', source: 'forensic.records' }], job_statuses: [],
  }) }))
  await page.route('**/api/v1/forensics/cases/records-demo/evidence**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ count: 0, items: [] }) }))
  await page.route('**/api/records/forensic/status**', route => {
    const selected = new URL(route.request().url()).searchParams.get('collection_id')
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({
      summary: { evidence_total: 0, evidence_in_flight: 0, accepted_rows: selected === 'records-demo' ? 4 : 18192, rejected_rows: 0, kb_assets_total: 0 },
      record_families: selected === 'records-demo' ? [{ record_type: 'subscriber', total_rows: 4 }] : [{ record_type: 'cdr', total_rows: 18192 }],
    }) })
  })
  await page.route('**/api/records/forensic/capabilities**', route => {
    const selected = new URL(route.request().url()).searchParams.get('collection_id')
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify({
      families: selected === 'records-demo'
        ? [{ id: 'subscriber', label: 'Subscriber identity', available: true, support_level: 'operational', indexed_records: 4 }]
        : [{ id: 'cdr', label: 'Call detail records', available: true, support_level: 'operational', indexed_records: 18192 }],
    }) })
  })

  await page.goto('/app/cases/records-demo-verified/overview')
  await expect(page.getByText('Call detail records')).toBeVisible()

  await page.getByLabel('Active collection').selectOption('records-demo')
  await expect(page).toHaveURL(/\/app\/cases\/records-demo\/overview$/)
  await expect(page.getByRole('heading', { name: 'records-demo' })).toBeVisible()
  await expect(page.getByText('Call detail records')).toHaveCount(0)
  await expect(page.getByRole('status').filter({ hasText: 'Verifying queryable family coverage' })).toBeVisible()

  releaseNextCase()
  await expect(page.getByText('Subscriber identity')).toBeVisible()
  await expect(page.getByText('Call detail records')).toHaveCount(0)
})
