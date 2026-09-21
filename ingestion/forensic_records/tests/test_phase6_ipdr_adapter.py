import csv
import json
import unittest
from pathlib import Path

from ingestion.forensic_records.adapters.ipdr import (
    adapter_capabilities,
    detect_schema,
    load_manifest,
    validate_headers,
    verify_normalized_record,
)
from ingestion.forensic_records.worker import ADAPTERS, IngestJob, normalize_generic_row, normalize_header


FIXTURES = Path(__file__).parent / "fixtures"
FIXTURE = FIXTURES / "pakistan_ipdr_messy_synthetic.csv"
GOLDEN = FIXTURES / "pakistan_phase4_structured_goldens_v1.json"


class Phase6IPDRAdapterTests(unittest.TestCase):
    def job(self):
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000062",
            tenant_id="default",
            user_id="phase6-ipdr-test",
            collection_id="phase6-ipdr-golden",
            file_id="ipdr-golden-1",
            source_file=FIXTURE.name,
            spool_path=str(FIXTURE),
            sha256="2" * 64,
            record_type="ipdr",
            headers=[],
            metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
        )

    def test_manifest_declares_bounded_operational_ipdr_slice(self):
        manifest = load_manifest()
        self.assertEqual(manifest["contract_version"], "forensics.adapter-manifest/v1")
        self.assertEqual(manifest["id"], "nexusai.adapter.ipdr")
        self.assertEqual(manifest["version"], "1.2.0")
        self.assertEqual(manifest["status"], "operational")
        self.assertTrue(manifest["preserves_legacy_output"])
        self.assertFalse(manifest["resource_profile"]["model_required"])
        self.assertEqual(manifest["resource_profile"]["maximum_result_limit"], 200)
        self.assertIn("ipdr.concurrent_sessions", manifest["operation_ids"])
        self.assertIn("ipdr.subscriber_sessions", manifest["operation_ids"])
        self.assertEqual(adapter_capabilities()["normalization_version"], "ipdr-session-canonical/v1")

    def test_detection_requires_both_endpoints_and_session_timestamp(self):
        accepted = detect_schema(
            ["session_start", "src_ip", "dst_ip", "nat_source_ip", "bytes"],
            normalize_header,
        )
        self.assertTrue(accepted["matched"])
        self.assertEqual(accepted["required_groups_matched"], 3)
        self.assertIn("nat_source_ip", accepted["matched_fields"])

        partial = detect_schema(["session_start", "source_ip", "bytes"], normalize_header)
        self.assertFalse(partial["matched"])
        self.assertEqual(partial["required_groups_matched"], 2)
        self.assertTrue(partial["review_required"])
        with self.assertRaisesRegex(ValueError, "destination IP"):
            validate_headers(["session_start", "source_ip"], normalize_header)

    def test_pakistan_golden_preserves_ipv6_nat_timezone_and_accounting(self):
        golden = json.loads(GOLDEN.read_text(encoding="utf-8"))["datasets"]["ipdr"]
        with FIXTURE.open(newline="", encoding="utf-8-sig") as handle:
            rows = list(csv.DictReader(handle))
        accepted = []
        rejected = []
        for row_number, raw in enumerate(rows, start=2):
            try:
                accepted.append(
                    normalize_generic_row(
                        self.job(), raw, self.job().job_id, row_number, ADAPTERS["ipdr"]
                    )
                )
            except ValueError as exc:
                rejected.append(str(exc))

        self.assertEqual(len(rows), golden["total_rows"])
        self.assertEqual(len(accepted), golden["accepted_rows"])
        self.assertEqual(len(rejected), golden["rejected_rows"])
        first = json.loads(accepted[0]["normalized_record"])
        ipv6 = json.loads(accepted[1]["normalized_record"])
        self.assertEqual(accepted[0]["observed_at"].isoformat(), golden["exact_checks"]["first_timestamp_utc"])
        self.assertEqual(first["duration_seconds"], golden["exact_checks"]["first_duration_seconds"])
        self.assertEqual(first["nat_source_ip_canonical"], golden["exact_checks"]["nat_source_canonical"])
        self.assertEqual(ipv6["source_ip_canonical"], golden["exact_checks"]["ipv6_source_canonical"])
        self.assertEqual(accepted[0]["row_hash"], accepted[3]["row_hash"])
        verification = verify_normalized_record(first)
        self.assertTrue(verification["valid"])
        self.assertTrue(verification["has_network_endpoints"])
        self.assertTrue(verification["has_nat_locator"])

    def test_verification_rejects_negative_derived_values(self):
        verification = verify_normalized_record(
            {
                "source_ip_canonical": "10.0.0.1",
                "destination_ip_canonical": "8.8.8.8",
                "byte_count": -1,
                "duration_seconds": -2,
            }
        )
        self.assertFalse(verification["valid"])
        self.assertIn("byte_count cannot be negative", verification["errors"])
        self.assertIn("duration_seconds cannot be negative", verification["errors"])


if __name__ == "__main__":
    unittest.main()
