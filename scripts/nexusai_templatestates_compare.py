#!/usr/bin/env python3
"""A3 template-answers-state-their-result comparator, against thresholds written before the run.

    python scripts/nexusai_templatestates_compare.py

Thresholds: reports/template-states-20260929/THRESHOLD_PREREGISTRATION.md
"""
import glob
import io
import json
import os
import sys

ROOT = "reports/template-states-20260929"
PRIOR = "reports/a1-1-a2-20260929/on"
REL = "reports/relational-conditions-20260928"
PLATE = "reports/plate-read-search-20260928"
SUITES = ("golden", "holdout", "media")
CHANGED_TEMPLATES = {"frequent_contacts", "cross_family_correlation"}
TARGETS = {
    "CDR-10": ("923009998887", "923009998883", "121 CDR records", "tied"),
    "ANPR-03": ("2026-07-14 06:00:00 UTC", "2026-07-19 07:22:00 UTC", "87 ANPR sightings"),
}


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


def raws(root):
    return {os.path.basename(p)[:-5]: jload(p) for p in glob.glob(os.path.join(root, "*", "raw", "*.json"))}


def probe_diff(name, off, on, key, problems):
    """Texts may change only on the two templates A3 touches; every change is printed to be read."""
    for qid in off:
        a, b = off[qid], on.get(qid)
        if b is None:
            problems.append("%s: %s missing in the on arm" % (name, qid))
            continue
        if a.get(key) != b.get(key):
            problems.append("%s: %s %s moved %s -> %s" % (name, qid, key, a.get(key), b.get(key)))
        if a.get("text") != b.get("text"):
            print("  %s %s [%s]\n    off: %s\n    on:  %s" % (name, qid, b.get("template"), (a.get("text") or "")[:220], (b.get("text") or "")[:220]))
            if b.get("template") not in CHANGED_TEMPLATES:
                problems.append("%s: %s text changed on template %s" % (name, qid, b.get("template")))


def main():
    problems = []
    off, on, prior = corpus(os.path.join(ROOT, "off")), corpus(os.path.join(ROOT, "on")), corpus(PRIOR)
    if not off or not on or not prior:
        print("MISSING ARM")
        return 1
    print("=" * 96)
    print("A3 TEMPLATE ANSWERS STATE THEIR RESULT")
    print("=" * 96)
    inert = [q for q in prior if prior[q]["verdict"] != off[q]["verdict"]
             or (prior[q].get("answer_text") or "") != (off[q].get("answer_text") or "")]
    print("off vs A1.1+A2 measured build (verdict or answer_text): %s" % (inert or "identical"))
    if inert:
        problems.append("switch-off build does not reproduce the measured build: %s" % inert)

    print("\n## TARGETS")
    for qid, wants in TARGETS.items():
        text = on[qid].get("answer_text") or ""
        print("  %s %s -> %s" % (qid, off[qid]["verdict"], on[qid]["verdict"]))
        print("    on: %s" % text.split("\n")[0][:300])
        if on[qid]["verdict"] != "CORRECT":
            problems.append("%s is %s, not CORRECT" % (qid, on[qid]["verdict"]))
        missing = [w for w in wants if w not in text]
        if missing:
            problems.append("%s text lacks %s" % (qid, missing))

    print("\n## CORPUS")
    for qid in sorted(off):
        if qid in TARGETS:
            continue
        if off[qid]["verdict"] != on[qid]["verdict"]:
            problems.append("verdict moved: %s %s -> %s" % (qid, off[qid]["verdict"], on[qid]["verdict"]))
        if (off[qid].get("answer_text") or "") != (on[qid].get("answer_text") or ""):
            template = on[qid].get("template")
            print("  text changed %s [%s]\n    off: %s\n    on:  %s" % (qid, template, (off[qid].get("answer_text") or "").split("\n")[0][:220],
                                                                   (on[qid].get("answer_text") or "").split("\n")[0][:220]))
            if template not in CHANGED_TEMPLATES:
                problems.append("answer text changed on template %s: %s" % (template, qid))

    print("\n## DATA GRID")
    roff, ron = raws(os.path.join(ROOT, "off")), raws(os.path.join(ROOT, "on"))
    grid_moved = [q for q in roff if ((roff[q].get("enterprise") or {}).get("data_grid")) != ((ron.get(q, {}).get("enterprise") or {}).get("data_grid"))]
    print("  data grids not byte-identical: %s" % (grid_moved or "none"))
    if grid_moved:
        problems.append("data grid changed: %s" % grid_moved[:8])

    print("\n## PROBES (every changed text printed)")
    offp = {r["question"]: r for r in jload(os.path.join(ROOT, "off", "preflight", "results.json"))}
    onp = {r["question"]: r for r in jload(os.path.join(ROOT, "on", "preflight", "results.json"))}
    probe_diff("pre-flight", offp, onp, "pass", problems)
    off14 = {r["question"]: r for r in jload(os.path.join(ROOT, "off", "adhoc", "adhoc_results.json"))}
    on14 = {r["question"]: r for r in jload(os.path.join(ROOT, "on", "adhoc", "adhoc_results.json"))}
    probe_diff("everyday", off14, on14, "state", problems)
    offr = {r["id"]: r for r in jload(os.path.join(REL, "a3-off", "results.json"))}
    onr = {r["id"]: r for r in jload(os.path.join(REL, "a3-on", "results.json"))}
    probe_diff("relational", offr, onr, "state", problems)
    plate_off = {r["id"]: r for r in jload(os.path.join(PLATE, "a3-off", "results.json"))}
    plate_on = {r["id"]: r for r in jload(os.path.join(PLATE, "a3-on", "results.json"))}
    probe_diff("plate", plate_off, plate_on, "pass", problems)
    passed = sum(bool(r["pass"]) for r in onp.values())
    plate_pass = sum(bool(r["pass"]) for r in plate_on.values())
    leaks = [q for q, r in plate_on.items() if r.get("leaked")]
    print("  pre-flight %d/%d on | plate %d/%d, leaks %s" % (passed, len(onp), plate_pass, len(plate_on), leaks))
    if passed != len(onp):
        problems.append("pre-flight %d/%d" % (passed, len(onp)))
    if plate_pass != len(plate_on) or leaks:
        problems.append("plate probe regressed")

    print("\n" + "=" * 96)
    print("THRESHOLD FIRED - %d item(s)" % len(problems) if problems else "NO THRESHOLD FIRED.")
    for item in problems:
        print("  * %s" % item)
    print("=" * 96)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
