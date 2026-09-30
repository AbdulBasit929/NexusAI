import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'
import { liveFixture } from './live-fixtures.js'

const reviewDirectory = fileURLToPath(new URL('../design-review/uiux-20260927/investigate-chat/', import.meta.url))
const answered = liveFixture('CDR-04.json')
const clarification = liveFixture('CDR-05.json')
const caseId = 'nexusai-forensic-demo'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(id => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', collectionId: id, caseIds: [id], capabilities: { hybrid_query: true } }
    localStorage.setItem(`nexusai.viewer.questions.${id}`, JSON.stringify([{ id: 'saved-1', query: 'Which identifiers occur most often?', state: 'answered', pinned: true, updatedAt: '2026-09-27T10:00:00Z' }]))
  }, caseId)
  await page.route('**/collections/status*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ collection_id: caseId, summary: { evidence_total: 4, evidence_completed: 4, evidence_in_flight: 0, evidence_failed: 0 }, record_families: [{ record_type: 'cdr', accepted_rows: 8642 }], recent_jobs: [], recent_evidence: [] }) }))
  await page.route('**/query/capabilities*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ families: [
    { id: 'cdr', label: 'CDR', availability: 'queryable', record_types: ['cdr'], suggested_queries: ['Show the call type breakdown', 'Which identifiers occur most often?'] },
    { id: 'anpr', label: 'ANPR', availability: 'limited', record_types: ['anpr'], suggested_queries: ['Count distinct plates'] },
    { id: 'subscriber_identity', label: 'Subscriber', availability: 'queryable', record_types: ['subscriber'], suggested_queries: ['Find linked subscriber identities'] },
  ] }) }))
})

test('freezes exact scope onto the turn, clears it on scope change, and can stop an active request', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns the Investigate slice contract.')
  const requests = []
  await page.route('**/query/hybrid', async route => {
    const payload = JSON.parse(route.request().postData() || '{}')
    requests.push(payload)
    if (payload.query === 'Keep checking until I stop') {
      await new Promise(resolve => setTimeout(resolve, 750))
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(answered) }).catch(() => {})
  })
  await page.goto(`/cases/${caseId}/investigate`)
  await page.locator('.thread-composer__scope > summary').click()
  await page.getByRole('button', { name: 'CDR', exact: true }).click()
  const question = page.getByLabel('Ask a question about this case')
  await question.fill('Break down CDR records by call type')
  await page.getByRole('button', { name: 'Ask', exact: true }).click()
  await expect(page.locator('.thread-turn__meta')).toContainText('CDR scope')
  expect(requests[0].record_type).toBe('cdr')
  await page.locator('.thread-composer__scope > summary').click()
  await page.getByRole('button', { name: 'ANPR', exact: true }).click()
  await expect(page.locator('.thread-turn')).toHaveCount(0)
  await expect(page.locator('.thread-composer__scope > summary')).toContainText('ANPR')

  await page.locator('.thread-composer__scope > summary').click()
  await page.getByRole('button', { name: 'CDR', exact: true }).click()
  await question.fill('Keep checking until I stop')
  await page.getByRole('button', { name: 'Ask', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Stop current question' })).toBeVisible()
  await page.getByRole('button', { name: 'Stop current question' }).click()
  await expect(page.getByText('This question was stopped before a result was returned.')).toBeVisible()
  await expect(page.getByLabel('Recent questions').getByText('This browser only', { exact: true })).toBeVisible()
})

test('captures the evidence-review workspace and careful clarification in both themes', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'Desktop owns deterministic review artifacts.')
  fs.mkdirSync(reviewDirectory, { recursive: true })
  await page.route('**/query/hybrid', route => {
    const query = JSON.parse(route.request().postData() || '{}').query
    const response = query === clarification.query_understanding.original_question ? clarification : answered
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(response) })
  })
  for (const theme of ['light', 'dark']) {
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.goto(`/cases/${caseId}/investigate`)
    await page.evaluate(value => { document.documentElement.dataset.theme = value; localStorage.setItem('nexusai.viewer.theme', value) }, theme)
    await expect(page.getByRole('heading', { name: 'What do you want to verify?' })).toBeVisible()
    await page.screenshot({ path: path.join(reviewDirectory, `empty-${theme}-1440.png`), fullPage: true })
    await page.setViewportSize({ width: 390, height: 844 })
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
    const compactSend = await page.getByRole('button', { name: 'Ask', exact: true }).boundingBox()
    expect(compactSend.width).toBeLessThanOrEqual(52)
    await page.screenshot({ path: path.join(reviewDirectory, `empty-${theme}-390.png`), fullPage: true })
    await page.setViewportSize({ width: 375, height: 812 })
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.locator('.thread-composer__scope > summary').click()
    await page.getByRole('button', { name: 'CDR', exact: true }).click()
    await page.getByLabel('Ask a question about this case').fill(answered.query_understanding.original_question)
    await page.getByRole('button', { name: 'Ask', exact: true }).click()
    await expect(page.locator('.answer-claim')).toBeVisible()
    await expect(page.getByText('Evidence meaning', { exact: true })).toBeVisible()
    await page.evaluate(() => { document.activeElement?.blur(); window.scrollTo(0, 0) })
    await page.screenshot({ path: path.join(reviewDirectory, `answered-${theme}-1440.png`), fullPage: true })

    await page.setViewportSize({ width: 390, height: 844 })
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0)
    await page.screenshot({ path: path.join(reviewDirectory, `answered-${theme}-390.png`), fullPage: true })

    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.goto(`/cases/${caseId}/investigate`)
    await page.getByLabel('Ask a question about this case').fill(clarification.query_understanding.original_question)
    await page.getByRole('button', { name: 'Ask', exact: true }).click()
    await expect(page.getByText('One detail is needed', { exact: true })).toBeVisible()
    await page.evaluate(() => { document.activeElement?.blur(); window.scrollTo(0, 0) })
    await page.screenshot({ path: path.join(reviewDirectory, `clarification-${theme}-1440.png`), fullPage: true })
  }
})
