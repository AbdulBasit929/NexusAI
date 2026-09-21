import hashlib
import tempfile
import unittest
from pathlib import Path

import generate_fixtures


class R8FixtureGeneratorTest(unittest.TestCase):
    def test_pack_is_complete_deterministic_and_lane_safe(self):
        with tempfile.TemporaryDirectory() as first_directory, tempfile.TemporaryDirectory() as second_directory:
            first = Path(first_directory)
            second = Path(second_directory)
            first_manifest = generate_fixtures.generate_pack(first)
            second_manifest = generate_fixtures.generate_pack(second)
            required = set(first_manifest["required_classes"])
            present = {fixture_class for fixture in first_manifest["fixtures"] for fixture_class in fixture["fixture_classes"]}
            self.assertEqual(required, present)
            self.assertEqual({"T0": 6, "T1": 23}, first_manifest["tier_counts"])
            self.assertEqual(29, first_manifest["fixture_count"])
            hostile = [fixture for fixture in first_manifest["fixtures"] if fixture["expected_admission_state"] == "manual_review_required"]
            self.assertEqual(4, len(hostile))
            self.assertTrue(all(not fixture["safe_for_model_input"] for fixture in hostile))
            self.assertTrue(all(not fixture["detector_evaluation_eligible"] and not fixture["ocr_evaluation_eligible"] for fixture in hostile))
            self.assertEqual(
                hashlib.sha256((first / "manifest.json").read_bytes()).hexdigest(),
                hashlib.sha256((second / "manifest.json").read_bytes()).hexdigest(),
            )


if __name__ == "__main__":
    unittest.main()
