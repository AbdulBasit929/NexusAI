# Forensic Records DB

This folder contains Phase 1 database migrations for the enterprise forensic records pipeline.

Apply with the TimescaleDB container in `docker-compose.forensic-records.yaml`; the SQL files are mounted into `/docker-entrypoint-initdb.d` on first database initialization.

Main objects:

- `forensic.evidence_items`
- `forensic.records_ingest_jobs`
- `forensic.cdr_records`
- `forensic.generic_records`
- `forensic.record_entities`
- `forensic.kb_collection_assets`
- `forensic.kb_active_metadata`
- `forensic.records_ingest_errors`
- `forensic.records_audit_log`
- `forensic.cdr_frequent_contacts`
- `forensic.entity_activity_summary`

The schema enables row-level security and expects callers to set:

```sql
SET app.tenant_id = 'default';
```

For databases initialized before a later migration was added, apply the new migration manually without deleting data:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/002_job_row_counters.sql
```

Phase 2 KB asset and entity indexing migration:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/003_phase2_kb_assets_and_entities.sql
```

Unified evidence registry migration:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/007_evidence_items.sql
```

`forensic.evidence_items` is the evidence-first catalog. It links raw uploads to Knowledge Base entries, structured ingest jobs, canonical records, and future document/media processing results through `evidence_id`.

Phase 3 evidence control-plane migration:

```powershell
# Do not run against an existing database until its scoped backup and migration
# application have been explicitly approved.
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/008_phase3_evidence_control_plane.preflight.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/008_phase3_evidence_control_plane.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/008_phase3_evidence_control_plane.verify.sql
```

The migration normalizes immutable storage objects, evidence versions, source
links, idempotent processing runs, append-only run events, typed derived
artifacts, and hash-chained custody events. It backfills existing registry links
without rewriting structured records and installs synchronization triggers for
new evidence, KB links, ingest jobs, and status changes.

For source uploads using `sha256-scope-v1`, preflight and migration 008 require
and validate the content-addressed object URI, receipt URI, actual byte size,
scope key, `verified` integrity state, `write_once=true`, verification time and
`sha256-full-read-after-retain` method. The scope key is recomputed in PostgreSQL
from the same UTF-8 length-prefixed tenant/collection tuple used by the API and
worker. Valid rows become backend `forensic_spool_content_addressed`; old rows
remain `legacy_spool` and are not falsely marked immutable. Details and the
filesystem trust boundary are in
`docs/design/forensic-content-addressed-retention.md`.

The preflight is a read-only transaction. It rejects invalid hashes or sizes,
non-monotonic job times, cross-tenant/collection evidence links, and normalized
version-ID collisions before schema work starts. The forward migration is one
transaction: a failed statement leaves no partial Phase 3 schema.

The `legacy_fixture.sql`, `smoke.sql`, `idempotency.verify.sql`, and
`rls.smoke.sql` files are synthetic acceptance tools for a disposable database
only. The RLS test assumes a temporary non-owner `NOBYPASSRLS` role, proves
same-tenant access, requires a cross-tenant write to fail, and rolls back the
role and test write. These files must not be run against a retained NexusAI
registry.

The rollback file drops the Phase 3 control-plane data and is therefore gated.
Run it only from a verified backup and with explicit approval:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -c "SET app.phase3_allow_destructive_rollback = 'true'" `
  -f /docker-entrypoint-initdb.d/008_phase3_evidence_control_plane.rollback.sql
```

Phase 3 durable queue lifecycle migration:

```powershell
# Drain the legacy subscriber and verify a fresh scoped backup first. Retained
# migration application requires explicit approval.
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -c "SET app.phase3_allow_legacy_queue_recovery = 'true'" `
  -f /docker-entrypoint-initdb.d/009_phase3_queue_lifecycle.legacy_recovery.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/009_phase3_queue_lifecycle.preflight.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/009_phase3_queue_lifecycle.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/009_phase3_queue_lifecycle.verify.sql
```

The gated `legacy_recovery.sql` file is a one-time retained-registry disposition
for the exact seven historical failures diagnosed on 2026-07-29. It is not a
general retry or cleanup tool. It aborts unless migration 008 is present, migration
009 is absent, both diagnostic signatures and all 7-job/output/storage invariants
match, and the explicit session gate is set. It preserves every original job,
error, attempt and timestamp; moves failed jobs to truthful `dead_letter`; repairs
their exact evidence job pointers and six stale queued evidence states; and appends
audit, processing and custody history in one transaction. Never run it on another
registry or after migration 009.

Migration 009 adds the transactional publication outbox, publication/worker
leases, bounded retry/DLQ state, acknowledgement time and append-only
reprocessing lineage. It makes evidence/job identity immutable, prevents attempt
counts from decreasing, prevents completed/dead-letter jobs from reopening and
requires terminal jobs to be lease-free. The preflight deliberately rejects
legacy queued/running/failed rows: drain the old subscriber and resolve those
jobs explicitly instead of guessing their state during DDL.

`009_phase3_queue_lifecycle.smoke.sql` is for a disposable database after the 008
smoke fixture. It proves retry and terminal transitions, terminal immutability
and processing-run synchronization, then rolls back. The 009 rollback drops
operational history columns and is gated by
`SET app.phase3_allow_destructive_rollback = 'true'`; never use it on retained
evidence without a verified backup and explicit approval.

## Non-owner runtime activation

Before starting the API or worker with a non-owner database identity, close the
three Phase 1 policy gaps and verify that every RLS-enabled forensic table has at
least one policy:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/010_runtime_rls_policies.preflight.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/010_runtime_rls_policies.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/010_runtime_rls_policies.verify.sql
```

`runtime_role.grants.sql` provisions the fixed `forensic_runtime` role with
connect, temporary-table, schema-usage, read and explicit ingest/update grants.
It has no owner, superuser, bypass-RLS, schema-create, delete or truncate rights.
Set its password separately and never put a password in tracked SQL or shell
history. Then run `runtime_role.rls.verify.sql` as the owner; its synthetic
same-tenant and cross-tenant probes are contained in a rolled-back transaction.
The complete credential, Compose, build, smoke and rollback sequence is in
`reports/nexusai-phase3-runtime-local-powershell-runbook-20260730.md`.

## R5 deterministic custody-chain tail correction

Migration 011 replaces only `forensic.prepare_custody_event()`. It selects the
actual unreferenced chain tail instead of assuming that random UUID order is
insertion order when multiple custody events share a timestamp. It rewrites no
retained row. Apply it only through
`scripts/apply_nexusai_r5_custody_fix.ps1 -ApproveSchemaMigration`, which stops
the writers, creates and validates a timestamped custom-format backup, runs the
read-only preflight, applies the transactional function replacement, verifies
the installed definition, restarts API/worker, and writes a non-secret marker.

The API independently verifies chain topology by predecessor hash, including a
single root, reachable events, missing predecessors, duplicate hashes, and
forked links. It no longer derives continuity from timestamp/UUID ordering.

## Typed families, reference tables, link-analysis layer and vectors (migrations 012 to 015) - DRAFT, not applied

Drafted 2026-10-12 under decision D2 of `docs/roadmap/nexusai-maturity-roadmap-20261012.md`. All additive, idempotent, one
transaction per file; tested on a disposable database only. **Do not apply to a retained database until the owner has said so
and a validated backup exists.** The full procedure (backup, apply, verify, rollback, what is and is not tested, the separate
PostGIS decision) is `docs/data/d2-migration-runbook.md`.

| Migration | Adds |
|---|---|
| `012_typed_families.sql` | `layout_versions`, hypertables `comm_event`, `data_session`, `app_usage`, `plate_sighting`, `subscriber_registration`, `subscriber_identity`, `normalize_plate()` |
| `013_reference_tables.sql` | `make_cell_key()`, `cell_site`, `camera_site`, `device_tac`, `ip_range_reference` |
| `014_entity_relationship_layer.sql` | `derivation`, `entity`, `entity_mention`, `relationship`, `relationship_evidence`, `ip_assignment` |
| `015_vectors_and_earth.sql` | extensions `vector`, `cube`, `earthdistance`; `content_embedding`; earth indexes |

Layout of this folder: the Docker entrypoint runs **every top-level `*.sql`** in alphabetical order when it initialises a new
database, so the helper scripts for 012 to 015 live in `ops/<migration>/` (`preflight.sql`, `verify.sql`, `smoke.sql`,
`rollback.sql`) and the optional PostGIS draft in `optional/` (the entrypoint ignores sub-folders). Run them the same way as the
older ones, with the path under the mount:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/ops/012_typed_families/preflight.sql
```

(The mount belongs to the main checkout; if 012 to 015 are not there yet, copy the reviewed folder into the container as the
runbook describes.) `smoke.sql` inserts synthetic evidence rows and rolls back: for disposable databases only.
`ops/Test-MigrationsThrowaway.ps1` runs the whole sequence, including rollbacks and idempotency, on a throwaway container
from the local image and prints `MigrationThrowawayTest=PASS`; add `-VolumeRows 500000` to also load synthetic events and print
load time, sizes and query timings (`ops/volume_probe.sql`).

Known problem, found while testing and not caused by 012 to 015: the auxiliary scripts of migrations 008 to 011 (`*.preflight.sql`,
`*.verify.sql`, `*.rollback.sql`, `*.smoke.sql`, `*.legacy_*.sql`, `*.rls.smoke.sql`, `*.idempotency.verify.sql`) sit at the top
level, so a **fresh** initialisation from this folder runs them in name order and stops at
`008_phase3_evidence_control_plane.idempotency.verify.sql` ("relation forensic.evidence_storage_objects does not exist"). Databases
created before 008 existed never hit it, because later migrations were applied by hand. With those scripts moved into `ops/`, a fresh
initialisation runs 001 to 015 and both runtime-role files and all four verify scripts pass. Moving them also means updating
the commands above and `scripts/apply_nexusai_r5_custody_fix.ps1`; not done here.
