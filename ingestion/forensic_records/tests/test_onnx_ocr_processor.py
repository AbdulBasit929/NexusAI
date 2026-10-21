# SPDX-License-Identifier: Apache-2.0
"""The ONNX OCR role with a fake detector and recognisers: contract, reading order, candidate selection, startup refusals. No model files are needed."""

import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import cv2
import numpy as np

from ingestion.forensic_records.media_pipeline import ModelUnavailableError
from ingestion.forensic_records.onnx_ocr_processor import OnnxPaddleOCRProcessor


class FakeDetector:
    def __init__(self, boxes):
        self.boxes = boxes

    def detect(self, image):
        return self.boxes


class FakeRecognizer:
    def __init__(self, outputs):
        self.outputs = list(outputs)

    def recognize(self, image):
        return self.outputs.pop(0)


def quad(x0, y0, x1, y1):
    return np.array([[x0, y0], [x1, y0], [x1, y1], [x0, y1]], np.float32)


class ProcessorTests(unittest.TestCase):
    def run_processor(self, boxes, recognizers):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "page.png"
            cv2.imwrite(str(path), np.full((200, 300, 3), 255, np.uint8))
            processor = OnnxPaddleOCRProcessor(FakeDetector(boxes), recognizers, model_hashes={"detector": "d" * 64, "latin": "l" * 64})
            return processor.process_image(path, evidence_id="e1", version_id="v1", source_sha256="a" * 64, source_file="page.png")

    def test_regions_are_read_top_to_bottom_with_identifiers_preserved_and_review_required(self):
        boxes = [quad(10, 120, 200, 150), quad(10, 20, 200, 50)]  # the lower box is listed first
        result = self.run_processor(boxes, {"latin": FakeRecognizer([("Phone 0300-1234567", 0.97), ("Total Rs. 1,200", 0.91)])})
        self.assertEqual(result.readiness, "READY")
        self.assertEqual([o.payload["reading_order"] for o in result.observations], [1, 2])
        # reading order is by position, the recogniser was called in that order: the upper box first
        self.assertEqual(result.observations[0].payload["raw_text"], "Phone 0300-1234567")
        self.assertEqual(result.observations[0].payload["identifiers_preserved"], ["0300-1234567"])
        for observation in result.observations:
            self.assertTrue(observation.payload["manual_review_required"])
            self.assertEqual(observation.observation_type, "general_image_ocr_observation")
            self.assertIn("bbox", observation.citation_locator)
            self.assertEqual(observation.citation_locator["coordinate_space"], "original_image_pixels")
        self.assertEqual(result.metadata["result_state"], "COMPLETE_RESULTS")

    def test_a_region_with_no_text_is_dropped_and_zero_results_are_stated(self):
        result = self.run_processor([quad(10, 20, 200, 50)], {"latin": FakeRecognizer([("   ", 0.2)])})
        self.assertEqual(result.observations, ())
        self.assertEqual(result.metadata["result_state"], "COMPLETE_ZERO_RESULTS")
        self.assertEqual(result.readiness, "READY_WITH_WARNINGS")

    def test_the_candidate_with_the_higher_confidence_is_selected_and_both_are_kept(self):
        recognizers = {"latin": FakeRecognizer([("abc", 0.6)]), "arabic_multilingual": FakeRecognizer([("xyz", 0.9)])}
        with patch.dict(os.environ, {}, clear=False):
            result = self.run_processor([quad(10, 20, 200, 50)], recognizers)
        payload = result.observations[0].payload
        self.assertEqual(payload["selected_recognizer"], "arabic_multilingual")
        self.assertEqual({c["recognizer_role"] for c in payload["recognizer_candidates"]}, {"latin", "arabic_multilingual"})

    def test_without_the_arabic_recogniser_the_limitation_is_stated(self):
        result = self.run_processor([quad(10, 20, 200, 50)], {"latin": FakeRecognizer([("abc", 0.9)])})
        self.assertTrue(any("Arabic-script" in item for item in result.limitations))


class StartupTests(unittest.TestCase):
    def test_missing_pinned_files_refuse_to_start(self):
        with patch.dict(os.environ, {}, clear=False):
            for name in ("FORENSIC_ONNX_OCR_DETECTOR_PATH", "FORENSIC_ONNX_OCR_LATIN_MODEL_PATH"):
                os.environ.pop(name, None)
            with self.assertRaises(ModelUnavailableError):
                OnnxPaddleOCRProcessor.from_environment()

    def test_a_hash_that_does_not_match_refuses_to_start(self):
        with tempfile.TemporaryDirectory() as directory:
            model = Path(directory) / "inference.onnx"
            model.write_bytes(b"not a model")
            env = {"FORENSIC_ONNX_OCR_DETECTOR_PATH": str(model), "FORENSIC_ONNX_OCR_DETECTOR_SHA256": "0" * 64}
            with patch.dict(os.environ, env, clear=False):
                with self.assertRaises(ModelUnavailableError):
                    OnnxPaddleOCRProcessor.from_environment()


if __name__ == "__main__":
    unittest.main()
