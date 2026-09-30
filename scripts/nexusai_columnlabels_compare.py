#!/usr/bin/env python3
"""A1 result-column-labels comparator, against thresholds written before the run.

    python scripts/nexusai_columnlabels_compare.py

Thresholds: reports/column-labels-20260929/THRESHOLD_PREREGISTRATION.md
"""
import glob
import io
import json
import os
import sys

ROOT = "reports/column-labels-20260929"
PRIOR = "reports/plate-read-search-20260928/on"
REL = "reports/relational-conditions-20260928"
PLATE = "reports/plate-read-search-20260928"
SUITES = ("golden", "holdout", "media")
FORBIDDEN = {"M1", "M2", "M3", "Metadata"}


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


def grids(arm):
    out = {}
    for path in glob.glob(os.path.join(ROOT, arm, "*", "raw", "*.json")):
        blob = jload(path)
        grid = (blob.get("enterprise") or {}).get("data_grid") or {}
        typed = bool((((blob.get("planner") or {}).get("query_plan") or {}).get("applied_filters") or {}).get("source_native"))
        out[os.path.basename(path)[:-5]] = (grid, typed)
    return out


def main():
    problems = []
    off, on, prior = corpus(os.path.join(ROOT, "off")), corpus(os.path.join(ROOT, "on")), corpus(PRIOR)
    if not off or not on or not prior:
        print("MISSING ARM")
        return 1
    print("=" * 96)
    print("A1 RESULT COLUMN LABELS")
    print("=" * 96)
    inert = [q for q in prior if prior[q]["verdict"] != off[q]["verdict"]]
    print("off vs prior deployed build: %d moved %s" % (len(inert), inert))
    if inert:
        problems.append("switch-off build does not reproduce the deployed build: %s" % inert)
    moved = [q for q in off if off[q]["verdict"] != on[q]["verdict"]]
    texts = [q for q in off if (off[q].get("answer_text") or "") != (on[q].get("answer_text") or "")]
    print("on vs off: verdicts moved %s   answer_text changed %s" % (moved, texts))
    if moved:
        problems.append("verdicts moved: %s" % moved)
    if texts:
        problems.append("answer text changed: %s" % texts)

    print("\n## DATA GRIDS")
    goff, gon = grids("off"), grids("on")
    relabelled, forbidden_left, examples = 0, [], []
    for qid, (grid_off, typed) in sorted(goff.items()):
        grid_on, _ = gon.get(qid, ({}, False))
        cols_off, cols_on = grid_off.get("columns") or [], grid_on.get("columns") or []
        if [c.get("key") for c in cols_off] != [c.get("key") for c in cols_on]:
            problems.append("%s column keys changed" % qid)
        if grid_off.get("rows") != grid_on.get("rows"):
            problems.append("%s row values changed" % qid)
        heads_off = [c.get("header") for c in cols_off]
        heads_on = [c.get("header") for c in cols_on]
        if heads_off != heads_on:
            relabelled += 1
            if len(examples) < 6:
                examples.append((qid, list(zip(heads_off, heads_on))))
        if typed and FORBIDDEN & set(heads_on):
            forbidden_left.append((qid, sorted(FORBIDDEN & set(heads_on))))
    print("  answers with relabelled headers: %d" % relabelled)
    for qid, pairs in examples:
        print("   %-22s %s" % (qid, "; ".join("%s -> %s" % (a, b) for a, b in pairs if a != b)))
    if forbidden_left:
        problems.append("typed-plan answers still showing raw headers: %s" % forbidden_left[:10])
    print("  typed-plan answers still showing M1/M2/Metadata: %d" % len(forbidden_left))

    print("\n## PROBES")
    offp = {r["question"]: (r["pass"], r["text"]) for r in jload(os.path.join(ROOT, "off", "preflight", "results.json"))}
    onp = {r["question"]: (r["pass"], r["text"]) for r in jload(os.path.join(ROOT, "on", "preflight", "results.json"))}
    dp = [q for q in offp if offp[q] != onp.get(q)]
    off14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "off", "adhoc", "adhoc_results.json"))}
    on14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "on", "adhoc", "adhoc_results.json"))}
    d14 = [q for q in off14 if off14[q] != on14.get(q)]
    offr = {r["id"]: (r["state"], r["text"]) for r in jload(os.path.join(REL, "labels-a1-off", "results.json"))}
    onr = {r["id"]: (r["state"], r["text"]) for r in jload(os.path.join(REL, "labels-a1-on", "results.json"))}
    dr = [q for q in offr if offr[q] != onr.get(q)]
    plate_on = jload(os.path.join(PLATE, "a1-on", "results.json"))
    print("  pre-flight %d/%d on, changed %s | everyday changed %s | relational changed %s | plate probe %d/8, leaks %s"
          % (sum(v[0] for v in onp.values()), len(onp), dp, d14, dr, sum(r["pass"] for r in plate_on),
             [r["id"] for r in plate_on if r["leaked"]]))
    for name, diff in (("pre-flight", dp), ("everyday", d14), ("relational", dr)):
        if diff:
            problems.append("%s changed: %s" % (name, diff))
    if sum(r["pass"] for r in plate_on) != 8 or any(r["leaked"] for r in plate_on):
        problems.append("plate probe regressed")

    print("\n" + "=" * 96)
    print("THRESHOLD FIRED - %d item(s)" % len(problems) if problems else "NO THRESHOLD FIRED.")
    for item in problems:
        print("  * %s" % item)
    print("=" * 96)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
