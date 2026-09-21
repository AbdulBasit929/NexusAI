from __future__ import annotations

import os
import unittest
from types import SimpleNamespace
from unittest.mock import patch

from ingestion.forensic_records.media_pipeline import ModelUnavailableError, configured_media_processor
from ingestion.forensic_records.video_anpr_onnx_v3 import (
    ADAPTER_ID,
    DETECTOR_SHA256,
    OCR_MODEL_SHA256,
    VideoANPROnnxV3Processor,
    observations_from_fastalpr_results,
)


class FakeFrame:
    shape = (100, 200, 3)

    def __getitem__(self, _key):
        return self


class FakeCV2:
    @staticmethod
    def imencode(_extension, _frame):
        return True, b"crop"


class VideoANPROnnxV3Tests(unittest.TestCase):
    def test_fastalpr_results_preserve_observed_text_and_source_time(self):
        result = SimpleNamespace(
            detection=SimpleNamespace(
                confidence=0.9,
                bounding_box=SimpleNamespace(x1=10, y1=20, x2=110, y2=60),
            ),
            ocr=SimpleNamespace(text=" ab-123 ", confidence=[0.8, 1.0]),
        )
        observations = observations_from_fastalpr_results(
            FakeFrame(),
            [result],
            source_sha256="a" * 64,
            source_file="sample.mp4",
            frame_number=25,
            timestamp_seconds=1.0,
            source_frame_sha256="b" * 64,
            detector_model_sha256=DETECTOR_SHA256,
            ocr_model_sha256=OCR_MODEL_SHA256,
            cv2_module=FakeCV2,
        )
        self.assertEqual(len(observations), 1)
        self.assertEqual(observations[0].raw_ocr, " ab-123 ")
        self.assertEqual(observations[0].normalized_ocr, "AB123")
        self.assertEqual(observations[0].timestamp_seconds, 1.0)
        self.assertIsNone(observations[0].vehicle_context_matched)
        self.assertAlmostEqual(observations[0].ocr_confidence, 0.9)

    def test_empty_ocr_is_not_promoted_to_an_observation(self):
        result = SimpleNamespace(
            detection=SimpleNamespace(
                confidence=0.9,
                bounding_box=SimpleNamespace(x1=10, y1=20, x2=110, y2=60),
            ),
            ocr=SimpleNamespace(text="---", confidence=[0.9]),
        )
        observations = observations_from_fastalpr_results(
            FakeFrame(), [result], source_sha256="a" * 64,
            source_file="sample.mp4", frame_number=0, timestamp_seconds=0.0,
            source_frame_sha256="b" * 64, detector_model_sha256=DETECTOR_SHA256,
            ocr_model_sha256=OCR_MODEL_SHA256, cv2_module=FakeCV2,
        )
        self.assertEqual(observations, ())

    def test_v2_and_v3_are_mutually_exclusive(self):
        environment = {
            "FORENSIC_ANPR_ENABLED": "false",
            "FORENSIC_OCR_ENABLED": "false",
            "FORENSIC_FACE_ENABLED": "false",
            "FORENSIC_IMAGE_EMBEDDING_ENABLED": "false",
            "FORENSIC_ASR_ENABLED": "false",
            "FORENSIC_VIDEO_ANPR_V2_ENABLED": "true",
            "FORENSIC_VIDEO_ANPR_V3_ENABLED": "true",
        }
        with patch.dict(os.environ, environment, clear=False):
            with self.assertRaisesRegex(ModelUnavailableError, "cannot be enabled together"):
                configured_media_processor()

    def test_processor_identity_is_demo_scoped(self):
        processor = VideoANPROnnxV3Processor(object(), FakeCV2())
        self.assertEqual(processor.processor_id, ADAPTER_ID)
        self.assertIn("development-demo", processor.processor_revision)


if __name__ == "__main__":
    unittest.main()
