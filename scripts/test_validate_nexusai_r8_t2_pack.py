import hashlib
import json
import tempfile
import unittest
from pathlib import Path

from scripts.validate_nexusai_r8_t2_pack import validate


class T2PackValidationTest(unittest.TestCase):
    def test_safe_owned_pack_is_ready_and_tampering_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            asset = root / "owned.png"
            asset.write_bytes(b"\x89PNG\r\n\x1a\ncontrolled-demo")
            image = {
                "image_id": "owned-001", "original_filename": asset.name,
                "sha256": hashlib.sha256(asset.read_bytes()).hexdigest(),
                "capture_source_class": "controlled_owned_vehicle", "width": 640, "height": 480,
                "orientation": "landscape", "plate_present": True, "plate_count": 1,
                "plates": [{"polygon_original_pixels": [[10, 20], [200, 20], [200, 80], [10, 80]], "raw_expected_text": "ABC-123", "normalized_expected_text": "ABC123", "script": "Latin", "style": "controlled_test"}],
                "plate_style": "controlled_test", "lighting": "daylight", "blur_level": "none",
                "occlusion": "none", "perspective": "frontal", "notes": "owned test fixture",
                "camera_orientation": "landscape", "distance": "near", "target_position": "center",
                "compression": "camera_default", "negative_class": None,
                "evaluation_partition": "holdout",
                "consent_ownership_status": "owned_or_explicitly_consented", "demo_eligible": True,
                "asset_class": "original", "ground_truth_source": "independent_human_annotation",
                "ground_truth_review": {"annotator": "annotator-a", "annotated_at": "2026-08-13T09:00:00Z",
                    "reviewer": "reviewer-b", "reviewed_at": "2026-08-13T10:00:00Z",
                    "reviewer_independent": True, "model_output_consulted": False,
                    "disagreements_resolved": True},
                "privacy_screen": {"review_completed": True, "unrelated_faces": False,
                    "unrelated_real_plates": False, "house_numbers": False, "addresses": False,
                    "documents": False, "screen_content": False, "gps_exif": "absent",
                    "sensitive_location_context": False, "reviewer": "owner", "reviewed_at": "2026-08-12T00:00:00Z"},
            }
            payload = {"contract_version": "forensics.controlled-demo-pack/v1", "tier": "T2", "required_image_count": 1,
                       "ground_truth_policy": "independent_human_annotation_only", "images": [image]}
            errors, ready = validate(payload, root)
            self.assertEqual([], errors)
            self.assertTrue(ready)
            asset.write_bytes(asset.read_bytes() + b"tampered")
            errors, ready = validate(payload, root)
            self.assertIn("owned-001: asset SHA-256 mismatch", errors)
            self.assertFalse(ready)

    def test_partition_policy_requires_development_and_holdout(self):
        payload = {
            "contract_version": "forensics.controlled-demo-pack/v1", "tier": "T2",
            "ground_truth_policy": "independent_human_annotation_only",
            "required_image_count": 0, "partition_policy": "sealed holdout", "images": [],
        }
        errors, ready = validate(payload, None)
        self.assertEqual([], errors)
        self.assertFalse(ready)


if __name__ == "__main__":
    unittest.main()
