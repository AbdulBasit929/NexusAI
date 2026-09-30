import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/navigation-rail/', import.meta.url))
const activeCase = 'nexusai-forensic-demo'
const otherCase = 'nexusai-multimodal-product-acceptance'
const overview = {
  collection_id: activeCase,
  summary: { evidence_total: 4, evidence_completed: 4, evidence_in_flight: 0, accepted_rows: 8642 },
  record_families: [{ record_type: 'cdr', accepted_rows: 8642 }],
  recent_jobs: [],
  recent_evidence: [],
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(({ current, alternate }) => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = {
      tenantId: 'default',
      caseIds: [current, alternate],
      capabilities: { hybrid_query: true },
    }
    const key = `nexusai.viewer.questions.${current}`
    if (!localStorage.getItem(key)) localStorage.setItem(key, JSON.stringify([
      { id: 'q-1', query: 'Who called 03001234567 most often?', label: '', state: 'answered', pinned: false, updatedAt: '2026-09-27T10:00:00Z' },
      { id: 'q-2', query: 'Show the busiest hour', label: '', state: 'answered', pinned: false, updatedAt: '2026-09-27T09:00:00Z' },
    ]))
  }, { current: activeCase, alternate: otherCase })
  await page.route('**/collections/status*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(overview) }))
})

test('rail supports persistent collapse, pin, rename, and safe case switching', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the persistent rail contract.')
  await page.goto(`/cases/${activeCase}/overview`)
  await expect(page.getByText('This browser only', { exact: true })).toBeVisible()

  const firstQuestion = 'Who called 03001234567 most often?'
  const pin = page.getByRole('button', { name: `Pin ${firstQuestion}` })
  await pin.click()
  await expect(pin).toHaveAttribute('aria-pressed', 'true')

  await page.getByRole('button', { name: `Rename ${firstQuestion}` }).click()
  const rename = page.getByLabel('Rename question')
  await rename.fill('Priority caller pattern')
  await page.getByRole('button', { name: 'Save question name' }).click()
  await expect(page.getByText('Priority caller pattern', { exact: true })).toBeVisible()
  await expect(page.locator('.rail-question__link').filter({ hasText: 'Priority caller pattern' })).toContainText(firstQuestion)

  await page.getByRole('button', { name: 'Collapse navigation' }).click()
  const dashboard = page.getByRole('link', { name: 'Dashboard', exact: true })
  await expect(dashboard).toHaveAttribute('title', 'Dashboard')
  await dashboard.hover()
  await expect(dashboard.locator('.rail-tooltip')).toBeVisible()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Expand navigation' })).toBeVisible()

  await page.getByRole('button', { name: 'Expand navigation' }).click()
  await expect(page.getByText('Priority caller pattern', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: `Unpin Priority caller pattern` })).toHaveAttribute('aria-pressed', 'true')

  await page.evaluate(() => sessionStorage.setItem('nexusai.case.filters', 'CDR'))
  await page.getByRole('button', { name: /Switch case/ }).first().click()
  const dialog = page.getByRole('dialog', { name: 'Switch case' })
  await expect(dialog).toBeVisible()
  await dialog.getByRole('button', { name: 'Open case' }).click()
  await expect(page).toHaveURL(new RegExp(`/cases/${otherCase}/overview$`))
  expect(await page.evaluate(() => sessionStorage.getItem('nexusai.case.filters'))).toBeNull()
})

test('rail and drawer review artifacts remain clean in both themes', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns deterministic review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })
  for (const theme of ['light', 'dark']) {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.goto(`/cases/${activeCase}/overview`)
    await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
    await expect(page.getByRole('heading', { name: 'Case overview', exact: true })).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `expanded-${theme}-1440.png`), fullPage: true })
    await page.getByRole('button', { name: 'Collapse navigation' }).click()
    const evidenceLink = page.getByRole('link', { name: 'Evidence', exact: true })
    await evidenceLink.hover()
    await expect(evidenceLink.locator('.rail-tooltip')).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `collapsed-${theme}-1440.png`), fullPage: true })
    await page.getByRole('button', { name: 'Expand navigation' }).click()

    await page.setViewportSize({ width: 768, height: 1000 })
    await page.locator('.drawer-trigger').click()
    const drawer = page.getByRole('dialog', { name: 'Navigation' })
    await expect(drawer).toBeVisible()
    expect(await page.locator('[id]').evaluateAll(nodes => nodes.length - new Set(nodes.map(node => node.id)).size)).toBe(0)
    await page.screenshot({ path: path.join(reviewDirectory, `drawer-${theme}-768.png`), fullPage: true })
    await page.keyboard.press('Escape')
  }
})
