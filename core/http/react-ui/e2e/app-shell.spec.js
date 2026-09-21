import { test, expect } from './coverage-fixtures.js'

test.describe('NexusAI application shell', () => {
  test('shows route and local-workspace context on desktop', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/app')

    const header = page.getByRole('region', { name: 'Application context' })
    await expect(header).toBeVisible()
    await expect(header).toContainText('Workspace')
    await expect(header).toContainText('Local workspace')
  })

  test('keeps route context at tablet width and uses the compact mobile header at 390px', async ({ page }) => {
    await page.setViewportSize({ width: 820, height: 900 })
    await page.goto('/app')
    await expect(page.getByRole('region', { name: 'Application context' })).toBeVisible()

    await page.setViewportSize({ width: 390, height: 844 })
    await expect(page.getByRole('region', { name: 'Application context' })).toBeHidden()
    await expect(page.locator('.mobile-header')).toContainText('NexusAI / Workspace')
  })

  test('shows only the currently enforced administrator role', async ({ page }) => {
    await page.route('**/api/auth/status', (route) => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        authEnabled: true,
        staticApiKeyRequired: false,
        user: {
          id: 'admin-uuid',
          name: 'Nadia Admin',
          email: 'nadia@example.test',
          role: 'admin',
          permissions: {},
        },
      }),
    }))
    await page.goto('/app')

    const header = page.getByRole('region', { name: 'Application context' })
    await expect(header).toContainText('System Administrator')
    await expect(header).toContainText('Nadia Admin')
  })

  test('uses the URL-bound case identifier without inventing case metadata', async ({ page }) => {
    await page.route('**/api/v1/forensics/cases', (route) => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        default_case_id: 'case-alpha',
        cases: [{ case_id: 'case-alpha', collection_id: 'case-alpha', display_name: 'Alpha case', selectable: true }],
      }),
    }))
    await page.route('**/api/v1/forensics/cases/case-alpha**', (route) => route.abort())
    await page.goto('/app/cases/case-alpha/overview')

    const header = page.getByRole('region', { name: 'Application context' })
    await expect(header).toContainText('Case Workspace')
    await expect(header.getByLabel('Case context: case-alpha')).toBeVisible()
    await expect(header).not.toContainText('Unclassified')
  })

  test('provides a keyboard skip link to route content', async ({ page }) => {
    await page.goto('/app')

    const skipLink = page.getByRole('link', { name: 'Skip to main content' })
    // Installed system browsers can require one Tab to move focus from the
    // browser chrome into the document before sequential navigation begins.
    for (let attempt = 0; attempt < 2 && !(await skipLink.evaluate(el => document.activeElement === el)); attempt++) {
      await page.keyboard.press('Tab')
    }
    await expect(skipLink).toBeFocused()
    await skipLink.click()
    await expect(page.locator('#main-route-content')).toBeFocused()
  })

  test('does not leave a blank screen while authorization resolves', async ({ page }) => {
    let releaseStatus
    await page.route('**/api/auth/status', async (route) => {
      await new Promise(resolve => { releaseStatus = resolve })
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ authEnabled: false, staticApiKeyRequired: false, user: null }),
      })
    })

    await page.goto('/app')
    await expect(page.locator('.nexus-loading-state')).toContainText('Verifying secure access')
    releaseStatus()
    await expect(page.locator('.home-page')).toBeVisible()
  })

  test('renders a semantic unavailable state for an unknown route', async ({ page }) => {
    await page.goto('/app/no-such-route')
    const state = page.locator('[data-state="unavailable"]')
    await expect(state).toBeVisible()
    await expect(state).toContainText('404')
  })

  test('renders a semantic error when authorized case context cannot resolve', async ({ page }) => {
    await page.route('**/api/v1/forensics/cases', (route) => route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'Case registry unavailable' }),
    }))
    await page.goto('/app/records')

    const state = page.locator('[data-state="error"]')
    await expect(state).toHaveAttribute('role', 'alert')
    await expect(state).toContainText('Case workspace unavailable')
  })
})
