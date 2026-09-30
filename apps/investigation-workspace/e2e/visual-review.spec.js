import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'
import { liveFixture } from './live-fixtures.js'

const outputDirectory = fileURLToPath(new URL('../design-review/wi-ui-6/', import.meta.url))

const overview = {
  collection_id: 'nexusai-forensic-demo',
  summary: { evidence_total: 4, evidence_completed: 3, evidence_in_flight: 1, accepted_rows: 8642 },
  record_families: [{ record_type: 'cdr', accepted_rows: 8642 }],
  recent_jobs: [{ evidence_id: 'cdr-1', accepted_rows: 8642, rejected_rows: 3, duplicate_rows: 12, attempt_count: 1 }],
  recent_evidence: [{ evidence_id: 'cdr-1', source_file: 'seed_cdr_large.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', updated_at: '2026-09-24T08:30:00Z' }],
}

const catalog = {
  summary: { evidence_total: 4, modality_counts: { structured_records: 2, document: 1, audio: 1 } },
  items: [
    { evidence_id: 'cdr-1', original_filename: 'seed_cdr_large.csv', source_file: 'seed_cdr_large.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', size_bytes: 84000 },
    { evidence_id: 'audio-1', original_filename: 'interview-urdu.wav', source_file: 'interview-urdu.wav', modality: 'audio', detected_type: 'audio', processing_status: 'processing', size_bytes: 2048000 },
  ],
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default' }
  })
})

test('writes the WI-UI-6 route, state, named-theme, viewport, and RTL review set', async ({ page }, testInfo) => {
  test.setTimeout(240_000)
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic project owns the visual-review artifacts.')
  fs.mkdirSync(outputDirectory, { recursive: true })
  const responses = { answered: liveFixture('CDR-04.json'), clarification: liveFixture('CDR-05.json'), zero: liveFixture('DOC-06.json') }
  const questions = {
    answered: responses.answered.query_understanding.original_question,
    clarification: responses.clarification.query_understanding.original_question,
    zero: responses.zero.query_understanding.original_question,
    failed: 'Compare call activity with unavailable tower history',
  }
  await page.route('**/query/hybrid', async route => {
    const query = JSON.parse(route.request().postData() || '{}').query
    if (query === questions.failed) {
      return route.fulfill({ status: 500, headers: { 'x-request-id': 'QRY-4821' }, body: '{}' })
    }
    const state = Object.keys(questions).find(key => questions[key] === query)
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(responses[state]) })
  })
  await page.route('**/collections/status*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(overview) }))
  await page.route('**/evidence?*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(catalog) }))

  for (const width of [390, 820, 1024, 1440]) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 })
    for (const theme of ['light', 'dark']) {
      for (const state of ['answered', 'clarification', 'zero', 'failed']) {
        await page.goto('/cases/nexusai-forensic-demo/investigate')
        await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
        await page.evaluate(() => { document.documentElement.dir = 'ltr' })
        await page.getByLabel('Ask a question about this case').fill(questions[state])
        await page.getByRole('button', { name: 'Ask' }).click()
        await expect(page.locator(state === 'clarification' ? '.clarification' : '.investigation-result')).toBeVisible()
        expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
        await page.screenshot({
          path: path.join(outputDirectory, `${state}-${theme === 'light' ? 'daylight-ledger' : 'operations-slate'}-${width}.png`),
          fullPage: true,
        })
      }

      for (const [name, route] of [['overview', 'overview'], ['evidence', 'evidence'], ['activity', 'activity']]) {
        await page.goto(`/cases/nexusai-forensic-demo/${route}`)
        await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
        await page.evaluate(() => { document.documentElement.dir = 'ltr' })
        await expect(page.locator('main h1')).toHaveCount(1)
        expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
        await page.screenshot({ path: path.join(outputDirectory, `${name}-${theme === 'light' ? 'daylight-ledger' : 'operations-slate'}-${width}.png`), fullPage: true })
      }
    }
  }

  for (const width of [390, 820, 1024, 1440]) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 })
    for (const theme of ['light', 'dark']) {
      await page.goto('/cases/nexusai-forensic-demo/investigate')
      await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
      await page.evaluate(() => { document.documentElement.dir = 'rtl' })
      await page.getByLabel('Ask a question about this case').fill(questions.answered)
      await page.getByRole('button', { name: 'Ask' }).click()
      await expect(page.locator('.investigation-result')).toBeVisible()
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
      if (width === 1440) {
        const navigation = await page.locator('.desktop-rail').boundingBox()
        const workingSurface = await page.locator('.investigate-page').boundingBox()
        expect(navigation.x).toBeGreaterThan(workingSurface.x)
      }
      await page.screenshot({
        path: path.join(outputDirectory, `rtl-answered-${theme === 'light' ? 'daylight-ledger' : 'operations-slate'}-${width}.png`),
        fullPage: true,
      })
    }
  }

  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto('/cases/nexusai-forensic-demo/investigate')
  await page.getByLabel('Ask a question about this case').fill(questions.answered)
  await page.getByRole('button', { name: 'Ask' }).click()
  await page.screenshot({ path: path.join(outputDirectory, 'aggregate-citation-rail-daylight-ledger-1440.png'), fullPage: true })
})
