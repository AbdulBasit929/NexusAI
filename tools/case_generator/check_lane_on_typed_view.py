#!/usr/bin/env python3
"""Runs the question lane's OWN Go code against the effective call-record view on a throwaway database.

    python -X utf8 tools/case_generator/check_lane_on_typed_view.py [CASE_DIR]

It starts a throwaway PostgreSQL (local image only, random password, published on 127.0.0.1 only because the Go specs must connect to it), applies migrations 001 to 016
and the runtime-role grants, writes a generated case through the PRODUCTION typed ingest, sets a password on the application role `forensic_runtime` and a marker table (the
specs refuse a database without it), and runs the Ginkgo specs "Typed families on a real database" in api/forensic_records with FORENSIC_TYPED_FAMILIES switched on, connecting
AS THE APPLICATION ROLE (row-level security applies). The specs build the lane's view and CTE with the real builders and compare what they read with the typed table.
Without CASE_DIR a small case is generated. The container and its volume are removed at the end. Exit code 0 only when the specs ran (not skipped) and passed.
"""
import os
import re
import socket
import subprocess
import sys
import tempfile
import uuid

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import generate  # noqa: E402
import load_case  # noqa: E402
import world as w  # noqa: E402
from ingestion.forensic_records.layout_registry import load_registry  # noqa: E402
from test_ingest_v2 import PsqlSink, ingest_case  # noqa: E402

SPEC_FOCUS = "Typed families on a real database"
EXPECTED_SPECS = 5


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def main(argv):
    registry = load_registry()
    tmp = tempfile.TemporaryDirectory()
    try:
        if argv:
            case_dir = argv[0]
        else:
            params = w.Params(seed=61, people=60, subjects=6, associates=12, period_days=20, p2_pairs=3, p3_pairs=2, p7_plants=3)
            case_dir = generate.write_case(params, tmp.name)
        port = free_port()
        db = load_case.Throwaway(publish_port=port)
        try:
            db.start()
            load_case.prepare(db, registry)
            key, stats, summaries = ingest_case(db, case_dir, registry, PsqlSink(db))
            runtime_password = uuid.uuid4().hex
            db.psql("-c", "ALTER ROLE forensic_runtime PASSWORD '%s'" % runtime_password)
            db.psql("-c", "CREATE TABLE public.nx_throwaway_marker (id int)")
            subject = next(s for s in key["subjects"] if s["layout_id"].startswith("call-c"))
            env = dict(os.environ, TYPED_FAMILIES_TEST_DSN="postgres://forensic_runtime:%s@127.0.0.1:%d/localrecall?sslmode=disable" % (runtime_password, port),
                       TYPED_FAMILIES_TEST_COLLECTION=key["case_id"], TYPED_FAMILIES_TEST_SUBJECT=subject["msisdn"])
            proc = subprocess.run(["go", "test", "./api/forensic_records/", "-count=1", "-v", "-ginkgo.no-color", "-ginkgo.focus", SPEC_FOCUS],
                                  cwd=load_case.REPO, env=env, capture_output=True, text=True)
            output = proc.stdout + proc.stderr
            summary = [line for line in output.splitlines() if re.search(r"Ran \d+ of|Passed|\[FAIL\]|^ok|^FAIL|panic", line)]
            print("\n".join(summary[:30]))
            ran = re.search(r"Ran (\d+) of", output)
            ok = proc.returncode == 0 and ran is not None and int(ran.group(1)) == EXPECTED_SPECS
            log = os.environ.get("LANE_CHECK_LOG")
            if log:
                with open(log, "w", encoding="utf-8") as handle:
                    handle.write(output)
            if not ok:
                print("the full Go output is kept when LANE_CHECK_LOG names a file")
            print("LANEONVIEW=%s (specs run: %s, expected %d)" % ("PASS" if ok else "FAIL", ran.group(1) if ran else "none", EXPECTED_SPECS))
            return 0 if ok else 1
        finally:
            left = db.stop()
            if left:
                print("WARNING: " + left)
    finally:
        tmp.cleanup()


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
