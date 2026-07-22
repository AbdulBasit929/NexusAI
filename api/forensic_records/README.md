# Forensic Records Webhook API

Go service for asynchronous Knowledge Base + structured-record ingestion.

Endpoint:

```text
POST /webhooks/records/upload
GET  /evidence
GET  /evidence/{evidence_id}
POST /query/hybrid
GET  /query/templates
POST /reports/generate
```

Multipart fields:

- `file` - required CSV file
- `collection_id` - required Knowledge Base / case collection identifier
- `tenant_id` - optional, defaults to `default`
- `case_id` - optional case/workspace identifier stored on the unified evidence item
- `user_id` - optional
- `file_id` - optional, generated when absent
- `record_type` - optional, `auto`, `cdr`, `ipdr`, `anpr`, `subscriber`, `tower_location`, `transaction`, `access_log`, or `generic`
- `force` - optional; when omitted, same SHA-256 in the same tenant/collection is treated as an idempotent duplicate and is not queued again
- `skip_kb_mirror` - optional; set `true` when the caller already uploaded the file to the KB collection
- `source_entry` - optional existing KB source entry key to attach when `skip_kb_mirror=true`

The API streams the upload to a spool volume, computes SHA-256, detects record type from headers, registers a `forensic.evidence_items` catalog row when Postgres is configured, and publishes a NATS job to:

```text
forensic.records.ingest.requested
```

It does not parse or insert the file synchronously.

Upload idempotency is enabled by default. If the same file hash already exists in the same tenant and collection with status `queued`, `running`, or `completed`, the API returns `status=duplicate` with the existing job and evidence item instead of publishing another ingest request. Use `force=true` only when a deliberate second batch is required.

When `FORENSIC_KB_MIRROR_ENABLED=true`, the API also mirrors the raw upload into LocalAI's existing Knowledge Base collection API before it publishes the ingest job. Configure:

- `LOCALAI_KB_URL` - default in compose is `http://host.docker.internal:8080`
- `LOCALAI_API_KEY` - optional bearer token when LocalAI auth is enabled

The worker records the mirror result in `forensic.kb_collection_assets.rag_status`.

## Unified Evidence Catalog

Every upload is registered as one evidence item before asynchronous processing begins. The evidence item is the durable spine for later operations:

- raw upload location in the spool/object layer
- Knowledge Base entry reference when mirrored or pre-uploaded
- structured ingest job and canonical record batch
- document/media extraction references in later pipelines
- processing status, warnings, errors, classifier route, and case metadata

List evidence in a collection:

```powershell
Invoke-RestMethod -Uri "http://localhost:8091/evidence?tenant_id=default&collection_id=records-demo&limit=50"
```

Useful filters:

- `case_id`
- `modality`
- `detected_type`
- `processing_status`
- `q` for filename, hash, KB entry, or metadata search

Inspect one evidence item and its linked jobs, KB assets, canonical row preview, and entity rollup:

```powershell
Invoke-RestMethod -Uri "http://localhost:8091/evidence/{evidence_id}?tenant_id=default&limit=25"
```

## Hybrid Query Router

`POST /query/hybrid` routes analyst questions to deterministic records SQL, Knowledge Base evidence lookup, or both.

Example:

```powershell
$body = @{
  tenant_id = "default"
  collection_id = "records-demo"
  query = "show frequent contacts"
  template = "frequent_contacts"
  limit = 10
} | ConvertTo-Json -Compress

Invoke-RestMethod -Uri "http://localhost:8091/query/hybrid" `
  -Method Post `
  -ContentType "application/json" `
  -Body $body
```

Supported templates:

- `collection_overview`
- `frequent_contacts`
- `call_type_breakdown`
- `temporal_activity`
- `shortest_call`
- `longest_call`
- `duration_extremes`
- `first_seen_last_seen`
- `activity_by_day`
- `activity_by_hour`
- `night_activity`
- `top_locations`
- `repeated_location_visits`
- `tower_activity`
- `geospatial_movement`
- `anpr_sightings`
- `entity_activity`
- `subscriber_profile`
- `imei_imsi_usage`
- `cross_dataset_entity_summary`
- `relationship_network`
- `co_travel_or_co_presence`
- `entity_timeline`
- `source_records`
- `source_file_audit`
- `duplicate_upload_audit`
- `schema_profile`
- `data_quality`
- `case_readiness`
- `suspicious_patterns`
- `anomaly_summary`
- `limitations_and_data_quality`
- `evidence_package_summary`
- `executive_case_brief`
- `court_ready_source_summary`
- `evidence`

The router does not execute arbitrary generated SQL. Exact analytics use hardcoded parameterized templates inside read-only transactions. KB lookup uses vector search first and falls back to raw entry previews when the current KB index cannot satisfy a search.

The `template` field is optional for runtime analyst questions. Examples:

- `show collection status and duplicate rows`
- `how many GPRS records are there`
- `show top locations for 923461678183`
- `where was ABC-123 seen`
- `summarize evidence for ABC-123 and show related entities`
- `show relationship network for ABC-123`
- `build timeline for ABC-123`
- `show source rows for ABC-123`
- `show detected headers and schema`
- `show duplicate and rejected row quality`
- `is this case ready for production use?`
- `shortest call duration of 923461678183`
- `which files were ingested`
- `generate an executive case brief`

## Report Generation

`POST /reports/generate` creates a deterministic Markdown forensic report from the same safe analytical templates used by `/query/hybrid`.

```powershell
$body = @{
  tenant_id = "default"
  collection_id = "records-demo"
  target = "ABC-123"
  include_evidence = $true
} | ConvertTo-Json -Compress

Invoke-RestMethod -Uri "http://localhost:8091/reports/generate" `
  -Method Post `
  -ContentType "application/json" `
  -Body $body
```

The report generator does not invent narrative facts. It renders computed aggregates, selected KB evidence previews, case readiness, and warnings.

## Demo Runbook

Use `records-demo` as the primary Knowledge Base / case collection. The analyst agent aliases `Forensic_Records_Analyst` and `forensic_records_analyst` resolve to `records-demo` when no explicit forensic collection is configured.

Safe start commands for the existing demo stack:

```powershell
docker compose -f docker-compose.forensic-records.yaml up -d forensic-postgres forensic-nats forensic-records-api forensic-records-worker
docker ps --filter "name=forensic-" --filter "name=local-ai"
```

Model install commands for the current demo models:

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/models/apply" -ContentType "application/json" -Body '{"id":"qwen3-0.6b"}'
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/models/apply" -ContentType "application/json" -Body '{"id":"granite-embedding-107m-multilingual"}'
```

Recommended demo ingest assets live under `fixtures/forensic_seed/records-demo/` and include CDR, mixed-format CDR, ANPR, IPDR, domain/IPDR, access logs, transactions, subscriber registry, tower reference data, policy notes, identity notes, and case notes.

The planner is not bound to any demo number. It extracts phone-like identifiers, ANPR plates, IP addresses, email addresses, IMSI/IMEI values, and tower/cell-site IDs from the user query, then applies deterministic templates to whatever matching entities exist in the selected collection. CDR phone comparisons normalize digits on both sides so common formatting differences such as `+92 346 167 8183`, `+92-346-167-8183`, and `923461678183` can resolve to the same target.

Safe validation commands:

```powershell
Invoke-RestMethod "http://localhost:8091/healthz"
Invoke-RestMethod "http://localhost:8080/api/agents/collections"

$body = @{ tenant_id="default"; collection_id="records-demo"; query="shortest call duration of 923461678183"; limit=10; max_kb_results=3 } | ConvertTo-Json -Compress
Invoke-RestMethod -Method Post -Uri "http://localhost:8091/query/hybrid" -ContentType "application/json" -Body $body

$readiness = @{ tenant_id="default"; collection_id="records-demo"; query="is this case ready for production use?"; limit=10; max_kb_results=1 } | ConvertTo-Json -Compress
Invoke-RestMethod -Method Post -Uri "http://localhost:8091/query/hybrid" -ContentType "application/json" -Body $readiness
```

Automated smoke check, after the running demo stack is available:

```powershell
.\scripts\smoke_forensic_records.ps1 -LocalAIUrl "http://localhost:8080" -RecordsApiUrl "http://localhost:8091" -CollectionId "records-demo"
```

Demo query checklist:

- `hi`
- `what can you do?`
- `what collections are available?`
- `how do I upload evidence?`
- `shortest call duration of 923461678183`
- `who are the frequent contacts?`
- `where was this number most often observed?`
- `show call type breakdown`
- `what evidence exists for ABC-123?`
- `which files were ingested?`
- `what data quality issues exist?`
- `is this case ready for production use?`
- `generate a report`

Rebuild only after preserving the current working image/container. Suggested safe commands to run after approval:

```powershell
docker tag localai-records:agent-bridge-enterprise localai-records:agent-bridge-enterprise-before-kb-records-hardening
docker compose -f docker-compose.forensic-records.yaml build forensic-records-api forensic-records-worker
docker compose -f docker-compose.forensic-records.yaml up -d forensic-records-api forensic-records-worker
```

Recovery checks if the demo breaks:

- Confirm `forensic-postgres`, `forensic-nats`, `forensic-records-api`, and `forensic-records-worker` are running.
- Keep Docker volumes intact; do not delete volumes or model directories during recovery.
- Check `http://localhost:8091/health` before testing the agent.
- If KB search returns fewer chunks than requested, retry with lower `max_kb_results`; the API also caps search requests automatically.
