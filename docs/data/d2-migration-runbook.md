# Database changes (decision D2): migrations 012 to 015 - runbook - DRAFT, NOT APPLIED

Status: drafted and tested on a disposable database on 2026-10-12. **Nothing in this runbook has been run against the live NexusAI database.** Applying it needs a separate go-ahead from the owner, after the backup in section 4 has been taken and validated. Design source: `docs/roadmap/nexusai-maturity-roadmap-20261012.md`, sections 4.5 and 4.6 and 7.

## 1. What the four migrations add

All additive: no existing table, view, trigger, policy or row is changed. Each file is one transaction, idempotent, and has `preflight`, `verify`, `smoke` and `rollback` scripts under `db/forensic_records/ops/<name>/`.

| Migration | Adds | Why |
|---|---|---|
| `012_typed_families.sql` | `layout_versions`; hypertables `comm_event`, `data_session`, `app_usage`, `plate_sighting`; `subscriber_registration`, `subscriber_identity`; function `normalize_plate` | Typed call, internet, application and plate records with source time zone and time basis, number classes, lineage to the layout version and the evidence item, a raw payload per row, and identity data kept apart |
| `013_reference_tables.sql` | Functions `make_cell_key`, `norm_cell_part`; `cell_site`, `camera_site`, `device_tac`, `ip_range_reference` | Time-valid, append-only enrichment data with a source and licence on every row |
| `014_entity_relationship_layer.sql` | `derivation`, `entity`, `entity_mention`, `relationship`, `relationship_evidence`, `ip_assignment` | Link analysis: typed edges (observed, inferred, candidate), each explainable to rows, each inferred edge tied to the run that produced it |
| `015_vectors_and_earth.sql` | Extensions `vector` (0.8.1 is in the current image), `cube`, `earthdistance`; `content_embedding`; earth indexes on `cell_site` and `camera_site` | Vector search over text and image embeddings (no similarity index until a model is chosen) and "cells within N metres" as an index lookup |

Deliberately **not** included: PostGIS (not in the current image; section 9), face embeddings or a face relationship type (policy level L0 until the case-authority migration is approved), the `device` table of the roadmap (an `imei` entity plus `device_tac` covers it), `btree_gist` (nothing in 012 to 015 needs it), the worker's dual-write and any backfill, and any change to the semantic-layer views the governed SQL lane reads.

Every typed table has `tenant_id`, row-level security with the same `USING` and `WITH CHECK` policy as the rest of the schema, and composite foreign keys to `forensic.evidence_items (tenant_id, collection_id, evidence_id)` so a row cannot point at another tenant's evidence. The runtime role (`forensic_runtime`), when it exists, is granted `SELECT` and `INSERT` only on event tables; column-level `UPDATE` only for analyst review on `relationship` and relabelling on `entity`; nothing on `subscriber_identity`.

## 2. Risk, in plain terms

- **Existing data and queries:** untouched. The only statements that reach existing objects are `CREATE EXTENSION IF NOT EXISTS` (a catalog change; `pg_trgm` already exists) and foreign keys that *reference* `evidence_items` and `derived_artifacts` (a lookup, no lock on those tables beyond a brief share lock while the constraint is created on an empty table).
- **API and worker:** nothing reads or writes the new tables yet, so they need no restart and no rebuild. Do not re-run `runtime_role.grants.sql` on the live database as part of this change: it revokes and re-grants, which could briefly cut the runtime role off. Each migration grants what it needs directly.
- **Size and time:** the new tables are empty. The throwaway run applied all four in under three seconds.
- **Unknowns not tested:** the live database itself, very large volumes, concurrent writers, and compression on these hypertables (see section 10).

## 3. Preconditions

1. The owner has said "apply D2" in this conversation (the drafts being approved is not the go-ahead to apply).
2. The stack is healthy: `docker compose ... ps` shows `forensic-postgres` healthy.
3. Free disk on the Docker volume host is at least twice the database size (443 MB at the last check) for the dump and its copy.
4. The files to apply are the reviewed ones. The compose file mounts the **main checkout's** `db/forensic_records` into the container, which does not contain 012 to 015 yet; the procedure below copies the reviewed folder into the container's `/tmp` instead, so it works from either checkout.

## 4. Backup first (as the R5 migration does)

From the repository root in PowerShell. `.env.forensic-runtime.local` is read by compose; never print it.

```powershell
$env:COMPOSE_PROJECT_NAME = 'nexusai'
$base = @('compose','-p','nexusai','--env-file','.env.forensic-runtime.local','-f','docker-compose.forensic-records.yaml','-f','docker-compose.forensic-records.runtime.yaml')
$stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
$name = "d2-pre-migration-012-015-$stamp.dump"

# full custom-format dump (a consistent snapshot even with writers running), then validate it
docker @base exec -T forensic-postgres pg_dump -U localrecall -d localrecall -Fc -f "/tmp/$name"
docker @base exec -T forensic-postgres pg_restore --list "/tmp/$name" | Select-Object -First 5

# copy it out, record its hash, and keep it outside any repository
$id = docker @base ps -q forensic-postgres
New-Item -ItemType Directory -Force -Path .phase2-backups | Out-Null
docker cp "${id}:/tmp/$name" ".phase2-backups\$name"
(Get-FileHash -Algorithm SHA256 ".phase2-backups\$name").Hash.ToLowerInvariant()
(Get-Item ".phase2-backups\$name").Length
```

`.phase2-backups` is the folder the R5 script uses (check that it is git-ignored before relying on that). Optional but recommended once: restore the dump into a throwaway container (`docker run --rm -d ... timescale/timescaledb:latest-pg16`, then `pg_restore`), run `ops/012_typed_families/preflight.sql` there and compare row counts, to prove the backup restores before it is needed.

## 5. Apply

```powershell
# copy the reviewed folder into the running container (ephemeral, in /tmp); the mount is read-only and belongs to the main checkout
# (the trailing \. copies the contents, so running this again refreshes the copy; without it a second copy nests a forensic_records folder)
$id = docker @base ps -q forensic-postgres
docker cp .\db\forensic_records\. "${id}:/tmp/nx-mig"

function Run-Sql($file) { docker @base exec -T forensic-postgres psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall -f "/tmp/nx-mig/$file" }

foreach ($m in '012_typed_families','013_reference_tables','014_entity_relationship_layer','015_vectors_and_earth') {
  Run-Sql "ops/$m/preflight.sql"      # read-only; any exception means stop here
  Run-Sql "$m.sql"                    # one transaction
  Run-Sql "ops/$m/verify.sql"         # prints migration_0NN_verified
}
Run-Sql 'ops/010_runtime_rls_policies/verify.sql'   # every RLS table still has a policy
```

Stop at the first error: the failed migration rolled itself back (single transaction), earlier ones stay applied and verified, and the rollback scripts below undo them in reverse order if you want a clean slate. **Never run any `smoke.sql` on the live database**: the smoke scripts insert synthetic evidence rows into the real registry's tables (and roll back), and are for disposable databases only.

Then check the services (no restart needed): the API health endpoint and the worker metrics endpoint, as the R5 script does (`http://localhost:8091/healthz`, `http://localhost:9109/metrics`).

## 6. Roll back

Only if verify fails or the owner decides to undo. Reverse order, each one gated and destructive for its own tables:

```powershell
foreach ($pair in @(@('015_vectors_and_earth','app.vectors_allow_destructive_rollback'),
                    @('014_entity_relationship_layer','app.entity_layer_allow_destructive_rollback'),
                    @('013_reference_tables','app.reference_tables_allow_destructive_rollback'),
                    @('012_typed_families','app.typed_families_allow_destructive_rollback'))) {
  docker @base exec -T forensic-postgres psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall `
    -c "SET $($pair[1]) = 'true'" -f "/tmp/nx-mig/ops/$($pair[0])/rollback.sql"
}
```

Each rollback prints the row count of every table it drops. Extensions are left installed (the 015 rollback file says how to drop them by hand). The 012 rollback refuses while the relationship layer exists. If a rollback cannot be used, restore the dump from section 4 into a fresh database; do not improvise.

## 7. Evidence from the disposable database

`powershell -ExecutionPolicy Bypass -File .\db\forensic_records\ops\Test-MigrationsThrowaway.ps1` (starts a container from the local image, never pulls, no published port, random password, removed at the end):

- migrations 001 to 011 and the runtime-role grants applied as a fresh database sees them;
- for each of 012 to 015: preflight, apply, verify, smoke (rolled back), apply a second time (idempotent), grants file again, verify and smoke again with the runtime role present;
- 010 verify and the runtime RLS probe still pass; a catalog digest of everything created by 001 to 011 is identical before and after 012 to 015, and after a full rollback and re-application; legacy row counts unchanged;
- each rollback refuses without its gate and succeeds with it; the 012 rollback refuses while the relationship layer exists;
- a fresh database through the real Docker entrypoint (step "fresh initialisation through the Docker entrypoint", run first): the folder is bind-mounted read-only at `/docker-entrypoint-initdb.d` of a throwaway container, as compose mounts it. Only `NNN_name.sql` and `runtime_role.*` files may sit at the top level (the helper scripts of 008 to 011 used to, which stopped a fresh initialisation at the first of them; they now live in `ops/<migration>/`). The entrypoint must run exactly those 17 files in name order and finish, every moved helper script must be at its `ops/` path, and the verify scripts of 008 to 015 must pass in the database it built; the container and its anonymous volume are then removed and shown to be gone. Four deliberate breakages each failed the harness: a helper script back at the top level, a top-level migration that fails during initialisation (exit code 3, log tail printed), a moved helper script missing, and a verify script that raises an error;
- last runs: **61 steps passed in 59 s** without the volume probe and **62 steps in 158 s** with it, on PostgreSQL 16.14 (musl), TimescaleDB 2.28, pgvector 0.8.1. The earlier figures (59 steps in 29.5 s and 60 steps in 95.7 s) were taken before the fresh-initialisation step and a small step that checks how the operator scripts find a helper script in the old and in the new layout were added.

**Volume probe** (`-VolumeRows 500000`; one laptop, 1.5 GB container, default memory settings, synthetic events spread evenly over 90 days, about 240 bytes of JSON per row). Orientation only: it is not the pre-registered performance budget.

| Measure | Result |
|---|---|
| Load 500,000 call events with every index and foreign key in place | about 40 s (12,000 rows per second); 250,000 internet sessions about 20 s |
| Storage | about 1,390 bytes per call event (heap 326 MB, indexes 336 MB for 500,000) and 1,230 bytes per session; ten million events would be roughly 14 GB before compression, which was not measured |
| Biggest indexes, 500,000 events | content hash 43 MB, cell 42 MB, IMSI 40 MB, calling number 33 MB, IMEI 32 MB, called number 29 MB, evidence 28 MB; the content-hash and evidence indexes are the first to trim if load speed or disk matters |
| Contact profile of one number (top 20 counterparties) | 3 to 11 ms; the same under the non-owner role with row-level security 4 to 5 ms |
| Common contacts of two numbers; co-location of two numbers (same cell, ten minutes) | 8 ms; 8 ms |
| Tower dump (distinct numbers in one cell for one day); IP attribution by address, port and time | 2 ms; 7 ms |
| Re-loading 1,000 rows that already exist | inserts nothing (`ON CONFLICT DO NOTHING` on the idempotency key) |

Each number in the probe has about 250 events, so this shows that the indexes are used, not how a number with 200,000 events behaves; that, hot cells, concurrent writers and compression still need a larger and less uniform test.

The checks themselves were tested by breaking the migrations on purpose (14 mutations: a removed check constraint, a permissive row-level-security policy, an extra `UPDATE` grant, a wrong plate key, a face relationship type allowed, wrong cell-key zeros, a missing earth index, a removed rollback gate, a missing cascade, a missing idempotency key, readable identity data, a system-created person, an unordered symmetric edge, an inferred edge without a derivation): the harness failed on every one.

The real-data tools have their own evidence: 31 Python tests and 13 PowerShell checks, with 10 deliberate breakages all caught (`docs/data/real-data-handling-policy.md`).

## 8. What happens after the apply (not part of this change)

1. Layout registry v1 (files, not code) for the layouts seen so far, with synthetic goldens.
2. Worker dual-write into the typed tables, behind a switch, with idempotent upserts on `(tenant_id, collection_id, file_id, row_hash, start_utc)`.
3. Backfill from `forensic.records` and `forensic.cdr_records`; reconcile with SQL and `tools/real_data/reconcile_aggregates.py`.
4. Switch one family's semantic-layer views to the typed table; the lane's gates must hold or the family stays on the old view.

## 9. PostGIS (D2b) - separate decision, separate approval

PostGIS is **not available** in the current image (`pg_available_extensions` has no `postgis`); pgvector is, and is what 015 uses. `db/forensic_records/optional/016_postgis_geometry.sql` (with verify and rollback) is a syntax-checked draft, never run. Reasons to keep it separate:

- It needs a different image, which means a pull (a download the owner approves) and a **dump and restore**, not an in-place change: the current image is Alpine (musl), the usual PostGIS images are glibc, and the two can order text differently, so reusing the data directory would leave text indexes wrong.
- Migrations 013 and 015 already give radius lookups through `earthdistance` (spherical approximation). PostGIS adds containment (geofence polygons), line-of-travel, coverage footprints and ellipsoid distance.

Outline, if approved: choose and pin an image that ships PostgreSQL 16, TimescaleDB, pgvector and PostGIS (verify the tag and licence; candidates are the `timescaledb-ha` and `postgis/postgis` families); test the whole sequence on a throwaway container first; stop writers; full dump; start the new container on a new volume; restore; run all verify scripts and compare row counts; switch compose; keep the old volume until two weeks of parity. Downtime is the dump and restore time (minutes at 443 MB).

## 10. Open questions

- **Compression on hypertables with row-level security:** TimescaleDB compression on these tables is not enabled here. Check on the installed version (2.28.3) whether compression policies work with RLS before turning it on; `raw_payload` compresses well and the event tables will be the large ones.
- **Which role the API uses today:** if it connects as the owner (`localrecall`), RLS is bypassed and the runtime-role grants matter only after the non-owner activation in the existing runbook.
- **Chunk interval:** seven days matches the existing tables; revisit after real volumes are known.
- **Retention:** no deletion schedule is encoded. Case policy is set with counsel.
- **Face data:** the case-authority migration (roadmap R9) must exist before any face table or relationship type does.
