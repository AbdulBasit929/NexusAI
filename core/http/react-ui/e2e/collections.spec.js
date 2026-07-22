import { test, expect } from './coverage-fixtures.js'

// Collections (Knowledge Base) feature page (src/pages/Collections.jsx).
test.describe('Collections page', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/auth/status', async route => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          authEnabled: false,
          staticApiKeyRequired: false,
          user: null,
        }),
      })
    })

    await page.route('**/api/features', async route => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ agents: true }),
      })
    })

    await page.route('**/api/agents/collections', async route => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ collections: [], count: 0 }),
      })
    })

    await page.goto('/app/collections')
  })

  test('renders the knowledge base with an empty state and create control', async ({ page }) => {
    await expect(page).toHaveURL(/\/app\/collections$/)
    await expect(page.getByRole('heading', { name: 'Knowledge Base' })).toBeVisible()
    await expect(page.getByText(/No collections yet/i)).toBeVisible()
    await expect(page.locator('button.btn-primary').filter({ hasText: 'Create' })).toBeVisible()
  })

  test('new-collection name field accepts input', async ({ page }) => {
    const input = page.locator('input, textarea').first()
    await expect(input).toBeVisible()
    await input.fill('my-kb')
    await expect(input).toHaveValue('my-kb')
  })

  test('supports CDR upload, raw search, and source interval inputs', async ({ page }) => {
    const collectionName = 'cdr-ui-e2e'
    const cdrCsv = [
      'call_id,started_at,source_msisdn,target_msisdn,duration_seconds,area',
      'CALL-0001,2026-07-10T08:41:03Z,923001112222,923334445555,183,Gulberg',
      'CALL-0002,2026-07-10T09:02:44Z,923001112222,923336667777,42,DHA',
      'CALL-0003,2026-07-10T09:17:21Z,923008889999,923334445555,7,Gulberg',
    ].join('\n')
    const sourceUrl = 'https://example.test/cdr-batches/daily.csv'
    let sawUpload = false
    let sawSearch = false
    let sawSourcePost = false
    let sourceAdded = false
    let sawRawEntryFallback = false

    await page.route(`**/api/agents/collections/${collectionName}/entries**`, async route => {
      if (route.request().url().includes('/entries/batch-001%2Fcdr_batch.csv')) {
        await route.fulfill({
          contentType: 'application/json',
          status: 500,
          body: JSON.stringify({ error: 'unsupported file type: .csv' }),
        })
        return
      }

      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          entries: ['batch-001/cdr_batch.csv'],
          count: 1,
        }),
      })
    })

    await page.route(`**/api/agents/collections/${collectionName}/entries-raw/**`, async route => {
      sawRawEntryFallback = true
      await route.fulfill({
        contentType: 'text/csv',
        body: cdrCsv,
      })
    })

    await page.route(`**/api/agents/collections/${collectionName}/upload**`, async route => {
      expect(route.request().method()).toBe('POST')
      expect(route.request().postData() || '').toContain('cdr_batch.csv')
      sawUpload = true
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'ok',
          filename: 'cdr_batch.csv',
          key: 'batch-001/cdr_batch.csv',
        }),
      })
    })

    await page.route(`**/api/agents/collections/${collectionName}/search**`, async route => {
      const body = route.request().postDataJSON()
      expect(body.query).toBe('Which CDR record has the shortest duration call on 2026-07-10 in Gulberg?')
      expect(body.max_results).toBe(10)
      sawSearch = true
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          results: [{
            similarity: 0.9921,
            content: 'call_id=CALL-0003, started_at=2026-07-10T09:17:21Z, source_msisdn=923008889999, target_msisdn=923334445555, duration_seconds=7, area=Gulberg',
          }],
          count: 1,
          requested_max_results: 10,
          effective_max_results: 1,
        }),
      })
    })

    await page.route(`**/api/agents/collections/${collectionName}/sources**`, async route => {
      if (route.request().method() === 'POST') {
        const body = route.request().postDataJSON()
        expect(body.url).toBe(sourceUrl)
        expect(body.update_interval).toBe('30m')
        sawSourcePost = true
        sourceAdded = true
        await route.fulfill({
          contentType: 'application/json',
          body: JSON.stringify({ status: 'ok' }),
        })
        return
      }

      const sources = sourceAdded ? [{ url: sourceUrl, update_interval: '30m' }] : []
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ sources, count: sources.length }),
      })
    })

    await page.goto(`/app/collections/${collectionName}`)
    await expect(page.getByRole('heading', { name: collectionName })).toBeVisible()
    await expect(page.getByText('batch-001/cdr_batch.csv')).toBeVisible()

    await page.locator('button[title="View Content"]').click()
    await expect.poll(() => sawRawEntryFallback).toBe(true)
    await expect(page.getByText('call_id,started_at,source_msisdn,target_msisdn,duration_seconds,area')).toBeVisible()
    await expect(page.getByText(/CALL-0003/)).toBeVisible()
    await page.locator('.collection-detail-modal-header button').click()

    await page.locator('input[type="file"]').setInputFiles({
      name: 'cdr_batch.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(cdrCsv),
    })
    await page.getByRole('button', { name: /Upload/ }).click()
    await expect.poll(() => sawUpload).toBe(true)

    await page.getByRole('button', { name: /Search/ }).click()
    await page.getByLabel('Query').fill('Which CDR record has the shortest duration call on 2026-07-10 in Gulberg?')
    await expect(page.getByLabel('Max Results')).toHaveValue('10')
    await page.locator('form.collection-detail-search-form').getByRole('button', { name: /Search/ }).click()
    await expect.poll(() => sawSearch).toBe(true)
    await expect(page.getByText('Effective max: 1')).toBeVisible()
    await expect(page.getByText('Auto-capped to available indexed chunks')).toBeVisible()
    await expect(page.getByText(/CALL-0003/)).toBeVisible()
    await expect(page.getByText(/duration_seconds=7/)).toBeVisible()

    await page.getByRole('button', { name: /Sources/ }).click()
    await page.getByLabel('URL').fill(sourceUrl)
    await page.getByLabel('Update Interval').fill('30m')
    await page.locator('form.collection-detail-source-form').getByRole('button', { name: /Add Source/ }).click()
    await expect.poll(() => sawSourcePost).toBe(true)
    await expect(page.getByText(sourceUrl)).toBeVisible()
    await expect(page.getByText('30m')).toBeVisible()
  })
})
