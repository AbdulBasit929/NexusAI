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

GENERATOR_VERSION = "0.1"
SPEC_VERSION = "0.1"
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
    subjects: int = 6
    associates: int = 24
    period_days: int = 90
    start_date: str = "2026-02-02"
    events_per_person_per_day: float = 4.0   # events a person originates per day; each also involves a second party
    communities: int = 6
    p2_pairs: int = 5
    p3_pairs: int = 5            # co-location plants; limited by the pairs of subjects on the same operator
    windows_per_plant: int = 4
    window_minutes: int = 15     # W of the co-location definition (specification section 10)
    dumps: bool = True           # one tower dump per operator that has a subject
    dump_days: int = 2
    sessions_per_subject_per_day: float = 6.0
    app_usage_exports: int = 2   # subjects that also get an app-usage export (layout A1)
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

    # --- background events ------------------------------------------------------------------------------------------------------------
    start = datetime.date.fromisoformat(params.start_date)
    raw = []
    for day in range(params.period_days):
        date = start + datetime.timedelta(days=day)
        weekend = date.weekday() >= 5
        for p in persons:
            r = random.Random("ev:%d:%s:%d" % (params.seed, p.pid, day))
            for _ in range(_poisson(r, params.events_per_person_per_day * (0.6 if weekend else 1.0))):
                adj = sorted(neighbours[p.pid].items())
                target = r.choices([k for k, _ in adj], weights=[w for _, w in adj])[0]
                when = datetime.datetime.combine(date, datetime.time(_hour(r), r.randrange(60), r.randrange(60)))
                sms = r.random() < 0.35
                raw.append((when, "sms" if sms else "call", p.pid, target, 0 if sms else min(1800, max(3, int(r.lognormvariate(4.2, 0.9)))), None, None))
            if r.random() < 0.15:  # an automated message from a service
                label = r.choice(SERVICES)[1]
                when = datetime.datetime.combine(date, datetime.time(_hour(r), r.randrange(60), r.randrange(60)))
                raw.append((when, "sms", label, p.pid, 0, None, None))

    # --- plant P2: shared contacts for pairs of subjects, with a service decoy and a hub decoy ----------------------------------------
    subjects = [p for p in persons if p.role == "subject"]
    pairs = [(a.pid, b.pid) for i, a in enumerate(subjects) for b in subjects[i + 1:]]
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
        for _ in range(count):
            date = start + datetime.timedelta(days=r.randrange(period_end))
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
    same_network = [(a.pid, b.pid) for i, a in enumerate(subjects) for b in subjects[i + 1:] if a.operator == b.operator]
    r3.shuffle(same_network)
    p3_cursor = cursor

    def background_cells(person):
        return {person.home_cell, person.work_cell, (person.home_cell + 1) % CELLS_PER_OPERATOR, (person.work_cell + 1) % CELLS_PER_OPERATOR}

    def local(day, hour, minute):
        return datetime.datetime.combine(start + datetime.timedelta(days=day), datetime.time(hour, minute, 0))

    def plant_call(tag, when, who, party, site):
        out = r3.random() < 0.5
        a, b = (who, party) if out else (party, who)
        raw.append((when, "call", a, b, r3.randrange(20, 300), tag, {who: site}))

    for number, (x, y) in enumerate(same_network[: params.p3_pairs], start=1):
        px, py = by_pid[x], by_pid[y]
        free = [c for c in range(CELLS_PER_OPERATOR) if c not in background_cells(px) | background_cells(py)]
        if len(free) < 2 or p3_cursor + 2 > len(pool) or period_end < 4:
            continue
        meet, elsewhere = r3.sample(free, 2)
        cx, cy = pool[p3_cursor], pool[p3_cursor + 1]
        p3_cursor += 2
        plant_id = "P3-%03d" % number
        windows = min(params.windows_per_plant, max(1, period_end - 3))
        days = r3.sample(range(period_end), min(period_end, windows + 3))
        for k, day in enumerate(days[:windows], start=1):
            t0 = local(day, r3.randrange(9, 19), r3.randrange(0, 40))
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
        world.p3.append({"id": plant_id, "x": x, "y": y, "operator": px.operator, "meeting_site": meet, "other_site": elsewhere,
                         "counterparties": {x: cx, y: cy}, "window_minutes": W})

    raw.sort(key=lambda e: (e[0], e[2], e[3], e[1]))
    for i, (when, kind, a, b, duration, plant, cells) in enumerate(raw, start=1):
        world.events.append(Event("E%07d" % i, kind, a, b, when, duration, plant, cells))
    build_sessions(world)
    return world


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
        for day in range(params.period_days):
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
    world.sessions.sort(key=lambda s: (s.start, s.pid))
    for i, s in enumerate(world.sessions, start=1):
        s.uid = "S%07d" % i


def cell_for(world: World, person: Person, event: Event):
    """The cell site a person is on at an event: home at night, work in office hours on weekdays, otherwise around them."""
    if event.cells and person.pid in event.cells:
        return event.cells[person.pid]
    r = random.Random("cell:%d:%s:%s" % (world.params.seed, person.pid, event.uid))
    hour, weekday = event.start.hour, event.start.weekday()
    if hour >= 22 or hour < 7:
        idx = person.home_cell
    elif 9 <= hour < 17 and weekday < 5:
        idx = person.work_cell
    else:
        idx = r.choice([person.home_cell, person.work_cell, (person.home_cell + 1) % CELLS_PER_OPERATOR, (person.work_cell + 1) % CELLS_PER_OPERATOR])
    return idx


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
        "structure": world.structure, "p2": world.p2, "p3": world.p3,
        "sessions": [[s.uid, s.pid, s.start.isoformat(sep=" "), s.end.isoformat(sep=" "), s.nat_start.isoformat(sep=" "), s.nat_end.isoformat(sep=" "), s.ue_ip, s.ue_port, s.public_ip, s.port_from, s.port_to, s.server_ip, s.server_port, s.protocol_num, s.bytes_up, s.bytes_down, s.packets_up, s.packets_down, list(s.app), s.site, s.plant] for s in world.sessions],
        "events": [[e.uid, e.kind, e.orig, e.recip, e.start.isoformat(sep=" "), e.duration_s, e.plant, e.cells] for e in world.events],
    }
    return json.dumps(doc, sort_keys=True, indent=1, ensure_ascii=True)
