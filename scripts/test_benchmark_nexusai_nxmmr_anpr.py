#!/usr/bin/env python3

import importlib.util
import unittest
from pathlib import Path


PATH = Path(__file__).with_name("benchmark_nexusai_nxmmr_anpr.py")
SPEC = importlib.util.spec_from_file_location("nxmmr_anpr", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(MODULE)


class ANPRScoringTests(unittest.TestCase):
    def test_plate_normalization_and_exact_counts(self) -> None:
        score = MODULE.score_plate_lists(["ICT-001"], ["ict 001"])
        self.assertEqual(1, score["exact_tp"])
        self.assertEqual(0, score["raw_exact_tp"])
        self.assertEqual(0, score["character_errors"])

    def test_miss_is_not_silently_paired(self) -> None:
        score = MODULE.score_plate_lists(["ABC123"], [])
        self.assertEqual(1, score["detection_fn"])
        self.assertEqual(6, score["character_errors"])

    def test_aggregate_retains_negative_false_positive(self) -> None:
        rows = [
            {"score": MODULE.score_plate_lists(["ABC123"], ["ABC123"]), "latency_seconds": 0.1},
            {"score": MODULE.score_plate_lists([], ["NOISE"]), "latency_seconds": 0.2},
        ]
        summary = MODULE.aggregate(rows, unit="frame")
        self.assertEqual({"tp": 1, "fp": 1, "fn": 0, "tn": 0}, summary["presence_confusion"])
        self.assertEqual(0.0, summary["presence_specificity"])

    def test_video_scores_events_gaps_grouping_and_time(self) -> None:
        events = [{
            "event_id": "event-1", "event_type": "plate", "plate_text": "ABC123",
            "start_seconds": 1.0, "end_seconds": 2.0, "readability": "readable",
        }]
        frames = [
            {"timestamp_seconds": 0.0, "predicted_raw": [], "latency_seconds": 0.1},
            {"timestamp_seconds": 1.0, "predicted_raw": ["ABC 123"], "latency_seconds": 0.2},
            {"timestamp_seconds": 2.0, "predicted_raw": ["ABC 123"], "latency_seconds": 0.2},
            {"timestamp_seconds": 3.0, "predicted_raw": ["FALSE"], "latency_seconds": 0.1},
        ]
        result = MODULE.score_video(events, frames)
        self.assertEqual(1.0, result["event_summary"]["event_detection_recall"])
        self.assertEqual(1, result["event_summary"]["false_positive_negative_frames"])
        self.assertEqual(1.0, result["grouping"]["exact_group_recall"])
        self.assertEqual(0.0, result["grouping"]["source_time_error_seconds"]["mean_absolute_boundary_error"])


if __name__ == "__main__":
    unittest.main()
