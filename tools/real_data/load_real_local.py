#!/usr/bin/env python3
"""Loads the owner's real files into a THROWAWAY database on this machine through the production typed ingest and reports aggregates only.

    python -I -X utf8 tools/real_data/load_real_local.py [--views-out PATH] [--keep-going]

Authority: the vault's register records the owner's extension of 2026-10-14 (local demonstration to the team lead who supplied the files, on this machine, the local model
allowed). This script prints counts, layout ids and statuses, never a value, a file name or a number from a file. Files are referred to as F01, F02, ... in a fixed order.
Nothing is written inside any repository: the output (--views-out) must be in the vault or another folder the location guard accepts. The throwaway database (local image only,
no published port, random password) and its volume are removed at the end, so the real rows exist only for the minutes this runs.

Files read: every .xlsx, .xls, .csv, .txt under <vault>\\working (the extracted archives) and the registered .xlsx under <vault>\\incoming. Archives inside archives (.zip, .rar)
are listed as skipped.
"""
import argparse
import collections
import hashlib
import json
import os
import sys
import uuid

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, REPO)
sys.path.insert(0, os.path.join(REPO, "tools", "case_generator"))
sys.path.insert(0, HERE)
import audit  # noqa: E402
import guard  # noqa: E402
import load_case  # noqa: E402
from ingestion.forensic_records import typed_ingest as ti  # noqa: E402
from ingestion.forensic_records.layout_registry import load_registry  # noqa: E402
from test_ingest_v2 import PsqlSink  # noqa: E402

VAULT = os.environ.get("NEXUSAI_REAL_DATA_VAULT", r"C:\NexusAI-RealDataVault")  # noqa: forbidigo-style note: this is a script, not the server
CASE_ID = "real-local-demo"
READABLE = {".xlsx", ".xls", ".csv", ".txt"}


def collect(vault):
    files, skipped = [], 0
    for root in (os.path.join(vault, "working"), os.path.join(vault, "incoming")):
        for dirpath, _, names in os.walk(root):
            for name in sorted(names):
                ext = os.path.splitext(name)[1].lower()
                if ext in READABLE:
                    files.append(os.path.join(dirpath, name))
                else:
                    skipped += 1
    unique, seen = [], set()
    for path in sorted(files):
        digest = sha256(path)
        if digest in seen:
            skipped += 1   # the same file in two folders is loaded once
            continue
        seen.add(digest)
        unique.append(path)
    return unique, skipped


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--views-out", help="write the interface view data (real numbers) here; must be outside every repository")
    parser.add_argument("--vault", default=VAULT)
    args = parser.parse_args(argv)
    if args.views_out:
        guard.require_safe_location(os.path.dirname(os.path.abspath(args.views_out)), role="output")
    files, skipped_archives = collect(args.vault)
    print("files to read: %d (skipped: archives and repeated copies, %d)" % (len(files), skipped_archives))
    registry = load_registry()
    db = load_case.Throwaway()
    try:
        db.start()
        load_case.prepare(db, registry)
        statements = ["SELECT set_config('app.tenant_id', %s, false);" % load_case.sql_text(load_case.TENANT)]
        lineage = {}
        for path in files:
            digest = sha256(path)
            evidence_id = str(uuid.uuid5(uuid.NAMESPACE_URL, "nexusai-real/" + digest))
            batch_id = str(uuid.uuid5(uuid.NAMESPACE_URL, "nexusai-real-batch/" + digest))
            lineage[path] = (evidence_id, batch_id, digest)
            statements.append(load_case.evidence_statement(evidence_id, CASE_ID, os.path.basename(path), os.path.splitext(path)[1].lstrip("."), os.path.getsize(path), digest))
        db.script("\n".join(statements))
        sink = PsqlSink(db)
        print("%-4s %-13s %-10s %-34s %9s %9s %11s" % ("file", "container", "status", "layout", "rows_read", "loaded", "quarantined"))
        totals = {"read": 0, "loaded": 0, "quarantined": 0}
        for index, path in enumerate(files, 1):
            evidence_id, batch_id, digest = lineage[path]
            summary = ti.ingest_file(path, ti.Lineage(load_case.TENANT, CASE_ID, evidence_id, "F%02d" % index, batch_id), audit.POLICY, sink, registry=registry)
            sheets = summary.get("sheets", [])
            layouts = sorted({s.get("layout_id") or "-" for s in sheets if s.get("status") == "loaded"}) or ["-"]
            read = sum(s.get("rows_read", 0) for s in sheets if s.get("status") == "loaded")
            loaded = sum(s.get("loaded", 0) for s in sheets if s.get("status") == "loaded")
            quarantined = sum(sum((s.get("quarantined") or {}).values()) for s in sheets if s.get("status") == "loaded")
            totals["read"] += read
            totals["loaded"] += loaded
            totals["quarantined"] += quarantined
            print("F%02d  %-13s %-10s %-34s %9d %9d %11d" % (index, summary.get("container", "-"), summary.get("status", "-"), ",".join(layouts)[:34], read, loaded, quarantined))
            reasons = collections.Counter()
            for s in sheets:
                for reason, count in (s.get("skipped") or {}).items():
                    reasons[reason] += count
                for reason, count in (s.get("quarantined") or {}).items():
                    reasons["quarantined:" + reason] += count
            if reasons:
                print("       reasons (codes and counts only): " + ", ".join("%s=%d" % kv for kv in sorted(reasons.items())))
        print("total rows read %(read)d, loaded %(loaded)d, quarantined %(quarantined)d" % totals)
        print("typed tables: " + ", ".join("%s=%s" % (t, db.sql("SELECT count(*) FROM forensic.%s" % t)) for t in ("comm_event", "data_session", "app_usage")))
        print("comm_event by dataset_kind: " + ", ".join("%s=%s" % (r["dataset_kind"], r["n"]) for r in db.rows("SELECT dataset_kind, count(*) AS n FROM forensic.comm_event GROUP BY 1 ORDER BY 1")))
        for r in db.rows("SELECT layout_id, count(*) AS n, count(cell_key) AS with_cell_key, count(lat) AS with_lat, count(a_norm) AS with_a, count(imei) AS with_imei, count(imsi) AS with_imsi, "
                         "count(DISTINCT a_norm) AS numbers FROM forensic.comm_event GROUP BY layout_id ORDER BY layout_id"):
            print("  %-26s rows %6d  cell_key %6d  lat %6d  a_norm %6d  imei %6d  imsi %6d  distinct numbers %5d" % (r["layout_id"], r["n"], r["with_cell_key"], r["with_lat"], r["with_a"], r["with_imei"], r["with_imsi"], r["numbers"]))
        if args.views_out:
            import export_views_real
            data = export_views_real.build(db, CASE_ID)
            with open(args.views_out, "w", encoding="utf-8", newline="\n") as handle:
                json.dump(data, handle, separators=(",", ":"), sort_keys=True)
            print("views written (%d nodes, %d edges, %d cells, %d movers, %d pairs, %d rhythm, %d addresses)" % (
                len(data["graph"]["nodes"]), len(data["graph"]["edges"]), len(data["map"]["cells"]), len(data["map"]["movers"]), len(data["pairs"]), len(data["rhythm"]), len(data["addresses"]), ))
            print("tower dumps in the views: %d (cells %d)" % (len(data["map"]["dumps"]), sum(len(d["cells"]) for d in data["map"]["dumps"])))
        return 0
    finally:
        left = db.stop()
        if left:
            print("WARNING: " + left)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
