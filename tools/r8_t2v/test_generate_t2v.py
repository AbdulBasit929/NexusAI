import hashlib
import json
import tempfile
import unittest
from pathlib import Path

import generate_t2v


CONTRACT = Path(__file__).parents[2] / "configuration" / "nexusai_r8_t2v_contract.json"


class T2VGeneratorTest(unittest.TestCase):
    def test_pack_is_deterministic_partitioned_and_complex(self):
        with tempfile.TemporaryDirectory() as first_root, tempfile.TemporaryDirectory() as second_root:
            first = Path(first_root); second = Path(second_root)
            first_result = generate_t2v.generate_pack(first, CONTRACT)
            second_result = generate_t2v.generate_pack(second, CONTRACT)
            self.assertEqual(96, first_result["manifest"]["fixture_count"])
            self.assertEqual({"development": 56, "validation": 20, "sealed_holdout": 20}, first_result["manifest"]["partition_counts"])
            self.assertEqual(hashlib.sha256((first / "manifest.json").read_bytes()).hexdigest(), hashlib.sha256((second / "manifest.json").read_bytes()).hexdigest())
            self.assertEqual(first_result["seal"], second_result["seal"])
            positives = [item for item in first_result["manifest"]["fixtures"] if "hard_positive" in item["fixture_classes"]]
            negatives = [item for item in first_result["manifest"]["fixtures"] if "hard_negative" in item["fixture_classes"]]
            self.assertGreaterEqual(len({item["scene_parameters"]["background"] for item in positives}), 6)
            self.assertGreaterEqual(len({item["scene_parameters"]["negative_class"] for item in negatives}), 14)
            self.assertTrue(all(item["plate_regions_original_pixels"] for item in positives))
            self.assertTrue(all(not item["plate_regions_original_pixels"] for item in negatives))


if __name__ == "__main__":
    unittest.main()
