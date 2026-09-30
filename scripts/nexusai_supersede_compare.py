#!/usr/bin/env python3
"""Arbitration-supersedes-selection comparator: four arms against thresholds
written before the run.

    python scripts/nexusai_supersede_compare.py

Thresholds: reports/arbitration-supersedes-20260928/THRESHOLD_PREREGISTRATION.md

  INERT     on-control  vs the shipped posture (compiler-gaps on-a31only)
  SAFETY    on-fix      vs on-control   -- the shipped posture must not regress
  PROGRESS  off-fix     vs off-control  -- CDR-12 CLARIFIED -> CORRECT, nothing lost
  PROBES    VID-01, H1, H13 unchanged in every arm
"""
import io
import json
import os
import sys

ROOT = "reports/arbitration-supersedes-20260928"
SHIPPED_REFERENCE = "reports/compiler-gaps-20260927/on-a31only"
SUITES = ("golden", "holdout", "media")
PREDICTED_PROGRESS = {"CDR-12"}
PROBES = ("VID-01", "H1-MEDIA-WHICH-PLATES", "H13-HONESTY-NOPLATE")


def load(path_root):
    out = {}
    for suite in SUITES:
        path = os.path.join(path_root, suite, "results.json")
        if not os.path.exists(path):
            return None
        with io.open(path, encoding="utf-8") as handle:
            for row in json.load(handle):
                row["_suite"] = suite
                out[row["id"]] = row
    return out


def raw(arm, qid):
    for suite in SUITES:
        path = os.path.join(ROOT, arm, suite, "raw", qid + ".json")
        if os.path.exists(path):
            with io.open(path, encoding="utf-8") as handle:
                return json.load(handle)
    return {}


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


def judge(label, before_rows, after_rows, after_name, problems, explain, predicted=()):
    moved = movements(before_rows, after_rows)
    if not moved:
        print("   no question moved")
    for qid, before, after in moved:
        if before == "CORRECT":
            flag = "   <<< THRESHOLD: CORRECT lost"
            problems.append("%s %s CORRECT -> %s" % (label, qid, after))
        elif after == "WRONG":
            flag = "   <<< THRESHOLD: new WRONG"
            problems.append("%s %s %s -> WRONG" % (label, qid, before))
        elif after == "CORRECT" and qid in predicted:
            flag = "   predicted  PASS"
        else:
            flag = "   (must be explained)"
            explain.append((label, qid, before, after))
        print("   %-24s %-11s -> %-11s%s" % (qid, before, after, flag))
        print("        says: %s" % (after_rows[qid].get("answer_text") or "")[:160].replace("\n", " "))
    return moved


def main():
    arms = {name: load(os.path.join(ROOT, name)) for name in ("on-control", "on-fix", "off-control", "off-fix")}
    shipped = load(SHIPPED_REFERENCE)
    for name, rows in list(arms.items()) + [("shipped reference", shipped)]:
        if rows is None:
            print("MISSING ARM: %s" % name)
            return 1
    problems, explain = [], []

    print("=" * 100)
    print("ARBITRATION SUPERSEDES THE REGISTERED SELECTION - four arms")
    print("=" * 100)
    print("  %-20s %s" % ("shipped (on-a31only)", render(tally(shipped))))
    for name, rows in arms.items():
        print("  %-20s %s" % (name, render(tally(rows))))

    print("\n## INERT: on-control vs the shipped posture (switch off must change nothing)")
    inert = judge("INERT", shipped, arms["on-control"], "on-control", problems, explain)
    if inert:
        problems.append("INERT: the switch-off build does not reproduce the shipped posture (%d moved)" % len(inert))

    print("\n## SAFETY: on-fix vs on-control (ladder on)")
    judge("SAFETY", arms["on-control"], arms["on-fix"], "on-fix", problems, explain)

    print("\n## PROGRESS: off-fix vs off-control (ladder off, the compiler path)")
    judge("PROGRESS", arms["off-control"], arms["off-fix"], "off-fix", problems, explain, PREDICTED_PROGRESS)
    for qid in sorted(PREDICTED_PROGRESS):
        row = arms["off-fix"].get(qid, {})
        print("\n   %s off-control %s -> off-fix %s" % (qid, arms["off-control"].get(qid, {}).get("verdict"), row.get("verdict")))
        if row.get("verdict") != "CORRECT":
            problems.append("PROGRESS %s did not reach CORRECT (%s)" % (qid, row.get("verdict")))
        sop = (raw("off-fix", qid).get("planner") or {}).get("semantic_operation_planner") or {}
        print("   audit: binding_state=%s ir_arbitrated=%s selected=%s" % (
            sop.get("binding_state"), sop.get("ir_arbitrated"), (sop.get("selected_operation") or {}).get("operation_id")))

    print("\n## PROBES: plate-text and honesty probes unchanged in every arm")
    for qid in PROBES:
        verdicts = [shipped.get(qid, {}).get("verdict")] + [arms[n].get(qid, {}).get("verdict") for n in arms]
        same = len(set(verdicts)) == 1
        print("   %-24s %s%s" % (qid, " / ".join(str(v) for v in verdicts), "  PASS" if same else "   <<< THRESHOLD"))
        if not same:
            problems.append("PROBE %s changed: %s" % (qid, verdicts))

    print("\n" + "=" * 100)
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
    print("=" * 100)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
