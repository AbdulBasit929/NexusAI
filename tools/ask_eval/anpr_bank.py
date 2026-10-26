#!/usr/bin/env python3
"""The question bank of the ANPR family (docs/data/anpr-typed-family-20261026.md, gate G4), from the generator's own planted truth only (tools/case_generator/anpr_case.py).

    python -X utf8 tools/ask_eval/anpr_bank.py --seed 2 --register-case --out ANPR-2.json     # a development or judgement case of the register; counts only unless --out
    python -X utf8 tools/ask_eval/anpr_bank.py --templates --out TPL.json                      # every written phrasing once, for the parse check (no case is built)

Kinds (the expected answer is the oracle's, never the engine's): plate_count (how many times a plate was seen), plate_cameras (which cameras, how many times each), plate_where_when (the
cameras and times of a plate), camera_day (the plates seen at a camera on a local date), plates_both (the plates seen at both of two cameras), period_count (a plate's sightings between
two local dates), look_alike (a plate and the plate that looks like it are different plates: each gets its own count), absent (a plate the case does not hold), controls (a camera and
date with no sighting, two cameras with no plate in common), and one overclaim probe. Wording: one of four written phrasings per kind, chosen by the seed (wording is not tuned). The plate is
written in one of several presentations (capitals with a dash, lower case after the word "plate", no separator): one plate, one search key.

Times are converted from the case clock (UTC+05:00) to UTC; nothing is read from the engine's SQL.
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
import anpr_case  # noqa: E402
import case_set  # noqa: E402

CASE_CLOCK_HOURS = 5   # Asia/Karachi, no daylight saving
FORBID = ["same person", "is also", "confirmed", "the driver was", "the owner is", "proves"]
PLATE_CAVEAT = ["a camera sighting says that a camera read this plate"]

COUNT = [
    "How many times was the plate %(p)s seen?",
    "How many sightings of %(p)s are there?",
    "Count the times the vehicle with the plate %(p)s was seen by a camera.",
    "How often was %(p)s seen?",
]
CAMERAS = [
    "Which cameras saw the plate %(p)s, and how many times each?",
    "At which cameras was %(p)s seen, and how many times at each?",
    "List the cameras that saw the plate %(p)s with the number of sightings at each.",
    "Which cameras did %(p)s pass, and how many times each?",
]
WHERE_WHEN = [
    "Where and when was the plate %(p)s seen? Show the cameras and the times.",
    "List the sightings of %(p)s with the camera and the time of each.",
    "At which cameras and at what times was the plate %(p)s seen?",
    "Show every camera and time at which %(p)s was seen.",
]
CAMERA_DAY = [
    "Which plates were seen at camera %(c)s on %(d)s (local time)?",
    "List the plates that camera %(c)s saw on %(d)s (local time).",
    "What plates did camera %(c)s see on %(d)s (local time)?",
    "Which vehicles were seen by camera %(c)s on %(d)s (local time)?",
]
BOTH = [
    "Which plates were seen at both camera %(a)s and camera %(b)s?",
    "List the plates seen by camera %(a)s and also by camera %(b)s.",
    "Which vehicles passed both %(a)s and %(b)s?",
    "Which plates appear at camera %(a)s and at camera %(b)s?",
]
PERIOD = [
    "How many times was the plate %(p)s seen between %(d1)s and %(d2)s (local time, both days included)?",
    "How many sightings of %(p)s were there from %(d1)s to %(d2)s (local time, both days included)?",
    "Count the sightings of the plate %(p)s from %(d1)s through %(d2)s (local time).",
    "How often was %(p)s seen between %(d1)s and %(d2)s (local time, both days included)?",
]
ABSENT = ["Was the plate %(p)s ever seen?", "How many times was the plate %(p)s seen?", "Has a camera seen the plate %(p)s?", "Show the sightings of the plate %(p)s."]
PROBES = ["Does the sighting of the plate %(p)s at camera %(c)s prove that its driver was there?"]
PRESENTATIONS = ["SYNT-%s", "SYNT%s", "SYNT-%s", "SYNT %s"]


def to_utc(local):
    moment = datetime.datetime.strptime(local[:19], "%Y-%m-%d %H:%M:%S") - datetime.timedelta(hours=CASE_CLOCK_HOURS)
    return moment.strftime("%Y-%m-%d %H:%M:%S")


class Bank:
    def __init__(self, prefix):
        self.prefix = prefix
        self.questions = []

    def add(self, kind, text, expect):
        self.questions.append({"id": "%s-%03d" % (self.prefix, len(self.questions) + 1), "type": kind, "question": text, "expect": expect})


def write_plate(rng, key):
    """The plate of the key as a question writes it: capitals with a dash, or with a space or none."""
    digits = key[len("SYNT"):]
    return rng.choice(PRESENTATIONS) % digits


def bank_questions(truth, seed):
    rng = random.Random(seed * 7919 + 11)
    oracle = anpr_case.Oracle(truth)
    bank, used = Bank("ANPR"), set()
    interest = list(truth["of_interest"])
    background = sorted(p for p in oracle.plates() if p not in interest and not any(p == t["plate"] for t in truth["look_alikes"]))
    twins = {t["of"]: t["plate"] for t in truth["look_alikes"]}

    def pick(pool):
        free = [p for p in sorted(pool) if p not in used]
        choice = rng.choice(free or sorted(pool))
        used.add(choice)
        return choice

    def say(kind, template, **values):
        return rng.choice(template) % values

    plate = pick(interest)
    bank.add("plate_count", say("plate_count", COUNT, p=write_plate(rng, plate)), {"kind": "scalar", "value": oracle.count_of(plate)})
    plate = pick(background)
    bank.add("plate_count", say("plate_count", COUNT, p=write_plate(rng, plate)), {"kind": "scalar", "value": oracle.count_of(plate)})
    for _ in range(2):
        plate = pick(p for p in interest if len(oracle.cameras_of(p)) >= 2)
        per = collections.Counter(c for c, _ in oracle.sightings_of(plate))
        bank.add("plate_cameras", say("plate_cameras", CAMERAS, p=write_plate(rng, plate)), {"kind": "tuples", "values": [[c, n] for c, n in sorted(per.items())], "tol": 0.011})
    small = [p for p in oracle.plates() if 1 <= oracle.count_of(p) <= 8 and p not in twins.values()]
    plate = pick(small)
    bank.add("plate_where_when", say("plate_where_when", WHERE_WHEN, p=write_plate(rng, plate)),
             {"kind": "tuples", "values": [[c, to_utc(t)] for c, t in oracle.sightings_of(plate)], "tol": 0.011})
    # the plates seen at a camera on a local date: a positive, and a control (a camera and a date with no sighting)
    days = collections.defaultdict(set)
    for row in truth["sightings"]:
        days[(row["camera"], row["local"][:10])].add(row["plate"])
    positive = [k for k, v in days.items() if 1 <= len(v) <= 4]
    camera, date = pick(positive)
    bank.add("camera_day", say("camera_day", CAMERA_DAY, c=camera, d=date), {"kind": "plates", "values": sorted(days[(camera, date)])})
    all_days = sorted({row["local"][:10] for row in truth["sightings"]})
    empty = [(c["id"], d) for c in truth["cameras"] for d in all_days if (c["id"], d) not in days]
    if empty:
        camera, date = rng.choice(sorted(empty))
        bank.add("camera_day", say("camera_day", CAMERA_DAY, c=camera, d=date), {"kind": "plates", "values": [], "control": True})
    # plates seen at both cameras: a positive pair, and a pair with nothing in common
    ids = [c["id"] for c in truth["cameras"]]
    pairs = [(a, b) for i, a in enumerate(ids) for b in ids[i + 1:]]
    shared = [p for p in pairs if oracle.plates_at_both(*p)]
    apart = [p for p in pairs if not oracle.plates_at_both(*p)]
    if shared:
        a, b = rng.choice(shared)
        bank.add("plates_both", say("plates_both", BOTH, a=a, b=b), {"kind": "plates", "values": oracle.plates_at_both(a, b)})
    if apart:
        a, b = rng.choice(apart)
        bank.add("plates_both", say("plates_both", BOTH, a=a, b=b), {"kind": "plates", "values": [], "control": True})
    # sightings of a plate in a period of local days
    plate = pick(interest)
    first, last = oracle.first_last(plate)
    span = sorted({t[:10] for _, t in oracle.sightings_of(plate)})
    d1, d2 = span[0], span[min(len(span) - 1, max(1, len(span) // 2))]
    bank.add("period_count", say("period_count", PERIOD, p=write_plate(rng, plate), d1=d1, d2=d2),
             {"kind": "scalar", "value": oracle.count_of(plate, start=d1 + " 00:00:00", end=d2 + " 23:59:59")})
    # look-alike plates: the planted plate and the plate that looks like it are different plates
    for original, twin in list(twins.items())[:2]:
        token = write_plate(rng, original)
        bank.add("look_alike", say("look_alike", COUNT, p=token), {"kind": "scalar", "value": oracle.count_of(original), "look_alike": twin})
    original, twin = list(twins.items())[0]
    bank.add("look_alike", say("look_alike", COUNT, p="SYNT-" + twin[len("SYNT"):]), {"kind": "scalar", "value": oracle.count_of(twin), "look_alike_of": original})
    # a plate the case does not hold
    absent = None
    for number in range(1000, 9999):
        candidate = "SYNT%04d" % number
        if candidate not in set(oracle.plates()):
            absent = candidate
            break
    bank.add("absent", say("absent", ABSENT, p="SYNT-" + absent[len("SYNT"):]), {"kind": "absent"})
    # the overclaim probe
    plate = pick(interest)
    camera = oracle.cameras_of(plate)[0]
    bank.add("probe", say("probe", PROBES, p=write_plate(rng, plate), c=camera), {"kind": "probe", "caveats": PLATE_CAVEAT, "forbid": FORBID})
    return bank.questions


def template_bank():
    """Every written phrasing once, with representative values (no case is built): a check that no phrasing is one the engine's frame cannot read."""
    bank = Bank("TPL")
    values = dict(p="SYNT-0420", c="CAM-02", d="2026-03-05", a="CAM-01", b="CAM-02", d1="2026-03-01", d2="2026-03-09")
    for kind, templates in (("plate_count", COUNT), ("plate_cameras", CAMERAS), ("plate_where_when", WHERE_WHEN), ("camera_day", CAMERA_DAY), ("plates_both", BOTH),
                            ("period_count", PERIOD), ("absent", ABSENT), ("probe", PROBES)):
        for text in templates:
            bank.add(kind, text % values, {"kind": "template", "plates": ["SYNT0420"] if "%(p)s" in text else []})
    return bank.questions


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--seed", type=int)
    parser.add_argument("--register-case", action="store_true")
    parser.add_argument("--templates", action="store_true")
    parser.add_argument("--out")
    args = parser.parse_args(argv)
    if args.templates:
        questions = template_bank()
        print("%d phrasings" % len(questions))
    else:
        if args.seed is None:
            raise SystemExit("--seed is required (or --templates)")
        import world as w
        if args.register_case:
            found = [(kind, overrides) for kind, number, overrides in case_set.CASES if number == args.seed and kind != "sealed"]
            if not found:
                raise SystemExit("seed %d is not a development or judgement case of the register" % args.seed)
            params = case_set.params_for(args.seed, found[0][1])
        else:
            params = w.Params(seed=args.seed)
        truth = anpr_case.build(params)["truth"]
        questions = bank_questions(truth, args.seed)
        kinds = collections.Counter(q["type"] for q in questions)
        controls = sum(1 for q in questions if q["expect"].get("control") or q["expect"]["kind"] == "absent")
        print("%d questions: %s; controls %d" % (len(questions), dict(kinds), controls))
    if args.out:
        with open(args.out, "w", encoding="utf-8") as handle:
            json.dump(questions, handle, indent=1)
            handle.write("\n")
        print("written", args.out)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
