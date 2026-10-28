"""Gates of stage S3b (programme section 10) on a THROWAWAY PostgreSQL: the documents of generated cases are kept as the worker keeps them, their mentions are indexed by the PRODUCTION code, and the
three views of migration 031 equal the generator's truth. No model, no real data.

    python -u -X utf8 tools/case_generator/test_doc_views.py                  # development case 2, the quick check
    python -u -X utf8 tools/case_generator/test_doc_views.py --cases 1,2,3    # chosen development cases
    python -u -X utf8 tools/case_generator/test_doc_views.py --all            # all eight development cases
    python -u -X utf8 tools/case_generator/test_doc_views.py --orion          # orion-lahore (seed 5001)

What it proves, through the read-only analyst role and the settings the application sets:
  * the migration: 031 passes its preflight, applies, verifies, applies again, rolls back (refused while a mention is stored, allowed when the table is empty) and applies again;
  * G-V1 the index: v_document_mentions equals the truth occurrence by occurrence (document, kind, normalised key), national ID numbers are counted per document and carry neither key nor text (no column
    of the table, no view row and no passage of the index holds one), v_document_values equals the truth per document, kind and key, a second write inserts nothing, a rejected mention is not shown, and a
    newer extractor revision replaces an older one;
  * G-M6 the links: v_document_links equals direct counts on the typed tables (calls by presence, registrations, wallet, statements and camera sightings by count), no key links to a family that does not hold
    it (zero false links) and none is missed; every key the truth ties to a person of the case is found where the truth says it must be (subscribers' numbers in the call records, accounts in the statements,
    plates of interest in the camera table);
  * scope and role: another collection or no scope shows nothing; the analyst role cannot read the table; the views hold no passage text and no name.
"""
import argparse
import collections
import dataclasses
import json
import os
import sys
import tempfile

import case_set
import generate
import load_case
from ingestion.forensic_records.layout_registry import load_registry
from load_documents import load_documents, LiteralCursor
from test_colocation_views import batch
from test_ingest_v2 import PsqlSink
from test_subscriber_oracle import ingest_all
import world as w

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "ask_eval"))
import doc_bank  # noqa: E402

MIG = "/tmp/mig/"
MIGRATIONS = ("016_effective_records.sql", "018_effective_ipdr.sql", "017_analyst_views.sql", "019_observed_cell_positions.sql", "020_contact_links_and_inventory.sql", "021_case_record.sql",
              "024_colocation_and_meetings.sql", "025_successor_candidates.sql", "026_movement_and_rhythm.sql", "027_plate_sightings.sql", "028_subscriber_device_views.sql",
              "030_transaction_views.sql")
ZONE = "Asia/Karachi"
VIEWS = ("v_document_mentions", "v_document_values", "v_document_links")
ORION = dict(seed=5001, people=100, subjects=12, associates=24, period_days=75, communities=5, p2_pairs=5, p3_pairs=2, p7_plants=3)
NOT_DUP = "NOT ('duplicate_row' = ANY(quality_flags))"


def rehearse_migration(db):
    db.psql("-f", MIG + "ops/031_document_mentions/preflight.sql")
    db.psql("-f", MIG + "031_document_mentions.sql")
    db.psql("-f", MIG + "ops/031_document_mentions/verify.sql")
    db.psql("-f", MIG + "031_document_mentions.sql")
    db.psql("-f", MIG + "ops/031_document_mentions/verify.sql")
    db.psql("-f", MIG + "ops/031_document_mentions/rollback.sql")
    assert db.sql("SELECT to_regclass('forensic.document_mention') IS NULL AND to_regclass('analyst.v_document_mentions') IS NULL AND to_regclass('analyst.v_document_links') IS NULL") == "t", "the rollback left an object"
    db.psql("-f", MIG + "031_document_mentions.sql")
    db.psql("-f", MIG + "ops/031_document_mentions/verify.sql")


def rollback_refused_when_a_mention_is_stored(db):
    try:
        db.psql("-f", MIG + "ops/031_document_mentions/rollback.sql")
    except RuntimeError as error:
        assert "holds rows" in str(error), str(error)[:200]
        assert db.sql("SELECT to_regclass('forensic.document_mention') IS NOT NULL AND to_regclass('analyst.v_document_mentions') IS NOT NULL") == "t", "the refused rollback changed something"
        return True
    return False


def direct(db, case_id, kind, value):
    """Records of the other families that carry the key, counted straight on the typed tables (not through the views). Calls are the target call records (a tower dump lists who was at a cell, it is not a call); they are compared by presence."""
    q = load_case.sql_text(value)
    scope = "tenant_id = %s AND collection_id = %s" % (load_case.sql_text(load_case.TENANT), load_case.sql_text(case_id))
    out = {}

    def count(sql):
        return int(db.sql(sql))
    if kind == "msisdn":
        out["calls"] = count("SELECT count(*) FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND (a_norm = %s OR b_norm = %s) AND %s" % (scope, q, q, NOT_DUP))
        out["registrations"] = count("SELECT count(*) FROM forensic.subscriber_registration WHERE %s AND msisdn = %s AND %s" % (scope, q, NOT_DUP))
        out["wallet"] = count("SELECT count(*) FROM forensic.bank_transaction WHERE %s AND counterparty_msisdn = %s AND %s" % (scope, q, NOT_DUP))
    elif kind == "account":
        out["statements"] = count("SELECT count(*) FROM forensic.bank_transaction WHERE %s AND (account_norm = %s OR counterparty_account_norm = %s) AND %s" % (scope, q, q, NOT_DUP))
    elif kind == "plate":
        out["sightings"] = count("SELECT count(*) FROM forensic.plate_sighting WHERE %s AND plate_norm = %s AND source_kind = 'csv' AND review_state <> 'rejected' AND %s" % (scope, q, NOT_DUP))
    elif kind == "iccid":
        out["registrations"] = count("SELECT count(*) FROM forensic.subscriber_registration WHERE %s AND iccid = %s AND %s" % (scope, q, NOT_DUP))
    elif kind == "imsi":
        out["registrations"] = count("SELECT count(*) FROM forensic.subscriber_registration WHERE %s AND imsi = %s AND %s" % (scope, q, NOT_DUP))
    elif kind == "imei":
        out["calls"] = count("SELECT count(*) FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND imei = %s AND %s" % (scope, q, NOT_DUP))
    return {family: n for family, n in out.items() if n}


def canonical(q):
    """The query an analyst would write over the views for a question of the bank (the engine's job is to find one like it); None for a question with no query (the overclaim probe)."""
    p, L, kind = q["expect"].get("params", {}), load_case.sql_text, q["type"]
    values = "analyst.v_document_values"
    if kind == "doc_mentions":
        return "SELECT DISTINCT document AS x FROM %s WHERE kind = %s AND value = %s ORDER BY 1" % (values, L(p["kind"]), L(p["value"]))
    if kind in ("doc_count", "doc_place_count"):
        return "SELECT count(DISTINCT document) AS x FROM %s WHERE kind = %s AND value = %s" % (values, L(p["kind"]), L(p["value"]))
    if kind == "doc_both":
        return ("SELECT DISTINCT a.document AS x FROM %s a JOIN %s b ON b.evidence_id = a.evidence_id WHERE a.kind = %s AND a.value = %s AND b.kind = %s AND b.value = %s ORDER BY 1"
                % (values, values, L(p["a"][0]), L(p["a"][1]), L(p["b"][0]), L(p["b"][1])))
    if kind == "doc_but_not":
        return ("SELECT DISTINCT a.document AS x FROM %s a LEFT JOIN %s b ON b.evidence_id = a.evidence_id AND b.kind = %s AND b.value = %s WHERE a.kind = %s AND a.value = %s AND b.evidence_id IS NULL ORDER BY 1"
                % (values, values, L(p["b"][0]), L(p["b"][1]), L(p["a"][0]), L(p["a"][1])))
    if kind == "doc_lists":
        return "SELECT DISTINCT value AS x FROM %s WHERE document = %s AND kind = 'msisdn' ORDER BY 1" % (values, L(p["document"]))
    if kind == "doc_case_numbers":
        return "SELECT DISTINCT n.value AS x FROM %s n WHERE n.kind = 'msisdn' AND EXISTS (SELECT 1 FROM analyst.v_comm_events e WHERE e.src = n.value) ORDER BY 1" % values
    if kind == "absent":
        return "SELECT value AS x FROM %s WHERE kind = %s AND value = %s" % (values, L(p["kind"]), L(p["value"]))
    if kind == "doc_national_kind":
        return "SELECT DISTINCT document AS x FROM analyst.v_document_mentions WHERE kind = 'national_id' ORDER BY 1"
    if kind == "control":
        return "SELECT mention_id AS x FROM analyst.v_document_mentions WHERE document = %s" % L(p["document"])
    return None


def check_bank(db, label, key, scope, problems, totals):
    """Every question of the documents bank, answered by its canonical query over the views, equals the oracle (the answer from the generator's truth)."""
    questions = doc_bank.bank_questions(key["document_truth"], key, key["seed"])
    sql = {q["id"]: canonical(q) for q in questions if canonical(q)}
    got = batch(db, scope, {i: s for i, s in sql.items()})
    for q in questions:
        if q["id"] not in sql:
            continue
        rows = got[q["id"]]
        expect = q["expect"]
        if expect["kind"] == "set":
            have, want = sorted(str(r["x"]) for r in rows), sorted(expect["values"])
        elif expect["kind"] == "scalar":
            have, want = int(rows[0]["x"]), expect["value"]
        else:
            have, want = len(rows), 0
        if have != want:
            problems.append("%s: %s (%s) answers %s from the views, the oracle says %s" % (label, q["id"], q["type"], str(have)[:80], str(want)[:80]))
        totals["bank_questions"] += 1


def check_case(db, label, root, key, stats, problems, totals):
    case_id = key["case_id"]
    truth = key["document_truth"]["documents"]
    names = {os.path.basename(e["file"]): e["name"] for e in key["documents"]}
    scope = {"TimeZone": ZONE, "app.tenant_id": load_case.TENANT, "app.collection_id": case_id}
    got = batch(db, scope, {"m": "SELECT document, kind, value, surface, tier, position_in_document FROM analyst.v_document_mentions",
                            "v": "SELECT document, kind, value, mentions FROM analyst.v_document_values",
                            "l": "SELECT kind, value, family, records FROM analyst.v_document_links"})
    # G-V1: the mentions equal the truth, occurrence by occurrence
    want = collections.Counter()
    nat = collections.Counter()
    for file_name, doc in names.items():
        for m in truth[doc]["mentions"]:
            if m["kind"] == "national_id":
                nat[file_name] += 1
            else:
                want[(file_name, m["kind"], m["value"])] += 1
    have = collections.Counter((r["document"], r["kind"], r["value"]) for r in got["m"] if r["kind"] != "national_id")
    if have != want:
        problems.append("%s: v_document_mentions differs from the truth: extra %s, missing %s" % (label, list((have - want).items())[:3], list((want - have).items())[:3]))
    have_nat = collections.Counter(r["document"] for r in got["m"] if r["kind"] == "national_id")
    if have_nat != nat:
        problems.append("%s: national ID numbers per document are %s, the truth says %s" % (label, dict(have_nat), dict(nat)))
    if any(r["value"] is not None or r["surface"] is not None for r in got["m"] if r["kind"] == "national_id"):
        problems.append("%s: a national ID number is shown" % label)
    raw_ids = [m["value"] for doc in truth.values() for m in doc["mentions"] if m["kind"] == "national_id"]
    for value in set(raw_ids):
        digits = value.replace("-", "")
        leaked = int(db.sql("SELECT count(*) FROM forensic.document_mention WHERE coalesce(value,'') LIKE %s OR coalesce(surface,'') LIKE %s OR locator::text LIKE %s" % ((load_case.sql_text("%" + digits[5:12] + "%"),) * 3)))
        if leaked:
            problems.append("%s: a national ID number is stored in the mention index" % label)
    for r in got["m"]:
        if r["tier"] not in ("high", "low") or r["position_in_document"] in (None, ""):
            problems.append("%s: a mention has no tier or no position" % label)
            break
    want_values = collections.Counter()
    for (file_name, kind, value), n in want.items():
        want_values[(file_name, kind, value)] = n
    have_values = collections.Counter({(r["document"], r["kind"], r["value"]): r["mentions"] for r in got["v"]})
    if have_values != want_values:
        problems.append("%s: v_document_values differs from the truth" % label)
    totals["mentions"] += sum(want.values()) + sum(nat.values())
    # the mention index is idempotent
    before = int(db.sql("SELECT count(*) FROM forensic.document_mention"))
    cursor = LiteralCursor(db)
    from ingestion.forensic_records import document_mentions
    for name, s in stats.items():
        document_mentions.CursorSink(cursor).write(load_case.TENANT, case_id, s["evidence_id"], s["rows"])
    cursor.flush()
    if int(db.sql("SELECT count(*) FROM forensic.document_mention")) != before:
        problems.append("%s: a second write inserted rows" % label)
    # a rejected mention is not shown; a newer revision replaces the older one for its passage
    first = db.rows("SELECT mention_id, artifact_id::text AS artifact_id, kind, value, surface, tier, rule, char_start, char_end, locator::text AS locator FROM forensic.document_mention WHERE kind <> 'national_id' AND collection_id = %s ORDER BY mention_id LIMIT 1" % load_case.sql_text(case_id))[0]
    db.sql("UPDATE forensic.document_mention SET review_state = 'rejected' WHERE mention_id = %s" % first["mention_id"])
    shown = batch(db, scope, {"c": "SELECT count(*) AS n FROM analyst.v_document_mentions WHERE mention_id = %s" % first["mention_id"]})["c"][0]["n"]
    if shown:
        problems.append("%s: a rejected mention is shown" % label)
    db.sql("UPDATE forensic.document_mention SET review_state = 'unreviewed' WHERE mention_id = %s" % first["mention_id"])
    other = int(db.sql("SELECT count(*) FROM forensic.document_mention WHERE artifact_id = %s::uuid" % load_case.sql_text(first["artifact_id"])))
    db.sql("INSERT INTO forensic.document_mention (tenant_id, collection_id, evidence_id, artifact_id, kind, value, surface, tier, rule, char_start, char_end, locator, extractor, extractor_revision) "
           "SELECT tenant_id, collection_id, evidence_id, artifact_id, kind, value, surface, tier, rule, char_start, char_end, locator, extractor, 'v9' FROM forensic.document_mention WHERE mention_id = %s" % first["mention_id"])
    ids = db.sql("SELECT string_agg(mention_id::text, ',') FROM forensic.document_mention WHERE artifact_id = %s::uuid" % load_case.sql_text(first["artifact_id"]))
    newest = batch(db, scope, {"c": "SELECT count(*) AS n FROM analyst.v_document_mentions m WHERE m.mention_id IN (%s)" % ids})["c"][0]["n"]
    if newest != 1:
        problems.append("%s: with a newer extractor revision for a passage, %d of its mentions are shown instead of the newer revision's 1" % (label, newest))
    db.sql("DELETE FROM forensic.document_mention WHERE extractor_revision = 'v9'")
    if other != int(db.sql("SELECT count(*) FROM forensic.document_mention WHERE artifact_id = %s::uuid" % load_case.sql_text(first["artifact_id"]))):
        problems.append("%s: the test left a stray row" % label)
    # G-M6: the links
    have_links = {(r["kind"], r["value"], r["family"]): r["records"] for r in got["l"]}
    named = sorted({(kind, value) for (_, kind, value) in want if kind not in ("place",)})
    expected = {}
    for kind, value in named:
        for family, n in direct(db, case_id, kind, value).items():
            expected[(kind, value, family)] = n
    false_links = [k for k in have_links if k not in expected]
    missed = [k for k in expected if k not in have_links]
    wrong_count = [(k, have_links[k], expected[k]) for k in expected if k in have_links and k[2] != "calls" and have_links[k] != expected[k]]
    if false_links or missed or wrong_count:
        problems.append("%s: links differ from direct counts: false %s, missed %s, wrong counts %s" % (label, false_links[:3], missed[:3], wrong_count[:3]))
    totals["named_keys"] += len(named)
    totals["links"] += len(expected)
    # what the truth ties to the case must be found where it belongs
    subjects = {s["msisdn"] for s in key["subjects"]}
    registered = {r["msisdn"] for r in key["subscriber_truth"]["registrations"]}
    accounts = {t["account"] for t in key["transaction_truth"]["transactions"]}
    plates = set(key["anpr"]["of_interest"])
    for (file_name, kind, value) in want:
        if kind == "msisdn" and value in subjects and (kind, value, "calls") not in have_links:
            problems.append("%s: a subscriber's number named in %s is not linked to the call records" % (label, file_name))
        if kind == "msisdn" and value in registered and (kind, value, "registrations") not in have_links:
            problems.append("%s: a registered number named in %s is not linked to the registrations" % (label, file_name))
        if kind == "account" and value in accounts and (kind, value, "statements") not in have_links:
            problems.append("%s: an account named in %s is not linked to the statements" % (label, file_name))
        if kind == "plate" and value in plates and (kind, value, "sightings") not in have_links:
            problems.append("%s: a plate named in %s is not linked to the camera table" % (label, file_name))
    check_bank(db, label, key, scope, problems, totals)
    # scope and what a view may hold
    nobody = batch(db, dict(scope, **{"app.collection_id": "nobody"}), {v: "SELECT count(*) AS n FROM analyst.%s" % v for v in VIEWS})
    unset = batch(db, {"TimeZone": ZONE}, {v: "SELECT count(*) AS n FROM analyst.%s" % v for v in VIEWS})
    if any(x[0]["n"] for x in list(nobody.values()) + list(unset.values())):
        problems.append("%s: scope fails open" % label)
    text = int(db.sql("SELECT count(*) FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'analyst' AND c.relname IN ('v_document_mentions','v_document_values','v_document_links') "
                      "AND a.attnum > 0 AND NOT a.attisdropped AND a.attname IN ('text','passage','snippet','metadata','name','locator')"))
    if text:
        problems.append("%s: a document view has a column that could hold passage text or a name" % label)
    print("%s: %d mentions indexed in %d documents (%d national ID numbers counted, none stored), %d named keys, %d links checked against direct counts" % (
        label, sum(s["mentions"] for s in stats.values()), len(stats), sum(nat.values()), len(named), len(expected)), flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--cases", default="2")
    parser.add_argument("--all", action="store_true")
    parser.add_argument("--orion", action="store_true")
    args = parser.parse_args()
    table = {seed: overrides for kind, seed, overrides in case_set.CASES if kind == "development"}
    wanted = [] if (args.orion and args.cases == "2") else (list(range(1, 9)) if args.all else [int(n) for n in args.cases.split(",")])
    cases = [("case %d" % seed, dataclasses.replace(case_set.params_for(seed, table[seed]), locale="lahore")) for seed in wanted]
    if args.orion:
        cases.append(("orion-lahore", w.Params(**dict(ORION, locale="lahore"))))
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
            print("migration 031: preflight, apply twice, verify, rollback (empty), apply again: OK", flush=True)
            sink = PsqlSink(db)
            for label, params in cases:
                root = generate.write_case(params, tmp, with_anpr=True, with_subscribers=True, with_transactions=True, with_documents=True)
                key, summaries = ingest_all(db, root, registry, sink)
                stats = load_documents(db, root, key, key["document_truth"]["gazetteer"])
                check_case(db, label, root, key, stats, problems, totals)
            if not rollback_refused_when_a_mention_is_stored(db):
                problems.append("the rollback ran with mentions stored")
            try:
                db.psql("-At", "-c", "SET ROLE forensic_analyst_ro; SELECT count(*) FROM forensic.document_mention")
                problems.append("the analyst role can read forensic.document_mention")
            except RuntimeError as error:
                if "permission denied" not in str(error):
                    problems.append("unexpected error reading the table as the analyst role: %s" % str(error)[:160])
        finally:
            left = db.stop()
            if left:
                print("container left behind: %s" % left)
    print("checked: %s" % dict(totals))
    for p in problems:
        print("PROBLEM:", p)
    print("DOCVIEWS=%s" % ("PASS" if not problems else "FAIL (%d problems)" % len(problems)))
    return 0 if not problems else 1


if __name__ == "__main__":
    sys.exit(main())
