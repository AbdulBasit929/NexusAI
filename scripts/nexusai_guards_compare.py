#!/usr/bin/env python3
"""A1c + A1d comparator: four ladder-on arms plus the relational probe, against
thresholds written before the run.

    python scripts/nexusai_guards_compare.py

Thresholds:
  reports/relational-conditions-20260928/THRESHOLD_PREREGISTRATION.md
  reports/location-obligation-20260928/THRESHOLD_PREREGISTRATION.md

  corpus   a1c vs control        0 moved
           a1d vs control        H11 WRONG -> not WRONG, nothing else moves, NEG-02 stays CORRECT
           both vs control       exactly the union of the above (the shipped configuration)
  probe    control               R1, R2 reproduce the recorded confident-wrong answers
           a1c / both            no R question ANSWERED from a plan computing no relationship
           a1c / both            every N question identical to control in state and text
"""
import io
import json
import os
import sys

ROOT = "reports/guards-20260928"
PROBE_ROOT = "reports/relational-conditions-20260928"
SUITES = ("golden", "holdout", "media")
RELATIONAL_TEMPLATES = {"cross_family_correlation", "relationship_network", "tower_cdr_join",
                        "anpr_co_travel", "multi_cdr_comparison", "subscriber_device_links"}


def load(arm):
    out = {}
    for suite in SUITES:
        path = os.path.join(ROOT, arm, suite, "results.json")
        if not os.path.exists(path):
            return None
        with io.open(path, encoding="utf-8") as handle:
            for row in json.load(handle):
                out[row["id"]] = row
    return out


def load_probe(arm):
    path = os.path.join(PROBE_ROOT, "guards-" + arm, "results.json")
    if not os.path.exists(path):
        return None
    with io.open(path, encoding="utf-8") as handle:
        return {row["id"]: row for row in json.load(handle)}


def tally(rows):
    out = {}
    for row in rows.values():
        out[row["verdict"]] = out.get(row["verdict"], 0) + 1
    return " | ".join("%s %d" % kv for kv in sorted(out.items(), key=lambda kv: -kv[1]))


def moved(before, after):
    return {qid: (before[qid]["verdict"], after[qid]["verdict"])
            for qid in sorted(before) if qid in after and before[qid]["verdict"] != after[qid]["verdict"]}


def withheld(row):
    """A withhold carries its refusal sentence in `direct_answer`, so the probe's
    extractor labels it ANSWERED. Read the contract instead: a clarification
    reason code, or a withheld/clarification route, means nothing was stated.
    Instrument defect found 2026-09-28 on this comparator's first run -- it
    scored all six correct withholds as answers."""
    route = row.get("route") or []
    return bool(row.get("reason_code")) or "verified_only_withheld" in route or "clarification" in route


def computes_relationship(row):
    plan = row.get("plan") or {}
    if row.get("template") in RELATIONAL_TEMPLATES:
        return True
    having = plan.get("having") or []
    return bool(plan.get("group_fields")) and any(h.get("op") in ("GT", "GTE") for h in having) \
        and any(op in ("COUNT", "COUNT_DISTINCT") for op in (plan.get("measures") or []))


def main():
    arms = {name: load(name) for name in ("control", "a1c", "a1d", "both")}
    probes = {name: load_probe(name) for name in ("control", "a1c", "a1d", "both")}
    for name in arms:
        if arms[name] is None or probes[name] is None:
            print("MISSING ARM OR PROBE: %s" % name)
            return 1
    problems = []
    print("=" * 100)
    print("A1c RELATIONAL CONDITIONS + A1d LOCATION OBLIGATION - four arms, ladder ON")
    print("=" * 100)
    for name, rows in arms.items():
        print("  %-8s %s" % (name, tally(rows)))

    control = arms["control"]
    print("\n## CORPUS")
    expectations = {"a1c": set(), "a1d": {"H11-HONESTY-CDRLOC"}, "both": {"H11-HONESTY-CDRLOC"}}
    for name, expected in expectations.items():
        diff = moved(control, arms[name])
        print("  %s vs control: %d moved" % (name, len(diff)))
        for qid, (before, after) in diff.items():
            flag = ""
            if before == "CORRECT":
                flag = "   <<< THRESHOLD: CORRECT lost"
                problems.append("%s: %s CORRECT -> %s" % (name, qid, after))
            elif after == "WRONG":
                flag = "   <<< THRESHOLD: new WRONG"
                problems.append("%s: %s %s -> WRONG" % (name, qid, before))
            elif qid not in expected:
                flag = "   <<< THRESHOLD: unpredicted movement"
                problems.append("%s: %s moved %s -> %s, not predicted" % (name, qid, before, after))
            else:
                flag = "   predicted  PASS"
            print("     %-22s %-10s -> %-10s%s" % (qid, before, after, flag))
            print("        says: %s" % (arms[name][qid].get("answer_text") or "")[:150].replace("\n", " "))
        for qid in expected:
            if arms[name].get(qid, {}).get("verdict") == "WRONG":
                problems.append("%s: %s is still WRONG" % (name, qid))
                print("     %s still WRONG   <<< THRESHOLD" % qid)
        if arms[name].get("NEG-02", {}).get("verdict") != "CORRECT":
            problems.append("%s: NEG-02 not CORRECT" % name)

    print("\n## RELATIONAL PROBE")
    for qid in ("R1", "R2"):
        row = probes["control"][qid]
        reproduced = row["state"].startswith("ANSWERED") and not withheld(row) and not computes_relationship(row)
        print("  control %s %-19s %s%s" % (qid, row["state"], row["text"][:80], "" if reproduced else "   <<< VOID"))
        if not reproduced:
            problems.append("VOID: control %s did not reproduce the recorded confident-wrong answer" % qid)
    for name in ("a1c", "both"):
        print("  -- %s" % name)
        for qid, row in sorted(probes[name].items()):
            base = probes["control"][qid]
            if qid.startswith("R"):
                bad = row["state"].startswith("ANSWERED") and not withheld(row) and not computes_relationship(row)
                flag = "   <<< THRESHOLD: answered without a relationship" if bad else "  PASS"
                if bad:
                    problems.append("%s: %s answered without computing a relationship" % (name, qid))
            else:
                same = row["state"] == base["state"] and row["text"] == base["text"]
                flag = "  PASS (unchanged)" if same else "   <<< THRESHOLD: near-miss changed"
                if not same:
                    problems.append("%s: near-miss %s changed %s -> %s" % (name, qid, base["state"], row["state"]))
            state = "WITHHELD" if withheld(row) else row["state"]
            base_state = "WITHHELD" if withheld(base) else base["state"]
            print("     %-3s %-19s -> %-19s reason=%s%s" % (qid, base_state, state, row.get("reason_code"), flag))
    print("  -- a1d (relational switch off: the probe must equal control)")
    for qid, row in sorted(probes["a1d"].items()):
        base = probes["control"][qid]
        if row["state"] != base["state"]:
            print("     %-3s %s -> %s   (explain)" % (qid, base["state"], row["state"]))

    print("\n" + "=" * 100)
    if problems:
        print("THRESHOLD FIRED - %d item(s)" % len(problems))
        for item in problems:
            print("  * %s" % item)
    else:
        print("NO THRESHOLD FIRED.")
    print("=" * 100)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
