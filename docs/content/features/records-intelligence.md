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

## Shared query and action execution

Records Intelligence uses the same governed execution foundation for Ask,
specialist-agent requests and typed investigation-workspace actions. The server
binds tenant/case/evidence scope, checks current capability and readiness,
validates a bounded read-only operation plan, and returns typed result state,
citations and limitations. The browser does not calculate forensic findings,
and neither a user query nor a model proposal can provide unrestricted SQL or
select an arbitrary executor.

Recorded plate-region derivatives use `forensics.plate-region-candidate/v1`.
They retain integer bounds in original-image pixels, immutable parent evidence/
version/hash identity, detector and version, confidence, review state and a
transform chain. Candidate lists are bounded and use deterministic
confidence-first non-max suppression. A crop is reconstructed losslessly from
the original-coordinate bounds and receives its own SHA-256 and lineage record.
The Evidence desk may display these candidate artifacts, but selection in the
read-only viewer is not an acceptance decision and candidates are never proof
of a plate string, vehicle owner, driver, route or intent.

The current R8 model gate has not approved or installed a detector or OCR role.
Until an approved visual pack passes localization, OCR, abstention, license and
resource benchmarks, image capability remains foundation-only and OCR text must
not be presented as available.

## Governed Case Workspace

The analyst UI is case-first. `/app/records` is a compatibility entry that
selects the governed default case and opens:

```text
/app/cases/:caseId/overview
/app/cases/:caseId/evidence
/app/cases/:caseId/analyze
/app/cases/:caseId/relationships
/app/cases/:caseId/jobs
/app/cases/:caseId/settings
```

The case ID and collection ID are identical in the v1 contract. The URL is the
authoritative scope for evidence, queries, reports, and forensic Agent Chat.
Requests that provide a different `case_id` or `collection_id` in the body are
rejected rather than silently redirected to another collection.

The case selector hides acceptance, validation, audit, and accidental/system
collections by default. `records-demo-verified` is the preferred local pilot
when it is accessible. Legacy demo collections remain separate and selectable
for reconciliation; the application does not merge, archive, or delete them.

Case APIs are protected by the existing `records` feature permission:

- `GET /api/v1/forensics/cases`
- `GET /api/v1/forensics/cases/{case_id}`
- `GET /api/v1/forensics/cases/{case_id}/manifest`
- `GET /api/v1/forensics/cases/{case_id}/evidence`
- `GET /api/v1/forensics/cases/{case_id}/evidence/{evidence_id}`
- `GET /api/v1/forensics/cases/{case_id}/evidence/{evidence_id}/reprocess-plan`
- `GET /api/v1/forensics/cases/{case_id}/evidence/compare`
- `GET /api/v1/forensics/cases/{case_id}/faces/similar`
- `GET /api/v1/forensics/cases/{case_id}/images/similar`
- `POST /api/v1/forensics/cases/{case_id}/query`
- `POST /api/v1/forensics/cases/{case_id}/reports`

The case-scoped evidence detail response includes immutable source identity,
exact ingestion accounting, canonical processing runs and events, derived
artifacts with stable `nexusai://` citation references, hash-chain-verified
custody history, linked Knowledge Base assets, bounded privacy-projected record
previews, and entity rollups.

The reprocess-plan route is read-only. It reports the latest immutable job
generation, terminal-state eligibility, and the separately governed request
contract. It always returns `approval.required: true`, `approval.state:
not_granted`, and `approval.execution_permitted: false`; opening Evidence
Operations never queues or executes a reprocess. An approved operator workflow
is required for any later mutation.

The manifest is read-only and accounts for KB entries, vectors, KB assets,
evidence, structured rows, entities, jobs, agents, reports, collection sources,
and audit events. A count stays `unresolved`, `external`, or `not_persisted`
when the owning subsystem cannot supply an exact value. It is never guessed.
Every manifest reports `deletion_count: 0`.

Example URL-bound query:

```bash
curl -X POST http://localhost:8080/api/v1/forensics/cases/case-001/query \
  -H 'Content-Type: application/json' \
  -d '{"query":"show frequent contacts","limit":20}'
```

The response uses `forensics.enterprise-response/v1` and keeps deterministic
findings, cited evidence, inferred relationships, successful model
interpretation, unsupported operations, limitations, tables, visualizations,
and execution trace in separate typed sections. Model interpretation is omitted
when synthesis falls back or fails.

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
- `GET /api/records/forensic/capabilities?collection_id=case-001`
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
- Tabs for Executive Brief, Data Grid, Timeline, family-typed Visuals, and Provenance.
- Optional developer audit modal and JSON export. Raw JSON is not shown in the normal analyst path.
- Telemetry panel using the existing status endpoint for jobs, failures, rejected rows, missing KB assets, and recent errors.

## Communications CDR Specialist

The first promoted structured family is `communications_cdr`, implemented by
adapter `nexusai.adapter.cdr` version `1.2.0` and specialist manifest
`Communications_CDR_Analyst`. The case-bound v1 query endpoint accepts these
deterministic operations:

- `cdr.frequent_contacts`
- `cdr.call_type_breakdown`
- `cdr.temporal_activity`
- `cdr.duration_extremes`
- `cdr.timeline`
- `cdr.geospatial_movement`
- `cdr.device_identity_changes`
- `cdr.service_usage`
- `cdr.tower_activity`

Every operation remains bound to the case in the URL. Exact SQL runs before any
optional model explanation. The enterprise response identifies the CDR adapter
and operation in `execution_trace`, attaches deterministic citations, and may
include a typed `bar`, `timeline`, or `map` visualization. Visual specifications
contain the returned bounded rows; the UI does not parse model prose to invent
chart values, graph edges, routes, or coordinates.

For telecom timeline/map results, Ask NexusAI synchronizes selection across the
chronology, supplied-coordinate plot and evidence-detail drawer. Tower-reference
views retain match status, datum, uncertainty and source file/row/hash locators.
Rows without both coordinates remain visible in the table/timeline but do not
become fabricated map points. The coordinate plot never draws an inferred route
or represents RF coverage, handset position, or subscriber presence.

Pakistan-shaped CDR handling preserves local `03xx`, `92`, and `+92` evidence
tokens, uses the configured source timezone such as `Asia/Karachi`, and retains
Urdu/provider fields. Normalized phone comparison is used for matching only;
raw evidence values remain available through cited source records.

## Network IPDR Specialist

The second promoted structured family is `network_ipdr`, implemented by adapter
`nexusai.adapter.ipdr` version `1.1.0` and the case-bound
`Network_IPDR_Capture_Analyst`. The accepted Phase 6.2 slice provides:

- `ipdr.endpoint_summary`
- `ipdr.domain_summary`
- `ipdr.protocol_breakdown`
- `ipdr.session_volume`
- `ipdr.subscriber_sessions`
- `ipdr.concurrent_sessions`
- `ipdr.timeline`

IPDR operations read only canonical normalized fields from `forensic.records`.
They preserve IPv4/IPv6 canonical values, explicit NAT addresses, ports,
provider protocol/domain tokens, bytes, session boundaries, subscriber/session
identifiers, source timezone, hashes, and row locators. Aggregate operations
carry query-level provenance; row-bearing subscriber, overlap, and timeline
operations include source-file and row provenance.

NexusAI does not infer subscriber ownership from an IP address, resolve a domain
to an address, invent CGNAT mappings, reconstruct routes, geolocate endpoints,
classify maliciousness, or execute/reconstruct payloads. Concurrent-session
results require an explicit subscriber and are computed only from explicit
session start/end fields. Packet/capture conclusions remain unavailable until a
separate capture adapter and parser pass their own acceptance gate.

## NexusAI R5 guarded rebuild and acceptance

From the repository root in PowerShell, run the source checks first:

```powershell
go test ./api/forensic_records -count=1
Set-Location .\core\http\react-ui
npm.cmd run build
Set-Location ..\..\..
```

Then run the guarded R5 activation gate. It reuses the proven sequential
combined deployment, preserves rollback image tags and named volumes, performs
health checks, and validates the R5 contract with GET requests only:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build_deploy_nexusai_r5_gate.ps1 `
  -CaseID 'nexusai-forensic-demo'
```

If local authentication is enabled, set `LOCALAI_API_KEY` in the current
PowerShell process before running the gate. Optionally pass `-EvidenceID` to
inspect a known item; otherwise the gate selects the first item returned by the
case-bound catalog. Success ends with `R5Activation=PASS` and writes
`reports/runtime-activation-20260810/r5-live-acceptance.json`. The gate does not
upload evidence, submit reprocessing, run migrations, delete data, or prune
Docker images/volumes.

## Legacy rebuild and rollback reference

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
python -m unittest ingestion.forensic_records.tests.test_phase6_ipdr_adapter
python scripts/benchmark_phase6_ipdr.py --iterations 2000
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

## Governed image intake

Case Workspace Evidence Operations can register authorized image evidence before
OCR or detection is enabled. Registration preserves the original content hash,
case scope and immutable storage reference, then performs bounded header-only
inspection under `forensics.image-intake/v1`.

Automatically inspected raster headers are PNG, JPEG, GIF, BMP, TIFF and WebP.
The evidence detail view reports the detected format, dimensions, pixel count,
color model where available, orientation and downstream-decode eligibility.
JPEG/TIFF metadata is privacy-projected: orientation can be reported, while EXIF
GPS and free-text fields are not exposed by the intake contract.

Declared images over 64 MiB are rejected. PNG, BMP and WebP also require exact
terminal/container boundaries, preventing appended payloads from passing the
bounded admission gate. Images exceeding 32,768 pixels on
either axis or 100,000,000 total pixels, malformed headers, extension/signature
mismatches and non-enabled image codecs are preserved only in an explicit
manual-review state and are not eligible for automatic pixel decoding.

{{% notice warning %}}
Successful image registration proves source identity and bounded technical
metadata only. It does not mean that a plate was detected, OCR text was read, or
a vehicle/person/owner was identified. Those capabilities require their later
R8 benchmark and review gates.
{{% /notice %}}
# Canonical field selection and planner deadline

The source `canonical_records` executor accepts an optional `projection` string
array, also exposed through the forensic tool and typed query plan. Allowed fields
are `record_type`, `timestamp`, `primary_target`, `secondary_target`, `source_file`,
`row_number`, and `ingested_at`. Provenance columns are always retained. Projection
runs after subscriber redaction and does not change filters, totals or paging.
Duplicate fields, expressions, payload fields and use with another operation are
rejected. This is bounded canonical projection; arbitrary source-schema projection
and composable grouping remain unfinished.

`FORENSIC_SEMANTIC_PLANNER_TIMEOUT` defaults to `180s` and can shorten the
open-ended planner deadline. Values above 180 seconds remain capped. Caller
cancellation propagates to the backend request. Synthesis keeps its separate
timeout. These source changes require activation before affecting a deployed service.
