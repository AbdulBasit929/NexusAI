import csv
import json
import unittest
from pathlib import Path

from ingestion.forensic_records.adapters.tower_location import (
    adapter_capabilities,
    detect_schema,
    load_manifest,
    verify_normalized_record,
)
from ingestion.forensic_records.worker import ADAPTERS, IngestJob, normalize_generic_row, normalize_header


FIXTURE = Path(__file__).parent / "fixtures" / "pakistan_tower_sector_messy_synthetic.csv"
HISTORY_FIXTURE = Path(__file__).parent / "fixtures" / "pakistan_tower_history_synthetic.csv"
HISTORY_GOLDEN = Path(__file__).resolve().parents[3] / "tests" / "fixtures" / "forensic_modalities" / "pakistan_tower_history_goldens_v1.json"


class Phase7TowerAdapterTests(unittest.TestCase):
    def job(self):
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000072",
            tenant_id="default",
            user_id="phase7-test",
            collection_id="phase7-tower-golden",
            file_id="tower-golden-1",
            source_file=FIXTURE.name,
            spool_path=str(FIXTURE),
            sha256="8" * 64,
            record_type="tower_location",
            headers=[],
            metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
        )

    def normalize(self, raw, row_number=2):
        return normalize_generic_row(
            self.job(), raw, "00000000-0000-0000-0000-000000000072", row_number, ADAPTERS["tower_location"]
        )

    def test_manifest_promotes_time_aware_location_slice(self):
        manifest = load_manifest()
        self.assertEqual(manifest["contract_version"], "forensics.adapter-manifest/v1")
        self.assertEqual(manifest["id"], "nexusai.adapter.tower_location")
        self.assertEqual(manifest["version"], "1.3.0")
        self.assertEqual(manifest["status"], "operational")
        self.assertFalse(manifest["resource_profile"]["model_required"])
        self.assertFalse(manifest["location_policy"]["rf_coverage_inference"])
        self.assertIn("tower.cdr_join", manifest["operation_ids"])
        self.assertEqual(adapter_capabilities()["normalization_version"], "tower-location/v2")

    def test_detection_requires_identifier_and_both_coordinates(self):
        accepted = detect_schema(["Site Code", "Latitude WGS84", "Longitude WGS84"], normalize_header)
        self.assertTrue(accepted["matched"])
        self.assertFalse(accepted["review_required"])
        partial = detect_schema(["Site Code", "Latitude WGS84"], normalize_header)
        self.assertFalse(partial["matched"])
        self.assertEqual(partial["required_groups_matched"], 2)

    def test_normalization_preserves_reference_facts_and_explicit_limitations(self):
        normalized = self.normalize({
            "Site Code": "PK-LHR-SYN-001",
            "Sector ID": "A",
            "Latitude WGS84": "31.5204",
            "Longitude WGS84": "74.3587",
            "Azimuth": "30",
            "Beam Width": "65",
            "Technology": "LTE",
            "LAC": "1001",
            "TAC": "2001",
            "Operator": "Synthetic Telco PK",
            "Site Location": "Gulberg synthetic site",
            "District": "Lahore",
            "Coordinate Datum": "WGS84",
            "Uncertainty Radius M": "75",
            "Updated At": "2026-07-18 08:00:00",
            "Site Status": "ACTIVE",
        })
        details = json.loads(normalized["normalized_record"])
        self.assertEqual(details["site_identifier"], "PK-LHR-SYN-001")
        self.assertEqual(details["sector_identifier"], "A")
        self.assertEqual(details["provider_alias_key"], "synthetictelcopk")
        self.assertEqual(details["reference_valid_from_at"], "2026-07-18T03:00:00+00:00")
        self.assertEqual(details["uncertainty_radius_m"], "75")
        self.assertFalse(details["rf_coverage_inference_allowed"])
        self.assertFalse(details["manual_review_required"])
        self.assertEqual(normalized["primary_entity"], "PK-LHR-SYN-001")
        verification = verify_normalized_record(details)
        self.assertTrue(verification["valid"])
        self.assertTrue(verification["time_aware_reference_ready"])

    def test_missing_reference_details_are_preserved_as_review_flags(self):
        normalized = self.normalize({
            "Site Code": "PK-ISB-SYN-004",
            "Latitude WGS84": "33.6844",
            "Longitude WGS84": "73.0479",
            "Technology": "3G",
            "Updated At": "2026-07-18 11:00:00",
            "Site Status": "RETIRED",
        })
        details = json.loads(normalized["normalized_record"])
        self.assertTrue(details["manual_review_required"])
        self.assertIn("coordinate_datum_assumed_wgs84", details["quality_flags"])
        self.assertIn("sector_identifier_missing", details["quality_flags"])
        self.assertIn("uncertainty_radius_missing", details["quality_flags"])

    def test_fixture_reconciles_five_unique_rows_one_duplicate_and_four_rejects(self):
        with FIXTURE.open("r", encoding="utf-8-sig", newline="") as stream:
            raw_rows = list(csv.DictReader(stream))
        accepted = []
        rejected = []
        for row_number, raw in enumerate(raw_rows, start=2):
            try:
                accepted.append(self.normalize(raw, row_number))
            except ValueError as exc:
                rejected.append(str(exc))
        unique = {row["row_hash"]: row for row in accepted}
        self.assertEqual(len(raw_rows), 10)
        self.assertEqual(len(accepted), 6)
        self.assertEqual(len(unique), 5)
        self.assertEqual(len(accepted) - len(unique), 1)
        self.assertEqual(len(rejected), 4)
        self.assertTrue(any("latitude" in error for error in rejected))
        self.assertTrue(any("longitude" in error for error in rejected))
        self.assertTrue(any("azimuth" in error for error in rejected))
        self.assertTrue(any("identifier is required" in error for error in rejected))

    def test_history_fixture_preserves_provider_sector_validity_and_uncertainty(self):
        golden = json.loads(HISTORY_GOLDEN.read_text(encoding="utf-8"))
        with HISTORY_FIXTURE.open("r", encoding="utf-8-sig", newline="") as stream:
            raw_rows = list(csv.DictReader(stream))
        normalized = [
            json.loads(self.normalize(row, row_number)["normalized_record"])
            for row_number, row in enumerate(raw_rows, start=2)
        ]
        history = [row for row in normalized if row["history_key"] == golden["history_key"]]
        self.assertEqual(len(history), golden["reference_rows"])
        self.assertTrue(all(row["provider_alias_key"] == golden["provider_alias_key"] for row in history))
        self.assertEqual(history[0]["validity_status"], golden["first_window_status"])
        self.assertEqual(history[1]["validity_status"], golden["current_window_status"])
        self.assertEqual(history[0]["coordinate_datum"], "WGS84")
        self.assertEqual(history[1]["coordinate_datum_raw"], "EPSG:4326")
        self.assertEqual(history[1]["coordinate_datum_basis"], "supplied_wgs84_equivalent")
        self.assertEqual(
            sorted({row["uncertainty_class"] for row in history}),
            sorted(golden["expected_uncertainty_classes"]),
        )
        self.assertTrue(all(row["coordinate_provenance"] == "supplied_source_row" for row in history))
        self.assertTrue(all(row["rf_coverage_inference_allowed"] is golden["rf_coverage_inference_allowed"] for row in history))
        verification = verify_normalized_record(history[0])
        self.assertTrue(verification["history_key_ready"])
        self.assertEqual(verification["validity_status"], "closed_window")

        overlap_pairs = 0
        for index, left in enumerate(history):
            left_start = left["reference_valid_from_at"]
            left_end = left["reference_valid_to_at"]
            for right in history[index + 1:]:
                right_start = right["reference_valid_from_at"]
                right_end = right["reference_valid_to_at"]
                if (left_end is None or right_start < left_end) and (right_end is None or left_start < right_end):
                    overlap_pairs += 1
        self.assertEqual(overlap_pairs, golden["overlap_pairs"])


if __name__ == "__main__":
    unittest.main()
