"""Migration 018, the effective internet-record view, on a THROWAWAY PostgreSQL with a generated case written by the PRODUCTION typed ingest.
Plain unittest, about two minutes (skipped when Docker or the image is absent):

    python -X utf8 tools/case_generator/test_effective_ipdr.py

What it proves:
  * the preflight, verify and rollback scripts pass on a database built the way a fresh one is, the migration applies twice, rolls back and forward;
  * the view has exactly the columns and types of forensic.records (verify.sql) and returns exactly the non-repeated typed sessions;
  * the lane's own way of reading the family (fields pulled out of raw_payload by their semantic-layer names) gives, for every subscriber, the typed table's session
    count, byte total (up + down) and distinct host count;
  * the semantic names are really there: subscriber_id, msisdn, timestamp, session_start, source_ip, destination_ip, bytes, domain;
  * an evidence item with no typed rows keeps its version-1 rows (so an old case is unchanged), and one with typed rows is not counted twice;
  * the application role sees its tenant's rows and nobody else's.
"""
import os
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import generate  # noqa: E402
import load_case  # noqa: E402
import world as w  # noqa: E402
from ingestion.forensic_records.layout_registry import load_registry  # noqa: E402
from test_ingest_v2 import PsqlSink, ingest_case  # noqa: E402
from test_load_case import docker_ready  # noqa: E402

VIEW = "forensic.records_effective_ipdr"
OPS = "/tmp/mig/ops/018_effective_ipdr/"
MIGRATION = "/tmp/mig/018_effective_ipdr.sql"


@unittest.skipUnless(docker_ready(), "Docker or the image %s is not available on this machine" % load_case.IMAGE)
class EffectiveIpdrTests(unittest.TestCase):
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
            cls.key, cls.stats, cls.summaries = ingest_case(cls.db, cls.root, cls.registry, PsqlSink(cls.db))
            cls.db.psql("-f", OPS + "preflight.sql")
            cls.db.psql("-f", MIGRATION)
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

    def scope(self):
        return load_case.scope(self.key)

    def test_a_the_ops_scripts_pass_and_the_migration_applies_twice(self):
        self.db.psql("-f", OPS + "verify.sql")
        self.db.psql("-f", MIGRATION)
        self.db.psql("-f", OPS + "verify.sql")

    def test_b_the_view_is_exactly_the_non_repeated_typed_sessions(self):
        in_view = int(self.db.sql("SELECT count(*) FROM %s WHERE %s" % (VIEW, self.scope())))
        typed = int(self.db.sql("SELECT count(*) FROM forensic.data_session WHERE %s AND NOT ('duplicate_row' = ANY (quality_flags))" % self.scope()))
        self.assertGreater(typed, 0)
        self.assertEqual(in_view, typed)

    def test_c_the_lane_s_reading_of_each_subscriber_equals_the_typed_table(self):
        lane = self.db.rows(
            "SELECT raw_payload->>'msisdn' AS number, count(*) AS sessions, sum((raw_payload->>'bytes')::numeric) AS bytes, count(DISTINCT raw_payload->>'domain') AS domains "
            "FROM %s WHERE %s GROUP BY 1" % (VIEW, self.scope()))
        typed = self.db.rows(
            "SELECT msisdn AS number, count(*) AS sessions, sum(coalesce(bytes_up, 0) + coalesce(bytes_down, 0)) AS bytes, count(DISTINCT host) AS domains "
            "FROM forensic.data_session WHERE %s AND NOT ('duplicate_row' = ANY (quality_flags)) GROUP BY 1" % self.scope())
        as_map = lambda rows: {r["number"]: (r["sessions"], float(r["bytes"] or 0), r["domains"]) for r in rows}  # noqa: E731
        self.assertEqual(as_map(lane), as_map(typed))
        self.assertGreaterEqual(len(lane), len(self.key["sessions"]) - 1)

    def test_d_the_semantic_names_are_present_on_typed_rows(self):
        for name in ("subscriber_id", "msisdn", "timestamp", "session_start", "session_end", "source_ip", "destination_ip", "bytes", "bytes_up", "bytes_down", "domain", "protocol"):
            n = int(self.db.sql("SELECT count(*) FROM %s WHERE %s AND raw_payload ? %s" % (VIEW, self.scope(), load_case.sql_text(name))))
            self.assertGreater(n, 0, "no typed row of the generated case carries %s" % name)
        row = self.db.rows("SELECT raw_payload, metadata FROM %s WHERE %s AND raw_payload ? 'msisdn' LIMIT 1" % (VIEW, self.scope()))[0]
        payload = row["raw_payload"]
        self.assertEqual(payload["subscriber_id"], payload["msisdn"])
        self.assertRegex(payload["timestamp"], r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\+00:00$")
        self.assertEqual(row["metadata"]["typed_source"]["table"], "data_session")
        both = self.db.rows("SELECT raw_payload->>'bytes' AS b, raw_payload->>'bytes_up' AS u, raw_payload->>'bytes_down' AS d FROM %s WHERE %s AND raw_payload ? 'bytes_up' AND raw_payload ? 'bytes_down' LIMIT 20" % (VIEW, self.scope()))
        self.assertTrue(both)
        for r in both:
            self.assertEqual(int(r["b"]), int(r["u"]) + int(r["d"]))

    def test_e_an_item_without_typed_rows_keeps_its_version_1_rows_and_an_item_with_them_is_not_counted_twice(self):
        script = "\n".join([
            "BEGIN;", "SELECT set_config('app.tenant_id', 'ipdr-eff', true);",
            "INSERT INTO forensic.evidence_items (evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension, size_bytes, sha256, modality, "
            "detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata) SELECT ('38000000-0000-4000-8000-00000000000' || n)::uuid, 'ipdr-eff', 'ipdr-eff-c', "
            "'SYNTHETIC-EFF', 'u', 'f' || n || '.csv', 'f' || n || '.csv', 'text/csv', 'csv', 1, repeat(n::text, 64), 'structured_records', 'ipdr', 1, 'w', 'completed', 'immutable://x/' || n, "
            "jsonb_build_object('request_id', 'r') FROM generate_series(1, 2) n;",
            # evidence 1: two version-1 rows and no typed row; evidence 2: one version-1 row and one typed row (the typed one wins, the version-1 row is not added)
            "INSERT INTO forensic.records (record_id, tenant_id, collection_id, file_id, batch_id, record_type, \"timestamp\", primary_target, secondary_target, source_file, row_number, row_hash, "
            "raw_payload, metadata, evidence_id) SELECT gen_random_uuid(), 'ipdr-eff', 'ipdr-eff-c', 'f' || e || '.csv', '39000000-0000-4000-8000-000000000001', 'ipdr', now(), '923000000001', '198.51.100.1', "
            "'f' || e || '.csv', r, repeat((e * 10 + r)::text, 64), jsonb_build_object('msisdn', '923000000001', 'bytes', '10'), '{}'::jsonb, ('38000000-0000-4000-8000-00000000000' || e)::uuid "
            "FROM (VALUES (1, 1), (1, 2), (2, 1)) AS t(e, r);",
            "INSERT INTO forensic.data_session (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, layout_version, start_utc, source_tz, time_basis, "
            "msisdn, bytes_up, bytes_down, raw_payload) VALUES ('ipdr-eff', 'ipdr-eff-c', '38000000-0000-4000-8000-000000000002', 'f2.csv', '39000000-0000-4000-8000-000000000002', 0, repeat('a', 64), "
            "repeat('a', 64), 'ipdr-i1-jazz-ufdr', 1, '2026-03-01 10:00:00+00', 'Asia/Karachi', 'declared', '923000000002', 5, 7, '{}'::jsonb);",
            "SELECT evidence_id::text, count(*) FROM %s WHERE tenant_id = 'ipdr-eff' GROUP BY 1 ORDER BY 1;" % VIEW,
            "ROLLBACK;"])
        out = [line.strip() for line in self.db.psql("-At", "-f", "-", input=script.encode()).splitlines() if "|" in line]
        self.assertEqual(out, ["38000000-0000-4000-8000-000000000001|2", "38000000-0000-4000-8000-000000000002|1"])

    def test_f_the_application_role_sees_its_tenant_and_nobody_else(self):
        script = ("SET ROLE forensic_runtime;\nSELECT set_config('app.tenant_id', %s, false);\nSELECT count(*) FROM %s;\n"
                  "SELECT set_config('app.tenant_id', 'another-tenant', false);\nSELECT count(*) FROM %s;\n" % (load_case.sql_text(load_case.TENANT), VIEW, VIEW))
        counts = [line for line in self.db.psql("-At", "-f", "-", input=script.encode()).split() if line.isdigit()]
        total = int(self.db.sql("SELECT count(*) FROM %s WHERE %s" % (VIEW, self.scope())))
        self.assertEqual(counts[-2:], [str(int(self.db.sql("SELECT count(*) FROM %s WHERE tenant_id = %s" % (VIEW, load_case.sql_text(load_case.TENANT))))), "0"])
        self.assertGreater(total, 0)

    def test_z_rollback_and_forward_again(self):
        before = self.db.sql("SELECT count(*) FROM %s" % VIEW)
        self.db.psql("-f", OPS + "rollback.sql")
        self.assertEqual(self.db.sql("SELECT to_regclass('%s') IS NULL" % VIEW), "t")
        self.db.psql("-f", OPS + "preflight.sql")
        self.db.psql("-f", MIGRATION)
        self.db.psql("-f", OPS + "verify.sql")
        self.assertEqual(self.db.sql("SELECT count(*) FROM %s" % VIEW), before)


if __name__ == "__main__":
    unittest.main()
