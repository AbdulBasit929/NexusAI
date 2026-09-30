import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'
import { liveFixture } from './live-fixtures.js'

const outputDirectory = fileURLToPath(new URL('../design-review/wi-ui-7/', import.meta.url))
const widths = [375, 820, 1024, 1440]
const names = { light: 'daylight-ledger', dark: 'operations-slate' }
const responses = { answered: liveFixture('CDR-04.json'), clarification: liveFixture('CDR-05.json'), zero: liveFixture('DOC-06.json') }
const questions = {
  answered: responses.answered.query_understanding.original_question,
  clarification: responses.clarification.query_understanding.original_question,
  zero: responses.zero.query_understanding.original_question,
  failed: 'Compare call activity with unavailable tower history',
}
const overview = {
  collection_id: 'nexusai-forensic-demo',
  summary: { evidence_total: 4, evidence_completed: 3, evidence_in_flight: 1, accepted_rows: 8642 },
  record_families: [{ record_type: 'cdr', accepted_rows: 8642 }],
  recent_jobs: [{ evidence_id: 'cdr-1', accepted_rows: 8642, rejected_rows: 3, duplicate_rows: 12, attempt_count: 1 }],
  recent_evidence: [{ evidence_id: 'cdr-1', source_file: 'seed_cdr_large.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', updated_at: '2026-09-24T08:30:00Z' }],
}
const catalog = {
  summary: { evidence_total: 3 },
  items: [
    { evidence_id: 'cdr-1', original_filename: 'seed_cdr_large.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', size_bytes: 84000 },
    { evidence_id: 'doc-1', original_filename: 'interview-notes.pdf', modality: 'document', detected_type: 'document', processing_status: 'completed', size_bytes: 48000 },
    { evidence_id: 'audio-1', original_filename: 'interview-urdu.wav', modality: 'audio', detected_type: 'audio', processing_status: 'processing', size_bytes: 2048000 },
  ],
}

async function setTheme(page, theme) {
  await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
}

async function assertNoOverflow(page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', collectionId: 'nexusai-forensic-demo', caseIds: ['nexusai-forensic-demo'], capabilities: { hybrid_query: true } }
  })
  await page.route('**/collections/status*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(overview) }))
  await page.route('**/evidence?*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(catalog) }))
  await page.route('**/query/hybrid', route => {
    const query = JSON.parse(route.request().postData() || '{}').query
    if (query === questions.failed) return route.fulfill({ status: 500, headers: { 'x-request-id': 'QRY-4821' }, body: '{}' })
    const state = Object.keys(questions).find(key => questions[key] === query)
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(responses[state]) })
  })
})

test('captures the WI-UI-7 contract review set at every acceptance width and theme', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic project owns review artifacts.')
  test.setTimeout(300_000)
  fs.mkdirSync(outputDirectory, { recursive: true })
  for (const width of widths) {
    await page.setViewportSize({ width, height: width === 375 ? 812 : 1000 })
    for (const theme of ['light', 'dark']) {
      for (const [name, route] of [['overview', 'overview'], ['evidence', 'evidence'], ['activity', 'activity']]) {
        await page.goto(`/cases/nexusai-forensic-demo/${route}`)
        await setTheme(page, theme)
        await expect(page.locator('main h1')).toHaveCount(1)
        await assertNoOverflow(page)
        await page.screenshot({ path: path.join(outputDirectory, `${name}-${names[theme]}-${width}.png`), fullPage: true })
      }
      await page.goto('/design/type-proof')
      await setTheme(page, theme)
      await assertNoOverflow(page)
      await page.screenshot({ path: path.join(outputDirectory, `type-proof-${names[theme]}-${width}.png`), fullPage: true })
      for (const state of ['answered', 'clarification', 'zero', 'failed']) {
        await page.goto('/cases/nexusai-forensic-demo/investigate')
        await setTheme(page, theme)
        await page.getByLabel('Ask a question about this case').fill(questions[state])
        await page.getByRole('button', { name: 'Ask', exact: true }).click()
        await expect(page.locator('.investigation-result')).toBeVisible()
        await assertNoOverflow(page)
        await page.screenshot({ path: path.join(outputDirectory, `${state}-${names[theme]}-${width}.png`), fullPage: true })
      }
    }
  }
})

test('records drawer focus, collapsed rail, and claim-level citation close-up', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic project owns review artifacts.')
  fs.mkdirSync(outputDirectory, { recursive: true })
  await page.setViewportSize({ width: 375, height: 812 })
  await page.goto('/cases/nexusai-forensic-demo/investigate')
  const trigger = page.locator('.drawer-trigger')
  await trigger.focus()
  await page.keyboard.press('Enter')
  const drawer = page.getByRole('dialog', { name: 'Navigation' })
  await expect(drawer).toBeVisible()
  await page.waitForTimeout(200)
  await page.screenshot({ path: path.join(outputDirectory, 'drawer-open-daylight-ledger-375.png'), fullPage: true })
  await page.keyboard.press('Escape')
  await expect(trigger).toBeFocused()

  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto('/cases/nexusai-forensic-demo/investigate')
  const quickFind = page.getByRole('button', { name: /Quick find/ })
  await quickFind.click()
  await expect(page.getByRole('dialog', { name: 'Quick find' })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(quickFind).toBeFocused()
  await page.evaluate(() => document.activeElement?.blur())
  await page.screenshot({ path: path.join(outputDirectory, 'shell-expanded-daylight-ledger-1440.png'), fullPage: true })
  await page.getByRole('button', { name: 'Collapse navigation' }).click()
  await page.screenshot({ path: path.join(outputDirectory, 'rail-collapsed-daylight-ledger-1440.png'), fullPage: true })
  await page.reload()
  await expect(page.getByRole('button', { name: 'Expand navigation' })).toBeVisible()
  await page.getByLabel('Ask a question about this case').fill(questions.answered)
  await page.getByRole('button', { name: 'Ask', exact: true }).click()
  await expect(page.locator('.answer-block .citation-marker')).toHaveCount(5)
  await page.locator('.answer-block .citation-marker').first().hover()
  const tooltip = page.getByRole('tooltip')
  await expect(tooltip).toBeVisible()
  const answerBox = await page.locator('.answer-block').boundingBox()
  const tooltipBox = await tooltip.boundingBox()
  const x = Math.max(0, Math.min(answerBox.x, tooltipBox.x) - 12)
  const y = Math.max(0, Math.min(answerBox.y, tooltipBox.y) - 12)
  const right = Math.min(1440, Math.max(answerBox.x + answerBox.width, tooltipBox.x + tooltipBox.width) + 12)
  const bottom = Math.max(answerBox.y + answerBox.height, tooltipBox.y + tooltipBox.height) + 12
  await page.screenshot({ path: path.join(outputDirectory, 'claim-level-citations-close-up-daylight-ledger-1440.png'), clip: { x, y, width: right - x, height: bottom - y } })
  await page.goto('/design/route-states')
  await page.screenshot({ path: path.join(outputDirectory, 'route-state-fixtures-daylight-ledger-1440.png'), fullPage: true })
})

test('records the keyboard-only drawer trap and core question loop', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'mobile', 'The mobile project records the drawer contract.')
  await page.setViewportSize({ width: 375, height: 812 })
  await page.goto('/cases/nexusai-forensic-demo/investigate')
  await expect(page.locator('#workspace-main')).toBeVisible()
  await assertNoOverflow(page)
  await page.keyboard.press('Tab')
  await expect(page.getByRole('link', { name: 'Skip to investigation' })).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(page.locator('#workspace-main')).toBeFocused()
  const trigger = page.locator('.drawer-trigger')
  await trigger.focus(); await page.keyboard.press('Enter')
  await expect(page.getByRole('dialog', { name: 'Navigation' })).toBeVisible()
  await page.keyboard.press('Shift+Tab')
  await expect(page.getByRole('dialog', { name: 'Navigation' }).locator('.shell-navigation a').last()).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(page.getByRole('button', { name: 'Close' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(trigger).toBeFocused()
  const question = page.getByLabel('Ask a question about this case')
  await question.focus(); await page.keyboard.type(questions.answered); await page.keyboard.press('Tab'); await page.keyboard.press('Enter')
  await expect(page.locator('.answer-block .citation-marker')).toHaveCount(5)
})
