import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/ui-redesign-20260929/application-shell/', import.meta.url))
const caseId = 'nexusai-forensic-demo'
const otherCase = 'nexusai-multimodal-product-acceptance'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(({ current, alternate }) => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = {
      tenantId: 'default',
      caseIds: [current, alternate],
      capabilities: { hybrid_query: true },
    }
    localStorage.setItem(`nexusai.viewer.questions.${current}`, JSON.stringify([
      { id: 'q-1', query: 'Who called most often?', label: 'Priority caller', state: 'answered', pinned: true, updatedAt: '2026-09-29T08:10:00Z' },
      { id: 'q-2', query: 'Show calls after 22:00', label: '', state: 'answered', pinned: false, updatedAt: '2026-09-29T08:05:00Z' },
      { id: 'q-3', query: 'Which towers carried the most traffic?', label: '', state: 'answered', pinned: false, updatedAt: '2026-09-29T08:00:00Z' },
    ]))
  }, { current: caseId, alternate: otherCase })
  await page.route('**/query/capabilities*', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ families: [], query_corpus: { entries: [] } }),
  }))
  await page.route('**/collections/status*', route => {
    const current = route.request().headers()['x-forensic-collection-id'] || caseId
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        collection_id: current,
        summary: { evidence_total: 4, evidence_completed: 4, evidence_in_flight: 0, evidence_failed: 0, completed_jobs_missing_kb_asset: 0, accepted_rows: 8642, duplicate_rows: 0, rejected_rows: 0 },
        record_families: [{ record_type: 'cdr', display_name: 'Call detail records', accepted_rows: 8642 }],
        recent_jobs: [],
        recent_evidence: [],
        missing_kb_assets: [],
      }),
    })
  })
  await page.route('**/evidence?*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ summary: { evidence_total: 0 }, items: [] }) }))
})

test('shell keeps keyboard access, case scope, and compact navigation intact', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic browser owns the shell interaction audit.')
  const browserErrors = []
  page.on('pageerror', error => browserErrors.push(error.message))
  page.on('console', message => { if (message.type() === 'error') browserErrors.push(message.text()) })

  await page.goto(`/cases/${caseId}/overview`)
  await expect(page.getByRole('banner').getByRole('button', { name: 'Quick find' })).toBeVisible()
  await expect(page.locator('.case-context-band')).toContainText(caseId)
  await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible()
  await expect(page.locator('.desktop-rail .rail-question')).toHaveCount(3)
  await expect(page.locator('.desktop-rail .rail-question').nth(1)).toBeVisible()

  const quickFind = page.getByRole('button', { name: 'Quick find' })
  await quickFind.focus()
  await page.keyboard.press('Control+k')
  await expect(page.getByRole('dialog', { name: 'Quick find' })).toBeVisible()
  await expect(page.getByText(`Ask in ${caseId}…`, { exact: true })).toBeVisible()
  await expect(page.getByRole('combobox')).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(quickFind).toBeFocused()

  await page.getByRole('button', { name: 'Collapse navigation' }).click()
  await expect(page.getByRole('button', { name: 'Expand navigation' })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Expand navigation' })).toBeVisible()

  await page.setViewportSize({ width: 375, height: 812 })
  const menu = page.getByRole('button', { name: 'Menu' })
  await menu.click()
  const drawer = page.getByRole('dialog', { name: 'Navigation' })
  await expect(drawer).toBeVisible()
  await expect(drawer.getByRole('button', { name: 'Close navigation' })).toBeFocused()
  await page.keyboard.press('Shift+Tab')
  await expect(drawer.locator(':focus')).toHaveCount(1)
  await page.keyboard.press('Tab')
  await expect(drawer.getByRole('button', { name: 'Close navigation' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(menu).toBeFocused()

  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
  const undersized = await page.locator('.workspace-header button:visible, .workspace-header a:visible, .case-context-band button:visible, .desktop-rail button:visible, .desktop-rail a:visible').evaluateAll(nodes => nodes.filter(node => {
    const rect = node.getBoundingClientRect()
    return rect.width > 0 && rect.height > 0 && (rect.width < 44 || rect.height < 44)
  }).map(node => ({ label: node.getAttribute('aria-label') || node.textContent?.trim(), width: node.getBoundingClientRect().width, height: node.getBoundingClientRect().height })))
  expect(undersized).toEqual([])
  expect(await page.locator('[id]').evaluateAll(nodes => nodes.length - new Set(nodes.map(node => node.id)).size)).toBe(0)
  expect(browserErrors).toEqual([])
})

test('captures the live dashboard and case shell in both Mineral Signal themes', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic browser owns review artifacts.')
  test.setTimeout(90_000)
  fs.mkdirSync(reviewDirectory, { recursive: true })

  for (const theme of ['light', 'dark']) {
    for (const width of [1440, 1280, 1024, 768, 375]) {
      await page.setViewportSize({ width, height: width >= 1024 ? 900 : 844 })
      await page.goto('/')
      await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
      await expect(page.getByRole('heading', { name: 'Case readiness' })).toBeVisible()
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
      await page.screenshot({ path: path.join(reviewDirectory, `dashboard-${theme}-${width}.png`), fullPage: true })
    }

    for (const width of [1440, 375]) {
      await page.setViewportSize({ width, height: width === 1440 ? 900 : 844 })
      await page.evaluate(() => localStorage.removeItem('nexusai.viewer.navigation-collapsed'))
      await page.goto(`/cases/${caseId}/overview`)
      await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
      await expect(page.getByText('Evidence ready', { exact: true })).toBeVisible()
      await page.screenshot({ path: path.join(reviewDirectory, `case-shell-${theme}-${width}.png`), fullPage: true })
    }

    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto(`/cases/${caseId}/overview`)
    await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value); localStorage.removeItem('nexusai.viewer.navigation-collapsed') }, theme)
    await page.reload()
    await page.getByRole('button', { name: 'Collapse navigation' }).click()
    await page.screenshot({ path: path.join(reviewDirectory, `rail-collapsed-${theme}-1440.png`), fullPage: true })
    await page.getByRole('button', { name: 'Quick find' }).click()
    await expect(page.getByRole('dialog', { name: 'Quick find' })).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `quick-find-${theme}-1440.png`), fullPage: false })
    await page.keyboard.press('Escape')

    await page.setViewportSize({ width: 375, height: 844 })
    await page.reload()
    await page.getByRole('button', { name: 'Menu' }).click()
    await page.waitForTimeout(200)
    await page.screenshot({ path: path.join(reviewDirectory, `drawer-open-${theme}-375.png`), fullPage: false })
  }
})
