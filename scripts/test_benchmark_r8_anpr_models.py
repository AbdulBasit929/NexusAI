import importlib.util
import subprocess
import sys
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("benchmark_r8_anpr_models.py")
SPEC = importlib.util.spec_from_file_location("benchmark_r8_anpr_models", MODULE_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
SPEC.loader.exec_module(MODULE)


class BenchmarkR8ANPRModelsTest(unittest.TestCase):
    def test_detector_calibration_uses_iou_outcome(self):
        fixtures = [{
            "fixture_id": "one",
            "plate_regions_original_pixels": [{"bounds": {"x": 0, "y": 0, "width": 100, "height": 50}}],
        }]
        predictions = {"one": {
            "plate_regions_original_pixels": [{
                "bounds": {"x": 0, "y": 0, "width": 100, "height": 50},
                "confidence": 0.9,
            }],
        }}
        result = MODULE.score_detector(fixtures, predictions, 0.5)
        self.assertEqual(1, result["true_positive"])
        self.assertEqual(1.0, result["f1"])
        self.assertAlmostEqual(0.01, result["confidence_calibration"]["brier_score"])

    def test_ocr_reports_raw_and_governed_normalized_accuracy(self):
        fixtures = [{"fixture_id": "one", "plate_text_raw_when_visible": "ICT-001"}]
        predictions = {"one": {
            "plate_text_raw": "ICT001",
            "plate_text_normalized": "ICT001",
            "confidence": 0.8,
        }}
        result = MODULE.score_ocr(fixtures, predictions)
        self.assertEqual(0.0, result["raw_exact_plate_accuracy"])
        self.assertEqual(1.0, result["normalized_exact_plate_accuracy"])
        self.assertEqual(0.0, result["normalized_character_error_rate"])
        self.assertEqual(0, result["incorrect_high_confidence_acceptance_count"])

    def test_fixture_coverage_does_not_invent_missing_classes(self):
        fixtures = [{
            "fixture_id": "urdu_clear",
            "plate_regions_original_pixels": [{"bounds": {"x": 1, "y": 1, "width": 2, "height": 2}}],
            "script_when_visible": "Arabic-derived Urdu",
            "adverse_conditions": [],
        }]
        present = MODULE.present_fixture_classes(fixtures)
        self.assertEqual({"clear_single_plate", "regional_style"}, present)
        self.assertNotIn("multiple_vehicles", present)

    def test_threshold_checks_are_role_specific(self):
        metrics = {
            "localization": {"precision": 1.0, "recall": 1.0, "confidence_calibration": {"brier_score": 0.01}},
            "abstention": {"precision": 1.0},
            "latency_ms": {"p95": 700.0},
            "resources": {"peak_process_rss_mib": 700.0},
        }
        gate = {"promotion_thresholds": {"detector": {
            "minimum_localization_precision": 0.95,
            "minimum_localization_recall": 0.95,
            "maximum_brier_score": 0.15,
        }}}
        checks = MODULE.threshold_checks(MODULE.DETECTOR_ROLE, metrics, gate)
        self.assertTrue(all(check["passed"] for check in checks))
        specialized = MODULE.threshold_checks(MODULE.SPECIALIZED_DETECTOR_ROLE, metrics, gate)
        self.assertEqual(
            ["minimum_localization_precision", "minimum_localization_recall", "maximum_brier_score"],
            [check["metric"] for check in specialized],
        )
        self.assertTrue(all(check["passed"] for check in specialized))

    def test_hostile_admission_fixtures_are_excluded_from_model_lanes(self):
        fixtures = [
            {"fixture_id": "safe", "detector_evaluation_eligible": True, "ocr_evaluation_eligible": True},
            {"fixture_id": "hostile", "detector_evaluation_eligible": False, "ocr_evaluation_eligible": False},
        ]
        self.assertEqual(["safe"], [item["fixture_id"] for item in MODULE.fixtures_for_role(fixtures, MODULE.DETECTOR_ROLE)])
        self.assertEqual(["safe"], [item["fixture_id"] for item in MODULE.fixtures_for_role(fixtures, MODULE.SPECIALIZED_DETECTOR_ROLE)])
        self.assertEqual(["safe"], [item["fixture_id"] for item in MODULE.fixtures_for_role(fixtures, MODULE.OCR_ROLE)])

    def test_tuning_mode_rejects_sealed_holdout_before_reading_inputs(self):
        result = subprocess.run(
            [sys.executable, str(MODULE_PATH), "--fixtures", "missing.json", "--predictions", "missing.json", "--partition", "sealed_holdout", "--evaluation-mode", "tuning"],
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(2, result.returncode)
        self.assertIn("tuning mode cannot access a holdout partition", result.stderr)


if __name__ == "__main__":
    unittest.main()
