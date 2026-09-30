import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/activity/', import.meta.url))
const caseId = 'nexusai-forensic-demo'
const overview = {
  collection_id: caseId,
  summary: { evidence_total: 3, evidence_completed: 1, evidence_in_flight: 1, evidence_failed: 1 },
  recent_evidence: [
    { evidence_id: 'ev-cdr-01', source_file: 'network-export-01.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', created_at: '2026-09-27T08:10:00Z', updated_at: '2026-09-27T08:11:00Z' },
    { evidence_id: 'ev-anpr-01', source_file: 'checkpoint-west.csv', modality: 'structured_records', detected_type: 'anpr', processing_status: 'processing', created_at: '2026-09-27T08:03:00Z', updated_at: '2026-09-27T08:04:00Z' },
    { evidence_id: 'ev-doc-01', source_file: 'interview-notes.pdf', modality: 'document', detected_type: 'document', processing_status: 'failed', created_at: '2026-09-27T07:35:00Z', updated_at: '2026-09-27T07:36:00Z' },
  ],
  recent_jobs: [
    { evidence_id: 'ev-cdr-01', job_id: 'job-01', status: 'completed', attempt_count: 1, accepted_rows: 8642, rejected_rows: 3, duplicate_rows: 18 },
    { evidence_id: 'ev-anpr-01', job_id: 'job-02', status: 'processing', attempt_count: 1 },
    { evidence_id: 'ev-doc-01', job_id: 'job-03', status: 'failed', attempt_count: 1 },
  ],
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(id => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', collectionId: id, caseIds: [id] }
    localStorage.setItem(`nexusai.viewer.questions.${id}`, JSON.stringify([
      { id: 'q-1', query: 'Which call types occur most often?', answer: 'Data sessions occur most often.', state: 'answered', pinned: true, updatedAt: '2026-09-27T09:15:00Z' },
      { id: 'q-2', query: 'Compare subscriber and vehicle activity', answer: 'Choose which subscriber identifier to compare.', state: 'clarify', pinned: false, updatedAt: '2026-09-27T09:00:00Z' },
    ]))
  }, caseId)
  await page.route('**/collections/status*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(overview) }))
})

test('activity is a searchable working record with explicit source boundaries', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the deterministic Activity slice interaction proof.')
  await page.goto(`/cases/${caseId}/activity`)
  await expect(page.getByRole('heading', { name: 'Working record, not audit history' })).toBeVisible()
  await expect(page.getByText('Saved in this browser').first()).toBeVisible()
  await expect(page.getByText('Recent collection status').first()).toBeVisible()
  await expect(page.getByText('Not retained with this saved entry').first()).toBeVisible()
  await expect(page.getByText('Technical details')).toHaveCount(0)
  await expect(page.getByText('Request reference')).toHaveCount(0)

  await page.getByRole('searchbox', { name: 'Search activity' }).fill('checkpoint-west')
  await expect(page.getByRole('list', { name: 'Activity' }).getByRole('listitem')).toHaveCount(1)
  await expect(page.getByText('checkpoint-west.csv')).toBeVisible()
  await page.getByRole('button', { name: 'Clear filters' }).click()

  await page.getByRole('button', { name: 'Source All' }).click()
  await page.getByRole('option', { name: 'Questions' }).click()
  await expect(page.getByRole('list', { name: 'Activity' }).getByRole('listitem')).toHaveCount(2)
  await expect(page.getByText('Recent collection status')).toHaveCount(0)
})

test('activity review captures remain legible in both themes and at 375px', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the deterministic Activity slice review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })
  for (const theme of ['light', 'dark']) {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.goto(`/cases/${caseId}/activity`)
    await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
    await expect(page.getByText('Recent collection status').first()).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `activity-${theme}-1440.png`), fullPage: true })

    await page.setViewportSize({ width: 375, height: 812 })
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
    await page.screenshot({ path: path.join(reviewDirectory, `activity-${theme}-375.png`), fullPage: true })
  }
})
