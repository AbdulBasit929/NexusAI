#!/usr/bin/env python3
"""Backfill of `typed_headers` for evidence ingested before the ingest kept the file's own header list (engine round 9e, fault f).

    python -X utf8 tools/dev/backfill_typed_headers.py [--collection ID]                       # DRY RUN: the plan, from the database alone (counts, no cell, no file)
    python -X utf8 tools/dev/backfill_typed_headers.py --collection ID --files-root DIR [--show-names]
                                                                                                # DRY RUN: also reads the header row of copies of the files found in DIR
    $env:NEXUSAI_OWNER_GO_TYPED_HEADERS = 'yes'; python -X utf8 tools/dev/backfill_typed_headers.py --collection ID --files-root DIR --apply   # NOT RUN on the live database: needs the owner's go
    python -X utf8 tools/dev/backfill_typed_headers.py --container nx-9e-stage --files-root DIR --apply                                   # a restored throwaway copy: no go needed (this is how the write was rehearsed)

What it is for. A workbook in the old XML format (.xls) and a text export (.txt) are typed straight into the typed tables, whose rows keep their cells in a JSON object with no order, so the
file-rows page (GET /evidence/{id}/file-rows) laid their columns out in the database's key order. From round 9e the typed ingest keeps the file's header list in the evidence's metadata
(`typed_headers`) for new ingests. This script computes the same list for evidence that is already stored, so that old files show their columns in their own order too.

What it never does. It never reprocesses an evidence file and never writes a typed row: it reads the header row of a COPY of the file (or nothing, in the plain dry run) and, with --apply and the
owner's go, adds one key to `forensic.evidence_items.metadata`. The retained originals are not opened by this script: --files-root is a folder of copies the operator made. The plain dry run
reads only counts from the database. For the real-data collection use counts only (no --show-names).
"""
import argparse
import json
import os
import subprocess
import sys

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
TYPED_TABLES = ("comm_event", "data_session", "app_usage", "plate_sighting", "subscriber_registration", "bank_transaction")
CANDIDATES = """
SET app.tenant_id = 'default';
SELECT i.evidence_id::text, coalesce(nullif(i.original_filename, ''), i.source_file, ''), t.table_name, t.n
FROM forensic.evidence_items i
JOIN LATERAL (%s) t ON t.n > 0
WHERE %s NOT (i.metadata ? 'typed_headers') AND NOT EXISTS (SELECT 1 FROM forensic.records r WHERE r.evidence_id = i.evidence_id)
ORDER BY 2, 3;
""" % (" UNION ALL ".join("SELECT '%s' AS table_name, count(*) AS n FROM forensic.%s x WHERE x.evidence_id = i.evidence_id" % (t, t) for t in TYPED_TABLES), "%s")


LIVE = "nexusai-forensic-postgres-1"
CONTAINER = LIVE      # the database container; --container names a throwaway copy (only the live one needs the owner's go to be written)


def psql(sql):
    out = subprocess.run(["docker", "exec", "-i", CONTAINER, "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-F", "\t", "-v", "ON_ERROR_STOP=1"],
                         input=sql, capture_output=True, text=True, encoding="utf-8")
    if out.returncode != 0:
        raise SystemExit("psql failed: " + out.stderr[-400:])
    return [line.split("\t") for line in out.stdout.strip().splitlines() if line.strip() and not line.startswith("SET")]


class DiscardSink:
    """A sink that keeps nothing: the typed ingest is run only to find the header row of each sheet."""

    def __init__(self):
        from ingestion.forensic_records import typed_ingest as ti
        self.columns = sorted({c for t in TYPED_TABLES for c in [r[0] for r in psql(
            "SELECT column_name FROM information_schema.columns WHERE table_schema = 'forensic' AND table_name = '%s'" % t)]} | set(ti.REQUIRED_COLUMNS))

    def table_columns(self, table):
        return [(c, "text") for c in self.columns]

    def insert_batch(self, table, columns, rows):
        return len(rows)


def headers_of(path):
    sys.path.insert(0, REPO)
    from ingestion.forensic_records import typed_ingest as ti
    from ingestion.forensic_records.layout_registry import Policy
    lineage = ti.Lineage("default", "backfill", "00000000-0000-4000-8000-000000000001", os.path.basename(path), "00000000-0000-4000-8000-000000000002")
    summary = ti.ingest_file(path, lineage, Policy(source_tz="Asia/Karachi", tz_basis="declared"), DiscardSink())
    return ti.file_headers(summary)


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--collection", help="limit to one collection")
    ap.add_argument("--files-root", help="a folder of COPIES of the files, found by original file name")
    ap.add_argument("--show-names", action="store_true", help="print the header names (synthetic collections only)")
    ap.add_argument("--apply", action="store_true", help="write typed_headers (on the live database it needs NEXUSAI_OWNER_GO_TYPED_HEADERS=yes; always --files-root)")
    ap.add_argument("--container", default=LIVE, help="the Postgres container (default: the live one; a restored throwaway copy needs no go)")
    args = ap.parse_args(argv)
    global CONTAINER
    CONTAINER = args.container
    if args.apply and not args.files_root:
        raise SystemExit("--apply needs --files-root: the header row is read from copies of the files")
    if args.apply and CONTAINER == LIVE and os.environ.get("NEXUSAI_OWNER_GO_TYPED_HEADERS") != "yes":
        raise SystemExit("--apply on the LIVE database needs the owner's go (NEXUSAI_OWNER_GO_TYPED_HEADERS=yes); this is a dry run otherwise")
    where = ("i.collection_id = '%s' AND" % args.collection.replace("'", "''")) if args.collection else ""
    plan = psql(CANDIDATES % where)
    print("%d evidence items have typed rows, no v1 records and no typed_headers%s" % (len({r[0] for r in plan}), (" in " + args.collection) if args.collection else ""))
    for evidence_id, name, table, n in plan:
        line = "  %s  %-44s %-16s %6s typed rows" % (evidence_id, name, table, n)
        if args.files_root:
            path = os.path.join(args.files_root, name)
            if not os.path.isfile(path):
                print(line + "  (no copy in the folder)")
                continue
            found = headers_of(path)
            names = sum((s["headers"] for s in found), [])
            line += "  header row read: %d columns%s" % (len(names), (": " + ", ".join(names)) if args.show_names else "")
            if args.apply and found:
                payload = json.dumps(found).replace("'", "''")
                psql("SET app.tenant_id = 'default'; UPDATE forensic.evidence_items SET metadata = metadata || jsonb_build_object('typed_headers', '%s'::jsonb) "
                     "WHERE evidence_id = '%s'::uuid AND NOT (metadata ? 'typed_headers');" % (payload, evidence_id))
                line += "  WRITTEN"
        print(line)
    if not args.apply:
        print("dry run: nothing was written")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
