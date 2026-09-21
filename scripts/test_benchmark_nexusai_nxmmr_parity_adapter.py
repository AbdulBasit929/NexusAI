# SPDX-License-Identifier: MIT

from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPTS = Path(__file__).resolve().parent
REPOSITORY = SCRIPTS.parent
for root in (SCRIPTS, REPOSITORY):
    if str(root) not in sys.path:
        sys.path.insert(0, str(root))

import benchmark_nexusai_nxmmr_parity_adapter as benchmark
from ingestion.forensic_records.video_anpr_parity_adapter import TimeInterval


class ParityAdapterBenchmarkTests(unittest.TestCase):
    def test_overlap_components_keep_simultaneous_events_in_one_partition(self):
        events = [
            {"event_id": "event-a", "start_seconds": "1.0", "end_seconds": "2.0"},
            {"event_id": "event-b", "start_seconds": "1.5", "end_seconds": "2.5"},
            {"event_id": "event-c", "start_seconds": "4.0", "end_seconds": "5.0"},
        ]
        components = benchmark.overlap_components(events)
        self.assertEqual([[item["event_id"] for item in group] for group in components], [
            ["event-a", "event-b"], ["event-c"]
        ])

    def test_reserved_interval_complement_is_deterministic(self):
        allowed = benchmark.complement_intervals(
            10.0,
            (TimeInterval(2.0, 3.0), TimeInterval(5.0, 6.0)),
        )
        self.assertEqual(allowed, (
            TimeInterval(0.0, 2.0),
            TimeInterval(3.0, 5.0),
            TimeInterval(6.0, 10.0),
        ))

    def test_frozen_output_refuses_changed_content(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "frozen.json"
            benchmark.write_new_json(path, {"value": 1})
            benchmark.write_new_json(path, {"value": 1})
            with self.assertRaisesRegex(RuntimeError, "refusing to overwrite"):
                benchmark.write_new_json(path, {"value": 2})

    def test_selection_rank_prefers_exact_f1_then_event_recall(self):
        def row(exact_f1, event_recall, candidate_id):
            return {
                "candidate_id": candidate_id,
                "coverage": {"frames_analyzed_by_plate_detector": 10, "wall_seconds": 1},
                "selected_aggregation_score": {
                    "events": {
                        "normalized_exact_plate": {"f1": exact_f1},
                        "event_detection_recall": event_recall,
                        "negative_frame_false_positive_rate": 0,
                    },
                    "grouping": {"exact_group_f1": 0.5},
                },
            }

        self.assertGreater(
            benchmark.strategy_rank(row(0.6, 0.5, "higher-exact")),
            benchmark.strategy_rank(row(0.5, 1.0, "higher-recall")),
        )

    def test_split_contract_never_authorizes_candidate_evaluation(self):
        self.assertEqual(
            benchmark.SPLIT_CONTRACT,
            "nexusai.nxmmr.video-development-reserved-split/v1",
        )
        self.assertEqual(benchmark.RESERVED_EVENT_TARGET, 7)

    def test_group_frame_rows_preserves_analyzed_zero_result_frames(self):
        rows = benchmark.group_frame_rows(
            (),
            (),
            (
                {"timestamp_seconds": 1.0, "latency_seconds": 0.1},
                {"timestamp_seconds": 2.0, "latency_seconds": 0.2},
            ),
        )
        self.assertEqual(rows, [
            {"timestamp_seconds": 1.0, "predicted_raw": [], "latency_seconds": 0.1},
            {"timestamp_seconds": 2.0, "predicted_raw": [], "latency_seconds": 0.2},
        ])

    def test_timestamp_filter_is_inclusive_and_split_scoped(self):
        intervals = (TimeInterval(1.0, 2.0), TimeInterval(4.0, 5.0))
        self.assertTrue(benchmark._timestamp_allowed(1.0, intervals))
        self.assertTrue(benchmark._timestamp_allowed(5.0, intervals))
        self.assertFalse(benchmark._timestamp_allowed(3.0, intervals))

    def test_minimum_support_candidates_are_bounded(self):
        config = json.loads(
            (REPOSITORY / "configuration" / "nxmmr_reference_parity_adapter_v1.json").read_text(
                encoding="utf-8"
            )
        )
        self.assertEqual(config["minimum_group_support_candidates"], [1, 2, 3])
        self.assertIn(
            "fixed-4fps-vehicle-majority",
            [candidate["id"] for candidate in config["development_candidates"]],
        )
        selected = next(
            candidate for candidate in config["development_candidates"]
            if candidate["id"] == "fixed-4fps-plate-only-majority-support2"
        )
        self.assertEqual(selected["minimum_selected_support"], 2)
        self.assertTrue(selected["one_vote_per_actual_crop"])


if __name__ == "__main__":
    unittest.main()
