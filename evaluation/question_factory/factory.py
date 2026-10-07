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
        "anpr": ["vehicle sightings", "camera sightings", "ANPR records"], "access_log": ["access log entries", "log entries", "web requests"],
        "transaction": ["transactions", "payments", "financial transactions"], "subscriber": ["subscribers", "subscriber records", "registered numbers"],
        "tower_location": ["towers", "cell towers", "tower records"]}
MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"]
# Synonyms that read badly inside a question ("have where DHA Lahore", "distinct type appear").
BAD_SYNONYMS = {"failed", "who", "versus", "in or out", "type", "kind", "where", "when", "what", "how", "number of", "data"}
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


def phrases(field, doc):
    """Ways to name a field in a question. A synonym that another field of the same family also answers to ("number" for both the subscriber and the dialled number)
    makes the QUESTION ambiguous, and the right answer to an ambiguous question is to ask back, so such words are not used as the only name of a field."""
    others = [f for f in doc["fields"] if f["id"] != field["id"]]
    pool = " | ".join(" ".join([o["display_name"]] + list(o.get("synonyms", []))).lower() for o in others)
    clear = []
    for syn in field.get("synonyms", []):
        syn = syn.strip()
        if len(syn) > 3 and syn.lower() not in BAD_SYNONYMS and not re.search(r"\b" + re.escape(syn.lower()) + r"\b", pool):
            clear.append(syn)
    return clear or [field["display_name"].lower()]


def generate(db, seed):
    rnd = random.Random(seed)
    out, counter, family_total = [], {}, {}

    def add(item_family, intent, question, sql, kind, **extra):
        extra.setdefault("total", family_total.get(item_family))
        counter[(item_family, intent)] = counter.get((item_family, intent), 0) + 1
        out.append(spec("%s-%s-%02d" % (item_family.upper()[:4], intent, counter[(item_family, intent)]), item_family, intent, question, sql, kind, **extra))

    layer = load_layer()
    for family, doc in layer.items():
        base = scope(db, family)
        total = int(db.scalar("SELECT count(*) FROM forensic.records WHERE %s" % base) or 0)
        if total == 0:
            continue
        family_total[family] = total
        nouns = NOUN[family]
        fields = [f for f in doc["fields"] if f.get("sensitivity") != "PII"]
        identifier_values = []
        for phrase in ("How many {n} are there in this case?", "Count the {n}", "What is the total number of {n}?"):
            add(family, "count_all", phrase.format(n=rnd.choice(nouns)), "SELECT count(*) FROM forensic.records WHERE %s" % base, "number")
        for field in fields:
            expr, name = typed_expr(field), field["display_name"].lower()
            syns = phrases(field, doc)
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
                            identifier_values.append(value)
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
                    shown = field["display_name"].lower()
                    add(family, "earliest", rnd.choice(["What is the earliest %s in the %s?", "Which is the first %s among the %s?"]) % (shown, rnd.choice(nouns)), "SELECT min(%s)::date FROM forensic.records WHERE %s" % (expr, base), "date")
                    add(family, "latest", rnd.choice(["What is the latest %s in the %s?", "Which is the most recent %s among the %s?"]) % (shown, rnd.choice(nouns)), "SELECT max(%s)::date FROM forensic.records WHERE %s" % (expr, base), "date")
                    d1 = datetime.date.fromisoformat(first)
                    d2 = datetime.date.fromisoformat(last)
                    if (d2 - d1).days >= 10:
                        a = d1 + datetime.timedelta(days=rnd.randint(0, max(1, (d2 - d1).days // 2)))
                        b = a + datetime.timedelta(days=rnd.randint(3, max(4, (d2 - d1).days // 2)))
                        add(family, "date_range", ("How many %s are there between %s and %s?" % (rnd.choice(nouns), a.isoformat(), b.isoformat())) if family in ("cdr", "ipdr", "anpr", "access_log", "transaction")
                            else ("How many %s have a %s between %s and %s?" % (rnd.choice(nouns), shown, a.isoformat(), b.isoformat())),
                            "SELECT count(*) FROM forensic.records WHERE %s AND %s::date BETWEEN '%s' AND '%s'" % (base, expr, a, b), "number")
                        add(family, "month_count", "How many %s were there in %s %d?" % (rnd.choice(nouns), MONTHS[a.month - 1], a.year),
                            "SELECT count(*) FROM forensic.records WHERE %s AND date_trunc('month', %s) = '%s-%02d-01'::timestamp" % (base, expr, a.year, a.month), "number")
                    if family in ("cdr", "ipdr", "access_log", "anpr"):
                        add(family, "night", "How many %s were there at night?" % rnd.choice(nouns),
                            "SELECT count(*) FROM forensic.records WHERE %s AND extract(hour from %s) IN (0,1,2,3,4,5)" % (base, expr), "number", note="night = 00:00 to 05:59 in the source time zone")
                    break
        # "Involving": the value may sit in ANY identifier field (the subscriber or the dialled party), so the key is the union. A search of one field only reports a false absence.
        id_fields = [f for f in fields if f.get("sensitivity") == "IDENTIFIER" and f["type"] == "STRING"]
        for value in rnd.sample(sorted(set(identifier_values)), min(3, len(set(identifier_values)))):
            union = " OR ".join("%s = %s" % (typed_expr(f), sql_str(value)) for f in id_fields)
            add(family, "involving", rnd.choice(["How many %s involve %s?", "How many %s are linked to %s?", "How many %s mention the number %s?"]) % (rnd.choice(nouns), value), "SELECT count(*) FROM forensic.records WHERE %s AND (%s)" % (base, union), "number")
        # Questions that name something the data cannot bind: a total is the wrong answer to every one of these.
        if family in ("cdr", "ipdr", "anpr", "access_log", "transaction"):
            who = rnd.choice(UNKNOWN_NAMES)
            add(family, "unbound_name", "How many %s did %s have?" % (rnd.choice(nouns), who), "SELECT count(*) FROM forensic.records WHERE %s" % base, "unbound", total=total)
    return out


# ---------------------------------------------------------------- judging
ABSTAIN = re.compile(r"(did not apply the condition|counts every record of its kind rather than|could not|cannot|can't|unable|not able|which (number|subscriber|plate|field)|name the|tell me|please (name|specify|restate)|clarif|only state this from a plan|no (single figure|matching|such)|not stated|did not (find|match))", re.I)

DISCLOSURE = re.compile(r"did not apply the condition|counts every record of its kind rather than", re.I)


def number_in(text, expected):
    if expected is None or expected == "":
        return False
    value = float(expected)
    forms = {fmt_num(value), str(int(value)) if value.is_integer() else str(value), "{:,}".format(int(value)) if value.is_integer() else "{:,.2f}".format(value), "{:.2f}".format(value), "{:.1f}".format(value)}
    if any(re.search(r"(?<![\d,.])" + re.escape(f) + r"(?![\d,]|\.\d)", text) for f in forms):
        return True
    return rounded_in(text, expected)


def rounded_in(text, expected):
    """A number the text states that IS the expected value correctly rounded to the places it shows (at least two), with or without
    thousands separators: "31.6118" for 31.611806967, "3,905,649.9336" for 3905649.9336. A wrong value is never within rounding of the key."""
    from decimal import Decimal, ROUND_HALF_UP, InvalidOperation
    try:
        want = Decimal(str(expected))
    except InvalidOperation:
        return False
    for token in re.findall(r"(?<![\d.])-?\d[\d,]*\.\d{2,}", text):
        shown = token.replace(",", "")
        places = len(shown.split(".")[1])
        try:
            if Decimal(shown) == want.quantize(Decimal(1).scaleb(-places), rounding=ROUND_HALF_UP):
                return True
        except InvalidOperation:
            continue
    return False


def date_in(text, iso):
    d = datetime.date.fromisoformat(iso)
    forms = [iso, "%s %d, %d" % (MONTHS[d.month - 1], d.day, d.year), "%d %s %d" % (d.day, MONTHS[d.month - 1], d.year), "%s %d %d" % (MONTHS[d.month - 1][:3], d.day, d.year)]
    return any(f.lower() in text.lower() for f in forms)


TAGS = [
    (re.compile(r"Computed collection ingest, KB asset", re.I), "collection overview given instead of an answer"),
    (re.compile(r"Showing \d[\d,]* of [\d,]+ .*records", re.I), "rows listed instead of an answer"),
    (re.compile(r"Retrieved \d+ cited evidence result", re.I), "retrieval stub instead of an answer"),
    (re.compile(r"\b(there are no|no \w+ (records|sightings|entries)|not found)\b", re.I), "absence claimed"),
]


def tag_of(item, expected, text):
    for pattern, tag in TAGS:
        if pattern.search(text):
            return tag
    if item["kind"] == "number" and expected not in (None, "") and float(expected) != 0 and re.search(r"(?<![\d,.])0(?!\d|[,.]\d)", text):
        return "zero returned"
    if item["kind"] == "unbound" or (item.get("total") is not None and number_in(text, item["total"])):
        return "total returned for a question about something else"
    return "wrong value or wrong field"


def judge(item, expected, text, status, route):
    verdict, why = _judge(item, expected, text, status, route)
    if verdict == "WRONG":
        why = tag_of(item, expected, text) + " | " + why
    return verdict, why


def _judge(item, expected, text, status, route):
    if status != 200:
        return "ERROR", "HTTP %s" % status
    if not text.strip():
        return "NOT_STATED", "empty answer"
    abstained = bool(ABSTAIN.search(text)) or "clarification" in (route or []) or "verified_only_withheld" in (route or [])
    kind = item["kind"]
    if kind == "number":
        if number_in(text, expected):
            return "CORRECT", "states %s" % expected
        if abstained and (not re.search(r"\d", text) or DISCLOSURE.search(text)):
            return "ABSTAINED", "no figure" if not re.search(r"\d", text) else "disclosed the unapplied condition"
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
        if kind == "absence" and (re.search(r"(?<![\d,.])0(?!\d|[,.]\d)", text) or re.search(r"no (matching|records?)\b|does not appear|searched every", text, re.I)):
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
            headers = {k.lower(): v for k, v in response.headers.items()}
            return response.status, json.loads(response.read().decode("utf-8")), time.time() - started, headers
    except urllib.error.HTTPError as exc:
        return exc.code, {}, time.time() - started, {}
    except Exception as exc:  # noqa: BLE001 - a probe reports its own failure
        return 0, {"_error": str(exc)[:200]}, time.time() - started, {}


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
    tags = {}
    for r in rows:
        if r["verdict"] == "WRONG":
            tag = (r.get("why") or "").split(" | ")[0]
            tags[tag] = tags.get(tag, 0) + 1
    n = len(rows) or 1
    lines.append("TOTAL %d: correct %d (%.0f%%)  wrong %d (%.1f%%)  abstained %d  other %d" % (len(rows), total["CORRECT"], 100.0 * total["CORRECT"] / n, total["WRONG"], 100.0 * total["WRONG"] / n, total["ABSTAINED"], total["NOT_STATED"] + total["ERROR"]))
    if tags:
        lines.append("\nCONFIDENT-WRONG BY KIND:")
        for tag, count in sorted(tags.items(), key=lambda kv: -kv[1]):
            lines.append("  %3d  %s" % (count, tag))
    return "\n".join(lines)


def lane_fields(header):
    out = {}
    for part in (header or "").split(";"):
        if "=" in part:
            key, value = part.split("=", 1)
            out[key.strip()] = value.strip()
    return out


def percentile(values, q):
    values = sorted(values)
    if not values:
        return 0
    return values[min(len(values) - 1, int(round(q * (len(values) - 1))))]


def lane_summary(rows):
    """What the governed SQL lane did, from the X-Governed-SQL header on each response (gates G4-G8)."""
    seen = [(r, lane_fields(r.get("lane"))) for r in rows if r.get("lane")]
    if not seen:
        return ""
    lines = ["\nGOVERNED SQL LANE (%d of %d responses carry the header):" % (len(seen), len(rows))]
    states = {}
    for r, f in seen:
        states.setdefault(f.get("state", "?"), []).append((r, f))
    for state in ("answered", "abstained", "declined"):
        group = states.get(state, [])
        verdicts = {}
        for r, _ in group:
            verdicts[r["verdict"]] = verdicts.get(r["verdict"], 0) + 1
        lines.append("  %-9s %3d   by verdict: %s" % (state, len(group), ", ".join("%s %d" % kv for kv in sorted(verdicts.items())) or "-"))
    answered = states.get("answered", [])
    if answered:
        model = [float(f.get("model_ms", 0)) / 1000.0 for _, f in answered]
        run = [float(f.get("exec_ms", 0)) / 1000.0 for _, f in answered]
        total = [float(f.get("total_ms", 0)) / 1000.0 for _, f in answered]
        attempts = [int(f.get("attempts", 1)) for _, f in answered]
        lines.append("  answered: model s median %.1f p90 %.1f max %.1f | execution s median %.2f p90 %.2f | total s median %.1f p90 %.1f | two attempts: %d" % (
            percentile(model, 0.5), percentile(model, 0.9), max(model), percentile(run, 0.5), percentile(run, 0.9), percentile(total, 0.5), percentile(total, 0.9), sum(1 for a in attempts if a > 1)))
    reasons = {}
    for _, f in states.get("declined", []):
        reason = f.get("reason", "?")[:70]
        reasons[reason] = reasons.get(reason, 0) + 1
    if reasons:
        lines.append("  declined because:")
        for reason, count in sorted(reasons.items(), key=lambda kv: -kv[1])[:8]:
            lines.append("    %3d  %s" % (count, reason))
    return "\n".join(lines)


def compare(a_arm, b_arm):
    def load(arm):
        with io.open(os.path.join(HERE, arm, "results.json"), encoding="utf-8") as handle:
            return {r["id"]: r for r in json.load(handle)}
    a, b = load(a_arm), load(b_arm)
    ids = [i for i in a if i in b]
    order = {"CORRECT": 3, "ABSTAINED": 2, "NOT_STATED": 1, "ERROR": 0, "WRONG": 0}
    better = [i for i in ids if order[b[i]["verdict"]] > order[a[i]["verdict"]]]
    worse = [i for i in ids if order[b[i]["verdict"]] < order[a[i]["verdict"]]]
    def count(rows, v):
        return sum(1 for i in ids if rows[i]["verdict"] == v)
    print("compared %d questions: %s -> %s" % (len(ids), a_arm, b_arm))
    for v in ("CORRECT", "WRONG", "ABSTAINED", "NOT_STATED", "ERROR"):
        print("  %-10s %3d -> %3d" % (v, count(a, v), count(b, v)))
    print("improved: %d   worse: %d   changed text only: %d" % (len(better), len(worse), sum(1 for i in ids if a[i]["verdict"] == b[i]["verdict"] and a[i]["text"] != b[i]["text"])))
    for label, group in (("IMPROVED", better), ("WORSE", worse)):
        for i in group:
            print("\n%s [%s] %s\n   %s -> %s\n   was: %s\n   now: %s" % (label, i, a[i]["question"], a[i]["verdict"], b[i]["verdict"], a[i]["text"][:110].replace("\n", " "), b[i]["text"][:110].replace("\n", " ")))


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="cmd", required=True)
    for name in ("generate", "selftest"):
        p = sub.add_parser(name)
        p.add_argument("--psql", required=True, help="command that reads SQL on stdin and prints rows (psql -At -F '|')")
        p.add_argument("--collection", required=True)
        p.add_argument("--seed", type=int, default=1)
        p.add_argument("--out", default="questions.json")
    e = sub.add_parser("explain", help="ask ONE question again and print the answer key, the answer, the lane header and the query the lane ran")
    e.add_argument("--questions", default="questions.json")
    e.add_argument("--id", required=True)
    j = sub.add_parser("rejudge", help="re-read saved results with the current number matcher (rounded renditions count); writes <arm>/results-rejudged.json and prints the table")
    j.add_argument("arms", nargs="+")
    c = sub.add_parser("compare")
    c.add_argument("a")
    c.add_argument("b")
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

    if args.cmd == "compare":
        compare(args.a, args.b)
        return 0

    if args.cmd == "rejudge":
        for arm in args.arms:
            path = os.path.join(HERE, arm, "results.json")
            with io.open(path, encoding="utf-8") as handle:
                rows = json.load(handle)
            changed = []
            for row in rows:
                if row["kind"] == "number" and row["verdict"] != "CORRECT" and row.get("expected") not in (None, "") and rounded_in(row.get("text", ""), row["expected"]):
                    changed.append((row["id"], row["verdict"]))
                    row["verdict"], row["why"] = "CORRECT", "states %s (rounded)" % row["expected"]
            with io.open(os.path.join(HERE, arm, "results-rejudged.json"), "w", encoding="utf-8") as handle:
                json.dump(rows, handle, indent=1, ensure_ascii=False)
            print("== %s: %d verdict(s) changed to CORRECT (%s)" % (arm, len(changed), ", ".join("%s was %s" % c for c in changed) or "none"))
            print("   " + "  ".join("%s %d" % (v, sum(1 for r in rows if r["verdict"] == v)) for v in ("CORRECT", "WRONG", "ABSTAINED", "NOT_STATED", "ERROR")))
        return 0

    if args.cmd == "explain":
        with io.open(args.questions, encoding="utf-8") as handle:
            data = json.load(handle)
        item = next((i for i in data["questions"] if i["id"] == args.id), None)
        if item is None:
            print("no such question id: %s" % args.id)
            return 1
        url = os.environ.get("NEXUSAI_PROBE_URL", "http://127.0.0.1:8091/query/hybrid")
        status, blob, seconds, headers = ask(url, os.environ.get("FORENSIC_RECORDS_API_KEY", ""), item["collection"], item["question"])
        derivation = ((blob.get("enterprise") or {}).get("derivation") or {}) if isinstance(blob, dict) else {}
        print("id       : %s\nquestion : %s\nexpected : %s\noracle   : %s\nhttp     : %s  %.1fs\nlane     : %s\nanswer   : %s\nquery    : %s\nchecked  : %s\n" % (
            item["id"], item["question"], item.get("expected"), item["oracle_sql"], status, seconds, headers.get("x-governed-sql"),
            answer_text(blob).replace("\n", " | ")[:300], derivation.get("sql"), derivation.get("conditions_checked") or derivation.get("checked")))
        return 0

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
        status, blob, seconds, headers = ask(url, key, item["collection"], item["question"])
        text = answer_text(blob)
        verdict, why = judge(item, expected, text, status, blob.get("route"))
        row = {k: item[k] for k in ("id", "family", "intent", "question", "kind")}
        row.update(expected=expected, verdict=verdict, why=why, route=blob.get("route"), seconds=round(seconds, 1), text=text[:500])
        if headers.get("x-governed-sql"):
            row["lane"] = headers["x-governed-sql"]
        rows.append(row)
        print("%-9s %-24s %-8s %6.1fs  %s" % (verdict, item["id"], item["kind"], seconds, item["question"]))
        with io.open(os.path.join(out_dir, "results.json"), "w", encoding="utf-8") as handle:
            json.dump(rows, handle, indent=1, ensure_ascii=False)
    print("\n" + summarise(rows) + lane_summary(rows))
    wrong = [r for r in rows if r["verdict"] == "WRONG"]
    with io.open(os.path.join(out_dir, "summary.txt"), "w", encoding="utf-8") as handle:
        handle.write(summarise(rows) + lane_summary(rows) + "\n\nCONFIDENT-WRONG (%d):\n" % len(wrong))
        for r in wrong:
            handle.write("  [%s] %s\n      key: %s   answered: %s\n      -> %s\n" % (r["id"], r["question"], r["expected"], r["text"][:140].replace("\n", " "), r["why"]))
    print("\nwritten: %s" % out_dir)
    return 0


if __name__ == "__main__":
    sys.exit(main())
