#!/usr/bin/env python3
"""The wording gate of Amendment 5 (docs/data/link-analysis-stage3-preregistration-20261022.md): what the FIRST SENTENCE of an analysis answer says when there is no row, when a score is
given for one pair, and when the question states a conclusion the records cannot support. The numbers of the answers are judged by judge.py as before; this reads the sentences.

    python -X utf8 tools/ask_eval/wording_gate.py build --seed 113 --out W113.json      # the subset of the stage-3 banks that carries the wording (counts only unless --out)
    python -X utf8 tools/case_generator/check_ask_engine.py --questions W113.json --out R113.jsonl --seed 113 --register-case
    python -X utf8 tools/ask_eval/judge.py W113.json R113.jsonl --show-wrong            # W1: the numbers
    python -X utf8 tools/ask_eval/wording_gate.py check W113.json R113.jsonl            # W2 to W7: the sentences

The subset: every control (an empty result is the right answer), every overclaim probe, and the first positive of each of four kinds (windows, successor candidates, anchor change, speed
alerts), taken from the three banks of the case. Nothing is tuned: the banks are the ones already written, chosen by the seed.
"""
import argparse
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
sys.path.insert(0, os.path.join(HERE, "..", ".."))

FORBID = ["they met", "met with", "same person", "is also", "confirmed", "proves they know"]
NO_ROW = "No row matches what you asked"
ANALYSIS = {"v_colocation_windows", "v_meeting_candidates", "v_silent_numbers", "v_successor_candidates", "v_weekly_anchors", "v_anchor_runs", "v_movement_anchors", "v_anchor_changes",
            "v_anchor_deviations", "v_speed_alerts", "v_rhythm_changes", "v_cell_visits"}
LEAD = {
    "meeting": "Being at the same cell site does not show that two people met or know each other.",
    "successor": "A similar pattern of use does not show that two numbers have one user.",
}
CANDIDATE = "A candidate, not proof"
CUE = re.compile(r"\b(?:prove[sd]?|proof|proving|same person|knows? each other|know one another|definitely|certainly|obviously|surely|meet|met|associates?)\b")
DEFAULTS = {7, 14, 35, 2, 3, 200, 10, 5, 15, 60, 1.5}
POSITIVE_KINDS = ("coloc_windows", "successor_candidates", "anchor_change", "speed_alerts")
NUMBER = re.compile(r"\d+(?:\.\d+)?")
MSISDN = re.compile(r"92\d{10}")


def is_control(q):
    e = q["expect"]
    return bool(e.get("control")) or e.get("values") == [] or (e.get("value") == 0 and e.get("empty_is_zero"))


def select(questions):
    out, seen = [], set()
    for q in questions:
        if q["expect"].get("kind") == "probe" or is_control(q):
            out.append(q)
        elif q["type"] in POSITIVE_KINDS and q["type"] not in seen:
            seen.add(q["type"])
            out.append(q)
    return out


def build(seed):
    import case_set
    import stage3a_bank
    import stage3bc_bank
    found = [(kind, overrides) for kind, number, overrides in case_set.CASES if number == seed and kind != "sealed"]
    if not found:
        raise SystemExit("seed %d is not a development or judgement case of the register" % seed)
    params = case_set.params_for(seed, found[0][1])
    world = stage3bc_bank.World(params)
    return select(stage3a_bank.bank_questions(params) + stage3bc_bank.bank_3b(world, seed) + stage3bc_bank.bank_3c(world, seed))


def where_of(sql):
    """The WHERE clause of a query, lower case, up to the next clause (a condition is applied only if it is here: a column of the select list is not one)."""
    lower = (sql or "").lower()
    at = lower.find(" where ")
    if at < 0:
        return ""
    rest = lower[at + len(" where "):]
    for nxt in (" group by ", " order by ", " having ", " limit ", " union "):
        if nxt in rest:
            rest = rest[:rest.index(nxt)]
    return rest


def numbers_of(text):
    return {float(x) for x in NUMBER.findall(text)}


def param_tokens(params):
    """What a parameter value looks like in a sentence: the value, and for the ratio of the two daily means the number of times."""
    out = set()
    for name, value in (params or {}).items():
        out.add(float(value))
        if name == "rhythm_ratio_percent":
            out.add(float(value) / 100.0)
    return out


def check_record(q, rec):
    """The failures (gate, text) of one record; an empty list is a pass. Records that are not answered are not read for wording."""
    fails = []
    answer = rec.get("answer") or ""
    lower = answer.lower()
    for phrase in FORBID:
        if phrase in lower:
            fails.append(("W5", "forbidden phrase %r in %r" % (phrase, answer)))
    if rec.get("state") != "answered":
        return fails
    rows, views = rec.get("rows") or [], set(rec.get("views") or [])
    question = q["question"]
    params = dict(q["expect"].get("params") or {})
    for key, name in (("window_minutes", "colocation_minutes"), ("gap_minutes", "contact_gap_minutes")):
        if q["expect"].get(key):
            params.setdefault(name, q["expect"][key])
    probe = q["expect"].get("kind") == "probe"
    analysis = bool(views & ANALYSIS)
    # W2: no row on an analysis view says what was searched
    if analysis and not rows and not probe and NO_ROW in answer:
        if not answer.startswith(NO_ROW + ": "):
            fails.append(("W2", "no row, and the sentence does not say what was searched: %r" % answer))
        else:
            for number in MSISDN.findall(question):
                if number not in answer:
                    fails.append(("W2", "%s of the question is not in %r" % (number, answer)))
            for name, value in params.items():
                if not (param_tokens({name: value}) & numbers_of(answer)):
                    fails.append(("W2", "%s=%s is not in %r" % (name, value, answer)))
            allowed = numbers_of(question) | param_tokens(params) | DEFAULTS
            stray = sorted(numbers_of(answer) - allowed)
            if stray:
                fails.append(("W6", "numbers %s of %r are neither the question's nor a parameter's" % (stray, answer)))
            if "no record" in lower:
                fails.append(("W2", "says 'no record' where the records were searched: %r" % answer))
            # W8: a no-row sentence names a condition only if the query applied it
            where = where_of(rec.get("sql"))
            if "shares its handset" in answer and "shared_device" not in where:
                fails.append(("W8", "says the search required a shared handset, which the query's WHERE clause does not: %r" % answer))
            for slot in ("night", "day"):
                if (slot + " place") in answer or (slot + " anchor") in answer:
                    if ("'%s'" % slot) not in where:
                        fails.append(("W8", "says the search was of the %s slot, which the query's WHERE clause does not name: %r" % (slot, answer)))
    # W3 and W4: a score for one pair, and the correction in front of a question that states a conclusion
    if probe and analysis:
        lead = LEAD["meeting"] if views & {"v_colocation_windows", "v_meeting_candidates"} else LEAD["successor"] if "v_successor_candidates" in views else ""
        cued = CUE.search(question.lower())
        scored = len(rows) == 1 and any("score" in str(k).lower() for k in rows[0])
        if cued and lead and not answer.startswith(lead):
            fails.append(("W4", "the question states a conclusion and the answer does not begin with the correction: %r" % answer))
        elif scored and "successor" in " ".join(views) and ("switch" in question.lower() or "replace" in question.lower()):
            body = answer[len(lead) + 1:] if lead and answer.startswith(lead) else answer
            if not body.startswith(CANDIDATE):
                fails.append(("W3", "a score for one pair is not given as a candidate: %r" % answer))
    # W9: one move is a sentence that states where from, where to, the week and the distance of the row; W10: a cell visit is not a position; W11: successor candidates are candidates
    if q["type"] == "anchor_change" and len(rows) == 1 and "v_anchor_changes" in views:
        row = rows[0]
        week = str(row.get("first_week_start") or "")[:10]
        if "1 change of" not in answer or (week and week not in answer):
            fails.append(("W9", "one move is not stated with its week: %r" % answer))
        if row.get("distance_km") not in (None, "") and not any(abs(f - float(row["distance_km"])) <= 0.011 for f in numbers_of(answer)):
            fails.append(("W9", "distance_km %s is not in %r" % (row["distance_km"], answer)))
    if "v_cell_visits" in views and rows and ("position" in lower.split(".")[0]):
        fails.append(("W10", "a cell visit is called a position: %r" % answer))
    if "v_successor_candidates" in views and rows:
        body = answer[len(LEAD["successor"]) + 1:] if answer.startswith(LEAD["successor"]) else answer
        if not body.lower().startswith(("candidates, not proof", "a candidate, not proof")):
            fails.append(("W11", "successor candidates are not given as candidates: %r" % answer))
    # W7: one speed alert is a sentence that states its own distance and speed
    if q["type"] == "speed_alerts" and len(rows) == 1 and "v_speed_alerts" in views:
        figures = numbers_of(answer)
        row = rows[0]
        for column, tolerance in (("distance_km", 0.011), ("speed_kmh", 0.06)):
            if column in row and row[column] not in (None, ""):
                if not any(abs(f - float(row[column])) <= tolerance for f in figures):
                    fails.append(("W7", "%s %s is not in %r" % (column, row[column], answer)))
    return fails


def check(questions, records):
    by_id = {q["id"]: q for q in questions}
    failures, read = [], 0
    for rec in records:
        q = by_id.get(rec.get("id"))
        if q is None or rec.get("arm", "engine") != "engine":
            continue
        read += 1
        for gate, text in check_record(q, rec):
            failures.append((rec["id"], gate, text))
    return read, failures


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)
    b = sub.add_parser("build")
    b.add_argument("--seed", type=int, required=True)
    b.add_argument("--out")
    c = sub.add_parser("check")
    c.add_argument("questions")
    c.add_argument("results")
    args = parser.parse_args(argv)
    if args.command == "build":
        questions = build(args.seed)
        kinds = {}
        for q in questions:
            kinds[q["type"]] = kinds.get(q["type"], 0) + 1
        print("%d questions: %s; controls %d; probes %d" % (len(questions), kinds, sum(1 for q in questions if is_control(q)), sum(1 for q in questions if q["expect"].get("kind") == "probe")))
        if args.out:
            with open(args.out, "w", encoding="utf-8") as handle:
                json.dump(questions, handle, indent=1)
                handle.write("\n")
            print("written", args.out)
        return 0
    with open(args.questions, encoding="utf-8") as handle:
        questions = json.load(handle)
    with open(args.results, encoding="utf-8") as handle:
        records = [json.loads(line) for line in handle if line.strip()]
    read, failures = check(questions, records)
    print("%d answers read, %d failures" % (read, len(failures)))
    for ident, gate, text in failures:
        print("  %s %s: %s" % (ident, gate, text))
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
