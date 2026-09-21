# NexusAI Phase 6.3 ANPR and Geospatial Source Acceptance

Date: 2026-07-31  
Scope: bounded structured ANPR/vehicle-observation vertical slice  
Collection used for read-only profiling: `records-demo-verified`

## Outcome

Phase 6.3 is source-complete across ingestion, contracts, deterministic query
operations, governed API responses, specialist configuration, Records
Intelligence UI, tests, benchmark, documentation, and rollback-safe deployment
preparation. The slice does not add image OCR or infer facts absent from the
structured source.

The retained collection was inspected read-only before implementation. Its 750
ANPR observations span six plate search keys and four cameras per plate between
2026-07-14 06:00:00Z and 2026-07-19 13:03:00Z. Existing evidence, jobs, vectors,
KB assets, and row counts were not changed.

## Adapter and accounting

- Adapter: `nexusai.adapter.anpr`, version `1.1.0`.
- Family: `anpr_vehicles`; persisted compatibility record type: `anpr`.
- Exact aliases cover plate, camera, observation time, location, latitude,
  longitude, supplied confidence, province, image/crop reference, and crop hash.
- Raw plate text remains evidence. A separate deterministic search key supports
  punctuation/case variants without replacing the source value.
- UTC conversion is explicit and source timezone is retained.
- Coordinates and confidence are accepted only when parseable and in range.
- Row SHA-256 and canonical evidence/source-row locators remain authoritative.
- Duplicate accounting is hash-based and deterministic.
- Pakistan/Urdu text remains UTF-8 and no province is inferred when absent.

The Phase 6.3 synthetic golden accounts for ten inputs as six accepted plus four
rejected; the six accepted rows comprise five unique hashes and one duplicate.
Rejections cover missing required identity/time/camera content and invalid
validated values without leaking rejected source values into diagnostics.

## Deterministic operations

The governed case API exposes seven ANPR operations:

1. `anpr.sightings`
2. `anpr.camera_sequence`
3. `anpr.camera_activity`
4. `anpr.co_travel`
5. `anpr.route_timing`
6. `anpr.plate_variants`
7. `anpr.timeline`

All operations query canonical `forensic.records` rows scoped by tenant and
collection, joined to the current evidence-item version for stable provenance.
The observation projection supports accepted normalized metadata and exact
legacy raw-field aliases so retained Phase 4 rows remain queryable without a
rewrite. Numeric and timestamp casts are guarded before use.

Target-sensitive sequence, co-observation, timing, variant, and timeline
operations stop with `needs_input` when no plate is supplied. Aggregates receive
canonical-table query provenance; row-bearing results retain source file, row,
hash, and evidence identifiers.

## Interpretation boundaries

- A sighting is an explicit structured observation, not an independent claim
  that image OCR or camera time was correct.
- Camera activity is an observation count, not camera reliability or coverage.
- Co-travel means only another plate was observed at the same camera within the
  fixed five-minute window. It is not proof of association or shared travel.
- Route timing compares consecutive observations. Optional distance is WGS84
  straight-line distance between supplied coordinates, never a road route.
- No operation invents owner, driver, occupant, identity, province, plate class,
  camera calibration, clock correction, route, speed, or continuous movement.

These boundaries are included in the enterprise response and repeated beside
the relevant UI result.

## Specialist and catalog

- Catalog: `2026-07-31.phase6.3`.
- Catalog totals: four adapters and 34 operations.
- Specialist: `Vehicle_ANPR_Geospatial_Analyst` version `1.1.0`, status
  `operational_family_slice`.
- Tools: deterministic records SQL plus bounded KB retrieval; compatibility
  fallback remains `Forensic_Records_Analyst`.
- The retained specialist is active with the accepted Qwen model, forensic mode,
  prior resource/safety baseline, and `records-demo-verified` binding.
- Its prompt requires citations, abstention, structured-observation language,
  and the interpretation boundaries above.

Agent creation also materialized two empty name-derived collections,
`Vehicle_ANPR_Geospatial_Analyst` and
`vehicle_anpr_geospatial_analyst`. They contain no entries and are recorded as
cleanup candidates. They were not deleted because collection deletion is a
separate destructive action requiring explicit approval.

## UI and analyst workflow

Records Intelligence now includes a responsive ANPR Intelligence command panel
with concise operation shortcuts, the active specialist handoff, and a visible
structured-observation safety note. Typed results render:

- sighting/sequence/timeline events with plate, camera, location, and time;
- camera activity and plate-variant bars;
- same-camera temporal co-observation bars;
- route-timing map/table rows with elapsed seconds and optional straight-line
  distance.

The panel reuses the active case context and does not silently switch collection
or tenant. Empty results remain empty and never fabricate a visualization.

## Verification

- `go test ./api/forensic_records -count=1`: PASS in 8.997 seconds; 182 passed,
  two intentionally skipped, 184 specs total.
- `go vet ./api/forensic_records`: PASS.
- Focused CDR/IPDR/ANPR/Phase 5A Python suite: 15/15 PASS.
- Python bytecode compilation for worker and ANPR adapter: PASS.
- Catalog JSON parse and invariants: PASS at phase6.3, four adapters, 34
  operations, operational ANPR specialist.
- Targeted ESLint for changed UI/E2E files: zero errors; five existing style
  warnings from the repository configuration.
- React production build: PASS, 658 modules transformed in 1.91 seconds; only
  the existing chunk-size and deprecated `inlineDynamicImports` warnings remain.
- Changed-scope whitespace validation: PASS; line-ending notices only.
- Authored Playwright ANPR coverage is present, but the standalone attempt could
  not launch because its pinned Chromium headless-shell binary is not installed.
  No browser assertion from that attempt is counted as passed; live in-app UI
  verification remains the acceptance mechanism for this activation.

The repository-wide lint command still reports the known baseline of six errors
and 590 warnings, including existing E2E fixture hook-rule findings. Targeted
lint of the files changed by this slice is clean.

## Read-only benchmark

At 2,000 iterations the six-row Phase 6.3 golden measured:

- cold pass: 98.3856 ms;
- warm normalization: 2,315.324 microseconds per input row;
- throughput: 431.9 input rows/second;
- peak traced allocation: 378,931 bytes;
- adapter package size: 9,158 bytes;
- database writes, queue writes, model calls, and retained-data changes: zero.

These figures characterize the local source adapter and are not a production
SLA.

## Rollback and safety

The deployment gate preserves API, worker, and LocalAI images under
`rollback-before-phase6.3-20260731`, requires at least 6 GiB free physical RAM,
builds services sequentially, streams per-stage logs, checks all four health
surfaces, writes `phase6.3-live-activation.json`, and restores prior images on
failure. Named volumes are never replaced by the gate.

No upload, reprocess, migration, model download, evidence deletion, collection
deletion, or sensitive external source access was part of this slice.

## Live activation and post-activation refinement

The guarded Phase 6.3 activation subsequently passed at the strict 6 GiB RAM
gate. API, worker, LocalAI, NATS, and the four health surfaces passed; API,
worker, and LocalAI image IDs changed while rollback images and named volumes
were preserved. Live governed requests passed for ANPR sightings, camera
sequence, co-observation, route timing, plate variants, and timeline using
`ABC-123`. A target-free camera-activity UI request returned 20 exact rows.

That acceptance exposed two integration defects and one productivity problem:

- the LocalAI public discovery surface did not proxy adapters, operations,
  specialists, or contracts even though the sidecar exposed them;
- an operation-only v1 request could convert `anpr.camera_activity` into the
  accidental target `CAMERA_ACTIVITY`;
- the result page repeated technical metadata, limitations, and tabs while
  pushing the analyst answer and exact rows below the fold.

The source now registers authenticated public discovery proxies, separates
operation IDs from target extraction, and includes regression coverage. The
Records page is redesigned around one analyst-summary flow with human-readable
answer, KPIs, typed visual analysis, cited findings, exact preview, next checks,
collapsed cautions, and four secondary audit views. Empty `Value: {}` output and
the duplicate case-health sidebar are removed. CDR/IPDR/ANPR template discovery
and target handling are explicit.

The Collections and case-selection interfaces now hide retained system,
acceptance, legacy-demo, and zero-entry specialist-name collections by default.
Nothing was deleted. The exact read-only inventory and ten zero-entry deletion
candidates are recorded in
`reports/forensic-collection-cleanup-plan-20260731.md`.

These post-activation source refinements require the next operator-run rebuild;
they are not claimed as live until that rebuild and the sequential UI acceptance
complete. The authoritative rebuild, logs, API matrix, desktop/mobile UX, and
collection checks are in
`reports/nexusai-records-ui-acceptance-runbook-20260731.md`.
