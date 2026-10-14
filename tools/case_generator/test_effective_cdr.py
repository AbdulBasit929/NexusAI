"""Migration 016, the effective call-record view, on a THROWAWAY PostgreSQL with a generated case written by the PRODUCTION typed ingest.
Plain unittest, about three minutes (skipped when Docker or the image is absent):

    python -X utf8 tools/case_generator/test_effective_cdr.py

What it proves, beyond the synthetic checks of ops/016_effective_records/smoke.sql:
  * the preflight, verify and smoke scripts pass on a database built the way a fresh one is, and the migration applies twice and rolls back and forward;
  * the verify script FAILS when the view is damaged (a missing security_invoker, a missing column);
  * the question lane's own way of reading a family (fields pulled out of raw_payload by their semantic-layer names, time from timestamp) gives, for every
    subject of the generated case, the contact profile the key says (A1: the counterparties, their event counts, first and last time);
  * the view returns exactly the non-repeated target call records of the typed table, and no tower dump;
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

VIEW = "forensic.records_effective_cdr"
OPS = "/tmp/mig/ops/016_effective_records/"
LOCAL = "'YYYY-MM-DD HH24:MI:SS'"


def view_a1(key, msisdn):
    """The contact profile of one subject as the lane reads it: through the view's columns, with fields pulled out of the payload by their semantic-layer names."""
    return ("SELECT raw_payload->>'CALL_DIALED_NUM' AS number, count(*) AS events, "
            "to_char(min((raw_payload->>'CALL_START_DT_TM')::timestamptz) AT TIME ZONE 'Asia/Karachi', %s) AS first, "
            "to_char(max((raw_payload->>'CALL_START_DT_TM')::timestamptz) AT TIME ZONE 'Asia/Karachi', %s) AS last "
            "FROM %s WHERE %s AND raw_payload->>'MSISDN' = %s AND raw_payload->>'CALL_DIALED_NUM' IS NOT NULL GROUP BY 1"
            % (LOCAL, LOCAL, VIEW, load_case.scope(key), load_case.sql_text(msisdn)))


@unittest.skipUnless(docker_ready(), "Docker or the image %s is not available on this machine" % load_case.IMAGE)
class EffectiveCdrTests(unittest.TestCase):
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

    def run_ops(self, name):
        return self.db.psql("-f", OPS + name)

    def test_a_the_ops_scripts_pass_on_a_database_built_like_a_fresh_one(self):
        self.run_ops("preflight.sql")
        self.run_ops("verify.sql")
        self.run_ops("smoke.sql")

    def test_b_the_lane_s_reading_of_every_subject_matches_the_keys_contact_profile(self):
        for subject in self.key["subjects"]:
            rows = self.db.rows(view_a1(self.key, subject["msisdn"]))
            got = {r["number"]: (r["events"], r["first"], r["last"]) for r in rows}
            want = {r["number"]: (r["outgoing"] + r["incoming"] + r["unknown"], r["first"], r["last"]) for r in self.key["oracle"]["A1"][subject["msisdn"]]}
            missing = {n: want[n] for n in want if got.get(n) != want[n]}
            self.assertEqual(missing, {}, "%s: counterparties whose count or first and last time differ" % subject["person"])
            extra = set(got) - set(want)
            if extra:
                # a counterparty the key leaves out can only be a raw label that is neither a number nor a short code, sender id or service label
                kinds = {r["b_kind"] for r in self.db.rows(
                    "SELECT DISTINCT b_kind FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND a_norm = %s AND b_norm IS NULL AND coalesce(b_raw, '') IN (%s)"
                    % (load_case.scope(self.key), load_case.sql_text(subject["msisdn"]), ", ".join(load_case.sql_text(n) for n in extra)))}
                self.assertTrue(kinds <= {None, "unknown"}, "%s: unexplained extra counterparties %s of kinds %s" % (subject["person"], sorted(extra)[:5], kinds))

    def test_b2_the_view_removes_every_alias_the_semantic_layer_lists_for_the_fields_a_typed_row_owns(self):
        import re
        with open(os.path.join(load_case.REPO, "semantic_layer", "cdr.yaml"), encoding="utf-8") as handle:
            yaml_text = handle.read()
        with open(os.path.join(load_case.REPO, "db", "forensic_records", "016_effective_records.sql"), encoding="utf-8") as handle:
            sql_text = handle.read()
        removal = sql_text[sql_text.index("(e.raw_payload - ARRAY["):sql_text.index("|| o.overlay")]
        for field in ("cdr.msisdn", "cdr.dialed_number", "cdr.call_start", "cdr.call_end", "cdr.imei", "cdr.imsi"):
            block = yaml_text[yaml_text.index("id: " + field):]
            aliases = re.search(r"source_names: \[(.*?)\]", block).group(1).split(",")
            for alias in (a.strip() for a in aliases):
                self.assertIn("'%s'" % alias, removal, "%s: the alias %s is not removed from the payload of a typed row" % (field, alias))
        for field in ("cdr.originating_number", "cdr.cell_site_id", "cdr.location"):
            block = yaml_text[yaml_text.index("id: " + field):]
            first = re.search(r"source_names: \[(.*?)\]", block).group(1).split(",")[0].strip()
            self.assertNotIn("'%s'" % first, removal, "%s is not owned by the typed row and must stay in the payload" % field)

    def test_c_the_view_is_exactly_the_target_call_records_without_repeats(self):
        in_view = int(self.db.sql("SELECT count(*) FROM %s WHERE %s" % (VIEW, load_case.scope(self.key))))
        typed = int(self.db.sql("SELECT count(*) FROM forensic.comm_event WHERE %s AND dataset_kind = 'target_cdr' AND %s" % (load_case.scope(self.key), load_case.NOT_DUPLICATE)))
        self.assertGreater(typed, 0)
        self.assertEqual(in_view, typed)
        dumps = int(self.db.sql("SELECT count(*) FROM forensic.comm_event WHERE %s AND dataset_kind = 'tower_dump'" % load_case.scope(self.key)))
        self.assertGreater(dumps, 0, "the case has tower dumps, and none of them is in the view")
        self.assertEqual(int(self.db.sql("SELECT count(*) FROM %s v WHERE %s AND EXISTS (SELECT 1 FROM forensic.comm_event t WHERE t.event_id::text IS NOT NULL "
                                         "AND t.dataset_kind = 'tower_dump' AND t.evidence_id = v.evidence_id)" % (VIEW, load_case.scope(self.key)))), 0)

    def test_d_every_row_has_the_lineage_the_builders_read(self):
        row = self.db.rows("SELECT * FROM %s WHERE %s ORDER BY row_number LIMIT 1" % (VIEW, load_case.scope(self.key)))[0]
        self.assertEqual(sorted(row), sorted(["record_id", "tenant_id", "collection_id", "file_id", "batch_id", "record_type", "timestamp", "primary_target",
                                              "secondary_target", "source_file", "row_number", "row_hash", "raw_payload", "metadata", "ingested_at", "evidence_id"]))
        self.assertEqual(row["record_type"], "cdr")
        self.assertTrue(row["source_file"].endswith((".xlsx", ".csv", ".xls", ".txt")))
        nulls = int(self.db.sql("SELECT count(*) FROM %s WHERE %s AND (record_id IS NULL OR source_file IS NULL OR row_number IS NULL OR \"timestamp\" IS NULL OR evidence_id IS NULL)"
                                % (VIEW, load_case.scope(self.key))))
        self.assertEqual(nulls, 0)
        self.assertEqual(int(self.db.sql("SELECT count(*) - count(DISTINCT record_id) FROM %s WHERE %s" % (VIEW, load_case.scope(self.key)))), 0, "record ids are unique")

    def test_e_the_application_role_sees_its_tenant_and_nobody_else(self):
        script = ("SET ROLE forensic_runtime;\nSELECT set_config('app.tenant_id', %s, false);\nSELECT count(*) FROM %s;\n"
                  "SELECT set_config('app.tenant_id', 'another-tenant', false);\nSELECT count(*) FROM %s;\n" % (load_case.sql_text(load_case.TENANT), VIEW, VIEW))
        out = self.db.psql("-At", "-f", "-", input=script.encode()).split()
        counts = [line for line in out if line.isdigit()]
        total = int(self.db.sql("SELECT count(*) FROM %s WHERE %s" % (VIEW, load_case.scope(self.key))))
        self.assertEqual(counts, [str(total), "0"])

    def test_f_the_migration_applies_twice_and_rolls_back_and_forward(self):
        self.db.psql("-f", "/tmp/mig/016_effective_records.sql")
        self.run_ops("verify.sql")
        before = self.db.sql("SELECT count(*) FROM %s" % VIEW)
        self.run_ops("rollback.sql")
        self.assertEqual(self.db.sql("SELECT to_regclass('forensic.records_effective_cdr') IS NULL"), "t")
        self.run_ops("preflight.sql")   # on a database where the view does not exist (a casting of the missing name raised before)
        self.db.psql("-f", "/tmp/mig/016_effective_records.sql")
        self.run_ops("verify.sql")
        self.assertEqual(self.db.sql("SELECT count(*) FROM %s" % VIEW), before)
        self.assertEqual(self.db.sql("SELECT has_table_privilege('forensic_runtime', '%s', 'SELECT')" % VIEW), "t")

    def test_z_verify_fails_when_the_view_is_damaged(self):
        """Runs last: damages the view in two ways and requires verify.sql to notice each; restores it at the end."""
        definition = self.db.sql("SELECT pg_get_viewdef('%s'::regclass, true)" % VIEW)
        try:
            self.db.psql("-c", "CREATE OR REPLACE VIEW %s AS %s" % (VIEW, definition.rstrip(";")))   # same text, but without security_invoker
            with self.assertRaises(RuntimeError) as caught:
                self.run_ops("verify.sql")
            self.assertIn("security_invoker", str(caught.exception))
            self.db.psql("-c", "DROP VIEW %s; CREATE VIEW %s WITH (security_invoker = true) AS SELECT record_id, tenant_id FROM forensic.records" % (VIEW, VIEW))
            with self.assertRaises(RuntimeError) as caught:
                self.run_ops("verify.sql")
            self.assertIn("view has nothing", str(caught.exception))
        finally:
            self.db.psql("-c", "DROP VIEW IF EXISTS %s" % VIEW)
            self.db.psql("-f", "/tmp/mig/016_effective_records.sql")
        self.run_ops("verify.sql")


if __name__ == "__main__":
    unittest.main()
