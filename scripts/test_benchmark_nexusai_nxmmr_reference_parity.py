#!/usr/bin/env python3

import importlib.util
import sys
import unittest
from pathlib import Path


SCRIPTS = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPTS))
PATH = SCRIPTS / "benchmark_nexusai_nxmmr_reference_parity.py"
SPEC = importlib.util.spec_from_file_location("nxmmr_reference_parity", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(MODULE)


class ReferenceParityScoringTests(unittest.TestCase):
    def test_track_candidate_uses_highest_confidence(self) -> None:
        raw = [
            {"car_id": "1", "license_number_score": "0.2", "license_number": "WRONG"},
            {"car_id": "1", "license_number_score": "0.9", "license_number": "RIGHT"},
        ]
        interpolated = [{"car_id": "1", "frame_nmr": "3"}]
        self.assertEqual("RIGHT", MODULE.selected_track_rows(raw, interpolated)[0]["license_number"])

    def test_dense_frame_loader_keeps_empty_frames(self) -> None:
        frames = MODULE.load_video_frames([{"frame_nmr": "2", "license_number": "ABC123"}])
        self.assertEqual(3600, len(frames))
        self.assertEqual([], frames[1]["predicted_raw"])
        self.assertEqual(["ABC123"], frames[2]["predicted_raw"])

    def test_frame_presence_counts_temporal_false_positive(self) -> None:
        events = [{"start_seconds": 1.0, "end_seconds": 1.0}]
        frames = [
            {"timestamp_seconds": 0.0, "predicted_raw": ["FP"]},
            {"timestamp_seconds": 1.0, "predicted_raw": ["TP"]},
        ]
        result = MODULE.frame_presence_summary(events, frames)
        self.assertEqual((1, 1, 0, 0), (result["tp"], result["fp"], result["fn"], result["tn"]))


if __name__ == "__main__":
    unittest.main()
