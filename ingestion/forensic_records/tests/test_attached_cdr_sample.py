import csv
import os
import unittest
from collections import Counter
from datetime import datetime
from pathlib import Path


SAMPLE_PATH = Path(os.getenv("FORENSIC_CDR_SAMPLE", r"C:\Users\W S Mughal\Downloads\923461678183.csv"))


@unittest.skipUnless(SAMPLE_PATH.exists(), f"sample CDR file not found: {SAMPLE_PATH}")
class AttachedCDRSampleTest(unittest.TestCase):
    def test_reference_schema_and_profile(self):
        with SAMPLE_PATH.open(newline="", encoding="utf-8-sig") as fh:
            rows = list(csv.DictReader(fh))

        self.assertEqual(len(rows), 3931)
        self.assertEqual(
            list(rows[0].keys()),
            [
                "MSISDN",
                "call_org_num",
                "CALL_DIALED_NUM",
                "IMSI",
                "IMEI",
                "CALL_START_DT_TM",
                "CALL_END_DT_TM",
                "INBOUND_OUTBOUND_IND",
                "Call_Network_Volume",
                "Lac_Id",
                "Site_Id",
                "Cell_SITE_ID",
                "lat",
                "longitude",
                "CALL_TYPE",
                "location",
            ],
        )

        call_types = Counter(row["CALL_TYPE"] for row in rows)
        directions = Counter(row["INBOUND_OUTBOUND_IND"] for row in rows)
        targets = {row["CALL_DIALED_NUM"] for row in rows}
        locations = {row["location"] for row in rows}
        timestamps = [datetime.strptime(row["CALL_START_DT_TM"], "%Y-%m-%d %H:%M:%S") for row in rows]
        duplicates = len(rows) - len({tuple(sorted(row.items())) for row in rows})

        self.assertEqual(call_types, {"GPRS": 3410, "SMS": 341, "VOLTE": 140, "CALL": 40})
        self.assertEqual(directions, {"DATA": 3410, "INCOMING": 404, "OUTGOING": 117})
        self.assertEqual(len(targets), 37)
        self.assertEqual(len(locations), 34)
        self.assertEqual(duplicates, 297)
        self.assertEqual(min(timestamps).strftime("%Y-%m-%d %H:%M:%S"), "2026-04-02 01:22:34")
        self.assertEqual(max(timestamps).strftime("%Y-%m-%d %H:%M:%S"), "2026-06-20 19:08:01")


if __name__ == "__main__":
    unittest.main()
