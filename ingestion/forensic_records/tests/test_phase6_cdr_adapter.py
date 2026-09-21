import csv
import json
import unittest
from pathlib import Path

from ingestion.forensic_records.adapters.cdr import (
    adapter_capabilities,
    detect_schema,
    load_manifest,
    validate_headers,
    verify_normalized_record,
)
from ingestion.forensic_records.worker import IngestJob, normalize_cdr_row, normalize_header


FIXTURE = Path(__file__).parent / "fixtures" / "pakistan_cdr_messy_synthetic.csv"
ROLE_GOLDEN = Path(__file__).resolve().parents[3] / "tests" / "fixtures" / "forensic_modalities" / "pakistan_cdr_subscriber_role_goldens_v1.json"


class Phase6CDRAdapterTests(unittest.TestCase):
    def job(self):
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000006",
            tenant_id="default",
            user_id="phase6-test",
            collection_id="phase6-cdr-golden",
            file_id="cdr-golden-1",
            source_file=FIXTURE.name,
            spool_path=str(FIXTURE),
            sha256="6" * 64,
            record_type="cdr",
            headers=[],
            metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
        )

    def test_manifest_promotes_one_bounded_operational_family_slice(self):
        manifest = load_manifest()
        self.assertEqual(manifest["contract_version"], "forensics.adapter-manifest/v1")
        self.assertEqual(manifest["id"], "nexusai.adapter.cdr")
        self.assertEqual(manifest["version"], "1.4.0")
        self.assertEqual(manifest["schema_profile_version"], "cdr-source-mapping/v2")
        self.assertEqual(manifest["status"], "operational")
        self.assertTrue(manifest["preserves_legacy_output"])
        self.assertFalse(manifest["resource_profile"]["model_required"])
        self.assertEqual(manifest["resource_profile"]["maximum_result_limit"], 200)
        self.assertIn("cdr.device_identity_changes", manifest["operation_ids"])
        self.assertIn("cdr.service_usage", manifest["operation_ids"])
        self.assertEqual(adapter_capabilities()["normalization_version"], "cdr-canonical/v2")

    def test_detection_is_explainable_and_rejects_partial_cdr_shapes(self):
        accepted = detect_schema(["call_time", "source_number", "target_number", "duration_seconds"], normalize_header)
        self.assertTrue(accepted["matched"])
        self.assertEqual(accepted["required_groups_matched"], 3)
        self.assertFalse(accepted["review_required"])

        partial = detect_schema(["call_time", "source_number", "duration_seconds"], normalize_header)
        self.assertFalse(partial["matched"])
        self.assertEqual(partial["required_groups_matched"], 2)
        self.assertTrue(partial["review_required"])
        with self.assertRaisesRegex(ValueError, "target column"):
            validate_headers(["call_time", "source_number"], normalize_header)

    def test_pakistan_golden_preserves_provider_tokens_urdu_and_source_locator(self):
        with FIXTURE.open(newline="", encoding="utf-8-sig") as handle:
            rows = list(csv.DictReader(handle))
        normalized = [
            normalize_cdr_row(self.job(), row, "00000000-0000-0000-0000-000000000006", index)
            for index, row in enumerate(rows, start=2)
        ]
        self.assertEqual(len(normalized), 5)
        self.assertEqual(normalized[1]["call_dialed_num"], "INTERNET")
        self.assertEqual(normalized[1]["cell_site_id"], "-1")
        self.assertEqual(normalized[2]["call_dialed_num"], "*123#")
        self.assertIn("اسلام", normalized[2]["location"])
        self.assertEqual(normalized[0]["row_hash"], normalized[4]["row_hash"])
        self.assertEqual(normalized[0]["call_start_ts"].isoformat(), "2026-07-10T03:00:00+00:00")
        self.assertEqual(normalized[0]["msisdn_canonical"], "+923001234567")
        self.assertEqual(normalized[0]["originating_number_role"], "explicit_originating_party")
        self.assertEqual(normalized[0]["called_number_canonical"], "+923111234567")
        self.assertEqual(normalized[0]["called_number_role"], "explicit_called_party")
        self.assertEqual(normalized[0]["direction_canonical"], "OUTGOING")
        self.assertEqual(normalized[0]["service_class"], "VOICE")
        self.assertEqual(normalized[0]["source_timezone"], "Asia/Karachi")
        self.assertEqual(normalized[0]["timestamp_basis"], "assumed_source_timezone")
        self.assertEqual(normalized[0]["canonical_timezone"], "UTC")
        expected = json.loads(ROLE_GOLDEN.read_text(encoding="utf-8"))["cdr"]["expected"]
        self.assertEqual({key: normalized[0][key] for key in expected}, expected)
        self.assertEqual(normalized[1]["called_number_kind"], "packet_service_label")
        self.assertEqual(normalized[1]["service_class"], "PACKET_DATA")
        self.assertIn("cell_site_identifier_sentinel", normalized[1]["quality_flags"])
        self.assertEqual(normalized[2]["called_number_kind"], "service_or_short_code")
        self.assertEqual(normalized[2]["service_class"], "USSD")
        verification = verify_normalized_record(normalized[0])
        self.assertTrue(verification["valid"])
        self.assertTrue(verification["has_source_locator"])
        self.assertTrue(verification["has_device_identity"])
        self.assertTrue(verification["has_explicit_party_roles"])
        self.assertTrue(verification["has_timezone_provenance"])

    def test_verification_flags_negative_duration_without_mutating_evidence(self):
        record = {
            "call_start_ts": "2026-07-10T03:00:00Z",
            "call_org_num": "03001234567",
            "call_dialed_num": "03111234567",
            "duration_seconds": -1,
            "source_file": "adversarial.csv",
            "row_number": 2,
        }
        verification = verify_normalized_record(record)
        self.assertFalse(verification["valid"])
        self.assertIn("duration_seconds cannot be negative", verification["errors"])


if __name__ == "__main__":
    unittest.main()
