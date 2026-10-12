"""Plain-Python reference answers (specification section 10): A1 contact profile and A2 common contacts.

They read the events as the files show them (the visible rows of each subject's export), not the product, SQL or the files themselves.
"""
from __future__ import annotations

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
            a = agg.setdefault(label, {"number": label, "kind": row["b_kind"], "outgoing": 0, "incoming": 0, "voice_seconds": 0, "first": row["start_local"], "last": row["start_local"]})
            a[row["direction"]] += 1
            a["voice_seconds"] += row["duration_s"] or 0
            a["first"] = min(a["first"], row["start_local"])
            a["last"] = max(a["last"], row["start_local"])
        out[subject] = sorted(agg.values(), key=lambda a: (-(a["outgoing"] + a["incoming"]), a["number"]))
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
