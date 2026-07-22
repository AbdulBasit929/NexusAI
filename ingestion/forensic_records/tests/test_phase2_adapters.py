import unittest

from ingestion.forensic_records.worker import (
    IngestJob,
    ADAPTERS,
    normalize_generic_row,
    normalize_cdr_row,
    resolve_adapter,
)


class Phase2AdapterTests(unittest.TestCase):
    def job(self, record_type="auto"):
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000001",
            tenant_id="default",
            user_id="",
            collection_id="records-demo",
            file_id="file-1",
            source_file="sample.csv",
            spool_path="sample.csv",
            sha256="0" * 64,
            record_type=record_type,
            headers=[],
            metadata={},
        )

    def test_detects_anpr_headers(self):
        adapter = resolve_adapter(self.job(), ["plate_number", "camera_id", "capture_time", "location"])
        self.assertEqual(adapter.name, "anpr")

    def test_detects_ipdr_headers(self):
        adapter = resolve_adapter(self.job(), ["session_start", "source_ip", "destination_ip", "bytes"])
        self.assertEqual(adapter.name, "ipdr")

    def test_detects_subscriber_registry_headers(self):
        adapter = resolve_adapter(self.job("subscriber"), ["msisdn", "subscriber_name", "cnic_last4", "home_city"])
        self.assertEqual(adapter.name, "subscriber")

    def test_requested_adapter_requires_matching_headers(self):
        with self.assertRaises(ValueError):
            resolve_adapter(self.job("anpr"), ["source_ip", "destination_ip"])

    def test_generic_row_preserves_raw_and_extracts_entities(self):
        raw = {
            "plate_number": "ABC-123",
            "camera_id": "CAM-7",
            "capture_time": "2026-07-14 10:11:12",
            "location": "Gate 4",
        }
        normalized = normalize_generic_row(self.job("anpr"), raw, "00000000-0000-0000-0000-000000000001", 2, ADAPTERS["anpr"])
        self.assertEqual(normalized["record_type"], "anpr")
        self.assertEqual(normalized["primary_entity"], "ABC-123")
        self.assertEqual(normalized["secondary_entity"], "CAM-7")
        self.assertEqual(normalized["location"], "Gate 4")
        self.assertIsNotNone(normalized["observed_at"])
        self.assertIn("plate_number", normalized["raw_record"])

    def test_cdr_row_accepts_iso_timestamps_and_formatted_numbers(self):
        raw = {
            "MSISDN": "+92 346 167 8183",
            "call_org_num": "+92-300-111-2222",
            "CALL_DIALED_NUM": "923334445555",
            "IMSI": "410010123456789",
            "IMEI": "356789012345678",
            "CALL_START_DT_TM": "2026-07-10T21:04:12Z",
            "CALL_END_DT_TM": "2026-07-10T21:08:02Z",
            "INBOUND_OUTBOUND_IND": "OUTGOING",
            "Call_Network_Volume": "0",
            "Lac_Id": "LAC-12",
            "Site_Id": "SITE-014",
            "Cell_SITE_ID": "LHR-GUL-014",
            "lat": "31.520604",
            "longitude": "74.358747",
            "CALL_TYPE": "VOICE",
            "location": "Gulberg",
        }
        normalized = normalize_cdr_row(self.job("cdr"), raw, "00000000-0000-0000-0000-000000000001", 2)
        self.assertEqual(normalized["duration_seconds"], 230)
        self.assertEqual(normalized["direction"], "OUTGOING")
        self.assertEqual(normalized["cell_site_id"], "LHR-GUL-014")
        self.assertIsNotNone(normalized["call_start_ts"])


if __name__ == "__main__":
    unittest.main()
