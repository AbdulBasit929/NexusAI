# NexusAI Phase 3 retained queue activation completion

Date: 2026-07-29 (Asia/Karachi)

## 1. Objective

Complete the retained-database portion of Phase 3 by preserving seven historical
failures truthfully, passing the migration 009 compatibility gate, applying the
durable queue lifecycle transaction, verifying all control-plane/queue invariants,
and sealing an independent post-009 recovery point.

## 2. Assumptions verified

- Migration 008 was installed and officially verified.
- API and worker remained stopped, so the legacy queue was drained.
- The seven historical failures had no materialized outputs or successful sibling
  jobs and could be preserved truthfully as dead-letter history.
- The existing `dead_letter` enum value allowed terminalization before migration
  009 without weakening its compatibility gate.
- Database activation does not authorize runtime-role grants, service rebuilds,
  secrets, model changes, sensitive ingestion, or Git publication.

## 3. Files changed

- Added the one-time gated recovery transaction:
  `db/forensic_records/009_phase3_queue_lifecycle.legacy_recovery.sql`.
- Updated the database runbook and queue-lifecycle design status.
- Updated the recovery disposition, team-lead brief, this completion report, and
  the living continuation checkpoint.
- Added two ignored custom-format database recovery artifacts.

No API, worker, LocalAI, adapter, UI, model, or migration 009 source was changed.

## 4. Schema, API, and configuration changes

Migration 009 is now applied. `forensic.records_ingest_jobs` has stable queue
message identity, publication/worker lease fields, retry/error/DLQ timestamps,
acknowledgement state, reprocessing parent/generation/request identity, three
lifecycle constraints, four indexes, and the history-protection trigger.

No API endpoint, runtime secret, role/grant, model, service image, or active
application configuration was changed.

## 5. Checks and exact results

- Post-008/pre-recovery backup: round-trip hash, 493-line catalog, and full read
  passed.
- Recovery source SHA-256:
  `cc882ccbfae9f93b748d63fe5cfa013b6033b7988269bfadc4cbd0583a611bf5`.
- Guarded recovery: passed all preconditions/postconditions and committed.
- Recovery result: 34 completed, 7 dead-letter, 0 blocking jobs, 7 dead-letter
  processing runs, and 7 recovery audit rows.
- Official 008 verifier after recovery: passed; 48 processing events and 116
  custody events reflect append-only deltas of 7 and 6.
- Migration 009 preflight: passed and rolled back read-only.
- Migration 009 SHA-256:
  `0d285379d6b719086508bb0f8ec41126647bc5e1d1f6b12143588c51c00fae9c`.
- Migration 009: exited zero and reached `COMMIT`.
- Official 009 verifier: passed and rolled back read-only.
- Final official 008 verifier: passed.
- Final aggregate lifecycle check: 41/41 message IDs valid, 41/41 original
  generation, 41/41 zero publish attempts, zero worker leases, three constraints,
  and one history-protection trigger.

## 6. Live-runtime checks

PostgreSQL and NATS remained healthy. Forensic API and worker remained stopped
through backup, recovery, preflight, migration, verification, and final backup.
LocalAI was not rebuilt or restarted by this phase.

## 7. Dataset and evidence counts

- 110 evidence items, storage objects, and versions.
- 214 source links.
- 41 jobs/runs: 34 completed/succeeded and 7 dead-letter.
- 48 processing events.
- 116 custody events.
- 0 derived artifacts.
- 7 recovery audit rows.
- All seven recovered evidence items have exact failed status and job pointers.
- No evidence/job/version/source count drift and no new canonical evidence rows.

## 8. Models

No model was called, searched, downloaded, installed, benchmarked, promoted, or
reconfigured. The recorded Qwen baseline remains unchanged.

## 9. Accuracy and retrieval metrics

Not applicable to this database activation. Exact lineage, status, queue identity,
constraint, lease, and append-only history invariants passed; no retrieval-quality
claim was created.

## 10. Latency and memory

All database operations completed inside bounded commands. No production model,
query-latency, concurrency, or memory benchmark is claimed from migration timing.

## 11. Failures, fallbacks, and unresolved blockers

No recovery, preflight, migration, or verification failure occurred. Both backups
emitted the disclosed TimescaleDB continuous-aggregate circular-FK warning; each
was a full custom-format dump, and round-trip hash, catalog inspection, and full
archive reading passed.

The open Phase 3 blockers are runtime-only: higher-memory LocalAI package
validation, non-owner role/grants and retained RLS proof, required sidecar auth,
matching persistent JetStream settings, rollback-tagged rebuild/deployment, and
synthetic live queue/reprocess failure-path smokes.

## 12. Security, tenant isolation, and provenance

The recovery selected only exact diagnostic signatures under seven-job invariants,
preserved IDs/errors/attempts/timestamps, appended audit and processing history,
corrected exact evidence pointers, and never printed raw identifiers or errors.
Migration 009 now makes evidence/job/reprocessing identity immutable, attempts
monotonic, and completed/dead-letter states terminal.

## 13. Git state

The existing dirty worktree remains unstaged, uncommitted, and unpushed. No Git or
GitHub action was performed.

## 14. Rollback and recovery points

- Pre-008:
  `.phase2-backups/phase3-activation-preparation-pre-migration-20260729T155543.dump`,
  13,018,415 bytes, SHA-256
  `0760ef0613a074d2ace96639f0a3d85fb2019f93339cc79732c17ab103b47cea`.
- Post-008/pre-recovery:
  `.phase2-backups/phase3-post-008-pre-recovery-20260729T162041.dump`,
  13,161,166 bytes, SHA-256
  `15d2188c680dae310baacba609cdb44479db388b76fd89ecdd05b3b6ea96ae10`.
- Post-009 activated:
  `.phase2-backups/phase3-post-009-activated-20260729T162714.dump`,
  13,169,864 bytes, SHA-256
  `442a9d9f2d55eec958a33968d3fb54d7e4732f4574e16af69f8de22b5db1fc41`.

Every listed dump passed round-trip SHA-256, catalog inspection, and full archive
reading. Destructive rollback scripts were not run.

## 15. Next action and approval required

The Phase 3 retained-database/control-plane/queue-migration phase is complete.
Next requires a separate runtime activation approval: run the LocalAI focused
package on a higher-memory runner, provision the non-owner runtime role/grants and
required auth/JetStream settings, build/recreate API and worker with rollback tags,
then smoke synthetic success, retry, DLQ, redelivery, tenant isolation, and linked
reprocessing before admitting real evidence. Models, sensitive ingestion, and Git
publication remain separately gated. Phase 5 document/OCR source work can proceed
independently without claiming the new runtime is deployed.
