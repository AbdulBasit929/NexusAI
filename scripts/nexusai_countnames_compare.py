#!/usr/bin/env python3
"""A4 a-count-of-a-field-names-the-field comparator, against thresholds written before the run.

    python scripts/nexusai_countnames_compare.py

Thresholds: reports/count-names-field-20260929/THRESHOLD_PREREGISTRATION.md
"""
import glob
import io
import json
import os
import re
import sys

ROOT = "reports/count-names-field-20260929"
PRIOR = "reports/template-states-20260929/on"
REL = "reports/relational-conditions-20260928"
PLATE = "reports/plate-read-search-20260928"
SUITES = ("golden", "holdout", "media")
TARGETS = {
    "CDR-08": "10 distinct subscriber number values",
    "ANPR-05": "6 distinct plate number values",
    "H1-CDR-HANDSETS": "4,997 distinct device IMEI values",
}
# A probe answer may change only from the old field-less sentence to one that names the field.
OLD = re.compile(r"^The count across |^There (?:is|are) [\d,]+ .+ in this case\.")
NEW = re.compile(r"distinct .+ values? across |There are no .+ values across | ha(?:s|ve) an? .+ value\.")


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


def find(obj, key, acc):
    if isinstance(obj, dict):
        for k, v in obj.items():
            if k == key:
                acc.append(v)
            find(v, key, acc)
    elif isinstance(obj, list):
        for v in obj:
            find(v, key, acc)
    return acc


def counts_a_field(blob):
    """True when the executed plan is an ungrouped COUNT_DISTINCT or COUNT of a field."""
    measures = [m for m in find(blob, "measures", []) if isinstance(m, list) and m and all(isinstance(x, dict) for x in m)]
    groups = [g for g in find(blob, "group_fields", []) if isinstance(g, list)]
    if not measures or len(measures[0]) != 1 or (groups and groups[0]):
        return False
    measure = measures[0][0]
    return measure.get("op") in ("COUNT", "COUNT_DISTINCT") and bool(measure.get("field_id"))


def first_line(text):
    return (text or "").split("\n")[0]


def probe_diff(name, off, on, key, problems, frozen=False):
    for qid in off:
        a, b = off[qid], on.get(qid)
        if b is None:
            problems.append("%s: %s missing in the on arm" % (name, qid))
            continue
        if a.get(key) != b.get(key):
            problems.append("%s: %s %s moved %s -> %s" % (name, qid, key, a.get(key), b.get(key)))
        if a.get("text") != b.get("text"):
            print("  %s %s\n    off: %s\n    on:  %s" % (name, qid, first_line(a.get("text"))[:220], first_line(b.get("text"))[:220]))
            if frozen or not (OLD.search(first_line(a.get("text"))) and NEW.search(first_line(b.get("text")))):
                problems.append("%s: %s text changed outside the A4 shape" % (name, qid))


def main():
    problems = []
    off, on, prior = corpus(os.path.join(ROOT, "off")), corpus(os.path.join(ROOT, "on")), corpus(PRIOR)
    if not off or not on or not prior:
        print("MISSING ARM")
        return 1
    print("=" * 96)
    print("A4 A COUNT OF A FIELD NAMES THE FIELD")
    print("=" * 96)
    inert = [q for q in prior if prior[q]["verdict"] != off[q]["verdict"]
             or (prior[q].get("answer_text") or "") != (off[q].get("answer_text") or "")]
    print("off vs A3 measured build (verdict or answer_text): %s" % (inert or "identical"))
    if inert:
        problems.append("switch-off build does not reproduce the measured build: %s" % inert)

    print("\n## TARGETS")
    for qid, want in TARGETS.items():
        text = on[qid].get("answer_text") or ""
        print("  %s %s -> %s\n    off: %s\n    on:  %s" % (qid, off[qid]["verdict"], on[qid]["verdict"], first_line(off[qid].get("answer_text")), first_line(text)))
        if on[qid]["verdict"] != "CORRECT":
            problems.append("%s is %s, not CORRECT" % (qid, on[qid]["verdict"]))
        if want not in text:
            problems.append("%s text lacks %r" % (qid, want))

    print("\n## CORPUS")
    ron, roff = raws(os.path.join(ROOT, "on")), raws(os.path.join(ROOT, "off"))
    for qid in sorted(off):
        if off[qid]["verdict"] != on[qid]["verdict"] and qid not in TARGETS:
            problems.append("verdict moved: %s %s -> %s" % (qid, off[qid]["verdict"], on[qid]["verdict"]))
        if (off[qid].get("answer_text") or "") != (on[qid].get("answer_text") or "") and qid not in TARGETS:
            shape = counts_a_field(ron.get(qid, {}))
            print("  text changed %s (counts a field: %s)\n    off: %s\n    on:  %s" % (qid, shape, first_line(off[qid].get("answer_text"))[:220], first_line(on[qid].get("answer_text"))[:220]))
            if not shape:
                problems.append("answer text changed outside the A4 shape: %s" % qid)

    print("\n## DATA GRID")
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
    offr = {r["id"]: r for r in jload(os.path.join(REL, "a4-off", "results.json"))}
    onr = {r["id"]: r for r in jload(os.path.join(REL, "a4-on", "results.json"))}
    probe_diff("relational", offr, onr, "state", problems)
    plate_off = {r["id"]: r for r in jload(os.path.join(PLATE, "a4-off", "results.json"))}
    plate_on = {r["id"]: r for r in jload(os.path.join(PLATE, "a4-on", "results.json"))}
    probe_diff("plate", plate_off, plate_on, "pass", problems, frozen=True)
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
