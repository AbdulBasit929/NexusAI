# Forensic Records DB

This folder contains Phase 1 database migrations for the enterprise forensic records pipeline.

Apply with the TimescaleDB container in `docker-compose.forensic-records.yaml`; the SQL files are mounted into `/docker-entrypoint-initdb.d` on first database initialization, and the entrypoint runs every top-level `*.sql` there in name order. Only the numbered migrations and the two `runtime_role.*` files therefore sit at the top level; the helper scripts (preflight, verify, rollback, smoke, fixtures) live in `ops/<migration>/`. See "Folder layout and helper scripts" below.

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
  -f /docker-entrypoint-initdb.d/ops/008_phase3_evidence_control_plane/preflight.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/008_phase3_evidence_control_plane.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/ops/008_phase3_evidence_control_plane/verify.sql
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
`rls.smoke.sql` files (in `ops/008_phase3_evidence_control_plane/`) are synthetic
acceptance tools for a disposable database only. The RLS test assumes a temporary non-owner `NOBYPASSRLS` role, proves
same-tenant access, requires a cross-tenant write to fail, and rolls back the
role and test write. These files must not be run against a retained NexusAI
registry.

The rollback file drops the Phase 3 control-plane data and is therefore gated.
Run it only from a verified backup and with explicit approval:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -c "SET app.phase3_allow_destructive_rollback = 'true'" `
  -f /docker-entrypoint-initdb.d/ops/008_phase3_evidence_control_plane/rollback.sql
```

Phase 3 durable queue lifecycle migration:

```powershell
# Drain the legacy subscriber and verify a fresh scoped backup first. Retained
# migration application requires explicit approval.
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -c "SET app.phase3_allow_legacy_queue_recovery = 'true'" `
  -f /docker-entrypoint-initdb.d/ops/009_phase3_queue_lifecycle/legacy_recovery.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/ops/009_phase3_queue_lifecycle/preflight.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/009_phase3_queue_lifecycle.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/ops/009_phase3_queue_lifecycle/verify.sql
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

`ops/009_phase3_queue_lifecycle/smoke.sql` is for a disposable database after the 008
smoke fixture. It proves retry and terminal transitions, terminal immutability
and processing-run synchronization, then rolls back. The 009 rollback drops
operational history columns and is gated by
`SET app.phase3_allow_destructive_rollback = 'true'`; never use it on retained
evidence without a verified backup and explicit approval.

## Folder layout and helper scripts

The compose file mounts this folder read-only at `/docker-entrypoint-initdb.d`, and the PostgreSQL entrypoint runs **every
top-level `*.sql`** there, in name order, when it initialises an empty data directory. The top level therefore holds only what a
new database must run: the numbered migrations `001_*.sql` to `017_*.sql` and the two `runtime_role.*` files. Everything else
(preflight, verify, smoke, rollback, fixtures, one-off recovery tools) lives in `ops/<migration>/<name>.sql`, for example
`ops/010_runtime_rls_policies/verify.sql` or `ops/009_phase3_queue_lifecycle/legacy_recovery.sql`. The entrypoint ignores
sub-folders and this README; `optional/` holds drafts that nothing runs automatically.

Until 2026-10-12 the helper scripts of migrations 008 to 011 sat at the top level as `<migration>.<name>.sql`, so a fresh
initialisation ran them in name order and stopped at the first one (the 008 idempotency check: "relation
forensic.evidence_storage_objects does not exist"). They were moved with `git mv`, content unchanged.
`ops/Test-MigrationsThrowaway.ps1` now has a step, "fresh initialisation through the Docker entrypoint", that mounts this folder
read-only in a throwaway container exactly as compose does, checks that the entrypoint ran the top-level files and nothing
else, and runs the verify scripts of 008 to 015 in the database it built; a helper script put back at the top level fails it.

### Running a helper script while the live mount is still on the old layout

The live database container mounts the **main checkout's** `db/forensic_records`. Until that checkout is brought to this layout,
its mount still has the helper scripts of 008 to 011 under their old top-level names (`<migration>.<name>.sql`, such as the 010
verify script) and has no `ops/` folders for 012 to 015, so the `-f /docker-entrypoint-initdb.d/ops/...` commands in this README
fail there with "No such file or directory". Look at what the mount has:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec -T forensic-postgres ls /docker-entrypoint-initdb.d
```

Then either run the old name from the old mount (`-f /docker-entrypoint-initdb.d/<migration>.<name>.sql`; same content,
different path), or copy this checkout's folder into the container's `/tmp` and run from there. The copy works whatever the
mount holds and is what `docs/data/d2-migration-runbook.md` does:

```powershell
$id = docker compose -f docker-compose.forensic-records.yaml ps -q forensic-postgres
docker cp .\db\forensic_records\. "${id}:/tmp/nx-mig"

docker compose -f docker-compose.forensic-records.yaml exec -T forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /tmp/nx-mig/ops/010_runtime_rls_policies/verify.sql
```

The copy is ephemeral (it lives in that container's `/tmp` until the container is recreated) and changes nothing in the
database. The trailing `\.` copies the folder's contents, so repeating the command refreshes the copy; without it a second
`docker cp` nests a `forensic_records` folder inside the first. Copy from the checkout whose files you have reviewed. The rules
for the scripts themselves do not change: never run a `smoke.sql`, `legacy_fixture.sql` or `rls.smoke.sql` against a retained
database, and every `rollback.sql` and `legacy_recovery.sql` keeps its setting gate and needs a verified backup.

`scripts/apply_nexusai_r5_custody_fix.ps1` and `scripts/apply_forensic_runtime_gate4.ps1` look for each helper script at the new
path first and fall back to the old name, so they work against either layout; they stop before changing anything when a helper
is in neither place.

## Non-owner runtime activation

Before starting the API or worker with a non-owner database identity, close the
three Phase 1 policy gaps and verify that every RLS-enabled forensic table has at
least one policy:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/ops/010_runtime_rls_policies/preflight.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/010_runtime_rls_policies.sql

docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/ops/010_runtime_rls_policies/verify.sql
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
The preflight, verify and rollback scripts it runs are in `ops/011_r5_custody_chain_tail/`; the script finds them there or
under their old top-level names (see "Folder layout and helper scripts") and checks that all three exist before it stops
anything.

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

Layout of this folder: see "Folder layout and helper scripts" above. The helper scripts for 012 to 015 live in
`ops/<migration>/` (`preflight.sql`, `verify.sql`, `smoke.sql`, `rollback.sql`) and the optional PostGIS draft in `optional/`.
Run them the same way as the older ones, with the path under the mount:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/ops/012_typed_families/preflight.sql
```

(The mount belongs to the main checkout; if 012 to 015 are not there yet, use the copy-in procedure under "Running a helper
script while the live mount is still on the old layout", as the runbook does.) `smoke.sql` inserts synthetic evidence rows and rolls back: for disposable databases only.
`ops/Test-MigrationsThrowaway.ps1` runs the whole sequence, including rollbacks and idempotency, on a throwaway container
from the local image and prints `MigrationThrowawayTest=PASS`; add `-VolumeRows 500000` to also load synthetic events and print
load time, sizes and query timings (`ops/volume_probe.sql`).

## Effective record views (migration 016) - DRAFT, not applied

`016_effective_records.sql` adds **one view**, `forensic.records_effective_cdr`: the call-record family as the question lane reads it, with exactly the columns and
types of `forensic.records`, serving per evidence item the typed rows (`comm_event`, `dataset_kind = target_cdr`, repeats excluded) if it has any and its
`forensic.records` rows otherwise. It changes no table, row or policy, is `security_invoker` (the caller's row-level security applies), and is read only by the lane
when `FORENSIC_TYPED_FAMILIES` lists the family. Design, evidence and the steps to turn it on: `docs/data/typed-family-switch.md`. Helper scripts in
`ops/016_effective_records/` (`preflight.sql`, `verify.sql`, `smoke.sql`, `rollback.sql`); the rollback drops the view and nothing else, so it needs no gate, but
migration 012's rollback refuses while the view exists. `ops/Test-MigrationsThrowaway.ps1` drives 016 like 012 to 015.

## Analyst views and the read-only analyst role (migration 017) - DRAFT, not applied

`017_analyst_views.sql` adds schema `analyst` with seven reviewed, thick, scoped views (`v_comm_events`, `v_contact_pairs`, `v_cells`, `v_cell_visits`, `v_device_pairings`, `v_data_sessions`, `v_app_usage`) and the NOLOGIN role `forensic_analyst_ro`, which can read those views and nothing else; `forensic_runtime` may `SET LOCAL ROLE` to it. Scope is in the views (settings `app.tenant_id`, `app.collection_id`; unset returns no rows). Changes no table, row or policy. Design, evidence and traps: `docs/data/analyst-views.md`. Helper scripts in `ops/017_analyst_views/` (`preflight.sql`, `verify.sql`, `rollback.sql`); the test is `tools/case_generator/test_analyst_views.py`. The repository's throwaway migration gate does not drive 017 yet.
