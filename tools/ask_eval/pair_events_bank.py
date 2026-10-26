#!/usr/bin/env python3
"""The question bank of the pair-events fix (docs/data/link-analysis-stage3-preregistration-20261022.md, Amendment 4), from the generator's oracle and the case's own records only.

    python -X utf8 tools/ask_eval/pair_events_bank.py --register-case --seed 112 --out S4-112.json   # the bank for one register case
    python -X utf8 tools/ask_eval/pair_events_bank.py --register-case --seed 2                       # development case 2: prints the counts only

Why it exists: a call between two SUBSCRIBERS is in both of their exports (outgoing in one, incoming in the other, at the same time), and the stage-1 bank paired a subscriber only with a
number that is not one, so "How many events are there between A and B?" was never asked of two subscribers; on the live service it answered 8 where the records hold 4.

Classes (the right answer is the events of the first subscriber's own records with the other, which equal the other's when the two exports agree; a pair whose exports disagree is not asked):
  pair_sub           two subscribers, all time, six pairs; the order in which the two numbers are named alternates (so the one named first is sometimes the other subscriber)
  pair_nonsub_first  the number named FIRST is not a subscriber (it has no records of its own): only the second number's records hold the events
  pair_month_sub     two subscribers, the events of one month (the case clock), read from the events themselves
  pair_control       two subscribers with no contact at all: the right answer is none, or zero
Wording: one of two written phrasings per class, chosen by the seed (wording is not tuned). Nothing is read from the engine. The script prints COUNTS only unless --out is given.
"""
import argparse
import collections
import json
import os
import random
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
sys.path.insert(0, os.path.join(HERE, ".."))
sys.path.insert(0, os.path.join(HERE, "..", ".."))
import case_set  # noqa: E402
import generate  # noqa: E402
import world as w  # noqa: E402

BETWEEN = "How many events are there between %s and %s?"
HAVE_WITH = "How many calls and messages did %s have with %s?"
MONTH = "How many events are there between %s and %s in %s?"  # "did A and B have in March" can mean the events of each, not between them (found on development case 2)
MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"]
PERSON_KINDS = ("person_like", "international")


def build(params):
    _, renders, key = generate.build_case(params)
    rng = random.Random(params.seed * 6151 + 29)
    subjects = [s["msisdn"] for s in key["subjects"]]
    person = {s["msisdn"]: s["person"] for s in key["subjects"]}
    visible = {m: renders[person[m]].visible for m in subjects}
    a1 = key["oracle"]["A1"]

    def row(x, y):
        found = [r for r in a1[x] if r["number"] == y]
        return found[0] if found else None

    def events(r):
        return r["incoming"] + r["outgoing"] + r["unknown"]

    def times(x, y):
        return sorted(r["start_local"] for r in visible[x] if r["b_norm"] == y)

    asked, dropped = [], collections.Counter()
    pairs = []
    for i, x in enumerate(subjects):
        for y in subjects[i + 1:]:
            rx, ry = row(x, y), row(y, x)
            if rx is None and ry is None:
                pairs.append((x, y, 0, None))
                continue
            if rx is None or ry is None or events(rx) != events(ry) or times(x, y) != times(y, x):
                dropped["exports differ"] += 1
                continue
            pairs.append((x, y, events(rx), times(x, y)))
    with_contact = [p for p in pairs if p[2] >= 3]
    without = [p for p in pairs if p[2] == 0]
    questions = []

    def add(kind, text, expect):
        questions.append({"id": "S4-%03d" % (len(questions) + 1), "type": kind, "question": text, "expect": expect})

    chosen = rng.sample(sorted(with_contact), min(6, len(with_contact)))
    for i, (x, y, n, _) in enumerate(chosen):
        first, second = (x, y) if i % 2 == 0 else (y, x)
        template = BETWEEN if (i // 2) % 2 == 0 else HAVE_WITH
        add("pair_sub", template % (first, second), {"kind": "scalar", "value": n})
    # the first number named is not a subscriber: only the subscriber's records hold the events
    outside = []
    for s in sorted(subjects):
        for r in a1[s]:
            if r["number"] not in subjects and r["kind"] in PERSON_KINDS and events(r) >= 3:
                outside.append((s, r["number"], events(r)))
    for i, (s, n, count) in enumerate(rng.sample(outside, min(2, len(outside)))):
        add("pair_nonsub_first", (BETWEEN if i % 2 == 0 else HAVE_WITH) % (n, s), {"kind": "scalar", "value": count})
    # one month of a pair of subscribers, from the events themselves
    month_pairs = []
    for x, y, n, when in with_contact:
        by_month = collections.Counter(t[:7] for t in when)
        month, count = max(by_month.items(), key=lambda kv: (kv[1], kv[0]))
        if count >= 2:
            month_pairs.append((x, y, month, count))
    for x, y, month, count in rng.sample(sorted(month_pairs), min(2, len(month_pairs))):
        add("pair_month_sub", MONTH % (x, y, "%s %s" % (MONTHS[int(month[5:7]) - 1], month[:4])), {"kind": "scalar", "value": count})
    if without:
        x, y, _, _ = rng.choice(sorted(without))
        add("pair_control", BETWEEN % (x, y), {"kind": "scalar", "value": 0, "empty_is_zero": True, "control": True})
    return questions, dropped


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--register-case", action="store_true")
    parser.add_argument("--out")
    args = parser.parse_args(argv)
    if args.register_case:
        found = [(kind, overrides) for kind, number, overrides in case_set.CASES if number == args.seed and kind != "sealed"]
        if not found:
            raise SystemExit("seed %d is not a development or judgement case of the register" % args.seed)
        params = case_set.params_for(args.seed, found[0][1])
    else:
        params = w.Params(seed=args.seed)
    questions, dropped = build(params)
    kinds = collections.Counter(q["type"] for q in questions)
    print("%d questions: %s; pairs not asked because the two exports disagree: %d" % (len(questions), dict(kinds), dropped["exports differ"]))
    if args.out:
        with open(args.out, "w", encoding="utf-8") as handle:
            json.dump(questions, handle, indent=1)
            handle.write("\n")
        print("written", args.out)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
