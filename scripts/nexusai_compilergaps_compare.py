#!/usr/bin/env python3
"""A3.1 + A3.2 comparator: four arms against thresholds written before the run.

    python scripts/nexusai_compilergaps_compare.py

Thresholds: reports/compiler-gaps-20260927/THRESHOLD_PREREGISTRATION.md

  SAFETY    on-fix  vs on-control   -- the shipped posture must not regress
  PROGRESS  off-fix vs off-baseline -- the compiler path should improve
"""
import io
import json
import os
import sys

ROOT = "reports/compiler-gaps-20260927"
SUITES = ("golden", "holdout", "media")
SHIPPED = {"CORRECT": 67, "CLARIFIED": 28, "WRONG": 4,
           "NOT_STATED": 2, "MANUAL": 1, "ROWCOUNT_ONLY": 1}
PREDICTED_PROGRESS = {"CDR-12", "CDR-16"}


def load(arm):
    out = {}
    for suite in SUITES:
        path = os.path.join(ROOT, arm, suite, "results.json")
        if not os.path.exists(path):
            return None
        with io.open(path, encoding="utf-8") as handle:
            for row in json.load(handle):
                row["_suite"] = suite
                out[row["id"]] = row
    return out


def tally(rows):
    out = {}
    for row in rows.values():
        out[row["verdict"]] = out.get(row["verdict"], 0) + 1
    return out


def render(counts):
    return " | ".join("%s %d" % kv for kv in sorted(counts.items(), key=lambda kv: -kv[1]))


def movements(before, after):
    return [(qid, before[qid]["verdict"], after[qid]["verdict"])
            for qid in sorted(before) if qid in after and before[qid]["verdict"] != after[qid]["verdict"]]


def main():
    arms = {name: load(name) for name in ("on-control", "on-fix", "ladderoff-baseline", "off-fix")}
    for name, rows in arms.items():
        if rows is None:
            print("MISSING ARM: %s" % name)
            return 1
    problems, explain = [], []

    print("=" * 96)
    print("A3.1 + A3.2 COMPILER GAPS - four arms")
    print("=" * 96)
    for name, rows in arms.items():
        print("  %-20s %s" % (name, render(tally(rows))))

    print("\n## on-control reproduces the shipped posture")
    if tally(arms["on-control"]) != SHIPPED:
        problems.append("on-control did not reproduce the shipped posture - VOID")
        print("   <<< VOID")
    else:
        print("   reproduced exactly  PASS")

    print("\n## SAFETY: on-fix vs on-control (shipped posture)")
    safety = movements(arms["on-control"], arms["on-fix"])
    if not safety:
        print("   no question moved  PASS")
    for qid, before, after in safety:
        flag = ""
        if before == "CORRECT":
            flag = "   <<< THRESHOLD: CORRECT lost"
            problems.append("SAFETY %s CORRECT -> %s" % (qid, after))
        elif after == "WRONG":
            flag = "   <<< THRESHOLD: new WRONG"
            problems.append("SAFETY %s %s -> WRONG" % (qid, before))
        else:
            flag = "   (must be explained)"
            explain.append(("on", qid, before, after))
        print("   %-24s %-11s -> %-11s%s" % (qid, before, after, flag))
        print("        says: %s" % (arms["on-fix"][qid].get("answer_text") or "")[:140].replace("\n", " "))

    print("\n## PROGRESS: off-fix vs ladderoff-baseline (compiler path)")
    progress = movements(arms["ladderoff-baseline"], arms["off-fix"])
    if not progress:
        print("   no question moved  -- the fixes are not reached on the compiler path")
    for qid, before, after in progress:
        flag = ""
        if before == "CORRECT":
            flag = "   <<< compiler-path regression"
            problems.append("PROGRESS %s CORRECT -> %s" % (qid, after))
        elif after == "WRONG":
            flag = "   <<< new WRONG on the compiler path"
            problems.append("PROGRESS %s %s -> WRONG" % (qid, before))
        elif after == "CORRECT" and qid in PREDICTED_PROGRESS:
            flag = "   predicted  PASS"
        else:
            flag = "   (must be explained)"
            explain.append(("off", qid, before, after))
        print("   %-24s %-11s -> %-11s%s" % (qid, before, after, flag))
        print("        says: %s" % (arms["off-fix"][qid].get("answer_text") or "")[:140].replace("\n", " "))

    for qid in sorted(PREDICTED_PROGRESS):
        row = arms["off-fix"].get(qid, {})
        print("\n   %s in off-fix: %s" % (qid, row.get("verdict")))

    print("\n" + "=" * 96)
    if problems:
        print("THRESHOLD FIRED - %d item(s)" % len(problems))
        for item in problems:
            print("  * %s" % item)
    else:
        print("NO THRESHOLD FIRED.")
    if explain:
        print("MOVEMENT TO EXPLAIN BEFORE SHIPPING: %d" % len(explain))
        for side, qid, before, after in explain:
            print("  - [%s] %s %s -> %s" % (side, qid, before, after))
    print("=" * 96)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
