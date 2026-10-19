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
    path     whether a connection exists within N hops     rows exist exactly when a path exists, every row is a real contact, and the answer names the numbers between the two (D22)
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


def with_check_digit(imei14):
    total = 0
    for i, ch in enumerate(reversed(imei14)):
        d = int(ch)
        if i % 2 == 0:
            d *= 2
            if d > 9:
                d -= 9
        total += d
    return imei14 + str((10 - total % 10) % 10)


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
        # `scores` lets the judge accept any order among numbers with equal counts (a tie inside the top three has no single right order).
        add("top_contacts", text, {"kind": "ordered", "values": [r["number"] for r in ranked[:n]], "scores": {r["number"]: events(r) for r in ranked}})
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
        # The key holds the 14-digit identifier. Some provider layouts store the 14 digits and others the 15-digit IMEI (the same digits and the Luhn check digit), so the
        # expectation is the 14 digits and the judge compares the first 14 digits of any 15-digit value (found 2026-10-15, in two measured runs).
        imeis = sorted({c["old_imei"] for c in entry["changes"]} | {c["new_imei"] for c in entry["changes"]})
        text = ("Which handsets (IMEI) has %s been used with?" if i % 2 == 0 else "List every IMEI seen with %s.") % s
        add("devices", text, {"kind": "set", "values": imeis})
    # 8. who held an address at an instant (A13), unique and ambiguous instants
    for i, a in enumerate(oracle["A13"][:6]):
        text = ("Who held the public address %s port %d at %s?" if i % 2 == 0 else "Which subscribers were using %s port %d at %s?") % (a["public_ip"], a["port"], a["instant"])
        add("address_holder", text, {"kind": "set", "values": sorted(a["holders"])}, plant=a["plant"], result=a["result"])
    # 9. connections (decision D22, 2026-10-16): two numbers are connected when they are in contact with each other or through other numbers, in EITHER direction, so
    #    two subscribers who both contact the same number are connected through that shared contact. Services and numbers everyone contacts are not passed through.
    #    The oracle is a breadth-first search over the subscribers' own ordinary contacts; each question carries the distance, the numbers on the shortest routes
    #    (`via`, which the answer must name) and the real edges (`graph`, which the returned rows must all belong to).
    adjacent = collections.defaultdict(set)
    for s in subjects:
        for r in ordinary(s):
            adjacent[s].add(r["number"])
            adjacent[r["number"]].add(s)

    def distances(start_at, limit=6):
        dist = {start_at: 0}
        frontier = [start_at]
        for d in range(1, limit + 1):
            nxt = []
            for n in frontier:
                for m in sorted(adjacent.get(n, ())):
                    if m not in dist:
                        dist[m] = d
                        nxt.append(m)
            frontier = nxt
        return dist

    edge_list = sorted({tuple(sorted((x, y))) for x in adjacent for y in adjacent[x]})
    words = {2: "two", 3: "three"}
    chosen = []  # (a, b, hops asked)
    d1, d2, d3 = [], [], []
    for a in subjects:
        dist = distances(a)
        for b in subjects:
            if a < b and a not in hubs and b not in hubs:
                (d1 if dist.get(b) == 1 else d2 if dist.get(b) == 2 else d3).append((a, b))
    chosen += [(a, b, 3) for a, b in d2[:3]] + [(a, b, 3) for a, b in d1[:1]] + [(a, b, 2) for a, b in d3[:2]]
    mixed = []
    for a in subjects:
        dist = distances(a)
        for n in sorted(dist):
            if dist[n] == 3 and n not in subjects and n not in hubs:
                mixed.append((a, n, 3))
                break
    chosen += mixed[:2]
    for i, (a, b, limit) in enumerate(chosen):
        dist_a, dist_b = distances(a), distances(b)
        d = dist_a.get(b)
        exists = d is not None and d <= limit
        via = sorted(n for n in dist_a if n in dist_b and n not in (a, b) and dist_a[n] + dist_b[n] == d) if exists else []
        if i % 2 == 0:
            text = "How are %s and %s connected within %s hops, not through services or hubs?" % (a, b, words[limit])
        else:
            text = "Is there a link between %s and %s within %s hops, leaving out services and numbers everyone contacts?" % (a, b, words[limit])
        add("path", text, {"kind": "path", "a": a, "b": b, "hops": limit, "exists": exists, "distance": d if exists else None, "via": via, "graph": [list(e) for e in edge_list]})
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
