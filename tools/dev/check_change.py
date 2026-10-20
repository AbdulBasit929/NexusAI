#!/usr/bin/env python3
"""One command for the test tiers of docs/roadmap/reconciliation-20261020.md, section 6, so that a change never waits an hour.

    python -X utf8 tools/dev/check_change.py                       # T0 for what changed (git diff against HEAD and untracked files), nothing live
    python -X utf8 tools/dev/check_change.py --typing nexusai-forensic-demo nexusai-multimodal-product-acceptance     # + T1 typing report per collection
    python -X utf8 tools/dev/check_change.py --smoke               # + T2 live smoke (about 5 minutes; --quick about 3; --real adds the six checks on the real collection)
    python -X utf8 tools/dev/check_change.py --db-specs            # + the live-database specs on a throwaway Postgres (about 2 minutes), when the executor, the coverage or the audit changed

Tiers: T0 unit and live-DB specs of the paths that changed; T1 typing report (seconds, no model); T2 live smoke; T3 is by hand (re-ask only what failed: `factory.py run ... --resume`, or
`factory.py reeffective ARM ...` to re-judge saved answers on a typed case with no model); T4 (full banks and spreads) is a milestone run, in the background, never from here.

It only runs what the change can touch: a Go change under api/forensic_records runs the Ask engine specs, a change under the layout registry or the ingestion tests runs the registry and typed-ingest
tests, a change under tools/ask_eval runs the judge tests, a change under evaluation/question_factory runs the factory tests. Exit status 1 if any step fails; a table of steps and times at the end.
"""
import argparse
import os
import subprocess
import sys
import time

REPO = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
PY = [sys.executable, "-X", "utf8"]


def changed_files():
    out = subprocess.run(["git", "status", "--porcelain"], cwd=REPO, capture_output=True, text=True).stdout
    return [line[3:].strip().strip('"').replace("\\", "/") for line in out.splitlines() if line[3:].strip()]


def run(name, command, results, timeout=1800):
    started = time.time()
    print("\n== %s\n   %s" % (name, " ".join(command)), flush=True)
    try:
        code = subprocess.run(command, cwd=REPO, timeout=timeout).returncode
    except subprocess.TimeoutExpired:
        code = 124
    results.append((name, code == 0, time.time() - started))
    return code == 0


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--typing", nargs="*", default=[], metavar="COLLECTION", help="run the typing report for each collection (T1)")
    parser.add_argument("--smoke", action="store_true", help="the live smoke (T2)")
    parser.add_argument("--quick", action="store_true", help="smoke without the two connection questions")
    parser.add_argument("--real", action="store_true", help="smoke with the six checks on the private real collection (counts only)")
    parser.add_argument("--db-specs", action="store_true", help="the live-database specs on a throwaway Postgres")
    parser.add_argument("--all-t0", action="store_true", help="run every T0 step whatever changed")
    args = parser.parse_args(argv)
    files = changed_files()
    touched = lambda *prefixes: args.all_t0 or any(f.startswith(prefixes) for f in files)  # noqa: E731
    results = []
    print("changed: %d file(s)%s" % (len(files), "" if files else " (nothing: only the T1 and T2 steps you asked for run)"))

    if touched("api/forensic_records/"):
        run("T0 Ask engine specs (Go)", ["go", "test", "./api/forensic_records/", "-count=1", "-timeout", "20m", "-ginkgo.focus", "Ask engine"], results)
    if touched("ingestion/forensic_records/layout_registry/", "ingestion/forensic_records/tests/test_layout_registry.py", "tools/real_data/make_golden.py"):
        run("T0 layout registry tests", PY + ["-m", "unittest", "ingestion.forensic_records.tests.test_layout_registry"], results)
    if touched("ingestion/forensic_records/typed_ingest.py", "ingestion/forensic_records/layout_registry/", "ingestion/forensic_records/tests/test_typed_ingest.py"):
        run("T0 typed ingest tests", PY + ["-m", "unittest", "ingestion.forensic_records.tests.test_typed_ingest"], results)
    if touched("tools/ask_eval/"):
        run("T0 judge tests", PY + ["tools/ask_eval/test_judge_cells.py"], results)
    if touched("evaluation/question_factory/"):
        run("T0 factory tests", PY + ["evaluation/question_factory/test_factory_keys.py"], results)
    if touched("tools/real_data/"):
        run("T0 real-data tool tests", PY + ["tools/real_data/test_real_data_tools.py"], results)
    if args.db_specs:
        run("T0 live-database specs (throwaway Postgres)", PY + ["tools/case_generator/check_ask_engine.py"], results)
    for collection in args.typing:
        run("T1 typing report: " + collection, PY + ["tools/dev/typing_report.py", collection], results, timeout=300)
    if args.smoke or args.quick or args.real:
        command = PY + ["tools/dev/live_smoke.py"] + (["--quick"] if args.quick else []) + (["--real"] if args.real else [])
        run("T2 live smoke", command, results, timeout=3600)

    print("\n== summary")
    for name, ok, seconds in results:
        print("  %-52s %-5s %6.0f s" % (name, "OK" if ok else "FAIL", seconds))
    if not results:
        print("  nothing to run: no change under a tested path; ask for --typing, --smoke or --db-specs")
    return 0 if all(ok for _, ok, _ in results) else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
