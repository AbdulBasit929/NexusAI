#!/usr/bin/env bash
# LIVE WINDOW for migration 032 (a hub threshold per case; a statement repeated across files counted once). Rehearsed first on a restored copy of the live database (rehearse-032.sh).
#   bash tools/dev/live-window/1-backup.sh        step 1: baseline and a fresh validated backup (writes the stamp this script reads)
#   bash tools/dev/live-window/2b-migration-032.sh   this: preflight, apply, verify, the 010 runtime-RLS verify, then the catalog and exact-count comparison with the baseline. STOPS at the first failed check.
#   .\tools\dev\live-window\3-recreate-api.ps1 -Apply -To <image tag> -RollbackTo <tag of the image that ran before>      step 3: the engine that reads the setting (dry run first)
# The migration is backward compatible with the engine that runs now (it neither reads the new column nor depends on the text of the view), so the order is not critical, but the new engine needs the column:
# apply this first. No ingest job may be in flight. The behaviour test (it writes inside a transaction and rolls back) is NOT run on the live database: it was run on the restored copy.
export MSYS_NO_PATHCONV=1
REPO="$(cd "$(dirname "$0")/../../.." && pwd)"
MAIN="$(cd "$REPO/../NexusAI" && pwd)"
B="$MAIN/.phase2-backups"
BW="$(cygpath -m "$B" 2>/dev/null || echo "$B")"   # Windows Python and docker.exe read /c/... as C:\c\...
C=nexusai-forensic-postgres-1
MIG=032_case_hub_threshold_and_statement_repeats
STAMP=$(cat "$B/.live-window-stamp") || { echo "run step 1 first"; exit 1; }
LOG="$B/live-$STAMP.apply-032.log"
: > "$LOG"
say() { echo "$@" | tee -a "$LOG"; }
q() { docker exec -i $C psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall "$@"; }
stop() { say "STOP: $1"; exit 1; }
cd "$REPO" || exit 1
say "== window $STAMP, migration 032 from $(git log --oneline -1 -- db/forensic_records | cut -c1-70), start $(date -u +%H:%M:%S) UTC"
inflight=$(q -At -c "select count(*) from forensic.records_ingest_jobs where status::text in ('queued','running')") || stop "could not count the jobs in flight"
[ "$inflight" = "0" ] || stop "jobs are in flight ($inflight): wait for the worker to be idle"
docker exec $C mkdir -p /tmp/nx-mig && docker cp "$(cygpath -m "$REPO")/db/forensic_records/." $C:/tmp/nx-mig || stop "could not copy the reviewed folder"
out=$(q -f /tmp/nx-mig/ops/$MIG/preflight.sql 2>&1) || { say "$out"; stop "preflight"; }
say "$(date -u +%H:%M:%S) preflight passed"
out=$(q -f /tmp/nx-mig/$MIG.sql 2>&1) || { say "$out"; stop "apply"; }
say "$(date -u +%H:%M:%S) applied"
out=$(q -f /tmp/nx-mig/ops/$MIG/verify.sql 2>&1) || { say "$out"; stop "verify"; }
say "$(date -u +%H:%M:%S) verify passed ($(echo "$out" | grep -o 'migration_[0-9]*_verified' | head -1))"
out=$(q -f /tmp/nx-mig/ops/010_runtime_rls_policies/verify.sql 2>&1) || { say "$out"; stop "010 runtime-RLS verify"; }
say "$(date -u +%H:%M:%S) 010 runtime-RLS verify passed"
q -At -f - < db/forensic_records/ops/evidence/catalog_lines.sql > "$B/live-$STAMP.catalog-after-032.txt" || stop "catalog after"
q -At -f - < db/forensic_records/ops/evidence/exact_counts.sql > "$B/live-$STAMP.exact-counts-after-032.txt" || stop "counts after"
say "-- catalog comparison (expected: the column, its check, its comment, the view and its comment; nothing else):"
python -X utf8 db/forensic_records/ops/evidence/catalog_compare.py "$BW/live-$STAMP.catalog-before.txt" "$BW/live-$STAMP.catalog-after-032.txt" | tee -a "$LOG" | cut -c1-250
say "-- row counts, before vs after (expected: no difference; the baseline is older than the first loads of this day, so audit rows may differ):"
diff "$B/live-$STAMP.exact-counts-before.txt" "$B/live-$STAMP.exact-counts-after-032.txt" | tee -a "$LOG"
say "-- the bank view as the analyst role, per collection (expected: 626, 626, 135 as before):"
for tc in $(q -At -c "select tenant_id || '|' || collection_id from forensic.bank_transaction group by 1 order by 1"); do
  ten=${tc%%|*}; col=${tc##*|}
  n=$(q -At -c "BEGIN; SET LOCAL ROLE forensic_analyst_ro; SELECT set_config('app.tenant_id','$ten',true); SELECT set_config('app.collection_id','$col',true); SELECT count(*) FROM analyst.v_bank_transactions; ROLLBACK;" 2>&1 | grep -E '^[0-9]+$' | head -1)
  say "   $col: $n rows"
done
say "done $(date -u +%H:%M:%S) UTC. NEXT: the engine (3-recreate-api.ps1), then set the case's hub threshold."
