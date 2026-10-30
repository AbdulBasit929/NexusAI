#!/usr/bin/env bash
# LIVE WINDOW, steps 2 and 3 (docs/data/migrations-027-031-runbook.md section 7): migrations 027 to 031 on the LIVE database (preflight, apply, verify each), the 010 runtime-RLS verify, the four
# missing layouts, then the catalog and row-count comparison with the baseline of step 1. STOPS at the first failed check. Every migration is one transaction; the rollback is in section 4.
# Run from Git Bash, directly after step 1 and directly before step 3 (the engine must follow within minutes: the running engine reads the new, empty views from the database):
#   bash tools/dev/live-window/2-migrations.sh
export MSYS_NO_PATHCONV=1
REPO="$(cd "$(dirname "$0")/../../.." && pwd)"
MAIN="$(cd "$REPO/../NexusAI" && pwd)"
B="$MAIN/.phase2-backups"
C=nexusai-forensic-postgres-1
STAMP=$(cat "$B/.live-window-stamp") || { echo "run step 1 first"; exit 1; }
LOG="$B/live-$STAMP.apply.log"
: > "$LOG"
say() { echo "$@" | tee -a "$LOG"; }
q() { docker exec -i $C psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall "$@"; }
stop() { say "STOP: $1"; exit 1; }
cd "$REPO" || exit 1
say "== window $STAMP, migrations from $(git log --oneline -1 -- db/forensic_records | cut -c1-60), start $(date -u +%H:%M:%S) UTC"
LAYOUTS="$(mktemp)"
python -X utf8 -m ingestion.forensic_records.layout_registry sql > "$LAYOUTS" 2>/dev/null || stop "could not generate the layout SQL"
docker exec $C mkdir -p /tmp/nx-mig && docker cp db/forensic_records/. $C:/tmp/nx-mig || stop "could not copy the reviewed folder"
for m in 027_plate_sightings 028_subscriber_device_views 029_bank_transactions 030_transaction_views 031_document_mentions; do
  out=$(q -f /tmp/nx-mig/ops/$m/preflight.sql 2>&1) || { say "$out"; stop "preflight $m"; }
  out=$(q -f /tmp/nx-mig/$m.sql 2>&1) || { say "$out"; stop "apply $m"; }
  out=$(q -f /tmp/nx-mig/ops/$m/verify.sql 2>&1) || { say "$out"; stop "verify $m"; }
  say "$(date -u +%H:%M:%S) $m: preflight, apply, verify passed ($(echo "$out" | grep -o 'migration_[0-9]*_verified' | head -1))"
done
out=$(q -f /tmp/nx-mig/ops/010_runtime_rls_policies/verify.sql 2>&1) || { say "$out"; stop "010 runtime-RLS verify"; }
say "$(date -u +%H:%M:%S) 010 runtime-RLS verify passed"
out=$(q -f - < "$LAYOUTS" 2>&1) || { say "$out"; stop "layout recording"; }
say "$(date -u +%H:%M:%S) layouts recorded: $(q -At -c "select count(*) from forensic.layout_versions") rows now"
q -At -f - < db/forensic_records/ops/evidence/catalog_lines.sql > "$B/live-$STAMP.catalog-after.txt" || stop "catalog after"
q -At -f - < db/forensic_records/ops/evidence/exact_counts.sql > "$B/live-$STAMP.exact-counts-after.txt" || stop "counts after"
say "-- catalog comparison (expected: 407 added, 0 removed, 3 changed):"
python -X utf8 db/forensic_records/ops/evidence/catalog_compare.py "$B/live-$STAMP.catalog-before.txt" "$B/live-$STAMP.catalog-after.txt" | tee -a "$LOG" | cut -c1-250
say "-- row counts, before vs after (expected: only the two new tables, empty):"
diff "$B/live-$STAMP.exact-counts-before.txt" "$B/live-$STAMP.exact-counts-after.txt" | tee -a "$LOG"
say "-- layouts live: $(q -At -c "select string_agg(layout_id, ',' order by layout_id) from forensic.layout_versions where layout_id in ('anpr-a1','subscriber-s1','bank-stmt-t1','wallet-t1')")"
say "done $(date -u +%H:%M:%S) UTC. NEXT: step 3, the engine, now."
