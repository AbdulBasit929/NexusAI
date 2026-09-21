#!/usr/bin/env python3
"""Run a bounded non-retained image ANPR matrix with separate ground truth."""

from __future__ import annotations

import argparse
import hashlib
import json
import time
from pathlib import Path
from typing import Any

import cv2

try:
    from ingestion.forensic_records.media_pipeline import FastALPRImageProcessor, normalize_plate
except ModuleNotFoundError:
    from media_pipeline import FastALPRImageProcessor, normalize_plate


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def derive_transforms(source: Path, output: Path) -> list[dict[str, Any]]:
    image = cv2.imread(str(source))
    if image is None:
        raise RuntimeError(f"cannot decode transform source: {source}")
    output.mkdir(parents=True, exist_ok=True)
    height, width = image.shape[:2]
    transforms: list[tuple[str, Any, str, list[int]]] = [
        ("resize_50_percent", cv2.resize(image, (max(1, width // 2), max(1, height // 2))), ".png", []),
        ("jpeg_quality_45", image, ".jpg", [cv2.IMWRITE_JPEG_QUALITY, 45]),
        ("gaussian_blur", cv2.GaussianBlur(image, (9, 9), 2.5), ".png", []),
        ("low_light", cv2.convertScaleAbs(image, alpha=0.35, beta=0), ".png", []),
        ("bright_light", cv2.convertScaleAbs(image, alpha=1.5, beta=55), ".png", []),
    ]
    partial = image.copy()
    cv2.rectangle(partial, (width // 2, height // 3), (width * 3 // 4, height * 2 // 3), (0, 0, 0), -1)
    transforms.append(("partial_obstruction", partial, ".png", []))
    rows = []
    for name, value, suffix, params in transforms:
        target = output / f"{name}{suffix}"
        if not cv2.imwrite(str(target), value, params):
            raise RuntimeError(f"could not write transform: {target}")
        rows.append(
            {
                "id": f"derived-lef1981-{name}",
                "runtime_path": str(target),
                "category": name,
                "expected_plate": "LEF1981",
                "license_ownership": "non-retained deterministic transform of the manifest source",
                "derived_from_sha256": sha256(source),
                "transform": name,
            }
        )
    return rows


def benchmark(processor: FastALPRImageProcessor, fixture: dict[str, Any]) -> dict[str, Any]:
    path = Path(fixture["runtime_path"])
    source_hash = sha256(path)
    started = time.perf_counter()
    failure = None
    result = None
    try:
        result = processor.process_image(
            path,
            evidence_id=f"non-retained-{fixture['id']}",
            version_id="non-retained-v1",
            source_sha256=source_hash,
            source_file=path.name,
        )
    except Exception as exc:
        failure = {"type": type(exc).__name__, "message": str(exc)}
    latency = time.perf_counter() - started
    observations = list(result.observations) if result is not None else []
    expected = fixture.get("expected_plate")
    normalized_expected = normalize_plate(expected or "")
    actual = [str(item.payload.get("normalized_plate_text") or "") for item in observations]
    if expected is None:
        verdict = "PASS" if not observations and failure is None else "FAIL"
    elif normalized_expected in actual:
        verdict = "PASS"
    elif observations:
        verdict = "PARTIAL"
    else:
        verdict = "FAIL"
    return {
        **fixture,
        "source_sha256": source_hash,
        "size_bytes": path.stat().st_size,
        "failure": failure,
        "latency_seconds": round(latency, 6),
        "detected": bool(observations),
        "expected_normalized": normalized_expected or None,
        "actual_normalized": actual,
        "observations": [
            {
                "observation_id": item.observation_id,
                "raw_ocr": item.payload.get("raw_plate_text"),
                "normalized_plate": item.payload.get("normalized_plate_text"),
                "detector_confidence": item.payload.get("detection_confidence"),
                "ocr_confidence": item.payload.get("ocr_confidence"),
                "bbox": item.citation_locator.get("bbox"),
                "crop": item.payload.get("crop"),
                "detector_model": item.payload.get("detector_model"),
                "ocr_model": item.payload.get("ocr_model"),
                "manual_review_required": item.payload.get("manual_review_required"),
            }
            for item in observations
        ],
        "limitations": list(result.limitations) if result is not None else [],
        "verdict": verdict,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    fixtures = list(manifest["fixtures"])
    transform_source = next(
        Path(row["runtime_path"])
        for row in fixtures
        if row["id"] == manifest["derived_transform_plan"]["source_fixture"]
    )
    fixtures.extend(derive_transforms(transform_source, Path("/tmp/mmv2-images/derived")))
    processor = FastALPRImageProcessor.from_environment()
    results = [benchmark(processor, fixture) for fixture in fixtures]
    report = {
        "contract_version": "nexusai.mmv2.image-anpr-real-world-matrix/v1",
        "retained_data_mutated": False,
        "ground_truth_authority": manifest["ground_truth_authority"],
        "processor": {
            "id": processor.processor_id,
            "revision": processor.processor_revision,
            "detector_model": processor.detector_id,
            "ocr_model": processor.ocr_id,
        },
        "summary": {
            "fixture_count": len(results),
            "pass": sum(row["verdict"] == "PASS" for row in results),
            "partial": sum(row["verdict"] == "PARTIAL" for row in results),
            "fail": sum(row["verdict"] == "FAIL" for row in results),
            "claim_boundary": "Small bounded matrix; model observations remain review-required and do not establish population accuracy.",
        },
        "results": results,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"output": str(args.output), "summary": report["summary"]}, indent=2))


if __name__ == "__main__":
    main()
