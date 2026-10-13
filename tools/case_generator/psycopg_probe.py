"""Runs INSIDE the worker image (see check_psycopg_sink.py): the production typed-ingest code with the worker's own database driver and role.

It is mounted next to a read-only copy of ingestion/forensic_records laid out flat (as the Dockerfile lays the worker out in /app), a read-only
generated case and a plan file. It connects to the throwaway database as the application role (forensic_runtime: not the owner, no RLS bypass,
INSERT-only on the typed tables), gives every export to `typed_ingest.run_shadow` in one transaction, then runs the same exports again to prove they
write nothing, and finally calls the worker's own hook (`run_typed_ingest_shadow`) once to prove the wiring and the audit row.
It prints one JSON document and nothing else on stdout. Nothing here reads a real file or reaches a network.
"""
import dataclasses
import json
import os
import sys
import traceback
from pathlib import Path

FLAT = os.environ.get("FLAT_DIR", "/v2")
sys.path.insert(0, FLAT)

import psycopg  # noqa: E402  (the worker's driver, from the image)
import typed_ingest as ti  # noqa: E402  (flat import: the module's own fallback finds layout_registry beside it)

TIME_POLICY = {"source_timezone": "Asia/Karachi", "timezone_state": "profile_defaulted"}


def main():
    with open(os.environ["PLAN"], encoding="utf-8") as handle:
        plan = json.load(handle)
    out = {"psycopg": psycopg.__version__, "first": {}, "second": {}, "hook": None}
    conninfo = "host=127.0.0.1 dbname=localrecall user=forensic_runtime"
    with psycopg.connect(conninfo, autocommit=False) as conn:
        conn.execute("SELECT set_config('app.tenant_id', %s, true)", (plan["tenant"],))
        for round_name in ("first", "second"):
            for item in plan["exports"]:
                lineage = ti.Lineage(plan["tenant"], plan["case_id"], item["evidence_id"], item["rel"], item["batch_id"])
                summary = ti.run_shadow(conn, os.path.join(plan["case_dir"], *item["rel"].split("/")), lineage, TIME_POLICY, mode="shadow")
                out[round_name][item["name"]] = summary
            conn.commit()
            conn.execute("SELECT set_config('app.tenant_id', %s, true)", (plan["tenant"],))
        out["hook"] = run_hook(conn, plan)
        out["isolation"] = run_isolation(conn, plan)
        conn.commit()
    print(json.dumps(out, sort_keys=True))


def run_isolation(conn, plan):
    """Two real database refusals inside the shadow: neither may leave the connection unusable or touch what the job already wrote in the same transaction."""
    item = plan["exports"][0]
    path = os.path.join(plan["case_dir"], *item["rel"].split("/"))
    result = {}
    with conn.cursor() as cur:
        cur.execute("SELECT count(*) FROM forensic.comm_event WHERE tenant_id = %s", (plan["tenant"],))
        result["rows_before"] = cur.fetchone()[0]
    # 1. an evidence item that does not exist: the foreign key refuses the first batch
    stray = ti.Lineage(plan["tenant"], plan["case_id"], "00000000-0000-4000-8000-00000000dead", "stray/" + item["rel"], item["batch_id"])
    summary = ti.run_shadow(conn, path, stray, TIME_POLICY, mode="shadow")
    result["foreign_key"] = {"status": summary["status"], "reason": summary.get("reason"), "constraint": summary.get("constraint")}
    # 2. rows for a tenant other than the one the connection is scoped to: row-level security refuses them
    other = ti.Lineage("another-tenant", plan["case_id"], item["evidence_id"], "other/" + item["rel"], item["batch_id"])
    summary = ti.run_shadow(conn, path, other, TIME_POLICY, mode="shadow")
    result["row_level_security"] = {"status": summary["status"], "reason": summary.get("reason")}
    with conn.cursor() as cur:   # the connection and the outer transaction are still fine
        cur.execute("SELECT 1")
        result["connection_usable"] = cur.fetchone()[0] == 1
        cur.execute("SELECT count(*) FROM forensic.comm_event WHERE tenant_id = %s", (plan["tenant"],))
        result["rows_after"] = cur.fetchone()[0]
        cur.execute("SELECT count(*) FROM forensic.comm_event WHERE file_id IN (%s, %s)", ("stray/" + item["rel"], "other/" + item["rel"]))
        result["stray_rows"] = cur.fetchone()[0]
    return result


def run_hook(conn, plan):
    """The worker's own hook on the first export, as process_job calls it: switch on, a real job, a real cursor."""
    try:
        import worker
    except Exception as exc:  # noqa: BLE001 - reported, not hidden
        return {"error": "worker import failed: %s: %s" % (type(exc).__name__, str(exc)[:200])}
    item = plan["exports"][0]
    values = {"job_id": item["batch_id"], "tenant_id": plan["tenant"], "user_id": "case-loader", "collection_id": plan["case_id"], "file_id": item["rel"],
              "source_file": os.path.basename(item["rel"]), "record_type": "auto", "metadata": {"source_timezone": "Asia/Karachi"},
              "evidence_id": item["evidence_id"], "request_id": "psycopg-probe"}
    wanted = {f.name: values.get(f.name, "") for f in dataclasses.fields(worker.IngestJob) if f.default is dataclasses.MISSING or f.name in values}
    job = worker.IngestJob(**wanted)
    os.environ["FORENSIC_INGEST_V2"] = "shadow"
    try:
        with conn.cursor() as cur:
            worker.run_typed_ingest_shadow(conn, cur, job, Path(plan["case_dir"]) / item["rel"], {"classification": {"time_policy": TIME_POLICY}})
            cur.execute("SELECT action, details->'typed_ingest'->>'status', details->'typed_ingest'->>'rows_inserted', details->'typed_ingest'->>'rows_loaded' "
                        "FROM forensic.records_audit_log WHERE action = 'records.ingest.typed_shadow' AND file_id = %s", (item["rel"],))
            rows = [list(r) for r in cur.fetchall()]
        return {"audit_rows": rows}
    except Exception as exc:  # noqa: BLE001
        return {"error": "hook failed: %s: %s" % (type(exc).__name__, traceback.format_exc()[-400:])}


main()
