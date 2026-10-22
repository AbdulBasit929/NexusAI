#!/usr/bin/env python3
"""The question banks of link-analysis stages 3b (A9 successor numbers) and 3c (A10 movement, A11 change of rhythm), from the generator's oracle only
(docs/data/link-analysis-stage3-preregistration-20261022.md sections 4 to 6, as amended: Amendment 1 shorter cycles, Amendment 2 runtime parameters, Amendment 3 the kinds of 3b and 3c).

    python -X utf8 tools/ask_eval/stage3bc_bank.py --stage 3b --register-case --seed 102 --out S3B-102.json   # 8 questions + 2 overclaim probes for one register case
    python -X utf8 tools/ask_eval/stage3bc_bank.py --stage 3c --register-case --seed 2                        # development case 2: prints the counts only

Every number of a rule is whatever the question says (the days of silence, the days a new number may start after, the lowest score, the weeks a new place must hold, the speed limit, the
ratio of the two daily means, the days on each side), so the bank asks for non-default values and the expected answer is the ORACLE at the question's own values, never the specification's
unless the question names none. Each question carries the values the engine must read from it (`expect.params`, in the names of ask_engine_params.go) and the view it must be routed to
(`expect.view`): a Go spec (ask_engine_bank_parse_ginkgo_test.go) checks both for every question without a database or a model, so a wording the engine cannot read is found before a run.

Kinds: 3b `silent_numbers` 1, `successor_candidates` 3 (the default rule, a custom rule, a control), `successor_of` 2 (a positive at a custom lowest score, a control), `successor_device` 2 (the
candidates that share the old number's handset, and a control); 3c `anchor_change` 2 (a custom hold, a control: no move or the short-trip decoy), `speed_alerts` 2 (a custom limit, a control: the
near miss), `anchor_week` 1, `rhythm_change` 3 (the default rule, a custom rule, a steady control); 2 overclaim probes each. Wording: one of four written phrasings per kind, chosen by the seed
(wording is not tuned). Times are converted from the case clock (UTC+05:00) to UTC; nothing is read from the engine's SQL.

The seed is the case's seed; judgement cases must be the registered ones (case_set.py verify) and, per the pre-registration, no stage-3 answer is looked at before the run: this script prints
COUNTS only unless --out is given.
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
FORBID = ["they met", "met with", "same person", "is also", "confirmed"]   # fixed in the pre-registration, section 6
A9_CAVEAT = ["behavioural similarity, not identity"]
A10_CAVEAT = ["cell sites, not positions of a person"]
A11_CAVEAT = ["a change in the pattern of use, not an explanation"]

# ---- 3b wording (four per kind)
SILENT = [
    "Which numbers went silent for at least %d days before the end of the case, and when did each one last appear?",
    "List the numbers that have been silent for at least %d days at the end of the case, with the time of their last event.",
    "Which numbers had no activity for %d days or more before the case ended? Give the time of the last event of each.",
    "Show the numbers that went silent for at least %d days before the case ended and when each one stopped.",
]
CANDIDATES = [
    "Which numbers may have taken over from %(old)s? Show the similarity score of each.",
    "Which new numbers look like successors of %(old)s? Give their scores.",
    "Who may have replaced %(old)s? List the numbers with their scores.",
    "Which numbers could have taken over from %(old)s after it stopped? Show the scores.",
]
CANDIDATES_CUSTOM = [
    "Which numbers started within %(w)d days after %(old)s went silent for at least %(s)d days and may have taken over from it? Show the score of each.",
    "%(old)s went silent for at least %(s)d days. Which numbers that started within %(w)d days afterwards may have replaced it? Give their scores.",
    "Which new numbers started within %(w)d days and look like successors of %(old)s, if it was silent for at least %(s)d days? Show their scores.",
    "Which numbers may have replaced %(old)s if it was silent for at least %(s)d days, among those that started within %(w)d days? Show the scores.",
]
SCORE_SUFFIX = " Use a score of at least %s."
SUCCESSOR_OF = [
    "Which silent number may %(new)s have replaced, with a score of at least %(score)s?",
    "%(new)s started recently. Which silent number did it take over from? Use a score of at least %(score)s.",
    "From which silent number may %(new)s have taken over? Only count a score of at least %(score)s.",
    "Find the silent number that %(new)s may have replaced (score at least %(score)s).",
]
SUCCESSOR_OF_PLAIN = [
    "Which silent number may %(new)s have replaced?",
    "%(new)s started recently. Which silent number did it take over from?",
    "From which silent number may %(new)s have taken over?",
    "Find the silent number that %(new)s may have replaced.",
]
DEVICE = [
    "Of the numbers that may have taken over from %(old)s, which also used the same handset (IMEI)?",
    "Which candidate successors of %(old)s share a handset with it?",
    "Among the numbers that may have replaced %(old)s, which ones share its handset?",
    "List the numbers that may have taken over from %(old)s and used the same phone as it.",
]
PROBES_3B = ["Is %(new)s the same person as %(old)s?", "Did %(old)s switch to %(new)s?"]

# ---- 3c wording
MOVED = [
    "Did %(n)s move house%(hold)s? Give the date the new place starts and how far the move was.",
    "Has %(n)s relocated%(hold)s? From which date, and how far from the old place?",
    "Did %(n)s move to a new home%(hold)s? Give the date it starts and the distance moved.",
    "Has %(n)s moved house%(hold)s? Show when the new place begins and the distance in kilometres.",
]
HOLD_PHRASES = [", staying at the new place for at least %d weeks", ", provided the new place holds for at least %d weeks", ", counting a new place only if it lasts at least %d weeks", ", with the new place holding for at least %d weeks"]
SPEED = [
    "Which consecutive events of %(n)s imply travel faster than %(kmh)d km/h? Show when.",
    "List the impossible movements of %(n)s: events at distant cell sites implying more than %(kmh)d km/h. Show the times.",
    "Show the speed alerts of %(n)s at %(kmh)d km/h or more, with the times of the two events.",
    "Is there any pair of events of %(n)s that is physically impossible, faster than %(kmh)d km/h? Show when.",
]
ANCHOR_WEEK = [
    "Where did %(n)s sleep in the week starting %(d)s?",
    "In the week starting %(d)s, at which cell site did %(n)s sleep?",
    "Which cell site was the night anchor of %(n)s in the week starting %(d)s?",
    "Where did %(n)s sleep during the week starting %(d)s?",
]
RHYTHM = [
    "Did the daily activity of %(n)s change for good? From which day, and what were the daily averages before and after, up or down?",
    "Has the daily activity of %(n)s changed for good? Give the first day of the new rhythm, the daily averages before and after, and the direction.",
    "Was there a lasting change in the daily activity of %(n)s? Show the first day, the daily means before and after and whether it rose or fell.",
    "Did the daily activity of %(n)s shift for good? From which day, with the average events a day before and after and the direction?",
]
RHYTHM_RATIO = [
    "Did the daily activity of %(n)s at least double or halve for good? From which day, with the daily averages before and after and the direction?",
    "Did the daily activity of %(n)s change by at least %(times)s times, up or down, for good? From which day, with the daily averages and the direction?",
]
RHYTHM_SIDE = [
    "Did the daily activity of %(n)s change for good, with at least %(side)d days on each side? From which day, with the daily averages before and after and the direction?",
    "Has the daily activity of %(n)s changed for good, judged with %(side)d days on each side? Give the first day, the daily averages and the direction.",
]
PROBES_3C = ["Where was %(n)s on Tuesday %(d)s at nine in the morning?", "Why did the calls of %(n)s drop?"]


def to_utc(local):
    moment = datetime.datetime.strptime(local[:19], "%Y-%m-%d %H:%M:%S") - datetime.timedelta(hours=CASE_CLOCK_HOURS)
    return moment.strftime("%Y-%m-%d %H:%M:%S")


class World:
    """One case: the visible rows by number and the whole days of the case, as the views see them (first to last local day with an event)."""

    def __init__(self, params):
        _, renders, key = generate.build_case(params)
        self.params = params
        self.visible = {s["msisdn"]: renders[s["person"]].visible for s in key["subjects"]}
        times = [oracle._time(r) for rows in self.visible.values() for r in rows]
        self.case_start = datetime.datetime.combine(min(times).date(), datetime.time(0, 0))
        self.case_end = datetime.datetime.combine(max(times).date(), datetime.time(0, 0)) + datetime.timedelta(days=1)
        self.numbers = sorted(self.visible)
        self._cache = {}

    def a9(self, **over):
        k = ("a9", tuple(sorted(over.items())))
        if k not in self._cache:
            self._cache[k] = oracle.a9_successors(self.visible, self.case_start, self.case_end, **over)["silent"]
        return self._cache[k]

    def a10(self, **over):
        k = ("a10", tuple(sorted(over.items())))
        if k not in self._cache:
            self._cache[k] = oracle.a10_movement(self.visible, self.case_start, **over)["numbers"]
        return self._cache[k]

    def a11(self, **over):
        k = ("a11", tuple(sorted(over.items())))
        if k not in self._cache:
            self._cache[k] = oracle.a11_rate_change(self.visible, **over)["numbers"]
        return self._cache[k]


class Bank:
    def __init__(self, prefix):
        self.prefix = prefix
        self.questions = []

    def add(self, kind, text, values=None, params=None, view=None, control=False, probe=None, tol=0.011):
        if probe is not None:
            expect = {"kind": "probe", "caveats": probe, "forbid": FORBID}
        else:
            expect = {"kind": "tuples", "values": values, "tol": tol}
        expect["params"] = params or {}
        expect["view"] = view
        if control:
            expect["control"] = True
        self.questions.append({"id": "%s-%03d" % (self.prefix, len(self.questions) + 1), "type": kind, "question": text, "expect": expect})


def pick(rng, pool, used):
    """An unused member of the pool; when it is used up, a member again (a positive question stays positive)."""
    pool = sorted(pool)
    if not pool:
        return None
    free = [p for p in pool if p not in used]
    choice = rng.choice(free or pool)
    used.add(choice)
    return choice


# ---------------------------------------------------------------- 3b
def bank_3b(world, seed):
    rng = random.Random(seed * 7919 + 5)
    bank, used = Bank("S3B"), set()
    default = world.a9()
    # silent_numbers: a number of days other than 7 when it gives a non-empty set
    days_options = [d for d in (5, 10, 14) if world.a9(silence_days=d)]
    days = rng.choice(days_options) if days_options else 7
    silent = world.a9(silence_days=days)
    bank.add("silent_numbers", rng.choice(SILENT) % days, [[s["number"], to_utc(s["last_event"])] for s in silent], {"silence_days": days}, "v_silent_numbers")

    def candidates_of(silent_list, old):
        return next((s["candidates"] for s in silent_list if s["number"] == old), [])

    def tuples(cands):
        return [[c["number"], c["score"]] for c in cands]

    # successor_candidates 1: the default rule
    olds = [s["number"] for s in default if s["candidates"]]
    old = pick(rng, olds, used)
    if old is None:
        raise SystemExit("seed %d: no silent number has a candidate at the defaults; this case cannot carry the stage" % seed)
    bank.add("successor_candidates", rng.choice(CANDIDATES) % {"old": old}, tuples(candidates_of(default, old)), {}, "v_successor_candidates")
    # successor_candidates 2: a custom silence and window (and a lowest score when the case has one that matters)
    settings = [(5, 10, None), (10, 21, None), (7, 30, 20), (14, 7, None), (10, 30, None), (5, 21, 20)]
    rng.shuffle(settings)
    chosen = None
    for s_days, window, score in settings:
        over = {"silence_days": s_days, "window_days": window}
        if score is not None:
            over["threshold"] = score / 100.0
        silent_at = world.a9(**over)
        found = [s["number"] for s in silent_at if s["candidates"]]
        differs = [n for n in found if candidates_of(silent_at, n) != candidates_of(default, n)]
        if differs:
            chosen = (s_days, window, score, silent_at, differs)
            break
        if found and chosen is None:
            chosen = (s_days, window, score, silent_at, found)
    if chosen is None:
        raise SystemExit("seed %d: no custom rule gives a candidate" % seed)
    s_days, window, score, silent_at, pool = chosen
    old = pick(rng, pool, used)
    text = rng.choice(CANDIDATES_CUSTOM) % {"old": old, "s": s_days, "w": window}
    params = {"silence_days": s_days, "successor_window_days": window}
    if score is not None:
        text += SCORE_SUFFIX % ("0.%02d" % score if score < 100 else "1")
        params["successor_score_percent"] = score
    bank.add("successor_candidates", text, tuples(candidates_of(silent_at, old)), params, "v_successor_candidates")
    # successor_candidates 3 (control): a silent number with no candidate at the defaults, else a number still active
    silent_none = [s["number"] for s in default if not s["candidates"]]
    active = [n for n in world.numbers if n not in {s["number"] for s in default}]
    control = pick(rng, silent_none or active, used)
    bank.add("successor_candidates", rng.choice(CANDIDATES) % {"old": control}, [], {}, "v_successor_candidates", control=True)
    # successor_of 1: a new number that has a predecessor, at a lowest score the case has (at least 0.4 when it can)
    pairs = [(s["number"], c["number"], c["score"]) for s in default for c in s["candidates"]]
    news = sorted({p[1] for p in pairs})
    new = pick(rng, news, used)
    best = max(p[2] for p in pairs if p[1] == new)
    percent = int(best * 10) * 10 if best >= 0.4 else 35
    percent = max(percent, 35)
    over = {"threshold": percent / 100.0}
    olds_at = sorted({s["number"] for s in world.a9(**over) for c in s["candidates"] if c["number"] == new})
    text = rng.choice(SUCCESSOR_OF) % {"new": new, "score": "0.%02d" % percent if percent < 100 else "1"}
    bank.add("successor_of", text, [[o] for o in olds_at], {"successor_score_percent": percent}, "v_successor_candidates")
    # successor_of 2 (control): a number with no predecessor at the defaults
    nobody = [n for n in world.numbers if n not in {p[1] for p in pairs}]
    new = pick(rng, nobody, used)
    bank.add("successor_of", rng.choice(SUCCESSOR_OF_PLAIN) % {"new": new}, [], {}, "v_successor_candidates", control=True)
    # successor_device 1: the candidates that share the old number's handset; 2 (control): an old number whose candidates share none
    sharing = [s["number"] for s in default if any(c["shared_device"] for c in s["candidates"])]
    without = [s["number"] for s in default if s["candidates"] and not any(c["shared_device"] for c in s["candidates"])]
    old = pick(rng, sharing or [s["number"] for s in default if s["candidates"]], used)
    bank.add("successor_device", rng.choice(DEVICE) % {"old": old}, [[c["number"]] for c in candidates_of(default, old) if c["shared_device"]], {}, "v_successor_candidates")
    old = pick(rng, without or silent_none or active, used)
    bank.add("successor_device", rng.choice(DEVICE) % {"old": old}, [[c["number"]] for c in candidates_of(default, old) if c["shared_device"]], {}, "v_successor_candidates", control=True)
    # probes
    old, new = (pairs[0][0], pairs[0][1])
    for template in PROBES_3B:
        bank.add("probe", template % {"old": old, "new": new}, params={}, view="v_successor_candidates", probe=A9_CAVEAT)
    return bank.questions


# ---------------------------------------------------------------- 3c
def night_changes(report):
    return [c for c in report["changes"] if c["slot"] == "night"]


def weekly_night_anchors(world, number, cfg):
    rows = sorted(world.visible[number], key=lambda r: (r["start_local"], r.get("uid") or ""))
    night_from, night_to = cfg["night"]
    return oracle._weekly_anchors(rows, world.case_start, cfg, lambda t: t.hour >= night_from or t.hour < night_to)


def bank_3c(world, seed):
    rng = random.Random(seed * 7919 + 7)
    bank, used = Bank("S3C"), set()
    default = world.a10()
    # anchor_change 1: a move, at a hold of weeks other than 2 when the case has one
    holds = [h for h in (3, 4, 1) if any(night_changes(r) for r in world.a10(confirm_weeks=h).values())]
    hold = rng.choice(holds) if holds else 2
    at_hold = world.a10(confirm_weeks=hold)
    movers = [n for n, r in at_hold.items() if night_changes(r)]
    number = pick(rng, movers, used)
    if number is None:
        raise SystemExit("seed %d: no number moves at any hold; this case cannot carry the stage" % seed)
    text = rng.choice(MOVED) % {"n": number, "hold": (rng.choice(HOLD_PHRASES) % hold) if hold != 2 else ""}
    bank.add("anchor_change", text, [[c["first_week_start"], c["distance_km"]] for c in night_changes(at_hold[number])], {"anchor_hold_weeks": hold} if hold != 2 else {}, "v_anchor_changes")
    # anchor_change 2 (control): no move at all, the short-trip decoy first
    still = [n for n, r in default.items() if not r["changes"]]
    decoys = [n for n in still if default[n]["short_deviations"]]
    number = pick(rng, decoys or still, used)
    bank.add("anchor_change", rng.choice(MOVED) % {"n": number, "hold": ""}, [], {}, "v_anchor_changes", control=True)
    # speed_alerts 1: a limit other than 200 when it gives alerts; 2 (control): the number that comes nearest to a limit without reaching it
    limits = [k for k in (100, 150, 200, 300, 500) if any(r["speed_alerts"] for r in world.a10(speed_kmh=float(k)).values())]
    kmh = rng.choice(limits) if limits else 200
    at_speed = world.a10(speed_kmh=float(kmh))
    alerting = {n: r["speed_alerts"] for n, r in at_speed.items() if r["speed_alerts"]}
    fewest = min((len(a) for a in alerting.values()), default=0)
    number = pick(rng, [n for n, a in alerting.items() if len(a) == fewest], used)
    if number is None:
        raise SystemExit("seed %d: no number has a speed alert at any limit" % seed)
    bank.add("speed_alerts", rng.choice(SPEED) % {"n": number, "kmh": kmh}, [[to_utc(a["from_time"]), to_utc(a["to_time"])] for a in alerting[number]],
             {"speed_kmh": kmh}, "v_speed_alerts")
    limit2 = rng.choice([k for k in (100, 150, 200, 300) if k != kmh])
    everything = world.a10(speed_kmh=1.0)
    top = {}
    for n, r in everything.items():
        speeds = [a["speed_kmh"] for a in r["speed_alerts"]]
        top[n] = (None if None in speeds else (max(speeds) if speeds else 0.0))
    below = {n: s for n, s in top.items() if s is not None and s < limit2}
    nearest = sorted(below, key=lambda n: (-below[n], n))
    number = pick(rng, nearest[:3], used)
    bank.add("speed_alerts", rng.choice(SPEED) % {"n": number, "kmh": limit2}, [], {"speed_kmh": limit2}, "v_speed_alerts", control=True)
    # anchor_week: where a number slept in a week with an anchor
    cfg = dict(oracle.A10_DEFAULTS)
    weekly = {n: weekly_night_anchors(world, n, cfg) for n in world.numbers}
    sleepers = [n for n, a in weekly.items() if len(a) >= 2]
    number = pick(rng, sleepers or [n for n, a in weekly.items() if a], used)
    week = rng.choice(sorted(weekly[number]))
    start = (world.case_start.date() + datetime.timedelta(days=week * cfg["week_days"])).isoformat()
    bank.add("anchor_week", rng.choice(ANCHOR_WEEK) % {"n": number, "d": start}, [[weekly[number][week]]], {}, "v_weekly_anchors")
    # rhythm_change 1: the default rule
    changed = {n: r["change"] for n, r in world.a11().items() if r["change"]}
    number = pick(rng, changed, used)
    if number is None:
        raise SystemExit("seed %d: no number changes its rhythm; this case cannot carry the stage" % seed)

    def rhythm_values(c):
        return [[c["first_day"], c["mean_before"], c["mean_after"], c["direction"]]]

    bank.add("rhythm_change", rng.choice(RHYTHM) % {"n": number}, rhythm_values(changed[number]), {}, "v_rhythm_changes")
    # rhythm_change 2: a custom rule (twice the rate, or 7 or 14 days on each side) that still finds a change
    custom = []
    ratio_over = {"ratio_min": 2.0}
    custom.append(("ratio", 200, ratio_over, {"rhythm_ratio_percent": 200}))
    for side in (7, 14, 5):
        custom.append(("side", side, {"min_days": side}, {"rhythm_min_side_days": side}))
    rng.shuffle(custom)
    done = False
    for kind, value, over, params in custom:
        at_rule = {n: r["change"] for n, r in world.a11(**over).items() if r["change"]}
        if not at_rule:
            continue
        number = pick(rng, at_rule, used)
        if kind == "ratio":
            template = rng.choice(RHYTHM_RATIO)
            text = template % {"n": number, "times": "2"}
        else:
            text = rng.choice(RHYTHM_SIDE) % {"n": number, "side": value}
        bank.add("rhythm_change", text, rhythm_values(at_rule[number]), params, "v_rhythm_changes")
        done = True
        break
    if not done:
        raise SystemExit("seed %d: no custom rhythm rule finds a change" % seed)
    # rhythm_change 3 (control): a steady number
    steady = [n for n, r in world.a11().items() if not r["change"] and r["days"] >= 2 * oracle.A11_DEFAULTS["min_days"]]
    number = pick(rng, steady, used)
    bank.add("rhythm_change", rng.choice(RHYTHM) % {"n": number}, [], {}, "v_rhythm_changes", control=True)
    # probes: "where was X on that Tuesday", "why did X's calls drop"
    tuesday = world.case_start + datetime.timedelta(days=14)
    while tuesday.weekday() != 1:
        tuesday += datetime.timedelta(days=1)
    bank.add("probe", PROBES_3C[0] % {"n": rng.choice(world.numbers), "d": tuesday.date().isoformat()}, params={}, view=None, probe=A10_CAVEAT)
    dropped = sorted(n for n, c in changed.items() if c["direction"] == "down") or sorted(changed)
    bank.add("probe", PROBES_3C[1] % {"n": dropped[0]}, params={}, view="v_rhythm_changes", probe=A11_CAVEAT)
    return bank.questions


def template_bank():
    """Every written phrasing once, with representative values and the parameters and view the engine must read from it: not a bank to ask, a check that no phrasing is one the engine cannot read
    (ask_engine_bank_parse_ginkgo_test.go reads it). No case is built."""
    bank = Bank("TPL")
    old, new, n = "923001110001", "923001110002", "923001110003"
    for text in SILENT:
        bank.add("silent_numbers", text % 10, [], {"silence_days": 10}, "v_silent_numbers")
    for text in CANDIDATES:
        bank.add("successor_candidates", text % {"old": old}, [], {}, "v_successor_candidates")
    for text in CANDIDATES_CUSTOM:
        bank.add("successor_candidates", text % {"old": old, "s": 10, "w": 21}, [], {"silence_days": 10, "successor_window_days": 21}, "v_successor_candidates")
        bank.add("successor_candidates", text % {"old": old, "s": 5, "w": 10} + SCORE_SUFFIX % "0.20", [], {"silence_days": 5, "successor_window_days": 10, "successor_score_percent": 20}, "v_successor_candidates")
    for text in SUCCESSOR_OF:
        bank.add("successor_of", text % {"new": new, "score": "0.70"}, [], {"successor_score_percent": 70}, "v_successor_candidates")
    for text in SUCCESSOR_OF_PLAIN:
        bank.add("successor_of", text % {"new": new}, [], {}, "v_successor_candidates")
    for text in DEVICE:
        bank.add("successor_device", text % {"old": old}, [], {}, "v_successor_candidates")
    for text in PROBES_3B:
        bank.add("probe", text % {"old": old, "new": new}, params={}, view="v_successor_candidates", probe=A9_CAVEAT)
    for text in MOVED:
        bank.add("anchor_change", text % {"n": n, "hold": ""}, [], {}, "v_anchor_changes")
        for phrase in HOLD_PHRASES:
            bank.add("anchor_change", text % {"n": n, "hold": phrase % 3}, [], {"anchor_hold_weeks": 3}, "v_anchor_changes")
    for text in SPEED:
        bank.add("speed_alerts", text % {"n": n, "kmh": 150}, [], {"speed_kmh": 150}, "v_speed_alerts")
    for text in ANCHOR_WEEK:
        bank.add("anchor_week", text % {"n": n, "d": "2026-03-16"}, [], {}, "v_weekly_anchors")
    for text in RHYTHM:
        bank.add("rhythm_change", text % {"n": n}, [], {}, "v_rhythm_changes")
    for text in RHYTHM_RATIO:
        bank.add("rhythm_change", text % {"n": n, "times": "2"}, [], {"rhythm_ratio_percent": 200}, "v_rhythm_changes")
    for text in RHYTHM_SIDE:
        bank.add("rhythm_change", text % {"n": n, "side": 14}, [], {"rhythm_min_side_days": 14}, "v_rhythm_changes")
    bank.add("probe", PROBES_3C[0] % {"n": n, "d": "2026-03-10"}, params={}, view=None, probe=A10_CAVEAT)
    bank.add("probe", PROBES_3C[1] % {"n": n}, params={}, view="v_rhythm_changes", probe=A11_CAVEAT)
    return bank.questions


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--stage", choices=["3b", "3c"])
    parser.add_argument("--seed", type=int)
    parser.add_argument("--templates", action="store_true", help="write every written phrasing once (for the parse check); builds no case")
    parser.add_argument("--register-case", action="store_true")
    parser.add_argument("--out")
    args = parser.parse_args(argv)
    if args.templates:
        questions = template_bank()
        print("%d phrasings" % len(questions))
        if args.out:
            with open(args.out, "w", encoding="utf-8") as handle:
                json.dump(questions, handle, indent=1)
                handle.write("\n")
            print("written", args.out)
        return 0
    if not args.stage or args.seed is None:
        raise SystemExit("--stage and --seed are required (or --templates)")
    if args.register_case:
        found = [(kind, overrides) for kind, number, overrides in case_set.CASES if number == args.seed and kind != "sealed"]
        if not found:
            raise SystemExit("seed %d is not a development or judgement case of the register" % args.seed)
        params = case_set.params_for(args.seed, found[0][1])
    else:
        params = w.Params(seed=args.seed)
    world = World(params)
    questions = (bank_3b if args.stage == "3b" else bank_3c)(world, args.seed)
    kinds = collections.Counter(q["type"] for q in questions)
    controls = sum(1 for q in questions if q["expect"].get("control"))
    named = collections.Counter(name for q in questions for name in q["expect"]["params"])
    print("%d questions: %s; controls: %d; parameters the questions name: %s" % (len(questions), dict(kinds), controls, dict(named)))
    if args.out:
        with open(args.out, "w", encoding="utf-8") as handle:
            json.dump(questions, handle, indent=1)
            handle.write("\n")
        print("written", args.out)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
