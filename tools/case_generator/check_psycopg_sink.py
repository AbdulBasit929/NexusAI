#!/usr/bin/env python3
"""Proves the part the unit tests cannot: the production typed-ingest code with the worker's own driver, in the worker's own image, as the application role.

    python -X utf8 tools/case_generator/check_psycopg_sink.py CASE_DIR [--image nexusai-forensic-records-worker:latest]

It starts a throwaway PostgreSQL (local image only, no published port), applies migrations 001 to 015 and the runtime-role grants, registers the case's evidence items, and
runs tools/case_generator/psycopg_probe.py inside the worker image on the throwaway's network namespace, with `ingestion/forensic_records` mounted read-only and flat (as the
Dockerfile lays /app out), the case mounted read-only and a plan file. The probe connects as `forensic_runtime` (not the owner, no RLS bypass, INSERT-only on the typed tables),
so a missing grant, a temp-table refusal or a row-level-security rejection would show up here. Then every SQL check of the case is run on what the probe wrote:

    ok  = the checks pass, the second run wrote nothing, and the worker's hook wrote its audit row.

The image is only run (never rebuilt, pushed or started as a service); the throwaway container and its volume are removed at the end.
"""
import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import load_case  # noqa: E402
import test_ingest_v2 as t2  # noqa: E402
from ingestion.forensic_records.layout_registry import load_registry  # noqa: E402

DEFAULT_IMAGE = "nexusai-forensic-records-worker:latest"


def run_probe(db, image, case_dir, plan):
    with tempfile.TemporaryDirectory() as plan_dir:
        with open(os.path.join(plan_dir, "plan.json"), "w", encoding="utf-8") as handle:
            json.dump(plan, handle)
        flat = os.path.join(load_case.REPO, "ingestion", "forensic_records")
        command = ["docker", "run", "--rm", "--network", "container:" + db.name, "--memory", "1g", "--read-only", "--tmpfs", "/tmp",
                   "-v", flat + ":/v2:ro", "-v", HERE + ":/probe:ro", "-v", case_dir + ":/case:ro", "-v", plan_dir + ":/plan:ro",
                   "-e", "PLAN=/plan/plan.json", "-e", "PYTHONDONTWRITEBYTECODE=1", "--entrypoint", "python", image, "/probe/psycopg_probe.py"]
        proc = subprocess.run(command, capture_output=True)
        if proc.returncode != 0:
            raise RuntimeError("the probe failed (%d): %s" % (proc.returncode, proc.stderr.decode("utf-8", "replace")[-1500:]))
        return json.loads(proc.stdout.decode("utf-8"))


def check(case_dir, image=DEFAULT_IMAGE):
    registry = load_registry()
    db = load_case.Throwaway()
    try:
        db.start()
        load_case.prepare(db, registry)
        key = t2.register_evidence(db, case_dir)
        exports = []
        for item, _, name in load_case.export_list(key):
            evidence_id, batch_id = load_case.lineage_ids(key["case_id"], item["file"])
            exports.append({"name": name, "rel": item["file"], "evidence_id": evidence_id, "batch_id": batch_id})
        plan = {"tenant": load_case.TENANT, "case_id": key["case_id"], "case_dir": "/case", "exports": exports}
        out = run_probe(db, image, case_dir, plan)
        stats = t2.stats_from_summaries(key, out["first"])
        report = load_case.run_checks(db, key, stats)
        first = sum(s["rows_inserted"] for s in out["first"].values() if s.get("rows_inserted"))
        second = sum(s.get("rows_inserted") or 0 for s in out["second"].values())
        statuses = sorted({s["status"] for r in ("first", "second") for s in out[r].values()})
        report.add("every export loaded, none failed or skipped", statuses == ["loaded"], "statuses %s" % statuses)
        report.add("the second run wrote nothing", second == 0, "%d new rows" % second)
        hook = out["hook"] or {}
        rows = hook.get("audit_rows") or []
        report.add("the worker's hook wrote its audit row", len(rows) == 1 and rows[0][1] == "loaded" and rows[0][2] == "0", str(hook)[:300])
        iso = out.get("isolation") or {}
        fk, rls = iso.get("foreign_key") or {}, iso.get("row_level_security") or {}
        report.add("a foreign-key refusal is contained, named by its constraint", fk.get("status") == "failed" and fk.get("constraint") == "comm_event_evidence_fk", str(fk))
        report.add("a row-level-security refusal is contained", rls.get("status") == "failed", str(rls))
        report.add("after both the connection works and the job's rows are intact", iso.get("connection_usable") is True and iso.get("stray_rows") == 0
                   and iso.get("rows_before") == iso.get("rows_after"), str(iso))
        counts = {t: int(db.sql("SELECT count(*) FROM forensic.%s" % t)) for t in load_case.TABLE_OF.values()}
        return report, counts, first, out["psycopg"]
    finally:
        left = db.stop()
        if left:
            print("WARNING: " + left)


def main(argv=None):
    args = list(sys.argv[1:] if argv is None else argv)
    image = DEFAULT_IMAGE
    if "--image" in args:
        at = args.index("--image")
        image = args[at + 1]
        del args[at:at + 2]
    if len(args) != 1:
        print(__doc__)
        return 2
    report, counts, first, version = check(os.path.abspath(args[0]), image)
    for name, ok, detail in report.items:
        print("%s  %-46s %s" % ("ok  " if ok else "FAIL", name, detail[:160]))
    print("psycopg %s; rows written by the probe in its first run: %d; rows in the database: %s" % (version, first, counts))
    print("PSYCOPGSINK=%s checks=%d failed=%d" % ("FAIL" if report.failed else "PASS", len(report.items), len(report.failed)))
    return 1 if report.failed else 0


if __name__ == "__main__":
    sys.exit(main())
