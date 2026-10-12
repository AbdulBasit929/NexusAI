"""Layout registry v1: entries, matching, reference interpretation, and synthetic goldens.

Plain unittest (no third-party packages):  python -m unittest ingestion.forensic_records.tests.test_layout_registry

The goldens come from tools/real_data/make_golden.py, which writes files with the shape of each provider layout and values only
from reserved ranges, together with the truth it generated. These tests prove that the registry machinery and every entry agree
with each other. They do not prove an entry is right about the real file; tools/real_data/confirm_layout.py does that, from a
structure report of the real file, inside the vault.
"""

from __future__ import annotations

import contextlib
import copy
import io
import json
import re
import sys
import tempfile
import unittest
from datetime import datetime
from pathlib import Path
from unittest import mock

from ingestion.forensic_records.layout_registry import (
    Policy, Registry, definition_sha256, header_cells, interpret_record, interpret_rows, load_registry, match_header,
    normalize_header, rows_to_records, summarize, validate_entry,
)
from ingestion.forensic_records.layout_registry import __main__ as cli
from ingestion.forensic_records.layout_registry import spec as spec_module
from ingestion.forensic_records.layout_registry.matching import find_header_row
from ingestion.forensic_records.layout_registry.parties import classify_party, device_identifier, make_cell_key
from ingestion.forensic_records.layout_registry.registry import LAYOUT_DIR, LayoutRegistryError
from ingestion.forensic_records.layout_registry.timeparse import excel_serial_to_datetime, parse_time, to_utc

REPO = Path(__file__).resolve().parents[3]
TOOLS = REPO / "tools" / "real_data"
EXPECTED_IDS = {
    "appusage-a1", "call-c1-company", "call-c2-strt-tm", "call-c3-ufone-style", "call-c4-telenor-style", "geofence-c5-mobilink",
    "geofence-c6-telenor", "geofence-c7-ufone", "geofence-c8-zong", "ipdr-i1-jazz-ufdr", "ipdr-i2-telenor-export", "ipdr-i3-ufone-xls",
    "ipdr-i4-zong-txt",
}
KARACHI = Policy(source_tz="Asia/Karachi", tz_basis="assumed_by_profile")

try:
    sys.path.insert(0, str(TOOLS))
    import make_golden  # noqa: E402
    import profile_structure  # noqa: E402
    import confirm_layout  # noqa: E402
    import scan_for_real_data  # noqa: E402
    TOOLS_AVAILABLE = True
except Exception:  # pragma: no cover - the worker image carries no tools folder
    TOOLS_AVAILABLE = False

REGISTRY = load_registry()
ALLOW = {"msisdn": set(), "ips": set(), "paths": [], "ignore_paths": []}


def entry_copy(layout_id="call-c4-telenor-style"):
    return copy.deepcopy(REGISTRY.get(layout_id))


class EntryTests(unittest.TestCase):
    def test_thirteen_layouts_load_and_validate(self):
        self.assertEqual(set(REGISTRY.ids()), EXPECTED_IDS)
        for entry in REGISTRY.entries:
            with self.subTest(layout=entry["layout_id"]):
                self.assertEqual(validate_entry(entry), [])
                self.assertEqual(entry["status"], "draft")
                self.assertTrue(entry["confirmation"]["unconfirmed"], "a draft layout states what is still unconfirmed")

    def test_entries_hold_no_real_looking_value(self):
        if not TOOLS_AVAILABLE:
            self.skipTest("tools/real_data not available")
        for path in sorted(Path(LAYOUT_DIR).glob("*.json")):
            with self.subTest(file=path.name):
                found = scan_for_real_data.scan_text(path.name, path.read_text(encoding="utf-8"), ALLOW)
                self.assertEqual([f.format() for f in found], [])

    def test_hash_covers_rules_not_life_cycle(self):
        entry = entry_copy()
        base = definition_sha256(entry)
        self.assertRegex(base, r"^[0-9a-f]{64}$")
        promoted = copy.deepcopy(entry)
        promoted["status"] = "retired"
        promoted["confirmation"]["unconfirmed"] = []
        self.assertEqual(definition_sha256(promoted), base)
        changed = copy.deepcopy(entry)
        changed["enums"][0]["map"]["INCOMING"] = {"direction": "outgoing"}
        self.assertNotEqual(definition_sha256(changed), base)

    def test_register_sql_covers_every_entry_and_guards_changed_definitions(self):
        sql = REGISTRY.register_sql()
        for entry in REGISTRY.entries:
            self.assertIn("'%s'" % entry["layout_id"], sql)
            self.assertIn(REGISTRY.hashes[(entry["layout_id"], entry["layout_version"])], sql)
        self.assertIn("ON CONFLICT (layout_id, layout_version) DO NOTHING", sql)
        self.assertIn("changed without a version bump", sql)

    def run_cli(self, *argv):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = cli.main(list(argv))
        return code, out.getvalue()

    def test_command_line_validates_lists_and_writes_sql(self):
        code, text = self.run_cli("validate")
        self.assertEqual((code, text.strip()), (0, "LAYOUTS=PASS entries=13"))
        code, text = self.run_cli("list")
        self.assertEqual(code, 0)
        self.assertEqual(len(text.strip().splitlines()), 14, "a header line and one line per layout")
        for layout_id in EXPECTED_IDS:
            self.assertIn(layout_id, text)
        code, text = self.run_cli("sql")
        self.assertEqual((code, text), (0, REGISTRY.register_sql()))
        with tempfile.TemporaryDirectory() as d:
            target = Path(d) / "layouts.sql"
            self.assertEqual(self.run_cli("sql", "--out", str(target))[0], 0)
            raw = target.read_bytes()
            self.assertNotIn(b"\r", raw, "the file keeps LF line endings on Windows too")
            self.assertEqual(raw.decode("utf-8"), REGISTRY.register_sql())

    def test_command_line_reports_a_broken_registry(self):
        broken = LayoutRegistryError("a.v1.json: file name must be b.v1.json; c.v1.json: header.columns is empty")
        with mock.patch.object(cli, "load_registry", side_effect=broken):
            code, text = self.run_cli("validate")
        self.assertEqual(code, 1)
        self.assertIn("LAYOUTS=FAIL", text)
        self.assertIn("a.v1.json: file name must be b.v1.json", text)
        self.assertIn("c.v1.json: header.columns is empty", text)

    def test_vocabularies_match_migration_012(self):
        sql = (REPO / "db" / "forensic_records" / "012_typed_families.sql").read_text(encoding="utf-8")

        def values(constraint):
            m = re.search(r"CONSTRAINT\s+%s\s+CHECK\s*\((?:[^()]*?)\bIN\s*\(([^)]*)\)" % constraint, sql, re.S)
            self.assertIsNotNone(m, constraint)
            return tuple(re.findall(r"'([^']*)'", m.group(1)))

        self.assertEqual(values("layout_versions_family_chk"), spec_module.FAMILIES)
        self.assertEqual(values("layout_versions_kind_chk"), spec_module.DATASET_KINDS)
        self.assertEqual(values("layout_versions_status_chk"), spec_module.STATUSES)
        self.assertEqual(values("comm_event_event_type_chk"), spec_module.EVENT_TYPES)
        self.assertEqual(values("comm_event_direction_chk"), spec_module.DIRECTIONS)
        self.assertEqual(values("comm_event_a_kind_chk"), spec_module.NUMBER_KINDS)
        self.assertEqual(values("comm_event_time_basis_chk"), spec_module.TIME_BASES)

    def test_header_normalisation_vectors(self):
        vectors = {"Sr #": "sr", "A-Party": "a_party", "Date & Time": "date_time", "Uplink Traffic(Byte)": "uplink_traffic_byte",
                   "cell/eci": "cell_eci", "Duration (milliseconds)": "duration_milliseconds", "  MACAddress ": "macaddress",
                   "CALL_ORIG_NUM": "call_orig_num", "Cell_ID": "cell_id", "No.": "no", "": "", None: ""}
        for raw, expected in vectors.items():
            self.assertEqual(normalize_header(raw), expected, repr(raw))

    def test_header_normalisation_matches_the_worker(self):
        try:
            from ingestion.forensic_records.worker import normalize_header as worker_normalize
        except Exception:  # the worker's own dependencies may be absent on a bare interpreter
            self.skipTest("worker module not importable here")
        names = ["Sr #", "A-Party", "Date & Time", "Uplink Traffic(Byte)", "cell/eci", "Duration (milliseconds)", "No.", "  MACAddress ", "", None]
        for entry in REGISTRY.entries:
            names += [c["name"] for c in entry["header"]["columns"]]
        for name in names:
            self.assertEqual(normalize_header(name), worker_normalize("" if name is None else name), repr(name))

    def test_loader_rejects_duplicates_and_misnamed_files(self):
        with tempfile.TemporaryDirectory() as d:
            entry = entry_copy()
            Path(d, "wrong-name.json").write_text(json.dumps(entry), encoding="utf-8")
            with self.assertRaisesRegex(LayoutRegistryError, "file name must be"):
                load_registry(d)
            Path(d, "wrong-name.json").unlink()
            Path(d, "call-c4-telenor-style.v1.json").write_text(json.dumps(entry), encoding="utf-8")
            Path(d, "extra.json").write_text(json.dumps(entry), encoding="utf-8")
            with self.assertRaises(LayoutRegistryError):
                load_registry(d)


class ValidationTests(unittest.TestCase):
    def problems(self, mutate, layout_id="call-c4-telenor-style"):
        entry = entry_copy(layout_id)
        mutate(entry)
        return validate_entry(entry)

    def assertProblem(self, text, mutate, layout_id="call-c4-telenor-style"):
        self.assertTrue(any(text in p for p in self.problems(mutate, layout_id)), "%r not in %r" % (text, self.problems(mutate, layout_id)))

    def test_rejections(self):
        self.assertProblem("layout_id", lambda e: e.update(layout_id="Call_C4"))
        self.assertProblem("family must be", lambda e: e.update(family="telepathy"))
        self.assertProblem("status must be", lambda e: e.update(status="final"))
        self.assertProblem("is not in header.columns", lambda e: e["header"]["required"].append("Nope"))
        self.assertProblem("is not a field of dataset kind", lambda e: e["fields"].update(public_ip="IMSI"))
        self.assertProblem("not one of", lambda e: e["enums"][0]["map"].update(INCOMING={"direction": "sideways"}))
        self.assertProblem("not an enumerated field", lambda e: e["enums"][0]["map"].update(INCOMING={"speed": "fast"}))
        self.assertProblem("confirmed must be", lambda e: e["enums"][0].pop("confirmed"))
        self.assertProblem("time.formats", lambda e: e["time"].update(formats=["julian"]))
        self.assertProblem("date_order must be stated", lambda e: e["time"].pop("date_order") or e["time"].update(formats=["slash_24h_seconds"]))
        self.assertProblem("source_tz", lambda e: e["time"].update(source_tz="Karachi"))
        self.assertProblem("an active layout cannot have unconfirmed items", lambda e: e.update(status="active"))
        self.assertProblem("from_times needs", lambda e: e["time"]["columns"].pop("end"))
        self.assertProblem("occurrence", lambda e: e["header"]["columns"][1].update(occurrence=3))
        self.assertProblem("golden.columns names", lambda e: e["golden"]["columns"].update(Nope="empty"))
        self.assertProblem("operator_label", lambda e: e.update(operator_label=7))
        self.assertProblem("container.kinds", lambda e: e["container"].update(kinds=["pdf"]))
        self.assertProblem("min_score", lambda e: e["header"].update(min_score=2))

    def test_active_layout_needs_everything_resolved(self):
        entry = entry_copy()
        entry["status"] = "active"
        entry["confirmation"]["unconfirmed"] = []
        self.assertEqual(validate_entry(entry), [])


# --------------------------------------------------------------------------------------------------------------------------
# Values
# --------------------------------------------------------------------------------------------------------------------------
class PartyTests(unittest.TestCase):
    S = "923000000001"          # reserved synthetic block

    def test_classification(self):
        cases = [
            (self.S, ("person_like", self.S, [])),
            ("0" + self.S[2:], ("person_like", self.S, [])),
            (self.S[2:], ("person_like", self.S, [])),
            ("+" + self.S, ("person_like", self.S, [])),
            ("00" + self.S, ("person_like", self.S, [])),
            (self.S + ".0", ("person_like", self.S, [])),
            ("92 300 0000001", ("person_like", self.S, [])),
            ("786", ("short_code", None, [])),
            ("667788", ("short_code", None, [])),
            ("*123#", ("short_code", None, ["ussd_code"])),
            ("BankA", ("sender_id", None, [])),
            ("4a123b5c", ("sender_id", None, [])),
            ("INTERNET", ("service_label", None, [])),
            ("Unknown", ("service_label", None, [])),
            ("gprs", ("service_label", None, [])),
            ("0099912345678", ("international", "99912345678", [])),
            ("+99912345678", ("international", "99912345678", [])),
            ("1234567", ("unknown", None, ["party_format_unrecognized"])),
            ("9212345678", ("unknown", None, ["party_format_unrecognized"])),
            ("", (None, None, ["party_missing"])),
            ("?", (None, None, ["party_sentinel"])),
            ("-1", (None, None, ["party_sentinel"])),
            ("0", (None, None, ["party_sentinel"])),
        ]
        for raw, expected in cases:
            with self.subTest(raw=raw):
                self.assertEqual(classify_party(raw), expected)

    def test_device_identifiers(self):
        fifteen = "001234560000009"
        self.assertEqual(device_identifier(fifteen, "imei"), (fifteen, "00123456", []))
        self.assertEqual(device_identifier(fifteen[:14], "imei"), (fifteen[:14], "00123456", []))
        self.assertEqual(device_identifier(fifteen[:13], "imei"), (fifteen[:13], None, ["imei_length_13"]))
        self.assertEqual(device_identifier(fifteen + "1", "imei"), (fifteen + "1", "00123456", []))
        self.assertEqual(device_identifier(fifteen + ".0", "imei"), (fifteen, "00123456", []))
        self.assertEqual(device_identifier("abc", "imei"), (None, None, ["imei_not_numeric"]))
        self.assertEqual(device_identifier("", "imei"), (None, None, []))
        self.assertEqual(device_identifier("001010000000001", "imsi"), ("001010000000001", None, []))
        self.assertEqual(device_identifier("00101000000001", "imsi"), ("00101000000001", None, ["imsi_length_14"]))

    def test_cell_key_vectors(self):
        """The same vectors are checked against forensic.make_cell_key in ops/013_reference_tables/verify.sql."""
        vectors = [
            (("41001", "0100", "000200"), "41001:100:200"), ((None, "100", "200"), "?:100:200"), (("41001", None, "200"), "41001:?:200"),
            (("41001", "100", None), None), (("41001", "100", "   "), None), ((" 41001 ", " 100 ", " 0 "), "41001:100:0"),
            (("41001", "AB1", "Cd2"), "41001:ab1:cd2"), (("00101", "100", "200"), "00101:100:200"),
        ]
        for args, expected in vectors:
            with self.subTest(args=args):
                self.assertEqual(make_cell_key(*args), expected)


class TimeTests(unittest.TestCase):
    def test_excel_serial(self):
        self.assertEqual(excel_serial_to_datetime("45000.5"), datetime(2023, 3, 15, 12, 0, 0))
        self.assertEqual(excel_serial_to_datetime("44927"), datetime(2023, 1, 1))
        self.assertEqual(excel_serial_to_datetime("45000.526168981481"), datetime(2023, 3, 15, 12, 37, 41))   # 45461 s of the day, 12 decimals
        self.assertIsNone(excel_serial_to_datetime("0.5"))
        self.assertIsNone(excel_serial_to_datetime("not a number"))

    def test_iso_and_offsets(self):
        r = parse_time("2026-01-05 10:00:00.123", ["iso_naive"])
        self.assertEqual((r.value, r.offset), (datetime(2026, 1, 5, 10, 0, 0, 123000), None))
        r = parse_time("2026-01-05T10:00:00+05:00", ["iso_naive"])
        self.assertEqual((r.value, r.offset), (datetime(2026, 1, 5, 10, 0, 0), "+05:00"))
        self.assertEqual(to_utc(r.value, "Asia/Karachi", r.offset).isoformat(), "2026-01-05T05:00:00+00:00")
        self.assertEqual(parse_time("2026-13-05 10:00:00", ["iso_naive"]).issue, "unparsed")

    def test_slash_dates_are_never_guessed(self):
        ambiguous = parse_time("03/04/2026 10:00:00", ["slash_24h_seconds"])
        self.assertEqual((ambiguous.value, ambiguous.issue), (None, "ambiguous_date_order"))
        self.assertEqual(parse_time("03/04/2026 10:00:00", ["slash_24h_seconds"], "DMY").value, datetime(2026, 4, 3, 10, 0, 0))
        self.assertEqual(parse_time("03/04/2026 10:00:00", ["slash_24h_seconds"], "MDY").value, datetime(2026, 3, 4, 10, 0, 0))
        proven = parse_time("25/04/2026 10:00:00", ["slash_24h_seconds"])
        self.assertEqual((proven.value, proven.order, proven.order_source), (datetime(2026, 4, 25, 10, 0, 0), "DMY", "structure_proven"))
        proven = parse_time("04/25/2026 10:00:00", ["slash_24h_seconds"])
        self.assertEqual((proven.value, proven.order), (datetime(2026, 4, 25, 10, 0, 0), "MDY"))
        self.assertEqual(parse_time("31/02/2026 10:00:00", ["slash_24h_seconds"], "DMY").issue, "unparsed")

    def test_minutes_and_dot_and_twelve_hour(self):
        self.assertEqual(parse_time("25/04/2026 10:07", ["slash_24h_minutes", "slash_24h_seconds"]).value, datetime(2026, 4, 25, 10, 7))
        self.assertIsNone(parse_time("25/04/2026 10:07:09", ["slash_24h_minutes"]).value)       # seconds present: not this format
        self.assertEqual(parse_time("05.01.2026 10:00:09", ["dot_dmy_24h_seconds"]).value, datetime(2026, 1, 5, 10, 0, 9))
        for text, expected in (("4/25/2026 12:30:00 AM", (0, 30)), ("4/25/2026 12:30:00 PM", (12, 30)), ("4/25/2026 1:05:09 PM", (13, 5)),
                               ("4/25/2026 11:59:59 AM", (11, 59))):
            r = parse_time(text, ["slash_12h_seconds"], "MDY")
            self.assertEqual((r.value.hour, r.value.minute), expected, text)
        self.assertIsNone(parse_time("4/25/2026 13:00:00 PM", ["slash_12h_seconds"], "MDY").value)

    def test_utc_conversion_and_unknown_zone(self):
        self.assertEqual(to_utc(datetime(2026, 1, 5, 10, 0, 0), "Asia/Karachi").isoformat(), "2026-01-05T05:00:00+00:00")
        self.assertEqual(to_utc(datetime(2026, 1, 5, 10, 0, 0), "UTC").isoformat(), "2026-01-05T10:00:00+00:00")
        with self.assertRaises(ValueError):
            to_utc(datetime(2026, 1, 5, 10, 0, 0), "Nowhere/Land")


# --------------------------------------------------------------------------------------------------------------------------
# Matching
# --------------------------------------------------------------------------------------------------------------------------
@unittest.skipUnless(TOOLS_AVAILABLE, "tools/real_data not available")
class MatchingTests(unittest.TestCase):
    def golden(self, layout_id, **kw):
        return make_golden.generate(REGISTRY.get(layout_id), seed=3, **kw)

    def test_each_golden_header_matches_only_its_own_layout(self):
        for entry in REGISTRY.entries:
            with self.subTest(layout=entry["layout_id"]):
                g = make_golden.generate(entry, seed=1)
                report = REGISTRY.find_header(g.table)
                self.assertEqual(report.decision, "match", [m.summary() for m in report.candidates[:3]])
                self.assertEqual(report.best.layout_id, entry["layout_id"])
                self.assertEqual(report.header_row, g.header_index)
                self.assertEqual(report.best.score, 1.0)
                self.assertEqual(report.best.missing_required, [])
                self.assertEqual([m.layout_id for m in report.candidates if m.qualifies], [entry["layout_id"]])

    def test_close_relatives_are_told_apart(self):
        """C4 and C6 differ by three columns and a header typo; C1 contains all of C5's columns."""
        for own, other in (("call-c4-telenor-style", "geofence-c6-telenor"), ("geofence-c6-telenor", "call-c4-telenor-style"),
                           ("call-c1-company", "geofence-c5-mobilink"), ("geofence-c5-mobilink", "call-c1-company")):
            g = self.golden(own)
            by_id = {m.layout_id: m for m in REGISTRY.find_header(g.table).candidates}
            self.assertTrue(by_id[own].qualifies and not by_id[other].qualifies, (own, other, by_id[other].summary()))

    def test_header_row_is_found_with_and_without_preamble(self):
        with_preamble = self.golden("call-c2-strt-tm")
        self.assertEqual(REGISTRY.find_header(with_preamble.table).header_row, 3)
        twin = with_preamble.table[with_preamble.header_index:]
        report = REGISTRY.find_header(twin)
        self.assertEqual((report.decision, report.header_row, report.best.layout_id), ("match", 0, "call-c2-strt-tm"))

    def test_declared_alias_covers_a_known_typo_and_is_reported(self):
        header = [c["name"] for c in REGISTRY.get("geofence-c6-telenor")["header"]["columns"]]
        header[header.index("CALL_ORIG_NUM")] = "call_org_num"
        report = REGISTRY.match(header)
        self.assertEqual((report.decision, report.best.layout_id), ("match", "geofence-c6-telenor"))
        self.assertEqual(report.best.by_alias, [("CALL_ORIG_NUM", "call_org_num")])

    def test_unknown_typo_is_a_near_miss_not_a_match(self):
        header = [c["name"] for c in REGISTRY.get("call-c4-telenor-style")["header"]["columns"]]
        header[header.index("CALL_START_DT_TM")] = "CALL_START_DT_TIM"
        report = REGISTRY.match(header)
        c4 = next(m for m in report.candidates if m.layout_id == "call-c4-telenor-style")
        self.assertFalse(c4.required_ok)
        self.assertIn(("CALL_START_DT_TM", "CALL_START_DT_TIM"), c4.near_misses)
        self.assertNotEqual(report.decision, "match")

    def test_extra_columns_lower_the_score_until_it_stops_qualifying(self):
        base = [c["name"] for c in REGISTRY.get("call-c4-telenor-style")["header"]["columns"]]
        one_more = REGISTRY.match(base + ["Extra One"])
        self.assertEqual((one_more.decision, one_more.best.layout_id), ("match", "call-c4-telenor-style"))
        self.assertLess(one_more.best.score, 1.0)
        self.assertIn("Extra One", one_more.best.extra_columns)
        self.assertEqual(REGISTRY.match(base + ["Extra One", "Extra Two", "Extra Three"]).decision, "none")

    def test_duplicate_header_is_matched_by_occurrence_in_both_spellings(self):
        entry = REGISTRY.get("geofence-c7-ufone")
        header = [c["name"] for c in entry["header"]["columns"]]
        self.assertEqual(header.count("Cell_ID"), 2)
        reader_style = list(header)
        reader_style[len(header) - 1 - header[::-1].index("Cell_ID")] = "Cell_ID__duplicate_2"
        for cells in (header, reader_style):
            report = REGISTRY.match(cells)
            self.assertEqual((report.decision, report.best.layout_id, report.best.score), ("match", "geofence-c7-ufone", 1.0))
        self.assertEqual(sorted(p for (n, o), p in report.best.positions.items() if n == "Cell_ID"), [8, 10])

    def test_blank_cells_and_noise_are_not_a_header(self):
        self.assertEqual(REGISTRY.match(["", None, "  "]).decision, "none")
        self.assertEqual(REGISTRY.match(["Alpha", "Beta", "Gamma", "Delta"]).decision, "none")
        rows = [["Title"], ["Report", "x"], [], ["a", "b", "c", "d", "e"]]
        self.assertEqual(find_header_row(REGISTRY.entries, rows).decision, "none")

    def test_two_entries_with_the_same_columns_are_ambiguous_not_arbitrary(self):
        a = entry_copy("geofence-c5-mobilink")
        b = copy.deepcopy(a)
        b["layout_id"] = "geofence-c5-twin"
        report = match_header([a, b], [c["name"] for c in a["header"]["columns"]])
        self.assertEqual(report.decision, "ambiguous")

    def test_draft_layouts_can_be_excluded(self):
        g = self.golden("call-c4-telenor-style")
        self.assertEqual(REGISTRY.match(g.header, include_draft=False).decision, "none")


# --------------------------------------------------------------------------------------------------------------------------
# Reference interpretation against the generated truth
# --------------------------------------------------------------------------------------------------------------------------
@unittest.skipUnless(TOOLS_AVAILABLE, "tools/real_data not available")
class InterpretTests(unittest.TestCase):
    def check_against_truth(self, entry, golden, policy):
        results = list(interpret_rows(entry, golden.header, golden.rows, policy=policy))
        self.assertEqual(len(results), len(golden.truth))
        for number, (result, truth) in enumerate(zip(results, golden.truth), start=1):
            where = "%s row %d (%s)" % (entry["layout_id"], number, truth["tag"])
            self.assertEqual(result.status, truth["status"], where)
            if truth["status"] == "quarantined":
                self.assertEqual(result.reason, truth["reason"], where)
            if truth["status"] == "accepted":
                for key, wanted in truth["record"].items():
                    self.assertEqual(result.record.get(key), wanted, "%s: %s" % (where, key))
                self.assertLessEqual(set(truth["flags"]), set(result.flags), where)
        return results

    def test_every_layout_reads_back_to_the_truth(self):
        for entry in REGISTRY.entries:
            with self.subTest(layout=entry["layout_id"]):
                golden = make_golden.generate(entry, seed=1)
                policy = Policy(source_tz="Asia/Karachi", tz_basis="assumed_by_profile", date_order=golden.date_order)
                results = self.check_against_truth(entry, golden, policy)
                summary = summarize(results)
                accepted = sum(1 for t in golden.truth if t["status"] == "accepted")
                self.assertEqual(summary["status"].get("accepted"), accepted)
                self.assertEqual(summary["status"].get("skipped"), 1)
                self.assertEqual(summary["quarantine_reasons"], {"blank_row": 1, "time_missing": 1, "time_unparsed": 1})

    def test_other_seeds_and_sizes_agree_too(self):
        for entry in REGISTRY.entries:
            for seed, rows in ((2, 20), (7, 100)):
                with self.subTest(layout=entry["layout_id"], seed=seed):
                    golden = make_golden.generate(entry, rows=rows, seed=seed)
                    policy = Policy(source_tz="Asia/Karachi", tz_basis="assumed_by_profile", date_order=golden.date_order)
                    self.check_against_truth(entry, golden, policy)

    def test_without_a_time_zone_rows_are_flagged_and_have_no_utc(self):
        entry = REGISTRY.get("call-c4-telenor-style")
        golden = make_golden.generate(entry, seed=1)
        for result in interpret_rows(entry, golden.header, golden.rows):
            if result.status == "accepted":
                self.assertIn("source_timezone_unknown", result.flags)
                self.assertIsNone(result.record["start_utc"])
                self.assertIsNone(result.record["source_tz"])
                self.assertIsNone(result.record["time_basis"])

    def test_with_a_time_zone_the_basis_is_recorded_and_utc_is_five_hours_earlier(self):
        entry = REGISTRY.get("call-c4-telenor-style")
        golden = make_golden.generate(entry, seed=1)
        result = next(r for r in interpret_rows(entry, golden.header, golden.rows, policy=KARACHI) if r.status == "accepted")
        self.assertEqual((result.record["source_tz"], result.record["time_basis"]), ("Asia/Karachi", "assumed_by_profile"))
        local = datetime.fromisoformat(result.record["start_local"])
        utc = datetime.fromisoformat(result.record["start_utc"])
        self.assertEqual((local - utc.replace(tzinfo=None)).total_seconds(), 5 * 3600)
        confirmed = Policy(source_tz="Asia/Karachi", tz_basis="confirmed_by_analyst")
        again = next(r for r in interpret_rows(entry, golden.header, golden.rows, policy=confirmed) if r.status == "accepted")
        self.assertEqual(again.record["time_basis"], "confirmed_by_analyst")

    def test_slash_dates_without_an_order_are_quarantined_or_proven_by_structure(self):
        for layout_id in ("call-c2-strt-tm", "geofence-c7-ufone", "geofence-c8-zong", "ipdr-i4-zong-txt"):
            with self.subTest(layout=layout_id):
                entry = REGISTRY.get(layout_id)
                golden = make_golden.generate(entry, seed=1, defects=False)
                results = list(interpret_rows(entry, golden.header, golden.rows, policy=KARACHI))
                reasons = {r.reason for r in results if r.status == "quarantined"}
                accepted = [r for r in results if r.status == "accepted"]
                self.assertEqual(reasons, {"ambiguous_date_order"})
                self.assertTrue(accepted, "rows with a day above 12 are settled by structure")
                for r in accepted:
                    day = datetime.fromisoformat(r.record["start_local"]).day
                    self.assertGreater(day, 12)
                resolved = list(interpret_rows(entry, golden.header, golden.rows, policy=Policy("Asia/Karachi", "assumed_by_profile", golden.date_order)))
                self.assertEqual({r.status for r in resolved}, {"accepted"})

    def test_unconfirmed_direction_codes_are_kept_as_unknown_and_flagged(self):
        entry = REGISTRY.get("geofence-c6-telenor")
        golden = make_golden.generate(entry, seed=1, defects=False)
        results = [r for r in interpret_rows(entry, golden.header, golden.rows, policy=KARACHI) if r.status == "accepted"]
        self.assertEqual({r.record["direction"] for r in results}, {"unknown"})
        for r in results:
            self.assertIn("enum_unconfirmed:direction", r.flags)

    def test_an_unmapped_label_is_flagged_and_never_guessed(self):
        entry = REGISTRY.get("call-c3-ufone-style")
        golden = make_golden.generate(entry, seed=1, defects=False)
        index = golden.header.index("Direction")
        row = list(golden.rows[0])
        row[index] = "SIDEWAYS"
        result = next(interpret_rows(entry, golden.header, [row], policy=KARACHI))
        self.assertEqual(result.record["direction"], "unknown")
        self.assertIn("unmapped_label:direction", result.flags)

    def test_counterparty_follows_the_direction(self):
        entry = REGISTRY.get("call-c4-telenor-style")
        record = {"MSISDN": "923000000001", "call_org_num": "923000000002", "CALL_DIALED_NUM": "923000000003", "INBOUND_OUTBOUND_IND": "INCOMING",
                  "CALL_TYPE": "CALL", "CALL_START_DT_TM": "2026-01-05 10:00:00", "CALL_END_DT_TM": "2026-01-05 10:01:00"}
        incoming = interpret_record(entry, record, policy=KARACHI)
        self.assertEqual((incoming.record["b_raw"], incoming.record["duration_s"]), ("923000000002", 60))
        record["INBOUND_OUTBOUND_IND"] = "OUTGOING"
        self.assertEqual(interpret_record(entry, record, policy=KARACHI).record["b_raw"], "923000000003")
        record["INBOUND_OUTBOUND_IND"] = "UNKNOWN-WORD"
        unresolved = interpret_record(entry, record, policy=KARACHI)
        self.assertIsNone(unresolved.record["b_raw"])
        self.assertIn("counterparty_direction_unresolved", unresolved.flags)

    def test_counterparty_equal_to_subject_is_dropped(self):
        entry = REGISTRY.get("geofence-c6-telenor")
        record = {"MSISDN": "923000000001", "CALL_ORIG_NUM": "923000000001", "INBOUND_OUTBOUND_IND": "1", "CALL_TYPE": "CALL",
                  "CALL_START_DT_TM": "2026-01-05 10:00:00", "CALL_END_DT_TM": "2026-01-05 10:01:00"}
        result = interpret_record(entry, record, policy=KARACHI)
        self.assertEqual((result.record["b_raw"], result.record["b_kind"]), (None, None))
        self.assertIn("counterparty_equals_subject", result.flags)

    def test_numeric_codes_read_the_same_with_or_without_a_decimal_point(self):
        entry = REGISTRY.get("geofence-c6-telenor")
        for code in ("2", "2.0"):
            record = {"MSISDN": "923000000001", "CALL_ORIG_NUM": "786", "INBOUND_OUTBOUND_IND": code, "CALL_TYPE": "GPRS",
                      "CALL_START_DT_TM": "2026-01-05 10:00:00"}
            result = interpret_record(entry, record, policy=KARACHI)
            self.assertNotIn("unmapped_label:direction", result.flags)
            self.assertEqual(result.record["event_type"], "data")

    def test_duration_modes_and_end_before_start(self):
        c2 = REGISTRY.get("call-c2-strt-tm")
        record = {"CALL_TYPE": "Call - Outgoing", "MSISDN": "923000000001", "STRT_TM": "25/04/2026 10:00:00", "BNUMBER": "3000000002",
                  "MINS": "2", "SECS": "5", "LAC_ID": "100", "CELL_ID": "200", "IMEI": "001234560000009"}
        self.assertEqual(interpret_record(c2, record, policy=KARACHI).record["duration_s"], 125)
        record["MINS"], record["SECS"] = "x", "5"
        result = interpret_record(c2, record, policy=KARACHI)
        self.assertEqual(result.record["duration_s"], 5)
        self.assertIn("duration_not_numeric", result.flags)
        c4 = REGISTRY.get("call-c4-telenor-style")
        record = {"MSISDN": "923000000001", "CALL_DIALED_NUM": "923000000002", "INBOUND_OUTBOUND_IND": "OUTGOING", "CALL_TYPE": "CALL",
                  "CALL_START_DT_TM": "2026-01-05 10:05:00", "CALL_END_DT_TM": "2026-01-05 10:00:00"}
        result = interpret_record(c4, record, policy=KARACHI)
        self.assertIsNone(result.record["duration_s"])
        self.assertIn("negative_duration", result.flags)
        self.assertIn("end_before_start", result.flags)

    def test_noise_columns_are_flagged_and_kept_out_of_the_fields(self):
        entry = REGISTRY.get("call-c4-telenor-style")
        record = {"MSISDN": "923000000001", "CALL_DIALED_NUM": "923000000002", "INBOUND_OUTBOUND_IND": "OUTGOING", "CALL_TYPE": "CALL",
                  "CALL_START_DT_TM": "2026-01-05 10:00:00", "Description": "Example Virtual Adapter", "MACAddress": "00:00:5e:00:53:01"}
        result = interpret_record(entry, record, policy=KARACHI)
        self.assertIn("noise_columns_populated", result.flags)
        self.assertEqual(result.record["raw_payload"]["Description"], "Example Virtual Adapter")
        self.assertNotIn("Example Virtual Adapter", [v for k, v in result.record.items() if k != "raw_payload"])

    def test_null_tokens_and_protocol_numbers(self):
        i2 = REGISTRY.get("ipdr-i2-telenor-export")
        record = {"Start Time": "2026-01-05 10:00:00.250", "End Time": "2026-01-05 10:00:01.500", "Duration(ms)": "1250", "IMSI": "001010000000001",
                  "MSISDN": "923000000001", "Private Source IP": "10.1.2.3", "Private Source Port": "50000", "Protocol": "TCP",
                  "Public Source IP": "192.0.2.9", "Public Source Port": "0", "Dest IP": "198.51.100.7", "Dest Port": "443", "URL": "0"}
        result = interpret_record(i2, record, policy=KARACHI)
        self.assertIsNone(result.record["url"])
        self.assertEqual((result.record["protocol"], result.record["protocol_num"], result.record["public_port_from"]), ("tcp", None, 0))
        self.assertEqual(result.record["end_local"], "2026-01-05 10:00:01.500000")
        i3 = REGISTRY.get("ipdr-i3-ufone-xls")
        record = {"Public IP": "192.0.2.9", "Public Port Start": "1024", "Public Port End": "2047", "Log Date": "05.01.2026 10:00:09", "imei": "001234560000009",
                  "imsi": "001010000000001", "msisdn": "923000000001", "sid": "x1", "dst_address": "198.51.100.7", "dst_port": "443", "src_address": "10.1.2.3",
                  "src_port": "50000", "protocol": "6", "lac/tac": "12345", "cell/eci": "67890"}
        result = interpret_record(i3, record, policy=KARACHI)
        self.assertEqual((result.record["protocol"], result.record["protocol_num"]), ("tcp", 6))
        self.assertEqual((result.record["public_port_from"], result.record["public_port_to"]), (1024, 2047))
        record["protocol"] = "99"
        self.assertIn("protocol_number_unknown", interpret_record(i3, record, policy=KARACHI).flags)
        record["Public Port Start"], record["Public Port End"] = "3000", "2000"
        reversed_range = interpret_record(i3, record, policy=KARACHI)
        self.assertIn("public_port_range_reversed", reversed_range.flags)
        self.assertEqual((reversed_range.record["public_port_from"], reversed_range.record["public_port_to"]), (None, None))
        i1 = REGISTRY.get("ipdr-i1-jazz-ufdr")
        record = {"UFDR start time": "2026-01-05 10:00:00", "IMSI": "001010000000001", "MSISDN": "923000000001", "EGN Sub Protocol Type": "6"}
        self.assertEqual((interpret_record(i1, record, policy=KARACHI).record["protocol"], interpret_record(i1, record, policy=KARACHI).record["protocol_num"]), ("6", None))

    def test_positional_rows_and_reader_style_records_agree(self):
        entry = REGISTRY.get("geofence-c7-ufone")
        golden = make_golden.generate(entry, seed=1, defects=False)
        policy = Policy("Asia/Karachi", "assumed_by_profile", golden.date_order)
        positional = [r.record for r in interpret_rows(entry, golden.header, golden.rows, policy=policy)]
        reader_style = []
        for record in rows_to_records(golden.header, golden.rows):
            self.assertIn("Cell_ID__duplicate_2", record)
            reader_style.append(interpret_record(entry, record, policy=policy).record)
        self.assertEqual([p["cell_key"] for p in positional], [r["cell_key"] for r in reader_style])
        self.assertEqual([p["cell_raw"] for p in positional], [r["cell_raw"] for r in reader_style])

    def test_summary_aggregates_match_the_truth(self):
        entry = REGISTRY.get("call-c3-ufone-style")
        golden = make_golden.generate(entry, seed=1)
        results = list(interpret_rows(entry, golden.header, golden.rows, policy=KARACHI))
        summary = summarize(results)
        truth_accepted = [t["record"] for t in golden.truth if t["status"] == "accepted"]
        self.assertEqual(sum(summary["histograms"]["a_kind"].values()), len(truth_accepted))
        starts = sorted(t["start_local"] for t in truth_accepted)
        self.assertEqual((summary["first_start_local"], summary["last_start_local"]), (starts[0], starts[-1]))
        self.assertEqual(summary["distinct"]["a_raw"], len({t["a_raw"] for t in truth_accepted}))


# --------------------------------------------------------------------------------------------------------------------------
# The generated files themselves
# --------------------------------------------------------------------------------------------------------------------------
@unittest.skipUnless(TOOLS_AVAILABLE, "tools/real_data not available")
class GoldenFileTests(unittest.TestCase):
    def test_goldens_contain_only_reserved_values(self):
        for entry in REGISTRY.entries:
            with self.subTest(layout=entry["layout_id"]):
                golden = make_golden.generate(entry, seed=5)
                text = "\n".join(",".join(str(c) for c in row) for row in golden.table)
                found = scan_for_real_data.scan_text(entry["layout_id"] + ".csv", text, ALLOW)
                self.assertEqual([f.format() for f in found], [])

    def test_generation_is_deterministic(self):
        for entry in REGISTRY.entries[:4]:
            a = make_golden.generate(entry, seed=11)
            b = make_golden.generate(entry, seed=11)
            c = make_golden.generate(entry, seed=12)
            self.assertEqual(a.table, b.table)
            self.assertNotEqual(a.table, c.table)

    def test_files_are_written_read_back_and_confirm_against_their_own_profile(self):
        """golden file -> structure profile (the D1 tool) -> confirm_layout: the whole toolchain on synthetic input."""
        with tempfile.TemporaryDirectory() as d:
            for entry in REGISTRY.entries:
                with self.subTest(layout=entry["layout_id"]):
                    golden = make_golden.generate(entry, seed=4, defects=False)
                    path = golden.write(d)
                    report = profile_structure.build_report(path, "shareable", None, False, 0, 200000)
                    sheet = report["sheets"][0]
                    self.assertEqual(sheet["header_row_index"], golden.header_index)
                    result = confirm_layout.confirm(entry, report)
                    self.assertEqual(result.failed, 0, result.text())
                    self.assertNotIn("FAIL", result.text())

    def test_confirm_fails_when_the_file_differs_from_the_entry(self):
        with tempfile.TemporaryDirectory() as d:
            entry = REGISTRY.get("call-c4-telenor-style")
            golden = make_golden.generate(entry, seed=4, defects=False)
            path = golden.write(d)
            report = profile_structure.build_report(path, "shareable", None, False, 0, 200000)
            wrong = REGISTRY.get("geofence-c6-telenor")
            result = confirm_layout.confirm(wrong, report)
            self.assertGreater(result.failed, 0)
            self.assertIn("header names", result.text())
            # a label the entry does not map
            label_column = next(c for c in report["sheets"][0]["columns"] if c["header"] == "CALL_TYPE")
            label_column["vocabulary"]["VIDEO"] = 0.1
            result = confirm_layout.confirm(entry, report)
            self.assertIn("not mapped by the entry: ['VIDEO']", result.text())

    def test_command_lines(self):
        import io
        from contextlib import redirect_stdout
        with tempfile.TemporaryDirectory() as d:
            buf = io.StringIO()
            with redirect_stdout(buf):
                self.assertEqual(make_golden.main(["--list"]), 0)
                self.assertEqual(make_golden.main(["--all", "--out", d, "--rows", "12"]), 0)
            self.assertEqual(len(list(Path(d).iterdir())), 13)
            self.assertEqual(set(buf.getvalue().split()) & EXPECTED_IDS, EXPECTED_IDS)


if __name__ == "__main__":
    unittest.main()
