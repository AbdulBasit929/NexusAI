import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from validate_nexusai_hardware_governance import (
    validate_hardware,
    validate_matrix,
    validate_t3,
)


ROOT = Path(__file__).resolve().parents[1]
HARDWARE = ROOT / "configuration" / "nexusai_hardware_tiers.json"
MATRIX = ROOT / "configuration" / "nexusai_r8_candidate_hardware_matrix.json"
T3 = ROOT / "configuration" / "nexusai_r8_t3_acquisition.json"


class HardwareGovernanceTests(unittest.TestCase):
    def test_repository_contracts_pass(self):
        self.assertEqual(validate_hardware(HARDWARE)["tiers"], 4)
        matrix = validate_matrix(MATRIX)
        self.assertEqual(matrix["selected_small_evaluations"], 0)
        self.assertEqual(matrix["retained_efficient_references"], 1)
        self.assertEqual(validate_t3(T3)["selected"], "ccpd-2019-zenodo-15647076")

    def test_future_tier_cannot_claim_installed(self):
        payload = json.loads(HARDWARE.read_text(encoding="utf-8"))
        payload["tiers"]["P1"]["current_machine"] = True
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "hardware.json"
            path.write_text(json.dumps(payload), encoding="utf-8")
            with self.assertRaises(AssertionError):
                validate_hardware(path)

    def test_candidate_promotion_authority_fails_closed(self):
        payload = json.loads(MATRIX.read_text(encoding="utf-8"))
        payload["automatic_promotion"] = True
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "matrix.json"
            path.write_text(json.dumps(payload), encoding="utf-8")
            with self.assertRaises(AssertionError):
                validate_matrix(path)

    def test_t3_demo_use_fails_closed(self):
        payload = json.loads(T3.read_text(encoding="utf-8"))
        payload["privacy_disposition"]["safe_for_demo"] = True
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "t3.json"
            path.write_text(json.dumps(payload), encoding="utf-8")
            with self.assertRaises(AssertionError):
                validate_t3(path)


if __name__ == "__main__":
    unittest.main()
