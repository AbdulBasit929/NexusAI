"""Ingest v2 against a real database: a generated case goes through the PRODUCTION module (ingestion/forensic_records/typed_ingest.py) into a throwaway
PostgreSQL, and the same SQL checks that prove the reference loader (load_case.py) are run on what the production code wrote.  Plain unittest, about two minutes
(skipped when Docker or the image is absent):

    python -X utf8 tools/case_generator/test_ingest_v2.py            (the tests)
    python -X utf8 tools/case_generator/test_ingest_v2.py --case DIR  (one whole case, every check printed)

What the reference loader proves is that the typed schema answers the questions of the key. What this proves is that the worker's code writes the same rows:
the same row hashes, the same counts, the same flags, so that the same answers follow. The rows reach the database through the SQL the worker uses (the stage table,
COPY, INSERT ... ON CONFLICT DO NOTHING); only the connection differs (psql in the container instead of psycopg), and that last step is proved by running the
module inside the worker image (check_psycopg_sink.py; docs/data/ingest-v2.md section 6).
"""
import hashlib
import json
import os
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import audit  # noqa: E402  (also puts the repository and the vault tools on the path)
import generate  # noqa: E402
import load_case  # noqa: E402
import world as w  # noqa: E402
from ingestion.forensic_records import typed_ingest as ti  # noqa: E402
from ingestion.forensic_records.layout_registry import Policy, interpret_rows, load_registry  # noqa: E402
from test_load_case import docker_ready  # noqa: E402


NEWLINE = chr(10)


class PsqlSink:
    """The worker's sink, over psql in the throwaway container: the same stage table, COPY and conflict-ignoring INSERT, from the same SQL builders."""

    def __init__(self, db):
        self.db, self._columns, self.batches = db, {}, 0

    def table_columns(self, table):
        if table not in self._columns:
            self._columns[table] = [(r["column_name"], r["udt_name"]) for r in self.db.rows(ti.COLUMNS_QUERY % load_case.sql_text(table))]
        return self._columns[table]

    def insert_batch(self, table, columns, rows):
        udts = dict(self.table_columns(table))
        script = "\n".join(["BEGIN;", "SELECT set_config('app.tenant_id', %s, true);" % load_case.sql_text(rows[0]["tenant_id"]),
                            ti.stage_sql(table, columns) + ";", ti.copy_sql(columns) + ";"]) + "\n"
        script += ti.copy_text(columns, udts, rows) + "\\.\n" + ti.insert_sql(table, columns) + ";\nCOMMIT;\n"
        out = self.db.psql("-At", "-f", "-", input=script.encode("utf-8"))
        self.batches += 1
        return int([line for line in out.splitlines() if line.strip()][-1])


def register_evidence(db, case_dir):
    """The evidence items the typed rows point at (their foreign key), one per export of the case."""
    with open(os.path.join(case_dir, "key", "key.json"), encoding="utf-8") as handle:
        key = json.load(handle)
    with open(os.path.join(case_dir, "manifest.json"), encoding="utf-8") as handle:
        manifest = json.load(handle)
    statements = ["SELECT set_config('app.tenant_id', %s, false);" % load_case.sql_text(load_case.TENANT)]
    for item, _, _ in load_case.export_list(key):
        rel = item["file"]
        evidence_id, _ = load_case.lineage_ids(key["case_id"], rel)
        statements.append(load_case.evidence_statement(evidence_id, key["case_id"], os.path.basename(rel), os.path.splitext(rel)[1].lstrip("."),
                                                       manifest["files"][rel]["bytes"], manifest["files"][rel]["sha256"]))
    db.script(NEWLINE.join(statements))
    return key


def stats_from_summaries(key, summaries):
    """The loader's per-export statistics, from the quality summaries typed_ingest returns."""
    stats = {}
    for item, family, name in load_case.export_list(key):
        evidence_id, _ = load_case.lineage_ids(key["case_id"], item["file"])
        loaded = [s for s in summaries[name]["sheets"] if s["status"] == "loaded"]
        stats[name] = {"name": name, "family": family, "evidence_id": evidence_id, "table": load_case.TABLE_OF[family], "read": sum(s["rows_read"] for s in loaded),
                       "loaded": sum(s["loaded"] for s in loaded), "quarantined": sum(sum(s["quarantined"].values()) for s in loaded),
                       "skipped": sum(sum(s["skipped"].values()) for s in loaded), "duplicates": sum(s["duplicates_in_file"] for s in loaded)}
    return stats


def ingest_case(db, case_dir, registry, sink, policy=audit.POLICY, file_prefix="", register=True):
    """Register the evidence items, then give every export of the case to typed_ingest. Returns (key, per-export statistics in the loader's shape, summaries)."""
    if register:
        key = register_evidence(db, case_dir)
    else:
        with open(os.path.join(case_dir, "key", "key.json"), encoding="utf-8") as handle:
            key = json.load(handle)
    summaries = {}
    for item, family, name in load_case.export_list(key):
        rel = item["file"]
        evidence_id, batch_id = load_case.lineage_ids(key["case_id"], rel)
        lineage = ti.Lineage(load_case.TENANT, key["case_id"], evidence_id, file_prefix + rel, batch_id)
        summaries[name] = ti.ingest_file(os.path.join(case_dir, *rel.split("/")), lineage, policy, sink, registry=registry)
    return key, stats_from_summaries(key, summaries), summaries


def reference_row_hashes(case_dir, key, registry):
    """The row hashes the reference loader would write, from the loader's own formula over the key's layout: {export name: set of hashes}."""
    wanted = {}
    for item, _, name in load_case.export_list(key):
        rel = item["file"]
        rows = audit.read_export(os.path.join(case_dir, *rel.split("/")))
        report = registry.find_header(rows)
        entry = registry.get(item["layout_id"])
        hashes = set()
        for result in interpret_rows(entry, rows[report.header_row], rows[report.header_row + 1:], policy=audit.POLICY):
            if result.status == "accepted":
                content = load_case.content_of(result.record)
                hashes.add(hashlib.sha256(("%s|%s|%s" % (rel, result.record["source_row"], content)).encode("utf-8")).hexdigest())
        wanted[name] = hashes
    return wanted


@unittest.skipUnless(docker_ready(), "Docker or the image %s is not available on this machine" % load_case.IMAGE)
class IngestV2Tests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.params = w.Params(seed=61, people=60, subjects=6, associates=12, period_days=20, p2_pairs=3, p3_pairs=2, p7_plants=3)
        cls.root = generate.write_case(cls.params, cls.tmp.name)
        cls.registry = load_registry()
        cls.db = load_case.Throwaway()
        try:
            cls.db.start()
            load_case.prepare(cls.db, cls.registry)
            cls.sink = PsqlSink(cls.db)
            cls.key, cls.stats, cls.summaries = ingest_case(cls.db, cls.root, cls.registry, cls.sink)
            cls.report = load_case.run_checks(cls.db, cls.key, cls.stats)
        except Exception:
            cls.db.stop()
            cls.tmp.cleanup()
            raise

    @classmethod
    def tearDownClass(cls):
        cls.left = cls.db.stop()
        cls.tmp.cleanup()
        if cls.left:
            raise AssertionError(cls.left)

    def count(self, table, where="true"):
        return int(self.db.sql("SELECT count(*) FROM forensic.%s WHERE %s" % (table, where)))

    def test_every_check_passes_on_what_the_production_code_wrote(self):
        failed = ["%s: %s" % (name, detail) for name, ok, detail in self.report.items if not ok]
        self.assertEqual(failed, [])
        self.assertGreater(len(self.report.items), 50)

    def test_the_registry_chose_the_layout_the_key_names_for_every_export(self):
        for item, _, name in load_case.export_list(self.key):
            sheets = [s for s in self.summaries[name]["sheets"] if s["status"] == "loaded"]
            self.assertEqual([s["layout_id"] for s in sheets], [item["layout_id"]], name)
            self.assertTrue(self.summaries[name]["reconciles"], name)

    def test_the_row_hashes_are_exactly_the_reference_loaders(self):
        wanted = reference_row_hashes(self.root, self.key, self.registry)
        for name, hashes in wanted.items():
            s = self.stats[name]
            got = {r["row_hash"] for r in self.db.rows("SELECT row_hash FROM forensic.%s WHERE evidence_id = %s" % (s["table"], load_case.sql_text(s["evidence_id"])))}
            self.assertEqual(got, hashes, name)

    def test_the_rows_carry_their_lineage_and_point_at_registered_layouts(self):
        for table in load_case.TABLE_OF.values():
            total = self.count(table)
            self.assertGreater(total, 0, table)
            self.assertEqual(self.count(table, "tenant_id = 'case' AND file_id <> '' AND source_row >= 1 AND length(row_hash) = 64 AND length(content_hash) = 64"), total, table)
            self.assertEqual(self.count(table, "layout_id IN (SELECT layout_id FROM forensic.layout_versions)"), total, table)
            self.assertEqual(self.count(table, "batch_id IN (%s)" % ", ".join(load_case.sql_text(load_case.lineage_ids(self.key["case_id"], i["file"])[1])
                                                                          for i, _, _ in load_case.export_list(self.key))), total, table)

    def test_a_second_run_of_the_whole_case_writes_nothing_and_the_answers_stay_right(self):
        before = {t: self.count(t) for t in load_case.TABLE_OF.values()}
        key, stats, summaries = ingest_case(self.db, self.root, self.registry, self.sink, register=False)   # the evidence items are already there
        self.assertEqual({t: self.count(t) for t in load_case.TABLE_OF.values()}, before)
        for name, out in summaries.items():
            self.assertEqual(out["rows_inserted"], 0, name)
            loaded = [s for s in out["sheets"] if s["status"] == "loaded"]
            self.assertEqual([s["already_present"] for s in loaded], [s["loaded"] for s in loaded], name)
        self.assertEqual([n for n, ok, _ in load_case.run_checks(self.db, key, stats).items if not ok], [])

    def test_a_job_without_a_declared_zone_writes_no_row_at_all(self):
        before = {t: self.count(t) for t in load_case.TABLE_OF.values()}
        _, stats, summaries = ingest_case(self.db, self.root, self.registry, self.sink, policy=Policy(), file_prefix="nozone/", register=False)
        self.assertEqual({t: self.count(t) for t in load_case.TABLE_OF.values()}, before)
        self.assertEqual(sum(s["loaded"] for s in stats.values()), 0)
        reasons = {r for out in summaries.values() for s in out["sheets"] for r in s["quarantined"]}
        self.assertIn("quarantined:source_timezone_unknown", reasons)

    def test_zz_the_checks_notice_when_the_production_rows_are_damaged(self):
        """Runs last: removes one subject's rows from one export and requires the checks to fail."""
        s = self.stats[self.key["subjects"][0]["person"]]
        self.db.sql("DELETE FROM forensic.%s WHERE evidence_id = %s AND source_row <= 3" % (s["table"], load_case.sql_text(s["evidence_id"])))
        report = load_case.run_checks(self.db, self.key, self.stats)
        self.assertTrue(report.failed, "deleting three rows must fail at least one reconciliation or answer check")


def run(case_dir):
    """The whole case through the production module into a throwaway database, then every SQL check (what the unit tests do on a small case)."""
    registry = load_registry()
    db = load_case.Throwaway()
    try:
        db.start()
        load_case.prepare(db, registry)
        key, stats, summaries = ingest_case(db, case_dir, registry, PsqlSink(db))
        report = load_case.run_checks(db, key, stats)
        loaded = {t: int(db.sql("SELECT count(*) FROM forensic.%s" % t)) for t in load_case.TABLE_OF.values()}
        return report, loaded, summaries
    finally:
        left = db.stop()
        if left:
            print("WARNING: " + left)


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "--case":
        result, rows_loaded, _ = run(sys.argv[2])
        for item_name, item_ok, item_detail in result.items:
            print("%s  %-46s %s" % ("ok  " if item_ok else "FAIL", item_name, item_detail[:160]))
        print("loaded rows: %s" % rows_loaded)
        print("INGESTV2=%s checks=%d failed=%d" % ("FAIL" if result.failed else "PASS", len(result.items), len(result.failed)))
        sys.exit(1 if result.failed else 0)
    unittest.main()
