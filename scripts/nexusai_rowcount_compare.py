#!/usr/bin/env python3
"""S1 row-count headline guard comparator, against thresholds written before the run.

    python scripts/nexusai_rowcount_compare.py

Thresholds: reports/rowcount-headline-20260928/THRESHOLD_PREREGISTRATION.md
"""
import io
import json
import os
import re
import sys

ROOT = "reports/rowcount-headline-20260928"
SHIPPED = "reports/guards-20260928/shipped"
REL = "reports/relational-conditions-20260928"
SUITES = ("golden", "holdout", "media")
ABC = "Show me everything you have about plate ABC-123"
SUSPICIOUS = "Summarise the suspicious activity in this case"
TARGETS = {ABC, SUSPICIOUS}


def jload(path):
    with io.open(path, encoding="utf-8") as handle:
        return json.load(handle)


def corpus(root):
    out = {}
    for suite in SUITES:
        path = os.path.join(root, suite, "results.json")
        if not os.path.exists(path):
            return None
        for row in jload(path):
            out[row["id"]] = row
    return out


def moved(before, after):
    return [(q, before[q]["verdict"], after[q]["verdict"]) for q in sorted(before)
            if q in after and before[q]["verdict"] != after[q]["verdict"]]


def main():
    problems = []
    arms = {n: corpus(os.path.join(ROOT, n)) for n in ("off", "on")}
    shipped = corpus(SHIPPED)
    for name, rows in list(arms.items()) + [("shipped", shipped)]:
        if rows is None:
            print("MISSING: %s" % name)
            return 1
    print("=" * 96)
    print("S1 ROW-COUNT HEADLINE GUARD")
    print("=" * 96)

    print("\n## CORPUS")
    inert = moved(shipped, arms["off"])
    print("  off vs deployed build: %d moved %s" % (len(inert), inert))
    if inert:
        problems.append("switch-off build does not reproduce the deployed build: %s" % inert)
    effect = moved(arms["off"], arms["on"])
    print("  on vs off: %d moved %s" % (len(effect), effect))
    if effect:
        problems.append("corpus moved: %s" % effect)

    print("\n## EVERYDAY 14")
    off14 = {r["question"]: r for r in jload(os.path.join(ROOT, "off", "adhoc", "adhoc_results.json"))}
    on14 = {r["question"]: r for r in jload(os.path.join(ROOT, "on", "adhoc", "adhoc_results.json"))}
    for question in off14:
        before, after = off14[question]["text"], on14.get(question, {}).get("text", "")
        changed = before != after
        flag = ""
        if changed and question not in TARGETS:
            flag = "   <<< THRESHOLD: unrelated question changed"
            problems.append("everyday: %r changed" % question)
        print("  %-8s %s%s" % ("CHANGED" if changed else "same", question, flag))
        if changed:
            print("      off: %s" % before[:150].replace("\n", " "))
            print("      on : %s" % after[:150].replace("\n", " "))
    abc = on14.get(ABC, {}).get("text", "")
    if "218" not in abc or "ABC-123" not in abc or "1 ANPR sighting" in abc:
        problems.append("ABC-123 headline does not state 218 for ABC-123: %r" % abc[:160])
    sus = on14.get(SUSPICIOUS, {}).get("text", "")
    if re.search(r"\b\d[\d,]*\s+records? matched", sus):
        problems.append("suspicious summary still states a row count: %r" % sus[:160])

    print("\n## DEMO PRE-FLIGHT (38)")
    offp = {r["question"]: r for r in jload(os.path.join(ROOT, "off", "preflight", "results.json"))}
    onp = {r["question"]: r for r in jload(os.path.join(ROOT, "on", "preflight", "results.json"))}
    diffs = [q for q in offp if (offp[q]["pass"], offp[q]["text"]) != (onp.get(q, {}).get("pass"), onp.get(q, {}).get("text"))]
    print("  off passed %d, on passed %d, differing %d" % (sum(r["pass"] for r in offp.values()), sum(r["pass"] for r in onp.values()), len(diffs)))
    for q in diffs:
        print("   <<< THRESHOLD: %s\n      off: %s\n      on : %s" % (q, offp[q]["text"][:120], onp[q]["text"][:120]))
        problems.append("pre-flight changed: %s" % q)

    print("\n## RELATIONAL PROBE (16)")
    offr = {r["id"]: r for r in jload(os.path.join(REL, "rowcount-off", "results.json"))}
    onr = {r["id"]: r for r in jload(os.path.join(REL, "rowcount-on", "results.json"))}
    rdiff = [q for q in offr if (offr[q]["state"], offr[q].get("reason_code")) != (onr[q]["state"], onr[q].get("reason_code"))]
    print("  differing: %d %s" % (len(rdiff), rdiff))
    if rdiff:
        problems.append("relational probe changed: %s" % rdiff)

    print("\n" + "=" * 96)
    if problems:
        print("THRESHOLD FIRED - %d item(s)" % len(problems))
        for item in problems:
            print("  * %s" % item)
    else:
        print("NO THRESHOLD FIRED.")
    print("=" * 96)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
