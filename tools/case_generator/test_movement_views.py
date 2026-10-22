"""Entry criterion E1 of the stage-3 pre-registration, sub-stage 3c (A10 movement, A11 change of rhythm): the views of migration 026 equal the generator's oracle FOR ANY SETTING OF THE RULES,
because every number of the rules is a runtime parameter that the application sets from the question. A THROWAWAY PostgreSQL, development cases written by the PRODUCTION typed ingest, no model.

    python -u -X utf8 tools/case_generator/test_movement_views.py                 # development case 2, the quick check
    python -u -X utf8 tools/case_generator/test_movement_views.py --cases 1,2,3   # chosen development cases
    python -u -X utf8 tools/case_generator/test_movement_views.py --all           # all eight development cases (entry criterion E1)

What it proves, through the read-only analyst role and the settings as the application uses them (the case clock is set as the executor sets it):
  * ops/026 preflight and verify pass, the migration applies twice, and rolls back and forward;
  * for each of A10_SETTINGS (the defaults and four other combinations of the fewest events of a week, the weeks an anchor must hold and the speed limit): the weekly anchors, the anchors, the moves,
    the trips and the speed alerts of EVERY number equal the oracle's (cell, weeks, dates, distances to 0.002 km, gaps to the second, speeds to 0.11 km/h);
  * for each of A11_SETTINGS (the defaults and three other combinations of the fewest days on a side and the least ratio): the numbers listed with a change of rhythm, their first day, means, t and ratio
    equal the oracle's, and the numbers it does not list are not listed;
  * a question for one number gets the matching rows of the full list;
  * nothing set means the specification's values; empty, zero, negative or oversized settings are clamped, never an error;
  * the deliberate breakages: a speed limit of 60, 100, 150 or 400 km/h, a hold of 1 week, a ratio of 110 or 300 percent and 3, 5, 7 or 20 days on a side DO differ from the oracle at the defaults somewhere, so the checks can fail;
  * scope: another tenant or another collection gives no rows.
"""
import argparse
import datetime
import sys
import tempfile

import case_set
import generate
import load_case
import oracle
from ingestion.forensic_records.layout_registry import load_registry
from test_colocation_views import batch
from test_ingest_v2 import PsqlSink, ingest_case

MIG = "/tmp/mig/"
CLOCK = {"TimeZone": "Asia/Karachi"}
LOCAL = "'YYYY-MM-DD HH24:MI:SS'"
A10_SETTINGS = [
    ("defaults", {}, {}),
    ("min events 2, hold 3 weeks", {"app.anchor_min_events": "2", "app.anchor_hold_weeks": "3"}, {"min_events": 2, "confirm_weeks": 3}),
    ("hold 1 week, 150 km/h", {"app.anchor_hold_weeks": "1", "app.speed_kmh": "150"}, {"confirm_weeks": 1, "speed_kmh": 150.0}),
    ("min events 5, 300 km/h", {"app.anchor_min_events": "5", "app.speed_kmh": "300"}, {"min_events": 5, "speed_kmh": 300.0}),
    ("min events 1, hold 4 weeks, 80 km/h", {"app.anchor_min_events": "1", "app.anchor_hold_weeks": "4", "app.speed_kmh": "80"}, {"min_events": 1, "confirm_weeks": 4, "speed_kmh": 80.0}),
]
A11_SETTINGS = [
    ("defaults", {}, {}),
    ("7 days a side, double", {"app.rhythm_min_side_days": "7", "app.rhythm_ratio_percent": "200"}, {"min_days": 7, "ratio_min": 2.0}),
    ("14 days a side, 120 percent", {"app.rhythm_min_side_days": "14", "app.rhythm_ratio_percent": "120"}, {"min_days": 14, "ratio_min": 1.2}),
    ("5 days a side, 110 percent", {"app.rhythm_min_side_days": "5", "app.rhythm_ratio_percent": "110"}, {"min_days": 5, "ratio_min": 1.1}),
]
SLOTS = {"night": lambda t: t.hour >= 22 or t.hour < 7, "day": lambda t: 9 <= t.hour < 17 and t.weekday() < 5}
QUERIES = {
    "weekly": "SELECT number, slot, week, cell_key, events_at_anchor, events_in_slot FROM analyst.v_weekly_anchors ORDER BY number, slot, week",
    "anchors": "SELECT number, slot, cell_key, first_week, last_week FROM analyst.v_movement_anchors ORDER BY number, slot, first_week",
    "changes": "SELECT number, slot, from_cell_key, to_cell_key, first_week, to_char(first_week_start, 'YYYY-MM-DD') AS first_week_start, distance_km FROM analyst.v_anchor_changes ORDER BY number, slot, first_week",
    "deviations": "SELECT number, slot, cell_key, first_week, last_week FROM analyst.v_anchor_deviations ORDER BY number, slot, first_week",
    "speed": ("SELECT number, to_char(from_time_utc AT TIME ZONE 'Asia/Karachi', %s) AS from_time, to_char(to_time_utc AT TIME ZONE 'Asia/Karachi', %s) AS to_time, from_cell_key, to_cell_key, "
              "distance_km, gap_s, speed_kmh, limit_kmh FROM analyst.v_speed_alerts ORDER BY number, from_time_utc, to_time_utc" % (LOCAL, LOCAL)),
}
RHYTHM_SQL = ("SELECT number, days, first_day::text AS first_day, mean_before, mean_after, t, ratio, direction, min_side_days, ratio_percent FROM analyst.v_rhythm_changes ORDER BY number")
BOUNDS_SQL = ("SELECT to_char(date_trunc('day', min(start_utc AT TIME ZONE 'Asia/Karachi')), 'YYYY-MM-DD') AS first_day FROM analyst.v_comm_events")


def close(a, b, tolerance):
    return (a is None and b is None) or (a is not None and b is not None and abs(a - b) <= tolerance)


def check_a10(rows, visible, case_start, overrides, label, problems):
    cfg = dict(oracle.A10_DEFAULTS, **overrides)
    reports = oracle.a10_movement(visible, case_start, **overrides)["numbers"]
    # weekly anchors
    want = {}
    for number, data in visible.items():
        ordered = sorted(data, key=lambda r: (r["start_local"], r.get("uid") or ""))
        for slot, in_slot in SLOTS.items():
            for week, cell in oracle._weekly_anchors(ordered, case_start, cfg, in_slot).items():
                want[(number, slot, week)] = cell
    got = {(r["number"], r["slot"], r["week"]): r["cell_key"] for r in rows["weekly"]}
    if got != want:
        problems.append("%s: weekly anchors differ (view %d, oracle %d; first difference %s)" % (label, len(got), len(want), sorted(set(got.items()) ^ set(want.items()))[:2]))
    # anchors, moves, trips
    want_anchors = {(n, slot, a["cell_key"], a["first_week"], a["last_week"]) for n, rep in reports.items() for slot, lst in rep["anchors"].items() for a in lst}
    got_anchors = {(r["number"], r["slot"], r["cell_key"], r["first_week"], r["last_week"]) for r in rows["anchors"]}
    if got_anchors != want_anchors:
        problems.append("%s: anchors differ (%s)" % (label, sorted(got_anchors ^ want_anchors)[:2]))
    want_dev = {(n, d["slot"], d["cell_key"], d["first_week"], d["last_week"]) for n, rep in reports.items() for d in rep["short_deviations"]}
    got_dev = {(r["number"], r["slot"], r["cell_key"], r["first_week"], r["last_week"]) for r in rows["deviations"]}
    if got_dev != want_dev:
        problems.append("%s: trips differ (%s)" % (label, sorted(got_dev ^ want_dev)[:2]))
    want_changes = {(n, c["slot"], c["from_cell"], c["to_cell"], c["first_week"], c["first_week_start"]): c["distance_km"] for n, rep in reports.items() for c in rep["changes"]}
    got_changes = {(r["number"], r["slot"], r["from_cell_key"], r["to_cell_key"], r["first_week"], r["first_week_start"]): r["distance_km"] for r in rows["changes"]}
    if set(got_changes) != set(want_changes) or any(not close(got_changes[k], want_changes[k], 0.002) for k in want_changes if k in got_changes):
        problems.append("%s: moves differ (%s)" % (label, sorted(set(got_changes) ^ set(want_changes))[:2] or "distance"))
    # speed alerts
    want_alerts = {(n, a["from_time"], a["to_time"]): a for n, rep in reports.items() for a in rep["speed_alerts"]}
    got_alerts = {(r["number"], r["from_time"], r["to_time"]): r for r in rows["speed"]}
    if set(got_alerts) != set(want_alerts):
        problems.append("%s: speed alerts differ (view %d, oracle %d; %s)" % (label, len(got_alerts), len(want_alerts), sorted(set(got_alerts) ^ set(want_alerts))[:2]))
    else:
        for key, a in want_alerts.items():
            g = got_alerts[key]
            if (g["from_cell_key"], g["to_cell_key"], g["gap_s"]) != (a["from_cell"], a["to_cell"], a["gap_s"]) or not close(g["distance_km"], a["distance_km"], 0.002) or not close(g["speed_kmh"], a["speed_kmh"], 0.11):
                problems.append("%s: alert %s differs: view %s, oracle %s" % (label, key, g, a))
                break
    return len(want), len(want_anchors), len(want_changes), len(want_dev), len(want_alerts)


def check_a11(rows, visible, overrides, label, problems):
    reports = oracle.a11_rate_change(visible, **overrides)["numbers"]
    want = {n: r["change"] for n, r in reports.items() if r["change"]}
    got = {r["number"]: r for r in rows}
    if set(got) != set(want):
        problems.append("%s: rhythm changes listed differ: view %s, oracle %s" % (label, sorted(got), sorted(want)))
        return len(want)
    for n, c in want.items():
        g = got[n]
        if g["first_day"] != c["first_day"] or g["direction"] != c["direction"] or not close(g["mean_before"], c["mean_before"], 0.0006) or not close(g["mean_after"], c["mean_after"], 0.0006) \
                or not close(g["t"], c["t"], 0.011) or not close(g["ratio"], c["ratio"], 0.0006) or g["days"] != reports[n]["days"]:
            problems.append("%s: %s differs: view %s, oracle %s" % (label, n[-4:], {k: g[k] for k in ("first_day", "mean_before", "mean_after", "t", "ratio", "direction", "days")}, c))
    return len(want)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--cases", default="2")
    parser.add_argument("--all", action="store_true")
    args = parser.parse_args()
    wanted = list(range(1, 9)) if args.all else [int(n) for n in args.cases.split(",")]
    table = {seed: overrides for kind, seed, overrides in case_set.CASES if kind == "development"}
    registry = load_registry()
    problems, broken, totals = [], {"speed": 0, "hold": 0, "ratio": 0, "side": 0}, {"a10": [0, 0, 0, 0, 0], "a11": 0}
    with tempfile.TemporaryDirectory() as tmp:
        db = load_case.Throwaway()
        try:
            db.start()
            load_case.prepare(db, registry)
            db.psql("-f", MIG + "ops/017_analyst_views/preflight.sql")
            for migration in ("017_analyst_views.sql", "018_effective_ipdr.sql", "019_observed_cell_positions.sql", "020_contact_links_and_inventory.sql"):
                db.psql("-f", MIG + migration)
            db.psql("-f", MIG + "ops/026_movement_and_rhythm/preflight.sql")
            db.psql("-f", MIG + "026_movement_and_rhythm.sql")
            db.psql("-f", MIG + "ops/026_movement_and_rhythm/verify.sql")
            db.psql("-f", MIG + "026_movement_and_rhythm.sql")
            db.psql("-f", MIG + "ops/026_movement_and_rhythm/verify.sql")
            db.psql("-f", MIG + "ops/026_movement_and_rhythm/rollback.sql")
            assert db.sql("SELECT to_regclass('analyst.v_weekly_anchors') IS NULL") == "t", "rollback left a view"
            db.psql("-f", MIG + "026_movement_and_rhythm.sql")
            db.psql("-f", MIG + "ops/026_movement_and_rhythm/verify.sql")
            print("migration 026: preflight, apply, verify, apply again, rollback, apply again: OK", flush=True)
            for seed in wanted:
                params = case_set.params_for(seed, table[seed])
                root = generate.write_case(params, tmp)
                key, _, _ = ingest_case(db, root, registry, PsqlSink(db))
                _, renders, _ = generate.build_case(params)
                visible = {s["msisdn"]: renders[s["person"]].visible for s in key["subjects"]}
                scope = dict(CLOCK, **{"app.tenant_id": load_case.TENANT, "app.collection_id": key["case_id"]})
                first_day = batch(db, scope, {"b": BOUNDS_SQL})["b"][0]["first_day"]
                case_start = datetime.datetime.fromisoformat(first_day)
                nominal = datetime.datetime.combine(datetime.date.fromisoformat(params.start_date), datetime.time(0, 0))
                note = "" if case_start == nominal else " (the case's first day %s differs from the nominal %s; the oracle is run on the case's day)" % (first_day, nominal.date())
                for name, settings, overrides in A10_SETTINGS:
                    rows = batch(db, dict(scope, **settings), QUERIES)
                    counts = check_a10(rows, visible, case_start, overrides, "case %d [%s]" % (seed, name), problems)
                    totals["a10"] = [x + y for x, y in zip(totals["a10"], counts)]
                for name, settings, overrides in A11_SETTINGS:
                    rows = batch(db, dict(scope, **settings), {"r": RHYTHM_SQL})["r"]
                    totals["a11"] += check_a11(rows, visible, overrides, "case %d [%s]" % (seed, name), problems)
                # one number gets the matching rows of the full list
                full = batch(db, scope, {"s": QUERIES["speed"], "r": RHYTHM_SQL, "c": QUERIES["changes"]})
                for kind, query, field in (("s", QUERIES["speed"], "number"), ("c", QUERIES["changes"], "number"), ("r", RHYTHM_SQL, "number")):
                    if full[kind]:
                        number = full[kind][0][field]
                        one = batch(db, scope, {"o": query.replace(" ORDER BY", " WHERE number = '%s' ORDER BY" % number, 1)})["o"]
                        if one != [r for r in full[kind] if r[field] == number]:
                            problems.append("case %d: a question for one number does not equal its rows of the full list (%s)" % (seed, kind))
                # clamping: empty, zero, negative and oversized settings never error and give the bounded value
                for setting, value, expected, label in (("app.speed_kmh", "0", 1.0, "speed"), ("app.speed_kmh", "999999", 2000.0, "speed"), ("app.anchor_hold_weeks", "0", 1, "hold"),
                                                         ("app.anchor_hold_weeks", "99", 52, "hold")):
                    rows = batch(db, dict(scope, **{setting: value}), QUERIES)
                    ov = {"speed_kmh": expected} if label == "speed" else {"confirm_weeks": expected}
                    check_a10(rows, visible, case_start, ov, "case %d clamp %s=%s" % (seed, setting, value), problems)
                for setting, value, ov in (("app.rhythm_min_side_days", "0", {"min_days": 2}), ("app.rhythm_ratio_percent", "5", {"ratio_min": 1.01})):
                    rows = batch(db, dict(scope, **{setting: value}), {"r": RHYTHM_SQL})["r"]
                    check_a11(rows, visible, ov, "case %d clamp %s=%s" % (seed, setting, value), problems)
                # the deliberate breakages: the view at another setting against the oracle at the defaults
                base10 = oracle.a10_movement(visible, case_start)["numbers"]
                want_alerts = {(n, a["from_time"], a["to_time"]) for n, r in base10.items() for a in r["speed_alerts"]}
                want_changes = {(n, c["slot"], c["to_cell"], c["first_week"]) for n, r in base10.items() for c in r["changes"]}
                for kmh in ("60", "100", "150", "400"):
                    rows = batch(db, dict(scope, **{"app.speed_kmh": kmh}), {"s": QUERIES["speed"]})
                    if {(r["number"], r["from_time"], r["to_time"]) for r in rows["s"]} != want_alerts:
                        broken["speed"] += 1
                rows = batch(db, dict(scope, **{"app.anchor_hold_weeks": "1"}), {"c": QUERIES["changes"]})
                if {(r["number"], r["slot"], r["to_cell_key"], r["first_week"]) for r in rows["c"]} != want_changes:
                    broken["hold"] += 1
                want_rhythm = {n for n, r in oracle.a11_rate_change(visible)["numbers"].items() if r["change"]}
                for ratio, side in (("110", "10"), ("150", "3"), ("150", "5"), ("150", "7"), ("150", "20"), ("300", "10")):
                    rows = batch(db, dict(scope, **{"app.rhythm_ratio_percent": ratio, "app.rhythm_min_side_days": side}), {"r": RHYTHM_SQL})["r"]
                    if {r["number"] for r in rows} != want_rhythm:
                        broken["ratio" if side == "10" else "side"] += 1
                for bad in ({"app.tenant_id": "other", "app.collection_id": key["case_id"]}, {"app.tenant_id": load_case.TENANT, "app.collection_id": "other-case"}):
                    out = batch(db, dict(CLOCK, **bad), dict(QUERIES, r=RHYTHM_SQL))
                    if any(out.values()):
                        problems.append("case %d: rows leaked under %s" % (seed, bad))
                print("development %d: %d numbers; A10 and A11 compared over %d and %d settings of the rules%s" % (seed, len(visible), len(A10_SETTINGS), len(A11_SETTINGS), note), flush=True)
            for name, count in broken.items():
                if count == 0 and len(wanted) >= 4:  # whether a rule changes a reading depends on the case: a single small case may not show every one
                    problems.append("the deliberate breakage '%s' did not change any reading: that check cannot fail on this data" % name)
            print("deliberate breakage differs in: %s" % broken)
        finally:
            left = db.stop()
            if left:
                problems.append(str(left))
    if problems:
        print("\nFAILED (%d):" % len(problems))
        for item in problems[:20]:
            print(" -", item)
        sys.exit(1)
    a = totals["a10"]
    print("\nOK: the views equal the oracle for every tested setting on %d development case(s): %d weekly anchors, %d anchors, %d moves, %d trips, %d speed alerts and %d rhythm changes compared."
          % (len(wanted), a[0], a[1], a[2], a[3], a[4], totals["a11"]))


if __name__ == "__main__":
    main()
