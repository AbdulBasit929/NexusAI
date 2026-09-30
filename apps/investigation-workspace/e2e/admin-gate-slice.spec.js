import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: [], capabilities: { hybrid_query: true, admin: true } }
  })
})

test('keeps Admin absent without a server-authoritative identity and authorization decision', async ({ page }) => {
  await page.goto('/')
  await expect(page.locator('#workspace-main')).toBeVisible()
  await expect(page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Admin' })).toHaveCount(0)
  await page.keyboard.press('Control+K')
  await expect(page.getByRole('dialog', { name: 'Quick find' })).toBeVisible()
  await expect(page.getByRole('option', { name: /Admin/ })).toHaveCount(0)
  await page.keyboard.press('Escape')

  await page.goto('/admin')
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByRole('heading', { name: 'Investigation workspace' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Admin' })).toHaveCount(0)
  await expect(page.getByText('Administration data is unavailable')).toHaveCount(0)
})
