import { expect, test } from '@playwright/test'

const caseId = 'case-alpha'
const id = '00000000-0000-4000-8000-000000000007'
const preview = Array.from({ length: 40 }, (_, i) => ({ row_number: i + 1, caller: `0300${1000000 + i * 37}`, call_type: i % 3 ? 'Call' : 'SMS', duration_seconds: 20 + i * 3, row_hash: `abc${i}def` }))

test.beforeEach(async ({ page }) => {
  await page.addInitScript(cases => { window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', caseIds: cases, capabilities: {} } }, [caseId])
  await page.route('**/collections/status*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ summary: { evidence_total: 12, evidence_completed: 12 }, record_families: [], recent_jobs: [], recent_evidence: [] }) }))
  await page.route(url => url.pathname.endsWith(`/evidence/${id}`), route => (route.request().resourceType() === 'document' ? route.continue() : route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      item: { evidence_id: id, original_filename: 'call-log-march-2026.csv', modality: 'structured_records', detected_type: 'cdr', size_bytes: 48213, processing_status: 'completed', accepted_rows: 8642, created_at: '2026-09-27T10:00:00Z', sha256: 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855' },
      records_preview: preview,
      derived_artifacts: [],
      processing_runs: [],
    }),
  })))
})

test('the evidence viewer is a workspace: file header, arrival strip, source canvas and an inspector', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the viewer contract.')
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  await page.goto(`/cases/${caseId}/evidence/${id}?row=12&row_hash=abc11def`)
  await expect(page.getByRole('heading', { level: 1, name: 'call-log-march-2026.csv' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Evidence', exact: true }).first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Exact source location' })).toBeVisible()
  await expect(page.locator('.evv-locator')).toContainText('abc11def')
  await expect(page.getByRole('heading', { name: 'Structured source' })).toBeVisible()
  // Details first, with the ID and hash copyable; Lineage one key away.
  const inspector = page.getByRole('complementary', { name: 'Evidence inspector' })
  await expect(inspector.getByRole('tab', { name: 'Details' })).toHaveAttribute('aria-selected', 'true')
  await expect(inspector).toContainText('Call detail records')
  await expect(inspector.getByRole('button', { name: 'Copy Evidence ID' })).toBeVisible()
  await inspector.getByRole('tab', { name: 'Details' }).focus()
  await page.keyboard.press('ArrowRight')
  await expect(inspector.getByRole('tab', { name: 'Lineage' })).toHaveAttribute('aria-selected', 'true')
  await expect(page.getByRole('heading', { name: 'Evidence lineage' })).toBeVisible()
  // The page itself does not scroll; the canvas does.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
  expect(errors).toEqual([])
})

test('the viewer has no page overflow on a phone', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the viewer contract.')
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/cases/${caseId}/evidence/${id}`)
  await expect(page.getByRole('heading', { level: 1, name: 'call-log-march-2026.csv' })).toBeVisible()
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
})
