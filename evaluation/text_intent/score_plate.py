#!/usr/bin/env python3
"""Score the plate-routing decisions against Amendment 3 of docs/roadmap/text-intent-preregistration-20261110.md.

    python -I -X utf8 evaluation/text_intent/score_plate.py --bank evaluation/text_intent/bank-plate-20261110.json --decisions C:/tmp/ti-plate-run1.json

P1 routing and fidelity, P2 false routes, P4 time are decided here; P3 (no regression on words) uses score_tier1.py on development bank 2 plus `plate_routed_words` below.
"""
import argparse
import json
import re
import statistics


def key(text):
    return re.sub(r"[^A-Z0-9]", "", (text or "").upper())


def percentile(values, share):
    ordered = sorted(values)
    return ordered[min(len(ordered) - 1, int(round(share * (len(ordered) - 1))))] if ordered else 0


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--bank", required=True)
    parser.add_argument("--decisions", required=True)
    parser.add_argument("--words-decisions", help="decisions of the word-search bank (dev bank 2) to count any routed to plate")
    args = parser.parse_args()
    bank = json.load(open(args.bank, encoding="utf-8"))
    decisions = {d["id"]: d for d in json.load(open(args.decisions, encoding="utf-8"))}
    ok = [p for p in bank["positives"] if decisions[p["id"]]["state"] == "ROUTED" and decisions[p["id"]]["family"] == "plate" and decisions[p["id"]]["literal"] == key(p["plate"])]
    routed_wrong_key = [p for p in bank["positives"] if decisions[p["id"]]["state"] == "ROUTED" and decisions[p["id"]]["family"] == "plate" and decisions[p["id"]]["literal"] != key(p["plate"])]
    false_routes = [n for n in bank["negatives"] if decisions[n["id"]]["state"] == "ROUTED" and decisions[n["id"]]["family"] == "plate"]
    share = len(ok) / len(bank["positives"])
    print(f"P1 routed to plate with the plate as typed: {len(ok)}/{len(bank['positives'])} = {share:.1%} (gate >= 90%) -> {'MET' if share >= .9 else 'NOT MET'}")
    print(f"   routed to plate with a different key: {len(routed_wrong_key)}")
    print(f"P2 plate negatives routed to plate: {len(false_routes)}/{len(bank['negatives'])} (gate 0) -> {'MET' if not false_routes else 'NOT MET'}")
    times = [d["ms"] for d in decisions.values() if d.get("ms")]
    median, p95 = statistics.median(times) / 1000, percentile(times, .95) / 1000
    print(f"P4 time per question: median {median:.1f} s (gate <= 8), p95 {p95:.1f} s (gate <= 25) -> {'MET' if median <= 8 and p95 <= 25 else 'NOT MET'}")
    by_shape = {}
    for p in bank["positives"]:
        by_shape.setdefault(p["shape"], [0, 0])
        by_shape[p["shape"]][1] += 1
        by_shape[p["shape"]][0] += p in ok
    print("   by plate shape:", {k: f"{a}/{b}" for k, (a, b) in by_shape.items()})
    states = {}
    for d in decisions.values():
        states[d["state"]] = states.get(d["state"], 0) + 1
    print("   states:", states)
    for p in bank["positives"]:
        if p not in ok:
            d = decisions[p["id"]]
            print(f"   missed {p['id']} [{p['shape']}] {d['state']} {d.get('reason', '')} about={d.get('about')} proposed={d.get('proposed_literal')!r} :: {p['question']}")
    for n in false_routes:
        print(f"   FALSE ROUTE {n['id']} :: {n['question']}")
    if args.words_decisions:
        words = json.load(open(args.words_decisions, encoding="utf-8"))
        wrong = [d for d in words if d["state"] == "ROUTED" and d["family"] == "plate"]
        plate_kind = [d for d in words if d.get("about") == "plate_in_images_or_video"]
        print(f"P3 word-bank questions the model called a plate question: {len(plate_kind)} (routed to plate: {len(wrong)}) (gate 0)")
        for d in plate_kind:
            print(f"   {d['id']} {d['state']} :: {d['question']}")


if __name__ == "__main__":
    main()
