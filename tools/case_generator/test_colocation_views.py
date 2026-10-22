"""Entry criterion E1 of the stage-3 pre-registration (docs/data/link-analysis-stage3-preregistration-20261022.md), sub-stage 3a: the views of migration 024 equal the generator's
oracle FOR ANY WINDOW AND GAP, because both are runtime parameters (app.colocation_minutes, app.contact_gap_minutes) that the application sets from the question. A THROWAWAY
PostgreSQL, development cases written by the PRODUCTION typed ingest, no model.

    python -X utf8 tools/case_generator/test_colocation_views.py                 # development case 2 (60 days), the quick check
    python -X utf8 tools/case_generator/test_colocation_views.py --cases 2,4,5   # chosen development cases
    python -X utf8 tools/case_generator/test_colocation_views.py --all           # all eight development cases (entry criterion E1)

What it proves, through the read-only analyst role and the scope settings as the application uses them:
  * ops/024 preflight and verify pass, the migration applies twice, and rolls back and forward;
  * for EVERY same-operator pair of a case and for each window W in WINDOWS (10, 15, 20, 22, 30 minutes): v_colocation_windows gives exactly the oracle's windows
    (cell key, start, end, pairs to the second), and the oracle at W = 15 equals the key the generator wrote (so the oracle itself is the one the specification states);
  * for every pair and each (W, G) in MEETINGS: v_meeting_candidates gives exactly the oracle's candidates (cell, window, pairs, number of contacts, gap to 0.01 minute, score to 0.0001);
  * nothing set means 15 and 60 (the key's own answers), and out-of-range or empty settings are clamped to 1..180 or the default, never an error and never a wrong window;
  * the deliberate breakage: the view run at 14 minutes DOES differ from the oracle at 15 on at least one pair, and the view at G = 30 differs from the oracle at G = 60, so the check can fail;
  * scope is structural: another tenant or another collection gives no rows.
"""
import argparse
import json
import sys
import tempfile

import case_set
import generate
import load_case
import oracle
from ingestion.forensic_records.layout_registry import load_registry
from test_ingest_v2 import PsqlSink, ingest_case

LOCAL = "'YYYY-MM-DD HH24:MI:SS'"
MIG = "/tmp/mig/"
WINDOWS = [10, 15, 20, 22, 30]
MEETINGS = [(15, 60), (20, 30), (22, 45), (30, 90)]
BOTH_ORDERS = {15, 22}


def batch(db, settings, queries):
    """Run several SELECTs in ONE read-only transaction as the analyst role with the given settings; returns {name: rows}."""
    lines = ["BEGIN READ ONLY;", "SET LOCAL ROLE forensic_analyst_ro;", "SET LOCAL statement_timeout = '60s';"]
    for name, value in settings.items():
        lines.append("SELECT set_config('%s', %s, true);" % (name, load_case.sql_text(value)))
    names = list(queries)
    for index, key in enumerate(names):
        lines.append("SELECT 'R|%d|' || coalesce(jsonb_agg(t)::text, '[]') FROM (%s) t;" % (index, queries[key].strip().rstrip(";")))
    lines.append("COMMIT;")
    out = db.psql("-At", "-f", "-", input="\n".join(lines).encode())
    rows = {}
    for line in out.splitlines():
        if line.startswith("R|"):
            _, index, payload = line.split("|", 2)
            rows[names[int(index)]] = json.loads(payload)
    assert set(rows) == set(queries), (sorted(set(queries) - set(rows)), out[-300:])
    return rows


def windows_query(x, y):
    return ("SELECT cell_key, to_char(start_utc AT TIME ZONE 'Asia/Karachi', %s) AS start_local, to_char(end_utc AT TIME ZONE 'Asia/Karachi', %s) AS end_local, pairs, window_minutes "
            "FROM analyst.v_colocation_windows WHERE a = %s AND b = %s ORDER BY start_utc, cell_key" % (LOCAL, LOCAL, load_case.sql_text(x), load_case.sql_text(y)))


def meetings_query(x, y):
    return ("SELECT cell_key, to_char(start_utc AT TIME ZONE 'Asia/Karachi', %s) AS start_local, to_char(end_utc AT TIME ZONE 'Asia/Karachi', %s) AS end_local, pairs, "
            "contacts, gap_minutes, score, window_minutes, allowed_gap_minutes FROM analyst.v_meeting_candidates WHERE a = %s AND b = %s ORDER BY start_utc, cell_key"
            % (LOCAL, LOCAL, load_case.sql_text(x), load_case.sql_text(y)))


def as_tuples(windows):
    return [(w["cell_key"], w["start_local"], w["end_local"], w["pairs"]) for w in windows]


def same_candidates(got, want):
    return len(got) == len(want) and all(
        (g["cell_key"], g["start_local"], g["end_local"], g["pairs"], g["contacts"]) == (c["cell_key"], c["start_local"], c["end_local"], c["pairs"], len(c["contacts_before"]))
        and abs(g["gap_minutes"] - c["gap_minutes"]) <= 1e-2 and abs(g["score"] - c["score"]) <= 1e-4 for g, c in zip(got, want))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--cases", default="2", help="development case numbers, comma separated")
    parser.add_argument("--all", action="store_true")
    args = parser.parse_args()
    wanted = list(range(1, 9)) if args.all else [int(n) for n in args.cases.split(",")]
    table = {seed: overrides for kind, seed, overrides in case_set.CASES if kind == "development"}
    registry = load_registry()
    problems = []
    broken_window = broken_gap = 0
    with tempfile.TemporaryDirectory() as tmp:
        db = load_case.Throwaway()
        try:
            db.start()
            load_case.prepare(db, registry)
            db.psql("-f", MIG + "ops/017_analyst_views/preflight.sql")
            for migration in ("017_analyst_views.sql", "018_effective_ipdr.sql", "019_observed_cell_positions.sql", "020_contact_links_and_inventory.sql"):
                db.psql("-f", MIG + migration)
            # --- the migration itself
            db.psql("-f", MIG + "ops/024_colocation_and_meetings/preflight.sql")
            db.psql("-f", MIG + "024_colocation_and_meetings.sql")
            db.psql("-f", MIG + "ops/024_colocation_and_meetings/verify.sql")
            db.psql("-f", MIG + "024_colocation_and_meetings.sql")          # idempotent
            db.psql("-f", MIG + "ops/024_colocation_and_meetings/verify.sql")
            db.psql("-f", MIG + "ops/024_colocation_and_meetings/rollback.sql")
            assert db.sql("SELECT to_regclass('analyst.v_colocation_windows') IS NULL") == "t", "rollback left the view"
            db.psql("-f", MIG + "024_colocation_and_meetings.sql")
            db.psql("-f", MIG + "ops/024_colocation_and_meetings/verify.sql")
            print("migration 024: preflight, apply, verify, apply again, rollback, apply again: OK")
            for seed in wanted:
                params = case_set.params_for(seed, table[seed])
                root = generate.write_case(params, tmp)
                key, _, _ = ingest_case(db, root, registry, PsqlSink(db))
                _, renders, _ = generate.build_case(params)
                visible = {s["msisdn"]: renders[s["person"]].visible for s in key["subjects"]}
                operator = {s["msisdn"]: s["operator"] for s in key["subjects"]}
                numbers = sorted(visible)
                pairs = [(x, y) for i, x in enumerate(numbers) for y in numbers[i + 1:] if operator[x] == operator[y]]
                scope = {"app.tenant_id": load_case.TENANT, "app.collection_id": key["case_id"]}
                counted = {"windows": 0, "candidates": 0, "queries": 0}
                # the oracle at 15 equals the key the generator wrote (the oracle is the specification's)
                keyed = {(a["x"], a["y"]): a for a in key["oracle"]["A5"]}
                for x, y in pairs:
                    if (x, y) in keyed and as_tuples(oracle.a5_colocation(visible[x], visible[y], 15)["windows"]) != [(w["cell_key"], w["start_local"], w["end_local"], w["pairs"]) for w in keyed[(x, y)]["windows"]]:
                        problems.append("case %d: the oracle at 15 minutes differs from the key for %s/%s" % (seed, x[-4:], y[-4:]))
                # A5 for every window
                for minutes in WINDOWS:
                    queries, want = {}, {}
                    for x, y in pairs:
                        windows = oracle.a5_colocation(visible[x], visible[y], minutes)["windows"]
                        for a, b in ((x, y), (y, x)) if minutes in BOTH_ORDERS else ((x, y),):
                            queries["%s|%s" % (a, b)] = windows_query(a, b)
                            want["%s|%s" % (a, b)] = as_tuples(windows)
                    got = batch(db, dict(scope, **{"app.colocation_minutes": str(minutes)}), queries)
                    counted["queries"] += len(queries)
                    for name, rows in got.items():
                        if as_tuples(rows) != want[name] or any(r["window_minutes"] != minutes for r in rows):
                            problems.append("case %d W=%d %s: view %s, oracle %s" % (seed, minutes, name.replace("|", "/")[-17:], as_tuples(rows)[:3], want[name][:3]))
                    counted["windows"] += sum(len(v) for v in want.values())
                # A7 for every (window, gap)
                for minutes, gap in MEETINGS:
                    queries, want = {}, {}
                    for x, y in pairs:
                        windows = oracle.a5_colocation(visible[x], visible[y], minutes)["windows"]
                        candidates = oracle.a7_meetings(visible[x], visible[y], x, y, windows, gap)["candidates"]
                        queries["%s|%s" % (x, y)] = meetings_query(x, y)
                        want["%s|%s" % (x, y)] = candidates
                    got = batch(db, dict(scope, **{"app.colocation_minutes": str(minutes), "app.contact_gap_minutes": str(gap)}), queries)
                    counted["queries"] += len(queries)
                    for name, rows in got.items():
                        if not same_candidates(rows, want[name]) or any(r["window_minutes"] != minutes or r["allowed_gap_minutes"] != gap for r in rows):
                            problems.append("case %d W=%d G=%d %s: view %s, oracle %s" % (seed, minutes, gap, name.replace("|", "/")[-17:], [(r["start_local"], r["gap_minutes"]) for r in rows][:3],
                                                                                    [(c["start_local"], c["gap_minutes"]) for c in want[name]][:3]))
                    counted["candidates"] += sum(len(v) for v in want.values())
                # nothing set: 15 and 60, the key's own answers
                queries = {"w|%s|%s" % (x, y): windows_query(x, y) for x, y in pairs}
                queries.update({"m|%s|%s" % (x, y): meetings_query(x, y) for x, y in pairs})
                got = batch(db, scope, queries)
                for x, y in pairs:
                    windows = oracle.a5_colocation(visible[x], visible[y], 15)["windows"]
                    if as_tuples(got["w|%s|%s" % (x, y)]) != as_tuples(windows):
                        problems.append("case %d defaults: windows differ for %s/%s" % (seed, x[-4:], y[-4:]))
                    if not same_candidates(got["m|%s|%s" % (x, y)], oracle.a7_meetings(visible[x], visible[y], x, y, windows, 60)["candidates"]):
                        problems.append("case %d defaults: meeting candidates differ for %s/%s" % (seed, x[-4:], y[-4:]))
                # clamping: empty, zero and too large settings never error and never give a wrong window
                probe = next(((x, y) for x, y in pairs if oracle.a5_colocation(visible[x], visible[y], 15)["windows"]), pairs[0])
                for setting, expected in (("", 15), ("0", 1), ("-5", 1), ("500", 180)):
                    rows = batch(db, dict(scope, **{"app.colocation_minutes": setting}), {"w": windows_query(*probe)})["w"]
                    want_windows = as_tuples(oracle.a5_colocation(visible[probe[0]], visible[probe[1]], expected)["windows"])
                    if as_tuples(rows) != want_windows or any(r["window_minutes"] != expected for r in rows):
                        problems.append("case %d: the setting %r did not give the window %d" % (seed, setting, expected))
                # the deliberate breakage
                for x, y in pairs:
                    want15 = as_tuples(oracle.a5_colocation(visible[x], visible[y], 15)["windows"])
                    if as_tuples(batch(db, dict(scope, **{"app.colocation_minutes": "14"}), {"w": windows_query(x, y)})["w"]) != want15:
                        broken_window += 1
                    want60 = oracle.a7_meetings(visible[x], visible[y], x, y, oracle.a5_colocation(visible[x], visible[y], 15)["windows"], 60)["candidates"]
                    if not same_candidates(batch(db, dict(scope, **{"app.contact_gap_minutes": "30"}), {"m": meetings_query(x, y)})["m"], want60):
                        broken_gap += 1
                # scope: nothing for another tenant or another collection
                for bad in ({"app.tenant_id": "other", "app.collection_id": key["case_id"]}, {"app.tenant_id": load_case.TENANT, "app.collection_id": "other-case"}):
                    if batch(db, bad, {"w": windows_query(*probe)})["w"]:
                        problems.append("case %d: rows leaked under %s" % (seed, bad))
                print("development %d: %d same-operator pairs; %d windows over %d window lengths and %d meeting candidates over %d (window, gap) settings equal the oracle (%d queries)"
                      % (seed, len(pairs), counted["windows"], len(WINDOWS), counted["candidates"], len(MEETINGS), counted["queries"]))
            if broken_window == 0:
                problems.append("the deliberate breakage (the view at 14 minutes against the oracle at 15) did not change any pair: the window check cannot fail on this data")
            if broken_gap == 0:
                problems.append("the deliberate breakage (the view at a 30-minute gap against the oracle at 60) did not change any pair: the gap check cannot fail on this data")
            print("deliberate breakage: 14 minutes differs on %d pair reading(s), a 30-minute gap on %d: the checks can fail" % (broken_window, broken_gap))
        finally:
            left = db.stop()
            if left:
                problems.append(str(left))
    if problems:
        print("\nFAILED (%d):" % len(problems))
        for item in problems[:20]:
            print(" -", item)
        sys.exit(1)
    print("\nOK: the views equal the oracle for every tested window and gap on %d development case(s)." % len(wanted))


if __name__ == "__main__":
    main()
