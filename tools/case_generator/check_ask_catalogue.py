#!/usr/bin/env python3
"""Runs the Ask engine's catalogue and example specs against the analyst views (migration 017) on a throwaway database.

    python -X utf8 tools/case_generator/check_ask_catalogue.py [--write-golden]

It starts a throwaway PostgreSQL (local image only, random password, published on 127.0.0.1 only because the Go specs must connect to it), applies migrations 001 to 015 and
the runtime-role grants and then 017, sets a password on the application role `forensic_runtime` and a marker table (the specs refuse a database without it), and runs the Ginkgo
specs "Ask engine on a real database" in api/forensic_records AS THE APPLICATION ROLE. The specs build the catalogue from the database's own comments, compare it with the pinned
snapshot api/forensic_records/ask_engine_catalogue.golden.json, and run every verified example as the read-only analyst role. With --write-golden the snapshot is rewritten
from the database instead of compared. The container and its volume are removed at the end. Exit code 0 only when the specs ran (not skipped) and passed.
"""
import os
import re
import socket
import subprocess
import sys
import uuid

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import load_case  # noqa: E402
from ingestion.forensic_records.layout_registry import load_registry  # noqa: E402

SPEC_FOCUS = "Ask engine on a real database"
EXPECTED_SPECS = 4
GOLDEN = os.path.join(load_case.REPO, "api", "forensic_records", "ask_engine_catalogue.golden.json")


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def main(argv):
    write_golden = "--write-golden" in argv
    registry = load_registry()
    port = free_port()
    db = load_case.Throwaway(publish_port=port)
    try:
        db.start()
        load_case.prepare(db, registry)
        db.psql("-f", "/tmp/mig/017_analyst_views.sql")
        runtime_password = uuid.uuid4().hex
        db.psql("-c", "ALTER ROLE forensic_runtime PASSWORD '%s'" % runtime_password)
        db.psql("-c", "CREATE TABLE public.nx_throwaway_marker (id int)")
        env = dict(os.environ, ASK_ENGINE_TEST_DSN="postgres://forensic_runtime:%s@127.0.0.1:%d/localrecall?sslmode=disable" % (runtime_password, port))
        if write_golden:
            env["ASK_ENGINE_WRITE_GOLDEN"] = GOLDEN
        proc = subprocess.run(["go", "test", "./api/forensic_records/", "-count=1", "-v", "-ginkgo.no-color", "-ginkgo.focus", SPEC_FOCUS],
                              cwd=load_case.REPO, env=env, capture_output=True, text=True)
        output = proc.stdout + proc.stderr
        summary = [line for line in output.splitlines() if re.search(r"Ran \d+ of|Passed|\[FAIL\]|^ok|^FAIL|panic|Expected|to equal|regenerate", line)]
        print("\n".join(summary[:30]))
        ran = re.search(r"Ran (\d+) of", output)
        ok = proc.returncode == 0 and ran is not None and int(ran.group(1)) == EXPECTED_SPECS
        log = os.environ.get("ASK_CHECK_LOG")
        if log:
            with open(log, "w", encoding="utf-8") as handle:
                handle.write(output)
        if not ok:
            print("the full Go output is kept when ASK_CHECK_LOG names a file")
        print("ASKCATALOGUE=%s (specs run: %s, expected %d%s)" % ("PASS" if ok else "FAIL", ran.group(1) if ran else "none", EXPECTED_SPECS, ", golden written" if write_golden and ok else ""))
        return 0 if ok else 1
    finally:
        left = db.stop()
        if left:
            print("WARNING: " + left)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
