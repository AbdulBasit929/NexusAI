import { expect, test } from '@playwright/test'

const cases = ['case-alpha', 'case-bravo', 'case-charlie', 'case-empty']
const summaries = {
  'case-alpha': { evidence_total: 43, evidence_completed: 43, evidence_in_flight: 0, evidence_failed: 0, completed_jobs_missing_kb_asset: 0, accepted_rows: 22660 },
  'case-bravo': { evidence_total: 12, evidence_completed: 10, evidence_in_flight: 0, evidence_failed: 2, completed_jobs_missing_kb_asset: 0, accepted_rows: 4200 },
  'case-charlie': { evidence_total: 5, evidence_completed: 5, evidence_in_flight: 0, evidence_failed: 0, completed_jobs_missing_kb_asset: 0, accepted_rows: 90 },
  'case-empty': { evidence_total: 0, evidence_completed: 0, evidence_in_flight: 0, evidence_failed: 0, completed_jobs_missing_kb_asset: 0, accepted_rows: 0 },
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(caseIds => { window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', caseIds, capabilities: { hybrid_query: true } } }, cases)
  await page.route('**/query/capabilities*', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ families: [{ id: 'cdr', label: 'Call detail records', availability: 'queryable' }], query_corpus: { entries: [{ id: 'q1', family_id: 'cdr', query: 'Who called this number most often?', suggested: true }, { id: 'q2', family_id: 'cdr', query: 'Which identifiers occur most often?', suggested: true }] } }),
  }))
  await page.route('**/collections/status*', route => {
    const caseId = route.request().headers()['x-forensic-collection-id']
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ collection_id: caseId, summary: summaries[caseId], record_families: [], recent_jobs: [], recent_evidence: [], missing_kb_assets: [] }) })
  })
  await page.route('**/query/hybrid', route => {
    const caseId = route.request().headers()['x-forensic-collection-id']
    const body = caseId === 'case-alpha' ? { enterprise: { executive_answer: 'The number 0300-1234567 appears in 12 call records between 2 and 9 March.', result_state: 'complete' } }
      : caseId === 'case-charlie' ? { enterprise: { executive_answer: '', result_state: 'no_match' } }
        : { enterprise: { executive_answer: 'The number appears in 3 call records.', result_state: 'complete' } }
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
  })
})

test('one question is asked of every case with evidence and answered per case, best first', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the contract.')
  const browserErrors = []
  page.on('pageerror', error => browserErrors.push(error.message))
  page.on('console', message => { if (message.type() === 'error') browserErrors.push(message.text()) })
  await page.goto('/investigate')
  await expect(page.getByRole('heading', { level: 1, name: 'Ask a question' })).toBeVisible()
  await expect(page.getByText(/Searching 3 cases, 1 skipped/)).toBeVisible()
  await page.screenshot({ path: '/tmp/claude-0/-home-user-NexusAI/c6820396-bfc6-5f3d-99ab-de1c0d690aec/scratchpad/gi-landing.png' })
  await expect(page.getByRole('radio')).toHaveCount(0)
  await page.getByRole('button', { name: 'Who called this number most often?' }).click()
  await expect(page.getByRole('textbox', { name: 'Your question' })).toHaveValue('Who called this number most often?')
  await page.getByRole('button', { name: 'Ask' }).click()
  await expect(page.getByRole('heading', { name: 'Found in 2 of 3 cases.' })).toBeVisible()
  await expect(page.locator('.ax-case')).toHaveCount(3)
  await expect(page.locator('.ax-case').last().locator('.ax-case__state')).toHaveText('No matches')
  await expect(page.getByText(/Not searched: case-empty \(no evidence yet\)/)).toBeVisible()
  await expect(page.getByText('12 call records').first()).toBeVisible()
  await page.screenshot({ path: '/tmp/claude-0/-home-user-NexusAI/c6820396-bfc6-5f3d-99ab-de1c0d690aec/scratchpad/gi-results.png' })
  await page.setViewportSize({ width: 375, height: 812 })
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
  await page.screenshot({ path: '/tmp/claude-0/-home-user-NexusAI/c6820396-bfc6-5f3d-99ab-de1c0d690aec/scratchpad/ax-results-375.png', fullPage: true })
  expect(browserErrors).toEqual([])
})
