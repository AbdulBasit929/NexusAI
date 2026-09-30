import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/timeline/', import.meta.url))
const caseId = 'nexusai-forensic-demo'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(id => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', collectionId: id, caseIds: [id] }
  }, caseId)
  await page.route('**/collections/status*', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      collection_id: caseId,
      summary: { evidence_total: 6, evidence_completed: 5, evidence_in_flight: 1, evidence_failed: 0 },
      record_families: [{ record_type: 'cdr' }, { record_type: 'anpr' }],
      recent_evidence: [],
      recent_jobs: [],
    }),
  }))
  await page.route('**/query/capabilities*', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      deployment_attestation: 'INTERNAL VALUE MUST NOT RENDER',
      families: [
        { id: 'cdr', label: 'CDR', availability: 'queryable', record_types: ['cdr'], adapter: 'internal-cdr-adapter', suggested_queries: ['show communications timeline', 'count calls'] },
        { id: 'anpr', label: 'ANPR', availability: 'queryable', record_types: ['anpr'], adapter: 'internal-anpr-adapter', suggested_queries: ['show camera sequence'] },
        { id: 'ipdr', label: 'IPDR', availability: 'no_data', record_types: ['ipdr'], suggested_queries: ['show network session timeline'] },
      ],
    }),
  }))
})

test('withholds a fake cross-family timeline and offers only curated supported paths', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the deterministic Timeline interaction proof.')
  await page.goto(`/cases/${caseId}/timeline`)
  await expect(page.getByRole('heading', { name: 'A reliable source chronology is not available' })).toBeVisible()
  await expect(page.getByText('6', { exact: true })).toBeVisible()
  await expect(page.getByText('CDR · ANPR', { exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Ask a narrower chronology question' })).toBeVisible()
  const cdrQuestion = page.getByRole('link', { name: 'show communications timeline' })
  await expect(cdrQuestion).toHaveAttribute('href', /question=show\+communications\+timeline/)
  await expect(cdrQuestion).toHaveAttribute('href', /scope=cdr/)
  await expect(page.getByRole('link', { name: 'show camera sequence' })).toHaveAttribute('href', /scope=anpr/)
  await expect(page.getByText('show network session timeline')).toHaveCount(0)
  await expect(page.getByText('INTERNAL VALUE MUST NOT RENDER')).toHaveCount(0)
  await expect(page.getByText('internal-cdr-adapter')).toHaveCount(0)
  await expect(page.locator('select:disabled, input:disabled, button:disabled')).toHaveCount(0)
})

test('timeline unavailable state is composed and overflow-free in both themes', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the deterministic Timeline review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })
  for (const theme of ['light', 'dark']) {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.goto(`/cases/${caseId}/timeline`)
    await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
    await expect(page.getByRole('heading', { name: 'What a trustworthy timeline needs' })).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `timeline-${theme}-1440.png`), fullPage: true })

    await page.setViewportSize({ width: 375, height: 812 })
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
    await page.screenshot({ path: path.join(reviewDirectory, `timeline-${theme}-375.png`), fullPage: true })
  }
})
