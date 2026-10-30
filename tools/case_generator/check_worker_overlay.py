#!/usr/bin/env python3
"""Proves a worker overlay IMAGE end to end, on throwaway databases, before it is allowed near the live one.

    python -X utf8 tools/case_generator/check_worker_overlay.py [--image nexusai-forensic-records-worker:families-20261030] [--seed 5001] [--keep-going]

A generated case with every family (calls, internet records, app usage, camera sightings, registrations and devices, statements as CSV, PDF and Word, a wallet list, fourteen documents) is put
through two databases that both have migrations 001 to 031:

  reference  the direct path the test-suite uses (the production typed ingest and the production mention indexer called from Python on the host);
  overlay    the image's own `worker.process_job` over every file of the case (the evidence registered as the API registers it, the metadata as the API sets it), in a container on the
             throwaway's network, with the switches the live worker would have (FORENSIC_INGEST_V2=shadow, FORENSIC_DOCUMENT_MENTIONS=true, the Lahore gazetteer).

It then compares what the two databases hold: for each typed table the set of row content hashes, and the mention index of the fourteen documents tuple by tuple. Equal means the image does with
the worker's real code path exactly what the tested modules do. Differences are printed, never hidden. The image is only run (never started as a service, pushed or tagged); the throwaway
containers and their volumes are removed at the end. Nothing here reads a real file or reaches a network.
"""
import argparse
import dataclasses
import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import check_ask_engine as cae  # noqa: E402
import generate  # noqa: E402
import load_case  # noqa: E402
import world as w  # noqa: E402
from ingestion.forensic_records.layout_registry import load_registry  # noqa: E402

VIEW_MIGRATIONS = ("017_analyst_views.sql", "018_effective_ipdr.sql", "019_observed_cell_positions.sql", "020_contact_links_and_inventory.sql", "021_case_record.sql",
                   "024_colocation_and_meetings.sql", "025_successor_candidates.sql", "026_movement_and_rhythm.sql", "027_plate_sightings.sql", "028_subscriber_device_views.sql",
                   "030_transaction_views.sql", "031_document_mentions.sql")
TYPED = {"comm_event": "start_utc", "data_session": "start_utc", "app_usage": None, "plate_sighting": None, "subscriber_registration": None, "bank_transaction": "txn_utc"}
TIME_POLICY = {"source_timezone": "Asia/Karachi", "source_timezone_state": "profile_defaulted", "allow_profile_timezone_default": "true",   # the policy of the direct path: assumed_by_profile
               "source_date_order": "DMY", "source_date_order_state": "analyst_confirmed"}   # the uploader declares day-first for the case, as the direct path does for the files that need it
DOCUMENT_SUFFIXES = {".txt", ".docx", ".pdf"}


def prepare_overlay_database(db, registry):
    db.start()
    load_case.prepare(db, registry)
    for name in VIEW_MIGRATIONS:
        db.psql("-f", "/tmp/mig/" + name)


def register_everything(db, root, key, manifest):
    """The evidence items the API would have registered for every file of the case (the trigger of migration 008 creates the version), and a processing run for each document."""
    statements = ["SELECT set_config('app.tenant_id', %s, false);" % load_case.sql_text(load_case.TENANT)]
    plan = []
    document_names = {os.path.basename(e["file"]) for e in key.get("documents", [])}
    for rel, info in sorted(manifest["files"].items()):
        if rel.startswith("key/") or rel == "manifest.json":
            continue
        evidence_id, job_id = load_case.lineage_ids(key["case_id"], rel)
        name, suffix = os.path.basename(rel), os.path.splitext(rel)[1].lower()
        document = suffix in DOCUMENT_SUFFIXES
        if document:
            statements.append(
                "INSERT INTO forensic.evidence_items (evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension, size_bytes, sha256, "
                "modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata) VALUES (%s, %s, %s, %s, 'case-loader', %s, %s, "
                "'application/octet-stream', %s, %d, %s, 'document', 'document', 1, 'document_native_text', 'queued', %s, jsonb_build_object('request_id', 'case-load'));"
                % (load_case.sql_text(evidence_id), load_case.sql_text(load_case.TENANT), load_case.sql_text(key["case_id"]), load_case.sql_text("SYNTHETIC-" + key["case_id"]),
                   load_case.sql_text(name), load_case.sql_text(name), load_case.sql_text(suffix.lstrip(".")), info["bytes"], load_case.sql_text(info["sha256"]),
                   load_case.sql_text("immutable://case/sha256/" + info["sha256"])))
            statements.append(
                "INSERT INTO forensic.processing_runs (run_id, tenant_id, collection_id, evidence_id, version_id, run_kind, pipeline_id, idempotency_key, status, input_sha256) "
                "SELECT %s::uuid, tenant_id, collection_id, evidence_id, current_version_id, 'extract', 'bounded-native-document-extractor', %s, 'running', sha256 "
                "FROM forensic.evidence_items WHERE evidence_id = %s::uuid;" % (load_case.sql_text(job_id), load_case.sql_text("overlay/" + evidence_id), load_case.sql_text(evidence_id)))
            # what the live API sends (read from the jobs of the first live load, 2026-10-30): a Word or PDF file is a document; a plain .txt DOCUMENT is called structured_records although its route is the
            # document route (the old rule of the worker, modality in {document, text}, then sent it to the generic row adapter: no passages, no mention index); a provider's .txt export is text
            if suffix == ".txt":
                modality = "structured_records" if name in document_names else "text"
            else:
                modality = "document"
            metadata = {"evidence_modality": modality, "evidence_processing_route": "native_document_worker"}
        else:
            statements.append(load_case.evidence_statement(evidence_id, key["case_id"], name, suffix.lstrip("."), info["bytes"], info["sha256"]))
            metadata = {"evidence_modality": "tabular" if suffix in (".xlsx", ".xls") else "structured_records", "evidence_processing_route": "forensic_records_worker"}
        metadata.update(TIME_POLICY, size_bytes=str(info["bytes"]))
        plan.append({"rel": rel, "name": name, "job_id": job_id, "evidence_id": evidence_id, "sha256": info["sha256"], "metadata": metadata, "document": document})
    db.script("\n".join(statements))
    return plan


def run_probe(db, image, root, plan_jobs, key, gazetteer_in_image):
    with tempfile.TemporaryDirectory() as plan_dir:
        with open(os.path.join(plan_dir, "plan.json"), "w", encoding="utf-8") as handle:
            json.dump({"tenant": load_case.TENANT, "case_id": key["case_id"], "spool": "/spool", "jobs": plan_jobs}, handle)
        command = ["docker", "run", "--rm", "--network", "container:" + db.name, "--memory", "2g", "--tmpfs", "/tmp",
                   "-v", root + ":/spool:ro", "-v", HERE + ":/probe:ro", "-v", plan_dir + ":/plan:ro",
                   "-e", "PLAN=/plan/plan.json", "-e", "PYTHONDONTWRITEBYTECODE=1",
                   "-e", "FORENSIC_DATABASE_URL=postgresql://localrecall@127.0.0.1:5432/localrecall", "-e", "FORENSIC_SPOOL_DIR=/spool",
                   "-e", "FORENSIC_INGEST_V2=shadow", "-e", "FORENSIC_DOCUMENT_MENTIONS=true", "-e", "FORENSIC_DOCUMENT_GAZETTEER=" + gazetteer_in_image,
                   "--entrypoint", "python", image, "/probe/overlay_probe.py"]
        proc = subprocess.run(command, capture_output=True)
        if proc.returncode != 0:
            raise RuntimeError("the probe failed (%d): %s" % (proc.returncode, proc.stderr.decode("utf-8", "replace")[-2500:]))
        return json.loads(proc.stdout.decode("utf-8").strip().splitlines()[-1])


# columns that say HOW a row arrived (keys, lineage, bookkeeping), not WHAT it says
BOOKKEEPING = ("event_id", "session_id", "usage_id", "sighting_id", "registration_id", "txn_id", "file_id", "batch_id", "record_id", "record_ts", "row_hash", "content_hash",
               "raw_payload", "raw_payload_redacted", "ingested_at", "derived_artifact_id")


def typed_rows(db, table):
    """({(evidence_id, source_row): the row without its bookkeeping columns}, {evidence_id: file name}) for one typed table."""
    drop = ",".join(load_case.sql_text(c) for c in BOOKKEEPING)
    rows = db.rows("SELECT t.evidence_id::text AS ev, t.source_row AS sr, e.original_filename AS fn, (to_jsonb(t) - ARRAY[%s]) AS j FROM forensic.%s t "
                   "JOIN forensic.evidence_items e ON e.evidence_id = t.evidence_id" % (drop, table))
    return {(row["ev"], row["sr"]): row["j"] for row in rows}, {row["ev"]: row["fn"] for row in rows}


def compare_table(ref, ov):
    """(same, report lines): the rows of both sides paired by (evidence, source row); what is only on one side, by file; which columns differ in the pairs."""
    (ref_rows, ref_names), (ov_rows, ov_names) = ref, ov
    names = dict(ref_names, **ov_names)
    lines, only_ref, only_ov = [], sorted(set(ref_rows) - set(ov_rows)), sorted(set(ov_rows) - set(ref_rows))
    differing, examples = {}, {}
    for key in set(ref_rows) & set(ov_rows):
        for column in sorted(set(ref_rows[key]) | set(ov_rows[key])):
            if ref_rows[key].get(column) != ov_rows[key].get(column):
                differing[column] = differing.get(column, 0) + 1
                examples.setdefault(column, (ref_rows[key].get(column), ov_rows[key].get(column)))
    for label, only in (("reference", only_ref), ("overlay", only_ov)):
        by_file = {}
        for evidence, _ in only:
            by_file[names.get(evidence, evidence)] = by_file.get(names.get(evidence, evidence), 0) + 1
        if only:
            lines.append("   %d rows only in the %s: %s" % (len(only), label, dict(sorted(by_file.items()))))
    for column, count in sorted(differing.items()):
        lines.append("   column %s differs in %d rows, e.g. reference %r, overlay %r" % (column, count, str(examples[column][0])[:70], str(examples[column][1])[:70]))
    return not (only_ref or only_ov or differing), lines


def mention_tuples(db, names):
    quoted = ",".join(load_case.sql_text(n) for n in names)
    rows = db.rows("SELECT e.original_filename AS document, m.kind, coalesce(m.value, '') AS value, m.char_start, m.char_end, m.tier FROM forensic.document_mention m "
                   "JOIN forensic.evidence_items e ON e.tenant_id = m.tenant_id AND e.collection_id = m.collection_id AND e.evidence_id = m.evidence_id "
                   "WHERE e.original_filename IN (%s) ORDER BY 1, 4, 5, 2" % quoted)
    return [tuple(row.values()) for row in rows]


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--image", default="nexusai-forensic-records-worker:families-20261030")
    parser.add_argument("--seed", type=int, default=5001)
    parser.add_argument("--gazetteer", default="/app/gazetteers/lahore.json", help="the path of the places file INSIDE the image")
    args = parser.parse_args(argv)
    registry = load_registry()
    tmp = tempfile.TemporaryDirectory()
    port = cae.free_port()
    ref = load_case.Throwaway(publish_port=port)
    ov = load_case.Throwaway()
    try:
        params = dataclasses.replace(w.Params(**dict(cae.PARAMS, seed=args.seed)), locale="lahore")
        root = generate.write_case(params, tmp.name, with_anpr=True, with_subscribers=True, with_transactions=True, with_documents=True, with_statements=True)
        with open(os.path.join(root, "manifest.json"), encoding="utf-8") as handle:
            manifest = json.load(handle)
        with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
            key = json.load(handle)
        document_names = [os.path.basename(e["file"]) for e in key["documents"]]
        print("case %s: %d files, %d documents, %d statement documents" % (key["case_id"], len(manifest["files"]), len(document_names), len(key.get("statements", []))))

        cae.build_case_database(ref, root, registry, port)
        print("reference database built (direct path)")

        prepare_overlay_database(ov, registry)
        plan = register_everything(ov, root, key, manifest)
        # device reference lists are tenant-wide reference data, not an upload of the case: the reference database has it from its loader, the overlay database gets the same from the same loader
        if key.get("subscriber_truth"):
            from ingestion.forensic_records import reference_loader as rl
            from test_ingest_v2 import PsqlSink
            reference = os.path.join(root, *key["subscriber_truth"]["reference_file"].split("/"))
            rl.load_tac_list(reference, PsqlSink(ov), source="synthetic-test", source_license="test data", tenant_id=load_case.TENANT)
            plan = [p for p in plan if p["rel"] != key["subscriber_truth"]["reference_file"]]
        result = run_probe(ov, args.image, root, plan, key, args.gazetteer)
        print("image: hook present %s, switches %s" % (result["hook_present"], result["switches"]))
        failed = [j for j in result["jobs"] if not j["ok"]]
        print("process_job: %d jobs, %d failed" % (len(result["jobs"]), len(failed)))
        for job in failed:
            print("  FAILED %s: %s" % (job["name"], job["error"]))

        problems = [("process_job failed for " + j["name"]) for j in failed]
        print("\nwhat the typed writer recorded in the audit log, for every sheet that was not loaded in full:")
        audit = ov.rows("SELECT file_id, details->'typed_ingest' AS s FROM forensic.records_audit_log WHERE action = 'records.ingest.typed_shadow' ORDER BY file_id")
        shown = 0
        for row in audit:
            summary = row["s"] or {}
            for sheet in summary.get("sheets", []):
                if sheet.get("status") != "loaded" or sheet.get("quarantined") or sheet.get("skipped"):
                    shown += 1
                    print("   %-52s %s %s loaded=%s quarantined=%s skipped=%s" % (row["file_id"].split("/")[-1][:52], sheet.get("status"), sheet.get("layout_id"), sheet.get("loaded"), sheet.get("quarantined"), sheet.get("skipped")))
        print("   (%d audit rows, %d sheets listed)" % (len(audit), shown))
        print("\n%-26s %-10s %-10s" % ("typed table", "reference", "overlay"))
        statement_documents = {os.path.basename(s["file"]): s["account"] for s in key.get("statements", [])}
        for table in TYPED:
            mine, theirs = typed_rows(ref, table), typed_rows(ov, table)
            if table == "bank_transaction":
                # the PDF and Word renditions of statements are not read by the direct path; they are checked against the CSV rows of their account below
                theirs = ({k: v for k, v in theirs[0].items() if theirs[1][k[0]] not in statement_documents}, theirs[1])
            same, lines = compare_table(mine, theirs)
            print("%-26s %-10d %-10d %s" % (table, len(mine[0]), len(theirs[0]), "the same rows, column by column" if same else "DIFFERENT"))
            for line in lines:
                print(line)
            if not same:
                problems.append("%s: the rows differ" % table)
        print("\nthe statements read from PDF and Word, against the CSV rows of the same account (the statement database-level check):")
        columns = "to_char(txn_utc AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS') AS at, direction, amount::text AS amount, coalesce(balance_after::text, '') AS balance"
        for name, account in sorted(statement_documents.items()):
            # an exact repeat of a row inside a file is kept and flagged duplicate_row, and the analyst views leave it out: so does this check
            csv_rows = ref.rows("SELECT %s FROM forensic.bank_transaction WHERE account_norm = %s AND NOT ('duplicate_row' = ANY (quality_flags))" % (columns, load_case.sql_text(account)))
            left_out = ref.sql("SELECT count(*) FROM forensic.bank_transaction WHERE account_norm = %s AND 'duplicate_row' = ANY (quality_flags)" % load_case.sql_text(account))
            doc_rows = ov.rows("SELECT %s FROM forensic.bank_transaction t JOIN forensic.evidence_items e ON e.evidence_id = t.evidence_id WHERE e.original_filename = %s "
                               "AND NOT ('duplicate_row' = ANY (t.quality_flags))"
                               % (columns.replace("txn_utc", "t.txn_utc").replace("direction", "t.direction").replace("amount::", "t.amount::").replace("balance_after", "t.balance_after"), load_case.sql_text(name)))
            as_set = lambda rows: sorted(tuple(r.values()) for r in rows)   # noqa: E731
            same_rows = as_set(csv_rows) == as_set(doc_rows)
            print("   %-22s account %s: %d rows in the file, %d CSV rows of the account (%s flagged repeats left out): %s"
                  % (name, account[-4:], len(doc_rows), len(csv_rows), left_out, "the same rows (time, direction, amount, balance)" if same_rows else "DIFFERENT"))
            if not same_rows:
                problems.append("%s: its rows differ from the CSV rows of its account" % name)
                for row in sorted(set(as_set(csv_rows)) ^ set(as_set(doc_rows)))[:5]:
                    print("      only on one side:", row)
        mine, theirs = mention_tuples(ref, document_names), mention_tuples(ov, document_names)
        print("\nmention index of the %d documents: reference %d tuples, overlay %d tuples: %s" % (len(document_names), len(mine), len(theirs), "identical" if mine == theirs else "DIFFERENT"))
        if mine != theirs:
            problems.append("the mention tuples of the documents differ")
            for row in sorted(set(mine) ^ set(theirs))[:8]:
                print("   only on one side:", row)
        extra = ov.rows("SELECT e.original_filename, count(*) FROM forensic.document_mention m JOIN forensic.evidence_items e ON e.tenant_id = m.tenant_id AND e.collection_id = m.collection_id "
                        "AND e.evidence_id = m.evidence_id WHERE e.original_filename NOT IN (%s) GROUP BY 1 ORDER BY 1" % ",".join(load_case.sql_text(n) for n in document_names))
        print("mentions in the files that are not among the fourteen documents (overlay only): %s" % ([(r["original_filename"], r["count"]) for r in extra] or "none"))
        by_kind = ov.rows("SELECT kind, count(*) FROM forensic.document_mention GROUP BY 1 ORDER BY 1")
        print("overlay mention index by kind: %s" % {r["kind"]: r["count"] for r in by_kind})
        print("\nOVERLAY=%s" % ("PASS" if not problems else "FAIL"))
        for problem in problems:
            print("  - " + problem)
        return 0 if not problems else 1
    finally:
        for db in (ref, ov):
            left = db.stop()
            if left:
                print("WARNING: " + left)
        tmp.cleanup()


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
