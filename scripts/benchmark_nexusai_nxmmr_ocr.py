#!/usr/bin/env python3
"""Offline Tesseract/PaddleOCR comparison on identical sealed synthetic crops."""

from __future__ import annotations

import argparse
import json
import platform
import statistics
import subprocess
import tempfile
import time
import unicodedata
from pathlib import Path
from typing import Any

try:
    import resource
except ImportError:  # Scoring unit tests also run on Windows.
    resource = None


def normalize(value: str) -> str:
    return " ".join(unicodedata.normalize("NFKC", value).casefold().split())


def identifier_normalize(value: str) -> str:
    return "".join(character for character in normalize(value) if character.isalnum())


def edit_distance(left: list[str] | str, right: list[str] | str) -> int:
    prior = list(range(len(right) + 1))
    for row, left_item in enumerate(left, 1):
        current = [row]
        for column, right_item in enumerate(right, 1):
            current.append(min(
                current[-1] + 1,
                prior[column] + 1,
                prior[column - 1] + int(left_item != right_item),
            ))
        prior = current
    return prior[-1]


def saved_result_payload(result: Any) -> dict[str, Any]:
    with tempfile.TemporaryDirectory() as directory:
        destination = Path(directory) / "result.json"
        result.save_to_json(save_path=str(destination))
        payload = json.loads(destination.read_text(encoding="utf-8"))
    return payload.get("res", payload)


def tesseract(path: Path, language: str) -> tuple[str, float | None]:
    completed = subprocess.run(
        ["tesseract", str(path), "stdout", "--oem", "1", "--psm", "7", "-l", language, "tsv"],
        check=False, capture_output=True, text=True, timeout=30,
    )
    if completed.returncode != 0:
        return "", None
    words: list[str] = []
    confidences: list[float] = []
    for line in completed.stdout.splitlines()[1:]:
        columns = line.split("\t")
        if len(columns) < 12 or not columns[11].strip():
            continue
        words.append(columns[11].strip())
        try:
            confidence = float(columns[10])
        except ValueError:
            continue
        if confidence >= 0:
            confidences.append(confidence / 100)
    return " ".join(words), statistics.fmean(confidences) if confidences else None


def score(expected: str, actual: str) -> dict[str, Any]:
    reference = normalize(expected)
    candidate = normalize(actual)
    reference_words = reference.split()
    candidate_words = candidate.split()
    normalized_reference = identifier_normalize(expected)
    normalized_candidate = identifier_normalize(actual)
    return {
        "line_exact": candidate == reference,
        "identifier_exact": normalized_candidate == normalized_reference,
        "character_errors": edit_distance(reference, candidate),
        "reference_characters": len(reference),
        "word_errors": edit_distance(reference_words, candidate_words),
        "reference_words": len(reference_words),
        "insertions_deletions_substitutions": "aggregate_edit_distance_only",
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--candidate", choices=("tesseract", "paddle-arabic"), required=True)
    parser.add_argument("--fixtures", type=Path, required=True)
    parser.add_argument("--paddle-model", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--limit", type=int)
    args = parser.parse_args()
    wall_started = time.perf_counter()
    cpu_started = time.process_time()
    if args.candidate == "paddle-arabic" and not args.paddle_model:
        parser.error("--paddle-model is required for paddle-arabic")
    manifest = json.loads(args.fixtures.read_text(encoding="utf-8"))
    if manifest.get("generator") != "nexusai-r8-synthetic-v1":
        parser.error("only the sealed generated NX-MMR/R8 fixture pack is admitted")
    model = None
    runtime: dict[str, Any]
    if args.candidate == "paddle-arabic":
        from paddleocr import TextRecognition
        import paddle
        import paddleocr

        model = TextRecognition(
            model_name="arabic_PP-OCRv5_mobile_rec",
            model_dir=str(args.paddle_model),
            device="cpu", cpu_threads=4, enable_mkldnn=False,
        )
        runtime = {"paddle": paddle.__version__, "paddleocr": paddleocr.__version__}
    else:
        version = subprocess.run(
            ["tesseract", "--version"], check=True, capture_output=True, text=True
        ).stdout.splitlines()[0]
        runtime = {"tesseract": version}
    admitted_fixtures = list(manifest["fixtures"])
    if args.limit is not None:
        if args.limit <= 0:
            parser.error("--limit must be positive")
        admitted_fixtures = admitted_fixtures[:args.limit]
    results: list[dict[str, Any]] = []
    for fixture in admitted_fixtures:
        expected = str(fixture.get("plate_text_raw_when_visible") or "")
        crop = fixture.get("truth_crop_file")
        if not crop:
            results.append({
                "fixture_id": fixture["fixture_id"], "script": "negative",
                "expected": "", "actual": "", "confidence": None,
                "latency_ms": 0.0, "negative_control": True,
                **score("", ""),
            })
            continue
        path = args.fixtures.parent / crop
        started = time.perf_counter()
        if args.candidate == "tesseract":
            language = "urd+eng" if fixture.get("script_when_visible") == "Arabic-derived Urdu" else "eng"
            actual, confidence = tesseract(path, language)
        else:
            predictions = list(model.predict(input=str(path), batch_size=1))
            payload = saved_result_payload(predictions[0]) if predictions else {}
            actual = str(payload.get("rec_text") or "").strip()
            value = payload.get("rec_score")
            confidence = float(value) if isinstance(value, (int, float)) else None
            language = "arabic_multilingual"
        results.append({
            "fixture_id": fixture["fixture_id"],
            "script": "urdu" if fixture.get("script_when_visible") == "Arabic-derived Urdu" else "english_latin",
            "condition": fixture.get("adverse_conditions") or ["clear"],
            "expected": expected,
            "actual": actual,
            "confidence": confidence,
            "latency_ms": round((time.perf_counter() - started) * 1000, 3),
            "language": language,
            "negative_control": False,
            **score(expected, actual),
        })
    strata: dict[str, Any] = {}
    for name in ("english_latin", "urdu", "negative"):
        rows = [row for row in results if row["script"] == name]
        positive = [row for row in rows if not row["negative_control"]]
        characters = sum(row["reference_characters"] for row in positive)
        words = sum(row["reference_words"] for row in positive)
        strata[name] = {
            "samples": len(rows),
            "line_exact_rate": sum(row["line_exact"] for row in positive) / len(positive) if positive else None,
            "identifier_exact_rate": sum(row["identifier_exact"] for row in positive) / len(positive) if positive else None,
            "cer": sum(row["character_errors"] for row in positive) / characters if characters else None,
            "wer": sum(row["word_errors"] for row in positive) / words if words else None,
            "negative_control_pass": all(not row["actual"] for row in rows if row["negative_control"]),
            "latency_ms_mean": statistics.fmean(row["latency_ms"] for row in rows) if rows else None,
        }
    report = {
        "contract_version": "nexusai.nxmmr.ocr-benchmark/v1",
        "candidate": args.candidate,
        "fixture_count": len(admitted_fixtures),
        "evidence_tier": "FIXTURE",
        "ground_truth": "deterministic_generator_before_candidate_execution",
        "same_samples_and_truth_crops": True,
        "runtime": {**runtime, "python": platform.python_version()},
        "resource": {
            "peak_process_rss_mib": (
                round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024, 3)
                if resource is not None else None
            ),
            "cpu_threads": 4 if args.candidate == "paddle-arabic" else None,
            "wall_seconds": round(time.perf_counter() - wall_started, 6),
            "process_cpu_seconds": round(time.process_time() - cpu_started, 6),
        },
        "strata": strata,
        "results": results,
        "promotion_authority": "FIXTURE_CERTIFIED_ONLY",
        "retained_state_mutated": False,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({
        "candidate": args.candidate, "strata": strata,
        "peak_process_rss_mib": report["resource"]["peak_process_rss_mib"],
        "wall_seconds": report["resource"]["wall_seconds"],
        "process_cpu_seconds": report["resource"]["process_cpu_seconds"],
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
