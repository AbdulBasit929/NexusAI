# NexusAI Phase 6.2 Network IPDR Source Acceptance

Date: 2026-07-31  
Status: source-complete deployment candidate; retained image activation pending memory gate  
Safety: approved specialist configuration created; no evidence rows, files, jobs, vectors, models, database schema, queue, container volume, or Git state changed

## Outcome

The second bounded Phase 6 evidence-family slice is implemented across a
dedicated IPDR adapter and manifest, deterministic network-session operations,
case-bound specialist contract, enterprise API response, typed UI, Pakistan
goldens, read-only SQL planning, benchmark, documentation, and rollback posture.

Phase 6.2 is not declared live-complete. The retained API, worker, and LocalAI
images still predate Phase 5/6 because free host RAM is 4.09 GiB versus the
enforced 5.25 GiB build gate. Existing services remain healthy and the safety
threshold was not weakened.

## Adapter and normalization

- Adapter: `nexusai.adapter.ipdr` version `1.1.0`.
- Family: `network_ipdr`.
- Package: `ingestion/forensic_records/adapters/ipdr/`.
- Normalization contract: `ipdr-session-canonical/v1`.
- Status: `operational_family_slice` in source catalog
  `2026-07-31.phase6.2`.
- Required detection groups: source IP, destination IP, and session timestamp.
- Accepted formats: CSV, JSON, JSONL/NDJSON, Parquet, TSV, and bounded XLSX.
- Existing worker persistence and row-hash behavior remain the compatibility
  authority; the family package owns aliases, validation, normalized session
  fields, verification, lifecycle metadata, resource bounds, and rollback.

Normalized fields include canonical IPv4/IPv6 endpoints, explicit translated
NAT endpoints, ports, protocol, byte count, session end/duration, explicit
subscriber/session identifiers, IDNA-normalized domain, source timezone, and
source/hash locators. Invalid addresses, ports, domains, time ordering, negative
bytes, and values beyond the signed 64-bit audit bound fail closed.

## Deterministic operations

The catalog now contains 25 total operations, including these seven bounded
IPDR query operations:

1. `ipdr.endpoint_summary`
2. `ipdr.domain_summary`
3. `ipdr.protocol_breakdown`
4. `ipdr.session_volume`
5. `ipdr.subscriber_sessions`
6. `ipdr.concurrent_sessions`
7. `ipdr.timeline`

`ipdr.detect_schema` and `ipdr.normalize_records` remain ingestion lifecycle
operations rather than case-query operations.

All query operations read `record_type='ipdr'` from the canonical
`forensic.records` store. Endpoint/domain/protocol/volume aggregates have
query-level table provenance. Subscriber, overlap, and timeline rows return
`source_file`, `row_number`, `row_hash`, and `evidence_id` where present.

Subscriber sessions, overlaps, and timelines require an explicit target.
Concurrent sessions are reported only when explicit session end timestamps
overlap a later explicit start for the same explicit subscriber identifier.

## Non-inference controls

The adapter, query layer, specialist prompt, enterprise limitations, and product
documentation prohibit:

- subscriber or person ownership inferred from an IP address;
- DNS resolutions inferred from a domain token;
- CGNAT/NAT mappings without translated-address fields;
- route, geography, device, organization, attribution, or maliciousness claims;
- payload execution, reconstruction, or interpretation;
- packet/capture conclusions before the separate capture adapter/parser gate.

Exact facts remain deterministic. The existing Qwen model can only provide an
optional bounded explanation over returned cited facts.

## Specialist configuration

`Network_IPDR_Capture_Analyst` version `1.1.0` is promoted in the source catalog
and created as the third active retained runtime agent. Readback confirms:

- active: true;
- forensic mode: true;
- collection: `records-demo-verified`;
- model: `qwen_qwen3-4b-instruct-2507`;
- accepted safety/resource settings cloned from the CDR specialist;
- IPDR-only deterministic/citation/abstention prompt installed.

Its Phase 6.2 tools are not available in the retained process until image
activation. Records Intelligence contains a case-preserving `IPDR Specialist`
handoff alongside the CDR and compatibility analyst handoffs.

LocalAI agent creation also materialized two empty, name-derived collections:
`Network_IPDR_Capture_Analyst` and `network_ipdr_capture_analyst`. Both have zero
entries and are recorded as cleanup candidates. The two earlier CDR-derived
collections are also empty. No collection was deleted, merged, hidden, or
renamed because cleanup remains manifest-first and separately approval-gated.

## API and UI

- The governed v1 query plan maps every IPDR operation to a bounded deterministic
  template and forces `record_type=ipdr` for `network_ipdr`.
- Clarification responses are returned before tool execution when target-bound
  operations lack an entity.
- Enterprise traces name `nexusai.adapter.ipdr`, adapter version `1.1.0`, family,
  and exact operation.
- Deterministic findings carry citations; aggregate limitations distinguish
  query-level provenance from row-level locators.
- Typed endpoint/domain/protocol charts and session timelines contain only
  bounded deterministic result rows.
- The visualization tab is family-neutral (`Typed Visuals`) and supports
  compound source-to-destination endpoint labels.

## Goldens and real-format posture

The authorized Pakistan IPDR fixture from Phase 4 remains the real-format
synthetic provider golden. Phase 6.2 reuses it without ingestion or raw-value
logging and verifies:

- six input rows;
- four accepted rows;
- two visible rejects;
- three unique normalized hashes;
- one exact duplicate;
- `Asia/Karachi` to UTC conversion;
- explicit-offset preservation;
- IPv4, IPv6, and explicit NAT canonicalization;
- exact duration and signed-64-bit byte bounds;
- invalid address/domain/port/time/value rejection.

No supplied sensitive or real IPDR source was read or ingested in this slice.

## Validation

- Full forensic Go package: PASS in 16.560 s.
- `go vet ./api/forensic_records`: PASS.
- Phase 6.2 plus CDR/Phase 4/Phase 5A Python focus: 15/15 PASS in
  0.074 s.
- Full worker discovery: 64 PASS, two historical external-sample skips, in
  0.676 s. Expected mocked failure-path logs were emitted.
- Python bytecode and both JSON manifests: PASS.
- React production build: PASS, 658 modules, 1.02 s.
- ESLint for touched UI/E2E: zero errors; five existing JSX/no-unused warnings.
- Playwright: 9/9 PASS in 21.4 s, including CDR and IPDR typed visuals,
  specialist handoffs, keyboard behavior, and desktop/tablet/mobile overflow.
- `git diff --check`: PASS with existing line-ending conversion warnings only.

The first read-only SQL prepare correctly caught that retained
`forensic.records` does not expose a direct `version_id` column. The projection
was corrected to the actual schema. Final PostgreSQL `PREPARE` and `EXPLAIN`
passed for the endpoint aggregate and subscriber-overlap window query inside
`BEGIN READ ONLY` / `ROLLBACK`; no evidence rows were returned.

## Reproducible adapter benchmark

```powershell
python scripts/benchmark_phase6_ipdr.py --iterations 2000
```

Observed on this workstation:

- input rows/pass: 6;
- accepted/rejected per pass: 4/2;
- cold pass: 156.0175 ms;
- warm input row: 1,985.4705 microseconds;
- warm throughput: 503.66 input rows/s;
- peak Python allocation: 548,001 bytes;
- adapter package: 10,758 bytes;
- unique hashes: 3; duplicate rows by hash: 1;
- evidence writes, database queries, model calls: zero.

These source microbenchmarks are not production SLAs. Live API latency,
container peak RAM, disk growth, and end-to-end throughput remain deployment
acceptance items.

## Final retained reconciliation

- LocalAI `/readyz`: 200.
- forensic sidecar `/healthz`: 200.
- forensic worker `/healthz`: 200.
- NATS `/healthz`: 200.
- active agents: three; all bind to `records-demo-verified`, have forensic mode
  enabled, and use `qwen_qwen3-4b-instruct-2507`.
- collection catalog: 24, including four zero-entry specialist-name collections.
- governed pilot: 9,250 accepted, zero duplicate/rejected/failed jobs, four
  completed jobs, eight KB assets, six evidence items.
- family rows: CDR 5,000; IPDR 2,500; access log 1,000; ANPR 750.
- no upload, reprocess, evidence-returning query, deletion, or data migration.
- running API/worker/LocalAI images remain the pre-Phase-5/6 images.

## Definition-of-done disposition

| Requirement | Status |
| --- | --- |
| Real-format and adversarial fixtures | PASS using authorized Phase 4 provider-shaped fixture and new Phase 6.2 adversarial checks |
| Classification/profile threshold | PASS: 3/3 required groups for promotion; partial shapes require review |
| Row/artifact accounting | PASS: 6 = 4 accepted + 2 rejected; 4 accepted = 3 unique + 1 duplicate |
| Deterministic operation contracts | PASS in source; endpoint and overlap SQL plans verified read-only |
| Entities, relations, locators | PASS for explicit endpoints, NAT, subscriber/session and source locators; no inferred relations |
| Model accuracy/citation/abstention | Deterministic path requires no model; specialist abstention/citation contract PASS; no new model call |
| Latency, throughput, RAM, disk | Source benchmark PASS; live/container measurements pending deployment |
| API schema and authorization | PASS through governed Phase 5 auth/case boundary and focused tests |
| UI workflow and accessibility | PASS |
| Failures/retries/reprocessing | Existing auditable surfaces preserved; no failure injection against retained data |
| Docs/demo/rollback/brief | PASS in source and this report |
| Separately approved live deployment | PENDING: image activation blocked by 5.25 GiB RAM gate; specialist config is live |

## Rollback

- Image rollback tags from Phase 6.1 remain:
  `nexusai/forensic-records-api:rollback-before-phase5-20260731` and
  `nexusai/localai-forensic:rollback-before-phase5-20260731`.
- Source rollback is bounded to the IPDR adapter/catalog/API/UI/tests/docs files.
- Removing `Network_IPDR_Capture_Analyst` or either empty generated collection is
  destructive and requires separate explicit approval.
- No database or volume rollback is required.

## Next gate

Free at least 5.25 GiB host RAM, rerun the approved memory-safe deployment, and
live-smoke catalog `2026-07-31.phase6.2`, all Phase 5 case routes, CDR operations,
all seven IPDR operations, both specialist handoffs, retained counts, latency,
container RAM, and disk. Only then mark Phase 6.1 and Phase 6.2 live-complete.
The next source slice after that gate is Phase 6.3 ANPR/geospatial.

## Live activation addendum — 2026-07-31

This report's earlier deployment-pending statements are superseded for image
activation by the guarded Phase 6 deployment marker at
`reports/runtime-activation-20260731/phase6-live-activation.json`.

- A controlled Docker/WSL shutdown released 7.96 GiB before Docker restart.
- Unused BuildKit cache cleanup reclaimed 31.51 GB without deleting images,
  containers, models, named volumes, database data, queue data, or spool data.
- Reversible Windows working-set trimming crossed the strict 6 GiB gate without
  terminating user applications. The successful run began with 6.72 GiB free.
- The first clean LocalAI build failed closed on transient Docker DNS failures
  resolving `proxy.golang.org`; API and worker builds had succeeded. Automatic
  rollback restored all four health surfaces. DNS and HTTPS were then verified
  from a disposable Alpine container.
- The hardened retry completed API in 6.2 seconds, worker in 7.8 seconds and
  LocalAI/UI in 851.7 seconds with one LocalAI attempt.
- Deployed images are API `sha256:eaa48e89507af733043a25d589a4a70db82a2afad4e25a18ff5dac4884a52d11`,
  worker `sha256:a5edb8612aff2aaad357ec881f4953257228fc33b08ec454f0503b8a0977a1b0`,
  and LocalAI `sha256:d63879907f15e9f99a6254ee4b6eead12efe4257b9bec1c8faa55be748036ddd`.
- API, worker, NATS and LocalAI returned HTTP 200 after activation. Rollback
  images and named volumes remain preserved.

Image activation is therefore PASS. Exhaustive retained-count, catalog, CDR,
IPDR, provenance, latency/RAM/disk and specialist-handoff acceptance remains a
post-restart verification gate at the user's request. It must pass before Phase
6.1/6.2 is declared fully live-complete or Phase 6.3 begins. The exact operator
runbook is `reports/nexusai-phase6-clean-rebuild-powershell-runbook-20260731.md`.

## Post-restart live acceptance closure — 2026-07-31

The deferred read-only acceptance subsequently completed against
`records-demo-verified`. All nine governed CDR operations and all seven governed
IPDR operations returned the enterprise v1 contract through the live case route;
operation/adapter traces, source locators, aggregate provenance, limitations,
clarification behavior, specialist handoffs, and empty-result behavior were
checked. Retained evidence and job counts remained unchanged.

The live IPDR audit exposed retained Phase 4 rows that predate normalized
metadata. The canonical projection now falls back only to exact accepted raw
aliases, validates values before casts, normalizes UUID-like provenance without
discarding the evidence link, and suppresses typed visualizations for an empty
result. These are compatibility corrections, not inferred enrichment. Focused
tests passed and the corrected API was activated.

Phase 6.1 and Phase 6.2 are therefore fully live-complete. Packet content, DNS
resolution, ownership, route, geolocation, maliciousness, and attribution remain
explicitly out of scope unless supported by a separately accepted evidence path.
