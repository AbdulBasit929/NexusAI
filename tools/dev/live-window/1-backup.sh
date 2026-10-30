#!/usr/bin/env bash
# LIVE WINDOW, step 1 (docs/data/migrations-027-031-runbook.md section 7): the catalog and exact-count baseline and a fresh validated backup of the LIVE database. Read only: nothing is changed.
# Run from Git Bash:  bash tools/dev/live-window/1-backup.sh        It writes into the git-ignored .phase2-backups folder of the main checkout and remembers the stamp for step 2.
export MSYS_NO_PATHCONV=1
REPO="$(cd "$(dirname "$0")/../../.." && pwd)"
MAIN="$(cd "$REPO/../NexusAI" && pwd)"
B="$MAIN/.phase2-backups"
mkdir -p "$B"
C=nexusai-forensic-postgres-1
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
echo "$STAMP" > "$B/.live-window-stamp"
q() { docker exec -i $C psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall "$@"; }
echo "window stamp $STAMP; $(date -u +%H:%M:%S) UTC; backups in $B"
q -At -f - < "$REPO/db/forensic_records/ops/evidence/catalog_lines.sql" > "$B/live-$STAMP.catalog-before.txt" || exit 1
q -At -f - < "$REPO/db/forensic_records/ops/evidence/exact_counts.sql" > "$B/live-$STAMP.exact-counts-before.txt" || exit 1
echo "baseline: $(wc -l < "$B/live-$STAMP.catalog-before.txt") catalog lines, $(wc -l < "$B/live-$STAMP.exact-counts-before.txt") tables counted"
docker exec $C pg_dump -U localrecall -d localrecall -Fc -f "/tmp/live-$STAMP.dump" || { echo "DUMP FAILED"; exit 1; }
echo "toc entries: $(docker exec $C pg_restore --list "/tmp/live-$STAMP.dump" | grep -c '^[0-9]')"
docker cp "$C:/tmp/live-$STAMP.dump" "$B/live-$STAMP.dump" || { echo "COPY FAILED"; exit 1; }
docker exec $C find /tmp -maxdepth 1 -name "live-$STAMP.dump" -delete
echo "bytes: $(stat -c %s "$B/live-$STAMP.dump")"
echo "sha256: $(sha256sum "$B/live-$STAMP.dump" | cut -d' ' -f1)"
echo "done $(date -u +%H:%M:%S) UTC"
