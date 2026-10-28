"""Gates G1 and G3 of docs/data/anpr-typed-family-20261026.md: a generated case WITH ANPR sightings is written by the PRODUCTION typed ingest, and the plate views equal the generator's oracle. A THROWAWAY
PostgreSQL, development cases, no model.

    python -u -X utf8 tools/case_generator/test_plate_oracle.py                 # development case 2, the quick check
    python -u -X utf8 tools/case_generator/test_plate_oracle.py --cases 1,2,3   # chosen development cases
    python -u -X utf8 tools/case_generator/test_plate_oracle.py --all           # all eight development cases

What it proves, through the read-only analyst role and the settings as the application uses them:
  * G1: the camera export is recognised as the layout anpr-a1, the rows loaded are the unique sightings plus the planted repeats, the quarantined rows are the planted ones, the flags are the planted
    ones (duplicate_row, confidence_percent) and no other, and a second ingest writes nothing;
  * G3: v_plate_sightings holds exactly the planted unique sightings (plate, camera, local time); v_plate_visits equals the oracle per plate and camera (count, first, last); for EVERY plate the
    count and the cameras equal the oracle; for EVERY pair of cameras the plates seen at both equal the oracle; the sightings at every camera on every local date equal the oracle; each look-alike
    plate has its own sightings and the planted plate has none of them; another collection or no scope shows nothing;
  * the migration: ops/027 preflight, apply, verify, apply again, rollback, apply again.
"""
import argparse
import collections
import sys
import tempfile

import case_set
import generate
import load_case
from anpr_case import Oracle
from ingestion.forensic_records.layout_registry import load_registry
from test_colocation_views import batch
from test_ingest_v2 import PsqlSink, ingest_case

MIG = "/tmp/mig/"
MIGRATIONS = ("016_effective_records.sql", "018_effective_ipdr.sql", "017_analyst_views.sql", "019_observed_cell_positions.sql", "020_contact_links_and_inventory.sql", "021_case_record.sql",
              "024_colocation_and_meetings.sql", "025_successor_candidates.sql", "026_movement_and_rhythm.sql")
LOCAL = "'YYYY-MM-DD HH24:MI:SS'"
ALL = ("SELECT plate_norm, camera_id, to_char(observed_utc AT TIME ZONE 'Asia/Karachi', %s) AS local FROM analyst.v_plate_sightings ORDER BY observed_utc, plate_norm, camera_id" % LOCAL)
VISITS = ("SELECT plate_norm, camera_id, sightings, to_char(first_utc AT TIME ZONE 'Asia/Karachi', %s) AS first_local, to_char(last_utc AT TIME ZONE 'Asia/Karachi', %s) AS last_local "
          "FROM analyst.v_plate_visits ORDER BY plate_norm, camera_id" % (LOCAL, LOCAL))
BOTH = ("SELECT plate_norm FROM analyst.v_plate_visits WHERE camera_id IN ('%s', '%s') GROUP BY plate_norm HAVING count(DISTINCT camera_id) = 2 ORDER BY plate_norm")
DAY = ("SELECT plate_norm, to_char(observed_utc AT TIME ZONE 'Asia/Karachi', %s) AS local FROM analyst.v_plate_sightings WHERE camera_id = '%s' AND observed_utc >= '%s 00:00:00+05' "
       "AND observed_utc < ('%s 00:00:00+05'::timestamptz + interval '1 day') ORDER BY observed_utc, plate_norm")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--cases", default="2")
    parser.add_argument("--all", action="store_true")
    args = parser.parse_args()
    wanted = list(range(1, 9)) if args.all else [int(n) for n in args.cases.split(",")]
    table = {seed: overrides for kind, seed, overrides in case_set.CASES if kind == "development"}
    registry = load_registry()
    problems, totals = [], collections.Counter()
    with tempfile.TemporaryDirectory() as tmp:
        db = load_case.Throwaway()
        try:
            db.start()
            load_case.prepare(db, registry)
            for name in MIGRATIONS:
                db.psql("-f", MIG + name)
            db.psql("-f", MIG + "ops/027_plate_sightings/preflight.sql")
            db.psql("-f", MIG + "027_plate_sightings.sql")
            db.psql("-f", MIG + "ops/027_plate_sightings/verify.sql")
            db.psql("-f", MIG + "027_plate_sightings.sql")
            db.psql("-f", MIG + "ops/027_plate_sightings/rollback.sql")
            assert db.sql("SELECT to_regclass('analyst.v_plate_sightings') IS NULL") == "t", "rollback left a view"
            db.psql("-f", MIG + "027_plate_sightings.sql")
            db.psql("-f", MIG + "ops/027_plate_sightings/verify.sql")
            print("migration 027: preflight, apply, verify, apply again, rollback, apply again: OK", flush=True)
            for seed in wanted:
                params = case_set.params_for(seed, table[seed])
                root = generate.write_case(params, tmp, with_anpr=True)
                key, _, summaries = ingest_case(db, root, registry, PsqlSink(db))
                truth = key["anpr"]
                oracle = Oracle(truth)
                planted = truth["planted"]
                label = "case %d" % seed
                # G1
                sheet = summaries["anpr"]["sheets"][0]
                if sheet["layout_id"] != "anpr-a1" or sheet["table"] != "plate_sighting":
                    problems.append("%s: the export was read as %s into %s" % (label, sheet["layout_id"], sheet["table"]))
                unique = len(truth["sightings"])
                if sheet["loaded"] != unique + planted["duplicate_rows"]:
                    problems.append("%s: %d rows loaded, the truth is %d unique + %d repeats" % (label, sheet["loaded"], unique, planted["duplicate_rows"]))
                quarantined = {k.split(":", 1)[1]: v for k, v in sheet["quarantined"].items()}
                if quarantined != planted["quarantined"]:
                    problems.append("%s: quarantined %s, planted %s" % (label, quarantined, planted["quarantined"]))
                flags = {k: v for k, v in sheet["flags"].items()}
                if flags.get("duplicate_row", 0) != planted["duplicate_rows"] or flags.get("confidence_percent", 0) != planted["confidence_percent_rows"] \
                        or set(flags) - {"duplicate_row", "confidence_percent"}:
                    problems.append("%s: flags %s, planted duplicates %d, percent %d" % (label, flags, planted["duplicate_rows"], planted["confidence_percent_rows"]))
                again = ingest_case(db, root, registry, PsqlSink(db), register=False)[2]["anpr"]
                if again["rows_inserted"] != 0:
                    problems.append("%s: a second ingest inserted %d rows" % (label, again["rows_inserted"]))
                totals["rows"] += sheet["loaded"]
                # G3
                scope = {"TimeZone": "Asia/Karachi", "app.tenant_id": load_case.TENANT, "app.collection_id": key["case_id"]}
                got = batch(db, scope, {"all": ALL, "visits": VISITS})
                have = sorted((r["plate_norm"], r["camera_id"], r["local"]) for r in got["all"])
                want = sorted((t["plate"], t["camera"], t["local"]) for t in truth["sightings"])
                if have != want:
                    extra, missing = sorted(set(have) - set(want)), sorted(set(want) - set(have))
                    problems.append("%s: v_plate_sightings differs from the truth: %d rows against %d (extra %s, missing %s)" % (label, len(have), len(want), extra[:2], missing[:2]))
                visits = {(r["plate_norm"], r["camera_id"]): (r["sightings"], r["first_local"], r["last_local"]) for r in got["visits"]}
                expected = collections.defaultdict(list)
                for t in truth["sightings"]:
                    expected[(t["plate"], t["camera"])].append(t["local"])
                wanted_visits = {k: (len(v), min(v), max(v)) for k, v in expected.items()}
                if visits != wanted_visits:
                    problems.append("%s: v_plate_visits differs from the oracle on %d of %d plate-camera pairs" % (label, len(set(visits.items()) ^ set(wanted_visits.items())), len(wanted_visits)))
                per_plate = batch(db, scope, {"p": "SELECT plate_norm, count(*) AS n FROM analyst.v_plate_sightings GROUP BY plate_norm ORDER BY plate_norm"})["p"]
                if {r["plate_norm"]: r["n"] for r in per_plate} != {p: oracle.count_of(p) for p in oracle.plates()}:
                    problems.append("%s: the count of sightings per plate differs from the oracle" % label)
                cams = [c["id"] for c in truth["cameras"]]
                queries = {}
                for i, a in enumerate(cams):
                    for b in cams[i + 1:]:
                        queries["%s|%s" % (a, b)] = BOTH % (a, b)
                both = batch(db, scope, queries)
                for pair, rows in both.items():
                    a, b = pair.split("|")
                    if [r["plate_norm"] for r in rows] != oracle.plates_at_both(a, b):
                        problems.append("%s: the plates seen at both %s and %s differ from the oracle" % (label, a, b))
                    totals["pairs"] += 1
                days = sorted({t["local"][:10] for t in truth["sightings"]})
                day_queries = {"%s|%s" % (c, d): DAY % (LOCAL, c, d, d) for c in cams for d in days[:6]}
                day_rows = batch(db, scope, day_queries)
                for k, rows in day_rows.items():
                    c, d = k.split("|")
                    if sorted((r["plate_norm"], r["local"]) for r in rows) != oracle.at_camera(c, d):   # the view is ordered by time, the oracle by plate: the sightings are compared as sets of rows
                        problems.append("%s: the sightings at %s on %s differ from the oracle" % (label, c, d))
                    totals["days"] += 1
                for pair in truth["look_alikes"]:
                    mine = [r for r in got["all"] if r["plate_norm"] == pair["of"]]
                    twin = [r for r in got["all"] if r["plate_norm"] == pair["plate"]]
                    if len(mine) != oracle.count_of(pair["of"]) or len(twin) != oracle.count_of(pair["plate"]) or not twin:
                        problems.append("%s: the plate %s and its look-alike %s are not counted apart" % (label, pair["of"], pair["plate"]))
                    totals["look_alikes"] += 1
                none = batch(db, dict(scope, **{"app.collection_id": "nobody"}), {"n": "SELECT count(*) AS n FROM analyst.v_plate_sightings"})["n"][0]["n"]
                unset = batch(db, {"TimeZone": "Asia/Karachi"}, {"n": "SELECT count(*) AS n FROM analyst.v_plate_visits"})["n"][0]["n"]
                if none != 0 or unset != 0:
                    problems.append("%s: scope fails open (%s and %s rows)" % (label, none, unset))
                print("%s: %d rows loaded (%d unique sightings, %d repeats, %s quarantined), %d plates, %d cameras: checked" %
                      (label, sheet["loaded"], unique, planted["duplicate_rows"], planted["quarantined"], len(oracle.plates()), len(cams)), flush=True)
        finally:
            left = db.stop()
            if left:
                print("WARNING: " + left)
    print("checked: %d rows, %d camera pairs, %d camera-days, %d look-alike pairs" % (totals["rows"], totals["pairs"], totals["days"], totals["look_alikes"]))
    if problems:
        print("PLATEORACLE=FAIL")
        for line in problems[:30]:
            print("  " + line)
        return 1
    print("PLATEORACLE=PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
