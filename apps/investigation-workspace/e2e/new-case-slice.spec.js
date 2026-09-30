import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/new-case/', import.meta.url))
const caseId = 'operation-falcon-2026'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', caseIds: [] }
  })
  await page.route('**/webhooks/records/upload', route => route.fulfill({ status: 202, contentType: 'application/json', body: JSON.stringify({ evidence_id: 'ev-first-01' }) }))
  await page.route('**/collections/status*', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      collection_id: caseId,
      summary: { evidence_total: 1, evidence_completed: 1, evidence_in_flight: 0, evidence_failed: 0 },
      recent_evidence: [{ evidence_id: 'ev-first-01', source_file: 'first-cdr.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed' }],
      recent_jobs: [],
    }),
  }))
})

test('starts the case only after the first accepted file and locks its identifier', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the deterministic New case interaction proof.')
  await page.goto('/cases/new')
  const identifier = page.getByRole('textbox', { name: 'Case identifier' })
  await identifier.fill(caseId)
  await expect(page.getByText(`Case will use this exact identifier: ${caseId}`)).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Add the first evidence' })).toBeVisible()
  await page.getByLabel('Choose evidence files to add to this case').setInputFiles({ name: 'first-cdr.csv', mimeType: 'text/csv', buffer: Buffer.from('msisdn,call_type\n03001234567,CALL') })
  await expect(page.getByText('Accepted — processing has started')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'The case now exists' })).toBeVisible()
  await expect(identifier).toHaveAttribute('readonly', '')
  await expect(page.getByText(`Case identifier locked after acceptance: ${caseId}`)).toBeVisible()
  await expect(page.getByText('All 1 evidence items have finished processing.')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Open case' })).toBeEnabled()
  await expect(page.locator('.new-case__accepted progress')).toHaveCount(0)
})

test('new case intake is clear in both themes and has no page overflow at 375px', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the deterministic New case review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })
  for (const theme of ['light', 'dark']) {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.goto('/cases/new')
    await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
    await page.getByRole('textbox', { name: 'Case identifier' }).fill(caseId)
    await expect(page.getByRole('button', { name: 'Choose files' })).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `new-case-${theme}-1440.png`), fullPage: true })

    await page.setViewportSize({ width: 375, height: 812 })
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
    await page.screenshot({ path: path.join(reviewDirectory, `new-case-${theme}-375.png`), fullPage: true })
  }
})
