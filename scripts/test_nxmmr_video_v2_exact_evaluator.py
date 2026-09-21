# SPDX-License-Identifier: MIT
"""Non-oracle, dependency-free tests of the added evaluation harness."""
import json
import csv
import io
import sys
import unittest
import tempfile
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import evaluate_nxmmr_video_v2_exact_runtime as evaluator
import provision_nxmmr_video_v2_runtime as provision
from project_nxmmr_video_development_oracle import selected_records
import run_nxmmr_video_v2_exact_runtime as runner


class EvaluatorTests(unittest.TestCase):
    def test_reserved_host_guard_refuses_failed_development_before_launch(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            for name in ("imports", "smoke"):
                (root / (name + ".json")).write_text(json.dumps({"state": "PASS"}))
            (root / "development.json").write_text(json.dumps({"numeric_utility_gate": "FAIL"}))
            (root / "development-admission.json").write_text(json.dumps({"development_gate": "FAIL"}))
            with patch.object(runner, "RESULTS", root), patch.object(runner, "available_ram_gib", return_value=4), patch.object(runner.subprocess, "Popen") as launch:
                with self.assertRaises(AssertionError):
                    runner.run("reserved")
                launch.assert_not_called()

    def test_completed_phase_cannot_be_repeated(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            (root / "reserved.json").write_text("{}")
            with patch.object(runner, "RESULTS", root), patch.object(runner.subprocess, "Popen") as launch:
                with self.assertRaises(RuntimeError):
                    runner.run("reserved")
                launch.assert_not_called()

    def test_projection_discards_nonallowlisted_multiline_records(self):
        raw = 'event_id,plate_text\nDEV,"SYN123\nSYN456"\nLOCKED,"DO_NOT_RETURN\nSECOND_LINE"\n'
        projected = ''.join(selected_records(io.StringIO(raw), {"DEV"}))
        self.assertNotIn("DO_NOT_RETURN", projected)
        self.assertNotIn("SECOND_LINE", projected)
        self.assertEqual(list(csv.DictReader(io.StringIO(projected))), [{"event_id": "DEV", "plate_text": "SYN123\nSYN456"}])

    def test_projection_handles_escaped_quotes_and_missing_final_newline(self):
        raw = 'event_id,plate_text\nLOCKED,"SKIP ""QUOTED""\nVALUE"\nDEV,"SYN""TEXT"'
        projected = ''.join(selected_records(io.StringIO(raw), {"DEV"}))
        self.assertNotIn("SKIP", projected)
        self.assertEqual(list(csv.DictReader(io.StringIO(projected)))[0]["plate_text"], 'SYN"TEXT')

    def test_core_pins_equal_frozen_declaration(self):
        freeze = json.loads((evaluator.ROOT / "local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/video-v2-product-source-freeze.json").read_text())
        for name, version in evaluator.CORE.items():
            self.assertEqual(freeze["runtime"][name.replace("-", "_")], version)
        self.assertEqual(evaluator.CORE, provision.CORE)

    def test_prospective_gate_boundaries(self):
        events = {"event_detection_recall": .75, "normalized_exact_plate": {"f1": .6}}
        groups = {"exact_group_precision": .8, "exact_group_f1": .65}
        self.assertTrue(all(evaluator.utility_checks(events, groups).values()))
        events["normalized_exact_plate"]["f1"] = .599999
        self.assertFalse(evaluator.utility_checks(events, groups)["exact_f1"])

    def test_missing_scores_cannot_admit(self):
        events = {"event_detection_recall": None, "normalized_exact_plate": {"f1": None}}
        groups = {"exact_group_precision": None, "exact_group_f1": None}
        self.assertFalse(any(evaluator.utility_checks(events, groups).values()))

    def test_raw_ocr_and_group_negative_fpr_are_distinct(self):
        events = [{"event_type": "plate", "plate_text": "SYN123", "start_seconds": 1, "end_seconds": 2}]
        frames = [{"timestamp_seconds": 0, "detections": 1, "predicted_raw": []},
                  {"timestamp_seconds": 3, "detections": 1, "predicted_raw": ["NOISE9"]},
                  {"timestamp_seconds": 1.5, "detections": 1, "predicted_raw": ["SYN123"]}]
        score = {"event_summary": {"negative_frame_false_positive_rate": 0},
                 "grouping": {"exact_group_fp": 0, "exact_group_precision": 1}}
        stages = evaluator.stage_metrics(events, frames, [], score)
        self.assertEqual(stages["RawDetectorNegativeFrameFPR"], 1)
        self.assertEqual(stages["OCRCandidateNegativeFrameFPR"], .5)
        self.assertEqual(stages["GroupSelectedNegativeFrameFPR"], 0)

    def test_zero_negative_denominator_is_unavailable(self):
        score = {"event_summary": {"negative_frame_false_positive_rate": None},
                 "grouping": {"exact_group_fp": 0, "exact_group_precision": None}}
        stages = evaluator.stage_metrics([], [], [], score)
        self.assertIsNone(stages["RawDetectorNegativeFrameFPR"])
        self.assertIsNone(stages["OCRCandidateNegativeFrameFPR"])

    def test_unique_false_groups_distinguished_from_packets(self):
        score = {"event_summary": {"negative_frame_false_positive_rate": None},
                 "grouping": {"exact_group_fp": 1, "exact_group_precision": 0}}
        groups = [SimpleNamespace(selected_normalized_plate="SYN999") for _ in range(2)]
        stages = evaluator.stage_metrics([], [], groups, score)
        self.assertEqual(stages["FinalFalseGroupCount"], 1)
        self.assertEqual(stages["false_temporal_packet_count_by_plate_value"], 2)


if __name__ == "__main__":
    unittest.main()
