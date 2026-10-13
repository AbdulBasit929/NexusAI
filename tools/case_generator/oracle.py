"""Plain-Python reference answers (specification section 10): A1 contact profile, A2 common contacts and A5 co-location and A6 tower-dump population, A7 possible meetings and A13 address attribution.

They read the events as the files show them (the visible rows of each subject's export), not the product, SQL or the files themselves.
"""
from __future__ import annotations

import bisect
import collections
import datetime

PERSON_KINDS = ("person_like", "international")


def contacts_by_subject(visible):
    """visible: {subject msisdn: [row, ...]} -> {subject msisdn: {number: [rows]}} for person-like counterparties."""
    out = {}
    for subject, rows in visible.items():
        mine = {}
        for row in rows:
            if row["b_kind"] in PERSON_KINDS and row["b_norm"]:
                mine.setdefault(row["b_norm"], []).append(row)
        out[subject] = mine
    return out


def hub_numbers(contacts, threshold):
    """Numbers in contact with more than `threshold` distinct subjects (counted over the loaded exports)."""
    seen = {}
    for subject, mine in contacts.items():
        for number in mine:
            seen.setdefault(number, set()).add(subject)
    return {number: len(subjects) for number, subjects in seen.items() if len(subjects) > threshold}


def a1_profile(visible):
    """Who does each subject talk to most: per counterparty, counts by direction, voice seconds, first and last time."""
    out = {}
    for subject, rows in visible.items():
        agg = {}
        for row in rows:
            label = row["b_norm"] if row["b_norm"] else row["b_raw_label"]
            if label is None:
                continue   # no usable counterparty (a sentinel or the subscriber itself): the event cannot be attributed to a contact
            a = agg.setdefault(label, {"number": label, "kind": row["b_kind"], "outgoing": 0, "incoming": 0, "unknown": 0, "voice_seconds": 0,
                                       "first": row["start_local"], "last": row["start_local"]})
            a[row["direction"]] += 1
            a["voice_seconds"] += row["duration_s"] or 0
            a["first"] = min(a["first"], row["start_local"])
            a["last"] = max(a["last"], row["start_local"])
        out[subject] = sorted(agg.values(), key=lambda a: (-(a["outgoing"] + a["incoming"] + a["unknown"]), a["number"]))
    return out


def a2_common(visible, x, y, threshold):
    """Common contacts of two subjects: person-like counterparties of both, minus themselves, minus hubs; services reported as excluded."""
    contacts = contacts_by_subject(visible)
    hubs = hub_numbers(contacts, threshold)
    cx, cy = contacts[x], contacts[y]
    both = sorted((set(cx) & set(cy)) - {x, y})
    shared, excluded = [], []
    for number in both:
        if number in hubs:
            excluded.append({"number": number, "reason": "hub", "subjects_in_contact": hubs[number]})
            continue
        shared.append({"number": number, "x_count": len(cx[number]), "y_count": len(cy[number]),
                       "x_first": min(r["start_local"] for r in cx[number]), "x_last": max(r["start_local"] for r in cx[number]),
                       "y_first": min(r["start_local"] for r in cy[number]), "y_last": max(r["start_local"] for r in cy[number])})
    service_x = {r["b_raw_label"] for r in visible[x] if r["b_kind"] not in PERSON_KINDS and r["b_raw_label"]}
    service_y = {r["b_raw_label"] for r in visible[y] if r["b_kind"] not in PERSON_KINDS and r["b_raw_label"]}
    for label in sorted(service_x & service_y):
        excluded.append({"number": label, "reason": "service"})
    return {"x": x, "y": y, "shared": shared, "excluded": excluded}


def _time(row):
    return datetime.datetime.fromisoformat(row["start_local"])


def _bucket(when):
    """Time-of-week class used by the chance baseline: weekday or weekend, and the hour of the day."""
    return (when.weekday() >= 5, when.hour)


def _cell_shares(events):
    """({bucket: [events in the bucket, Counter of cells]}, overall Counter) from (time, cell) pairs."""
    by_bucket, overall = {}, collections.Counter()
    for when, cell in events:
        entry = by_bucket.setdefault(_bucket(when), [0, collections.Counter()])
        entry[0] += 1
        entry[1][cell] += 1
        overall[cell] += 1
    return by_bucket, overall


def _same_cell_probability(shares_x, shares_y, bucket):
    """Chance that two subjects who choose their cell independently, each as they usually do at that time of week, pick the same one."""
    def distribution(shares):
        by_bucket, overall = shares
        if bucket in by_bucket:
            total, counter = by_bucket[bucket]
        else:
            counter = overall
            total = sum(overall.values())
        return {cell: n / total for cell, n in counter.items()} if total else {}
    px, py = distribution(shares_x), distribution(shares_y)
    return sum(p * py.get(cell, 0.0) for cell, p in px.items())


def a5_colocation(rows_x, rows_y, window_minutes=15):
    """Co-location of two subjects (specification section 10).

    Two events, one of each subject, are co-located when they carry the same cell and their times differ by at most `window_minutes`.
    Co-located pairs on one cell are merged into windows (a pair joins the open window when it starts within `window_minutes` of the
    window's latest time). The chance baseline is the number of co-located pairs expected if the two subjects chose cells independently:
    over every pair of events within the window (same cell or not), the probability that both are at the same cell given the time of
    week (weekday or weekend, hour) of the earlier event, taken from each subject's own habits. lift = observed pairs / expected pairs,
    undefined when fewer than 0.5 pairs are expected. Proximity is not association.
    """
    width = datetime.timedelta(minutes=window_minutes)
    ex = sorted((_time(r), r["cell_key"]) for r in rows_x if r.get("cell_key"))
    ey = sorted((_time(r), r["cell_key"]) for r in rows_y if r.get("cell_key"))
    ytimes = [when for when, _ in ey]
    shares_x, shares_y = _cell_shares(ex), _cell_shares(ey)
    adjacent, expected, pairs = 0, 0.0, []
    for tx, cx in ex:
        for j in range(bisect.bisect_left(ytimes, tx - width), bisect.bisect_right(ytimes, tx + width)):
            ty, cy = ey[j]
            adjacent += 1
            early = min(tx, ty)
            expected += _same_cell_probability(shares_x, shares_y, _bucket(early))
            if cx == cy:
                pairs.append((cx, early, max(tx, ty)))
    windows = []
    for cell, early, late in sorted(pairs):
        last = windows[-1] if windows else None
        if last and last["cell_key"] == cell and early - datetime.datetime.fromisoformat(last["end_local"]) <= width:
            if late > datetime.datetime.fromisoformat(last["end_local"]):
                last["end_local"] = late.isoformat(sep=" ")
            last["pairs"] += 1
        else:
            windows.append({"cell_key": cell, "start_local": early.isoformat(sep=" "), "end_local": late.isoformat(sep=" "), "pairs": 1})
    windows.sort(key=lambda w: (w["start_local"], w["cell_key"]))
    lift = {"value": round(len(pairs) / expected, 4), "note": None} if expected >= 0.5 else {"value": None, "note": "too few events"}
    return {"window_minutes": window_minutes, "pairs_observed": len(pairs), "pairs_within_window": adjacent, "pairs_expected": round(expected, 4),
            "lift": lift, "windows": windows, "caveat": "proximity is not association"}


def a6_population(rows, cell_keys):
    """Who is in a tower dump (specification section 10, A6): per number the events and the cells they were seen at, and the numbers seen
    at two or more of the dump's cells. A layout without a cell column (C5) lists numbers only: the cells are the dump's request, not a column."""
    wanted = [k for k in cell_keys if k]
    per = {}
    for row in rows:
        entry = per.setdefault(row["a_norm"], {"events": 0, "cells": {}})
        entry["events"] += 1
        if row["cell_key"] and (not wanted or row["cell_key"] in wanted):
            entry["cells"][row["cell_key"]] = entry["cells"].get(row["cell_key"], 0) + 1
    population = [{"number": n, "events": per[n]["events"], "cells": dict(sorted(per[n]["cells"].items()))} for n in sorted(per)]
    seen_twice = sorted(n for n in per if len(per[n]["cells"]) >= 2) if wanted else None
    return {"cell_keys": cell_keys, "population": population, "at_two_or_more_cells": seen_twice,
            "note": None if wanted else "the layout has no cell column: the population is every number in the file"}


def a13_holders(sessions, public_ip, port, instant):
    """Who held this public address and port at this instant (specification section 10, A13).

    sessions: dicts with public_ip, port_from, port_to, begin, end (local 'YYYY-MM-DD HH:MM:SS') and msisdn. A session holds its address and
    port block from `begin` to `end`, both included. Distinct subscriptions are counted: one holder is `unique`, two or more `ambiguous`,
    none `none`. Ambiguity is stated, never resolved by guessing.
    """
    holders = sorted({s["msisdn"] for s in sessions
                      if s["public_ip"] == public_ip and s["port_from"] <= port <= s["port_to"] and s["begin"] <= instant <= s["end"]})
    return {"result": "none" if not holders else "unique" if len(holders) == 1 else "ambiguous", "holders": holders}


def a7_meetings(rows_x, rows_y, x, y, windows, gap_minutes=60):
    """Possible meetings of two subjects (specification section 10, A7): co-location windows (A5) preceded by a direct call or SMS between
    them within `gap_minutes` before the window starts (a contact at the start counts, one after it does not). Output per candidate: the
    window, the contacts in the gap, the gap to the latest of them and a documented score,
    score = 0.5 * min(pairs, 3) / 3 + 0.5 * (1 - gap / gap_minutes). A candidate, not a finding."""
    contact_times = sorted({r["start_local"] for r in rows_x if r["b_norm"] == y} | {r["start_local"] for r in rows_y if r["b_norm"] == x})
    width = datetime.timedelta(minutes=gap_minutes)
    candidates = []
    for window in windows:
        begin = datetime.datetime.fromisoformat(window["start_local"])
        before = [c for c in contact_times if begin - width <= datetime.datetime.fromisoformat(c) <= begin]
        if not before:
            continue
        gap = (begin - datetime.datetime.fromisoformat(max(before))).total_seconds() / 60
        candidates.append({"cell_key": window["cell_key"], "start_local": window["start_local"], "end_local": window["end_local"], "pairs": window["pairs"],
                           "contacts_before": before, "gap_minutes": round(gap, 2),
                           "score": round(0.5 * min(window["pairs"], 3) / 3 + 0.5 * (1 - gap / gap_minutes), 4)})
    return {"gap_minutes": gap_minutes, "candidates": candidates, "caveat": "a candidate, not a finding: proximity and contact are not proof of a meeting"}
