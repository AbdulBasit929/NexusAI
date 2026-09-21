#!/usr/bin/env python3
"""Produce transparent detector/OCR error evidence from saved R8 predictions."""

from __future__ import annotations

import argparse
from collections import Counter
import json
import unicodedata
from pathlib import Path
from typing import Any


DETECTOR_ROLE = "localization_baseline_text_detector_adapter"
OCR_ROLE = "ocr_only_annotated_crop"
NEGATIVE_CATEGORIES = {
    "signboard": "signage",
    "vehicle_no_plate": "vehicle_body_regions",
    "rectangles": "background_structures",
    "document": "documents",
    "logo": "logos",
    "noise": "noise",
    "empty_scene": "background_structures",
}


def normalize(value: str | None) -> str:
    return "".join(
        character
        for character in unicodedata.normalize("NFKC", value or "").upper()
        if character.isalnum()
    )


def iou(left: dict[str, int], right: dict[str, int]) -> float:
    lx2, ly2 = left["x"] + left["width"], left["y"] + left["height"]
    rx2, ry2 = right["x"] + right["width"], right["y"] + right["height"]
    width = max(0, min(lx2, rx2) - max(left["x"], right["x"]))
    height = max(0, min(ly2, ry2) - max(left["y"], right["y"]))
    intersection = width * height
    union = left["width"] * left["height"] + right["width"] * right["height"] - intersection
    return intersection / union if union else 0.0


def align(expected: str, actual: str) -> list[tuple[str, str, str]]:
    """Return deterministic Levenshtein operations for confusion accounting."""
    rows, columns = len(expected) + 1, len(actual) + 1
    cost = [[0] * columns for _ in range(rows)]
    for row in range(rows):
        cost[row][0] = row
    for column in range(columns):
        cost[0][column] = column
    for row in range(1, rows):
        for column in range(1, columns):
            cost[row][column] = min(
                cost[row - 1][column] + 1,
                cost[row][column - 1] + 1,
                cost[row - 1][column - 1] + (expected[row - 1] != actual[column - 1]),
            )
    operations: list[tuple[str, str, str]] = []
    row, column = len(expected), len(actual)
    while row or column:
        if row and column and cost[row][column] == cost[row - 1][column - 1] + (expected[row - 1] != actual[column - 1]):
            kind = "match" if expected[row - 1] == actual[column - 1] else "substitution"
            operations.append((kind, expected[row - 1], actual[column - 1]))
            row -= 1
            column -= 1
        elif row and cost[row][column] == cost[row - 1][column] + 1:
            operations.append(("deletion", expected[row - 1], ""))
            row -= 1
        else:
            operations.append(("insertion", "", actual[column - 1]))
            column -= 1
    return list(reversed(operations))


def detector_analysis(fixtures: list[dict[str, Any]], predictions: dict[str, dict[str, Any]]) -> dict[str, Any]:
    rows: list[dict[str, Any]] = []
    category_counts: Counter[str] = Counter()
    true_confidences: list[float] = []
    false_confidences: list[float] = []
    for fixture in fixtures:
        if fixture.get("detector_evaluation_eligible", True) is not True:
            continue
        prediction = predictions[fixture["fixture_id"]]
        expected = fixture.get("plate_regions_original_pixels", [])
        detected = prediction.get("plate_regions_original_pixels", [])
        matched: set[int] = set()
        false_positives = 0
        for candidate in sorted(detected, key=lambda item: float(item.get("confidence") or 0), reverse=True):
            best_index, best_iou = -1, 0.0
            for index, truth in enumerate(expected):
                if index in matched:
                    continue
                overlap = iou(candidate["bounds"], truth["bounds"])
                if overlap > best_iou:
                    best_index, best_iou = index, overlap
            confidence = candidate.get("confidence")
            if best_index >= 0 and best_iou >= 0.5:
                matched.add(best_index)
                if isinstance(confidence, (int, float)):
                    true_confidences.append(float(confidence))
            else:
                false_positives += 1
                if isinstance(confidence, (int, float)):
                    false_confidences.append(float(confidence))
        false_negatives = len(expected) - len(matched)
        if false_positives:
            conditions = fixture.get("adverse_conditions", [])
            category = next((NEGATIVE_CATEGORIES[item] for item in conditions if item in NEGATIVE_CATEGORIES), None)
            if category is None:
                category = "multiple_object_confusion" if len(expected) > 1 else "text_like_plate_scene"
            category_counts[category] += false_positives
        rows.append({
            "fixture_id": fixture["fixture_id"],
            "conditions": fixture.get("adverse_conditions", []),
            "expected_regions": len(expected),
            "detected_regions": len(detected),
            "true_positive": len(matched),
            "false_positive": false_positives,
            "false_negative": false_negatives,
        })
    return {
        "false_positive_categories": dict(sorted(category_counts.items())),
        "true_positive_confidence": summarize(true_confidences),
        "false_positive_confidence": summarize(false_confidences),
        "exploratory_confidence_sweep": [
            detector_threshold_result(fixtures, predictions, threshold)
            for threshold in (0.0, 0.3, 0.5, 0.7, 0.8, 0.9, 0.95)
        ],
        "exploratory_nms_sweep": [
            detector_threshold_result(fixtures, predictions, 0.0, nms_iou=threshold)
            for threshold in (0.3, 0.5, 0.7)
        ],
        "tuning_policy": "exploratory T1 only; no threshold is eligible for acceptance before held-out T2/T3 validation",
        "by_fixture": rows,
    }


def detector_threshold_result(
    fixtures: list[dict[str, Any]], predictions: dict[str, dict[str, Any]], threshold: float,
    nms_iou: float | None = None,
) -> dict[str, Any]:
    true_positive = false_positive = false_negative = 0
    for fixture in fixtures:
        if fixture.get("detector_evaluation_eligible", True) is not True:
            continue
        expected = fixture.get("plate_regions_original_pixels", [])
        candidates = [
            item for item in predictions[fixture["fixture_id"]].get("plate_regions_original_pixels", [])
            if isinstance(item.get("confidence"), (int, float)) and float(item["confidence"]) >= threshold
        ]
        if nms_iou is not None:
            candidates = non_maximum_suppression(candidates, nms_iou)
        matched: set[int] = set()
        for candidate in sorted(candidates, key=lambda item: float(item["confidence"]), reverse=True):
            best_index, best_iou = -1, 0.0
            for index, truth in enumerate(expected):
                if index in matched:
                    continue
                overlap = iou(candidate["bounds"], truth["bounds"])
                if overlap > best_iou:
                    best_index, best_iou = index, overlap
            if best_index >= 0 and best_iou >= 0.5:
                matched.add(best_index)
                true_positive += 1
            else:
                false_positive += 1
        false_negative += len(expected) - len(matched)
    return {
        "minimum_confidence": threshold,
        "nms_iou": nms_iou,
        "true_positive": true_positive,
        "false_positive": false_positive,
        "false_negative": false_negative,
        "precision": true_positive / (true_positive + false_positive) if true_positive + false_positive else None,
        "recall": true_positive / (true_positive + false_negative) if true_positive + false_negative else None,
    }


def non_maximum_suppression(candidates: list[dict[str, Any]], threshold: float) -> list[dict[str, Any]]:
    kept: list[dict[str, Any]] = []
    for candidate in sorted(candidates, key=lambda item: float(item.get("confidence") or 0), reverse=True):
        if all(iou(candidate["bounds"], prior["bounds"]) <= threshold for prior in kept):
            kept.append(candidate)
    return kept


def summarize(values: list[float]) -> dict[str, Any]:
    return {
        "count": len(values),
        "minimum": min(values) if values else None,
        "maximum": max(values) if values else None,
        "mean": sum(values) / len(values) if values else None,
    }


def ocr_analysis(fixtures: list[dict[str, Any]], predictions: dict[str, dict[str, Any]]) -> dict[str, Any]:
    rows: list[dict[str, Any]] = []
    confusions: Counter[str] = Counter()
    conditions: Counter[str] = Counter()
    script_errors: Counter[str] = Counter()
    for fixture in fixtures:
        if fixture.get("ocr_evaluation_eligible", True) is not True:
            continue
        prediction = predictions[fixture["fixture_id"]]
        expected_raw = fixture.get("plate_text_raw_when_visible")
        actual_raw = prediction.get("plate_text_raw") or ""
        expected = normalize(expected_raw)
        actual = prediction.get("plate_text_normalized") or normalize(actual_raw)
        operations = align(expected, actual)
        errors = [operation for operation in operations if operation[0] != "match"]
        for kind, left, right in errors:
            confusions[f"{kind}:{left or '∅'}->{right or '∅'}"] += 1
        if errors:
            script_errors[fixture.get("script_when_visible") or "none"] += 1
            for condition in fixture.get("adverse_conditions", []) or ["clear"]:
                conditions[condition] += 1
        rows.append({
            "fixture_id": fixture["fixture_id"],
            "script": fixture.get("script_when_visible"),
            "conditions": fixture.get("adverse_conditions", []),
            "expected_raw": expected_raw,
            "actual_raw": actual_raw or None,
            "expected_normalized": expected,
            "actual_normalized": actual or None,
            "confidence": prediction.get("confidence"),
            "abstention_state": prediction.get("abstention_state"),
            "normalization_steps": prediction.get("normalization_steps", []),
            "errors": [{"kind": kind, "expected": left or None, "actual": right or None} for kind, left, right in errors],
        })
    return {
        "confusion_counts": dict(confusions.most_common()),
        "error_fixtures_by_script": dict(script_errors),
        "error_fixtures_by_condition": dict(conditions),
        "by_fixture": rows,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--fixtures", type=Path, required=True)
    parser.add_argument("--predictions", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    manifest = json.loads(args.fixtures.read_text(encoding="utf-8-sig"))
    payload = json.loads(args.predictions.read_text(encoding="utf-8-sig"))
    predictions = {item["fixture_id"]: item for item in payload.get("predictions", [])}
    fixtures = [item for item in manifest.get("fixtures", []) if item["fixture_id"] in predictions]
    role = payload.get("evaluation_role")
    if role == DETECTOR_ROLE:
        analysis = detector_analysis(fixtures, predictions)
    elif role == OCR_ROLE:
        analysis = ocr_analysis(fixtures, predictions)
    else:
        raise SystemExit(f"unsupported evaluation role: {role}")
    result = {
        "contract_version": "forensics.anpr-error-analysis/v1",
        "candidate": payload.get("candidate"),
        "preprocessing_variant": payload.get("preprocessing_variant", "baseline"),
        "evaluation_role": role,
        "analysis": analysis,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
