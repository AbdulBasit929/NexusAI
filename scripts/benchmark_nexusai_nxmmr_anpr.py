#!/usr/bin/env python3
"""Score the incumbent FastALPR against the locked private NX-MMR human oracle."""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import math
import os
import re
import statistics
import subprocess
import tempfile
import time
from collections import Counter
from pathlib import Path
from typing import Any

try:
    import resource
except ModuleNotFoundError:  # pragma: no cover - Windows development host
    resource = None  # type: ignore[assignment]


CONTRACT = "nexusai.nxmmr.incumbent-anpr-benchmark/v1"
EXPECTED_GOLD_DIGEST = "88c6827c23fbed467f097843404ed09c947bf7f27432e5cb5fa8427be9f48567"
EXPECTED_VIDEO_SHA256 = "d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee"
VIDEO_INTERVAL_SECONDS = 1.0
MODEL_THRESHOLD = 0.75


def make_processor() -> Any:
    try:
        from media_pipeline import FastALPRImageProcessor
    except ModuleNotFoundError:
        from ingestion.forensic_records.media_pipeline import FastALPRImageProcessor
    processor = FastALPRImageProcessor.from_environment()
    if not math.isclose(float(processor.minimum_confidence), MODEL_THRESHOLD):
        raise RuntimeError("incumbent detection threshold differs from the frozen benchmark configuration")
    return processor


def hash_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def normalize_plate(value: str) -> str:
    return "".join(character for character in value.upper() if character.isalnum())


def raw_plate(value: str) -> str:
    return " ".join(value.upper().split())


def plate_lines(value: str) -> list[str]:
    return [line.strip() for line in value.splitlines() if line.strip()]


def safe_div(numerator: int | float, denominator: int | float) -> float | None:
    return round(float(numerator) / float(denominator), 6) if denominator else None


def f1(precision: float | None, recall: float | None) -> float | None:
    if precision is None or recall is None or precision + recall == 0:
        return 0.0 if precision is not None and recall is not None else None
    return round(2 * precision * recall / (precision + recall), 6)


def percentile(values: list[float], fraction: float) -> float | None:
    if not values:
        return None
    ordered = sorted(values)
    position = (len(ordered) - 1) * fraction
    lower = math.floor(position)
    upper = math.ceil(position)
    result = ordered[lower] if lower == upper else ordered[lower] + (ordered[upper] - ordered[lower]) * (position - lower)
    return round(result, 6)


def wilson(successes: int, total: int, z: float = 1.959963984540054) -> list[float] | None:
    if total <= 0:
        return None
    observed = successes / total
    denominator = 1 + z * z / total
    center = (observed + z * z / (2 * total)) / denominator
    radius = z * math.sqrt(observed * (1 - observed) / total + z * z / (4 * total * total)) / denominator
    return [round(max(0, center - radius), 6), round(min(1, center + radius), 6)]


def edit_distance(left: str, right: str) -> int:
    prior = list(range(len(right) + 1))
    for left_index, left_character in enumerate(left, start=1):
        current = [left_index]
        for right_index, right_character in enumerate(right, start=1):
            current.append(min(
                current[-1] + 1,
                prior[right_index] + 1,
                prior[right_index - 1] + (left_character != right_character),
            ))
        prior = current
    return prior[-1]


def greedy_pairs(truth: list[str], predictions: list[str]) -> tuple[list[tuple[str, str]], list[str], list[str]]:
    remaining_truth = list(truth)
    remaining_predictions = list(predictions)
    pairs: list[tuple[str, str]] = []
    while remaining_truth and remaining_predictions:
        best = min(
            (edit_distance(expected, actual), expected_index, actual_index)
            for expected_index, expected in enumerate(remaining_truth)
            for actual_index, actual in enumerate(remaining_predictions)
        )
        _, expected_index, actual_index = best
        pairs.append((remaining_truth.pop(expected_index), remaining_predictions.pop(actual_index)))
    return pairs, remaining_truth, remaining_predictions


def score_plate_lists(truth_raw: list[str], prediction_raw: list[str]) -> dict[str, Any]:
    truth = [normalize_plate(value) for value in truth_raw]
    predictions = [normalize_plate(value) for value in prediction_raw if normalize_plate(value)]
    exact = sum((Counter(truth) & Counter(predictions)).values())
    detection_tp = min(len(truth), len(predictions))
    pairs, missing, extra = greedy_pairs(truth, predictions)
    char_errors = sum(edit_distance(expected, actual) for expected, actual in pairs)
    char_errors += sum(len(expected) for expected in missing)
    return {
        "truth_count": len(truth), "prediction_count": len(predictions),
        "detection_tp": detection_tp, "detection_fp": max(0, len(predictions) - len(truth)),
        "detection_fn": max(0, len(truth) - len(predictions)),
        "exact_tp": exact, "exact_fp": max(0, len(predictions) - exact),
        "exact_fn": max(0, len(truth) - exact),
        "raw_exact_tp": sum((Counter(map(raw_plate, truth_raw)) & Counter(map(raw_plate, prediction_raw))).values()),
        "character_errors": char_errors, "reference_characters": sum(len(value) for value in truth),
        "unmatched_truth": missing, "unmatched_predictions": extra,
    }


def aggregate(rows: list[dict[str, Any]], *, unit: str) -> dict[str, Any]:
    totals = {
        key: sum(int(row["score"][key]) for row in rows)
        for key in (
            "truth_count", "prediction_count", "detection_tp", "detection_fp", "detection_fn",
            "exact_tp", "exact_fp", "exact_fn", "raw_exact_tp", "character_errors", "reference_characters",
        )
    }
    presence_tp = sum(row["score"]["truth_count"] > 0 and row["score"]["prediction_count"] > 0 for row in rows)
    presence_fn = sum(row["score"]["truth_count"] > 0 and row["score"]["prediction_count"] == 0 for row in rows)
    presence_fp = sum(row["score"]["truth_count"] == 0 and row["score"]["prediction_count"] > 0 for row in rows)
    presence_tn = sum(row["score"]["truth_count"] == 0 and row["score"]["prediction_count"] == 0 for row in rows)
    detection_precision = safe_div(totals["detection_tp"], totals["detection_tp"] + totals["detection_fp"])
    detection_recall = safe_div(totals["detection_tp"], totals["detection_tp"] + totals["detection_fn"])
    exact_precision = safe_div(totals["exact_tp"], totals["exact_tp"] + totals["exact_fp"])
    exact_recall = safe_div(totals["exact_tp"], totals["exact_tp"] + totals["exact_fn"])
    latencies = [float(row["latency_seconds"]) for row in rows]
    return {
        "unit": unit, "evaluated_units": len(rows),
        "positive_units": sum(row["score"]["truth_count"] > 0 for row in rows),
        "negative_units": sum(row["score"]["truth_count"] == 0 for row in rows),
        "ground_truth_plates": totals["truth_count"], "predicted_detections": totals["prediction_count"],
        "presence_confusion": {"tp": presence_tp, "fp": presence_fp, "fn": presence_fn, "tn": presence_tn},
        "presence_recall": safe_div(presence_tp, presence_tp + presence_fn),
        "presence_recall_wilson_95": wilson(presence_tp, presence_tp + presence_fn),
        "presence_specificity": safe_div(presence_tn, presence_tn + presence_fp),
        "count_proxy_detection": {
            "tp": totals["detection_tp"], "fp": totals["detection_fp"], "fn": totals["detection_fn"],
            "precision": detection_precision, "recall": detection_recall,
            "recall_wilson_95": wilson(totals["detection_tp"], totals["detection_tp"] + totals["detection_fn"]),
            "f1": f1(detection_precision, detection_recall),
        },
        "normalized_exact_plate": {
            "tp": totals["exact_tp"], "fp": totals["exact_fp"], "fn": totals["exact_fn"],
            "precision": exact_precision, "recall": exact_recall,
            "accuracy_per_truth_plate": safe_div(totals["exact_tp"], totals["truth_count"]),
            "accuracy_wilson_95": wilson(totals["exact_tp"], totals["truth_count"]),
            "f1": f1(exact_precision, exact_recall),
        },
        "raw_exact_plate_accuracy": safe_div(totals["raw_exact_tp"], totals["truth_count"]),
        "normalized_cer": safe_div(totals["character_errors"], totals["reference_characters"]),
        "latency_seconds": {
            "mean": round(statistics.fmean(latencies), 6) if latencies else None,
            "p50": percentile(latencies, 0.5), "p95": percentile(latencies, 0.95),
            "maximum": round(max(latencies), 6) if latencies else None,
        },
    }


def load_oracle(oracle_root: Path) -> tuple[dict[str, Any], dict[str, Any]]:
    validation = json.loads((oracle_root / "ground-truth-validation.json").read_text(encoding="utf-8"))
    state = json.loads((oracle_root / "review-state.json").read_text(encoding="utf-8"))
    if validation.get("GroundTruthValidation") != "PASS" or validation.get("status") != "PASS" or validation.get("errors"):
        raise RuntimeError("GroundTruthValidation is not PASS")
    if not state.get("locked") or state.get("gold_digest") != validation.get("gold_digest"):
        raise RuntimeError("ground truth is not locked to the validation digest")
    if state.get("gold_digest") != EXPECTED_GOLD_DIGEST:
        raise RuntimeError("ground-truth digest differs from the authorized locked pack")
    if validation.get("model_output_used_to_create_truth") is not False:
        raise RuntimeError("oracle independence receipt is invalid")
    return validation, state


def observation_values(result: Any) -> tuple[list[str], list[dict[str, Any]]]:
    observations = list(result.observations)
    raw = [str(item.payload.get("raw_plate_text") or "") for item in observations]
    details = [{
        "raw_plate_text": value,
        "normalized_plate_text": normalize_plate(value),
        "detection_confidence": item.payload.get("detection_confidence"),
        "ocr_confidence": item.payload.get("ocr_confidence"),
        "bbox": item.citation_locator.get("bbox"),
    } for item, value in zip(observations, raw)]
    return raw, details


def process_image(processor: Any, path: Path, identity: str, source_sha256: str, **locator: Any) -> tuple[Any, float]:
    started = time.perf_counter()
    result = processor.process_image(
        path, evidence_id=f"private-nxmmr-{identity}", version_id="locked-gold-v1",
        source_sha256=source_sha256, source_file=path.name,
        locator_prefix=locator or None,
    )
    return result, time.perf_counter() - started


def resource_receipt(started_wall: float, started_cpu: float) -> dict[str, Any]:
    usage = resource.getrusage(resource.RUSAGE_SELF) if resource is not None else None
    peak_rss_kib = int(usage.ru_maxrss) if usage is not None else None
    return {
        "wall_seconds": round(time.perf_counter() - started_wall, 6),
        "process_cpu_seconds": round(time.process_time() - started_cpu, 6),
        "peak_rss_kib": peak_rss_kib,
        "peak_rss_mib": round(float(peak_rss_kib) / 1024, 3) if peak_rss_kib is not None else None,
        "parallel_heavy_models": 1,
        "execution_provider": os.getenv("FORENSIC_ANPR_ONNX_PROVIDERS", ""),
        "cpu_limit": os.getenv("NXMMR_CPU_LIMIT", "not_reported"),
        "memory_limit": os.getenv("NXMMR_MEMORY_LIMIT", "not_reported"),
    }


def run_images(args: argparse.Namespace) -> dict[str, Any]:
    validation, state = load_oracle(args.oracle_root)
    if args.split == "sealed_holdout":
        if not args.development_receipt or not args.development_receipt.is_file():
            raise RuntimeError("sealed holdout requires the completed development receipt")
        development = json.loads(args.development_receipt.read_text(encoding="utf-8"))
        if development.get("split") != "development" or development.get("ground_truth", {}).get("gold_digest") != state["gold_digest"]:
            raise RuntimeError("development receipt does not bind to this locked gold pack")
        if development.get("configuration_changed_after_development") is not False:
            raise RuntimeError("development receipt indicates a configuration change")
    with (args.oracle_root / "image-labels.csv").open("r", encoding="utf-8-sig", newline="") as source:
        labels = [row for row in csv.DictReader(source) if row["split"] == args.split]
    expected = 10 if args.split == "development" else 22
    if len(labels) != expected:
        raise RuntimeError(f"{args.split} must contain exactly {expected} labels")
    processor = make_processor()
    started_wall, started_cpu = time.perf_counter(), time.process_time()
    rows = []
    for label in labels:
        path = (args.image_root / label["relative_file"]).resolve()
        if not path.is_relative_to(args.image_root.resolve()):
            raise RuntimeError(f"source escapes image root: {label['sample_id']}")
        if not path.is_file() or hash_file(path) != label["sha256"]:
            raise RuntimeError(f"source integrity mismatch: {label['sample_id']}")
        result, latency = process_image(processor, path, label["sample_id"], label["sha256"])
        predicted_raw, details = observation_values(result)
        truth_raw = plate_lines(label["plate_text"]) if label["plate_presence"] == "yes" else []
        score = score_plate_lists(truth_raw, predicted_raw)
        rows.append({
            "sample_id": label["sample_id"], "source_sha256": label["sha256"], "split": args.split,
            "readability": label["readability"], "difficulty": label["difficulty"],
            "truth_raw": truth_raw, "predictions": details, "score": score,
            "latency_seconds": round(latency, 6),
            "error_class": (
                "miss" if score["prediction_count"] == 0
                else "exact" if score["exact_tp"] == score["truth_count"] and score["exact_fp"] == 0
                else "ocr_or_count_error"
            ),
        })
    report = {
        "contract_version": CONTRACT, "benchmark": "pakistan_image_anpr", "split": args.split,
        "evaluated_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "ground_truth": {"status": validation["status"], "gold_digest": state["gold_digest"], "independent_human": True},
        "processor": {
            "id": processor.processor_id, "revision": processor.processor_revision,
            "detector": processor.detector_id, "ocr": processor.ocr_id,
            "minimum_detection_confidence": MODEL_THRESHOLD,
        },
        "configuration_changed_after_development": False,
        "summary": aggregate(rows, unit="image"),
        "resource": resource_receipt(started_wall, started_cpu),
        "limitations": [
            "all selected images are human-labeled positives, so image-only specificity is not estimable",
            "the oracle has counts and text but no boxes; detection precision/recall is a count proxy, not IoU localization accuracy",
            "small private target-domain sample; do not generalize as universal Pakistan accuracy",
        ],
        "results": rows,
    }
    return report


def probe_video(path: Path) -> tuple[float, float]:
    completed = subprocess.run(
        ["ffprobe", "-v", "error", "-show_format", "-show_streams", "-of", "json", str(path)],
        check=True, capture_output=True, text=True, timeout=30,
    )
    metadata = json.loads(completed.stdout)
    duration = float(metadata["format"]["duration"])
    stream = next(item for item in metadata["streams"] if item.get("codec_type") == "video")
    numerator, denominator = str(stream["avg_frame_rate"]).split("/", 1)
    return duration, float(numerator) / float(denominator)


def extract_video_frames(video: Path, directory: Path, duration: float) -> list[tuple[Path, float]]:
    directory.mkdir(parents=True, exist_ok=True)
    pattern = directory / "frame-%04d.jpg"
    count = math.ceil(duration / VIDEO_INTERVAL_SECONDS)
    completed = subprocess.run([
        "ffmpeg", "-nostdin", "-v", "info", "-i", str(video),
        "-vf", f"select=isnan(prev_selected_t)+gte(t-prev_selected_t\\,{VIDEO_INTERVAL_SECONDS}),showinfo",
        "-frames:v", str(count), "-fps_mode", "vfr", "-q:v", "3", str(pattern),
    ], check=True, capture_output=True, text=True, timeout=300)
    timestamps = [float(value) for value in re.findall(r"Parsed_showinfo[^\r\n]*\bpts_time:([0-9]+(?:\.[0-9]+)?)", completed.stderr)]
    paths = sorted(directory.glob("frame-*.jpg"))
    if len(paths) != len(timestamps):
        raise RuntimeError(f"sampled {len(paths)} frames but parsed {len(timestamps)} timestamps")
    return list(zip(paths, timestamps))


def event_truth(event: dict[str, Any]) -> list[str]:
    return plate_lines(str(event.get("plate_text", ""))) if event.get("event_type") == "plate" else []


def contains(event: dict[str, Any], timestamp: float) -> bool:
    return float(event["start_seconds"]) <= timestamp <= float(event["end_seconds"])


def score_video(events: list[dict[str, Any]], frames: list[dict[str, Any]]) -> dict[str, Any]:
    event_rows = []
    for event in events:
        truth = event_truth(event)
        sampled = [frame for frame in frames if contains(event, float(frame["timestamp_seconds"]))]
        predictions = [value for frame in sampled for value in frame["predicted_raw"]]
        best_detection_count = max((len(frame["predicted_raw"]) for frame in sampled), default=0)
        exact_predictions = list({normalize_plate(value): value for value in predictions if normalize_plate(value)}.values())
        score = score_plate_lists(truth, exact_predictions)
        score["detection_tp"] = min(len(truth), best_detection_count)
        score["detection_fp"] = max(0, best_detection_count - len(truth))
        score["detection_fn"] = max(0, len(truth) - best_detection_count)
        event_rows.append({
            "event_id": event["event_id"], "start_seconds": float(event["start_seconds"]),
            "end_seconds": float(event["end_seconds"]), "readability": event["readability"],
            "truth_raw": truth, "sampled_frame_count": len(sampled), "prediction_raw": predictions,
            "latency_seconds": sum(float(frame["latency_seconds"]) for frame in sampled), "score": score,
        })
    negative_frames = [frame for frame in frames if not any(contains(event, float(frame["timestamp_seconds"])) for event in events)]
    negative_fp_frames = [frame for frame in negative_frames if frame["predicted_raw"]]
    event_summary = aggregate(event_rows, unit="human_plate_event")
    event_summary["event_detection_recall"] = safe_div(
        sum(row["score"]["prediction_count"] > 0 for row in event_rows), len(event_rows)
    )
    event_summary["event_detection_recall_wilson_95"] = wilson(
        sum(row["score"]["prediction_count"] > 0 for row in event_rows), len(event_rows)
    )
    event_summary["negative_sampled_frames"] = len(negative_frames)
    event_summary["false_positive_negative_frames"] = len(negative_fp_frames)
    event_summary["negative_frame_false_positive_rate"] = safe_div(len(negative_fp_frames), len(negative_frames))

    truth_groups: dict[str, dict[str, float]] = {}
    for event in events:
        for value in event_truth(event):
            plate = normalize_plate(value)
            group = truth_groups.setdefault(plate, {"first": float(event["start_seconds"]), "last": float(event["end_seconds"])})
            group["first"] = min(group["first"], float(event["start_seconds"]))
            group["last"] = max(group["last"], float(event["end_seconds"]))
    prediction_groups: dict[str, list[float]] = {}
    for frame in frames:
        for value in frame["predicted_raw"]:
            plate = normalize_plate(value)
            if plate:
                prediction_groups.setdefault(plate, []).append(float(frame["timestamp_seconds"]))
    matched = sorted(set(truth_groups) & set(prediction_groups))
    first_errors = [min(prediction_groups[plate]) - truth_groups[plate]["first"] for plate in matched]
    last_errors = [max(prediction_groups[plate]) - truth_groups[plate]["last"] for plate in matched]
    absolute_errors = [abs(value) for value in first_errors + last_errors]
    grouping_tp = len(matched)
    grouping_precision = safe_div(grouping_tp, len(prediction_groups))
    grouping_recall = safe_div(grouping_tp, len(truth_groups))
    grouping = {
        "ground_truth_groups": len(truth_groups), "predicted_groups": len(prediction_groups),
        "exact_group_tp": grouping_tp, "exact_group_fp": len(set(prediction_groups) - set(truth_groups)),
        "exact_group_fn": len(set(truth_groups) - set(prediction_groups)),
        "exact_group_precision": grouping_precision, "exact_group_recall": grouping_recall,
        "exact_group_f1": f1(grouping_precision, grouping_recall),
        "source_time_error_seconds": {
            "matched_groups": len(matched),
            "mean_absolute_boundary_error": round(statistics.fmean(absolute_errors), 6) if absolute_errors else None,
            "median_absolute_boundary_error": percentile(absolute_errors, 0.5),
            "p95_absolute_boundary_error": percentile(absolute_errors, 0.95),
            "mean_signed_first_seen_error": round(statistics.fmean(first_errors), 6) if first_errors else None,
            "mean_signed_last_seen_error": round(statistics.fmean(last_errors), 6) if last_errors else None,
        },
    }
    return {
        "event_summary": event_summary, "grouping": grouping,
        "event_results": event_rows,
        "negative_frame_results": negative_frames,
        "truth_group_values": truth_groups, "prediction_group_values": prediction_groups,
    }


def run_video(args: argparse.Namespace) -> dict[str, Any]:
    validation, state = load_oracle(args.oracle_root)
    video = args.video.resolve()
    if hash_file(video) != EXPECTED_VIDEO_SHA256:
        raise RuntimeError("sample.mp4 integrity mismatch")
    events = [event for event in state["video_events"] if event["event_type"] == "plate"]
    if not state["video_review"].get("watched_full_source") or not events:
        raise RuntimeError("video oracle is incomplete")
    processor = make_processor()
    duration, fps = probe_video(video)
    started_wall, started_cpu = time.perf_counter(), time.process_time()
    frames = []
    with tempfile.TemporaryDirectory(prefix="nxmmr-video-anpr-") as temporary:
        for path, timestamp in extract_video_frames(video, Path(temporary), duration):
            frame_hash = hash_file(path)
            result, latency = process_image(
                processor, path, f"video-{timestamp:.3f}", frame_hash,
                timestamp_seconds=timestamp, frame_number_estimate=round(timestamp * fps),
            )
            predicted_raw, details = observation_values(result)
            frames.append({
                "timestamp_seconds": timestamp, "frame_number_estimate": round(timestamp * fps),
                "source_frame_sha256": frame_hash, "predicted_raw": predicted_raw,
                "predictions": details, "latency_seconds": round(latency, 6),
            })
    scored = score_video(events, frames)
    report = {
        "contract_version": CONTRACT, "benchmark": "sample_mp4_video_anpr",
        "evaluated_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "ground_truth": {"status": validation["status"], "gold_digest": state["gold_digest"], "independent_human": True},
        "source": {"sha256": EXPECTED_VIDEO_SHA256, "size_bytes": video.stat().st_size, "duration_seconds": duration, "fps": fps},
        "sampling": {
            "interval_seconds": VIDEO_INTERVAL_SECONDS, "sampled_frames": len(frames),
            "policy_origin": "pre-oracle fixed_1s_full_duration_for_anpr_only selection",
            "configuration_tuned_to_video_oracle": False,
        },
        "processor": {
            "id": processor.processor_id, "revision": processor.processor_revision,
            "detector": processor.detector_id, "ocr": processor.ocr_id,
            "minimum_detection_confidence": MODEL_THRESHOLD,
        },
        "summary": {"events": scored["event_summary"], "grouping": scored["grouping"]},
        "resource": resource_receipt(started_wall, started_cpu),
        "limitations": [
            "one-second pre-oracle sampling can miss human events shorter than the cadence",
            "human intervals provide event counts/text but no per-frame boxes; localization IoU is not measurable",
            "grouping is scored by exact normalized plate value because the oracle has no separate physical-track IDs",
            "single 60-second source; do not generalize as universal video ANPR accuracy",
        ],
        "private_results": {
            "frames": frames, "events": scored["event_results"],
            "negative_frames": scored["negative_frame_results"],
            "truth_groups": scored["truth_group_values"], "prediction_groups": scored["prediction_group_values"],
        },
    }
    return report


def write_report(path: Path, report: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temporary, path)
    print(json.dumps({
        "output": str(path), "benchmark": report["benchmark"], "split": report.get("split"),
        "summary": report["summary"], "resource": report["resource"],
    }, ensure_ascii=False))


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    images = subparsers.add_parser("images")
    images.add_argument("--oracle-root", type=Path, required=True)
    images.add_argument("--image-root", type=Path, required=True)
    images.add_argument("--split", choices=("development", "sealed_holdout"), required=True)
    images.add_argument("--development-receipt", type=Path)
    images.add_argument("--output", type=Path, required=True)
    video = subparsers.add_parser("video")
    video.add_argument("--oracle-root", type=Path, required=True)
    video.add_argument("--video", type=Path, required=True)
    video.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    report = run_images(args) if args.command == "images" else run_video(args)
    write_report(args.output, report)


if __name__ == "__main__":
    main()
