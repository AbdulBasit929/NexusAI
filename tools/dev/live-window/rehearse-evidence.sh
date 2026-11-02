#!/usr/bin/env bash
# REHEARSAL of the evidence behind a figure (api/forensic_records/ask_engine_evidence.go) on a RESTORED COPY of the live database, never on the live one. Read only on the live side: it uses the dump
# that 1-backup.sh wrote (the stamp of .live-window-stamp, or the first argument), restores it into a throwaway container published on 127.0.0.1 only, and runs the opt-in spec
# "Ask engine evidence on the restored Orion case" as the application role.
#   bash tools/dev/live-window/rehearse-evidence.sh [STAMP]            restore, run the spec, remove the container
#   KEEP=1 bash tools/dev/live-window/rehearse-evidence.sh [STAMP]     the same, and keep the container (the restore takes about 9 minutes)
#   bash tools/dev/live-window/rehearse-evidence.sh [STAMP] --reuse    run the spec again on the kept container (about 3 minutes); remove it with:  docker rm -f nx-rehearsal-evidence-STAMP
export MSYS_NO_PATHCONV=1
REPO="$(cd "$(dirname "$0")/../../.." && pwd)"
MAIN="$(cd "$REPO/../NexusAI" && pwd)"
B="$MAIN/.phase2-backups"
BW="$(cygpath -m "$B" 2>/dev/null || echo "$B")"   # docker.exe reads /c/... as C:\c\...: a Windows path for docker cp
STAMP=${1:-$(cat "$B/.live-window-stamp")} || exit 1
DUMP="live-$STAMP.dump"
N=nx-rehearsal-evidence-$STAMP
CONN="$B/.rehearsal-evidence-$STAMP.conn"
t() { date -u +"%H:%M:%S"; }
LOG="$B/rehearsal-evidence-$STAMP.log"
say() { echo "$@" | tee -a "$LOG"; }
cleanup() { if [ -z "$KEEP" ] && [ "$2" != "--reuse" ]; then docker rm -f "$N" > /dev/null 2>&1; rm -f "$CONN"; say "$(t) container removed"; else say "$(t) container kept: $N"; fi; }
trap 'cleanup "$@"' EXIT

run_spec() {  # password, port
  export ASK_EVIDENCE_ORION_DSN="postgres://forensic_runtime:$1@127.0.0.1:$2/localrecall?sslmode=disable"
  say "$(t) running the spec against 127.0.0.1:$2"
  cd "$REPO" || exit 1
  go test ./api/forensic_records/ -run TestForensicRecordsSynthesis '-ginkgo.focus=Ask engine evidence on the restored Orion case' -count=1 -v 2>&1 | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^EVIDENCE|Ran [0-9]+ of|Passed|\[FAILED\]|Expected|to equal|to contain|FAIL|ok " | tee -a "$LOG"
  say "$(t) done"
}

if [ "$2" = "--reuse" ]; then
  [ -f "$CONN" ] && docker ps -q -f "name=$N" | grep -q . || { say "no kept container $N: run without --reuse (KEEP=1 to keep it)"; exit 1; }
  read -r PW PORT < "$CONN"
  run_spec "$PW" "$PORT"
  exit 0
fi

: > "$LOG"
PW=$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')
say "== rehearsal of the evidence behind a figure on the restored copy of $DUMP ($(t) UTC)"
docker run --rm -d --name "$N" -p 127.0.0.1::5432 -e POSTGRES_USER=localrecall -e POSTGRES_PASSWORD="$PW" -e POSTGRES_DB=scratch --shm-size=256m timescale/timescaledb:latest-pg16 > /dev/null || { say "RUN FAILED"; exit 1; }
for i in $(seq 1 90); do
  if docker logs "$N" 2>&1 | grep -q "init process complete" && docker exec "$N" psql -U localrecall -d scratch -At -c "select 1" > /dev/null 2>&1; then break; fi
  sleep 2
done
docker exec "$N" psql -U localrecall -d postgres -v ON_ERROR_STOP=1 -c "CREATE ROLE forensic_runtime LOGIN" -c "CREATE ROLE forensic_analyst_ro NOLOGIN" -c "CREATE DATABASE localrecall" > /dev/null || { say "SETUP FAILED"; exit 1; }
docker exec "$N" psql -U localrecall -d localrecall -v ON_ERROR_STOP=1 -c "CREATE EXTENSION IF NOT EXISTS timescaledb" -c "SELECT timescaledb_pre_restore()" > /dev/null || { say "PRE_RESTORE FAILED"; exit 1; }
docker cp "$BW/$DUMP" "$N:/tmp/$DUMP" || { say "COPY FAILED"; exit 1; }
say "$(t) pg_restore (serial: a parallel restore leaves the hypertables empty)"
docker exec "$N" pg_restore -U localrecall -d localrecall "/tmp/$DUMP" > "$B/rehearsal-evidence-$STAMP.restore.out" 2> "$B/rehearsal-evidence-$STAMP.restore.err"
say "pg_restore exit=$? errors=$(grep -c 'error:' "$B/rehearsal-evidence-$STAMP.restore.err")"
docker exec "$N" psql -U localrecall -d localrecall -v ON_ERROR_STOP=1 -c "SELECT timescaledb_post_restore()" > /dev/null || say "POST_RESTORE FAILED"
docker exec "$N" psql -U localrecall -d localrecall -v ON_ERROR_STOP=1 -c "ALTER ROLE forensic_runtime PASSWORD '$PW'" -c "GRANT forensic_analyst_ro TO forensic_runtime" -c "CREATE TABLE public.nx_throwaway_marker (id int)" > /dev/null || { say "ROLE SETUP FAILED"; exit 1; }
say "restored: $(docker exec "$N" psql -U localrecall -d localrecall -At -c "select count(*) from forensic.comm_event where collection_id='orion-lahore-reference-20261030'") Orion call events, $(docker exec "$N" psql -U localrecall -d localrecall -At -c "select count(*) from forensic.records where collection_id='orion-lahore-reference-20261030'") Orion source rows"
PORT=$(docker port "$N" 5432/tcp | head -1 | sed 's/.*://')
echo "$PW $PORT" > "$CONN"
run_spec "$PW" "$PORT"
