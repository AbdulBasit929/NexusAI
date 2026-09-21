# NexusAI Phase 6.1 Communications CDR Source Acceptance

Date: 2026-07-31  
Status: source-complete deployment candidate; retained image activation pending memory gate  
Safety: approved agent configuration changed; no evidence, collection, database, queue, model, or volume mutation

## Outcome

The first bounded Phase 6 evidence-family slice is implemented across adapter,
manifest, deterministic operations, specialist trace, governed case API, typed
UI, Pakistan-shaped goldens, benchmarks, documentation, and rollback posture.
The accepted CDR persistence/accounting path is preserved. Phase 6.1 is not
declared live-complete because the approved image rebuild stopped at its host
memory guard before compilation.

## Adapter and specialist

- Dedicated package: `ingestion/forensic_records/adapters/cdr/`.
- Adapter: `nexusai.adapter.cdr` version `1.1.0`.
- Family: `communications_cdr`.
- Specialist: `Communications_CDR_Analyst` version `1.1.0`.
- Status: `operational_family_slice` in the embedded discovery catalog.
- Manifest declares lifecycle, formats, normalization version, provenance,
  accounting, Pakistan localization, resource bounds, and compatibility rollback.
- Worker runtime hooks provide the accepted timezone/parser/hash behavior. The
  extracted package builds the same canonical fields and row hashes.
- `Forensic_Records_Analyst` remains the compatibility alias.
- The retained runtime now has an active `Communications_CDR_Analyst`, bound to
  `records-demo-verified` with forensic mode enabled and the accepted Qwen
  baseline. Its Phase 6 query operations remain image-deployment-pending.

## Deterministic operations

The case query contract now maps these registered operations:

1. `cdr.frequent_contacts`
2. `cdr.call_type_breakdown`
3. `cdr.temporal_activity`
4. `cdr.duration_extremes`
5. `cdr.timeline`
6. `cdr.geospatial_movement`
7. `cdr.device_identity_changes`
8. `cdr.service_usage`
9. `cdr.tower_activity`

Ingestion-only `cdr.detect_schema` and `cdr.normalize_records` remain registered
but are correctly rejected by the case query capability guard.

Service usage classifies only explicit call-type or dialed service tokens into
VOICE, SMS, USSD, PACKET_DATA, or OTHER_OR_UNSPECIFIED. Device identity changes
require an originating subscriber/MSISDN/IMEI/IMSI target, exclude a dialed
counterparty as device owner, and compare IMEI/IMSI values within an originating
subscriber partition. Missing targets return `needs_input` before SQL execution.

## Typed response and UI

- CDR responses name the operation and `nexusai.adapter.cdr` in the execution
  trace and include adapter version/family parameters.
- Deterministic findings receive citation IDs.
- Aggregate results receive explicit query-level `forensic.cdr_records`
  provenance and a limitation directing analysts to row-level drill-down.
- Row-bearing timelines/device changes preserve source file and row locators.
- Typed bar, timeline, and coordinate-inventory map specifications contain only
  returned bounded rows.
- The CDR Visuals tab renders exact values and citation IDs. It never parses
  model prose into values or fabricates edges, routes, coordinates, or events.

## Pakistan/local acceptance

The five-row synthetic Pakistan CDR fixture verifies:

- `Asia/Karachi` naive timestamps convert to UTC;
- Urdu/provider text is retained;
- INTERNET and `*123#` service tokens remain unchanged;
- `-1` provider sentinel handling remains accepted;
- one intentional duplicate produces the same row hash;
- source file/row locator, device identity, and location verification;
- partial headers fail with an explainable 2/3 required-group decision;
- a negative duration is rejected by verification without evidence mutation.

No supplied real CDR was ingested. Prior authorized real-format audit results
remain the real-format evidence for this family; this phase read no raw values.

## Validation

- Full forensic Go package: PASS, 163 specs discovered; 161 executed and passed,
  two historical external-sample specs skipped; final package time 16.623 s.
- Phase 6 plus Phase 5A/Phase 2 Python focus: 27/27 PASS in 0.102 s.
- Full worker discovery: 60 tests PASS, two external-sample skips, 1.770 s.
- Phase 6 package/worker bytecode and both JSON manifests: PASS.
- `go vet ./api/forensic_records`: PASS using the accepted repository-local
  Windows cache. A broader four-package command timed out without diagnostics;
  the three unchanged packages passed in Phase 5.
- ESLint for touched UI: zero errors; existing JSX/no-unused warnings remain.
- React production build: PASS, 658 modules, final rerun 2.51 s.
- Playwright with installed Chrome: 8/8 PASS in 23.3 s, including the CDR
  specialist handoff, CDR typed visuals, preserved Records flows,
  desktop/tablet/mobile overflow, and keyboard.
- `git diff --check`: PASS with existing line-ending conversion warnings only.
- PostgreSQL `PREPARE`/`EXPLAIN` for service usage and device changes: PASS
  inside `BEGIN READ ONLY` / `ROLLBACK`; no evidence rows were returned.

## Reproducible adapter benchmark

Command:

```powershell
python scripts/benchmark_phase6_cdr.py --iterations 2000
```

Observed on this workstation:

- five accepted rows per pass;
- cold five-row pass: 36.8521 ms;
- warm normalization: 770.8514 microseconds/row;
- warm throughput: 1,297.27 rows/second;
- peak traced Python allocation: 352,538 bytes;
- adapter package: 9,205 bytes;
- four unique hashes and one intentional duplicate;
- zero evidence writes, database queries, or model calls.

These are source microbenchmarks, not a production SLA. Live SQL latency,
end-to-end throughput, container peak RAM, and disk growth remain deployment
acceptance items.

## Approved retained configuration changes

`Forensic_Records_Analyst.forensic_collection_id` changed from
`nexusai-structured-demo-v2-20260730` to `records-demo-verified`. The complete
existing configuration was read, only that field was changed through the agent
API, and readback returned `records-demo-verified` with forensic mode enabled.

The active `Communications_CDR_Analyst` was created from the accepted resource
and safety baseline, bound to `records-demo-verified`, and given a CDR-specific
prompt that requires deterministic facts, cited source rows, originating-party
device attribution, explicit service tokens, and no invented routes, edges, or
model facts. Final readback shows two active agents; both use
`qwen_qwen3-4b-instruct-2507`, have forensic mode enabled, and bind to the
governed pilot. Records Intelligence contains a case-preserving CDR Specialist
handoff link.

Rollback: PUT the preserved configuration with
`forensic_collection_id=nexusai-structured-demo-v2-20260730`.
Removing the new specialist is destructive and therefore requires separate
explicit approval; no such deletion was performed.

## Deployment attempt and rollback

Explicit pre-deployment rollback tags were created:

- API: `nexusai/forensic-records-api:rollback-before-phase5-20260731` ->
  `sha256:62760165e98c6253f09c91c74f7758e8ee8c33affd436bedd743fcb8592aeaec`.
- LocalAI: `nexusai/localai-forensic:rollback-before-phase5-20260731` ->
  `sha256:7232b0002f0ac244886b3aa76adafc57af2d71e028d7d51ea0425533d7192212`.

The approved memory-safe rebuild stopped before compilation because free host
RAM was 4.28 GiB versus the enforced 5.25 GiB minimum. Its rollback handler
recreated the prior API and LocalAI images. Later read-only checks showed 3.53
GiB and finally 3.85 GiB free, so the guard was not weakened and unrelated applications were not
terminated. PostgreSQL, NATS, worker, and named database/queue/spool volumes
remained intact.

## Final live read-only reconciliation

- LocalAI `/readyz`: 200.
- forensic sidecar `/healthz`: 200.
- forensic worker `/healthz`: 200.
- NATS `/healthz`: 200.
- collections: 20.
- governed pilot: 9,250 accepted, zero duplicate/rejected/failed jobs, four
  completed jobs, eight KB assets, six evidence items.
- family rows: CDR 5,000; IPDR 2,500; access log 1,000; ANPR 750.
- active agents: two (`Forensic_Records_Analyst` and
  `Communications_CDR_Analyst`); both have forensic mode enabled and bind to
  `records-demo-verified` with `qwen_qwen3-4b-instruct-2507`.
- running API/LocalAI images remain the pre-deployment image IDs above.

## Definition-of-done disposition

| Requirement | Status |
| --- | --- |
| Real-format and adversarial fixtures | PASS using prior authorized audit plus new adversarial verification |
| Classification/profile threshold | PASS for the synthetic required-header golden; broader provider threshold remains deployment report work |
| Row/artifact accounting | PASS for accepted normalization goldens; retained data unchanged |
| Deterministic operation contracts | PASS source mapping/tests and read-only SQL prepare/plan |
| Entities, relations, locators | PASS for CDR device/source locators; no inferred relations added |
| Model accuracy/citation/abstention | deterministic path requires no model; optional explanation retains prior bounded policy |
| Latency, throughput, RAM, disk | source microbenchmark PASS; live/container measurements pending |
| API schema and authorization | PASS |
| UI and accessibility | PASS |
| Failures/retries visible | preserved accepted queue/job surfaces |
| Docs/demo/rollback/brief | PASS in source and this report |
| Separately approved live deployment | Image activation ATTEMPTED and safely blocked by memory gate; both approved agent configurations are live |

## Next gate

Free at least 5.25 GiB host RAM without terminating unrelated applications, then
rerun the approved memory-safe deployment. After deployment: verify discovery
catalog `2026-07-31.phase6.1`, all Phase 5 case routes, both agent bindings, one
bounded synthetic CDR typed query per new operation, end-to-end latency/container
RAM/disk, and unchanged retained counts. Only then mark Phase 6.1 live-complete
and begin Phase 6.2 IPDR/network sessions.

## Live activation addendum — 2026-07-31

The shared Phase 6.1/6.2 guarded rebuild subsequently passed with 6.72 GiB free
RAM at preflight. API, worker and LocalAI/UI were rebuilt sequentially and
deployed as independently addressable images; all four runtime health surfaces
returned HTTP 200, rollback images remained preserved, and named volumes were
not replaced. The activation marker is
`reports/runtime-activation-20260731/phase6-live-activation.json`.

The first clean LocalAI attempt failed on transient Docker DNS resolution of
`proxy.golang.org` and automatically restored the prior runtime. After container
DNS/HTTPS verification, the hardened retry passed. The exact post-restart
rebuild, logging, validation and rollback procedure is recorded in
`reports/nexusai-phase6-clean-rebuild-powershell-runbook-20260731.md`.

Image activation is PASS. Full post-restart CDR/IPDR live-query, provenance,
retained-count and resource acceptance remains pending by operator choice before
Phase 6.1/6.2 is marked fully live-complete.

## Post-restart live acceptance closure — 2026-07-31

The deferred read-only acceptance subsequently completed against
`records-demo-verified`. All nine governed CDR operations and all seven governed
IPDR operations returned the enterprise v1 contract through the live case route;
operation/adapter traces, source locators, aggregate provenance, limitations,
clarification behavior, specialist handoffs, and empty-result behavior were
checked. Retained evidence and job counts remained unchanged.

The live audit found and corrected three compatibility defects rather than
masking them: legacy IPDR rows now project exact accepted raw aliases when
normalized metadata is absent; UUID-like provenance values are normalized
without losing their evidence link; and empty IPDR results no longer create an
invalid typed visualization. Focused tests passed after each correction and a
compatibility-only API image was activated before this closure.

Phase 6.1 and Phase 6.2 are therefore fully live-complete. Their deterministic
operations remain bounded to explicit stored facts and retain all previously
documented non-inference rules.
