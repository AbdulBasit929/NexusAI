#!/usr/bin/env python3
"""U2b search-only plate lookup comparator, against thresholds written before the run.

    python scripts/nexusai_platesearch_compare.py

Thresholds: reports/plate-read-search-20260928/THRESHOLD_PREREGISTRATION.md
"""
import io
import json
import os
import sys

ROOT = "reports/plate-read-search-20260928"
PRIOR = "reports/evidence-search-first-20260928/on"
REL = "reports/relational-conditions-20260928"
SUITES = ("golden", "holdout", "media")
PREFLIGHT_FLIPS = {"Which image shows plate MN1367?": "image-test-plate-test_plate.jpg",
                   "Find plate LEB15491 in the images": "image-positive.JPG"}


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


def main():
    problems = []
    off, on, prior = corpus(os.path.join(ROOT, "off")), corpus(os.path.join(ROOT, "on")), corpus(PRIOR)
    if not off or not on or not prior:
        print("MISSING ARM")
        return 1
    print("=" * 96)
    print("U2b SEARCH-ONLY PLATE LOOKUP")
    print("=" * 96)
    inert = [q for q in prior if prior[q]["verdict"] != off[q]["verdict"]]
    print("off vs prior deployed build: %d moved %s" % (len(inert), inert))
    if inert:
        problems.append("switch-off build does not reproduce the deployed build: %s" % inert)
    moved = [(q, off[q]["verdict"], on[q]["verdict"]) for q in off if off[q]["verdict"] != on[q]["verdict"]]
    print("on vs off: %s" % moved)
    if moved:
        problems.append("corpus moved: %s" % moved)
    for probe in ("H1-MEDIA-WHICH-PLATES", "VID-01"):
        if (off[probe]["verdict"], off[probe].get("answer_text")) != (on[probe]["verdict"], on[probe].get("answer_text")):
            problems.append("privacy probe %s changed" % probe)

    print("\n## PLATE PROBE")
    for arm in ("off", "on"):
        rows = jload(os.path.join(ROOT, arm, "results.json"))
        print("  -- %s: %d of %d pass" % (arm, sum(r["pass"] for r in rows), len(rows)))
        for r in rows:
            print("     %s %s %-8s %s\n          %s%s" % ("PASS" if r["pass"] else "FAIL", r["id"], r["kind"], r["question"],
                                                   r["text"][:150], ("   LEAKED %s" % r["leaked"]) if r["leaked"] else ""))
            if r["leaked"]:
                problems.append("%s arm %s disclosed plates %s" % (arm, r["id"], r["leaked"]))
            if arm == "on" and not r["pass"]:
                problems.append("on arm %s failed" % r["id"])

    print("\n## PRE-FLIGHT")
    offp = {r["question"]: r for r in jload(os.path.join(ROOT, "off", "preflight", "results.json"))}
    onp = {r["question"]: r for r in jload(os.path.join(ROOT, "on", "preflight", "results.json"))}
    for q in offp:
        a, b = offp[q], onp[q]
        if q in PREFLIGHT_FLIPS:
            ok = b["pass"] and PREFLIGHT_FLIPS[q] in b["text"]
            print("  %s %s\n      on: %s" % ("PASS" if ok else "<<< THRESHOLD", q, b["text"][:140]))
            if not ok:
                problems.append("pre-flight %s" % q)
        elif (a["pass"], a["text"]) != (b["pass"], b["text"]):
            problems.append("pre-flight changed: %s" % q)
            print("  <<< changed: %s" % q)
    print("  off passed %d, on passed %d of %d" % (sum(r["pass"] for r in offp.values()), sum(r["pass"] for r in onp.values()), len(onp)))

    print("\n## EVERYDAY + RELATIONAL")
    off14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "off", "adhoc", "adhoc_results.json"))}
    on14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "on", "adhoc", "adhoc_results.json"))}
    d14 = [q for q in off14 if off14[q] != on14.get(q)]
    offr = {r["id"]: (r["state"], r["text"]) for r in jload(os.path.join(REL, "plate-off", "results.json"))}
    onr = {r["id"]: (r["state"], r["text"]) for r in jload(os.path.join(REL, "plate-on", "results.json"))}
    dr = [q for q in offr if offr[q] != onr.get(q)]
    print("  everyday changed: %s   relational changed: %s" % (d14, dr))
    if d14:
        problems.append("everyday changed: %s" % d14)
    if dr:
        problems.append("relational changed: %s" % dr)

    print("\n" + "=" * 96)
    print("THRESHOLD FIRED - %d item(s)" % len(problems) if problems else "NO THRESHOLD FIRED.")
    for item in problems:
        print("  * %s" % item)
    print("=" * 96)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
