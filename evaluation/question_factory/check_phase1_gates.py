#!/usr/bin/env python3
"""Phase 1 gates S1 to S4 and S6, and the corpus and pre-flight reading (S5), applied to saved results
(reports/governed-sql-20261005/PREREGISTRATION.md, "Phase 1 plan"). Run 14 repeats them unchanged ("Run 14 plan", R14-2 to R14-5).

    python check_phase1_gates.py arms      --a arm-lane-A-mm --c arm-lane-C-mm13 --b arm-lane-B-mmp2 [--prev arm-lane-B-mmp1]
    python check_phase1_gates.py corpus    --off replay-lane-A --on replay-lane-B-p2 [--convention CDR-09 --convention H13-HONESTY-NOPLATE]
    python check_phase1_gates.py preflight --off preflight-lane-A --on preflight-lane-B-p2

`arms` reads arms under evaluation/question_factory; `corpus` and `preflight` read the folders under reports/free-question-baseline-20261002.
The gates were written before anything was measured; this only applies them, and reports what the comparison tool of the corpus does not:

* the corpus tool scores numeric checks only. Here the text-match checks (`contains`, and `top`) are scored too, with the same rule: every
  expected string present, commas ignored;
* an abstention never counts as a pass, even when its message happens to contain the expected string (Phase 1: `ACCE-top_group-01` and
  `P2-PROVENANCE-AUDIO-MIX` were scored correct by the judge for quoting the value they declined to answer);
* a lane answer whose check fails is listed for reading: it is the candidate for a confident wrong answer the checks cannot name.
"""
import argparse
import io
import json
import math
import os
import re
import statistics
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)  # `python -I` leaves the script's folder off the path

import factory as f  # noqa: E402
BASELINE = os.path.normpath(os.path.join(HERE, "..", "..", "reports", "free-question-baseline-20261002"))
NAME_INTENTS = ("unbound_name", "absent_value")
ABSTAIN_START = re.compile(r"^\s*I did not run this question", re.I)


def load_results(arm):
    with io.open(os.path.join(HERE, arm, "results.json"), encoding="utf-8") as handle:
        return {r["id"]: r for r in json.load(handle)}


def lane_state(row):
    return f.lane_fields(row.get("lane")).get("state")


def counts(rows):
    out = {}
    for r in rows.values():
        out[r["verdict"]] = out.get(r["verdict"], 0) + 1
    return out


# ---------------------------------------------------------------- arms: S1 to S4, S6

def thresholds(arm_a):
    """S1: correct at least arm A's plus 15 points of the questions; confident wrong at most half of arm A's and at most 10% of the questions."""
    n = len(arm_a)
    c = counts(arm_a)
    return c.get("CORRECT", 0) + math.ceil(0.15 * n), min(int(0.10 * n), c.get("WRONG", 0) // 2)


def has_reason(row):
    """An abstention names what it could not apply: the lane says so in its message, and so does the existing path's own wording."""
    return bool(row["text"].strip()) and bool(f.ABSTAIN.search(row["text"]) or ABSTAIN_START.search(row["text"]))


def gate_s1(arm_a, arm_b):
    need_ok, max_wrong = thresholds(arm_a)
    c = counts(arm_b)
    ok, wrong = c.get("CORRECT", 0), c.get("WRONG", 0)
    silent = [r["id"] for r in arm_b.values() if r["verdict"] == "ABSTAINED" and not has_reason(r)]
    passed = ok >= need_ok and wrong <= max_wrong and not silent
    return {"pass": passed, "correct": ok, "need_correct": need_ok, "wrong": wrong, "max_wrong": max_wrong, "abstentions_without_reason": silent}


def moves(prev, cur):
    """Verdict changes of the questions both arms hold. Returns (correct to wrong, correct to abstained, gained, other changes)."""
    ids = [i for i in cur if i in prev]
    c2w = [i for i in ids if prev[i]["verdict"] == "CORRECT" and cur[i]["verdict"] == "WRONG"]
    c2a = [i for i in ids if prev[i]["verdict"] == "CORRECT" and cur[i]["verdict"] == "ABSTAINED"]
    gained = [i for i in ids if prev[i]["verdict"] != "CORRECT" and cur[i]["verdict"] == "CORRECT"]
    other = [i for i in ids if prev[i]["verdict"] != cur[i]["verdict"] and i not in c2w + c2a + gained]
    return c2w, c2a, gained, other


def gate_s2(prev, cur):
    c2w, c2a, gained, other = moves(prev, cur)
    return {"pass": not c2w and len(c2a) <= 3, "correct_to_wrong": c2w, "correct_to_abstained": c2a, "gained": len(gained), "other": other}


def gate_s3(arm_b):
    mine = [r for r in arm_b.values() if r["intent"] in NAME_INTENTS]
    bad = [r["id"] for r in mine if r["verdict"] not in ("CORRECT", "ABSTAINED")]
    return {"pass": not bad, "questions": len(mine), "not_correct_or_abstained": bad}


def gate_s4(arm_b):
    other = {r["id"]: r["verdict"] for r in arm_b.values() if r["verdict"] not in ("CORRECT", "WRONG", "ABSTAINED")}
    return {"pass": not other, "errors": other}


def credited_abstentions(arm_b):
    """The judge scored these CORRECT, and the lane did not answer: its abstention quoted the value."""
    return [r["id"] for r in arm_b.values() if lane_state(r) == "abstained" and r["verdict"] == "CORRECT"]


def quantiles(values):
    if not values:
        return None
    return {"n": len(values), "median": round(statistics.median(values), 1), "p90": round(f.percentile(values, 0.9), 1), "max": round(max(values), 1)}


def timing(arm_b):
    answered = [r for r in arm_b.values() if lane_state(r) == "answered"]
    model = [int(f.lane_fields(r.get("lane")).get("model_ms", "0")) / 1000.0 for r in answered]
    return {"all": quantiles([r["seconds"] for r in arm_b.values()]), "lane_answered": quantiles([r["seconds"] for r in answered]), "lane_model": quantiles(model)}


def run_arms(args):
    a, c, b = load_results(args.a), load_results(args.c), load_results(args.b)
    print("arm A %s: %s\nstage 1 %s: %s\nthis arm %s: %s" % (args.a, counts(a), args.c, counts(c), args.b, counts(b)))
    s1, s2, s3, s4 = gate_s1(a, b), gate_s2(c, b), gate_s3(b), gate_s4(b)
    print("\nS1 accuracy       %s  correct %d (need %d), wrong %d (need <= %d), abstentions without a reason: %s"
          % ("PASS" if s1["pass"] else "FAIL", s1["correct"], s1["need_correct"], s1["wrong"], s1["max_wrong"], s1["abstentions_without_reason"] or "none"))
    print("S2 not worse (vs stage 1) %s  correct to wrong %s, correct to abstained %s, gained %d, other moves %s"
          % ("PASS" if s2["pass"] else "FAIL", s2["correct_to_wrong"], s2["correct_to_abstained"], s2["gained"], s2["other"]))
    ok = s1["pass"] and s2["pass"]
    if args.prev:
        s2p = gate_s2(load_results(args.prev), b)
        print("R14-3 not worse (vs %s) %s  correct to wrong %s, correct to abstained %s, gained %d, other moves %s"
              % (args.prev, "PASS" if s2p["pass"] else "FAIL", s2p["correct_to_wrong"], s2p["correct_to_abstained"], s2p["gained"], s2p["other"]))
        ok = ok and s2p["pass"]
    print("S3 no silent drop %s  %d name and absent-value questions, not correct or abstained: %s" % ("PASS" if s3["pass"] else "FAIL", s3["questions"], s3["not_correct_or_abstained"] or "none"))
    print("S4 instrument     %s  verdicts other than correct, wrong, abstained: %s" % ("PASS" if s4["pass"] else "FAIL", s4["errors"] or "none"))
    credited = credited_abstentions(b)
    if credited:
        print("\nREADING NOTE: %d abstention(s) scored CORRECT because the message quotes the key: %s. Counted as abstentions the arm is %d correct."
              % (len(credited), credited, s1["correct"] - len(credited)))
    print("\nS6 time (seconds): %s" % json.dumps(timing(b)))
    n = len(b)
    c_b = counts(b)
    print("goal reading (not gated): correct %.1f%%, confident wrong %.1f%%" % (100.0 * c_b.get("CORRECT", 0) / n, 100.0 * c_b.get("WRONG", 0) / n))
    wrong = [r for r in b.values() if r["verdict"] == "WRONG"]
    for r in wrong:
        print("  WRONG %-24s %s | key %s | %s" % (r["id"], r["intent"], r.get("expected"), re.sub(r"\s+", " ", r["text"])[:110]))
    return 0 if (ok and s3["pass"] and s4["pass"]) else 1


# ---------------------------------------------------------------- the corpus (S5)

def has(text, token):
    t, s = (text or "").lower(), str(token).lower()
    return s in t or s in t.replace(",", "")


def is_abstention(row):
    route = row.get("route") or []
    return "clarification" in route or "verified_only_withheld" in route or bool(ABSTAIN_START.search(row.get("answer_text") or ""))


def score(row):
    """True or False for a check that can be scored by text (number, contains, top), None for the others. An abstention never passes."""
    kind, expect, text = row.get("check"), row.get("expect"), row.get("answer_text") or ""
    if kind == "number":
        passed = bool(row.get("stated"))
    elif kind == "contains":
        passed = all(has(text, e) for e in (expect if isinstance(expect, list) else [expect]))
    elif kind == "top":
        passed = any(has(text, w) for w in (expect if isinstance(expect, list) else [expect]))
    else:
        return None
    return False if passed and is_abstention(row) else passed


def load_replay(arm):
    with io.open(os.path.join(BASELINE, arm, "replay.json"), encoding="utf-8") as handle:
        return {r["id"]: r for r in json.load(handle)}


def norm(text):
    return re.sub(r"\s+", " ", text or "").strip()


def compare_corpus(off, on):
    ids = [i for i in off if i in on]
    lost, gained, both_ok, both_bad = [], [], [], []
    for i in ids:
        a, b = score(off[i]), score(on[i])
        if a is None or b is None:
            continue
        if a and not b:
            lost.append(i)
        elif b and not a:
            gained.append(i)
        elif a:
            both_ok.append(i)
        else:
            both_bad.append(i)
    changed = [i for i in ids if norm(off[i]["answer_text"]) != norm(on[i]["answer_text"])]
    errors = [i for i in ids if off[i]["http"] != 200 or on[i]["http"] != 200]
    answered_failed = [i for i in ids if "governed_sql" in (on[i].get("route") or []) and not is_abstention(on[i]) and score(on[i]) is False]
    answered_by_lane = [i for i in ids if "governed_sql" in (on[i].get("route") or []) and not is_abstention(on[i])]
    return {"lost": lost, "gained": gained, "both_ok": both_ok, "both_bad": both_bad, "changed": changed, "errors": errors,
            "lane_answered": answered_by_lane, "lane_answered_failed": answered_failed}


def run_corpus(args):
    off, on = load_replay(args.off), load_replay(args.on)
    result = compare_corpus(off, on)
    conventions = set(args.convention or [])
    open_losses = [i for i in result["lost"] if i not in conventions]
    print("compared %d questions (%s vs %s)" % (len([i for i in off if i in on]), args.off, args.on))
    print("scored checks: pass in both %d, fail in both %d" % (len(result["both_ok"]), len(result["both_bad"])))
    print("LOST   (pass in %s, fail now): %d %s" % (args.off, len(result["lost"]), result["lost"]))
    print("   of which documented conventions or artefacts: %s" % sorted(set(result["lost"]) & conventions))
    print("GAINED (fail in %s, pass now): %d %s" % (args.off, len(result["gained"]), result["gained"]))
    print("answer text changed: %d; HTTP errors: %s; answered by the lane: %d" % (len(result["changed"]), result["errors"] or "none", len(result["lane_answered"])))
    if result["lane_answered_failed"]:
        print("\nLANE ANSWERS WHOSE CHECK FAILS (read each: a confident wrong answer, or an expectation older than the case clock):")
        for i in result["lane_answered_failed"]:
            print("  %-26s %s\n      expect %s | %s" % (i, off[i]["question"][:70], json.dumps(on[i].get("expect"), ensure_ascii=False)[:60], norm(on[i]["answer_text"])[:150]))
    if args.prev:
        prev = load_replay(args.prev)
        worse = [i for i in prev if i in on and score(prev[i]) and score(on[i]) is False]
        print("\nR14-4 vs %s: scored checks that passed there and fail now: %s" % (args.prev, worse or "none"))
        open_losses = sorted(set(open_losses) | {i for i in worse if i not in conventions})
    gate = not open_losses and not result["errors"]
    print("\nS5 corpus %s  (losses outside the documented conventions: %s)" % ("PASS" if gate else "FAIL", open_losses or "none"))
    return 0 if gate else 1


# ---------------------------------------------------------------- the pre-flight

def run_preflight(args):
    def load(arm):
        with io.open(os.path.join(BASELINE, arm, "results.json"), encoding="utf-8") as handle:
            return json.load(handle)
    off, on = load(args.off), load(args.on)
    index = {r["question"]: r for r in off}
    print("pre-flight: %s passed %d of %d; %s passed %d of %d" % (args.off, sum(r["pass"] for r in off), len(off), args.on, sum(r["pass"] for r in on), len(on)))
    errors = [r["question"] for r in on if str(r.get("why", "")).startswith("ERROR")]
    for r in on:
        if not r["pass"]:
            before = index.get(r["question"], {})
            print("\nFAIL [%s] %s\n   was [%s] %s\n   now [%s] %s\n   why: %s"
                  % (r["group"], r["question"], ",".join(before.get("route") or []), norm(before.get("text"))[:130], ",".join(r.get("route") or []), norm(r.get("text"))[:160], r.get("why")))
    print("\nerrors: %s" % (errors or "none"))
    return 0 if not errors else 1


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="mode", required=True)
    arms = sub.add_parser("arms")
    arms.add_argument("--a", required=True, help="arm A (the lane off), the control")
    arms.add_argument("--c", required=True, help="the stage 1 arm of the same set (run 13 arm C)")
    arms.add_argument("--b", required=True, help="the lane-first arm under test")
    arms.add_argument("--prev", help="the previous lane-first arm of the same set (Phase 1), for the run 14 'nothing right gets worse' gate")
    corpus = sub.add_parser("corpus")
    corpus.add_argument("--off", required=True)
    corpus.add_argument("--on", required=True)
    corpus.add_argument("--prev")
    corpus.add_argument("--convention", action="append", help="a question whose loss is documented (a clock expectation, a checker artefact)")
    pre = sub.add_parser("preflight")
    pre.add_argument("--off", required=True)
    pre.add_argument("--on", required=True)
    args = parser.parse_args(argv)
    return {"arms": run_arms, "corpus": run_corpus, "preflight": run_preflight}[args.mode](args)


if __name__ == "__main__":
    sys.exit(main())
