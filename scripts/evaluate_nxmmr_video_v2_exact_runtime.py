#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Fixed-candidate evaluator; never searches/selects candidate policies.

The caller supplies only the admitted partition's oracle projection. Inference
uses the unchanged product factory/frame processor, scheduler and aggregation;
primary scoring reuses the existing development implementation verbatim.
"""
from __future__ import annotations

import argparse
import csv
import hashlib
import importlib.metadata as metadata
import json
import os
import platform
try:
    import resource
except ImportError:  # Host-side pure scoring tests do not measure Linux RSS.
    resource = None
import sys
import threading
import time
from dataclasses import asdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT), str(ROOT / "scripts")]
from ingestion.forensic_records.video_anpr_parity_adapter import (
    FramePolicy, TimeInterval, canonical_digest, coverage_state,
    make_coverage_receipt, schedule_base_frames,
)
from ingestion.forensic_records.video_anpr_product_v2 import (
    SELECTED_POLICY, VideoANPRProductV2Processor, aggregate_product_observations,
)
import benchmark_nexusai_nxmmr_anpr as scorer
from benchmark_nexusai_nxmmr_parity_adapter import group_frame_rows
from nxmmr_evaluator_resources import finalized_resources

FREEZE = "212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20"
CORE = {"torch": "2.5.1+cpu", "torchvision": "0.20.1+cpu", "ultralytics": "8.0.114",
        "numpy": "2.3.5", "fast-alpr": "0.4.0", "fast-plate-ocr": "1.1.0", "open-image-models": "0.6.0"}


def hash_file(path):
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def write_new(path, value):
    value["receipt_digest"] = canonical_digest(value)
    with path.open("x", encoding="utf-8") as output:
        json.dump(value, output, indent=2, ensure_ascii=False)
        output.write("\n")


def stage_metrics(events, frames, groups, score, observations=None):
    negative = [frame for frame in frames if not any(scorer.contains(event, frame["timestamp_seconds"]) for event in events)]
    detector_fp = sum(frame["detections"] > 0 for frame in negative)
    ocr_fp = sum(bool(frame["predicted_raw"]) for frame in negative)
    actual_texts = {scorer.normalize_plate(value) for event in events for value in scorer.event_truth(event)}
    false_temporal = None
    if observations is not None:
        by_id = {item.observation_id: item for item in observations}
        false_temporal = sum(not any(
            group.selected_normalized_plate in {scorer.normalize_plate(value) for value in scorer.event_truth(event)}
            and any(scorer.contains(event, by_id[identity].timestamp_seconds) for identity in group.observation_ids)
            for event in events) for group in groups)
    return {"negative_frames": len(negative), "raw_detector_false_frames": detector_fp,
        "RawDetectorNegativeFrameFPR": scorer.safe_div(detector_fp, len(negative)),
        "ocr_candidate_false_frames": ocr_fp, "OCRCandidateNegativeFrameFPR": scorer.safe_div(ocr_fp, len(negative)),
        "GroupSelectedNegativeFrameFPR": score["event_summary"]["negative_frame_false_positive_rate"],
        "FinalFalseGroupCount": score["grouping"]["exact_group_fp"],
        "FinalGroupPrecision": score["grouping"]["exact_group_precision"],
        "temporal_group_packet_count": len(groups),
        "false_temporal_packet_count_by_plate_and_actual_time": false_temporal,
        "temporal_packet_match_definition": "selected string in a human event and at least one actual supporting observation within that event; not vehicle identity",
        "false_temporal_packet_count_by_plate_value": sum(group.selected_normalized_plate not in actual_texts for group in groups)}


def utility_checks(event_score, group_score):
    return {"event_recall": (event_score["event_detection_recall"] or 0) >= .75,
        "group_precision": (group_score["exact_group_precision"] or 0) >= .8,
        "group_f1": (group_score["exact_group_f1"] or 0) >= .65,
        "exact_f1": (event_score["normalized_exact_plate"]["f1"] or 0) >= .6}


def verify(args):
    if getattr(args, "candidate", "v2") == "v21":
        assert args.phase != "reserved", "V2.1 reserved evaluation is not authorized"
        admission = json.loads(args.evaluation_manifest.read_text())
        expected = admission.pop("receipt_digest")
        assert canonical_digest(admission) == expected
        assert admission["old_v2_freeze"] == FREEZE
        for name, digest in admission["source_files"].items():
            assert hash_file(ROOT / name) == digest, name
        assert hash_file(Path("/evaluator/resolved-runtime-lock.json")) == admission["runtime_lock_sha256"]
    freeze = json.loads(args.freeze.read_text())
    digest = freeze.pop("freeze_digest")
    assert digest == FREEZE == canonical_digest(freeze)
    manifest = json.loads(args.manifest.read_text())
    for name, expected in manifest["source_freeze_files"].items():
        assert hash_file(ROOT / name) == expected, name
    assert asdict(SELECTED_POLICY) == freeze["selected_policy"]
    assert hash_file(ROOT / "scripts/benchmark_nexusai_nxmmr_anpr.py") == "a65fdb599773cee766966ae24674db8435322130fc005218d0df1d046ab68830"
    assert hash_file(ROOT / "scripts/benchmark_nexusai_nxmmr_parity_adapter.py") == "42b00af2259c127ff26bb05b181b300d03e8a251f9b8ea21ec6a6c38af2ce4ca"
    assert sys.version_info[:2] == (3, 11)
    assert platform.system() == "Linux" and platform.machine() == "x86_64"
    actual = {name: metadata.version(name) for name in CORE}
    assert actual == CORE, actual
    lock = json.loads(Path("/evaluator/resolved-runtime-lock.json").read_text())
    for name, version in lock["resolved_versions"].items():
        assert metadata.version(name) == version, (name, version, metadata.version(name))
    for env, key in (
        ("FORENSIC_VIDEO_ANPR_PLATE_DETECTOR_PATH", "plate_detector_sha256"),
        ("FORENSIC_VIDEO_ANPR_VEHICLE_DETECTOR_PATH", "vehicle_detector_sha256"),
        ("FORENSIC_ANPR_OCR_MODEL_PATH", "ocr_model_sha256"),
        ("FORENSIC_ANPR_OCR_CONFIG_PATH", "ocr_config_sha256"),
    ):
        assert hash_file(os.environ[env]) == freeze["models"][key], env
    return actual


class ResourceMonitor:
    def __init__(self):
        import psutil
        self.psutil = psutil
        self.process = psutil.Process()
        self.initial_rss = self.process.memory_info().rss
        self.peak_rss = self.initial_rss
        self.peak_child_rss = 0
        self.peak_tree_rss = self.initial_rss
        self.end = threading.Event()
        self.thread = threading.Thread(target=self.sample, daemon=True)
        self.thread.start()

    def sample(self):
        while not self.end.is_set():
            try:
                own = self.process.memory_info().rss
                children = sum(child.memory_info().rss for child in self.process.children(recursive=True))
                self.peak_rss = max(self.peak_rss, own)
                self.peak_child_rss = max(self.peak_child_rss, children)
                self.peak_tree_rss = max(self.peak_tree_rss, own + children)
            except self.psutil.Error:
                pass
            self.end.wait(0.1)

    def finish(self):
        if hasattr(self, "snapshot"):
            return self.snapshot
        assert resource is not None, "inference resource measurement requires the admitted Linux runtime"
        self.end.set()
        self.thread.join()
        self.snapshot = {"initial_process_rss_mib": self.initial_rss / 2**20,
                "process_peak_rss_mib": resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024,
                "sampled_process_peak_rss_mib": self.peak_rss / 2**20,
                "child_process_peak_rss_mib": self.peak_child_rss / 2**20,
                "incremental_tree_peak_mib": (self.peak_tree_rss - self.initial_rss) / 2**20,
                "resource_sampling_seconds": 0.1}
        return self.snapshot


def run(args):
    if args.output.exists():
        raise RuntimeError("refusing to overwrite an existing evaluation receipt")
    if getattr(args, "candidate", "v2") == "v21" and args.phase == "reserved":
        raise RuntimeError("V2.1 reserved evaluation is not authorized")
    monitor = ResourceMonitor()
    started, cpu_started = time.perf_counter(), time.process_time()
    with finalized_resources(args.output.with_name(args.phase + "-process-resources.json"),
                             monitor, phase=args.phase, started=started, cpu_started=cpu_started):
        pins = verify(args)
        return run_admitted(args, pins, monitor, started, cpu_started)


def run_admitted(args, pins, monitor, started, cpu_started):
    v21 = getattr(args, "candidate", "v2") == "v21"
    import torch
    import torchvision
    import ultralytics
    import numpy as np
    import cv2
    import fast_alpr
    import fast_plate_ocr
    import open_image_models
    assert torch.version.cuda is None and not torch.cuda.is_available()
    assert torch.zeros(1, device="cpu").device.type == "cpu"
    if args.phase == "imports":
        write_new(args.output, {"phase": "imports", "state": "PASS", "core_versions": pins,
                  "python": sys.version, "platform": platform.platform(),
                  "packages": {name: metadata.version(name) for name in {d.metadata["Name"].lower().replace("_", "-") for d in metadata.distributions()}},
                  "interpreter_sha256": hash_file(sys.executable),
                  "module_origins": {name: str(module.__file__) for name, module in {"torch": torch, "torchvision": torchvision, "ultralytics": ultralytics, "numpy": np, "cv2": cv2, "fast_alpr": fast_alpr, "fast_plate_ocr": fast_plate_ocr, "open_image_models": open_image_models}.items()},
                  "cuda_available": False, "model_loads": 0,
                  "wall_seconds": time.perf_counter() - started,
                  "cpu_seconds": time.process_time() - cpu_started,
                  "resources": monitor.finish(), "candidate_freeze": None if v21 else FREEZE,
                  "candidate_revision": "v21" if v21 else "v2", "old_v2_freeze": FREEZE})
        return
    if v21:
        from ingestion.forensic_records.video_anpr_product_v21 import VideoANPRProductV21Processor, TRANSFORM_ID
        processor = VideoANPRProductV21Processor.from_environment()
    else:
        processor = VideoANPRProductV2Processor.from_environment()
    if args.phase == "smoke":
        frame = np.zeros((640, 640, 3), dtype=np.uint8)
        observations, crops, detections = processor.frame_processor.process_frame(
            frame, source_sha256="0" * 64, source_file="synthetic-zero-frame",
            frame_number=0, timestamp_seconds=0.0, source_frame_sha256=hashlib.sha256(frame.tobytes()).hexdigest())
        # Exercise the frozen 2D OCR input interface without an oracle/model answer.
        if v21:
            from ingestion.forensic_records.video_anpr_parity_adapter import PlateBounds
            from unittest.mock import patch
            # Deterministic synthetic boxes force the actual processor's crop,
            # threshold and authoritative RGB adapter without relying on detector
            # predictions or any oracle. Both real detectors ran above.
            frame[0:64, 0:64] = 255
            with patch.object(processor.frame_processor.plate_detector, "detect", return_value=[(PlateBounds(0, 0, 128, 64), 1.0, 0)]), patch.object(processor.frame_processor.vehicle_detector, "detect", return_value=[(PlateBounds(0, 0, 640, 640), 1.0, 2)]):
                synthetic_ocr, forced_crops, _ = processor.frame_processor.process_frame(
                    frame, source_sha256="0" * 64, source_file="synthetic-contract-frame",
                    frame_number=1, timestamp_seconds=0.25, source_frame_sha256=hashlib.sha256(frame.tobytes()).hexdigest())
            assert forced_crops == 1
            contract = processor.frame_processor.ocr.last_input_contract
            assert contract["shape"] == [64, 128, 3] and contract["dtype"] == "uint8"
        else:
            synthetic_ocr = processor.frame_processor.ocr.read(np.zeros((64, 128), dtype=np.uint8))
            contract = None
        write_new(args.output, {"phase": "smoke", "state": "PASS", "synthetic_only": True,
                  "detectors_loaded": 2, "ocr_model_loaded": True, "frames": 1,
                  "detections": detections, "ocr_crops": crops, "observations": len(observations),
                  "synthetic_ocr_interface_calls": 1, "synthetic_ocr_output_count": len(synthetic_ocr),
                  "ocr_input_contract": contract, "candidate_revision": "v21" if v21 else "v2",
                  "model_hashes": json.loads(args.freeze.read_text())["models"],
                  "wall_seconds": time.perf_counter() - started,
                  "cpu_seconds": time.process_time() - cpu_started,
                  "resources": monitor.finish(), "candidate_freeze": None if v21 else FREEZE})
        return
    split = json.loads(args.split.read_text())
    split_digest = split.pop("split_digest")
    assert canonical_digest(split) == split_digest
    if args.phase == "reserved":
        prior = json.loads(args.development_receipt.read_text())
        assert prior["development_gate"] == "PASS" and prior["candidate_freeze"] == FREEZE
        # Consumption is recorded before any reserved decoding/scoring.
        with args.output.with_suffix(".consumed").open("x") as marker:
            marker.write(FREEZE + "\n")
    ids = set(split[args.phase + "_event_ids"])
    with args.oracle.open(encoding="utf-8-sig", newline="") as source:
        events = list(csv.DictReader(source))
    assert {event["event_id"] for event in events} == ids
    assert all(event["event_type"] == "plate" and event["label_source"] == "independent_human_annotation" for event in events)
    interval_key = "development_allowed_intervals" if args.phase == "development" else "reserved_exclusion_intervals"
    allowed = tuple(TimeInterval(**row) for row in split[interval_key])
    assert hash_file(args.video) == split["source_video_sha256"]
    capture = cv2.VideoCapture(str(args.video))
    assert capture.isOpened()
    fps = capture.get(cv2.CAP_PROP_FPS)
    count = int(capture.get(cv2.CAP_PROP_FRAME_COUNT))
    duration = count / fps
    assert duration <= processor.maximum_duration_seconds
    policy = FramePolicy("fixed-4fps-fastplate-vehicle-v2", 4.0, maximum_analyzed_frames=processor.maximum_frames)
    timestamps = schedule_base_frames(duration, fps, policy, allowed_intervals=allowed)
    observations, frames = [], []
    failed, total_crops = 0, 0
    try:
        for index, timestamp in enumerate(timestamps):
            if args.stop_file.exists():
                raise RuntimeError("HOST_RAM_SAFETY_STOP")
            number = round(timestamp * fps)
            actual_time = number / fps
            assert any(interval.contains(actual_time) for interval in allowed)
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
                frame_number=number, timestamp_seconds=actual_time, source_frame_sha256=frame_hash)
            observations.extend(observed)
            total_crops += crops
            frames.append({"frame_number": number, "timestamp_seconds": actual_time,
                           "source_frame_sha256": frame_hash, "detections": detections,
                           "ocr_crops": crops, "predicted_raw": [item.raw_ocr for item in observed],
                           "latency_seconds": time.perf_counter() - frame_started})
            if index % 20 == 0:
                print(json.dumps({"progress_frames": index + 1, "scheduled": len(timestamps)}), flush=True)
    finally:
        capture.release()
    groups = aggregate_product_observations(observations, SELECTED_POLICY)
    group_frames = group_frame_rows(groups, observations, frames)
    score = scorer.score_video(events, group_frames)
    sources = {item.observation_id: item for item in observations}
    assert all(group.selected_normalized_plate in {sources[i].normalized_ocr for i in group.observation_ids} for group in groups)
    wall, cpu = time.perf_counter() - started, time.process_time() - cpu_started
    resources = monitor.finish()
    coverage = make_coverage_receipt(source_fps=fps, source_duration_seconds=duration, source_frame_count=count,
        frames_decoded=len(frames), frames_analyzed_by_plate_detector=len(frames), frames_refined=0,
        ocr_crops_processed=total_crops, sampling_policy=policy.policy_id, dropped_or_failed_frames=failed,
        wall_seconds=wall, cpu_seconds=cpu, peak_ram_mib=resources["process_peak_rss_mib"])
    state, limitations = coverage_state(coverage, len(observations))
    event_score, group_score = score["event_summary"], score["grouping"]
    utility = utility_checks(event_score, group_score)
    # Source-time/resources require explicit review even when numerical gates pass.
    gate = "FAIL" if not all(utility.values()) else None
    receipt = {"phase": args.phase, "candidate_freeze": None if v21 else FREEZE, "runtime_core": pins,
        "candidate_revision": "v21" if v21 else "v2", "old_v2_freeze": FREEZE,
        "evaluation_manifest_sha256": hash_file(args.evaluation_manifest) if v21 else None,
        "split_digest": split_digest, "oracle_projection_sha256": hash_file(args.oracle),
        "policy": asdict(SELECTED_POLICY), "preprocessing": TRANSFORM_ID if v21 else "GRAY_BINARY_INV_64",
        "score": {"events": event_score, "grouping": group_score},
        "event_detection": {
            "tp": sum(row["score"]["prediction_count"] > 0 for row in score["event_results"]),
            "fn": sum(row["score"]["prediction_count"] == 0 for row in score["event_results"]),
            "definition": "registered event_detection_recall: at least one group-selected prediction inside the human event; not detector IoU"},
        "stage_fpr": stage_metrics(events, frames, groups, score, observations),
        "utility_gate_checks": utility, "development_gate": gate if args.phase == "development" else None,
        "numeric_utility_gate": "PASS" if all(utility.values()) else "FAIL",
        "source_time_and_resource_review_required_before_reserved": True,
        "wall_seconds": wall, "cpu_seconds": cpu, "resources": resources,
        "coverage": asdict(coverage), "coverage_state": state, "limitations": limitations,
        "private_output": {"frames": frames, "observations": [asdict(x) for x in observations],
           "groups": [asdict(x) for x in groups], "scorer_details": score},
        "reserved_evaluation_count": int(args.phase == "reserved"), "post_hoc_tuning": False,
        "runtime_mutated": False, "retained_state_mutated": False}
    write_new(args.output, receipt)
    print(json.dumps({key: receipt[key] for key in ("phase", "score", "stage_fpr", "development_gate", "wall_seconds", "cpu_seconds", "resources", "coverage")}), flush=True)


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("phase", choices=["imports", "smoke", "development", "reserved"])
    p.add_argument("--candidate", choices=["v2", "v21"], default="v2")
    p.add_argument("--evaluation-manifest", type=Path)
    for name in ("freeze", "manifest", "output"):
        p.add_argument("--" + name, type=Path, required=True)
    for name in ("oracle", "split", "video", "development-receipt"):
        p.add_argument("--" + name, type=Path)
    p.add_argument("--stop-file", type=Path, default=Path("/results/HOST_RAM_STOP"))
    run(p.parse_args())
