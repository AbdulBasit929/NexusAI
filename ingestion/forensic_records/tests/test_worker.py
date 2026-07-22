import csv
import json
import tempfile
import unittest
from pathlib import Path

from ingestion.forensic_records.worker import (
    ADAPTERS,
    IngestJob,
    classify_columns,
    normalize_generic_row,
    profile_records,
)


class DynamicWorkerTests(unittest.TestCase):
    def job(self):
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000001",
            tenant_id="default",
            user_id="",
            collection_id="case-dynamic",
            file_id="file-1",
            source_file="dynamic.csv",
            spool_path="dynamic.csv",
            sha256="0" * 64,
            record_type="generic",
            headers=[],
            metadata={},
            request_id="req-1",
        )

    def test_classifies_arbitrary_identifier_columns_from_values(self):
        rows = [
            {
                "subscriberRef": "+92 346 167 8183",
                "counterpartyValue": "analyst@example.org",
                "eventWhen": "2026-07-10T12:00:00Z",
                "vehicleSeen": "ABC-123",
            },
            {
                "subscriberRef": "923001112222",
                "counterpartyValue": "case.officer@example.org",
                "eventWhen": "2026-07-10T12:05:00Z",
                "vehicleSeen": "LHR-447",
            },
        ]
        detected = classify_columns(rows[0].keys(), rows)
        self.assertEqual(detected["columns"]["subscriberRef"]["role"], "phone")
        self.assertEqual(detected["columns"]["counterpartyValue"]["role"], "email")
        self.assertEqual(detected["columns"]["eventWhen"]["role"], "timestamp")
        self.assertEqual(detected["primary_target"], "subscriberRef")
        self.assertEqual(detected["secondary_target"], "counterpartyValue")

    def test_normalizes_generic_row_using_detected_schema(self):
        profile = {
            "classification": {
                "timestamp": "eventWhen",
                "primary_target": "subscriberRef",
                "secondary_target": "counterpartyValue",
            }
        }
        raw = {
            "subscriberRef": "923461678183",
            "counterpartyValue": "10.20.30.40",
            "eventWhen": "2026-07-10T12:00:00Z",
            "note": "arbitrary payload",
        }
        row = normalize_generic_row(self.job(), raw, self.job().job_id, 2, ADAPTERS["generic"], profile)
        self.assertEqual(row["primary_entity"], "923461678183")
        self.assertEqual(row["secondary_entity"], "10.20.30.40")
        self.assertIsNotNone(row["observed_at"])
        self.assertIn("arbitrary payload", row["raw_record"])

    def test_ingest_job_parses_evidence_id_from_queue_payload(self):
        payload = {
            "job_id": "00000000-0000-0000-0000-000000000001",
            "evidence_id": "11111111-1111-1111-1111-111111111111",
            "tenant_id": "default",
            "collection_id": "case-dynamic",
            "file_id": "file-1",
            "source_file": "dynamic.csv",
            "spool_path": "dynamic.csv",
            "sha256": "0" * 64,
            "record_type": "generic",
            "headers": ["eventWhen", "subscriberRef"],
            "metadata": {"case_id": "case-42"},
            "request_id": "req-1",
        }
        job = IngestJob.from_message(json.dumps(payload).encode("utf-8"))
        self.assertEqual(job.evidence_id, "11111111-1111-1111-1111-111111111111")
        self.assertEqual(job.metadata["case_id"], "case-42")

    def test_profiles_dynamic_csv_with_non_demo_headers(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "dynamic.csv"
            with path.open("w", newline="", encoding="utf-8") as fh:
                writer = csv.DictWriter(fh, fieldnames=["eventWhen", "subscriberRef", "counterpartyValue"])
                writer.writeheader()
                for idx in range(1000):
                    writer.writerow(
                        {
                            "eventWhen": f"2026-07-10T12:{idx % 60:02d}:00Z",
                            "subscriberRef": f"92300111{idx:04d}",
                            "counterpartyValue": "analyst@example.org",
                        }
                    )
            profile = profile_records(path)

        self.assertEqual(profile["sample_rows"], 1000)
        self.assertEqual(profile["classification"]["columns"]["eventWhen"]["role"], "timestamp")
        self.assertEqual(profile["classification"]["columns"]["subscriberRef"]["role"], "phone")


if __name__ == "__main__":
    unittest.main()
