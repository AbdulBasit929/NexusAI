"""Tests for the harness loader (load_case.py). They start ONE throwaway PostgreSQL container from an image already on this machine
(skipped when Docker or the image is absent), load a generated case and run the SQL checks.  Plain unittest, about two minutes:

    python -X utf8 tools/case_generator/test_load_case.py

The last test damages the loaded data on purpose and requires the checks to notice; it is named so that it runs after the others.
"""
import os
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import generate  # noqa: E402
import load_case  # noqa: E402
import world as w  # noqa: E402
from ingestion.forensic_records.layout_registry import load_registry  # noqa: E402


def docker_ready():
    try:
        return subprocess.run(["docker", "image", "inspect", load_case.IMAGE], capture_output=True).returncode == 0
    except OSError:
        return False


@unittest.skipUnless(docker_ready(), "Docker or the image %s is not available on this machine" % load_case.IMAGE)
class LoadCaseTests(unittest.TestCase):
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
            cls.key, cls.stats = load_case.load_case(cls.db, cls.root, cls.registry)
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

    def test_every_check_passes(self):
        failed = ["%s: %s" % (name, detail) for name, ok, detail in self.report.items if not ok]
        self.assertEqual(failed, [])
        self.assertGreater(len(self.report.items), 50)

    def test_the_checks_cover_every_answer_family(self):
        names = " ".join(name for name, _, _ in self.report.items)
        for family in ("reconcile", "A1 ", "A2 ", "A5 ", "A6 ", "A13 ", "quarantined rows", "repeated rows"):
            self.assertIn(family, names)

    def test_the_typed_tables_hold_exactly_the_accepted_rows(self):
        for table in ("comm_event", "data_session", "app_usage"):
            expected = sum(s["loaded"] for s in self.stats.values() if s["table"] == table)
            self.assertEqual(int(self.db.sql("SELECT count(*) FROM forensic.%s" % table)), expected, table)
            self.assertGreater(expected, 100, table)

    def test_quarantined_and_blank_rows_are_not_loaded(self):
        read = sum(s["read"] for s in self.stats.values())
        loaded = sum(s["loaded"] for s in self.stats.values())
        quarantined = sum(s["quarantined"] for s in self.stats.values())
        skipped = sum(s["skipped"] for s in self.stats.values())
        self.assertEqual(read, loaded + quarantined + skipped)
        self.assertGreater(quarantined, 0)
        self.assertGreater(skipped, 0)

    def test_repeated_rows_are_kept_with_lineage_and_flagged(self):
        flagged = self.db.rows("SELECT evidence_id, content_hash, count(*) AS n FROM forensic.comm_event GROUP BY 1, 2 HAVING count(*) > 1")
        self.assertTrue(flagged, "a repeated row is kept, not dropped")
        twin = self.db.sql("SELECT count(*) FROM forensic.comm_event WHERE 'duplicate_row' = ANY(quality_flags)")
        self.assertEqual(int(twin), sum(s["duplicates"] for s in self.stats.values() if s["table"] == "comm_event"))
        # the first copy of every repeated row is not flagged
        unflagged = self.db.sql("SELECT count(*) FROM (SELECT content_hash FROM forensic.comm_event GROUP BY evidence_id, content_hash "
                                "HAVING count(*) > 1 AND count(*) FILTER (WHERE NOT ('duplicate_row' = ANY(quality_flags))) <> 1) q")
        self.assertEqual(int(unflagged), 0)

    def test_every_row_keeps_its_source_row_and_layout_lineage(self):
        self.assertEqual(int(self.db.sql("SELECT count(*) FROM forensic.comm_event e LEFT JOIN forensic.layout_versions l ON l.layout_id = e.layout_id "
                                         "AND l.layout_version = e.layout_version WHERE l.layout_id IS NULL")), 0)
        self.assertEqual(int(self.db.sql("SELECT count(*) FROM forensic.comm_event WHERE jsonb_typeof(raw_payload) <> 'object' OR source_row < 1")), 0)

    def test_numbers_stay_inside_the_reserved_block(self):
        outside = self.db.sql("SELECT count(*) FROM forensic.comm_event WHERE a_norm !~ '^9230000[0-9]{5}$' OR (b_kind = 'person_like' AND b_norm !~ '^9230000[0-9]{5}$')")
        self.assertEqual(int(outside), 0)

    def test_row_level_security_hides_the_case_from_another_tenant(self):
        have_role = self.db.sql("SELECT count(*) FROM pg_roles WHERE rolname = 'forensic_runtime'")
        if have_role != "1":
            self.skipTest("the runtime role is not present in this image build")
        mine = self.db.sql("SET ROLE forensic_runtime; SELECT set_config('app.tenant_id', 'case', false); SELECT count(*) FROM forensic.comm_event;").splitlines()[-1]
        other = self.db.sql("SET ROLE forensic_runtime; SELECT set_config('app.tenant_id', 'another', false); SELECT count(*) FROM forensic.comm_event;").splitlines()[-1]
        self.assertGreater(int(mine), 100)
        self.assertEqual(int(other), 0)

    def test_the_window_sql_merges_and_splits_by_hand(self):
        """Hand-made events in a collection of their own: the gap rule of the co-location windows, which a generated case rarely exercises."""
        collection = "handcheck"
        evidence = "5a000000-0000-4000-8000-000000000001"
        self.db.script(
            "INSERT INTO forensic.evidence_items (evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension, "
            "size_bytes, sha256, modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata) VALUES "
            "('%s', '%s', '%s', 'SYNTHETIC-HAND', 'case-loader', 'hand.csv', 'hand.csv', 'text/csv', 'csv', 1, repeat('d', 64), 'structured_records', 'cdr', 1, "
            "'forensic_records_worker', 'completed', 'immutable://case/sha256/' || repeat('d', 64), jsonb_build_object('request_id', 'hand'));\n" % (evidence, load_case.TENANT, collection))
        times = {"X": ["05:00", "05:20", "05:50"], "Y": ["05:05", "05:25", "05:57"]}      # UTC; Karachi is five hours ahead
        n = 0
        rows = []
        for who, stamps in times.items():
            for stamp in stamps:
                n += 1
                rows.append("('%s', '%s', '%s', 'hand.csv', '5a000000-0000-4000-8000-0000000000a1', %d, 'h%d', 'c%d', 'call-c4-telenor-style', 1, 'voice', '%s', 'A', "
                            "timestamptz '2026-03-02 %s:00+00', 'Asia/Karachi', 'declared', '{}')" % (load_case.TENANT, collection, evidence, n, n, n, who, stamp))
        self.db.script("INSERT INTO forensic.comm_event (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, "
                       "layout_version, event_type, a_norm, cell_key, start_utc, source_tz, time_basis, raw_payload) VALUES\n" + ",\n".join(rows) + ";\n")
        # the same cell throughout; pairs within 15 minutes: (05:00,05:05) (05:05,05:20) (05:20,05:25) and (05:50,05:57); 25 minutes separate the groups
        fake = {"case_id": collection}
        windows = [(w["start_local"], w["end_local"], w["pairs"]) for w in self.db.rows(load_case.sql_a5_windows(fake, "X", "Y", 15))]
        self.assertEqual(windows, [("2026-03-02 10:00:00", "2026-03-02 10:25:00", 3), ("2026-03-02 10:50:00", "2026-03-02 10:57:00", 1)])
        pairs = self.db.rows(load_case.sql_a5_pairs(fake, "X", "Y", 15))
        self.assertEqual(len(pairs), 4)
        self.assertEqual(len(self.db.rows(load_case.sql_a5_pairs(fake, "X", "Y", 14))), 3, "the 15-minute pair (05:05, 05:20) drops out at 14 minutes")

    def test_zz_damaged_data_is_noticed(self):
        subject = self.key["subjects"][0]["msisdn"]
        self.db.sql("DELETE FROM forensic.comm_event WHERE ctid = (SELECT ctid FROM forensic.comm_event WHERE a_norm = %s AND dataset_kind = 'target_cdr' "
                    "AND b_kind = 'person_like' AND NOT ('duplicate_row' = ANY(quality_flags)) ORDER BY start_utc LIMIT 1)" % load_case.sql_text(subject))
        damaged = load_case.run_checks(self.db, self.key, self.stats)
        failed = [name for name, ok, _ in damaged.items if not ok]
        self.assertTrue(any(n.startswith("reconcile") for n in failed), failed)
        self.assertTrue(any(n.startswith("A1 ") for n in failed), failed)


if __name__ == "__main__":
    unittest.main(verbosity=1)
