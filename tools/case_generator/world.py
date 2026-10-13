"""The canonical truth of a synthetic case: people, subscriptions, devices, cell sites, the contact structure (plant P1), the shared
contacts with their decoys (plant P2) and the stream of events every file and every answer derives from.

Specification: docs/data/case-generator-spec.md. Everything is drawn from the reserved ranges of tools/real_data/reserved.py.
Only the standard library is used. The same Params give the same world, event for event.
"""
from __future__ import annotations

import datetime
import json
import math
import os
import random
import sys
from dataclasses import asdict, dataclass, field

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, os.path.join(REPO, "tools", "real_data"))
import make_golden as mg  # noqa: E402
import reserved  # noqa: E402

GENERATOR_VERSION = "0.6"
SPEC_VERSION = "0.6"
LOCAL_ZONE = "Asia/Karachi"
LOCAL_UTC_OFFSET_HOURS = 5   # Asia/Karachi has no daylight saving time

OPERATOR_LABELS = ["ufone", "zong", "mobilink", "telenor"]
OPERATOR_LAYOUTS = ["call-c1-company", "call-c2-strt-tm", "call-c3-ufone-style", "call-c4-telenor-style"]
# The tower-dump layout of each operator (ufone, zong, mobilink, telenor), by the operator labels of the layout entries.
OPERATOR_DUMP_LAYOUTS = ["geofence-c7-ufone", "geofence-c8-zong", "geofence-c5-mobilink", "geofence-c6-telenor"]
# The internet-record export of each operator (ufone I3, zong I4, mobilink I1, telenor I2); I3 writes port ranges, the others one port.
OPERATOR_IPDR_LAYOUTS = ["ipdr-i3-ufone-xls", "ipdr-i4-zong-txt", "ipdr-i1-jazz-ufdr", "ipdr-i2-telenor-export"]
APP_USAGE_LAYOUT = "appusage-a1"
PORT_BLOCK = {0: 64}                  # operators whose layout writes a range allocate blocks of this many ports; the others one port
# Carrier-grade address translation: a few public addresses per operator (documentation block), shared by many subscriptions.
PUBLIC_POOL = [[reserved.synthetic_ipv4(op * 3 + k) for k in range(3)] for op in range(4)]
SERVER_IPS = [reserved.synthetic_ipv4(300 + k) for k in range(8)]
APPS = [("Social Networking", "app_a", "feature_a"), ("Video", "app_b", "feature_b"), ("Messaging", "app_c", "feature_a"), ("Web Browsing", "app_a", "feature_b")]
# Service counterparties: short codes and sender ids cannot be real person numbers (they are the ones the goldens use).
SERVICES = [("short_code", s) for s in mg.SHORT_CODES] + [("sender_id", s) for s in mg.SENDERS]
DECOY_SERVICE = "786"
HOUR_WEIGHTS = [1, 1, 1, 1, 1, 2, 4, 6, 8, 9, 9, 9, 8, 8, 9, 9, 9, 9, 8, 8, 7, 6, 4, 2]
CELLS_PER_OPERATOR = 12


@dataclass
class Params:
    seed: int = 1
    people: int = 120            # persons in the world (services are extra)
    subjects: int = 12
    associates: int = 24
    period_days: int = 90
    start_date: str = "2026-02-02"
    events_per_person_per_day: float = 4.0   # events a person originates per day; each also involves a second party
    communities: int = 6
    p2_pairs: int = 5
    p3_pairs: int = 2            # co-location plants; each needs a pair of subjects on the same operator
    p6_pairs: int = 2            # successor-number pairs (plant P6); they take the same-operator pairs P3 leaves
    p5: int = 1                  # 1 plants the device and SIM changes (plant P5), 0 leaves identities alone
    p8: int = 1                  # 1 plants movement and rhythm (plant P8) on the subjects left over by the other parts, 0 leaves them alone
    windows_per_plant: int = 4
    window_minutes: int = 15     # W of the co-location definition (specification section 10)
    dumps: bool = True           # one tower dump per operator that has a subject
    dump_days: int = 2
    sessions_per_subject_per_day: float = 6.0
    app_usage_exports: int = 2   # subjects that also get an app-usage export (layout A1)
    defects_per_class: int = 3   # rows damaged on purpose per defect class and call-record export (0 = none)
    meeting_windows: int = 2     # co-location windows (per P3 plant) that are preceded by a direct contact: plant P4
    contact_gap_minutes: int = 60   # G of the possible-meeting definition (specification section 10)
    p7_plants: int = 3           # address-translation plants, on operators whose layout gives a session end (not ufone)
    shared_min: int = 3
    shared_max: int = 5
    hub_threshold: int | None = None  # distinct subjects (files) in contact with a number above which it is a hub; default subjects - 2

    def resolved_hub_threshold(self) -> int:
        return self.hub_threshold if self.hub_threshold is not None else max(2, self.subjects - 2)


@dataclass
class Person:
    pid: str
    role: str                    # subject | associate | bystander
    community: int
    operator: int
    n: int                       # index into the reserved number block
    msisdn: str
    imsi: str
    imei: str
    home_cell: int
    work_cell: int
    segments: list = field(default_factory=list)   # identity changes: {"from": local time, "imsi": str, "imei_n": int}, oldest first


@dataclass
class Event:
    uid: str
    kind: str                    # call | sms
    orig: str                    # person id or service label
    recip: str
    start: datetime.datetime     # local (Asia/Karachi) wall clock
    duration_s: int
    plant: str | None = None     # plant id (or plant:detail tag) for planted events
    cells: dict | None = None    # person id -> cell site index, for events whose place is planted


@dataclass
class Session:
    uid: str
    pid: str
    start: datetime.datetime
    end: datetime.datetime
    nat_start: datetime.datetime
    nat_end: datetime.datetime
    ue_ip: str
    ue_port: int
    public_ip: str
    port_from: int
    port_to: int
    server_ip: str
    server_port: int
    protocol_num: int
    bytes_up: int
    bytes_down: int
    packets_up: int
    packets_down: int
    app: tuple
    site: int
    plant: str | None = None


@dataclass
class World:
    params: Params
    persons: list = field(default_factory=list)
    events: list = field(default_factory=list)
    structure: dict = field(default_factory=dict)   # plant P1: communities, chains, bridges, hubs
    p2: list = field(default_factory=list)          # plant P2 records
    p3: list = field(default_factory=list)          # plant P3 records
    sessions: list = field(default_factory=list)    # data sessions (subjects, and the subscriptions of plant P7)
    p7: list = field(default_factory=list)          # plant P7 records
    p5: list = field(default_factory=list)          # plant P5 records
    p6: list = field(default_factory=list)          # plant P6 records
    active: dict = field(default_factory=dict)      # person id -> (first day, last day) on which the number is in use, where it is not the whole period
    roles: dict = field(default_factory=dict)       # which subjects play which part (allocate_roles)
    p8: list = field(default_factory=list)          # plant P8 records
    moves: dict = field(default_factory=dict)       # person id -> home changes: {"from_day", "to_day" (None = for good), "cell"}, oldest first
    rates: dict = field(default_factory=dict)       # person id -> {"change": [from_day, multiplier], "outlier": [day, multiplier]} on the calls the person originates

    def person(self, pid):
        return next(p for p in self.persons if p.pid == pid)


def _poisson(rng, lam):
    limit, k, p = math.exp(-lam), 0, 1.0
    while True:
        p *= rng.random()
        if p <= limit:
            return k
        k += 1


def _hour(rng):
    return rng.choices(range(24), weights=HOUR_WEIGHTS)[0]


def cell_geometry(operator, idx):
    """A fictional grid in the open ocean (specification section 8): about 2 km apart, a slightly different origin per operator."""
    lat = 0.40 + 0.02 * (idx // 4) + 0.0007 * operator
    lon = -30.60 + 0.02 * (idx % 4) + 0.0011 * operator
    return round(lat, 6), round(lon, 6)


def identity_at(person, when):
    """(IMSI, device number) a person's handset reports at a local time: the last identity segment that has begun, else the original."""
    imsi, imei_n = person.imsi, person.n
    stamp = when.isoformat(sep=" ")
    for segment in person.segments:
        if segment["from"] <= stamp:
            imsi, imei_n = segment["imsi"], segment["imei_n"]
    return imsi, imei_n


def allocate_roles(subjects, params):
    """Who plays which part, so that plants do not collide. Subjects on one operator are paired (cell identities are per operator):
    the first `p3_pairs` pairs carry co-location (P3), the next `p6_pairs` carry a successor number (P6, old first); the subjects
    left over are the silent number without a successor and the late starter without a predecessor (P6 decoys), then the free ones."""
    r = random.Random("roles:%d" % params.seed)
    by_operator = {}
    for s in subjects:
        by_operator.setdefault(s.operator, []).append(s.pid)
    pairs, singles = [], []
    for op in sorted(by_operator):
        members = list(by_operator[op])
        r.shuffle(members)
        while len(members) >= 2:
            pairs.append(tuple(sorted((members.pop(), members.pop()))))   # ordered by subject number, so x < y wherever a pair is named
        singles += members
    r.shuffle(pairs)
    r.shuffle(singles)
    p3 = pairs[: params.p3_pairs]
    p6 = pairs[params.p3_pairs: params.p3_pairs + params.p6_pairs]
    for a, b in pairs[params.p3_pairs + params.p6_pairs:]:
        singles += [a, b]
    silent = singles.pop(0) if p6 and singles else None
    late = singles.pop(0) if p6 and singles else None
    return {"p3_pairs": p3, "p6_pairs": p6, "p6_silent": silent, "p6_late": late, "free": singles, "p8": singles[:2] if params.p8 else []}


def build_world(params: Params) -> World:
    world = World(params)
    rng = random.Random("world:%d" % params.seed)
    n_people = params.people
    roles = ["subject"] * params.subjects + ["associate"] * params.associates
    roles += ["bystander"] * (n_people - len(roles))
    # persons: ids P-000.., numbers from the reserved block (offset 1000 keeps clear of the numbers the goldens use)
    for i, role in enumerate(roles):
        community = i % params.communities if role != "bystander" else rng.randrange(params.communities)
        operator = i % len(OPERATOR_LABELS) if role == "subject" else rng.randrange(len(OPERATOR_LABELS))
        n = 1000 + i
        world.persons.append(Person(
            pid="P-%03d" % i, role=role, community=community, operator=operator, n=n, msisdn=reserved.synthetic_msisdn(n),
            imsi=reserved.synthetic_imsi(n), imei=reserved.synthetic_imei(n),
            home_cell=rng.randrange(CELLS_PER_OPERATOR), work_cell=rng.randrange(CELLS_PER_OPERATOR)))
    persons = world.persons
    world.roles = allocate_roles([p for p in persons if p.role == "subject"], params)
    by_community = {}
    for p in persons:
        by_community.setdefault(p.community, []).append(p.pid)

    # --- plant P1: the contact structure -------------------------------------------------------------------------------------------
    neighbours = {p.pid: {} for p in persons}

    def link(a, b, weight):
        neighbours[a][b] = max(neighbours[a].get(b, 0), weight)
        neighbours[b][a] = max(neighbours[b].get(a, 0), weight)

    for members in by_community.values():
        for i, a in enumerate(members):
            for b in members[i + 1:]:
                if rng.random() < 0.35:
                    link(a, b, round(0.5 + rng.random(), 3))
    bridges = []
    ids = [p.pid for p in persons]
    for _ in range(max(2, params.communities)):
        a, b = rng.sample(ids, 2)
        if persons[int(a[2:])].community != persons[int(b[2:])].community:
            link(a, b, 0.15)
            bridges.append([a, b])
    chains = []
    non_subjects = [p.pid for p in persons if p.role != "subject"]
    for _ in range(3):
        chain = rng.sample(non_subjects, 4)
        for a, b in zip(chain, chain[1:]):
            link(a, b, 3.0)
        chains.append(chain)
    bystanders = [p.pid for p in persons if p.role == "bystander"]
    hub = bystanders[0]
    world.structure = {"communities": {str(k): v for k, v in sorted(by_community.items())}, "bridges": bridges, "chains": chains, "hubs": [hub]}
    for pid, adj in neighbours.items():
        if not adj:  # nobody is isolated
            other = rng.choice([x for x in ids if x != pid])
            link(pid, other, 0.5)

    # --- plant P6: successor numbers, a silent number and a late starter ---------------------------------------------------------------
    # An old number goes silent on a known day; a new number on the same operator starts a few days later with the old one's contacts (their
    # edges are copied) and cell footprint (same home and work cells); in the first pair the new number also reuses the old handset. Decoys:
    # a number that goes silent with no successor, and a number that starts late with contacts of its own.
    r6 = random.Random("p6:%d" % params.seed)
    early_by_pid = {p.pid: p for p in persons}
    horizon = params.period_days
    roles = world.roles
    for number, pair in enumerate(roles["p6_pairs"], start=1):
        old, new = pair if r6.random() < 0.5 else (pair[1], pair[0])
        stop = int(horizon * (0.40 + 0.08 * (number - 1)))
        gap = r6.randint(1, 3)
        with_device = number % 2 == 1
        o, n_ = early_by_pid[old], early_by_pid[new]
        n_.home_cell, n_.work_cell = o.home_cell, o.work_cell
        for q in list(neighbours[new]):
            neighbours[q].pop(new, None)
        neighbours[new] = {}
        for q, weight in sorted(neighbours[old].items()):
            if q != new:
                link(new, q, weight)
        if with_device:
            n_.segments.append({"from": "0000-01-01 00:00:00", "imsi": n_.imsi, "imei_n": o.n})
        world.active[old] = (0, stop - 1)
        world.active[new] = (stop + gap, horizon - 1)
        world.p6.append({"id": "P6-%03d" % number, "kind": "successor", "old": old, "new": new, "last_day_old": stop - 1, "first_day_new": stop + gap, "with_device": with_device})
    if roles["p6_silent"]:
        world.active[roles["p6_silent"]] = (0, int(horizon * 0.62) - 1)
        world.p6.append({"id": "P6-%03d" % (len(world.p6) + 1), "kind": "silent_without_successor", "pid": roles["p6_silent"], "last_day": int(horizon * 0.62) - 1})
    if roles["p6_late"]:
        world.active[roles["p6_late"]] = (int(horizon * 0.30), horizon - 1)
        world.p6.append({"id": "P6-%03d" % (len(world.p6) + 1), "kind": "late_without_predecessor", "pid": roles["p6_late"], "first_day": int(horizon * 0.30)})
    p6_subjects = {pid for pair in roles["p6_pairs"] for pid in pair} | {roles["p6_silent"], roles["p6_late"]} - {None}

    # --- plant P8 (plan): movement and rhythm ------------------------------------------------------------------------------------------
    # The first free subject moves house for good (the night anchor changes) and has two impossible-speed pairs and a slow decoy planted below;
    # the second has one week away from home (a trip: the anchor changes and returns), one day with six times the usual calls (an outlier) and,
    # from a known day, four times the calls it originates (a change of rhythm). A period under 42 days is too short for either.
    r8 = random.Random("p8:%d" % params.seed)
    p8_subjects = list(roles["p8"]) if params.period_days >= 42 else []

    def far_from(cell):
        return max(range(CELLS_PER_OPERATOR), key=lambda c: ((c // 4 - cell // 4) ** 2 + (c % 4 - cell % 4) ** 2, c))
    if p8_subjects:
        a = early_by_pid[p8_subjects[0]]
        day_move = int(horizon * 0.5) + r8.randint(-3, 3)
        world.moves[a.pid] = [{"from_day": day_move, "to_day": None, "cell": far_from(a.home_cell)}]
        world.p8.append({"id": "P8-001", "kind": "relocation", "pid": a.pid, "from_day": day_move, "old_cell": a.home_cell, "new_cell": far_from(a.home_cell)})
    if len(p8_subjects) > 1:
        b = early_by_pid[p8_subjects[1]]
        day_rate = int(horizon * 0.55) + r8.randint(-3, 3)
        day_outlier = int(horizon * 0.25) + r8.randint(-3, 3)
        trip_from = 7 * int(horizon * 0.8 / 7)   # a whole week of the case, so the week of the trip is one week
        world.rates[b.pid] = {"change": [day_rate, 4.0], "outlier": [day_outlier, 6.0]}
        world.moves[b.pid] = [{"from_day": trip_from, "to_day": trip_from + 6, "cell": far_from(b.home_cell)}]
        world.p8.append({"id": "P8-005", "kind": "rate_change", "pid": b.pid, "from_day": day_rate, "multiplier": 4.0})
        world.p8.append({"id": "P8-006", "kind": "outlier_day", "pid": b.pid, "day": day_outlier, "multiplier": 6.0})
        world.p8.append({"id": "P8-007", "kind": "short_trip", "pid": b.pid, "from_day": trip_from, "to_day": trip_from + 6, "home_cell": b.home_cell, "trip_cell": far_from(b.home_cell)})

    # --- background events ------------------------------------------------------------------------------------------------------------
    start = datetime.date.fromisoformat(params.start_date)
    raw = []
    for day in range(params.period_days):
        date = start + datetime.timedelta(days=day)
        weekend = date.weekday() >= 5
        for p in persons:
            r = random.Random("ev:%d:%s:%d" % (params.seed, p.pid, day))
            factor = 1.0
            rate = world.rates.get(p.pid)
            if rate:
                factor = rate["change"][1] if day >= rate["change"][0] else 1.0
                if day == rate["outlier"][0]:
                    factor = rate["outlier"][1]
            for _ in range(_poisson(r, params.events_per_person_per_day * (0.6 if weekend else 1.0) * factor)):
                adj = sorted(neighbours[p.pid].items())
                target = r.choices([k for k, _ in adj], weights=[w for _, w in adj])[0]
                when = datetime.datetime.combine(date, datetime.time(_hour(r), r.randrange(60), r.randrange(60)))
                sms = r.random() < 0.35
                raw.append((when, "sms" if sms else "call", p.pid, target, 0 if sms else min(1800, max(3, int(r.lognormvariate(4.2, 0.9)))), None, None))
            if p.pid in p8_subjects and r.random() < 0.9:   # a night owl (plant P8): about one call a night, so the night anchor of every week is well supported
                adj = sorted(neighbours[p.pid].items())
                target = r.choices([k for k, _ in adj], weights=[w for _, w in adj])[0]
                night = datetime.datetime.combine(date, datetime.time(r.choice([22, 23, 0, 1, 2, 3, 4, 5, 6]), r.randrange(60), r.randrange(60)))
                raw.append((night, "call", p.pid, target, min(900, max(3, int(r.lognormvariate(3.5, 0.9)))), None, None))
            if r.random() < 0.15:  # an automated message from a service
                label = r.choice(SERVICES)[1]
                when = datetime.datetime.combine(date, datetime.time(_hour(r), r.randrange(60), r.randrange(60)))
                raw.append((when, "sms", label, p.pid, 0, None, None))

    # --- plant P2: shared contacts for pairs of subjects, with a service decoy and a hub decoy ----------------------------------------
    subjects = [p for p in persons if p.role == "subject"]
    pairs = [(a.pid, b.pid) for i, a in enumerate(subjects) for b in subjects[i + 1:] if a.pid not in p6_subjects and b.pid not in p6_subjects]
    rng.shuffle(pairs)
    operator_of = {p.pid: p.operator for p in persons}
    # Shared contacts are persons no subject is linked to in the background graph, so the planted events are the only way a subject reaches them
    # (otherwise a third subject could make a planted contact look like a hub).
    subject_ids = {s.pid for s in subjects}
    pool = [p.pid for p in persons if p.role != "subject" and p.pid != hub and not (set(neighbours[p.pid]) & subject_ids)]
    rng.shuffle(pool)
    threshold = params.resolved_hub_threshold()
    # Subjects that do NOT contact the hub: those on the layout that shows no service traffic first, so more pairs see both decoys.
    leave_out = sorted(subjects, key=lambda s: (s.operator != 1, s.pid))[: max(0, len(subjects) - (threshold + 1))]
    skip_hub = set(s.pid for s in leave_out)
    hub_contacts = [s.pid for s in subjects if s.pid not in skip_hub]
    period_end = params.period_days

    def planted(plant_id, who, other, count, kind=None):
        r = random.Random("plant:%d:%s:%s:%s" % (params.seed, plant_id, who, other))
        first_day, last_day = world.active.get(who, (0, period_end - 1))
        if first_day > last_day:
            return
        for _ in range(count):
            date = start + datetime.timedelta(days=r.randint(first_day, last_day))
            when = datetime.datetime.combine(date, datetime.time(r.randrange(8, 21), r.randrange(60), r.randrange(60)))
            sms = kind == "sms"  # shared contacts are reached by calls: every layout can express a call, not every layout an SMS
            out = r.random() < 0.5
            a, b = (who, other) if out else (other, who)
            raw.append((when, "sms" if sms else "call", a, b, 0 if sms else r.randrange(20, 400), plant_id, None))

    def both_decoys_visible(pair):
        return all(operator_of[s] != 1 and s not in skip_hub for s in pair)  # operator 1 is the layout without service traffic
    good = [pr for pr in pairs if both_decoys_visible(pr)]
    others = [pr for pr in pairs if pr not in good[:2]]
    pairs = good[:2] + others
    cursor = 0
    for number, (x, y) in enumerate(pairs[: params.p2_pairs], start=1):
        plant_id = "P2-%03d" % number
        size = rng.randint(params.shared_min, params.shared_max)
        shared = [pool[(cursor + k) % len(pool)] for k in range(size)]
        cursor += size
        counts = {}
        for s in shared:
            counts[s] = {x: rng.randint(3, 8), y: rng.randint(3, 8)}
            planted(plant_id, x, s, counts[s][x])
            planted(plant_id, y, s, counts[s][y])
        service_counts = {x: rng.randint(3, 6), y: rng.randint(3, 6)}
        for who in (x, y):
            planted(plant_id, who, DECOY_SERVICE, service_counts[who], kind="sms")
        world.p2.append({"id": plant_id, "x": x, "y": y, "shared": shared, "counts": counts,
                         "decoys": [{"kind": "service_in_common", "number": DECOY_SERVICE, "counts": service_counts},
                                    {"kind": "hub_in_common", "person": hub, "contacted_by": hub_contacts}]})
    for number, who in enumerate(hub_contacts, start=1):
        planted("P2-hub", who, hub, 4)

    # --- plant P3: co-location -------------------------------------------------------------------------------------------------------
    # Two subjects on the SAME operator (cell identities are per operator) are at one cell within W minutes of each other, on several days.
    # The meeting cell is one neither of them uses in the background, so the planted windows are the only way they meet there. Each plant
    # has three decoys: the same cell far apart in time, the same cell just outside the window, and the same moment at different cells.
    # Their counterparties are persons that no subject is linked to, one per subject and plant, so no contact is shared by accident.
    r3 = random.Random("p3:%d" % params.seed)
    by_pid = {p.pid: p for p in persons}
    W = params.window_minutes
    same_network = [tuple(pair) for pair in world.roles["p3_pairs"]]
    p3_cursor = cursor

    def background_cells(person):
        return {person.home_cell, person.work_cell, (person.home_cell + 1) % CELLS_PER_OPERATOR, (person.work_cell + 1) % CELLS_PER_OPERATOR}

    def local(day, hour, minute):
        return datetime.datetime.combine(start + datetime.timedelta(days=day), datetime.time(hour, minute, 0))

    def plant_call(tag, when, who, party, site):
        out = r3.random() < 0.5
        a, b = (who, party) if out else (party, who)
        raw.append((when, "call", a, b, r3.randrange(20, 300), tag, {who: site}))

    for number, (x, y) in enumerate(same_network, start=1):
        px, py = by_pid[x], by_pid[y]
        free = [c for c in range(CELLS_PER_OPERATOR) if c not in background_cells(px) | background_cells(py)]
        if len(free) < 2 or p3_cursor + 2 > len(pool) or period_end < 4:
            continue
        meet, elsewhere = r3.sample(free, 2)
        cx, cy = pool[p3_cursor], pool[p3_cursor + 1]
        p3_cursor += 2
        plant_id = "P3-%03d" % number
        windows = min(params.windows_per_plant, max(1, period_end - 3))
        days = r3.sample(range(period_end), min(period_end, windows + 4))
        # plant P4: the pair's contacts are planted ones only, placed at cells that differ (so a contact is never itself a co-location)
        cell_x, cell_y = px.home_cell, py.home_cell
        if cell_x == cell_y:
            cell_y = py.work_cell
        if cell_x == cell_y:
            cell_y = (cell_y + 1) % CELLS_PER_OPERATOR

        def contact(tag, when, x=x, y=y, cell_x=cell_x, cell_y=cell_y):
            out = r3.random() < 0.5
            a, b = (x, y) if out else (y, x)
            raw.append((when, "call", a, b, r3.randrange(20, 300), tag, {x: cell_x, y: cell_y}))
        for k, day in enumerate(days[:windows], start=1):
            t0 = local(day, r3.randrange(9, 19), r3.randrange(0, 40))
            if k <= params.meeting_windows:
                contact("%s:m%d" % (plant_id, k), t0 - datetime.timedelta(minutes=r3.randint(10, 40)))
            elif k == params.meeting_windows + 1:
                contact("%s:early" % plant_id, t0 - datetime.timedelta(minutes=params.contact_gap_minutes + 120))
            for i in range(r3.randint(1, 3)):
                plant_call("%s:w%d" % (plant_id, k), t0 + datetime.timedelta(minutes=6 * i), x, cx, meet)
                plant_call("%s:w%d" % (plant_id, k), t0 + datetime.timedelta(minutes=6 * i + 3), y, cy, meet)
        spare = days[windows:]
        if len(spare) > 0:
            t = local(spare[0], r3.randrange(8, 14), r3.randrange(60))
            plant_call("%s:d:far_apart" % plant_id, t, x, cx, meet)
            plant_call("%s:d:far_apart" % plant_id, t + datetime.timedelta(hours=3), y, cy, meet)
        if len(spare) > 1:
            t = local(spare[1], r3.randrange(8, 20), r3.randrange(60))
            plant_call("%s:d:just_outside" % plant_id, t, x, cx, meet)
            plant_call("%s:d:just_outside" % plant_id, t + datetime.timedelta(minutes=W + 1), y, cy, meet)
        if len(spare) > 2:
            t = local(spare[2], r3.randrange(8, 20), r3.randrange(60))
            plant_call("%s:d:other_cell" % plant_id, t, x, cx, meet)
            plant_call("%s:d:other_cell" % plant_id, t + datetime.timedelta(minutes=3), y, cy, elsewhere)
        if len(spare) > 3:
            contact("%s:d:contact_alone" % plant_id, local(spare[3], r3.randrange(10, 20), r3.randrange(60)))
        world.p3.append({"id": plant_id, "x": x, "y": y, "operator": px.operator, "meeting_site": meet, "other_site": elsewhere,
                         "counterparties": {x: cx, y: cy}, "window_minutes": W, "meeting_windows": params.meeting_windows,
                         "contact_gap_minutes": params.contact_gap_minutes})
    # the pairs of plant P3 have no background contact with each other: their only direct contacts are the planted ones (P4)
    linked = {frozenset((p["x"], p["y"])) for p in world.p3}
    before = len(raw)
    raw[:] = [e for e in raw if e[5] is not None or frozenset((e[2], e[3])) not in linked]
    world.removed_contacts = before - len(raw)

    # --- plant P8 (events): impossible speed and its slow decoy ------------------------------------------------------------------------
    # Two pairs of calls 90 seconds apart at the two corner sites of the grid (about 8 km: over 300 km/h) and, as the decoy, the same two
    # sites three hours apart (under 3 km/h). The subject's background events within 30 minutes of a planted call are removed, so the
    # neighbours of a planted call are far enough away never to make a second alert; the counterparty is a person no subject is linked to.
    if p8_subjects and p3_cursor < len(pool):
        a = by_pid[p8_subjects[0]]
        party = pool[p3_cursor]
        p3_cursor += 1
        corner_a, corner_b = 0, CELLS_PER_OPERATOR - 1
        days = r8.sample(range(int(horizon * 0.12), int(horizon * 0.45)), 3)
        planted_times = []
        for k, day in enumerate(sorted(days), start=1):
            kind = "impossible" if k <= 2 else "slow"
            t0 = local(day, r8.randrange(10, 15), r8.randrange(0, 50))
            gap = datetime.timedelta(seconds=90) if kind == "impossible" else datetime.timedelta(hours=3)
            tag = "P8-%03d:%d" % (k + 1, 1) if kind == "impossible" else "P8-004:d"
            for when, site in ((t0, corner_a), (t0 + gap, corner_b)):
                raw.append((when, "call", a.pid, party, r8.randrange(10, 40), tag, {a.pid: site}))
                planted_times.append(when)
            world.p8.append({"id": "P8-%03d" % (k + 1) if kind == "impossible" else "P8-004", "kind": "impossible_speed" if kind == "impossible" else "slow_decoy",
                             "pid": a.pid, "day": day, "tag": tag, "from_site": corner_a, "to_site": corner_b, "gap_s": int(gap.total_seconds())})
        near = datetime.timedelta(minutes=30)
        raw[:] = [e for e in raw if e[5] is not None or a.pid not in (e[2], e[3]) or all(abs(e[0] - t) > near for t in planted_times)]

    if world.active:
        def in_use(pid, when):
            window = world.active.get(pid)
            return window is None or window[0] <= (when.date() - start).days <= window[1]
        raw[:] = [e for e in raw if in_use(e[2], e[0]) and in_use(e[3], e[0])]
    plant_p5(world, start)
    raw.sort(key=lambda e: (e[0], e[2], e[3], e[1]))
    for i, (when, kind, a, b, duration, plant, cells) in enumerate(raw, start=1):
        world.events.append(Event("E%07d" % i, kind, a, b, when, duration, plant, cells))
    build_sessions(world)
    return world


IDENTIFIER_OPERATORS = (0, 2, 3)   # operators whose call-record layout (C1, C3, C4) shows both the IMSI and the IMEI; C2 shows the IMEI only


def plant_p5(world, start_day):
    """Plant P5, device and SIM changes. On a known local date a subject's handset, SIM or both change (the number stays); and two
    subjects on one operator share a handset for the whole period (a dual-SIM phone: the decoy, since the IMEI sits under two numbers at
    once and nothing was swapped). Only subjects whose layout shows the identifiers are used, and never a subject that plays a part in P6."""
    params = world.params
    if not params.p5:
        return
    r5 = random.Random("p5:%d" % params.seed)
    roles = world.roles
    busy = {pid for pair in roles["p6_pairs"] for pid in pair} | {roles["p6_silent"], roles["p6_late"]}
    subjects = [p for p in world.persons if p.role == "subject" and p.pid not in busy]
    # the dual-SIM decoy first, on an operator whose layout hides the IMSI if there is one (the IMSI-showing subjects are kept for the changes)
    by_operator = {}
    for p in subjects:
        by_operator.setdefault(p.operator, []).append(p)
    dual = None
    for op in sorted(by_operator, key=lambda o: (o in IDENTIFIER_OPERATORS, o)):
        if len(by_operator[op]) >= 2:
            dual = by_operator[op][:2]
            break
    taken = {p.pid for p in dual} if dual else set()
    eligible = [p for p in subjects if p.operator in IDENTIFIER_OPERATORS and p.pid not in taken]
    r5.shuffle(eligible)
    counter = [0]

    def fresh():
        counter[0] += 1
        return 20000 + counter[0]
    kinds = ["handset_change", "sim_change", "both_change"]
    for number, (kind, person) in enumerate(zip(kinds, eligible), start=1):
        day = r5.randrange(max(1, params.period_days * 3 // 10), max(2, params.period_days * 7 // 10))
        stamp = datetime.datetime.combine(start_day + datetime.timedelta(days=day), datetime.time(0, 0)).isoformat(sep=" ")
        old_imsi, old_imei_n = identity_at(person, datetime.datetime.fromisoformat(stamp))
        new_imsi = reserved.synthetic_imsi(fresh()) if kind in ("sim_change", "both_change") else old_imsi
        new_imei_n = fresh() if kind in ("handset_change", "both_change") else old_imei_n
        person.segments.append({"from": stamp, "imsi": new_imsi, "imei_n": new_imei_n})
        world.p5.append({"id": "P5-%03d" % number, "kind": kind, "pid": person.pid, "from": stamp, "old_imsi": old_imsi, "new_imsi": new_imsi,
                         "old_imei_n": old_imei_n, "new_imei_n": new_imei_n})
    if dual:
        first, second = dual
        second.segments.append({"from": "0000-01-01 00:00:00", "imsi": second.imsi, "imei_n": first.n})
        world.p5.append({"id": "P5-%03d" % (len(world.p5) + 1), "kind": "dual_sim", "pids": [first.pid, second.pid], "imei_n": first.n})


def allocate_port_block(allocations, public_ip, start, end, size, rng):
    """A port block on a public address that no other session holds during [start, end]: the background never collides."""
    taken = allocations.setdefault(public_ip, [])
    for _ in range(400):
        low = rng.randrange(1024, 65536 - size)
        high = low + size - 1
        if all(not (a_start <= end and start <= a_end and low <= a_high and a_low <= high) for a_start, a_end, a_low, a_high in taken):
            taken.append((start, end, low, high))
            return low, high
    raise RuntimeError("no free port block on %s" % public_ip)


def build_sessions(world):
    """Data sessions of the subjects: a private address, a public address and port block that no one else holds at that time, a server,
    volumes and an application. Other subscriptions have sessions only where a plant needs them (P7)."""
    params = world.params
    start_day = datetime.date.fromisoformat(params.start_date)
    allocations = world.allocations = {}
    counter = 0
    for p in [x for x in world.persons if x.role == "subject"]:
        block = PORT_BLOCK.get(p.operator, 1)
        first_day, last_day = world.active.get(p.pid, (0, params.period_days - 1))
        for day in range(params.period_days):
            if not (first_day <= day <= last_day):
                continue
            r = random.Random("sess:%d:%s:%d" % (params.seed, p.pid, day))
            weekend = (start_day + datetime.timedelta(days=day)).weekday() >= 5
            for _ in range(_poisson(r, params.sessions_per_subject_per_day * (0.7 if weekend else 1.0))):
                when = datetime.datetime.combine(start_day + datetime.timedelta(days=day), datetime.time(_hour(r), r.randrange(60), r.randrange(60)))
                length = min(2400, max(5, int(r.lognormvariate(4.5, 1.0))))
                end = when + datetime.timedelta(seconds=length)
                public_ip = PUBLIC_POOL[p.operator][r.randrange(len(PUBLIC_POOL[p.operator]))]
                low, high = allocate_port_block(allocations, public_ip, when, end, block, r)
                counter += 1
                world.sessions.append(Session(
                    "S%07d" % counter, p.pid, when, end, when, end, "10.%d.%d.%d" % (p.n % 200, r.randrange(256), r.randrange(1, 255)), r.randrange(1024, 65536),
                    public_ip, low, high if block > 1 else low, r.choice(SERVER_IPS), r.choice([53, 80, 443, 8080]), r.choice([6, 6, 6, 17]),
                    r.randrange(0, 99999), r.randrange(0, 999999), r.randrange(1, 99), r.randrange(1, 99), r.choice(APPS), r.randrange(CELLS_PER_OPERATOR), None))
    plant_p7(world, start_day)
    world.sessions.sort(key=lambda s: (s.start, s.pid))
    for i, s in enumerate(world.sessions, start=1):
        s.uid = "S%07d" % i


P7_OPERATORS = (1, 2, 3)   # zong, mobilink, telenor: their layouts write when a session ends (ufone's I3 writes only a log time)


def plant_p7(world, start_day):
    """Plant P7, address-translation collisions. On a public address of its own, three subscriptions of one operator (bystanders) hold ports:
    two overlap in time and port (a collision: ambiguous), one shares the time but not the port, one repeats the port later, two use one
    port back to back, one second apart. Each probe (address, port, instant) states the intended answer; the oracle must agree."""
    params = world.params
    r7 = random.Random("p7:%d" % params.seed)
    used = set()
    for number, op in enumerate([o for o in P7_OPERATORS][: params.p7_plants], start=1):
        people = [p for p in world.persons if p.role == "bystander" and p.operator == op]
        if len(people) < 3 or params.period_days < 2:
            continue
        s1, s2, s3 = r7.sample(people, 3)
        plant_id = "P7-%03d" % number
        ip = reserved.synthetic_ipv4(40 + op)   # a documentation address no other session uses
        day = r7.randrange(params.period_days)
        base = datetime.datetime.combine(start_day + datetime.timedelta(days=day), datetime.time(r7.randrange(8, 14), r7.randrange(60), 0))
        block = PORT_BLOCK.get(op, 1)
        port_a = r7.randrange(2000, 30000)
        port_b, port_c = port_a + 1000, port_a + 2000
        minute = datetime.timedelta(minutes=1)

        def hold(tag, person, begin, finish, port):
            world.sessions.append(Session(
                "", person.pid, begin, finish, begin, finish, "10.%d.%d.%d" % (person.n % 200, r7.randrange(256), r7.randrange(1, 255)), r7.randrange(1024, 65536), ip,
                port, port + block - 1, r7.choice(SERVER_IPS), r7.choice([80, 443]), 6, r7.randrange(0, 99999), r7.randrange(0, 999999), r7.randrange(1, 99),
                r7.randrange(1, 99), r7.choice(APPS), r7.randrange(CELLS_PER_OPERATOR), "%s:%s" % (plant_id, tag)))
        hold("collision-a", s1, base, base + 20 * minute, port_a)
        hold("collision-b", s2, base + 10 * minute, base + 30 * minute, port_a)
        hold("same-time-other-port", s3, base, base + 20 * minute, port_b)
        hold("same-port-later", s2, base + 120 * minute, base + 140 * minute, port_a)
        hold("back-to-back-a", s1, base + 180 * minute, base + 190 * minute, port_c)
        hold("back-to-back-b", s3, base + 190 * minute + datetime.timedelta(seconds=1), base + 200 * minute, port_c)
        iso = lambda t: t.isoformat(sep=" ")
        probes = [
            ("collision", port_a, base + 15 * minute, "ambiguous", [s1, s2]),
            ("before the second holder starts", port_a, base + 5 * minute, "unique", [s1]),
            ("same time, other port", port_b, base + 15 * minute, "unique", [s3]),
            ("same port, later", port_a, base + 125 * minute, "unique", [s2]),
            ("back to back: last second of the first", port_c, base + 190 * minute, "unique", [s1]),
            ("back to back: first second of the second", port_c, base + 190 * minute + datetime.timedelta(seconds=1), "unique", [s3]),
            ("nobody", port_a + 7, base + 15 * minute, "none", []),
        ]
        world.p7.append({"id": plant_id, "operator": op, "public_ip": ip, "block": block, "day": day,
                         "window_start": iso(datetime.datetime.combine(start_day + datetime.timedelta(days=day), datetime.time(0, 0))),
                         "window_end": iso(datetime.datetime.combine(start_day + datetime.timedelta(days=day + 1), datetime.time(0, 0))),
                         "subscriptions": [s1.pid, s2.pid, s3.pid],
                         "probes": [{"label": l, "port": port, "instant": iso(t), "intent": intent, "intended_holders": [p.pid for p in who]} for l, port, t, intent, who in probes]})


def cell_for(world: World, person: Person, event: Event):
    """The cell site a person is on at an event: home at night, work in office hours on weekdays, otherwise around them."""
    if event.cells and person.pid in event.cells:
        return event.cells[person.pid]
    hour, weekday = event.start.hour, event.start.weekday()
    home = home_at(world, person, event.start.date())
    if hour >= 22 or hour < 7:
        idx = home
    elif 9 <= hour < 17 and weekday < 5:
        idx = person.work_cell
    else:
        # one choice per three-hour block, so two calls minutes apart are at the same site unless a block boundary falls between them
        r = random.Random("cell:%d:%s:%s:%d" % (world.params.seed, person.pid, event.start.date().isoformat(), hour // 3))
        idx = r.choice([home, person.work_cell, (home + 1) % CELLS_PER_OPERATOR, (person.work_cell + 1) % CELLS_PER_OPERATOR])
    return idx


def home_at(world: World, person: Person, date: datetime.date):
    """The cell a person sleeps at on a date: the home cell, unless a move (for good, or a trip) is in force (plant P8)."""
    day = (date - datetime.date.fromisoformat(world.params.start_date)).days
    cell = person.home_cell
    for move in world.moves.get(person.pid, []):
        if move["from_day"] <= day and (move["to_day"] is None or day <= move["to_day"]):
            cell = move["cell"]
    return cell


def to_utc(local: datetime.datetime) -> datetime.datetime:
    return local - datetime.timedelta(hours=LOCAL_UTC_OFFSET_HOURS)


def world_summary(world: World) -> dict:
    return {"params": asdict(world.params), "persons": len(world.persons), "events": len(world.events),
            "planted_events": sum(1 for e in world.events if e.plant)}


def world_to_json(world: World) -> str:
    doc = {
        "generator_version": GENERATOR_VERSION, "spec_version": SPEC_VERSION, "params": asdict(world.params), "clock": {"local_zone": LOCAL_ZONE, "utc_offset_hours": LOCAL_UTC_OFFSET_HOURS},
        "persons": [asdict(p) for p in world.persons],
        "services": [{"kind": k, "label": v} for k, v in SERVICES],
        "structure": world.structure, "p2": world.p2, "p3": world.p3, "p7": world.p7, "p5": world.p5, "p6": world.p6, "p8": world.p8, "moves": world.moves, "rates": world.rates, "active": world.active, "roles": world.roles,
        "sessions": [[s.uid, s.pid, s.start.isoformat(sep=" "), s.end.isoformat(sep=" "), s.nat_start.isoformat(sep=" "), s.nat_end.isoformat(sep=" "), s.ue_ip, s.ue_port, s.public_ip, s.port_from, s.port_to, s.server_ip, s.server_port, s.protocol_num, s.bytes_up, s.bytes_down, s.packets_up, s.packets_down, list(s.app), s.site, s.plant] for s in world.sessions],
        "events": [[e.uid, e.kind, e.orig, e.recip, e.start.isoformat(sep=" "), e.duration_s, e.plant, e.cells] for e in world.events],
    }
    return json.dumps(doc, sort_keys=True, indent=1, ensure_ascii=True)
