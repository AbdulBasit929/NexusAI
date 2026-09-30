import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/header-context/', import.meta.url))
const caseA = 'nexusai-forensic-demo'
const caseB = 'nexusai-multimodal-product-acceptance'
const status = {
  collection_id: caseA,
  summary: { evidence_total: 4, evidence_completed: 3, evidence_in_flight: 1, evidence_failed: 0, accepted_rows: 8642 },
  record_families: [{ record_type: 'cdr', accepted_rows: 8642 }],
  recent_jobs: [],
  recent_evidence: [],
}
const catalog = {
  summary: { evidence_total: 2 },
  items: [
    { evidence_id: 'ev-1', original_filename: 'calls-september.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', size_bytes: 12000 },
    { evidence_id: 'ev-2', original_filename: 'interview-notes.pdf', modality: 'document', detected_type: 'document', processing_status: 'completed', size_bytes: 8000 },
  ],
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(({ first, second }) => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', caseIds: [first, second], capabilities: { hybrid_query: true } }
    localStorage.setItem(`nexusai.viewer.questions.${first}`, JSON.stringify([{ id: 'q-1', query: 'Who called most often?', label: 'Priority caller', state: 'answered', pinned: true, updatedAt: '2026-09-27T10:00:00Z' }]))
  }, { first: caseA, second: caseB })
  await page.route('**/collections/status*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(status) }))
  await page.route('**/evidence?*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(catalog) }))
})

test('global, case, and task bands remain distinct and Quick find states its scope', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the header/context contract.')
  await page.goto('/')
  await expect(page.locator('.case-context-band')).toHaveCount(0)
  await expect(page.locator('.page-header')).toHaveCount(1)

  await page.goto(`/cases/${caseA}/overview`)
  const context = page.locator('.case-context-band')
  await expect(context.getByText(caseA, { exact: true })).toBeVisible()
  await expect(context.getByText('Evidence processing', { exact: true })).toBeVisible()
  await expect(context.getByText('3 ready · 1 processing', { exact: true })).toBeVisible()
  await expect(page.locator('.page-header h1')).toHaveCount(1)

  const trigger = page.getByRole('button', { name: /Quick find/ })
  await trigger.click()
  const dialog = page.getByRole('dialog', { name: 'Quick find' })
  await expect(dialog).toBeVisible()
  for (const heading of ['Routes', 'Cases', 'Evidence', 'Recent questions']) await expect(dialog.getByRole('heading', { name: heading, exact: true })).toBeVisible()
  await expect(dialog.getByText('Configured collections only', { exact: true })).toBeVisible()
  await expect(dialog.getByText('Up to 100 items returned for the active case', { exact: true })).toBeVisible()
  await expect(dialog.getByText('This browser only', { exact: true })).toBeVisible()
  await expect(dialog.getByText('calls-september.csv', { exact: true })).toBeVisible()
  await expect(dialog.getByText('Priority caller', { exact: true })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(trigger).toBeFocused()

  await page.evaluate(id => sessionStorage.setItem(`nexusai.case.${id}.scope.evidence`, 'cdr'), caseA)
  await trigger.click()
  await dialog.getByRole('combobox').fill(caseB)
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(new RegExp(`/cases/${caseB}/overview$`))
  expect(await page.evaluate(() => sessionStorage.length)).toBe(0)
})

test('captures the header, authoritative context, and bounded Quick find in both themes', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns deterministic review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })
  for (const theme of ['light', 'dark']) {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.goto(`/cases/${caseA}/overview`)
    await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
    await expect(page.getByText('Evidence processing', { exact: true })).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `bands-${theme}-1440.png`), fullPage: true })
    await page.getByRole('button', { name: /Quick find/ }).click()
    await expect(page.getByRole('dialog', { name: 'Quick find' }).getByText('calls-september.csv', { exact: true })).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `quick-find-${theme}-1440.png`), fullPage: true })
    await page.keyboard.press('Escape')

    await page.setViewportSize({ width: 390, height: 844 })
    await expect(page.locator('.case-context-band')).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `bands-${theme}-390.png`), fullPage: true })
  }
})
