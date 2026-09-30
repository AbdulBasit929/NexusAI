#!/usr/bin/env python3
"""A1.1 table-header casing + A2 citation truth state comparator, against thresholds written
before the run.

    python scripts/nexusai_a11a2_compare.py

Thresholds: reports/a1-1-a2-20260929/THRESHOLD_PREREGISTRATION.md
"""
import glob
import io
import json
import os
import re
import sys

ROOT = "reports/a1-1-a2-20260929"
PRIOR = "reports/column-labels-20260929/on"
REL = "reports/relational-conditions-20260928"
PLATE = "reports/plate-read-search-20260928"
SUITES = ("golden", "holdout", "media")
TRUTH = {
    "forensics.audio-timestamp-segment/v1": "derived_model_observation",
    "forensics.image-ocr-observation/v1": "derived_model_observation",
    "forensics.document-native-text-passage/v1": "derived_native_text",
    "forensics.audio-roman-urdu-segment/v1": "derived_text_representation",
}
BAD_WORDS = re.compile(r"\b(?:END|ROW|AT|NON|Imei|Imsi|Cnic|Msisdn)\b")
# The reference arm was measured BEFORE the A1 acronym fix that shipped in the 10:56 image
# (reports/column-labels-20260929/RESULT.md, defect 1). This one header difference is that fix and
# nothing else; any other difference still fires.
SHIPPED_SINCE_REFERENCE = {("ACC-02", "Count of hTTP status code", "Count of HTTP status code")}


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


def raws(arm):
    return {os.path.basename(p)[:-5]: jload(p) for p in glob.glob(os.path.join(ROOT, arm, "*", "raw", "*.json"))}


def main():
    problems = []
    off, on, prior = corpus(os.path.join(ROOT, "off")), corpus(os.path.join(ROOT, "on")), corpus(PRIOR)
    if not off or not on or not prior:
        print("MISSING ARM")
        return 1
    print("=" * 96)
    print("A1.1 TABLE HEADER CASING + A2 CITATION TRUTH STATE")
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

    roff, ron = raws("off"), raws("on")
    rprior = {os.path.basename(p)[:-5]: jload(p) for p in glob.glob(os.path.join(PRIOR, "*", "raw", "*.json"))}
    drift = []
    for qid, blob in rprior.items():
        a, b = blob.get("enterprise") or {}, (roff.get(qid) or {}).get("enterprise") or {}
        h_ref = [c.get("header") for c in (a.get("data_grid") or {}).get("columns") or []]
        h_off = [c.get("header") for c in (b.get("data_grid") or {}).get("columns") or []]
        diffs = {(qid, x, y) for x, y in zip(h_ref, h_off) if x != y}
        if len(h_ref) != len(h_off) or (diffs and not diffs <= SHIPPED_SINCE_REFERENCE):
            drift.append((qid, "headers", sorted(diffs - SHIPPED_SINCE_REFERENCE)[:3]))
        elif diffs:
            print("  known, shipped since the reference: %s" % sorted(diffs))
        if any(i.get("source") == "kb_rag" and i.get("source_truth_state") for i in b.get("provenance") or []) or \
                any(c.get("source_truth_state") for c in (b.get("fact_packet") or {}).get("citations") or []):
            drift.append((qid, "truth state with switch off"))
    print("off vs prior deployed build, headers and truth state: %s" % (drift[:8] or "identical"))
    if drift:
        problems.append("switch-off build is not inert: %s" % drift[:8])
    print("\n## A1.1 HEADERS")
    changed_headers, bad_left, examples = 0, [], []
    for qid, blob_off in sorted(roff.items()):
        g_off = (blob_off.get("enterprise") or {}).get("data_grid") or {}
        g_on = (ron.get(qid, {}).get("enterprise") or {}).get("data_grid") or {}
        c_off, c_on = g_off.get("columns") or [], g_on.get("columns") or []
        if [c.get("key") for c in c_off] != [c.get("key") for c in c_on]:
            problems.append("%s column keys changed" % qid)
        if g_off.get("rows") != g_on.get("rows"):
            problems.append("%s row values changed" % qid)
        h_off = [c.get("header") for c in c_off]
        h_on = [c.get("header") for c in c_on]
        if h_off != h_on:
            changed_headers += 1
            pairs = [(a, b) for a, b in zip(h_off, h_on) if a != b]
            if len(examples) < 8 and pairs:
                examples.append((qid, pairs[:4]))
        for header in h_on:
            if header and BAD_WORDS.search(str(header)):
                bad_left.append((qid, header))
    print("  answers with changed headers: %d" % changed_headers)
    for qid, pairs in examples:
        print("   %-22s %s" % (qid, "; ".join("%s -> %s" % p for p in pairs)))
    print("  headers still badly cased in the on arm: %s" % bad_left[:10])
    if bad_left:
        problems.append("badly cased headers remain: %s" % bad_left[:10])

    # Every provenance item citing a derived text artifact, from either the search path (kb_rag, which
    # A2 changes) or the typed derived-artifact path (which already carried it), must carry the stored
    # truth state for its type. Fact-packet citations are built index-aligned with provenance, so each
    # citation is checked against its own item, not "some citation has one".
    print("\n## A2 CITATION TRUTH STATE")
    carried, gained_search, wrong, record_leak, cite_bad, cite_carried = 0, 0, [], [], [], 0
    for qid, blob in sorted(ron.items()):
        ent = blob.get("enterprise") or {}
        prov = ent.get("provenance") or []
        prov_off = ((roff.get(qid) or {}).get("enterprise") or {}).get("provenance") or []
        for index, item in enumerate(prov):
            artifact_type = item.get("artifact_type")
            state = item.get("source_truth_state")
            if artifact_type in TRUTH:
                if state == TRUTH[artifact_type]:
                    carried += 1
                    before = prov_off[index].get("source_truth_state") if index < len(prov_off) else None
                    if item.get("source") == "kb_rag" and not before:
                        gained_search += 1
                else:
                    wrong.append((qid, item.get("source"), artifact_type, state))
            if item.get("source") == "records_sql" and state:
                before = prov_off[index].get("source_truth_state") if index < len(prov_off) else None
                if not before:
                    record_leak.append((qid, state))
        citations = ((ent.get("fact_packet") or {}).get("citations")) or []
        for index, citation in enumerate(citations):
            expected = (prov[index].get("source_truth_state") or "") if index < len(prov) else ""
            got = citation.get("source_truth_state") or ""
            if got != expected:
                cite_bad.append((qid, citation.get("citation_id"), expected, got))
            elif got:
                cite_carried += 1
    print("  provenance items citing a derived text artifact with the stored truth state: %d" % carried)
    print("    of which search (kb_rag) items that gained it with the switch on: %d" % gained_search)
    print("  wrong or missing: %s" % wrong[:10])
    print("  records_sql items that gained a truth state: %s" % record_leak[:5])
    print("  fact-packet citations carrying their item's truth state: %d; mismatched: %s" % (cite_carried, cite_bad[:6]))
    if gained_search == 0:
        problems.append("no search provenance item gained its truth state")
    if wrong:
        problems.append("truth state wrong or missing: %s" % wrong[:10])
    if record_leak:
        problems.append("structured record citations gained a truth state: %s" % record_leak[:5])
    if cite_bad:
        problems.append("fact-packet citations do not match provenance: %s" % cite_bad[:6])

    print("\n## PROBES")
    offp = {r["question"]: (r["pass"], r["text"]) for r in jload(os.path.join(ROOT, "off", "preflight", "results.json"))}
    onp = {r["question"]: (r["pass"], r["text"]) for r in jload(os.path.join(ROOT, "on", "preflight", "results.json"))}
    dp = [q for q in offp if offp[q] != onp.get(q)]
    off14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "off", "adhoc", "adhoc_results.json"))}
    on14 = {r["question"]: r["text"] for r in jload(os.path.join(ROOT, "on", "adhoc", "adhoc_results.json"))}
    d14 = [q for q in off14 if off14[q] != on14.get(q)]
    offr = {r["id"]: (r["state"], r["text"]) for r in jload(os.path.join(REL, "a11a2-off", "results.json"))}
    onr = {r["id"]: (r["state"], r["text"]) for r in jload(os.path.join(REL, "a11a2-on", "results.json"))}
    dr = [q for q in offr if offr[q] != onr.get(q)]
    plate_on = jload(os.path.join(PLATE, "a11a2-on", "results.json"))
    print("  pre-flight %d/%d on, changed %s | everyday changed %s | relational changed %s | plate %d/8 leaks %s"
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
