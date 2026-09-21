# NexusAI retained migration 008 activation

Date: 2026-07-29 (Asia/Karachi)

## 1. Objective

Apply and verify Phase 3 evidence-control-plane migration 008 against the retained
forensic registry from the verified pre-migration recovery point, while keeping
all processors stopped and stopping before legacy-job recovery or migration 009.

## 2. Assumptions verified

- The verified pre-migration dump still had its recorded size and SHA-256.
- PostgreSQL and NATS were healthy before application.
- The forensic API and worker were stopped before, during, and after application.
- The retained-database procedure is preflight, forward migration, and official
  verification. Synthetic fixture, idempotency, smoke, and RLS scripts are for a
  disposable database only and were not run here.

## 3. Files changed

- This retained-migration report.
- `NEXUSAI_CONTINUATION.md`, updated as the living status ledger.

No application, adapter, UI, model, migration source, or runtime configuration
file was changed.

## 4. Schema, API, and configuration changes

Migration 008 was applied transactionally. It added the seven normalized Phase 3
relations for storage objects, evidence versions, source links, processing runs,
processing events, derived artifacts, and custody events. It added the current
version pointer, synchronization and immutability triggers/functions, indexes,
foreign keys, constraints, and one tenant-isolation policy on each Phase 3 table.

No API endpoint, worker, model, queue lifecycle, or active service configuration
was changed. Migration 009 was not run.

## 5. Checks and exact results

- Recovery dump: 13,018,415 bytes; recorded SHA-256 matched.
- Migration 008 source SHA-256:
  `b130044ea7c5f39338ffd35f271934c0a36e99a6d86f9336aca91bef0b10476a`.
- Immediate preflight: passed with 110 evidence items, 104 linked KB assets, and
  41 linked ingest jobs.
- Forward migration: exited zero and reached `COMMIT`.
- Official verification: passed.
- Post-migration aggregate integrity transaction: passed and rolled back.
- RLS metadata: 7 of 7 Phase 3 tables enabled; exactly 7 policies.
- Missing current-version pointers: 0.
- Invalid custody-event SHA-256 values: 0.

The first-application `IF EXISTS`/`IF NOT EXISTS` notices were expected and no SQL
error occurred.

## 6. Live-runtime checks

PostgreSQL and NATS remained healthy. The forensic API and worker remained stopped
throughout, preventing upload, query, publication, or queue-consumer activity.
LocalAI was not rebuilt or restarted by this step.

## 7. Dataset and evidence counts

Official migration verification returned:

- evidence items: 110
- storage objects: 110
- evidence versions: 110
- source links: 214
- processing runs: 41
- processing events: 41
- derived artifacts: 0
- custody events: 110

The original linked inventory remains 104 KB assets and 41 jobs. Job status remains
34 completed and 7 failed. No raw evidence value or identifier was printed.

## 8. Models

No model was called, searched, downloaded, installed, benchmarked, promoted, or
reconfigured. The recorded Qwen baseline remains unchanged.

## 9. Accuracy and retrieval metrics

Not applicable to this schema activation. Verification establishes exact control-
plane lineage counts and constraints; it does not create a retrieval-quality claim.

## 10. Latency and memory

The forward migration completed in approximately two seconds of command runtime;
the official verifier completed in under one second after execution began. These
are operational observations, not production performance benchmarks.

## 11. Failures, fallbacks, and unresolved blockers

No migration or verification failure occurred. Seven legacy ingest jobs remain in
`failed` state. Migration 009 intentionally rejects failed jobs, so it remains
blocked until a separately approved, audit-preserving investigation and recovery
decision is completed.

## 12. Security, tenant isolation, and provenance

All seven new Phase 3 relations have RLS enabled and one tenant policy each.
Existing evidence was backfilled into immutable version/source/run/event/custody
lineage without changing the original job outcomes. Retained-data checks exposed
aggregates only.

## 13. Git state

The existing dirty worktree remains unstaged, uncommitted, and unpushed. No Git or
GitHub action was performed.

## 14. Rollback and recovery point

The authoritative pre-migration recovery point remains:

- `.phase2-backups/phase3-activation-preparation-pre-migration-20260729T155543.dump`
- 13,018,415 bytes
- SHA-256: `0760ef0613a074d2ace96639f0a3d85fb2019f93339cc79732c17ab103b47cea`
- round-trip hash, 396-line catalog, and full archive read passed

The destructive migration 008 rollback script was not run and remains separately
gated.

## 15. Next action and approval required

Stop before mutation. The next step requires separate approval to inspect the
seven failed jobs using privacy-safe aggregate diagnostics and prepare an explicit
recovery disposition that preserves their existing rows and error history. Only
after approved recovery should migration 009 preflight be rerun. Applying migration
009, starting API/worker, runtime grants, rebuilds, model work, sensitive ingestion,
and Git publication remain separate approval gates.
