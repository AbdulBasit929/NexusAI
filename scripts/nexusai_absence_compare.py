#!/usr/bin/env python3
"""S2 + S3 comparator, against thresholds written before the run.

    python scripts/nexusai_absence_compare.py

Thresholds: reports/absence-text-20260928/THRESHOLD_PREREGISTRATION.md
"""
import io
import json
import os
import sys

ROOT = "reports/absence-text-20260928"
PRIOR = "reports/derived-labels-20260928/on"
REL = "reports/relational-conditions-20260928"
SUITES = ("golden", "holdout", "media")


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


def raw(arm, qid):
    for suite in SUITES:
        path = os.path.join(ROOT, arm, suite, "raw", qid + ".json")
        if os.path.exists(path):
            return jload(path)
    return {}


def main():
    problems = []
    off, on, prior = corpus(os.path.join(ROOT, "off")), corpus(os.path.join(ROOT, "on")), corpus(PRIOR)
    if not off or not on or not prior:
        print("MISSING ARM")
        return 1
    print("=" * 96)
    print("S2 + S3 ABSENCE STATEMENTS")
    print("=" * 96)
    inert = [q for q in prior if prior[q]["verdict"] != off[q]["verdict"]]
    print("off vs prior deployed build: %d moved %s" % (len(inert), inert))
    if inert:
        problems.append("switch-off build does not reproduce the deployed build: %s" % inert)
    moved = [(q, off[q]["verdict"], on[q]["verdict"]) for q in off if off[q]["verdict"] != on[q]["verdict"]]
    print("on vs off: %s" % moved)
    for q, before, after in moved:
        if q != "P2-PROVENANCE-AUDIO-MIX":
            problems.append("unpredicted movement %s %s -> %s" % (q, before, after))
    p2 = on.get("P2-PROVENANCE-AUDIO-MIX", {})
    p2code = (raw("on", "P2-PROVENANCE-AUDIO-MIX").get("clarification") or {}).get("reason_code")
    print("\nP2 on: %s  reason=%s\n   %s" % (p2.get("verdict"), p2code, (p2.get("answer_text") or "")[:170]))
    if p2.get("verdict") == "WRONG" or p2code != "text_search_not_absence":
        problems.append("P2 not withheld as text_search_not_absence")
    for q in ("DOC-06",):
        if (off[q]["verdict"], off[q].get("answer_text")) != (on[q]["verdict"], on[q].get("answer_text")):
            problems.append("%s changed" % q)
        print("%s on: %s | %s" % (q, on[q]["verdict"], (on[q].get("answer_text") or "")[:100]))

    print("\n## RELATIONAL 16")
    offr = {r["id"]: r for r in jload(os.path.join(REL, "absence-off", "results.json"))}
    onr = {r["id"]: r for r in jload(os.path.join(REL, "absence-on", "results.json"))}
    r5 = onr["R5"]["text"]
    print("R5 off: %s\nR5 on : %s" % (offr["R5"]["text"][:140], r5[:140]))
    if "were found" in r5 or "Which exact MSISDN" not in r5:
        problems.append("R5 does not state its clarification question")
    others = [q for q in offr if q != "R5" and (offr[q]["state"], offr[q]["text"]) != (onr[q]["state"], onr[q]["text"])]
    print("other relational changed: %s" % others)
    if others:
        problems.append("relational changed: %s" % others)

    print("\n## PRE-FLIGHT + EVERYDAY")
    offp = {r["question"]: (r["pass"], r["text"]) for r in jload(os.path.join(ROOT, "off", "preflight", "results.json"))}
    onp = {r["question"]: (r["pass"], r["text"]) for r in jload(os.path.join(ROOT, "on", "preflight", "results.json"))}
    dp = [q for q in offp if offp[q] != onp.get(q)]
    off14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "off", "adhoc", "adhoc_results.json"))}
    on14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "on", "adhoc", "adhoc_results.json"))}
    d14 = [q for q in off14 if off14[q] != on14.get(q)]
    print("pre-flight changed: %s   everyday changed: %s" % (dp, d14))
    if dp:
        problems.append("pre-flight changed: %s" % dp)
    if d14:
        problems.append("everyday changed: %s" % d14)

    print("\n" + "=" * 96)
    print("THRESHOLD FIRED - %d item(s)" % len(problems) if problems else "NO THRESHOLD FIRED.")
    for item in problems:
        print("  * %s" % item)
    print("=" * 96)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
