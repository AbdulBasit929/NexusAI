#!/usr/bin/env python3
"""Score a run of the text-intent step on the video development bank (Amendment 7).

    python -I -X utf8 evaluation/text_intent/score_video.py --bank evaluation/text_intent/bank-video-20261112.json --decisions C:/tmp/ti-video-run1.json

V1: negatives (questions about videos and their readings) routed as a search for text or for a plate: should be 0.
V2: typed-plate positives routed to the plate search with the plate exactly as typed (letters and digits): should stay high; a file name in the question must not stop it.
Reported, not a gate: development bank. The questions are printed with the model's `about` so a miss can be analysed.
"""
import argparse
import collections
import json
import re


def key(text):
    return re.sub(r"[^A-Za-z0-9]", "", text or "").upper()


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--bank", required=True)
    p.add_argument("--decisions", required=True)
    a = p.parse_args()
    bank = json.load(open(a.bank, encoding="utf-8"))
    plates = {q["id"]: q["plate"] for q in bank["positives"]}
    decisions = json.load(open(a.decisions, encoding="utf-8"))
    neg = [d for d in decisions if d["id"].startswith("TI-V-N")]
    pos = [d for d in decisions if d["id"].startswith("TI-V-P")]
    routed_neg = [d for d in neg if d["state"] == "ROUTED"]
    print("V1 negatives routed as a search: %d of %d" % (len(routed_neg), len(neg)))
    print("   about of the negatives:", dict(collections.Counter(d["about"] for d in neg)))
    for d in routed_neg:
        print("   ROUTED  %-12s about=%-22s %s" % (d["id"], d["about"], d["question"]))
    ok = [d for d in pos if d["state"] == "ROUTED" and d["family"] == "plate" and key(d["literal"]) == key(plates[d["id"]])]
    print("V2 typed plates routed with the plate as typed: %d of %d" % (len(ok), len(pos)))
    for d in pos:
        if d not in ok:
            print("   MISS  %-12s state=%s about=%s reason=%s | %s" % (d["id"], d["state"], d["about"], d.get("reason", ""), d["question"]))
    ms = sorted(d["ms"] for d in decisions)
    print("time: median %d ms, p95 %d ms" % (ms[len(ms) // 2], ms[int(0.95 * (len(ms) - 1))]))


if __name__ == "__main__":
    main()
