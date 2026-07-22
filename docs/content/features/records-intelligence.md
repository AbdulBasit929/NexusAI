+++
title = "Records Intelligence"
weight = 45
+++

Records Intelligence adds deterministic analytics for structured record files alongside the Knowledge Base.

Use the three layers together:

- **Knowledge Base** stores raw evidence and retrieves semantically relevant chunks or previews.
- **Records Intelligence** parses structured rows and computes exact filters, counts, rankings, min/max, distinct values, and correlations.
- **Agents** orchestrate both: they use records tools for exact analytics and Knowledge Base search for evidence and explanation.

{{% notice warning %}}
Do not use semantic Knowledge Base retrieval for exact analytics over bulk records. If the question asks for an exact count, list, shortest/longest row, top N, duplicate, join, or date-filtered result, query Records Intelligence.
{{% /notice %}}

## Supported Inputs

The first implementation supports:

- CSV
- TSV
- JSON arrays
- JSONL / NDJSON
- TXT and log lines with simple key/value parsing

LocalAI detects columns, preserves original field names, and adds normalized aliases. Record type inference currently recognises:

- `cdr`
- `anpr`
- `ipdr`
- `subscriber`
- `tower_location`
- `transaction`
- `access_log`
- `generic`

## Ingest Records

Upload a file with `POST /api/records/ingest`:

```bash
curl -F "file=@cdr_sample.csv" \
  -F "collection_name=case-001" \
  -F "record_type=auto" \
  http://localhost:8080/api/records/ingest
```

When `collection_name` is set, LocalAI also stores the raw file in that Knowledge Base collection. The parsed batch records the `batch_id`, `collection_name`, `record_type`, `source_file`, `source_entry`, row count, detected schema, and parse errors.

## Query Records

Run exact filters with `POST /api/records/query`:

```json
{
  "record_type": "cdr",
  "filters": [
    { "field": "area", "op": "contains", "value": "Gulberg" },
    { "field": "duration_seconds", "op": "lt", "value": 30 }
  ],
  "sort": [{ "field": "timestamp", "direction": "asc" }],
  "limit": 50
}
```

Supported filter operators include `eq`, `ne`, `contains`, `gt`, `gte`, `lt`, `lte`, `in`, `exists`, and `not_exists`.

## Aggregate Records

Use `POST /api/records/aggregate` for exact counts and rankings:

```json
{
  "record_type": "cdr",
  "operation": "top_by_field",
  "field": "target_number",
  "top_n": 10
}
```

Supported operations include `count`, `distinct`, `min`, `max`, `top_by_field`, `count_by_field`, `min_by_field`, and `max_by_field`.

## Helper Queries

The generic query endpoint also accepts a `helper` field.

CDR helpers include `shortest_call`, `longest_call`, `target_numbers_by_source`, `calls_by_area`, `calls_by_date_range`, `calls_between_numbers`, `calls_by_cell_id`, `failed_calls`, `roaming_calls`, `imei_to_msisdns`, and `imsi_to_msisdns`.

ANPR helpers include `plate_search`, `sightings_by_plate`, `sightings_by_location`, `sightings_by_time_range`, `repeat_plate_locations`, and `plates_seen_near_location`.

Generic helpers include `records_by_field`, `distinct_values`, `count_by_field`, `min_by_field`, `max_by_field`, `top_by_field`, and `timeline_by_entity`.

## Forensic Sidecar Operations

When `FORENSIC_RECORDS_API_URL` or `LOCALAI_FORENSIC_RECORDS_API_URL` is configured, LocalAI exposes a same-origin proxy for the forensic records sidecar:

- `GET /api/records/forensic/status?tenant_id=default&collection_id=case-001`
- `GET /api/records/forensic/templates`
- `POST /api/records/forensic/query`

The sidecar path is the production path for large forensic collections. It stores parsed records in PostgreSQL/TimescaleDB, tracks duplicate uploads, mirrors Knowledge Base assets, and runs deterministic SQL templates for hybrid records + evidence questions.

Example hybrid query:

```json
{
  "tenant_id": "default",
  "collection_id": "case-001",
  "query": "what happened on 2026-07-10?",
  "limit": 20,
  "max_kb_results": 3
}
```

The response includes the selected template, route, planner metadata, exact records output, evidence previews when requested, and limitations. The React Records Intelligence page uses these endpoints for collection status, template discovery, clarification handling, and natural-language forensic queries.

## Evidence Model Benchmarking

Use [Forensic Model Benchmarking]({{% relref "features/forensic-model-benchmarking" %}}) to plan and measure models/backends per evidence type. The benchmark catalog keeps OCR, ASR, embeddings, VLMs, rerankers, and analyst synthesis models configurable so LocalAI can suggest suitable models without hardcoding one global choice.

## Agent Behavior

KB-enabled agents receive records tools:

- `records_query`
- `records_aggregate`
- `records_schema`
- `records_explain_batch`
- `cdr_query`
- `anpr_query`
- `correlate_records`

Agents are instructed to use these tools for exact analytics and Knowledge Base search for evidence/context. Answers should cite `batch_id`, `source_file`, `source_entry`, and `row_number` when returned.

## Enterprise Response Workspace

The forensic sidecar `POST /api/records/forensic/query` response includes an `enterprise` object for production UI and agent rendering:

- `summary`: analyst-ready executive brief.
- `metrics`: planner, route, row-count, evidence-count, provenance, and coverage measurements.
- `data_grid`: normalized columns and capped rows for table rendering.
- `provenance`: row-level SQL sources and Knowledge Base citations when returned.
- `limitations`: coverage warnings, clarification messages, and query caveats.
- `coverage`: route, tenant, collection, template, target, and date-bound context.
- `synthesis.claims`: facts labeled as deterministic SQL, semantic Knowledge Base context, or planner guidance.

The React Records Intelligence page renders this as a guided workbench:

- Schema-aware prompt chips based on sidecar status and available templates.
- Template discovery grouped by operational category.
- Tabs for Executive Brief, Data Grid, Timeline, and Provenance.
- Optional developer audit modal and JSON export. Raw JSON is not shown in the normal analyst path.
- Telemetry panel using the existing status endpoint for jobs, failures, rejected rows, missing KB assets, and recent errors.

## Rebuild And Rollback Procedure

Rebuild after code and contract validation, not after every small edit. The recommended trigger is:

- Focused sidecar tests pass.
- Focused agent formatter tests pass.
- React production build passes.
- The current running containers have rollback snapshots.

Validation commands:

```powershell
$tmp=[System.IO.Path]::GetTempPath()
$cache=Join-Path $tmp 'localai-go-cache'
$appdata=Join-Path $tmp 'localai-go-appdata'
New-Item -ItemType Directory -Force -Path $cache | Out-Null
New-Item -ItemType Directory -Force -Path $appdata | Out-Null
$env:GOCACHE=$cache
$env:APPDATA=$appdata
$env:GOTELEMETRY='off'
go test ./api/forensic_records
go test ./core/services/agents --ginkgo.focus "forensic records tools"
```

```powershell
cd D:\Projects\LocalAI\core\http\react-ui
npm run build
```

Create rollback snapshots before deployment:

```powershell
docker commit local-ai localai-main:pre-phase3-20260720-enterprise-ui
docker commit localai-forensic-records-api-1 forensic-records-api:pre-phase3-20260720-enterprise-contract
```

Rebuild and restart the sidecar API only:

```powershell
cd D:\Projects\LocalAI
docker compose -f docker-compose.forensic-records.yaml build forensic-records-api
docker tag localai-forensic-records-api:latest forensic-records-api:phase3-enterprise-contract-20260720
docker compose -f docker-compose.forensic-records.yaml up -d --no-deps forensic-records-api
```

Rebuild and replace the main LocalAI container while preserving runtime bind mounts:

```powershell
cd D:\Projects\LocalAI
docker compose -f docker-compose.yaml build api
docker tag quay.io/go-skynet/local-ai:master localai-main:phase3-enterprise-ui-20260720
docker stop local-ai
docker rename local-ai local-ai-pre-phase3-20260720-enterprise-ui
docker run -d --name local-ai -p 8080:8080 `
  -e FORENSIC_RECORDS_API_URL=http://host.docker.internal:8091 `
  -e HEALTHCHECK_ENDPOINT=http://localhost:8080/readyz `
  -e NVIDIA_VISIBLE_DEVICES=all `
  -e NVIDIA_DRIVER_CAPABILITIES=compute,utility `
  -v D:\LocalAI-runtime\models:/models `
  -v D:\LocalAI-runtime\backends:/backends `
  -v D:\LocalAI-runtime\configuration:/configuration `
  -v D:\LocalAI-runtime\data:/data `
  localai-main:phase3-enterprise-ui-20260720
```

Post-deploy checks:

```powershell
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
Invoke-RestMethod 'http://localhost:8080/api/records/forensic/status?tenant_id=default&collection_id=records-demo&limit=5'
Invoke-RestMethod 'http://localhost:8080/api/records/forensic/templates'
```

Rollback to the preserved Phase 2 container:

```powershell
docker stop local-ai
docker rename local-ai local-ai-phase3-failed-20260720
docker rename local-ai-pre-phase3-20260720-enterprise-ui local-ai
docker start local-ai
```

Rollback only the sidecar API image:

```powershell
docker compose -f docker-compose.forensic-records.yaml stop forensic-records-api
docker tag forensic-records-api:pre-phase3-20260720-enterprise-contract localai-forensic-records-api:latest
docker compose -f docker-compose.forensic-records.yaml up -d --no-deps forensic-records-api
```

## Limitations

- TXT/log support is intentionally conservative and handles line records or simple key/value pairs.
- The first storage backend is durable JSONL under the LocalAI data path. It is deterministic and portable, but very large deployments may eventually want a SQL-backed store.
- Geospatial helper names such as `plates_seen_near_location` currently perform text/location matching unless latitude/longitude-specific logic is added.
