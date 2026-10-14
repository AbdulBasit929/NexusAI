"""The interface view data (visual spike) for REAL call records, computed from the typed tables of a throwaway database.

Used only by tools/real_data/load_real_local.py, which writes the result outside every repository. The answers are computed by the same pure functions that score the
planted-truth cases (tools/case_generator/oracle.py), over the rows of the typed tables, so a real file gets the same views as a generated one. Numbers are shown in full: the
owner asked for real numbers in every view, for the team lead who supplied the files, on this machine (vault register, extension of 2026-10-14).

No value is printed by this module.
"""
import collections
import datetime
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
import oracle  # noqa: E402
import load_case  # noqa: E402

LAYOUT_OPERATOR = {"call-c1-company": "company", "call-c2-strt-tm": "strt-tm", "call-c3-ufone-style": "ufone", "call-c4-telenor-style": "telenor"}
CELL_UNCERTAINTY_M = 900   # an assumed radius drawn around a cell site, as in the generated case's views
MIN_SUBJECT_EVENTS = 20
MAX_SUBJECTS = 40
MAX_CONTACT_NODES = 600
ROWS_SQL = ("SELECT a_norm, b_norm, b_raw, b_kind, direction, event_type, duration_s, to_char(start_utc AT TIME ZONE 'Asia/Karachi', 'YYYY-MM-DD HH24:MI:SS') AS start_local, "
            "cell_key, lat, lon, imsi, imei, layout_id FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND NOT ('duplicate_row' = ANY (quality_flags)) "
            "AND a_norm IS NOT NULL ORDER BY a_norm, start_utc")
CELLS_SQL = ("SELECT cell_key, avg(lat) AS lat, avg(lon) AS lon FROM forensic.comm_event WHERE %s AND cell_key IS NOT NULL AND lat IS NOT NULL GROUP BY cell_key")
SESSIONS_SQL = ("SELECT msisdn, host(public_ip) AS public_ip, public_port_from, public_port_to, "
                "to_char(coalesce(nat_start_utc, start_utc) AT TIME ZONE 'Asia/Karachi', 'YYYY-MM-DD HH24:MI:SS') AS begin_local, "
                "to_char(coalesce(nat_end_utc, end_utc, start_utc) AT TIME ZONE 'Asia/Karachi', 'YYYY-MM-DD HH24:MI:SS') AS end_local "
                "FROM forensic.data_session WHERE %s AND public_ip IS NOT NULL AND public_port_from IS NOT NULL AND msisdn IS NOT NULL AND NOT ('duplicate_row' = ANY (quality_flags))")


def _scope(case_id):
    return "tenant_id = %s AND collection_id = %s" % (load_case.sql_text(load_case.TENANT), load_case.sql_text(case_id))


def build(db, case_id):
    scope = _scope(case_id)
    raw = db.rows(ROWS_SQL % scope)
    coords = {r["cell_key"]: (r["lat"], r["lon"]) for r in db.rows(CELLS_SQL % scope)}
    per = collections.defaultdict(list)
    for r in raw:
        r["b_raw_label"] = r["b_raw"] if r["b_kind"] not in oracle.PERSON_KINDS else None
        if r.get("lat") is None and r.get("cell_key") in coords:
            r["lat"], r["lon"] = coords[r["cell_key"]]
        per[r["a_norm"]].append(r)
    ranked = sorted(((n, rows) for n, rows in per.items() if len(rows) >= MIN_SUBJECT_EVENTS), key=lambda x: -len(x[1]))[:MAX_SUBJECTS]
    rows = {n: r for n, r in ranked}
    numbers = sorted(rows)
    operator_of = {n: LAYOUT_OPERATOR.get(collections.Counter(r["layout_id"] for r in rows[n]).most_common(1)[0][0], "other") for n in numbers}
    all_times = sorted(r["start_local"] for n in numbers for r in rows[n])
    start_dt = datetime.datetime.fromisoformat(all_times[0]).replace(hour=0, minute=0, second=0)
    end_dt = datetime.datetime.fromisoformat(all_times[-1])
    days = (end_dt.date() - start_dt.date()).days + 1

    # --- link chart --------------------------------------------------------------------------------------------------------------------------
    nodes, edges = {}, {}
    for n in numbers:
        times = [r["start_local"] for r in rows[n]]
        nodes[n] = {"id": n, "label": n, "kind": "subject", "operator": operator_of[n], "events": len(times), "first": min(times), "last": max(times)}
    contact_events = collections.Counter()
    contact_subjects = collections.defaultdict(set)
    for n in numbers:
        for r in rows[n]:
            other = r["b_norm"] if r["b_kind"] in oracle.PERSON_KINDS and r["b_norm"] else None
            if other and other != n:
                contact_events[other] += 1
                contact_subjects[other].add(n)
    keep = {c for c, _ in contact_events.most_common(MAX_CONTACT_NODES)} | {c for c, s in contact_subjects.items() if len(s) >= 2}
    for n in numbers:
        for r in rows[n]:
            other = r["b_norm"] if r["b_kind"] in oracle.PERSON_KINDS and r["b_norm"] else None
            if not other or other == n or (other not in nodes and other not in keep):
                continue
            nodes.setdefault(other, {"id": other, "label": other, "kind": "contact"})
            pair = tuple(sorted((n, other)))
            e = edges.setdefault(pair, {"source": pair[0], "target": pair[1], "kind": "observed", "count": 0, "first": r["start_local"], "last": r["start_local"], "rows": []})
            e["count"] += 1
            e["first"], e["last"] = min(e["first"], r["start_local"]), max(e["last"], r["start_local"])
            if len(e["rows"]) < 6:
                e["rows"].append({"time": r["start_local"], "direction": r["direction"], "type": r["event_type"], "duration_s": r["duration_s"], "owner": n})
    for node in nodes.values():
        if node["kind"] == "contact":
            node["degree"] = sum(1 for e in edges.values() if node["id"] in (e["source"], e["target"]))
    threshold = max(2, len(numbers) - 2)
    contacts = oracle.contacts_by_subject(rows)
    hubs = oracle.hub_numbers(contacts, threshold)
    for number in hubs:
        if number in nodes:
            nodes[number]["kind"] = "hub"
    a2 = []
    for i, x in enumerate(numbers):
        for y in numbers[i + 1:]:
            answer = oracle.a2_common(rows, x, y, threshold)
            if answer["shared"]:
                a2.append({"x": x, "y": y, **answer})
    a2.sort(key=lambda a: -len(a["shared"]))
    inferred = [{"source": a["x"], "target": a["y"], "kind": "inferred", "count": len(a["shared"]), "shared": [s["number"] for s in a["shared"]], "plant": None,
                 "rows": [{"time": s["x_first"], "direction": "shared", "type": "common contact", "duration_s": None, "owner": s["number"]} for s in a["shared"][:6]]} for a in a2[:60]]
    candidates = []
    try:
        a9 = oracle.a9_successors(rows, start_dt, end_dt)
        for silent in a9["silent"]:
            for c in silent["candidates"][:1]:
                candidates.append({"source": silent["number"], "target": c["number"], "kind": "candidate", "count": 1, "score": c["score"],
                                   "rows": [{"time": c["first_event"], "direction": "successor", "type": "score %.2f" % c["score"], "duration_s": None, "owner": silent["number"]}]})
    except Exception:  # a successor needs enough history; without it the views simply have no candidate edge
        candidates = []

    # --- map ---------------------------------------------------------------------------------------------------------------------------------
    cells = {}
    visits = collections.defaultdict(collections.Counter)
    for n in numbers:
        for r in rows[n]:
            if r.get("cell_key"):
                visits[n][r["cell_key"]] += 1
                if r.get("lat") is not None and r["cell_key"] not in cells:
                    cells[r["cell_key"]] = {"key": r["cell_key"], "lat": r["lat"], "lon": r["lon"], "operator": operator_of[n], "radius_m": CELL_UNCERTAINTY_M}
    colocation = []
    meetings_of = {}
    for i, x in enumerate(numbers):
        if not visits[x]:
            continue
        for y in numbers[i + 1:]:
            if not (set(visits[x]) & set(visits[y])):
                continue
            answer = oracle.a5_colocation(rows[x], rows[y])
            if not answer["pairs_observed"]:
                continue
            m = oracle.a7_meetings(rows[x], rows[y], x, y, answer["windows"], 60)
            colocation.append({"x": x, "y": y, "windows": answer["windows"], "pairs_observed": answer["pairs_observed"], "expected": answer["pairs_expected"],
                               "lift": answer["lift"]["value"], "lift_note": answer["lift"]["note"],
                               "meetings": [{k: c[k] for k in ("start_local", "end_local", "cell_key", "pairs", "contacts_before", "gap_minutes", "score")} for c in m["candidates"]]})
            meetings_of[(x, y)] = m
    colocation.sort(key=lambda c: (-len(c["meetings"]), -len(c["windows"]), c["x"]))
    movers = []
    a10 = oracle.a10_movement(rows, start_dt)
    for n, report in a10["numbers"].items():
        if report["changes"] or report["speed_alerts"] or report["short_deviations"]:
            pts = sorted((r["start_local"], r["lat"], r["lon"], r["cell_key"]) for r in rows[n] if r.get("lat") is not None)
            movers.append({"number": n, "label": n, "operator": operator_of[n], "points": [list(p) for p in pts], "changes": report["changes"],
                           "short_deviations": report["short_deviations"], "speed_alerts": report["speed_alerts"], "anchors": report["anchors"], "visits": dict(visits[n])})
    movers.sort(key=lambda m_: (-len(m_["changes"]) - len(m_["speed_alerts"]), m_["number"]))
    movers = movers[:12]
    if cells:
        origin = {"lat": sum(c["lat"] for c in cells.values()) / len(cells), "lon": sum(c["lon"] for c in cells.values()) / len(cells)}
    else:
        origin = {"lat": 30.0, "lon": 70.0}

    # --- swim lanes --------------------------------------------------------------------------------------------------------------------------
    a8 = oracle.a8_device_history(rows)
    lanes = []
    for n in numbers:
        runs = []
        for r in sorted((x for x in rows[n] if x.get("imsi") or x.get("imei")), key=lambda x: x["start_local"]):
            ident = (r.get("imsi"), oracle.imei14(r.get("imei")))
            if runs and runs[-1]["ident"] == ident:
                runs[-1]["last"] = r["start_local"]
            else:
                runs.append({"ident": ident, "first": r["start_local"], "last": r["start_local"]})
        lanes.append({"number": n, "label": n, "operator": operator_of[n], "runs": [{"imsi": run["ident"][0] or "", "imei": run["ident"][1] or "", "first": run["first"], "last": run["last"]} for run in runs],
                      "changes": [{"kind": c["kind"], "last_seen_old": c["last_seen_old"], "first_seen_new": c["first_seen_new"]} for c in a8["numbers"][n]["changes"]], "has_identifiers": bool(runs)})
    pairs = []
    for c in colocation[:6]:
        x, y = c["x"], c["y"]
        pairs.append({"x": x, "y": y, "labels": [x, y], "windows": c["windows"], "meetings": c["meetings"], "lift": c["lift"], "lift_note": c["lift_note"], "expected": c["expected"],
                      "observed": c["pairs_observed"], "events": {k: [[r["start_local"], r["cell_key"]] for r in rows[k] if r.get("cell_key")] for k in (x, y)},
                      "contacts": sorted({r["start_local"] for r in rows[x] if r["b_norm"] == y} | {r["start_local"] for r in rows[y] if r["b_norm"] == x})})

    # --- rhythm: the best split of the daily counts for the busiest numbers (thresholds lowered so that every one of them has a best split to show) ---
    rhythm = []
    busiest = [n for n in numbers if len({r["start_local"][:10] for r in rows[n]}) >= 10][:3]
    a11 = oracle.a11_rate_change({n: rows[n] for n in busiest}, t_min=0.0, ratio_min=1.0) if busiest else {"numbers": {}}
    for n, report in a11["numbers"].items():
        if not report.get("change"):
            continue
        counts, heat = collections.Counter(), collections.Counter()
        for r in rows[n]:
            when = datetime.datetime.fromisoformat(r["start_local"])
            counts[when.date()] += 1
            heat[((when.date() - start_dt.date()).days, when.hour)] += 1
        day_list = [start_dt.date() + datetime.timedelta(days=i) for i in range(days)]
        rhythm.append({"number": n, "label": n, "daily": [[d.isoformat(), counts.get(d, 0)] for d in day_list], "change": report["change"],
                       "heat": [[d, h, k] for (d, h), k in sorted(heat.items())]})

    # --- address attribution: observed internet sessions ------------------------------------------------------------------------------------------
    sessions = db.rows(SESSIONS_SQL % scope)
    seen = [{"public_ip": s["public_ip"], "port_from": s["public_port_from"], "port_to": s["public_port_to"], "begin": s["begin_local"], "end": s["end_local"], "msisdn": s["msisdn"]} for s in sessions]
    per_ip = collections.defaultdict(list)
    for s in seen:
        per_ip[s["public_ip"]].append(s)
    addresses = []
    for ip, items in sorted(per_ip.items(), key=lambda kv: -len({s["msisdn"] for s in kv[1]})):
        if len(addresses) >= 4:
            break
        probe = items[len(items) // 2]
        when = probe["begin"]
        answer = oracle.a13_holders(seen, ip, probe["port_from"], when)
        addresses.append({"public_ip": ip, "label": "observed in the internet records", "port": probe["port_from"], "instant": when, "result": answer["result"], "holders": answer["holders"]})

    # --- tower dumps (A6): per dump layout, the cells with the numbers they served and the numbers seen at two or more cells -----------------------------
    dumps = []
    place = "coalesce(cell_key, round(lat::numeric, 4)::text || ',' || round(lon::numeric, 4)::text)"   # a dump row without a cell key is placed by its coordinates
    cell_rows = db.rows("SELECT layout_id, %s AS cell_key, count(*) AS events, count(DISTINCT a_norm) AS numbers, avg(lat) AS lat, avg(lon) AS lon FROM forensic.comm_event WHERE %s "
                        "AND dataset_kind = 'tower_dump' AND (cell_key IS NOT NULL OR lat IS NOT NULL) GROUP BY layout_id, %s" % (place, scope, place))
    totals = {r["layout_id"]: r for r in db.rows("SELECT layout_id, count(*) AS events, count(DISTINCT a_norm) AS numbers FROM forensic.comm_event WHERE %s AND dataset_kind = 'tower_dump' GROUP BY layout_id" % scope)}
    multi = db.rows("SELECT layout_id, a_norm AS number, count(DISTINCT %s) AS cells, count(*) AS events FROM forensic.comm_event WHERE %s AND dataset_kind = 'tower_dump' "
                    "AND a_norm IS NOT NULL AND (cell_key IS NOT NULL OR lat IS NOT NULL) GROUP BY layout_id, a_norm HAVING count(DISTINCT %s) >= 2 "
                    "ORDER BY layout_id, count(DISTINCT %s) DESC, count(*) DESC" % (place, scope, place, place))
    for layout in sorted(totals):
        cells_of, cells_total = [], 0
        for r in sorted((r for r in cell_rows if r["layout_id"] == layout), key=lambda r: -r["events"]):
            cells_total += 1
            lat, lon = (r["lat"], r["lon"]) if r["lat"] is not None else coords.get(r["cell_key"], (None, None))
            if lat is not None:
                cells_of.append({"key": r["cell_key"], "lat": lat, "lon": lon, "events": r["events"], "numbers": r["numbers"]})
        mine = [m for m in multi if m["layout_id"] == layout]
        if cells_total:
            dumps.append({"name": "Tower dump, layout %s" % layout, "cells": cells_of[:400], "cells_total": cells_total, "events": totals[layout]["events"], "numbers": totals[layout]["numbers"],
                          "at_two_or_more_total": len(mine), "at_two_or_more": [{"number": m["number"], "cells": m["cells"], "events": m["events"]} for m in mine[:15]]})
    shared = [{"imei": d["imei"], "relation": d["relation"], "numbers": [{"label": n["number"], "first": n["first"], "last": n["last"]} for n in d["numbers"]]} for d in a8["shared_devices"]]
    return {"case": {"real": True, "start": start_dt.date().isoformat(), "days": days, "subjects": len(numbers), "origin": origin,
                     "note": "real call and internet records supplied for testing: shown to the team lead on this machine only"},
            "caveats": {"co_location": "proximity is not association", "successor": "behavioural similarity, not identity", "movement": "cell sites, not positions of a person"},
            "graph": {"nodes": list(nodes.values()), "edges": list(edges.values()) + inferred + candidates},
            "map": {"cells": list(cells.values()), "colocation": colocation, "movers": movers, "dumps": dumps},
            "lanes": {"devices": lanes, "shared_devices": shared},
            "pairs": pairs, "rhythm": rhythm, "addresses": addresses}
