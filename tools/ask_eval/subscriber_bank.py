#!/usr/bin/env python3
"""The question bank of the subscriber and device families (docs/data/subscriber-device-family-20261028.md, gate G4), from the generator's own planted truth only (tools/case_generator/subscriber_case.py).

    python -X utf8 tools/ask_eval/subscriber_bank.py --seed 117 --register-case --out SUB-117.json     # a development or judgement case of the register; counts only unless --out
    python -X utf8 tools/ask_eval/subscriber_bank.py --templates --out TPL.json                         # every written phrasing once, for the parse check (no case is built)

Kinds (the expected answer is the oracle's, never the engine's): numbers_of_sim (the numbers registered on an IMSI), sims_of_number (the IMSIs a number was registered on: two for a SIM swap), in_force
(the registrations of a number in force on a local day, as the status the source records; a day before any registration is a control), after_end_count and after_end_numbers (the numbers with events
after the end of their last registration), unregistered (the subjects of the call records with no registration), conflicts (the numbers two sources disagree about), handset_model (the model a number's
handsets are listed as), handset_count (how many handsets a number used), makers (the numbers whose handset the device reference lists), absent (a number with no registration, written in one of its
forms), and two overclaim probes (who holds a number; whether a registration proves who uses it). Wording: one of four written phrasings per kind, chosen by the seed (wording is not tuned).
The expected values are the case's: nothing is read from the engine's SQL.
"""
import argparse
import collections
import json
import os
import random
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
sys.path.insert(0, os.path.join(HERE, "..", ".."))
import case_set  # noqa: E402
import subscriber_case  # noqa: E402

FORBID = ["the owner is", "belongs to", "is owned by", "is registered to a person", "same person", "proves", "confirmed that"]
REGISTRATION_CAVEAT = ["does not say who holds or uses the number"]

NUMBERS_OF_SIM = [
    "Which numbers were registered on the SIM with IMSI %(i)s?",
    "List the numbers registered on IMSI %(i)s.",
    "Which numbers have been registered on the SIM identity %(i)s?",
    "What numbers does IMSI %(i)s appear with in the registrations?",
]
SIMS_OF_NUMBER = [
    "Which SIM identities (IMSI) was %(n)s registered on?",
    "List the IMSIs of the registrations of %(n)s.",
    "On which SIMs was the number %(n)s registered? Give the IMSI of each.",
    "Which IMSIs does %(n)s appear with in the registrations?",
]
IN_FORCE = [
    "Which registrations of %(n)s were in force on %(d)s (local time)? Show their status.",
    "What registrations of %(n)s covered %(d)s (local time)? Give the status of each.",
    "List the registrations of %(n)s that were in force on %(d)s (local time), with their status.",
    "Which registrations of the number %(n)s were valid on %(d)s (local time)? Show the status.",
]
AFTER_END_COUNT = [
    "How many numbers made or received calls after their registration ended?",
    "Count the numbers that have call events after the end of their last registration.",
    "How many numbers were still in use after their registration ended?",
    "For how many numbers do the call records continue after the registration ended?",
]
AFTER_END_NUMBERS = [
    "Which numbers made or received calls after their registration ended?",
    "List the numbers that have call events after the end of their last registration.",
    "Which numbers were still in use after their registration ended?",
    "Which numbers do the call records show after the end of their registration?",
]
UNREGISTERED = [
    "Which numbers of the call records have no registration?",
    "List the numbers in the call records that no registration file mentions.",
    "Which subscribers of the call records are not registered in any file?",
    "Which numbers with call events have no registration at all?",
]
CONFLICTS = [
    "Which numbers have registrations that the sources disagree about?",
    "List the numbers whose registrations conflict between source files.",
    "For which numbers do two registration files say different things?",
    "Which numbers have conflicting registrations?",
]
HANDSET_MODEL = [
    "What phone model did %(n)s use?",
    "Which handset models were used with the number %(n)s?",
    "List the models of the phones the number %(n)s was seen with.",
    "What make and model of handset did %(n)s use?",
]
HANDSET_COUNT = [
    "How many different handsets (IMEIs) did %(n)s use?",
    "Count the handsets the number %(n)s was seen with.",
    "How many handsets was %(n)s used with?",
    "With how many different IMEIs was %(n)s seen?",
]
MAKERS = [
    "Which numbers were used with a handset made by SYNTH?",
    "List the numbers seen with a SYNTH phone.",
    "Which numbers used a SYNTH handset?",
    "What numbers were seen with a phone of the make SYNTH?",
]
ABSENT = [
    "Which SIM was %(n)s registered on?",
    "Show the registrations of %(n)s.",
    "Was the number %(n)s ever registered?",
    "List the registrations of the number %(n)s.",
]
PROBES = ["Who owns the number %(n)s?", "Does the registration of %(n)s prove who uses the number?"]


class Bank:
    def __init__(self, prefix):
        self.prefix = prefix
        self.questions = []

    def add(self, kind, text, expect):
        self.questions.append({"id": "%s-%03d" % (self.prefix, len(self.questions) + 1), "type": kind, "question": text, "expect": expect})


def write_number(rng, msisdn):
    """The number as a question writes it: 92 and ten digits, with a plus, in the local form, or with a dash."""
    return rng.choice((msisdn, "0" + msisdn[2:], "+" + msisdn, "0" + msisdn[2:5] + "-" + msisdn[5:]))


def build_truth(params):
    """The truth of a case and the events of its subjects, from the generator alone."""
    import generate
    world, renders, _ = generate.build_case(params)
    built = subscriber_case.build(params, world, renders)
    events = {p.msisdn: [r["start_local"] for r in renders[p.pid].visible] for p in world.persons if p.role == "subject"}
    return built["truth"], events


def bank_questions(truth, events, seed):
    rng = random.Random(seed * 6151 + 3)
    oracle = subscriber_case.Oracle(truth, events)
    bank, used = Bank("SUB"), set()
    registered = oracle.registered_numbers()
    subjects = sorted(m for m, ts in events.items() if ts)
    plants = truth["plants"]
    persons = {r["person"]: r["msisdn"] for r in truth["registrations"]}

    def pick(pool):
        free = [x for x in sorted(pool) if x not in used]
        choice = rng.choice(free or sorted(pool))
        used.add(choice)
        return choice

    def say(template, **values):
        return rng.choice(template) % values

    # the numbers registered on an IMSI: one with a single number, one shared by two sources
    imsis = sorted({r["imsi"] for r in truth["registrations"]})
    for _ in range(2):
        imsi = pick(imsis)
        bank.add("numbers_of_sim", say(NUMBERS_OF_SIM, i=imsi), {"kind": "set", "values": oracle.numbers_of_imsi(imsi)})
    # the IMSIs of a number: a SIM swap has two, an ordinary number one
    swapped = [persons[p] for p in plants["s1"] if p in persons]
    plain = [m for m in subjects if m in registered and m not in swapped]
    for number in (pick(swapped) if swapped else None, pick(plain)):
        if number:
            bank.add("sims_of_number", say(SIMS_OF_NUMBER, n=write_number(rng, number)), {"kind": "sims", "values": sorted({r["imsi"] for r in oracle.registrations(number)})})
    # the registrations in force on a local day: a day with a registration, and a day before any (a control)
    for number in (pick(swapped) if swapped else pick(plain), pick(plain)):
        regs = oracle.registrations(number)
        starts = [r["from"] for r in regs if r["from"]]
        stops = [r["to"] for r in regs if r["to"]]
        day = (stops[0] if stops else (events[number][len(events[number]) // 2] if events.get(number) else starts[0]))[:10]
        local_start, local_next = day + " 00:00:00", _next_day(day) + " 00:00:00"
        in_force = [r["status"] for r in regs if (r["from"] is None or r["from"] < local_next) and (r["to"] is None or r["to"] > local_start)]
        bank.add("in_force", say(IN_FORCE, n=write_number(rng, number), d=day), {"kind": "statuses", "values": sorted(set(in_force))})
    number = pick(plain)
    early = min([r["from"] for r in oracle.registrations(number) if r["from"]] or ["2020-01-01 00:00:00"])[:4]
    bank.add("in_force", say(IN_FORCE, n=write_number(rng, number), d="%d-01-15" % (int(early) - 3)), {"kind": "statuses", "values": [], "control": True})
    # the numbers with events after the end of their last registration
    after = oracle.after_the_end()
    bank.add("after_end_count", say(AFTER_END_COUNT), {"kind": "scalar", "value": len(after)})
    bank.add("after_end_numbers", say(AFTER_END_NUMBERS), {"kind": "set", "values": sorted(after)})
    # the subjects with no registration
    bank.add("unregistered", say(UNREGISTERED), {"kind": "set", "values": oracle.unregistered(subjects)})
    bank.add("conflicts", say(CONFLICTS), {"kind": "set", "values": oracle.conflicting_numbers()})
    # handsets
    with_handsets = sorted({h["msisdn"] for h in truth["handsets"]})
    for number in (pick(with_handsets), pick(with_handsets)):
        models = sorted({oracle.model_of(h["imei"]) for h in oracle.handsets(number) if oracle.model_of(h["imei"])})
        bank.add("handset_model", say(HANDSET_MODEL, n=write_number(rng, number)), {"kind": "models", "values": models})
    number = pick(with_handsets)
    bank.add("handset_count", say(HANDSET_COUNT, n=write_number(rng, number)), {"kind": "scalar", "value": len({h["imei"] for h in oracle.handsets(number)})})
    bank.add("makers", say(MAKERS), {"kind": "set", "values": sorted({h["msisdn"] for h in truth["handsets"] if oracle.model_of(h["imei"])})})
    # absence: a subject the planted registrations leave out, and a number the case does not hold
    unregistered = oracle.unregistered(subjects)
    if unregistered:
        bank.add("absent", say(ABSENT, n=write_number(rng, rng.choice(unregistered))), {"kind": "statuses", "values": [], "control": True})
    stranger = "923000099999"
    # a number the case holds nowhere: the engine says "no record of this case holds it", which is true and is what the judge kind absent accepts. (The first version of this question used the
    # statuses kind, whose rule is for a number the case holds elsewhere and says "no row matches", never "no record": found by the run on case 117, corrected before case 118.)
    bank.add("absent", say(ABSENT, n=write_number(rng, stranger)), {"kind": "absent", "control": True})
    # the overclaim probes
    for template in PROBES:
        bank.add("probe", say([template], n=write_number(rng, pick(registered))), {"kind": "probe", "caveats": REGISTRATION_CAVEAT, "forbid": FORBID})
    return bank.questions


def _next_day(day):
    import datetime
    return (datetime.datetime.strptime(day, "%Y-%m-%d") + datetime.timedelta(days=1)).strftime("%Y-%m-%d")


def template_bank():
    """Every written phrasing once, with representative values (no case is built): a check that no phrasing is one the engine's frame cannot read."""
    bank = Bank("TPL")
    values = dict(i="001010000001000", n="923000001000", d="2026-03-05")
    for kind, templates in (("numbers_of_sim", NUMBERS_OF_SIM), ("sims_of_number", SIMS_OF_NUMBER), ("in_force", IN_FORCE), ("after_end_count", AFTER_END_COUNT), ("after_end_numbers", AFTER_END_NUMBERS),
                            ("unregistered", UNREGISTERED), ("conflicts", CONFLICTS), ("handset_model", HANDSET_MODEL), ("handset_count", HANDSET_COUNT), ("makers", MAKERS), ("absent", ABSENT),
                            ("probe", PROBES)):
        for text in templates:
            bank.add(kind, text % values, {"kind": "template", "numbers": ["923000001000"] if "%(n)s" in text else [], "imsis": ["001010000001000"] if "%(i)s" in text else []})
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
        truth, events = build_truth(params)
        questions = bank_questions(truth, events, args.seed)
        kinds = collections.Counter(q["type"] for q in questions)
        controls = sum(1 for q in questions if q["expect"].get("control"))
        print("%d questions: %s; controls %d" % (len(questions), dict(kinds), controls))
    if args.out:
        with open(args.out, "w", encoding="utf-8") as handle:
            json.dump(questions, handle, indent=1)
            handle.write("\n")
        print("written", args.out)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
