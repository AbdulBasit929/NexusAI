#!/usr/bin/env bash
# REHEARSAL of migration 032 (a hub threshold per case; a statement repeated across files counted once) on a RESTORED COPY of the live database, never on the live one.
#   bash tools/dev/live-window/1-backup.sh        first: a fresh read-only dump and the catalog and exact-count baseline of the live database (writes the stamp this script reads)
#   bash tools/dev/live-window/rehearse-032.sh    this: restore the dump into a throwaway container (same local image, no published port, removed on stop), then
#        1. preflight, apply, verify; the behaviour test (writes inside a transaction and always rolls back); verify again; apply again (idempotency); verify again;
#        2. the bank view as the read-only analyst role on every collection that has bank rows, before and after the migration: the counts must be equal except where a repeat across files exists;
#        3. the case setting as the application role: set, read back, a value outside 2 to 1000 refused, clear (all inside a rolled-back transaction);
#        4. the rollback (it must leave the catalog as it was) and the apply again.
# Prints the result of every step; the container is left running for a look and removed with:  docker rm -f <name printed at the end>
export MSYS_NO_PATHCONV=1
REPO="$(cd "$(dirname "$0")/../../.." && pwd)"
MAIN="$(cd "$REPO/../NexusAI" && pwd)"
B="$MAIN/.phase2-backups"
BW="$(cygpath -m "$B" 2>/dev/null || echo "$B")"   # docker.exe reads /c/... as C:\c\...: a Windows path for docker cp
STAMP=$(cat "$B/.live-window-stamp") || exit 1
DUMP="live-$STAMP.dump"
MIG=032_case_hub_threshold_and_statement_repeats
N=nx-rehearsal-032-$STAMP
PW=$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')
t() { date -u +"%H:%M:%S"; }
LOG="$B/rehearsal-032-$STAMP.log"
: > "$LOG"
say() { echo "$@" | tee -a "$LOG"; }
sql() { docker exec "$N" psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall "$@"; }
snap() { docker exec -i "$N" psql -U localrecall -d localrecall -At -v ON_ERROR_STOP=1 -f - < "$REPO/db/forensic_records/ops/evidence/catalog_lines.sql" > "$B/rehearsal-032-$STAMP.$1.catalog.txt"
         docker exec -i "$N" psql -U localrecall -d localrecall -At -v ON_ERROR_STOP=1 -f - < "$REPO/db/forensic_records/ops/evidence/exact_counts.sql" > "$B/rehearsal-032-$STAMP.$1.counts.txt"; }
run_file() {  # file
  if out=$(sql -f "/tmp/nx-mig/$1" 2>&1); then rc=0; else rc=$?; fi
  echo "$out" >> "$LOG"
  printf '%s %-62s rc=%s %s\n' "$(t)" "$1" "$rc" "$(echo "$out" | grep -E 'verified|behaviour_ok|ERROR|EXCEPTION|refus|stops|preflight passed|repeat|different amount|NOTICE' | tr '\n' ' ' | cut -c1-230)" | tee -a "$LOG"
  return $rc
}

say "== rehearsal of migration 032 on the restored copy of $DUMP ($(t) UTC)"
docker run --rm -d --name "$N" -e POSTGRES_USER=localrecall -e POSTGRES_PASSWORD="$PW" -e POSTGRES_DB=scratch --shm-size=256m timescale/timescaledb:latest-pg16 > /dev/null || { say "RUN FAILED"; exit 1; }
for i in $(seq 1 90); do
  if docker logs "$N" 2>&1 | grep -q "init process complete" && docker exec "$N" psql -U localrecall -d scratch -At -c "select 1" > /dev/null 2>&1; then break; fi
  sleep 2
done
docker exec "$N" psql -U localrecall -d postgres -v ON_ERROR_STOP=1 -c "CREATE ROLE forensic_runtime LOGIN" -c "CREATE ROLE forensic_analyst_ro NOLOGIN" -c "CREATE DATABASE localrecall" > /dev/null || { say "SETUP FAILED"; exit 1; }
docker exec "$N" psql -U localrecall -d localrecall -v ON_ERROR_STOP=1 -c "CREATE EXTENSION IF NOT EXISTS timescaledb" -c "SELECT timescaledb_pre_restore()" > /dev/null || { say "PRE_RESTORE FAILED"; exit 1; }
docker cp "$BW/$DUMP" "$N:/tmp/$DUMP" || { say "COPY FAILED"; exit 1; }
say "$(t) pg_restore (serial: a parallel restore leaves the hypertables empty)"
docker exec "$N" pg_restore -U localrecall -d localrecall "/tmp/$DUMP" > "$B/rehearsal-032-$STAMP.restore.out" 2> "$B/rehearsal-032-$STAMP.restore.err"
say "pg_restore exit=$? errors=$(grep -c 'error:' "$B/rehearsal-032-$STAMP.restore.err")"
docker exec "$N" psql -U localrecall -d localrecall -v ON_ERROR_STOP=1 -c "SELECT timescaledb_post_restore()" > /dev/null || say "POST_RESTORE FAILED"
snap before
say "restored copy against the live baseline: catalog differences $(diff "$B/live-$STAMP.catalog-before.txt" "$B/rehearsal-032-$STAMP.before.catalog.txt" | grep -c '^[<>]'), count differences $(diff "$B/live-$STAMP.exact-counts-before.txt" "$B/rehearsal-032-$STAMP.before.counts.txt" | grep -c '^[<>]')"
docker exec "$N" sh -c 'mkdir -p /tmp/nx-mig' && docker cp "$(cygpath -m "$REPO")/db/forensic_records/." "$N:/tmp/nx-mig" || { say "COPY FAILED"; exit 1; }

say "== 2 (before). the bank view as the analyst role, per collection that has bank rows, BEFORE the migration"
COLLS=$(sql -At -c "select tenant_id || '|' || collection_id from forensic.bank_transaction group by 1 order by 1")
count_view() {  # label
  for tc in $COLLS; do
    ten=${tc%%|*}; col=${tc##*|}
    n=$(sql -At -c "BEGIN; SET LOCAL ROLE forensic_analyst_ro; SELECT set_config('app.tenant_id','$ten',true); SELECT set_config('app.collection_id','$col',true); SELECT count(*) FROM analyst.v_bank_transactions; ROLLBACK;" 2>&1 | grep -E '^[0-9]+$' | head -1)
    stored=$(sql -At -c "select count(*) from forensic.bank_transaction where tenant_id='$ten' and collection_id='$col'")
    say "$1 $col: view $n rows, stored $stored"
  done
}
count_view before

say "== 1. preflight, apply, verify, behaviour, verify, apply again, verify"
run_file "ops/$MIG/preflight.sql" || { say "PREFLIGHT FAILED"; exit 1; }
run_file "$MIG.sql" || { say "APPLY FAILED"; exit 1; }
run_file "ops/$MIG/verify.sql" || { say "VERIFY FAILED"; exit 1; }
run_file "ops/$MIG/behaviour.sql" || { say "BEHAVIOUR FAILED"; exit 1; }
run_file "ops/$MIG/verify.sql" || say "VERIFY AFTER BEHAVIOUR FAILED"
run_file "$MIG.sql" || say "SECOND APPLY FAILED"
run_file "ops/$MIG/verify.sql" || say "SECOND VERIFY FAILED"
run_file "ops/010_runtime_rls_policies/verify.sql" || say "RLS VERIFY FAILED"
snap applied
say "behaviour test left nothing: counts after the behaviour test vs after the apply: $(docker exec "$N" psql -U localrecall -d localrecall -At -c 'select count(*) from forensic.bank_transaction')"

say "== 2 (after). the same counts AFTER the migration"
count_view after

say "== 3. the case setting as the application role (rolled back)"
FIRST=$(sql -At -c "select tenant_id || '|' || collection_id from forensic.evidence_items group by 1 order by count(*) desc limit 1")
TEN=${FIRST%%|*}; COL=${FIRST##*|}
res=$(sql -At -c "BEGIN; SET LOCAL ROLE forensic_runtime; SELECT set_config('app.tenant_id','$TEN',true);
  INSERT INTO forensic.case_record (tenant_id, collection_id, hub_threshold) VALUES ('$TEN','$COL',10) ON CONFLICT (tenant_id, collection_id) DO UPDATE SET hub_threshold = 10;
  SELECT 'set -> ' || hub_threshold FROM forensic.case_record WHERE tenant_id='$TEN' AND collection_id='$COL';
  UPDATE forensic.case_record SET hub_threshold = NULL WHERE tenant_id='$TEN' AND collection_id='$COL';
  SELECT 'cleared -> ' || coalesce(hub_threshold::text, 'NULL') FROM forensic.case_record WHERE tenant_id='$TEN' AND collection_id='$COL';
  ROLLBACK;" 2>&1 | tr '\n' ' ')
say "runtime role on $COL: $res"
bad=$(sql -At -c "BEGIN; SET LOCAL ROLE forensic_runtime; SELECT set_config('app.tenant_id','$TEN',true); UPDATE forensic.case_record SET hub_threshold = 1 WHERE tenant_id='$TEN'; ROLLBACK;" 2>&1 | grep -c 'case_record_hub_threshold_chk')
say "a threshold of 1 is refused by the table's check: $([ "$bad" -ge 1 ] && echo yes || echo NO)"

say "== 4. rollback (must restore the catalog), then apply again"
run_file "ops/$MIG/rollback.sql" || say "ROLLBACK FAILED"
snap rolledback
say "catalog differences before vs rolled back: $(diff "$B/rehearsal-032-$STAMP.before.catalog.txt" "$B/rehearsal-032-$STAMP.rolledback.catalog.txt" | grep -c '^[<>]')"
diff "$B/rehearsal-032-$STAMP.before.catalog.txt" "$B/rehearsal-032-$STAMP.rolledback.catalog.txt" | head -6 | cut -c1-200 | tee -a "$LOG"
run_file "$MIG.sql" || say "REAPPLY FAILED"
run_file "ops/$MIG/verify.sql" || say "REVERIFY FAILED"
snap reapplied
say "== what migration 032 changes in the catalog (before -> applied): added $(diff "$B/rehearsal-032-$STAMP.before.catalog.txt" "$B/rehearsal-032-$STAMP.applied.catalog.txt" | grep -c '^>'), removed $(diff "$B/rehearsal-032-$STAMP.before.catalog.txt" "$B/rehearsal-032-$STAMP.applied.catalog.txt" | grep -c '^<')"
diff "$B/rehearsal-032-$STAMP.before.catalog.txt" "$B/rehearsal-032-$STAMP.applied.catalog.txt" | cut -c1-230 | tee -a "$LOG" | head -14
say "row counts before vs applied: $(diff "$B/rehearsal-032-$STAMP.before.counts.txt" "$B/rehearsal-032-$STAMP.applied.counts.txt" | grep -c '^[<>]') differences"
say "$(t) DONE; container $N left running; remove it with: docker rm -f $N"
