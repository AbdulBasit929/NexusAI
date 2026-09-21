#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""One-time fixed V2.2 RAW_RGB evaluator for the locked reserved partition."""
from __future__ import annotations

import argparse
import csv
import hashlib
import importlib.metadata as metadata
import json
import os
import platform
import sys
import time
from dataclasses import asdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT), str(ROOT / "scripts")]
import evaluate_nxmmr_video_v2_exact_runtime as common
from ingestion.forensic_records.video_anpr_product_v22 import (
    VideoANPRProductV22Processor, color_contract,
)
from nxmmr_evaluator_resources import finalized_resources

FREEZE_DIGEST = "b0caae6d646e9d188ec5bc387585ac43aa7f1e22504d5b2d741699bab8d9f394"
MODE = "RAW_RGB"


def read_signed(path):
    value = json.loads(path.read_text())
    digest = value.pop("receipt_digest")
    assert common.canonical_digest(value) == digest
    return value


def verify(args):
    registration = read_signed(args.registration)
    assert registration["candidate_freeze_digest"] == FREEZE_DIGEST
    assert registration["selected_mode"] == MODE
    assert registration["reserved_evaluation_authorized"] is True
    assert registration["reserved_evaluation_limit"] == 1
    for name, digest in registration["harness_source_files"].items():
        assert common.hash_file(ROOT / name) == digest, name
    freeze_path = ROOT / registration["candidate_freeze_file"]
    assert common.hash_file(freeze_path) == registration["candidate_freeze_file_sha256"]
    freeze = json.loads(freeze_path.read_text())
    digest = freeze.pop("freeze_digest")
    assert digest == FREEZE_DIGEST == common.canonical_digest(freeze)
    assert freeze["selection_decision"] == "RAW_RGB_SELECTED"
    assert freeze["selected_ocr_input_mode"] == MODE
    assert freeze["reserved_evaluation_count"] == 0
    for name, expected in freeze["source_files"].items():
        assert common.hash_file(ROOT / name) == expected, name
    config_path = ROOT / "configuration/nxmmr_video_anpr_product_baseline_v22.json"
    assert common.hash_file(config_path) == freeze["configuration_sha256"]
    config = json.loads(config_path.read_text())
    assert config["ocr_input"]["selected_mode"] == MODE
    assert config["ocr_input"]["ocr_input_contract"] == "PASS"
    assert config["live_activation"] is False
    assert asdict(common.SELECTED_POLICY) == freeze["selected_policy"]
    assert common.hash_file(Path("/evaluator/resolved-runtime-lock.json")) == freeze["runtime"]["top_level_lock_sha256"]
    assert common.hash_file(sys.executable) == freeze["runtime"]["interpreter_sha256"]
    assert sys.version_info[:3] == (3, 11, 15)
    assert platform.system() == "Linux" and platform.machine() == "x86_64"
    packages = {name: metadata.version(name) for name in {
        dist.metadata["Name"].lower().replace("_", "-") for dist in metadata.distributions()}}
    assert packages == freeze["runtime"]["effective_distributions"]
    assert len(packages) == freeze["runtime"]["effective_distribution_count"] == 77
    for env, key in (
        ("FORENSIC_VIDEO_ANPR_PLATE_DETECTOR_PATH", "plate_detector_sha256"),
        ("FORENSIC_VIDEO_ANPR_VEHICLE_DETECTOR_PATH", "vehicle_detector_sha256"),
        ("FORENSIC_ANPR_OCR_MODEL_PATH", "ocr_model_sha256"),
        ("FORENSIC_ANPR_OCR_CONFIG_PATH", "ocr_config_sha256"),
    ):
        assert common.hash_file(os.environ[env]) == freeze["models"][key], env
    split = json.loads(args.split.read_text())
    split_digest = split.pop("split_digest")
    assert common.canonical_digest(split) == split_digest == freeze["development_split_digest"]
    assert common.hash_file(args.video) == split["source_video_sha256"]
    projection = read_signed(args.projection_receipt)
    assert projection["candidate_freeze_digest"] == FREEZE_DIGEST
    assert projection["split_digest"] == split_digest
    assert projection["reserved_event_count"] == split["reserved_event_count"] == 8
    assert common.hash_file(args.oracle) == projection["projection_sha256"]
    start = read_signed(args.consumption_start)
    assert start["authorization_consumed"] is True
    assert start["reserved_evaluation_count"] == 1
    assert start["candidate_freeze_digest"] == FREEZE_DIGEST
    return registration, freeze, split, split_digest, packages


def evaluate(args, freeze, split, split_digest, processor, result, cv2):
    with args.oracle.open(encoding="utf-8-sig", newline="") as stream:
        events = list(csv.DictReader(stream))
    assert len(events) == 8
    assert {event["event_id"] for event in events} == set(split["reserved_event_ids"])
    assert all(event["event_type"] == "plate" and event["label_source"] == "independent_human_annotation" for event in events)
    allowed = [common.TimeInterval(**item) for item in split["reserved_exclusion_intervals"]]
    prohibited = [common.TimeInterval(**item) for item in split["development_allowed_intervals"]]
    capture = cv2.VideoCapture(str(args.video))
    assert capture.isOpened()
    try:
        fps = float(capture.get(cv2.CAP_PROP_FPS))
        count = int(capture.get(cv2.CAP_PROP_FRAME_COUNT))
        duration = count / fps
        assert 0 < duration <= processor.maximum_duration_seconds
        policy = common.FramePolicy("fixed-4fps-fastplate-vehicle-v2", 4.0,
                                    maximum_analyzed_frames=processor.maximum_frames)
        timestamps = common.schedule_base_frames(duration, fps, policy, allowed_intervals=allowed)
        assert timestamps
        assert not any(interval.contains(round(t * fps) / fps) for t in timestamps for interval in prohibited)
        frames, observations, crops_total, failed = [], [], 0, 0
        for index, timestamp in enumerate(timestamps):
            if (args.output.parent / "HOST_RAM_STOP").exists():
                raise RuntimeError("HOST_RAM_SAFETY_STOP")
            number = round(timestamp * fps)
            capture.set(cv2.CAP_PROP_POS_FRAMES, number)
            ok, frame = capture.read()
            if not ok:
                failed += 1
                continue
            encoded, data = cv2.imencode(".png", frame)
            frame_hash = hashlib.sha256(bytes(data)).hexdigest() if encoded else ""
            frame_started = time.perf_counter()
            observed, crops, detections = processor.frame_processor.process_frame(
                frame, source_sha256=split["source_video_sha256"], source_file="sample.mp4",
                frame_number=number, timestamp_seconds=number / fps,
                source_frame_sha256=frame_hash)
            observations.extend(observed)
            crops_total += crops
            frames.append({"frame_number": number, "timestamp_seconds": number / fps,
                "source_frame_sha256": frame_hash, "detections": detections,
                "ocr_crops": crops, "predicted_raw": [item.raw_ocr for item in observed],
                "latency_seconds": time.perf_counter() - frame_started,
                "upstream_trace": processor.frame_processor.last_frame_trace})
            if index % 20 == 0:
                print(json.dumps({"phase": "reserved", "progress_frames": index + 1,
                                  "scheduled": len(timestamps)}), flush=True)
    finally:
        capture.release()
    groups = common.aggregate_product_observations(observations, common.SELECTED_POLICY)
    score = common.scorer.score_video(events, common.group_frame_rows(groups, observations, frames))
    by_id = {item.observation_id: item for item in observations}
    assert all(group.selected_normalized_plate in {
        by_id[identity].normalized_ocr for identity in group.observation_ids} for group in groups)
    utility = common.utility_checks(score["event_summary"], score["grouping"])
    result.update(
        split_digest=split_digest,
        oracle_projection_sha256=common.hash_file(args.oracle),
        policy=asdict(common.SELECTED_POLICY),
        frame_policy=asdict(policy),
        score={"events": score["event_summary"], "grouping": score["grouping"]},
        stage_fpr=common.stage_metrics(events, frames, groups, score, observations),
        utility_gate_checks=utility,
        numeric_utility_gate="PASS" if all(utility.values()) else "FAIL",
        source_time_review_required=True,
        coverage={"source_fps": fps, "source_duration_seconds": duration,
            "source_frame_count": count, "frames_analyzed": len(frames),
            "frames_scheduled": len(timestamps), "ocr_calls": crops_total,
            "failed_frames": failed, "partial_sampling": len(frames) < count,
            "effective_frame_coverage": common.scorer.safe_div(len(frames), count)},
        private_output={"frames": frames, "observations": [asdict(item) for item in observations],
                        "groups": [asdict(item) for item in groups], "scorer_details": score})


def run(args):
    if args.output.exists():
        raise RuntimeError("reserved output already exists; repeat forbidden")
    monitor = common.ResourceMonitor()
    started, cpu_started = time.perf_counter(), time.process_time()
    with finalized_resources(args.output.with_name("reserved-process-resources.json"),
                             monitor, phase="reserved", started=started,
                             cpu_started=cpu_started):
        registration, freeze, split, split_digest, packages = verify(args)
        import torch
        import cv2
        assert torch.version.cuda is None and not torch.cuda.is_available()
        processor = VideoANPRProductV22Processor.from_environment(ocr_input_mode=MODE)
        result = {
            "contract_version": "nexusai.nxmmr.video-v22-reserved-evaluation/v1",
            "phase": "reserved", "state": "PASS", "candidate_freeze_digest": FREEZE_DIGEST,
            "selected_mode": MODE, "input_contract": color_contract(MODE),
            "registration_sha256": common.hash_file(args.registration),
            "runtime_packages": packages, "interpreter_sha256": common.hash_file(sys.executable),
            "reserved_evaluation_count": 1, "post_hoc_tuning": False,
            "model_downloads": 0, "runtime_mutated": False,
            "retained_state_mutated": False, "activity_mutated": False,
            "database_migration": False, "volumes_changed": False,
            "deployment_performed": False, "image_holdout_accessed": False,
        }
        evaluate(args, freeze, split, split_digest, processor, result, cv2)
        result.update(wall_seconds=time.perf_counter() - started,
                      cpu_seconds=time.process_time() - cpu_started,
                      resources=monitor.finish())
        common.write_new(args.output, result)
        print(json.dumps({key: value for key, value in result.items()
                          if key not in ("private_output", "runtime_packages")}), flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("output", "registration", "oracle", "projection-receipt",
                 "consumption-start", "split", "video"):
        parser.add_argument("--" + name, dest=name.replace("-", "_"), type=Path, required=True)
    run(parser.parse_args())
