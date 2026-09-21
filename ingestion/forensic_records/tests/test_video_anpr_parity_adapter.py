# SPDX-License-Identifier: MIT

from __future__ import annotations

import json
from pathlib import Path

import pytest

from ingestion.forensic_records.video_anpr_parity_adapter import (
    ADAPTER_ID,
    AGGREGATION_REVISION,
    CandidateModelUnavailable,
    AggregationPolicy,
    FramePolicy,
    PlateBounds,
    PlateObservation,
    TimeInterval,
    aggregate_observations,
    build_freeze_manifest,
    canonical_digest,
    coverage_state,
    make_coverage_receipt,
    normalize_plate_neutral,
    require_hash_pinned_asset,
    schedule_base_frames,
    schedule_refinement_frames,
    sha256_file,
)


def observation(
    value: str,
    timestamp: float,
    *,
    frame: int | None = None,
    x: float = 100,
    confidence: float = 0.8,
    detector_confidence: float = 0.9,
    suffix: str = "",
) -> PlateObservation:
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
        detector_confidence=detector_confidence,
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


def test_neutral_normalization_preserves_observed_characters_without_substitution():
    assert normalize_plate_neutral(" ab-0 i/8 ") == "AB0I8"
    assert normalize_plate_neutral("O01IB8") == "O01IB8"


def test_schedule_uses_actual_frame_grid_and_allowed_intervals():
    frame_policy = FramePolicy("fixed-2fps", base_cadence_fps=2, maximum_analyzed_frames=20)
    scheduled = schedule_base_frames(
        4.0,
        10.0,
        frame_policy,
        allowed_intervals=(TimeInterval(1.0, 2.0), TimeInterval(3.0, 3.5)),
    )
    assert scheduled == (1.0, 1.5, 2.0, 3.0, 3.5)


def test_schedule_is_strictly_bounded_and_zero_video_is_empty():
    frame_policy = FramePolicy("bounded", base_cadence_fps=10, maximum_analyzed_frames=3)
    assert schedule_base_frames(5, 20, frame_policy) == (0.0, 0.1, 0.2)
    assert schedule_base_frames(0, 20, frame_policy) == ()


def test_refinement_stays_in_allowed_intervals_and_excludes_coarse_frames():
    frame_policy = FramePolicy(
        "adaptive",
        base_cadence_fps=2,
        refinement_enabled=True,
        refinement_window_seconds=0.5,
        refinement_cadence_fps=10,
        maximum_refinement_frames_per_trigger=4,
    )
    refined = schedule_refinement_frames(
        (2.0,),
        5.0,
        10.0,
        frame_policy,
        already_scheduled=(2.0,),
        allowed_intervals=(TimeInterval(1.8, 2.2),),
    )
    assert refined == (1.8, 1.9, 2.1, 2.2)


def test_same_string_repeated_forms_one_bounded_group():
    groups = aggregate_observations(
        [observation("ABC123", 1.0), observation("ABC123", 1.2), observation("ABC123", 1.4)],
        policy(),
    )
    assert len(groups) == 1
    assert groups[0].selected_normalized_plate == "ABC123"
    assert groups[0].support_count == 3
    assert groups[0].first_observed_seconds == 1.0
    assert groups[0].last_observed_seconds == 1.4


def test_one_character_conflict_preserves_alternative_and_majority_wins():
    groups = aggregate_observations(
        [observation("ABC123", 1.0), observation("ABC128", 1.2), observation("ABC123", 1.4)],
        policy(),
    )
    assert groups[0].selected_normalized_plate == "ABC123"
    assert [item.normalized_plate_text for item in groups[0].alternatives] == ["ABC123", "ABC128"]
    assert groups[0].alternatives[1].support_count == 1


def test_equal_support_tie_is_deterministic():
    inputs = [observation("ABC124", 1.0), observation("ABC123", 1.2)]
    first = aggregate_observations(inputs, policy())
    second = aggregate_observations(reversed(inputs), policy())
    assert first == second
    assert first[0].selected_normalized_plate == "ABC123"


def test_best_confidence_can_select_single_stronger_reading():
    groups = aggregate_observations(
        [
            observation("ABC123", 1.0, confidence=0.55),
            observation("ABC123", 1.2, confidence=0.56),
            observation("ABC128", 1.4, confidence=0.99),
        ],
        policy("best_confidence"),
    )
    assert groups[0].selected_normalized_plate == "ABC128"


def test_confidence_weighted_support_rejects_low_confidence_outlier():
    groups = aggregate_observations(
        [
            observation("ABC123", 1.0, confidence=0.9),
            observation("ABC123", 1.2, confidence=0.9),
            observation("ABC128", 1.4, confidence=0.01, detector_confidence=0.1),
        ],
        policy("confidence_weighted_support"),
    )
    assert groups[0].selected_normalized_plate == "ABC123"


def test_large_temporal_gap_splits_same_text():
    groups = aggregate_observations(
        [observation("ABC123", 1.0), observation("ABC123", 4.0)],
        policy(maximum_temporal_gap_seconds=0.5),
    )
    assert len(groups) == 2


def test_nearby_separate_events_split_at_gap_boundary():
    groups = aggregate_observations(
        [observation("ABC123", 1.0), observation("ABC123", 2.01)],
        policy(maximum_temporal_gap_seconds=1.0),
    )
    assert len(groups) == 2


def test_spatially_distant_observations_do_not_merge():
    groups = aggregate_observations(
        [observation("ABC123", 1.0, x=10), observation("ABC123", 1.1, x=800)],
        policy(maximum_center_distance_ratio=1.0),
    )
    assert len(groups) == 2


def test_multiple_plates_in_same_frame_do_not_create_identity_merge():
    groups = aggregate_observations(
        [observation("ABC123", 1.0, frame=10, x=10), observation("ABC128", 1.0, frame=10, x=20)],
        policy(),
    )
    assert len(groups) == 2


def test_duplicate_frame_result_is_deduplicated_by_stronger_support():
    groups = aggregate_observations(
        [
            observation("ABC123", 1.0, frame=10, confidence=0.2, suffix="weak"),
            observation("ABC123", 1.0, frame=10, confidence=0.9, suffix="strong"),
        ],
        policy(),
    )
    assert len(groups) == 1
    assert len(groups[0].observation_ids) == 1
    assert groups[0].best_observation_id.endswith("strong")


def test_one_crop_casts_one_temporal_vote_when_ocr_returns_multiple_strings():
    groups = aggregate_observations(
        [
            observation("ABC123", 1.0, frame=10, confidence=0.9, suffix="primary"),
            observation("ABC128", 1.0, frame=10, confidence=0.2, suffix="secondary"),
            observation("ABC123", 1.2, frame=12, confidence=0.8, suffix="next"),
        ],
        policy(),
    )
    assert len(groups) == 1
    assert groups[0].support_count == 2
    assert len(groups[0].alternatives) == 1


def test_invalid_or_empty_ocr_is_not_grouped():
    assert aggregate_observations([observation("---", 1.0)], policy()) == ()
    assert aggregate_observations([], policy()) == ()


def test_minimum_group_support_filters_single_frame_noise_only_from_group_output():
    raw = [observation("ABC123", 1.0)]
    assert len(raw) == 1
    assert aggregate_observations(raw, policy(minimum_selected_support=2)) == ()


def test_group_packet_preserves_raw_alternatives_and_rejects_tracking_semantics():
    group = aggregate_observations(
        [observation("ab-c123", 1.0), observation("ABC128", 1.2)],
        policy(),
    )[0]
    packet = group.to_packet(
        source_file="synthetic-development-video.mp4",
        evidence_id="evidence-development",
        version_id="version-development",
    )
    assert packet["contract_version"] == "forensics.video-anpr-plate-group/v1"
    assert packet["payload"]["persistent_tracking"] is False
    assert packet["payload"]["interpolated_observations"] is False
    assert packet["payload"]["aggregation_revision"] == AGGREGATION_REVISION
    assert packet["citation_locator"]["first_seen_seconds"] == 1.0
    assert len(packet["payload"]["alternatives"]) == 2


def test_raw_observation_packet_has_actual_source_time_and_model_authority():
    packet = observation("ab-c123", 1.25, frame=75).to_packet(
        evidence_id="evidence-development", version_id="version-development"
    )
    assert packet["authority"] == "model_observation"
    assert packet["citation_locator"]["timestamp_seconds"] == 1.25
    assert packet["citation_locator"]["frame_number"] == 75
    assert packet["payload"]["raw_plate_text"] == "ab-c123"
    assert packet["payload"]["normalized_plate_text"] == "ABC123"
    assert packet["payload"]["interpolated"] is False


def receipt(**overrides):
    values = {
        "source_fps": 10,
        "source_duration_seconds": 2,
        "source_frame_count": 20,
        "frames_decoded": 4,
        "frames_analyzed_by_plate_detector": 4,
        "frames_refined": 0,
        "ocr_crops_processed": 0,
        "sampling_policy": "fixed-2fps",
        "dropped_or_failed_frames": 0,
        "wall_seconds": 1.25,
        "cpu_seconds": 1.0,
        "peak_ram_mib": 200,
    }
    values.update(overrides)
    return make_coverage_receipt(**values)


def test_partial_zero_state_never_claims_no_plates_in_video():
    state, limitations = coverage_state(receipt(), 0)
    assert state == "complete_zero_sampled_frames"
    assert "partial" in limitations[0]


def test_zero_frames_model_missing_and_resource_stop_states_are_distinct():
    assert coverage_state(receipt(frames_decoded=0, frames_analyzed_by_plate_detector=0), 0)[0] == "failed_zero_frames"
    assert coverage_state(receipt(frames_analyzed_by_plate_detector=0), 0)[0] == "processor_failed"
    assert coverage_state(receipt(resource_stop=True), 0)[0] == "resource_stopped"


def test_full_coverage_zero_can_use_complete_zero_semantics():
    full = receipt(frames_decoded=20, frames_analyzed_by_plate_detector=20)
    assert coverage_state(full, 0) == ("complete_zero_full_coverage", ())


def test_hash_pinned_asset_accepts_exact_bytes_and_rejects_missing_or_changed(tmp_path):
    model = tmp_path / "model.bin"
    model.write_bytes(b"registered model bytes")
    expected = sha256_file(model)
    assert require_hash_pinned_asset(model, expected, "test") == model
    with pytest.raises(CandidateModelUnavailable, match="SHA-256 mismatch"):
        require_hash_pinned_asset(model, "0" * 64, "test")
    with pytest.raises(CandidateModelUnavailable, match="missing"):
        require_hash_pinned_asset(tmp_path / "missing.bin", expected, "test")


def test_freeze_digest_is_stable_and_excludes_candidate_evaluation():
    config = {
        "source_commit": "base-commit",
        "models": {"plate": {"sha256": "a" * 64}},
        "normalization": {"revision": "neutral"},
        "bounds": {"maximum_frames": 5},
        "thresholds": {"detector": 0.25},
    }
    selected = {"id": "candidate", "aggregation": "normalized_majority"}
    first = build_freeze_manifest(
        config,
        adapter_source_sha256="b" * 64,
        selected_candidate=selected,
        development_split_digest="c" * 64,
    )
    second = build_freeze_manifest(
        json.loads(json.dumps(config)),
        adapter_source_sha256="b" * 64,
        selected_candidate=selected,
        development_split_digest="c" * 64,
    )
    assert first == second
    assert first["freeze_digest"] == canonical_digest({
        key: value for key, value in first.items() if key != "freeze_digest"
    })
    assert first["candidate_evaluation_performed"] is False
    assert first["adapter_id"] == ADAPTER_ID


def test_configuration_declares_no_persistent_or_interpolated_semantics():
    config_path = Path(__file__).parents[3] / "configuration" / "nxmmr_reference_parity_adapter_v1.json"
    config = json.loads(config_path.read_text(encoding="utf-8"))
    assert config["activation_state"] == "source_development_only"
    assert "persistent_tracking" in config["forbidden_semantics"]
    assert "interpolated_observation" in config["forbidden_semantics"]
    assert config["normalization"]["uk_character_substitution"] is False
    assert config["normalization"]["raw_ocr_preserved"] is True
