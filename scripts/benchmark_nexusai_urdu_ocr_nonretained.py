#!/usr/bin/env python3
"""Create a lawful synthetic Urdu OCR pack and record the current baseline."""

from __future__ import annotations

import argparse
import hashlib
import json
import time
import unicodedata
from pathlib import Path
from typing import Any

from PIL import Image, ImageDraw, ImageFont, features

try:
    from ingestion.forensic_records.media_pipeline import TesseractImageOCRProcessor
except ModuleNotFoundError:  # Standalone forensic worker image.
    from media_pipeline import TesseractImageOCRProcessor


GROUND_TRUTH = {
    "urdu-heading.png": ["پاکستان فرانزک انٹیلی جنس"],
    "urdu-paragraph.png": [
        "یہ ایک واضح اردو عبارت ہے",
        "تمام نتائج کا انسانی جائزہ ضروری ہے",
    ],
    "urdu-english.png": ["NexusAI", "کیس Islamabad", "رپورٹ 2026"],
    "urdu-numbers.png": ["تاریخ 24-08-2026", "وقت 10:35", "نمبر 03001234567"],
    "urdu-scene.png": ["اسلام آباد", "شواہد کا جائزہ"],
    "negative-no-text.png": [],
}


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def normalize(value: str) -> str:
    value = unicodedata.normalize("NFKC", value).casefold()
    return " ".join(value.split())


def levenshtein(left: str, right: str) -> int:
    previous = list(range(len(right) + 1))
    for left_index, left_character in enumerate(left, start=1):
        current = [left_index]
        for right_index, right_character in enumerate(right, start=1):
            current.append(min(
                current[-1] + 1,
                previous[right_index] + 1,
                previous[right_index - 1] + int(left_character != right_character),
            ))
        previous = current
    return previous[-1]


def character_error_rate(expected: list[str], actual: list[str]) -> float:
    left = normalize("\n".join(expected))
    right = normalize("\n".join(actual))
    if not left:
        return 0.0 if not right else 1.0
    return levenshtein(left, right) / len(left)


def draw_rtl(draw: ImageDraw.ImageDraw, xy: tuple[int, int], text: str, font: ImageFont.FreeTypeFont) -> None:
    kwargs: dict[str, Any] = {"anchor": "ra"}
    if features.check("raqm"):
        kwargs.update({"direction": "rtl", "language": "ur"})
    draw.text(xy, text, fill="black", font=font, **kwargs)


def create_fixtures(root: Path) -> dict[str, Path]:
    root.mkdir(parents=True, exist_ok=True)
    font_path = Path("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
    heading_font = ImageFont.truetype(str(font_path), 76)
    body_font = ImageFont.truetype(str(font_path), 54)
    latin_font = ImageFont.truetype(str(font_path), 48)
    fixtures: dict[str, Path] = {}

    heading = Image.new("RGB", (1600, 500), "white")
    draw_rtl(ImageDraw.Draw(heading), (1510, 220), GROUND_TRUTH["urdu-heading.png"][0], heading_font)
    fixtures["urdu-heading.png"] = root / "urdu-heading.png"
    heading.save(fixtures["urdu-heading.png"])

    paragraph = Image.new("RGB", (1800, 700), "white")
    draw = ImageDraw.Draw(paragraph)
    draw_rtl(draw, (1710, 220), GROUND_TRUTH["urdu-paragraph.png"][0], body_font)
    draw_rtl(draw, (1710, 360), GROUND_TRUTH["urdu-paragraph.png"][1], body_font)
    fixtures["urdu-paragraph.png"] = root / "urdu-paragraph.png"
    paragraph.save(fixtures["urdu-paragraph.png"])

    mixed = Image.new("RGB", (1800, 700), "white")
    draw = ImageDraw.Draw(mixed)
    draw.text((90, 100), "NexusAI", fill="black", font=latin_font)
    draw_rtl(draw, (1710, 260), "کیس Islamabad", body_font)
    draw_rtl(draw, (1710, 410), "رپورٹ 2026", body_font)
    fixtures["urdu-english.png"] = root / "urdu-english.png"
    mixed.save(fixtures["urdu-english.png"])

    numbers = Image.new("RGB", (1800, 760), "white")
    draw = ImageDraw.Draw(numbers)
    for index, line in enumerate(GROUND_TRUTH["urdu-numbers.png"]):
        draw_rtl(draw, (1710, 170 + index * 170), line, body_font)
    fixtures["urdu-numbers.png"] = root / "urdu-numbers.png"
    numbers.save(fixtures["urdu-numbers.png"])

    scene = Image.new("RGB", (1600, 900), (224, 229, 235))
    draw = ImageDraw.Draw(scene)
    draw.rectangle((180, 170, 1420, 690), fill=(248, 246, 236), outline=(70, 75, 85), width=8)
    draw_rtl(draw, (1320, 350), GROUND_TRUTH["urdu-scene.png"][0], heading_font)
    draw_rtl(draw, (1320, 530), GROUND_TRUTH["urdu-scene.png"][1], body_font)
    fixtures["urdu-scene.png"] = root / "urdu-scene.png"
    scene.save(fixtures["urdu-scene.png"])

    negative = Image.new("RGB", (1600, 900), (222, 227, 232))
    draw = ImageDraw.Draw(negative)
    draw.rectangle((120, 130, 1480, 770), fill=(236, 238, 241), outline=(160, 165, 170), width=5)
    fixtures["negative-no-text.png"] = root / "negative-no-text.png"
    negative.save(fixtures["negative-no-text.png"])
    return fixtures


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    fixture_root = args.output.parent / "urdu-ocr-fixtures"
    fixtures = create_fixtures(fixture_root)
    processor = TesseractImageOCRProcessor.from_environment()
    results: dict[str, Any] = {}
    aggregate_expected: list[str] = []
    aggregate_actual: list[str] = []

    for name, path in fixtures.items():
        started = time.perf_counter()
        result = processor.process_image(
            path,
            evidence_id=f"non-retained-urdu-ocr-{name}",
            version_id="non-retained-v1",
            source_sha256=sha256(path),
            source_file=name,
        )
        latency_ms = (time.perf_counter() - started) * 1000
        actual = [str(item.payload.get("raw_text") or "") for item in result.observations]
        expected = GROUND_TRUTH[name]
        if expected:
            aggregate_expected.extend(expected)
            aggregate_actual.extend(actual)
        width, height = Image.open(path).size
        bounds_valid = all(
            0 <= item.payload["bbox"]["x"] < width
            and 0 <= item.payload["bbox"]["y"] < height
            and item.payload["bbox"]["width"] > 0
            and item.payload["bbox"]["height"] > 0
            and item.payload["bbox"]["x"] + item.payload["bbox"]["width"] <= width
            and item.payload["bbox"]["y"] + item.payload["bbox"]["height"] <= height
            for item in result.observations
        )
        results[name] = {
            "fixture_path": str(path),
            "fixture_sha256": sha256(path),
            "license": "NexusAI repository-owned generated benchmark fixture",
            "ground_truth": expected,
            "recognized_text": actual,
            "character_error_rate": character_error_rate(expected, actual),
            "latency_ms": round(latency_ms, 3),
            "readiness": result.readiness,
            "observation_count": len(result.observations),
            "script_families": [item.payload.get("script_family") for item in result.observations],
            "confidences": [item.confidence for item in result.observations],
            "bboxes": [item.payload.get("bbox") for item in result.observations],
            "bbox_within_source": bounds_valid,
            "limitations": list(result.limitations),
        }

    report = {
        "contract_version": "nexusai.mmv1.urdu-ocr-baseline/v1",
        "scope": "synthetic_nonretained_repository_owned",
        "retained_data_mutated": False,
        "baseline_recorded_before_tuning": True,
        "fixture_rendering": {
            "pillow_raqm_available": features.check("raqm"),
            "font": "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
            "ground_truth_separate_from_processor_output": True,
        },
        "processor": {
            "processor_id": processor.processor_id,
            "processor_revision": processor.processor_revision,
            "runtime_version": processor.runtime_version,
            "languages": processor.language_list,
            "page_segmentation_mode": processor.page_segmentation_mode,
            "tessdata_revision": processor.tessdata_revision,
            "tessdata_sha256": processor.tessdata_hashes,
        },
        "fixtures": results,
        "aggregate_nonnegative_character_error_rate": character_error_rate(
            aggregate_expected, aggregate_actual
        ),
        "negative_control_pass": results["negative-no-text.png"]["observation_count"] == 0,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({
        "output": str(args.output),
        "aggregate_cer": report["aggregate_nonnegative_character_error_rate"],
        "negative_control_pass": report["negative_control_pass"],
        "fixtures": {
            name: {
                "cer": value["character_error_rate"],
                "recognized_text": value["recognized_text"],
                "latency_ms": value["latency_ms"],
            }
            for name, value in results.items()
        },
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
