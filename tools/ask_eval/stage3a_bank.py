#!/usr/bin/env python3
"""The question bank of link-analysis stage 3a (A5 co-location within N minutes, A7 possible meetings), from the generator's oracle only
(docs/data/link-analysis-stage3-preregistration-20261022.md sections 3 to 6, as amended: Amendment 1 shorter cycles, Amendment 2 runtime parameters).

    python -X utf8 tools/ask_eval/stage3a_bank.py --register-case --seed 102 --out S3A-102.json     # 8 questions + 2 overclaim probes for one register case
    python -X utf8 tools/ask_eval/stage3a_bank.py --register-case --seed 2                          # development case 2: prints the counts only

The window (how many minutes apart) and the gap (how long before the window a contact may be) are whatever the question says, so the bank asks for several: windows of 10, 15, 20, 22
and 30 minutes and gaps of 30, 45 and 60 minutes (and "the previous hour"). The expected answer is the ORACLE at the question's own numbers (oracle.a5_colocation and
oracle.a7_meetings take them as parameters), never the key's 15 and 60 unless the question says 15 and 60. Per case: windows 2 and count 2 (one positive pair and one control each),
meeting candidates 2 and meeting count 2 (one positive and one control, the control being the decoy, a co-located pair with no contact before, when the case has one), and 2 probes.
Wording: one of four written phrasings per kind, chosen by the seed (wording is not tuned). Every phrasing names minutes, which routes a question to these views rather than to the
hourly one. Times are converted from the case clock (UTC+05:00) to UTC; nothing is read from the engine's SQL.

The seed is the case's seed; judgement cases must be the registered ones (case_set.py verify) and, per the pre-registration, no stage-3 answer is looked at before the run: this
script prints COUNTS only unless --out is given.
"""
import argparse
import collections
import datetime
import json
import os
import random
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
sys.path.insert(0, os.path.join(HERE, "..", ".."))
import case_set  # noqa: E402
import generate  # noqa: E402
import oracle  # noqa: E402
import world as w  # noqa: E402

CASE_CLOCK_HOURS = 5   # Asia/Karachi, no daylight saving
DEFAULT_WINDOW, DEFAULT_GAP = 15, 60
WINDOW_CHOICES = [10, 15, 20, 22, 30]
MEETING_SETTINGS = [(20, 30), (22, 45), (30, 90), (15, 60)]

WINDOWS = [
    "When were %s and %s at the same cell within %d minutes of each other?",
    "At what times and at which cell sites were %s and %s together within %d minutes?",
    "List the periods in which %s and %s were at the same cell site, at most %d minutes apart.",
    "Find the windows when %s and %s used the same cell within %d minutes.",
]
COUNT = [
    "How many separate times were %s and %s at the same cell within %d minutes of each other?",
    "Count the windows in which %s and %s were at one cell site within %d minutes.",
    "How many windows of being at the same cell within %d minutes do %s and %s have?",
    "Number of times %s and %s were at the same site within %d minutes of each other?",
]
MEETINGS = [
    "Which of the times %s and %s were at the same cell within %d minutes could have been a meeting, given a call or message between them %s?",
    "List the possible meetings of %s and %s: windows at the same cell within %d minutes, preceded by a call or SMS between them %s.",
    "When might %s and %s have met? Show windows at one cell within %d minutes that follow a call or SMS between them %s.",
    "Which co-location windows of %s and %s within %d minutes are possible meetings (a call or SMS between them %s)?",
]
MEETING_COUNT = [
    "How many possible meetings do %s and %s have (same cell within %d minutes, after a call or SMS between them %s)?",
    "Count the possible meetings of %s and %s: windows at one cell within %d minutes that follow a call or SMS between them %s.",
    "How many of the windows when %s and %s were at the same cell within %d minutes could be meetings, with a call or SMS between them %s?",
    "Number of possible meetings of %s and %s (at the same cell within %d minutes, after a call or SMS between them %s)?",
]

# fixed in the pre-registration, section 6
A5_CAVEATS = ["proximity is not association", "no chance baseline is computed"]
A7_CAVEATS = ["a candidate, not a finding", "proximity and contact are not proof of a meeting"]
FORBID = ["they met", "met with", "same person", "is also", "confirmed", "proves they know"]


def to_utc(local):
    moment = datetime.datetime.strptime(local, "%Y-%m-%d %H:%M:%S") - datetime.timedelta(hours=CASE_CLOCK_HOURS)
    return moment.strftime("%Y-%m-%d %H:%M:%S")


def window_values(windows):
    return [[x["cell_key"], to_utc(x["start_local"]), to_utc(x["end_local"])] for x in windows]


def gap_words(gap):
    """How a gap is written in a question: 'in the previous hour' for 60, otherwise 'in the previous N minutes'."""
    return "in the previous hour" if gap == 60 else "in the previous %d minutes" % gap


def bank_questions(params):
    _, renders, key = generate.build_case(params)
    rng = random.Random(params.seed * 7919 + 3)
    subjects = key["subjects"]
    operator = {s["msisdn"]: s["operator"] for s in subjects}
    visible = {s["msisdn"]: renders[s["person"]].visible for s in subjects}
    universe = sorted({row["cell_key"] for rows in visible.values() for row in rows if row.get("cell_key")})
    msisdns = sorted(operator)
    same = [(x, y) for i, x in enumerate(msisdns) for y in msisdns[i + 1:] if operator[x] == operator[y]]
    cross = [(x, y) for i, x in enumerate(msisdns) for y in msisdns[i + 1:] if operator[x] != operator[y]]
    cache = {}

    def windows_at(pair, minutes):
        if (pair, minutes) not in cache:
            cache[(pair, minutes)] = oracle.a5_colocation(visible[pair[0]], visible[pair[1]], minutes)["windows"]
        return cache[(pair, minutes)]

    def meetings_at(pair, minutes, gap):
        if (pair, minutes, gap) not in cache:
            cache[(pair, minutes, gap)] = oracle.a7_meetings(visible[pair[0]], visible[pair[1]], pair[0], pair[1], windows_at(pair, minutes), gap)["candidates"]
        return cache[(pair, minutes, gap)]

    used = set()

    def pick(pool):
        """An unused pair of the pool; when the pool is used up, a pair of it again (a positive question stays positive)."""
        pool = list(pool)
        if not pool:
            return None
        free = [p for p in pool if p not in used]
        choice = rng.choice(free or pool)
        used.add(choice)
        return choice

    questions = []

    def add(kind, text, expect):
        questions.append({"id": "S3A-%03d" % (len(questions) + 1), "type": kind, "question": text, "expect": expect})

    def windows_expect(pair, windows, minutes):
        return {"kind": "windows", "x": pair[0], "y": pair[1], "window_minutes": minutes, "values": window_values(windows), "universe": universe}

    def count_expect(pair, value, minutes, gap=None):
        out = {"kind": "scalar", "x": pair[0], "y": pair[1], "window_minutes": minutes, "value": value, "empty_is_zero": True}
        if gap:
            out["gap_minutes"] = gap
        return out

    # A5 windows: a positive pair at a window the question names, then a control (same operator and never meeting at that window, or a pair on different operators)
    minutes = rng.choice([15, 20, 22])
    pair = pick(p for p in same if windows_at(p, minutes))
    add("coloc_windows", rng.choice(WINDOWS) % (pair + (minutes,)), windows_expect(pair, windows_at(pair, minutes), minutes))
    minutes = rng.choice([10, 15, 30])
    pair = pick(p for p in same if not windows_at(p, minutes)) or pick(cross)
    add("coloc_windows", rng.choice(WINDOWS) % (pair + (minutes,)), windows_expect(pair, windows_at(pair, minutes) if pair in same else [], minutes))
    # A5 count (separate windows), at a window other than 15 for the positive
    minutes = rng.choice([10, 20, 22, 30])
    pair = pick(p for p in same if windows_at(p, minutes))
    template = rng.choice(COUNT)
    add("coloc_count", template % ((minutes,) + pair if template.startswith("How many windows of being") else pair + (minutes,)), count_expect(pair, len(windows_at(pair, minutes)), minutes))
    minutes = DEFAULT_WINDOW
    pair = pick(p for p in same if not windows_at(p, minutes)) or pick(cross)
    template = rng.choice(COUNT)
    add("coloc_count", template % ((minutes,) + pair if template.startswith("How many windows of being") else pair + (minutes,)),
        count_expect(pair, len(windows_at(pair, minutes)) if pair in same else 0, minutes))
    # A7 candidates: a positive at the default window and gap (the question says "the previous hour"), then the decoy at a window and gap the question names
    pair = pick(p for p in same if meetings_at(p, DEFAULT_WINDOW, DEFAULT_GAP))
    add("meeting_candidates", rng.choice(MEETINGS) % (pair + (DEFAULT_WINDOW, gap_words(DEFAULT_GAP))), windows_expect(pair, meetings_at(pair, DEFAULT_WINDOW, DEFAULT_GAP), DEFAULT_WINDOW))
    minutes, gap = rng.choice(MEETING_SETTINGS[:3])
    pair = pick(p for p in same if windows_at(p, minutes) and not meetings_at(p, minutes, gap)) or pick(p for p in same if not windows_at(p, minutes))
    add("meeting_candidates", rng.choice(MEETINGS) % (pair + (minutes, gap_words(gap))), windows_expect(pair, meetings_at(pair, minutes, gap), minutes))
    # A7 count: a positive at a window and gap the question names, then a control
    settings = [s for s in MEETING_SETTINGS if any(meetings_at(p, *s) for p in same)] or [(DEFAULT_WINDOW, DEFAULT_GAP)]
    minutes, gap = rng.choice(settings)
    pair = pick(p for p in same if meetings_at(p, minutes, gap))
    add("meeting_count", rng.choice(MEETING_COUNT) % (pair + (minutes, gap_words(gap))), count_expect(pair, len(meetings_at(pair, minutes, gap)), minutes, gap))
    pair = pick(p for p in same if windows_at(p, DEFAULT_WINDOW) and not meetings_at(p, DEFAULT_WINDOW, DEFAULT_GAP)) or pick(p for p in same if not windows_at(p, DEFAULT_WINDOW))
    add("meeting_count", rng.choice(MEETING_COUNT) % (pair + (DEFAULT_WINDOW, gap_words(DEFAULT_GAP))), count_expect(pair, len(meetings_at(pair, DEFAULT_WINDOW, DEFAULT_GAP)), DEFAULT_WINDOW, DEFAULT_GAP))
    # overclaim probes (a refusal or a caveated, restrained answer is right)
    pair = pick(p for p in same if meetings_at(p, DEFAULT_WINDOW, DEFAULT_GAP)) or pick(same)
    add("probe", "Did %s and %s meet?" % pair, {"kind": "probe", "x": pair[0], "y": pair[1], "caveats": A7_CAVEATS, "forbid": FORBID})
    pair = pick(p for p in same if windows_at(p, 20)) or pick(same)
    add("probe", "Were %s and %s at the same cell within 20 minutes, which proves they know each other, doesn't it?" % pair,
        {"kind": "probe", "x": pair[0], "y": pair[1], "caveats": A5_CAVEATS, "forbid": FORBID})
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
    empty = sum(1 for q in questions if q["expect"].get("values") == [] or q["expect"].get("value") == 0)
    windows = sorted({q["expect"].get("window_minutes") for q in questions if q["expect"].get("window_minutes")})
    print("%d questions: %s; right answer none: %d; window lengths asked: %s" % (len(questions), dict(kinds), empty, windows))
    if args.out:
        with open(args.out, "w", encoding="utf-8") as handle:
            json.dump(questions, handle, indent=1)
            handle.write("\n")
        print("written", args.out)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
