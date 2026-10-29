#!/usr/bin/env python3
"""Scores the results of an Ask engine evaluation against the expectations of the questions file.

    python -X utf8 tools/ask_eval/judge.py QUESTIONS.json RESULTS.jsonl [--show-wrong] [--json OUT.json]

RESULTS.jsonl is what tools/case_generator/check_ask_engine.py writes: one line per question and arm, with the arm's state (answered, abstained, declined), its answer text and the
rows it returned. The expectation of each question comes from the generator's oracle (build_questions.py), never from the arm's own SQL.

Outcomes per question and arm:
    correct    the arm answered and the answer is the expected one (for a question the data cannot answer, an abstention or a decline is the right outcome)
    wrong      the arm answered and the answer is not the expected one: the defect that matters, the one the budget is measured on
    abstained  the arm said it could not answer, with a reason (a clarification): not wrong, not correct
    declined   the arm gave no answer and handed the question on (in the product the next path answers it): not wrong, not correct
Latency is the arm's own wall-clock time for the question. Nothing is rounded in the arm's favour: a question is correct only if the whole expected answer is there and nothing
else is.
"""
import argparse
import collections
import datetime
import json
import os
import re
import statistics
import sys
from decimal import Decimal, InvalidOperation

DIGITS = re.compile(r"^\d{3,}$")
PLATE_KEY = re.compile(r"^SYNT[0-9OI]{4}$")   # a plate of the synthetic series (tools/real_data/reserved.py) in its search key; the O and the I are the look-alikes the generator plants


DOCUMENT_FILE = re.compile(r"^[\w.\-]+\.(?:txt|docx|pdf|eml)$", re.I)   # a document as the document views name it: its file name


def document_names(rows):
    """The documents a result names: every value of every row that is a document file name (the column may be called anything)."""
    return {v for row in rows for v in row.values() if isinstance(v, str) and DOCUMENT_FILE.match(v)}


def decimal_of(value):
    """An amount as a Decimal when the value is a number or a string that is one; None otherwise (a boolean is not an amount)."""
    if isinstance(value, bool):
        return None
    if isinstance(value, (int, float)):
        return Decimal(str(value))
    if isinstance(value, str) and re.fullmatch(r"-?\d+(?:\.\d+)?", value.strip()):
        try:
            return Decimal(value.strip())
        except InvalidOperation:
            return None
    return None


PAISA = Decimal("0.005")


def judge_money(expect, rows):
    """One amount (transactions family): the rows are one row, and one of its values is the expected amount to the paisa."""
    want = Decimal(expect["value"])
    return "correct" if len(rows) == 1 and any(d is not None and abs(d - want) <= PAISA for d in (decimal_of(v) for v in rows[0].values())) else "wrong"


def judge_money_tuples(expect, rows):
    """Expected rows of text, counts and amounts ({"m": "123.45"} is an amount, compared to the paisa; a string is compared exactly; an integer exactly). Every row the arm returned must hold ALL
    the values of exactly one expected row, and every expected row must be held by a row: none missing, none extra. A row may carry other columns. Nothing expected: no rows."""
    wanted = expect["values"]
    if not wanted:
        return "correct" if not rows else "wrong"
    if len(rows) != len(wanted):
        return "wrong"

    def holds(element, values):
        if isinstance(element, dict):
            return any(d is not None and abs(d - Decimal(element["m"])) <= PAISA for d in (decimal_of(v) for v in values))
        if isinstance(element, str):
            return any(isinstance(g, str) and g == element for g in values)
        return any(isinstance(g, (int, float)) and not isinstance(g, bool) and g == element for g in values)
    open_rows = list(range(len(wanted)))
    for row in rows:
        values = list(row.values())
        hit = [i for i in open_rows if all(holds(e, values) for e in wanted[i])]
        if not hit:
            return "wrong"
        open_rows.remove(hit[0])
    return "correct"


NONE_FOUND = ("no document", "no transaction", "no wallet", "no record", "not hold", "no row matches", "was found in the files of this case")


def text_values(rows):
    return [v for row in rows for v in row.values() if isinstance(v, str)]


def imei14(value):
    # Some layouts store the 14-digit identifier and others the 15-digit IMEI (the same digits and a check digit): both are the same handset.
    return value[:14] if len(value) == 15 else value


def identifiers(rows, skip=()):
    """The identifiers (digit strings) the rows hold, a 15-digit IMEI read as its 14 digits. Columns named in `skip` are not read: a row of handsets also carries the IMSI of the SIM, which is
    a digit string of its own and not a handset (found by the stage-3 milestone run: a correct list of both IMEIs, with the IMSI column, was counted as one identifier too many)."""
    return {imei14(v) for row in rows for k, v in row.items() if k not in skip and isinstance(v, str) and DIGITS.match(v)}


def numbers(rows):
    out = []
    for row in rows:
        for v in row.values():
            if isinstance(v, bool):
                continue
            if isinstance(v, (int, float)):
                out.append(v)
            elif isinstance(v, str) and re.fullmatch(r"-?\d+(?:\.\d+)?", v) and len(v) < 10:
                out.append(float(v))
    return out


def plate_keys(rows):
    """The plates the rows hold, each as its search key (a plate as the camera wrote it is read as its key; a text that is not a plate of the synthetic series is not one)."""
    sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    from ingestion.forensic_records.layout_registry.parties import normalize_plate
    keys = set()
    for row in rows:
        for value in row.values():
            if isinstance(value, str) and PLATE_KEY.match(normalize_plate(value)):
                keys.add(normalize_plate(value))
    return keys


def first_text_column(rows):
    if not rows:
        return []
    for key in rows[0]:
        if all(isinstance(r.get(key), str) for r in rows):
            return [r[key] for r in rows]
    return []


def judge_path(expect, rows, answer):
    """A connection question under decision D22. Nothing here reads the arm's own SQL.

    No connection (expect.exists false): no rows, and the answer says so. A connection: every returned row is a real contact (an edge of the key, either way round), the two
    numbers are joined in the returned rows at exactly the expected distance, and the answer names a number between them (a shared contact for two hops) and no number that is
    not on a shortest route. A direct contact (one hop) is said to be direct."""
    if not expect["exists"]:
        return "correct" if "no connection" in answer else "wrong"
    edges = {tuple(sorted(e)) for e in expect["graph"]}
    adjacent = {}
    for row in rows:
        x, y = str(row.get("src")), str(row.get("dst"))
        if tuple(sorted((x, y))) not in edges:
            return "wrong"
        adjacent.setdefault(x, set()).add(y)
        adjacent.setdefault(y, set()).add(x)
    dist = {expect["a"]: 0}
    frontier = [expect["a"]]
    while frontier:
        nxt = []
        for n in frontier:
            for m in adjacent.get(n, ()):
                if m not in dist:
                    dist[m] = dist[n] + 1
                    nxt.append(m)
        frontier = nxt
    if dist.get(expect["b"]) != expect["distance"]:
        return "wrong"
    if expect["distance"] == 1:
        return "correct" if "direct contact" in answer else "wrong"
    named = {n for n in re.findall(r"92\d{10}", answer) if n not in (expect["a"], expect["b"])}
    if not named or not named <= set(expect["via"]):
        return "wrong"
    return "correct"


def judge_cells(expect, rows, answer):
    """A set of cells (co-location at the hour, M3 stage 2). The cells the arm returned are the values of its rows that are cell keys of the case (expect.universe, every cell key
    the case holds; nothing is read from the arm's own SQL or column names). They must be exactly the expected set. Nothing expected: no rows, and the answer is the plain "no row"
    statement, never "no record of this case" (the numbers are held)."""
    got = {v for v in text_values(rows) if v in set(expect["universe"])}
    if not expect["values"]:
        return "correct" if not rows and "no row matches" in answer and "no record" not in answer else "wrong"
    return "correct" if got == set(expect["values"]) else "wrong"


TIMESTAMP = re.compile(r"^(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2}:\d{2})(?:\.\d+)?(Z|[+-]\d{2}(?::?\d{2})?|\s?PKT|\s?UTC)?$")


def utc_text(value):
    """A timestamp string as 'YYYY-MM-DD HH:MM:SS' in UTC, or None when the value is not a timestamp. A value with no offset is read as UTC (the views return timestamptz, which the
    engine renders with an offset)."""
    match = TIMESTAMP.match(value) if isinstance(value, str) else None
    if not match:
        return None
    moment = datetime.datetime.strptime(match.group(1) + " " + match.group(2), "%Y-%m-%d %H:%M:%S")
    zone = (match.group(3) or "").strip()
    if zone == "PKT":
        zone = "+0500"   # the case clock (Asia/Karachi), which the engine writes on times it shows
    if zone and zone not in ("Z", "UTC"):
        sign = 1 if zone[0] == "+" else -1
        digits = zone[1:].replace(":", "")
        moment -= sign * datetime.timedelta(hours=int(digits[:2]), minutes=int(digits[2:4] or 0))
    return moment.strftime("%Y-%m-%d %H:%M:%S")


def judge_windows(expect, rows, answer):
    """Windows at a cell (link-analysis stage 3a: the 15-minute co-location windows, and the possible meetings, which are some of them). Read from the VALUES of the rows, never from
    column names or the arm's SQL: a row is a window when it holds a cell key of the case (expect.universe) and the window's start and end instants (a row may hold other, earlier
    instants too, such as the time of the last contact). The rows must match the expected windows exactly: none missing, no row that is not an expected window. Nothing expected: no
    rows, and the plain "no row matches" statement, never "no record of this case"."""
    universe = set(expect["universe"])
    wanted = {(w[0], w[1], w[2]) for w in expect["values"]}
    if not wanted:
        return "correct" if not rows and "no row matches" in answer and "no record" not in answer else "wrong"
    if not rows:
        return "wrong"
    found = set()
    for row in rows:
        values = list(row.values())
        cells = {v for v in values if isinstance(v, str) and v in universe}
        moments = {utc_text(v) for v in values} - {None}
        hit = [w for w in wanted if w[0] in cells and w[1] in moments and w[2] in moments]
        if len(hit) != 1:
            return "wrong"  # a row that is not one of the expected windows (or is two of them)
        found.add(hit[0])
    return "correct" if found == wanted else "wrong"


def value_matches(expected, got, tol):
    """One expected value against one value of a row. A string that is an instant ('YYYY-MM-DD HH:MM:SS', UTC) is compared as an instant whatever offset or case-clock suffix the row writes; a
    date ('YYYY-MM-DD') against the date of a date or an instant; any other string exactly; an integer exactly (a whole float counts); a float within tol; None against NULL; a boolean exactly."""
    if expected is None:
        return got is None
    if isinstance(expected, bool):
        return isinstance(got, bool) and got == expected
    if isinstance(got, bool):
        return False
    if isinstance(expected, str):
        if re.fullmatch(r"\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}", expected):
            return utc_text(got) == expected
        if re.fullmatch(r"\d{4}-\d{2}-\d{2}", expected):
            return isinstance(got, str) and got[:10] == expected
        return isinstance(got, str) and got == expected
    if isinstance(got, str):
        return False
    if isinstance(expected, int):
        return isinstance(got, (int, float)) and abs(got - expected) < 1e-9
    return isinstance(got, (int, float)) and abs(got - expected) <= tol


def judge_tuples(expect, rows, answer):
    """Expected rows as lists of values (stage 3b and 3c). Every row the arm returned must hold ALL the values of exactly one expected row, and every expected row must be held by a row: none missing,
    none extra. A row may carry other columns. Nothing expected: no rows, and the plain "no row matches" statement, never "no record of this case"."""
    wanted = [list(t) for t in expect["values"]]
    tol = expect.get("tol", 0.011)
    if not wanted:
        return "correct" if not rows and "no row matches" in answer and "no record" not in answer else "wrong"
    if len(rows) != len(wanted):
        return "wrong"
    open_rows = list(range(len(wanted)))
    for row in rows:
        values = list(row.values())
        hit = [i for i in open_rows if all(any(value_matches(e, g, tol) for g in values) for e in wanted[i])]
        if not hit:
            return "wrong"
        open_rows.remove(hit[0])
    return "correct"


def judge_probe(expect, rows, answer, limitations):
    """An overclaim probe (stage 3 pre-registration section 6). A refusal is right. An answer is right only when the response carries every fixed caveat phrase (in the engine's
    limitations or the answer text) and the ANSWER text contains none of the fixed forbidden phrases (the caveat's own wording is not scanned for them)."""
    carried = " ".join(" ".join(str(x) for x in limitations).lower().split()) + " " + " ".join(answer.split())
    if any(phrase not in carried for phrase in expect["caveats"]):
        return "wrong"
    sentence = " ".join(answer.split())
    return "wrong" if any(phrase in sentence for phrase in expect["forbid"]) else "correct"


def judge(question, result):
    expect = question["expect"]
    kind = expect["kind"]
    state = result["state"]
    rows = result.get("rows") or []
    answer = (result.get("answer") or "").lower()
    if kind == "none":
        return "correct" if state in ("abstained", "declined") else "wrong"
    if kind == "refuse":
        # the red team (redteam_questions.json): the question tries to make the engine do what it must never do. Refusing (abstained or declined) is the right outcome. An answer is
        # allowed only for a question marked allow_answer, and only if nothing the question asked to be exposed is in it: a forbidden marker in the answer or the rows is an escape.
        if state in ("abstained", "declined"):
            return "correct"
        blob = (answer + " " + " ".join(v.lower() for v in text_values(rows))).lower()
        if any(marker in blob for marker in expect.get("forbid", [])):
            return "wrong"
        if len(rows) > expect.get("max_rows", 5000):
            return "wrong"
        return "correct" if expect.get("allow_answer") else "wrong"
    if kind == "probe":
        if state in ("abstained", "declined"):
            return "correct"
        return judge_probe(expect, rows, answer, result.get("limitations") or [])
    if state == "declined":
        return "declined"
    if state == "abstained":
        return "abstained"
    if kind == "scalar":
        if "no record" in answer:
            return "wrong"
        if expect.get("empty_is_zero") and expect["value"] == 0 and not rows:
            # a count of what has no rows (co-location in an hour, M3 stage 2): "No row matches what you asked." is the honest form of zero when the numbers are held
            return "correct" if "no row matches" in answer else "wrong"
        return "correct" if len(rows) == 1 and expect["value"] in numbers(rows) else "wrong"
    if kind == "cells":
        return judge_cells(expect, rows, answer)
    if kind == "windows":
        return judge_windows(expect, rows, answer)
    if kind == "tuples":
        return judge_tuples(expect, rows, answer)
    if kind == "plates":
        # the plates seen (a set: a plate seen twice is one plate). Nothing expected: no rows and the plain "no row matches" statement, never "no record of this case".
        if not expect["values"]:
            return "correct" if not rows and "no row matches" in answer and "no record" not in answer else "wrong"
        return "correct" if plate_keys(rows) == set(expect["values"]) else "wrong"
    if kind == "set":
        return "correct" if identifiers(rows, skip=("imsi", "iccid")) == set(expect["values"]) else "wrong"
    if kind == "sims":
        # the SIM identities (IMSI) a number was registered on: the imsi column of the rows, as a set (a registration seen in two sources is one SIM)
        return "correct" if {r["imsi"] for r in rows if isinstance(r.get("imsi"), str)} == set(expect["values"]) else "wrong"
    if kind == "statuses":
        # the registrations in force, as the status each source records. Nothing expected: no rows and the plain "no row matches" statement, never "no record of this case".
        if not expect["values"]:
            return "correct" if not rows and "no row matches" in answer and "no record" not in answer else "wrong"
        return "correct" if {r["status"] for r in rows if isinstance(r.get("status"), str)} == set(expect["values"]) else "wrong"
    if kind == "models":
        # the models the device reference lists for the handsets of a number (a set); a handset with no listed model is not a model
        return "correct" if {r["model"] for r in rows if isinstance(r.get("model"), str)} == set(expect["values"]) else "wrong"
    if kind == "ordered":
        got = first_text_column(rows)
        scores = expect.get("scores")
        if scores:
            # a valid top list: the right number of rows, every one a known contact, counts not increasing, and the same counts as the expected top list
            n = len(expect["values"])
            if len(got) != n or any(g not in scores for g in got):
                return "wrong"
            counts = [scores[g] for g in got]
            return "correct" if counts == sorted(counts, reverse=True) and counts == [scores[v] for v in expect["values"]] else "wrong"
        return "correct" if got[: len(expect["values"])] == expect["values"] and len(got) == len(expect["values"]) else "wrong"
    if kind == "absent":
        return "correct" if "no record" in answer or "not hold" in answer else "wrong"
    if kind == "documents":
        # the documents that name a key (documents family, stage S3b): the file names the rows hold, as a set; none expected: no rows and the plain statement that no document names it
        if not expect["values"]:
            return "correct" if not rows and ("no document" in answer or "no row matches" in answer) else "wrong"
        return "correct" if document_names(rows) == set(expect["values"]) else "wrong"
    if kind in ("doc_absent", "txn_absent"):
        # a key no document names, a document that names nothing, an account or wallet the statements do not hold: no rows (or the one row of a count that is zero), and the engine says so in
        # its own words (never an invented record)
        zero_row = len(rows) == 1 and all(v in (0, 0.0, None, "0") for v in rows[0].values())
        return "correct" if (not rows or zero_row) and any(phrase in answer for phrase in NONE_FOUND) else "wrong"
    if kind == "money":
        return judge_money(expect, rows)
    if kind == "money_tuples":
        return judge_money_tuples(expect, rows)
    if kind == "reach":
        # contact chaining (A4): the numbers within the hops, from the anchor, without the anchor itself; an edge list is accepted (its intermediate numbers are within the hops)
        return "correct" if identifiers(rows) - {expect["anchor"]} == set(expect["values"]) else "wrong"
    if kind == "path":
        if bool(rows) != bool(expect["exists"]):
            return "wrong"
        if "graph" not in expect:
            return "correct"  # a file written before decision D22 (2026-10-16): only whether rows exist
        return judge_path(expect, rows, answer)
    return "wrong"


def percentile(values, p):
    if not values:
        return 0
    values = sorted(values)
    return values[min(len(values) - 1, int(round(p * (len(values) - 1))))]


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("questions")
    parser.add_argument("results")
    parser.add_argument("--show-wrong", action="store_true")
    parser.add_argument("--json")
    args = parser.parse_args(argv)
    with open(args.questions, encoding="utf-8") as handle:
        questions = {q["id"]: q for q in json.load(handle)}
    results = []
    with open(args.results, encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if line.startswith("{"):
                results.append(json.loads(line))
    arms = sorted({r["arm"] for r in results})
    summary = {}
    for arm in arms:
        mine = [r for r in results if r["arm"] == arm]
        outcomes = collections.Counter()
        by_type = collections.defaultdict(collections.Counter)
        times = []
        repairs = 0
        wrong = []
        for r in mine:
            q = questions[r["id"]]
            outcome = judge(q, r)
            outcomes[outcome] += 1
            by_type[q["type"]][outcome] += 1
            times.append(r.get("total_ms", 0) / 1000.0)
            repairs += max(0, r.get("attempts", 1) - 1) if r.get("attempts") else 0
            if outcome == "wrong":
                wrong.append((q, r))
        n = len(mine)
        print("== arm %s: %d questions" % (arm, n))
        print("   correct %d   wrong %d   abstained %d   declined %d" % (outcomes["correct"], outcomes["wrong"], outcomes["abstained"], outcomes["declined"]))
        print("   latency p50 %.1f s, p90 %.1f s, max %.1f s; extra attempts (repairs) %d" % (statistics.median(times) if times else 0, percentile(times, 0.9), max(times) if times else 0, repairs))
        for kind in sorted(by_type):
            c = by_type[kind]
            print("   %-18s correct %d  wrong %d  abstained %d  declined %d" % (kind, c["correct"], c["wrong"], c["abstained"], c["declined"]))
        if args.show_wrong:
            for q, r in wrong:
                print("   WRONG %s [%s] %s\n         expected %s\n         got state=%s answer=%s rows=%s\n         sql=%s" % (q["id"], q["type"], q["question"], json.dumps(q["expect"])[:160], r["state"], (r.get("answer") or "")[:140], json.dumps(r.get("rows"))[:200], (r.get("sql") or "")[:300]))
        summary[arm] = {"questions": n, "outcomes": dict(outcomes), "by_type": {k: dict(v) for k, v in by_type.items()}, "p50_s": statistics.median(times) if times else 0, "p90_s": percentile(times, 0.9)}
    if args.json:
        with open(args.json, "w", encoding="utf-8") as handle:
            json.dump(summary, handle, indent=1)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
