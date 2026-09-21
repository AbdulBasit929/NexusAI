# SPDX-License-Identifier: MIT

from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

REPOSITORY = Path(__file__).resolve().parents[1]
if str(REPOSITORY) not in sys.path:
    sys.path.insert(0, str(REPOSITORY))

from ingestion.forensic_records.video_anpr_parity_adapter import (
    AggregationPolicy,
    CandidateModelUnavailable,
    FramePolicy,
    PlateBounds,
    PlateObservation,
    TimeInterval,
    aggregate_observations,
    coverage_state,
    make_coverage_receipt,
    normalize_plate_neutral,
    require_hash_pinned_asset,
    schedule_base_frames,
    schedule_refinement_frames,
    sha256_file,
)


def observation(value, timestamp, *, frame=None, x=100, confidence=0.8, suffix=""):
    frame_number = frame if frame is not None else round(timestamp * 10)
    return PlateObservation(
        observation_id=f"observation-{frame_number}-{x}-{value}-{suffix}",
        source_sha256="a" * 64,
        source_file="synthetic-development-video.mp4",
        frame_number=frame_number,
        timestamp_seconds=timestamp,
        source_frame_sha256=f"{frame_number:064x}"[-64:],
        bounds=PlateBounds(x, 100, 80, 24),
        raw_ocr=value,
        normalized_ocr=normalize_plate_neutral(value),
        detector_confidence=0.9,
        ocr_confidence=confidence,
        crop_sha256="b" * 64,
        detector_model_sha256="c" * 64,
        ocr_model_sha256="d" * 64,
        crop_quality=0.6,
    )


def policy(strategy="normalized_majority", **overrides):
    values = {
        "strategy": strategy,
        "maximum_temporal_gap_seconds": 1.0,
        "minimum_spatial_iou": 0.05,
        "maximum_center_distance_ratio": 2.0,
        "maximum_candidate_edit_distance": 1,
        "maximum_observations_per_group": 24,
        "maximum_active_groups": 128,
    }
    values.update(overrides)
    return AggregationPolicy(**values)


class VideoANPRParityAdapterClosureTests(unittest.TestCase):
    def test_neutral_normalization_does_not_substitute_characters(self):
        self.assertEqual(normalize_plate_neutral(" ab-0 i/8 "), "AB0I8")
        self.assertEqual(normalize_plate_neutral("O01IB8"), "O01IB8")

    def test_base_and_refinement_scheduling_are_bounded(self):
        fixed = FramePolicy("fixed", 2, maximum_analyzed_frames=20)
        self.assertEqual(
            schedule_base_frames(4, 10, fixed, allowed_intervals=(TimeInterval(1, 2),)),
            (1.0, 1.5, 2.0),
        )
        adaptive = FramePolicy(
            "adaptive", 2, refinement_enabled=True, refinement_window_seconds=0.5,
            refinement_cadence_fps=10, maximum_refinement_frames_per_trigger=4,
        )
        self.assertEqual(
            schedule_refinement_frames(
                (2,), 5, 10, adaptive, already_scheduled=(2,),
                allowed_intervals=(TimeInterval(1.8, 2.2),),
            ),
            (1.8, 1.9, 2.1, 2.2),
        )

    def test_repetition_conflict_and_ambiguity_are_preserved(self):
        groups = aggregate_observations([
            observation("ABC123", 1.0),
            observation("ABC128", 1.2),
            observation("ABC123", 1.4),
        ], policy())
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0].selected_normalized_plate, "ABC123")
        self.assertEqual([item.normalized_plate_text for item in groups[0].alternatives], [
            "ABC123", "ABC128"
        ])

    def test_equal_support_is_deterministic(self):
        values = [observation("ABC124", 1.0), observation("ABC123", 1.2)]
        self.assertEqual(
            aggregate_observations(values, policy()),
            aggregate_observations(reversed(values), policy()),
        )
        self.assertEqual(aggregate_observations(values, policy())[0].selected_normalized_plate, "ABC123")

    def test_best_confidence_and_weighted_support_are_distinct(self):
        values = [
            observation("ABC123", 1.0, confidence=0.7),
            observation("ABC123", 1.2, confidence=0.7),
            observation("ABC128", 1.4, confidence=0.99),
        ]
        self.assertEqual(
            aggregate_observations(values, policy("best_confidence"))[0].selected_normalized_plate,
            "ABC128",
        )
        self.assertEqual(
            aggregate_observations(values, policy("confidence_weighted_support"))[0].selected_normalized_plate,
            "ABC123",
        )

    def test_temporal_gap_and_spatial_distance_split_groups(self):
        self.assertEqual(len(aggregate_observations([
            observation("ABC123", 1), observation("ABC123", 4)
        ], policy(maximum_temporal_gap_seconds=0.5))), 2)
        self.assertEqual(len(aggregate_observations([
            observation("ABC123", 1, x=10), observation("ABC123", 1.1, x=800)
        ], policy(maximum_center_distance_ratio=1))), 2)

    def test_same_frame_multiple_plates_are_not_identity_merged(self):
        groups = aggregate_observations([
            observation("ABC123", 1, frame=10, x=10),
            observation("ABC128", 1, frame=10, x=20),
        ], policy())
        self.assertEqual(len(groups), 2)

    def test_duplicate_frame_result_keeps_stronger_observation(self):
        groups = aggregate_observations([
            observation("ABC123", 1, frame=10, confidence=0.2, suffix="weak"),
            observation("ABC123", 1, frame=10, confidence=0.9, suffix="strong"),
        ], policy())
        self.assertEqual(len(groups[0].observation_ids), 1)
        self.assertTrue(groups[0].best_observation_id.endswith("strong"))

    def test_one_crop_casts_one_vote_even_when_ocr_returns_conflicting_strings(self):
        groups = aggregate_observations([
            observation("ABC123", 1, frame=10, confidence=0.9, suffix="primary"),
            observation("ABC128", 1, frame=10, confidence=0.2, suffix="secondary"),
            observation("ABC123", 1.2, frame=12, confidence=0.8, suffix="next-frame"),
        ], policy())
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0].support_count, 2)
        self.assertEqual(len(groups[0].alternatives), 1)

    def test_empty_ocr_and_empty_input_produce_no_group(self):
        self.assertEqual(aggregate_observations([observation("---", 1)], policy()), ())
        self.assertEqual(aggregate_observations([], policy()), ())

    def test_minimum_support_omits_single_frame_group_without_dropping_raw_observation(self):
        raw = [observation("ABC123", 1)]
        self.assertEqual(len(raw), 1)
        self.assertEqual(
            aggregate_observations(raw, policy(minimum_selected_support=2)),
            (),
        )

    def test_packets_reject_tracking_and_interpolation_authority(self):
        group = aggregate_observations([
            observation("ABC123", 1), observation("ABC128", 1.2)
        ], policy())[0]
        packet = group.to_packet(
            source_file="synthetic-development-video.mp4",
            evidence_id="development-evidence",
            version_id="development-version",
        )
        self.assertFalse(packet["payload"]["persistent_tracking"])
        self.assertFalse(packet["payload"]["interpolated_observations"])
        self.assertEqual(packet["citation_locator"]["first_seen_seconds"], 1)
        self.assertEqual(len(packet["payload"]["alternatives"]), 2)

    def test_partial_zero_failure_and_resource_states_are_distinct(self):
        values = {
            "source_fps": 10, "source_duration_seconds": 2, "source_frame_count": 20,
            "frames_decoded": 4, "frames_analyzed_by_plate_detector": 4,
            "frames_refined": 0, "ocr_crops_processed": 0, "sampling_policy": "fixed",
            "dropped_or_failed_frames": 0, "wall_seconds": 1, "cpu_seconds": 1,
            "peak_ram_mib": 100,
        }
        partial = make_coverage_receipt(**values)
        self.assertEqual(coverage_state(partial, 0)[0], "complete_zero_sampled_frames")
        self.assertEqual(coverage_state(make_coverage_receipt(**{
            **values, "frames_decoded": 0, "frames_analyzed_by_plate_detector": 0
        }), 0)[0], "failed_zero_frames")
        self.assertEqual(coverage_state(make_coverage_receipt(**{
            **values, "resource_stop": True
        }), 0)[0], "resource_stopped")

    def test_hash_pinning_fails_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            model = Path(directory) / "model.bin"
            model.write_bytes(b"registered model bytes")
            self.assertEqual(require_hash_pinned_asset(model, sha256_file(model), "test"), model)
            with self.assertRaises(CandidateModelUnavailable):
                require_hash_pinned_asset(model, "0" * 64, "test")
            with self.assertRaises(CandidateModelUnavailable):
                require_hash_pinned_asset(Path(directory) / "missing.bin", "0" * 64, "test")


if __name__ == "__main__":
    unittest.main()
