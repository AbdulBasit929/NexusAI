#!/usr/bin/env python3
"""Score the Tier 1 decisions of the text-search intent step against the gates of docs/roadmap/text-intent-preregistration-20261110.md.

    python -I -X utf8 evaluation/text_intent/score_tier1.py --bank evaluation/text_intent/bank-20261110.json --decisions C:/tmp/ti-tier1-run1.json [--out report.md]

G1 routing, G3 literal fidelity, G4 false routes and G5 added time are decided here; G2 (answers) is Tier 2, G6 is a unit test, G7 is Tier 3.
"""
import argparse
import collections
import json
import statistics
import unicodedata


def norm(text):
    return unicodedata.normalize("NFC", " ".join((text or "").split())).casefold()


def percentile(values, share):
    ordered = sorted(values)
    return ordered[min(len(ordered) - 1, int(round(share * (len(ordered) - 1))))] if ordered else 0


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--bank", required=True)
    parser.add_argument("--decisions", required=True)
    parser.add_argument("--out")
    args = parser.parse_args()
    bank = json.load(open(args.bank, encoding="utf-8"))
    decisions = {d["id"]: d for d in json.load(open(args.decisions, encoding="utf-8"))}
    lines, misses = [], []

    named = [p for p in bank["positives"] if p["expected_family"] != "any"]
    free = [p for p in bank["positives"] if p["expected_family"] == "any"]
    named_ok = [p for p in named if decisions[p["id"]]["state"] == "ROUTED" and decisions[p["id"]]["family"] == p["expected_family"]]
    routed = [p for p in bank["positives"] if decisions[p["id"]]["state"] == "ROUTED"]
    g1a = len(named_ok) / len(named)
    g1b = len(routed) / len(bank["positives"])
    lines.append(f"G1a routed to the named family: {len(named_ok)}/{len(named)} = {g1a:.1%} (gate >= 90%) -> {'MET' if g1a >= .9 else 'NOT MET'}")
    lines.append(f"G1b routed at all (any family): {len(routed)}/{len(bank['positives'])} = {g1b:.1%} (gate >= 90%) -> {'MET' if g1b >= .9 else 'NOT MET'}")
    by_family = collections.defaultdict(lambda: [0, 0])
    for p in named:
        by_family[p["expected_family"]][1] += 1
        by_family[p["expected_family"]][0] += p in named_ok
    lines.append("   by named family: " + ", ".join(f"{k} {v[0]}/{v[1]}" for k, v in sorted(by_family.items())))
    free_ok = [p for p in free if decisions[p["id"]]["state"] == "ROUTED" and decisions[p["id"]]["family"] == "any"]
    lines.append(f"   source-free sentences routed as 'any': {len(free_ok)}/{len(free)}; routed to a specific family instead: "
                 f"{sum(1 for p in free if decisions[p['id']]['state'] == 'ROUTED' and decisions[p['id']]['family'] != 'any')}")

    fidelity_bad = [p for p in routed if norm(decisions[p["id"]]["literal"]) != norm(p["literal"])]
    lines.append(f"G3 literal fidelity: {len(routed) - len(fidelity_bad)}/{len(routed)} routed searched the expected literal (gate 100%) -> {'MET' if not fidelity_bad else 'NOT MET'}")
    for p in fidelity_bad:
        misses.append(f"  literal  {p['id']}: expected {p['literal']!r}, searched {decisions[p['id']]['literal']!r}")

    plain = [n for n in bank["negatives"] if n["kind"] == "plain"]
    hard = [n for n in bank["negatives"] if n["kind"] == "hard"]
    false_routes = [n for n in plain if decisions[n["id"]]["state"] == "ROUTED"]
    lines.append(f"G4 false routes on plain negatives: {len(false_routes)}/{len(plain)} (gate 0) -> {'MET' if not false_routes else 'NOT MET'}")
    for n in false_routes:
        misses.append(f"  false route {n['id']}: {n['question']!r} -> {decisions[n['id']]['family']} {decisions[n['id']]['literal']!r}")
    hard_routed = [n for n in hard if decisions[n["id"]]["state"] == "ROUTED"]
    lines.append(f"   hard negatives routed (reported, not gated): {len(hard_routed)}/{len(hard)}: " + "; ".join(f"{n['question']!r}" for n in hard_routed))

    times = [d["ms"] for d in decisions.values() if d["state"] != "UNAVAILABLE"]
    warm = times[1:] if len(times) > 1 else times
    med, p95 = statistics.median(warm), percentile(warm, .95)
    lines.append(f"G5 added time per question (model call, {len(warm)} warm calls): median {med / 1000:.1f} s (gate <= 8), p95 {p95 / 1000:.1f} s (gate <= 25) -> "
                 f"{'MET' if med <= 8000 and p95 <= 25000 else 'NOT MET'}; slowest {max(times) / 1000:.1f} s, first call {times[0] / 1000:.1f} s")

    states = collections.Counter(d["state"] for d in decisions.values())
    reasons = collections.Counter(d.get("reason", "") for d in decisions.values() if d["state"] in ("REJECTED", "UNAVAILABLE"))
    lines.append(f"states: {dict(states)}; refusal reasons: {dict(reasons)}")
    for p in named:
        d = decisions[p["id"]]
        if p not in named_ok:
            misses.append(f"  missed {p['id']} [{p['expected_family']}] {d['state']} {d.get('reason', '')} kind={d['kind']} family={d['family']} proposed={d['proposed_literal']!r} :: {p['question']}")
    for p in free:
        d = decisions[p["id"]]
        if p not in free_ok and d["state"] != "ROUTED":
            misses.append(f"  missed {p['id']} [any] {d['state']} {d.get('reason', '')} kind={d['kind']} proposed={d['proposed_literal']!r} :: {p['question']}")
    report = "\n".join(lines + ["", "misses:"] + (misses or ["  none"]))
    print(report)
    if args.out:
        open(args.out, "w", encoding="utf-8", newline="\n").write(report + "\n")


if __name__ == "__main__":
    main()
