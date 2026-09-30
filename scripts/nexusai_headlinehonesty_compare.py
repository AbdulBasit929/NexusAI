#!/usr/bin/env python3
"""Headline honesty comparator (filters stated, empty aggregates), against thresholds written first.

    python scripts/nexusai_headlinehonesty_compare.py

Thresholds: reports/headline-honesty-20260929/THRESHOLD_PREREGISTRATION.md
"""
import glob
import io
import json
import os
import re
import sys

ROOT = "reports/headline-honesty-20260929"
PRIOR = "reports/count-names-field-20260929/on"
REL = "reports/relational-conditions-20260928"
PLATE = "reports/plate-read-search-20260928"
NEWQ = "reports/laptop-speed-20260929"
SUITES = ("golden", "holdout", "media")
RANGE_OPS = {"GT", "GTE", "LT", "LTE"}
CNIC_LIKE = re.compile(r"\b\d{5}-?\d{7}-?\d\b")
NEW_WANT = {
    "S5": "with plate number LHR-2026",
    "S7": "with protocol HTTPS",
    "S8": "No bytes uploaded values are recorded",
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


def honesty_shape(blob):
    """True when the executed plan has a non-range filter, or an aggregate that could be empty."""
    for filters in find(blob, "filters", []):
        if isinstance(filters, list):
            for f in filters:
                if isinstance(f, dict) and f.get("op") and f.get("op") not in RANGE_OPS:
                    return True
    for measures in find(blob, "measures", []):
        if isinstance(measures, list):
            for m in measures:
                if isinstance(m, dict) and m.get("op") in ("SUM", "AVG", "MIN", "MAX"):
                    return True
    return False


def first(text):
    return (text or "").split("\n")[0][:220]


def main():
    problems = []
    off, on, prior = corpus(os.path.join(ROOT, "off")), corpus(os.path.join(ROOT, "on")), corpus(PRIOR)
    if not off or not on or not prior:
        print("MISSING ARM")
        return 1
    print("=" * 96)
    print("HEADLINE HONESTY: FILTERS STATED, EMPTY AGGREGATES")
    print("=" * 96)
    # DOC-01 quotes a different (more relevant) passage of the same PDF since the LocalAI container was
    # restarted during the laptop-speed arms (reports/laptop-speed-20260929). It is CORRECT in every run,
    # identical in both honesty arms and stable when re-asked. Only its text may differ from the
    # reference; its verdict may not.
    known_environment = {"DOC-01"}
    inert = [q for q in prior if prior[q]["verdict"] != off[q]["verdict"]
             or (q not in known_environment and (prior[q].get("answer_text") or "") != (off[q].get("answer_text") or ""))]
    print("off vs the A4 build (verdict or answer_text): %s" % (inert or "identical"))
    if inert:
        problems.append("switch-off build does not reproduce the measured build: %s" % inert)

    print("\n## NEW QUESTIONS (S1-S8)")
    noff = {r["id"]: r for r in jload(os.path.join(NEWQ, "hh-off", "results.json"))}
    non = {r["id"]: r for r in jload(os.path.join(NEWQ, "hh-on", "results.json"))}
    for qid in sorted(noff):
        a, b = noff[qid].get("text") or "", non.get(qid, {}).get("text") or ""
        mark = "changed" if a != b else "same"
        print("  %s %-7s off: %s\n             on:  %s" % (qid, mark, a[:150], b[:150]))
        if qid in NEW_WANT:
            if NEW_WANT[qid] not in b:
                problems.append("%s lacks %r" % (qid, NEW_WANT[qid]))
        elif a != b:
            problems.append("%s changed but is not a target" % qid)

    print("\n## CORPUS")
    ron = raws(os.path.join(ROOT, "on"))
    roff = raws(os.path.join(ROOT, "off"))
    for qid in sorted(off):
        if off[qid]["verdict"] != on[qid]["verdict"]:
            problems.append("verdict moved: %s %s -> %s" % (qid, off[qid]["verdict"], on[qid]["verdict"]))
        ta, tb = off[qid].get("answer_text") or "", on[qid].get("answer_text") or ""
        if ta != tb:
            shape = honesty_shape(ron.get(qid, {}))
            print("  text changed %s (shape %s)\n    off: %s\n    on:  %s" % (qid, shape, first(ta), first(tb)))
            if not shape:
                problems.append("answer text changed outside the shape: %s" % qid)
            if CNIC_LIKE.search(first(tb)) and not CNIC_LIKE.search(first(ta)):
                problems.append("a CNIC-like value appeared in %s" % qid)

    grid_moved = [q for q in roff if ((roff[q].get("enterprise") or {}).get("data_grid")) != ((ron.get(q, {}).get("enterprise") or {}).get("data_grid"))]
    print("\n## DATA GRID: not byte-identical: %s" % (grid_moved or "none"))
    if grid_moved:
        problems.append("data grid changed: %s" % grid_moved[:8])

    print("\n## PROBES (every changed text printed)")

    def probe(name, a_path, b_path, key, frozen=False):
        a = {r.get("id") or r.get("question"): r for r in jload(a_path)}
        b = {r.get("id") or r.get("question"): r for r in jload(b_path)}
        for qid, row in a.items():
            other = b.get(qid)
            if other is None:
                problems.append("%s: %s missing" % (name, qid))
                continue
            if row.get(key) != other.get(key):
                problems.append("%s: %s %s moved %s -> %s" % (name, qid, key, row.get(key), other.get(key)))
            if row.get("text") != other.get("text"):
                print("  %s %s\n    off: %s\n    on:  %s" % (name, qid, first(row.get("text")), first(other.get("text"))))
                if frozen:
                    problems.append("%s: %s text changed" % (name, qid))
                if CNIC_LIKE.search(first(other.get("text"))) and not CNIC_LIKE.search(first(row.get("text"))):
                    problems.append("%s: a CNIC-like value appeared in %s" % (name, qid))

    probe("pre-flight", os.path.join(ROOT, "off", "preflight", "results.json"), os.path.join(ROOT, "on", "preflight", "results.json"), "pass")
    probe("everyday", os.path.join(ROOT, "off", "adhoc", "adhoc_results.json"), os.path.join(ROOT, "on", "adhoc", "adhoc_results.json"), "state")
    probe("relational", os.path.join(REL, "hh-off", "results.json"), os.path.join(REL, "hh-on", "results.json"), "state")
    probe("plate", os.path.join(PLATE, "hh-off", "results.json"), os.path.join(PLATE, "hh-on", "results.json"), "pass", frozen=True)
    onp = jload(os.path.join(ROOT, "on", "preflight", "results.json"))
    plate_on = jload(os.path.join(PLATE, "hh-on", "results.json"))
    passed = sum(bool(r["pass"]) for r in onp)
    print("  pre-flight %d/%d | plate %d/%d leaks %s" % (passed, len(onp), sum(bool(r["pass"]) for r in plate_on), len(plate_on), [r["id"] for r in plate_on if r.get("leaked")]))
    if passed != len(onp):
        problems.append("pre-flight %d/%d" % (passed, len(onp)))
    if any(r.get("leaked") for r in plate_on) or not all(r["pass"] for r in plate_on):
        problems.append("plate probe regressed")

    print("\n" + "=" * 96)
    print("THRESHOLD FIRED - %d item(s)" % len(problems) if problems else "NO THRESHOLD FIRED.")
    for item in problems:
        print("  * %s" % item)
    print("=" * 96)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
