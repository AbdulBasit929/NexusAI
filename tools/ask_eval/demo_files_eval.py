#!/usr/bin/env python3
"""Evaluates the Ask engine on the project's own SYNTHETIC demonstration call-record files (not the generator's case), with the answers computed by plain Python from the files.

    python -X utf8 tools/ask_eval/demo_files_eval.py --files-dir DIR [--out RESULTS.jsonl] [--arms engine] [--model-url http://127.0.0.1:8080]

The generated cases of check_ask_engine.py come from one generator, so a result on them says the engine is accurate on that generator's data. These files are another family of
data: the demonstration cases' CDR exports (the company-style layout read as the Telenor-style layout, several subscribers in one file, data sessions with no counterparty,
short codes and a sender name among the counterparties, exact repeated rows). The files themselves are not in the repository (copy them out of the spool, see
docs/data/ask-engine-v2.md); this script reads the CSV files of DIR whose header is the company style (MSISDN, call_org_num, CALL_DIALED_NUM, INBOUND_OUTBOUND_IND, ...).

The oracle is deliberately simple and does not look at the database: the subscriber is MSISDN; the counterparty is the dialed number of an OUTGOING row and the originating number
of an INCOMING row (a DATA row has none); an exact repeat of a row is left out; a counterparty that is not a 12-digit 92 number is a service (a short code or a sender name).
It writes the same question kinds as build_questions.py and is scored by judge.py.
"""
import argparse
import collections
import csv
import glob
import json
import os
import re
import subprocess
import sys
import tempfile
import uuid

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
import load_case  # noqa: E402
from ingestion.forensic_records import typed_ingest as ti  # noqa: E402
from ingestion.forensic_records.layout_registry import Policy, load_registry  # noqa: E402
from test_ingest_v2 import PsqlSink  # noqa: E402
from check_ask_engine import free_port  # noqa: E402

COLLECTION = "demo-files-eval"
POLICY = Policy(source_tz="Asia/Karachi", tz_basis="assumed_by_profile")
HEADER = {"MSISDN", "call_org_num", "CALL_DIALED_NUM", "INBOUND_OUTBOUND_IND"}


def digits(value):
    return re.sub(r"\D", "", value or "")


def person_like(number):
    return len(number) == 12 and number.startswith("92")


def read_files(files_dir):
    rows = []
    for path in sorted(glob.glob(os.path.join(files_dir, "*.csv"))):
        with open(path, encoding="utf-8", newline="") as handle:
            reader = csv.DictReader(handle)
            if not HEADER <= set(reader.fieldnames or []):
                continue
            seen = set()
            for r in reader:
                key = tuple(r.values())
                if key in seen:
                    continue   # an exact repeat of a row of the same file
                seen.add(key)
                rows.append(r)
    return rows


def oracle(rows):
    pair = collections.defaultdict(lambda: [0, 0, 0])   # (subscriber, counterparty) -> events, incoming, outgoing
    for r in rows:
        a = digits(r["MSISDN"])
        direction = (r["INBOUND_OUTBOUND_IND"] or "").strip().upper()
        if direction == "OUTGOING":
            b = digits(r["CALL_DIALED_NUM"]) or (r["CALL_DIALED_NUM"] or "").strip()
        elif direction == "INCOMING":
            b = digits(r["call_org_num"]) or (r["call_org_num"] or "").strip()
        else:
            continue
        if not b or b == a:
            continue
        p = pair[(a, b)]
        p[0] += 1
        p[1] += direction == "INCOMING"
        p[2] += direction == "OUTGOING"
    return pair


def build_questions(files_dir):
    rows = read_files(files_dir)
    pair = oracle(rows)
    subscribers = sorted({a for a, _ in pair}, key=lambda a: -sum(v[0] for (x, _), v in pair.items() if x == a))
    reached = collections.Counter(b for (_, b) in pair)
    assert max(reached.values()) <= 20, "a hub would change what 'everyone contacts' means; the oracle does not model it"
    questions = []

    def add(kind, text, expect, **extra):
        questions.append(dict(id="D%03d" % (len(questions) + 1), type=kind, question=text, expect=expect, **extra))

    for i, s in enumerate(subscribers[:6]):
        mine = {b: v for (a, b), v in pair.items() if a == s and person_like(b)}
        if not mine:
            continue
        best = sorted(mine.items(), key=lambda kv: (-kv[1][0], kv[0]))
        b, v = best[0]
        add("pair_count", ("How many events are there between %s and %s?" if i % 2 == 0 else "How many calls and messages did %s have with %s?") % (s, b), {"kind": "scalar", "value": v[0]})
        inc = [(b2, v2) for b2, v2 in best if v2[1] >= 2]
        if inc:
            b2, v2 = inc[0]
            add("incoming_count", ("How many incoming events did %s receive from %s?" if i % 2 == 0 else "How many times did %s get a call or message from %s?") % (s, b2), {"kind": "scalar", "value": v2[1]})
        if len(best) > 3 and best[2][1][0] != best[3][1][0]:
            add("top_contacts", ("Who does %s talk to most? Give the top three, leaving out services and numbers that everyone contacts." if i % 2 == 0
                                 else "List the three numbers %s has the most events with, not counting short codes or numbers that everybody contacts.") % s,
                {"kind": "ordered", "values": [b3 for b3, _ in best[:3]], "scores": {b3: v3[0] for b3, v3 in best}})
        add("distinct_contacts", ("How many different numbers did %s contact, leaving out services?" if i % 2 == 0 else "How many distinct phone numbers has %s been in contact with, excluding short codes?") % s,
            {"kind": "scalar", "value": len(mine)})
    for i, n in enumerate(["923000099998", "923000099997"]):
        add("absent", ("How many calls does %s have?" if i % 2 == 0 else "How many events involve the number %s?") % n, {"kind": "absent"})
    add("none", "What is the home address of %s?" % subscribers[0], {"kind": "none"})
    return questions


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--files-dir", required=True)
    parser.add_argument("--out", default=os.path.join(tempfile.gettempdir(), "demo-files-eval.jsonl"))
    parser.add_argument("--arms", default="engine")
    parser.add_argument("--only-recognised", action="store_true", help="leave out of the case the files the registry does not read (the guard declines a case that has one)")
    parser.add_argument("--model-url", default="http://127.0.0.1:8080")
    parser.add_argument("--model", default="qwen3-4b-instruct-2507-q4km-nxb21d-dev")
    parser.add_argument("--questions-out", default=os.path.join(tempfile.gettempdir(), "demo-files-questions.json"))
    args = parser.parse_args(argv)
    questions = build_questions(args.files_dir)
    with open(args.questions_out, "w", encoding="utf-8") as handle:
        json.dump(questions, handle, indent=1)
    print("%d questions from the files: %s" % (len(questions), dict(collections.Counter(q["type"] for q in questions))))
    if os.path.exists(args.out):
        os.remove(args.out)

    registry = load_registry()
    port = free_port()
    db = load_case.Throwaway(publish_port=port)
    try:
        db.start()
        load_case.prepare(db, registry)
        sink = PsqlSink(db)
        loaded = 0
        for path in sorted(glob.glob(os.path.join(args.files_dir, "*.csv")) + glob.glob(os.path.join(args.files_dir, "*.tsv"))):
            evidence, batch, sha = str(uuid.uuid4()), str(uuid.uuid4()), uuid.uuid4().hex * 2
            name = os.path.basename(path)
            db.psql("-c", "INSERT INTO forensic.evidence_items (evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension, size_bytes, sha256, "
                          "modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata) VALUES ('%s', '%s', '%s', 'DEMO-FILES', 'u', '%s', '%s', 'text/csv', 'csv', 1, '%s', "
                          "'structured_records', 'cdr', 1, 'w', 'completed', 'immutable://demo/%s', jsonb_build_object('request_id', 'r'))" % (evidence, load_case.TENANT, COLLECTION, name, name, sha, evidence))
            summary = ti.ingest_file(path, ti.Lineage(load_case.TENANT, COLLECTION, evidence, name, batch), POLICY, sink, registry=registry)
            sheets = summary.get("sheets", [])
            print("  file %-2d %-13s %s" % (loaded + 1, summary.get("status"), [(s.get("layout_id"), s.get("loaded")) for s in sheets][:1]))
            loaded += 1
            if summary.get("status") != "loaded" and args.only_recognised:
                # the case under test holds only the files the registry reads: the coverage guard (rightly) declines a case with a file that has no typed rows
                db.psql("-c", "UPDATE forensic.evidence_items SET detected_type = 'generic' WHERE evidence_id = '%s'" % evidence)
        for name in ("016_effective_records.sql", "018_effective_ipdr.sql", "017_analyst_views.sql", "019_observed_cell_positions.sql"):
            db.psql("-f", "/tmp/mig/" + name)
        password = uuid.uuid4().hex
        db.psql("-c", "ALTER ROLE forensic_runtime PASSWORD '%s'" % password)
        db.psql("-c", "CREATE TABLE public.nx_throwaway_marker (id int)")
        env = dict(os.environ, ASK_ENGINE_CASE_DSN="postgres://forensic_runtime:%s@127.0.0.1:%d/localrecall?sslmode=disable" % (password, port), ASK_ENGINE_CASE_TENANT=load_case.TENANT,
                   ASK_ENGINE_CASE_COLLECTION=COLLECTION, ASK_EVAL_QUESTIONS=os.path.abspath(args.questions_out), ASK_EVAL_OUT=os.path.abspath(args.out), ASK_EVAL_ARMS=args.arms,
                   ASK_EVAL_MODEL_URL=args.model_url, ASK_EVAL_MODEL=args.model, FORENSIC_TYPED_FAMILIES="communications_cdr,network_ipdr", FORENSIC_GOVERNED_SQL="true")
        proc = subprocess.run(["go", "test", "./api/forensic_records/", "-count=1", "-timeout", "6h", "-v", "-ginkgo.no-color", "-ginkgo.focus", "Ask engine evaluation"],
                              cwd=load_case.REPO, env=env, capture_output=True, text=True)
        output = proc.stdout + proc.stderr
        print("\n".join(line for line in output.splitlines() if re.search(r"Ran \d+ of|Passed|\[FAIL\]|^ok|^FAIL|panic|Expected|\[FAILED\]", line))[:3000])
        print("DEMOFILES=%s results in %s, questions in %s" % ("DONE" if proc.returncode == 0 else "FAIL", args.out, args.questions_out))
        return proc.returncode
    finally:
        left = db.stop()
        if left:
            print("WARNING: " + left)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
