#!/usr/bin/env python3
"""Runs the Ask engine's executor specs, or an evaluation of the engine (and the governed SQL lane) with the real local model, on a generated case in a THROWAWAY database.

    python -X utf8 tools/case_generator/check_ask_engine.py                       # the executor specs, scripted model, about five minutes
    python -X utf8 tools/case_generator/check_ask_engine.py --questions tools/ask_eval/questions.json --out RESULTS.jsonl --arms engine,lane --model-url http://127.0.0.1:8080

It starts a throwaway PostgreSQL (local image only, random password, published on 127.0.0.1 only because the Go side must connect to it), applies migrations 001 to 015,
the runtime-role grants, the 13 layouts and then 016, 017 and 018, writes a GENERATED case through the PRODUCTION typed ingest (no real data anywhere), sets a password on the
application role `forensic_runtime` and a marker table (the Go side refuses a database without it), and runs the Ginkgo specs in api/forensic_records AS THE APPLICATION ROLE.
The planted-truth keys of the generator (the independent oracle) are written to a temporary file and handed to the Go side. The container and its volume are removed at the end.

Specs mode: "Ask engine executor on a case database": the executor's walls (the role, the scope, the cost ceiling, the row cap, the timeout, the case clock), the probe, and the
whole run with a scripted model, against the keys. Evaluation mode: the questions of --questions are put to each arm (the Ask engine with the real model; the governed SQL lane)
and every result is written as one JSON line; tools/ask_eval/judge.py scores them against the keys.
"""
import argparse
import json
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

SPEC_FOCUS = "Ask engine executor on a case database"
EXPECTED_SPECS = 15
PARAMS = dict(seed=61, people=60, subjects=6, associates=12, period_days=20, p2_pairs=3, p3_pairs=2, p7_plants=3)


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def build_case_database(db, root, registry, port):
    db.start()
    load_case.prepare(db, registry)
    key, _stats, _summaries = ingest_case(db, root, registry, PsqlSink(db))
    for name in ("016_effective_records.sql", "018_effective_ipdr.sql", "017_analyst_views.sql"):
        db.psql("-f", "/tmp/mig/" + name)
    runtime_password = uuid.uuid4().hex
    db.psql("-c", "ALTER ROLE forensic_runtime PASSWORD '%s'" % runtime_password)
    db.psql("-c", "CREATE TABLE public.nx_throwaway_marker (id int)")
    return key, "postgres://forensic_runtime:%s@127.0.0.1:%d/localrecall?sslmode=disable" % (runtime_password, port)


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--questions", help="evaluation mode: a JSON list of {id, question, ...}")
    parser.add_argument("--out", help="evaluation mode: where the JSON lines are written")
    parser.add_argument("--arms", default="engine", help="evaluation mode: engine, lane or engine,lane")
    parser.add_argument("--model-url", default="http://127.0.0.1:8080")
    parser.add_argument("--model", default="qwen3-4b-instruct-2507-q4km-nxb21d-dev")
    parser.add_argument("--keep-truth", help="also keep the planted-truth keys in this file")
    parser.add_argument("--seed", type=int, default=PARAMS["seed"])
    parser.add_argument("--ids", help="evaluation mode: only these question ids, comma separated")
    args = parser.parse_args(argv)
    evaluation = bool(args.questions)

    params = dict(PARAMS, seed=args.seed)
    tmp = tempfile.TemporaryDirectory()
    registry = load_registry()
    port = free_port()
    db = load_case.Throwaway(publish_port=port)
    try:
        root = generate.write_case(w.Params(**params), tmp.name)
        key, dsn = build_case_database(db, root, registry, port)
        truth_path = args.keep_truth or os.path.join(tmp.name, "truth.json")  # the evaluation judge reads the keys the questions file carries, not this file
        with open(truth_path, "w", encoding="utf-8") as handle:
            json.dump({"case_id": key["case_id"], "tenant": load_case.TENANT, "subjects": key["subjects"], "oracle": key["oracle"]}, handle)
        env = dict(os.environ, ASK_ENGINE_CASE_DSN=dsn, ASK_ENGINE_CASE_TENANT=load_case.TENANT, ASK_ENGINE_CASE_COLLECTION=key["case_id"], ASK_ENGINE_CASE_TRUTH=truth_path)
        if evaluation:
            env.update(ASK_EVAL_QUESTIONS=os.path.abspath(args.questions), ASK_EVAL_OUT=os.path.abspath(args.out), ASK_EVAL_ARMS=args.arms,
                       ASK_EVAL_MODEL_URL=args.model_url, ASK_EVAL_MODEL=args.model, FORENSIC_TYPED_FAMILIES="communications_cdr,network_ipdr", FORENSIC_GOVERNED_SQL="true",
                       FORENSIC_ASK_V2_HUB_THRESHOLD=str(key["oracle"]["parameters"]["hub_threshold"]))
            if args.ids:
                env["ASK_EVAL_IDS"] = args.ids
            focus = "Ask engine evaluation"
        else:
            focus = SPEC_FOCUS
        proc = subprocess.run(["go", "test", "./api/forensic_records/", "-count=1", "-timeout", "6h", "-v", "-ginkgo.no-color", "-ginkgo.focus", focus],
                              cwd=load_case.REPO, env=env, capture_output=True, text=True)
        output = proc.stdout + proc.stderr
        log = os.environ.get("ASK_CHECK_LOG")
        if log:
            with open(log, "w", encoding="utf-8") as handle:
                handle.write(output)
        summary = [line for line in output.splitlines() if re.search(r"Ran \d+ of|Passed|\[FAIL\]|^ok|^FAIL|panic|Expected|to equal|to contain|\[FAILED\]", line)]
        print("\n".join(summary[:40]))
        ran = re.search(r"Ran (\d+) of", output)
        if evaluation:
            ok = proc.returncode == 0 and ran is not None
            print("ASKEVAL=%s (specs run: %s; results in %s)" % ("DONE" if ok else "FAIL", ran.group(1) if ran else "none", args.out))
        else:
            ok = proc.returncode == 0 and ran is not None and int(ran.group(1)) == EXPECTED_SPECS
            print("ASKENGINE=%s (specs run: %s, expected %d)" % ("PASS" if ok else "FAIL", ran.group(1) if ran else "none", EXPECTED_SPECS))
        if not ok:
            print("the full Go output is kept when ASK_CHECK_LOG names a file")
        return 0 if ok else 1
    finally:
        left = db.stop()
        tmp.cleanup()
        if left:
            print("WARNING: " + left)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
