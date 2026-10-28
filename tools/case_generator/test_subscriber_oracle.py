"""Gates G1, G3 and G5 (the database part) of docs/data/subscriber-device-family-20261028.md on a THROWAWAY PostgreSQL: generated cases WITH registration files and a device reference are written by the
PRODUCTION typed ingest and reference loader, and the three views of migration 028 equal the generator's oracle. No model, no real data.

    python -u -X utf8 tools/case_generator/test_subscriber_oracle.py                 # development case 2, the quick check
    python -u -X utf8 tools/case_generator/test_subscriber_oracle.py --cases 1,2,3   # chosen development cases
    python -u -X utf8 tools/case_generator/test_subscriber_oracle.py --all           # all eight development cases

What it proves, through the read-only analyst role and the settings the application sets:
  * the migration: ops/028 preflight, apply, verify, apply again, rollback, apply again, verify;
  * G1: both registration exports are recognised as subscriber-s1 and written to subscriber_registration, rows loaded equal the oracle's unique rows plus the planted repeat, the quarantined rows are the
    planted ones, the flags are the planted ones and no other (every loaded row, every identity column: identity_dropped), a second ingest writes nothing; the device reference loads with the planted
    counts (a padded TAC, two rejects, a repeat, a conflict) and a second load writes nothing;
  * G3: v_subscriber_registrations equals the oracle row for row (number, SIM, status, validity, source), `conflicts` equals the oracle's set, v_number_registration_activity equals it per number
    (events, outside every registration, after the last end, first, last), the numbers of every SIM and the registrations valid at a time equal it, v_handsets equals the handsets of the call records with
    the make and model of the device reference, the unregistered subjects are the planted ones, another collection or no scope shows nothing, the role cannot read the tables;
  * fixtures: a registration file with no dates (and no zone) is written with an empty time basis and shows in the views; handsets of 14 and 16 digits resolve to the listed TAC, an unlisted TAC and a
    non-numeric IMEI do not, two sources for one TAC give the first source, another tenant's list is not visible;
  * G5a: after everything is loaded, a dump of the whole throwaway database contains no synthetic name, national ID number or address of the registration files.
"""
import argparse
import collections
import hashlib
import os
import sys
import tempfile

import case_set
import generate
import load_case
from ingestion.forensic_records import reference_loader as rl
from ingestion.forensic_records import typed_ingest as ti
from ingestion.forensic_records.layout_registry import Policy, load_registry
from subscriber_case import Oracle
from test_colocation_views import batch
from test_ingest_v2 import PsqlSink, register_evidence

MIG = "/tmp/mig/"
MIGRATIONS = ("016_effective_records.sql", "018_effective_ipdr.sql", "017_analyst_views.sql", "019_observed_cell_positions.sql", "020_contact_links_and_inventory.sql", "021_case_record.sql",
              "024_colocation_and_meetings.sql", "025_successor_candidates.sql", "026_movement_and_rhythm.sql", "027_plate_sightings.sql")
OPS = "ops/028_subscriber_device_views/"
LOCAL = "'YYYY-MM-DD HH24:MI:SS'"
ZONE = "Asia/Karachi"
POLICY = Policy(source_tz=ZONE, tz_basis="assumed_by_profile")
POLICY_SUBSCRIBERS = Policy(source_tz=ZONE, tz_basis="assumed_by_profile", date_order="DMY")   # the job's date order: two planted dates are dd/mm/yyyy
SOURCE = "synthetic-test"
IDENTITY_FLAGS = ("identity_dropped:person_name", "identity_dropped:national_id", "identity_dropped:address")
CASE_FIXTURE = "subscriber-fixture"

REGS = ("SELECT source_file, msisdn, imsi, iccid, status, operator_label, service_type, to_char(valid_from AT TIME ZONE '%s', %s) AS vfrom, to_char(valid_to AT TIME ZONE '%s', %s) AS vto, conflicts "
        "FROM analyst.v_subscriber_registrations" % (ZONE, LOCAL, ZONE, LOCAL))
ACTIVITY = ("SELECT msisdn, registrations, sources, conflicts, events, events_outside_registration AS outside, events_after_last_end AS after, "
            "to_char(first_event_utc AT TIME ZONE '%s', %s) AS first_local, to_char(last_event_utc AT TIME ZONE '%s', %s) AS last_local "
            "FROM analyst.v_number_registration_activity ORDER BY msisdn" % (ZONE, LOCAL, ZONE, LOCAL))
HANDSETS = ("SELECT src, imei, imsi, events, to_char(first_seen AT TIME ZONE '%s', %s) AS first_local, to_char(last_seen AT TIME ZONE '%s', %s) AS last_local, tac, make, model, listed_by, tac_listed "
            "FROM analyst.v_handsets ORDER BY src, imei, imsi" % (ZONE, LOCAL, ZONE, LOCAL))


def k(value):
    return "" if value is None else str(value)


def ingest_all(db, root, registry, sink, register=True):
    """Register every export as evidence, then give each to the typed ingest with the policy of its kind (the registration exports carry dd/mm/yyyy dates: the job says day first)."""
    key = register_evidence(db, root) if register else None
    summaries = {}
    import json
    with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
        key = json.load(handle)
    for item, family, name in load_case.export_list(key):
        rel = item["file"]
        evidence_id, batch_id = load_case.lineage_ids(key["case_id"], rel)
        lineage = ti.Lineage(load_case.TENANT, key["case_id"], evidence_id, rel, batch_id)
        summaries[name] = ti.ingest_file(os.path.join(root, *rel.split("/")), lineage, POLICY_SUBSCRIBERS if family in ("subscriber", "transaction") else POLICY, sink, registry=registry)
    return key, summaries


def register(db, case_id, name, content):
    evidence_id, batch_id = load_case.lineage_ids(case_id, name)
    db.script(load_case.evidence_statement(evidence_id, case_id, name, "csv", len(content), hashlib.sha256(content).hexdigest()))
    return ti.Lineage(load_case.TENANT, case_id, evidence_id, name, batch_id)


def rehearse_migration(db):
    db.psql("-f", MIG + OPS + "preflight.sql")
    db.psql("-f", MIG + "028_subscriber_device_views.sql")
    db.psql("-f", MIG + OPS + "verify.sql")
    db.psql("-f", MIG + "028_subscriber_device_views.sql")
    db.psql("-f", MIG + OPS + "verify.sql")
    db.psql("-f", MIG + OPS + "rollback.sql")
    gone = db.sql("SELECT to_regclass('analyst.v_subscriber_registrations') IS NULL AND to_regclass('analyst.v_number_registration_activity') IS NULL AND to_regclass('analyst.v_handsets') IS NULL")
    assert gone == "t", "rollback left a view"
    assert db.sql("SELECT is_nullable FROM information_schema.columns WHERE table_schema = 'forensic' AND table_name = 'subscriber_registration' AND column_name = 'time_basis'") == "NO", \
        "rollback did not restore the time basis rule"
    db.psql("-f", MIG + "028_subscriber_device_views.sql")
    db.psql("-f", MIG + OPS + "verify.sql")


def fixture_checks(db, registry, tmp, problems):
    """Registrations with no dates and no zone, and the handset join with several TACs, two sources and another tenant's list."""
    header = ["MSISDN", "Subscriber ID", "Subscriber Name", "CNIC", "Address", "IMSI", "ICCID", "Status", "Activation Date", "Deactivation Date", "Service Type", "Operator"]
    rows = [["923000090001", "SUB-F1", "Synthetic Person 9001", "00000-0090001-1", "1 Synthetic Street, Test City", "001010000090001", "8900000000000900010", "Active", "", "", "Prepaid", "Operator 1"],
            ["923000090002", "SUB-F2", "Synthetic Person 9002", "00000-0090002-2", "2 Synthetic Street, Test City", "001010000090002", "", "Suspended", "", "", "Postpaid", "Operator 2"]]
    path = os.path.join(tmp, "no-dates.csv")
    rl_text = ",".join(header) + "\n" + "\n".join(",".join('"%s"' % c for c in r) for r in rows) + "\n"
    open(path, "w", encoding="utf-8", newline="").write(rl_text)
    lineage = register(db, CASE_FIXTURE, "no-dates.csv", rl_text.encode("utf-8"))
    sink = PsqlSink(db)
    out = ti.ingest_file(path, lineage, Policy(), sink, registry=registry)["sheets"][0]
    if (out["loaded"], out["inserted"]) != (2, 2) or out["flags"].get("source_timezone_unknown"):
        problems.append("fixture no dates: %s" % {x: out[x] for x in ("status", "loaded", "inserted", "flags", "quarantined", "reason")})
    basis = db.rows("SELECT time_basis, valid_from, source_tz FROM forensic.subscriber_registration WHERE collection_id = %s" % load_case.sql_text(CASE_FIXTURE))
    if any(r["time_basis"] is not None or r["valid_from"] is not None or r["source_tz"] is not None for r in basis) or len(basis) != 2:
        problems.append("fixture no dates: rows were written with a time basis or a validity: %s" % basis)
    scope = {"TimeZone": ZONE, "app.tenant_id": load_case.TENANT, "app.collection_id": CASE_FIXTURE}
    shown = batch(db, scope, {"r": "SELECT msisdn, status, valid_from, valid_to, conflicts FROM analyst.v_subscriber_registrations ORDER BY msisdn",
                              "a": "SELECT msisdn, registrations, events, events_outside_registration AS outside, events_after_last_end AS after FROM analyst.v_number_registration_activity ORDER BY msisdn"})
    if [(r["msisdn"], r["status"], r["valid_from"], r["valid_to"], r["conflicts"]) for r in shown["r"]] != [("923000090001", "active", None, None, False), ("923000090002", "suspended", None, None, False)]:
        problems.append("fixture no dates: the view shows %s" % shown["r"])
    if [(r["msisdn"], r["registrations"], r["events"], r["outside"], r["after"]) for r in shown["a"]] != [("923000090001", 1, 0, 0, 0), ("923000090002", 1, 0, 0, 0)]:
        problems.append("fixture no dates: the activity view shows %s" % shown["a"])

    # handsets
    call_header = ["MSISDN", "call_org_num", "CALL_DIALED_NUM", "IMSI", "IMEI", "CALL_START_DT_TM", "CALL_END_DT_TM", "INBOUND_OUTBOUND_IND", "Call_Network_Volume", "Lac_Id", "Site_Id", "Cell_SITE_ID",
                   "lat", "longitude", "CALL_TYPE", "location", "Description", "MACAddress"]
    imeis = {"923000090001": "00290611123456", "923000090002": "0029061112345601", "923000090003": "00860000123456", "923000090004": "ABCDEFGHIJKLMN"}
    lines = []
    for n, (number, imei) in enumerate(imeis.items()):
        row = [number, "", "923000090009", "00101000009%04d" % n, imei, "2026-03-01 10:0%d:00" % n, "2026-03-01 10:0%d:30" % n, "OUTGOING", "0", "1001", "1", "2002", "", "", "CALL", "Somewhere", "", ""]
        lines.append(",".join('"%s"' % c for c in row))
    text = ",".join(call_header) + "\n" + "\n".join(lines) + "\n"
    calls = os.path.join(tmp, "calls.csv")
    open(calls, "w", encoding="utf-8", newline="").write(text)
    lineage = register(db, CASE_FIXTURE, "calls.csv", text.encode("utf-8"))
    ti.ingest_file(calls, lineage, Policy(source_tz=ZONE, tz_basis="declared"), sink, registry=registry)
    for source, make, model in (("src-b", "BETAMAKE", "BETA ONE"), ("src-a", "ALPHAMAKE", "ALPHA ONE, A1")):
        p = os.path.join(tmp, "tac-%s.csv" % source)
        open(p, "w", encoding="utf-8", newline="").write("Brand,TAC,SPECS\n%s,00290611,\"%s\"\n" % (make, model))
        rl.load_tac_list(p, sink, source=source, source_license="test data", tenant_id=load_case.TENANT)
    p = os.path.join(tmp, "tac-other.csv")
    open(p, "w", encoding="utf-8", newline="").write("Brand,TAC,SPECS\nOTHERTENANT,00860000,SHOULD NOT SHOW\n")
    rl.load_tac_list(p, sink, source="src-x", source_license="test data", tenant_id="another-tenant")
    got = batch(db, scope, {"h": HANDSETS})["h"]
    want = [("923000090001", "00290611123456", "00290611", "ALPHAMAKE", "ALPHA ONE, A1", "src-a", True),
            ("923000090002", "0029061112345601", "00290611", "ALPHAMAKE", "ALPHA ONE, A1", "src-a", True),
            ("923000090003", "00860000123456", "00860000", None, None, None, False)]
    have = [(r["src"], r["imei"], r["tac"], r["make"], r["model"], r["listed_by"], r["tac_listed"]) for r in got]
    if have != want:
        problems.append("fixture handsets: %s" % have)


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
            rehearse_migration(db)
            print("migration 028: preflight, apply, verify, apply again, rollback, apply again, verify: OK", flush=True)
            fixture_checks(db, registry, tmp, problems)
            print("fixtures: registrations without dates, handsets with several TACs, two sources, another tenant: %s" % ("OK" if not problems else "PROBLEMS"), flush=True)
            sink = PsqlSink(db)
            identity_tokens = set()
            for seed in wanted:
                params = case_set.params_for(seed, table[seed])
                root = generate.write_case(params, tmp, with_subscribers=True)
                key, summaries = ingest_all(db, root, registry, sink)
                truth = key["subscriber_truth"]
                label = "case %d" % seed
                registrations = truth["registrations"]
                # G1: the registration exports
                for item in key["subscribers"]:
                    sheet = summaries[item["name"]]["sheets"][0]
                    source = item["source"]
                    mine = [r for r in registrations if r["source"] == source]
                    repeats = truth["planted"]["duplicate_rows"] if source == "A" else 0
                    if sheet["layout_id"] != "subscriber-s1" or sheet["table"] != "subscriber_registration":
                        problems.append("%s %s: read as %s into %s" % (label, item["name"], sheet["layout_id"], sheet["table"]))
                    if sheet["loaded"] != len(mine) + repeats:
                        problems.append("%s %s: %d rows loaded, the truth is %d unique + %d repeats" % (label, item["name"], sheet["loaded"], len(mine), repeats))
                    quarantined = {x.split(":", 1)[1]: v for x, v in sheet["quarantined"].items()}
                    wanted_q = truth["planted"]["quarantined"] if source == "A" else {}
                    if quarantined != wanted_q:
                        problems.append("%s %s: quarantined %s, planted %s" % (label, item["name"], quarantined, wanted_q))
                    expected_flags = {f: sheet["loaded"] for f in IDENTITY_FLAGS}
                    if repeats:
                        expected_flags["duplicate_row"] = repeats
                    dormant = sum(1 for r in mine if r["word"] == "Dormant")
                    if dormant:
                        expected_flags["unmapped_label:status"] = dormant
                    if sheet["flags"] != expected_flags:
                        problems.append("%s %s: flags %s, expected %s" % (label, item["name"], sheet["flags"], expected_flags))
                    totals["registration_rows"] += sheet["loaded"]
                again = ingest_all(db, root, registry, sink, register=False)[1]
                for item in key["subscribers"]:
                    if again[item["name"]]["rows_inserted"] != 0:
                        problems.append("%s: a second ingest of %s inserted %d rows" % (label, item["name"], again[item["name"]]["rows_inserted"]))
                # the device reference
                reference_path = os.path.join(root, *truth["reference_file"].split("/"))
                counts = rl.load_tac_list(reference_path, sink, source=SOURCE + "-%d" % seed, source_license="test data", tenant_id=load_case.TENANT)
                planted = truth["device_reference"]["planted"]
                if (counts["flags"], counts["rejected"], counts["repeats"], counts["conflicts"]) != ({"tac_padded": planted["tac_padded"]}, planted["rejected"], planted["repeats"], planted["conflicts"]) \
                        or counts["rows_read"] != truth["device_reference"]["rows_in_file"] or counts["inserted"] != counts["accepted"]:
                    problems.append("%s: the device reference loaded as %s" % (label, {x: counts[x] for x in ("rows_read", "accepted", "inserted", "flags", "rejected", "repeats", "conflicts")}))
                if rl.load_tac_list(reference_path, sink, source=SOURCE + "-%d" % seed, source_license="test data", tenant_id=load_case.TENANT)["inserted"] != 0:
                    problems.append("%s: a second load of the device reference wrote rows" % label)
                # G3
                case_id = key["case_id"]
                scope = {"TimeZone": ZONE, "app.tenant_id": load_case.TENANT, "app.collection_id": case_id}
                events = {}
                import json
                world = json.load(open(os.path.join(root, "key", "world.json"), encoding="utf-8"))
                subjects = {p["pid"]: p["msisdn"] for p in world["persons"] if p["role"] == "subject"}
                call = batch(db, scope, {"e": "SELECT src, to_char(start_utc AT TIME ZONE '%s', %s) AS t FROM analyst.v_comm_events ORDER BY src, start_utc" % (ZONE, LOCAL)})["e"]
                for r in call:
                    events.setdefault(r["src"], []).append(r["t"])
                oracle = Oracle(truth, events)
                got = batch(db, scope, {"regs": REGS, "act": ACTIVITY, "hand": HANDSETS,
                                        "unreg": "SELECT DISTINCT e.src FROM analyst.v_comm_events e WHERE NOT EXISTS (SELECT 1 FROM analyst.v_subscriber_registrations r WHERE r.msisdn = e.src) ORDER BY 1"})
                file_source = {item["file"]: item["source"] for item in key["subscribers"]}
                have = sorted((r["msisdn"], k(r["vfrom"]), file_source[r["source_file"]], r["imsi"], k(r["iccid"]), r["status"], k(r["vto"]), r["operator_label"], r["service_type"]) for r in got["regs"])
                want = sorted((t["msisdn"], k(t["from"]), t["source"], t["imsi"], k(t["iccid"]), t["status"], k(t["to"]), t["operator"], t["service"]) for t in registrations)
                if have != want:
                    problems.append("%s: v_subscriber_registrations differs from the truth: %d rows against %d (extra %s, missing %s)" %
                                    (label, len(have), len(want), sorted(set(have) - set(want))[:2], sorted(set(want) - set(have))[:2]))
                if sorted({r["msisdn"] for r in got["regs"] if r["conflicts"]}) != oracle.conflicting_numbers():
                    problems.append("%s: the conflicting numbers differ from the oracle: %s against %s" % (label, sorted({r["msisdn"] for r in got["regs"] if r["conflicts"]}), oracle.conflicting_numbers()))
                activity = {r["msisdn"]: r for r in got["act"]}
                if sorted(activity) != oracle.registered_numbers():
                    problems.append("%s: v_number_registration_activity covers other numbers than the registered ones" % label)
                for msisdn in oracle.registered_numbers():
                    r = activity.get(msisdn)
                    wanted_a = oracle.activity(msisdn)
                    regs = oracle.registrations(msisdn)
                    if r is None or (r["events"], r["outside"], r["after"], r["first_local"], r["last_local"]) != wanted_a or r["registrations"] != len(regs) \
                            or r["sources"] != len({x["source"] for x in regs}):
                        problems.append("%s: the activity of %s is %s, the oracle says %s" % (label, msisdn, None if r is None else (r["events"], r["outside"], r["after"], r["first_local"], r["last_local"], r["registrations"], r["sources"]),
                                                                                   (wanted_a, len(regs))))
                    totals["numbers"] += 1
                after = {m: r["after"] for m, r in activity.items() if r["after"]}
                if after != oracle.after_the_end():
                    problems.append("%s: the numbers with events after their last registration ended are %s, the oracle says %s" % (label, after, oracle.after_the_end()))
                if not after:
                    problems.append("%s: no number is active after its deactivation: the plant is missing" % label)
                # the SIMs and the registrations valid at a time
                imsis = sorted({t["imsi"] for t in registrations})
                queries = {"i%d" % n: "SELECT DISTINCT msisdn FROM analyst.v_subscriber_registrations WHERE imsi = %s ORDER BY 1" % load_case.sql_text(imsi) for n, imsi in enumerate(imsis)}
                times = {}
                for msisdn in oracle.registered_numbers()[:40]:
                    sample = sorted(events.get(msisdn, []))
                    if sample:
                        for n, moment in enumerate((sample[0], sample[len(sample) // 2], sample[-1])):
                            times[(msisdn, n)] = moment
                for (msisdn, n), moment in times.items():
                    queries["t|%s|%d" % (msisdn, n)] = ("SELECT status, imsi FROM analyst.v_subscriber_registrations WHERE msisdn = %s AND (valid_from IS NULL OR valid_from <= timestamp '%s' AT TIME ZONE '%s') "
                                                         "AND (valid_to IS NULL OR timestamp '%s' AT TIME ZONE '%s' < valid_to) ORDER BY 1, 2" % (load_case.sql_text(msisdn), moment, ZONE, moment, ZONE))
                shown = batch(db, scope, queries)
                for n, imsi in enumerate(imsis):
                    if [r["msisdn"] for r in shown["i%d" % n]] != oracle.numbers_of_imsi(imsi):
                        problems.append("%s: the numbers of a SIM differ from the oracle" % label)
                for (msisdn, n), moment in times.items():
                    have_t = sorted((r["status"], r["imsi"]) for r in shown["t|%s|%d" % (msisdn, n)])
                    want_t = sorted((r["status"], r["imsi"]) for r in oracle.registered_at(msisdn, moment))
                    if have_t != want_t:
                        problems.append("%s: the registration of %s at %s is %s, the oracle says %s" % (label, msisdn, moment, have_t, want_t))
                    totals["valid_at"] += 1
                # handsets
                want_h = []
                for h in truth["handsets"]:
                    listed = truth["device_reference"]["listed"].get(h["imei"][:8])
                    want_h.append((h["msisdn"], h["imei"], h["imsi"], h["events"], h["first"], h["last"], h["imei"][:8], None if listed is None else listed["make"], None if listed is None else listed["model"],
                                   listed is not None))
                have_h = [(r["src"], r["imei"], r["imsi"], r["events"], r["first_local"], r["last_local"], r["tac"], r["make"], r["model"], r["tac_listed"]) for r in got["hand"]]
                if sorted(have_h, key=lambda x: tuple(map(k, x))) != sorted(want_h, key=lambda x: tuple(map(k, x))):
                    problems.append("%s: v_handsets differs from the oracle: %d rows against %d" % (label, len(have_h), len(want_h)))
                models = sorted({r["model"] for r in got["hand"] if r["model"]})
                mq = batch(db, scope, {"m%d" % n: "SELECT DISTINCT src FROM analyst.v_handsets WHERE model = %s ORDER BY 1" % load_case.sql_text(m) for n, m in enumerate(models)})
                for n, m in enumerate(models):
                    if [r["src"] for r in mq["m%d" % n]] != oracle.numbers_using(m):
                        problems.append("%s: the numbers using %r differ from the oracle" % (label, m))
                with_events = sorted(msisdn for msisdn, ts in events.items() if ts)
                if [r["src"] for r in got["unreg"]] != oracle.unregistered(with_events):
                    problems.append("%s: the unregistered subjects are %s, the oracle says %s" % (label, [r["src"] for r in got["unreg"]], oracle.unregistered(with_events)))
                if len(oracle.unregistered(with_events)) != len(truth["plants"]["s4"]):
                    problems.append("%s: the planted unregistered subjects are not the ones without registrations" % label)
                # scope fails closed
                nobody = batch(db, dict(scope, **{"app.collection_id": "nobody"}), {v: "SELECT count(*) AS n FROM analyst.%s" % v for v in ("v_subscriber_registrations", "v_number_registration_activity")})
                unset = batch(db, {"TimeZone": ZONE}, {v: "SELECT count(*) AS n FROM analyst.%s" % v for v in ("v_subscriber_registrations", "v_number_registration_activity", "v_handsets")})
                if any(x[0]["n"] for x in list(nobody.values()) + list(unset.values())):
                    problems.append("%s: scope fails open" % label)
                for item in key["subscribers"]:
                    identity_tokens.add(item["file"])
                print("%s: %d registration rows loaded (%d unique), %d numbers, %d handsets: checked" % (label, sum(summaries[i["name"]]["sheets"][0]["loaded"] for i in key["subscribers"]), len(registrations),
                                                                                                         len(oracle.registered_numbers()), len(truth["handsets"])), flush=True)
            # the role cannot read the tables
            for table in ("subscriber_registration", "subscriber_identity", "device_tac"):
                try:
                    db.psql("-At", "-c", "SET ROLE forensic_analyst_ro; SELECT count(*) FROM forensic.%s" % table)
                    problems.append("the analyst role can read forensic.%s" % table)
                except RuntimeError as error:
                    if "permission denied" not in str(error):
                        problems.append("forensic.%s: unexpected error %s" % (table, str(error)[:120]))
            if db.sql("SELECT count(*) FROM forensic.subscriber_identity") != "0":
                problems.append("rows were written to forensic.subscriber_identity")
            # G5a: no identity value in the whole database
            dump = db.docker("exec", db.name, "pg_dump", "-U", "localrecall", "-d", "localrecall", "--data-only").stdout.decode("utf-8", "replace")
            leaks = [t for t in ("Synthetic Person", "Synthetic Street", "00000-00") if t in dump]
            print("G5a: a dump of the database (%d bytes) contains %d of the identity markers" % (len(dump), len(leaks)), flush=True)
            if leaks:
                problems.append("G5a: identity markers found in the database: %s" % leaks)
        finally:
            left = db.stop()
            if left:
                print("WARNING: " + left)
    print("checked: %d registration rows, %d numbers, %d registrations valid at a time" % (totals["registration_rows"], totals["numbers"], totals["valid_at"]))
    if problems:
        print("SUBSCRIBERORACLE=FAIL")
        for line in problems[:30]:
            print("  " + line)
        return 1
    print("SUBSCRIBERORACLE=PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
