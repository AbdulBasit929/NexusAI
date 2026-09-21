# SPDX-License-Identifier: MIT
"""Evaluate the preregistered ONNX Video V3 candidate on development rows only."""

from __future__ import annotations

import argparse
import csv
import json
import resource
import sys
import time
from dataclasses import asdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT), str(ROOT / "scripts")]

import evaluate_nxmmr_video_v2_exact_runtime as common
from ingestion.forensic_records.video_anpr_onnx_v3 import (
    ADAPTER_ID,
    ADAPTER_VERSION,
    FRAME_POLICY_ID,
    VideoANPROnnxV3Processor,
)


def main(args: argparse.Namespace) -> None:
    if args.output.exists():
        raise RuntimeError("refusing to overwrite or repeat the Video V3 development result")
    registration = json.loads(args.registration.read_text(encoding="utf-8"))
    if registration["candidate_id"] != ADAPTER_ID or registration["reserved_evidence_permitted"]:
        raise RuntimeError("Video V3 registration does not authorize this development-only run")
    if common.hash_file(args.candidate) != registration["candidate_configuration_sha256"]:
        raise RuntimeError("Video V3 candidate configuration differs from preregistration")
    if common.hash_file(ROOT / "ingestion/forensic_records/video_anpr_onnx_v3.py") != registration["processor_sha256"]:
        raise RuntimeError("Video V3 processor differs from preregistration")
    split = json.loads(args.split.read_text(encoding="utf-8"))
    split_digest = split.pop("split_digest")
    if common.canonical_digest(split) != split_digest or split_digest != registration["split_digest"]:
        raise RuntimeError("development split integrity failed")
    if common.hash_file(args.oracle) != registration["development_oracle_sha256"]:
        raise RuntimeError("development oracle projection integrity failed")
    if common.hash_file(args.video) != split["source_video_sha256"]:
        raise RuntimeError("sample.mp4 integrity failed")

    with args.oracle.open(encoding="utf-8-sig", newline="") as stream:
        events = list(csv.DictReader(stream))
    if {event["event_id"] for event in events} != set(split["development_event_ids"]):
        raise RuntimeError("development oracle contains rows outside the registered development split")
    if any(event["label_source"] != "independent_human_annotation" for event in events):
        raise RuntimeError("model output cannot serve as the development oracle")
    allowed = [common.TimeInterval(**item) for item in split["development_allowed_intervals"]]
    excluded = [common.TimeInterval(**item) for item in split["reserved_exclusion_intervals"]]

    processor = VideoANPROnnxV3Processor.from_environment()
    started, cpu_started = time.perf_counter(), time.process_time()
    run = processor.run_video(
        args.video,
        context={
            "evidence_id": "private-nxmmr-video-v3-development",
            "version_id": "development-projection-v1",
            "source_sha256": split["source_video_sha256"],
            "source_file": "sample.mp4",
        },
        allowed_intervals=allowed,
    )
    if any(interval.contains(frame.timestamp_seconds) for frame in run.frames for interval in excluded):
        raise RuntimeError("reserved-exclusion timestamp reached the Video V3 development run")

    observed_by_frame: dict[int, list[str]] = {}
    for item in run.observations:
        observed_by_frame.setdefault(item.frame_number, []).append(item.raw_ocr)
    frames = [
        {
            "frame_number": frame.frame_number,
            "timestamp_seconds": frame.timestamp_seconds,
            "source_frame_sha256": frame.source_frame_sha256,
            "detections": frame.raw_detection_count,
            "ocr_crops": frame.raw_detection_count,
            "predicted_raw": observed_by_frame.get(frame.frame_number, []),
            "latency_seconds": frame.latency_seconds,
        }
        for frame in run.frames
    ]
    score = common.scorer.score_video(events, common.group_frame_rows(run.groups, run.observations, frames))
    stage_fpr = common.stage_metrics(events, frames, run.groups, score, run.observations)
    utility = common.utility_checks(score["event_summary"], score["grouping"])
    selected_ids = {identity for group in run.groups for identity in group.observation_ids}
    if any(
        group.selected_normalized_plate
        not in {item.normalized_ocr for item in run.observations if item.observation_id in group.observation_ids}
        for group in run.groups
    ):
        raise RuntimeError("a selected group candidate was not actually observed")
    usage = resource.getrusage(resource.RUSAGE_SELF)
    report = {
        "contract_version": "nexusai.nxmmr.video-anpr-onnx-demo-development/v3",
        "candidate_id": ADAPTER_ID,
        "candidate_version": ADAPTER_VERSION,
        "authority": "DEVELOPMENT_ONLY",
        "reserved_evaluation_count": 1,
        "consumed_reserved_rows_accessed": 0,
        "configuration_tuned_after_result": False,
        "split_digest": split_digest,
        "development_oracle_sha256": registration["development_oracle_sha256"],
        "frame_policy": FRAME_POLICY_ID,
        "score": {"events": score["event_summary"], "grouping": score["grouping"]},
        "stage_fpr": stage_fpr,
        "utility_gate_checks": utility,
        "numeric_utility_gate": "PASS" if all(utility.values()) else "FAIL",
        "coverage": {
            **run.result.metadata["coverage"],
            "frames_scheduled": len(run.frames),
            "raw_detector_positive_frames": sum(frame.raw_detection_count > 0 for frame in run.frames),
            "ocr_candidate_positive_frames": sum(frame.ocr_candidate_count > 0 for frame in run.frames),
            "group_selected_positive_frames": sum(frame.selected_group_observation_count > 0 for frame in run.frames),
        },
        "resource": {
            "wall_seconds": time.perf_counter() - started,
            "cpu_seconds": time.process_time() - cpu_started,
            "peak_rss_mib": usage.ru_maxrss / 1024,
        },
        "semantics": {
            "vehicle_detector": False,
            "ultralytics_used": False,
            "persistent_tracking": False,
            "interpolation": False,
            "selected_candidate_observed": len(selected_ids) >= 0,
            "manual_review_required": True,
        },
        "private_output": {
            "frames": frames,
            "observations": [asdict(item) for item in run.observations],
            "groups": [asdict(item) for item in run.groups],
            "scorer_details": score,
        },
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({key: value for key, value in report.items() if key != "private_output"}, ensure_ascii=False))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("candidate", "registration", "split", "oracle", "video", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    main(parser.parse_args())
