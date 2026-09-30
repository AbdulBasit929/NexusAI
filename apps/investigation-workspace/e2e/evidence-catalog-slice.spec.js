import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/evidence-catalog/', import.meta.url))
const caseId = 'evidence-research-case'

function item(index, overrides = {}) {
  return {
    evidence_id: `00000000-0000-4000-8000-${String(index).padStart(12, '0')}`,
    original_filename: `source-${String(index).padStart(2, '0')}.csv`,
    source_file: `source-${String(index).padStart(2, '0')}.csv`,
    modality: 'structured_records',
    detected_type: 'cdr',
    size_bytes: 1024 * index,
    processing_status: 'completed',
    accepted_rows: 100 + index,
    created_at: `2026-09-${String(Math.min(index, 28)).padStart(2, '0')}T10:30:00Z`,
    ...overrides,
  }
}

function responseFor(url) {
  const offset = Number(url.searchParams.get('offset') || 0)
  const detectedType = url.searchParams.get('detected_type')
  const modality = url.searchParams.get('modality')
  const status = url.searchParams.get('processing_status')
  const query = url.searchParams.get('q')

  let items
  let total
  if (modality === 'image') {
    items = []
    total = 0
  } else if (detectedType || status || query) {
    items = [item(2, {
      original_filename: query ? `${query}-result.csv` : 'failed-calls.csv',
      detected_type: detectedType || 'cdr',
      processing_status: status || 'failed',
    })]
    total = 1
  } else {
    const count = offset === 0 ? 25 : 5
    items = Array.from({ length: count }, (_, position) => item(offset + position + 1, position === 2 && offset === 0 ? { processing_status: 'failed' } : {}))
    total = 30
  }

  const completed = items.filter(entry => entry.processing_status === 'completed').length
  const failed = items.filter(entry => entry.processing_status === 'failed').length
  return {
    collection_id: caseId,
    filters: { limit: 25, offset, q: query || '', detected_type: detectedType || '', modality: modality || '', processing_status: status || '' },
    summary: { evidence_total: total, completed, queued: 0, processing: 0, failed, total_size_bytes: items.reduce((sum, entry) => sum + entry.size_bytes, 0) },
    pagination: { limit: 25, offset, returned: items.length, has_next: offset + items.length < total, has_previous: offset > 0, evidence_total: total },
    items,
  }
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(id => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', caseIds: [id], capabilities: { hybrid_query: true } }
  }, caseId)
  await page.route('**/collections/status*', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ collection_id: caseId, summary: { evidence_total: 30, evidence_completed: 29, evidence_in_flight: 0, evidence_failed: 1 }, recent_jobs: [], recent_evidence: [] }),
  }))
  await page.route('**/evidence?*', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(responseFor(new URL(route.request().url()))),
  }))
})

test('uses authoritative catalog filters and pagination without widening scope', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the evidence catalog contract.')
  const requests = []
  page.on('request', request => {
    if (new URL(request.url()).pathname.endsWith('/evidence')) requests.push(new URL(request.url()))
  })

  await page.goto(`/cases/${caseId}/evidence`)
  await expect(page.getByRole('table', { name: /Showing 1–25 of 30/ })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: 'Evidence' })).toBeVisible()
  await expect(page.locator('.evidence-table tbody tr')).toHaveCount(25)
  await expect(page.getByRole('table').getByText('Failed', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Retry' })).toHaveCount(0)

  await page.getByRole('button', { name: 'Next' }).click()
  await expect(page.getByText('Showing 26–30 of 30 evidence items.').first()).toBeVisible()
  await expect(page.getByText('source-30.csv', { exact: true })).toBeVisible()
  await expect.poll(() => requests.at(-1)?.searchParams.get('offset')).toBe('25')

  await page.getByRole('button', { name: 'CDR' }).click()
  await expect.poll(() => requests.at(-1)?.searchParams.get('detected_type')).toBe('cdr')

  await page.locator('.select-control__trigger').click()
  await page.getByRole('option', { name: 'Failed' }).click()
  await expect.poll(() => requests.at(-1)?.searchParams.get('processing_status')).toBe('failed')

  await page.getByRole('searchbox', { name: 'Search evidence' }).fill('calls')
  await page.getByRole('button', { name: 'Search', exact: true }).click()
  await expect.poll(() => requests.at(-1)?.searchParams.get('q')).toBe('calls')

  await page.getByRole('button', { name: 'Images' }).click()
  await expect(page.getByRole('heading', { name: 'No evidence matches these filters' })).toBeVisible()
  await expect(page.getByText('It was not silently widened.')).toBeVisible()
  await expect.poll(() => requests.at(-1)?.searchParams.get('modality')).toBe('image')
  await page.getByRole('button', { name: 'Clear filters' }).first().click()
  await expect(page.getByRole('table')).toBeVisible()
})

test('captures the evidence inventory in both themes and responsive layouts', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns deterministic review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })
  for (const theme of ['light', 'dark']) {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.goto(`/cases/${caseId}/evidence`)
    await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
    await expect(page.locator('.evidence-table tbody tr')).toHaveCount(25)
    await page.screenshot({ path: path.join(reviewDirectory, `evidence-catalog-${theme}-1440.png`), fullPage: true })

    await page.setViewportSize({ width: 390, height: 844 })
    await expect(page.locator('body')).toHaveJSProperty('scrollWidth', 390)
    await page.screenshot({ path: path.join(reviewDirectory, `evidence-catalog-${theme}-390.png`), fullPage: true })
  }
})
