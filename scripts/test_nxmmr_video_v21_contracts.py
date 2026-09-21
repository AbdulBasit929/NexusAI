# SPDX-License-Identifier: MIT
"""Synthetic contract tests; no evidence frames or oracle labels are read."""
import json
import sys
import tempfile
import time
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT), str(ROOT / "scripts")]
import numpy as np
import cv2
from ingestion.forensic_records.video_anpr_product_v21 import (
    thresholded_rgb_input, RGBCompatibleFastPlateOCR, V21PlateFrameProcessor,
    TRANSFORM_ID, VideoANPRProductV21Processor,
)
from ingestion.forensic_records.video_anpr_parity_adapter import ParityAdapterError, PlateBounds
from ingestion.forensic_records.video_anpr_product_v2 import (
    VideoANPRProductV2Processor, SELECTED_POLICY, OCR_MODEL_SHA256, OCR_CONFIG_SHA256,
)
import evaluate_nxmmr_video_v2_exact_runtime as evaluator
import run_nxmmr_video_v2_exact_runtime as runner
from nxmmr_evaluator_resources import finalized_resources


class ChannelContractTests(unittest.TestCase):
    def test_two_dimensional_pixel_identity(self):
        source = np.zeros((64, 128), dtype=np.uint8)
        source[:, ::2] = 255
        result = thresholded_rgb_input(source)
        self.assertEqual(result.shape, (64, 128, 3))
        self.assertEqual(result.dtype, np.uint8)
        self.assertTrue(result.flags.c_contiguous)
        for channel in range(3):
            np.testing.assert_array_equal(result[:, :, channel], source)
        self.assertEqual(set(np.unique(result)), {0, 255})

    def test_single_channel_three_channel_and_noncontiguous(self):
        source = np.zeros((16, 30), dtype=np.uint8)
        source[:, ::2] = 255
        for valid in (source[:, :, None], np.repeat(source[:, :, None], 3, 2), source[:, ::2]):
            result = thresholded_rgb_input(valid)
            plane = valid if valid.ndim == 2 else valid[:, :, 0]
            self.assertTrue(result.flags.c_contiguous)
            for channel in range(3):
                np.testing.assert_array_equal(result[:, :, channel], plane)

    def test_malformed_inputs_fail_explicitly(self):
        invalid = [None, [], np.zeros(()), np.zeros((4,), np.uint8),
                   np.zeros((2, 3, 4), np.uint8), np.zeros((2, 3, 2), np.uint8),
                   np.zeros((2, 3, 1, 1), np.uint8), np.zeros((0, 3), np.uint8),
                   np.zeros((3, 0), np.uint8), np.zeros((3, 4, 0), np.uint8),
                   np.zeros((2, 3), np.float32), np.zeros((2, 3), np.int16),
                   np.ones((2, 3), np.uint8)]
        for source in invalid:
            with self.subTest(shape=getattr(source, "shape", None)):
                with self.assertRaises(ParityAdapterError):
                    thresholded_rgb_input(source)

    def test_colored_raw_bgr_never_falls_back(self):
        colored = np.zeros((4, 5, 3), dtype=np.uint8)
        colored[:, :, 1] = 255
        with self.assertRaisesRegex(ParityAdapterError, "no raw-color fallback"):
            thresholded_rgb_input(colored)

    def test_actual_preprocessing_and_packet_provenance(self):
        calls = []
        def read(image):
            calls.append(image.copy())
            return (("SYN123", .9),)
        pinned = SimpleNamespace(model_sha256=OCR_MODEL_SHA256, read=read)
        bounds = PlateBounds(0, 0, 128, 64)
        detector = SimpleNamespace(model_sha256="a" * 64, detect=lambda _: [(bounds, .9, 0)])
        processor = V21PlateFrameProcessor(detector, RGBCompatibleFastPlateOCR(pinned), cv2,
                                           vehicle_detector=detector)
        source = np.zeros((64, 128, 3), np.uint8)
        source[:, :64] = 64
        source[:, 64:] = 65
        observations, crops, detections = processor.process_frame(
            source, source_sha256="b" * 64, source_file="synthetic", frame_number=0,
            timestamp_seconds=0, source_frame_sha256="c" * 64)
        self.assertEqual((crops, detections), (1, 1))
        self.assertTrue(np.all(calls[0][:, :64] == 255))
        self.assertTrue(np.all(calls[0][:, 64:] == 0))
        self.assertEqual(processor.binary_inverse_threshold, 64)
        self.assertEqual(observations[0].preprocessing, TRANSFORM_ID)
        packet = observations[0].to_packet(evidence_id="synthetic-evidence", version_id="synthetic-version")
        self.assertIn("replicate_thresholded_pixels_to_RGB_interface", packet["payload"]["transform_chain"])
        self.assertFalse(packet["payload"]["rgb_representation_adds_color_information"])
        self.assertEqual(packet["payload"]["raw_plate_text"], "SYN123")

    def test_factory_inherits_threshold_and_policies(self):
        frame = SimpleNamespace(plate_detector=object(), vehicle_detector=object(),
            ocr=SimpleNamespace(model_sha256=OCR_MODEL_SHA256), cv2=cv2,
            detection_threshold=.25, ocr_threshold=0, binary_inverse_threshold=64,
            maximum_detections_per_frame=32, maximum_ocr_results_per_crop=4)
        baseline = SimpleNamespace(frame_processor=frame, cv2=cv2, maximum_frames=300,
                                   maximum_duration_seconds=900)
        with patch.object(VideoANPRProductV2Processor, "from_environment", return_value=baseline):
            actual = VideoANPRProductV21Processor.from_environment()
        self.assertIs(actual.frame_processor.plate_detector, frame.plate_detector)
        self.assertIs(actual.frame_processor.vehicle_detector, frame.vehicle_detector)
        self.assertIs(actual.frame_processor.ocr.pinned_ocr, frame.ocr)
        self.assertEqual(actual.frame_processor.binary_inverse_threshold, 64)
        self.assertEqual(SELECTED_POLICY.minimum_selected_support, 3)
        self.assertEqual(SELECTED_POLICY.minimum_best_ocr_confidence, .5)
        self.assertEqual(SELECTED_POLICY.minimum_weighted_support, 1.5)

    def test_original_source_and_model_config_hashes_unchanged(self):
        manifest = json.loads((ROOT / "configuration/nxmmr_anpr_ocr_vertical_activation_v1.json").read_text())
        for name, digest in manifest["source_freeze_files"].items():
            self.assertEqual(evaluator.hash_file(ROOT / name), digest)
        self.assertEqual(evaluator.hash_file(ROOT / "local-acceptance-models/nxmmr/cct_xs_v2_global.onnx"), OCR_MODEL_SHA256)
        self.assertEqual(evaluator.hash_file(ROOT / "local-acceptance-models/nxmmr/cct_xs_v2_global_plate_config.yaml"), OCR_CONFIG_SHA256)


class ResourceFinalizationTests(unittest.TestCase):
    def test_temporal_packet_requires_matching_actual_source_time(self):
        event = {"event_type": "plate", "plate_text": "SYN123", "start_seconds": 1, "end_seconds": 2}
        groups = [SimpleNamespace(selected_normalized_plate="SYN123", observation_ids=["x"])]
        score = {"event_summary": {"negative_frame_false_positive_rate": None},
                 "grouping": {"exact_group_fp": 0, "exact_group_precision": 1}}
        observations = [SimpleNamespace(observation_id="x", timestamp_seconds=3)]
        actual = evaluator.stage_metrics([event], [], groups, score, observations)
        self.assertEqual(actual["false_temporal_packet_count_by_plate_value"], 0)
        self.assertEqual(actual["false_temporal_packet_count_by_plate_and_actual_time"], 1)

    def record(self, root, monitor):
        return finalized_resources(root / "resources.json", monitor, phase="test",
            started=time.perf_counter(), cpu_started=time.process_time())

    def test_success_finalizes(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            with self.record(root, SimpleNamespace(finish=lambda: {"process_peak_rss_mib": 1})):
                pass
            receipt = json.loads((root / "resources.json").read_text())
            self.assertEqual(receipt["state"], "PASS")
            self.assertEqual(receipt["resources"]["process_peak_rss_mib"], 1)

    def test_exception_finalizes_and_original_survives(self):
        original = ValueError("synthetic processing exception")
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            with self.assertRaises(ValueError) as raised:
                with self.record(root, SimpleNamespace(finish=lambda: {"process_peak_rss_mib": 1})):
                    raise original
            self.assertIs(raised.exception, original)
            receipt = json.loads((root / "resources.json").read_text())
            self.assertEqual(receipt["state"], "FAIL")
            self.assertEqual(receipt["error"]["message"], str(original))

    def test_recording_failure_never_masks_processing_failure(self):
        def broken():
            raise OSError("synthetic recording exception")
        original = ValueError("original")
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaises(ValueError) as raised:
                with self.record(Path(folder), SimpleNamespace(finish=broken)):
                    raise original
            self.assertIs(raised.exception, original)
            self.assertIn("recording exception", original.__notes__[0])

    def test_v21_reserved_refused_before_any_launch(self):
        with patch.object(runner, "CANDIDATE", "v21"), patch.object(runner.subprocess, "Popen") as launch:
            with self.assertRaisesRegex(RuntimeError, "forbids reserved"):
                runner.run("reserved")
            launch.assert_not_called()


if __name__ == "__main__":
    unittest.main()
