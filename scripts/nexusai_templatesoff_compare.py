#!/usr/bin/env python3
"""Templates-off census: what the runtime query path (LLM plan + compiler) answers alone.

    python scripts/nexusai_templatesoff_compare.py

Compares the ladder-on arm (reports/count-names-field-20260929/on) with the ladder-off arm
(reports/templates-off-20260929/ladder-off), same image and switches. A census, not a ship decision.
"""
import collections
import glob
import io
import json
import os
import sys

ON = "reports/count-names-field-20260929/on"
OFF = "reports/templates-off-20260929/ladder-off"
SUITES = ("golden", "holdout", "media")


def jload(path):
    with io.open(path, encoding="utf-8") as handle:
        return json.load(handle)


def corpus(root):
    out = {}
    for suite in SUITES:
        for row in jload(os.path.join(root, suite, "results.json")):
            out[row["id"]] = row
    return out


def raws(root):
    return {os.path.basename(p)[:-5]: jload(p) for p in glob.glob(os.path.join(root, "*", "raw", "*.json"))}


def path_of(blob):
    records = blob.get("records") if isinstance(blob.get("records"), dict) else {}
    if "plan" in records or "source_native_results" in records:
        sop = (blob.get("planner") or {}).get("semantic_operation_planner") or {}
        return "llm-plan" if sop.get("ir_fallback_outcome") == "accepted" else "compiler-plan"
    template = blob.get("template") or ""
    return "terminal" if template in ("", "terminal") else "template:" + template


def first(text):
    return (text or "").split("\n")[0][:200]


def main():
    on, off = corpus(ON), corpus(OFF)
    ron, roff = raws(ON), raws(OFF)
    print("=" * 100)
    print("TEMPLATES OFF -- WHAT THE RUNTIME PATH ANSWERS ALONE")
    print("=" * 100)
    for name, rows in (("ladder ON ", on), ("ladder OFF", off)):
        print("%s  %s" % (name, dict(collections.Counter(r["verdict"] for r in rows.values()))))
    paths_on = collections.Counter((path_of(ron.get(q, {})), on[q]["verdict"]) for q in on if on[q]["verdict"] == "CORRECT")
    paths_off = collections.Counter((path_of(roff.get(q, {})), off[q]["verdict"]) for q in off if off[q]["verdict"] == "CORRECT")
    print("\ncorrect by path, ladder on :", dict(collections.Counter(k[0].split(":")[0] for k in paths_on.elements())))
    print("correct by path, ladder off:", dict(collections.Counter(k[0].split(":")[0] for k in paths_off.elements())))

    lost, gained, new_wrong = [], [], []
    for q in sorted(on):
        a, b = on[q]["verdict"], off[q]["verdict"]
        if a == b:
            continue
        entry = (q, a, b, path_of(ron.get(q, {})), path_of(roff.get(q, {})), first(on[q].get("answer_text")), first(off[q].get("answer_text")))
        if b == "WRONG":
            new_wrong.append(entry)
        if a == "CORRECT":
            lost.append(entry)
        elif b == "CORRECT":
            gained.append(entry)
    for title, items in (("CONFIDENT WRONG WITH TEMPLATES OFF (must be closed first)", new_wrong),
                         ("CORRECT ONLY WITH TEMPLATES", lost), ("CORRECT ONLY WITH TEMPLATES OFF", gained)):
        print("\n## %s: %d" % (title, len(items)))
        for q, a, b, pa, pb, ta, tb in items:
            print("  %-22s %s -> %s   [%s] -> [%s]\n    on:  %s\n    off: %s" % (q, a, b, pa, pb, ta, tb))
    families = collections.Counter(e[3] for e in lost)
    print("\n## WORK ORDER (templates whose answers the runtime path does not yet give)")
    for template, count in families.most_common():
        print("  %-40s %d" % (template, count))
    return 0


if __name__ == "__main__":
    sys.exit(main())
