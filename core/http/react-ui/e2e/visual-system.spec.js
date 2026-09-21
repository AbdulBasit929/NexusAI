import { test, expect } from './coverage-fixtures.js'

test.describe('NexusAI R3.1 visual system', () => {
  test('applies the command-system tokens and enterprise shell dimensions', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.addInitScript(() => localStorage.setItem('localai-theme', 'dark'))
    await page.goto('/app')
    await expect(page.locator('.sidebar')).toBeVisible()
    await expect(page.locator('.app-shell-header')).toBeVisible()

    const metrics = await page.evaluate(() => {
      const root = getComputedStyle(document.documentElement)
      const sidebar = document.querySelector('.sidebar').getBoundingClientRect()
      const header = document.querySelector('.app-shell-header').getBoundingClientRect()
      return {
        action: root.getPropertyValue('--nx-action').trim(),
        accent: root.getPropertyValue('--nx-accent').trim(),
        sidebar: Math.round(sidebar.width),
        header: Math.round(header.height),
      }
    })

    expect(metrics).toEqual({ action: '#2563eb', accent: '#14b8a6', sidebar: 248, header: 64 })
    await expect(page.getByRole('link', { name: /Home/ }).first()).toHaveClass(/active/)
  })

  test('persists theme and collapsed-navigation preferences', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/app')

    const initialTheme = await page.locator('html').getAttribute('data-theme')
    await page.locator('.sidebar .theme-toggle:not(.language-switcher-trigger)').click()
    const changedTheme = initialTheme === 'dark' ? 'light' : 'dark'
    await expect(page.locator('html')).toHaveAttribute('data-theme', changedTheme)

    await page.locator('.sidebar-collapse-btn').click()
    await expect(page.locator('.sidebar')).toHaveClass(/collapsed/)
    await page.reload()
    await expect(page.locator('html')).toHaveAttribute('data-theme', changedTheme)
    await expect(page.locator('.sidebar')).toHaveClass(/collapsed/)
  })

  test('keeps standard controls readable and focus-visible', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/app')

    const primary = page.getByRole('button', { name: /Browse Gallery/i })
    await expect(primary).toBeVisible()
    const box = await primary.boundingBox()
    expect(box.height).toBeGreaterThanOrEqual(40)
    await primary.focus()
    const ring = await primary.evaluate(el => getComputedStyle(el).boxShadow)
    expect(ring).not.toBe('none')
  })

  test('contains dense data and dialog surfaces at mobile width', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/app/models')
    await expect(page.locator('body')).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy()

    await page.goto('/app')
    const stop = page.getByRole('button', { name: 'Stop model' }).first()
    if (await stop.isVisible().catch(() => false)) {
      await stop.click()
      const dialog = page.getByRole('dialog')
      await expect(dialog).toBeVisible()
      const dialogBox = await dialog.boundingBox()
      expect(dialogBox.width).toBeLessThanOrEqual(358)
    }
  })
})
