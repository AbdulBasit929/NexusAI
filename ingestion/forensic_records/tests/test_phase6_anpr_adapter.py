import csv
import json
import unittest
from pathlib import Path

from ingestion.forensic_records.adapters.anpr import (
    adapter_capabilities,
    detect_schema,
    load_manifest,
    validate_headers,
    verify_normalized_record,
)
from ingestion.forensic_records.worker import ADAPTERS, IngestJob, normalize_generic_row, normalize_header


FIXTURES = Path(__file__).parent / "fixtures"
FIXTURE = FIXTURES / "pakistan_anpr_messy_synthetic.csv"
GOLDEN = FIXTURES / "pakistan_structured_goldens_v1.json"


class Phase6ANPRAdapterTests(unittest.TestCase):
    def job(self):
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000063",
            tenant_id="default",
            user_id="phase6-anpr-test",
            collection_id="phase6-anpr-golden",
            file_id="anpr-golden-1",
            source_file=FIXTURE.name,
            spool_path=str(FIXTURE),
            sha256="3" * 64,
            record_type="anpr",
            headers=[],
            metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
        )

    def test_manifest_declares_bounded_operational_anpr_slice(self):
        manifest = load_manifest()
        self.assertEqual(manifest["contract_version"], "forensics.adapter-manifest/v1")
        self.assertEqual(manifest["id"], "nexusai.adapter.anpr")
        self.assertEqual(manifest["version"], "1.2.0")
        self.assertEqual(manifest["status"], "operational")
        self.assertTrue(manifest["preserves_legacy_output"])
        self.assertFalse(manifest["resource_profile"]["model_required"])
        self.assertEqual(manifest["resource_profile"]["maximum_result_limit"], 200)
        self.assertIn("anpr.co_travel", manifest["operation_ids"])
        self.assertIn("anpr.route_timing", manifest["operation_ids"])
        self.assertEqual(adapter_capabilities()["normalization_version"], "anpr-sighting-canonical/v1")

    def test_detection_accepts_pakistan_vendor_aliases_and_rejects_missing_plate(self):
        accepted = detect_schema(
            ["Registration No.", "Camera Code", "Captured At", "Camera Location", "OCR Confidence"],
            normalize_header,
        )
        self.assertTrue(accepted["matched"])
        self.assertIn("plate", accepted["matched_fields"])
        self.assertIn("camera_id", accepted["matched_fields"])
        self.assertIn("confidence", accepted["matched_fields"])

        partial = detect_schema(["Camera Code", "Captured At"], normalize_header)
        self.assertFalse(partial["matched"])
        self.assertTrue(partial["review_required"])
        with self.assertRaisesRegex(ValueError, "plate column"):
            validate_headers(["Camera Code", "Captured At"], normalize_header)

    def test_pakistan_golden_preserves_script_confidence_timezone_and_accounting(self):
        golden = json.loads(GOLDEN.read_text(encoding="utf-8"))["datasets"]["anpr"]
        with FIXTURE.open(newline="", encoding="utf-8-sig") as handle:
            rows = list(csv.DictReader(handle))
        accepted = []
        rejected = []
        for row_number, raw in enumerate(rows, start=2):
            try:
                accepted.append(normalize_generic_row(self.job(), raw, self.job().job_id, row_number, ADAPTERS["anpr"]))
            except ValueError as exc:
                rejected.append(str(exc))

        self.assertEqual(len(rows), golden["total_rows"])
        self.assertEqual(len(accepted), golden["accepted_rows"])
        self.assertEqual(len(rejected), golden["rejected_rows"])
        first = json.loads(accepted[0]["normalized_record"])
        urdu = json.loads(accepted[2]["normalized_record"])
        low_confidence = json.loads(accepted[3]["normalized_record"])
        self.assertEqual(accepted[0]["observed_at"].isoformat(), golden["exact_checks"]["first_timestamp_utc"])
        self.assertEqual(accepted[1]["observed_at"].isoformat(), golden["exact_checks"]["explicit_offset_timestamp_utc"])
        self.assertEqual(first["ocr_confidence"], "0.98")
        self.assertEqual(first["image_crop_sha256"], "a" * 64)
        self.assertEqual(urdu["plate_script"], "arabic_derived")
        self.assertEqual(urdu["plate_search_key"], golden["exact_checks"]["urdu_plate_search_key"])
        self.assertEqual(low_confidence["manual_review_required"], golden["exact_checks"]["low_confidence_manual_review"])
        self.assertIn("low_ocr_confidence", low_confidence["quality_flags"])
        self.assertEqual(accepted[0]["row_hash"], accepted[5]["row_hash"])
        verification = verify_normalized_record(first)
        self.assertTrue(verification["valid"])
        self.assertTrue(verification["has_plate_locator"])
        self.assertTrue(verification["has_image_crop_hash"])

    def test_verification_rejects_invalid_confidence_and_crop_hash(self):
        verification = verify_normalized_record(
            {"plate_raw": "ABC-123", "plate_search_key": "ABC123", "ocr_confidence": "1.2", "image_crop_sha256": "bad"}
        )
        self.assertFalse(verification["valid"])
        self.assertIn("ocr_confidence must be within 0..1", verification["errors"])
        self.assertIn("image_crop_sha256 must be lowercase hexadecimal SHA-256", verification["errors"])


if __name__ == "__main__":
    unittest.main()
