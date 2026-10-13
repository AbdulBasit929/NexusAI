"""Plain-Python reference answers (specification section 10): A1 contact profile, A2 common contacts and A5 co-location and A6 tower-dump population, A7 possible meetings, A8 device history, A9 successor candidates and A13 address attribution.

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


def imei14(value):
    """The first 14 digits of an IMEI: the same device whatever its layout writes (14, 15 or 16 digits)."""
    return value[:14] if value else None


def a8_device_history(visible_by_number):
    """Device and SIM history (specification section 10, A8). Per number: its (IMSI, device) timeline as runs of unchanged identity, and each
    change between two runs (`imei`: another handset, `imsi`: another SIM, `both`) with the last time the old identity and the first time the new
    one was seen. Across numbers: a device (14-digit IMEI) seen under two or more numbers, `concurrent` when their periods of use overlap (a
    dual-SIM phone, or one phone shared), `sequential` when one stops before the other starts (a handset passed on)."""
    numbers, by_device = {}, {}
    for number in sorted(visible_by_number):
        seen = sorted((r["start_local"], r.get("imsi"), imei14(r.get("imei"))) for r in visible_by_number[number] if r.get("imsi") or r.get("imei"))
        runs = []
        for when, imsi, imei in seen:
            if runs and (runs[-1]["imsi"], runs[-1]["imei"]) == (imsi, imei):
                runs[-1]["last"] = when
                runs[-1]["events"] += 1
            else:
                runs.append({"imsi": imsi, "imei": imei, "first": when, "last": when, "events": 1})
        changes = []
        for old, new in zip(runs, runs[1:]):
            kind = "both" if old["imsi"] != new["imsi"] and old["imei"] != new["imei"] else "imei" if old["imei"] != new["imei"] else "imsi"
            changes.append({"kind": kind, "last_seen_old": old["last"], "first_seen_new": new["first"], "old_imsi": old["imsi"], "new_imsi": new["imsi"],
                            "old_imei": old["imei"], "new_imei": new["imei"]})
        numbers[number] = {"runs": len(runs), "changes": changes}
        for run in runs:
            if run["imei"]:
                by_device.setdefault(run["imei"], {}).setdefault(number, []).append((run["first"], run["last"]))
    shared = []
    for device in sorted(by_device):
        per = by_device[device]
        if len(per) < 2:
            continue
        span = {n: (min(f for f, _ in v), max(l for _, l in v)) for n, v in per.items()}
        names = sorted(span)
        overlap = any(span[a][0] <= span[b][1] and span[b][0] <= span[a][1] for i, a in enumerate(names) for b in names[i + 1:])
        shared.append({"imei": device, "numbers": [{"number": n, "first": span[n][0], "last": span[n][1]} for n in names],
                       "relation": "concurrent" if overlap else "sequential"})
    return {"numbers": numbers, "shared_devices": shared}


A9_DEFAULTS = {"silence_days": 7, "window_days": 14, "overlap_days": 2, "history_days": 30, "new_after_days": 7, "threshold": 0.35}


def _cosine(a, b):
    dot = sum(n * b.get(k, 0) for k, n in a.items())
    na, nb = sum(n * n for n in a.values()) ** 0.5, sum(n * n for n in b.values()) ** 0.5
    return dot / (na * nb) if na and nb else 0.0


def a9_successors(visible_by_number, case_start, case_end, **overrides):
    """Successor candidates (specification section 10, A9): who may have replaced a number that went silent.

    A number is silent when its last event is at least `silence_days` before the end of the case. A candidate is another number whose first
    event falls between `overlap_days` before and `window_days` after the silent number's last event, and that was not already in use in the
    first `new_after_days` of the case. Features: the cosine of the two contact-count vectors (person-like counterparties, the silent number's last
    `history_days` against the candidate's first `history_days`, the two numbers left out), the Jaccard index of the cells each used at least twice
    in those windows, and whether they share a 14-digit IMEI at any time. score = 0.6 * cosine + 0.2 * jaccard + 0.2 * shared device; a
    candidate is listed when score >= threshold. Behavioural similarity, not identity.
    """
    cfg = dict(A9_DEFAULTS, **overrides)
    day = datetime.timedelta(days=1)
    times = {n: sorted(_time(r) for r in rows) for n, rows in visible_by_number.items() if rows}
    first = {n: t[0] for n, t in times.items()}
    last = {n: t[-1] for n, t in times.items()}
    silent = []
    for x in sorted(times):
        if case_end - last[x] < cfg["silence_days"] * day:
            continue
        x_from = last[x] - cfg["history_days"] * day
        candidates = []
        for y in sorted(times):
            if y == x or (first[y] - case_start) < cfg["new_after_days"] * day:
                continue
            if not (last[x] - cfg["overlap_days"] * day <= first[y] <= last[x] + cfg["window_days"] * day):
                continue
            y_to = first[y] + cfg["history_days"] * day
            cx, cy, cells_x, cells_y = {}, {}, {}, {}
            for r in visible_by_number[x]:
                when = _time(r)
                if x_from < when <= last[x]:
                    if r["b_kind"] in PERSON_KINDS and r["b_norm"] and r["b_norm"] not in (x, y):
                        cx[r["b_norm"]] = cx.get(r["b_norm"], 0) + 1
                    if r.get("cell_key"):
                        cells_x[r["cell_key"]] = cells_x.get(r["cell_key"], 0) + 1
            for r in visible_by_number[y]:
                when = _time(r)
                if first[y] <= when < y_to:
                    if r["b_kind"] in PERSON_KINDS and r["b_norm"] and r["b_norm"] not in (x, y):
                        cy[r["b_norm"]] = cy.get(r["b_norm"], 0) + 1
                    if r.get("cell_key"):
                        cells_y[r["cell_key"]] = cells_y.get(r["cell_key"], 0) + 1
            set_x, set_y = {c for c, n in cells_x.items() if n >= 2}, {c for c, n in cells_y.items() if n >= 2}
            jaccard = len(set_x & set_y) / len(set_x | set_y) if set_x | set_y else 0.0
            devices_x = {imei14(r.get("imei")) for r in visible_by_number[x]} - {None}
            devices_y = {imei14(r.get("imei")) for r in visible_by_number[y]} - {None}
            shared_device = bool(devices_x & devices_y)
            cosine = _cosine(cx, cy)
            score = round(0.6 * cosine + 0.2 * jaccard + (0.2 if shared_device else 0.0), 4)
            if score >= cfg["threshold"]:
                candidates.append({"number": y, "first_event": first[y].isoformat(sep=" "), "score": score, "contact_cosine": round(cosine, 4),
                                   "shared_contacts": len(set(cx) & set(cy)), "cell_jaccard": round(jaccard, 4), "shared_device": shared_device})
        candidates.sort(key=lambda c: (-c["score"], c["number"]))
        silent.append({"number": x, "last_event": last[x].isoformat(sep=" "), "candidates": candidates})
    return {"parameters": cfg, "silent": silent, "caveat": "behavioural similarity, not identity"}
