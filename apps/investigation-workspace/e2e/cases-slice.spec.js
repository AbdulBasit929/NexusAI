import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/cases/', import.meta.url))
const cases = ['atlas-10', 'atlas-2', 'falcon-review', 'signal-processing', 'restricted-collection']
const summaries = {
  'atlas-10': { evidence_total: 8, evidence_completed: 8, evidence_in_flight: 0, evidence_failed: 0, accepted_rows: 8642 },
  'atlas-2': { evidence_total: 2, evidence_completed: 2, evidence_in_flight: 0, evidence_failed: 0, accepted_rows: 240 },
  'falcon-review': { evidence_total: 3, evidence_completed: 2, evidence_in_flight: 0, evidence_failed: 1, accepted_rows: 400 },
  'signal-processing': { evidence_total: 4, evidence_completed: 2, evidence_in_flight: 2, evidence_failed: 0, accepted_rows: 1200 },
}
const activityAt = {
  'atlas-10': '2026-09-28T09:30:00Z',
  'atlas-2': '2026-09-28T07:15:00Z',
  'falcon-review': '2026-09-28T10:00:00Z',
  'signal-processing': '2026-09-28T09:45:00Z',
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(caseIds => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', caseIds, capabilities: { hybrid_query: true } }
  }, cases)
  await page.route('**/collections/status*', route => {
    const caseId = route.request().headers()['x-forensic-collection-id']
    if (caseId === 'restricted-collection') {
      route.fulfill({ status: 403, contentType: 'application/json', body: '{}' })
      return
    }
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        collection_id: caseId,
        summary: summaries[caseId],
        record_families: caseId === 'atlas-10'
          ? [{ record_type: 'cdr', accepted_rows: 6000 }, { record_type: 'anpr', accepted_rows: 2642 }]
          : caseId === 'signal-processing' ? [{ record_type: 'ipdr', accepted_rows: 1200 }] : [],
        recent_evidence: [{ evidence_id: `${caseId}-source`, source_file: `${caseId}.csv`, updated_at: activityAt[caseId] }],
        recent_jobs: [],
      }),
    })
  })
})

test('cases is a searchable, sortable and truthful configured collection directory', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the directory interaction contract.')
  const errors = []
  page.on('console', message => message.type() === 'error' && errors.push(message.text()))
  await page.goto('/cases')
  await expect(page.getByRole('heading', { name: 'Case directory' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Some statuses are unavailable' })).toBeVisible()
  const table = page.locator('.case-directory__table')
  await expect(table).toBeVisible()
  await expect(table).toHaveAccessibleName(/5 of 5 configured collections/)
  const rows = table.locator('tbody tr')
  await expect(rows).toHaveCount(5)
  await expect(rows.nth(0)).toContainText('falcon-review')
  await expect(rows.nth(1)).toContainText('signal-processing')
  await expect(rows.filter({ hasText: 'atlas-10' }).getByRole('img', { name: /accepted rows by evidence family/i })).toBeVisible()

  const restricted = rows.filter({ hasText: 'restricted-collection' })
  await expect(restricted).toContainText('Access restricted')
  await expect(restricted).toContainText('Outside your access scope')
  await expect(restricted).not.toContainText('0 evidence')
  await expect(restricted.getByRole('link')).toHaveCount(0)

  await page.getByRole('button', { name: /Evidence families.*not sorted/ }).click()
  await expect(table.locator('thead th').nth(2)).toHaveAttribute('aria-sort', 'descending')
  await expect(rows.first()).toContainText('atlas-10')

  await page.getByRole('searchbox', { name: 'Search cases' }).fill('falcon')
  await expect(rows).toHaveCount(1)
  await expect(rows.first()).toContainText('falcon-review')
  await page.getByRole('button', { name: 'Clear' }).click()
  await expect(rows).toHaveCount(5)

  await page.locator('.case-directory__filters button').filter({ hasText: 'Processing' }).click()
  await expect(rows).toHaveCount(1)
  await expect(rows.first()).toContainText('signal-processing')
  await expect(rows.first().getByRole('link', { name: 'View processing' })).toBeVisible()
  await page.getByText('Directory scope').click()
  await expect(page.getByText(/Names, owners, classification and priority are absent/)).toBeVisible()

  await page.getByRole('button', { name: 'Clear' }).click()
  await page.getByRole('button', { name: /Evidence activity.*not sorted/ }).click()
  const firstCase = page.getByRole('link', { name: 'falcon-review' })
  await firstCase.focus()
  await page.keyboard.press('ArrowDown')
  await expect(page.getByRole('link', { name: 'signal-processing' })).toBeFocused()
  await page.keyboard.press('End')
  await expect(page.getByRole('link', { name: 'atlas-2' })).toBeFocused()

  await page.setViewportSize({ width: 1024, height: 900 })
  await expect.poll(() => table.locator('tbody th').first().evaluate(element => getComputedStyle(element).position)).toBe('sticky')

  await page.setViewportSize({ width: 375, height: 812 })
  await expect(page.getByText('Sort', { exact: true })).toBeVisible()
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
  const minTarget = await page.locator('.case-directory__actions a').first().evaluate(element => element.getBoundingClientRect().height)
  expect(minTarget).toBeGreaterThanOrEqual(44)
  const duplicateIds = await page.locator('[id]').evaluateAll(elements => {
    const ids = elements.map(element => element.id)
    return ids.filter((id, index) => ids.indexOf(id) !== index)
  })
  expect(duplicateIds).toEqual([])
  const expectedRestrictedResponses = errors.filter(message => message.includes('403 (Forbidden)'))
  expect(expectedRestrictedResponses).toHaveLength(1)
  expect(errors.filter(message => !message.includes('403 (Forbidden)'))).toEqual([])
})

test('captures the directory in both themes and responsive forms', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns deterministic review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })

  await page.goto('/cases')
  await expect(page.getByRole('heading', { name: 'Some statuses are unavailable' })).toBeVisible()
  const viewports = [
    { width: 1440, height: 1000 },
    { width: 1280, height: 960 },
    { width: 1024, height: 900 },
    { width: 768, height: 900 },
    { width: 375, height: 812 },
  ]
  for (const theme of ['light', 'dark']) {
    await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
    for (const viewport of viewports) {
      await page.setViewportSize(viewport)
      await page.mouse.move(1, 1)
      await page.screenshot({ path: path.join(reviewDirectory, `cases-${theme}-${viewport.width}.png`), fullPage: true })
    }
  }
})
