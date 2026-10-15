#!/usr/bin/env python3
"""Builds the Ask engine's evaluation questions from the planted-truth keys of a GENERATED case (no real data anywhere).

    python -X utf8 tools/ask_eval/build_questions.py [--seed 61] [--out tools/ask_eval/questions-seed61.json]

The case is the one tools/case_generator/check_ask_engine.py loads (same parameters and seed), so every identifier in a question is a number of that case and every expected
answer comes from the generator's oracle (the independent plain-Python implementation of the analyses), never from the SQL the engine or the lane writes. Each question carries
its expectation:

    scalar   one number                                    the answer must be exactly that value
    set      a set of identifiers                          the identifiers in the result must be exactly that set
    ordered  an ordered list of identifiers                the first rows must be those identifiers in that order
    absent   (no value)                                    the answer must say that no record exists, not give a number
    path     whether a connection exists within N hops     rows exist exactly when a path exists
    none     (no value)                                    a question the data cannot answer: the right outcome is an abstention or a decline, never an answer

Each type has two phrasings (the subjects alternate between them) so that the result is not the accident of one sentence. Run the judge with tools/ask_eval/judge.py.
"""
import argparse
import collections
import json
import os
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
import generate  # noqa: E402
import world as w  # noqa: E402

PARAMS = dict(people=60, subjects=6, associates=12, period_days=20, p2_pairs=3, p3_pairs=2, p7_plants=3)
SERVICE_KINDS = {"short_code", "sender_id", "service_label"}


def load_key(seed):
    tmp = tempfile.mkdtemp()
    root = generate.write_case(w.Params(seed=seed, **PARAMS), tmp)
    with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
        return json.load(handle)


def events(row):
    return row["incoming"] + row["outgoing"] + row["unknown"]


def build(key):
    oracle = key["oracle"]
    a1 = oracle["A1"]
    threshold = oracle["parameters"]["hub_threshold"]
    subjects = [s["msisdn"] for s in key["subjects"]]
    reached_by = collections.Counter()
    for lst in a1.values():
        for r in lst:
            reached_by[r["number"]] += 1
    hubs = {n for n, c in reached_by.items() if c > threshold}
    ordinary = lambda s: [r for r in a1[s] if r["kind"] not in SERVICE_KINDS and r["number"] not in hubs]  # noqa: E731
    questions = []

    def add(kind, text, expect, **extra):
        questions.append(dict(id="Q%03d" % (len(questions) + 1), type=kind, question=text, expect=expect, **extra))

    # 1. one pair's event count (v_contact_pairs.events)
    for i, s in enumerate(subjects):
        pairs = [r for r in ordinary(s) if events(r) >= 3]
        if not pairs:
            continue
        r = pairs[0]
        text = ("How many events are there between %s and %s?" if i % 2 == 0 else "How many calls and messages did %s have with %s?") % (s, r["number"])
        add("pair_count", text, {"kind": "scalar", "value": events(r)})
    # 2. incoming from one number
    for i, s in enumerate(subjects):
        pairs = [r for r in ordinary(s) if r["incoming"] >= 2]
        if not pairs:
            continue
        r = pairs[0]
        text = ("How many incoming events did %s receive from %s?" if i % 2 == 0 else "How many times did %s get a call or message from %s?") % (s, r["number"])
        add("incoming_count", text, {"kind": "scalar", "value": r["incoming"]})
    # 3. top contacts, services and hubs left out
    for i, s in enumerate(subjects):
        ranked = sorted(ordinary(s), key=lambda r: -events(r))
        n = 3
        if len(ranked) <= n or events(ranked[n - 1]) == events(ranked[n]):
            continue
        text = ("Who does %s talk to most? Give the top three, leaving out services and numbers that everyone contacts." if i % 2 == 0
                else "List the three numbers %s has the most events with, not counting short codes or numbers that everybody contacts.") % s
        add("top_contacts", text, {"kind": "ordered", "values": [r["number"] for r in ranked[:n]]})
    # 4. common contacts (A2)
    for i, answer in enumerate(a for a in oracle["A2"] if a["shared"]):
        text = ("Which numbers do %s and %s both contact, leaving out services and numbers that everyone contacts?" if i % 2 == 0
                else "What contacts do %s and %s have in common, excluding short codes and numbers that everyone contacts?") % (answer["x"], answer["y"])
        add("common_contacts", text, {"kind": "set", "values": sorted(s["number"] for s in answer["shared"])})
    # 5. how many different ordinary numbers
    for i, s in enumerate(subjects[:4]):
        text = ("How many different numbers did %s contact, leaving out services?" if i % 2 == 0 else "How many distinct phone numbers has %s been in contact with, excluding short codes?") % s
        add("distinct_contacts", text, {"kind": "scalar", "value": len([r for r in a1[s] if r["kind"] not in SERVICE_KINDS])})
    # 6. a number the case does not hold
    for i, n in enumerate(["923000099999", "923000099998"]):
        text = ("How many calls does %s have?" if i % 2 == 0 else "How many events involve the number %s?") % n
        add("absent", text, {"kind": "absent"})
    # 7. handsets of a number that changed handset (A8)
    for i, (s, entry) in enumerate(sorted(oracle["A8"]["numbers"].items())):
        if not entry["changes"]:
            continue
        imeis = sorted({c["old_imei"] for c in entry["changes"]} | {c["new_imei"] for c in entry["changes"]})
        text = ("Which handsets (IMEI) has %s been used with?" if i % 2 == 0 else "List every IMEI seen with %s.") % s
        add("devices", text, {"kind": "set", "values": imeis})
    # 8. who held an address at an instant (A13), unique and ambiguous instants
    for i, a in enumerate(oracle["A13"][:6]):
        text = ("Who held the public address %s port %d at %s?" if i % 2 == 0 else "Which subscribers were using %s port %d at %s?") % (a["public_ip"], a["port"], a["instant"])
        add("address_holder", text, {"kind": "set", "values": sorted(a["holders"])}, plant=a["plant"], result=a["result"])
    # 9. a connection within three hops, over the subjects' own records (directed, services and hubs not passed through)
    edges = collections.defaultdict(set)
    for s in subjects:
        for r in a1[s]:
            if r["kind"] not in SERVICE_KINDS:
                edges[s].add(r["number"])
    def distance(a, b, limit=3):
        frontier, seen = {a}, {a}
        for d in range(1, limit + 1):
            nxt = set()
            for n in frontier:
                for m in edges.get(n, ()):
                    if m == b:
                        return d
                    if m not in seen and m not in hubs:
                        seen.add(m)
                        nxt.add(m)
            frontier = nxt
        return None
    seen_pairs = 0
    for a in subjects:
        for b in subjects:
            if a < b and seen_pairs < 4:
                d = distance(a, b)
                text = "How are %s and %s connected within three hops, not through services or hubs?" % (a, b)
                add("path", text, {"kind": "path", "exists": d is not None, "distance": d})
                seen_pairs += 1
    # 10. questions the data cannot answer
    for text in ["What is the home address of %s?" % subjects[0], "Who owns the number %s?" % subjects[1], "Is %s a criminal?" % subjects[2]]:
        add("none", text, {"kind": "none"})
    return questions


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--seed", type=int, default=61)
    parser.add_argument("--out", default=os.path.join(HERE, "questions-seed61.json"))
    args = parser.parse_args(argv)
    questions = build(load_key(args.seed))
    with open(args.out, "w", encoding="utf-8") as handle:
        json.dump(questions, handle, indent=1)
        handle.write("\n")
    counts = collections.Counter(q["type"] for q in questions)
    print("%d questions: %s -> %s" % (len(questions), ", ".join("%s %d" % kv for kv in sorted(counts.items())), args.out))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
