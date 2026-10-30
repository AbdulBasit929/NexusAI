# Migrations 027 to 031: rehearsal record and runbook for the live application

Written 2026-10-30. **Nothing in this document has been applied to the live database.** Applying it needs the owner's explicit go. The rehearsal below ran on a restored copy of the live database in a throwaway container; the live database was only read (one `pg_dump`, two catalog queries).

## 1. What the five migrations do

| Migration | Adds | Changes |
|---|---|---|
| 027 plate sightings | views `v_plate_sightings`, `v_plate_visits` | nothing (the table is from 012) |
| 028 subscriber and device views | views `v_subscriber_registrations`, `v_number_registration_activity`, `v_handsets` | `forensic.subscriber_registration.time_basis` may be empty (a registration without a date has no time basis) and its check allows that |
| 029 bank transactions | table `forensic.bank_transaction` (row-level security, grants, indexes) | the list of layout kinds accepts `bank_transaction` |
| 030 transaction views | views `v_bank_transactions`, `v_account_counterparties`, `v_account_activity`, `v_account_flows`, `v_account_limit_bands` | nothing |
| 031 document mentions | table `forensic.document_mention`, views `v_document_mentions`, `v_document_values`, `v_document_links` | nothing |

Measured on the restored copy: **407 catalog lines added (183 columns, 148 comments, 21 constraints, 15 grants, 10 indexes, 2 policies, 15 tables and views), 0 removed, 3 changed** (the `time_basis` column and its check, and the layout kind check: all three are loosenings that no existing row can violate). Both new tables are empty. Every existing table keeps its exact row count.

**Order with the engine: the two must change in one short window.** The Ask engine reads the analyst views from the database when it builds its catalogue (every view and column with a description; cached five minutes), and its coverage query reads `forensic.bank_transaction` and `forensic.document_mention`. (a) A NEW engine (commit 0ac8e02 or later) against a database without 027 to 031 cannot read its coverage and declines every question to the older path: safe, but the engine is off. (b) The engine now running live (image 4eebc53ef7f2, `FORENSIC_ASK_V2_FIRST=true`) does not know the new families: after the migrations it would list the 13 new, empty views in its catalogue with no coverage rule for them, and could shortlist one for a question and answer from an empty table (the confident zero the coverage guard exists to prevent). So the migrations are applied and the new engine is deployed one directly after the other, and the questions of section 3 are asked in between only if the deploy is delayed. A rollback of a migration needs the new engine rolled back first (each rollback script says so).

## 2. Rehearsal record (2026-10-30)

* **Backup:** `pg_dump -Fc` of the live database (PostgreSQL 16.14, TimescaleDB 2.28.3, 3,123 MB) in about three minutes with the copy out of the container: `pre-027-031-20261030T044743Z.dump`, 456,579,451 bytes, SHA-256 `5f639b2393fb795a56473842dbb80a54eb7b9aca74782209d0ee0ab56c34f175`, 4,964 table-of-contents entries, kept in the git-ignored `.phase2-backups` of the main checkout with the catalog listing (1,855 lines) and the exact row counts of the 39 tables.
* **Restore proven:** into a throwaway container of the same local image (`timescale/timescaledb:latest-pg16`, no pull, no published port), serially: 0 errors, the exact row counts of all 39 tables identical to live (largest: `record_entities` 440,132, `records` 166,377, `comm_event` 119,452), 2 of 1,855 catalog lines printed differently after the round trip (a check constraint's brackets, an owner-only ACL), no difference in meaning.
* **Rehearsal on the restored copy:** for each of 027 to 031: preflight, apply, verify (`migration_0NN_verified`), all passed; the whole sequence took 11 seconds; the 010 runtime-RLS verify passed; a second apply and verify of all five changed nothing (0 catalog differences); the 13 new views counted as the read-only analyst role on the largest real collection in 1.2 seconds in total; the five rollbacks in reverse order brought the catalog back to exactly the starting one (0 differences); applying all five again gave exactly the catalog of the first apply (0 differences).
* **Evidence files** (git-ignored, in `.phase2-backups`): `rehearsal-027-031-20261030T044743Z.log` (every step), `.comparison.txt` (the four catalog comparisons and the row counts), `.catalog-after-apply.txt`.

## 3. Procedure for the live application (when the owner says go)

Preconditions: the stack is healthy (`docker ps` shows `nexusai-forensic-postgres-1` healthy); the worker is idle (no upload being typed); a fresh backup is taken **immediately before** (the data may have changed since the one above); the files applied are the reviewed ones of the lane checkout (`db/forensic_records`, commit 0ac8e02 or later), not the main checkout's mount.

```bash
export MSYS_NO_PATHCONV=1                      # Git Bash only
C=nexusai-forensic-postgres-1
q() { docker exec -i $C psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall "$@"; }
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
# 1. baseline (read-only) and backup (consistent snapshot, validated, copied out, hashed)
q -At -f - < db/forensic_records/ops/evidence/catalog_lines.sql > .phase2-backups/live-$STAMP.catalog-before.txt
q -At -f - < db/forensic_records/ops/evidence/exact_counts.sql  > .phase2-backups/live-$STAMP.exact-counts-before.txt
docker exec $C pg_dump -U localrecall -d localrecall -Fc -f /tmp/live-$STAMP.dump
docker exec $C pg_restore --list /tmp/live-$STAMP.dump | grep -c '^[0-9]'
docker cp $C:/tmp/live-$STAMP.dump .phase2-backups/live-$STAMP.dump && sha256sum .phase2-backups/live-$STAMP.dump
# 2. the reviewed folder into the container (the trailing /. copies the contents; the mount is the main checkout's and read-only)
docker exec $C mkdir -p /tmp/nx-mig && docker cp db/forensic_records/. $C:/tmp/nx-mig
# 3. each migration: preflight (read-only; stop on any error), apply (one transaction), verify
for m in 027_plate_sightings 028_subscriber_device_views 029_bank_transactions 030_transaction_views 031_document_mentions; do
  q -f /tmp/nx-mig/ops/$m/preflight.sql && q -f /tmp/nx-mig/$m.sql && q -f /tmp/nx-mig/ops/$m/verify.sql || { echo "STOP at $m"; break; }
done
q -f /tmp/nx-mig/ops/010_runtime_rls_policies/verify.sql
# 4. after: the catalog must differ from the baseline by the additions and the three loosenings only, the counts by the two new tables only
q -At -f - < db/forensic_records/ops/evidence/catalog_lines.sql > .phase2-backups/live-$STAMP.catalog-after.txt
q -At -f - < db/forensic_records/ops/evidence/exact_counts.sql  > .phase2-backups/live-$STAMP.exact-counts-after.txt
python -X utf8 db/forensic_records/ops/evidence/catalog_compare.py .phase2-backups/live-$STAMP.catalog-before.txt .phase2-backups/live-$STAMP.catalog-after.txt
diff .phase2-backups/live-$STAMP.exact-counts-before.txt .phase2-backups/live-$STAMP.exact-counts-after.txt
```

Expected: `added objects: 407, removed objects: 0, changed objects: 3`, and the count diff lists only `forensic.bank_transaction|0` and `forensic.document_mention|0`. Anything else: stop, do not deploy the engine, and use section 4.

After the apply: `/healthz` of the forensic API and the worker metrics answer 200; all containers stay up (the migrations need no restart). Do not run any `smoke.sql` on the live database.

## 4. Rollback

Reverse order, each script is gated (a table that holds rows is never dropped; the script stops and changes nothing):

```bash
for m in 031_document_mentions 030_transaction_views 029_bank_transactions 028_subscriber_device_views 027_plate_sightings; do q -f /tmp/nx-mig/ops/$m/rollback.sql || break; done
```

Rehearsed: this brings the catalog back to exactly the starting one. If something else went wrong, restore the dump into a fresh database (section 5) and switch over; the live database is never edited by hand.

## 5. Traps found during the rehearsal

* Restore **serially**. A parallel `pg_restore -j 2` failed on TimescaleDB's own catalog tables (foreign key order) and silently left the hypertables empty (`comm_event`, `cdr_records`, `app_usage`): 10 row-count differences. Create the roles `forensic_runtime` (login) and `forensic_analyst_ro` (nologin) first, create the extension, run `SELECT timescaledb_pre_restore()` before and `SELECT timescaledb_post_restore()` after.
* A new `timescale/timescaledb` container restarts its server once after its init scripts: wait for "init process complete" in the logs and a successful query, not for `pg_isready` (the first server answers it and then shuts down).
* Use `docker run --rm` so that stopping the container removes it and its anonymous volume. Compare the restored copy with the live baseline first, and compare the migrations with the restored copy's own baseline.
* The PowerShell tool blocks commands that contain `rm`; use the Git Bash tool for these commands, or `find ... -delete`.

## 6. What the new families need live besides these migrations (NOT rehearsed, NOT done)

The migrations create the tables and views. Nothing writes into the new tables until all of this is in place; until then every question about a plate, a registration, a statement or a document is declined by the coverage guard with the reason "the case has no typed ...", which is the intended behaviour.

1. **Four layouts to record.** The registry holds 24 layout rows; the live `forensic.layout_versions` holds 21. Missing live: `anpr-a1`, `subscriber-s1`, `bank-stmt-t1`, `wallet-t1`. Recording them is the SQL printed by `python -m ingestion.forensic_records.layout_registry sql` (insert-if-absent with a guard against a changed definition), after 029 (which lets `bank_transaction` be a layout kind), in one transaction, as in section 12 of `d2-migration-runbook.md`.
2. **The worker.** The worker image running live (the Ingest v2 overlay `ac6388c57cbc`) has the typed readers of the call and internet records only. The readers of the new families (plates, registrations and devices, statements as CSV, PDF and Word, wallet lists), the mention index of documents and the container sniffing for PDF and Word are in the lane; they reach the live worker as a new overlay on the same image (not a full rebuild), with a rollback tag.
3. **The switch.** `FORENSIC_TYPED_FAMILIES` is `communications_cdr,network_ipdr` live; the new families are written only when it names them. Turning it on changes what a new upload does and is a separate act from the migrations.
4. **The engine image.** Built from the lane, cross-compiled on the host, with a rollback tag of the running image (as at each earlier deploy), recreated only through the Run-Arms helpers, then the interface workspace restarted.
5. **A live smoke.** Load the Orion Lahore case through the real upload path into a new collection, ask a set of questions at :4181 for each family, read every sentence, compare with the oracle answers, and keep the rollback ready.
