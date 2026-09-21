# Forensic Evidence Control Plane

Status: Phase 3 migration, authenticated-scope and content-addressed-retention
candidates pass source and disposable acceptance. Forward migration, legacy
backfill, native ingest, repeat-apply, failure atomicity, non-owner RLS and
storage verification pass. The retained NexusAI database remains unmigrated and
the new auth/storage configuration remains undeployed until separate gates are
approved. Reliable queue acknowledgement/retry/DLQ/reprocessing is the next open
Phase 3 implementation slice.

## Purpose

The Phase 2 registry identifies evidence and links it to structured records and
Knowledge Base entries. Phase 3 makes version, storage, processing, artifact, and
custody state first-class and queryable without changing the immutable source or
erasing earlier results.

## Normalized objects

| Object | Responsibility |
| --- | --- |
| `evidence_storage_objects` | Content hash, byte size, storage URI/backend, integrity verification, retention and legal hold |
| `evidence_versions` | Ordered immutable versions of one logical evidence item |
| `evidence_source_links` | Multiple upload, KB, external, legacy or derived source references for one version |
| `processing_runs` | Idempotent pipeline execution with adapter/model revisions, parameters, attempts and terminal state |
| `processing_events` | Append-only ordered state/events for a processing run |
| `derived_artifacts` | Typed output linked to exact evidence version, run, parent artifact, storage object and citation locator |
| `evidence_custody_events` | Append-only, per-evidence SHA-256 chain for custody/material lifecycle events |

## Required invariants

1. Evidence byte identity (`tenant`, `collection`, SHA-256, byte size and raw
   storage reference) cannot be edited in place. Corrections create a version.
2. A current version belongs to the same tenant, collection and evidence item.
3. Source links are append-only. Identical bytes may have many distinct source
   links without creating false duplicate evidence objects.
4. A processing run has one tenant-scoped idempotency key and records the exact
   input version, adapter/model revisions, parameters and output manifest.
5. Processing and custody events cannot be updated or deleted through normal SQL.
6. Custody inserts serialize per evidence item and incorporate the prior event
   hash, identity, actor, time, reason and canonical JSON payload into SHA-256.
   This is tamper-evident database lineage, not an external notarization claim.
7. Every derived artifact points to its input version and producing run and has a
   typed citation locator. An artifact never becomes a new source evidence fact.
8. Every new table carries tenant RLS `USING` and `WITH CHECK` policies. A
   disposable acceptance test assumes a `NOSUPERUSER`, `NOINHERIT`,
   `NOBYPASSRLS` role, proves same-tenant reads/writes, and rejects a cross-tenant
   insert. Runtime database grants and ownership still require deployment review;
   table-owner bypass means RLS alone is not the final security boundary.
9. LocalAI, not a request body, establishes actor, subject, tenant, collection,
   and case. The sidecar verifies a shared bearer credential, rejects scope
   switching, and restricts repair to authenticated administrators.
10. New raw bytes are addressed by a tenant/collection scope hash plus content
    SHA-256, published atomically without overwrite, full-read verified, and
    paired with a read-only receipt before classification or queueing. The worker
    independently checks path, URI, scope, receipt, size and hash from a read-only
    mount. This is an application contract; infrastructure WORM remains a
    deployment control.

## Compatibility and backfill

Migration `008_phase3_evidence_control_plane.sql` is additive and performs these
idempotent mappings:

- each existing evidence item becomes one legacy storage object and version 1;
- valid `sha256-scope-v1` metadata becomes a verified, write-once
  `forensic_spool_content_addressed` object; legacy metadata remains unverified;
- the existing metadata version UUID is retained when it is valid;
- each evidence source and KB asset becomes a distinct normalized source link;
- each evidence-linked ingest job becomes an idempotent processing run with one
  legacy snapshot event;
- each existing evidence item receives a hash-chained legacy-registration custody
  event;
- no canonical/legacy record row, job, KB asset, audit row or evidence item is
  deleted.

`008_phase3_evidence_control_plane.preflight.sql` runs in a read-only transaction
and rejects incompatible hashes, sizes, lifecycle times, cross-scope links, and
version-ID collisions. The forward and rollback scripts are transactional so an
error cannot leave a partially installed or partially removed control plane.

After backfill, triggers synchronize new evidence registrations, KB source links,
ingest job status/counters, and evidence processing-status changes. Authenticated
actor/subject scope is implemented in source, including LocalAI collection
ownership checks and sidecar admin enforcement. Explicit artifact APIs remain a
later Phase 3 slice.

## Validation sequence

1. Take a scoped custom-format backup and verify it with `pg_restore -l`.
2. Record evidence/job/KB/record/audit counts and hashes before migration.
3. Run the read-only preflight and resolve every reported incompatibility.
4. Apply migrations with `psql -v ON_ERROR_STOP=1`.
5. Run `008_phase3_evidence_control_plane.verify.sql`.
6. Exercise a synthetic registration, KB source link, queued/running/completed
   job, custody chain and append-only mutation rejection.
7. Reapply the forward migration to prove idempotency and exact count stability.
8. Confirm original Phase 2 counts and query results are unchanged.
9. Assume a non-owner, non-bypass role; verify same-tenant visibility and writes,
   then prove a cross-tenant write is denied and rolled back.
10. Run the rollback only in a disposable database first. Production rollback is
   destructive to new Phase 3 data and needs explicit approval plus a verified
   restore point.

## Approval boundaries

The migration, verification, smoke and rollback files may be reviewed and tested
against a disposable isolated database. Applying them to the running NexusAI
database, restoring/replacing data, or enabling authenticated permission changes
requires a separate explicit approval. No new model or full LocalAI build is part
of this control-plane slice.

The 2026-07-27 read-only preflight observed 110 evidence items, 104 linked KB
assets and 41 linked ingest jobs with zero compatibility failures. A follow-up
schema query confirmed migration 008 and `current_version_id` are not installed
in the running registry.
