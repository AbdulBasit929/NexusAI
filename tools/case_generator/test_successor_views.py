"""Entry criterion E1 of the stage-3 pre-registration, sub-stage 3b (A9 successor candidates): the views of migration 025 equal the generator's oracle FOR ANY SETTING OF THE RULE, because
every number of the rule is a runtime parameter that the application sets from the question. A THROWAWAY PostgreSQL, development cases written by the PRODUCTION typed ingest, no model.

    python -u -X utf8 tools/case_generator/test_successor_views.py                 # development case 2, the quick check
    python -u -X utf8 tools/case_generator/test_successor_views.py --cases 1,2,3   # chosen development cases
    python -u -X utf8 tools/case_generator/test_successor_views.py --all           # all eight development cases (entry criterion E1)

What it proves, through the read-only analyst role and the settings as the application uses them (the case clock is set as the executor sets it):
  * ops/025 preflight and verify pass, the migration applies twice, and rolls back and forward;
  * for each of SETTINGS (the defaults and four other combinations of the silence, the window, the overlap, the history, the quiet start and the lowest score): v_silent_numbers lists exactly the
    oracle's silent numbers, and v_successor_candidates exactly the oracle's candidates (the pair, the first event to the second, the score, the cosine and the Jaccard index to 0.0001, the number of
    shared contacts, the shared handset), for every silent number;
  * a question for one old number, and for one new number, gets the matching rows of the full list;
  * nothing set means the specification's values, and empty, zero, negative or oversized settings are clamped, never an error;
  * the deliberate breakages: a lowest score of 45 percent and a silence of 5 days DO differ from the oracle at the defaults, so the check can fail;
  * scope: another tenant or another collection gives no rows.
"""
import argparse
import datetime
import json
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
# (name, settings the application would set, the oracle's overrides for the same rule)
SETTINGS = [
    ("defaults", {}, {}),
    ("silence 5, window 10", {"app.silence_days": "5", "app.successor_window_days": "10"}, {"silence_days": 5, "window_days": 10}),
    ("silence 10, window 21, score 20", {"app.silence_days": "10", "app.successor_window_days": "21", "app.successor_score_percent": "20"}, {"silence_days": 10, "window_days": 21, "threshold": 0.2}),
    ("everything else", {"app.silence_days": "14", "app.successor_window_days": "7", "app.successor_overlap_days": "0", "app.successor_history_days": "14", "app.successor_new_after_days": "3",
                         "app.successor_score_percent": "50"},
     {"silence_days": 14, "window_days": 7, "overlap_days": 0, "history_days": 14, "new_after_days": 3, "threshold": 0.5}),
    ("score 0, window 30", {"app.successor_window_days": "30", "app.successor_score_percent": "0"}, {"window_days": 30, "threshold": 0.0}),
]
SILENT_SQL = ("SELECT number, to_char(last_event_utc AT TIME ZONE 'Asia/Karachi', %s) AS last_event, silence_days FROM analyst.v_silent_numbers ORDER BY number" % LOCAL)
CANDIDATES_SQL = ("SELECT old_number, new_number, to_char(new_first_event_utc AT TIME ZONE 'Asia/Karachi', %s) AS first_event, score, contact_cosine, shared_contacts, cell_jaccard, shared_device, "
                  "silence_days, window_days, score_percent FROM analyst.v_successor_candidates ORDER BY old_number, new_number" % LOCAL)
BOUNDS_SQL = ("SELECT to_char(date_trunc('day', min(start_utc AT TIME ZONE 'Asia/Karachi')), 'YYYY-MM-DD') AS first_day, "
              "to_char(date_trunc('day', max(start_utc AT TIME ZONE 'Asia/Karachi')), 'YYYY-MM-DD') AS last_day FROM analyst.v_comm_events")


def oracle_answer(visible, case_start, case_end, overrides):
    return oracle.a9_successors(visible, case_start, case_end, **overrides)["silent"]


def compare(rows_silent, rows_candidates, silent, label, problems):
    want_silent = {s["number"]: s["last_event"] for s in silent}
    got_silent = {r["number"]: r["last_event"] for r in rows_silent}
    if want_silent != got_silent:
        problems.append("%s: silent numbers differ: view %s, oracle %s" % (label, sorted(got_silent.items())[:3], sorted(want_silent.items())[:3]))
    want = {(s["number"], c["number"]): c for s in silent for c in s["candidates"]}
    got = {(r["old_number"], r["new_number"]): r for r in rows_candidates}
    if set(want) != set(got):
        problems.append("%s: candidate pairs differ: only in the view %s, only in the oracle %s" % (label, sorted(set(got) - set(want))[:3], sorted(set(want) - set(got))[:3]))
        return
    for pair, c in want.items():
        g = got[pair]
        if (g["first_event"], g["shared_contacts"], g["shared_device"]) != (c["first_event"][:19], c["shared_contacts"], c["shared_device"]) \
                or abs(g["score"] - c["score"]) > 1.5e-4 or abs(g["contact_cosine"] - c["contact_cosine"]) > 1.5e-4 or abs(g["cell_jaccard"] - c["cell_jaccard"]) > 1.5e-4:
            problems.append("%s: %s differs: view %s, oracle %s" % (label, "/".join(x[-4:] for x in pair), {k: g[k] for k in ("first_event", "score", "contact_cosine", "shared_contacts", "cell_jaccard", "shared_device")}, c))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--cases", default="2")
    parser.add_argument("--all", action="store_true")
    args = parser.parse_args()
    wanted = list(range(1, 9)) if args.all else [int(n) for n in args.cases.split(",")]
    table = {seed: overrides for kind, seed, overrides in case_set.CASES if kind == "development"}
    registry = load_registry()
    problems, broken, totals = [], {"score": 0, "silence": 0}, {"silent": 0, "candidates": 0}
    with tempfile.TemporaryDirectory() as tmp:
        db = load_case.Throwaway()
        try:
            db.start()
            load_case.prepare(db, registry)
            db.psql("-f", MIG + "ops/017_analyst_views/preflight.sql")
            for migration in ("017_analyst_views.sql", "018_effective_ipdr.sql", "019_observed_cell_positions.sql", "020_contact_links_and_inventory.sql"):
                db.psql("-f", MIG + migration)
            db.psql("-f", MIG + "ops/025_successor_candidates/preflight.sql")
            db.psql("-f", MIG + "025_successor_candidates.sql")
            db.psql("-f", MIG + "ops/025_successor_candidates/verify.sql")
            db.psql("-f", MIG + "025_successor_candidates.sql")
            db.psql("-f", MIG + "ops/025_successor_candidates/verify.sql")
            db.psql("-f", MIG + "ops/025_successor_candidates/rollback.sql")
            assert db.sql("SELECT to_regclass('analyst.v_silent_numbers') IS NULL") == "t", "rollback left the view"
            db.psql("-f", MIG + "025_successor_candidates.sql")
            db.psql("-f", MIG + "ops/025_successor_candidates/verify.sql")
            print("migration 025: preflight, apply, verify, apply again, rollback, apply again: OK", flush=True)
            for seed in wanted:
                params = case_set.params_for(seed, table[seed])
                root = generate.write_case(params, tmp)
                key, _, _ = ingest_case(db, root, registry, PsqlSink(db))
                _, renders, _ = generate.build_case(params)
                visible = {s["msisdn"]: renders[s["person"]].visible for s in key["subjects"]}
                scope = dict(CLOCK, **{"app.tenant_id": load_case.TENANT, "app.collection_id": key["case_id"]})
                nominal_start = datetime.datetime.combine(datetime.date.fromisoformat(params.start_date), datetime.time(0, 0))
                nominal_end = nominal_start + datetime.timedelta(days=params.period_days)
                bounds = batch(db, scope, {"b": BOUNDS_SQL})["b"][0]
                case_start = datetime.datetime.fromisoformat(bounds["first_day"])
                case_end = datetime.datetime.fromisoformat(bounds["last_day"]) + datetime.timedelta(days=1)
                note = "" if (case_start, case_end) == (nominal_start, nominal_end) else " (the case's own days %s to %s differ from the nominal period; the oracle is run on the case's days)" % (case_start.date(), case_end.date())
                # the oracle at the defaults equals the key the generator wrote when the bounds agree
                if not note:
                    keyed = {s["number"]: [c["number"] for c in s["candidates"]] for s in oracle_answer(visible, nominal_start, nominal_end, {})}
                    from_key = {a["number"]: [c["number"] for c in a["candidates"]] for a in key["oracle"]["A9"]["silent"]} if isinstance(key["oracle"].get("A9"), dict) and "silent" in key["oracle"]["A9"] else None
                    if from_key is not None and keyed != from_key:
                        problems.append("case %d: the oracle at the defaults differs from the key" % seed)
                for name, settings, overrides in SETTINGS:
                    got = batch(db, dict(scope, **settings), {"s": SILENT_SQL, "c": CANDIDATES_SQL})
                    silent = oracle_answer(visible, case_start, case_end, overrides)
                    compare(got["s"], got["c"], silent, "case %d [%s]" % (seed, name), problems)
                    totals["silent"] += len(silent)
                    totals["candidates"] += sum(len(s["candidates"]) for s in silent)
                    if name == "score 0, window 30" and silent:
                        # one old number and one new number: the same rows as the full list
                        pairs = [(s["number"], c["number"]) for s in silent for c in s["candidates"]]
                        if pairs:
                            old, new = pairs[0]
                            one = batch(db, dict(scope, **settings), {"o": CANDIDATES_SQL.replace("ORDER BY", "WHERE old_number = '%s' ORDER BY" % old), "n": CANDIDATES_SQL.replace("ORDER BY", "WHERE new_number = '%s' ORDER BY" % new)})
                            if {(r["old_number"], r["new_number"]) for r in one["o"]} != {p for p in pairs if p[0] == old} or {(r["old_number"], r["new_number"]) for r in one["n"]} != {p for p in pairs if p[1] == new}:
                                problems.append("case %d: a question for one old or one new number does not equal the full list" % seed)
                # nothing set / clamping
                for setting, expected in (("", 7), ("0", 1), ("-4", 1), ("9999", 365)):
                    got = batch(db, dict(scope, **{"app.silence_days": setting}), {"s": SILENT_SQL})["s"]
                    want = {s["number"] for s in oracle_answer(visible, case_start, case_end, {"silence_days": expected})}
                    if {r["number"] for r in got} != want or any(r["silence_days"] != expected for r in got):
                        problems.append("case %d: the silence setting %r did not give %d days" % (seed, setting, expected))
                # deliberate breakages: the view at 45 percent / 5 days against the oracle at the defaults
                at_default = oracle_answer(visible, case_start, case_end, {})
                want_pairs = {(s["number"], c["number"]) for s in at_default for c in s["candidates"]}
                for percent in ("45", "60", "80", "95"):
                    got = batch(db, dict(scope, **{"app.successor_score_percent": percent}), {"c": CANDIDATES_SQL})["c"]
                    if {(r["old_number"], r["new_number"]) for r in got} != want_pairs:
                        broken["score"] += 1
                for days in ("3", "5", "14", "30"):
                    got = batch(db, dict(scope, **{"app.silence_days": days}), {"s": SILENT_SQL})["s"]
                    if {r["number"] for r in got} != {s["number"] for s in at_default}:
                        broken["silence"] += 1
                for bad in ({"app.tenant_id": "other", "app.collection_id": key["case_id"]}, {"app.tenant_id": load_case.TENANT, "app.collection_id": "other-case"}):
                    out = batch(db, dict(CLOCK, **bad), {"s": SILENT_SQL, "c": CANDIDATES_SQL})
                    if out["s"] or out["c"]:
                        problems.append("case %d: rows leaked under %s" % (seed, bad))
                print("development %d: %d subjects; %d silent numbers and %d candidates compared over %d settings of the rule%s"
                      % (seed, len(visible), sum(len(oracle_answer(visible, case_start, case_end, o)) for _, _, o in SETTINGS), sum(sum(len(s["candidates"]) for s in oracle_answer(visible, case_start, case_end, o)) for _, _, o in SETTINGS),
                         len(SETTINGS), note), flush=True)
            if broken["score"] == 0 and len(wanted) >= 4:  # whether a rule changes a reading depends on the case: a single small case may not show every one
                problems.append("the deliberate breakage (the view at a lowest score of 45, 60, 80 or 95 percent against the oracle at 35) did not change any case: the score check cannot fail on this data")
            if broken["silence"] == 0 and len(wanted) >= 4:
                problems.append("the deliberate breakage (the view at 3, 5, 14 or 30 days of silence against the oracle at 7) did not change any case: the silence check cannot fail on this data")
            print("deliberate breakage: other scores differ in %d reading(s), other silences in %d: the checks can fail" % (broken["score"], broken["silence"]))
        finally:
            left = db.stop()
            if left:
                problems.append(str(left))
    if problems:
        print("\nFAILED (%d):" % len(problems))
        for item in problems[:20]:
            print(" -", item)
        sys.exit(1)
    print("\nOK: the views equal the oracle for every tested setting of the rule on %d development case(s) (%d silent numbers and %d candidates in all)." % (len(wanted), totals["silent"], totals["candidates"]))


if __name__ == "__main__":
    main()
