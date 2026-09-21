# NexusAI Phase 3 Queue Lifecycle — Completion Record

Date: 2026-07-29 (Asia/Karachi)

Outcome: Phase 3 queue/control-plane source development and isolated acceptance
are complete. The retained database and services were not migrated, rebuilt or
recreated. Production activation and the resource-heavy LocalAI wrapper package
check remain explicit gates.

## What was delivered

- File-backed JetStream work queue and bounded DLQ with stable publication IDs.
- Atomic evidence registration plus PostgreSQL transactional outbox.
- Short producer leases, bounded publication backoff and background recovery.
- Durable explicit-ack worker, database-owned leases, ack heartbeats and
  commit-before-`AckSync` processing.
- Bounded transient retry, permanent/attempt-exhausted DLQ and poison-message
  handling.
- Database-enforced terminal immutability, monotonic attempts and reprocessing
  lineage in additive migration 009.
- User-scoped, collection-authorized, idempotent evidence reprocessing that
  creates a new job without deleting prior outputs.
- Queue/outbox/lease/DLQ/reprocessing visibility in evidence-job responses.
- Persistent NATS Compose volume and fail-closed API/worker schema checks.

## Acceptance evidence

- Offline forensic Go suite: 125 registered specs, 124 passed and one live-NATS
  spec intentionally skipped; package passed in 28.815 seconds.
- Real JetStream acceptance: two full package runs passed in 8.018 and 9.381
  seconds. Duplicate publish stored one message; it survived a disposable NATS
  restart; payload/message ID recovered; `AckSync` drained the work queue.
- Python worker/adapter suite: 32/32 passed in 1.122 seconds; compilation passed.
- `go vet ./api/forensic_records`: passed.
- Compose config: exit zero. Docker emitted the known non-fatal sandbox warning
  while reading the user Docker config.
- Disposable TimescaleDB: complete 001-009 chain, 008 legacy fixture/preflight/
  verify/smoke/idempotency/RLS, 009 preflight/forward/verify/smoke, second 009
  application and final verification passed. Final queue-column assertion was
  `phase3-db-ok|4`.
- All disposable database, NATS container and NATS volume resources were removed.
- The focused LocalAI endpoint package did not link within repeated bounded
  five-minute runs on this low-memory laptop and emitted no compiler diagnostic.
  It must run on CI or a higher-memory runner before deployment; this is not
  recorded as a pass.

## Required 15-point ledger

1. **Objective:** eliminate job-loss and duplicate-effect crash windows; add
   reliable retry, DLQ, concurrency leases and immutable reprocessing.
2. **Assumptions verified:** JetStream is at-least-once; PostgreSQL must own
   idempotency; structured evidence already has verified retained bytes; retained
   state and Git were outside this source-only authorization.
3. **Files changed:** queue/reprocess Go source and tests, Python worker/tests,
   LocalAI proxy/route/auth map/client helper, Compose, migration 009 family,
   operator/design docs, checkpoint and this report.
4. **Schema/API/config:** additive queue/outbox/lease/DLQ/reprocess fields and
   constraints; new evidence reprocess route; persistent JetStream data volume;
   no applied retained schema or active config change.
5. **Tests:** exact successful results are listed above. LocalAI wrapper package
   remains a transparent resource-bound pre-deployment check.
6. **Live runtime:** no retained service build, restart, recreate, health-state or
   database change. Only isolated disposable containers were used and removed.
7. **Dataset/evidence:** synthetic migration/queue fixtures only. The supplied
   real Pakistan CDR and retained registry were not read into this acceptance or
   modified.
8. **Models:** no model call, download, install, promotion or parameter change;
   the established Qwen chat/embedding baseline remains unchanged.
9. **Accuracy/retrieval:** no new model claim. Phase 2 modality and exact-row
   baselines remain fixed. Queue tests prove lifecycle correctness, not semantic
   accuracy.
10. **Latency/memory:** JetStream acceptance package runs were 8.018/9.381 s;
    offline package 28.815 s; Python 1.122 s. No load-test percentile or live RAM
    claim was made. LocalAI linking exceeded 300 s on the constrained host.
11. **Failures/fallbacks:** corrected retry enum state, atomic registration gap,
    RLS transaction scope, heartbeat masking, nullable evidence scan, reprocess
    generation race, Ginkgo PowerShell flag parsing and Docker random-port change
    after restart. Final core acceptance is green.
12. **Security/provenance:** trusted tenant context is set per transaction; exact
    collection ownership and case forwarding are retained; immutable source/job/
    lineage identity and append-only history remain; DLQ contains hashes and
    bounded diagnostic data, not source bytes.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed. No branch,
    commit, push, PR or GitHub action was performed.
14. **Rollback/recovery:** migration 009 rollback is destructive and gated by
    `phase3_allow_destructive_rollback`; production rollout requires a verified
    scoped backup. Source remains undeployed, so current retained services are
    already the runtime rollback point.
15. **Next action/approval:** run the focused LocalAI package in CI/high-memory;
    then obtain explicit approvals for backup, migrations 008/009, runtime grants,
    NATS persistent storage, secrets and targeted API/worker/LocalAI deployment.
    After activation acceptance, begin Phase 4 structured-data depth.

## Easy team-lead brief

“I completed the reliability layer around our forensic evidence pipeline. An
upload and its job are now saved together, NATS stores pending work on disk, and
the worker only acknowledges after database results commit. Temporary failures
retry with limits; bad or exhausted jobs go to a traceable dead-letter queue.
Crashes can cause a message to be delivered again, but the database lease and
terminal state prevent duplicate effects. Reprocessing creates a new linked run
and preserves the old evidence and outputs. I proved the database migrations in
a fresh TimescaleDB and proved real NATS deduplication, restart recovery and
synchronous acknowledgement. Nothing was deployed or pushed. The remaining
release gates are a higher-memory LocalAI proxy test, backup/migration approval,
runtime permissions/secrets and targeted deployment. After that, Phase 4 deepens
real Pakistan-format financial, log and structured adapters.”
