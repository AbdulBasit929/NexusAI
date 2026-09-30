import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/ui-redesign-20260929/visual-identity/', import.meta.url))
const viewports = [
  { width: 1440, height: 1000 },
  { width: 1280, height: 960 },
  { width: 1024, height: 900 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
]

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', caseIds: [], capabilities: { hybrid_query: true } }
  })
})

test('gallery controls are keyboard complete and return focus after overlays', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the deterministic interaction pass.')
  const errors = []
  page.on('console', message => message.type() === 'error' && errors.push(message.text()))
  page.on('pageerror', error => errors.push(error.message))

  await page.goto('/design/type-proof')
  await expect(page.getByRole('heading', { name: 'Clarity for evidence-heavy work.' })).toBeVisible()

  const summary = page.getByRole('tab', { name: 'Summary' })
  await summary.focus()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('tab', { name: 'Source' })).toBeFocused()
  await expect(page.getByRole('tab', { name: 'Source' })).toHaveAttribute('aria-selected', 'true')

  const modalTrigger = page.getByRole('button', { name: 'Open modal' })
  await modalTrigger.click()
  await expect(page.getByRole('dialog', { name: 'Confirm a bounded action' })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog', { name: 'Confirm a bounded action' })).toHaveCount(0)
  await expect(modalTrigger).toBeFocused()

  const drawerTrigger = page.getByRole('button', { name: 'Open drawer' })
  await drawerTrigger.click()
  await expect(page.getByRole('dialog', { name: 'Source details' })).toBeVisible()
  await page.keyboard.press('Tab')
  await page.keyboard.press('Tab')
  await expect(page.getByRole('button', { name: 'Close drawer' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(drawerTrigger).toBeFocused()

  await page.getByRole('button', { name: 'View options' }).click()
  await expect(page.getByRole('dialog', { name: 'View options' })).toBeVisible()
  await page.getByRole('button', { name: 'Show toast' }).click()
  await expect(page.getByRole('status').filter({ hasText: 'Preference saved' })).toBeVisible()
  await page.getByRole('button', { name: 'Dismiss notification' }).click()

  await page.setViewportSize({ width: 375, height: 812 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
  const undersizedTargets = await page.locator('button:visible').evaluateAll(buttons => buttons
    .map(button => ({ label: button.getAttribute('aria-label') || button.textContent.trim(), box: button.getBoundingClientRect() }))
    .filter(item => item.box.width < 44 || item.box.height < 44))
  expect(undersizedTargets).toEqual([])
  expect(errors).toEqual([])
})

test('captures Mineral Signal in both themes at every contracted review width', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns deterministic review artifacts.')
  test.setTimeout(180_000)
  fs.mkdirSync(reviewDirectory, { recursive: true })

  await page.goto('/design/type-proof')
  await page.evaluate(() => document.fonts.ready)
  for (const theme of ['light', 'dark']) {
    await page.evaluate(value => {
      document.documentElement.dataset.theme = value
      localStorage.setItem('nexusai.viewer.theme', value)
    }, theme)
    for (const viewport of viewports) {
      await page.setViewportSize(viewport)
      await page.mouse.move(1, 1)
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), `${theme} at ${viewport.width}px`).toBe(0)
      await page.screenshot({ path: path.join(reviewDirectory, `visual-identity-mineral-signal-${theme}-${viewport.width}.png`), fullPage: true })
    }
  }
})
