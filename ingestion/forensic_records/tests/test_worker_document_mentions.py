"""The worker's hook for the mention index of documents (migration 031), tested without a database.

Plain unittest:  python -m unittest ingestion.forensic_records.tests.test_worker_document_mentions

The hook (`worker.run_document_mentions_shadow`) is off unless FORENSIC_DOCUMENT_MENTIONS asks for it, runs in a savepoint inside the job's transaction after the passages are stored, and must
never fail the job. That the SQL does what a fake cursor here only records is proved by tools/case_generator/test_doc_views.py (the production indexer against a throwaway database).
"""

from __future__ import annotations

import contextlib
import dataclasses
import inspect
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

from ingestion.forensic_records import worker

REPO = Path(__file__).resolve().parents[3]
PACKAGE = REPO / "ingestion" / "forensic_records"


class FakeCursor:
    def __init__(self, rows=None, fail_on: str | None = None) -> None:
        self.calls: list[tuple[str, object]] = []
        self.rows = rows or []
        self.fail_on = fail_on

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def execute(self, sql, params=None):
        self.calls.append((sql, params))
        if self.fail_on and self.fail_on in sql:
            raise RuntimeError("the database refused the row")

    def fetchall(self):
        return self.rows


class FakeConn:
    """A connection whose transaction() is a savepoint: it records begin, commit or rollback and re-raises what went wrong."""

    def __init__(self, rows=None, fail_on: str | None = None) -> None:
        self.cur = FakeCursor(rows, fail_on)
        self.log: list[str] = []

    def cursor(self):
        return self.cur

    @contextlib.contextmanager
    def transaction(self):
        self.log.append("begin")
        try:
            yield
        except BaseException:
            self.log.append("rollback")
            raise
        self.log.append("commit")


def passage(text, number):
    return SimpleNamespace(text=text, artifact_id="11111111-1111-5111-8111-11111111111%d" % number, locator={"evidence_id": "e", "paragraph": number, "passage": 1}, text_sha256="x" * 64)


def job():
    return SimpleNamespace(job_id="j1", tenant_id="default", collection_id="c1", evidence_id="5a000000-0000-4000-8000-0000000000a1", source_file="memo.docx")


class Switch(unittest.TestCase):
    def test_on_only_for_the_words_that_mean_on_and_never_for_a_typo(self):
        for value in ("true", "TRUE", " yes ", "1", "on", "shadow"):
            self.assertTrue(worker.document_mentions_enabled(value), value)
        for value in ("", "tru", "false", "0", "off", "no", "enabled", None):
            with mock.patch.dict(os.environ, {worker.DOCUMENT_MENTIONS_ENV: value or ""}):
                self.assertFalse(worker.document_mentions_enabled(value), repr(value))

    def test_off_by_default_touches_nothing(self):
        cur = FakeCursor()
        with mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop(worker.DOCUMENT_MENTIONS_ENV, None)
            worker.run_document_mentions_shadow(cur, job(), SimpleNamespace(passages=[passage("Mobile 0300-0001000", 1)]))
        self.assertEqual(cur.calls, [])


class Gazetteer(unittest.TestCase):
    def setUp(self):
        worker.document_gazetteer.cache_clear()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def write(self, name, content):
        path = os.path.join(self.tmp.name, name)
        Path(path).write_text(content, encoding="utf-8")
        return path

    def test_a_good_file_is_read_with_its_urdu_spellings(self):
        path = self.write("g.json", json.dumps({"model town": ["Model Town", "ماڈل ٹاؤن"]}, ensure_ascii=False))
        self.assertEqual(worker.document_gazetteer(path), {"model town": ["Model Town", "ماڈل ٹاؤن"]})

    def test_nothing_configured_a_missing_file_a_damaged_file_and_a_file_of_the_wrong_shape_mean_no_places(self):
        self.assertEqual(worker.document_gazetteer(""), {})
        with self.assertLogs(worker.LOG, "WARNING"):
            self.assertEqual(worker.document_gazetteer(os.path.join(self.tmp.name, "absent.json")), {})
        with self.assertLogs(worker.LOG, "WARNING"):
            self.assertEqual(worker.document_gazetteer(self.write("bad.json", "{not json")), {})
        with self.assertLogs(worker.LOG, "WARNING"):
            self.assertEqual(worker.document_gazetteer(self.write("list.json", "[1, 2]")), {})

    def test_the_shipped_lahore_file_is_what_the_generator_gives_the_extractor(self):
        shipped = json.loads((PACKAGE / "gazetteers" / "lahore.json").read_text(encoding="utf-8"))
        sys.path.insert(0, str(REPO / "tools" / "case_generator"))
        try:
            import pk_locale
        except Exception:  # pragma: no cover - the worker image carries no tools folder
            self.skipTest("the generator is not available")
        finally:
            sys.path.pop(0)
        self.assertEqual(shipped, pk_locale.gazetteer("lahore"))


def stored(*texts):
    """What the SELECT of the stored passages returns: (artifact id, text, locator) in document order."""
    return [("11111111-1111-5111-8111-11111111111%d" % n, text, {"evidence_id": "e", "paragraph": n, "passage": 1}) for n, text in enumerate(texts, start=1)]


class Hook(unittest.TestCase):
    def setUp(self):
        worker.document_gazetteer.cache_clear()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.gazetteer = os.path.join(self.tmp.name, "g.json")
        Path(self.gazetteer).write_text(json.dumps({"model town": ["Model Town"]}), encoding="utf-8")
        self.env = mock.patch.dict(os.environ, {worker.DOCUMENT_MENTIONS_ENV: "true", worker.DOCUMENT_GAZETTEER_ENV: self.gazetteer})
        self.env.start()
        self.addCleanup(self.env.stop)

    def test_the_rows_are_written_in_a_savepoint_with_the_job_scope_and_the_gazetteer_places(self):
        conn = FakeConn(stored("Accused CNIC 00000-1234567-1, mobile 0300-0001000, seen in Model Town."))
        with self.assertLogs(worker.LOG, "INFO") as logs:
            worker.run_document_mentions_shadow(conn, job())
        self.assertEqual(conn.log, ["begin", "commit"])
        select, *inserts = conn.cur.calls
        self.assertIn("FROM forensic.derived_artifacts", select[0])
        self.assertEqual(select[1][:3], ("default", "c1", "5a000000-0000-4000-8000-0000000000a1"))
        self.assertEqual(sorted(p[4] for _, p in inserts), ["msisdn", "national_id", "place"])
        for _, params in inserts:
            self.assertEqual(params[:3], ("default", "c1", "5a000000-0000-4000-8000-0000000000a1"))
        by_kind = {p[4]: p for _, p in inserts}
        self.assertEqual((by_kind["msisdn"][5], by_kind["place"][5]), ("923000001000", "model town"))
        self.assertEqual((by_kind["national_id"][5], by_kind["national_id"][6]), (None, None), "a national ID number is counted and withheld")
        self.assertNotIn("1234567", json.dumps([params for _, params in conn.cur.calls], default=str))
        self.assertNotIn("0300-0001000", "\n".join(logs.output), "the log carries counts by kind, never a key")

    def test_the_passages_are_read_back_of_the_current_version_in_document_order(self):
        conn = FakeConn(stored("x"))
        worker.run_document_mentions_shadow(conn, job())
        sql = conn.cur.calls[0][0]
        self.assertIn("current_version_id", sql)
        self.assertIn("ORDER BY", sql)
        self.assertLess(sql.index("(citation_locator->>'paragraph')::int"), sql.index("(citation_locator->>'passage')::int"))

    def test_off_by_default_and_for_a_file_the_typed_writer_loaded_touches_nothing(self):
        conn = FakeConn(stored("Mobile 0300-0001000"))
        worker.run_document_mentions_shadow(conn, job(), skip=True)
        self.assertEqual((conn.log, conn.cur.calls), ([], []))
        with mock.patch.dict(os.environ, {worker.DOCUMENT_MENTIONS_ENV: ""}):
            worker.run_document_mentions_shadow(conn, job())
        self.assertEqual((conn.log, conn.cur.calls), ([], []))

    def test_no_gazetteer_means_no_places_and_the_rest_is_unchanged(self):
        with mock.patch.dict(os.environ, {worker.DOCUMENT_GAZETTEER_ENV: ""}):
            conn = FakeConn(stored("Mobile 0300-0001000, seen in Model Town."))
            worker.run_document_mentions_shadow(conn, job())
        self.assertEqual(sorted(p[4] for sql, p in conn.cur.calls if sql.startswith("INSERT")), ["msisdn"])

    def test_a_document_that_names_nothing_writes_no_row(self):
        conn = FakeConn(stored("Minutes of the weekly meeting. Nothing to report."))
        worker.run_document_mentions_shadow(conn, job())
        self.assertEqual(conn.log, ["begin", "commit"])
        self.assertEqual([sql for sql, _ in conn.cur.calls if sql.startswith("INSERT")], [])

    def test_a_failure_rolls_the_savepoint_back_is_not_raised_and_is_logged_by_class_name_only(self):
        conn = FakeConn(stored("Mobile 0300-0001000"), fail_on="INSERT INTO forensic.document_mention")
        with self.assertLogs(worker.LOG, "ERROR") as logs:
            worker.run_document_mentions_shadow(conn, job())
        self.assertEqual(conn.log, ["begin", "rollback"])
        self.assertIn("RuntimeError", "\n".join(logs.output))
        self.assertNotIn("refused", "\n".join(logs.output), "the database message is not logged")

    def test_a_cursor_that_cannot_read_the_passages_does_not_fail_the_job(self):
        conn = FakeConn(fail_on="FROM forensic.derived_artifacts")
        with self.assertLogs(worker.LOG, "ERROR"):
            worker.run_document_mentions_shadow(conn, job())
        self.assertEqual(conn.log, ["begin", "rollback"])


class WherePlaced(unittest.TestCase):
    def test_the_index_follows_the_typed_writer_in_the_document_branch_and_before_the_commit(self):
        source = inspect.getsource(worker.process_job)
        typed = source.index("records_loaded = run_typed_ingest_shadow_for_text(conn, job, path, document_profile)")
        index = source.index("run_document_mentions_shadow(conn, job, skip=records_loaded)")
        self.assertLess(source.index("process_document_job(conn, job, path)"), typed)
        self.assertLess(typed, index)
        self.assertLess(index, source.index("conn.commit()", typed))

    def test_the_document_pipeline_itself_is_not_touched(self):
        self.assertNotIn("run_document_mentions_shadow", inspect.getsource(worker.persist_document_result))
        self.assertNotIn("run_document_mentions_shadow", inspect.getsource(worker.process_document_job))


def job_dataclass():
    return worker.IngestJob(job_id="j1", tenant_id="default", user_id="u", collection_id="c1", file_id="f", source_file="x.txt", spool_path="/x", sha256="0" * 64, record_type="generic",
                            headers=[], metadata={}, evidence_id="5a000000-0000-4000-8000-0000000000a1")


class TypedStepReports(unittest.TestCase):
    def test_a_summary_with_a_loaded_sheet_means_the_file_is_records(self):
        self.assertTrue(worker.typed_summary_loaded({"sheets": [{"status": "unrecognised"}, {"status": "loaded", "loaded": 12}]}))
        for summary in (None, {}, {"sheets": []}, {"sheets": [{"status": "unrecognised"}]}, {"sheets": [{"status": "loaded", "loaded": 0}]}, {"status": "failed", "sheets": []}, "text", 5):
            self.assertFalse(worker.typed_summary_loaded(summary), repr(summary))

    def test_the_text_helper_says_whether_records_were_loaded(self):
        for name in ("ipdr.txt", "statement.pdf", "statement.docx"):
            row = dataclasses.replace(job_dataclass(), source_file=name)
            with mock.patch.object(worker, "run_typed_ingest_shadow", return_value={"sheets": [{"status": "loaded", "loaded": 3}]}):
                self.assertTrue(worker.run_typed_ingest_shadow_for_text(FakeConn(), row, Path("x"), {}), name)
            with mock.patch.object(worker, "run_typed_ingest_shadow", return_value={"sheets": [{"status": "unrecognised"}]}):
                self.assertFalse(worker.run_typed_ingest_shadow_for_text(FakeConn(), row, Path("x"), {}), name)
            with mock.patch.object(worker, "run_typed_ingest_shadow", return_value=None):
                self.assertFalse(worker.run_typed_ingest_shadow_for_text(FakeConn(), row, Path("x"), {}), name)
        notes = dataclasses.replace(job_dataclass(), source_file="notes.md")
        with mock.patch.object(worker, "run_typed_ingest_shadow") as run:
            self.assertFalse(worker.run_typed_ingest_shadow_for_text(FakeConn(), notes, Path("x"), {}))
        run.assert_not_called()


class FlatLayout(unittest.TestCase):
    """The worker image keeps its modules flat in /app: the extractor and the indexer must import there as they do in the package."""

    def test_the_indexer_imports_and_runs_with_no_parent_package(self):
        with tempfile.TemporaryDirectory() as tmp:
            for name in ("document_mentions.py", "mention_extractor.py"):
                shutil.copy(PACKAGE / name, tmp)
            shutil.copytree(PACKAGE / "layout_registry", os.path.join(tmp, "layout_registry"), ignore=shutil.ignore_patterns("__pycache__"))
            code = ("import sys; sys.path.insert(0, %r); import types, document_mentions as d; "
                    "p = types.SimpleNamespace(text='Mobile 0300-0001000', artifact_id='a', locator={}); "
                    "print([(r['kind'], r['value']) for r in d.mention_rows([p])])" % tmp)
            out = subprocess.run([sys.executable, "-I", "-c", code], capture_output=True, text=True, cwd=tmp, timeout=60)
        self.assertEqual(out.returncode, 0, out.stderr)
        self.assertIn("('msisdn', '923000001000')", out.stdout)


if __name__ == "__main__":
    unittest.main()
