#!/usr/bin/env python3
"""A1a comparator: two arms against thresholds written before the run.

    python scripts/nexusai_constraint_compare.py

Threshold source: reports/compiler-first-20260927/THRESHOLD_PREREGISTRATION.md
The predicted corpus movement is ZERO -- the census captured 0 of 103 -- so any
movement at all fires.
"""
import io
import json
import os
import sys

ROOT = "reports/compiler-first-20260927"
ARMS = ("control", "compiler")
SUITES = ("golden", "holdout", "media")

# 3.1: arm A must reproduce the shipped posture exactly, or everything
# downstream is void.
SHIPPED = {"CORRECT": 67, "CLARIFIED": 28, "WRONG": 4,
           "NOT_STATED": 2, "MANUAL": 1, "ROWCOUNT_ONLY": 1}


def load(arm, suite):
    path = os.path.join(ROOT, arm, suite, "results.json")
    if not os.path.exists(path):
        return None
    with io.open(path, encoding="utf-8") as handle:
        return {row["id"]: row for row in json.load(handle)}


def tally(rows):
    out = {}
    for row in rows.values():
        out[row["verdict"]] = out.get(row["verdict"], 0) + 1
    return out


def render(counts):
    return " | ".join("%s %d" % kv for kv in sorted(counts.items(), key=lambda kv: -kv[1]))


def main():
    problems = []
    print("=" * 92)
    print("A2 COMPILER-FIRST - two arms, thresholds written before the run")
    print("=" * 92)

    totals = {arm: {} for arm in ARMS}
    for suite in SUITES:
        control, guard = load("control", suite), load("compiler", suite)
        if control is None or guard is None:
            print("\n## %s  MISSING ARM" % suite.upper())
            problems.append("%s: an arm is missing" % suite)
            continue
        for arm, rows in (("control", control), ("compiler", guard)):
            for verdict, count in tally(rows).items():
                totals[arm][verdict] = totals[arm].get(verdict, 0) + count

        print("")
        print("## %s (%d questions)" % (suite.upper(), len(control)))
        print("   control  %s" % render(tally(control)))
        print("   guard    %s" % render(tally(guard)))

        moved = [(qid, control[qid]["verdict"], guard[qid]["verdict"])
                 for qid in sorted(control)
                 if qid in guard and control[qid]["verdict"] != guard[qid]["verdict"]]
        if not moved:
            print("   no question moved   <- this is the PASS for this suite")
        for qid, before, after in moved:
            # 3.2: the census captured 0 of 103, so ANY movement means the
            # census was wrong. There is no tolerated movement in this arm.
            print("     %-28s %-11s -> %-11s   <<< THRESHOLD: census predicted zero"
                  % (qid, before, after))
            problems.append("%s/%s %s -> %s" % (suite, qid, before, after))

    print("")
    print("## 3.1 ARM A REPRODUCES THE SHIPPED POSTURE")
    print("   expected %s" % render(SHIPPED))
    print("   measured %s" % render(totals["control"]))
    if totals["control"] != SHIPPED:
        for verdict in sorted(set(SHIPPED) | set(totals["control"])):
            want, got = SHIPPED.get(verdict, 0), totals["control"].get(verdict, 0)
            if want != got:
                print("     %-14s expected %d, measured %d   <<< VOID" % (verdict, want, got))
        problems.append("arm A did not reproduce the shipped posture - the run is VOID")
    else:
        print("   reproduced exactly  PASS")

    print("")
    print("## GUARD ARM TOTALS")
    print("   %s" % render(totals["compiler"]))
    if totals["compiler"].get("WRONG", 0) > SHIPPED["WRONG"]:
        problems.append("guard arm introduced a new WRONG")

    print("")
    print("=" * 92)
    if problems:
        print("THRESHOLD FIRED - %d item(s). Revert the whole arm." % len(problems))
        for item in problems:
            print("  * %s" % item)
    else:
        print("NO CORPUS THRESHOLD FIRED.")
        print("The corpus is a SAFETY instrument only - it cannot prove the guard works.")
        print("Proof of function is the ad-hoc probe (threshold 4), scored separately.")
    print("=" * 92)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
