import { expect, test } from '@playwright/test'
import { liveFixture } from './live-fixtures.js'

const fixture = liveFixture('CDR-04.json')

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default' }
  })
})

test('asks, renders a cited answer, and keeps the document within the viewport', async ({ page }) => {
  await page.route('**/query/hybrid', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fixture) }))
  await page.goto('/cases/nexusai-forensic-demo/investigate')
  await page.getByLabel('Ask a question about this case').fill('Break down CDR records by call type')
  await page.getByRole('button', { name: 'Ask', exact: true }).click()
  await expect(page.locator('.answer-claim')).toContainText('8,642')
  await expect(page.locator('.answer-claim')).toContainText('CDR')
  await expect(page.getByRole('columnheader', { name: 'Call type' })).toBeVisible()
  await expect(page.locator('.citation-marker').first()).toHaveAttribute('href', /row_hash=/)
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  expect(overflow).toBeLessThanOrEqual(0)
})

test('supports the keyboard-only ask and source path', async ({ page }) => {
  await page.route('**/query/hybrid', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fixture) }))
  await page.goto('/cases/nexusai-forensic-demo/investigate')
  await expect(page.locator('#workspace-main')).toBeVisible()
  await page.keyboard.press('Tab')
  await page.keyboard.press('Enter')
  await expect(page.locator('#workspace-main')).toBeFocused()
  await page.getByLabel('Ask a question about this case').focus()
  await expect(page.getByLabel('Ask a question about this case')).toBeFocused()
  await page.keyboard.type('Break down CDR records by call type')
  await page.keyboard.press('Tab')
  await expect(page.getByRole('button', { name: 'Ask', exact: true })).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(page.locator('.citation-marker').first()).toBeVisible()
  await page.locator('.citation-marker').first().focus()
  await expect(page.locator('.citation-marker').first()).toBeFocused()
  const preview = page.getByRole('tooltip')
  await expect(preview).toContainText('Exact locator')
  const box = await preview.boundingBox()
  const viewport = page.viewportSize()
  expect(box.x).toBeGreaterThanOrEqual(0)
  expect(box.y).toBeGreaterThanOrEqual(0)
  expect(box.x + box.width).toBeLessThanOrEqual(viewport.width)
  expect(box.y + box.height).toBeLessThanOrEqual(viewport.height)
  await page.keyboard.press('Escape')
  await expect(preview).toBeHidden()
})

test('renders every investigation outcome without collapsing its meaning', async ({ page }) => {
  const questions = {
    clarify: 'Which identifier should I use for the requested comparison?',
    zero: 'Find matching records outside the retained period',
    unsupported: 'Predict what the subject will do next',
    partial: 'Show the available ANPR observations for the target vehicle',
    processing: 'Summarize the evidence that is still processing',
    failed: 'Compare call activity with unavailable tower history',
  }
  const responses = {
    clarify: liveFixture('CDR-07.json'),
    zero: liveFixture('NEG-02.json'),
    unsupported: { intent: 'semantic', answer: { answer: 'This question is outside the evidence analysis available for this case.' } },
    partial: { intent: 'records', collection_id: 'case', enterprise: { result_state: 'results_present', executive_answer: 'A bounded result is available.', proof_state: { rows: 'bounded' }, data_grid: { columns: [], rows: [] } } },
    processing: { intent: 'records', collection_id: 'case', enterprise: { result_state: 'processing', processing_state: 'processing', executive_answer: 'Evidence is still processing.', data_grid: { columns: [], rows: [] } } },
  }
  await page.route('**/query/hybrid', async route => {
    const query = JSON.parse(route.request().postData() || '{}').query
    if (query === questions.failed) return route.fulfill({ status: 500, headers: { 'x-request-id': 'QRY-4821' }, body: '{}' })
    const state = Object.keys(questions).find(key => questions[key] === query)
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(responses[state]) })
  })
  await page.goto('/cases/nexusai-forensic-demo/investigate')
  const field = page.getByLabel('Ask a question about this case')
  const checks = [
    ['clarify', 'One detail is needed'],
    ['zero', 'Analysis complete · no match'],
    ['unsupported', 'Analysis unavailable'],
    ['partial', 'Analysis complete with limits'],
    ['processing', 'Evidence is being processed'],
    ['failed', 'Analysis could not be completed'],
  ]
  for (const [state, expected] of checks) {
    await field.fill(questions[state])
    await page.getByRole('button', { name: 'Ask', exact: true }).click()
    await expect(page.getByText(expected, { exact: true }).first()).toBeVisible()
    if (state === 'unsupported' || state === 'failed') {
      const latestResult = page.locator('.investigation-result').last()
      await expect(latestResult.getByText('Supported finding', { exact: true })).toHaveCount(0)
      await expect(latestResult.getByText('0 sources', { exact: true })).toHaveCount(0)
    }
  }
})

test('renders a document answer with its page and passage citation', async ({ page }) => {
  const documentAnswer = liveFixture('DOC-02.json')
  await page.route('**/query/hybrid', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(documentAnswer),
  }))
  await page.goto('/cases/nexusai-multimodal-product-acceptance/investigate')
  await page.getByLabel('Ask a question about this case').fill('Which document mentions contact number 03001234567?')
  await page.getByRole('button', { name: 'Ask', exact: true }).click()

  const result = page.locator('.investigation-result').last()
  await expect(result.getByText('nexusai-multimodal-acceptance-brief.pdf', { exact: true }).first()).toBeVisible()
  await expect(result.getByText('No verified answer is available', { exact: true })).toHaveCount(0)
  await expect(result.getByText('1 source', { exact: true })).toBeVisible()

  const marker = result.locator('.citation-marker').first()
  await expect(marker).toHaveAttribute('href', /page=1/)
  await expect(marker).toHaveAttribute('href', /passage=1/)
  await marker.hover()
  await expect(page.getByRole('tooltip')).toContainText('page 1 · passage 1')
  await expect(page.getByRole('tooltip')).toContainText('Source type not reported')
})

test('supports forced light and dark themes', async ({ page }) => {
  await page.goto('/cases/nexusai-forensic-demo/investigate')
  await expect(page.getByLabel('Ask a question about this case')).toBeVisible()
  await page.getByRole('button', { name: /^Appearance/ }).click()
  for (const [theme, label] of [['light', /^Light/], ['dark', /^Dark/]]) {
    await page.getByRole('radio', { name: label }).check()
    await expect(page.locator('body')).toHaveCSS('color-scheme', theme)
  }
})

test('shows the exact clarification query, re-runs it, and retains the decision trail', async ({ page }) => {
  const first = liveFixture('X-01.json')
  const answer = liveFixture('DOC-02.json')
  const originalQuestion = first.query_understanding.original_question
  const selectedLabel = 'Documents mentioning 03001234567'
  const selectedQuery = 'Which document mentions 03001234567?'
  const sentQueries = []
  await page.route('**/query/hybrid', async route => {
    const query = JSON.parse(route.request().postData() || '{}').query
    sentQueries.push(query)
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(query === originalQuestion ? first : answer),
    })
  })
  await page.goto('/cases/nexusai-multimodal-product-acceptance/investigate')
  await page.getByLabel('Ask a question about this case').fill(originalQuestion)
  await page.getByRole('button', { name: 'Ask', exact: true }).click()
  const choice = page.getByRole('button', { name: new RegExp(selectedLabel) })
  await expect(choice).toContainText(`Ask: “${selectedQuery}”`)
  await choice.click()
  const trail = page.locator('.question-trail')
  await expect(trail.getByRole('heading', { name: 'How this question changed' })).toBeVisible()
  const trailLines = trail.locator('li').first().locator('p')
  await expect(trailLines.nth(0)).toContainText(originalQuestion)
  await expect(trailLines.nth(1)).toContainText(selectedLabel)
  await expect(trailLines.nth(2)).toContainText(selectedQuery)
  await expect(page.locator('.answer-block')).toBeVisible()
  expect(sentQueries).toEqual([originalQuestion, selectedQuery])
})
