# SPDX-License-Identifier: MIT
"""Compare saved development arms without inference, tuning or reserved access."""
import csv
import hashlib
import json
import sys
from pathlib import Path
from types import SimpleNamespace

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT), str(ROOT / "scripts")]
import evaluate_nxmmr_video_v2_exact_runtime as common

RESULTS = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/raw-color-parity-20260831"
MODES = ("RAW_BGR_PARITY", "RAW_RGB")


def read_signed(path):
    value = json.loads(path.read_text())
    digest = value.pop("receipt_digest")
    assert common.canonical_digest(value) == digest
    return value


def category(a, b):
    return "both" if a and b else "bgr_only" if a else "rgb_only" if b else "neither"


def main():
    registration = read_signed(RESULTS / "registration.json")
    source = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/v21-development/development-oracle.csv"
    assert hashlib.sha256(source.read_bytes()).hexdigest() == registration["development_oracle_sha256"]
    with source.open(encoding="utf-8-sig", newline="") as stream:
        events = list(csv.DictReader(stream))
    split = json.loads((ROOT / "local-acceptance-models/nxmmr/private-benchmarks/reference-parity-adapter-v1/video-development-reserved-split.json").read_text())
    split_digest = split.pop("split_digest")
    assert common.canonical_digest(split) == split_digest == registration["split_digest"]
    assert {row["event_id"] for row in events} == set(split["development_event_ids"])
    excluded = [common.TimeInterval(**row) for row in split["reserved_exclusion_intervals"]]
    receipts, rows_by_mode, summaries = {}, {}, {}
    for mode in MODES:
        value = read_signed(RESULTS / mode / "development.json")
        assert value["mode"] == mode and value["reserved_evaluation_count"] == 0
        frames = value["private_output"]["frames"]
        assert not any(interval.contains(frame["timestamp_seconds"]) for frame in frames for interval in excluded)
        observations = [SimpleNamespace(**row) for row in value["private_output"]["observations"]]
        groups = [SimpleNamespace(**row) for row in value["private_output"]["groups"]]
        selected = common.group_frame_rows(groups, observations, frames)
        score = common.scorer.score_video(events, selected)
        assert score == value["private_output"]["scorer_details"]
        assert {"events": score["event_summary"], "grouping": score["grouping"]} == value["score"]
        assert common.stage_metrics(events, frames, groups, score, observations) == value["stage_fpr"]
        host = json.loads((RESULTS / mode / "development-host-resources.json").read_text())
        process = json.loads((RESULTS / mode / "development-process-resources.json").read_text())
        assert host["container_exit_code"] == 0 and not host["safety_stop"] and process["state"] == "PASS"
        receipts[mode] = value
        rows_by_mode[mode] = (frames, selected)
        summaries[mode] = {key: value[key] for key in ("score", "stage_fpr", "coverage", "wall_seconds", "cpu_seconds", "resources", "numeric_utility_gate", "utility_gate_checks", "input_contract")}
        summaries[mode].update(host_resources=host, process_resources=process,
            raw_observations=len(observations), temporal_packets=len(groups),
            incremental_peak_plus_headroom_gib=value["resources"]["incremental_tree_peak_mib"] / 1024 + 1.5,
            scorer_consistency="PASS", stage_consistency="PASS",
            development_receipt_sha256=hashlib.sha256((RESULTS / mode / "development.json").read_bytes()).hexdigest())
    upstream = lambda mode: [{key: frame[key] for key in (
        "frame_number", "timestamp_seconds", "source_frame_sha256", "detections", "ocr_crops", "upstream_trace")}
        for frame in rows_by_mode[mode][0]]
    parity = upstream(MODES[0]) == upstream(MODES[1])
    assert parity, "Upstream detector/crop mismatch: do not attribute differences to OCR or select a winner"
    counters = lambda: dict(both=0, bgr_only=0, rgb_only=0, neither=0)
    event_differential, occurrence_differential = counters(), counters()
    errors = {mode: {"exact_selected": 0, "exact_never_generated": 0, "exact_generated_but_not_retained": 0,
                     "events_without_any_raw_ocr": 0, "events_without_any_selected_ocr": 0} for mode in MODES}
    private_events = []
    for event in events:
        truth = [common.scorer.normalize_plate(value) for value in common.scorer.event_truth(event)]
        raw, selected = {}, {}
        for mode in MODES:
            all_frames, selected_frames = rows_by_mode[mode]
            values = lambda frames: {common.scorer.normalize_plate(value) for frame in frames
                if common.scorer.contains(event, frame["timestamp_seconds"]) for value in frame["predicted_raw"]}
            raw[mode], selected[mode] = values(all_frames), values(selected_frames)
            errors[mode]["events_without_any_raw_ocr"] += not raw[mode]
            errors[mode]["events_without_any_selected_ocr"] += not selected[mode]
            for text in truth:
                field = "exact_selected" if text in selected[mode] else "exact_generated_but_not_retained" if text in raw[mode] else "exact_never_generated"
                errors[mode][field] += 1
        event_class = category(bool(set(truth) & raw[MODES[0]]), bool(set(truth) & raw[MODES[1]]))
        event_differential[event_class] += 1
        for text in truth:
            occurrence_differential[category(text in raw[MODES[0]], text in raw[MODES[1]])] += 1
        private_events.append({"event_id": event["event_id"], "raw_exact_class": event_class,
            "selected_exact_counts": {mode: sum(text in selected[mode] for text in truth) for mode in MODES}})
    a, b = (receipts[mode]["score"]["events"]["normalized_exact_plate"] for mode in MODES)
    material = registration["bgr_materiality_convention"]
    winning_events = sum(row["selected_exact_counts"][MODES[0]] > row["selected_exact_counts"][MODES[1]] for row in private_events)
    material_checks = {"exact_f1_delta": a["f1"] is not None and b["f1"] is not None and a["f1"] - b["f1"] >= material["minimum_exact_f1_advantage"],
        "additional_exact_tp": a["tp"] - b["tp"] >= material["minimum_additional_exact_tp"],
        "distributed_event_advantage": winning_events >= material["minimum_events_with_exact_tp_advantage"],
        "upstream_parity": parity, "deterministic_score_recomputation": True}
    result = {"contract_version": "nexusai.nxmmr.raw-color-comparison/v1",
        "registration_sha256": hashlib.sha256((RESULTS / "registration.json").read_bytes()).hexdigest(),
        "raw_detector_parity": "PASS", "source_frame_parity": "PASS", "containment_parity": "PASS", "source_crop_parity": "PASS",
        "upstream_trace_digest": common.canonical_digest(upstream(MODES[0])),
        "arms": summaries, "event_raw_exact_differential": event_differential,
        "truth_occurrence_raw_exact_differential": occurrence_differential,
        "error_decomposition": errors, "bgr_materiality_checks": material_checks,
        "bgr_material_advantage": all(material_checks.values()), "bgr_advantage_event_count": winning_events,
        "reserved_evaluation_count": 0, "reserved_frames_analyzed": 0,
        "private_event_differential": private_events}
    common.write_new(RESULTS / "comparison.json", result)
    print(json.dumps({key: value for key, value in result.items() if key != "private_event_differential"}, indent=2))


if __name__ == "__main__":
    main()
