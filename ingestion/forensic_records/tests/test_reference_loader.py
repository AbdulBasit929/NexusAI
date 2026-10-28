"""The device reference loader (docs/data/subscriber-device-family-20261028.md, piece d), tested without a database.

Plain unittest:  python -m unittest ingestion.forensic_records.tests.test_reference_loader

The public TAC list (tools/family_data/fetch.py device, outside the repository) is read when it is present: its counts are the robustness check that only real data can give.
"""

from __future__ import annotations

import os
import tempfile
import unittest

from ingestion.forensic_records import reference_loader as rl
from ingestion.forensic_records import typed_ingest as ti

TAC_FILE = os.environ.get("NEXUSAI_FAMILY_DATA", "C:/tmp/nexusai-family-data") + "/tac/tac_full.csv"
COLUMNS = ("tenant_id", "tac", "source", "make", "model", "marketing_name", "device_type", "source_license", "retrieved_at", "notes", "created_at")


class Sink:
    def __init__(self):
        self.rows, self.keys = [], set()

    def table_columns(self, table):
        assert table == "device_tac"
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


def write(directory, lines, header="Brand,TAC,SPECS"):
    path = os.path.join(directory, "tac.csv")
    with open(path, "w", encoding="utf-8", newline="") as handle:
        handle.write(header + "\n" + "\n".join(lines) + "\n")
    return path


class LoaderTests(unittest.TestCase):
    def test_the_key_of_the_reference_table_is_its_own(self):
        self.assertEqual(ti.conflict_key("device_tac"), "tenant_id, tac, source")

    def test_what_is_loaded_padded_rejected_repeated_and_conflicting(self):
        lines = ["SYNTH,00000011,SYNTH PHONE 1, Variant A", "SYNTH,0000012,SYNTH PHONE 2", "SYNTH,000013,SYNTH PHONE 3", "SYNTH,,SYNTH PHONE 4", "SYNTH,0000001X,SYNTH PHONE 5",
                 "SYNTH,00000011,SYNTH PHONE 1, Variant A", "SYNTH,00000011,SYNTH PHONE ONE"]
        with tempfile.TemporaryDirectory() as directory:
            sink = Sink()
            counts = rl.load_tac_list(write(directory, lines), sink, source="synthetic-test", source_license="test data")
            self.assertEqual((counts["rows_read"], counts["accepted"], counts["inserted"], counts["already_present"]), (7, 2, 2, 0))
            self.assertEqual(counts["rejected"], {"tac_length_6": 1, "tac_missing": 1, "tac_not_numeric": 1})
            self.assertEqual((counts["flags"], counts["repeats"], counts["conflicts"]), ({"tac_padded": 1}, 1, 1))
            self.assertEqual(sorted(r["tac"] for r in sink.rows), ["00000011", "00000012"])
            first = next(r for r in sink.rows if r["tac"] == "00000011")
            self.assertEqual((first["make"], first["marketing_name"], first["source"], first["source_license"]), ("SYNTH", "SYNTH PHONE 1", "synthetic-test", "test data"))
            self.assertEqual(next(r for r in sink.rows if r["tac"] == "00000012")["notes"], "tac_padded")
            again = rl.load_tac_list(write(directory, lines), sink, source="synthetic-test", source_license="test data")
            self.assertEqual((again["inserted"], again["already_present"]), (0, 2))

    def test_another_source_is_another_row_and_never_overwrites(self):
        with tempfile.TemporaryDirectory() as directory:
            path = write(directory, ["SYNTH,00000011,SYNTH PHONE 1"])
            sink = Sink()
            rl.load_tac_list(path, sink, source="a", source_license="test data")
            rl.load_tac_list(write(directory, ["OTHER,00000011,OTHER NAME"]), sink, source="b", source_license="test data")
            self.assertEqual(sorted((r["source"], r["make"]) for r in sink.rows), [("a", "SYNTH"), ("b", "OTHER")])

    def test_a_list_is_loaded_with_its_source_and_licence_and_must_have_a_tac_column(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(rl.ReferenceError):
                rl.load_tac_list(write(directory, ["SYNTH,00000011,X"]), Sink(), source="", source_license="test data")
            with self.assertRaises(rl.ReferenceError):
                list(rl.read_tac_rows(write(directory, ["SYNTH,X"], header="Brand,SPECS")))

    def test_other_header_spellings_and_a_byte_order_mark(self):
        with tempfile.TemporaryDirectory() as directory:
            path = os.path.join(directory, "tac.csv")
            with open(path, "w", encoding="utf-8-sig", newline="") as handle:
                handle.write("tac,make,model\n00000011,SYNTH,SYNTH PHONE 1\n")
            self.assertEqual(list(rl.read_tac_rows(path)), [{"tac": "00000011", "make": "SYNTH", "model": "SYNTH PHONE 1"}])


@unittest.skipUnless(os.path.exists(TAC_FILE), "the public TAC list is not in the family data folder")
class RealListTests(unittest.TestCase):
    def test_the_public_list_loads_with_the_counts_measured_on_2026_10_28(self):
        _, counts = rl.build_rows(rl.read_tac_rows(TAC_FILE), source="public-list", source_license="MIT repository; mixed provenance", tenant_id="default", retrieved_at=None)
        self.assertEqual(counts["rows_read"], 255002)
        self.assertEqual(counts["accepted"] + counts["rejected_total"] + counts["repeats"] + counts["conflicts"], counts["rows_read"])
        self.assertEqual(counts["flags"], {"tac_padded": 6344})
        self.assertEqual(counts["rejected"], {"tac_length_1": 1, "tac_length_6": 277, "tac_missing": 2})
        self.assertEqual((counts["accepted"], counts["repeats"], counts["conflicts"]), (254708, 2, 12))


if __name__ == "__main__":
    unittest.main()
