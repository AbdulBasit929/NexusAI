#!/usr/bin/env python3
"""Independent audit of a generated case: reads ONLY the rendered files and the key, never the generator.

    python -X utf8 tools/case_generator/audit.py CASE_DIR

It reads each export with the vault tools' file reader, selects the layout with the registry, interprets every row, recomputes the
common contacts itself, and checks that every plant is present in the data, every decoy is not a plant, the key's reference answers
match what the files show, and that every identifier is from a reserved range. Prints AUDIT=PASS or AUDIT=FAIL with the problems.
"""
import datetime
import hashlib
import json
import math
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, REPO)
sys.path.insert(0, os.path.join(REPO, "tools", "real_data"))
import profile_structure  # noqa: E402
import reserved  # noqa: E402
from ingestion.forensic_records.layout_registry import Policy, interpret_rows, load_registry  # noqa: E402

def is_test_device(value):
    """An IMEI as exported: 14 to 16 digits whose first eight (the type allocation code) start with the test prefix."""
    digits = "".join(ch for ch in str(value) if ch.isdigit())
    return 14 <= len(digits) <= 16 and digits.startswith(reserved.TEST_TAC_PREFIX)


POLICY = Policy(source_tz="Asia/Karachi", tz_basis="assumed_by_profile")


def read_export(path):
    """The rows of the first sheet of an export in any of the containers (xlsx, SpreadsheetML, CSV, UTF-16 text), read as the vault tools read them."""
    kind = profile_structure.sniff_kind(path)
    if kind == "xlsx":
        meta, sheets = profile_structure.iter_xlsx_sheets(path)
    elif kind == "spreadsheetml":
        meta, sheets = profile_structure.iter_spreadsheetml_sheets(path)
    else:
        meta, sheets = profile_structure.iter_text_rows(path)
    name, rows = sheets[0]
    return [list(r) for r in rows()]


def check_rows(label, results, expectations, problems):
    """Every row the key says was damaged must become what the key says; every other row must be accepted."""
    expected = {e["data_row"]: e for e in expectations}
    wrong = 0
    for n, r in enumerate(results):
        e = expected.get(n)
        if e is None:
            if r.status != "accepted":
                problems.append("%s: row %d is %s (%s) but nothing was damaged there" % (label, n, r.status, r.reason))
                wrong += 1
            continue
        if r.status != e["status"] or (e["reason"] and r.reason != e["reason"]):
            problems.append("%s: the %s row %d should be %s (%s), the registry says %s (%s)" % (label, e["kind"], n, e["status"], e["reason"], r.status, r.reason))
        elif e["status"] == "accepted":
            rec = r.record or {}
            if any(flag not in (r.flags or []) for flag in e["flags"]):
                problems.append("%s: the %s row %d lacks the flag %s" % (label, e["kind"], n, e["flags"]))
            if e["kind"] == "unknown_label" and rec.get("direction") != "unknown":
                problems.append("%s: an unmapped direction label was guessed (%s)" % (label, rec.get("direction")))
            if e["kind"] in ("party_sentinel", "self_counterparty") and rec.get("b_norm") is not None:
                problems.append("%s: a sentinel counterparty was kept" % label)
    for n in expected:
        if n >= len(results):
            problems.append("%s: the key damages row %d, the file has %d rows" % (label, n, len(results)))
            break


def record_signature(rec):
    """What the loader's content hash covers: everything about the row except where it sits in the file."""
    return tuple(sorted((k, str(v)) for k, v in rec.items() if k != "source_row"))


def audit(case_dir, registry=None):
    problems = []
    registry = registry or load_registry()
    with open(os.path.join(case_dir, "key", "key.json"), encoding="utf-8") as handle:
        key = json.load(handle)
    with open(os.path.join(case_dir, "manifest.json"), encoding="utf-8") as handle:
        manifest = json.load(handle)
    for rel, info in manifest["files"].items():
        full = os.path.join(case_dir, *rel.split("/"))
        if not os.path.isfile(full):
            problems.append("manifest lists %s, which is missing" % rel)
            continue
        with open(full, "rb") as handle:
            if hashlib.sha256(handle.read()).hexdigest() != info["sha256"]:
                problems.append("%s does not match its manifest hash" % rel)

    visible = {}   # subject msisdn -> list of (counterparty norm or label, kind, direction)
    placed = {}    # subject msisdn -> list of (start time, cell key)
    identity_rows = {}   # subject msisdn -> list of (start time, imsi, 14-digit imei)
    trail = {}   # subject msisdn -> list of (start time, cell key, lat, lon) in file order
    for subject in key["subjects"]:
        path = os.path.join(case_dir, *subject["file"].split("/"))
        rows = read_export(path)
        report = registry.find_header(rows)
        if report.matched_entry_id != subject["layout_id"]:
            problems.append("%s: the registry selects %s (%s), the key says %s" % (subject["file"], report.matched_entry_id, report.decision, subject["layout_id"]))
            continue
        entry = registry.get(subject["layout_id"])
        header = rows[report.header_row]
        results = list(interpret_rows(entry, header, rows[report.header_row + 1:], policy=POLICY))
        check_rows(subject["file"], results, key.get("defects", {}).get(subject["person"], []), problems)
        if len(results) != subject["rows"]:
            problems.append("%s: %d rows read, the key says %d" % (subject["file"], len(results), subject["rows"]))
        mine, where, repeated, identities, steps = [], [], set(), [], []
        for r in results:
            rec = r.record or {}
            if r.status != "accepted":
                continue
            if record_signature(rec) in repeated:
                continue   # an exact repeat is kept and flagged by the loader and counted once by the analyses
            repeated.add(record_signature(rec))
            if rec["a_norm"] != subject["msisdn"]:
                problems.append("%s: a row belongs to another subscriber" % subject["file"])
            for value, check in ((rec.get("a_norm"), reserved.is_reserved_msisdn), (rec.get("b_norm") if rec.get("b_kind") == "person_like" else None, reserved.is_reserved_msisdn),
                                 (rec.get("imsi"), reserved.is_test_imsi), (rec.get("imei"), is_test_device)):
                if value and not check(value):
                    problems.append("%s: a value outside the reserved ranges" % subject["file"])
                    break
            label = rec["b_norm"] if rec.get("b_norm") else rec.get("b_raw")
            mine.append((label, rec.get("b_kind"), rec["direction"], rec["start_local"]))
            identities.append((rec["start_local"], rec.get("imsi"), (rec.get("imei") or "")[:14] or None))
            if rec.get("cell_key"):
                where.append((datetime.datetime.fromisoformat(rec["start_local"]), rec["cell_key"]))
                steps.append((datetime.datetime.fromisoformat(rec["start_local"]), rec["cell_key"], rec.get("lat"), rec.get("lon")))
        visible[subject["msisdn"]] = mine
        placed[subject["msisdn"]] = where
        identity_rows[subject["msisdn"]] = identities
        trail[subject["msisdn"]] = steps

    threshold = key["oracle"]["parameters"]["hub_threshold"]

    def contacts(msisdn):
        out = {}
        for label, kind, direction, when in visible.get(msisdn, []):
            if kind in ("person_like", "international") and label:
                out.setdefault(label, []).append(when)
        return out
    all_contacts = {m: contacts(m) for m in visible}
    seen = {}
    for m, c in all_contacts.items():
        for number in c:
            seen.setdefault(number, set()).add(m)
    hubs = {n for n, subjects in seen.items() if len(subjects) > threshold}

    def common(x, y):
        cx, cy = all_contacts[x], all_contacts[y]
        return {n: (len(cx[n]), len(cy[n])) for n in (set(cx) & set(cy)) - {x, y} - hubs}

    for plant in key["plants"]:
        if plant["type"] != "common_contacts":
            continue
        x, y = plant["x"]["msisdn"], plant["y"]["msisdn"]
        if x not in visible or y not in visible:
            problems.append("%s: a member's file could not be read" % plant["id"])
            continue
        found = common(x, y)
        for shared in plant["planted_shared"]:
            n = shared["number"]
            if n not in found:
                problems.append("%s: planted shared contact %s is not common in the files" % (plant["id"], n[:6] + "..."))
            else:
                if found[n][0] < shared["x_count_planted"] or found[n][1] < shared["y_count_planted"]:
                    problems.append("%s: fewer events with a planted contact than planted" % plant["id"])
        for decoy in plant["decoys"]:
            if decoy["kind"] == "service_in_common":
                in_both = all(any(l == decoy["number"] for l, k, d, t in visible[m]) for m in (x, y))
                if in_both != decoy["visible_in_both_files"]:
                    problems.append("%s: the service decoy's visibility differs from the key" % plant["id"])
                if decoy["number"] in found:
                    problems.append("%s: the service decoy %s counts as a common contact" % (plant["id"], decoy["number"]))
            else:
                n = decoy["number"]
                if len(seen.get(n, ())) != decoy["subjects_in_contact"]:
                    problems.append("%s: the hub is in contact with %d subjects in the files, the key says %d" % (plant["id"], len(seen.get(n, ())), decoy["subjects_in_contact"]))
                if decoy["subjects_in_contact"] <= threshold:
                    problems.append("%s: the hub decoy is not above the threshold" % plant["id"])
                if n in found:
                    problems.append("%s: the hub decoy counts as a common contact" % plant["id"])

    for answer in key["oracle"]["A2"]:
        if answer["x"] not in visible or answer["y"] not in visible:
            continue  # a file that could not be read has been reported above
        found = common(answer["x"], answer["y"])
        expected = {s["number"]: (s["x_count"], s["y_count"]) for s in answer["shared"]}
        if found != expected:
            problems.append("A2 for %s and %s: the key and the files disagree" % (answer["x"][-5:], answer["y"][-5:]))

    # --- tower dumps (A6): read back, checked against their window and cells, population recomputed --------------------------------------
    answers = {a["dump"]: a for a in key["oracle"].get("A6", [])}
    for dump in key.get("dumps", []):
        path = os.path.join(case_dir, *dump["file"].split("/"))
        rows = read_export(path)
        report = registry.find_header(rows)
        if report.matched_entry_id != dump["layout_id"]:
            problems.append("%s: the registry selects %s, the key says %s" % (dump["file"], report.matched_entry_id, dump["layout_id"]))
            continue
        entry = registry.get(dump["layout_id"])
        results = list(interpret_rows(entry, rows[report.header_row], rows[report.header_row + 1:], policy=POLICY))
        check_rows(dump["file"], results, key.get("defects", {}).get(dump["name"], []), problems)
        if len(results) != dump["rows"]:
            problems.append("%s: %d rows read, the key says %d" % (dump["file"], len(results), dump["rows"]))
        low, high = dump["window_start"], dump["window_end"]
        wanted = [k for k in dump["cell_keys"] if k]
        population = {}
        unique, repeated = [], set()
        for r in results:
            if r.status != "accepted" or record_signature(r.record) in repeated:
                continue
            repeated.add(record_signature(r.record))
            unique.append(r)
        results = unique
        for r in results:
            rec = r.record or {}
            if not (low <= rec.get("start_local", "") <= high):
                problems.append("%s: a row lies outside the dump's window" % dump["file"])
                break
        for r in results:
            rec = r.record or {}
            if wanted and rec.get("cell_key") not in wanted:
                problems.append("%s: a row is at a cell the dump did not ask for" % dump["file"])
                break
        for r in results:
            rec = r.record or {}
            entry_ = population.setdefault(rec.get("a_norm"), {"events": 0, "cells": {}})
            entry_["events"] += 1
            if rec.get("cell_key") and (not wanted or rec["cell_key"] in wanted):
                entry_["cells"][rec["cell_key"]] = entry_["cells"].get(rec["cell_key"], 0) + 1
        answer = answers.get(dump["name"])
        if answer is None:
            problems.append("%s: no population answer in the key" % dump["name"])
            continue
        expected = {p["number"]: {"events": p["events"], "cells": p["cells"]} for p in answer["population"]}
        if population != expected:
            problems.append("%s: the population in the files differs from the key" % dump["name"])
        twice = sorted(n for n, p in population.items() if len(p["cells"]) >= 2) if wanted else None
        if twice != answer["at_two_or_more_cells"]:
            problems.append("%s: the numbers seen at two or more cells differ from the key" % dump["name"])

    # --- internet-record and application-usage exports: read back, addresses reserved, no two sessions hold one address and port at once ----
    held = []   # (public ip, port from, port to, start, end, msisdn)
    for item in key.get("sessions", []):
        rows = read_export(os.path.join(case_dir, *item["file"].split("/")))
        report = registry.find_header(rows)
        if report.matched_entry_id != item["layout_id"]:
            problems.append("%s: the registry selects %s, the key says %s" % (item["file"], report.matched_entry_id, item["layout_id"]))
            continue
        entry = registry.get(item["layout_id"])
        results = list(interpret_rows(entry, rows[report.header_row], rows[report.header_row + 1:], policy=POLICY))
        check_rows(item["file"], results, key.get("defects", {}).get(item["name"], []), problems)
        if len(results) != item["rows"]:
            problems.append("%s: %d rows read, the key says %d" % (item["file"], len(results), item["rows"]))
        repeated = set()
        for r in results:
            rec = r.record or {}
            if r.status != "accepted" or record_signature(rec) in repeated:
                continue
            repeated.add(record_signature(rec))
            if rec.get("msisdn") != item["msisdn"]:
                problems.append("%s: a row belongs to another subscriber" % item["file"])
                break
            if not (reserved.is_reserved_ip(rec.get("public_ip") or "") and (rec.get("server_ip") is None or reserved.is_reserved_ip(rec["server_ip"]))):
                problems.append("%s: a public or server address is outside the documentation blocks" % item["file"])
                break
            if not rec.get("public_port_from") or rec["public_port_from"] > rec["public_port_to"]:
                problems.append("%s: a public port block is empty or reversed" % item["file"])
                break
            begin = rec.get("nat_start_local") or rec["start_local"]
            finish = rec.get("nat_end_local") or rec.get("end_local") or rec["start_local"]
            if finish < begin:
                problems.append("%s: a session ends before it starts" % item["file"])
                break
            held.append((rec["public_ip"], rec["public_port_from"], rec["public_port_to"], begin, finish, rec["msisdn"]))
    by_ip = {}
    for ip, lo, hi, begin, finish, who in held:
        by_ip.setdefault(ip, []).append((lo, hi, begin, finish, who))
    for ip, spans in by_ip.items():
        spans.sort(key=lambda s: s[2])
        for i, a in enumerate(spans):
            clash = False
            for b in spans[i + 1:]:
                if b[2] > a[3]:
                    break
                if a[0] <= b[1] and b[0] <= a[1]:
                    clash = True
                    break
            if clash:
                problems.append("two sessions hold the same public address and port at the same time (%s)" % ip)
                break
    for item in key.get("app_usage", []):
        rows = read_export(os.path.join(case_dir, *item["file"].split("/")))
        report = registry.find_header(rows)
        if report.matched_entry_id != item["layout_id"]:
            problems.append("%s: the registry selects %s" % (item["file"], report.matched_entry_id))
            continue
        entry = registry.get(item["layout_id"])
        results = list(interpret_rows(entry, rows[report.header_row], rows[report.header_row + 1:], policy=POLICY))
        if any(r.status != "accepted" for r in results) or len(results) != item["rows"]:
            problems.append("%s: rows differ from the key" % item["file"])

    # --- address translation (A13): who held each probed address and port, recomputed from the query exports ---------------------------------
    for query in key.get("ip_queries", []):
        rows = read_export(os.path.join(case_dir, *query["file"].split("/")))
        report = registry.find_header(rows)
        if report.matched_entry_id != query["layout_id"]:
            problems.append("%s: the registry selects %s, the key says %s" % (query["file"], report.matched_entry_id, query["layout_id"]))
            continue
        entry = registry.get(query["layout_id"])
        results = list(interpret_rows(entry, rows[report.header_row], rows[report.header_row + 1:], policy=POLICY))
        if any(r.status != "accepted" for r in results) or len(results) != query["rows"]:
            problems.append("%s: rows differ from the key" % query["file"])
        holds = []
        for r in results:
            rec = r.record or {}
            if rec.get("public_ip") != query["public_ip"]:
                problems.append("%s: a row is on another public address" % query["file"])
                break
            holds.append((rec["public_port_from"], rec["public_port_to"], rec.get("nat_start_local") or rec["start_local"], rec.get("nat_end_local") or rec.get("end_local"), rec["msisdn"]))
        plant = next((p for p in key["plants"] if p["id"] == query["plant"]), None)
        if plant is None:
            problems.append("%s: no plant %s in the key" % (query["file"], query["plant"]))
            continue
        for probe in plant["probes"]:
            who = sorted({m for lo, hi, begin, finish, m in holds if lo <= probe["port"] <= hi and begin <= probe["instant"] <= finish})
            verdict = "none" if not who else "unique" if len(who) == 1 else "ambiguous"
            if (verdict, who) != (probe["result"], probe["holders"]):
                problems.append("%s: probe '%s' gives %s %s in the files, the key says %s %s" % (plant["id"], probe["label"], verdict, who, probe["result"], probe["holders"]))
            if (verdict, who) != (probe["intent"], probe["intended_holders"]):
                problems.append("%s: probe '%s' does not show what was planted (%s)" % (plant["id"], probe["label"], probe["intent"]))

    # --- co-location (A5): recomputed by brute force over the events read from the files -------------------------------------------------
    window = key["oracle"]["parameters"].get("window_minutes", 15)
    computed = {}
    for answer in key["oracle"].get("A5", []):
        x, y = answer["x"], answer["y"]
        if x not in placed or y not in placed:
            continue
        mine = brute_force_colocation(placed[x], placed[y], window)
        computed[(x, y)] = mine
        if (mine["pairs"], mine["adjacent"], [(w["cell_key"], w["start"], w["end"], w["pairs"]) for w in mine["windows"]]) != (
                answer["pairs_observed"], answer["pairs_within_window"],
                [(w["cell_key"], w["start_local"], w["end_local"], w["pairs"]) for w in answer["windows"]]):
            problems.append("A5 for %s and %s: the key and the files disagree on the windows" % (x[-5:], y[-5:]))
        elif abs(mine["expected"] - answer["pairs_expected"]) > 1e-3:
            problems.append("A5 for %s and %s: the chance baseline differs (%.4f in the files, %.4f in the key)" % (x[-5:], y[-5:], mine["expected"], answer["pairs_expected"]))
    for plant in key["plants"]:
        if plant["type"] != "colocation":
            continue
        x, y = plant["x"]["msisdn"], plant["y"]["msisdn"]
        mine = computed.get((x, y))
        if mine is None:
            problems.append("%s: no co-location answer for its pair" % plant["id"])
            continue
        for planted in plant["planted_windows"]:
            if not any(w["cell_key"] == planted["cell_key"] and w["start"] <= planted["start_local"] and w["end"] >= planted["end_local"] for w in mine["windows"]):
                problems.append("%s: planted window %s is not a co-location window in the files" % (plant["id"], planted["tag"]))
        paired = {(subject, when, cell) for subject, events in ((x, mine["x_paired"]), (y, mine["y_paired"])) for when, cell in events}
        for decoy in plant["decoys"]:
            for event in decoy["events"]:
                if (event["subject"], event["start_local"], event["cell_key"]) in paired:
                    problems.append("%s: a %s event takes part in a co-located pair" % (plant["id"], decoy["kind"]))
    # --- device and SIM history (A8), recomputed from the identities in the files ----------------------------------------------------------
    history, devices = {}, {}
    for number, seen in identity_rows.items():
        runs = []
        for when, imsi, imei in sorted(x for x in seen if x[1] or x[2]):
            if runs and runs[-1][0] == (imsi, imei):
                runs[-1][2] = when
            else:
                runs.append([(imsi, imei), when, when])
        history[number] = [(a[0], b[0], a[2], b[1]) for a, b in zip(runs, runs[1:])]    # (old identity, new identity, last old, first new)
        for identity, first, last in runs:
            if identity[1]:
                devices.setdefault(identity[1], {}).setdefault(number, []).append((first, last))
    answer_a8 = key["oracle"].get("A8")
    if answer_a8 is not None:
        for number, entry_ in answer_a8["numbers"].items():
            mine = [(c["kind"], (c["old_imsi"], c["old_imei"]), (c["new_imsi"], c["new_imei"]), c["last_seen_old"], c["first_seen_new"]) for c in entry_["changes"]]
            theirs = [("both" if o[0] != n[0] and o[1] != n[1] else "imei" if o[1] != n[1] else "imsi", o, n, lo, fn) for o, n, lo, fn in history.get(number, [])]
            if mine != theirs:
                problems.append("A8 for %s: the key and the files disagree on the identity changes" % number[-5:])
        shared_mine = sorted(d for d, per in devices.items() if len(per) > 1)
        if shared_mine != sorted(s["imei"] for s in answer_a8["shared_devices"]):
            problems.append("A8: the devices shared by two numbers differ from the key")
    planted_changes = {(p["number"], p["kind"]) for p in key["plants"] if p["type"] == "device_change"}
    for plant in key["plants"]:
        if plant["type"] == "device_change":
            wanted = {"handset_change": "imei", "sim_change": "imsi", "both_change": "both"}[plant["kind"]]
            found = [h for h in history.get(plant["number"], [])
                     if ("both" if h[0][0] != h[1][0] and h[0][1] != h[1][1] else "imei" if h[0][1] != h[1][1] else "imsi") == wanted]
            if len(found) != 1:
                problems.append("%s: the %s of %s is not in the files exactly once" % (plant["id"], plant["kind"], plant["number"][-5:]))
            else:
                old, new, last_old, first_new = found[0]
                if not (last_old < plant["from"] <= first_new):
                    problems.append("%s: the planted date is not between the last old and the first new identity" % plant["id"])
                if (old[1], new[1]) != (plant["old_imei"], plant["new_imei"]) and wanted != "imsi":
                    problems.append("%s: the devices in the files differ from the planted ones" % plant["id"])
                if (old[0], new[0]) != (plant["old_imsi"], plant["new_imsi"]) and wanted != "imei":
                    problems.append("%s: the SIMs in the files differ from the planted ones" % plant["id"])
        elif plant["type"] == "dual_sim":
            per = devices.get(plant["imei"], {})
            if sorted(per) != plant["numbers"]:
                problems.append("%s: the device is not shared by exactly the planted numbers" % plant["id"])
            else:
                spans = [(min(f for f, _ in v), max(l for _, l in v)) for v in per.values()]
                if not (spans[0][0] <= spans[1][1] and spans[1][0] <= spans[0][1]):
                    problems.append("%s: the dual-SIM numbers do not use the device at the same time" % plant["id"])
    changed_numbers = {n for n, h in history.items() if h}
    unexpected = changed_numbers - {n for n, _ in planted_changes}
    if unexpected:
        problems.append("A8: identity changes in the files that nothing planted (%d numbers)" % len(unexpected))

    # --- successor numbers (A9), recomputed from the files --------------------------------------------------------------------------------
    answer_a9 = key["oracle"].get("A9")
    if answer_a9 is not None:
        cfg = answer_a9["parameters"]
        case_start = datetime.datetime.fromisoformat(key["params"]["start_date"])
        case_end = case_start + datetime.timedelta(days=key["params"]["period_days"])
        day = datetime.timedelta(days=1)
        stamps = {n: sorted(datetime.datetime.fromisoformat(w_) for _, _, _, w_ in rows) for n, rows in visible.items() if rows}
        found = {}
        for x in sorted(stamps):
            if case_end - stamps[x][-1] < cfg["silence_days"] * day:
                continue
            listed = []
            for y in sorted(stamps):
                if y == x or stamps[y][0] - case_start < cfg["new_after_days"] * day:
                    continue
                if not (stamps[x][-1] - cfg["overlap_days"] * day <= stamps[y][0] <= stamps[x][-1] + cfg["window_days"] * day):
                    continue
                counts_x, counts_y = {}, {}
                for label, kind, direction, when in visible[x]:
                    moment = datetime.datetime.fromisoformat(when)
                    if stamps[x][-1] - cfg["history_days"] * day < moment <= stamps[x][-1] and kind in ("person_like", "international") and label and label not in (x, y):
                        counts_x[label] = counts_x.get(label, 0) + 1
                for label, kind, direction, when in visible[y]:
                    moment = datetime.datetime.fromisoformat(when)
                    if stamps[y][0] <= moment < stamps[y][0] + cfg["history_days"] * day and kind in ("person_like", "international") and label and label not in (x, y):
                        counts_y[label] = counts_y.get(label, 0) + 1
                cells_x, cells_y = {}, {}
                for moment, cell in placed.get(x, []):
                    if stamps[x][-1] - cfg["history_days"] * day < moment <= stamps[x][-1]:
                        cells_x[cell] = cells_x.get(cell, 0) + 1
                for moment, cell in placed.get(y, []):
                    if stamps[y][0] <= moment < stamps[y][0] + cfg["history_days"] * day:
                        cells_y[cell] = cells_y.get(cell, 0) + 1
                set_x, set_y = {c for c, n in cells_x.items() if n >= 2}, {c for c, n in cells_y.items() if n >= 2}
                jaccard = len(set_x & set_y) / len(set_x | set_y) if set_x | set_y else 0.0
                dot = sum(n * counts_y.get(k, 0) for k, n in counts_x.items())
                norm = (sum(n * n for n in counts_x.values()) ** 0.5) * (sum(n * n for n in counts_y.values()) ** 0.5)
                cosine = dot / norm if norm else 0.0
                device = bool({i for _, _, i in identity_rows.get(x, []) if i} & {i for _, _, i in identity_rows.get(y, []) if i})
                score = round(0.6 * cosine + 0.2 * jaccard + (0.2 if device else 0.0), 4)
                if score >= cfg["threshold"]:
                    listed.append((y, score, device))
            listed.sort(key=lambda c: (-c[1], c[0]))
            found[x] = listed
        theirs = {s["number"]: [(c["number"], c["score"], c["shared_device"]) for c in s["candidates"]] for s in answer_a9["silent"]}
        if sorted(found) != sorted(theirs):
            problems.append("A9: the silent numbers in the files differ from the key")
        for x in found:
            mine, other = found[x], theirs.get(x, [])
            if [m[0] for m in mine] != [o[0] for o in other] or any(abs(m[1] - o[1]) > 1e-3 or m[2] != o[2] for m, o in zip(mine, other)):
                problems.append("A9 for %s: the key and the files disagree on the successor candidates" % x[-5:])
        for plant in key["plants"]:
            if plant["type"] == "successor":
                top = found.get(plant["old"], [])
                if not top or top[0][0] != plant["new"]:
                    problems.append("%s: the planted successor is not the first candidate in the files" % plant["id"])
                elif top[0][2] != plant["with_device"]:
                    problems.append("%s: the shared handset differs from what was planted" % plant["id"])
                if stamps.get(plant["old"]) and stamps[plant["old"]][-1].strftime("%Y-%m-%d") > plant["last_day_old"]:
                    problems.append("%s: the old number is active after its planted last day" % plant["id"])
                if stamps.get(plant["new"]) and stamps[plant["new"]][0].strftime("%Y-%m-%d") < plant["first_day_new"]:
                    problems.append("%s: the new number is active before its planted first day" % plant["id"])
            elif plant["type"] == "silent_without_successor":
                if found.get(plant["number"]):
                    problems.append("%s: the silent number has a successor candidate" % plant["id"])
            elif plant["type"] == "late_without_predecessor":
                if any(c[0] == plant["number"] for listed in found.values() for c in listed):
                    problems.append("%s: the late starter is listed as somebody's successor" % plant["id"])

    # --- movement (A10) and change of rhythm (A11), recomputed from the files --------------------------------------------------------------
    answer_a10, answer_a11 = key["oracle"].get("A10"), key["oracle"].get("A11")
    if answer_a10 is not None and answer_a11 is not None:
        cfg10, cfg11 = answer_a10["parameters"], answer_a11["parameters"]
        key_numbers = [s_["msisdn"] for s_ in key["subjects"] if s_["msisdn"] in visible]
        day0 = datetime.date.fromisoformat(key["params"]["start_date"])

        def km_between(la1, lo1, la2, lo2):
            a = math.sin(math.radians(la2 - la1) / 2) ** 2 + math.cos(math.radians(la1)) * math.cos(math.radians(la2)) * math.sin(math.radians(lo2 - lo1) / 2) ** 2
            return 12742.0176 * math.asin(math.sqrt(a))
        found_alerts, found_changes, found_short, found_rhythm = {}, {}, {}, {}
        for number in key_numbers:
            steps = sorted(trail.get(number, []), key=lambda s: s[0])
            # speed alerts: consecutive events at different cells that are at least the threshold speed apart
            alerts = []
            for (t1, c1, la1, lo1), (t2, c2, la2, lo2) in zip(steps, steps[1:]):
                if c1 == c2 or la1 is None or la2 is None:
                    continue
                seconds = (t2 - t1).total_seconds()
                km = km_between(la1, lo1, la2, lo2)
                if seconds <= 0 or km * 3600.0 / seconds >= cfg10["speed_kmh"]:
                    alerts.append((t1.isoformat(sep=" "), t2.isoformat(sep=" "), c1, c2))
            found_alerts[number] = alerts
            # anchors: the most used cell of each slot per week of the case
            found_changes[number], found_short[number] = [], []
            for slot in ("night", "day"):
                per_week = {}
                for moment, cell, _, _ in steps:
                    hour = moment.hour
                    inside = (hour >= cfg10["night"][0] or hour < cfg10["night"][1]) if slot == "night" else (cfg10["day"][0] <= hour < cfg10["day"][1] and moment.weekday() < 5)
                    if inside:
                        per_week.setdefault((moment.date() - day0).days // cfg10["week_days"], {}).setdefault(cell, 0)
                        per_week[(moment.date() - day0).days // cfg10["week_days"]][cell] += 1
                stretches = []
                for week in sorted(per_week):
                    cells = per_week[week]
                    if sum(cells.values()) < cfg10["min_events"]:
                        continue
                    top = min(cells, key=lambda c: (-cells[c], c))
                    if stretches and stretches[-1][0] == top:
                        stretches[-1][2] += 1
                    else:
                        stretches.append([top, week, 1])
                kept = []
                for cell, week, weeks in stretches:
                    if weeks >= cfg10["confirm_weeks"]:
                        if kept and kept[-1][0] == cell:
                            kept[-1][2] += weeks
                        else:
                            kept.append([cell, week, weeks])
                    else:
                        found_short[number].append((slot, cell, week))
                for before, after in zip(kept, kept[1:]):
                    found_changes[number].append((slot, before[0], after[0], after[1]))
            # rhythm: the split of the daily counts with the largest Welch t
            days = {}
            for _, _, _, when in visible.get(number, []):
                moment = datetime.datetime.fromisoformat(when).date()
                days[moment] = days.get(moment, 0) + 1
            found_rhythm[number] = None
            if days:
                first_day = min(days)
                series = [days.get(first_day + datetime.timedelta(days=i), 0) for i in range((max(days) - first_day).days + 1)]
                best = None
                for k in range(cfg11["min_days"], len(series) - cfg11["min_days"] + 1):
                    a, b = series[:k], series[k:]
                    ma, mb = sum(a) / len(a), sum(b) / len(b)
                    va, vb = sum((x - ma) ** 2 for x in a) / (len(a) - 1), sum((x - mb) ** 2 for x in b) / (len(b) - 1)
                    spread = math.sqrt(va / len(a) + vb / len(b))
                    stat = (0.0 if ma == mb else math.inf) if spread == 0 else (mb - ma) / spread
                    if best is None or abs(stat) > abs(best[1]):
                        best = (k, stat, ma, mb)
                if best is not None:
                    k, stat, ma, mb = best
                    ratio = max(ma, mb) / min(ma, mb) if min(ma, mb) > 0 else math.inf
                    if abs(stat) >= cfg11["t_min"] and ratio >= cfg11["ratio_min"]:
                        found_rhythm[number] = (first_day + datetime.timedelta(days=k), "up" if mb > ma else "down")
        for number in key_numbers:
            theirs = answer_a10["numbers"][number]
            mine_alerts = [(a["from_time"], a["to_time"], a["from_cell"], a["to_cell"]) for a in theirs["speed_alerts"]]
            if mine_alerts != found_alerts[number]:
                problems.append("A10 for %s: the speed alerts in the key differ from the files" % number[-5:])
            if [(c["slot"], c["from_cell"], c["to_cell"], c["first_week"]) for c in theirs["changes"]] != found_changes[number]:
                problems.append("A10 for %s: the anchor changes in the key differ from the files" % number[-5:])
            if [(d["slot"], d["cell_key"], d["first_week"]) for d in theirs["short_deviations"]] != found_short[number]:
                problems.append("A10 for %s: the short deviations in the key differ from the files" % number[-5:])
            change = answer_a11["numbers"][number]["change"]
            if (change is None) != (found_rhythm[number] is None) or (change and (change["first_day"], change["direction"]) != (found_rhythm[number][0].isoformat(), found_rhythm[number][1])):
                problems.append("A11 for %s: the change of rhythm in the key differs from the files" % number[-5:])
        planted_moves = {pl["number"] for pl in key["plants"] if pl["type"] == "relocation"}
        for number in key_numbers:
            if found_changes[number] and number not in planted_moves:
                problems.append("A10: %s has an anchor change that nothing planted" % number[-5:])
        for plant in key["plants"]:
            number = plant.get("number")
            if plant["type"] == "relocation":
                moves = [c for c in found_changes[number] if c[0] == "night" and c[1] == plant["old_cell_key"] and c[2] == plant["new_cell_key"]]
                if len(moves) != 1 or not (7 * (moves[0][3] - 1) <= plant["from_day"] < 7 * (moves[0][3] + 1)):
                    problems.append("%s: the planted move is not in the files once, within a week of its date" % plant["id"])
                if any(c[0] == "day" for c in found_changes[number]):
                    problems.append("%s: the day anchor changed although only the home moved" % plant["id"])
            elif plant["type"] == "impossible_speed":
                first, second = sorted(e["start_local"] for e in plant["events"])
                if not any(a[0] == first and a[1] == second for a in found_alerts[number]):
                    problems.append("%s: the impossible pair is not a speed alert in the files" % plant["id"])
            elif plant["type"] == "slow_decoy":
                first, second = sorted(e["start_local"] for e in plant["events"])
                if any(a[0] >= first and a[1] <= second for a in found_alerts[number]):
                    problems.append("%s: the slow pair is a speed alert" % plant["id"])
            elif plant["type"] == "rate_change":
                got = found_rhythm[number]
                planted_day = datetime.date.fromisoformat(plant["from_date"])
                if got is None or got[1] != "up" or abs((got[0] - planted_day).days) > 5:
                    problems.append("%s: the change of rhythm is not found within five days of its date" % plant["id"])
            elif plant["type"] == "outlier_day":
                got = found_rhythm[number]
                if got is not None and abs((got[0] - datetime.date.fromisoformat(plant["day"])).days) <= 3:
                    problems.append("%s: the one unusual day was taken for a change of rhythm" % plant["id"])
            elif plant["type"] == "short_trip":
                if (("night", plant["trip_cell_key"], plant["first_week"]) not in found_short[number]) or any(c[0] == "night" for c in found_changes[number]):
                    problems.append("%s: the week away is not a short deviation, or it was taken for a move" % plant["id"])

    # --- possible meetings (A7): co-location windows preceded by a direct contact, recomputed from the files ---------------------------------
    gap = key["oracle"]["parameters"].get("contact_gap_minutes", 60)
    candidates = {}
    for answer in key["oracle"].get("A7", []):
        x, y = answer["x"], answer["y"]
        if (x, y) not in computed or x not in visible or y not in visible:
            continue
        contacts = sorted({when for label, kind, direction, when in visible[x] if label == y} | {when for label, kind, direction, when in visible[y] if label == x})
        mine = []
        for window in computed[(x, y)]["windows"]:
            begin = datetime.datetime.fromisoformat(window["start"])
            near = [c for c in contacts if 0 <= (begin - datetime.datetime.fromisoformat(c)).total_seconds() <= gap * 60]
            if near:
                lag = (begin - datetime.datetime.fromisoformat(max(near))).total_seconds() / 60
                mine.append((window["cell_key"], window["start"], window["end"], window["pairs"], near, round(lag, 2),
                             round(0.5 * min(window["pairs"], 3) / 3 + 0.5 * (1 - lag / gap), 4)))
        theirs = [(c["cell_key"], c["start_local"], c["end_local"], c["pairs"], c["contacts_before"], c["gap_minutes"], c["score"]) for c in answer["candidates"]]
        if mine != theirs:
            problems.append("A7 for %s and %s: the key and the files disagree on the candidate meetings" % (x[-5:], y[-5:]))
        candidates[(x, y)] = {c[1] for c in mine}
    for plant in key["plants"]:
        if plant["type"] != "possible_meeting":
            continue
        pair = (plant["x"]["msisdn"], plant["y"]["msisdn"])
        starts = candidates.get(pair)
        if starts is None:
            problems.append("%s: no meeting answer for its pair" % plant["id"])
            continue
        for meeting in plant["planted_meetings"]:
            if meeting["window_start"] not in starts:
                problems.append("%s: planted meeting %s is not a candidate in the files" % (plant["id"], meeting["window_tag"]))
        for negative in plant["negatives"]:
            if negative["window_start"] and negative["window_start"] in starts:
                problems.append("%s: a '%s' window is a candidate meeting" % (plant["id"], negative["kind"]))
    return problems


def brute_force_colocation(events_x, events_y, window_minutes):
    """Every pair of events, one of each subject (no shortcuts): co-located pairs, pairs within the window, the chance baseline, the windows."""
    limit = window_minutes * 60

    def klass(when):
        return (when.weekday() >= 5, when.hour)

    def habits(events):
        per, overall = {}, {}
        for when, cell in events:
            per.setdefault(klass(when), {}).setdefault(cell, 0)
            per[klass(when)][cell] += 1
            overall[cell] = overall.get(cell, 0) + 1
        return per, overall

    def share(habit, bucket, cell):
        per, overall = habit
        counts = per.get(bucket) or overall
        total = sum(counts.values())
        return counts.get(cell, 0) / total if total else 0.0
    hx, hy = habits(events_x), habits(events_y)
    cells = sorted(set(c for _, c in events_x) | set(c for _, c in events_y))
    adjacent, expected, pairs = 0, 0.0, []
    x_paired, y_paired = set(), set()
    for tx, cx in events_x:
        for ty, cy in events_y:
            if abs((tx - ty).total_seconds()) > limit:
                continue
            adjacent += 1
            bucket = klass(min(tx, ty))
            expected += sum(share(hx, bucket, c) * share(hy, bucket, c) for c in cells)
            if cx == cy:
                pairs.append((cx, min(tx, ty), max(tx, ty)))
                x_paired.add((tx.isoformat(sep=" "), cx))
                y_paired.add((ty.isoformat(sep=" "), cy))
    windows = []
    for cell in sorted(set(c for c, _, _ in pairs)):
        current = None
        for _, early, late in sorted(p for p in pairs if p[0] == cell):
            if current and (early - current["end_dt"]).total_seconds() <= limit:
                current["end_dt"] = max(current["end_dt"], late)
                current["pairs"] += 1
            else:
                current = {"cell_key": cell, "start_dt": early, "end_dt": late, "pairs": 1}
                windows.append(current)
    out = [{"cell_key": w_["cell_key"], "start": w_["start_dt"].isoformat(sep=" "), "end": w_["end_dt"].isoformat(sep=" "), "pairs": w_["pairs"]} for w_ in windows]
    out.sort(key=lambda w_: (w_["start"], w_["cell_key"]))
    return {"pairs": len(pairs), "adjacent": adjacent, "expected": expected, "windows": out, "x_paired": x_paired, "y_paired": y_paired}


def main(argv=None):
    argv = sys.argv[1:] if argv is None else argv
    if len(argv) != 1:
        print("usage: audit.py CASE_DIR")
        return 2
    problems = audit(argv[0])
    for p in problems:
        print("FAIL  " + p)
    print("AUDIT=%s problems=%d" % ("FAIL" if problems else "PASS", len(problems)))
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
