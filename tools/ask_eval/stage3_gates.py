#!/usr/bin/env python3
"""The gates of the stage-3 pre-registration (docs/data/link-analysis-stage3-preregistration-20261022.md section 7), read from banks and result files:

    python -X utf8 tools/ask_eval/stage3_gates.py --pair S3B-102.json S3B-102-results.jsonl --pair S3B-104.json S3B-104-results.jsonl --pair S3B-106.json S3B-106-results.jsonl

Gate 1 wrong answers zero, in total and in each kind; gate 2 at least 90% correct in each kind (a decline or an abstention counts against it); gate 3 controls 100% correct (a bank
question marked `control`); gate 4 the fixed caveat on 100% of the answers that return rows of an analysis and on every probe that is answered; gate 5 the derivation in plain words on every
answered question (the engine's note that the query was model-written and checked, and, for a view that reads a parameter of the question, the line that says the value used and where it came
from); gate 6 latency (the gate fails only if the 90th percentile is above 120 s or any answer takes more than 300 s). Gate 7 (no regression after a deploy) is measured by the live smoke
and is not read here.
"""
import argparse
import collections
import json
import os
import statistics
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import judge  # noqa: E402

CAVEAT_OF_VIEW = {
    "v_colocation_windows": ["proximity is not association", "no chance baseline is computed"],
    "v_meeting_candidates": ["a candidate, not a finding", "proximity and contact are not proof of a meeting"],
    "v_successor_candidates": ["behavioural similarity, not identity"],
    "v_weekly_anchors": ["cell sites, not positions of a person"],
    "v_anchor_runs": ["cell sites, not positions of a person"],
    "v_movement_anchors": ["cell sites, not positions of a person"],
    "v_anchor_changes": ["cell sites, not positions of a person"],
    "v_anchor_deviations": ["cell sites, not positions of a person"],
    "v_speed_alerts": ["cell sites, not positions of a person"],
    "v_rhythm_changes": ["a change in the pattern of use, not an explanation"],
}
# views that read a parameter the analyst can name (the answer must say the value used and where it came from)
PARAMETER_VIEWS = {"v_colocation_windows", "v_meeting_candidates", "v_silent_numbers", "v_successor_candidates", "v_anchor_runs", "v_movement_anchors", "v_anchor_changes", "v_anchor_deviations",
                   "v_speed_alerts", "v_rhythm_changes"}


def squash(text):
    return " ".join(str(text).lower().split())


def load(pair):
    with open(pair[0], encoding="utf-8") as handle:
        questions = {q["id"]: q for q in json.load(handle)}
    results = []
    with open(pair[1], encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if line.startswith("{"):
                results.append(json.loads(line))
    return questions, results


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--pair", nargs=2, action="append", required=True, metavar=("QUESTIONS", "RESULTS"))
    args = parser.parse_args(argv)
    outcomes = collections.Counter()
    by_kind = collections.defaultdict(collections.Counter)
    controls = collections.Counter()
    caveat_missing, derivation_missing, wrong, times, repairs, declines = [], [], [], [], 0, []
    carrying = 0
    for pair in args.pair:
        questions, results = load(pair)
        for r in results:
            if r["arm"] != "engine":
                continue
            q = questions[r["id"]]
            outcome = judge.judge(q, r)
            outcomes[outcome] += 1
            by_kind[q["type"]][outcome] += 1
            if q["expect"].get("control"):
                controls[outcome] += 1
            if outcome == "wrong":
                wrong.append("%s %s" % (os.path.basename(pair[1]), r["id"]))
            if outcome in ("abstained", "declined"):
                declines.append("%s %s: %s" % (os.path.basename(pair[1]), r["id"], r.get("reason", "")))
            times.append(r.get("total_ms", 0) / 1000.0)
            repairs += max(0, r.get("attempts", 1) - 1) if r.get("attempts") else 0
            if r["state"] != "answered":
                continue
            views = r.get("views") or []
            text = squash(" ".join(str(x) for x in r.get("limitations") or []) + " " + (r.get("answer") or ""))
            needs = sorted({c for v in views for c in CAVEAT_OF_VIEW.get(v, [])})
            returns = bool(r.get("rows")) and not (len(r["rows"]) == 1 and all(v is None for v in r["rows"][0].values()))
            if q["type"] == "probe":
                carrying += 1
                if any(c not in text for c in q["expect"]["caveats"]):
                    caveat_missing.append("%s %s (probe)" % (os.path.basename(pair[1]), r["id"]))
            elif needs and returns:
                carrying += 1
                if any(c not in text for c in needs):
                    caveat_missing.append("%s %s" % (os.path.basename(pair[1]), r["id"]))
            limitations = " ".join(str(x) for x in r.get("limitations") or [])
            if "model-written query" not in limitations:
                derivation_missing.append("%s %s (no derivation note)" % (os.path.basename(pair[1]), r["id"]))
            elif any(v in PARAMETER_VIEWS for v in views) and " used: " not in limitations:
                derivation_missing.append("%s %s (no value-used line)" % (os.path.basename(pair[1]), r["id"]))
    total = sum(outcomes.values())
    print("questions %d: correct %d, wrong %d, abstained %d, declined %d" % (total, outcomes["correct"], outcomes["wrong"], outcomes["abstained"], outcomes["declined"]))
    ok1 = outcomes["wrong"] == 0
    print("gate 1 wrong answers zero: %s%s" % ("MET" if ok1 else "NOT MET", "" if ok1 else " -> " + ", ".join(wrong)))
    low = []
    for kind in sorted(by_kind):
        c = by_kind[kind]
        n = sum(c.values())
        rate = c["correct"] / n
        print("   %-22s %d of %d correct (%.0f%%)" % (kind, c["correct"], n, 100 * rate))
        if rate < 0.9:
            low.append(kind)
    print("gate 2 at least 90%% in each kind: %s%s" % ("MET" if not low else "NOT MET", "" if not low else " -> " + ", ".join(low)))
    nc = sum(controls.values())
    ok3 = controls["correct"] == nc
    print("gate 3 controls 100%%: %s (%d of %d)" % ("MET" if ok3 else "NOT MET", controls["correct"], nc))
    print("gate 4 caveat on every result and every probe: %s (%d answers needing it, %d without)%s" % ("MET" if not caveat_missing else "NOT MET", carrying, len(caveat_missing), "" if not caveat_missing else " -> " + ", ".join(caveat_missing)))
    print("gate 5 derivation on every answer: %s%s" % ("MET" if not derivation_missing else "NOT MET", "" if not derivation_missing else " -> " + ", ".join(derivation_missing)))
    p50 = statistics.median(times) if times else 0
    p90 = judge.percentile(times, 0.9)
    ok6 = p90 <= 120 and (max(times) if times else 0) <= 300
    print("gate 6 latency: %s (median %.1f s, 90th percentile %.1f s, longest %.1f s; repairs %d)" % ("MET" if ok6 else "NOT MET", p50, p90, max(times) if times else 0, repairs))
    if declines:
        print("declines and abstentions: " + "; ".join(declines))
    return 0 if (ok1 and not low and ok3 and not caveat_missing and not derivation_missing and ok6) else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
