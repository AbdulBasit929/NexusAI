#!/usr/bin/env python3
"""Live smoke of the running stack (tier T2 of docs/roadmap/reconciliation-20261020.md): about 8 questions and 3 reads, 5 to 6 minutes on the laptop, run after every deploy.

    python -X utf8 tools/dev/live_smoke.py                 # the synthetic checks only
    python -X utf8 tools/dev/live_smoke.py --real          # plus the six checks on the private real collection (prints counts, kinds and booleans only)
    python -X utf8 tools/dev/live_smoke.py --quick         # skip the two slowest questions (the connection questions), about 2 minutes

It goes through the workspace's dev proxy (default http://127.0.0.1:4181/api, override with --base) and the Postgres container, as the earlier hand-run checks did. Synthetic collection only
unless --real: `ingest-v2-shadow-synthetic-20261013`, which holds shared contacts, handsets stored in 14 and in 15 digits, and cell positions. Exit status 1 if any check fails.

What it checks (each one is a regression that was once real): the shared contact is named in a connection answer (decision D22); a handset stored as 14 digits is found when asked as 15; a
template request writes no engine audit row; a cell answer says how many cell sites; the engine-health read returns its contract and counts only real questions; (--real) the six answers
on the real collection equal SQL on the typed table.
"""
import argparse
import json
import re
import subprocess
import sys
import time
import urllib.request

COLLECTION = "ingest-v2-shadow-synthetic-20261013"
POSTGRES = "nexusai-forensic-postgres-1"


def headers(collection):
    return {"Content-Type": "application/json", "X-Forensic-Tenant-ID": "default", "X-Forensic-Collection-ID": collection, "X-Forensic-Actor-ID": "investigation-workspace",
            "X-Forensic-Subject-ID": "investigation-workspace", "X-Forensic-Actor-Role": "user"}


class Stack:
    def __init__(self, base):
        self.base = base.rstrip("/")

    def call(self, path, body=None, collection=COLLECTION):
        request = urllib.request.Request(self.base + path, data=json.dumps(body).encode() if body is not None else None, method="POST" if body is not None else "GET", headers=headers(collection))
        with urllib.request.urlopen(request, timeout=900) as response:
            return response.status, json.loads(response.read().decode("utf-8"))

    def ask(self, question, collection=COLLECTION, **extra):
        started = time.time()
        _, data = self.call("/query/hybrid", dict({"tenant_id": "default", "collection_id": collection, "query": question}, **extra), collection)
        return data, time.time() - started


def psql(sql):
    out = subprocess.run(["docker", "exec", POSTGRES, "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-F", "|", "-c", sql], capture_output=True, text=True)
    if out.returncode:
        raise SystemExit("psql failed: " + out.stderr.strip()[:200])
    return [line.split("|") for line in out.stdout.strip().splitlines() if line]


def luhn15(imei14):
    total = 0
    for i, ch in enumerate(reversed(imei14)):
        d = int(ch)
        if i % 2 == 0:
            d = d * 2 - 9 if d * 2 > 9 else d * 2
        total += d
    return imei14 + str((10 - total % 10) % 10)


def numbers_of(data):
    grid = (data.get("enterprise") or {}).get("data_grid") or {}
    return {str(v) for r in (grid.get("rows") or []) for v in r.values() if isinstance(v, str) and v.isdigit() and len(v) >= 11}


def answer_of(data):
    return str((data.get("answer") or {}).get("answer", ""))


def short(text):
    return re.sub(r"92300000\d{4}", lambda m: m.group(0)[-5:], text)[:90]


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--base", default="http://127.0.0.1:4181/api")
    parser.add_argument("--real", action="store_true")
    parser.add_argument("--quick", action="store_true")
    args = parser.parse_args(argv)
    stack = Stack(args.base)
    results = []

    def record(name, ok, detail, seconds=None):
        results.append(ok)
        print("%-34s %-5s %s%s" % (name, "OK" if ok else "FAIL", ("%6.1fs " % seconds) if seconds is not None else "        ", detail))

    started = time.time()
    # 1. engine health: the contract, and only real questions are counted
    _, health = stack.call("/analysis/engine-health?collection_id=%s&days=30" % COLLECTION)
    totals = health.get("totals", {})
    record("engine-health read", health.get("contract") == "forensics.engine-health/v1" and totals.get("questions", 0) >= totals.get("answered", 0) + totals.get("declined", 0) - 1,
           "questions %s, answered %s, declined %s" % (totals.get("questions"), totals.get("answered"), totals.get("declined")))

    # 2. a template request writes no engine audit row
    _, templates = stack.call("/query/templates")
    names = [t.get("name") or t.get("id") for t in (templates.get("templates") if isinstance(templates, dict) else templates)]
    before = int(psql("SELECT count(*) FROM forensic.records_audit_log WHERE action = 'ask_engine_v2.question'")[0][0])
    stack.ask("Summarise the case", template=names[0])
    after = int(psql("SELECT count(*) FROM forensic.records_audit_log WHERE action = 'ask_engine_v2.question'")[0][0])
    record("template request, no audit row", before == after, "audit rows %d -> %d" % (before, after))

    # 3. IMEI: a handset stored as 14 digits, asked as 15 (the original failure), and one stored as 15 (control)
    base = "collection_id='%s' AND dataset_kind='target_cdr' AND NOT ('duplicate_row' = ANY (quality_flags))" % COLLECTION
    only14 = psql("SELECT imei FROM forensic.comm_event WHERE %s AND length(imei)=14 AND imei NOT IN (SELECT left(imei,14) FROM forensic.comm_event WHERE %s AND length(imei)=15) GROUP BY 1 ORDER BY count(*) DESC LIMIT 1" % (base, base))
    as15 = psql("SELECT imei FROM forensic.comm_event WHERE %s AND length(imei)=15 GROUP BY 1 ORDER BY count(*) DESC LIMIT 1" % base)
    for name, stored, asked in (("IMEI stored 14, asked 15", only14[0][0] if only14 else None, luhn15(only14[0][0]) if only14 else None), ("IMEI stored 15, asked 15", as15[0][0] if as15 else None, as15[0][0] if as15 else None)):
        if stored is None:
            record(name, False, "the synthetic collection holds no such handset")
            continue
        truth = {r[0] for r in psql("SELECT a_norm FROM forensic.comm_event WHERE %s AND left(imei,14)='%s' GROUP BY 1" % (base, stored[:14]))}
        data, seconds = stack.ask("Which numbers were used with the handset with IMEI %s?" % asked)
        record(name, bool(truth) and numbers_of(data) == truth and "no record" not in answer_of(data).lower(), "truth %d numbers, got %d" % (len(truth), len(numbers_of(data))), seconds)

    # 4. the cell answer says how many cell sites
    subject = psql("SELECT a_norm FROM forensic.comm_event WHERE %s AND cell_key IS NOT NULL GROUP BY 1 ORDER BY count(*) DESC LIMIT 1" % base)[0][0]
    data, seconds = stack.ask("Which cell sites has %s used?" % subject)
    record("cell-site headline", "cell site" in answer_of(data), short(answer_of(data)), seconds)

    # 5. connection questions (D22): the shared contact is named; "direct contact" is said to be direct
    if not args.quick:
        for name, question, expect in (
            ("connection, shared contact", "How are 923000001003 and 923000001002 connected within three hops, not through services or hubs?", ["923000001036"]),
            ("connection, direct contact", "How are 923000001003 and 923000001036 connected within three hops, not through services or hubs?", ["direct contact"]),
        ):
            data, seconds = stack.ask(question)
            record(name, all(e in answer_of(data) for e in expect) and "ask_engine_v2" in "/".join(data.get("route", [])), short(answer_of(data)), seconds)

    if args.real:
        print("\n-- the six checks on the private real collection (counts and booleans only) --")
        code = subprocess.run([sys.executable, "-X", "utf8", __file__.replace("live_smoke.py", "live_smoke_real.py")]).returncode
        record("real collection, six checks", code == 0, "exit %d" % code)

    print("\nLIVE SMOKE: %s (%d of %d, %.0f s)" % ("PASS" if all(results) else "FAIL", sum(results), len(results), time.time() - started))
    return 0 if all(results) else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
