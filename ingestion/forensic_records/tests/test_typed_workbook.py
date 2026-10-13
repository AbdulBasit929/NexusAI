"""SpreadsheetML 2003 workbooks (a provider's .xls export): the one file type that only the typed ingest can read.

Plain unittest:  python -m unittest ingestion.forensic_records.tests.test_typed_workbook

Version 1 has no reader for this format, and its CSV fallback would store the XML text as records. So the API queues such a workbook (judged by its content, not
its name) and the worker gives it to the typed ingest alone: strict, with nothing swallowed, completing the job and the evidence itself. These tests cover the
sniffing, the strict write, the hardening of the reader, the arithmetic of the job counts, and the worker's branch with fakes in place of the database.
"""

from __future__ import annotations

import contextlib
import inspect
import os
import sys
import unittest
from pathlib import Path
from unittest import mock

from ingestion.forensic_records import typed_ingest as ti
from ingestion.forensic_records import worker
from ingestion.forensic_records.layout_registry import readers
from ingestion.forensic_records.tests.test_typed_ingest import COLUMNS, KARACHI, LINEAGE, REGISTRY, FakeConn, FakeSink, Workdir

REPO = Path(__file__).resolve().parents[3]
try:
    sys.path.insert(0, str(REPO / "tools" / "real_data"))
    import make_golden  # noqa: E402
    GOLDEN_AVAILABLE = True
except Exception:  # pragma: no cover - the worker image carries no tools folder
    GOLDEN_AVAILABLE = False

SESSION_COLUMNS = COLUMNS + ("msisdn", "public_ip", "server_ip", "bytes_up", "bytes_down")
TIME_POLICY = {"source_timezone": "Asia/Karachi", "timezone_state": "analyst_confirmed"}
SPREADSHEETML = '<?xml version="1.0"?><Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet" xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet"><Worksheet ss:Name="S"><Table>%s</Table></Worksheet></Workbook>'


def workbook_xml(rows):
    cells = "".join("<Row>%s</Row>" % "".join('<Cell><Data ss:Type="String">%s</Data></Cell>' % value for value in row) for row in rows)
    return SPREADSHEETML % cells


class RecordingConn(FakeConn):
    """A fake connection that keeps every cursor it handed out, so the SQL a function ran can be read back."""

    def __init__(self):
        super().__init__()
        self.cursors = []

    @contextlib.contextmanager
    def cursor(self):
        cursor = mock.Mock()
        self.cursors.append(cursor)
        yield cursor


@unittest.skipUnless(GOLDEN_AVAILABLE, "tools/real_data/make_golden.py is not available (the worker image carries no tools folder)")
class WorkbookFileTests(Workdir):
    def golden_path(self, defects=False, seed=3):
        golden = make_golden.generate(REGISTRY.get("ipdr-i3-ufone-xls"), seed=seed, defects=defects)
        self.assertEqual(golden.container, "spreadsheetml")
        return golden.write(self.tmp.name, stem="ufone"), golden

    def test_the_generated_ufone_export_is_a_spreadsheetml_workbook_under_an_xls_name(self):
        path, _ = self.golden_path()
        self.assertTrue(path.endswith(".xls"))
        self.assertTrue(ti.is_spreadsheetml(path))

    def test_it_is_read_and_written_to_data_session_with_its_layout(self):
        path, golden = self.golden_path()
        sink = FakeSink(columns=SESSION_COLUMNS)
        out = ti.ingest_file(path, LINEAGE, KARACHI, sink, registry=REGISTRY)
        sheet = out["sheets"][0]
        self.assertEqual((out["container"], sheet["layout_id"], sheet["table"], sheet["loaded"], sheet["reconciles"]),
                         ("spreadsheetml", "ipdr-i3-ufone-xls", "data_session", len(golden.rows), True))
        self.assertEqual(len(sink.rows), len(golden.rows))

    def test_a_declaration_after_a_long_comment_is_refused_not_missed(self):
        path = os.path.join(self.tmp.name, "bomb.xls")
        padding = "<!--" + "x" * 100_000 + "-->"
        Path(path).write_text('<?xml version="1.0"?>' + padding + '<!DOCTYPE bomb [<!ENTITY a "aaaa">]>' + workbook_xml([["a"]])[len('<?xml version="1.0"?>'):], encoding="utf-8")
        with self.assertRaises(readers.Unsupported):
            readers.iter_spreadsheetml_sheets(path)
        out = ti.ingest_file(path, LINEAGE, KARACHI, FakeSink(), registry=REGISTRY)
        self.assertEqual((out["status"], out["reason"].split(":")[0]), ("skipped", "unsupported_container"))

    def test_the_whole_file_is_scanned_across_chunk_boundaries(self):
        path = os.path.join(self.tmp.name, "split.xls")
        Path(path).write_bytes(b"x" * 15 + b"<!DOCT" + b"YPE a>" + b"y" * 40)
        with self.assertRaises(readers.Unsupported):
            readers.reject_entities_in_file(path, chunk=20)
        clean = os.path.join(self.tmp.name, "clean.xls")
        Path(clean).write_bytes(b"x" * 200)
        self.assertIsNone(readers.reject_entities_in_file(clean, chunk=20))


class SniffTests(Workdir):
    def write(self, name, data):
        path = os.path.join(self.tmp.name, name)
        Path(path).write_bytes(data)
        return path

    def test_only_a_spreadsheetml_document_is_one(self):
        ole = bytes([0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1]) + bytes(600)
        cases = (("workbook.xls", workbook_xml([["a"]]).encode(), True), ("bom.xls", b"\xef\xbb\xbf" + workbook_xml([["a"]]).encode(), True),
                 ("legacy.xls", ole, False), ("note.xml", b'<?xml version="1.0"?><note><to>x</to></note>', False),
                 ("calls.csv", b"No.,Start Time\n1,2026-01-01\n", False), ("empty.xls", b"", False))
        for name, data, expected in cases:
            self.assertEqual(ti.is_spreadsheetml(self.write(name, data)), expected, name)
        self.assertFalse(ti.is_spreadsheetml(os.path.join(self.tmp.name, "missing.xls")))


class StrictWriteTests(Workdir):
    def workbook(self, rows, name="unknown.xls"):
        return self.write_xml(workbook_xml(rows), name)

    def write_xml(self, text, name):
        path = os.path.join(self.tmp.name, name)
        Path(path).write_text(text, encoding="utf-8")
        return path

    @unittest.skipUnless(GOLDEN_AVAILABLE, "make_golden is not available")
    def test_a_recognised_workbook_is_written_and_summarised(self):
        golden = make_golden.generate(REGISTRY.get("ipdr-i3-ufone-xls"), seed=3, defects=False)
        path = golden.write(self.tmp.name, stem="ufone")
        sink = FakeSink(columns=SESSION_COLUMNS)
        summary = ti.ingest_required(None, path, LINEAGE, TIME_POLICY, registry=REGISTRY, sink=sink, mode="shadow")
        self.assertEqual((summary["status"], summary["mode"], summary["rows_inserted"], len(sink.rows)), ("loaded", "shadow", len(golden.rows), len(golden.rows)))

    @unittest.skipUnless(GOLDEN_AVAILABLE, "make_golden is not available")
    def test_with_the_switch_off_the_job_is_a_permanent_failure_that_says_why(self):
        path = make_golden.generate(REGISTRY.get("ipdr-i3-ufone-xls"), seed=3, defects=False).write(self.tmp.name, stem="ufone")
        sink = FakeSink(columns=SESSION_COLUMNS)
        with self.assertRaisesRegex(ValueError, "FORENSIC_INGEST_V2 is off"):
            ti.ingest_required(None, path, LINEAGE, TIME_POLICY, registry=REGISTRY, sink=sink, mode="off")
        self.assertEqual((sink.rows, sink.calls), ([], []))

    def test_a_workbook_that_matches_no_layout_is_rejected_not_quietly_completed(self):
        sink = FakeSink()
        with self.assertRaisesRegex(ValueError, "no registered layout matches"):
            ti.ingest_required(None, self.workbook([["alpha", "beta", "gamma", "delta"], ["1", "2", "3", "4"]]), LINEAGE, TIME_POLICY, registry=REGISTRY, sink=sink, mode="shadow")
        self.assertEqual((sink.rows, sink.calls), ([], []))

    def test_a_declaration_makes_it_a_permanent_failure_with_the_reason(self):
        path = self.write_xml('<?xml version="1.0"?><!DOCTYPE a [<!ENTITY b "c">]>' + SPREADSHEETML % "", "doctype.xls")
        with self.assertRaisesRegex(ValueError, "unsupported_container"):
            ti.ingest_required(None, path, LINEAGE, TIME_POLICY, registry=REGISTRY, sink=FakeSink(), mode="shadow")

    @unittest.skipUnless(GOLDEN_AVAILABLE, "make_golden is not available")
    def test_no_evidence_id_is_a_permanent_failure(self):
        path = make_golden.generate(REGISTRY.get("ipdr-i3-ufone-xls"), seed=3, defects=False).write(self.tmp.name, stem="ufone")
        with self.assertRaisesRegex(ValueError, "no_evidence_id"):
            ti.ingest_required(None, path, ti.Lineage("t", "c", "", "f", LINEAGE.batch_id), TIME_POLICY, registry=REGISTRY, sink=FakeSink(), mode="shadow")

    @unittest.skipUnless(GOLDEN_AVAILABLE, "make_golden is not available")
    def test_a_database_error_is_not_swallowed(self):
        path = make_golden.generate(REGISTRY.get("ipdr-i3-ufone-xls"), seed=3, defects=False).write(self.tmp.name, stem="ufone")

        class Broken(FakeSink):
            def insert_batch(self, table, columns, rows):
                raise RuntimeError("connection lost")

        with self.assertRaisesRegex(RuntimeError, "connection lost"):
            ti.ingest_required(None, path, LINEAGE, TIME_POLICY, registry=REGISTRY, sink=Broken(columns=SESSION_COLUMNS), mode="shadow")

    @unittest.skipUnless(GOLDEN_AVAILABLE, "make_golden is not available")
    def test_a_file_over_the_row_limit_is_a_permanent_failure(self):
        path = make_golden.generate(REGISTRY.get("ipdr-i3-ufone-xls"), seed=3, defects=False).write(self.tmp.name, stem="ufone")
        with self.assertRaises(ValueError):
            ti.ingest_required(None, path, LINEAGE, TIME_POLICY, registry=REGISTRY, sink=FakeSink(columns=SESSION_COLUMNS), mode="shadow", max_rows=2)
        self.assertEqual(worker.classify_job_error(ti.TooManyRows("x")), "permanent_input")

    def test_the_workers_error_classes_for_these_failures(self):
        self.assertEqual(worker.classify_job_error(ValueError("no layout")), "permanent_input")
        self.assertEqual(worker.classify_job_error(RuntimeError("database")), "transient_processing")


class JobCountsTests(unittest.TestCase):
    def test_total_is_loaded_plus_quarantined_and_blank_rows_are_not_rows_of_data(self):
        summary = {"sheets": [
            {"status": "loaded", "loaded": 10, "inserted": 7, "quarantined": {"a": 2, "b": 1}, "skipped": {"skipped:blank_row": 3}},
            {"status": "loaded", "loaded": 5, "inserted": 5, "quarantined": {}, "skipped": {}},
            {"status": "unrecognised", "loaded": 0, "inserted": 0, "quarantined": {}, "skipped": {}}]}
        counts = ti.job_counts(summary)
        self.assertEqual(counts, {"total_rows": 18, "accepted_rows": 12, "duplicate_rows": 3, "rejected_rows": 3, "skipped_rows": 3})
        self.assertEqual(counts["total_rows"], counts["accepted_rows"] + counts["duplicate_rows"] + counts["rejected_rows"])


class WorkerBranchTests(Workdir):
    def job(self, source_file="ufone.xls"):
        import dataclasses
        fields = {"job_id": LINEAGE.batch_id, "tenant_id": "tenant-1", "user_id": "u", "collection_id": "case-1", "file_id": "f1", "source_file": source_file,
                  "record_type": "generic", "evidence_id": LINEAGE.evidence_id, "request_id": "req-1", "metadata": {"source_timezone": "Asia/Karachi"}}
        wanted = {f.name: fields[f.name] if f.name in fields else "" for f in dataclasses.fields(worker.IngestJob) if f.default is dataclasses.MISSING or f.name in fields}
        return worker.IngestJob(**wanted)

    def test_process_job_gives_the_workbook_to_the_typed_ingest_before_version_one_profiles_it(self):
        source = inspect.getsource(worker.process_job)
        branch = source.index("is_typed_workbook_job(job, path)")
        self.assertLess(source.index("process_media_job("), branch)
        self.assertLess(branch, source.index("profile_records(path, job.source_file)"))
        after = source[branch:source.index("profile_records(path, job.source_file)")]
        self.assertIn("process_typed_workbook_job(conn, job, path)", after)
        self.assertLess(after.index("conn.commit()"), after.index("return"), "it commits and returns: version 1 never sees the file")

    def test_which_jobs_take_the_branch(self):
        good = os.path.join(self.tmp.name, "good")
        Path(good).write_text(workbook_xml([["a"]]), encoding="utf-8")
        ole = os.path.join(self.tmp.name, "ole")
        Path(ole).write_bytes(bytes([0xD0, 0xCF, 0x11, 0xE0]) + bytes(600))
        for name, path, expected in (("a.xls", good, True), ("A.XLS", good, True), ("b.xml", good, True), ("c.csv", good, False), ("d.xlsx", good, False),
                                     ("e.xls", ole, False), ("", good, False)):
            self.assertEqual(worker.is_typed_workbook_job(self.job(name), Path(path)), expected, name)

    def test_the_branch_completes_the_job_and_the_evidence_and_audits_the_summary(self):
        summary = {"status": "loaded", "mode": "shadow", "reconciles": True,
                   "sheets": [{"status": "loaded", "layout_id": "ipdr-i3-ufone-xls", "layout_version": 1, "table": "data_session", "loaded": 10, "inserted": 8,
                               "quarantined": {"quarantined:time_missing": 2}, "skipped": {"skipped:blank_row": 1}}]}
        conn = RecordingConn()
        with mock.patch.object(ti, "ingest_required", return_value=summary) as strict, mock.patch.object(worker, "audit") as audit:
            profile, stats = worker.process_typed_workbook_job(conn, self.job(), Path("stored-object"))
        lineage = strict.call_args.args[2]
        self.assertEqual((lineage.tenant_id, lineage.collection_id, lineage.evidence_id, lineage.file_id, lineage.batch_id),
                         ("tenant-1", "case-1", LINEAGE.evidence_id, "f1", LINEAGE.batch_id))
        self.assertEqual(strict.call_args.args[3]["source_timezone"], "Asia/Karachi")
        calls = [call for cursor in conn.cursors for call in cursor.execute.call_args_list]
        job_sql, job_params = calls[0].args
        evidence_sql, evidence_params = calls[1].args
        self.assertIn("UPDATE forensic.records_ingest_jobs", job_sql)
        self.assertIn("status='completed'", job_sql)
        self.assertEqual(job_params[:4], (12, 8, 2, 2))
        self.assertIn("UPDATE forensic.evidence_items", evidence_sql)
        self.assertIn("processing_status='completed'", evidence_sql)
        self.assertEqual(evidence_params[0], LINEAGE.batch_id)
        self.assertEqual(audit.call_args.args[2], "records.ingest.typed_workbook.completed")
        self.assertEqual(audit.call_args.args[3]["counts"]["accepted_rows"], 8)
        self.assertEqual((stats["total_rows"], stats["rejected_rows"]), (12, 2))
        self.assertEqual((profile["format"], profile["classification"]["layouts"]), ("spreadsheetml_2003", ["ipdr-i3-ufone-xls"]))

    def test_a_failure_in_the_typed_ingest_ends_the_job_it_is_not_swallowed(self):
        conn = RecordingConn()
        with mock.patch.object(ti, "ingest_required", side_effect=ValueError("no registered layout matches")), mock.patch.object(worker, "audit") as audit:
            with self.assertRaisesRegex(ValueError, "no registered layout"):
                worker.process_typed_workbook_job(conn, self.job(), Path("stored-object"))
        self.assertEqual((conn.cursors, audit.called), ([], False), "nothing is marked completed")


if __name__ == "__main__":
    unittest.main()
