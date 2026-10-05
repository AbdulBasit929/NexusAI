#!/usr/bin/env python3
"""QUESTION FACTORY: unseen questions about the case data, each with an independent SQL answer key computed at generation time.

Why it exists: the 103 evaluation questions were used to tune the system, so they cannot say how it behaves on a phrasing nobody wrote for it.
This generates hundreds of questions from the semantic layer (field names, synonyms, value lists) and from values sampled out of the database
under test, and computes the correct answer for each with plain SQL that does not go through the product's compiler.
Test-only: nothing here is shipped, and the product never sees these templates.

    # 1. generate (reads the database; writes questions.json)
    python factory.py generate --psql "docker exec -i nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -F |" \
        --collection nexusai-forensic-demo --seed 1 --out questions.json
    # 2. ask the running API every question and judge the answers
    python factory.py run --questions questions.json --arm baseline
    # 3. offline check of the generator and the answer keys (no API needed)
    python factory.py selftest --psql "runuser -u postgres -- psql -d nexusai_replica -At -F |" --collection records-demo

Read-only: every statement is a SELECT inside a READ ONLY transaction with a statement timeout.
Verdicts: CORRECT, WRONG (a confident answer that contradicts the key, including a total returned for a question that names something it cannot bind),
ABSTAINED (a clarification or an honest "cannot verify"), NOT_STATED (no usable answer text), ERROR.
"""
import argparse
import datetime
import glob
import io
import json
import os
import random
import re
import shlex
import statistics
import subprocess
import sys
import time
import urllib.error
import urllib.request

import yaml

HERE = os.path.dirname(os.path.abspath(__file__))
LAYER = os.path.normpath(os.path.join(HERE, "..", "..", "semantic_layer"))
RECORD_FAMILIES = ("cdr", "ipdr", "anpr", "access_log", "transaction", "subscriber", "tower_location")
# Phrasing only. Test data; the product never reads this.
NOUN = {"cdr": ["call records", "calls", "CDR records"], "ipdr": ["internet sessions", "IPDR records", "data sessions"],
        "anpr": ["vehicle sightings", "plate reads", "ANPR records"], "access_log": ["access log entries", "log entries", "web requests"],
        "transaction": ["transactions", "payments", "financial transactions"], "subscriber": ["subscribers", "subscriber records", "registered numbers"],
        "tower_location": ["towers", "cell towers", "tower records"]}
MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"]
# Synonyms that read badly inside a question ("have where DHA Lahore", "distinct type appear").
BAD_SYNONYMS = {"who", "versus", "in or out", "type", "kind", "where", "when", "what", "how", "number of", "data"}
UNKNOWN_NAMES = ["Zubair", "Mr Qureshi", "Hamza Ghauri", "Nadia Farooqui"]  # chosen not to appear in any seed file


# ---------------------------------------------------------------- database access (read only)
class Db:
    def __init__(self, command, collection, tenant="default"):
        self.command, self.collection, self.tenant = shlex.split(command), collection, tenant

    def rows(self, sql):
        if not re.match(r"^\s*(select|with)\b", sql, re.I):
            raise ValueError("only SELECT statements are allowed: " + sql[:80])
        script = ("BEGIN READ ONLY;\nSET LOCAL statement_timeout = '60s';\nSELECT set_config('app.tenant_id', '%s', true);\n%s;\nCOMMIT;\n" % (self.tenant, sql.rstrip().rstrip(";")))
        done = subprocess.run(self.command, input=script, capture_output=True, text=True, timeout=180)
        if done.returncode != 0:
            raise RuntimeError("psql failed: " + done.stderr[:300])
        lines = [l for l in done.stdout.splitlines() if l not in ("BEGIN", "SET", "COMMIT", "") and not re.match(r"^(t|f)?$", l)]
        # psql -At prints the set_config value (the tenant) as its own line: drop exactly that one
        if lines and lines[0] == self.tenant:
            lines = lines[1:]
        return [l.split("|") for l in lines]

    def scalar(self, sql):
        rows = self.rows(sql)
        return rows[0][0] if rows and rows[0] else None


# ---------------------------------------------------------------- semantic layer
def load_layer():
    families = {}
    for path in sorted(glob.glob(os.path.join(LAYER, "*.yaml"))):
        with io.open(path, encoding="utf-8") as handle:
            doc = yaml.safe_load(handle)
        if doc.get("record_type") in RECORD_FAMILIES:
            families[doc["record_type"]] = doc
    return families


def sql_str(value):
    return "'" + str(value).replace("'", "''") + "'"


def raw_expr(field):
    names = field.get("source_names") or []
    return "COALESCE(" + ", ".join("raw_payload->>%s" % sql_str(n) for n in names) + ")"


def typed_expr(field):
    raw = raw_expr(field)
    if field["type"] == "NUMBER":
        return "(CASE WHEN %s ~ '^-?[0-9]+(\\.[0-9]+)?$' THEN (%s)::numeric ELSE NULL END)" % (raw, raw)
    if field["type"] == "TIMESTAMP":
        return "(CASE WHEN %s ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}' THEN replace(%s,'Z','')::timestamp ELSE NULL END)" % (raw, raw)
    return raw


def scope(db, family):
    return "tenant_id = %s AND collection_id = %s AND record_type = %s" % (sql_str(db.tenant), sql_str(db.collection), sql_str(family))


# ---------------------------------------------------------------- generation
def fmt_num(value):
    value = float(value)
    return "{:,}".format(int(value)) if value.is_integer() else "{:,.2f}".format(value)


def spec(qid, family, intent, question, sql, kind, **extra):
    return dict({"id": qid, "family": family, "intent": intent, "question": question, "oracle_sql": sql, "kind": kind}, **extra)


def generate(db, seed):
    rnd = random.Random(seed)
    out, counter = [], {}

    def add(item_family, intent, question, sql, kind, **extra):
        counter[(item_family, intent)] = counter.get((item_family, intent), 0) + 1
        out.append(spec("%s-%s-%02d" % (item_family.upper()[:4], intent, counter[(item_family, intent)]), item_family, intent, question, sql, kind, **extra))

    layer = load_layer()
    for family, doc in layer.items():
        base = scope(db, family)
        total = int(db.scalar("SELECT count(*) FROM forensic.records WHERE %s" % base) or 0)
        if total == 0:
            continue
        nouns = NOUN[family]
        fields = [f for f in doc["fields"] if f.get("sensitivity") != "PII"]
        for phrase in ("How many {n} are there in this case?", "Count the {n}", "What is the total number of {n}?"):
            add(family, "count_all", phrase.format(n=rnd.choice(nouns)), "SELECT count(*) FROM forensic.records WHERE %s" % base, "number")
        for field in fields:
            expr, name = typed_expr(field), field["display_name"].lower()
            syns = [s for s in field.get("synonyms", []) if len(s) > 3 and s.lower() not in BAD_SYNONYMS] or [name]
            if field["type"] == "STRING" and field.get("groupable"):
                top = db.rows("SELECT %s v, count(*) c FROM forensic.records WHERE %s AND %s IS NOT NULL AND %s <> '' GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 6" % (expr, base, expr, expr))
                top = [(r[0], int(r[1])) for r in top if len(r) == 2]
                distinct = int(db.scalar("SELECT count(DISTINCT %s) FROM forensic.records WHERE %s" % (expr, base)) or 0)
                if top and distinct > 1:
                    best = top[0][1]
                    winners = [v for v, c in top if c == best]
                    others = [v for v, c in top if c != best][:3]
                    add(family, "top_group", rnd.choice(["Which %s appears most often in the %s?", "What is the most common %s in the %s?", "Which %s has the most %s?"]) % (rnd.choice(syns), rnd.choice(nouns)),
                        "SELECT %s v, count(*) FROM forensic.records WHERE %s GROUP BY 1 ORDER BY 2 DESC LIMIT 3" % (expr, base), "top", winners=winners, others=others, count=best)
                    add(family, "count_distinct", rnd.choice(["How many different %s are in the %s?", "How many distinct %s appear in the %s?", "How many unique %s do the %s have?"]) % (rnd.choice(syns), rnd.choice(nouns)),
                        "SELECT count(DISTINCT %s) FROM forensic.records WHERE %s AND %s IS NOT NULL AND %s <> ''" % (expr, base, expr, expr), "number")
                    for value, count in rnd.sample(top, min(2, len(top))):
                        phrasing = rnd.choice(["How many %s have %s %s?", "How many %s are there where the %s is %s?", "Count the %s with %s %s"])
                        add(family, "count_eq", phrasing % (rnd.choice(nouns), rnd.choice(syns), value), "SELECT count(*) FROM forensic.records WHERE %s AND %s = %s" % (base, expr, sql_str(value)), "number")
                        if field.get("sensitivity") == "IDENTIFIER" and family in ("cdr", "ipdr"):
                            add(family, "entity_events", "What did %s do? How many %s are there for it?" % (value, rnd.choice(nouns)), "SELECT count(*) FROM forensic.records WHERE %s AND %s = %s" % (base, expr, sql_str(value)), "number")
                    if field.get("sensitivity") == "IDENTIFIER":
                        fake = "9230" + "".join(rnd.choice("0123456789") for _ in range(8))
                        add(family, "absent_value", "How many %s does %s have?" % (rnd.choice(nouns), fake), "SELECT count(*) FROM forensic.records WHERE %s AND %s = %s" % (base, expr, sql_str(fake)), "absence", total=total)
            if field["type"] == "NUMBER":
                for agg, words in (("SUM", ["What is the total %s of all the %s?", "What is the sum of the %s across the %s?"]), ("AVG", ["What is the average %s of the %s?"]),
                                   ("MAX", ["What is the largest %s among the %s?", "What is the highest %s in the %s?"]), ("MIN", ["What is the smallest %s among the %s?"])):
                    if agg in field.get("allowed_aggregates", []):
                        add(family, agg.lower(), rnd.choice(words) % (rnd.choice(syns), rnd.choice(nouns)), "SELECT %s(%s) FROM forensic.records WHERE %s" % (agg, expr, base), "number")
            if field["type"] == "TIMESTAMP" and "MIN" in field.get("allowed_aggregates", []):
                first = db.scalar("SELECT min(%s)::date FROM forensic.records WHERE %s" % (expr, base))
                last = db.scalar("SELECT max(%s)::date FROM forensic.records WHERE %s" % (expr, base))
                if first and last and first != "":
                    add(family, "earliest", rnd.choice(["When was the earliest of the %s?", "What is the date of the first of the %s?"]) % rnd.choice(nouns), "SELECT min(%s)::date FROM forensic.records WHERE %s" % (expr, base), "date")
                    add(family, "latest", rnd.choice(["When was the most recent of the %s?", "What is the date of the last of the %s?"]) % rnd.choice(nouns), "SELECT max(%s)::date FROM forensic.records WHERE %s" % (expr, base), "date")
                    d1 = datetime.date.fromisoformat(first)
                    d2 = datetime.date.fromisoformat(last)
                    if (d2 - d1).days >= 10:
                        a = d1 + datetime.timedelta(days=rnd.randint(0, max(1, (d2 - d1).days // 2)))
                        b = a + datetime.timedelta(days=rnd.randint(3, max(4, (d2 - d1).days // 2)))
                        add(family, "date_range", "How many %s are there between %s and %s?" % (rnd.choice(nouns), a.isoformat(), b.isoformat()),
                            "SELECT count(*) FROM forensic.records WHERE %s AND %s::date BETWEEN '%s' AND '%s'" % (base, expr, a, b), "number")
                        add(family, "month_count", "How many %s were there in %s %d?" % (rnd.choice(nouns), MONTHS[a.month - 1], a.year),
                            "SELECT count(*) FROM forensic.records WHERE %s AND date_trunc('month', %s) = '%s-%02d-01'::timestamp" % (base, expr, a.year, a.month), "number")
                    if family in ("cdr", "ipdr", "access_log", "anpr"):
                        add(family, "night", "How many %s were there at night?" % rnd.choice(nouns),
                            "SELECT count(*) FROM forensic.records WHERE %s AND extract(hour from %s) IN (0,1,2,3,4,5)" % (base, expr), "number", note="night = 00:00 to 05:59 in the source time zone")
                    break
        # Questions that name something the data cannot bind: a total is the wrong answer to every one of these.
        if family in ("cdr", "ipdr", "anpr", "access_log", "transaction"):
            who = rnd.choice(UNKNOWN_NAMES)
            add(family, "unbound_name", "How many %s did %s have?" % (rnd.choice(nouns), who), "SELECT count(*) FROM forensic.records WHERE %s" % base, "unbound", total=total)
    return out


# ---------------------------------------------------------------- judging
ABSTAIN = re.compile(r"(could not|cannot|can't|unable|not able|which (number|subscriber|plate|field)|name the|tell me|please (name|specify|restate)|clarif|only state this from a plan|no (single figure|matching|such)|not stated|did not (find|match))", re.I)


def number_in(text, expected):
    if expected is None or expected == "":
        return False
    value = float(expected)
    forms = {fmt_num(value), str(int(value)) if value.is_integer() else str(value), "{:,}".format(int(value)) if value.is_integer() else "{:,.2f}".format(value), "{:.2f}".format(value), "{:.1f}".format(value)}
    return any(re.search(r"(?<![\d,.])" + re.escape(f) + r"(?![\d,]|\.\d)", text) for f in forms)


def date_in(text, iso):
    d = datetime.date.fromisoformat(iso)
    forms = [iso, "%s %d, %d" % (MONTHS[d.month - 1], d.day, d.year), "%d %s %d" % (d.day, MONTHS[d.month - 1], d.year), "%s %d %d" % (MONTHS[d.month - 1][:3], d.day, d.year)]
    return any(f.lower() in text.lower() for f in forms)


def judge(item, expected, text, status, route):
    if status != 200:
        return "ERROR", "HTTP %s" % status
    if not text.strip():
        return "NOT_STATED", "empty answer"
    abstained = bool(ABSTAIN.search(text)) or "clarification" in (route or []) or "verified_only_withheld" in (route or [])
    kind = item["kind"]
    if kind == "number":
        if number_in(text, expected):
            return "CORRECT", "states %s" % expected
        if abstained and not re.search(r"\d", text):
            return "ABSTAINED", "no figure"
        return ("ABSTAINED", "withheld") if abstained and "clarification" in (route or []) else ("WRONG", "expected %s" % expected)
    if kind == "date":
        return ("CORRECT", "states %s" % expected) if date_in(text, expected) else (("ABSTAINED", "withheld") if abstained else ("WRONG", "expected %s" % expected))
    if kind == "top":
        winners, others = item["winners"], item["others"]
        if any(w.lower() in text.lower() for w in winners):
            return "CORRECT", "names %s" % winners[0]
        if abstained:
            return "ABSTAINED", "withheld"
        return ("WRONG", "names a different value, expected %s" % winners) if any(o.lower() in text.lower() for o in others) else ("WRONG", "expected %s" % winners)
    if kind in ("absence", "unbound"):
        total = item["total"]
        if number_in(text, total) and not abstained:
            return "WRONG", "states the total (%s) as the answer" % fmt_num(total)
        if kind == "absence" and (re.search(r"(?<![\d,.])0(?![\d,.])", text) or re.search(r"no (matching|records)", text, re.I)):
            return "CORRECT", "reports nothing found"
        if abstained or "clarification" in (route or []):
            return "ABSTAINED", "withheld or asked back"
        return ("WRONG", "states a figure for something that cannot be bound") if re.search(r"\d", text) else ("ABSTAINED", "no figure")
    return "NOT_STATED", "unknown kind"


# ---------------------------------------------------------------- running against the API
def ask(url, key, collection, question, timeout=900):
    payload = json.dumps({"tenant_id": "default", "collection_id": collection, "query": question}).encode("utf-8")
    request = urllib.request.Request(url, data=payload, method="POST")
    for header, value in (("Content-Type", "application/json"), ("X-Forensic-Tenant-ID", "default"), ("X-Forensic-Collection-ID", collection),
                          ("X-Forensic-Actor-ID", "investigation-workspace"), ("X-Forensic-Subject-ID", "investigation-workspace"), ("X-Forensic-Actor-Role", "user")):
        request.add_header(header, value)
    if key:
        request.add_header("Authorization", "Bearer " + key)
    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return response.status, json.loads(response.read().decode("utf-8")), time.time() - started
    except urllib.error.HTTPError as exc:
        return exc.code, {}, time.time() - started
    except Exception as exc:  # noqa: BLE001 - a probe reports its own failure
        return 0, {"_error": str(exc)[:200]}, time.time() - started


def answer_text(blob):
    enterprise = blob.get("enterprise") if isinstance(blob.get("enterprise"), dict) else {}
    narrative = enterprise.get("narrative") if isinstance(enterprise.get("narrative"), dict) else {}
    parts = [enterprise.get("executive_answer"), narrative.get("direct_answer"), enterprise.get("summary")]
    return "\n".join(p.strip() for p in parts if isinstance(p, str) and p.strip())


def expected_of(db, item):
    return db.scalar(item["oracle_sql"]) if item["kind"] in ("number", "date") else None


def summarise(rows):
    by = {}
    for r in rows:
        key = (r["family"], r["intent"])
        by.setdefault(key, {"CORRECT": 0, "WRONG": 0, "ABSTAINED": 0, "NOT_STATED": 0, "ERROR": 0})[r["verdict"]] += 1
    total = {"CORRECT": 0, "WRONG": 0, "ABSTAINED": 0, "NOT_STATED": 0, "ERROR": 0}
    for r in rows:
        total[r["verdict"]] += 1
    lines = ["%-14s %-15s %5s %5s %5s %5s" % ("family", "intent", "ok", "wrong", "abst", "other")]
    for (family, intent), c in sorted(by.items()):
        lines.append("%-14s %-15s %5d %5d %5d %5d" % (family, intent, c["CORRECT"], c["WRONG"], c["ABSTAINED"], c["NOT_STATED"] + c["ERROR"]))
    n = len(rows) or 1
    lines.append("TOTAL %d: correct %d (%.0f%%)  wrong %d (%.1f%%)  abstained %d  other %d" % (len(rows), total["CORRECT"], 100.0 * total["CORRECT"] / n, total["WRONG"], 100.0 * total["WRONG"] / n, total["ABSTAINED"], total["NOT_STATED"] + total["ERROR"]))
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="cmd", required=True)
    for name in ("generate", "selftest"):
        p = sub.add_parser(name)
        p.add_argument("--psql", required=True, help="command that reads SQL on stdin and prints rows (psql -At -F '|')")
        p.add_argument("--collection", required=True)
        p.add_argument("--seed", type=int, default=1)
        p.add_argument("--out", default="questions.json")
    r = sub.add_parser("run")
    r.add_argument("--questions", default="questions.json")
    r.add_argument("--arm", required=True)
    r.add_argument("--psql", help="needed to re-evaluate the answer keys now (recommended); otherwise the generation-time keys are used")
    r.add_argument("--collection")
    r.add_argument("--limit", type=int, default=0)
    r.add_argument("--per-intent", type=int, default=0, help="take at most this many questions per (family, intent): a spread across everything in far fewer questions")
    r.add_argument("--budget-minutes", type=float, default=0, help="stop cleanly after this long; results so far are kept and --resume continues")
    r.add_argument("--resume", action="store_true", help="skip ids already in <arm>/results.json")
    args = parser.parse_args()

    if args.cmd in ("generate", "selftest"):
        db = Db(args.psql, args.collection)
        items = generate(db, args.seed)
        for item in items:
            item["expected"] = expected_of(db, item)
            item["collection"] = args.collection
        # An aggregate over a field with no values has no answer; asking it would test nothing.
        items = [i for i in items if i["kind"] not in ("number", "date") or i["expected"] not in (None, "")]
        if args.cmd == "generate":
            with io.open(args.out, "w", encoding="utf-8") as handle:
                json.dump({"seed": args.seed, "collection": args.collection, "generated": datetime.datetime.now().isoformat(timespec="seconds"), "questions": items}, handle, indent=1, ensure_ascii=False)
        by = {}
        for item in items:
            by[(item["family"], item["intent"])] = by.get((item["family"], item["intent"]), 0) + 1
        print("%d questions" % len(items))
        for (family, intent), n in sorted(by.items()):
            print("  %-14s %-15s %3d" % (family, intent, n))
        print("sample:")
        for item in items[:: max(1, len(items) // 10)][:10]:
            print("  [%s] %s  ->  %s" % (item["id"], item["question"], item.get("expected") if item["kind"] in ("number", "date") else item["kind"]))
        return 0

    with io.open(args.questions, encoding="utf-8") as handle:
        data = json.load(handle)
    db = Db(args.psql, args.collection or data["collection"]) if args.psql else None
    url = os.environ.get("NEXUSAI_PROBE_URL", "http://127.0.0.1:8091/query/hybrid")
    key = os.environ.get("FORENSIC_RECORDS_API_KEY", "")
    out_dir = os.path.join(HERE, args.arm)
    os.makedirs(out_dir, exist_ok=True)
    rows = []
    items = data["questions"]
    if args.per_intent:
        seen, picked = {}, []
        for item in items:
            group = (item["family"], item["intent"])
            if seen.get(group, 0) < args.per_intent:
                seen[group] = seen.get(group, 0) + 1
                picked.append(item)
        items = picked
    if args.limit:
        items = items[: args.limit]
    results_path = os.path.join(out_dir, "results.json")
    if args.resume and os.path.exists(results_path):
        with io.open(results_path, encoding="utf-8") as handle:
            rows = json.load(handle)
        done = {r["id"] for r in rows}
        items = [i for i in items if i["id"] not in done]
        print("resuming: %d done, %d to go" % (len(done), len(items)))
    started_all = time.time()
    for item in items:
        if args.budget_minutes and (time.time() - started_all) > args.budget_minutes * 60:
            print("time budget reached; stopping cleanly. Continue with --resume.")
            break
        expected = expected_of(db, item) if db else item.get("expected")
        status, blob, seconds = ask(url, key, item["collection"], item["question"])
        text = answer_text(blob)
        verdict, why = judge(item, expected, text, status, blob.get("route"))
        row = {k: item[k] for k in ("id", "family", "intent", "question", "kind")}
        row.update(expected=expected, verdict=verdict, why=why, route=blob.get("route"), seconds=round(seconds, 1), text=text[:500])
        rows.append(row)
        print("%-9s %-24s %-8s %6.1fs  %s" % (verdict, item["id"], item["kind"], seconds, item["question"]))
        with io.open(os.path.join(out_dir, "results.json"), "w", encoding="utf-8") as handle:
            json.dump(rows, handle, indent=1, ensure_ascii=False)
    print("\n" + summarise(rows))
    wrong = [r for r in rows if r["verdict"] == "WRONG"]
    with io.open(os.path.join(out_dir, "summary.txt"), "w", encoding="utf-8") as handle:
        handle.write(summarise(rows) + "\n\nCONFIDENT-WRONG (%d):\n" % len(wrong))
        for r in wrong:
            handle.write("  [%s] %s\n      key: %s   answered: %s\n      -> %s\n" % (r["id"], r["question"], r["expected"], r["text"][:140].replace("\n", " "), r["why"]))
    print("\nwritten: %s" % out_dir)
    return 0


if __name__ == "__main__":
    sys.exit(main())
