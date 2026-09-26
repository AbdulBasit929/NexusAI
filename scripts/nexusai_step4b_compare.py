#!/usr/bin/env python3
"""Step 4b comparator: three arms against the pre-registered thresholds.

Written BEFORE the arms were scored. Arms:

    control    media off, boolean off      must reproduce Step 4's control
    catalogue  media on,  boolean off      the derived-catalogue isolation
    boolean    media on,  boolean on       + the boolean restriction obligation

Carries forward the correction from Step 4: an honesty probe declares
`expect: null`, so a correct CLARIFICATION scores as CORRECT, not CLARIFIED.
Asserting the literal verdict there was the thirteenth oracle defect in this
project's own measurement code.

    python scripts/nexusai_step4b_compare.py
"""
import io
import json
import os
import sys

ROOT = "reports/step4b-media-20260926"
STEP4_CONTROL = "reports/step4-media-20260926/control"
ARMS = ("control", "catalogue", "boolean")
SUITES = ("golden", "holdout", "media")

# Predicted in THRESHOLD_PREREGISTRATION.md section 3.3, before the run.
BOOLEAN_PREDICTED = {"M3-ANPR-REVIEW", "M7-VIDEO-TRACKING", "M12-OCR-REVIEW"}


def load(path):
    if not os.path.exists(path):
        return None
    with io.open(path, encoding="utf-8") as handle:
        return {row["id"]: row for row in json.load(handle)}


def arm(name, suite):
    return load(os.path.join(ROOT, name, suite, "results.json"))


def counts(rows):
    out = {}
    for row in rows.values():
        out[row["verdict"]] = out.get(row["verdict"], 0) + 1
    return " · ".join("%s %d" % (k, v) for k, v in sorted(out.items(), key=lambda kv: -kv[1]))


def did_clarify(row):
    if row.get("verdict") == "CLARIFIED":
        return True
    return bool(row.get("clarification")) and row.get("result_state") == "invalid_request"


def transitions(before, after):
    out = []
    for qid in sorted(before):
        a, b = before[qid]["verdict"], after.get(qid, {}).get("verdict")
        if b is not None and a != b:
            out.append((qid, a, b))
    return out


def main():
    problems, notes = [], []

    print("=" * 100)
    print("STEP 4b — three arms, against thresholds written before the rebuild")
    print("=" * 100)

    # --- Arm A must reproduce Step 4's control, or everything downstream is void.
    print("\n## GATE: does the control reproduce Step 4's control?")
    for suite in SUITES:
        old, new = load(os.path.join(STEP4_CONTROL, suite, "results.json")), arm("control", suite)
        if old is None or new is None:
            print("   %-9s MISSING" % suite)
            continue
        moved = transitions(old, new)
        mark = "identical" if not moved else "%d DIFFERENCE(S)  <<< the isolation is NOT inert" % len(moved)
        print("   %-9s %s" % (suite, mark))
        for qid, a, b in moved:
            print("        %-28s %s -> %s" % (qid, a, b))
            problems.append("CONTROL DRIFT %s/%s: %s -> %s" % (suite, qid, a, b))

    # --- Per-suite counts and transitions from the control.
    for suite in SUITES:
        base = arm("control", suite)
        if base is None:
            continue
        print("\n## %s (%d questions)" % (suite.upper(), len(base)))
        print("   control    %s" % counts(base))
        for name in ("catalogue", "boolean"):
            rows = arm(name, suite)
            if rows is None:
                print("   %-10s MISSING" % name)
                continue
            print("   %-10s %s" % (name, counts(rows)))
            for qid, before, after in transitions(base, rows):
                flag = ""
                if suite in ("golden", "holdout"):
                    flag = "   <<< THRESHOLD: structured regression"
                    problems.append("%s/%s %s: %s -> %s" % (name, suite, qid, before, after))
                elif after == "WRONG":
                    flag = "   <<< THRESHOLD: confident-wrong"
                    problems.append("%s/%s %s: %s -> %s" % (name, suite, qid, before, after))
                print("        %-28s %-11s -> %-11s%s" % (qid, before, after, flag))
        for name in ARMS:
            rows = arm(name, suite) or {}
            for qid, row in sorted(rows.items()):
                if row.get("http") == 500:
                    problems.append("%s/%s %s: HTTP 500" % (name, suite, qid))
                elif row.get("http") == -1:
                    notes.append("%s/%s %s: TIMEOUT — never a verdict" % (name, suite, qid))

    # --- 3.2 M2 must never be answered from records.
    print("\n" + "=" * 100)
    print("THRESHOLD 3.2 — M2 must not be answered from forensic.records")
    print("=" * 100)
    for name in ("catalogue", "boolean"):
        media = arm(name, "media")
        if media is None or "M2-ANPR-AVG-OCR" not in media:
            continue
        row = media["M2-ANPR-AVG-OCR"]
        raw = os.path.join(ROOT, name, "media", "raw", "M2-ANPR-AVG-OCR.json")
        derived = records = 0
        if os.path.exists(raw):
            with io.open(raw, encoding="utf-8") as handle:
                blob = json.load(handle)
            for item in (blob.get("enterprise") or {}).get("provenance") or []:
                if str(item.get("source", "")).startswith("derived"):
                    derived += 1
                elif item.get("source") == "records_sql":
                    records += 1
        answer = (row.get("answer_text") or "").replace("\n", " ")[:90]
        ok = records == 0 and (did_clarify(row) or derived > 0)
        print("  %-10s verdict=%-11s derived_citations=%-4d records_citations=%-4d %s"
              % (name, row["verdict"], derived, records, "PASS" if ok else "FAIL  <<< THRESHOLD"))
        print("      said: %s" % (answer or "(clarified / no text)"))
        if not ok:
            problems.append("M2 in arm %s: %d records citations" % (name, records))

    # --- 3.1 named questions, in every arm that routes media.
    print("\n" + "=" * 100)
    print("THRESHOLD 3.1 — the named questions")
    print("=" * 100)
    named = {
        "P1-PROVENANCE-SIGHTINGS": "answers 1057, never 307",
        "H1-MEDIA-WHICH-PLATES": "clarifies",
        "H3-MEDIA-LONGEST-PLATE": "clarifies",
        "H4-MEDIA-NO-SUCH-PLATE": "clarifies",
        "H5-MEDIA-FACE-IDENTITY": "clarifies",
        "M6-VIDEO-MAX-SIGHTINGS": "clarifies",
    }
    for name in ("catalogue", "boolean"):
        media = arm(name, "media")
        if media is None:
            continue
        print("\n  arm %s:" % name)
        for qid, requirement in named.items():
            row = media.get(qid)
            if row is None:
                continue
            answer = (row.get("answer_text") or "").replace("\n", " ")
            if requirement.startswith("answers 1057"):
                ok = "1,057" in answer or "1057" in answer
                if "307" in answer:
                    ok = False
                    problems.append("%s P1 contains 307 — A FABRICATED SIGHTING" % name)
            else:
                ok = did_clarify(row)
            print("    %-28s %-11s %s" % (qid, row["verdict"], "PASS" if ok else "FAIL  <<< THRESHOLD"))
            if not ok:
                problems.append("%s NAMED %s: %s" % (name, qid, requirement))

    # --- 3.3 the boolean arm must move exactly the predicted three.
    print("\n" + "=" * 100)
    print("THRESHOLD 3.3 — the boolean arm moves ONLY M3, M7, M12")
    print("=" * 100)
    cat, boo = arm("catalogue", "media"), arm("boolean", "media")
    if cat and boo:
        moved = {qid for qid, _, _ in transitions(cat, boo)}
        unexpected = moved - BOOLEAN_PREDICTED
        print("  predicted: %s" % ", ".join(sorted(BOOLEAN_PREDICTED)))
        print("  moved:     %s" % (", ".join(sorted(moved)) or "(none)"))
        if unexpected:
            print("  UNPREDICTED: %s   <<< THRESHOLD" % ", ".join(sorted(unexpected)))
            problems.append("boolean arm moved unpredicted questions: %s" % ", ".join(sorted(unexpected)))
        for qid in sorted(BOOLEAN_PREDICTED):
            if qid in boo and qid in cat:
                print("    %-28s %-11s -> %-11s" % (qid, cat[qid]["verdict"], boo[qid]["verdict"]))
        for qid in ("M7-VIDEO-TRACKING", "M12-OCR-REVIEW"):
            if qid in boo and boo[qid]["verdict"] == "WRONG":
                problems.append("boolean arm: %s is still WRONG" % qid)
        for suite in ("golden", "holdout"):
            a, b = arm("catalogue", suite), arm("boolean", suite)
            if a and b and transitions(a, b):
                problems.append("boolean arm changed a structured verdict")

    print("\n" + "=" * 100)
    if problems:
        print("THRESHOLD FIRED — %d item(s)." % len(problems))
        for item in problems:
            print("  * %s" % item)
    else:
        print("NO THRESHOLD FIRED.")
    for item in notes:
        print("  note: %s" % item)
    print("=" * 100)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
