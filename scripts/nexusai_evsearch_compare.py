#!/usr/bin/env python3
"""U2a evidence-search-first comparator, against thresholds written before the run.

    python scripts/nexusai_evsearch_compare.py

Thresholds: reports/evidence-search-first-20260928/THRESHOLD_PREREGISTRATION.md
"""
import io
import json
import os
import sys

ROOT = "reports/evidence-search-first-20260928"
PRIOR = "reports/absence-text-20260928/on"
REL = "reports/relational-conditions-20260928"
SUITES = ("golden", "holdout", "media")
PREDICTED_FLIPS = {
    "Find OCR text mentioning BCX-567": "DSC_1105.JPG",
    "Search image OCR for BCX-567": "DSC_1105.JPG",
    "Find OCR text mentioning MNA-08": "DSC_0990.JPG",
}
STILL_U2B = {"Which image shows plate MN1367?", "Find plate LEB15491 in the images"}


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
    print("U2a EVIDENCE SEARCH FIRST")
    print("=" * 96)
    inert = [q for q in prior if prior[q]["verdict"] != off[q]["verdict"]]
    print("off vs prior deployed build: %d moved %s" % (len(inert), inert))
    if inert:
        problems.append("switch-off build does not reproduce the deployed build: %s" % inert)
    moved = [(q, off[q]["verdict"], on[q]["verdict"]) for q in off if off[q]["verdict"] != on[q]["verdict"]]
    print("on vs off: %s" % moved)
    for q, before, after in moved:
        if q != "DOC-01":
            problems.append("unpredicted movement %s %s -> %s" % (q, before, after))
    doc01 = on.get("DOC-01", {})
    print("DOC-01 on: %s | %s" % (doc01.get("verdict"), (doc01.get("answer_text") or "")[:140]))
    if doc01.get("verdict") != "CORRECT":
        problems.append("DOC-01 did not reach CORRECT (%s)" % doc01.get("verdict"))
    for probe in ("H1-MEDIA-WHICH-PLATES", "VID-01"):
        if off[probe]["verdict"] != on[probe]["verdict"] or off[probe].get("answer_text") != on[probe].get("answer_text"):
            problems.append("privacy probe %s changed" % probe)
    print("privacy probes H1 / VID-01: %s / %s" % (on["H1-MEDIA-WHICH-PLATES"]["verdict"], on["VID-01"]["verdict"]))

    print("\n## PRE-FLIGHT")
    offp = {r["question"]: r for r in jload(os.path.join(ROOT, "off", "preflight", "results.json"))}
    onp = {r["question"]: r for r in jload(os.path.join(ROOT, "on", "preflight", "results.json"))}
    for q in offp:
        a, b = offp[q], onp[q]
        if q in PREDICTED_FLIPS:
            ok = b["pass"] and PREDICTED_FLIPS[q] in b["text"]
            print("  %s %s\n      on: %s" % ("PASS" if ok else "<<< THRESHOLD", q, b["text"][:120]))
            if not ok:
                problems.append("pre-flight %s did not find %s" % (q, PREDICTED_FLIPS[q]))
        elif q in STILL_U2B:
            print("  (U2b, expected unchanged) %s: pass=%s" % (q, b["pass"]))
        elif (a["pass"], a["text"]) != (b["pass"], b["text"]):
            print("  <<< THRESHOLD changed: %s\n      on: %s" % (q, b["text"][:120]))
            problems.append("pre-flight changed: %s" % q)
    print("  off passed %d, on passed %d" % (sum(r["pass"] for r in offp.values()), sum(r["pass"] for r in onp.values())))

    print("\n## EVERYDAY + RELATIONAL")
    off14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "off", "adhoc", "adhoc_results.json"))}
    on14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "on", "adhoc", "adhoc_results.json"))}
    d14 = [q for q in off14 if off14[q] != on14.get(q)]
    offr = {r["id"]: (r["state"], r["text"]) for r in jload(os.path.join(REL, "evsearch-off", "results.json"))}
    onr = {r["id"]: (r["state"], r["text"]) for r in jload(os.path.join(REL, "evsearch-on", "results.json"))}
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
