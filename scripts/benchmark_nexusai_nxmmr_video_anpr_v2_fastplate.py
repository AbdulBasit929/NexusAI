#!/usr/bin/env python3
"""Re-OCR actual V1 development crops with the existing FastPlateOCR model."""

from __future__ import annotations

import argparse
import csv
import hashlib
import itertools
import json
import statistics
import sys
import time
import uuid
from dataclasses import asdict
from pathlib import Path
from typing import Any


REPOSITORY = Path(__file__).resolve().parents[1]
for path in (REPOSITORY, REPOSITORY / "scripts"):
    if str(path) not in sys.path:
        sys.path.insert(0, str(path))

import benchmark_nexusai_nxmmr_anpr as scorer  # noqa: E402
from benchmark_nexusai_nxmmr_parity_adapter import group_frame_rows, peak_rss_mib, summarize_score  # noqa: E402
from ingestion.forensic_records.video_anpr_parity_adapter import (  # noqa: E402
    PlateBounds,
    PlateObservation,
    canonical_digest,
    normalize_plate_neutral,
)
from ingestion.forensic_records.video_anpr_product_v2 import (  # noqa: E402
    ADAPTER_ID,
    ADAPTER_VERSION,
    ProductAggregationPolicy,
    aggregate_product_observations,
)


EXPECTED_VIDEO_SHA256 = "d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee"
EXPECTED_OCR_SHA256 = "8031afb5fdc6b4d80462c9d542f1284ebd2cfddf5dbacd62609848d7e2855f44"
EXPECTED_CONFIG_SHA256 = "0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6"


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def load_events(path: Path, split: dict[str, Any]) -> list[dict[str, str]]:
    with path.open("r", encoding="utf-8-sig", newline="") as source:
        rows = [row for row in csv.DictReader(source) if row.get("event_type") == "plate"]
    allowed, reserved = set(split["development_event_ids"]), set(split["reserved_event_ids"])
    if allowed & reserved or allowed | reserved != {row["event_id"] for row in rows}:
        raise RuntimeError("split/oracle accounting mismatch")
    return [row for row in rows if row["event_id"] in allowed]


def source_candidate(receipt: dict[str, Any], identity: str) -> dict[str, Any]:
    return next(row for row in receipt["candidates"] if row.get("candidate_id") == identity)


def rank(row: dict[str, Any]) -> tuple[Any, ...]:
    events, groups = row["score"]["events"], row["score"]["grouping"]
    return (
        float(events["normalized_exact_plate"].get("f1") or 0),
        float(events.get("event_detection_recall") or 0),
        float(groups.get("exact_group_f1") or 0),
        -float(events.get("negative_frame_false_positive_rate") or 0),
        -int(row["group_count"]),
        row["policy_id"],
    )


def reocr(candidate: dict[str, Any], video: Path, recognizer: Any, cv2: Any) -> tuple[list[PlateObservation], dict[str, Any]]:
    source = [
        PlateObservation(**{**row, "bounds": PlateBounds(**row["bounds"])})
        for row in candidate["private_output"]["observations"]
    ]
    unique: dict[tuple[Any, ...], PlateObservation] = {}
    for item in source:
        key = (item.frame_number, item.crop_sha256, round(item.bounds.x), round(item.bounds.y), round(item.bounds.width), round(item.bounds.height))
        unique.setdefault(key, item)
    capture = cv2.VideoCapture(str(video))
    if not capture.isOpened():
        raise RuntimeError("could not open the exact sample video")
    output: list[PlateObservation] = []
    latencies: list[float] = []
    failed = 0
    try:
        for item in sorted(unique.values(), key=lambda row: (row.frame_number, row.observation_id)):
            capture.set(cv2.CAP_PROP_POS_FRAMES, item.frame_number)
            ok, frame = capture.read()
            if not ok:
                failed += 1
                continue
            x1, y1 = max(0, round(item.bounds.x)), max(0, round(item.bounds.y))
            x2 = min(frame.shape[1], round(item.bounds.x + item.bounds.width))
            y2 = min(frame.shape[0], round(item.bounds.y + item.bounds.height))
            if x2 <= x1 or y2 <= y1:
                failed += 1
                continue
            crop = frame[y1:y2, x1:x2]
            started = time.perf_counter()
            predictions = recognizer.run(crop, return_confidence=True)
            latencies.append((time.perf_counter() - started) * 1000)
            prediction = predictions[0] if predictions else None
            raw = str(getattr(prediction, "plate", "") or "")
            if not normalize_plate_neutral(raw):
                continue
            confidence_values = getattr(prediction, "char_probs", None)
            confidence = statistics.fmean(float(value) for value in confidence_values) if confidence_values is not None and len(confidence_values) else None
            output.append(PlateObservation(
                observation_id=str(uuid.uuid5(uuid.NAMESPACE_URL, f"{item.observation_id}:fastplate-v2:{raw}")),
                source_sha256=item.source_sha256,
                source_file=item.source_file,
                frame_number=item.frame_number,
                timestamp_seconds=item.timestamp_seconds,
                source_frame_sha256=item.source_frame_sha256,
                bounds=item.bounds,
                raw_ocr=raw,
                normalized_ocr=normalize_plate_neutral(raw),
                detector_confidence=item.detector_confidence,
                ocr_confidence=confidence,
                crop_sha256=item.crop_sha256,
                detector_model_sha256=item.detector_model_sha256,
                ocr_model_sha256=EXPECTED_OCR_SHA256,
                vehicle_context_matched=item.vehicle_context_matched,
                crop_quality=item.crop_quality,
            ))
    finally:
        capture.release()
    return output, {
        "source_observations": len(source),
        "unique_actual_crops": len(unique),
        "reocr_observations": len(output),
        "failed_frame_or_crop_reads": failed,
        "mean_ocr_latency_ms": statistics.fmean(latencies) if latencies else None,
        "p95_ocr_latency_ms": sorted(latencies)[min(len(latencies) - 1, round(.95 * (len(latencies) - 1)))] if latencies else None,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--video", type=Path, required=True)
    parser.add_argument("--oracle", type=Path, required=True)
    parser.add_argument("--split", type=Path, required=True)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--plate-only-receipt", type=Path, required=True)
    parser.add_argument("--vehicle-receipt", type=Path, required=True)
    parser.add_argument("--v1-replay", type=Path, required=True)
    parser.add_argument("--ocr-model", type=Path, required=True)
    parser.add_argument("--ocr-config", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--freeze-output", type=Path, required=True)
    args = parser.parse_args()
    if sha256_file(args.video) != EXPECTED_VIDEO_SHA256 or sha256_file(args.ocr_model) != EXPECTED_OCR_SHA256 or sha256_file(args.ocr_config) != EXPECTED_CONFIG_SHA256:
        raise RuntimeError("video or FastPlateOCR asset integrity mismatch")
    split = json.loads(args.split.read_text(encoding="utf-8"))
    if split.get("candidate_evaluation_performed") is not False:
        raise RuntimeError("reserved evaluation boundary is not intact")
    events = load_events(args.oracle, split)
    config = json.loads(args.config.read_text(encoding="utf-8"))
    plate = source_candidate(json.loads(args.plate_only_receipt.read_text(encoding="utf-8")), "fixed-4fps-plate-only-majority")
    vehicle = source_candidate(json.loads(args.vehicle_receipt.read_text(encoding="utf-8")), "fixed-4fps-vehicle-majority")
    from fast_plate_ocr import LicensePlateRecognizer
    import cv2
    recognizer = LicensePlateRecognizer(
        hub_ocr_model=None, device="cpu", providers=["CPUExecutionProvider"],
        onnx_model_path=args.ocr_model, plate_config_path=args.ocr_config, force_download=False,
    )
    wall_started, cpu_started = time.perf_counter(), time.process_time()
    sources = []
    for name, candidate in (("plate_only", plate), ("vehicle_context", vehicle)):
        observed, resources = reocr(candidate, args.video, recognizer, cv2)
        sources.append((name, candidate, observed, resources))
    comparison = config["bounded_comparison"]
    policies = list(itertools.product(
        comparison["strategies"], comparison["minimum_selected_support"],
        comparison["minimum_candidate_length"], comparison["minimum_best_ocr_confidence"],
        comparison["minimum_weighted_support"], comparison["minimum_observed_duration_seconds"],
    ))
    experiments = []
    for name, candidate, observed, resources in sources:
        for strategy, support, length, confidence, weight, duration in policies:
            policy = ProductAggregationPolicy(
                strategy=strategy, minimum_selected_support=int(support),
                minimum_candidate_length=int(length), maximum_candidate_length=int(comparison["maximum_candidate_length"]),
                minimum_best_ocr_confidence=float(confidence), minimum_weighted_support=float(weight),
                minimum_observed_duration_seconds=float(duration),
            )
            groups = aggregate_product_observations(observed, policy)
            score = summarize_score(scorer.score_video(events, group_frame_rows(groups, observed, candidate["private_output"]["frames"])))
            value = asdict(policy)
            experiments.append({
                "policy_id": canonical_digest({"ocr": "fastplate", "context": name, **value})[:16],
                "vehicle_context": name, "ocr": "cct-xs-v2-global-model",
                "policy": value, "group_count": len(groups), "score": score,
            })
    selected = max(experiments, key=rank)
    best_by_context = {name: max((row for row in experiments if row["vehicle_context"] == name), key=rank) for name, *_ in sources}
    v1 = json.loads(args.v1_replay.read_text(encoding="utf-8"))
    v1_row = next(row for row in v1["experiments"] if row["frame_policy_id"] == "fixed-4fps-plate-only-majority" and row["aggregation_strategy"] == "normalized_majority" and int(row["minimum_selected_support"]) == 2)
    selected_events, selected_groups = selected["score"]["events"], selected["score"]["grouping"]
    v1_events, v1_groups = v1_row["score"]["events"], v1_row["score"]["grouping"]
    materially_better = (
        float(selected_events["normalized_exact_plate"]["f1"] or 0) >= float(v1_events["normalized_exact_plate"]["f1"] or 0) + .10
        and float(selected_groups["exact_group_f1"] or 0) >= float(v1_groups["exact_group_f1"] or 0) + .10
        and float(selected_events["event_detection_recall"] or 0) >= .70
    )
    report = {
        "contract_version": "nexusai.nxmmr.video-anpr-v2-fastplate-development/v1",
        "adapter_id": ADAPTER_ID, "adapter_version": ADAPTER_VERSION,
        "development_only": True, "development_split_digest": split["split_digest"],
        "development_event_count": len(events), "reserved_event_count": int(split["reserved_event_count"]),
        "candidate_evaluation_performed": False, "reserved_candidate_metrics_computed": False,
        "actual_crops_only": True, "interpolated_observations": False,
        "models": {"ocr": EXPECTED_OCR_SHA256, "ocr_config": EXPECTED_CONFIG_SHA256},
        "reocr_resources": {name: resources for name, _, _, resources in sources},
        "vehicle_context_ab": best_by_context, "selected": selected,
        "v1_comparator": v1_row, "material_precision_recall_improvement_gate": materially_better,
        "resource": {"wall_seconds": time.perf_counter() - wall_started, "cpu_seconds": time.process_time() - cpu_started, "peak_ram_mib": peak_rss_mib()},
        "runtime_mutated": False, "retained_state_mutated": False, "activity_mutated": False, "database_or_volumes_mutated": False,
    }
    report["receipt_digest"] = canonical_digest(report)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    if materially_better:
        freeze = {
            "contract_version": "nexusai.nxmmr.video-anpr-v2-freeze/v1", "adapter_id": ADAPTER_ID,
            "adapter_version": ADAPTER_VERSION, "configuration_sha256": sha256_file(args.config),
            "adapter_source_sha256": sha256_file(REPOSITORY / "ingestion/forensic_records/video_anpr_product_v2.py"),
            "benchmark_source_sha256": sha256_file(Path(__file__)), "development_receipt_sha256": sha256_file(args.output),
            "development_split_digest": split["split_digest"], "selected_vehicle_context": selected["vehicle_context"],
            "selected_ocr_model_sha256": EXPECTED_OCR_SHA256, "selected_policy": selected["policy"],
            "candidate_evaluation_performed": False,
        }
        freeze["freeze_digest"] = canonical_digest(freeze)
        args.freeze_output.write_text(json.dumps(freeze, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"selected": selected, "vehicle_context_ab": best_by_context, "materially_better": materially_better, "freeze_written": materially_better, "resource": report["resource"]}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
