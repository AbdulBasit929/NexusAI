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
    meta, sheets = profile_structure.iter_xlsx_sheets(path)
    name, rows = sheets[0]
    return [list(r) for r in rows()]


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
        rejected = [r for r in results if r.status != "accepted"]
        if rejected:
            problems.append("%s: %d rows are not accepted (first reason: %s)" % (subject["file"], len(rejected), rejected[0].reason))
        if len(results) != subject["rows"]:
            problems.append("%s: %d rows read, the key says %d" % (subject["file"], len(results), subject["rows"]))
        mine, where = [], []
        for r in results:
            rec = r.record or {}
            if r.status != "accepted":
                continue
            if rec["a_norm"] != subject["msisdn"]:
                problems.append("%s: a row belongs to another subscriber" % subject["file"])
            for value, check in ((rec.get("a_norm"), reserved.is_reserved_msisdn), (rec.get("b_norm") if rec.get("b_kind") == "person_like" else None, reserved.is_reserved_msisdn),
                                 (rec.get("imsi"), reserved.is_test_imsi), (rec.get("imei"), is_test_device)):
                if value and not check(value):
                    problems.append("%s: a value outside the reserved ranges" % subject["file"])
                    break
            label = rec["b_norm"] if rec.get("b_norm") else rec.get("b_raw")
            mine.append((label, rec.get("b_kind"), rec["direction"], rec["start_local"]))
            if rec.get("cell_key"):
                where.append((datetime.datetime.fromisoformat(rec["start_local"]), rec["cell_key"]))
        visible[subject["msisdn"]] = mine
        placed[subject["msisdn"]] = where

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
        if any(r.status != "accepted" for r in results):
            problems.append("%s: rows are not accepted" % dump["file"])
        if len(results) != dump["rows"]:
            problems.append("%s: %d rows read, the key says %d" % (dump["file"], len(results), dump["rows"]))
        low, high = dump["window_start"], dump["window_end"]
        wanted = [k for k in dump["cell_keys"] if k]
        population = {}
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
