import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/settings/', import.meta.url))

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: [] }
    localStorage.setItem('nexusai.viewer.questions.case-a', JSON.stringify([
      { id: 'q-1', query: 'Who called most often?', state: 'answered' },
      { id: 'q-2', query: 'Show the call type breakdown', state: 'answered' },
    ]))
    localStorage.setItem('nexusai.viewer.draft.case-a', 'Which numbers appear in both files?')
  })
})

test('keeps appearance synchronized and clears only the stated browser-local work', async ({ page }) => {
  await page.goto('/settings')
  const pageTheme = page.locator('.settings-page .theme-control')
  const localSummary = page.locator('.settings-summary')
  await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible()
  await expect(localSummary.getByText('Saved questions', { exact: true }).locator('..')).toContainText('2')
  await expect(localSummary.getByText('Unfinished drafts', { exact: true }).locator('..')).toContainText('1')
  await expect(page.getByText('Tenant')).toHaveCount(0)
  await expect(page.getByText('Role', { exact: true })).toHaveCount(0)
  await expect(page.getByText('API path')).toHaveCount(0)

  await pageTheme.getByRole('radio', { name: /Dark/ }).check()
  await expect.poll(() => page.evaluate(() => ({ theme: document.documentElement.dataset.theme, stored: localStorage.getItem('nexusai.viewer.theme') }))).toEqual({ theme: 'dark', stored: 'dark' })
  await page.getByRole('button', { name: /^Appearance, dark/ }).click()
  await expect(page.getByRole('group', { name: 'Appearance' }).getByRole('radio', { name: /Dark/ })).toBeChecked()
  await page.keyboard.press('Escape')

  await page.getByRole('button', { name: 'Clear question history and drafts…' }).click()
  await expect(page.getByRole('heading', { name: 'Clear 3 local items?' })).toBeVisible()
  await page.getByRole('button', { name: 'Clear from this browser' }).click()
  await expect(page.getByRole('status')).toContainText('were cleared from this browser')
  await expect(page.getByRole('heading', { name: 'No local questions or drafts' })).toBeVisible()
  await expect.poll(() => page.evaluate(() => ({ history: localStorage.getItem('nexusai.viewer.questions.case-a'), draft: localStorage.getItem('nexusai.viewer.draft.case-a'), theme: localStorage.getItem('nexusai.viewer.theme') }))).toEqual({ history: null, draft: null, theme: 'dark' })

  await pageTheme.getByRole('radio', { name: /Use device setting/ }).check()
  await expect.poll(() => page.evaluate(() => ({ theme: document.documentElement.getAttribute('data-theme'), stored: localStorage.getItem('nexusai.viewer.theme') }))).toEqual({ theme: null, stored: null })
})

test('settings remain legible in both themes and at the 375px contract width', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the deterministic Settings review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })
  for (const theme of ['light', 'dark']) {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.goto('/settings')
    await page.locator('.settings-page .theme-control').getByRole('radio', { name: new RegExp(`^${theme}`, 'i') }).check()
    await expect(page.getByRole('heading', { name: 'Remembered in this browser' })).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `settings-${theme}-1440.png`), fullPage: true })

    await page.setViewportSize({ width: 375, height: 812 })
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
    const targets = await page.locator('.settings-page .theme-control__option, .settings-page button').evaluateAll(nodes => nodes.filter(node => node.getClientRects().length).map(node => {
      const box = node.getBoundingClientRect()
      return { width: box.width, height: box.height, label: node.textContent.trim() }
    }))
    for (const target of targets) {
      expect(target.height, `${target.label} height`).toBeGreaterThanOrEqual(44)
      expect(target.width, `${target.label} width`).toBeGreaterThanOrEqual(44)
    }
    await page.screenshot({ path: path.join(reviewDirectory, `settings-${theme}-375.png`), fullPage: true })
  }
})
