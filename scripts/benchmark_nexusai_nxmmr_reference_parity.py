#!/usr/bin/env python3
"""Score frozen pre-existing ANPR references against the locked NX-MMR oracle."""

from __future__ import annotations

import argparse
import csv
import ctypes
import hashlib
import importlib.metadata
import json
import math
import os
import statistics
import sys
import time
from collections import defaultdict
from pathlib import Path
from typing import Any

import benchmark_nexusai_nxmmr_anpr as baseline


CONTRACT = "nexusai.nxmmr.reference-pipeline-parity/v1"
REFERENCE_IMAGE_CANDIDATE = "pre-existing-fastalpr-reference-20260817"
REFERENCE_VIDEO_CANDIDATE = "pre-existing-yolov8-sort-easyocr-reference-20260817"
EXPECTED_IMAGE_SOURCE_SHA256 = "6a700f949cf408e2a1ebd8d7b737317d0432e19e91958ddc8c6c3bda4ffc920b"
EXPECTED_HISTORICAL_IMAGE_CSV_SHA256 = "56ad58d2c4a8564f7f9430ea7b42a0091db8687faeda49e3b53c4b118eb54268"
EXPECTED_VIDEO_SOURCE_SHA256 = "e4e6ad1e7ba0ff100e3cc508b05d93a1f95f03f5bc3d1d50c09807c2b1f91a65"
EXPECTED_VIDEO_UTIL_SHA256 = "de12e427d925188b9f9fb75903588047a5adc8c4c3eebc20ec6472b0c3477f5d"
EXPECTED_VIDEO_CSV_SHA256 = "40be951dc3093d066c81d442d77953f1fd71e03ef73686c24053969c4f836107"
EXPECTED_VIDEO_INTERPOLATED_SHA256 = "456a6fdeca4845be912d80ee1340ac18c5ada5d5009f51b8e5e66fbf450f1af4"
EXPECTED_VIDEO_SHA256 = baseline.EXPECTED_VIDEO_SHA256
EXPECTED_MODEL_HASHES = {
    "detector": "888397b96d761c89db40bc9c305838e8652660f5e282c2cadebbe8d2951a77a8",
    "ocr": "8031afb5fdc6b4d80462c9d542f1284ebd2cfddf5dbacd62609848d7e2855f44",
    "ocr_config": "0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6",
}


def package_version(name: str) -> str:
    return importlib.metadata.version(name)


def image_candidate_configuration(reference_source: Path) -> dict[str, Any]:
    cache_root = Path.home() / ".cache"
    paths = {
        "detector": cache_root / "open-image-models" / "yolo-v9-t-384-license-plate-end2end" /
        "yolo-v9-t-384-license-plates-end2end.onnx",
        "ocr": cache_root / "fast-plate-ocr" / "cct-xs-v2-global-model" / "cct_xs_v2_global.onnx",
        "ocr_config": cache_root / "fast-plate-ocr" / "cct-xs-v2-global-model" /
        "cct_xs_v2_global_plate_config.yaml",
    }
    for role, path in paths.items():
        if not path.is_file() or baseline.hash_file(path) != EXPECTED_MODEL_HASHES[role]:
            raise RuntimeError(f"frozen image reference {role} cache integrity failed")
    if baseline.hash_file(reference_source) != EXPECTED_IMAGE_SOURCE_SHA256:
        raise RuntimeError("frozen image reference source integrity failed")
    configuration = {
        "candidate_id": REFERENCE_IMAGE_CANDIDATE,
        "reference_source_sha256": EXPECTED_IMAGE_SOURCE_SHA256,
        "packages": {
            name: package_version(name)
            for name in (
                "fast-alpr", "fast-plate-ocr", "open-image-models", "onnxruntime-openvino",
                "openvino", "opencv-python-headless", "numpy",
            )
        },
        "model_hashes": EXPECTED_MODEL_HASHES,
        "detector_model": "yolo-v9-t-384-license-plate-end2end",
        "ocr_model": "cct-xs-v2-global-model",
        "detector_internal_threshold": 0.4,
        "historical_post_filter_threshold": 0.75,
        "providers_requested": None,
        "normalization": "uppercase and retain A-Z/0-9 only",
        "crop": "integer detector box clipped to source bounds; no padding",
        "batch_size": 1,
    }
    encoded = json.dumps(configuration, sort_keys=True, separators=(",", ":")).encode()
    configuration["configuration_sha256"] = hashlib.sha256(encoded).hexdigest()
    configuration["cache_paths"] = {role: str(path) for role, path in paths.items()}
    return configuration


def windows_peak_rss_mib() -> float | None:
    if os.name != "nt":
        try:
            import resource
            return round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024, 3)
        except (ImportError, AttributeError):
            return None

    class ProcessMemoryCounters(ctypes.Structure):
        _fields_ = [
            ("cb", ctypes.c_ulong), ("PageFaultCount", ctypes.c_ulong),
            ("PeakWorkingSetSize", ctypes.c_size_t), ("WorkingSetSize", ctypes.c_size_t),
            ("QuotaPeakPagedPoolUsage", ctypes.c_size_t), ("QuotaPagedPoolUsage", ctypes.c_size_t),
            ("QuotaPeakNonPagedPoolUsage", ctypes.c_size_t), ("QuotaNonPagedPoolUsage", ctypes.c_size_t),
            ("PagefileUsage", ctypes.c_size_t), ("PeakPagefileUsage", ctypes.c_size_t),
        ]

    counters = ProcessMemoryCounters()
    counters.cb = ctypes.sizeof(counters)
    handle = ctypes.windll.kernel32.GetCurrentProcess()
    from ctypes import wintypes
    get_memory = ctypes.WinDLL("kernel32", use_last_error=True).K32GetProcessMemoryInfo
    get_memory.argtypes = [wintypes.HANDLE, ctypes.POINTER(ProcessMemoryCounters), wintypes.DWORD]
    get_memory.restype = wintypes.BOOL
    if not get_memory(handle, ctypes.byref(counters), counters.cb):
        return None
    return round(counters.PeakWorkingSetSize / 1024 / 1024, 3)


def actual_providers(alpr: Any) -> dict[str, list[str] | None]:
    detector_session = getattr(getattr(alpr.detector, "detector", None), "model", None)
    if detector_session is None:
        detector_session = getattr(alpr.detector, "model", None)
    ocr_session = getattr(getattr(alpr.ocr, "ocr_model", None), "model", None)
    return {
        "detector": detector_session.get_providers() if hasattr(detector_session, "get_providers") else None,
        "ocr": ocr_session.get_providers() if hasattr(ocr_session, "get_providers") else None,
    }


def result_details(results: list[Any]) -> tuple[list[str], list[dict[str, Any]]]:
    accepted = [item for item in results if float(item.detection.confidence) >= 0.75]
    raw = [str(item.ocr.text or "") for item in accepted if item.ocr is not None]
    details = []
    for item in accepted:
        bbox = item.detection.bounding_box
        text = str(item.ocr.text or "") if item.ocr is not None else ""
        confidence = item.ocr.confidence if item.ocr is not None else None
        if isinstance(confidence, list):
            confidence = statistics.fmean(float(value) for value in confidence) if confidence else None
        details.append({
            "raw_plate_text": text,
            "normalized_plate_text": baseline.normalize_plate(text),
            "detection_confidence": float(item.detection.confidence),
            "ocr_confidence": float(confidence) if confidence is not None else None,
            "bbox": [int(bbox.x1), int(bbox.y1), int(bbox.x2), int(bbox.y2)],
        })
    return raw, details


def load_image_labels(oracle_root: Path, split: str) -> tuple[dict[str, Any], list[dict[str, str]]]:
    validation, state = baseline.load_oracle(oracle_root)
    with (oracle_root / "image-labels.csv").open("r", encoding="utf-8-sig", newline="") as source:
        rows = [row for row in csv.DictReader(source) if row["split"] == split]
    expected = 10 if split == "development" else 22
    if len(rows) != expected:
        raise RuntimeError(f"{split} oracle count is not {expected}")
    return {"validation": validation, "state": state}, rows


def compare_current_receipt(rows: list[dict[str, Any]], receipt_path: Path | None) -> dict[str, Any] | None:
    if receipt_path is None:
        return None
    receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
    current = {row["sample_id"]: row for row in receipt["results"]}
    agreements = 0
    for row in rows:
        current_values = sorted(
            item["normalized_plate_text"] for item in current[row["sample_id"]]["predictions"]
            if item["normalized_plate_text"]
        )
        reference_values = sorted(
            item["normalized_plate_text"] for item in row["predictions"] if item["normalized_plate_text"]
        )
        agreements += current_values == reference_values
    return {
        "current_receipt_sha256": baseline.hash_file(receipt_path),
        "evaluated_samples": len(rows),
        "exact_prediction_multiset_agreements": agreements,
        "agreement_rate": baseline.safe_div(agreements, len(rows)),
    }


def run_image_reference(args: argparse.Namespace) -> dict[str, Any]:
    oracle, labels = load_image_labels(args.oracle_root, args.split)
    configuration = image_candidate_configuration(args.reference_source)
    if args.split == "sealed_holdout":
        if args.development_receipt is None or not args.development_receipt.is_file():
            raise RuntimeError("reference holdout requires its frozen development receipt")
        development = json.loads(args.development_receipt.read_text(encoding="utf-8"))
        if (
            development.get("candidate", {}).get("configuration_sha256") != configuration["configuration_sha256"]
            or development.get("split") != "development"
            or development.get("configuration_changed_after_development") is not False
        ):
            raise RuntimeError("reference candidate configuration is not frozen to development")

    from fast_alpr import ALPR
    import cv2

    model_started = time.perf_counter()
    alpr = ALPR(
        detector_model="yolo-v9-t-384-license-plate-end2end",
        ocr_model="cct-xs-v2-global-model",
    )
    model_load_seconds = time.perf_counter() - model_started
    configuration["providers_actual"] = actual_providers(alpr)
    started_wall, started_cpu = time.perf_counter(), time.process_time()
    rows = []
    args.diagnostic_dir.mkdir(parents=True, exist_ok=True)
    for index, label in enumerate(labels):
        path = (args.image_root / label["relative_file"]).resolve()
        if not path.is_relative_to(args.image_root.resolve()) or baseline.hash_file(path) != label["sha256"]:
            raise RuntimeError(f"image source integrity failed: {label['sample_id']}")
        frame = cv2.imread(str(path))
        if frame is None:
            raise RuntimeError(f"image decode failed: {label['sample_id']}")
        started = time.perf_counter()
        drawn = alpr.draw_predictions(frame)
        latency = time.perf_counter() - started
        predicted_raw, details = result_details(list(drawn.results))
        truth_raw = baseline.plate_lines(label["plate_text"]) if label["plate_presence"] == "yes" else []
        score = baseline.score_plate_lists(truth_raw, predicted_raw)
        if index < 4 or score["exact_tp"] != score["truth_count"] or score["exact_fp"]:
            diagnostic_path = args.diagnostic_dir / f"{args.split}-{label['sample_id']}-reference.jpg"
            cv2.imwrite(str(diagnostic_path), drawn.image)
        rows.append({
            "sample_id": label["sample_id"], "source_sha256": label["sha256"],
            "split": args.split, "readability": label["readability"], "difficulty": label["difficulty"],
            "truth_raw": truth_raw, "predictions": details, "score": score,
            "latency_seconds": round(latency, 6),
            "error_class": (
                "miss" if score["prediction_count"] == 0
                else "exact" if score["exact_tp"] == score["truth_count"] and score["exact_fp"] == 0
                else "ocr_or_count_error"
            ),
        })
    return {
        "contract_version": CONTRACT, "benchmark": "reference_fastalpr_image", "split": args.split,
        "evaluated_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "ground_truth": {"status": oracle["validation"]["status"], "gold_digest": oracle["state"]["gold_digest"]},
        "candidate": configuration, "configuration_changed_after_development": False,
        "summary": baseline.aggregate(rows, unit="image"),
        "comparison_to_current_nxmmr": compare_current_receipt(rows, args.current_receipt),
        "resource": {
            "model_load_seconds": round(model_load_seconds, 6),
            "benchmark_wall_seconds": round(time.perf_counter() - started_wall, 6),
            "process_cpu_seconds": round(time.process_time() - started_cpu, 6),
            "process_peak_rss_mib": windows_peak_rss_mib(),
            "parallel_heavy_models": 1,
        },
        "diagnostic_directory": str(args.diagnostic_dir),
        "results": rows,
    }


def run_historical_images(args: argparse.Namespace) -> dict[str, Any]:
    if baseline.hash_file(args.csv) != EXPECTED_HISTORICAL_IMAGE_CSV_SHA256:
        raise RuntimeError("historical image CSV integrity failed")
    baseline.load_oracle(args.oracle_root)
    with args.csv.open("r", encoding="utf-8-sig", newline="") as source:
        by_file: dict[str, list[dict[str, str]]] = defaultdict(list)
        for row in csv.DictReader(source):
            by_file[row["filename"]].append(row)
    with (args.oracle_root / "image-labels.csv").open("r", encoding="utf-8-sig", newline="") as source:
        labels = list(csv.DictReader(source))
    rows = []
    for label in labels:
        filename = Path(label["relative_file"]).name
        if filename not in by_file:
            continue
        predicted_raw = [
            row.get("predicted_plate") or row.get("plate_text") or ""
            for row in by_file[filename]
            if row.get("predicted_plate") or row.get("plate_text")
        ]
        truth_raw = baseline.plate_lines(label["plate_text"]) if label["plate_presence"] == "yes" else []
        score = baseline.score_plate_lists(truth_raw, predicted_raw)
        rows.append({
            "sample_id": label["sample_id"], "split": label["split"], "truth_raw": truth_raw,
            "prediction_raw": predicted_raw, "score": score, "latency_seconds": 0,
        })
    summaries = {}
    for split in ("development", "sealed_holdout"):
        selected = [row for row in rows if row["split"] == split]
        summary = baseline.aggregate(selected, unit="historical_overlap_image") if selected else None
        if summary:
            summary["latency_seconds"] = None
        summaries[split] = summary
    return {
        "contract_version": CONTRACT, "benchmark": "historical_fastalpr_image_output_overlap",
        "candidate_id": REFERENCE_IMAGE_CANDIDATE, "source_csv_sha256": EXPECTED_HISTORICAL_IMAGE_CSV_SHA256,
        "ground_truth_gold_digest": baseline.EXPECTED_GOLD_DIGEST, "summary_by_split": summaries,
        "resource": "not recoverable from historical CSV", "results": rows,
    }


def load_video_frames(rows: list[dict[str, str]], *, fps: float = 60.0) -> list[dict[str, Any]]:
    by_frame: dict[int, list[str]] = defaultdict(list)
    for row in rows:
        value = row.get("license_number", "")
        if value and value != "0":
            by_frame[int(float(row["frame_nmr"]))].append(value)
    return [
        {"timestamp_seconds": frame / fps, "predicted_raw": by_frame.get(frame, []), "latency_seconds": 0}
        for frame in range(3600)
    ]


def selected_track_rows(raw_rows: list[dict[str, str]], interpolated_rows: list[dict[str, str]]) -> list[dict[str, str]]:
    candidates: dict[int, tuple[float, str]] = {}
    for row in raw_rows:
        score = float(row["license_number_score"])
        car_id = int(float(row["car_id"]))
        if car_id not in candidates or score > candidates[car_id][0]:
            candidates[car_id] = (score, row["license_number"])
    selected = []
    for row in interpolated_rows:
        car_id = int(float(row["car_id"]))
        if car_id in candidates:
            selected.append({**row, "license_number": candidates[car_id][1]})
    return selected


def frame_presence_summary(events: list[dict[str, Any]], frames: list[dict[str, Any]]) -> dict[str, Any]:
    positive = [frame for frame in frames if any(baseline.contains(event, frame["timestamp_seconds"]) for event in events)]
    negative = [frame for frame in frames if frame not in positive]
    tp = sum(bool(frame["predicted_raw"]) for frame in positive)
    fp = sum(bool(frame["predicted_raw"]) for frame in negative)
    precision = baseline.safe_div(tp, tp + fp)
    recall = baseline.safe_div(tp, len(positive))
    return {
        "positive_frames": len(positive), "negative_frames": len(negative),
        "tp": tp, "fp": fp, "fn": len(positive) - tp, "tn": len(negative) - fp,
        "precision": precision, "recall": recall,
        "f1": baseline.f1(precision, recall),
        "false_positive_rate": baseline.safe_div(fp, len(negative)),
        "specificity": baseline.safe_div(len(negative) - fp, len(negative)),
    }


def compact_video_score(events: list[dict[str, Any]], frames: list[dict[str, Any]]) -> dict[str, Any]:
    scored = baseline.score_video(events, frames)
    scored["event_summary"]["latency_seconds"] = None
    return {
        "events": scored["event_summary"], "grouping": scored["grouping"],
        "frame_presence": frame_presence_summary(events, frames),
        "private_event_results": scored["event_results"],
    }


def run_historical_video(args: argparse.Namespace) -> dict[str, Any]:
    validation, state = baseline.load_oracle(args.oracle_root)
    expected = {
        args.reference_source: EXPECTED_VIDEO_SOURCE_SHA256,
        args.reference_util: EXPECTED_VIDEO_UTIL_SHA256,
        args.csv: EXPECTED_VIDEO_CSV_SHA256,
        args.interpolated_csv: EXPECTED_VIDEO_INTERPOLATED_SHA256,
        args.video: EXPECTED_VIDEO_SHA256,
    }
    for path, digest in expected.items():
        if baseline.hash_file(path) != digest:
            raise RuntimeError(f"historical video artifact integrity failed: {path.name}")
    with args.csv.open("r", encoding="utf-8-sig", newline="") as source:
        raw_rows = list(csv.DictReader(source))
    with args.interpolated_csv.open("r", encoding="utf-8-sig", newline="") as source:
        interpolated_rows = list(csv.DictReader(source))
    final_rows = selected_track_rows(raw_rows, interpolated_rows)
    events = [event for event in state["video_events"] if event["event_type"] == "plate"]
    raw_frames = load_video_frames(raw_rows)
    final_frames = load_video_frames(final_rows)
    return {
        "contract_version": CONTRACT, "benchmark": "historical_yolov8_sort_easyocr_video_output",
        "evaluated_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "candidate_id": REFERENCE_VIDEO_CANDIDATE,
        "ground_truth": {"status": validation["status"], "gold_digest": state["gold_digest"]},
        "source": {"sha256": EXPECTED_VIDEO_SHA256, "frames": 3600, "fps": 60.0, "input_parity": "EXACT"},
        "artifact_hashes": {
            "main.py": EXPECTED_VIDEO_SOURCE_SHA256, "util.py": EXPECTED_VIDEO_UTIL_SHA256,
            "test.csv": EXPECTED_VIDEO_CSV_SHA256, "test_interpolated.csv": EXPECTED_VIDEO_INTERPOLATED_SHA256,
        },
        "historical_output": {
            "raw_rows": len(raw_rows), "raw_unique_frames": len({row["frame_nmr"] for row in raw_rows}),
            "interpolated_rows": len(interpolated_rows),
            "interpolated_unique_frames": len({row["frame_nmr"] for row in interpolated_rows}),
            "track_ids": len({row["car_id"] for row in raw_rows}),
        },
        "raw_observation_score": compact_video_score(events, raw_frames),
        "final_visualization_track_candidate_score": compact_video_score(events, final_frames),
        "resource": {
            "exact_runtime_measurement": None,
            "reason": "historical CSV/video contain no runtime telemetry; full rerun is separately resource-gated",
        },
        "private_detected_frames": {
            "raw": [frame for frame in raw_frames if frame["predicted_raw"]],
            "final": [frame for frame in final_frames if frame["predicted_raw"]],
        },
    }


def write_report(path: Path, report: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.exists():
        raise RuntimeError(f"refusing to overwrite parity receipt: {path}")
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temporary, path)
    print(json.dumps({
        "output": str(path), "benchmark": report["benchmark"],
        "split": report.get("split"),
        "summary": report.get("summary") or report.get("summary_by_split") or {
            "raw": report.get("raw_observation_score", {}).get("events"),
            "final": report.get("final_visualization_track_candidate_score", {}).get("events"),
        },
        "resource": report.get("resource"),
    }, ensure_ascii=False))


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    image = subparsers.add_parser("image-reference")
    image.add_argument("--oracle-root", type=Path, required=True)
    image.add_argument("--image-root", type=Path, required=True)
    image.add_argument("--reference-source", type=Path, required=True)
    image.add_argument("--split", choices=("development", "sealed_holdout"), required=True)
    image.add_argument("--development-receipt", type=Path)
    image.add_argument("--current-receipt", type=Path)
    image.add_argument("--diagnostic-dir", type=Path, required=True)
    image.add_argument("--output", type=Path, required=True)
    historical_image = subparsers.add_parser("image-historical")
    historical_image.add_argument("--oracle-root", type=Path, required=True)
    historical_image.add_argument("--csv", type=Path, required=True)
    historical_image.add_argument("--output", type=Path, required=True)
    video = subparsers.add_parser("video-historical")
    video.add_argument("--oracle-root", type=Path, required=True)
    video.add_argument("--video", type=Path, required=True)
    video.add_argument("--reference-source", type=Path, required=True)
    video.add_argument("--reference-util", type=Path, required=True)
    video.add_argument("--csv", type=Path, required=True)
    video.add_argument("--interpolated-csv", type=Path, required=True)
    video.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.command == "image-reference":
        report = run_image_reference(args)
    elif args.command == "image-historical":
        report = run_historical_images(args)
    else:
        report = run_historical_video(args)
    write_report(args.output, report)


if __name__ == "__main__":
    main()
