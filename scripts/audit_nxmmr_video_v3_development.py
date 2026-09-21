# SPDX-License-Identifier: MIT
"""Independently recompute the completed Video V3 development receipt."""

from __future__ import annotations

import csv
import hashlib
import json
import sys
from pathlib import Path
from types import SimpleNamespace

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT), str(ROOT / "scripts")]

import evaluate_nxmmr_video_v2_exact_runtime as evaluator
from benchmark_nexusai_nxmmr_parity_adapter import group_frame_rows
from ingestion.forensic_records.video_anpr_parity_adapter import TimeInterval


def main() -> None:
    base = ROOT / "local-acceptance-models/nxmmr/private-benchmarks"
    results = base / "video-v3-development-20260831"
    registration = json.loads((results / "registration.json").read_text(encoding="utf-8"))
    receipt_path = results / "development.json"
    receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
    split_path = base / "reference-parity-adapter-v1/video-development-reserved-split.json"
    oracle_path = base / "anpr-ocr-vertical-v1/v21-development/development-oracle.csv"

    assert hashlib.sha256(split_path.read_bytes()).hexdigest() == registration["split_file_sha256"]
    assert hashlib.sha256(oracle_path.read_bytes()).hexdigest() == registration["development_oracle_sha256"]
    assert receipt["candidate_id"] == registration["candidate_id"]
    assert receipt["split_digest"] == registration["split_digest"]
    assert receipt["reserved_evaluation_count"] == 1
    assert receipt["consumed_reserved_rows_accessed"] == 0

    split = json.loads(split_path.read_text(encoding="utf-8"))
    with oracle_path.open(encoding="utf-8-sig", newline="") as stream:
        events = list(csv.DictReader(stream))
    assert {event["event_id"] for event in events} == set(split["development_event_ids"])
    allowed = [TimeInterval(**item) for item in split["development_allowed_intervals"]]
    excluded = [TimeInterval(**item) for item in split["reserved_exclusion_intervals"]]

    private = receipt["private_output"]
    frames = private["frames"]
    assert all(any(interval.contains(frame["timestamp_seconds"]) for interval in allowed) for frame in frames)
    assert not any(
        interval.contains(frame["timestamp_seconds"])
        for frame in frames
        for interval in excluded
    )
    observations = [SimpleNamespace(**item) for item in private["observations"]]
    groups = [SimpleNamespace(**item) for item in private["groups"]]
    selected_frames = group_frame_rows(groups, observations, frames)
    score = evaluator.scorer.score_video(events, selected_frames)
    assert score == private["scorer_details"]
    assert receipt["score"] == {
        "events": score["event_summary"],
        "grouping": score["grouping"],
    }
    assert evaluator.stage_metrics(events, frames, groups, score, observations) == receipt["stage_fpr"]
    assert evaluator.utility_checks(score["event_summary"], score["grouping"]) == receipt["utility_gate_checks"]

    by_id = {item.observation_id: item for item in observations}
    assert all(
        group.selected_normalized_plate
        in {by_id[identity].normalized_ocr for identity in group.observation_ids}
        for group in groups
    )

    errors = {
        "truth_occurrences": 0,
        "exact_selected": 0,
        "miss_without_any_exact_raw_observation": 0,
        "miss_with_exact_raw_observation_but_not_selected": 0,
        "human_events_without_any_raw_ocr": 0,
        "human_events_without_any_selected_ocr": 0,
    }
    for event in events:
        raw = {
            evaluator.scorer.normalize_plate(text)
            for frame in frames
            if evaluator.scorer.contains(event, frame["timestamp_seconds"])
            for text in frame["predicted_raw"]
        }
        selected = {
            evaluator.scorer.normalize_plate(text)
            for frame in selected_frames
            if evaluator.scorer.contains(event, frame["timestamp_seconds"])
            for text in frame["predicted_raw"]
        }
        errors["human_events_without_any_raw_ocr"] += int(not raw)
        errors["human_events_without_any_selected_ocr"] += int(not selected)
        for truth in evaluator.scorer.event_truth(event):
            expected = evaluator.scorer.normalize_plate(truth)
            errors["truth_occurrences"] += 1
            if expected in selected:
                errors["exact_selected"] += 1
            elif expected in raw:
                errors["miss_with_exact_raw_observation_but_not_selected"] += 1
            else:
                errors["miss_without_any_exact_raw_observation"] += 1

    result = {
        "contract_version": "nexusai.nxmmr.video-anpr-onnx-demo-development-audit/v1",
        "development_receipt_sha256": hashlib.sha256(receipt_path.read_bytes()).hexdigest(),
        "scorer_recompute_identical": True,
        "stage_recompute_identical": True,
        "registered_source_hashes_unchanged": True,
        "reserved_frames_analyzed": 0,
        "reserved_evaluation_count": 1,
        "consumed_reserved_rows_accessed": 0,
        "raw_observation_count": len(observations),
        "temporal_packet_count": len(groups),
        "error_analysis": errors,
        "score": receipt["score"],
        "stage_fpr": receipt["stage_fpr"],
        "utility_gate_checks": receipt["utility_gate_checks"],
        "numeric_utility_gate": receipt["numeric_utility_gate"],
        "coverage": receipt["coverage"],
        "resource": receipt["resource"],
    }
    output = results / "development-audit.json"
    with output.open("x", encoding="utf-8") as stream:
        json.dump(result, stream, ensure_ascii=False, indent=2)
        stream.write("\n")
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
