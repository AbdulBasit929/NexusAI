"""Subscriber registrations through the typed ingest (docs/data/subscriber-device-family-20261028.md, pieces a to c), tested without a database.

Plain unittest:  python -m unittest ingestion.forensic_records.tests.test_subscriber_family

The rule that matters most is the privacy one: a name, a national ID number and an address of a source file are recognised, counted and DROPPED, and nothing else carries them into a row.
"""

from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

from ingestion.forensic_records import typed_ingest as ti
from ingestion.forensic_records.layout_registry import Policy, interpret_rows, load_registry
from ingestion.forensic_records.layout_registry.parties import normalize_iccid, normalize_tac

REPO = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(REPO / "tools" / "real_data"))
import make_golden  # noqa: E402
import reserved  # noqa: E402

REGISTRY = load_registry()
ENTRY = REGISTRY.get("subscriber-s1")
KARACHI = Policy(source_tz="Asia/Karachi", tz_basis="declared", date_order="DMY")
EVIDENCE = "5a000000-0000-4000-8000-0000000000a1"
BATCH = "6a000000-0000-4000-8000-0000000000b1"
LINEAGE = ti.Lineage("tenant-1", "case-1", EVIDENCE, "subscribers.csv", BATCH)
COLUMNS = ("tenant_id", "collection_id", "evidence_id", "file_id", "batch_id", "source_row", "row_hash", "layout_id", "layout_version", "msisdn", "msisdn_raw",
           "subscriber_ref", "imsi", "iccid", "status", "operator_label", "service_type", "activated_at", "deactivated_at", "valid_from", "valid_to", "source_tz", "time_basis",
           "raw_payload_redacted", "quality_flags")


class Sink:
    """Keeps rows under the key of the real table (the table's own UNIQUE: tenant, collection, file, row hash)."""

    def __init__(self):
        self.rows, self.keys = [], set()

    def table_columns(self, table):
        return [(c, "text") for c in COLUMNS]

    def insert_batch(self, table, columns, rows):
        new = 0
        for row in rows:
            key = tuple(row[c.strip()] for c in ti.conflict_key(table).split(","))
            if key not in self.keys:
                self.keys.add(key)
                self.rows.append(row)
                new += 1
        return new


def golden_file(directory, rows=36, seed=3, defects=True):
    return make_golden.generate(ENTRY, rows=rows, seed=seed, defects=defects).write(directory)


class NormalisersTests(unittest.TestCase):
    def test_iccid(self):
        good = reserved.synthetic_iccid(7)
        self.assertEqual(normalize_iccid(good), (good, []))
        self.assertEqual(normalize_iccid(good + "F"), (good, []))                      # a trailing filler
        self.assertEqual(normalize_iccid(good[:6] + " " + good[6:] + ".0"), (None, ["iccid_not_numeric"]))
        self.assertEqual(normalize_iccid(good[:-1] + str((int(good[-1]) + 1) % 10)), (good[:-1] + str((int(good[-1]) + 1) % 10), ["iccid_luhn_fail"]))
        self.assertEqual(normalize_iccid("8.99E+18"), (None, ["iccid_not_numeric"]))
        self.assertEqual(normalize_iccid("12345"), ("12345", ["iccid_length_5"]))
        self.assertEqual(normalize_iccid(""), (None, []))

    def test_tac(self):
        self.assertEqual(normalize_tac("35290611"), ("35290611", []))
        self.assertEqual(normalize_tac("5290611"), ("05290611", ["tac_padded"]))          # a spreadsheet dropped the leading zero
        self.assertEqual(normalize_tac("5290611.0"), ("05290611", ["tac_padded"]))
        self.assertEqual(normalize_tac("529061"), (None, ["tac_length_6"]))
        self.assertEqual(normalize_tac("3529061X"), (None, ["tac_not_numeric"]))
        self.assertEqual(normalize_tac(""), (None, ["tac_missing"]))

    def test_synthetic_iccid_is_reserved_and_valid(self):
        self.assertTrue(all(reserved.is_reserved_iccid(reserved.synthetic_iccid(n)) for n in range(50)))
        self.assertFalse(reserved.is_reserved_iccid("8992" + "0" * 15))


class WriterTests(unittest.TestCase):
    def test_a_registration_has_no_time_in_its_key_and_needs_none(self):
        self.assertEqual(ti.conflict_key("subscriber_registration"), "tenant_id, collection_id, file_id, row_hash")
        self.assertNotIn("start_utc", ti.required_columns("subscriber_registration"))
        self.assertEqual(ti.conflict_key("plate_sighting"), "tenant_id, collection_id, file_id, row_hash, observed_utc")    # the others are unchanged
        self.assertEqual(ti.conflict_key("comm_event"), "tenant_id, collection_id, file_id, row_hash, start_utc")
        self.assertEqual(ti.TABLE_FOR_FAMILY["subscriber"], "subscriber_registration")

    def test_the_golden_file_is_written_once_and_a_second_ingest_writes_nothing(self):
        with tempfile.TemporaryDirectory() as directory:
            path = golden_file(directory)
            sink = Sink()
            out = ti.ingest_file(path, LINEAGE, KARACHI, sink, registry=REGISTRY)
            sheet = out["sheets"][0]
            self.assertEqual((sheet["status"], sheet["layout_id"], sheet["table"]), ("loaded", "subscriber-s1", "subscriber_registration"))
            self.assertTrue(sheet["reconciles"])
            self.assertEqual(sheet["inserted"], len(sink.rows))
            self.assertGreater(len(sink.rows), 20)
            again = ti.ingest_file(path, LINEAGE, KARACHI, sink, registry=REGISTRY)["sheets"][0]
            self.assertEqual((again["inserted"], again["already_present"]), (0, again["loaded"]))

    def test_identity_is_recognised_counted_and_never_stored(self):
        with tempfile.TemporaryDirectory() as directory:
            path = golden_file(directory)
            sink = Sink()
            sheet = ti.ingest_file(path, LINEAGE, KARACHI, sink, registry=REGISTRY)["sheets"][0]
            stored = json.dumps(sink.rows, default=str)
            for n in range(1, 13):
                for value in (reserved.synthetic_name(n), reserved.synthetic_cnic(n), reserved.synthetic_address(n)):
                    self.assertNotIn(value, stored, "an identity value reached a row")
            for name in ("person_name", "national_id", "address"):
                self.assertEqual(sheet["flags"].get("identity_dropped:" + name), sheet["loaded"], name)    # every loaded row, every identity column
            self.assertTrue(all("identity_ref" not in row or row["identity_ref"] is None for row in sink.rows))

    def test_a_column_the_layout_does_not_declare_cannot_carry_an_identity(self):
        header = [c["name"] for c in ENTRY["header"]["columns"]] + ["Father Name"]
        row = ["923000000001", "SUB-1", "Synthetic Person 0001", "00000-0000001-1", "1 Synthetic Street", reserved.synthetic_imsi(1), reserved.synthetic_iccid(1), "Active",
               "2026-03-01 10:00:00", "", "Prepaid", "Operator 1", "Synthetic Parent 0001"]
        result = next(interpret_rows(ENTRY, header, [row], policy=KARACHI))
        self.assertEqual(result.status, "accepted")
        payload = result.record["raw_payload_redacted"]
        self.assertNotIn("Father Name", payload)
        self.assertNotIn("Synthetic Parent 0001", json.dumps(result.record, default=str))
        self.assertEqual(sorted(payload), ["Activation Date", "Deactivation Date", "ICCID", "IMSI", "MSISDN", "Operator", "Service Type", "Status", "Subscriber ID"])

    def test_dates_without_a_zone_leave_the_validity_open_and_are_flagged_but_the_row_is_kept(self):
        with tempfile.TemporaryDirectory() as directory:
            path = golden_file(directory, defects=False)
            sink = Sink()
            sheet = ti.ingest_file(path, LINEAGE, Policy(), sink, registry=REGISTRY)["sheets"][0]
            self.assertEqual((sheet["status"], sheet["loaded"]), ("loaded", sheet["rows_read"]))
            self.assertTrue(all(row["activated_at"] is None and row["valid_from"] is None for row in sink.rows))
            self.assertEqual(sheet["flags"].get("source_timezone_unknown"), sheet["loaded"])

    def test_with_a_zone_the_validity_is_in_utc_five_hours_before_the_local_time_and_the_basis_is_recorded(self):
        with tempfile.TemporaryDirectory() as directory:
            path = golden_file(directory, defects=False)
            sink = Sink()
            ti.ingest_file(path, LINEAGE, KARACHI, sink, registry=REGISTRY)
            dated = [r for r in sink.rows if r["activated_at"]]
            self.assertTrue(dated)
            self.assertTrue(all((r["source_tz"], r["time_basis"]) == ("Asia/Karachi", "declared") for r in dated))
            self.assertTrue(all(r["valid_from"] == r["activated_at"] and r["valid_to"] == r["deactivated_at"] for r in dated))
            deactivated = [r for r in dated if r["deactivated_at"]]
            self.assertTrue(deactivated)
            self.assertTrue(all(r["deactivated_at"] > r["activated_at"] for r in deactivated))

    def test_a_deactivation_before_the_activation_is_flagged_and_left_open(self):
        header = [c["name"] for c in ENTRY["header"]["columns"]]
        row = ["923000000001", "SUB-1", "", "", "", reserved.synthetic_imsi(1), reserved.synthetic_iccid(1), "Deactivated", "2026-03-10 10:00:00", "2026-03-01 10:00:00", "Prepaid", "Operator 1"]
        record = next(interpret_rows(ENTRY, header, [row], policy=KARACHI)).record
        self.assertIn("deactivated_before_activated", record["quality_flags"])
        self.assertIsNone(record["valid_to"])
        self.assertEqual(record["status"], "deactivated")

    def test_the_number_forms_are_one_number_and_an_unusable_number_is_refused(self):
        header = [c["name"] for c in ENTRY["header"]["columns"]]

        def read(number):
            row = [number, "S", "", "", "", "", "", "Active", "", "", "", ""]
            return next(interpret_rows(ENTRY, header, [row], policy=KARACHI))

        forms = ["923000000001", "03000000001", "+923000000001", "3000000001", "0092 300 0000001"]
        self.assertEqual({read(f).record["msisdn"] for f in forms}, {"923000000001"})
        refused = read("12")
        self.assertEqual((refused.status, refused.reason), ("quarantined", "msisdn_unrecognised"))
        self.assertEqual(read("").reason, "msisdn_missing")


if __name__ == "__main__":
    unittest.main()
