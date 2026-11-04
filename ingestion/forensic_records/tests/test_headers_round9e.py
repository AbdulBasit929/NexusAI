"""Engine round 9e (docs/data/engine-round-9e-20261103.md): a byte-order mark is not part of a heading, and the typed ingest keeps the file's header list.

Plain unittest:  python -m unittest ingestion.forensic_records.tests.test_headers_round9e

The live check of round 9d on the real Orion files found the IPDR CSV's first column named with the byte-order mark ("BOM + UFDR start time"; its cells showed blank), and the columns of a
workbook in the old XML format and of a text export in the order of the database's keys, not the file's. The first is stripped wherever a heading is read; the second needs the header list
at ingest, which the summary now carries (SheetSummary.headers) and the evidence keeps (`typed_headers`).
"""

from __future__ import annotations

import os
import unittest

from ingestion.forensic_records import typed_ingest as ti
from ingestion.forensic_records import worker
from ingestion.forensic_records.layout_registry.interpret import BOM, header_keys, rows_to_records
from ingestion.forensic_records.tests.test_typed_ingest import GOOD, HEADER, KARACHI, LINEAGE, REGISTRY, FakeSink, Workdir


class HeadingTests(unittest.TestCase):
    def test_a_byte_order_mark_is_not_part_of_a_heading(self):
        self.assertEqual(header_keys([BOM + "UFDR start time", " b ", None, "b", ""]), ["UFDR start time", "b", None, "b__duplicate_2", None])
        self.assertEqual(list(rows_to_records([BOM + "a", "b"], [["1", "2"]])), [{"a": "1", "b": "2"}])

    def test_the_worker_keeps_no_byte_order_mark_in_a_header(self):
        self.assertEqual(worker.disambiguate_headers([BOM + "UFDR start time", "x", "x"]), ["UFDR start time", "x", "x__duplicate_2"])


class HeaderListTests(Workdir):
    def test_the_summary_keeps_the_header_list_in_the_files_order(self):
        summary, _ = self.ingest(GOOD)
        sheet = summary["sheets"][0]
        self.assertEqual(sheet["status"], "loaded")
        self.assertEqual(sheet["headers"], HEADER)
        self.assertEqual(ti.file_headers(summary), [{"sheet": sheet["sheet"], "table": "comm_event", "headers": HEADER}])

    def test_a_sheet_that_was_not_written_has_no_header_list_to_keep(self):
        summary = {"sheets": [{"sheet": "a", "status": "unrecognised", "headers": ["x"], "table": None}, {"sheet": "b", "status": "loaded", "headers": [], "table": "comm_event"}]}
        self.assertEqual(ti.file_headers(summary), [])

    def test_a_mark_the_decoder_kept_does_not_stop_the_layout_from_matching_or_stay_in_the_first_heading(self):
        # two marks: the reader strips the first, a second would stay in the first cell
        path = os.path.join(self.tmp.name, "marked.csv")
        text = ",".join(HEADER) + "\n" + ",".join(GOOD[0]) + "\n"
        with open(path, "wb") as handle:
            handle.write(b"\xef\xbb\xbf\xef\xbb\xbf" + text.encode("utf-8"))
        summary = ti.ingest_file(path, LINEAGE, KARACHI, FakeSink(), registry=REGISTRY)
        sheet = summary["sheets"][0]
        self.assertEqual(sheet["status"], "loaded")
        self.assertEqual(sheet["headers"][0], HEADER[0])
        self.assertNotIn(BOM, "".join(sheet["headers"]))


if __name__ == "__main__":
    unittest.main()
