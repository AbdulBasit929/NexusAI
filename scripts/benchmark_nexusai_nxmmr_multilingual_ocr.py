#!/usr/bin/env python3
"""Generate and score a small model-independent printed multilingual OCR pack."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import statistics
import sys
import time
import unicodedata
from pathlib import Path
from typing import Any

from PIL import Image, ImageDraw, ImageFont, features

try:
    import resource
except ImportError:
    resource = None


REPOSITORY = Path(__file__).resolve().parents[1]
if str(REPOSITORY) not in sys.path:
    sys.path.insert(0, str(REPOSITORY))

from ingestion.forensic_records.multilingual_ocr import (  # noqa: E402
    PaddleMultilingualOCRProcessor,
    directory_digest,
)


PACK = {
    "english": [
        ["Investigation Workspace", "Reference REF-2026-01"],
        ["Evidence Review", "Case NX-204"],
        ["Islamabad Office", "Date 30-08-2026"],
        ["Vehicle Observation", "Candidate ABC-123"],
        ["Analyst Notes", "Phone 03001234567"],
    ],
    "urdu": [
        ["پاکستان فرانزک انٹیلی جنس", "حوالہ ۲۰۲۶"],
        ["شواہد کا جائزہ", "اسلام آباد"],
        ["یہ ایک واضح اردو عبارت ہے", "انسانی جائزہ ضروری ہے"],
        ["تاریخ ۳۰-۰۸-۲۰۲۶", "وقت ۱۰:۳۵"],
        ["کیس کی تفصیل", "رپورٹ نمبر ۷۸۶"],
    ],
    "mixed": [
        ["NexusAI", "کیس Islamabad", "Reference REF-2026-02"],
        ["Evidence شواہد", "Case NX-205"],
        ["رپورٹ 2026", "Office Lahore"],
        ["Candidate ABC-123", "جائزہ ضروری ہے"],
        ["Phone 03001234567", "رابطہ نمبر"],
    ],
}


def normalize(value: str) -> str:
    return " ".join(unicodedata.normalize("NFKC", value).casefold().split())


def edit_distance(left: list[str] | str, right: list[str] | str) -> int:
    previous = list(range(len(right) + 1))
    for row, left_item in enumerate(left, 1):
        current = [row]
        for column, right_item in enumerate(right, 1):
            current.append(min(
                current[-1] + 1,
                previous[column] + 1,
                previous[column - 1] + int(left_item != right_item),
            ))
        previous = current
    return previous[-1]


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def render_pack(root: Path) -> dict[str, Any]:
    root.mkdir(parents=True, exist_ok=True)
    font = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 52)
    fixtures = []
    for stratum, samples in PACK.items():
        for index, lines in enumerate(samples, start=1):
            image = Image.new("RGB", (1600, 650), "white")
            draw = ImageDraw.Draw(image)
            for line_index, line in enumerate(lines):
                y = 110 + line_index * 145
                is_urdu = any("\u0600" <= character <= "\u06ff" for character in line)
                if is_urdu and features.check("raqm"):
                    draw.text((1510, y), line, fill="black", font=font, anchor="ra", direction="rtl", language="ur")
                elif is_urdu:
                    draw.text((1510, y), line, fill="black", font=font, anchor="ra")
                else:
                    draw.text((90, y), line, fill="black", font=font)
            path = root / f"{stratum}-{index:02d}.png"
            image.save(path, format="PNG", optimize=False)
            fixtures.append({
                "fixture_id": f"{stratum}-{index:02d}",
                "stratum": stratum,
                "source_file": path.name,
                "source_sha256": sha256_file(path),
                "expected_lines": lines,
                "oracle": "deterministic_rendered_text_fixed_before_candidate_inference",
            })
    manifest = {
        "contract_version": "nexusai.nxmmr.multilingual-ocr-fixture-pack/v1",
        "generator": "nxmmr-anpr-ocr-vertical-v1",
        "fixture_count": len(fixtures),
        "stratum_counts": {key: len(value) for key, value in PACK.items()},
        "pillow_raqm_available": features.check("raqm"),
        "font": "DejaVuSans.ttf",
        "fixtures": fixtures,
    }
    (root / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return manifest


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fixture-root", type=Path, required=True)
    parser.add_argument("--detector-dir", type=Path, required=True)
    parser.add_argument("--english-dir", type=Path, required=True)
    parser.add_argument("--arabic-dir", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--generate", action="store_true")
    args = parser.parse_args()
    manifest = render_pack(args.fixture_root) if args.generate else json.loads(
        (args.fixture_root / "manifest.json").read_text(encoding="utf-8")
    )
    os.environ.update({
        "FORENSIC_PADDLE_DETECTOR_MODEL_DIR": str(args.detector_dir),
        "FORENSIC_PADDLE_ENGLISH_MODEL_DIR": str(args.english_dir),
        "FORENSIC_PADDLE_ARABIC_MODEL_DIR": str(args.arabic_dir),
        "FORENSIC_PADDLE_DETECTOR_TREE_SHA256": directory_digest(args.detector_dir),
        "FORENSIC_PADDLE_ENGLISH_TREE_SHA256": directory_digest(args.english_dir),
        "FORENSIC_PADDLE_ARABIC_MULTILINGUAL_TREE_SHA256": directory_digest(args.arabic_dir),
    })
    processor = PaddleMultilingualOCRProcessor.from_environment()
    wall_started = time.perf_counter()
    cpu_started = time.process_time()
    rows = []
    for fixture in manifest["fixtures"]:
        path = args.fixture_root / fixture["source_file"]
        started = time.perf_counter()
        result = processor.process_image(
            path,
            evidence_id=f"non-retained-{fixture['fixture_id']}",
            version_id=fixture["source_sha256"],
            source_sha256=fixture["source_sha256"],
            source_file=fixture["source_file"],
        )
        expected = normalize("\n".join(fixture["expected_lines"]))
        actual_lines = [item.payload["raw_text"] for item in result.observations]
        actual = normalize("\n".join(actual_lines))
        expected_words, actual_words = expected.split(), actual.split()
        identifiers = [
            value for item in result.observations
            for value in item.payload["identifiers_preserved"]
        ]
        expected_identifiers = [
            token.strip(".,:;") for line in fixture["expected_lines"] for token in line.split()
            if any(character.isdigit() for character in token)
        ]
        rows.append({
            "fixture_id": fixture["fixture_id"],
            "stratum": fixture["stratum"],
            "expected_lines": fixture["expected_lines"],
            "actual_lines": actual_lines,
            "line_exact": actual == expected,
            "character_errors": edit_distance(expected, actual),
            "reference_characters": len(expected),
            "word_errors": edit_distance(expected_words, actual_words),
            "reference_words": len(expected_words),
            "expected_identifiers": expected_identifiers,
            "observed_identifiers": identifiers,
            "identifier_exact": all(value in identifiers for value in expected_identifiers),
            "directions": [item.payload["display_direction"] for item in result.observations],
            "region_count": len(result.observations),
            "latency_ms": round((time.perf_counter() - started) * 1000, 3),
            "result_state": result.metadata["result_state"],
        })
    strata = {}
    for name in PACK:
        selected = [row for row in rows if row["stratum"] == name]
        chars = sum(row["reference_characters"] for row in selected)
        words = sum(row["reference_words"] for row in selected)
        strata[name] = {
            "samples": len(selected),
            "cer": sum(row["character_errors"] for row in selected) / chars,
            "wer": sum(row["word_errors"] for row in selected) / words,
            "line_exact_rate": sum(row["line_exact"] for row in selected) / len(selected),
            "identifier_exact_rate": sum(row["identifier_exact"] for row in selected) / len(selected),
            "mean_latency_ms": statistics.fmean(row["latency_ms"] for row in selected),
        }
    report = {
        "contract_version": "nexusai.nxmmr.multilingual-ocr-development/v1",
        "scope": "generated_nonretained_printed_fixture_pack",
        "oracle": "deterministic_text_rendered_before_candidate_inference",
        "manifest_sha256": sha256_file(args.fixture_root / "manifest.json"),
        "processor": processor.processor_id,
        "processor_revision": processor.processor_revision,
        "model_tree_sha256": processor.model_hashes,
        "strata": strata,
        "results": rows,
        "resource": {
            "wall_seconds": round(time.perf_counter() - wall_started, 6),
            "process_cpu_seconds": round(time.process_time() - cpu_started, 6),
            "peak_process_rss_mib": round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024, 3) if resource else None,
            "cpu_threads": 4,
        },
        "promotion_authority": "FIXTURE_CERTIFIED_ONLY",
        "handwriting_evaluated": False,
        "retained_state_mutated": False,
        "activity_mutated": False,
        "database_or_volumes_mutated": False,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"strata": strata, "resource": report["resource"]}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
