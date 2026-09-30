import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const outputDirectory = fileURLToPath(new URL('../design-review/wi-ui-6/', import.meta.url))
const widths = [390, 820, 1024, 1440]
const proofStrings = [
  'کال 03001234567 at 1035',
  'مجھے معلوم نہیں آیا آپ نے محسوس کیا یا نہیں اس ملک میں سینٹر لمڈیکہ سے',
  'kal 03001234567 at 1035',
  'داری کام بہنی کھین چک ساتھ ہے',
]

const overview = {
  collection_id: 'demo-case',
  summary: { evidence_total: 1, evidence_completed: 1, evidence_in_flight: 0, accepted_rows: 12 },
  record_families: [{ record_type: 'cdr', accepted_rows: 12 }],
  recent_jobs: [{ evidence_id: 'ev-1', accepted_rows: 12, rejected_rows: 0, duplicate_rows: 0, attempt_count: 1 }],
  recent_evidence: [{ evidence_id: 'ev-1', source_file: 'calls.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', updated_at: '2026-09-24T08:30:00Z' }],
}
const catalog = {
  summary: { evidence_total: 1, modality_counts: { structured_records: 1 } },
  items: [{ evidence_id: 'ev-1', original_filename: 'calls.csv', source_file: 'calls.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', size_bytes: 2048 }],
}
const detail = {
  item: catalog.items[0],
  records_preview: [{ row_number: 1, calling_number: '03001234567', call_type: 'CALL' }],
  derived_artifacts: [],
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', collectionId: 'demo-case', capabilities: { hybrid_query: true } }
  })
  await page.route('**/collections/status*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(overview) }))
  await page.route('**/evidence?*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(catalog) }))
  await page.route('**/evidence/ev-1?*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(detail) }))
})

test('renders every proof string at every type step with stable line boxes and isolated digits', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic project owns cross-width proof artifacts.')
  test.setTimeout(120_000)
  fs.mkdirSync(outputDirectory, { recursive: true })

  for (const width of widths) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 })
    for (const theme of ['light', 'dark']) {
      await page.goto('/design/type-proof')
      await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
      await page.evaluate(() => document.fonts.ready)
      await expect(page.locator('[data-proof-step]')).toHaveCount(7)
      for (const value of proofStrings) await expect(page.locator('[data-proof-step]').getByText(value, { exact: true })).toHaveCount(7)

      const proof = await page.evaluate(() => {
        const steps = [...document.querySelectorAll('[data-proof-step]')]
        return {
          lineBoxes: steps.map(step => ({
            step: step.dataset.proofStep,
            values: [...step.querySelectorAll('.type-step')].map(node => getComputedStyle(node).lineHeight),
          })),
          mixedText: [...document.querySelectorAll('[data-proof-script="mixed-urdu"]')].map(node => ({
            text: node.textContent,
            identifiers: [...node.querySelectorAll('bdi[dir="ltr"]')].map(item => item.textContent),
            family: getComputedStyle(node).fontFamily,
          })),
          romanFamily: getComputedStyle(document.querySelector('[data-proof-script="roman-urdu"]')).fontFamily,
          baselineAlignment: getComputedStyle(document.querySelector('.type-proof__baseline > div')).alignItems,
          shellDirection: getComputedStyle(document.querySelector('.workspace-shell')).direction,
          overflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
          h1s: document.querySelectorAll('h1').length,
        }
      })

      for (const step of proof.lineBoxes) expect(new Set(step.values).size, `${step.step} line-height`).toBe(1)
      for (const row of proof.mixedText) {
        expect(row.text).toBe(proofStrings[0])
        expect(row.identifiers).toEqual(['03001234567', '1035'])
        expect(row.family).toContain('Noto Sans Arabic Variable')
      }
      expect(proof.romanFamily).toContain('Noto Sans Variable')
      expect(proof.baselineAlignment).toBe('baseline')
      expect(proof.shellDirection).toBe('ltr')
      expect(proof.overflow).toBeLessThanOrEqual(0)
      expect(proof.h1s).toBe(1)
      await page.screenshot({ path: path.join(outputDirectory, `type-proof-${theme === 'light' ? 'daylight-ledger' : 'operations-slate'}-${width}.png`), fullPage: true })
    }
  }
})

test('keeps every route to one H1 with no overflow or console errors at all acceptance widths', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic project owns the cross-width route audit.')
  test.setTimeout(120_000)
  const errors = []
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
  page.on('pageerror', error => errors.push(error.message))
  const routes = [
    '/cases', '/cases/new', '/cases/demo-case/overview', '/cases/demo-case/evidence',
    '/cases/demo-case/evidence/ev-1', '/cases/demo-case/investigate',
    '/cases/demo-case/timeline', '/cases/demo-case/activity', '/design/type-proof',
  ]
  for (const width of widths) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 })
    for (const route of routes) {
      await page.goto(route)
      await expect(page.locator('h1')).toHaveCount(1)
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), `${route} at ${width}px`).toBeLessThanOrEqual(0)
    }
  }
  expect(errors).toEqual([])
})
