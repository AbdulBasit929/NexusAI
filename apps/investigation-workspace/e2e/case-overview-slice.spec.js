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
  await page.route('**/query/capabilities*', route => route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }))
  await page.route('**/query/hybrid', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: { activity_by_day: [{ activity_date: '2026-03-01T00:00:00Z', record_type: 'cdr', event_count: 40 }, { activity_date: '2026-03-02T00:00:00Z', record_type: 'subscriber', event_count: 12 }] } }) }))
})

test('case overview separates evidence readiness, quality, and structured-row units', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the case overview contract.')
  await page.goto(`/cases/${caseId}/overview`)
  // A briefing: one verdict, the readiness ring with every count spelled out, and what to do next.
  await expect(page.getByText('2 sources need review')).toBeVisible()
  const legend = page.getByRole('list', { name: 'Evidence readiness' })
  await expect(legend).toContainText('Ready3')
  await expect(legend).toContainText('Processing1')
  await expect(legend).toContainText('Failed1')
  await expect(page.locator('#case-next').getByRole('link', { name: /Review 1 failed source/ })).toHaveAttribute('href', `/cases/${caseId}/evidence?status=failed`)
  await expect(page.locator('#case-families').getByRole('link', { name: /Call detail records/ })).toHaveAttribute('href', `/cases/${caseId}/evidence?family=cdr`)
  await expect(page.locator('#case-activity').getByRole('img', { name: /Activity by day, .* 52 events on 2 days/ })).toBeVisible()
  await expect(page.locator('#case-attention')).toContainText('Failed, cause not listed')
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

