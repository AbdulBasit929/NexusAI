import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/dashboard-v2/', import.meta.url))
const cases = ['case-ready', 'case-processing', 'case-review', 'case-empty']
const summaries = {
  'case-ready': { evidence_total: 4, evidence_completed: 4, evidence_in_flight: 0, evidence_failed: 0, completed_jobs_missing_kb_asset: 0, accepted_rows: 8642, rejected_rows: 3, duplicate_rows: 12 },
  'case-processing': { evidence_total: 2, evidence_completed: 0, evidence_in_flight: 2, evidence_failed: 0, completed_jobs_missing_kb_asset: 0, accepted_rows: 0 },
  'case-review': { evidence_total: 2, evidence_completed: 1, evidence_in_flight: 0, evidence_failed: 1, completed_jobs_missing_kb_asset: 0, accepted_rows: 420 },
  'case-empty': { evidence_total: 0, evidence_completed: 0, evidence_in_flight: 0, evidence_failed: 0, completed_jobs_missing_kb_asset: 0, accepted_rows: 0 },
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(caseIds => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', caseIds, capabilities: { hybrid_query: true } }
    localStorage.setItem('nexusai.viewer.questions.case-ready', JSON.stringify([{ id: 'q-1', query: 'Who called most often?', label: 'Priority caller', state: 'answered', pinned: true, updatedAt: '2026-09-27T10:00:00Z' }]))
  }, cases)
  await page.route('**/query/capabilities*', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      families: [{ id: 'cdr', label: 'Call detail records', availability: 'queryable' }],
      query_corpus: { entries: [{ id: 'cdr-1', family_id: 'cdr', query: 'Which identifiers occur most often?', suggested: true }] },
    }),
  }))
  // Whole-case activity for the hero chart: the deterministic activity_by_day template, shaped as the service returns it.
  await page.route('**/query/hybrid', route => {
    const caseId = route.request().headers()['x-forensic-collection-id']
    const rows = caseId === 'case-ready' ? [{ activity_date: '2026-09-01T00:00:00Z', record_type: 'cdr', event_count: 40 }, { activity_date: '2026-09-02T00:00:00Z', record_type: 'cdr', event_count: 25 }, { activity_date: '2026-09-02T00:00:00Z', record_type: 'anpr', event_count: 5 }] : []
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ records: { activity_by_day: rows, row_count: rows.length } }) })
  })
  await page.route('**/collections/status*', route => {
    const caseId = route.request().headers()['x-forensic-collection-id']
    const status = caseId === 'case-review' ? 'failed' : caseId === 'case-processing' ? 'processing' : 'completed'
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
      collection_id: caseId,
      summary: summaries[caseId],
      record_families: caseId === 'case-ready' ? [{ record_type: 'cdr', accepted_rows: 8642, duplicate_rows: 12, rejected_rows: 3 }] : [],
      recent_jobs: caseId === 'case-empty' ? [] : [{ job_id: `${caseId}-job`, evidence_id: `${caseId}-source`, source_file: `${caseId}.csv`, status, attempt_count: caseId === 'case-review' ? 2 : 1, max_attempts: 3, error_message: caseId === 'case-review' ? 'The source could not be parsed.' : '', started_at: '2026-09-27T09:58:20Z', completed_at: status === 'processing' ? null : '2026-09-27T10:00:00Z' }],
      recent_evidence: caseId === 'case-empty' ? [] : [{ evidence_id: `${caseId}-source`, source_file: `${caseId}.csv`, processing_status: status, updated_at: '2026-09-27T10:00:00Z' }],
      missing_kb_assets: [],
    }) })
  })
})

test('dashboard answers named questions, links every figure to its records and keeps a truthful action inspector', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the dashboard contract.')
  let statusRequests = 0
  const browserErrors = []
  page.on('pageerror', error => browserErrors.push(error.message))
  page.on('console', message => { if (message.type() === 'error') browserErrors.push(message.text()) })
  page.on('request', request => { if (new URL(request.url()).pathname.endsWith('/collections/status')) statusRequests += 1 })
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Dashboard' })).toBeVisible()

  // Totals are real sums of the four mocked cases, each linking to the rows behind it.
  const totals = page.getByRole('list', { name: 'Workspace totals' })
  // Figures count up for about 700 ms, so assert on the visible digits (auto-waits) rather than the raw text, which
  // also holds the screen-reader copy of the final value.
  const figure = label => totals.getByRole('listitem').filter({ hasText: label }).locator('.dash-kpi__num')
  await expect(figure('Ready')).toHaveText('5')
  await expect(totals.getByRole('listitem').filter({ hasText: 'Ready' })).toContainText('of 8')
  await expect(figure('Needs review')).toHaveText('1')
  await expect(figure('Processing')).toHaveText('2')
  await expect(figure('Structured rows')).toHaveText('9,062')

  // The sidebar carries the live count of cases needing review and the global Investigate entry.
  await expect(page.getByRole('link', { name: /Dashboard, 1 case needs review/ })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Investigate', exact: true })).toHaveAttribute('href', '/investigate')

  // The hero: activity over time from real day buckets, with an exact table alternative.
  const activity = page.locator('.activity-card')
  await expect(activity.getByRole('heading', { name: 'When did activity happen?' })).toBeVisible()
  await expect(activity.getByRole('img', { name: /Activity per day by record family, 2026-09-01 to 2026-09-02/ })).toBeVisible()
  await activity.getByRole('button', { name: /Table/ }).click()
  await expect(activity.getByRole('table')).toContainText('2026-09-02')
  await activity.getByRole('button', { name: /Chart/ }).click()

  // Each chart has a question for a title and an exact table alternative with real links.
  const readiness = page.locator('#dashboard-readiness')
  await expect(readiness.getByRole('heading', { name: /Is each case’s evidence ready to search\?/ })).toBeVisible()
  await expect(readiness.getByRole('link', { name: /1 failed of 2 sources in case-review/ })).toHaveAttribute('href', '/cases/case-review/evidence?status=failed')
  await readiness.getByRole('button', { name: /Table/ }).click()
  await expect(readiness.getByRole('link', { name: '1', exact: true }).first()).toHaveAttribute('href', /\/cases\/case-review\/evidence\?status=/)
  const families = page.locator('#dashboard-families')
  await expect(families.getByRole('heading', { name: /What is the evidence made of\?/ })).toBeVisible()
  // A bubble is a button: selecting a family filters the case queue, and the same button clears it.
  await families.getByRole('button', { name: /Call detail records: 8,642 accepted rows/ }).first().click()
  await expect(page.locator('.dashboard-case')).toHaveCount(1)
  await families.getByRole('button', { name: /Call detail records: 8,642 accepted rows/ }).first().click()
  await expect(page.locator('.dashboard-case')).toHaveCount(4)
  await families.getByRole('button', { name: /Table/ }).click()
  await families.getByRole('button', { name: 'Show only cases with Call detail records' }).click()
  const queueItems = page.locator('.dashboard-case')
  await expect(queueItems).toHaveCount(1)
  await expect(queueItems.first()).toContainText('case-ready')
  await page.getByRole('button', { name: 'All cases' }).click()
  await expect(queueItems).toHaveCount(4)

  // Needs-review list names the failed source and links to it.
  const attention = page.locator('#dashboard-attention')
  await expect(attention.getByRole('link', { name: 'Review case-review.csv' })).toHaveAttribute('href', '/cases/case-review/evidence/case-review-source')
  // Findings are grouped by cause; the biggest cause opens so its sources show at once.
  await expect(attention).toContainText('The source could not be parsed')
  await expect(attention.getByRole('button', { name: /The source could not be parsed/ })).toHaveAttribute('aria-expanded', 'true')

  // Where sources and rows end up, as exact numbers behind the flow diagram, and the honest key-entities card.
  const flow = page.locator('#dashboard-flow')
  await expect(flow.getByRole('heading', { name: 'Where do sources and rows end up?' })).toBeVisible()
  await flow.getByRole('button', { name: /Table/ }).click()
  await expect(flow.getByRole('table')).toContainText('Failed')
  await expect(page.locator('#dashboard-entities').getByRole('link', { name: /most frequent contacts/ })).toBeVisible()

  // Case workbench: readiness order, filters and the inspector.
  await expect(queueItems.nth(0)).toContainText('case-review')
  await expect(queueItems.nth(0)).toHaveAttribute('aria-pressed', 'true')
  await expect(page.getByRole('heading', { name: 'Review evidence in case-review' })).toBeVisible()
  await queueItems.filter({ hasText: 'case-ready' }).click()
  await expect(page.getByRole('heading', { name: 'Continue with case-ready' })).toBeVisible()
  await expect(page.getByText('8,642', { exact: true }).first()).toBeVisible()
  await page.getByRole('tab', { name: /Questions/ }).click()
  await expect(page.getByText('Which identifiers occur most often?', { exact: true })).toBeVisible()
  await page.getByRole('tab', { name: /Questions/ }).press('ArrowLeft')
  await expect(page.getByRole('tab', { name: /Processing/ })).toHaveAttribute('aria-selected', 'true')
  await expect(page.getByRole('link', { name: 'Start investigating' })).toBeVisible()
  await page.getByRole('group', { name: 'Filter cases' }).getByRole('button', { name: 'Needs review' }).click()
  await expect(queueItems).toHaveCount(1)
  await expect(queueItems.first()).toContainText('case-review')
  await page.getByRole('searchbox', { name: 'Find a case' }).fill('missing')
  await expect(page.getByRole('heading', { name: 'No cases match this view' })).toBeVisible()
  await page.getByRole('button', { name: 'Clear view' }).first().click()
  await expect(queueItems).toHaveCount(4)

  await expect.poll(() => statusRequests).toBeGreaterThanOrEqual(4)
  await page.getByRole('button', { name: 'Refresh' }).click()
  await expect.poll(() => statusRequests).toBeGreaterThanOrEqual(8)

  await expect(page.getByText('This browser only', { exact: true })).toBeVisible()
  await expect(page.getByText('Priority caller', { exact: true })).toBeVisible()
  await expect(page.getByText('Who called most often?', { exact: true })).toBeVisible()
  await expect(page.getByText(/Assignment, severity and investigative priority are not available/)).toBeVisible()

  await page.setViewportSize({ width: 375, height: 812 })
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
  expect(await page.evaluate(() => [...document.querySelectorAll('[id]')].map(node => node.id).filter((id, index, ids) => ids.indexOf(id) !== index))).toEqual([])
  expect(browserErrors).toEqual([])
})

test('captures the task-first dashboard in both themes and responsive sizes', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns deterministic review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })
  for (const theme of ['light', 'dark']) {
    for (const width of [1440, 1280, 1024, 768, 375]) {
      await page.setViewportSize({ width, height: width >= 1024 ? 900 : 844 })
      await page.goto('/')
      await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
      await expect(page.getByRole('heading', { name: 'Review evidence in case-review' })).toBeVisible()
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
      await page.screenshot({ path: path.join(reviewDirectory, `dashboard-${theme}-${width}.png`), fullPage: true })
    }
  }
})
