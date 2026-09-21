import { test, expect } from './coverage-fixtures.js'

// Login page (src/pages/Login.jsx). /login redirects to /analyst when auth is
// disabled, so the harness mocks /api/auth/status to enable auth with no
// signed-in user. With no password/OAuth providers configured, the page
// offers API-token login — the path exercised here.
test.describe('Login page', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/branding', (route) =>
      route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          instance_name: 'NexusAI',
          instance_tagline: '',
          logo_url: '/static/logo.png',
          logo_horizontal_url: '/static/logo_horizontal.png',
          favicon_url: '/favicon.svg',
        }),
      }),
    )
    await page.route('**/api/auth/status', (route) =>
      route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          authEnabled: true,
          staticApiKeyRequired: false,
          user: null,
          hasUsers: true,
          providers: [],
          registrationMode: 'open',
          permissions: {},
        }),
      }),
    )
    await page.goto('/login')
  })

  test('renders the API token login option', async ({ page }) => {
    await expect(page).toHaveURL(/\/login$/)
    await expect(page.getByRole('button', { name: /Login with API Token/i })).toBeVisible()
  })

  test('renders the NexusAI identity without a legacy image dependency', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'NexusAI' })).toBeVisible()
    await expect(page.getByText('Enterprise Forensic Intelligence')).toBeVisible()
    await expect(page.getByText('Private AI infrastructure for your applications.')).toBeVisible()
    await expect(page.locator('.login-header img[src*="/static/logo"]')).toHaveCount(0)
    await expect(page).toHaveTitle('NexusAI')
    await expect(page.locator("meta[name='application-name']")).toHaveAttribute('content', 'NexusAI')
    await expect(page.locator("link[rel='icon']")).toHaveAttribute('href', /\/favicon\.svg/)
  })

  test('reveals and accepts an API token', async ({ page }) => {
    await page.getByRole('button', { name: /Login with API Token/i }).click()
    const tokenInput = page.locator('input').first()
    await expect(tokenInput).toBeVisible()
    await tokenInput.fill('sk-test-token')
    await expect(tokenInput).toHaveValue('sk-test-token')
  })
})
