#!/usr/bin/env python3
"""Download and evaluate isolated R8 OCR candidates without evidence writes."""

from __future__ import annotations

import argparse
from contextlib import contextmanager
import hashlib
import json
import os
import platform
import resource
import subprocess
import tempfile
import time
import unicodedata
from pathlib import Path
from typing import Any


PADDLE_DETECTOR = "PP-OCRv5_mobile_det"
PADDLE_RECOGNIZERS = {
    "paddle-en": "en_PP-OCRv5_mobile_rec",
    "paddle-arabic": "arabic_PP-OCRv5_mobile_rec",
}
OCR_PREPROCESSING = ("baseline", "scale2x", "contrast")
CANDIDATE_REVISIONS = {
    "tesseract": "tesseract-5.5.0+tessdata_fast-eng-urd",
    "paddle-en": "PaddleOCR-3.7.0:en_PP-OCRv5_mobile_rec",
    "paddle-arabic": "PaddleOCR-3.7.0:arabic_PP-OCRv5_mobile_rec",
    "paddle-detector": "PaddleOCR-3.7.0:PP-OCRv5_mobile_det",
}


def file_sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def normalize(value: str) -> str:
    normalized = unicodedata.normalize("NFKC", value).upper()
    return "".join(character for character in normalized if character.isalnum())


@contextmanager
def prepared_ocr_input(image_path: Path, preprocessing: str):
    """Materialize exactly one declared OCR transformation, if requested."""
    if preprocessing == "baseline":
        yield image_path
        return
    from PIL import Image, ImageEnhance

    with tempfile.TemporaryDirectory() as directory:
        with Image.open(image_path) as source:
            image = source.convert("RGB")
            if preprocessing == "scale2x":
                image = image.resize(
                    (image.width * 2, image.height * 2),
                    resample=Image.Resampling.LANCZOS,
                )
            elif preprocessing == "contrast":
                image = ImageEnhance.Contrast(image).enhance(1.5)
            else:  # argparse constrains this, but keep direct callers fail-closed.
                raise ValueError(f"unsupported OCR preprocessing: {preprocessing}")
            destination = Path(directory) / f"{image_path.stem}-{preprocessing}.png"
            image.save(destination, format="PNG", optimize=False)
        yield destination


@contextmanager
def prepared_detector_input(image_path: Path, target_long_side: int):
    if target_long_side == 0:
        yield image_path, 1.0, 1.0
        return
    from PIL import Image

    with tempfile.TemporaryDirectory() as directory:
        with Image.open(image_path) as source:
            source_width, source_height = source.size
            scale = target_long_side / max(source.width, source.height)
            width = max(1, round(source.width * scale))
            height = max(1, round(source.height * scale))
            resized = source.convert("RGB").resize((width, height), Image.Resampling.LANCZOS)
            destination = Path(directory) / f"{image_path.stem}-long-{target_long_side}.png"
            resized.save(destination, format="PNG", optimize=False)
        yield destination, width / source_width, height / source_height


def tesseract_version() -> str:
    return subprocess.run(
        ["tesseract", "--version"], check=True, capture_output=True, text=True
    ).stdout.splitlines()[0]


def run_tesseract(image: Path, language: str) -> tuple[str, float | None, float]:
    started = time.perf_counter()
    result = subprocess.run(
        ["tesseract", str(image), "stdout", "--oem", "1", "--psm", "7", "-l", language, "tsv"],
        check=False,
        capture_output=True,
        text=True,
        timeout=30,
    )
    latency = (time.perf_counter() - started) * 1000
    if result.returncode != 0:
        return "", None, latency
    words: list[str] = []
    confidences: list[float] = []
    for row in result.stdout.splitlines()[1:]:
        columns = row.split("\t")
        if len(columns) < 12 or not columns[11].strip():
            continue
        words.append(columns[11].strip())
        try:
            confidence = float(columns[10])
        except ValueError:
            continue
        if confidence >= 0:
            confidences.append(confidence / 100.0)
    return " ".join(words), sum(confidences) / len(confidences) if confidences else None, latency


def saved_result_payload(result: Any) -> dict[str, Any]:
    """Use PaddleOCR's documented JSON serializer to avoid private fields."""
    with tempfile.TemporaryDirectory() as directory:
        destination = Path(directory) / "result.json"
        result.save_to_json(save_path=str(destination))
        payload = json.loads(destination.read_text(encoding="utf-8"))
    return payload.get("res", payload)


def rectangle_from_polygon(polygon: Any, width: int, height: int) -> dict[str, int] | None:
    points = [(float(point[0]), float(point[1])) for point in polygon]
    if not points:
        return None
    minimum_x, maximum_x = min(point[0] for point in points), max(point[0] for point in points)
    minimum_y, maximum_y = min(point[1] for point in points), max(point[1] for point in points)
    detected_width = max(1.0, maximum_x - minimum_x)
    detected_height = max(1.0, maximum_y - minimum_y)
    # This explicit baseline adapter expands text-line geometry into a plate-like
    # proposal. It is not a specialized plate detector and must pass the gate.
    x1 = max(0, int(round(minimum_x - detected_width * 0.15)))
    x2 = min(width, int(round(maximum_x + detected_width * 0.15)))
    y1 = max(0, int(round(minimum_y - detected_height * 0.75)))
    y2 = min(height, int(round(maximum_y + detected_height * 0.75)))
    return {"x": x1, "y": y1, "width": max(1, x2 - x1), "height": max(1, y2 - y1)}


def download_models() -> None:
    from paddleocr import TextDetection, TextRecognition

    TextDetection(
        model_name=PADDLE_DETECTOR, device="cpu", cpu_threads=4,
        enable_mkldnn=False,
    )
    for model_name in PADDLE_RECOGNIZERS.values():
        TextRecognition(
            model_name=model_name, device="cpu", cpu_threads=4,
            enable_mkldnn=False,
        )


def evaluate_tesseract(
    manifest: dict[str, Any], fixture_root: Path, preprocessing: str
) -> list[dict[str, Any]]:
    predictions = []
    version = tesseract_version()
    for fixture in manifest["fixtures"]:
        if fixture.get("ocr_evaluation_eligible", True) is not True:
            continue
        crop_name = fixture.get("truth_crop_file")
        if not crop_name:
            predictions.append({
                "fixture_id": fixture["fixture_id"], "plate_text_raw": None,
                "plate_text_normalized": None, "confidence": None,
                "abstention_state": "no_plate_found", "latency_ms": 0.0,
            })
            continue
        language = "urd+eng" if fixture.get("script_when_visible") == "Arabic-derived Urdu" else "eng"
        with prepared_ocr_input(fixture_root / crop_name, preprocessing) as prepared:
            raw, confidence, latency = run_tesseract(prepared, language)
        normalized = normalize(raw)
        predictions.append({
            "fixture_id": fixture["fixture_id"], "plate_text_raw": raw or None,
            "plate_text_normalized": normalized or None, "confidence": confidence,
            "abstention_state": "candidate" if normalized else "ocr_abstained",
            "latency_ms": round(latency, 3), "language": language,
            "engine_version": version,
            "preprocessing": preprocessing,
            "normalization_steps": ["Unicode uppercase", "retain alphanumeric code points"],
        })
    return predictions


def evaluate_paddle_recognizer(
    manifest: dict[str, Any], fixture_root: Path, candidate: str, preprocessing: str
) -> list[dict[str, Any]]:
    from paddleocr import TextRecognition

    model_name = PADDLE_RECOGNIZERS[candidate]
    model = TextRecognition(
        model_name=model_name, device="cpu", cpu_threads=4,
        enable_mkldnn=False,
    )
    predictions = []
    for fixture in manifest["fixtures"]:
        if fixture.get("ocr_evaluation_eligible", True) is not True:
            continue
        crop_name = fixture.get("truth_crop_file")
        if not crop_name:
            predictions.append({
                "fixture_id": fixture["fixture_id"], "plate_text_raw": None,
                "plate_text_normalized": None, "confidence": None,
                "abstention_state": "no_plate_found", "latency_ms": 0.0,
            })
            continue
        with prepared_ocr_input(fixture_root / crop_name, preprocessing) as prepared:
            started = time.perf_counter()
            results = list(model.predict(input=str(prepared), batch_size=1))
            latency = (time.perf_counter() - started) * 1000
        payload = saved_result_payload(results[0]) if results else {}
        raw = str(payload.get("rec_text") or "").strip()
        normalized = normalize(raw)
        score = payload.get("rec_score")
        predictions.append({
            "fixture_id": fixture["fixture_id"], "plate_text_raw": raw or None,
            "plate_text_normalized": normalized or None,
            "confidence": float(score) if isinstance(score, (int, float)) else None,
            "abstention_state": "candidate" if normalized else "ocr_abstained",
            "latency_ms": round(latency, 3), "model_name": model_name,
            "preprocessing": preprocessing,
            "normalization_steps": ["Unicode uppercase", "retain alphanumeric code points"],
        })
    return predictions


def evaluate_paddle_detector(
    manifest: dict[str, Any], fixture_root: Path, detector_input_long_side: int
) -> list[dict[str, Any]]:
    from paddleocr import TextDetection

    model = TextDetection(
        model_name=PADDLE_DETECTOR, device="cpu", cpu_threads=4,
        enable_mkldnn=False,
    )
    predictions = []
    for fixture in manifest["fixtures"]:
        if fixture.get("detector_evaluation_eligible", True) is not True:
            continue
        with prepared_detector_input(
            fixture_root / fixture["source_file"], detector_input_long_side
        ) as (prepared, scale_x, scale_y):
            started = time.perf_counter()
            results = list(model.predict(input=str(prepared), batch_size=1))
            latency = (time.perf_counter() - started) * 1000
        payload = saved_result_payload(results[0]) if results else {}
        polygons = payload.get("dt_polys") or []
        scores = [float(score) for score in payload.get("dt_scores") or []]
        regions = []
        for index, polygon in enumerate(polygons):
            original_polygon = [
                [float(point[0]) / scale_x, float(point[1]) / scale_y]
                for point in polygon
            ]
            bounds = rectangle_from_polygon(
                original_polygon, fixture["width_pixels"], fixture["height_pixels"]
            )
            if bounds is not None:
                regions.append({"bounds": bounds, "confidence": scores[index] if index < len(scores) else None})
        predictions.append({
            "fixture_id": fixture["fixture_id"],
            "plate_regions_original_pixels": regions,
            "confidence": max(scores) if scores else None,
            "abstention_state": "candidate" if regions else "no_plate_found",
            "latency_ms": round(latency, 3), "model_name": PADDLE_DETECTOR,
            "detector_input_long_side": detector_input_long_side or "original",
            "coordinate_transform": {"scale_x": scale_x, "scale_y": scale_y, "output": "original_pixels"},
        })
    return predictions


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--generate", action="store_true")
    parser.add_argument("--download-only", action="store_true")
    parser.add_argument(
        "--candidate", choices=("tesseract", "paddle-en", "paddle-arabic", "paddle-detector"),
        default="tesseract",
    )
    parser.add_argument("--fixtures", type=Path, default=Path("/work/fixtures/manifest.json"))
    parser.add_argument("--preprocessing", choices=OCR_PREPROCESSING, default="baseline")
    parser.add_argument("--detector-input-long-side", type=int, choices=(0, 640, 1280), default=0)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if args.generate:
        subprocess.run(["python", "/opt/nexusai-r8/generate_fixtures.py"], check=True)
    if args.download_only:
        download_models()
        print(json.dumps({"status": "downloaded", "models": [PADDLE_DETECTOR, *PADDLE_RECOGNIZERS.values()]}))
        return 0
    manifest = json.loads(args.fixtures.read_text(encoding="utf-8"))
    if args.candidate == "tesseract":
        predictions = evaluate_tesseract(manifest, args.fixtures.parent, args.preprocessing)
        role = "ocr_only_annotated_crop"
    elif args.candidate == "paddle-detector":
        if args.preprocessing != "baseline":
            parser.error("OCR preprocessing variants do not apply to the detector")
        predictions = evaluate_paddle_detector(
            manifest, args.fixtures.parent, args.detector_input_long_side
        )
        role = "localization_baseline_text_detector_adapter"
    else:
        predictions = evaluate_paddle_recognizer(
            manifest, args.fixtures.parent, args.candidate, args.preprocessing
        )
        role = "ocr_only_annotated_crop"
    output = {
        "contract_version": "forensics.ocr-evaluation-output/v1",
        "generated_at": "2026-08-12T00:00:00Z",
        "candidate": args.candidate,
        "candidate_id": args.candidate,
        "candidate_revision": CANDIDATE_REVISIONS[args.candidate],
        "evaluation_role": role,
        "fixture_manifest_sha256": file_sha256(args.fixtures),
        "source_tiers": sorted({str(item.get("tier", "unknown")) for item in manifest.get("fixtures", [])}),
        "evaluation_partition": "all_eligible_fixtures",
        "offline": os.environ.get("NEXUSAI_R8_OFFLINE") == "1",
        "host": {"platform": platform.platform(), "cpu_count": os.cpu_count(), "python": platform.python_version()},
        "preprocessing_variant": args.preprocessing,
        "detector_input_long_side": args.detector_input_long_side or "original",
        "preprocessing_recipe": (
            f"annotated truth crop plus exactly one declared OCR variant ({args.preprocessing}); "
            f"detector uses the full image at long side "
            f"{args.detector_input_long_side or 'original'} and text-box padding baseline"
        ),
        "peak_process_rss_mib": round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024, 3),
        "predictions": predictions,
    }
    destination = args.output or Path(f"/work/results/{args.candidate}-predictions.json")
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(json.dumps(output, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps({"status": "pass", "candidate": args.candidate, "fixtures": len(predictions), "output": str(destination)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
