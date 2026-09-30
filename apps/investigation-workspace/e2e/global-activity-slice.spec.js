import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/global-activity/', import.meta.url))
const cases = ['operation-falcon', 'harbour-review', 'signal-watch']

test.beforeEach(async ({ page }) => {
  await page.addInitScript(caseIds => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', caseIds, capabilities: { hybrid_query: true } }
  }, cases)
})

test('global Activity remains truthful and provides a case-scoped recovery path', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the global Activity contract.')
  const apiRequests = []
  page.on('request', request => {
    if (new URL(request.url()).pathname.startsWith('/api/')) apiRequests.push(request.url())
  })

  await page.goto('/activity')
  await expect(page.getByRole('heading', { name: 'Workspace-wide audit history is not available' })).toBeVisible()
  await expect(page.getByText(/available total cannot show who acted/i)).toBeVisible()
  const records = page.getByRole('link', { name: 'Open working record' })
  await expect(records).toHaveCount(3)
  await expect(records.first()).toHaveAttribute('href', '/cases/operation-falcon/activity')
  await expect(page.getByText(/Custody events, exports and team-wide viewer activity remain absent/)).toBeVisible()
  expect(apiRequests).toEqual([])

  await page.setViewportSize({ width: 375, height: 812 })
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
  await expect.poll(() => records.first().evaluate(element => element.getBoundingClientRect().height)).toBeGreaterThanOrEqual(44)
})

test('captures the unavailable gateway in both themes', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns deterministic review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })

  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto('/activity')
  await page.evaluate(() => { document.documentElement.dataset.theme = 'light'; localStorage.setItem('nexusai.viewer.theme', 'light') })
  await page.mouse.move(1, 1)
  await page.screenshot({ path: path.join(reviewDirectory, 'global-activity-light-1440.png'), fullPage: true })

  await page.setViewportSize({ width: 375, height: 812 })
  await page.evaluate(() => { document.documentElement.dataset.theme = 'dark'; localStorage.setItem('nexusai.viewer.theme', 'dark') })
  await page.mouse.move(1, 1)
  await page.screenshot({ path: path.join(reviewDirectory, 'global-activity-dark-375.png'), fullPage: true })
})
