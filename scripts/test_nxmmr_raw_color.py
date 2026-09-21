# SPDX-License-Identifier: MIT
"""Non-oracle raw-color and unchanged upstream contract tests."""
import json
import sys
import unittest
from dataclasses import asdict
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT), str(ROOT / "scripts")]
import numpy as np
import cv2
from ingestion.forensic_records.video_anpr_product_v22 import (
    raw_color_input, color_contract, RawColorPlateFrameProcessor,
    VideoANPRProductV22Processor, INPUT_MODES,
)
from ingestion.forensic_records.video_anpr_parity_adapter import PlateFrameProcessor, PlateBounds, ParityAdapterError
from ingestion.forensic_records.video_anpr_product_v2 import VideoANPRProductV2Processor
import run_nxmmr_raw_color as runner
import evaluate_nxmmr_video_v2_exact_runtime as common


class RawColorTests(unittest.TestCase):
    def test_bgr_is_exact_original_and_rgb_is_exact_permutation(self):
        crop = np.arange(64 * 128 * 3, dtype=np.uint8).reshape(64, 128, 3)
        before = crop.copy()
        a, b = (raw_color_input(crop, mode) for mode in INPUT_MODES)
        np.testing.assert_array_equal(a, before)
        np.testing.assert_array_equal(b, cv2.cvtColor(before, cv2.COLOR_BGR2RGB))
        np.testing.assert_array_equal(crop, before)
        for result in (a, b):
            self.assertEqual(result.shape, crop.shape)
            self.assertEqual(result.dtype, crop.dtype)
            self.assertTrue(result.flags.c_contiguous)

    def test_noncontiguous_crop_keeps_spatial_values(self):
        crop = np.arange(900, dtype=np.uint8).reshape(10, 30, 3)[:, ::2]
        for mode in INPUT_MODES:
            actual = raw_color_input(crop, mode)
            np.testing.assert_array_equal(actual, crop if mode == INPUT_MODES[0] else crop[:, :, ::-1])
            self.assertTrue(actual.flags.c_contiguous)

    def test_malformed_input_and_third_mode_refused(self):
        for value in (None, [], np.zeros((4, 4), np.uint8), np.zeros((4, 4, 1), np.uint8),
                      np.zeros((4, 4, 4), np.uint8), np.zeros((0, 4, 3), np.uint8),
                      np.zeros((4, 0, 3), np.uint8), np.zeros((4, 4, 3), np.float32)):
            with self.assertRaises(ParityAdapterError):
                raw_color_input(value, "RAW_RGB")
        with self.assertRaises(ParityAdapterError):
            raw_color_input(np.zeros((4, 4, 3), np.uint8), "THIRD_ARM")

    def test_model_color_deviation_not_misrepresented(self):
        self.assertTrue(color_contract("RAW_BGR_PARITY")["HistoricalParityMode"])
        self.assertEqual(color_contract("RAW_BGR_PARITY")["OCRInputContract"], "DECLARED_COLOR_DEVIATION")
        self.assertEqual(color_contract("RAW_RGB")["OCRInputContract"], "PASS")

    def test_integrated_crop_detector_containment_and_provenance(self):
        frame = np.arange(64 * 128 * 3, dtype=np.uint8).reshape(64, 128, 3)
        boxes = [(PlateBounds(-2, 2, 60, 20), .9, 0), (PlateBounds(90, 20, 20, 10), .9, 0),
                 (PlateBounds(1, 1, 10, 5), .1, 0)]
        detector = SimpleNamespace(model_sha256="a" * 64, detect=lambda _: boxes)
        vehicle = SimpleNamespace(detect=lambda _: [(PlateBounds(0, 0, 80, 64), .9, 2)])
        calls = []
        ocr = SimpleNamespace(model_sha256="b" * 64, read=lambda image: (calls.append(image.copy()) or (("SYN123", .9),)))
        context = dict(source_sha256="c" * 64, source_file="synthetic", frame_number=0,
                       timestamp_seconds=0.0, source_frame_sha256="d" * 64)
        old = PlateFrameProcessor(detector, ocr, cv2, vehicle_detector=vehicle)
        prior, prior_crops, prior_detections = old.process_frame(frame, **context)
        outputs, traces = [], []
        for mode in INPUT_MODES:
            processor = RawColorPlateFrameProcessor(detector, ocr, cv2, vehicle_detector=vehicle, ocr_input_mode=mode)
            with patch.object(cv2, "threshold", side_effect=AssertionError("threshold forbidden")), patch.object(cv2, "cvtColor", side_effect=AssertionError("grayscale forbidden")):
                observations, crops, detections = processor.process_frame(frame, **context)
            self.assertEqual((crops, detections), (prior_crops, prior_detections))
            self.assertEqual(observations[0].crop_sha256, prior[0].crop_sha256)
            self.assertEqual(observations[0].bounds, prior[0].bounds)
            packet = observations[0].to_packet(evidence_id="synthetic", version_id="synthetic")
            self.assertEqual(packet["payload"]["ocr_input_mode"], mode)
            outputs.append(calls[-1])
            traces.append(processor.last_frame_trace)
        self.assertEqual(traces[0], traces[1])
        np.testing.assert_array_equal(outputs[0], frame[2:22, 0:58])
        np.testing.assert_array_equal(outputs[1], outputs[0][:, :, ::-1])

    def test_factory_preserves_all_non_input_policy(self):
        frame = SimpleNamespace(plate_detector=object(), vehicle_detector=object(), ocr=object(), cv2=cv2,
            detection_threshold=.25, ocr_threshold=0, binary_inverse_threshold=64,
            maximum_detections_per_frame=32, maximum_ocr_results_per_crop=4)
        old = SimpleNamespace(frame_processor=frame, cv2=cv2, maximum_frames=300, maximum_duration_seconds=900)
        for mode in INPUT_MODES:
            with patch.object(VideoANPRProductV2Processor, "from_environment", return_value=old):
                actual = VideoANPRProductV22Processor.from_environment(ocr_input_mode=mode)
            self.assertIs(actual.frame_processor.ocr, frame.ocr)
            self.assertIs(actual.frame_processor.plate_detector, frame.plate_detector)
            self.assertEqual(actual.maximum_frames, 300)
            self.assertEqual(actual.frame_processor.detection_threshold, .25)

    def test_prior_v21_inputs_preserved(self):
        manifest = json.loads((ROOT / "local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/v21-development/development-input-manifest.json").read_text())
        for name, digest in manifest["source_files"].items():
            self.assertEqual(common.hash_file(ROOT / name), digest)

    def test_reserved_and_third_arm_refused_before_launch(self):
        with patch.object(runner.subprocess, "Popen") as launch:
            for phase, mode in (("reserved", "RAW_RGB"), ("development", "THIRD_ARM")):
                with self.assertRaises(RuntimeError):
                    runner.run(phase, mode)
            launch.assert_not_called()


if __name__ == "__main__":
    import test_nxmmr_video_v21_contracts as v21
    loader = unittest.TestLoader()
    suite = loader.loadTestsFromTestCase(RawColorTests)
    suite.addTests(loader.loadTestsFromModule(v21))
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    raise SystemExit(not result.wasSuccessful())
