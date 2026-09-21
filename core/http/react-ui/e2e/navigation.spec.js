import { test, expect } from './coverage-fixtures.js'

test.describe('Navigation', () => {
  test('/ redirects to the ordinary analyst portal', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveURL(/\/analyst\/home/)
  })

  test('/app shows the home page', async ({ page }) => {
    await page.goto('/app')
    await expect(page.locator('.sidebar')).toBeVisible()
    await expect(page.locator('.home-page')).toBeVisible()
  })

  test('top menu exposes Home and the administrator Model Library', async ({ page }) => {
    await page.goto('/app')
    await expect(page.locator('.sidebar-nav a.nav-item[href="/app"]')).toBeVisible()
    await expect(page.locator('.sidebar-nav a.nav-item[href="/app/models"]')).toBeVisible()
  })

  test('Analyst Tools stays an inline tier with Chat, Studio and Talk', async ({ page }) => {
    await page.goto('/app')
    await expect(page.locator('.sidebar-section-title', { hasText: 'Analyst Tools' })).toBeVisible()
    await expect(page.locator('.sidebar-nav a.nav-item[href="/app/chat"]')).toBeVisible()
    await expect(page.locator('.sidebar-nav a.nav-item[href="/app/studio"]')).toBeVisible()
    await expect(page.locator('.sidebar-nav a.nav-item[href="/app/talk"]')).toBeVisible()
  })

  test('Intelligence Tools opens its capability-aware console', async ({ page }) => {
    await page.goto('/app')
    const build = page.locator('.sidebar-nav a.nav-item', { hasText: 'Intelligence Tools' })
    await expect(build).toBeVisible()
    await build.click()
    await expect(page.locator('.console-rail .console-rail-header', { hasText: 'Intelligence Tools' })).toBeVisible()
  })

  test('System Administration is a single administrator entry', async ({ page }) => {
    await page.goto('/app')
    const operate = page.locator('.sidebar-nav a.nav-item', { hasText: 'System Administration' })
    await expect(operate).toBeVisible()
    await operate.click()
    await expect(page.locator('.console-rail .console-rail-header', { hasText: 'System Administration' })).toBeVisible()
  })

  test('Build console groups Automation, Training and Recognition', async ({ page }) => {
    await page.goto('/app/agents')
    const rail = page.locator('.console-rail')
    await expect(rail).toBeVisible()
    for (const group of ['Automation', 'Training', 'Recognition']) {
      await expect(rail.locator('.console-group-title', { hasText: group })).toBeVisible()
    }
    // Recognition (Faces/Voices) and Training (Fine-tune/Quantize) live here now.
    await expect(rail.locator('a.nav-item[href="/app/fine-tune"]')).toBeVisible()
    await expect(rail.locator('a.nav-item[href="/app/face"]')).toBeVisible()
  })

  test('390px drawer exposes a named navigation and returns focus to its trigger', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/app')

    const menuButton = page.getByRole('button', { name: 'Open menu' })
    await menuButton.click()
    const navigation = page.getByRole('complementary', { name: 'Primary navigation' })
    await expect(navigation).toBeVisible()
    await expect(page.getByRole('button', { name: 'Close menu' })).toBeFocused()

    await page.getByRole('button', { name: 'Close menu' }).click()
    await expect(menuButton).toBeFocused()
  })

  for (const width of [390, 820, 1024, 1440]) {
    test(`shell has no page-level horizontal overflow at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 })
      await page.goto('/app')
      await expect(page.locator('.main-content')).toBeVisible()
      const dimensions = await page.evaluate(() => ({
        viewport: document.documentElement.clientWidth,
        content: document.documentElement.scrollWidth,
      }))
      expect(dimensions.content).toBeLessThanOrEqual(dimensions.viewport)
    })
  }
})
