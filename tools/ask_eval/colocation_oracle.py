#!/usr/bin/env python3
"""The oracle for co-location at the granularity of the analyst view (milestone M3, stage 2; written 2026-10-19).

The generator's own co-location key (A5) answers "within 15 minutes at the same cell". The analyst view `v_cell_visits` holds one row per subscriber, cell and **hour**, so a
question the engine can answer from it is "were X and Y at the same cell in the same hour". Those are different questions with different truths, and an oracle at the wrong
granularity would mark a right answer wrong. This module computes the hourly truth in plain Python from the generator's own visible rows (the rows the typed ingest must
accept), with the same cell identity, and never from the engine's SQL.

    python -X utf8 tools/ask_eval/colocation_oracle.py --register-case --seed 1        # prints what the questions of a case would be (counts, no values)

The local hour and the UTC hour are the same hour of the same event (the case clock, Asia/Karachi, is a whole number of hours from UTC), so equality of the local hour is
equality of the UTC hour that the view uses.
"""
import argparse
import collections
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
sys.path.insert(0, os.path.join(HERE, "..", ".."))
import case_set  # noqa: E402
import generate  # noqa: E402
import world as w  # noqa: E402


def hour_of(row):
    """The local hour of an event as 'YYYY-MM-DD HH'."""
    return row["start_local"][:13]


def hourly_visits(rows):
    """{(cell_key, hour): number of events} for the events that carry a cell."""
    out = collections.Counter()
    for row in rows:
        if row.get("cell_key"):
            out[(row["cell_key"], hour_of(row))] += 1
    return out


def hourly_colocation(rows_x, rows_y):
    """The (cell, hour) pairs where both subscribers have at least one event, with each side's events: [(cell_key, hour, events_x, events_y)] in order."""
    vx, vy = hourly_visits(rows_x), hourly_visits(rows_y)
    return sorted((cell, hour, vx[(cell, hour)], vy[(cell, hour)]) for (cell, hour) in vx.keys() & vy.keys())


def bank_questions(params, per_case_limit=12):
    """Stage-2 questions for a case: for pairs of subscribers on the same operator (cell identities are per operator), which cells both used in the same hour, and in how many
    hours. Pairs with a co-location come first, then pairs with none (the right answer is 'no row' and zero), alternating so that both kinds are in the set."""
    _, renders, key = generate.build_case(params)
    subjects = [s for s in key["subjects"]]
    by_msisdn = {s["msisdn"]: s for s in subjects}
    visible = {}
    pids = {s["msisdn"]: None for s in subjects}
    # renders are keyed by person id; the key's subjects carry the person id
    for s in subjects:
        visible[s["msisdn"]] = renders[s["person"]].visible
    pairs = []
    for i, x in enumerate(sorted(visible)):
        for y in sorted(visible)[i + 1:]:
            if by_msisdn[x]["operator"] != by_msisdn[y]["operator"]:
                continue
            hits = hourly_colocation(visible[x], visible[y])
            pairs.append((x, y, hits))
    with_hits = [p for p in pairs if p[2]]
    without = [p for p in pairs if not p[2]]
    chosen = []
    for a, b in zip(with_hits, without + [None] * len(with_hits)):
        chosen.append(a)
        if b:
            chosen.append(b)
    chosen += with_hits[len(chosen):]
    questions = []
    for i, (x, y, hits) in enumerate(chosen[:per_case_limit]):
        cells = sorted({h[0] for h in hits})
        if i % 2 == 0:
            text = "Which cell sites were %s and %s both at during the same hour?" % (x, y)
        else:
            text = "At which cells did %s and %s have events in the same hour?" % (x, y)
        questions.append({"type": "coloc_cells", "question": text, "expect": {"kind": "cells", "x": x, "y": y, "values": cells}})
        text = ("In how many different hours were %s and %s at the same cell?" if i % 2 == 0 else "How many hours did %s and %s spend at the same cell site together, counting each cell and hour once?") % (x, y)
        questions.append({"type": "coloc_hours", "question": text, "expect": {"kind": "scalar", "x": x, "y": y, "value": len(hits)}})
    for n, q in enumerate(questions, start=1):
        q["id"] = "C%03d" % n
    return questions


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
    questions = bank_questions(params)
    kinds = collections.Counter(q["type"] for q in questions)
    zero = sum(1 for q in questions if q["type"] == "coloc_hours" and q["expect"]["value"] == 0)
    print("%d questions: %s; pairs with no co-location in the hour: %d" % (len(questions), dict(kinds), zero))
    if args.out:
        with open(args.out, "w", encoding="utf-8") as handle:
            json.dump(questions, handle, indent=1)
            handle.write("\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
