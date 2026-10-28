"""Ingest v2 (typed_ingest.py): the shadow write of recognised provider exports to the typed tables, tested without a database.

Plain unittest:  python -m unittest ingestion.forensic_records.tests.test_typed_ingest

A fake sink stands in for PostgreSQL and keeps the idempotency key the real tables have, so these tests check what the module decides (which rows,
which hashes, which flags, what is counted, what is refused). That the SQL does what the sink here only imitates is proved by the throwaway-database
tests in tools/case_generator/test_ingest_v2.py (a generated case through this module, then the 174 SQL checks against the key).
"""

from __future__ import annotations

import contextlib
import csv
import hashlib
import dataclasses
import inspect
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from ingestion.forensic_records import typed_ingest as ti
from ingestion.forensic_records import worker
from ingestion.forensic_records.layout_registry import Policy, load_registry

REPO = Path(__file__).resolve().parents[3]
try:
    sys.path.insert(0, str(REPO / "tools" / "real_data"))
    import make_golden  # noqa: E402
    GOLDEN_AVAILABLE = True
except Exception:  # pragma: no cover - the worker image carries no tools folder
    GOLDEN_AVAILABLE = False

HEADER = ["MSISDN", "call_org_num", "CALL_DIALED_NUM", "IMSI", "IMEI", "CALL_START_DT_TM", "CALL_END_DT_TM", "INBOUND_OUTBOUND_IND", "Call_Network_Volume",
          "Lac_Id", "Site_Id", "Cell_SITE_ID", "lat", "longitude", "CALL_TYPE", "location", "Description", "MACAddress"]
KARACHI = Policy(source_tz="Asia/Karachi", tz_basis="declared")
EVIDENCE = "5a000000-0000-4000-8000-0000000000a1"
BATCH = "6a000000-0000-4000-8000-0000000000b1"
LINEAGE = ti.Lineage("tenant-1", "case-1", EVIDENCE, "calls.csv", BATCH)
REGISTRY = load_registry()
COLUMNS = ("tenant_id", "collection_id", "evidence_id", "file_id", "batch_id", "source_row", "row_hash", "content_hash", "layout_id", "layout_version",
           "dataset_kind", "event_type", "direction", "a_raw", "a_norm", "a_kind", "b_raw", "b_norm", "b_kind", "start_utc", "end_utc", "duration_s",
           "source_tz", "time_basis", "raw_payload", "quality_flags")


def call(a, other, start, end, direction="OUTGOING", kind="CALL"):
    incoming = other if direction == "INCOMING" else ""
    outgoing = other if direction == "OUTGOING" else ""
    return [a, incoming, outgoing, "001010000000001", "00000000000001", start, end, direction, "0", "1001", "1", "2002", "", "", kind, "Somewhere", "", ""]


GOOD = [
    call("923000000001", "923000000002", "2026-03-01 10:00:00", "2026-03-01 10:01:00"),
    call("923000000001", "923000000002", "2026-03-01 10:00:00", "2026-03-01 10:01:00"),     # the same row again: kept and flagged
    call("923000000001", "923000000003", "2026-03-02 11:00:00", "2026-03-02 11:05:00", "INCOMING"),
]


class FakeSink:
    """Keeps rows under the key of the real tables, as ON CONFLICT DO NOTHING would."""

    def __init__(self, columns=COLUMNS):
        self.columns, self.rows, self.keys, self.calls = columns, [], set(), []

    def table_columns(self, table):
        return [(c, "text") for c in self.columns]

    def insert_batch(self, table, columns, rows):
        self.calls.append((table, list(columns), len(rows)))
        new = 0
        for row in rows:
            key = tuple(row[c] for c in ("tenant_id", "collection_id", "file_id", "row_hash", "start_utc"))
            if key not in self.keys:
                self.keys.add(key)
                self.rows.append(row)
                new += 1
        return new


class FakeConn:
    """A connection whose transaction() is a savepoint: a failure inside is recorded as a rollback and still raised."""

    def __init__(self):
        self.log = []

    @contextlib.contextmanager
    def cursor(self):
        yield mock.Mock()

    @contextlib.contextmanager
    def transaction(self):
        self.log.append("begin")
        try:
            yield
        except BaseException:
            self.log.append("rollback")
            raise
        self.log.append("commit")


class Workdir(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def write_csv(self, rows, header=HEADER, name="calls.csv"):
        path = os.path.join(self.tmp.name, name)
        with open(path, "w", newline="", encoding="utf-8") as handle:
            writer = csv.writer(handle)
            writer.writerow(header)
            writer.writerows(rows)
        return path

    def ingest(self, rows, policy=KARACHI, sink=None, header=HEADER, **kwargs):
        sink = sink if sink is not None else FakeSink()
        return ti.ingest_file(self.write_csv(rows, header), LINEAGE, policy, sink, registry=REGISTRY, **kwargs), sink


class ModeTests(unittest.TestCase):
    def test_off_is_the_default_and_a_typo_never_turns_the_write_on(self):
        for value in (None, "", "off", "OFF", "0", "false", "no", "  "):
            self.assertEqual(ti.ingest_v2_mode(value), "off", value)
        with self.assertLogs("forensic.typed_ingest", "WARNING"):
            self.assertEqual(ti.ingest_v2_mode("shadw"), "off")
        self.assertEqual(ti.ingest_v2_mode("Shadow"), "shadow")

    def test_the_jobs_time_policy_becomes_the_registrys_policy(self):
        policy = ti.policy_from_time_policy({"source_timezone": "Asia/Karachi", "timezone_state": "analyst_confirmed", "date_order": "DMY"})
        self.assertEqual((policy.source_tz, policy.tz_basis, policy.date_order), ("Asia/Karachi", "confirmed_by_analyst", "DMY"))
        self.assertEqual(ti.policy_from_time_policy({"source_timezone": "UTC", "timezone_state": "profile_defaulted"}).tz_basis, "assumed_by_profile")
        self.assertEqual(ti.policy_from_time_policy({"source_timezone": "UTC", "timezone_state": "source_declared"}).tz_basis, "declared")
        none = ti.policy_from_time_policy({"source_timezone": None, "timezone_state": "unknown", "date_order": None})
        self.assertEqual((none.source_tz, none.tz_basis, none.date_order), (None, None, None))


class ReadingTests(Workdir):
    def test_every_row_read_is_loaded_quarantined_or_skipped_and_the_counts_add_up(self):
        rows = GOOD + [call("923000000001", "923000000003", "not a time", "2026-03-02 11:05:00"), [""] * 18]
        out, sink = self.ingest(rows)
        sheet = out["sheets"][0]
        self.assertEqual((sheet["rows_read"], sheet["loaded"], sheet["inserted"], sheet["already_present"]), (5, 3, 3, 0))
        self.assertEqual(sheet["quarantined"], {"quarantined:time_unparsed": 1})
        self.assertEqual(sheet["skipped"], {"skipped:blank_row": 1})
        self.assertEqual(sheet["first_rows"], {"quarantined:time_unparsed": [4], "skipped:blank_row": [5]})
        self.assertTrue(sheet["reconciles"] and out["reconciles"])
        self.assertEqual((out["status"], out["rows_loaded"], out["rows_inserted"], len(sink.rows)), ("loaded", 3, 3, 3))
        self.assertEqual((sheet["layout_id"], sheet["layout_version"], sheet["layout_status"], sheet["table"], sheet["header_row"]),
                         ("call-c4-telenor-style", 2, "draft", "comm_event", 0))
        self.assertEqual((sheet["first_start_local"], sheet["last_start_local"]), ("2026-03-01 10:00:00", "2026-03-02 11:00:00"))

    def test_a_repeated_row_is_kept_and_flagged_not_dropped(self):
        out, sink = self.ingest(GOOD)
        self.assertEqual(out["sheets"][0]["duplicates_in_file"], 1)
        self.assertEqual(out["sheets"][0]["flags"]["duplicate_row"], 1)
        flagged = [r for r in sink.rows if "duplicate_row" in r["quality_flags"]]
        self.assertEqual([r["source_row"] for r in flagged], [2])
        self.assertEqual(len({r["content_hash"] for r in sink.rows}), 2, "the repeat has the same content hash as the first")
        self.assertEqual(len({r["row_hash"] for r in sink.rows}), 3, "but its own row hash, so the idempotency key keeps all three")

    def test_the_rows_carry_their_lineage_and_the_two_hashes_by_the_documented_formulas(self):
        _, sink = self.ingest(GOOD)
        first = sink.rows[0]
        self.assertEqual([first[k] for k in ("tenant_id", "collection_id", "evidence_id", "file_id", "batch_id")], ["tenant-1", "case-1", EVIDENCE, "calls.csv", BATCH])
        self.assertEqual((first["layout_id"], first["layout_version"], first["source_row"]), ("call-c4-telenor-style", 2, 1))
        self.assertEqual((len(first["content_hash"]), len(first["row_hash"])), (64, 64))
        self.assertEqual((first["source_tz"], first["time_basis"], first["start_utc"]), ("Asia/Karachi", "declared", "2026-03-01T05:00:00+00:00"))
        self.assertEqual(first["raw_payload"]["CALL_DIALED_NUM"], "923000000002")

    def test_the_row_hash_follows_the_file_the_source_row_and_the_content(self):
        record = {"source_row": 7, "a_norm": "x", "start_utc": "2026-03-01T05:00:00+00:00", "raw_payload": {"k": "v"}, "quality_flags": ["f"]}
        row = ti.lineage_row(record, ["f"], LINEAGE, ["a_norm", "start_utc", "source_row", "raw_payload", "quality_flags", "row_hash", "content_hash"])
        content = json.dumps({"a_norm": "x", "start_utc": "2026-03-01T05:00:00+00:00"}, sort_keys=True)
        self.assertEqual(row["content_hash"], hashlib.sha256(content.encode()).hexdigest())
        self.assertEqual(row["row_hash"], hashlib.sha256(("calls.csv|7|" + content).encode()).hexdigest())
        other = ti.lineage_row(record, ["f"], ti.Lineage("tenant-1", "case-1", EVIDENCE, "other.csv", BATCH), ["a_norm"])
        self.assertNotEqual(other["row_hash"], row["row_hash"], "the same row in another file is another row")
        self.assertEqual(other["content_hash"], row["content_hash"])

    def test_a_second_run_of_the_same_file_inserts_nothing(self):
        sink = FakeSink()
        first, _ = self.ingest(GOOD, sink=sink)
        again, _ = self.ingest(GOOD, sink=sink)
        self.assertEqual((first["rows_inserted"], again["rows_inserted"], len(sink.rows)), (3, 0, 3))
        sheet = again["sheets"][0]
        self.assertEqual((sheet["loaded"], sheet["inserted"], sheet["already_present"]), (3, 0, 3))
        self.assertTrue(sheet["reconciles"])

    def test_the_same_file_under_another_file_id_is_new_rows(self):
        sink = FakeSink()
        self.ingest(GOOD, sink=sink)
        again = ti.ingest_file(self.write_csv(GOOD), ti.Lineage("tenant-1", "case-1", EVIDENCE, "copy.csv", BATCH), KARACHI, sink, registry=REGISTRY)
        self.assertEqual(again["rows_inserted"], 3)

    def test_without_a_declared_zone_no_time_is_guessed_and_every_row_is_quarantined_with_that_reason(self):
        out, sink = self.ingest(GOOD, policy=Policy())
        sheet = out["sheets"][0]
        self.assertEqual((sheet["loaded"], sheet["quarantined"]), (0, {"quarantined:source_timezone_unknown": 3}))
        self.assertEqual((sink.rows, sink.calls), ([], []))
        self.assertTrue(sheet["reconciles"])

    def test_batches_are_written_in_the_size_asked_for(self):
        out, sink = self.ingest(GOOD, batch_rows=2)
        self.assertEqual([n for _, _, n in sink.calls], [2, 1])
        self.assertEqual(out["rows_inserted"], 3)
        self.assertTrue(all(cols for _, cols, _ in sink.calls))

    def test_only_columns_the_table_has_are_written(self):
        sink = FakeSink(columns=[c for c in COLUMNS if c not in ("a_kind", "b_kind", "dataset_kind")])
        out, _ = self.ingest(GOOD, sink=sink)
        self.assertEqual(out["rows_inserted"], 3)
        self.assertTrue(all("a_kind" not in r and "dataset_kind" not in r for r in sink.rows))

    def test_a_row_limit_stops_the_file_one_row_past_it(self):
        out, _ = self.ingest(GOOD, max_rows=3)
        self.assertEqual(out["rows_loaded"], 3)
        with self.assertRaises(ti.TooManyRows):
            self.ingest(GOOD, max_rows=2)


class RefusalTests(Workdir):
    def test_a_file_the_registry_does_not_recognise_is_reported_and_writes_nothing(self):
        out, sink = self.ingest([["a", "b", "c", "d"]], header=["alpha", "beta", "gamma", "delta"])
        self.assertEqual((out["status"], out["sheets"][0]["status"], out["sheets"][0]["reason"]), ("unrecognised", "unrecognised", "none"))
        self.assertEqual((sink.rows, sink.calls), ([], []))

    def test_an_ambiguous_header_is_reported_not_guessed(self):
        class Registry:
            def find_header(self, rows, include_draft=True):
                best = mock.Mock(layout_id="call-c4-telenor-style", score=0.91)
                return mock.Mock(decision="ambiguous", best=best, header_row=0)

        out = ti.ingest_file(self.write_csv(GOOD), LINEAGE, KARACHI, FakeSink(), registry=Registry())
        self.assertEqual((out["status"], out["sheets"][0]["status"], out["sheets"][0]["layout_id"]), ("unrecognised", "ambiguous", "call-c4-telenor-style"))

    def test_a_layout_family_without_a_typed_table_is_skipped(self):
        entry = dict(REGISTRY.get("call-c4-telenor-style"), family="cell_reference")

        class Registry:
            def find_header(self, rows, include_draft=True):
                return mock.Mock(decision="match", best=mock.Mock(layout_id="x", score=1.0), header_row=0)

            def get(self, layout_id):
                return entry

        out = ti.ingest_file(self.write_csv(GOOD), LINEAGE, KARACHI, sink := FakeSink(), registry=Registry())
        self.assertEqual((out["sheets"][0]["status"], out["sheets"][0]["reason"]), ("skipped", "family_without_typed_table"))
        self.assertEqual(sink.rows, [])

    def test_a_table_without_the_lineage_columns_is_skipped(self):
        out, sink = self.ingest(GOOD, sink=FakeSink(columns=[c for c in COLUMNS if c != "row_hash"]))
        self.assertEqual((out["sheets"][0]["status"], out["sheets"][0]["reason"]), ("skipped", "typed_table_lacks_row_hash"))
        self.assertEqual(sink.rows, [])

    def test_no_evidence_id_or_a_bad_id_means_no_write(self):
        path = self.write_csv(GOOD)
        for evidence, batch, reason in (("", BATCH, "no_evidence_id"), ("not-a-uuid", BATCH, "invalid_evidence_id"), (EVIDENCE, "nope", "invalid_batch_id")):
            sink = FakeSink()
            out = ti.ingest_file(path, ti.Lineage("t", "c", evidence, "calls.csv", batch), KARACHI, sink, registry=REGISTRY)
            self.assertEqual((out["status"], out["reason"], sink.calls), ("skipped", reason, []))

    def test_an_unsupported_container_is_reported(self):
        path = os.path.join(self.tmp.name, "legacy.xls")
        Path(path).write_bytes(b"\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1" + b"\x00" * 64)
        out = ti.ingest_file(path, LINEAGE, KARACHI, FakeSink(), registry=REGISTRY)
        self.assertEqual(out["status"], "skipped")
        self.assertTrue(out["reason"].startswith("unsupported_container"))

    def test_an_empty_sheet_is_reported(self):
        path = os.path.join(self.tmp.name, "empty.csv")
        Path(path).write_text("", encoding="utf-8")
        out = ti.ingest_file(path, LINEAGE, KARACHI, FakeSink(), registry=REGISTRY)
        self.assertNotEqual(out["status"], "loaded")


class ShadowTests(Workdir):
    def shadow(self, conn=None, **kwargs):
        return ti.run_shadow(conn or FakeConn(), self.write_csv(GOOD), LINEAGE, {"source_timezone": "Asia/Karachi", "timezone_state": "source_declared"},
                             registry=REGISTRY, **kwargs)

    def test_off_does_nothing_at_all(self):
        conn = FakeConn()
        self.assertIsNone(self.shadow(conn, sink=FakeSink(), mode="off"))
        with mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop("FORENSIC_INGEST_V2", None)
            self.assertIsNone(self.shadow(conn, sink=FakeSink()))
        self.assertEqual(conn.log, [])

    def test_shadow_writes_inside_a_savepoint_and_returns_the_summary(self):
        conn, sink = FakeConn(), FakeSink()
        summary = self.shadow(conn, sink=sink, mode="shadow")
        self.assertEqual((summary["mode"], summary["status"], summary["rows_inserted"], conn.log), ("shadow", "loaded", 3, ["begin", "commit"]))
        self.assertEqual(len(sink.rows), 3)

    def test_a_failure_is_contained_rolled_back_and_summarised_without_the_error_text(self):
        class Boom(RuntimeError):
            diag = mock.Mock(constraint_name="comm_event_time_basis_chk")

        class Sink(FakeSink):
            def insert_batch(self, table, columns, rows):
                raise Boom("Key (a_raw)=(923000000001) violates a check")

        conn = FakeConn()
        with self.assertLogs("forensic.typed_ingest", "ERROR"):
            summary = self.shadow(conn, sink=Sink(), mode="shadow")
        self.assertEqual((summary["status"], summary["reason"], summary["constraint"], conn.log), ("failed", "Boom", "comm_event_time_basis_chk", ["begin", "rollback"]))
        self.assertNotIn("923000000001", json.dumps(summary))

    def test_too_many_rows_is_a_skip_with_a_rollback(self):
        conn = FakeConn()
        with self.assertLogs("forensic.typed_ingest", "WARNING"):
            summary = self.shadow(conn, sink=FakeSink(), mode="shadow", max_rows=2)
        self.assertEqual((summary["status"], summary["reason"], conn.log), ("skipped", "too_many_rows", ["begin", "rollback"]))

    def test_the_environment_sets_the_mode_and_the_row_limit(self):
        with mock.patch.dict(os.environ, {"FORENSIC_INGEST_V2": "shadow", "FORENSIC_INGEST_V2_MAX_ROWS": "1"}):
            with self.assertLogs("forensic.typed_ingest", "WARNING"):
                summary = self.shadow(sink=FakeSink())
        self.assertEqual(summary["reason"], "too_many_rows")

    def test_draft_layouts_can_be_switched_off(self):
        with mock.patch.dict(os.environ, {"FORENSIC_INGEST_V2_INCLUDE_DRAFT": "false"}):
            summary = self.shadow(sink=FakeSink(), mode="shadow")
        self.assertEqual((summary["status"], summary["sheets"][0]["status"]), ("unrecognised", "unrecognised"),
                         "all 14 layouts are drafts today, so excluding drafts recognises nothing")


@unittest.skipUnless(GOLDEN_AVAILABLE, "tools/real_data/make_golden.py is not available (the worker image carries no tools folder)")
class TextExportTests(Workdir):
    """What a text export that took the document route looks like to the typed writer."""

    def test_a_utf16_tab_separated_internet_record_export_is_read_and_written(self):
        golden = make_golden.generate(REGISTRY.get("ipdr-i4-zong-txt"), seed=3, defects=False)
        path = golden.write(self.tmp.name, stem="zong")
        self.assertTrue(path.endswith(".txt"))
        with open(path, "rb") as handle:
            self.assertEqual(handle.read(2), bytes([0xFF, 0xFE]), "UTF-16 with a byte order mark, as the real export is")
        sink = FakeSink(columns=COLUMNS + ("msisdn", "public_ip", "server_ip", "bytes_up", "bytes_down"))
        out = ti.ingest_file(path, LINEAGE, KARACHI, sink, registry=REGISTRY)
        sheet = out["sheets"][0]
        self.assertEqual((out["container"], sheet["layout_id"], sheet["table"], sheet["loaded"], sheet["quarantined"], sheet["reconciles"]),
                         ("text", "ipdr-i4-zong-txt", "data_session", len(golden.rows), {}, True))
        self.assertEqual(len(sink.rows), len(golden.rows))
        self.assertEqual({row["layout_id"] for row in sink.rows}, {"ipdr-i4-zong-txt"})

    def test_the_same_export_with_its_defects_quarantines_them_with_reasons(self):
        golden = make_golden.generate(REGISTRY.get("ipdr-i4-zong-txt"), seed=3, defects=True)
        out = ti.ingest_file(golden.write(self.tmp.name, stem="zong-defects"), LINEAGE, KARACHI, FakeSink(), registry=REGISTRY)
        sheet = out["sheets"][0]
        self.assertGreater(sum(sheet["quarantined"].values()) + sum(sheet["skipped"].values()), 0)
        self.assertTrue(sheet["reconciles"])

    def test_ordinary_notes_in_a_text_file_are_left_alone(self):
        path = os.path.join(self.tmp.name, "notes.txt")
        newline = chr(10)
        paragraph = ["We met on Tuesday and discussed the budget, the schedule, and the open questions.", "Nothing tabular here: sentences, commas, and words only."]
        Path(path).write_text(newline.join(["Meeting notes, 12 March", ""] + paragraph * 30), encoding="utf-8")
        sink = FakeSink()
        out = ti.ingest_file(path, LINEAGE, KARACHI, sink, registry=REGISTRY)
        self.assertEqual((out["status"], out["sheets"][0]["status"], sink.rows, sink.calls), ("unrecognised", "unrecognised", [], []))

    def test_binary_content_under_a_text_name_is_reported_as_unreadable_and_not_written(self):
        path = os.path.join(self.tmp.name, "blob.txt")
        Path(path).write_bytes(bytes(range(256)) * 40)
        sink = FakeSink()
        out = ti.ingest_file(path, LINEAGE, KARACHI, sink, registry=REGISTRY)
        self.assertEqual((out["status"], out["sheets"][0]["status"], out["sheets"][0]["reason"]), ("unrecognised", "unreadable", "Error"))
        self.assertEqual((sink.rows, sink.calls), ([], []))

    def test_a_reader_error_after_the_header_rolls_the_whole_file_back_rather_than_leave_half_of_it(self):
        golden = make_golden.generate(REGISTRY.get("ipdr-i4-zong-txt"), seed=3, defects=False)
        path = golden.write(self.tmp.name, stem="zong-damaged")
        with open(path, "ab") as handle:
            handle.write(("\x00\x00\x00" + chr(10)).encode("utf-16-le"))   # damage at the end of an otherwise good file
        conn = FakeConn()
        with self.assertLogs("forensic.typed_ingest", "ERROR"):
            summary = ti.run_shadow(conn, path, LINEAGE, {"source_timezone": "Asia/Karachi", "timezone_state": "source_declared"}, registry=REGISTRY,
                                    sink=FakeSink(columns=COLUMNS + ("msisdn",)), mode="shadow", )
        self.assertEqual((summary["status"], conn.log), ("failed", ["begin", "rollback"]))


class CopyTextTests(unittest.TestCase):
    def test_null_is_a_bare_marker_and_a_text_equal_to_it_stays_text(self):
        self.assertEqual(ti.copy_line([None, "\\N", "a,b", 'say "hi"', "line\nbreak", ""]), '\\N,"\\N","a,b","say ""hi""","line\nbreak",""\n')

    def test_values_are_written_by_the_column_type(self):
        self.assertIsNone(ti.format_cell(None, "text"))
        self.assertEqual(ti.format_cell(["b", 'a"c', "d\\e"], "_text"), '{"b","a\\"c","d\\\\e"}')
        self.assertEqual(ti.format_cell([], "_text"), "{}")
        self.assertEqual(ti.format_cell({"b": "2", "a": "é"}, "jsonb"), '{"a": "\\u00e9", "b": "2"}')
        self.assertEqual(ti.format_cell(True, "bool"), "t")
        self.assertEqual(ti.format_cell(12.5, "float8"), "12.5")

    def test_a_nul_character_is_dropped_so_it_cannot_cost_the_batch(self):
        self.assertEqual(ti.format_cell("a\x00b", "text"), "ab")
        self.assertEqual(ti.format_cell({"k": "a\x00b"}, "jsonb"), '{"k": "ab"}')
        self.assertEqual(ti.format_cell(["x\x00"], "_text"), '{"x"}')

    def test_copy_text_is_one_line_per_row_in_the_column_order_asked_for(self):
        text = ti.copy_text(["a", "b"], {"a": "text", "b": "jsonb"}, [{"a": "1", "b": {"k": 1}}, {"a": None}])
        self.assertEqual(text, '"1","{""k"": 1}"\n\\N,\\N\n')


class SqlTests(unittest.TestCase):
    def test_the_insert_is_idempotent_on_the_documented_key_and_counts_only_new_rows(self):
        sql = ti.insert_sql("comm_event", ["tenant_id", "row_hash"])
        self.assertIn("ON CONFLICT (tenant_id, collection_id, file_id, row_hash, start_utc) DO NOTHING", sql)
        self.assertIn("RETURNING 1", sql)
        self.assertIn("INSERT INTO forensic.comm_event (tenant_id, row_hash) SELECT tenant_id, row_hash FROM _v2_stage", sql)

    def test_the_stage_is_a_temporary_empty_copy_of_the_table_loaded_by_copy(self):
        self.assertEqual(ti.stage_sql("data_session", ["a", "b"]), "CREATE TEMP TABLE _v2_stage ON COMMIT DROP AS SELECT a, b FROM forensic.data_session WITH NO DATA")
        self.assertEqual(ti.copy_sql(["a", "b"]), "COPY _v2_stage (a, b) FROM STDIN WITH (FORMAT csv, NULL '\\N')")
        self.assertEqual(ti.drop_stage_sql(), "DROP TABLE IF EXISTS _v2_stage")

    def test_the_conflict_key_is_the_one_migration_012_declares(self):
        sql = (Path(__file__).resolve().parents[3] / "db" / "forensic_records" / "012_typed_families.sql").read_text(encoding="utf-8")
        self.assertIn("UNIQUE (" + ti.CONFLICT_KEY + ")", sql)

    def test_every_family_maps_to_a_table_the_migration_creates(self):
        sql = (Path(__file__).resolve().parents[3] / "db" / "forensic_records" / "012_typed_families.sql").read_text(encoding="utf-8")
        for table in ti.TABLE_FOR_FAMILY.values():
            self.assertIn("CREATE TABLE IF NOT EXISTS forensic.%s (" % table, sql)
        families = {e["family"] for e in REGISTRY.entries}
        self.assertEqual(families - set(ti.TABLE_FOR_FAMILY), set(), "every family a registered layout has is written somewhere")


PLATE_HEADER = ["Date Time", "Plate Number", "Camera ID", "Camera Name", "Lane", "Latitude", "Longitude", "Direction", "Confidence"]
PLATE_COLUMNS = ("tenant_id", "collection_id", "evidence_id", "file_id", "batch_id", "source_row", "row_hash", "content_hash", "layout_id", "layout_version", "source_kind",
                 "plate_raw", "plate_norm", "plate_script", "province", "series", "observed_utc", "time_basis", "source_tz", "camera_id", "camera_label", "lane", "lat", "lon",
                 "loc_method", "confidence", "raw_payload", "quality_flags")


def sighting(when, plate, camera="CAM-01", lat="31.512", lon="74.317", confidence="0.93", lane="Lane 1", direction="Towards"):
    return [when, plate, camera, "Camera " + camera[-1], lane, lat, lon, direction, confidence]


class PlateSink(FakeSink):
    """The sighting table keeps its rows under its own key, which ends in the observed time (migration 012)."""

    def __init__(self):
        super().__init__(PLATE_COLUMNS)

    def insert_batch(self, table, columns, rows):
        self.calls.append((table, list(columns), len(rows)))
        new = 0
        for row in rows:
            key = tuple(row[c] for c in ("tenant_id", "collection_id", "file_id", "row_hash", "observed_utc"))
            if key not in self.keys:
                self.keys.add(key)
                self.rows.append(row)
                new += 1
        return new


@unittest.skipUnless(GOLDEN_AVAILABLE, "tools/real_data not available")
class PlateSightingTests(Workdir):
    GOOD = [sighting("2026-03-01 10:00:00", "SYNT-0001"), sighting("2026-03-01 10:05:00", "synt 0001", "CAM-02"), sighting("2026-03-01 11:00:00", "SYNT\u20130002", "CAM-03")]

    def ingest_plates(self, rows, sink=None, policy=KARACHI):
        sink = sink if sink is not None else PlateSink()
        return ti.ingest_file(self.write_csv(rows, PLATE_HEADER, "cameras.csv"), LINEAGE, policy, sink, registry=REGISTRY), sink

    def test_a_camera_export_is_written_to_the_sighting_table_with_the_plate_as_written_and_its_search_key(self):
        out, sink = self.ingest_plates(self.GOOD)
        sheet = out["sheets"][0]
        self.assertEqual((sheet["layout_id"], sheet["table"], sheet["loaded"], sheet["inserted"]), ("anpr-a1", "plate_sighting", 3, 3))
        self.assertEqual([(r["plate_raw"], r["plate_norm"]) for r in sink.rows], [("SYNT-0001", "SYNT0001"), ("synt 0001", "SYNT0001"), ("SYNT\u20130002", "SYNT0002")])
        first = sink.rows[0]
        self.assertEqual((first["source_kind"], first["plate_script"], first["loc_method"], first["confidence"]), ("csv", "latin", "source_supplied", 0.93))
        self.assertEqual((first["observed_utc"], first["source_tz"], first["time_basis"]), ("2026-03-01T05:00:00+00:00", "Asia/Karachi", "declared"))
        self.assertEqual((first["camera_id"], first["camera_label"], first["lane"], first["lat"], first["lon"]), ("CAM-01", "Camera 1", "Lane 1", 31.512, 74.317))
        self.assertEqual(first["raw_payload"]["Direction"], "Towards", "the direction has no column: it is kept")
        self.assertEqual((sink.calls[0][0], "start_utc" in sink.calls[0][1]), ("plate_sighting", False))

    def test_a_second_run_of_the_same_camera_file_inserts_nothing(self):
        sink = PlateSink()
        first, _ = self.ingest_plates(self.GOOD, sink=sink)
        again, _ = self.ingest_plates(self.GOOD, sink=sink)
        self.assertEqual((first["rows_inserted"], again["rows_inserted"], len(sink.rows)), (3, 0, 3))

    def test_a_missing_plate_is_quarantined_and_a_sentinel_is_not_a_plate(self):
        rows = self.GOOD + [sighting("2026-03-01 12:00:00", ""), sighting("2026-03-01 12:01:00", "-")]
        out, sink = self.ingest_plates(rows)
        sheet = out["sheets"][0]
        self.assertEqual((sheet["loaded"], sheet["quarantined"]), (3, {"quarantined:plate_missing": 2}))

    def test_a_confidence_in_percent_is_a_fraction_and_flagged_and_an_impossible_one_is_dropped_and_flagged(self):
        rows = [sighting("2026-03-01 10:00:00", "SYNT-0001", confidence="93"), sighting("2026-03-01 10:01:00", "SYNT-0002", confidence="250"),
                sighting("2026-03-01 10:02:00", "SYNT-0003", confidence="x"), sighting("2026-03-01 10:03:00", "SYNT-0004", confidence="")]
        _, sink = self.ingest_plates(rows)
        got = [(r["confidence"], r["quality_flags"]) for r in sink.rows]
        self.assertEqual(got[0], (0.93, ["confidence_percent"]))
        self.assertEqual(got[1], (None, ["confidence_out_of_range"]))
        self.assertEqual(got[2], (None, ["confidence_not_numeric"]))
        self.assertEqual(got[3], (None, []))

    def test_a_row_without_a_position_has_none_and_a_row_with_half_of_one_is_flagged(self):
        rows = [sighting("2026-03-01 10:00:00", "SYNT-0001", lat="", lon=""), sighting("2026-03-01 10:01:00", "SYNT-0002", lon="")]
        _, sink = self.ingest_plates(rows)
        self.assertEqual([(r["lat"], r["lon"], r["loc_method"]) for r in sink.rows], [(None, None, "none"), (None, None, "none")])
        self.assertIn("coordinate_incomplete", sink.rows[1]["quality_flags"])

    def test_without_a_declared_zone_no_sighting_is_given_a_time(self):
        out, sink = self.ingest_plates(self.GOOD, policy=Policy())
        self.assertEqual((out["sheets"][0]["loaded"], len(sink.rows)), (0, 0))

    def test_the_table_has_its_own_time_column_and_key(self):
        self.assertEqual(ti.required_columns("plate_sighting")[-1], "observed_utc")
        self.assertEqual(ti.required_columns("comm_event")[-1], "start_utc")
        self.assertIn("ON CONFLICT (tenant_id, collection_id, file_id, row_hash, observed_utc) DO NOTHING", ti.insert_sql("plate_sighting", ["tenant_id"]))
        sql = (Path(__file__).resolve().parents[3] / "db" / "forensic_records" / "012_typed_families.sql").read_text(encoding="utf-8")
        self.assertIn("UNIQUE (" + ti.conflict_key("plate_sighting") + ")", sql)


class WorkerHookTests(Workdir):
    def job(self):
        fields = {"job_id": BATCH, "tenant_id": "tenant-1", "user_id": "u", "collection_id": "case-1", "file_id": "calls.csv", "source_file": "calls.csv",
                  "object_key": "k", "storage_path": "p", "record_type": "auto", "metadata": {"source_timezone": "Asia/Karachi"}, "evidence_id": EVIDENCE}
        import dataclasses
        wanted = {f.name: fields[f.name] if f.name in fields else "" for f in dataclasses.fields(worker.IngestJob)
                  if f.default is dataclasses.MISSING or f.name in fields}
        return worker.IngestJob(**wanted)

    def test_off_means_the_module_is_not_even_imported(self):
        for value in ("", "off", "OFF", "0", "false", "no"):
            with mock.patch.dict(os.environ, {"FORENSIC_INGEST_V2": value}), mock.patch.object(worker, "audit") as audit,                     mock.patch.object(ti, "run_shadow") as run:
                conn = FakeConn()
                self.assertIsNone(worker.run_typed_ingest_shadow(conn, mock.Mock(), self.job(), Path("x"), {}))
            run.assert_not_called()
            audit.assert_not_called()
            self.assertEqual(conn.log, [])

    def test_shadow_audits_the_summary_in_its_own_savepoint(self):
        conn, cur = FakeConn(), mock.Mock()
        summary = {"status": "loaded", "rows_loaded": 3, "rows_inserted": 3}
        with mock.patch.dict(os.environ, {"FORENSIC_INGEST_V2": "shadow"}), mock.patch.object(ti, "run_shadow", return_value=summary) as run, \
                mock.patch.object(worker, "audit") as audit:
            worker.run_typed_ingest_shadow(conn, cur, self.job(), Path("x"), {"classification": {"time_policy": {"source_timezone": "UTC"}}})
        lineage = run.call_args.args[2]
        self.assertEqual((lineage.tenant_id, lineage.collection_id, lineage.evidence_id, lineage.file_id, lineage.batch_id),
                         ("tenant-1", "case-1", EVIDENCE, "calls.csv", BATCH))
        self.assertEqual(run.call_args.args[3], {"source_timezone": "UTC"})
        self.assertEqual(audit.call_args.args[2], "records.ingest.typed_shadow")
        self.assertEqual(audit.call_args.args[3]["typed_ingest"], summary)
        self.assertEqual(conn.log, ["begin", "commit"])

    def test_anything_that_goes_wrong_in_the_hook_leaves_the_job_alone(self):
        conn = FakeConn()
        with mock.patch.dict(os.environ, {"FORENSIC_INGEST_V2": "shadow"}), mock.patch.object(ti, "run_shadow", side_effect=RuntimeError("boom")):
            with self.assertLogs(worker.LOG, "ERROR"):
                self.assertIsNone(worker.run_typed_ingest_shadow(conn, mock.Mock(), self.job(), Path("x"), {}))
        with mock.patch.dict(os.environ, {"FORENSIC_INGEST_V2": "shadow"}), mock.patch.object(ti, "run_shadow", return_value={"status": "loaded"}), \
                mock.patch.object(worker, "audit", side_effect=RuntimeError("audit failed")):
            with self.assertLogs(worker.LOG, "ERROR"):
                worker.run_typed_ingest_shadow(conn, mock.Mock(), self.job(), Path("x"), {})
        self.assertEqual(conn.log[-2:], ["begin", "rollback"], "a failed audit rolls back its own savepoint only")

    def test_the_document_branch_offers_text_exports_after_the_document_pipeline_and_before_the_commit(self):
        source = inspect.getsource(worker.process_job)
        document = source.index("process_document_job(conn, job, path)")
        offered = source.index("run_typed_ingest_shadow_for_text(conn, job, path, document_profile)")
        self.assertLess(document, offered)
        self.assertLess(offered, source.index("conn.commit()", document))
        self.assertLess(offered, source.index('modality in {"image", "audio", "video"}'), "inside the document branch, not after it")

    def test_only_text_exports_are_offered_judged_by_the_original_file_name(self):
        names = (("ipdr.txt", True), ("IPDR.TXT", True), ("calls.csv", True), ("calls.tsv", True), ("notes.pdf", False), ("memo.docx", False),
                 ("readme.md", False), ("mail.eml", False), ("noextension", False), ("", False), ("archive.txt.zip", False))
        for name, offered in names:
            job = dataclasses.replace(self.job(), source_file=name)
            with mock.patch.object(worker, "run_typed_ingest_shadow") as run:
                worker.run_typed_ingest_shadow_for_text(FakeConn(), job, Path("x"), {"format": "txt"})
            self.assertEqual(run.called, offered, name)

    def test_a_text_export_is_offered_with_the_jobs_own_cursor_path_and_profile(self):
        job = dataclasses.replace(self.job(), source_file="ipdr.txt")
        profile = {"format": "txt"}
        with mock.patch.object(worker, "run_typed_ingest_shadow") as run:
            worker.run_typed_ingest_shadow_for_text(FakeConn(), job, Path("spool/object"), profile)
        conn, cur, offered_job, path, offered_profile = run.call_args.args
        self.assertEqual((offered_job, path, offered_profile), (job, Path("spool/object"), profile))

    def test_a_failure_in_the_text_helper_never_reaches_the_job(self):
        job = dataclasses.replace(self.job(), source_file="ipdr.txt")
        with mock.patch.object(worker, "run_typed_ingest_shadow", side_effect=RuntimeError("boom")):
            with self.assertLogs(worker.LOG, "ERROR"):
                self.assertIsNone(worker.run_typed_ingest_shadow_for_text(FakeConn(), job, Path("x"), {}))

    def test_a_none_summary_writes_no_audit_row(self):
        with mock.patch.dict(os.environ, {"FORENSIC_INGEST_V2": "shadow"}), mock.patch.object(ti, "run_shadow", return_value=None), \
                mock.patch.object(worker, "audit") as audit:
            worker.run_typed_ingest_shadow(FakeConn(), mock.Mock(), self.job(), Path("x"), {})
        audit.assert_not_called()

    def test_process_job_calls_the_hook_after_version_one_audits_and_before_it_commits(self):
        import inspect
        source = inspect.getsource(worker.process_job)
        self.assertLess(source.index('"records.ingest.completed"'), source.index("run_typed_ingest_shadow("))
        self.assertLess(source.index("run_typed_ingest_shadow("), source.index("conn.commit()\n            emit_progress(progress_cb, job, \"completed\", stats, profile"))


if __name__ == "__main__":
    unittest.main()
