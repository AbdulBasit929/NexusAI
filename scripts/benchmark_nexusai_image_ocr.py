#!/usr/bin/env python3
"""Non-retained benchmark for the bounded SigLIP and Tesseract image roles."""

from __future__ import annotations

import argparse
import hashlib
import importlib.metadata
import json
import math
import os
import resource
import time
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont, features

try:
    from ingestion.forensic_records.media_pipeline import (
        MediaProcessingError,
        SigLIPImageEmbeddingProcessor,
        TesseractImageOCRProcessor,
    )
except ModuleNotFoundError:  # supports validation against the packaged worker image
    from media_pipeline import (
        MediaProcessingError,
        SigLIPImageEmbeddingProcessor,
        TesseractImageOCRProcessor,
    )


MODEL_REVISION = "7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed"
MODEL_SHA256 = "2c63cb7d1f2e95ba501893cbb8faeb4ea9a3af295498d35097126228659c2af8"
TESSDATA_REVISION = "65727574dfcd264acbb0c3e07860e4e9e9b22185"
TESSDATA_HASHES = {
    "eng": "7d4322bd2a7749724879683fc3912cb542f19906c83bcc1a52132556427170b2",
    "urd": "62e8250ce2a994106e313a82e26a516a39e2cf159d0ce3c5b5008387fd0d555f",
}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--model-path", type=Path, required=True)
    parser.add_argument("--tessdata-path", type=Path, required=True)
    parser.add_argument("--face-fixture-path", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    started_rss = peak_rss_mib()
    started = time.perf_counter()
    embedder = SigLIPImageEmbeddingProcessor(
        args.model_path,
        model_id="google/siglip-base-patch16-224",
        model_revision=MODEL_REVISION,
        model_sha256=MODEL_SHA256,
        cpu_threads=1,
    )
    constructed_ms = elapsed_ms(started)

    fixture_names = [
        "subject-a-frontal.png",
        "subject-a-blur.png",
        "subject-a-pose.png",
        "subject-b-frontal.png",
        "group-four.png",
        "no-face-street.png",
    ]
    vectors: dict[str, list[float]] = {}
    latencies: dict[str, float] = {}
    for fixture_name in fixture_names:
        fixture = args.face_fixture_path / fixture_name
        started = time.perf_counter()
        result = embedder.process_image(
            fixture,
            evidence_id="00000000-0000-0000-0000-000000000001",
            version_id="00000000-0000-0000-0000-000000000002",
            source_sha256=sha256(fixture),
            source_file=fixture_name,
        )
        latencies[fixture_name] = elapsed_ms(started)
        observation = result.observations[0]
        vectors[fixture_name] = observation.payload["embedding"]
        if observation.payload["embedding_dimension"] != 768:
            raise RuntimeError("unexpected SigLIP embedding dimension")

    query_texts = ["a close portrait photograph of one person", "a street scene with no people", "پاکستان کی سڑک"]
    text_results: dict[str, list[dict[str, float | str]]] = {}
    for query in query_texts:
        query_vector = embedder.embed_text(query)
        ranked = sorted(
            ((name, cosine(query_vector, vector)) for name, vector in vectors.items()),
            key=lambda item: (-item[1], item[0]),
        )
        text_results[query] = [{"fixture": name, "cosine": score} for name, score in ranked]

    image_pairs = {
        "same_subject_blur": ("subject-a-frontal.png", "subject-a-blur.png"),
        "same_subject_pose": ("subject-a-frontal.png", "subject-a-pose.png"),
        "different_subject": ("subject-a-frontal.png", "subject-b-frontal.png"),
        "portrait_vs_street": ("subject-a-frontal.png", "no-face-street.png"),
    }
    pair_scores = {
        name: cosine(vectors[left], vectors[right]) for name, (left, right) in image_pairs.items()
    }

    fixture_output = args.output.parent / "synthetic-ocr"
    fixture_output.mkdir(parents=True, exist_ok=True)
    english_path, mixed_path, urdu_path, blank_path = create_ocr_fixtures(fixture_output)
    if fixture_output.is_dir():
        ocr_gold = {
            "english-scene.png": ["NEXUS AI EVIDENCE", "Islamabad Case 2026", "REVIEW REQUIRED"],
            "mixed-english-urdu.png": ["NEXUS AI EVIDENCE", "Islamabad Case 2026", "REVIEW REQUIRED", "پاکستان"],
            "urdu-scene.png": ["پاکستان زندہ باد", "اسلام آباد"],
            "blank.png": [],
        }
        ocr = TesseractImageOCRProcessor(
            Path("/usr/bin/tesseract"),
            args.tessdata_path,
            tessdata_revision=TESSDATA_REVISION,
            tessdata_hashes=TESSDATA_HASHES,
            max_observations=32,
        )
        ocr_results = {}
        for fixture_path in (english_path, mixed_path, urdu_path, blank_path):
            started = time.perf_counter()
            result = ocr.process_image(
                fixture_path,
                evidence_id="00000000-0000-0000-0000-000000000003",
                version_id="00000000-0000-0000-0000-000000000004",
                source_sha256=sha256(fixture_path),
                source_file=fixture_path.name,
            )
            predictions = [item.payload["raw_text"] for item in result.observations]
            gold = ocr_gold[fixture_path.name]
            ocr_results[fixture_path.name] = {
                "latency_ms": elapsed_ms(started),
                "readiness": result.readiness,
                "observation_count": len(result.observations),
                "raw_text": predictions,
                "gold_text": gold,
                "normalized_exact_accuracy": exact_accuracy(gold, predictions),
                "normalized_character_error_rate": character_error_rate(gold, predictions),
                "script_families": [item.payload["script_family"] for item in result.observations],
                "bounds": [item.payload["bbox"] for item in result.observations],
                "confidences": [item.confidence for item in result.observations],
                "limitations": list(result.limitations),
            }
        corrupt_path = fixture_output / "corrupt.png"
        corrupt_path.write_bytes(b"not-an-image")
        ocr_results["corrupt.png"] = {"error": capture_error(lambda: ocr.process_image(
            corrupt_path,
            evidence_id="00000000-0000-0000-0000-000000000003",
            version_id="00000000-0000-0000-0000-000000000004",
            source_sha256=sha256(corrupt_path),
            source_file=corrupt_path.name,
        ))}
        bounded_ocr = TesseractImageOCRProcessor(
            Path("/usr/bin/tesseract"), args.tessdata_path,
            tessdata_revision=TESSDATA_REVISION, tessdata_hashes=TESSDATA_HASHES,
            max_input_bytes=10,
        )
        ocr_results["oversized_control"] = {"error": capture_error(lambda: bounded_ocr.process_image(
            english_path,
            evidence_id="00000000-0000-0000-0000-000000000003",
            version_id="00000000-0000-0000-0000-000000000004",
            source_sha256=sha256(english_path),
            source_file=english_path.name,
        ))}

    package_versions = {
        name: importlib.metadata.version(name)
        for name in ("torch", "transformers", "safetensors", "sentencepiece", "Pillow")
    }
    verdict = "M1_ACCEPTABLE_LIMITED"
    if pair_scores["same_subject_blur"] <= pair_scores["portrait_vs_street"]:
        verdict = "REJECT"
    if ocr_results["english-scene.png"]["observation_count"] == 0:
        verdict = "REJECT"
    if ocr_results["blank.png"]["observation_count"] != 0:
        verdict = "REJECT"
    report = {
        "contract_version": "nexusai.post-bfa.image-ocr-benchmark/v1",
        "scope": "synthetic_nonretained_disposable",
        "verdict": verdict,
        "semantic_image_embedding": {
            "model": "google/siglip-base-patch16-224",
            "revision": MODEL_REVISION,
            "sha256": MODEL_SHA256,
            "model_bytes": (args.model_path / "model.safetensors").stat().st_size,
            "complete_role_bytes": sum(path.stat().st_size for path in args.model_path.iterdir() if path.is_file()),
            "license": "Apache-2.0",
            "embedding_dimension": 768,
            "constructor_checksum_latency_ms": constructed_ms,
            "image_latency_ms": latencies,
            "pair_cosine": pair_scores,
            "text_to_image_rankings": text_results,
            "english_text_to_image_top1_accuracy": float(
                text_results[query_texts[0]][0]["fixture"].startswith("subject-")
                and text_results[query_texts[1]][0]["fixture"] == "no-face-street.png"
            ),
            "urdu_street_query_top1_correct": text_results[query_texts[2]][0]["fixture"] == "no-face-street.png",
            "limitations": [
                "Scores are candidate semantic visual similarity, not evidence identity or fact.",
                "English WebLI pretraining makes Urdu and Roman-Urdu text queries LIMITED.",
                "Production API text-to-image execution is not exposed in this slice.",
            ],
        },
        "general_image_ocr": {
            "runtime": ocr.runtime_version,
            "tessdata_revision": TESSDATA_REVISION,
            "tessdata_sha256": TESSDATA_HASHES,
            "license": "Apache-2.0",
            "pillow_raqm_available": features.check("raqm"),
            "fixtures": ocr_results,
            "limitations": [
                "General scene OCR is LIMITED and does not replace FastALPR plate OCR.",
                "Script family is not a language identification claim.",
            ],
        },
        "resources": {
            "peak_rss_before_mib": started_rss,
            "peak_rss_after_mib": peak_rss_mib(),
            "cpu_threads": 1,
            "package_versions": package_versions,
        },
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({
        "verdict": verdict,
        "output": str(args.output),
        "pair_cosine": pair_scores,
        "ocr_counts": {name: value.get("observation_count") for name, value in ocr_results.items()},
        "peak_rss_mib": report["resources"]["peak_rss_after_mib"],
    }, ensure_ascii=False))
    return 0 if verdict != "REJECT" else 1


def create_ocr_fixtures(root: Path) -> tuple[Path, Path, Path, Path]:
    font_path = Path("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
    font = ImageFont.truetype(str(font_path), 52)
    small = ImageFont.truetype(str(font_path), 42)
    english = Image.new("RGB", (1200, 600), "white")
    draw = ImageDraw.Draw(english)
    draw.text((55, 60), "NEXUS AI EVIDENCE", fill="black", font=font)
    draw.text((55, 160), "Islamabad Case 2026", fill="black", font=small)
    draw.text((55, 250), "REVIEW REQUIRED", fill="black", font=small)
    english_path = root / "english-scene.png"
    english.save(english_path)

    mixed = english.copy()
    draw = ImageDraw.Draw(mixed)
    urdu = "پاکستان"
    kwargs = {"direction": "rtl", "language": "ur"} if features.check("raqm") else {}
    urdu_font = ImageFont.truetype(str(font_path), 72)
    draw.text((1100, 450), urdu, anchor="rm", fill="black", font=urdu_font, **kwargs)
    mixed_path = root / "mixed-english-urdu.png"
    mixed.save(mixed_path)

    urdu_only = Image.new("RGB", (1200, 600), "white")
    draw = ImageDraw.Draw(urdu_only)
    draw.text((1100, 250), "پاکستان زندہ باد", anchor="rm", fill="black", font=urdu_font, **kwargs)
    draw.text((1100, 400), "اسلام آباد", anchor="rm", fill="black", font=urdu_font, **kwargs)
    urdu_path = root / "urdu-scene.png"
    urdu_only.save(urdu_path)

    blank = Image.new("RGB", (1200, 600), "white")
    blank_path = root / "blank.png"
    blank.save(blank_path)
    return english_path, mixed_path, urdu_path, blank_path


def cosine(left: list[float], right: list[float]) -> float:
    dot = sum(a * b for a, b in zip(left, right))
    left_norm = math.sqrt(sum(value * value for value in left))
    right_norm = math.sqrt(sum(value * value for value in right))
    return dot / (left_norm * right_norm)


def normalized_text(value: str) -> str:
    return " ".join(value.casefold().split())


def exact_accuracy(gold: list[str], predictions: list[str]) -> float:
    if not gold:
        return 1.0 if not predictions else 0.0
    matches = sum(
        normalized_text(expected) == normalized_text(predicted)
        for expected, predicted in zip(gold, predictions)
    )
    return matches / max(len(gold), len(predictions))


def character_error_rate(gold: list[str], predictions: list[str]) -> float:
    expected = "\n".join(normalized_text(value) for value in gold)
    predicted = "\n".join(normalized_text(value) for value in predictions)
    if not expected:
        return 0.0 if not predicted else 1.0
    return levenshtein(expected, predicted) / len(expected)


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


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def capture_error(callback) -> str | None:
    try:
        callback()
    except MediaProcessingError as exc:
        return str(exc)
    return None


def peak_rss_mib() -> float:
    return resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024


def elapsed_ms(started: float) -> float:
    return round((time.perf_counter() - started) * 1000, 3)


if __name__ == "__main__":
    raise SystemExit(main())
