import { test } from '@playwright/test'

const caseId = 'nexusai-multimodal-product-acceptance'
const widths = [1440, 1280, 1024, 768]

const routes = [
  ['Dashboard', '/'],
  ['Cases', '/cases'],
  ['New case', '/cases/new'],
  ['Global activity', '/activity'],
  ['Settings', '/settings'],
  ['Case overview', `/cases/${caseId}/overview`],
  ['Evidence', `/cases/${caseId}/evidence`],
  ['Investigate', `/cases/${caseId}/investigate`],
  ['Timeline', `/cases/${caseId}/timeline`],
  ['Case activity', `/cases/${caseId}/activity`],
]

test('records the pre-redesign route audit without changing application state', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One browser owns the observational audit.')
  test.setTimeout(240_000)

  const consoleMessages = []
  page.on('console', message => {
    if (message.type() === 'error' || message.type() === 'warning') consoleMessages.push(`${message.type()}: ${message.text()}`)
  })
  page.on('pageerror', error => consoleMessages.push(`pageerror: ${error.message}`))

  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto(`/cases/${caseId}/evidence`)
  await page.locator('#workspace-main').waitFor()
  const evidenceHref = await page.locator(`a[href^="/cases/${caseId}/evidence/"]`).first().getAttribute('href').catch(() => null)
  const auditedRoutes = evidenceHref ? [...routes, ['Evidence detail', evidenceHref]] : routes
  const observations = []

  for (const width of widths) {
    await page.setViewportSize({ width, height: width === 768 ? 1024 : 900 })
    for (const [name, route] of auditedRoutes) {
      consoleMessages.length = 0
      await page.goto(route)
      await page.locator('main').first().waitFor()
      await page.waitForFunction(() => document.querySelectorAll('h1').length > 0, null, { timeout: 5000 }).catch(() => {})
      const state = await page.evaluate(() => {
        const visible = element => {
          const style = getComputedStyle(element)
          return style.display !== 'none' && style.visibility !== 'hidden' && element.getClientRects().length > 0
        }
        const interactiveSelector = 'a[href], button, input, select, textarea, summary, [role="button"], [role="link"], [role="menuitem"]'
        const unreachable = [...document.querySelectorAll(interactiveSelector)]
          .filter(element => visible(element) && !element.disabled && element.getAttribute('aria-hidden') !== 'true' && element.tabIndex < 0)
          .map(element => ({ tag: element.tagName.toLowerCase(), text: (element.getAttribute('aria-label') || element.textContent || '').trim().slice(0, 80) }))
        const ids = [...document.querySelectorAll('[id]')].map(element => element.id).filter(Boolean)
        const duplicateIds = [...new Set(ids.filter((id, index) => ids.indexOf(id) !== index))]
        return {
          title: document.title,
          h1Count: document.querySelectorAll('h1').length,
          duplicateIds,
          horizontalOverflow: Math.max(0, document.documentElement.scrollWidth - document.documentElement.clientWidth),
          unreachable,
        }
      })
      observations.push({ width, name, route, ...state, consoleMessages: [...consoleMessages] })
    }
  }

  console.log(`UIUX_AUDIT_JSON=${JSON.stringify({ evidenceDetailIncluded: Boolean(evidenceHref), observations })}`)
})
