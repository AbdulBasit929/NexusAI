# NexusAI legacy failed-job recovery disposition

Date: 2026-07-29 (Asia/Karachi)

Status: disposition executed and verified on 2026-07-29. The diagnosis below is
the preserved pre-execution record; final results are in
`reports/forensic-phase3-retained-queue-activation-completion-20260729.md`.

## 1. Objective

Diagnose the seven retained legacy jobs that block migration 009 and define a
truthful, audit-preserving recovery disposition without exposing evidence values,
changing job/evidence state, running migration 009 preflight, or applying 009.

## 2. Assumptions verified

- Migration 008 is installed and officially verified.
- Migration 009 rejects every `queued`, `running`, or `failed` legacy job.
- Existing failed jobs must never be deleted, reset, or mislabeled completed.
- `dead_letter` already exists in the legacy status enum and is the truthful
  terminal state for a preserved historical failure.
- After migration 009, any retry must be a new linked reprocessing job; the
  original terminal job cannot reopen.

## 3. Files changed

- This recovery-disposition report.
- `NEXUSAI_CONTINUATION.md`, updated as the living status ledger.

No application, migration, adapter, UI, model, runtime configuration, or retained
database row was changed.

## 4. Schema, API, and configuration changes

None. The work used read-only transactions only. Migration 009 preflight and
forward migration were not run. API and worker were not started.

## 5. Checks and exact results

Aggregate diagnosis found:

- 7 failed jobs, 7 distinct evidence items, 0 missing evidence links;
- 1 tenant scope and 2 anonymous collection scopes;
- queued between 2026-07-23 06:59:08 UTC and 2026-07-24 05:38:35 UTC;
- every job is attempt 1 of maximum 5 and has a terminal timestamp;
- record families: 1 CDR, 2 ANPR, 2 IPDR, and 2 access-log jobs;
- two diagnostic signatures only:
  - `7aa916a87064`: 6 jobs across ANPR/IPDR/access-log; 45 characters and only
    allowlisted technical token `type`;
  - `73a4d88751c7`: 1 CDR job; 53 characters and allowlisted technical tokens
    `adapter required missing`;
- no raw error text, filenames, collection names, source identifiers, hashes of
  evidence bytes, or record values were emitted.

## 6. Live-runtime checks

Only read transactions were issued through healthy PostgreSQL. API and worker
remained stopped, so no producer, consumer, retry, or source-file processing
could occur during diagnosis. NATS was not used by the diagnostic queries.

## 7. Dataset and evidence counts

- Job row accounting: 0 total, accepted, duplicate, and rejected rows.
- Materialized outputs for these jobs: 0 CDR rows, 0 generic rows, 0 canonical
  rows, 0 KB metadata rows, and 0 audit rows.
- Row-error records: 0; raw rejected-row payloads: 0.
- Sibling jobs on the same evidence: 0; all seven linked jobs are the failed jobs.
- KB assets for these evidence items: 0.
- Source links: 7 legacy/source links.
- Storage: 7 `legacy_spool`, `unverified`, non-write-once objects.
- Processing runs: 7 correctly preserved as `failed`.
- Evidence status: 1 failed and 6 stale queued.
- Evidence warnings are legacy JSON null on all seven; one evidence item has a
  non-empty errors value. These shapes were observed only and not normalized.

## 8. Models

No model was called, searched, downloaded, installed, benchmarked, promoted, or
reconfigured.

## 9. Accuracy and retrieval metrics

Not applicable. This was an exact database-lineage and state-consistency audit,
not a retrieval or model-quality evaluation.

## 10. Latency and memory

Each database diagnostic completed inside a bounded command and used aggregate
queries over seven target jobs. No production latency or memory claim is made.

## 11. Failures, fallbacks, and unresolved blockers

One initial read-only query attempted `jsonb_array_length` on legacy scalar JSON
and aborted its read transaction. The corrected shape-safe query passed and
identified JSON null warnings; no data changed.

Physical availability and full-byte integrity of the seven legacy spool objects
were not asserted. They must not be reprocessed until their paths are contained
and their bytes match the recorded SHA-256. Migration 009 remains blocked until
an approved recovery transaction changes the seven job states truthfully.

## 12. Security, tenant isolation, and provenance

All outputs were aggregates, anonymous scope numbers, status/family labels, and
short hashes of technical diagnostic messages. No raw evidence, raw rejected row,
filename, job/evidence ID, collection name, or error text was printed. Existing
job rows, processing runs, source links, and diagnostics remain unchanged.

## 13. Git state

The existing dirty worktree remains unstaged, uncommitted, and unpushed. No Git or
GitHub action was performed.

## 14. Rollback and recommended recovery transaction

Before any mutation, create and verify a fresh post-008/pre-recovery custom dump.
Keep the existing verified pre-008 dump as the deeper recovery point.

Recommended disposition, subject to separate approval and an exact guarded SQL
review:

1. Lock only the seven expected `failed` job rows and abort unless every aggregate
   invariant in this report still matches.
2. Insert one operator audit record per job containing the prior state, diagnostic
   signature, reason, approval reference, and recovery version—never raw evidence.
3. Transition each job from `failed` to `dead_letter`; preserve job ID, source
   identity, timestamps, attempt count, maximum attempts, row counters, metadata,
   and original error text. Migration 008 triggers will append the processing
   transition instead of rewriting its earlier event.
4. Change only the six stale evidence states from `queued` to `failed`; leave the
   already-failed item unchanged. The custody trigger will append these changes.
5. Assert 41 total jobs remain: 34 completed and 7 dead-letter; the seven original
   jobs still have zero output rows; processing runs become 7 dead-letter; source
   and version counts do not drift; exactly 7 audit records and 6 custody status
   events are added.
6. Rerun the official 008 verifier and the 009 read-only preflight. Stop again
   before applying migration 009.

Rejected alternatives:

- Do not mark failures completed; that would create false success.
- Do not delete jobs; foreign-key cascades and history loss are unacceptable.
- Do not reset them to queued/running; those states still block 009 and would
  reuse an obsolete execution identity.
- Do not weaken the 009 preflight.
- Do not retry in place. After queue activation, create new linked reprocessing
  jobs only after legacy bytes are independently found and hash-verified.

## 15. Next action and approval required

Stop before mutation. The next separately approved action should create the fresh
post-008 backup and execute a reviewed, invariant-guarded terminalization
transaction implementing the disposition above. After verification, rerun the
009 read-only preflight. Applying migration 009, starting services, runtime grants,
rebuilds, model work, sensitive ingestion, and Git publication remain separate
approval gates.
