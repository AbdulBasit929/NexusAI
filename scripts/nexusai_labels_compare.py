#!/usr/bin/env python3
"""S4 derived result labels comparator, against thresholds written before the run.

    python scripts/nexusai_labels_compare.py

Thresholds: reports/derived-labels-20260928/THRESHOLD_PREREGISTRATION.md
"""
import io
import json
import os
import re
import sys

ROOT = "reports/derived-labels-20260928"
PRIOR = "reports/rowcount-headline-20260928/on"
REL = "reports/relational-conditions-20260928"
SUITES = ("golden", "holdout", "media")
CENSUS = {
    "M1-ANPR-READS": "plate reads", "M2-ANPR-AVG-OCR": "plate reads", "M5-VIDEO-GROUPS": "video plate groups",
    "M9-OCR-REGIONS": "image text regions", "M13-AUDIO-SEGMENTS": "transcribed audio segments",
    "M16-FACES": "detected faces", "M17-FACE-AVG-CONF": "detected faces", "M19-FINGERPRINTS": "image fingerprints",
}
PREFLIGHT_ALLOWED = {"How many faces were detected in the evidence?", "How many plate reads were produced from the images?"}


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


def numbers(text):
    return re.findall(r"\d[\d,]*(?:\.\d+)?", text or "")


def main():
    problems = []
    off, on, prior = corpus(os.path.join(ROOT, "off")), corpus(os.path.join(ROOT, "on")), corpus(PRIOR)
    if not off or not on or not prior:
        print("MISSING ARM")
        return 1
    print("=" * 96)
    print("S4 DERIVED RESULT LABELS")
    print("=" * 96)

    inert = [q for q in prior if prior[q]["verdict"] != off[q]["verdict"]]
    print("off vs prior deployed build: %d verdicts moved %s" % (len(inert), inert))
    if inert:
        problems.append("switch-off build does not reproduce the deployed build: %s" % inert)
    moved = [q for q in off if off[q]["verdict"] != on[q]["verdict"]]
    print("on vs off: %d verdicts moved %s" % (len(moved), moved))
    if moved:
        problems.append("verdicts moved: %s" % moved)

    print("\n## ANSWER TEXT CHANGES (must be exactly the 8 census questions)")
    changed = sorted(q for q in off if (off[q].get("answer_text") or "") != (on[q].get("answer_text") or ""))
    for q in changed:
        before = (off[q].get("answer_text") or "").split("\n")[0]
        after = (on[q].get("answer_text") or "").split("\n")[0]
        flag = ""
        if q not in CENSUS:
            flag = "   <<< THRESHOLD: not in census"
            problems.append("text changed outside the census: %s" % q)
        else:
            if CENSUS[q] not in after:
                flag = "   <<< THRESHOLD: does not name %r" % CENSUS[q]
                problems.append("%s does not name %s" % (q, CENSUS[q]))
            if numbers(before) != numbers(after):
                flag += "   <<< THRESHOLD: number changed"
                problems.append("%s number changed" % q)
        print("  %-22s %s\n        off: %s\n        on : %s" % (q, flag or "PASS", before[:110], after[:110]))
    missing = sorted(set(CENSUS) - set(changed))
    if missing:
        problems.append("census questions not relabelled: %s" % missing)
        print("  NOT RELABELLED: %s   <<< THRESHOLD" % missing)
    p1 = on.get("P1-PROVENANCE-SIGHTINGS", {}).get("answer_text") or ""
    print("\n  P1 on: %s" % p1.split("\n")[0][:100])
    if "1,057 ANPR sightings" not in p1:
        problems.append("P1 camera sightings relabelled or changed")

    print("\n## PRE-FLIGHT")
    offp = {r["question"]: r for r in jload(os.path.join(ROOT, "off", "preflight", "results.json"))}
    onp = {r["question"]: r for r in jload(os.path.join(ROOT, "on", "preflight", "results.json"))}
    for q in offp:
        if offp[q]["pass"] != onp[q]["pass"]:
            problems.append("pre-flight pass/fail changed: %s" % q)
            print("  <<< pass/fail changed: %s" % q)
        elif offp[q]["text"] != onp[q]["text"]:
            ok = q in PREFLIGHT_ALLOWED
            print("  %s text: %s\n      on: %s" % ("allowed" if ok else "<<< THRESHOLD", q, onp[q]["text"][:110]))
            if not ok:
                problems.append("pre-flight text changed: %s" % q)
    print("  off passed %d, on passed %d" % (sum(r["pass"] for r in offp.values()), sum(r["pass"] for r in onp.values())))

    print("\n## EVERYDAY 14 + RELATIONAL 16")
    off14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "off", "adhoc", "adhoc_results.json"))}
    on14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "on", "adhoc", "adhoc_results.json"))}
    d14 = [q for q in off14 if off14[q] != on14.get(q)]
    offr = {r["id"]: (r["state"], r.get("reason_code")) for r in jload(os.path.join(REL, "labels-off", "results.json"))}
    onr = {r["id"]: (r["state"], r.get("reason_code")) for r in jload(os.path.join(REL, "labels-on", "results.json"))}
    dr = [q for q in offr if offr[q] != onr.get(q)]
    print("  everyday changed: %s   relational changed: %s" % (d14, dr))
    if d14:
        problems.append("everyday text changed: %s" % d14)
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
