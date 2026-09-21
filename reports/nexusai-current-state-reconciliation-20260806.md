# NexusAI current-state reconciliation

Date: 2026-08-06  
Workstream: R0 authoritative reconciliation  
Status: source and live read-only inspection complete for the bounded session scope

## Verified repository state

- Repository: `C:\Users\sheik\Workspace\Office\Projects\NexusAI`.
- Branch: `codex/forensic-hybrid-checkpoint-20260723`.
- HEAD and live `/version`: `40717b83510c08db25dc26b9d6674bf46db363ac`.
- Remote default reference: `origin/main` at the same base commit.
- Worktree before this session: 56 modified paths and 192 untracked paths; no deleted paths.
- The worktree is intentionally dirty, unstaged, uncommitted, and unpushed. Existing user work was preserved.
- No stash or tag was reported by the opening Git inspection.

## Authoritative-document reconciliation

The complete master prompt is now preserved in
`NEXUSAI_MASTER_DIRECTIVE.md`, which governs the roadmap/checkpoint/continuation
documents under the recorded precedence order. Historical Phase 7.3
professional answer presentation remains accepted evidence, not the active
phase. The active transformation program uses R0-R17.

R0 is complete at the bounded reconciliation exit criteria. R1 is source
accepted after exact route/backend inventories; R2 is the sole active phase.

## Verified live services

| Service | Image | State | Health |
| --- | --- | --- | --- |
| LocalAI/NexusAI UI | `nexusai/localai-forensic:phase3-runtime` (`0aff21e8f09b`) | running | healthy; `/healthz` and `/readyz` 200 |
| Forensic API | `nexusai/forensic-records-api:phase3-runtime` (`4c415a95e18d`) | running | `/healthz` 200 |
| Forensic worker | `nexusai/forensic-records-worker:phase3-runtime` (`7a62791e40cf`) | running | healthy; metrics endpoint 200 |
| NATS | `nats:2.11-alpine` | running | healthy; monitoring health 200 |
| PostgreSQL/TimescaleDB | `timescale/timescaledb:latest-pg16` | running | healthy |

Named volumes for models, images, backends, configuration, LocalAI data,
PostgreSQL, NATS JetStream, and forensic spool are present. Rollback images remain
present. No image, container, volume, collection, or evidence mutation occurred.

## Verified models, agents, adapters, and case

- Installed models are exactly `qwen_qwen3-4b-instruct-2507` and
  `qwen3-embedding-0.6b`.
- Six runtime agent bindings point at the single governed case
  `nexusai-forensic-demo`: orchestrator compatibility analyst plus CDR, IPDR,
  ANPR, subscriber, and tower specialists.
- Live catalog: six adapters, of which CDR, IPDR, ANPR, subscriber, and tower are
  operational family slices and generic tabular is compatibility-wrapped.
- Live specialist catalog truthfully separates operational, contract-only,
  compatibility, and pending adapter/model states.
- Default analyst case discovery returns one selectable case and hides four
  retained system/cleanup candidates.

## Governed case accounting

The read-only manifest for `nexusai-forensic-demo` reported:

| Resource | Count |
| --- | ---: |
| Evidence items | 8 |
| Evidence versions/KB assets | 8 KB assets; 11 KB entries |
| Canonical records | 9,272 |
| Record entities | 44,318 |
| Ingest jobs | 8 completed |
| Agent bindings | 6 |
| Audit events | 8 |
| Permanent deletions | 0 |

Family accounting is CDR 5,002 accepted plus one duplicate, IPDR 2,500,
access/security 1,000, ANPR 750, subscriber 11 accepted/1 duplicate/3 rejected,
tower 5 accepted/1 duplicate/4 rejected, and transaction 4 accepted. These are
manifest facts, not model claims.

PostgreSQL was inspected with the non-owner `forensic_runtime` role. The
`forensic` schema contains 17 tables covering evidence/version/custody/storage,
processing runs/events, canonical and legacy records, entities, KB linkage,
jobs/errors, and audit data. No SQL mutation was issued.

## Source topology baseline

- 1,362 Go source files across 146 source directories.
- 140 Python source files after excluding caches.
- 192 React/JavaScript/CSS source files and approximately 50,969 lines under the
  React source tree.
- 64 unique backend names in `.github/backend-matrix.yml`.
- 371 non-test Echo route registrations found by bounded static extraction,
  plus 17 forensic-sidecar `ServeMux` registrations.
- 593 Go test files, 16 Python test files, 43 React Playwright specs, and 90
  forensic-focused test/spec files.

## Live UI acceptance

The deployed Case Workspace and Agent Chat were inspected in the in-app browser.
The loaded case state showed 8 evidence items, 9,272 rows, 8 KB assets, seven
queryable families, six agent bindings, and no console warnings/errors. Overview
had zero page-level horizontal overflow and no broken images at 390, 820, 1024,
and 1440 pixels.

The running image is older than the latest source tree: deployed case selectors
still reveal four cleanup candidates, while current source uses the governed v1
case endpoint and shows only the one analyst case. This is source-verified but
not runtime-activated and must not be described as live.

## Bounded implementation completed

The browser exposed an incorrect transient state: before governed APIs finished,
the Overview claimed `Inventory only`, displayed zero evidence/rows/families, and
stated there was no queryable coverage. Source now renders `Verifying case state`,
em dashes, and explicit checks until all case data resolves. It no longer emits a
false zero-data conclusion during startup.

Verification:

- targeted ESLint: 0 errors; 12 pre-existing warnings in the untracked page;
- Vite production build: PASS, 658 modules;
- focused Case Workspace Playwright: PASS, 5/5 on installed Chrome;
- source preview: one governed analyst case, correct 9,272-row state, no console
  warning/error;
- deployed 390/820/1024/1440 overview: zero page overflow and zero broken images.

## Deployment boundary

The new truthfulness fix and governed source case selector are source-only. No
Docker build, restart, migration, backfill, upload, reprocess, agent update, model
call/download, collection cleanup, staging, commit, push, or publication occurred.

## Next bounded objective

Complete `R2-DES-01`: resolve the final asset, tagline-placement, role naming,
administration terminology, branding-configuration/report asset and visible
white-label decisions, then evaluate R2 source acceptance. Agent Chat lifecycle
work is deferred to future R6 and source-accepted `R4-PRE-01` remains undeployed.
