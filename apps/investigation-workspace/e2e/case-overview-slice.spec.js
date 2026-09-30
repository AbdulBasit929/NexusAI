import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/case-overview/', import.meta.url))
const caseId = 'nexusai-forensic-demo'
const status = {
  collection_id: caseId,
  summary: {
    evidence_total: 7,
    evidence_completed: 3,
    evidence_in_flight: 1,
    evidence_failed: 1,
    completed_jobs_missing_kb_asset: 1,
    total_rows: 9100,
    accepted_rows: 8642,
    duplicate_rows: 33,
    rejected_rows: 425,
  },
  record_families: [{ record_type: 'cdr', accepted_rows: 8000 }, { record_type: 'subscriber', accepted_rows: 642 }],
  // Deliberately contradictory bounded sample. The page must use summary totals.
  recent_jobs: [{ accepted_rows: 2, duplicate_rows: 1, rejected_rows: 1 }],
  recent_evidence: [],
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(id => { window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', caseIds: [id], capabilities: { hybrid_query: true } } }, caseId)
  await page.route('**/collections/status*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(status) }))
})

test('case overview separates evidence readiness, quality, and structured-row units', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the case overview contract.')
  await page.goto(`/cases/${caseId}/overview`)
  const readiness = page.locator('.readiness-panel')
  await expect(readiness.getByText('7', { exact: true })).toBeVisible()
  await expect(readiness.getByText('3', { exact: true })).toBeVisible()
  await expect(readiness.getByText('1', { exact: true })).toHaveCount(2)
  await expect(readiness.getByText('2', { exact: true })).toBeVisible()
  await expect(page.getByText('425 structured rows were rejected across the collection.')).toBeVisible()
  await expect(page.getByText('33 duplicate structured rows were excluded across the collection.')).toBeVisible()
  await expect(page.getByText('8,642 accepted rows across the reported record families')).toBeVisible()
  await expect(page.getByText('1 structured rows')).toHaveCount(0)
  await expect(page.getByRole('link', { name: 'Ask about this case' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Add evidence' })).toHaveAttribute('href', `/cases/${caseId}/evidence#add-evidence`)
  await expect(page.locator('.metric-grid')).toHaveCount(0)
})

test('captures the evidence-led overview in both themes and responsive sizes', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns deterministic review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })
  for (const theme of ['light', 'dark']) {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.goto(`/cases/${caseId}/overview`)
    await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
    await expect(page.getByRole('heading', { name: 'Data-quality notes' })).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `case-overview-${theme}-1440.png`), fullPage: true })
    await page.setViewportSize({ width: 390, height: 844 })
    await page.screenshot({ path: path.join(reviewDirectory, `case-overview-${theme}-390.png`), fullPage: true })
  }
})

