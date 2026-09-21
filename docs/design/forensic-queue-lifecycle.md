# Forensic Queue Lifecycle and Reprocessing

Status: Phase 3 source contract and isolated acceptance complete; retained
migrations 008/009 and the guarded historical-failure disposition were applied
and verified on 2026-07-29. Rebuild/deployment, secrets, runtime grants, retained
RLS proof and live synthetic queue smokes remain separately gated.

## Goal

Structured evidence must survive API, NATS and worker crashes without losing a
job, silently processing the same evidence twice, retrying forever without an
audit trail, or erasing earlier outputs. The queue is an at-least-once delivery
mechanism; PostgreSQL is the authority for job identity, attempts, leases,
terminal state and reprocessing lineage.

## Durable path

1. The API retains and fully verifies the source object.
2. Evidence registration and the initial `records_ingest_jobs` outbox row are
   committed in one PostgreSQL transaction.
3. The outbox claims a short publication lease and publishes to the file-backed
   JetStream work-queue stream with `Nats-Msg-Id=ingest-job:<job_uuid>`.
4. A durable, explicit-ack consumer receives the message. The worker locks the
   database job, checks retry time and lease expiry, increments the attempt only
   after a successful claim, and obtains a UUID processing lease.
5. The worker independently verifies the retained object and receipt, then
   commits normalized rows, processing state and audit history in PostgreSQL.
6. Only after the database commit does the worker call `AckSync`. If that ack is
   lost, redelivery sees the completed database state and only acknowledges; it
   does not process the evidence again.

The producer crash window is covered by the transactional outbox. The consumer
crash window is covered by the database lease and terminal-state check.

## JetStream contracts

| Stream | Retention | Storage | Limit | Purpose |
| --- | --- | --- | --- | --- |
| `FORENSIC_RECORDS_INGEST` | Work queue | File | 30 days; 10-minute duplicate window | Unacknowledged structured-ingest requests |
| `FORENSIC_RECORDS_DLQ` | Limits | File | 90 days; 100,000 messages; 1 GiB; 10-minute duplicate window | Terminal failure envelopes for investigation/recovery |

Subject names are `forensic.records.ingest.requested` and
`forensic.records.ingest.dead_letter`. The durable consumer is
`forensic-records-worker-v1` in queue group `forensic-records-workers`.
Stream subject, retention or storage mismatches fail startup. Compatible limit
drift is reconciled to the source contract.

## Database state machine

| Current state | Event | Next state | Queue action |
| --- | --- | --- | --- |
| `queued` or retryable `failed` | Due job and no live lease | `running` | Keep unacknowledged; start heartbeat |
| `running` | Database work commits | `completed` | `AckSync`, then record `acknowledged_at` |
| `running` | Transient failure below limit | `failed` | Store bounded `next_attempt_at`; delayed NAK |
| `running` | Permanent input failure or attempts exhausted | `dead_letter` | Publish stable DLQ envelope, then `Term` |
| `completed` | Redelivery after lost ack | `completed` | Ack only; never rerun |
| `dead_letter` | Redelivery before term was confirmed | `dead_letter` | Republish idempotent DLQ envelope, then term |
| Any terminal job | Analyst reprocess request | New linked `queued` job | Preserve the terminal job and all outputs |

`completed` and `dead_letter` cannot reopen. Attempt counts cannot decrease.
Evidence/job identity and reprocessing lineage are immutable. Terminal rows
cannot retain worker leases. Migration 009 installs these database-enforced
invariants and synchronizes status into Phase 3 processing runs.

## Retry and poison handling

Producer publication retries start at two seconds and double to a five-minute
cap. Worker retries use the same bounded schedule. Invalid JSON, missing required
message fields, invalid retained-object identity, corrupt bytes, schema/input
violations and permission failures are permanent. Connection, timeout,
operating-system I/O and database operational failures are transient. Unknown
processing exceptions are retryable until `max_attempts`, then become terminal.

The DLQ message contains a stable event ID, job/evidence/scope identifiers,
attempt counts, a bounded error, the original-message SHA-256 and event time. It
is published before `Term`; a DLQ publication failure leaves the source message
unacknowledged for recovery.

## Leases and concurrency

- Publication and worker leases use random UUID tokens plus explicit expiry.
- Row-level `FOR UPDATE` locking makes a claim and attempt increment atomic.
- A live lease produces a delayed NAK; an expired lease is recoverable.
- The worker's database transaction verifies that it still owns the lease before
  writing results or completion state.
- `max_ack_pending` is bounded by `FORENSIC_WORKER_MAX_INFLIGHT` (default one on
  this CPU/RAM-constrained deployment).
- JetStream progress acknowledgements prevent normal long processing from
  expiring the consumer ack wait; the database lease remains authoritative.

## Explicit reprocessing

LocalAI exposes:

```text
POST /api/records/forensic/evidence/{evidence_id}/reprocess
```

The internal sidecar route is
`POST /evidence/{evidence_id}/reprocess`. It requires exact collection ownership,
authenticated tenant/collection/case binding, a 3-1000 character reason, and an
8-128 character `Idempotency-Key`. `max_attempts` is bounded to 1-10.

The transaction takes an evidence-scoped advisory lock. An identical request key
returns the existing job; a different request is rejected while a prior job is
non-terminal. Once terminal, the next request creates an immutable linked job
with `reprocess_of_job_id` and incremented `reprocess_generation`. Earlier jobs,
rows, events and derived artifacts are never reset or deleted.

This is a normal user-scoped records operation, not an administrative lifecycle
endpoint, so no LocalAI admin MCP tool is added. The existing records capability
and exact collection-ownership gate apply.

## Failure behavior

| Failure point | Recovery |
| --- | --- |
| API dies before database commit | Evidence and job transaction rolls back together |
| API dies after commit but before publish | Outbox discovers and publishes the due job |
| Publish succeeds but result update fails | Lease expires; stable message ID deduplicates short retries and DB job identity handles later duplicates |
| Worker dies before database commit | Transaction rolls back; ack wait/lease expiry enables redelivery |
| Worker commits but dies before ack | Redelivery observes `completed` and only acknowledges |
| Transient processing failure | Audited `failed` state plus delayed NAK |
| Permanent/maximum-attempt failure | Audited `dead_letter`, DLQ first, then terminal acknowledgement |
| Migration 009 absent | API and worker refuse startup |

## Deployment sequence

1. Drain the legacy ephemeral subscriber and verify migration 009 preflight.
2. Create and verify a scoped database backup.
3. Apply migration 008 if still absent, then migration 009 and both verify files.
4. Configure a persistent NATS data volume and its backup/retention policy.
5. Build and deploy API and worker together with matching subjects, streams and
   consumer names; use a non-owner runtime role with explicit grants/RLS tests.
6. Enable required sidecar authentication and shared service secret.
7. Smoke one synthetic success, transient retry, permanent DLQ, lost-ack
   redelivery and idempotent reprocess path.
8. Observe queue lag, retries, oldest pending outbox row, expired leases, DLQ
   count and job/processing-run consistency before admitting real evidence.

Never apply the retained migration, recreate services, change secrets or runtime
grants, or ingest real sensitive evidence without the corresponding approval and
verified recovery point.
