import { test, expect } from './coverage-fixtures.js'

test.describe('Records Intelligence forensic runtime queries', () => {
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
        body: JSON.stringify({ agents: true, records: true }),
      })
    })

    await page.route('**/api/agents/collections', async route => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ collections: [{ name: 'records-demo' }], count: 1 }),
      })
    })

    await page.route('**/v1/models', async route => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ data: [{ id: 'deterministic-local' }] }),
      })
    })

    await page.route('**/api/records/batches**', async route => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ batches: [], count: 0 }),
      })
    })

    await page.route('**/api/records/forensic/status**', async route => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          summary: {
            accepted_rows: 18192,
            duplicate_rows: 0,
            kb_assets_total: 8,
            jobs_total: 4,
            failed_jobs: 0,
            rejected_rows: 0,
          },
          record_families: [
            { record_type: 'cdr', accepted_rows: 8639, duplicate_rows: 0 },
            { record_type: 'generic', accepted_rows: 9553, duplicate_rows: 0 },
          ],
          missing_kb_assets: [],
          recent_errors: [],
        }),
      })
    })

    await page.route('**/api/records/forensic/templates', async route => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          templates: [
            { name: 'source_file_audit', description: 'List ingested source files', route: ['records_sql'] },
            { name: 'canonical_records', description: 'Query canonical forensic records', route: ['records_sql'] },
            { name: 'case_readiness', description: 'Check readiness', route: ['records_sql'] },
          ],
        }),
      })
    })
  })

  test('runs canonical raw-field examples and renders auditable rows', async ({ page }) => {
    const queryBodies = []
    await page.route('**/api/records/forensic/query', async route => {
      const body = route.request().postDataJSON()
      queryBodies.push(body)
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          intent: 'records',
          template: 'canonical_records',
          route: ['records_sql'],
          planner: { field_hints: ['call_type'] },
          answer: {
            records_status: '3 rows returned from 5861 matching canonical records.',
            records_summary: 'Found 5,861 CDR records where call_type is GPRS; 3 oldest rows are shown.',
          },
          records: {
            row_count: 3,
            total_count: 5861,
            canonical_records: [
              {
                record_id: 'rec-1',
                record_type: 'cdr',
                source_file: 'seed_cdr_large.csv',
                timestamp: '2026-04-02T00:00:02Z',
                raw_payload: { CALL_TYPE: 'GPRS', IMEI: '35678901123747' },
                metadata: { normalized_fields: { call_type: 'GPRS', duration_seconds: 0 } },
              },
            ],
          },
          enterprise: {
            status: 'answered',
            summary: 'Found 5,861 CDR records where call_type is GPRS; 3 oldest rows are shown.',
            metrics: [{ label: 'Records Row Count', value: 5861, source: 'forensic.records' }],
            data_grid: {
              columns: [
                { key: 'record_type', header: 'Record Type' },
                { key: 'source_file', header: 'Source File' },
                { key: 'call_type', header: 'Call Type' },
                { key: 'imei', header: 'IMEI' },
              ],
              rows: [
                {
                  record_type: 'cdr',
                  source_file: 'seed_cdr_large.csv',
                  call_type: 'GPRS',
                  imei: '35678901123747',
                },
              ],
            },
            provenance: [
              { source: 'records_sql', source_file: 'seed_cdr_large.csv', row_number: 1 },
            ],
          },
        }),
      })
    })

    await page.goto('/app/records')

    await expect(page.getByRole('heading', { name: 'Records Intelligence' })).toBeVisible()
    await expect(page.getByText('Runtime Field Queries')).toBeVisible()

    await page.getByRole('button', { name: /CDR GPRS rows/ }).click()
    await expect.poll(() => queryBodies.length).toBe(1)
    expect(queryBodies[0]).toMatchObject({
      tenant_id: 'default',
      collection_id: 'records-demo',
      query: 'show CDR records where call type is GPRS limit 3 oldest first',
      template: 'canonical_records',
      limit: 25,
    })

    await expect(page.locator('.records-planner .records-chip').filter({ hasText: 'canonical_records' })).toBeVisible()
    await expect(page.getByText('Found 5,861 CDR records where call_type is GPRS')).toBeVisible()

    await page.getByRole('button', { name: 'Data Grid' }).click()
    await expect(page.getByRole('columnheader', { name: 'Call Type' })).toBeVisible()
    await expect(page.getByRole('cell', { name: 'seed_cdr_large.csv' })).toBeVisible()
    await expect(page.getByRole('cell', { name: 'GPRS' })).toBeVisible()
    await expect(page.getByRole('cell', { name: '35678901123747' })).toBeVisible()
  })

  test('auto-promotes typed runtime filters to canonical records', async ({ page }) => {
    const queryBodies = []
    await page.route('**/api/records/forensic/query', async route => {
      const body = route.request().postDataJSON()
      queryBodies.push(body)
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          intent: 'records',
          template: body.template,
          route: ['records_sql'],
          answer: { records_summary: 'Exact canonical query completed.' },
          records: { row_count: 0, total_count: 0, canonical_records: [] },
          enterprise: { status: 'answered', summary: 'Exact canonical query completed.' },
        }),
      })
    })

    await page.goto('/app/records')
    await page.getByLabel('Natural Query').fill('show CDR records where duration seconds > 60 limit 3')
    await page.locator('.records-sidecar-form').getByRole('button', { name: /Run/ }).click()

    await expect.poll(() => queryBodies.length).toBe(1)
    expect(queryBodies[0].template).toBe('canonical_records')
    expect(queryBodies[0].query).toBe('show CDR records where duration seconds > 60 limit 3')
  })
})
