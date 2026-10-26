"""Gates G2 and G3 (the view part) of docs/data/anpr-typed-family-20261026.md on a THROWAWAY PostgreSQL: the plate normaliser equals forensic.normalize_plate, migration 027 applies, verifies and rolls
back, and the two plate views show what they promise. No model, no real data.

    python -u -X utf8 tools/case_generator/test_plate_views.py

What it proves:
  * G2: the 63 plate tokens of ingestion/forensic_records/tests/plate_tokens.py give the same search key through forensic.normalize_plate (SQL) and normalize_plate (Python): zero differences;
  * ops/027 preflight and verify pass, the migration applies twice, rolls back and forward;
  * a golden camera export of the draft layout anpr-a1 is written by the PRODUCTION typed ingest into forensic.plate_sighting, and through the read-only analyst role and the settings the
    application sets: v_plate_sightings holds exactly the accepted rows, plate_raw as the file wrote it and plate_norm as the key; v_plate_visits equals the per-plate, per-camera counts of the
    same rows (count, first, last); a second ingest of the file writes zero rows;
  * a row read from an image (source_kind image), a rejected row and an exact repeat within a file (duplicate_row) are NOT in the views; a row of another collection is not visible, and with no
    scope set the views return nothing; the role cannot read the table itself.
"""
import hashlib
import json
import os
import sys
import tempfile

import load_case
from ingestion.forensic_records import typed_ingest as ti
from ingestion.forensic_records.layout_registry import Policy, load_registry
from ingestion.forensic_records.layout_registry.parties import normalize_plate
from ingestion.forensic_records.tests.plate_tokens import PLATE_TOKENS
from test_colocation_views import batch

sys.path.insert(0, os.path.join(load_case.REPO, "tools", "real_data"))
import make_golden  # noqa: E402
from test_ingest_v2 import PsqlSink  # noqa: E402

MIG = "/tmp/mig/"
CASE = "plate-view-check"
OTHER = "plate-view-check-other"
MIGRATIONS = ("016_effective_records.sql", "018_effective_ipdr.sql", "017_analyst_views.sql", "019_observed_cell_positions.sql", "020_contact_links_and_inventory.sql", "021_case_record.sql",
              "024_colocation_and_meetings.sql", "025_successor_candidates.sql", "026_movement_and_rhythm.sql")


def normalisation_differences(db):
    lines = []
    for index, token in enumerate(PLATE_TOKENS):
        lines.append("SELECT %d || '|' || forensic.normalize_plate(convert_from(decode('%s', 'hex'), 'UTF8'));" % (index, token.encode("utf-8").hex()))
    out = db.psql("-At", "-f", "-", input="\n".join(lines).encode("utf-8"))
    got = {}
    for line in out.splitlines():
        if "|" in line:
            index, _, value = line.partition("|")
            got[int(index)] = value
    problems = []
    for index, token in enumerate(PLATE_TOKENS):
        if got.get(index) != normalize_plate(token):
            problems.append("token %d %r: SQL %r, Python %r" % (index, token, got.get(index), normalize_plate(token)))
    return problems


def register(db, case_id, name, content):
    evidence_id, batch_id = load_case.lineage_ids(case_id, name)
    sha = hashlib.sha256(content).hexdigest()
    db.script(load_case.evidence_statement(evidence_id, case_id, name, "csv", len(content), sha))
    return ti.Lineage(load_case.TENANT, case_id, evidence_id, name, batch_id)


def main():
    registry = load_registry()
    problems = []
    with tempfile.TemporaryDirectory() as tmp:
        db = load_case.Throwaway()
        try:
            db.start()
            load_case.prepare(db, registry)
            differences = normalisation_differences(db)
            problems += ["G2 " + d for d in differences]
            print("G2: %d plate tokens through the SQL function and Python: %d differences" % (len(PLATE_TOKENS), len(differences)), flush=True)
            for name in MIGRATIONS:
                db.psql("-f", MIG + name)
            db.psql("-f", MIG + "ops/027_plate_sightings/preflight.sql")
            db.psql("-f", MIG + "027_plate_sightings.sql")
            db.psql("-f", MIG + "ops/027_plate_sightings/verify.sql")
            db.psql("-f", MIG + "027_plate_sightings.sql")
            db.psql("-f", MIG + "ops/027_plate_sightings/verify.sql")
            db.psql("-f", MIG + "ops/027_plate_sightings/rollback.sql")
            assert db.sql("SELECT to_regclass('analyst.v_plate_sightings') IS NULL AND to_regclass('analyst.v_plate_visits') IS NULL") == "t", "rollback left a view"
            db.psql("-f", MIG + "027_plate_sightings.sql")
            db.psql("-f", MIG + "ops/027_plate_sightings/verify.sql")
            print("migration 027: preflight, apply, verify, apply again, rollback, apply again: OK", flush=True)

            # a golden camera export, written by the production typed ingest
            golden = make_golden.generate(registry.get("anpr-a1"), rows=96, seed=5, defects=False)
            path = os.path.join(tmp, "cameras.csv")
            make_golden.write_delimited(path, golden.table, golden.encoding or "utf-8", golden.delimiter or ",")
            content = open(path, "rb").read()
            policy = Policy(source_tz="Asia/Karachi", tz_basis="declared")
            lineage = register(db, CASE, "cameras.csv", content)
            sink = PsqlSink(db)
            first = ti.ingest_file(path, lineage, policy, sink, registry=registry)
            again = ti.ingest_file(path, lineage, policy, sink, registry=registry)
            if (first["rows_loaded"], first["rows_inserted"]) != (96, 96) or again["rows_inserted"] != 0:
                problems.append("G1 ingest: first %s, second inserted %s" % ({k: first[k] for k in ("rows_loaded", "rows_inserted")}, again["rows_inserted"]))
            print("ingest: %d rows loaded, %d inserted; a second ingest inserted %d" % (first["rows_loaded"], first["rows_inserted"], again["rows_inserted"]), flush=True)

            # rows the views must not show: an image reading, a rejected row, an exact repeat, and a row of another collection
            other = register(db, OTHER, "other.csv", b"another case")
            seen = db.rows("SELECT sighting_id, plate_raw, plate_norm, observed_utc, camera_id FROM forensic.plate_sighting WHERE collection_id = %s ORDER BY sighting_id LIMIT 1" % load_case.sql_text(CASE))[0]
            clone = ("INSERT INTO forensic.plate_sighting (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, layout_version, source_kind, "
                     "plate_raw, plate_norm, observed_utc, time_basis, camera_id, review_state, quality_flags) "
                     "SELECT tenant_id, %s, %s, file_id, batch_id, 9000 + %d, 'x%d' || row_hash, content_hash, layout_id, layout_version, %s, plate_raw, plate_norm, observed_utc, time_basis, "
                     "camera_id, %s, %s FROM forensic.plate_sighting WHERE sighting_id = %d")
            for number, (collection, kind, review, flags) in enumerate((
                    (CASE, "'image'", "'unreviewed'", "'{}'"),
                    (CASE, "'csv'", "'rejected'", "'{}'"),
                    (CASE, "'csv'", "'unreviewed'", "'{duplicate_row}'"))):
                statement = clone % (load_case.sql_text(collection), "evidence_id", number, number, kind, review, flags, seen["sighting_id"])
                db.script("SELECT set_config('app.tenant_id', %s, false);\n%s;" % (load_case.sql_text(load_case.TENANT), statement))
            db.script("SELECT set_config('app.tenant_id', %s, false);\n%s;" % (load_case.sql_text(load_case.TENANT), clone % (load_case.sql_text(OTHER), load_case.sql_text(other.evidence_id) + "::uuid", 5, 5, "'csv'", "'unreviewed'", "'{}'", seen["sighting_id"])))

            scope = {"TimeZone": "Asia/Karachi", "app.tenant_id": load_case.TENANT, "app.collection_id": CASE}
            rows = batch(db, scope, {
                "all": "SELECT plate_raw, plate_norm, camera_id, observed_utc FROM analyst.v_plate_sightings ORDER BY observed_utc, plate_raw, camera_id",
                "visits": "SELECT plate_norm, camera_id, sightings, first_utc, last_utc FROM analyst.v_plate_visits ORDER BY plate_norm, camera_id",
                "count": "SELECT count(*) AS n FROM analyst.v_plate_sightings"})
            if rows["count"][0]["n"] != 96:
                problems.append("G3 v_plate_sightings holds %s rows, the file's accepted rows are 96 (image, rejected, repeat and other-collection rows must not show)" % rows["count"][0]["n"])
            # plate_raw as written, plate_norm as the key, for every row of the golden table
            truth = {}
            for cells in golden.table[len(golden.table) - 96:]:
                raw, camera = cells[1], cells[2]
                truth.setdefault((normalize_plate(raw), camera), 0)
                truth[(normalize_plate(raw), camera)] += 1
            for r in rows["all"]:
                if r["plate_norm"] != normalize_plate(r["plate_raw"]):
                    problems.append("G3 plate_norm %r is not the key of plate_raw %r" % (r["plate_norm"], r["plate_raw"]))
                    break
            got = {(r["plate_norm"], r["camera_id"]): r["sightings"] for r in rows["visits"]}
            if got != truth:
                problems.append("G3 v_plate_visits differs from the counts of the written rows: %s vs %s" % (sorted(got.items())[:4], sorted(truth.items())[:4]))
            if sum(got.values()) != 96:
                problems.append("G3 v_plate_visits sums to %s, not 96" % sum(got.values()))
            # scope and role
            other = batch(db, dict(scope, **{"app.collection_id": "nobody"}), {"n": "SELECT count(*) AS n FROM analyst.v_plate_sightings"})["n"][0]["n"]
            unset = batch(db, {"TimeZone": "Asia/Karachi"}, {"n": "SELECT count(*) AS n FROM analyst.v_plate_sightings", "v": "SELECT count(*) AS n FROM analyst.v_plate_visits"})
            if other != 0 or unset["n"][0]["n"] != 0 or unset["v"][0]["n"] != 0:
                problems.append("G3 scope: another collection shows %s rows, no scope shows %s and %s" % (other, unset["n"][0]["n"], unset["v"][0]["n"]))
            try:
                db.psql("-At", "-c", "SET ROLE forensic_analyst_ro; SELECT count(*) FROM forensic.plate_sighting;")
                problems.append("G3 role: forensic_analyst_ro can read the table forensic.plate_sighting")
            except RuntimeError:
                pass
            print("views: %d sightings, %d plate-camera pairs; image, rejected, repeat and other-collection rows hidden; scope fails closed" % (rows["count"][0]["n"], len(rows["visits"])), flush=True)
        finally:
            left = db.stop()
            if left:
                print("WARNING: " + left)
    if problems:
        print("PLATEVIEWS=FAIL")
        for line in problems:
            print("  " + line)
        return 1
    print("PLATEVIEWS=PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
