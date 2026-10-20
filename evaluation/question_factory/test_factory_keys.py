"""Tests for the factory's key source (plain unittest):  python -X utf8 evaluation/question_factory/test_factory_keys.py

Proves: with effective=False the oracle SQL is untouched; with effective=True a call-record question reads forensic.records_effective_cdr and an internet-record question
forensic.records_effective_ipdr, a question of another family is untouched, a name that merely starts with forensic.records is not rewritten, and the read-only script of a key query
turns parallel workers off.
"""
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import factory  # noqa: E402


def item(family, sql):
    return {"family": family, "oracle_sql": sql, "kind": "number"}


class KeySourceTests(unittest.TestCase):
    def test_version_one_is_the_default(self):
        sql = "SELECT count(*) FROM forensic.records WHERE collection_id = 'c'"
        self.assertEqual(factory.key_sql(item("cdr", sql)), sql)

    def test_the_effective_view_of_the_family_is_read(self):
        sql = "SELECT count(*) FROM forensic.records WHERE collection_id = 'c' AND record_type = 'cdr'"
        self.assertEqual(factory.key_sql(item("cdr", sql), True), sql.replace("forensic.records", "forensic.records_effective_cdr"))
        self.assertEqual(factory.key_sql(item("ipdr", sql), True), sql.replace("forensic.records", "forensic.records_effective_ipdr"))

    def test_other_families_and_other_tables_are_untouched(self):
        sql = "SELECT count(*) FROM forensic.records WHERE collection_id = 'c'"
        self.assertEqual(factory.key_sql(item("anpr", sql), True), sql)
        other = "SELECT count(*) FROM forensic.records_audit_log"
        self.assertEqual(factory.key_sql(item("cdr", other), True), other)
        self.assertEqual(factory.key_sql(item("cdr", "SELECT max(x) FROM forensic.records\nWHERE y"), True), "SELECT max(x) FROM forensic.records_effective_cdr\nWHERE y")

    def test_a_key_query_runs_without_parallel_workers(self):
        seen = {}

        class Fake(factory.Db):
            pass

        db = Fake("cat", "c")
        import subprocess
        real = subprocess.run
        try:
            subprocess.run = lambda command, input=None, **kw: seen.update(script=input) or type("Done", (), {"returncode": 0, "stdout": "BEGIN\nSET\nSET\ndefault\n7\nCOMMIT\n", "stderr": ""})()
            self.assertEqual(db.scalar("SELECT 7"), "7")
        finally:
            subprocess.run = real
        self.assertIn("max_parallel_workers_per_gather = 0", seen["script"])


if __name__ == "__main__":
    unittest.main()
