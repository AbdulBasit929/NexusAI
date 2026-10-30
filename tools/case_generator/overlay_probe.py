"""Runs INSIDE the worker image (see check_worker_overlay.py): the worker's own `process_job` over every file of a generated case, on a throwaway database.

The image's own code in /app is used (never a mounted copy): this is the proof of the image, with its own Python, its own psycopg, its own poppler and its own flat module layout. The case is mounted
read-only as the spool, a plan file lists the jobs (as the API would queue them: the evidence is already registered, the metadata carries the route and the time zone). The worker connects as the
owner role of the throwaway, as the live worker does. One JSON document on stdout, nothing else; nothing here reads a real file or reaches a network.
"""
import json
import os
import sys
import traceback

sys.path.insert(0, "/app")
os.chdir("/app")

import worker  # noqa: E402  (the image's own worker.py; FORENSIC_DATABASE_URL, FORENSIC_SPOOL_DIR and the switches come from the environment)


def main():
    with open(os.environ["PLAN"], encoding="utf-8") as handle:
        plan = json.load(handle)
    out = {"switches": {name: os.environ.get(name) for name in ("FORENSIC_INGEST_V2", "FORENSIC_DOCUMENT_MENTIONS", "FORENSIC_DOCUMENT_GAZETTEER")},
           "hook_present": hasattr(worker, "run_document_mentions_shadow"), "jobs": []}
    for item in plan["jobs"]:
        job = worker.IngestJob(job_id=item["job_id"], tenant_id=plan["tenant"], user_id="overlay-probe", collection_id=plan["case_id"], file_id=item["rel"], source_file=item["name"],
                               spool_path=os.path.join(plan["spool"], *item["rel"].split("/")), sha256=item["sha256"], record_type="generic", headers=[], metadata=dict(item["metadata"]),
                               request_id="overlay-probe", evidence_id=item["evidence_id"])
        try:
            worker.process_job(job)
            out["jobs"].append({"name": item["name"], "ok": True})
        except Exception as exc:  # noqa: BLE001 - reported, not hidden
            out["jobs"].append({"name": item["name"], "ok": False, "error": "%s: %s" % (type(exc).__name__, str(exc)[:300]), "trace": traceback.format_exc()[-600:]})
    print(json.dumps(out, sort_keys=True))


if __name__ == "__main__":
    main()
