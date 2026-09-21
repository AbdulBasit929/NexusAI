# Forensic Records Webhook API

Go service for asynchronous Knowledge Base + structured-record ingestion.

Endpoint:

```text
POST /webhooks/records/upload
GET  /evidence
GET  /evidence/compare
GET  /images/similar
GET  /faces/similar
GET  /evidence/{evidence_id}
POST /evidence/{evidence_id}/reprocess
POST /query/hybrid
GET  /query/templates
POST /reports/generate
```

## Shared typed query execution

NX-A1 extends the existing APF-3 query architecture with a validated
Investigation Context, derived capability/readiness snapshot, bounded plan
validation, Tool Result, Observation Packet and additive enterprise Answer
Envelope fields. Natural-language Ask and typed case-workspace actions resolve
registered operation IDs through the same read-only executor path. Public typed
plans cannot supply SQL, commands, URLs or implementation-key overrides.

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

The API streams the upload into scoped content-addressed retention, computes
SHA-256 and actual byte count, re-verifies the published object, detects record
type from headers, registers a `forensic.evidence_items` catalog row when
Postgres is configured, and publishes a NATS job to:

```text
forensic.records.ingest.requested
```

It does not parse or insert the file synchronously.

## Durable Queue Lifecycle

Structured evidence registration and the queue outbox row commit atomically in
PostgreSQL. A background dispatcher publishes the stable job ID into the
file-backed `FORENSIC_RECORDS_INGEST` JetStream work queue. Publication uses
`Nats-Msg-Id=ingest-job:<job_uuid>` and a short database lease, so an API crash
before publish is recoverable and short retries deduplicate at the broker.

The API refuses database-backed startup unless migration 009 and its history
trigger are present. A failed immediate publication returns HTTP 202 with
`status=publication_pending`; the job remains durable and the outbox retries it.
See `docs/design/forensic-queue-lifecycle.md` for ack, retry, DLQ, crash and
deployment invariants.

## Content-Addressed Source Retention

New uploads use layout `sha256-scope-v1`. Tenant and collection are converted to
an unambiguous SHA-256 scope key, and bytes are published by a create-only atomic
operation at a stable URI:

```text
forensic-spool://sha256-scope-v1/<scope-key>/<content-sha256>
```

The API performs a full read after retention, then creates a separate read-only
JSON receipt. Duplicate and concurrent identical uploads reuse one verified
object; neither the object nor its receipt is overwritten. If an existing hash
address is corrupt or conflicts with the receipt, ingestion fails closed and the
new bytes are preserved in a server-only integrity quarantine.

The worker mount is read-only and the worker independently verifies root
containment, canonical URI/path, scope, receipt, size and full SHA-256 before it
parses. Original filenames remain metadata so extensionless content objects can
still select the correct CSV/JSON/JSONL parser. Legacy jobs remain readable only
when their path stays under the configured root and their hash verifies.

This is an application-level local write-once contract, not infrastructure WORM.
Production still needs approved storage ACLs, backup/recovery, legal-hold and
retention policy, and object-lock/WORM where required. See
`docs/design/forensic-content-addressed-retention.md`.

Upload idempotency is enabled by default. If the same file hash already exists in the same tenant and collection with status `queued`, `running`, or `completed`, the API returns `status=duplicate` with the existing job and evidence item instead of publishing another ingest request. Use `force=true` only when a deliberate second batch is required.

When `FORENSIC_KB_MIRROR_ENABLED=true`, the API also mirrors the raw upload into LocalAI's existing Knowledge Base collection API before it publishes the ingest job. Configure:

- `LOCALAI_KB_URL` - default in compose is `http://host.docker.internal:8080`
- `LOCALAI_API_KEY` - optional bearer token when LocalAI auth is enabled

The worker records the mirror result in `forensic.kb_collection_assets.rag_status`.

## Authenticated Service Scope

The forensic sidecar supports a fail-closed server-to-server trust boundary.
LocalAI authenticates the human or agent, verifies that the requested collection
belongs to the effective user, and forwards an authenticated actor/subject scope
to the sidecar. Client-supplied tenant, user, collection, or case fields cannot
override that scope.

Configure both services with the same secret and tenant:

- LocalAI: `FORENSIC_RECORDS_API_KEY` and `FORENSIC_RECORDS_TENANT_ID`.
- Sidecar: `FORENSIC_API_KEY`, `FORENSIC_API_AUTH_REQUIRED=true`, and
  `FORENSIC_TRUSTED_TENANT_ID`.
- The Compose stack maps the shared values without embedding a secret in source.

`GET /healthz` remains public for container health checks. Other sidecar routes
require the bearer secret and the trusted `X-Forensic-*` actor, subject, tenant,
collection, and optional case headers when authentication is enabled. These are
internal service headers; browsers and external API clients must use the LocalAI
proxy and must not be allowed to set or preserve them at an edge proxy.

For backward-compatible source testing, authentication remains disabled when no
sidecar key is configured. The service logs that condition. Production must set
the key and `FORENSIC_API_AUTH_REQUIRED=true`; required mode refuses to start
without a key. This source configuration has not yet been deployed to the
retained NexusAI services.

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
Invoke-RestMethod -Uri "http://localhost:8091/evidence/{evidence_id}?tenant_id=default&collection_id=records-demo&limit=25"
```

Compare two distinct image evidence items without mutation:

```powershell
Invoke-RestMethod -Uri "http://localhost:8091/evidence/compare?tenant_id=default&collection_id=records-demo&case_id=records-demo&evidence_id_a={image_a}&evidence_id_b={image_b}"
```

The comparison requires authenticated tenant/collection/case membership for
both images and returns stable evidence/artifact citations. It reports exact
SHA-256 equality, dimensions, bounded dHash distance, OCR/ANPR overlap, face
counts and compatible SigLIP cosine similarity when those governed artifacts
exist. Every metric retains its distinct semantics.

Semantic image retrieval is read-only and never scans an unrestricted corpus:

```powershell
Invoke-RestMethod -Headers $ForensicHeaders -Uri "http://localhost:8091/images/similar?tenant_id=default&collection_id=records-demo&case_id=records-demo&authorization=explicit_case_evidence_scope&query_image_observation_id={image_embedding_artifact_id}&candidate_evidence_id={candidate_a}&candidate_evidence_id={candidate_b}&top_k=10"
```

The query and all candidates must resolve to compatible current-version image
embedding observations inside the authenticated tenant/collection/case scope.
The result says candidate semantic visual similarity, not evidence identity,
object identity, event identity or fact. Text-to-image execution is not exposed
by this API slice; its English/Urdu offline evaluation remains separately
reported.

Automation that needs only processing state and golden row counters should use
the privacy-safe accounting representation:

```text
GET /evidence/{evidence_id}?collection_id=<collection>&view=accounting&include_records_preview=false
```

It omits source previews, evidence/job identifiers and arbitrary source
metadata, returning only the evidence processing state and whitelisted job
accounting/timing fields. The default detail response remains unchanged for the
analyst UI.

Request an immutable new processing run after the current job is terminal:

```powershell
$body = @{
  collection_id = "records-demo"
  reason = "Re-run after approved adapter revision"
  idempotency_key = "records-demo:adapter-v2:001"
  max_attempts = 5
} | ConvertTo-Json

Invoke-RestMethod `
  -Uri "http://localhost:8091/evidence/{evidence_id}/reprocess" `
  -Method Post -ContentType "application/json" -Body $body `
  -Headers @{ "Idempotency-Key" = "records-demo:adapter-v2:001" }
```

Reprocessing never resets the prior terminal job or deletes its rows/artifacts.
The same idempotency key returns the same job. A different request conflicts
while the latest run is non-terminal, then creates the next linked generation
after that run completes or dead-letters.

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
- `subscriber_identity_lookup`
- `subscriber_validity_timeline`
- `subscriber_device_links`
- `subscriber_status_summary`
- `subscriber_conflict_audit`
- `subscriber_reuse_candidates`
- `imei_imsi_usage`
- `cross_dataset_entity_summary`
- `cross_family_correlation`
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
- `correlate 923461678183 across record families`
- `connect ABC-123 across datasets`
- `show relationship network for ABC-123`
- `build timeline for ABC-123`
- `show source rows for ABC-123`
- `show detected headers and schema`
- `show duplicate and rejected row quality`
- `is this case ready for production use?`
- `shortest call duration of 923461678183`
- `which files were ingested`

`cross_family_correlation` uses exact, bounded matching over canonical CDR,
IPDR, ANPR, subscriber, tower/location, transaction, access-log, and generic
records. It returns matched-family coverage, related entities, and stable source
row citations. Phone punctuation is normalized only for identifiers with at
least eight digits; mixed letter/digit identifiers such as plates may use an
alphanumeric compact key. It never uses substring matching. The older
`relationship_network` template remains available for exploratory legacy
behavior and is not silently substituted for the exact route.
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

## Capability contract

`GET /query/capabilities?collection_id=<collection>` returns the 20-family
capability catalog combined with collection record/evidence coverage. It is
protected by the same trusted tenant, actor, subject and collection scope as the
other forensic endpoints. The hybrid query response also contains a `capability`
assessment. Direct requests for pending derived processing are handled by
`capability_guard` before SQL, KB or model execution.

Evidence-derived coverage uses the canonical `evidence_items.extension` column.
Coverage query failures are returned as errors; they must never be converted
into a false `no_data` capability label.

This contract describes what source and collection data can support; it does not
claim that an arbitrary question is answerable. Missing evidence, ambiguous
targets and unaccepted adapters remain explicit outcomes.

The R6.4 discovery surface keeps suggestion UX separate from operation
completeness:

- `GET /query/templates` returns
  `forensics.query-template-catalog/v1` with all 65 accepted deterministic
  operations. Every entry declares its specialist family, stable operation ID,
  records/KB/hybrid source access, inputs, calculation, expected output and
  limitations.
- `query_corpus` in `GET /query/capabilities` returns
  `forensics.family-query-answer-corpus/v1`. The embedded corpus covers every
  accepted template, but only entries with `suggested: true` are intended for
  the Agent Chat starter grid. Governed ambiguity, no-data, unsupported-media,
  incompatible-model and source-access scenarios travel with the same version.
- Hybrid responses repeat the selected operation metadata under
  `enterprise.operation`; Agent Chat carries it into
  `forensics.agent-presentation/v1` so the UI can identify the responsible
  specialist and actual source boundary without inferring either from prose.

These endpoints are read-only discovery contracts. They do not execute an
operation, mutate evidence or broaden the model's authority.

### Phase 5A platform registry

The source-only Phase 5A foundation adds authenticated, read-only discovery
under `/api/v1/forensics`:

- `GET /api/v1/forensics/adapters` returns versioned CDR, IPDR, ANPR, and
  generic-tabular compatibility adapter descriptors plus the common lifecycle.
- `GET /api/v1/forensics/operations` returns typed deterministic family
  operation descriptors.
- `GET /api/v1/forensics/agents` returns specialist-agent manifests and
  model-role references.
- `GET /api/v1/forensics/contracts` returns the query-plan and enterprise-response
  contract identifiers and their required top-level fields.

The catalog is embedded from
`contracts/forensic-platform-v1.json`, so discovery does not depend on a mutable
runtime file. CDR, IPDR, and ANPR are operational family slices, while generic
tabular remains compatibility-wrapped. Other specialist manifests are
contract-only or pending, and the existing `Forensic_Records_Analyst` is
explicitly a compatibility alias. These endpoints do not change current
`/query/hybrid` payloads, accounting, SQL, storage, queue behavior, or configured
agents.

R7.6/R7.7 keep the same enterprise-response and agent-presentation contract
versions while deepening their typed content. Telecom `timeline` and `map`
descriptors carry bounded deterministic rows, citation IDs, source locator
fields, datum/uncertainty fields and tower-join match status. The primary
forensic analyst exposes the operational CDR, subscriber and tower surfaces;
specialists remain case-bound, citation-required and unable to delegate to a
peer. Models may explain these results but may not create coordinates, join
states, routes or RF-presence claims.

For a read-only provider/onboarding check before upload, run:

```powershell
python ingestion/forensic_records/worker.py audit-source "C:\path\to\records.csv" --record-type auto --source-timezone Asia/Karachi --jurisdiction PK
```

The command uses production schema/adapter logic but emits no raw rows,
identifiers, or rejected values and performs no database, queue, KB, or model
operation.

TSV and bounded read-only `.xlsx` row extraction are accepted source adapters.
XLSX queries use normalized rows carrying worksheet and source-row locators;
formula text and cached values remain evidence and are never recalculated. In
`record_type=auto`, each non-empty worksheet is mapped independently and only a
unique typed-header match is promoted; ambiguous or unrelated worksheets remain
generic with an explicit mapping decision and review state. Explicit typed
requests also validate per sheet and preserve non-matching sheets generically.
Macros, external-link traversal, legacy `.xls`, `.xlsm`, ODS and deeper
Arrow/Feather/Avro/ORC/XML spreadsheet paths remain unavailable until their own
acceptance gates pass.

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

### Phase 6.1 typed CDR queries

Use the governed case route for the promoted communications specialist:

```powershell
$plan = @{
  contract_version = "forensics.query-plan/v1"
  tenant_id = "default"
  case_id = "records-demo-verified"
  collection_id = "records-demo-verified"
  intent = "cdr.device_identity_changes"
  families = @("communications_cdr")
  entities = @(@{ type = "msisdn"; value = "923001234567" })
  time_range = @{ from = "2026-07-01"; to = "2026-08-01"; timezone = "Asia/Karachi" }
  limit = 25
} | ConvertTo-Json -Depth 8
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/forensics/cases/records-demo-verified/query" -ContentType "application/json" -Body $plan
```

The response is `forensics.enterprise-response/v1`. For CDR operations its
execution trace names `nexusai.adapter.cdr` and the exact operation ID. Typed
bar/timeline/map specifications are built only from deterministic result rows.
Query-level provenance is explicit for aggregates, and row-bearing operations
include `source_file` and `row_number` locators.

### Phase 6.2 typed IPDR queries

Use the same governed case route with a `network_ipdr` family and one of
`ipdr.endpoint_summary`, `ipdr.domain_summary`, `ipdr.protocol_breakdown`,
`ipdr.session_volume`, `ipdr.subscriber_sessions`,
`ipdr.concurrent_sessions`, or `ipdr.timeline`:

```powershell
$plan = @{
  contract_version = "forensics.query-plan/v1"
  tenant_id = "default"
  case_id = "records-demo-verified"
  collection_id = "records-demo-verified"
  intent = "ipdr.concurrent_sessions"
  families = @("network_ipdr")
  entities = @(@{ type = "subscriber_identifier"; value = "923001234567" })
  time_range = @{ from = "2026-07-01"; to = "2026-08-01"; timezone = "Asia/Karachi" }
  limit = 25
} | ConvertTo-Json -Depth 8
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/forensics/cases/records-demo-verified/query" -ContentType "application/json" -Body $plan
```

The trace names `nexusai.adapter.ipdr` and the exact operation. Endpoint,
domain, protocol, volume, subscriber, overlap, and timeline facts are derived
only from explicit normalized fields. The API does not infer DNS resolutions,
subscriber ownership, NAT mappings, routes, geolocation, payload content, or
threat attribution. Packet/capture conclusions remain separately gated.

### Phase 6.3 typed ANPR queries

Use the governed case route with an `anpr_vehicles` family and one of
`anpr.sightings`, `anpr.camera_sequence`, `anpr.camera_activity`,
`anpr.co_travel`, `anpr.route_timing`, `anpr.plate_variants`, or
`anpr.timeline`:

```powershell
$plan = @{
  contract_version = "forensics.query-plan/v1"
  tenant_id = "default"
  case_id = "records-demo-verified"
  collection_id = "records-demo-verified"
  intent = "anpr.camera_sequence"
  families = @("anpr_vehicles")
  entities = @(@{ type = "vehicle_plate"; value = "ABC-123" })
  time_range = @{ from = "2026-07-01"; to = "2026-08-01"; timezone = "Asia/Karachi" }
  limit = 25
} | ConvertTo-Json -Depth 8
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/forensics/cases/records-demo-verified/query" -ContentType "application/json" -Body $plan
```

The trace names `nexusai.adapter.anpr` and the exact deterministic operation.
Sightings and sequences cite canonical record/evidence locators. Camera activity
is an observation count; co-travel means only a same-camera observation within a
fixed five-minute window; route timing means consecutive observation timing and,
when coordinates were supplied, straight-line distance. Neither operation
asserts vehicle association, road travel, ownership, occupants, OCR correctness,
or clock correction. The UI renders typed timeline, bar, and map/table views and
repeats these limitations beside the result.

### Phase 7.1 typed subscriber identity queries

Use the governed case route with the `subscriber_identity` family and one of
`subscriber.identity_lookup`, `subscriber.validity_timeline`,
`subscriber.device_links`, `subscriber.status_summary`,
`subscriber.conflict_audit`, or `subscriber.reuse_candidates`:

```powershell
$plan = @{
  contract_version = "forensics.query-plan/v1"
  tenant_id = "default"
  case_id = "records-demo-verified"
  collection_id = "records-demo-verified"
  intent = "subscriber.validity_timeline"
  families = @("subscriber_identity")
  entities = @(@{ type = "msisdn"; value = "923001234567" })
  time_range = @{ from = "2026-07-01"; to = "2026-08-01"; timezone = "Asia/Karachi" }
  limit = 25
} | ConvertTo-Json -Depth 8
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/forensics/cases/records-demo-verified/query" -ContentType "application/json" -Body $plan
```

The trace names `nexusai.adapter.subscriber_identity` and the exact deterministic
operation. Exact-target operations accept a supplied MSISDN, subscriber
reference, IMSI, or IMEI. Default results mask CNIC and omit subscriber names;
aggregate operations count explicit source observations without exposing those
values. Validity windows, identifier links, conflicts, and reuse are reported
as reviewable evidence observations—not as proof of identity, ownership,
current control, SIM swap, fraud, or continuous device use. Model explanation
is optional and remains subordinate to exact rows and citations.

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
