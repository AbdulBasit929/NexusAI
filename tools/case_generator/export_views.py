#!/usr/bin/env python3
"""Export the data the interface views need from a generated case, as one JSON file (roadmap section 7.7, the visual spike).

    python -X utf8 tools/case_generator/export_views.py --seed 1 --out views.json [--days 90]

Everything in the file is an oracle answer or a row the oracles read, in the shape a view would get from the API: nothing here is
computed in the browser. The case is built in memory from its seed (same seed, same file). Numbers are shown by their last five digits only.
"""
import argparse
import collections
import datetime
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import generate  # noqa: E402
import oracle  # noqa: E402
import world as w  # noqa: E402

CELL_UNCERTAINTY_M = 900   # assumed radius drawn around a cell site; the specification gives a 2 km spacing, so the circles do not overlap


def short(number):
    return number[-5:] if number else None


def build_views(params):
    world, renders, key = generate.build_case(params)
    subjects = [p for p in world.persons if p.role == "subject"]
    by_number = {s.msisdn: s for s in subjects}
    rows = {s.msisdn: renders[s.pid].visible for s in subjects}
    start = datetime.date.fromisoformat(params.start_date)

    # --- link chart: observed edges from calls and SMS, inferred edges from common contacts (A2), candidate edges from successors (A9) -------
    edges, nodes = {}, {}
    for number in rows:    # every subject first, so that a subject another subject called is not taken for a contact
        times = sorted(r["start_local"] for r in rows[number])
        nodes[number] = {"id": number, "label": short(number), "kind": "subject", "operator": w.OPERATOR_LABELS[by_number[number].operator], "events": len(times),
                         "first": times[0] if times else None, "last": times[-1] if times else None}
    for number, subject_rows in rows.items():
        for r in subject_rows:
            other = r["b_norm"] if r["b_kind"] in oracle.PERSON_KINDS and r["b_norm"] else None
            if not other:
                continue
            nodes.setdefault(other, {"id": other, "label": short(other), "kind": "contact"})
            pair = tuple(sorted((number, other)))
            e = edges.setdefault(pair, {"source": pair[0], "target": pair[1], "kind": "observed", "count": 0, "first": r["start_local"], "last": r["start_local"], "rows": []})
            e["count"] += 1
            e["first"], e["last"] = min(e["first"], r["start_local"]), max(e["last"], r["start_local"])
            if len(e["rows"]) < 6:
                e["rows"].append({"time": r["start_local"], "direction": r["direction"], "type": r["event_type"], "duration_s": r["duration_s"], "owner": short(number)})
    for n in nodes.values():
        if n["kind"] == "contact":
            n["degree"] = sum(1 for e in edges.values() if n["id"] in (e["source"], e["target"]))
    threshold = key["oracle"]["parameters"]["hub_threshold"]
    contacts = oracle.contacts_by_subject(rows)
    hubs = oracle.hub_numbers(contacts, threshold)
    for number in hubs:
        if number in nodes:
            nodes[number]["kind"] = "hub"
    inferred = []
    for answer in key["oracle"]["A2"]:
        if answer["shared"]:
            inferred.append({"source": answer["x"], "target": answer["y"], "kind": "inferred", "count": len(answer["shared"]),
                             "shared": [s["number"] for s in answer["shared"]], "plant": answer.get("plant"),
                             "rows": [{"time": s["x_first"], "direction": "shared", "type": "common contact", "duration_s": None, "owner": short(s["number"])} for s in answer["shared"][:6]]})
    candidates = []
    for silent in key["oracle"]["A9"]["silent"]:
        for c in silent["candidates"][:1]:
            candidates.append({"source": silent["number"], "target": c["number"], "kind": "candidate", "count": 1, "score": c["score"],
                               "rows": [{"time": c["first_event"], "direction": "successor", "type": "score %.2f" % c["score"], "duration_s": None, "owner": short(silent["number"])}]})

    # --- map: cells, co-location windows (A5, A7), movement of the subjects with a change of anchor or a speed alert (A10) -----------------
    cells = {}
    for number, subject_rows in rows.items():
        for r in subject_rows:
            if r.get("cell_key") and r.get("lat") is not None and r["cell_key"] not in cells:
                cells[r["cell_key"]] = {"key": r["cell_key"], "lat": r["lat"], "lon": r["lon"], "operator": w.OPERATOR_LABELS[by_number[number].operator], "radius_m": CELL_UNCERTAINTY_M}
    visits = collections.defaultdict(collections.Counter)
    for number, subject_rows in rows.items():
        for r in subject_rows:
            if r.get("cell_key"):
                visits[number][r["cell_key"]] += 1
    colocation = []
    meetings = {(a["x"], a["y"]): a for a in key["oracle"]["A7"]}
    for answer in key["oracle"]["A5"]:
        if answer["pairs_observed"]:
            m = meetings.get((answer["x"], answer["y"]), {"candidates": []})
            colocation.append({"x": answer["x"], "y": answer["y"], "windows": answer["windows"], "pairs_observed": answer["pairs_observed"],
                               "expected": answer["pairs_expected"], "lift": answer["lift"]["value"], "lift_note": answer["lift"]["note"],
                               "meetings": [{"start_local": c["start_local"], "end_local": c["end_local"], "cell_key": c["cell_key"], "pairs": c["pairs"], "contacts_before": c["contacts_before"],
                                             "gap_minutes": c["gap_minutes"], "score": c["score"]} for c in m["candidates"]]})
    colocation.sort(key=lambda c: (-len(c["meetings"]), -len(c["windows"]), c["x"]))
    movers = []
    for number, report in key["oracle"]["A10"]["numbers"].items():
        if report["changes"] or report["speed_alerts"] or report["short_deviations"]:
            pts = sorted(((r["start_local"], r["lat"], r["lon"], r["cell_key"]) for r in rows[number] if r.get("lat") is not None))
            movers.append({"number": number, "label": short(number), "operator": w.OPERATOR_LABELS[by_number[number].operator], "points": [[t_, la, lo, ck] for t_, la, lo, ck in pts],
                           "changes": report["changes"], "short_deviations": report["short_deviations"], "speed_alerts": report["speed_alerts"], "anchors": report["anchors"],
                           "visits": dict(visits[number])})
    movers.sort(key=lambda m_: (-len(m_["changes"]) - len(m_["speed_alerts"]), m_["number"]))

    # --- swim lanes: identity runs per number (A8) and the two lanes of each co-located pair ---------------------------------------------------
    lanes = []
    for number, subject_rows in rows.items():
        runs = []
        for r in sorted((x for x in subject_rows if x.get("imsi") or x.get("imei")), key=lambda x: x["start_local"]):
            ident = (r.get("imsi"), oracle.imei14(r.get("imei")))
            if runs and runs[-1]["ident"] == ident:
                runs[-1]["last"] = r["start_local"]
            else:
                runs.append({"ident": ident, "first": r["start_local"], "last": r["start_local"]})
        changes = key["oracle"]["A8"]["numbers"][number]["changes"]
        lanes.append({"number": number, "label": short(number), "operator": w.OPERATOR_LABELS[by_number[number].operator],
                      "runs": [{"imsi": short(i[0]), "imei": (i[1] or "")[-6:], "first": r_["first"], "last": r_["last"]} for r_ in runs for i in [r_["ident"]]],
                      "changes": [{"kind": c["kind"], "last_seen_old": c["last_seen_old"], "first_seen_new": c["first_seen_new"]} for c in changes],
                      "has_identifiers": bool(runs)})
    pairs = []
    for c in colocation[:6]:
        x, y = c["x"], c["y"]
        pairs.append({"x": x, "y": y, "labels": [short(x), short(y)], "windows": c["windows"], "meetings": c["meetings"], "lift": c["lift"], "lift_note": c["lift_note"], "expected": c["expected"],
                      "observed": c["pairs_observed"],
                      "events": {k: [[r["start_local"], r["cell_key"]] for r in rows[k] if r.get("cell_key")] for k in (x, y)},
                      "contacts": sorted({r["start_local"] for r in rows[x] if r["b_norm"] == y} | {r["start_local"] for r in rows[y] if r["b_norm"] == x})})

    # --- series: daily counts and the change of rhythm (A11), hour-of-day by day, for every number the oracle flags -------------------------------
    rhythm = []
    for number, report in key["oracle"]["A11"]["numbers"].items():
        if not report["change"]:
            continue
        counts, heat = collections.Counter(), collections.Counter()
        for r in rows[number]:
            when = datetime.datetime.fromisoformat(r["start_local"])
            counts[when.date()] += 1
            heat[((when.date() - start).days, when.hour)] += 1
        days = [(start + datetime.timedelta(days=i)) for i in range(params.period_days)]
        rhythm.append({"number": number, "label": short(number), "daily": [[d.isoformat(), counts.get(d, 0)] for d in days], "change": report["change"],
                       "heat": [[d, h, n] for (d, h), n in sorted(heat.items())]})
    addresses = [{"public_ip": a["public_ip"], "label": a["label"], "port": a["port"], "instant": a["instant"], "result": a["result"], "holders": [short(h) for h in a["holders"]]} for a in key["oracle"]["A13"]]
    return {"case": {"seed": params.seed, "start": params.start_date, "days": params.period_days, "subjects": len(subjects),
                     "origin": {"lat": 0.43, "lon": -30.57}, "note": "synthetic case: fictional cell sites on a fictional grid"},
            "caveats": {"co_location": "proximity is not association", "successor": "behavioural similarity, not identity", "movement": "cell sites, not positions of a person"},
            "graph": {"nodes": list(nodes.values()), "edges": list(edges.values()) + inferred + candidates},
            "map": {"cells": list(cells.values()), "colocation": colocation, "movers": movers},
            "lanes": {"devices": lanes, "shared_devices": [{"imei": d["imei"][-6:], "relation": d["relation"], "numbers": [{"label": short(n["number"]), "first": n["first"], "last": n["last"]} for n in d["numbers"]]} for d in key["oracle"]["A8"]["shared_devices"]]},
            "pairs": pairs, "rhythm": rhythm, "addresses": addresses}


def main(argv=None):
    ap = argparse.ArgumentParser(description="Export the interface view data of a generated case.")
    ap.add_argument("--seed", type=int, required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--days", type=int, default=90)
    args = ap.parse_args(argv)
    data = build_views(w.Params(seed=args.seed, period_days=args.days))
    with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
        json.dump(data, handle, separators=(",", ":"), sort_keys=True)
    print("wrote %s (%.1f KB): %d nodes, %d edges, %d cells, %d movers, %d pairs, %d rhythm, %d addresses" % (args.out, os.path.getsize(args.out) / 1024, len(data["graph"]["nodes"]), len(data["graph"]["edges"]), len(data["map"]["cells"]), len(data["map"]["movers"]), len(data["pairs"]), len(data["rhythm"]), len(data["addresses"])))
    return 0


if __name__ == "__main__":
    sys.exit(main())
