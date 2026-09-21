#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Freeze and development-score the NX-MMR video ANPR parity adapter."""

from __future__ import annotations

import argparse
import csv
import ctypes
import hashlib
import json
import os
import sys
import time
from collections import defaultdict
from dataclasses import asdict
from pathlib import Path
from typing import Any, Iterable

REPOSITORY = Path(__file__).resolve().parents[1]
if str(REPOSITORY) not in sys.path:
    sys.path.insert(0, str(REPOSITORY))
if str(REPOSITORY / "scripts") not in sys.path:
    sys.path.insert(0, str(REPOSITORY / "scripts"))

from ingestion.forensic_records.video_anpr_parity_adapter import (  # noqa: E402
    ADAPTER_ID,
    ADAPTER_VERSION,
    AggregationPolicy,
    FramePolicy,
    HashPinnedEasyOCR,
    HashPinnedYOLODetector,
    PlateFrameProcessor,
    PlateBounds,
    PlateObservation,
    TimeInterval,
    aggregate_observations,
    build_freeze_manifest,
    canonical_digest,
    coverage_state,
    make_coverage_receipt,
    schedule_base_frames,
    schedule_refinement_frames,
    sha256_file,
)
import benchmark_nexusai_nxmmr_anpr as baseline  # noqa: E402
import benchmark_nexusai_nxmmr_reference_parity as reference  # noqa: E402


SPLIT_CONTRACT = "nexusai.nxmmr.video-development-reserved-split/v1"
BENCHMARK_CONTRACT = "nexusai.nxmmr.reference-parity-adapter-development/v1"
EXPECTED_VIDEO_SHA256 = baseline.EXPECTED_VIDEO_SHA256
EXPECTED_GOLD_DIGEST = baseline.EXPECTED_GOLD_DIGEST
SPLIT_SEED = "nxmmr-reference-parity-adapter-v1-event-split-20260830"
RESERVED_EVENT_TARGET = 7
RESERVED_GUARD_SECONDS = 0.75


def write_new_json(path: Path, value: dict[str, Any]) -> None:
    encoded = json.dumps(value, ensure_ascii=False, indent=2) + "\n"
    if path.exists():
        if path.read_text(encoding="utf-8") != encoded:
            raise RuntimeError(f"refusing to overwrite frozen output {path}")
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(encoded, encoding="utf-8")
    os.replace(temporary, path)


def load_video_events(oracle_root: Path) -> tuple[dict[str, Any], dict[str, Any], list[dict[str, str]]]:
    validation, state = baseline.load_oracle(oracle_root)
    if validation.get("status") != "PASS" or state.get("gold_digest") != EXPECTED_GOLD_DIGEST:
        raise RuntimeError("locked human oracle identity mismatch")
    with (oracle_root / "video-events.csv").open("r", encoding="utf-8-sig", newline="") as source:
        events = [row for row in csv.DictReader(source) if row.get("event_type") == "plate"]
    if len(events) != 19:
        raise RuntimeError("locked video oracle must contain 19 plate events")
    return validation, state, events


def overlap_components(events: list[dict[str, str]]) -> list[list[dict[str, str]]]:
    components: list[list[dict[str, str]]] = []
    component_end = -1.0
    for event in sorted(events, key=lambda row: (float(row["start_seconds"]), row["event_id"])):
        start = float(event["start_seconds"])
        end = float(event["end_seconds"])
        if not components or start > component_end:
            components.append([event])
            component_end = end
        else:
            components[-1].append(event)
            component_end = max(component_end, end)
    return components


def merge_intervals(intervals: Iterable[TimeInterval]) -> tuple[TimeInterval, ...]:
    merged: list[TimeInterval] = []
    for interval in sorted(intervals, key=lambda item: (item.start_seconds, item.end_seconds)):
        if not merged or interval.start_seconds > merged[-1].end_seconds:
            merged.append(interval)
        else:
            merged[-1] = TimeInterval(
                merged[-1].start_seconds,
                max(merged[-1].end_seconds, interval.end_seconds),
            )
    return tuple(merged)


def complement_intervals(
    duration_seconds: float, excluded: Iterable[TimeInterval]
) -> tuple[TimeInterval, ...]:
    output: list[TimeInterval] = []
    cursor = 0.0
    for interval in merge_intervals(excluded):
        if interval.start_seconds > cursor:
            output.append(TimeInterval(cursor, interval.start_seconds))
        cursor = max(cursor, interval.end_seconds)
    if cursor < duration_seconds:
        output.append(TimeInterval(cursor, duration_seconds))
    return tuple(output)


def freeze_split(args: argparse.Namespace) -> dict[str, Any]:
    _validation, state, events = load_video_events(args.oracle_root)
    capture, duration, fps, _frame_count = open_video_metadata(args.video)
    capture.release()
    if sha256_file(args.video) != EXPECTED_VIDEO_SHA256:
        raise RuntimeError("sample video identity mismatch")
    ranked_components = sorted(
        overlap_components(events),
        key=lambda component: canonical_digest({
            "seed": SPLIT_SEED,
            "event_ids": sorted(event["event_id"] for event in component),
        }),
    )
    reserved: list[dict[str, str]] = []
    development: list[dict[str, str]] = []
    for component in ranked_components:
        if len(reserved) < RESERVED_EVENT_TARGET:
            reserved.extend(component)
        else:
            development.extend(component)
    if not development or not reserved:
        raise RuntimeError("deterministic split did not preserve both partitions")
    reserved_intervals = merge_intervals(
        TimeInterval(
            max(0.0, float(event["start_seconds"]) - RESERVED_GUARD_SECONDS),
            min(duration, float(event["end_seconds"]) + RESERVED_GUARD_SECONDS),
        )
        for event in reserved
    )
    development_scope = complement_intervals(duration, reserved_intervals)
    receipt: dict[str, Any] = {
        "contract_version": SPLIT_CONTRACT,
        "created_before_candidate_inference": True,
        "selection_uses_model_output": False,
        "algorithm": "overlap components ranked by SHA-256(seed, sorted event IDs)",
        "seed": SPLIT_SEED,
        "source_video_sha256": EXPECTED_VIDEO_SHA256,
        "source_duration_seconds": duration,
        "source_fps": fps,
        "gold_digest": state["gold_digest"],
        "guard_seconds": RESERVED_GUARD_SECONDS,
        "development_event_ids": sorted(event["event_id"] for event in development),
        "reserved_event_ids": sorted(event["event_id"] for event in reserved),
        "development_event_count": len(development),
        "reserved_event_count": len(reserved),
        "reserved_exclusion_intervals": [asdict(interval) for interval in reserved_intervals],
        "development_allowed_intervals": [asdict(interval) for interval in development_scope],
        "candidate_evaluation_performed": False,
    }
    receipt["split_digest"] = canonical_digest(receipt)
    write_new_json(args.output, receipt)
    return receipt


def load_split(path: Path, video: Path, oracle_root: Path) -> tuple[dict[str, Any], list[dict[str, str]]]:
    split = json.loads(path.read_text(encoding="utf-8"))
    digest = split.pop("split_digest", None)
    if digest != canonical_digest(split):
        raise RuntimeError("development/reserved split digest mismatch")
    split["split_digest"] = digest
    if (
        split.get("contract_version") != SPLIT_CONTRACT
        or split.get("created_before_candidate_inference") is not True
        or split.get("candidate_evaluation_performed") is not False
        or split.get("source_video_sha256") != sha256_file(video)
    ):
        raise RuntimeError("development/reserved split contract mismatch")
    _validation, _state, events = load_video_events(oracle_root)
    development_ids = set(split["development_event_ids"])
    reserved_ids = set(split["reserved_event_ids"])
    if development_ids & reserved_ids or development_ids | reserved_ids != {event["event_id"] for event in events}:
        raise RuntimeError("development/reserved event accounting mismatch")
    development = [event for event in events if event["event_id"] in development_ids]
    return split, development


def peak_rss_mib() -> float | None:
    if os.name != "nt":
        try:
            import resource

            value = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
            return round(value / 1024, 3)
        except (ImportError, AttributeError):
            return None
    try:
        class ProcessMemoryCounters(ctypes.Structure):
            _fields_ = [
                ("cb", ctypes.c_ulong),
                ("PageFaultCount", ctypes.c_ulong),
                ("PeakWorkingSetSize", ctypes.c_size_t),
                ("WorkingSetSize", ctypes.c_size_t),
                ("QuotaPeakPagedPoolUsage", ctypes.c_size_t),
                ("QuotaPagedPoolUsage", ctypes.c_size_t),
                ("QuotaPeakNonPagedPoolUsage", ctypes.c_size_t),
                ("QuotaNonPagedPoolUsage", ctypes.c_size_t),
                ("PagefileUsage", ctypes.c_size_t),
                ("PeakPagefileUsage", ctypes.c_size_t),
            ]

        counters = ProcessMemoryCounters()
        counters.cb = ctypes.sizeof(counters)
        from ctypes import wintypes

        process = ctypes.windll.kernel32.GetCurrentProcess()
        get_memory = ctypes.WinDLL("kernel32", use_last_error=True).K32GetProcessMemoryInfo
        get_memory.argtypes = [
            wintypes.HANDLE,
            ctypes.POINTER(ProcessMemoryCounters),
            wintypes.DWORD,
        ]
        get_memory.restype = wintypes.BOOL
        if not get_memory(process, ctypes.byref(counters), counters.cb):
            return None
        return round(counters.PeakWorkingSetSize / (1024 * 1024), 3)
    except (AttributeError, OSError):
        return None


def free_ram_gib() -> float | None:
    try:
        import psutil  # type: ignore[import-not-found]

        return psutil.virtual_memory().available / (1024**3)
    except ModuleNotFoundError:
        return None


def open_video_metadata(video: Path) -> tuple[Any, float, float, int]:
    import cv2  # type: ignore[import-not-found]

    capture = cv2.VideoCapture(str(video))
    if not capture.isOpened():
        raise RuntimeError("OpenCV could not open the development video")
    fps = float(capture.get(cv2.CAP_PROP_FPS))
    frame_count = int(capture.get(cv2.CAP_PROP_FRAME_COUNT))
    duration = frame_count / fps if fps > 0 else 0.0
    if fps <= 0 or frame_count <= 0 or duration <= 0:
        capture.release()
        raise RuntimeError("video has invalid frame metadata")
    return capture, duration, fps, frame_count


def read_scheduled_frames(
    capture: Any,
    timestamps: Iterable[float],
    fps: float,
) -> Iterable[tuple[Any, int, float]]:
    import cv2  # type: ignore[import-not-found]

    for timestamp in sorted(set(timestamps)):
        frame_number = round(timestamp * fps)
        capture.set(cv2.CAP_PROP_POS_FRAMES, frame_number)
        ok, frame = capture.read()
        if not ok:
            yield None, frame_number, frame_number / fps
            continue
        yield frame, frame_number, frame_number / fps


def process_times(
    capture: Any,
    timestamps: Iterable[float],
    fps: float,
    processor: PlateFrameProcessor,
    *,
    source_sha256: str,
    source_file: str,
    maximum_ocr_crops: int,
) -> tuple[list[Any], list[dict[str, Any]], list[float], int, int]:
    import cv2  # type: ignore[import-not-found]

    observations: list[Any] = []
    frames: list[dict[str, Any]] = []
    triggers: list[float] = []
    failed = 0
    crops = 0
    for frame, frame_number, timestamp in read_scheduled_frames(capture, timestamps, fps):
        if frame is None:
            failed += 1
            continue
        encoded, frame_bytes = cv2.imencode(".png", frame)
        frame_hash = hashlib.sha256(bytes(frame_bytes)).hexdigest() if encoded else ""
        started = time.perf_counter()
        observed, frame_crops, detection_count = processor.process_frame(
            frame,
            source_sha256=source_sha256,
            source_file=source_file,
            frame_number=frame_number,
            timestamp_seconds=timestamp,
            source_frame_sha256=frame_hash,
        )
        if crops + frame_crops > maximum_ocr_crops:
            break
        crops += frame_crops
        observations.extend(observed)
        if detection_count:
            triggers.append(timestamp)
        frames.append({
            "timestamp_seconds": timestamp,
            "frame_number": frame_number,
            "source_frame_sha256": frame_hash,
            "predicted_raw": [item.raw_ocr for item in observed],
            "latency_seconds": round(time.perf_counter() - started, 6),
            "detections": detection_count,
            "ocr_crops": frame_crops,
        })
    return observations, frames, triggers, failed, crops


def group_frame_rows(
    groups: Iterable[Any],
    observations: Iterable[Any],
    analyzed_frames: Iterable[dict[str, Any]],
) -> list[dict[str, Any]]:
    by_id = {item.observation_id: item for item in observations}
    rows: dict[float, dict[str, Any]] = {
        float(frame["timestamp_seconds"]): {
            "timestamp_seconds": float(frame["timestamp_seconds"]),
            "predicted_raw": [],
            "latency_seconds": float(frame.get("latency_seconds") or 0),
        }
        for frame in analyzed_frames
    }
    for group in groups:
        for observation_id in group.observation_ids:
            item = by_id[observation_id]
            row = rows.setdefault(item.timestamp_seconds, {
                "timestamp_seconds": item.timestamp_seconds,
                "predicted_raw": [],
                "latency_seconds": 0.0,
            })
            if item.normalized_ocr == group.selected_normalized_plate:
                row["predicted_raw"].append(group.selected_raw_plate)
    return [rows[key] for key in sorted(rows)]


def summarize_score(score: dict[str, Any]) -> dict[str, Any]:
    return {
        "events": score["event_summary"],
        "grouping": score["grouping"],
    }


def _timestamp_allowed(timestamp: float, intervals: tuple[TimeInterval, ...]) -> bool:
    return any(interval.contains(timestamp) for interval in intervals)


def score_development_baselines(args: argparse.Namespace) -> dict[str, Any]:
    split, development_events = load_split(args.split, args.video, args.oracle_root)
    allowed = tuple(TimeInterval(**item) for item in split["development_allowed_intervals"])
    current = json.loads(args.current_receipt.read_text(encoding="utf-8"))
    if current.get("source", {}).get("sha256") != EXPECTED_VIDEO_SHA256:
        raise RuntimeError("current baseline receipt source identity mismatch")
    current_frames = [
        frame for frame in current["private_results"]["frames"]
        if _timestamp_allowed(float(frame["timestamp_seconds"]), allowed)
    ]
    if sha256_file(args.reference_csv) != reference.EXPECTED_VIDEO_CSV_SHA256:
        raise RuntimeError("historical raw CSV identity mismatch")
    if sha256_file(args.reference_interpolated_csv) != reference.EXPECTED_VIDEO_INTERPOLATED_SHA256:
        raise RuntimeError("historical interpolated CSV identity mismatch")
    with args.reference_csv.open("r", encoding="utf-8-sig", newline="") as source:
        raw_rows = list(csv.DictReader(source))
    with args.reference_interpolated_csv.open("r", encoding="utf-8-sig", newline="") as source:
        interpolated_rows = list(csv.DictReader(source))
    final_rows = reference.selected_track_rows(raw_rows, interpolated_rows)
    raw_frames = [
        frame for frame in reference.load_video_frames(raw_rows)
        if _timestamp_allowed(float(frame["timestamp_seconds"]), allowed)
    ]
    final_frames = [
        frame for frame in reference.load_video_frames(final_rows)
        if _timestamp_allowed(float(frame["timestamp_seconds"]), allowed)
    ]
    receipt = {
        "contract_version": "nexusai.nxmmr.parity-adapter-development-baselines/v1",
        "development_split_digest": split["split_digest"],
        "development_event_count": len(development_events),
        "reserved_event_count": int(split["reserved_event_count"]),
        "reserved_candidate_metrics_computed": False,
        "current_nxmmr_baseline_v1": {
            "receipt_sha256": sha256_file(args.current_receipt),
            "analyzed_development_frames": len(current_frames),
            "score": summarize_score(baseline.score_video(development_events, current_frames)),
        },
        "historical_raw_reference": {
            "source_csv_sha256": reference.EXPECTED_VIDEO_CSV_SHA256,
            "development_frames": len(raw_frames),
            "score": summarize_score(baseline.score_video(development_events, raw_frames)),
        },
        "historical_final_reference": {
            "source_csv_sha256": reference.EXPECTED_VIDEO_INTERPOLATED_SHA256,
            "development_frames": len(final_frames),
            "score": summarize_score(baseline.score_video(development_events, final_frames)),
        },
        "candidate_evaluation_performed": False,
    }
    write_new_json(args.output, receipt)
    return receipt


def replay_development_aggregation(args: argparse.Namespace) -> dict[str, Any]:
    config = json.loads(args.config.read_text(encoding="utf-8"))
    split, development_events = load_split(args.split, args.video, args.oracle_root)
    development = json.loads(args.development_receipt.read_text(encoding="utf-8"))
    if (
        development.get("development_split_digest") != split["split_digest"]
        or development.get("reserved_candidate_metrics_computed") is not False
    ):
        raise RuntimeError("development receipt is not bound to the locked split")
    bounds = config["bounds"]
    experiments: list[dict[str, Any]] = []
    for candidate in development["candidates"]:
        private = candidate.get("private_output")
        if not isinstance(private, dict):
            continue
        observations = tuple(
            PlateObservation(**{
                **row,
                "bounds": PlateBounds(**row["bounds"]),
            })
            for row in private["observations"]
        )
        frames = private["frames"]
        for strategy in config["aggregation_candidates"]:
            for minimum_support in config["minimum_group_support_candidates"]:
                aggregation_policy = AggregationPolicy(
                    strategy=strategy,
                    maximum_temporal_gap_seconds=float(bounds["maximum_temporal_gap_seconds"]),
                    minimum_spatial_iou=float(bounds["minimum_spatial_iou"]),
                    maximum_center_distance_ratio=float(bounds["maximum_center_distance_ratio"]),
                    maximum_candidate_edit_distance=int(bounds["maximum_candidate_edit_distance"]),
                    maximum_observations_per_group=int(bounds["maximum_observations_per_group"]),
                    maximum_active_groups=int(bounds["maximum_active_groups"]),
                    minimum_selected_support=int(minimum_support),
                )
                groups = aggregate_observations(observations, aggregation_policy)
                score = summarize_score(baseline.score_video(
                    development_events,
                    group_frame_rows(groups, observations, frames),
                ))
                experiments.append({
                    "frame_policy_id": candidate["candidate_id"],
                    "aggregation_strategy": strategy,
                    "minimum_selected_support": minimum_support,
                    "group_count": len(groups),
                    "coverage": {
                        "frames_analyzed_by_plate_detector": candidate["coverage"]["frames_analyzed_by_plate_detector"],
                        "frames_refined_corrected": max(
                            0,
                            int(candidate["coverage"]["frames_analyzed_by_plate_detector"])
                            - int(next(
                                row["coverage"]["frames_analyzed_by_plate_detector"]
                                for row in development["candidates"]
                                if row.get("candidate_id") == "fixed-2fps-plate-only-majority"
                            )),
                        ) if candidate["candidate_id"].startswith("adaptive-2fps") else 0,
                        "ocr_crops_processed": candidate["coverage"]["ocr_crops_processed"],
                        "wall_seconds": candidate["coverage"]["wall_seconds"],
                        "cpu_seconds": candidate["coverage"]["cpu_seconds"],
                    },
                    "score": score,
                })
    receipt = {
        "contract_version": "nexusai.nxmmr.parity-adapter-aggregation-replay/v1",
        "source_development_receipt_sha256": sha256_file(args.development_receipt),
        "development_split_digest": split["split_digest"],
        "development_event_count": len(development_events),
        "reserved_event_count": int(split["reserved_event_count"]),
        "replay_uses_existing_development_observations_only": True,
        "reserved_candidate_metrics_computed": False,
        "experiments": experiments,
        "candidate_evaluation_performed": False,
    }
    write_new_json(args.output, receipt)
    return receipt


def strategy_rank(row: dict[str, Any]) -> tuple[Any, ...]:
    score = row["selected_aggregation_score"]
    events = score["events"]
    grouping = score["grouping"]
    return (
        float(events["normalized_exact_plate"].get("f1") or 0),
        float(events.get("event_detection_recall") or 0),
        float(grouping.get("exact_group_f1") or 0),
        -float(events.get("negative_frame_false_positive_rate") or 0),
        -int(row["coverage"]["frames_analyzed_by_plate_detector"]),
        -float(row["coverage"]["wall_seconds"]),
        row["candidate_id"],
    )


def run_candidate(
    candidate: dict[str, Any],
    *,
    video: Path,
    source_hash: str,
    duration: float,
    fps: float,
    frame_count: int,
    capture: Any,
    processor: PlateFrameProcessor,
    allowed_intervals: tuple[TimeInterval, ...],
    development_events: list[dict[str, str]],
    config: dict[str, Any],
) -> dict[str, Any]:
    bounds = config["bounds"]
    frame_policy = FramePolicy(
        candidate["id"],
        base_cadence_fps=float(candidate["base_cadence_fps"]),
        refinement_enabled=bool(candidate.get("refinement_enabled")),
        refinement_window_seconds=float(candidate.get("refinement_window_seconds", 0.5)),
        refinement_cadence_fps=float(candidate.get("refinement_cadence_fps", 8)),
        maximum_analyzed_frames=int(bounds["maximum_analyzed_frames"]),
        maximum_refinement_frames_per_trigger=int(bounds["maximum_refinement_frames_per_trigger"]),
    )
    base_times = schedule_base_frames(
        duration, fps, frame_policy, allowed_intervals=allowed_intervals
    )
    wall_started = time.perf_counter()
    cpu_started = time.process_time()
    observations, frames, triggers, failed, crops = process_times(
        capture,
        base_times,
        fps,
        processor,
        source_sha256=source_hash,
        source_file=video.name,
        maximum_ocr_crops=int(bounds["maximum_ocr_crops"]),
    )
    refinement_times = schedule_refinement_frames(
        triggers,
        duration,
        fps,
        frame_policy,
        already_scheduled=base_times,
        allowed_intervals=allowed_intervals,
    )
    refined_analyzed_count = 0
    if refinement_times:
        refined, refined_frames, _refined_triggers, refined_failed, refined_crops = process_times(
            capture,
            refinement_times,
            fps,
            processor,
            source_sha256=source_hash,
            source_file=video.name,
            maximum_ocr_crops=max(0, int(bounds["maximum_ocr_crops"]) - crops),
        )
        observations.extend(refined)
        frames.extend(refined_frames)
        refined_analyzed_count = len(refined_frames)
        failed += refined_failed
        crops += refined_crops
    wall_seconds = time.perf_counter() - wall_started
    cpu_seconds = time.process_time() - cpu_started
    aggregation_rows: dict[str, Any] = {}
    selected_groups = ()
    for strategy in config["aggregation_candidates"]:
        aggregation_policy = AggregationPolicy(
            strategy=strategy,
            maximum_temporal_gap_seconds=float(bounds["maximum_temporal_gap_seconds"]),
            minimum_spatial_iou=float(bounds["minimum_spatial_iou"]),
            maximum_center_distance_ratio=float(bounds["maximum_center_distance_ratio"]),
            maximum_candidate_edit_distance=int(bounds["maximum_candidate_edit_distance"]),
            maximum_observations_per_group=int(bounds["maximum_observations_per_group"]),
            maximum_active_groups=int(bounds["maximum_active_groups"]),
            minimum_selected_support=int(candidate.get("minimum_selected_support", 1)),
        )
        groups = aggregate_observations(observations, aggregation_policy)
        score = baseline.score_video(
            development_events,
            group_frame_rows(groups, observations, frames),
        )
        aggregation_rows[strategy] = {
            "group_count": len(groups),
            "score": summarize_score(score),
        }
        if strategy == candidate["aggregation"]:
            selected_groups = groups
    raw_score = baseline.score_video(development_events, frames)
    selected_score = aggregation_rows[candidate["aggregation"]]["score"]
    coverage = make_coverage_receipt(
        source_fps=fps,
        source_duration_seconds=duration,
        source_frame_count=frame_count,
        frames_decoded=len(frames) + failed,
        frames_analyzed_by_plate_detector=len(frames),
        frames_refined=refined_analyzed_count,
        ocr_crops_processed=crops,
        sampling_policy=candidate["id"],
        dropped_or_failed_frames=failed,
        wall_seconds=wall_seconds,
        cpu_seconds=cpu_seconds,
        peak_ram_mib=peak_rss_mib(),
    )
    state, limitations = coverage_state(coverage, len(observations))
    return {
        "candidate_id": candidate["id"],
        "candidate": candidate,
        "state": state,
        "limitations": list(limitations),
        "coverage": asdict(coverage),
        "raw_observation_score": summarize_score(raw_score),
        "aggregation_experiments": aggregation_rows,
        "selected_aggregation_score": selected_score,
        "private_output": {
            "frames": sorted(frames, key=lambda item: item["timestamp_seconds"]),
            "observations": [asdict(item) for item in observations],
            "groups": [asdict(item) for item in selected_groups],
        },
    }


def run_development(args: argparse.Namespace) -> tuple[dict[str, Any], dict[str, Any]]:
    config = json.loads(args.config.read_text(encoding="utf-8"))
    if config.get("adapter_id") != ADAPTER_ID or config.get("activation_state") != "source_development_only":
        raise RuntimeError("adapter configuration is not the source-development contract")
    split, development_events = load_split(args.split, args.video, args.oracle_root)
    allowed_intervals = tuple(TimeInterval(**item) for item in split["development_allowed_intervals"])
    if args.output.exists() or args.freeze_output.exists():
        raise RuntimeError("refusing to overwrite development or freeze output")
    source_hash = sha256_file(args.video)
    import cv2  # type: ignore[import-not-found]

    capture, duration, fps, frame_count = open_video_metadata(args.video)
    try:
        if duration > float(config["bounds"]["maximum_video_duration_seconds"]):
            raise RuntimeError("video exceeds candidate duration bound")
        free_before = free_ram_gib()
        plate_model = config["models"]["plate_detector"]
        craft_model = config["models"]["ocr_detector"]
        recognizer_model = config["models"]["ocr_recognizer"]
        plate_detector = HashPinnedYOLODetector(
            args.plate_detector,
            plate_model["sha256"],
            input_size=int(plate_model["input_size"]),
            confidence=float(config["thresholds"]["plate_detection_confidence"]),
        )
        ocr = HashPinnedEasyOCR(
            args.easyocr_craft,
            craft_model["sha256"],
            args.easyocr_recognizer,
            recognizer_model["sha256"],
        )
        candidates = []
        candidate_specs = [
            candidate for candidate in config["development_candidates"]
            if not args.candidate_id or candidate["id"] in set(args.candidate_id)
        ]
        if not candidate_specs:
            raise RuntimeError("no configured development candidate matched --candidate-id")
        for candidate in candidate_specs:
            if candidate.get("vehicle_context_enabled"):
                if free_before is not None and float(free_before) < 4.5:
                    candidates.append({
                        "candidate_id": candidate["id"],
                        "candidate": candidate,
                        "state": "resource_blocked_before_model_load",
                        "required_free_ram_gib": 4.5,
                        "free_ram_gib": round(float(free_before), 3),
                        "candidate_evaluation_performed": False,
                    })
                    continue
                vehicle_model = config["models"]["vehicle_detector"]
                vehicle_detector = HashPinnedYOLODetector(
                    args.vehicle_detector,
                    vehicle_model["sha256"],
                    input_size=int(vehicle_model["input_size"]),
                    confidence=float(config["thresholds"]["plate_detection_confidence"]),
                    allowed_classes={2, 3, 5, 7},
                )
            else:
                vehicle_detector = None
            processor = PlateFrameProcessor(
                plate_detector,
                ocr,
                cv2,
                vehicle_detector=vehicle_detector,
                detection_threshold=float(config["thresholds"]["plate_detection_confidence"]),
                ocr_threshold=float(config["thresholds"]["ocr_confidence"]),
                binary_inverse_threshold=int(config["thresholds"]["historical_binary_inverse_threshold"]),
            )
            candidates.append(run_candidate(
                candidate,
                video=args.video,
                source_hash=source_hash,
                duration=duration,
                fps=fps,
                frame_count=frame_count,
                capture=capture,
                processor=processor,
                allowed_intervals=allowed_intervals,
                development_events=development_events,
                config=config,
            ))
        completed = [row for row in candidates if row.get("selected_aggregation_score")]
        if not completed:
            raise RuntimeError("no development candidate completed")
        selected = max(completed, key=strategy_rank)
        config_hash = sha256_file(args.config)
        report = {
            "contract_version": BENCHMARK_CONTRACT,
            "evaluated_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "adapter_id": ADAPTER_ID,
            "adapter_version": ADAPTER_VERSION,
            "configuration_sha256": config_hash,
            "development_split_digest": split["split_digest"],
            "development_event_count": len(development_events),
            "reserved_event_count": int(split["reserved_event_count"]),
            "reserved_candidate_metrics_computed": False,
            "candidate_evaluation_performed": False,
            "source": {
                "sha256": source_hash,
                "duration_seconds": duration,
                "fps": fps,
                "frame_count": frame_count,
            },
            "model_hashes": {
                role: value["sha256"] for role, value in config["models"].items()
            },
            "model_license_states": {
                role: value["license_state"] for role, value in config["models"].items()
            },
            "selection_rule": (
                "lexicographic: normalized exact F1, event recall, group F1, lower negative-frame FPR, "
                "fewer detector frames, lower wall time, candidate ID"
            ),
            "candidates": candidates,
            "selected_candidate_id": selected["candidate_id"],
            "selected_candidate": selected["candidate"],
            "free_ram_gib_before_models": round(float(free_before), 3) if free_before is not None else None,
            "free_ram_gib_after": round(float(free_ram_gib() or 0), 3) if free_ram_gib() is not None else None,
            "peak_process_ram_mib": peak_rss_mib(),
            "runtime_mutated": False,
            "retained_state_mutated": False,
            "activity_mutated": False,
            "database_or_volumes_mutated": False,
        }
        write_new_json(args.output, report)
        freeze = build_freeze_manifest(
            config,
            adapter_source_sha256=sha256_file(
                REPOSITORY / "ingestion" / "forensic_records" / "video_anpr_parity_adapter.py"
            ),
            selected_candidate=selected["candidate"],
            development_split_digest=split["split_digest"],
        )
        freeze["configuration_sha256"] = config_hash
        freeze["development_receipt_sha256"] = sha256_file(args.output)
        freeze["candidate_evaluation_performed"] = False
        freeze["freeze_digest"] = canonical_digest({
            key: value for key, value in freeze.items() if key != "freeze_digest"
        })
        write_new_json(args.freeze_output, freeze)
        return report, freeze
    finally:
        capture.release()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    split = subparsers.add_parser("freeze-split")
    split.add_argument("--oracle-root", type=Path, required=True)
    split.add_argument("--video", type=Path, required=True)
    split.add_argument("--output", type=Path, required=True)

    development = subparsers.add_parser("development")
    development.add_argument("--oracle-root", type=Path, required=True)
    development.add_argument("--video", type=Path, required=True)
    development.add_argument("--split", type=Path, required=True)
    development.add_argument("--config", type=Path, required=True)
    development.add_argument("--plate-detector", type=Path, required=True)
    development.add_argument("--vehicle-detector", type=Path, required=True)
    development.add_argument("--easyocr-craft", type=Path, required=True)
    development.add_argument("--easyocr-recognizer", type=Path, required=True)
    development.add_argument("--output", type=Path, required=True)
    development.add_argument("--freeze-output", type=Path, required=True)
    development.add_argument("--candidate-id", action="append")

    baselines = subparsers.add_parser("development-baselines")
    baselines.add_argument("--oracle-root", type=Path, required=True)
    baselines.add_argument("--video", type=Path, required=True)
    baselines.add_argument("--split", type=Path, required=True)
    baselines.add_argument("--current-receipt", type=Path, required=True)
    baselines.add_argument("--reference-csv", type=Path, required=True)
    baselines.add_argument("--reference-interpolated-csv", type=Path, required=True)
    baselines.add_argument("--output", type=Path, required=True)

    aggregation = subparsers.add_parser("development-aggregation-replay")
    aggregation.add_argument("--oracle-root", type=Path, required=True)
    aggregation.add_argument("--video", type=Path, required=True)
    aggregation.add_argument("--split", type=Path, required=True)
    aggregation.add_argument("--config", type=Path, required=True)
    aggregation.add_argument("--development-receipt", type=Path, required=True)
    aggregation.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    if args.command == "freeze-split":
        result = freeze_split(args)
        print(json.dumps({
            "output": str(args.output),
            "split_digest": result["split_digest"],
            "development_event_count": result["development_event_count"],
            "reserved_event_count": result["reserved_event_count"],
            "candidate_evaluation_performed": False,
        }, indent=2))
    elif args.command == "development":
        report, freeze = run_development(args)
        print(json.dumps({
            "output": str(args.output),
            "freeze_output": str(args.freeze_output),
            "selected_candidate_id": report["selected_candidate_id"],
            "freeze_digest": freeze["freeze_digest"],
            "reserved_candidate_metrics_computed": False,
        }, indent=2))
    elif args.command == "development-baselines":
        result = score_development_baselines(args)
        print(json.dumps({
            "output": str(args.output),
            "development_event_count": result["development_event_count"],
            "reserved_event_count": result["reserved_event_count"],
            "reserved_candidate_metrics_computed": False,
        }, indent=2))
    else:
        result = replay_development_aggregation(args)
        print(json.dumps({
            "output": str(args.output),
            "experiments": len(result["experiments"]),
            "development_event_count": result["development_event_count"],
            "reserved_candidate_metrics_computed": False,
        }, indent=2))


if __name__ == "__main__":
    main()
