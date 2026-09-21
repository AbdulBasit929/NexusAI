#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Recompute the saved V2.2 reserved score without another model run."""
from __future__ import annotations

import csv
import hashlib
import json
import sys
from pathlib import Path
from types import SimpleNamespace

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT), str(ROOT / "scripts")]
import evaluate_nxmmr_video_v2_exact_runtime as common

RESULTS = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/video-v22-reserved-20260831"
DEVELOPMENT = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/raw-color-parity-20260831/comparison.json"
SPLIT = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/reference-parity-adapter-v1/video-development-reserved-split.json"
FREEZE_DIGEST = "b0caae6d646e9d188ec5bc387585ac43aa7f1e22504d5b2d741699bab8d9f394"


def read_signed(path):
    value = json.loads(path.read_text())
    digest = value.pop("receipt_digest")
    assert common.canonical_digest(value) == digest
    return value


def main():
    if (RESULTS / "reserved-audit.json").exists():
        raise RuntimeError("refusing to overwrite reserved audit")
    consumption = read_signed(RESULTS / "consumption.json")
    assert consumption["state"] == "CONSUMED_SCORED"
    assert consumption["reserved_evaluation_count"] == 1
    reserved = read_signed(RESULTS / "reserved.json")
    assert reserved["candidate_freeze_digest"] == FREEZE_DIGEST
    assert reserved["reserved_evaluation_count"] == 1
    assert reserved["post_hoc_tuning"] is False
    assert hashlib.sha256((RESULTS / "reserved.json").read_bytes()).hexdigest() == consumption["reserved_receipt_sha256"]
    projection = read_signed(RESULTS / "reserved-oracle-projection.json")
    assert projection["reserved_event_count"] == 8
    assert hashlib.sha256((RESULTS / "reserved-oracle.csv").read_bytes()).hexdigest() == projection["projection_sha256"]
    with (RESULTS / "reserved-oracle.csv").open(encoding="utf-8-sig", newline="") as stream:
        events = list(csv.DictReader(stream))
    split = json.loads(SPLIT.read_text())
    split_digest = split.pop("split_digest")
    assert common.canonical_digest(split) == split_digest == projection["split_digest"]
    assert {event["event_id"] for event in events} == set(split["reserved_event_ids"])
    prohibited = [common.TimeInterval(**row) for row in split["development_allowed_intervals"]]
    frames = reserved["private_output"]["frames"]
    assert not any(interval.contains(frame["timestamp_seconds"]) for frame in frames for interval in prohibited)
    observations = [SimpleNamespace(**row) for row in reserved["private_output"]["observations"]]
    groups = [SimpleNamespace(**row) for row in reserved["private_output"]["groups"]]
    selected = common.group_frame_rows(groups, observations, frames)
    score = common.scorer.score_video(events, selected)
    assert score == reserved["private_output"]["scorer_details"]
    assert {"events": score["event_summary"], "grouping": score["grouping"]} == reserved["score"]
    stage = common.stage_metrics(events, frames, groups, score, observations)
    assert stage == reserved["stage_fpr"]
    host = read_signed(RESULTS / "reserved-host-resources.json")
    process = json.loads((RESULTS / "reserved-process-resources.json").read_text())
    assert host["container_exit_code"] == 0 and not host["safety_stop"]
    assert process["state"] == "PASS"
    development = json.loads(DEVELOPMENT.read_text())["arms"]["RAW_RGB"]
    dev_score, res_score = development["score"], reserved["score"]
    def metrics(value, stage_value):
        events_value, groups_value = value["events"], value["grouping"]
        exact = events_value["normalized_exact_plate"]
        return {
            "event_recall": events_value["event_detection_recall"],
            "exact_precision": exact["precision"], "exact_recall": exact["recall"],
            "exact_f1": exact["f1"], "cer": events_value["normalized_cer"],
            "group_precision": groups_value["exact_group_precision"],
            "group_recall": groups_value["exact_group_recall"],
            "group_f1": groups_value["exact_group_f1"],
            "final_false_groups": stage_value["FinalFalseGroupCount"],
            "RawDetectorNegativeFrameFPR": stage_value["RawDetectorNegativeFrameFPR"],
            "OCRCandidateNegativeFrameFPR": stage_value["OCRCandidateNegativeFrameFPR"],
            "GroupSelectedNegativeFrameFPR": stage_value["GroupSelectedNegativeFrameFPR"],
            "boundary_mae": groups_value["source_time_error_seconds"]["mean_absolute_boundary_error"],
        }
    dev_metrics = metrics(dev_score, development["stage_fpr"])
    reserved_metrics = metrics(res_score, reserved["stage_fpr"])
    delta = {key: (None if dev_metrics[key] is None or reserved_metrics[key] is None
                   else round(reserved_metrics[key] - dev_metrics[key], 6)) for key in dev_metrics}
    truth_occurrences = sum(len(common.scorer.event_truth(event)) for event in events)
    truth_strings = {common.scorer.normalize_plate(text) for event in events
                     for text in common.scorer.event_truth(event)}
    gates = common.utility_checks(res_score["events"], res_score["grouping"])
    drops = [-(delta[key] or 0) for key in ("event_recall", "exact_f1", "group_f1")]
    generalization = ("MATERIAL_DEGRADATION" if not all(gates.values()) else
                      "STABLE_WITHIN_SMALL_SAMPLE" if max(drops) <= .1 else
                      "MODERATE_DEGRADATION")
    result = {
        "contract_version": "nexusai.nxmmr.video-v22-reserved-audit/v1",
        "candidate_freeze_digest": FREEZE_DIGEST,
        "reserved_evaluation_count": 1,
        "reserved_population": {"events": len(events), "truth_plate_occurrences": truth_occurrences,
                                "truth_group_strings": len(truth_strings)},
        "scorer_recomputation": "PASS", "stage_fpr_recomputation": "PASS",
        "partition_isolation": "PASS", "resource_receipts": "PASS",
        "development_metrics": dev_metrics, "reserved_metrics": reserved_metrics,
        "reserved_minus_development": delta, "utility_gate_checks": gates,
        "numeric_utility_gate": "PASS" if all(gates.values()) else "FAIL",
        "generalization_assessment": generalization,
        "strong_generalization_claim_supported": False,
        "strong_generalization_limitation": "Eight events from the same single source video are insufficient for a universal claim.",
        "source_time_review_required": True,
        "raw_observations": len(observations), "temporal_packets": len(groups),
        "post_hoc_tuning": False, "repeat_reserved_evaluation_permitted": False,
        "private_event_ids": sorted(event["event_id"] for event in events),
    }
    common.write_new(RESULTS / "reserved-audit.json", result)
    print(json.dumps({key: value for key, value in result.items() if key != "private_event_ids"}, indent=2))


if __name__ == "__main__":
    main()
