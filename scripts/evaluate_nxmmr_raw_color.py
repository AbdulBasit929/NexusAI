# SPDX-License-Identifier: MIT
"""Exactly two fixed raw-color arms, development only, using the product path."""
import argparse
import csv
import hashlib
import importlib.metadata as metadata
import json
import sys
import time
from dataclasses import asdict
from pathlib import Path
from types import SimpleNamespace

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT), str(ROOT / "scripts")]
import evaluate_nxmmr_video_v2_exact_runtime as common
from nxmmr_evaluator_resources import finalized_resources
from ingestion.forensic_records.video_anpr_product_v22 import (
    VideoANPRProductV22Processor, INPUT_MODES, color_contract,
)
from ingestion.forensic_records.video_anpr_parity_adapter import PlateBounds


def run(args):
    if args.phase not in ("imports", "smoke", "development"):
        raise RuntimeError("Only imports, synthetic smoke and development are authorized")
    if args.output.exists():
        raise RuntimeError("refusing repeated phase")
    monitor = common.ResourceMonitor()
    started, cpu_started = time.perf_counter(), time.process_time()
    with finalized_resources(args.output.with_name(args.phase + "-process-resources.json"),
            monitor, phase=args.phase, started=started, cpu_started=cpu_started):
        # Reuse only the prior full runtime/hash verifier, never V2.1 inference.
        pins = common.verify(SimpleNamespace(
            candidate="v21", phase=args.phase, evaluation_manifest=args.registration,
            freeze=ROOT / "local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/video-v2-product-source-freeze.json",
            manifest=ROOT / "configuration/nxmmr_anpr_ocr_vertical_activation_v1.json"))
        registration = json.loads(args.registration.read_text())
        assert not registration["reserved_evaluation_authorized"]
        assert tuple(registration["arms"]) == INPUT_MODES
        import torch
        import torchvision
        import ultralytics
        import numpy as np
        import cv2
        import fast_alpr
        import fast_plate_ocr
        import open_image_models
        assert torch.version.cuda is None and not torch.cuda.is_available()
        packages = {name: metadata.version(name) for name in {
            d.metadata["Name"].lower().replace("_", "-") for d in metadata.distributions()}}
        prior = json.loads((ROOT / "local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/exact-runtime/imports.json").read_text())
        assert packages == prior["packages"]
        assert common.hash_file(sys.executable) == prior["interpreter_sha256"]
        result = {"phase": args.phase, "mode": args.mode, "state": "PASS",
            "runtime_core": pins, "runtime_packages": packages,
            "interpreter_sha256": prior["interpreter_sha256"],
            "registration_sha256": common.hash_file(args.registration),
            "reserved_evaluation_count": 0, "post_hoc_tuning": False,
            "model_downloads": 0, "runtime_mutated": False, "retained_state_mutated": False}
        if args.phase != "imports":
            result["input_contract"] = color_contract(args.mode)
            processor = VideoANPRProductV22Processor.from_environment(ocr_input_mode=args.mode)
            if args.phase == "smoke":
                from unittest.mock import patch
                frame = np.zeros((640, 640, 3), np.uint8)
                context = dict(source_sha256="0" * 64, source_file="synthetic-only",
                    frame_number=0, timestamp_seconds=0.0, source_frame_sha256="0" * 64)
                processor.frame_processor.process_frame(frame, **context)
                frame[:64, :128] = [17, 93, 211]
                with patch.object(processor.frame_processor.plate_detector, "detect", return_value=[(PlateBounds(0, 0, 128, 64), 1.0, 0)]), patch.object(processor.frame_processor.vehicle_detector, "detect", return_value=[(PlateBounds(0, 0, 640, 640), 1.0, 2)]):
                    _, crops, _ = processor.frame_processor.process_frame(frame, **context)
                assert crops == 1
                contract = processor.frame_processor.last_input_contract
                assert contract["shape"] == [64, 128, 3] and contract["dtype"] == "uint8"
                result.update(synthetic_only=True, detectors_loaded=2, ocr_model_loaded=True,
                    forced_synthetic_crops=crops, ocr_input_contract=contract,
                    model_hashes=json.loads((ROOT / "local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/video-v2-product-source-freeze.json").read_text())["models"])
            else:
                evaluate_development(args, registration, processor, result, cv2)
        result.update(wall_seconds=time.perf_counter() - started,
                      cpu_seconds=time.process_time() - cpu_started, resources=monitor.finish())
        common.write_new(args.output, result)
        print(json.dumps({key: value for key, value in result.items()
                          if key not in ("private_output", "runtime_packages")}), flush=True)


def evaluate_development(args, registration, processor, result, cv2):
    assert common.hash_file(args.oracle) == registration["development_oracle_sha256"]
    split = json.loads(args.split.read_text())
    digest = split.pop("split_digest")
    assert common.canonical_digest(split) == digest == registration["split_digest"]
    assert common.hash_file(args.video) == split["source_video_sha256"]
    with args.oracle.open(encoding="utf-8-sig", newline="") as stream:
        events = list(csv.DictReader(stream))
    assert {event["event_id"] for event in events} == set(split["development_event_ids"])
    assert all(event["label_source"] == "independent_human_annotation" and event["event_type"] == "plate" for event in events)
    allowed = [common.TimeInterval(**item) for item in split["development_allowed_intervals"]]
    excluded = [common.TimeInterval(**item) for item in split["reserved_exclusion_intervals"]]
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
        assert not any(interval.contains(round(t * fps) / fps) for t in timestamps for interval in excluded)
        frames, observations, crops_total, failed = [], [], 0, 0
        for index, timestamp in enumerate(timestamps):
            if (args.output.parent.parent / "HOST_RAM_STOP").exists():
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
            observed, crops, detections = processor.frame_processor.process_frame(frame,
                source_sha256=split["source_video_sha256"], source_file="sample.mp4",
                frame_number=number, timestamp_seconds=number / fps, source_frame_sha256=frame_hash)
            observations.extend(observed)
            crops_total += crops
            frames.append({"frame_number": number, "timestamp_seconds": number / fps,
                "source_frame_sha256": frame_hash, "detections": detections, "ocr_crops": crops,
                "predicted_raw": [item.raw_ocr for item in observed],
                "latency_seconds": time.perf_counter() - frame_started,
                "upstream_trace": processor.frame_processor.last_frame_trace})
            if index % 20 == 0:
                print(json.dumps({"mode": args.mode, "progress_frames": index + 1, "scheduled": len(timestamps)}), flush=True)
    finally:
        capture.release()
    groups = common.aggregate_product_observations(observations, common.SELECTED_POLICY)
    score = common.scorer.score_video(events, common.group_frame_rows(groups, observations, frames))
    by_id = {item.observation_id: item for item in observations}
    assert all(group.selected_normalized_plate in {by_id[i].normalized_ocr for i in group.observation_ids} for group in groups)
    utility = common.utility_checks(score["event_summary"], score["grouping"])
    result.update(split_digest=digest, oracle_projection_sha256=common.hash_file(args.oracle),
        policy=asdict(common.SELECTED_POLICY), frame_policy=asdict(policy),
        score={"events": score["event_summary"], "grouping": score["grouping"]},
        stage_fpr=common.stage_metrics(events, frames, groups, score, observations),
        utility_gate_checks=utility, numeric_utility_gate="PASS" if all(utility.values()) else "FAIL",
        source_time_review_required=True,
        coverage={"source_fps": fps, "source_duration_seconds": duration, "source_frame_count": count,
            "frames_analyzed": len(frames), "frames_scheduled": len(timestamps), "ocr_calls": crops_total,
            "failed_frames": failed, "partial_sampling": len(frames) < count,
            "effective_frame_coverage": common.scorer.safe_div(len(frames), count)},
        private_output={"frames": frames, "observations": [asdict(item) for item in observations],
                        "groups": [asdict(item) for item in groups], "scorer_details": score})


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=("imports", "smoke", "development"))
    parser.add_argument("--mode", choices=INPUT_MODES, default="RAW_RGB")
    for name in ("output", "registration"):
        parser.add_argument("--" + name, type=Path, required=True)
    for name in ("oracle", "split", "video"):
        parser.add_argument("--" + name, type=Path)
    run(parser.parse_args())
