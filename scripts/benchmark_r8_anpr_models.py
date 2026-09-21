#!/usr/bin/env python3
"""Offline, role-aware R8 detector/OCR benchmark and promotion gate.

The scorer consumes an authorized fixture manifest and saved candidate output.
It performs no inference, network access, package installation, or evidence
mutation. Detector and OCR candidates are deliberately scored against different
truth signals so that a detector is never penalized for not emitting text and an
OCR recognizer is never credited with localization it did not perform.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import statistics
import unicodedata
from pathlib import Path
from typing import Any


CONTRACT = "forensics.anpr-model-benchmark/v2"
DETECTOR_ROLE = "localization_baseline_text_detector_adapter"
SPECIALIZED_DETECTOR_ROLE = "plate_localization_detector"
DETECTOR_ROLES = {DETECTOR_ROLE, SPECIALIZED_DETECTOR_ROLE}
OCR_ROLE = "ocr_only_annotated_crop"


def normalize_plate(value: str | None) -> str | None:
    if value is None:
        return None
    normalized = unicodedata.normalize("NFKC", value).upper()
    return "".join(character for character in normalized if character.isalnum())


def iou(left: dict[str, int], right: dict[str, int]) -> float:
    lx2, ly2 = left["x"] + left["width"], left["y"] + left["height"]
    rx2, ry2 = right["x"] + right["width"], right["y"] + right["height"]
    width = max(0, min(lx2, rx2) - max(left["x"], right["x"]))
    height = max(0, min(ly2, ry2) - max(left["y"], right["y"]))
    intersection = width * height
    union = left["width"] * left["height"] + right["width"] * right["height"] - intersection
    return intersection / union if union else 0.0


def edit_distance(left: str, right: str) -> int:
    prior = list(range(len(right) + 1))
    for row, left_char in enumerate(left, 1):
        current = [row]
        for column, right_char in enumerate(right, 1):
            current.append(min(current[-1] + 1, prior[column] + 1, prior[column - 1] + (left_char != right_char)))
        prior = current
    return prior[-1]


def percentile(values: list[float], fraction: float) -> float | None:
    if not values:
        return None
    ordered = sorted(values)
    index = min(len(ordered) - 1, math.ceil(len(ordered) * fraction) - 1)
    return ordered[index]


def present_fixture_classes(fixtures: list[dict[str, Any]]) -> set[str]:
    present: set[str] = set()
    for fixture in fixtures:
        fixture_id = str(fixture.get("fixture_id", "")).lower()
        conditions = {str(item).lower() for item in fixture.get("adverse_conditions", [])}
        declared = fixture.get("fixture_classes", [])
        present.update(str(item) for item in declared)
        present.update(conditions & {"night", "glare", "blur", "skew", "occlusion", "empty_scene"})
        if fixture.get("script_when_visible") == "Arabic-derived Urdu" or "regional" in fixture_id or "urdu" in fixture_id:
            present.add("regional_style")
        if len(fixture.get("plate_regions_original_pixels", [])) == 1 and not conditions:
            present.add("clear_single_plate")
        if len(fixture.get("plate_regions_original_pixels", [])) > 1 or "multiple" in fixture_id:
            present.add("multiple_vehicles")
        if "crop" in fixture_id or "pre_cropped_plate" in conditions:
            present.add("pre_cropped_plate")
    return present


def fixtures_for_role(fixtures: list[dict[str, Any]], role: str | None) -> list[dict[str, Any]]:
    if role in DETECTOR_ROLES:
        return [item for item in fixtures if item.get("detector_evaluation_eligible", True) is True]
    if role == OCR_ROLE:
        return [item for item in fixtures if item.get("ocr_evaluation_eligible", True) is True]
    return fixtures


def score_detector(fixtures: list[dict[str, Any]], predictions: dict[str, Any], threshold: float) -> dict[str, Any]:
    true_positive = false_positive = false_negative = 0
    confidence_pairs: list[tuple[float, float]] = []
    for fixture in fixtures:
        prediction = predictions[fixture["fixture_id"]]
        expected = fixture.get("plate_regions_original_pixels", [])
        detected = prediction.get("plate_regions_original_pixels", [])
        matched: set[int] = set()
        for candidate in detected:
            best_index, best_score = -1, 0.0
            for index, truth in enumerate(expected):
                if index in matched:
                    continue
                score = iou(candidate["bounds"], truth["bounds"])
                if score > best_score:
                    best_index, best_score = index, score
            outcome = float(best_index >= 0 and best_score >= threshold)
            confidence = candidate.get("confidence", prediction.get("confidence"))
            if isinstance(confidence, (int, float)):
                confidence_pairs.append((float(confidence), outcome))
            if outcome:
                matched.add(best_index)
                true_positive += 1
            else:
                false_positive += 1
        false_negative += len(expected) - len(matched)
    precision_denominator = true_positive + false_positive
    recall_denominator = true_positive + false_negative
    precision = true_positive / precision_denominator if precision_denominator else None
    recall = true_positive / recall_denominator if recall_denominator else None
    f1 = 2 * precision * recall / (precision + recall) if precision is not None and recall is not None and precision + recall else None
    return {
        "true_positive": true_positive,
        "false_positive": false_positive,
        "false_negative": false_negative,
        "precision": precision,
        "recall": recall,
        "f1": f1,
        "iou_threshold": threshold,
        "confidence_calibration": brier(confidence_pairs),
    }


def score_ocr(fixtures: list[dict[str, Any]], predictions: dict[str, Any]) -> dict[str, Any]:
    raw_exact = normalized_exact = raw_errors = normalized_errors = 0
    raw_characters = normalized_characters = evaluated = 0
    confidence_pairs: list[tuple[float, float]] = []
    high_confidence_incorrect = 0
    for fixture in fixtures:
        expected_raw = fixture.get("plate_text_raw_when_visible")
        if expected_raw is None:
            continue
        evaluated += 1
        prediction = predictions[fixture["fixture_id"]]
        actual_raw = prediction.get("plate_text_raw") or ""
        expected_normalized = normalize_plate(expected_raw) or ""
        actual_normalized = prediction.get("plate_text_normalized")
        if actual_normalized is None:
            actual_normalized = normalize_plate(actual_raw) or ""
        raw_outcome = actual_raw == expected_raw
        normalized_outcome = actual_normalized == expected_normalized
        raw_exact += int(raw_outcome)
        normalized_exact += int(normalized_outcome)
        raw_errors += edit_distance(expected_raw, actual_raw)
        normalized_errors += edit_distance(expected_normalized, actual_normalized)
        raw_characters += max(1, len(expected_raw))
        normalized_characters += max(1, len(expected_normalized))
        confidence = prediction.get("confidence")
        if isinstance(confidence, (int, float)):
            confidence_pairs.append((float(confidence), float(normalized_outcome)))
            high_confidence_incorrect += int(float(confidence) >= 0.9 and not normalized_outcome)
    return {
        "evaluated_text_count": evaluated,
        "raw_exact_plate_accuracy": raw_exact / evaluated if evaluated else None,
        "normalized_exact_plate_accuracy": normalized_exact / evaluated if evaluated else None,
        "raw_character_error_rate": raw_errors / raw_characters if raw_characters else None,
        "normalized_character_error_rate": normalized_errors / normalized_characters if normalized_characters else None,
        "confidence_calibration": brier(confidence_pairs),
        "incorrect_high_confidence_acceptance_count": high_confidence_incorrect,
        "incorrect_high_confidence_acceptance_rate": high_confidence_incorrect / evaluated if evaluated else None,
        "normalization_policy": "Unicode NFKC, uppercase, retain alphanumeric code points; never infer missing characters",
    }


def brier(pairs: list[tuple[float, float]]) -> dict[str, Any]:
    return {
        "brier_score": sum((confidence - outcome) ** 2 for confidence, outcome in pairs) / len(pairs) if pairs else None,
        "evaluated_count": len(pairs),
    }


def score_abstention(fixtures: list[dict[str, Any]], predictions: dict[str, Any]) -> dict[str, Any]:
    correct = total = abstained = abstained_correct = 0
    for fixture in fixtures:
        expected = fixture.get("expected_abstention_state")
        if expected is None:
            continue
        actual = predictions[fixture["fixture_id"]].get("abstention_state")
        total += 1
        correct += int(actual == expected)
        if actual in {"no_plate_found", "ocr_abstained"}:
            abstained += 1
            abstained_correct += int(expected != "candidate")
    return {
        "accuracy": correct / total if total else None,
        "precision": abstained_correct / abstained if abstained else None,
        "evaluated_count": total,
        "abstained_count": abstained,
    }


def threshold_checks(role: str, metrics: dict[str, Any], gate: dict[str, Any]) -> list[dict[str, Any]]:
    thresholds = gate.get("promotion_thresholds", {}).get("detector" if role in DETECTOR_ROLES else "ocr", {})
    paths = {
        DETECTOR_ROLE: {
            "minimum_localization_precision": (metrics.get("localization", {}).get("precision"), ">="),
            "minimum_localization_recall": (metrics.get("localization", {}).get("recall"), ">="),
            "maximum_brier_score": (metrics.get("localization", {}).get("confidence_calibration", {}).get("brier_score"), "<="),
        },
        OCR_ROLE: {
            "minimum_normalized_exact_plate_accuracy": (metrics.get("ocr", {}).get("normalized_exact_plate_accuracy"), ">="),
            "maximum_normalized_character_error_rate": (metrics.get("ocr", {}).get("normalized_character_error_rate"), "<="),
            "maximum_brier_score": (metrics.get("ocr", {}).get("confidence_calibration", {}).get("brier_score"), "<="),
        },
        SPECIALIZED_DETECTOR_ROLE: {
            "minimum_localization_precision": (metrics.get("localization", {}).get("precision"), ">="),
            "minimum_localization_recall": (metrics.get("localization", {}).get("recall"), ">="),
            "maximum_brier_score": (metrics.get("localization", {}).get("confidence_calibration", {}).get("brier_score"), "<="),
        },
    }.get(role, {})
    common = {
        "minimum_abstention_precision": (metrics.get("abstention", {}).get("precision"), ">="),
        "maximum_p95_latency_ms": (metrics.get("latency_ms", {}).get("p95"), "<="),
        "maximum_peak_process_rss_mib": (metrics.get("resources", {}).get("peak_process_rss_mib"), "<="),
    }
    checks: list[dict[str, Any]] = []
    for name, target in thresholds.items():
        actual, operator = (paths | common).get(name, (None, None))
        passed = actual is not None and operator is not None and (actual >= target if operator == ">=" else actual <= target)
        checks.append({"metric": name, "operator": operator, "target": target, "actual": actual, "passed": passed})
    return checks


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--fixtures", type=Path, required=True)
    parser.add_argument("--predictions", type=Path, required=True)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--taxonomy", type=Path)
    parser.add_argument("--gate-config", type=Path)
    parser.add_argument("--iou-threshold", type=float, default=0.5)
    parser.add_argument("--partition", choices=("all", "development", "validation", "holdout", "sealed_holdout"), default="all")
    parser.add_argument("--evaluation-mode", choices=("benchmark", "tuning"), default="benchmark")
    args = parser.parse_args()

    if args.evaluation_mode == "tuning" and args.partition in {"all", "holdout", "sealed_holdout"}:
        parser.error("tuning mode cannot access a holdout partition; use development or validation")

    manifest = json.loads(args.fixtures.read_text(encoding="utf-8-sig"))
    predictions_payload = json.loads(args.predictions.read_text(encoding="utf-8-sig"))
    all_fixtures = manifest.get("fixtures", [])
    predictions = {item["fixture_id"]: item for item in predictions_payload.get("predictions", [])}
    role = predictions_payload.get("evaluation_role")
    partitioned_fixtures = all_fixtures if args.partition == "all" else [
        item for item in all_fixtures if item.get("evaluation_partition") == args.partition
    ]
    fixtures = fixtures_for_role(partitioned_fixtures, role)
    missing_predictions = [item["fixture_id"] for item in fixtures if item["fixture_id"] not in predictions]

    taxonomy = json.loads(args.taxonomy.read_text(encoding="utf-8-sig")) if args.taxonomy else {}
    required_classes = taxonomy.get("required_classes", [])
    present_classes = sorted(present_fixture_classes(all_fixtures))
    missing_classes = sorted(set(required_classes) - set(present_classes))
    gate_config = json.loads(args.gate_config.read_text(encoding="utf-8-sig")) if args.gate_config else {}

    candidate = predictions_payload.get("candidate")
    measured_package_sizes = gate_config.get("runtime_inventory", {}).get("isolated_evaluator", {}).get("candidate_asset_size_mib", {})
    package_size_mib = predictions_payload.get("package_size_mib", measured_package_sizes.get(candidate))
    metrics: dict[str, Any] = {
        "fixture_count": len(all_fixtures),
        "role_evaluated_fixture_count": len(fixtures),
        "evaluation_partition": args.partition,
        "evaluation_mode": args.evaluation_mode,
        "fixture_coverage": {
            "required_classes": required_classes,
            "present_classes": present_classes,
            "missing_classes": missing_classes,
            "complete": bool(required_classes) and not missing_classes,
        },
        "missing_fixture_predictions": missing_predictions,
        "resources": {
            "peak_process_rss_mib": predictions_payload.get("peak_process_rss_mib"),
            "package_size_mib": package_size_mib,
            "package_size_basis": "read_only_installed_asset_measurement" if package_size_mib is not None else "not_measured",
        },
    }
    if missing_predictions:
        role_metrics: dict[str, Any] = {}
    elif role in DETECTOR_ROLES:
        role_metrics = {"localization": score_detector(fixtures, predictions, args.iou_threshold), "ocr": {"state": "not_applicable_detector_only"}}
    elif role == OCR_ROLE:
        role_metrics = {"localization": {"state": "not_applicable_ocr_uses_annotated_truth_crop"}, "ocr": score_ocr(fixtures, predictions)}
    else:
        role_metrics = {}
    metrics.update(role_metrics)
    if not missing_predictions:
        latencies = [float(predictions[item["fixture_id"]].get("latency_ms")) for item in fixtures if isinstance(predictions[item["fixture_id"]].get("latency_ms"), (int, float)) and predictions[item["fixture_id"]].get("latency_ms") > 0]
        metrics["abstention"] = score_abstention(fixtures, predictions)
        metrics["latency_ms"] = {"p50": statistics.median(latencies) if latencies else None, "p95": percentile(latencies, 0.95), "evaluated_count": len(latencies), "zero_noop_excluded": True}

    checks = threshold_checks(role, metrics, gate_config.get("benchmark_gate", {}))
    blocked_reasons: list[str] = []
    manifest_sha256 = hashlib.sha256(args.fixtures.read_bytes()).hexdigest()
    attested_manifest_sha256 = predictions_payload.get("fixture_manifest_sha256")
    if not fixtures:
        blocked_reasons.append("approved visual fixture pack is empty")
    if missing_predictions:
        blocked_reasons.append("one or more approved fixtures have no prediction")
    if role not in DETECTOR_ROLES | {OCR_ROLE}:
        blocked_reasons.append("candidate evaluation_role is unsupported")
    if missing_classes:
        blocked_reasons.append("fixture taxonomy coverage is incomplete")
    if predictions_payload.get("offline") is not True:
        blocked_reasons.append("offline execution was not attested")
    if attested_manifest_sha256 != manifest_sha256:
        blocked_reasons.append("prediction fixture manifest hash is missing or does not match")
    if not predictions_payload.get("candidate_revision"):
        blocked_reasons.append("candidate revision is missing")
    if metrics["resources"]["package_size_mib"] is None:
        blocked_reasons.append("candidate package size was not measured")
    failed_checks = [check["metric"] for check in checks if not check["passed"]]
    if failed_checks:
        blocked_reasons.append("one or more quantitative promotion thresholds failed")
    state = "pass" if not blocked_reasons else "blocked_no_promotion"
    result = {
        "contract_version": CONTRACT,
        "fixture_contract": manifest.get("contract_version"),
        "candidate": candidate,
        "candidate_revision": predictions_payload.get("candidate_revision"),
        "evaluation_role": role,
        "provenance": {
            "fixture_manifest_sha256": manifest_sha256,
            "attested_fixture_manifest_sha256": attested_manifest_sha256,
            "source_tiers": predictions_payload.get("source_tiers", []),
            "evaluation_partition": args.partition,
            "evaluation_mode": args.evaluation_mode,
            "preprocessing_recipe": predictions_payload.get("preprocessing_recipe"),
            "environment": predictions_payload.get("host", {}),
        },
        "metrics": metrics,
        "threshold_checks": checks,
        "gate": {"state": state, "production_promotion_allowed": state == "pass", "blocked_reasons": blocked_reasons, "failed_thresholds": failed_checks},
    }
    rendered = json.dumps(result, indent=2, sort_keys=True, ensure_ascii=False) + "\n"
    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    else:
        print(rendered, end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
