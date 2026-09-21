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

    await page.route('**/api/v1/forensics/cases', async route => {
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
        contract_version: 'forensics.case-governance/v1', default_case_id: 'records-demo-verified',
        cases: [{ case_id: 'records-demo-verified', collection_id: 'records-demo-verified', display_name: 'Verified Records Demo', selectable: true }],
      }) })
    })

    await page.route('**/api/v1/forensics/cases/records-demo-verified', async route => {
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
        case_id: 'records-demo-verified', collection_id: 'records-demo-verified', display_name: 'Verified Records Demo',
        purpose: 'canonical_pilot', case_status: 'active', visibility: 'analyst', security_classification: 'internal',
      }) })
    })

    await page.route('**/api/v1/forensics/cases/records-demo-verified/manifest', async route => {
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
        case_id: 'records-demo-verified', collection_id: 'records-demo-verified', read_only: true, deletion_count: 0,
        resources: [], job_statuses: [], case_metadata: { warnings: [] },
      }) })
    })

    await page.route('**/api/v1/forensics/cases/records-demo-verified/evidence**', async route => {
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [], count: 0 }) })
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
            { record_type: 'ipdr', accepted_rows: 410, duplicate_rows: 0 },
            { record_type: 'anpr', accepted_rows: 750, duplicate_rows: 0 },
            { record_type: 'subscriber', accepted_rows: 250, duplicate_rows: 0 },
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
            { name: 'call_type_breakdown', description: 'CDR event counts by call type and direction', route: ['records_sql'] },
            { name: 'case_readiness', description: 'Check readiness', route: ['records_sql'] },
            { name: 'anpr_sightings', description: 'Exact ANPR sightings', route: ['records_sql'] },
            {
              name: 'anpr_camera_sequence', description: 'Camera sequence for one plate', route: 'records',
              operation_id: 'anpr.camera_sequence', family_id: 'anpr_vehicles',
              inputs: [{ name: 'target', label: 'Exact target', required: true, accepted_kinds: ['plate'], description: 'Exact authorized plate' }],
              measures: ['sighting count', 'elapsed time'], group_by: ['observation time', 'camera'],
              calculation: 'Orders exact observations for one plate by supplied timestamp and camera.',
              example_query: 'Show the chronological camera sequence for plate {target} from {date_from} to {date_to}',
              output_description: 'Cited camera observation sequence for an exact plate.',
              limitations: ['No ownership, driver, occupants, association, or route is inferred.'],
            },
            { name: 'anpr_camera_activity', description: 'ANPR camera activity', route: ['records_sql'] },
            { name: 'anpr_co_travel', description: 'Same-camera temporal co-observations', route: ['records_sql'] },
            { name: 'anpr_route_timing', description: 'Consecutive sighting timing', route: ['records_sql'] },
            { name: 'anpr_plate_variants', description: 'Observed plate variants', route: ['records_sql'] },
            { name: 'anpr_timeline', description: 'Source-bound plate timeline', route: ['records_sql'] },
            {
              name: 'subscriber_identity_lookup', description: 'Privacy-safe exact subscriber lookup', route: 'records',
              operation_id: 'subscriber.identity_lookup', family_id: 'subscriber_identity',
              inputs: [{ name: 'target', label: 'Exact target', required: true, accepted_kinds: ['MSISDN', 'subscriber reference', 'IMSI', 'IMEI'], description: 'Exact authorized non-CNIC identifier' }],
              calculation: 'Matches exact identifiers and masks CNIC.', example_query: 'Look up subscriber identity observations for {target}',
              limitations: ['An identifier match is not proof of identity or ownership.'],
            },
            { name: 'subscriber_validity_timeline', description: 'Explicit subscriber validity observations', route: ['records_sql'] },
            { name: 'subscriber_device_links', description: 'Explicit SIM and device observations', route: ['records_sql'] },
            { name: 'subscriber_status_summary', description: 'Subscriber status and review summary', route: ['records_sql'] },
            { name: 'subscriber_conflict_audit', description: 'Subscriber identity conflict audit', route: ['records_sql'] },
            { name: 'subscriber_reuse_candidates', description: 'Subscriber identifier reuse review candidates', route: ['records_sql'] },
            { name: 'entity_activity', description: 'Discover exact authorized case identifiers', route: 'records' },
          ],
        }),
      })
    })
  })

  test('runs canonical raw-field examples and renders auditable rows', async ({ page }) => {
    const queryBodies = []
    await page.route('**/api/v1/forensics/cases/records-demo-verified/query', async route => {
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

    await expect(page.getByLabel('Analysis locked to active case records-demo-verified')).toBeVisible()
    await page.getByRole('button', { name: /Advanced options/ }).click()
    await expect(page.getByText('Runtime Field Queries')).toBeVisible()

    await page.getByRole('button', { name: /CDR GPRS rows/ }).click()
    await expect.poll(() => queryBodies.length).toBe(1)
    expect(queryBodies[0]).toMatchObject({
      tenant_id: 'default',
      collection_id: 'records-demo-verified',
      query: 'show CDR records where call type is GPRS limit 3 oldest first',
      template: 'canonical_records',
      limit: 25,
    })

    await expect(page.getByLabel('Deterministic Template')).toHaveValue('canonical_records')
    await expect(page.getByText('Found 5,861 CDR records where call_type is GPRS')).toBeVisible()

    await page.getByRole('button', { name: 'All records' }).click()
    await expect(page.getByRole('columnheader', { name: 'Call Type' })).toBeVisible()
    await expect(page.getByRole('cell', { name: 'seed_cdr_large.csv' })).toBeVisible()
    await expect(page.getByRole('cell', { name: 'GPRS' })).toBeVisible()
    await expect(page.getByRole('cell', { name: '35678901123747' })).toBeVisible()
  })

  test('auto-promotes typed runtime filters to canonical records', async ({ page }) => {
    const queryBodies = []
    await page.route('**/api/v1/forensics/cases/records-demo-verified/query', async route => {
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
    await page.getByRole('button', { name: /Analyze/ }).click()

    await expect.poll(() => queryBodies.length).toBe(1)
    expect(queryBodies[0].template).toBe('canonical_records')
    expect(queryBodies[0].query).toBe('show CDR records where duration seconds > 60 limit 3')
  })

  test('leaves analytical natural-language questions for the governed planner', async ({ page }) => {
    const queryBodies = []
    await page.route('**/api/v1/forensics/cases/records-demo-verified/query', async route => {
      const body = route.request().postDataJSON()
      queryBodies.push(body)
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          intent: 'records',
          template: 'call_type_breakdown',
          route: ['records_sql'],
          answer: { records_summary: 'Exact CDR call-type breakdown completed.' },
          records: { row_count: 1, call_type_breakdown: [{ call_type: 'VOICE', direction: 'OUTGOING', count: 42 }] },
          enterprise: { status: 'answered', summary: 'Exact CDR call-type breakdown completed.' },
        }),
      })
    })

    await page.goto('/app/records')
    await page.getByLabel('Natural Query').fill('show CDR call type breakdown')
    await page.getByRole('button', { name: /Analyze/ }).click()

    await expect.poll(() => queryBodies.length).toBe(1)
    expect(queryBodies[0].template).toBeUndefined()
    expect(queryBodies[0].query).toBe('show CDR call type breakdown')
    await expect(page.getByText('Exact CDR call-type breakdown completed.')).toBeVisible()
  })

  test('discovers executable identifiers and submits transparent template scope', async ({ page }) => {
    const queryBodies = []
    await page.route('**/api/v1/forensics/cases/records-demo-verified/query', async route => {
      const body = route.request().postDataJSON()
      queryBodies.push(body)
      const discovering = body.template === 'entity_activity'
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          intent: 'records', template: body.template, route: ['records_sql'],
          answer: { records_summary: discovering ? 'Authorized case identifiers discovered.' : 'Exact plate sequence completed.' },
          enterprise: {
            status: 'answered', summary: discovering ? 'Authorized case identifiers discovered.' : 'Exact plate sequence completed.',
            coverage: { valid_target_examples: ['ABC***'] },
            data_grid: {
              columns: discovering ? [{ key: 'entity_type', header: 'Type' }, { key: 'entity_value', header: 'Exact Identifier' }] : [],
              rows: discovering ? [{ entity_type: 'primary', entity_value: 'ABC-123', record_types: ['anpr'] }] : [],
            },
          },
        }),
      })
    })

    await page.goto('/app/records')
    await page.getByRole('button', { name: /Discover exact case identifiers/ }).click()
    await expect.poll(() => queryBodies.length).toBe(1)
    expect(queryBodies[0]).toMatchObject({ template: 'entity_activity' })
    expect(queryBodies[0].target).toBeUndefined()
    await expect(page.getByRole('button', { name: 'ABC-123' })).toBeVisible()
    await expect(page.getByText(/Masked coverage hints \(not executable\): ABC\*\*\*/)).toBeVisible()
    await expect(page.locator('#forensic-recent-targets option[value="ABC***"]')).toHaveCount(0)
    await page.getByRole('button', { name: 'ABC-123' }).click()

    await page.getByRole('button', { name: /Advanced options/ }).click()
    await page.getByLabel('Start Time').fill('2026-07-01T10:00')
    await page.getByLabel('End Time').fill('2026-07-02T10:00')
    await page.getByRole('button', { name: /Show Templates/ }).click()
    const templateCard = page.locator('article.records-template-card').filter({ hasText: 'Anpr Camera Sequence' })
    await templateCard.getByText('Inputs & calculation').click()
    await expect(templateCard.getByText(/Target: Required/)).toBeVisible()
    await expect(templateCard.getByText(/Orders exact observations/)).toBeVisible()
    await expect(templateCard.getByText(/ABC-123/)).toBeVisible()
    await templateCard.getByRole('button', { name: 'Use workflow' }).click()
    await page.getByRole('button', { name: /Analyze/ }).click()

    await expect.poll(() => queryBodies.length).toBe(2)
    expect(queryBodies[1]).toMatchObject({ template: 'anpr_camera_sequence', target: 'ABC-123' })
    expect(queryBodies[1].date_from).toContain('2026-07-01T')
    expect(queryBodies[1].date_to).toContain('2026-07-02T')
  })

  test('returns to automatic planning after a preset query', async ({ page }) => {
    const queryBodies = []
    await page.route('**/api/v1/forensics/cases/records-demo-verified/query', async route => {
      const body = route.request().postDataJSON()
      queryBodies.push(body)
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          intent: 'records',
          template: body.template || 'cross_family_correlation',
          route: ['records_sql'],
          answer: { records_summary: 'Deterministic query completed.' },
          records: { row_count: 0, total_count: 0, canonical_records: [] },
          enterprise: { status: 'answered', summary: 'Deterministic query completed.' },
        }),
      })
    })

    await page.goto('/app/records')
    await page.getByRole('button', { name: 'Source audit', exact: true }).click()
    await expect.poll(() => queryBodies.length).toBe(1)
    expect(queryBodies[0].template).toBe('source_file_audit')

    await page.getByLabel('Natural Query').fill('correlate 35678901234567 across record families')
    await page.getByRole('button', { name: /Analyze/ }).click()

    await expect.poll(() => queryBodies.length).toBe(2)
    expect(queryBodies[1]).not.toHaveProperty('template')
    expect(queryBodies[1].query).toBe('correlate 35678901234567 across record families')
  })

  test('renders provenance-bound Phase 6 CDR typed visuals without inferring values', async ({ page }) => {
    await page.route('**/api/v1/forensics/cases/records-demo-verified/query', async route => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          contract_version: 'forensics.enterprise-response/v1',
          request_id: 'phase6-browser-1',
          case_id: 'records-demo-verified',
          collection_id: 'records-demo-verified',
          status: 'answered',
          executive_answer: 'Two deterministic CDR contacts were ranked.',
          deterministic_findings: [{ id: 'finding-cdr-1', statement: 'Two contacts were ranked.', measures: { row_count: 2 }, citation_ids: ['citation-cdr-query-1'] }],
          semantic_evidence: [{ id: 'citation-cdr-query-1', tenant_id: 'default', collection_id: 'records-demo-verified', evidence_id: '', version_id: '', locator: 'table:forensic.cdr_records;template:frequent_contacts', source_name: 'forensic.cdr_records' }],
          inferred_relationships: [], unsupported_operations: [], limitations: [], next_actions: [],
          tables: [{ id: 'primary-results', title: 'Frequent contacts', columns: [{ key: 'dialed_number', label: 'Dialed number', type: 'string' }, { key: 'total_interactions', label: 'Interactions', type: 'integer' }], rows: [{ dialed_number: '923001111111', total_interactions: 5 }, { dialed_number: '923002222222', total_interactions: 3 }] }],
          visualizations: [{ id: 'cdr-contact-frequency', type: 'bar', title: 'Frequent contacts', citation_ids: ['citation-cdr-query-1'], spec: { category_field: 'dialed_number', value_field: 'total_interactions', rows: [{ dialed_number: '923001111111', total_interactions: 5 }, { dialed_number: '923002222222', total_interactions: 3 }] } }],
          execution_trace: { route: ['records_sql'], tool_ids: ['cdr.frequent_contacts'], adapter_ids: ['nexusai.adapter.cdr'], model_roles: [], parameters: { template: 'frequent_contacts', family_id: 'communications_cdr' }, latency_ms: { db: 4 } },
        }),
      })
    })

    await page.goto('/app/records')
    await expect(page.getByRole('link', { name: 'CDR Specialist' })).toHaveAttribute('href', /Communications_CDR_Analyst\/chat.*case=records-demo-verified/)
    await page.getByLabel('Natural Query').fill('show frequent contacts')
    await page.getByRole('button', { name: /Analyze/ }).click()
    await expect(page.getByRole('button', { name: 'Analyst summary' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Visual analysis' })).toBeVisible()
    await expect(page.getByRole('figure', { name: 'Frequent contacts' })).toBeVisible()
    await expect(page.getByRole('figure', { name: 'Frequent contacts' }).getByText('923001111111')).toBeVisible()
    await expect(page.getByText('Value: {}')).toHaveCount(0)
    await page.getByRole('button', { name: 'Evidence', exact: true }).click()
    await expect(page.getByText('Source: forensic.cdr_records', { exact: true })).toBeVisible()
  })

  test('renders Phase 6.2 IPDR typed endpoint visuals and specialist handoff without inferred DNS or ownership', async ({ page }) => {
    await page.route('**/api/v1/forensics/cases/records-demo-verified/query', async route => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          contract_version: 'forensics.enterprise-response/v1',
          request_id: 'phase6-ipdr-browser-1',
          case_id: 'records-demo-verified',
          collection_id: 'records-demo-verified',
          status: 'answered_with_limitations',
          executive_answer: 'Two explicit endpoint combinations were counted.',
          deterministic_findings: [{ id: 'finding-ipdr-1', statement: 'Two endpoint combinations were counted.', measures: { row_count: 2 }, citation_ids: ['citation-ipdr-query-1'] }],
          semantic_evidence: [{ id: 'citation-ipdr-query-1', tenant_id: 'default', collection_id: 'records-demo-verified', evidence_id: '', version_id: '', locator: 'table:forensic.records;record_type:ipdr;template:ipdr_endpoint_summary', source_name: 'forensic.records' }],
          inferred_relationships: [], unsupported_operations: [],
          limitations: ['No subscriber ownership or DNS resolution was inferred.'], next_actions: [],
          tables: [{ id: 'primary-results', title: 'Network endpoints', columns: [{ key: 'source_ip', label: 'Source IP', type: 'string' }, { key: 'destination_ip', label: 'Destination IP', type: 'string' }, { key: 'session_count', label: 'Sessions', type: 'integer' }], rows: [{ source_ip: '10.20.1.7', destination_ip: '8.8.8.8', session_count: 7 }, { source_ip: '2001:db8::10', destination_ip: '2001:4860:4860::8888', session_count: 3 }] }],
          visualizations: [{ id: 'ipdr-endpoint-volume', type: 'bar', title: 'Network endpoint sessions', citation_ids: ['citation-ipdr-query-1'], spec: { category_fields: ['source_ip', 'destination_ip'], value_field: 'session_count', rows: [{ source_ip: '10.20.1.7', destination_ip: '8.8.8.8', session_count: 7 }, { source_ip: '2001:db8::10', destination_ip: '2001:4860:4860::8888', session_count: 3 }] } }],
          execution_trace: { route: ['records_sql'], tool_ids: ['ipdr.endpoint_summary'], adapter_ids: ['nexusai.adapter.ipdr'], model_roles: [], parameters: { template: 'ipdr_endpoint_summary', family_id: 'network_ipdr' }, latency_ms: { db: 5 } },
        }),
      })
    })

    await page.goto('/app/records')
    await expect(page.getByRole('link', { name: 'IPDR Specialist' })).toHaveAttribute('href', /Network_IPDR_Capture_Analyst\/chat.*case=records-demo-verified/)
    await page.getByLabel('Natural Query').fill('show IPDR endpoint summary')
    await page.getByRole('button', { name: /Analyze/ }).click()
    await expect(page.getByRole('heading', { name: 'Visual analysis' })).toBeVisible()
    await expect(page.getByRole('figure', { name: 'Network endpoint sessions' })).toBeVisible()
    await expect(page.getByText('10.20.1.7 → 8.8.8.8')).toBeVisible()
    await expect(page.getByText('Value: {}')).toHaveCount(0)
    await page.getByRole('button', { name: 'Evidence', exact: true }).click()
    await expect(page.getByText('citation-ipdr-query-1')).toBeVisible()
    await page.getByRole('button', { name: 'Audit details' }).click()
    await expect(page.getByText('No subscriber ownership or DNS resolution was inferred.').first()).toBeVisible()
  })

  test('renders the Phase 6.3 ANPR command center, typed camera activity, and bounded specialist handoff', async ({ page }) => {
    const queryBodies = []
    await page.route('**/api/v1/forensics/cases/records-demo-verified/query', async route => {
      queryBodies.push(route.request().postDataJSON())
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          contract_version: 'forensics.enterprise-response/v1',
          request_id: 'phase6-anpr-browser-1',
          case_id: 'records-demo-verified',
          collection_id: 'records-demo-verified',
          status: 'answered_with_limitations',
          executive_answer: 'Two cameras were counted from explicit ANPR observations.',
          deterministic_findings: [{ id: 'finding-anpr-1', statement: 'Two camera groups were counted.', measures: { row_count: 2 }, citation_ids: ['citation-anpr-query-1'] }],
          semantic_evidence: [{ id: 'citation-anpr-query-1', tenant_id: 'default', collection_id: 'records-demo-verified', evidence_id: '', version_id: '', locator: 'table:forensic.records;record_type:anpr;template:anpr_camera_activity', source_name: 'forensic.records' }],
          inferred_relationships: [], unsupported_operations: [],
          limitations: ['ANPR results do not establish vehicle ownership, occupants, association, road route, or image-level OCR accuracy.'], next_actions: [],
          tables: [{ id: 'primary-results', title: 'Camera activity', columns: [{ key: 'camera_id', label: 'Camera', type: 'string' }, { key: 'sighting_count', label: 'Sightings', type: 'integer' }], rows: [{ camera_id: 'CAM-7', location: 'Model Town', sighting_count: 12 }, { camera_id: 'CAM-9', location: 'North Road', sighting_count: 8 }] }],
          visualizations: [{ id: 'anpr-camera-activity', type: 'bar', title: 'ANPR camera activity', citation_ids: ['citation-anpr-query-1'], spec: { category_fields: ['camera_id', 'location'], value_field: 'sighting_count', rows: [{ camera_id: 'CAM-7', location: 'Model Town', sighting_count: 12 }, { camera_id: 'CAM-9', location: 'North Road', sighting_count: 8 }] } }],
          execution_trace: { route: ['records_sql'], tool_ids: ['anpr.camera_activity'], adapter_ids: ['nexusai.adapter.anpr'], model_roles: [], parameters: { template: 'anpr_camera_activity', family_id: 'anpr_vehicles' }, latency_ms: { db: 4 } },
        }),
      })
    })

    await page.goto('/app/records')
    await expect(page.getByRole('heading', { name: 'ANPR Intelligence' })).toBeVisible()
    await expect(page.getByText(/No ownership, passenger, association, road-route, or image-OCR claim/)).toBeVisible()
    await expect(page.getByRole('link', { name: 'ANPR Specialist' })).toHaveAttribute('href', /Vehicle_ANPR_Geospatial_Analyst\/chat.*case=records-demo-verified/)
    await page.getByRole('button', { name: 'Camera activity' }).first().click()
    await expect.poll(() => queryBodies.length).toBe(1)
    expect(queryBodies[0]).toMatchObject({ template: 'anpr_camera_activity', collection_id: 'records-demo-verified' })
    expect(queryBodies[0].target || '').toBe('')
    await expect(page.getByRole('heading', { name: 'Visual analysis' })).toBeVisible()
    await expect(page.getByRole('figure', { name: 'ANPR camera activity' })).toBeVisible()
    await expect(page.getByText('Value: {}')).toHaveCount(0)
    await page.getByRole('button', { name: 'Evidence', exact: true }).click()
    await expect(page.getByText('citation-anpr-query-1')).toBeVisible()
    await page.getByRole('button', { name: 'Audit details' }).click()
    await expect(page.getByText(/do not establish vehicle ownership/).first()).toBeVisible()

    await page.getByLabel('Exact Target').fill('ABC-123')
    await page.getByRole('button', { name: 'Exact sightings' }).first().click()
    await expect.poll(() => queryBodies.length).toBe(2)
    expect(queryBodies[1]).toMatchObject({ template: 'anpr_sightings', target: 'ABC-123' })
  })

  test('renders the Phase 7.1 privacy-safe subscriber command center and status result', async ({ page }) => {
    const queryBodies = []
    await page.route('**/api/v1/forensics/cases/records-demo-verified/query', async route => {
      queryBodies.push(route.request().postDataJSON())
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          contract_version: 'forensics.enterprise-response/v1', request_id: 'phase7-subscriber-browser-1',
          case_id: 'records-demo-verified', collection_id: 'records-demo-verified', status: 'answered_with_limitations',
          executive_answer: 'Two explicit subscriber status groups were counted.',
          deterministic_findings: [{ id: 'finding-subscriber-1', statement: 'Two status groups were counted.', measures: { row_count: 2 }, citation_ids: ['citation-subscriber-query-1'] }],
          semantic_evidence: [{ id: 'citation-subscriber-query-1', tenant_id: 'default', collection_id: 'records-demo-verified', locator: 'table:forensic.records;record_type:subscriber;template:subscriber_status_summary', source_name: 'forensic.records' }],
          inferred_relationships: [], unsupported_operations: [],
          limitations: ['CNIC is masked, names are omitted, and no identity, ownership, or current-control claim is inferred.'], next_actions: [],
          tables: [{ id: 'primary-results', title: 'Subscriber status', columns: [{ key: 'subscriber_status', label: 'Status', type: 'string' }, { key: 'row_count', label: 'Rows', type: 'integer' }], rows: [{ subscriber_status: 'ACTIVE', row_count: 7 }, { subscriber_status: 'SUSPENDED', row_count: 2 }] }],
          visualizations: [{ id: 'subscriber-status-summary', type: 'bar', title: 'Subscriber status and review posture', citation_ids: ['citation-subscriber-query-1'], spec: { category_field: 'subscriber_status', value_field: 'row_count', rows: [{ subscriber_status: 'ACTIVE', row_count: 7 }, { subscriber_status: 'SUSPENDED', row_count: 2 }] } }],
          execution_trace: { route: ['records_sql'], tool_ids: ['subscriber.status_summary'], adapter_ids: ['nexusai.adapter.subscriber_identity'], model_roles: [], parameters: { template: 'subscriber_status_summary', family_id: 'subscriber_identity' }, latency_ms: { db: 4 } },
        }),
      })
    })

    await page.goto('/app/records')
    await expect(page.getByRole('heading', { name: 'Subscriber Identity Intelligence' })).toBeVisible()
    await expect(page.getByText(/Full CNIC and names stay out of default results/)).toBeVisible()
    await expect(page.getByRole('link', { name: 'Subscriber Specialist' })).toHaveAttribute('href', /Subscriber_Identity_Analyst\/chat.*case=records-demo-verified/)
    await page.getByRole('button', { name: 'Status summary' }).click()
    await expect.poll(() => queryBodies.length).toBe(1)
    expect(queryBodies[0]).toMatchObject({ template: 'subscriber_status_summary', collection_id: 'records-demo-verified' })
    expect(queryBodies[0].target || '').toBe('')
    const statusFigure = page.getByRole('figure', { name: 'Subscriber status and review posture' })
    await expect(statusFigure).toBeVisible()
    await expect(statusFigure.getByText('ACTIVE', { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Audit details' }).click()
    await expect(page.getByText(/CNIC is masked/).first()).toBeVisible()
  })

  test('locks embedded data management and exact queries to the active case', async ({ page }) => {
    const structuredBodies = []
    const evidenceUploads = []
    const legacyIngests = []
    const collectionCreates = []
    const collectionLists = []
    page.on('request', request => {
      const url = new URL(request.url())
      if (url.pathname === '/api/agents/collections' && request.method() === 'POST') collectionCreates.push(request.url())
      if (url.pathname === '/api/agents/collections' && request.method() === 'GET') collectionLists.push(request.url())
    })
    await page.route('**/api/records/query', async route => {
      structuredBodies.push(route.request().postDataJSON())
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ records: [], count: 0, total: 0 }) })
    })
    await page.route('**/api/agents/collections/records-demo-verified/upload', async route => {
      evidenceUploads.push({ url: route.request().url(), body: route.request().postData() || '' })
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ records_status: 'queued', entry: 'bounded.csv' }) })
    })
    await page.route('**/api/records/ingest', async route => {
      legacyIngests.push(route.request().postData() || '')
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ batch: { id: 'batch-bound', source_file: 'bounded.csv', collection_name: 'records-demo-verified', record_type: 'cdr', row_count: 1 } }) })
    })

    await page.goto('/app/records')
    await expect(page.getByLabel('Analysis locked to active case records-demo-verified')).toBeVisible()
    await page.getByRole('button', { name: /Open/ }).click()

    const collection = page.getByLabel('Active Case Collection')
    await expect(collection).toHaveValue('records-demo-verified')
    await expect(collection).toHaveAttribute('readonly', '')
    await expect(page.locator('#records-collections')).toHaveCount(0)
    expect(collectionLists).toHaveLength(0)

    await page.getByRole('button', { name: 'Shortest call' }).click()
    await expect.poll(() => structuredBodies.length).toBe(1)
    expect(structuredBodies[0]).toMatchObject({ collection_name: 'records-demo-verified', helper: 'shortest_call' })

    await page.getByLabel('Files').setInputFiles({
      name: 'bounded.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from('source_number,target_number,duration_seconds\n03001234567,03007654321,42\n'),
    })
    await page.getByRole('button', { name: 'Ingest Records' }).click()
    await expect.poll(() => evidenceUploads.length).toBe(1)
    await expect.poll(() => legacyIngests.length).toBe(1)

    expect(evidenceUploads[0].url).toContain('/api/agents/collections/records-demo-verified/upload')
    expect(evidenceUploads[0].body).toContain('name="case_id"')
    expect(evidenceUploads[0].body).toContain('records-demo-verified')
    expect(legacyIngests[0]).toContain('name="collection_name"')
    expect(legacyIngests[0]).toContain('records-demo-verified')
    expect(collectionCreates).toHaveLength(0)
  })

  test('discards a late forensic answer after the active case changes', async ({ page }) => {
    let releaseFirstQuery
    let firstQueryStarted = false
    const firstQueryGate = new Promise(resolve => { releaseFirstQuery = resolve })

    await page.route('**/api/v1/forensics/cases', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
      default_case_id: 'records-demo-verified',
      cases: [
        { case_id: 'records-demo-verified', collection_id: 'records-demo-verified', display_name: 'Verified Records Demo', selectable: true },
        { case_id: 'case-beta', collection_id: 'case-beta', display_name: 'Case Beta', selectable: true },
      ],
    }) }))
    await page.route('**/api/v1/forensics/cases/case-beta', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
      case_id: 'case-beta', collection_id: 'case-beta', display_name: 'Case Beta', case_status: 'active', security_classification: 'internal',
    }) }))
    await page.route('**/api/v1/forensics/cases/case-beta/manifest', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ resources: [], job_statuses: [] }) }))
    await page.route('**/api/v1/forensics/cases/case-beta/evidence**', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [], count: 0 }) }))
    await page.route('**/api/v1/forensics/cases/records-demo-verified/query', async route => {
      firstQueryStarted = true
      await Promise.race([firstQueryGate, new Promise(resolve => setTimeout(resolve, 5000))])
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
        intent: 'records', template: 'case_readiness', route: ['records_sql'],
        answer: { records_summary: 'OLD CASE ANSWER MUST NOT RENDER' },
        records: { row_count: 0 },
        enterprise: { status: 'answered', summary: 'OLD CASE ANSWER MUST NOT RENDER' },
      }) })
    })

    await page.goto('/app/records')
    await page.getByLabel('Natural Query').fill('show case readiness')
    await page.getByRole('button', { name: /Analyze/ }).click()
    await expect.poll(() => firstQueryStarted).toBe(true)

    await page.getByLabel('Active collection').selectOption('case-beta')
    await expect(page).toHaveURL(/\/app\/cases\/case-beta\/ask$/)
    await expect(page.getByLabel('Analysis locked to active case case-beta')).toBeVisible()
    releaseFirstQuery()

    await expect(page.getByText('OLD CASE ANSWER MUST NOT RENDER')).toHaveCount(0)
  })
})
