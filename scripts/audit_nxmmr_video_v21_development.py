# SPDX-License-Identifier: MIT
"""Score-consistency/error audit of completed development receipts; no inference."""
import csv
import hashlib
import json
import sys
from pathlib import Path
from types import SimpleNamespace

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT), str(ROOT / "scripts")]
import evaluate_nxmmr_video_v2_exact_runtime as evaluator
from ingestion.forensic_records.video_anpr_parity_adapter import TimeInterval, canonical_digest
from benchmark_nexusai_nxmmr_parity_adapter import group_frame_rows


def main():
    base = ROOT / "local-acceptance-models/nxmmr/private-benchmarks"
    results = base / "anpr-ocr-vertical-v1/v21-development"
    receipt = json.loads((results / "development.json").read_text())
    digest = receipt.pop("receipt_digest")
    assert canonical_digest(receipt) == digest
    assert receipt["candidate_revision"] == "v21" and receipt["reserved_evaluation_count"] == 0
    split = json.loads((base / "reference-parity-adapter-v1/video-development-reserved-split.json").read_text())
    with (results / "development-oracle.csv").open(encoding="utf-8-sig", newline="") as stream:
        events = list(csv.DictReader(stream))
    assert {event["event_id"] for event in events} == set(split["development_event_ids"])
    allowed = [TimeInterval(**item) for item in split["development_allowed_intervals"]]
    excluded = [TimeInterval(**item) for item in split["reserved_exclusion_intervals"]]
    data = receipt["private_output"]
    frames = data["frames"]
    assert all(any(interval.contains(frame["timestamp_seconds"]) for interval in allowed) for frame in frames)
    assert not any(interval.contains(frame["timestamp_seconds"]) for frame in frames for interval in excluded)
    observations = [SimpleNamespace(**item) for item in data["observations"]]
    groups = [SimpleNamespace(**item) for item in data["groups"]]
    selected_frames = group_frame_rows(groups, observations, frames)
    score = evaluator.scorer.score_video(events, selected_frames)
    assert score == data["scorer_details"]
    assert receipt["score"] == {"events": score["event_summary"], "grouping": score["grouping"]}
    assert evaluator.stage_metrics(events, frames, groups, score, observations) == receipt["stage_fpr"]
    assert all(item.preprocessing == "GRAY_BINARY_INV_64_REPLICATED_RGB" for item in observations)
    by_id = {item.observation_id: item for item in observations}
    assert all(group.selected_normalized_plate in {by_id[i].normalized_ocr for i in group.observation_ids} for group in groups)

    errors = {"truth_occurrences": 0, "exact_selected": 0,
              "miss_without_any_exact_raw_observation": 0,
              "miss_with_exact_raw_observation_but_not_selected": 0,
              "human_events_without_any_raw_ocr": 0,
              "human_events_without_any_selected_ocr": 0}
    for event in events:
        raw = {evaluator.scorer.normalize_plate(text) for frame in frames
               if evaluator.scorer.contains(event, frame["timestamp_seconds"]) for text in frame["predicted_raw"]}
        selected = {evaluator.scorer.normalize_plate(text) for frame in selected_frames
                    if evaluator.scorer.contains(event, frame["timestamp_seconds"]) for text in frame["predicted_raw"]}
        errors["human_events_without_any_raw_ocr"] += not raw
        errors["human_events_without_any_selected_ocr"] += not selected
        for truth in evaluator.scorer.event_truth(event):
            expected = evaluator.scorer.normalize_plate(truth)
            errors["truth_occurrences"] += 1
            if expected in selected:
                errors["exact_selected"] += 1
            elif expected in raw:
                errors["miss_with_exact_raw_observation_but_not_selected"] += 1
            else:
                errors["miss_without_any_exact_raw_observation"] += 1

    prior = json.loads((base / "anpr-ocr-vertical-v1/video-v2-fastplate-development.json").read_text())
    historical = json.loads((base / "reference-parity-adapter-v1/development-baselines.json").read_text())
    assert prior["development_split_digest"] == historical["development_split_digest"] == receipt["split_digest"]
    host = json.loads((results / "development-host-resources.json").read_text())
    process = json.loads((results / "development-process-resources.json").read_text())
    assert not host["safety_stop"] and host["container_exit_code"] == 0 and process["state"] == "PASS"
    assert not list(results.glob("reserved*"))
    result = {
        "candidate": "v21", "development_receipt_sha256": hashlib.sha256((results / "development.json").read_bytes()).hexdigest(),
        "scorer_recompute_identical": True, "stage_recompute_identical": True,
        "reserved_frames_analyzed": 0, "reserved_evaluation_count": 0,
        "raw_observation_count": len(observations), "temporal_packet_count": len(groups),
        "error_analysis": errors, "score": receipt["score"], "stage_fpr": receipt["stage_fpr"],
        "event_detection": receipt["event_detection"], "utility_gate_checks": receipt["utility_gate_checks"],
        "numeric_utility_gate": receipt["numeric_utility_gate"], "coverage": receipt["coverage"],
        "resources": receipt["resources"], "host_resources": host, "process_finalization": process,
        "empirical_incremental_peak_plus_1p5_gib": receipt["resources"]["incremental_tree_peak_mib"] / 1024 + 1.5,
        "policy_minimum_free_ram_gib_unchanged": 3.0,
        "comparators": {"v1": prior["v1_comparator"]["score"],
                        "raw_bgr_v2": prior["selected"]["score"],
                        "historical_final": historical["historical_final_reference"]["score"]},
    }
    evaluator.write_new(results / "development-audit.json", result)
    print(json.dumps({key: value for key, value in result.items() if key != "comparators"}, indent=2))


if __name__ == "__main__":
    main()
