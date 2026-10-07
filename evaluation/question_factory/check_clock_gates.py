#!/usr/bin/env python3
"""Run 11 gates T1 to T6, applied to an arm's saved results (reports/governed-sql-20261005/PREREGISTRATION.md, "Run 11 plan").

    python check_clock_gates.py --arm arm-lane-C-mm11 --baseline arm-lane-C-mm [--ask --questions questions-multimodal-clock.json]

The gates were written there before anything was measured; this only applies them. A gate the data cannot decide (no baseline arm,
no re-ask) is reported NOT ASSESSABLE and is never counted as a pass. The questions an arm was asked are judged by the factory against
keys on the case clock (a file made by `factory.py rekey`), so a verdict here already means "against the case-clock key".

    T1  every `night` question the lane answers equals its key exactly
    T2  every `month_count`, `date_range`, `earliest` and `latest` question the lane answers equals its key exactly
    T3  no other question changes verdict or text against the baseline arm (run 10 arm C)
    T4  no HTTP 5xx and no timeout, other than the existing-path 500 already recorded
    T5  every lane answer that reads or shows a time says which clock it used (needs --ask: the notes are not in the saved results)
    T6  no ANPR time answer counts a plate read that has no recorded time (an ANPR time answer the lane gave must equal its key,
        and the key leaves those reads out)
"""
import argparse
import io
import json
import os
import sys

import factory as f

HERE = os.path.dirname(os.path.abspath(__file__))
# The existing path's HTTP 500 on "Which plate number appears most often ..." (a group-cardinality guard), in every arm since run 1.
RECORDED_ERROR = "plate number appears most often"


def load(arm):
    with io.open(os.path.join(HERE, arm, "results.json"), encoding="utf-8") as handle:
        return json.load(handle)


def lane_state(row):
    """answered / abstained / declined, or None when the lane was not consulted (the existing path answered or failed)."""
    return f.lane_fields(row.get("lane")).get("state")


def gate_keys(rows, intents):
    """T1 and T2: every time answer the lane gave equals its key. Returns (status, time rows, answered by the lane, wrong ones)."""
    mine = [r for r in rows if r["intent"] in intents]
    answered = [r for r in mine if lane_state(r) == "answered"]
    wrong = [r for r in answered if r["verdict"] != "CORRECT"]
    return ("NOT ASSESSABLE" if not answered else "FAIL" if wrong else "PASS"), mine, answered, wrong


def gate_unchanged(rows, base):
    """T3: outside the time questions, nothing changes against the baseline arm. Returns (status, verdict changes, text-only changes)."""
    if base is None:
        return "NOT ASSESSABLE", [], []
    old = {r["id"]: r for r in base}
    verdicts, texts = [], []
    for r in rows:
        if r["intent"] in f.TIME_INTENTS or r["id"] not in old:
            continue
        was = old[r["id"]]
        if r["verdict"] != was["verdict"]:
            verdicts.append((r["id"], was["verdict"], r["verdict"]))
        elif r["text"] != was["text"]:
            texts.append(r["id"])
    return ("FAIL" if verdicts or texts else "PASS"), verdicts, texts


def gate_errors(rows, base):
    """T4: an ERROR row is allowed only when it is the recorded existing-path 500 (same question, or the same HTTP code in the baseline).
    A timeout (HTTP 0) is never allowed: a timeout is not a verdict and the question is asked again."""
    was = {r["id"]: r["why"] for r in (base or []) if r["verdict"] == "ERROR"}
    bad = []
    for r in rows:
        if r["verdict"] != "ERROR":
            continue
        recorded = RECORDED_ERROR in r["question"] or was.get(r["id"]) == r["why"]
        if r["why"] == "HTTP 0" or not recorded:
            bad.append((r["id"], r["why"]))
    return ("FAIL" if bad else "PASS"), bad


def gate_clock_stated(rows, notes_of):
    """T5: every lane answer to a time question says which clock it used. notes_of(row) asks again and returns (lane state, notes).
    A question the lane no longer answers when asked again is listed apart; it is not counted either way."""
    checked, missing, moved = [], [], []
    for r in rows:
        if r["intent"] not in f.TIME_INTENTS or lane_state(r) != "answered":
            continue
        state, notes = notes_of(r)
        if state != "answered":
            moved.append((r["id"], state))
            continue
        checked.append(r["id"])
        if not any("case clock" in n.lower() for n in notes):
            missing.append(r["id"])
    return ("FAIL" if missing else "NOT ASSESSABLE" if not checked else "PASS"), checked, missing, moved


def gate_no_time_rows(rows):
    """T6: an ANPR time answer the lane gave equals its key (which leaves out reads with no recorded time)."""
    anpr = [r for r in rows if r["family"] == "anpr" and r["intent"] in f.TIME_INTENTS and lane_state(r) == "answered"]
    wrong = [r for r in anpr if r["verdict"] != "CORRECT"]
    return ("NOT ASSESSABLE" if not anpr else "FAIL" if wrong else "PASS"), anpr, wrong


def report(rows, base, notes_of):
    out = []
    t1, mine, answered, wrong = gate_keys(rows, ("night",))
    out.append(("T1", t1, "night questions %d, answered by the lane %d, wrong %s" % (len(mine), len(answered), [r["id"] for r in wrong] or "none")))
    t2, mine, answered, wrong = gate_keys(rows, ("month_count", "date_range", "earliest", "latest"))
    out.append(("T2", t2, "month/date/earliest/latest questions %d, answered by the lane %d, wrong %s" % (len(mine), len(answered), [r["id"] for r in wrong] or "none")))
    t3, verdicts, texts = gate_unchanged(rows, base)
    out.append(("T3", t3, "verdict changes %s; text-only changes %s" % (verdicts or "none", texts or "none") if base is not None else "no baseline arm given"))
    t4, bad = gate_errors(rows, base)
    out.append(("T4", t4, "errors other than the recorded 500: %s" % (bad or "none")))
    if notes_of is None:
        out.append(("T5", "NOT ASSESSABLE", "run with --ask to read the notes of each lane-answered time question"))
    else:
        t5, checked, missing, moved = gate_clock_stated(rows, notes_of)
        out.append(("T5", t5, "asked again %d, no clock stated in %s, no longer answered by the lane %s" % (len(checked), missing or "none", moved or "none")))
    t6, anpr, wrong = gate_no_time_rows(rows)
    out.append(("T6", t6, "ANPR time answers by the lane %d, wrong %s" % (len(anpr), [r["id"] for r in wrong] or "none")))
    return out


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--arm", required=True)
    parser.add_argument("--baseline", help="arm to read T3 and the recorded errors against (run 10 arm C on the multimodal set)")
    parser.add_argument("--ask", action="store_true", help="ask each lane-answered time question again to read its notes (T5); needs FORENSIC_RECORDS_API_KEY")
    parser.add_argument("--questions", help="the question file the arm was run with (needed with --ask for each question's collection)")
    args = parser.parse_args()
    rows = load(args.arm)
    base = load(args.baseline) if args.baseline else None
    notes_of = None
    if args.ask:
        with io.open(args.questions, encoding="utf-8") as handle:
            items = {i["id"]: i for i in json.load(handle)["questions"]}
        url = os.environ.get("NEXUSAI_PROBE_URL", "http://127.0.0.1:8091/query/hybrid")
        key = os.environ.get("FORENSIC_RECORDS_API_KEY", "")

        def notes_of(row):
            item = items[row["id"]]
            status, blob, _, headers = f.ask(url, key, item["collection"], item["question"])
            enterprise = blob.get("enterprise") if isinstance(blob, dict) and isinstance(blob.get("enterprise"), dict) else {}
            notes = [n for n in (enterprise.get("limitations") or []) if isinstance(n, str)]
            return f.lane_fields(headers.get("x-governed-sql")).get("state"), notes

    print("arm %s: %d questions%s" % (args.arm, len(rows), "" if base is None else "; baseline %s: %d questions" % (args.baseline, len(base))))
    results = report(rows, base, notes_of)
    for gate, status, detail in results:
        print("%-3s %-15s %s" % (gate, status, detail))
    # Exit 0 only when no gate FAILED; NOT ASSESSABLE is reported, never hidden, but is not a failure of the lane.
    return 1 if any(status == "FAIL" for _, status, _ in results) else 0


if __name__ == "__main__":
    sys.exit(main())
