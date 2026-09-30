import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'
import { liveFixture } from './live-fixtures.js'

const cdr = liveFixture('CDR-04.json')
const outputDirectory = fileURLToPath(new URL('../design-review/wi-ui-5/', import.meta.url))

const overview = {
  collection_id: 'nexusai-forensic-demo',
  summary: { evidence_total: 4, evidence_completed: 3, evidence_in_flight: 1, accepted_rows: 8642 },
  record_families: [{ record_type: 'cdr', accepted_rows: 8642 }],
}

const catalog = {
  summary: { evidence_total: 4, modality_counts: { structured_records: 2, document: 1, audio: 1 } },
  items: [
    { evidence_id: 'doc-1', original_filename: 'witness-statement.txt', source_file: 'witness-statement.txt', modality: 'document', detected_type: 'text', processing_status: 'completed', size_bytes: 4180 },
    { evidence_id: 'ddd78d1b-3edf-4ed2-bc18-945c0a61fc3f', original_filename: 'seed_cdr_large.csv', source_file: 'seed_cdr_large.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', size_bytes: 84000 },
  ],
}

function structuredDetail() {
  return {
    item: { original_filename: 'seed_cdr_large.csv', source_file: 'seed_cdr_large.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', size_bytes: 84000 },
    records_preview: cdr.enterprise.fact_packet.citations.map(citation => ({
      row_number: Number(citation.locator?.row_number),
      call_type: 'CALL',
      direction: 'OUTGOING',
      duration_seconds: 61,
    })),
    derived_artifacts: [],
  }
}

const documentDetail = {
  item: { original_filename: 'witness-statement.txt', source_file: 'witness-statement.txt', modality: 'document', detected_type: 'text', processing_status: 'completed', size_bytes: 4180, metadata: { source_text: 'The witness arrived at 08:42 and identified the vehicle near the north gate.' } },
  records_preview: [],
  derived_artifacts: [],
}

const audioDetail = {
  item: { original_filename: 'interview.wav', source_file: 'interview.wav', modality: 'audio', detected_type: 'audio', processing_status: 'completed', size_bytes: 2048 },
  records_preview: [],
  derived_artifacts: [{ artifact_id: 'transcript-1', artifact_type: 'transcript', confidence: 0.91, metadata: { transcript: 'The interview begins with an account of the north gate.' } }],
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', collectionId: 'nexusai-forensic-demo' }
  })
  await page.route('**/api/collections/status?*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(overview) }))
  await page.route('**/api/evidence?*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(catalog) }))
  await page.route('**/api/evidence/doc-1?*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(documentDetail) }))
  await page.route('**/api/evidence/audio-1?*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(audioDetail) }))
  await page.route('**/api/evidence/*/content?*', route => route.fulfill({ status: 200, contentType: 'text/plain', body: documentDetail.item.metadata.source_text }))
  await page.route('**/api/evidence/ddd78d1b-3edf-4ed2-bc18-945c0a61fc3f?*', route => {
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(structuredDetail()) })
  })
})

test('case overview and evidence catalog use working routes and family chips', async ({ page }) => {
  await page.goto('/cases/nexusai-forensic-demo/overview')
  await expect(page.getByRole('heading', { name: 'Case overview' })).toBeVisible()
  await expect(page.getByLabel('Case context')).toContainText('nexusai-forensic-demo')
  await expect(page.getByRole('definition').filter({ hasText: '8,642accepted rows' }).getByText('8,642', { exact: true })).toBeVisible()
  await page.goto('/cases/nexusai-forensic-demo/evidence')
  const cdrFilter = page.getByRole('button', { name: /CDR/ })
  await expect(cdrFilter).toBeVisible()
  await expect(cdrFilter).toContainText('1')
  await expect(page.getByRole('link', { name: /witness-statement.txt/ })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
})

test('document locator highlights the exact character span', async ({ page }) => {
  await page.goto('/cases/nexusai-forensic-demo/evidence/doc-1?page=1&char_span=%5B23%2C28%5D')
  await expect(page.getByRole('heading', { name: /Document source · page 1/ })).toBeVisible()
  await expect(page.locator('#exact-source')).toHaveText('08:42')
  await expect(page.getByLabel('Evidence strength')).toContainText('Source evidence')
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
})

test('audio locator opens the retained source at the cited offset', async ({ page }) => {
  await page.goto('/cases/nexusai-forensic-demo/evidence/audio-1?source_time=12.5&source_end=18')
  const audio = page.locator('audio')
  await expect(audio).toBeVisible()
  await audio.dispatchEvent('loadedmetadata')
  await expect.poll(() => audio.evaluate(node => node.currentTime)).toBe(12.5)
  await expect(page.getByLabel('Evidence strength')).toContainText('Source evidence')
})

test('follows an aggregate answer marker to the exact structured source row', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic project owns the review artifact.')
  fs.mkdirSync(outputDirectory, { recursive: true })
  await page.route('**/query/hybrid', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(cdr) }))
  await page.goto('/cases/nexusai-forensic-demo/investigate')
  await page.getByLabel('Ask a question about this case').fill('Show the call type breakdown')
  await page.getByRole('button', { name: 'Ask' }).click()
  const marker = page.locator('.citation-marker').first()
  await expect(marker).toBeVisible()
  await marker.click()
  const requestedRow = new URL(page.url()).searchParams.get('row')
  await expect(page.getByRole('heading', { name: 'Exact source location' })).toBeVisible()
  await expect(page.getByLabel('Structured source').getByText(requestedRow, { exact: true })).toBeVisible()
  await expect(page.getByLabel('Structured source').getByText('CALL', { exact: true }).first()).toBeVisible()
  await page.screenshot({ path: path.join(outputDirectory, 'marker-followed-to-exact-source-light-1440.png'), fullPage: true })
})
