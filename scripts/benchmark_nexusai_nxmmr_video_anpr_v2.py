#!/usr/bin/env python3
"""Replay a bounded Video ANPR V2 policy on locked development observations."""

from __future__ import annotations

import argparse
import csv
import hashlib
import itertools
import json
import sys
from dataclasses import asdict
from pathlib import Path
from typing import Any


REPOSITORY = Path(__file__).resolve().parents[1]
if str(REPOSITORY) not in sys.path:
    sys.path.insert(0, str(REPOSITORY))
if str(REPOSITORY / "scripts") not in sys.path:
    sys.path.insert(0, str(REPOSITORY / "scripts"))

import benchmark_nexusai_nxmmr_anpr as scorer  # noqa: E402
from benchmark_nexusai_nxmmr_parity_adapter import group_frame_rows, summarize_score  # noqa: E402
from ingestion.forensic_records.video_anpr_parity_adapter import (  # noqa: E402
    PlateBounds,
    PlateObservation,
    canonical_digest,
)
from ingestion.forensic_records.video_anpr_product_v2 import (  # noqa: E402
    ADAPTER_ID,
    ADAPTER_VERSION,
    ProductAggregationPolicy,
    aggregate_product_observations,
)


CONTRACT = "nexusai.nxmmr.video-anpr-v2-development-replay/v1"


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def load_development_events(oracle: Path, split: dict[str, Any]) -> list[dict[str, str]]:
    with oracle.open("r", encoding="utf-8-sig", newline="") as source:
        rows = [row for row in csv.DictReader(source) if row.get("event_type") == "plate"]
    allowed = set(split["development_event_ids"])
    reserved = set(split["reserved_event_ids"])
    if allowed & reserved or allowed | reserved != {row["event_id"] for row in rows}:
        raise RuntimeError("development/reserved oracle accounting mismatch")
    return [row for row in rows if row["event_id"] in allowed]


def source_candidate(receipt: dict[str, Any], candidate_id: str) -> dict[str, Any]:
    candidate = next(
        (row for row in receipt["candidates"] if row.get("candidate_id") == candidate_id),
        None,
    )
    if candidate is None or not isinstance(candidate.get("private_output"), dict):
        raise RuntimeError(f"development observation source is unavailable: {candidate_id}")
    return candidate


def observations(candidate: dict[str, Any]) -> tuple[PlateObservation, ...]:
    return tuple(
        PlateObservation(**{**row, "bounds": PlateBounds(**row["bounds"])})
        for row in candidate["private_output"]["observations"]
    )


def rank(row: dict[str, Any]) -> tuple[Any, ...]:
    score = row["score"]
    events, groups = score["events"], score["grouping"]
    return (
        float(events["normalized_exact_plate"].get("f1") or 0),
        float(events.get("event_detection_recall") or 0),
        float(groups.get("exact_group_f1") or 0),
        -float(events.get("negative_frame_false_positive_rate") or 0),
        -int(row["group_count"]),
        row["policy_id"],
    )


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--split", type=Path, required=True)
    parser.add_argument("--oracle", type=Path, required=True)
    parser.add_argument("--plate-only-receipt", type=Path, required=True)
    parser.add_argument("--vehicle-receipt", type=Path, required=True)
    parser.add_argument("--v1-replay", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--freeze-output", type=Path, required=True)
    args = parser.parse_args()

    config = json.loads(args.config.read_text(encoding="utf-8"))
    split = json.loads(args.split.read_text(encoding="utf-8"))
    if split.get("candidate_evaluation_performed") is not False:
        raise RuntimeError("reserved evaluation boundary is not intact")
    development_events = load_development_events(args.oracle, split)
    if len(development_events) != int(split["development_event_count"]):
        raise RuntimeError("development event count mismatch")

    plate_receipt = json.loads(args.plate_only_receipt.read_text(encoding="utf-8"))
    vehicle_receipt = json.loads(args.vehicle_receipt.read_text(encoding="utf-8"))
    sources = [
        ("plate_only", source_candidate(plate_receipt, "fixed-4fps-plate-only-majority")),
        ("vehicle_context", source_candidate(vehicle_receipt, "fixed-4fps-vehicle-majority")),
    ]
    for _, candidate in sources:
        if candidate.get("candidate", {}).get("base_cadence_fps") != 4:
            raise RuntimeError("V2 replay requires the fixed 4 FPS development source")

    comparison = config["bounded_comparison"]
    experiments: list[dict[str, Any]] = []
    grid = itertools.product(
        comparison["strategies"],
        comparison["minimum_selected_support"],
        comparison["minimum_candidate_length"],
        comparison["minimum_best_ocr_confidence"],
        comparison["minimum_weighted_support"],
        comparison["minimum_observed_duration_seconds"],
    )
    policies = list(grid)
    for context_name, candidate in sources:
        observed = observations(candidate)
        frames = candidate["private_output"]["frames"]
        for strategy, support, length, confidence, weight, duration in policies:
            policy = ProductAggregationPolicy(
                strategy=strategy,
                minimum_selected_support=int(support),
                minimum_candidate_length=int(length),
                maximum_candidate_length=int(comparison["maximum_candidate_length"]),
                minimum_best_ocr_confidence=float(confidence),
                minimum_weighted_support=float(weight),
                minimum_observed_duration_seconds=float(duration),
            )
            groups = aggregate_product_observations(observed, policy)
            score = summarize_score(scorer.score_video(
                development_events, group_frame_rows(groups, observed, frames)
            ))
            policy_value = asdict(policy)
            experiments.append({
                "policy_id": canonical_digest({"context": context_name, **policy_value})[:16],
                "vehicle_context": context_name,
                "policy": policy_value,
                "group_count": len(groups),
                "score": score,
            })

    best_by_context = {
        name: max((row for row in experiments if row["vehicle_context"] == name), key=rank)
        for name, _ in sources
    }
    selected = max(experiments, key=rank)
    v1 = json.loads(args.v1_replay.read_text(encoding="utf-8"))
    v1_row = next(
        row for row in v1["experiments"]
        if row["frame_policy_id"] == "fixed-4fps-plate-only-majority"
        and row["aggregation_strategy"] == "normalized_majority"
        and int(row["minimum_selected_support"]) == 2
    )
    v1_score = v1_row["score"]
    selected_events = selected["score"]["events"]
    selected_groups = selected["score"]["grouping"]
    v1_events, v1_groups = v1_score["events"], v1_score["grouping"]
    materially_better = (
        float(selected_events["normalized_exact_plate"]["f1"] or 0)
        >= float(v1_events["normalized_exact_plate"]["f1"] or 0) + 0.10
        and float(selected_groups["exact_group_f1"] or 0)
        >= float(v1_groups["exact_group_f1"] or 0) + 0.10
        and float(selected_events["event_detection_recall"] or 0) >= 0.70
    )
    receipt: dict[str, Any] = {
        "contract_version": CONTRACT,
        "adapter_id": ADAPTER_ID,
        "adapter_version": ADAPTER_VERSION,
        "development_only": True,
        "development_split_digest": split["split_digest"],
        "development_event_count": len(development_events),
        "reserved_event_count": int(split["reserved_event_count"]),
        "reserved_candidate_metrics_computed": False,
        "candidate_evaluation_performed": False,
        "replay_uses_existing_actual_development_observations_only": True,
        "configuration_sha256": sha256_file(args.config),
        "source_receipts": {
            "plate_only": sha256_file(args.plate_only_receipt),
            "vehicle_context": sha256_file(args.vehicle_receipt),
            "v1_replay": sha256_file(args.v1_replay),
        },
        "v1_comparator": v1_row,
        "vehicle_context_ab": best_by_context,
        "selected": selected,
        "material_precision_recall_improvement_gate": materially_better,
        "experiment_count": len(experiments),
        "experiments": experiments,
        "runtime_mutated": False,
        "retained_state_mutated": False,
        "activity_mutated": False,
        "database_or_volumes_mutated": False,
    }
    receipt["receipt_digest"] = canonical_digest(receipt)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    if materially_better:
        freeze = {
            "contract_version": "nexusai.nxmmr.video-anpr-v2-freeze/v1",
            "adapter_id": ADAPTER_ID,
            "adapter_version": ADAPTER_VERSION,
            "configuration_sha256": sha256_file(args.config),
            "adapter_source_sha256": sha256_file(
                REPOSITORY / "ingestion" / "forensic_records" / "video_anpr_product_v2.py"
            ),
            "development_receipt_sha256": sha256_file(args.output),
            "development_split_digest": split["split_digest"],
            "selected_vehicle_context": selected["vehicle_context"],
            "selected_policy": selected["policy"],
            "candidate_evaluation_performed": False,
        }
        freeze["freeze_digest"] = canonical_digest(freeze)
        args.freeze_output.write_text(json.dumps(freeze, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    elif args.freeze_output.exists():
        raise RuntimeError("refusing to leave a V2 freeze when the material-improvement gate failed")

    print(json.dumps({
        "selected": selected,
        "vehicle_context_ab": best_by_context,
        "materially_better": materially_better,
        "freeze_written": materially_better,
        "reserved_candidate_metrics_computed": False,
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
