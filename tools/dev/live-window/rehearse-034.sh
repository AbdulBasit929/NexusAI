#!/usr/bin/env bash
# REHEARSAL of migration 034 (conversations and the case layer) on a THROWAWAY database, never on the live one. HEAVY: take the lock first (C:\tmp\nexusai-locks\heavy.lock) and check free RAM (3 GB or more).
#   bash tools/dev/live-window/rehearse-034.sh                    build a fresh database from the migrations that are LIVE (001 to 021, 024 to 032), then
#        1. preflight, apply, verify; the behaviour test (rolled back); verify again; apply again (idempotency); verify again;
#        2. the rollback is refused without its gate, then runs with it and must leave the catalog exactly as it was before the migration (no catalog line added, changed or removed);
#        3. apply again and compare with the first application (the catalog must be identical).
#   SOURCE=dump DUMP=/path/to/live.dump bash tools/dev/live-window/rehearse-034.sh    the same on a RESTORED COPY of a live dump (what the deploy session does before the live window; the restore takes about 9 minutes)
#   KEEP=1 ...                                                    keep the container for the Go specs and write its connections to $OUT/conn.env; remove it with:  docker rm -f <name printed at the end>
# The container has a unique name, a 1 GB memory limit and a port published on 127.0.0.1 only. Without KEEP=1 it is removed when the script ends, also on failure.
export MSYS_NO_PATHCONV=1
REPO="$(cd "$(dirname "$0")/../../.." && pwd)"
MIG=034_conversations_and_case_layer
OPS=034_conversations
SOURCE=${SOURCE:-fresh}
OUT=${OUT:-$(mktemp -d)}
N=nx-casel-034-$(date +%H%M%S)-$RANDOM
PW=$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')
t() { date -u +"%H:%M:%S"; }
LOG="$OUT/rehearsal-034.log"
: > "$LOG"
say() { echo "$@" | tee -a "$LOG"; }
cleanup() { if [ -z "$KEEP" ]; then docker rm -f "$N" > /dev/null 2>&1; say "$(t) container removed"; else say "$(t) container kept: $N  (docker rm -f $N)"; fi; }
trap cleanup EXIT
sql() { docker exec "$N" psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall "$@"; }
snap() { docker exec -i "$N" psql -U localrecall -d localrecall -At -v ON_ERROR_STOP=1 -f - < "$REPO/db/forensic_records/ops/evidence/catalog_lines.sql" > "$OUT/$1.catalog.txt"
         docker exec -i "$N" psql -U localrecall -d localrecall -At -v ON_ERROR_STOP=1 -f - < "$REPO/db/forensic_records/ops/evidence/exact_counts.sql" > "$OUT/$1.counts.txt"; }
run_file() {  # a file below /tmp/nx-mig; prints rc and the lines that matter
  if out=$(sql -f "/tmp/nx-mig/$1" 2>&1); then rc=0; else rc=$?; fi
  echo "$out" >> "$LOG"
  printf '%s %-66s rc=%s %s\n' "$(t)" "$1" "$rc" "$(echo "$out" | grep -E 'verified|behaviour_ok|step [0-9]+ ok|ERROR|EXCEPTION|preflight passed|dropping' | tr '\n' ' ' | cut -c1-260)" | tee -a "$LOG"
  return $rc
}
fail() { say "FAILED: $1 (log: $LOG)"; exit 1; }

say "== rehearsal of migration 034, source=$SOURCE ($(t) UTC), output in $OUT"
if [ "$SOURCE" = "dump" ]; then DBNAME=scratch; else DBNAME=localrecall; fi
docker run --rm -d --name "$N" -m 1g --cpus 4 -p 127.0.0.1::5432 -e POSTGRES_USER=localrecall -e POSTGRES_PASSWORD="$PW" -e POSTGRES_DB=$DBNAME --shm-size=256m timescale/timescaledb:latest-pg16 > /dev/null || fail "docker run"
for i in $(seq 1 90); do
  if docker logs "$N" 2>&1 | grep -q "init process complete" && docker exec "$N" psql -U localrecall -d $DBNAME -At -c "select 1" > /dev/null 2>&1; then break; fi
  sleep 2
done
docker exec "$N" psql -U localrecall -d postgres -v ON_ERROR_STOP=1 -c "CREATE ROLE forensic_runtime LOGIN" -c "CREATE ROLE forensic_analyst_ro NOLOGIN" > /dev/null || fail "roles"
docker exec "$N" sh -c 'mkdir -p /tmp/nx-mig' && docker cp "$(cygpath -m "$REPO")/db/forensic_records/." "$N:/tmp/nx-mig" || fail "copy of the migrations"

if [ "$SOURCE" = "dump" ]; then
  [ -f "$DUMP" ] || fail "DUMP=$DUMP is not a file"
  docker exec "$N" psql -U localrecall -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE localrecall" > /dev/null || fail "create database"
  docker exec "$N" psql -U localrecall -d localrecall -v ON_ERROR_STOP=1 -c "CREATE EXTENSION IF NOT EXISTS timescaledb" -c "SELECT timescaledb_pre_restore()" > /dev/null || fail "pre_restore"
  docker cp "$(cygpath -m "$DUMP")" "$N:/tmp/live.dump" || fail "copy of the dump"
  say "$(t) pg_restore (serial: a parallel restore leaves the hypertables empty)"
  docker exec "$N" pg_restore -U localrecall -d localrecall /tmp/live.dump > "$OUT/restore.out" 2> "$OUT/restore.err"
  say "pg_restore exit=$? errors=$(grep -c 'error:' "$OUT/restore.err")"
  docker exec "$N" psql -U localrecall -d localrecall -v ON_ERROR_STOP=1 -c "SELECT timescaledb_post_restore()" > /dev/null || say "POST_RESTORE FAILED"
else
  say "$(t) building the live schema from the migrations (001 to 021, 024 to 032)"
  for f in $(cd "$REPO/db/forensic_records" && ls [0-9][0-9][0-9]_*.sql | grep -Ev '^(022|023|033|034)_' | sort); do
    run_file "$f" > /dev/null || fail "$f"
  done
  run_file runtime_role.grants.sql > /dev/null || fail "runtime_role.grants.sql"
fi
snap before
say "$(t) catalog lines before: $(wc -l < "$OUT/before.catalog.txt"), tables with counts: $(wc -l < "$OUT/before.counts.txt")"

say "== 1. preflight, apply, verify, behaviour, verify, apply again, verify"
run_file "ops/$OPS/preflight.sql" || fail "preflight"
run_file "$MIG.sql" || fail "apply"
run_file "ops/$OPS/verify.sql" || fail "verify"
snap applied
run_file "ops/$OPS/behaviour.sql" || fail "behaviour"
run_file "ops/$OPS/verify.sql" || fail "verify after behaviour"
run_file "$MIG.sql" || fail "second apply"
run_file "ops/$OPS/verify.sql" || fail "second verify"
snap reapplied
say "catalog differences applied vs applied-twice: $(diff "$OUT/applied.catalog.txt" "$OUT/reapplied.catalog.txt" | grep -c '^[<>]') (must be 0)"
say "what 034 changes in the catalog: added $(diff "$OUT/before.catalog.txt" "$OUT/applied.catalog.txt" | grep -c '^>'), removed $(diff "$OUT/before.catalog.txt" "$OUT/applied.catalog.txt" | grep -c '^<') (must be 0 removed), changed lines of earlier objects:"
diff "$OUT/before.catalog.txt" "$OUT/applied.catalog.txt" | grep '^<' | head -5 | cut -c1-200 | tee -a "$LOG"
say "row counts of the earlier tables, before vs applied: $(diff "$OUT/before.counts.txt" "$OUT/applied.counts.txt" | grep '^<' | wc -l) differences in existing tables (must be 0; the 15 new tables are additions)"

say "== 2. rollback: refused without the gate, then run with it; the catalog must be as before"
if sql -f "/tmp/nx-mig/ops/$OPS/rollback.sql" > "$OUT/rollback-nogate.out" 2>&1; then say "ROLLBACK RAN WITHOUT ITS GATE: wrong"; fail "gate"; else say "rollback refused without the gate: yes"; fi
if out=$(sql -c "SET app.caselayer_allow_destructive_rollback = 'true'" -f "/tmp/nx-mig/ops/$OPS/rollback.sql" 2>&1); then say "rollback with the gate: ok ($(echo "$out" | grep -c dropping) notices)"; else echo "$out" >> "$LOG"; fail "rollback"; fi
snap rolledback
say "catalog differences before vs rolled back: $(diff "$OUT/before.catalog.txt" "$OUT/rolledback.catalog.txt" | grep -c '^[<>]') (must be 0)"
diff "$OUT/before.catalog.txt" "$OUT/rolledback.catalog.txt" | head -6 | cut -c1-200 | tee -a "$LOG"

say "== 3. apply again after the rollback"
run_file "$MIG.sql" || fail "apply after rollback"
run_file "ops/$OPS/verify.sql" || fail "verify after rollback and apply"
run_file "ops/$OPS/behaviour.sql" || fail "behaviour after rollback and apply"
snap final
say "catalog differences first application vs after rollback and apply: $(diff "$OUT/applied.catalog.txt" "$OUT/final.catalog.txt" | grep -c '^[<>]') (must be 0)"

PORT=$(docker port "$N" 5432/tcp | head -1 | sed 's/.*://')
{
  echo "CASELAYER_TEST_DSN=postgres://forensic_runtime:$PW@127.0.0.1:$PORT/localrecall?sslmode=disable"
  echo "CASELAYER_TEST_OWNER_DSN=postgres://localrecall:$PW@127.0.0.1:$PORT/localrecall?sslmode=disable"
} > "$OUT/conn.env"
docker exec "$N" psql -U localrecall -d localrecall -v ON_ERROR_STOP=1 -c "ALTER ROLE forensic_runtime PASSWORD '$PW'" -c "CREATE TABLE IF NOT EXISTS public.nx_throwaway_marker (id int)" > /dev/null
say "$(t) DONE. Go specs: set the two variables of $OUT/conn.env, then  go test ./internal/caselayer/ -count=1  (the container must be kept: KEEP=1)"
