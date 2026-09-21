# NexusAI Phase 3 activation preparation

Date: 2026-07-29 (Asia/Karachi)

## 1. Objective

Prepare the retained forensic stack for a separately approved Phase 3 activation
without applying migrations, starting processing services, rebuilding images,
changing models, ingesting evidence, or publishing Git changes.

## 2. Assumptions verified

- Only PostgreSQL and NATS were required for this preparation step.
- The forensic API and worker had to remain stopped so no ingestion or background
  processing could alter retained state.
- Migration preflights are read-only. Migration 009 depends on migration 008 and
  additionally requires the legacy queue to contain no queued, running, or failed
  jobs.

## 3. Files changed

- This activation-preparation report.
- `NEXUSAI_CONTINUATION.md`, updated as the living status ledger.
- One ignored operational recovery artifact under `.phase2-backups/`.

No application, migration, adapter, UI, model, or deployment source was changed.

## 4. Schema, API, and configuration changes

None. No migration was applied, no endpoint was activated, and no configuration
was changed. PostgreSQL and NATS were started from the existing Compose definition.

## 5. Checks and exact results

- PostgreSQL health: `healthy`.
- NATS health: `healthy`.
- Forensic API: remained stopped (`Exited (255)`).
- Forensic worker: remained stopped (`Exited (255)`).
- Migration 008 read-only preflight: passed and committed its read-only transaction.
- Migration 009 read-only preflight: exited 3 with the expected dependency error,
  `migration 008 must be applied and verified before migration 009`.
- Aggregate read-only queue inventory: 41 jobs total, 34 completed and 7 failed;
  therefore 7 jobs also block migration 009 until an explicit recovery decision.

## 6. Live-runtime checks

Only the database and message-broker infrastructure are running. The API and
worker were not started, so no live upload, queue consumption, or query claim was
made. LocalAI was not rebuilt or restarted by this step.

## 7. Dataset and evidence counts

The migration 008 preflight counted 110 evidence items, 104 linked Knowledge Base
assets, and 41 linked ingest jobs. Queries returned aggregates only; no evidence
identifier or raw record value was printed or changed.

## 8. Models

No model was called, searched, downloaded, installed, benchmarked, promoted, or
reconfigured. The recorded Qwen baseline remains unchanged.

## 9. Accuracy and retrieval metrics

Not applicable to this infrastructure preparation. The exact compatibility and
queue counts above are database preflight results, not retrieval-quality claims.

## 10. Latency and memory

No new application latency or model-memory claim was made. Both infrastructure
containers reached healthy state within the bounded health check.

## 11. Failures, fallbacks, and unresolved blockers

Migration 009 is not ready to apply. Migration 008 is still absent, and seven
legacy jobs have terminal `failed` status. Both conditions are intentionally
preserved for a reviewed recovery sequence; no job status was rewritten or hidden.

`pg_dump` emitted a TimescaleDB catalog warning about circular foreign-key
constraints. The backup is a full custom-format dump, not a data-only dump, and
subsequent round-trip hashing, catalog inspection, and complete archive reading
all passed.

## 12. Security, tenant isolation, and provenance

The API and worker remained stopped. Database inspection used read-only
transactions and aggregate output. The backup remains inside the project backup
area and must be treated as sensitive retained evidence infrastructure data.

## 13. Git state

The existing dirty worktree remains unstaged, uncommitted, and unpushed. No Git or
GitHub action was performed.

## 14. Rollback and recovery point

- File: `.phase2-backups/phase3-activation-preparation-pre-migration-20260729T155543.dump`
- Size: 13,018,415 bytes
- SHA-256: `0760ef0613a074d2ace96639f0a3d85fb2019f93339cc79732c17ab103b47cea`
- Round-trip SHA-256: exact match
- `pg_restore` catalog lines: 396
- Full archive read: passed

The two exact temporary container copies were removed after verification. The
verified workspace dump was retained.

## 15. Next action and approval required

Stop at this recovery point. The next mutation must be separately approved and
reviewed: apply and verify migration 008 first, then inspect and resolve the seven
failed legacy jobs without erasing their audit history, rerun migration 009
preflight, and only then seek approval to apply and verify migration 009. API,
worker, runtime-role/grant activation, bounded rebuild, smoke testing, model work,
sensitive ingestion, and Git publication remain separate gates.
