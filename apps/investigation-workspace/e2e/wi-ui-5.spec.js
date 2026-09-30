import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'
import { liveFixture } from './live-fixtures.js'

const outputDirectory = fileURLToPath(new URL('../design-review/wi-ui-5/', import.meta.url))
const answered = liveFixture('CDR-04.json')
const clarified = liveFixture('CDR-05.json')
const audioAnswer = liveFixture('AUD-02.json')
const activityOverview = {
  collection_id: 'nexusai-forensic-demo',
  recent_evidence: [
    { evidence_id: 'e82e2f89-ee26-4787-8f8a-2940db6ad61d', source_file: '923461678183.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', created_at: '2026-08-20T07:25:24Z', updated_at: '2026-08-20T07:25:25Z' },
    { evidence_id: '1288b639-2ee7-40d8-b183-a34b0a583135', source_file: 'r5_live_intake_acceptance_20260810.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'failed', created_at: '2026-08-10T05:35:03Z', updated_at: '2026-08-10T05:35:04Z' },
  ],
  recent_jobs: [
    { evidence_id: 'e82e2f89-ee26-4787-8f8a-2940db6ad61d', job_id: 'c9a39765-8a2c-4ecd-9116-e046bfa6404c', status: 'completed', attempt_count: 1, accepted_rows: 3634, rejected_rows: 0, duplicate_rows: 297 },
    { evidence_id: '1288b639-2ee7-40d8-b183-a34b0a583135', job_id: 'ee187a54-ecfd-44e6-85e2-25a5dd7fbb7f', status: 'dead_letter', attempt_count: 1, accepted_rows: 0, rejected_rows: 0, duplicate_rows: 0 },
  ],
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default' }
  })
  await page.route(/http:\/\/localhost:8091\/collections\/status.*/, route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(activityOverview) }))
})

test('Activity and Timeline fit the supported viewport without horizontal overflow', async ({ page }) => {
  for (const route of ['activity', 'timeline']) {
    await page.goto(`/cases/nexusai-forensic-demo/${route}`)
    await expect(page.locator('#workspace-main')).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
  }
})

test('records answered and clarification outcomes in current-session activity', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic project owns the review artifact.')
  fs.mkdirSync(outputDirectory, { recursive: true })
  const responses = new Map([
    ['Show the call type breakdown', answered],
    ['How many calls of each type are there?', clarified],
  ])
  await page.route('**/query/hybrid', async route => {
    const query = JSON.parse(route.request().postData() || '{}').query
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(responses.get(query)) })
  })
  await page.goto('/cases/nexusai-forensic-demo/investigate')
  const field = page.getByLabel('Ask a question about this case')
  for (const question of responses.keys()) {
    await field.fill(question)
    await page.getByRole('button', { name: 'Ask', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Ask', exact: true })).toBeEnabled()
  }
  await page.getByRole('link', { name: 'Activity', exact: true }).last().click()
  await expect(page.getByRole('list').getByText('Answered', { exact: true }).first()).toBeVisible()
  await expect(page.getByRole('list').getByText('Clarification requested', { exact: true })).toBeVisible()
  await expect(page.getByText('nexusai-forensic-demo', { exact: true }).first()).toBeVisible()
  const reopen = page.getByRole('link', { name: 'Reopen with this question' }).first()
  await expect(reopen).toHaveAttribute('href', /question=/)
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
  await page.screenshot({ path: path.join(outputDirectory, 'activity-current-session-light-1440.png'), fullPage: true })
})

test('shows an honest timeline dependency instead of inferred events', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic project owns the review artifact.')
  fs.mkdirSync(outputDirectory, { recursive: true })
  await page.goto('/cases/nexusai-forensic-demo/timeline')
  await expect(page.getByRole('heading', { name: 'A reliable source chronology is not available' })).toBeVisible()
  await expect(page.getByLabel('Timeline filters')).toHaveCount(0)
  await expect(page.locator('select:disabled, input:disabled')).toHaveCount(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
  await page.screenshot({ path: path.join(outputDirectory, 'timeline-dependency-light-1440.png'), fullPage: true })
})

test('follows a refreshed live audio citation to the exact offset', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic project owns the review artifact.')
  fs.mkdirSync(outputDirectory, { recursive: true })
  const citation = audioAnswer.enterprise.fact_packet.citations[0]
  const evidenceId = citation.evidence_id
  const filename = citation.source_file
  await page.route('**/query/hybrid', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(audioAnswer) }))
  await page.route(new RegExp(`http://localhost:8091/evidence/${evidenceId}\\?.*`), route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      item: { evidence_id: evidenceId, original_filename: filename, source_file: filename, modality: 'audio', detected_type: 'audio', processing_status: 'completed', size_bytes: 0 },
      records_preview: [],
      derived_artifacts: [{ artifact_type: 'transcript', confidence: 0.9, metadata: { transcript: 'especially Japanese coconut sugar, and various aromatic spices.' } }],
    }),
  }))
  await page.route(new RegExp(`http://localhost:8091/evidence/${evidenceId}/content.*`), route => route.fulfill({ status: 200, contentType: 'audio/wav', body: '' }))
  await page.goto('/cases/nexusai-multimodal-product-acceptance/investigate')
  await page.getByLabel('Ask a question about this case').fill(audioAnswer.query_understanding.original_question)
  await page.getByRole('button', { name: 'Ask', exact: true }).click()
  await page.locator('.citation-marker').first().click()
  await expect(page.getByRole('heading', { name: 'Exact source location' })).toBeVisible()
  await expect(page.getByText(`${citation.locator.start_seconds} seconds`, { exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: filename })).toBeVisible()
  await page.screenshot({ path: path.join(outputDirectory, 'audio-citation-followed-to-offset-light-1440.png'), fullPage: true })
})
