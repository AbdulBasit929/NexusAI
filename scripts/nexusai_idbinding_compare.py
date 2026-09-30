#!/usr/bin/env python3
"""A3.3 comparator: three arms against thresholds written before the run.

    python scripts/nexusai_idbinding_compare.py

Thresholds: reports/identifier-binding-20260927/THRESHOLD_PREREGISTRATION.md
"""
import io
import json
import os
import sys

ROOT = "reports/identifier-binding-20260927"
SUITES = ("golden", "holdout", "media")
SHIPPED = {"CORRECT": 67, "CLARIFIED": 28, "WRONG": 4,
           "NOT_STATED": 2, "MANUAL": 1, "ROWCOUNT_ONLY": 1}
P95_GATE_MS = 15000
PREDICTED_C = {"TWR-02"}


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


def p95(rows):
    values = sorted(int(r.get("latency_ms") or 0) for r in rows.values())
    if not values:
        return 0
    return values[min(len(values) - 1, int(round(0.95 * (len(values) - 1))))]


def main():
    problems, explain = [], []
    arms = {arm: {suite: load(arm, suite) for suite in SUITES} for arm in ("control", "binding", "both")}
    for arm, suites in arms.items():
        for suite, rows in suites.items():
            if rows is None:
                print("MISSING %s/%s" % (arm, suite))
                return 1

    print("=" * 96)
    print("A3.3 IDENTIFIER BINDING + DETERMINISTIC ARBITRATION - three arms")
    print("=" * 96)

    totals = {}
    for arm in arms:
        merged = {}
        for suite in SUITES:
            for verdict, count in tally(arms[arm][suite]).items():
                merged[verdict] = merged.get(verdict, 0) + count
        totals[arm] = merged

    print("\n## 4.1 ARM A REPRODUCES THE SHIPPED POSTURE")
    print("   expected %s" % render(SHIPPED))
    print("   measured %s" % render(totals["control"]))
    if totals["control"] != SHIPPED:
        problems.append("arm A did not reproduce the shipped posture - VOID")
        print("   <<< VOID")
    else:
        print("   reproduced exactly  PASS")

    for arm in ("binding", "both"):
        print("\n## ARM %s vs CONTROL   totals: %s" % (arm.upper(), render(totals[arm])))
        moved = 0
        for suite in SUITES:
            control, treated = arms["control"][suite], arms[arm][suite]
            for qid in sorted(control):
                before, after = control[qid]["verdict"], treated[qid]["verdict"]
                if before == after:
                    continue
                moved += 1
                flag = ""
                if before == "CORRECT":
                    flag = "   <<< THRESHOLD: CORRECT lost"
                    problems.append("%s %s/%s CORRECT -> %s" % (arm, suite, qid, after))
                elif after == "WRONG":
                    flag = "   <<< THRESHOLD: new WRONG"
                    problems.append("%s %s/%s %s -> WRONG" % (arm, suite, qid, before))
                elif arm == "binding":
                    flag = "   <<< arm B predicted ZERO movement"
                    problems.append("binding arm moved %s/%s" % (suite, qid))
                elif qid not in PREDICTED_C:
                    flag = "   (unpredicted - must be explained)"
                    explain.append((arm, suite, qid, before, after))
                print("   %-8s %-26s %-12s -> %-12s%s" % (suite, qid, before, after, flag))
                print("            says: %s" % (treated[qid].get("answer_text") or "")[:150].replace("\n", " "))
        if moved == 0:
            print("   no question moved")

    print("\n## 4.2 LATENCY (golden p95, gate %d ms)" % P95_GATE_MS)
    for arm in arms:
        value = p95(arms[arm]["golden"])
        mark = "PASS" if value <= P95_GATE_MS else "<<< THRESHOLD"
        print("   %-8s p95 %6d ms   %s" % (arm, value, mark))
        if value > P95_GATE_MS:
            problems.append("%s p95 %d ms exceeds the gate" % (arm, value))

    print("\n## NAMED: TWR-02 in arm C")
    row = arms["both"]["golden"].get("TWR-02", {})
    text = row.get("answer_text") or ""
    print("   verdict %s" % row.get("verdict"))
    print("   says    %s" % text[:220].replace("\n", " "))
    if row.get("verdict") != "CORRECT":
        problems.append("TWR-02 did not become CORRECT in arm C - routing does not reach it")

    print("\n" + "=" * 96)
    if problems:
        print("THRESHOLD FIRED - %d item(s)" % len(problems))
        for item in problems:
            print("  * %s" % item)
    else:
        print("NO THRESHOLD FIRED.")
    if explain:
        print("UNPREDICTED MOVEMENT TO EXPLAIN BEFORE SHIPPING: %d" % len(explain))
        for arm, suite, qid, before, after in explain:
            print("  - %s %s/%s %s -> %s" % (arm, suite, qid, before, after))
    print("=" * 96)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
