import unittest

from scripts.analyze_r8_anpr_errors import detector_analysis, ocr_analysis


class R8ErrorAnalysisTest(unittest.TestCase):
    def test_detector_false_positive_is_classified_and_swept(self):
        fixtures = [{"fixture_id": "negative_sign", "detector_evaluation_eligible": True,
                     "plate_regions_original_pixels": [], "adverse_conditions": ["signboard"]}]
        predictions = {"negative_sign": {"plate_regions_original_pixels": [
            {"bounds": {"x": 1, "y": 1, "width": 20, "height": 10}, "confidence": 0.8}
        ]}}
        result = detector_analysis(fixtures, predictions)
        self.assertEqual({"signage": 1}, result["false_positive_categories"])
        self.assertEqual(1, result["exploratory_confidence_sweep"][0]["false_positive"])
        self.assertEqual(0, result["exploratory_confidence_sweep"][-1]["false_positive"])

    def test_ocr_preserves_raw_and_reports_substitution(self):
        fixtures = [{"fixture_id": "plate", "ocr_evaluation_eligible": True,
                     "plate_text_raw_when_visible": "ABC-123", "script_when_visible": "Latin",
                     "adverse_conditions": ["blur"]}]
        predictions = {"plate": {"plate_text_raw": "ABC-I23", "plate_text_normalized": "ABCI23",
                                  "confidence": 0.9, "abstention_state": "candidate",
                                  "normalization_steps": ["uppercase"]}}
        result = ocr_analysis(fixtures, predictions)
        self.assertEqual("ABC-I23", result["by_fixture"][0]["actual_raw"])
        self.assertEqual(1, result["confusion_counts"]["substitution:1->I"])


if __name__ == "__main__":
    unittest.main()
