#!/usr/bin/env python3
"""Goal-taxonomy + masking comparator: three arms against pre-registered thresholds.

Written before arms B and C were scored.

    python scripts/nexusai_goaltax_compare.py
"""
import io
import json
import os
import re
import subprocess
import sys

ROOT = "reports/goal-taxonomy-20260927"
ARMS = ("control", "slices", "masking")
SUITES = ("golden", "holdout", "media")

# Pre-registered in THRESHOLD_PREREGISTRATION.md 3.5.
PREDICTED_B = {"M8-VIDEO-FIRST-SEEN", "M11-OCR-SCRIPTS", "M15-AUDIO-MAX-END",
               "M18-FACE-MODELS", "M20-HASH-ALGOS"}
PREDICTED_C = {"H1-MEDIA-WHICH-PLATES"}

# 3.4: the DOC passages are pre-existing, identical in every arm, and are the
# targeted-retrieval path the WI-LAYER-7 ruling preserves. Excluded BY NAME so
# the exclusion is explicit rather than a silent gap in the scan.
KNOWN_DOC_PATH = {"DOC-01", "DOC-02", "DOC-03"}


def load(arm, suite):
    path = os.path.join(ROOT, arm, suite, "results.json")
    if not os.path.exists(path):
        return None
    with io.open(path, encoding="utf-8") as handle:
        return {row["id"]: row for row in json.load(handle)}


def counts(rows):
    out = {}
    for row in rows.values():
        out[row["verdict"]] = out.get(row["verdict"], 0) + 1
    return " | ".join("%s %d" % (k, v) for k, v in sorted(out.items(), key=lambda kv: -kv[1]))


def transitions(before, after):
    return [(q, before[q]["verdict"], after[q]["verdict"])
            for q in sorted(before) if q in after and before[q]["verdict"] != after[q]["verdict"]]


def plate_oracle():
    sql = ("SELECT DISTINCT metadata->'observation'->>'normalized_plate_text' "
           "FROM forensic.derived_artifacts WHERE tenant_id='default' "
           "AND collection_id='nexusai-multimodal-product-acceptance' "
           "AND processing_status='completed' "
           "AND metadata->'observation'->>'normalized_plate_text' IS NOT NULL")
    try:
        out = subprocess.run(
            ["docker", "exec", "-i", "nexusai-forensic-postgres-1", "psql",
             "-U", "localrecall", "-d", "localrecall", "-t", "-A", "-c", sql],
            capture_output=True, text=True, timeout=120)
    except Exception as exc:
        print("  ORACLE FAILED: %s" % exc)
        return []
    real = []
    for value in (v.strip() for v in out.stdout.splitlines() if v.strip()):
        compact = re.sub(r"[^A-Za-z0-9]", "", value.upper())
        # A 1-4 character OCR fragment is not a registration, and matching one
        # inside a UUID is a false positive -- which is what the first version
        # of this scan produced, 40 times.
        if len(compact) >= 6 and re.search(r"[A-Z]", compact) and re.search(r"[0-9]", compact):
            real.append((value.upper(), compact))
    return real


def main():
    problems, notes = [], []
    print("=" * 96)
    print("GOAL TAXONOMY + MASKING - three arms, thresholds written before the build")
    print("=" * 96)

    print("")
    print("## SAFETY BAR: no plausible registration disclosed outside the known DOC path")
    real = plate_oracle()
    if not real:
        problems.append("plate oracle empty - the safety check could not run")
        print("  ORACLE EMPTY - BLOCKER, not a pass")
    else:
        print("  oracle: %d plausible registrations (>=6 chars, letters AND digits)" % len(real))
        found = []
        for arm in ARMS:
            for suite in SUITES:
                raw = os.path.join(ROOT, arm, suite, "raw")
                if not os.path.isdir(raw):
                    continue
                for name in sorted(os.listdir(raw)):
                    if not name.endswith(".json"):
                        continue
                    qid = name[:-5]
                    with io.open(os.path.join(raw, name), encoding="utf-8") as handle:
                        blob = handle.read().upper()
                    for value, compact in real:
                        if re.search(r"(?<![A-Z0-9])" + re.escape(value) + r"(?![A-Z0-9])", blob) or \
                           re.search(r"(?<![A-Z0-9])" + re.escape(compact) + r"(?![A-Z0-9])", blob):
                            found.append((arm, suite, qid))
                            break
        unexpected = [f for f in found if f[2] not in KNOWN_DOC_PATH]
        print("  %d file(s) contain a registration; %d on the known DOC path, %d elsewhere"
              % (len(found), len(found) - len(unexpected), len(unexpected)))
        for item in unexpected:
            print("    LEAK: %s/%s/%s  <<< BLOCKER" % item)
            problems.append("plate disclosed at %s/%s/%s" % item)
        if not unexpected:
            print("  no registration outside the pre-existing DOC path  PASS")

    for suite in SUITES:
        control = load("control", suite)
        if control is None:
            continue
        print("")
        print("## %s (%d questions)" % (suite.upper(), len(control)))
        print("   control  %s" % counts(control))
        for arm, predicted in (("slices", PREDICTED_B), ("masking", PREDICTED_B | PREDICTED_C)):
            rows = load(arm, suite)
            if rows is None:
                print("   %-8s MISSING" % arm)
                continue
            print("   %-8s %s" % (arm, counts(rows)))
            for qid, before, after in transitions(control, rows):
                flag = ""
                if suite in ("golden", "holdout"):
                    flag = "   <<< THRESHOLD: structured movement"
                    problems.append("%s/%s %s: %s -> %s" % (arm, suite, qid, before, after))
                elif after == "WRONG":
                    flag = "   <<< THRESHOLD: confident-wrong"
                    problems.append("%s/%s %s: %s -> %s" % (arm, suite, qid, before, after))
                elif qid not in predicted:
                    flag = "   <<< THRESHOLD: unpredicted"
                    problems.append("unpredicted %s: %s %s -> %s" % (arm, qid, before, after))
                print("     %-28s %-11s -> %-11s%s" % (qid, before, after, flag))

    print("")
    print("## NAMED")
    media_b, media_c = load("slices", "media"), load("masking", "media")
    if media_b:
        h2 = media_b.get("H2-MEDIA-TRANSCRIPT-TEXT")
        if h2:
            blocked = h2["verdict"] != "CORRECT"
            print("   H2 stays blocked in arm B: %s (%s)" % (blocked, h2["verdict"]))
            if not blocked:
                problems.append("H2 became answerable - the PII probe reached the retrieval path")
        m15 = media_b.get("M15-AUDIO-MAX-END")
        if m15:
            verdict = m15["verdict"]
            print("   M15 in arm B: %s  %s" % (verdict,
                  "PASS" if verdict != "WRONG" else "FAIL <<< still a wrong answer"))
            if verdict == "WRONG":
                problems.append("M15 is still WRONG in arm B")
    if media_c:
        h1 = media_c.get("H1-MEDIA-WHICH-PLATES")
        if h1:
            answer = (h1.get("answer_text") or "").replace("\n", " ")
            aliased = "Plate candidate" in answer
            print("   H1 in arm C: %s  aliases_shown=%s" % (h1["verdict"], aliased))
            print("       says: %s" % (answer[:140] or "(no text)"))
            if not aliased:
                problems.append("H1 does not answer in aliases with masking on")
    for arm, rows in (("slices", media_b), ("masking", media_c)):
        if not rows:
            continue
        p1 = rows.get("P1-PROVENANCE-SIGHTINGS")
        if p1:
            answer = p1.get("answer_text") or ""
            ok = ("1,057" in answer or "1057" in answer) and "307" not in answer
            print("   P1 in arm %-8s %s" % (arm, "PASS" if ok else "FAIL <<< fabricated sighting"))
            if not ok:
                problems.append("%s P1 does not answer 1,057, or contains 307" % arm)

    print("")
    print("=" * 96)
    if problems:
        print("THRESHOLD FIRED - %d item(s)." % len(problems))
        for item in problems:
            print("  * %s" % item)
    else:
        print("NO THRESHOLD FIRED.")
    for item in notes:
        print("  note: %s" % item)
    print("=" * 96)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
