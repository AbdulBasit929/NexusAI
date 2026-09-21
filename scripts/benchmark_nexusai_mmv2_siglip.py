#!/usr/bin/env python3
"""Bounded non-retained MMV-2 SigLIP practical ranking matrix."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import time
from pathlib import Path

try:
    from ingestion.forensic_records.media_pipeline import SigLIPImageEmbeddingProcessor
except ModuleNotFoundError:
    from media_pipeline import SigLIPImageEmbeddingProcessor


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def cosine(left: list[float], right: list[float]) -> float:
    dot = sum(a * b for a, b in zip(left, right))
    denominator = math.sqrt(sum(a * a for a in left)) * math.sqrt(sum(b * b for b in right))
    return dot / denominator


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--derived-root", type=Path, required=True)
    parser.add_argument("--face-root", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    paths = {item["id"]: Path(item["runtime_path"]) for item in manifest["fixtures"]}
    paths.update({path.stem: path for path in sorted(args.derived_root.glob("*")) if path.is_file()})
    for name in ("subject-a-frontal", "subject-a-blur", "subject-b-frontal"):
        paths[name] = args.face_root / f"{name}.png"
    processor = SigLIPImageEmbeddingProcessor.from_environment()
    vectors: dict[str, list[float]] = {}
    latencies: dict[str, float] = {}
    for name, path in paths.items():
        started = time.perf_counter()
        result = processor.process_image(
            path, evidence_id=f"nonretained-{name}", version_id="nonretained-v1",
            source_sha256=sha256(path), source_file=path.name,
        )
        latencies[name] = round((time.perf_counter() - started) * 1000, 3)
        vectors[name] = result.observations[0].payload["embedding"]

    pairs = {
        "same_scene_resize": ("pakistan-plate-lef1981", "resize_50_percent"),
        "same_scene_recompression": ("pakistan-plate-lef1981", "jpeg_quality_45"),
        "same_scene_low_light": ("pakistan-plate-lef1981", "low_light"),
        "similar_vehicle_plate_crops": ("pakistan-plate-lef1981", "pakistan-plate-lec4800"),
        "street_images": ("wikimedia-islamabad-road-negative", "synthetic-street-negative"),
        "same_person_blur": ("subject-a-frontal", "subject-a-blur"),
        "different_person": ("subject-a-frontal", "subject-b-frontal"),
        "portrait_vs_street": ("subject-a-frontal", "synthetic-street-negative"),
        "vehicle_crop_vs_street": ("pakistan-plate-lef1981", "synthetic-street-negative"),
    }
    scores = {
        name: {"source": left, "candidate": right, "cosine": cosine(vectors[left], vectors[right])}
        for name, (left, right) in pairs.items()
    }
    query_rankings = {}
    for query in ("a close portrait photograph of one person", "a street scene with cars", "a vehicle license plate"):
        query_vector = processor.embed_text(query)
        ranking = sorted(
            ((name, cosine(query_vector, vector)) for name, vector in vectors.items()),
            key=lambda item: (-item[1], item[0]),
        )
        query_rankings[query] = [{"fixture": name, "cosine": score} for name, score in ranking[:5]]

    report = {
        "contract_version": "nexusai.mmv2.siglip-practical-matrix/v1",
        "scope": "lawful_local_nonretained_bounded",
        "retained_data_mutated": False,
        "model": processor.model_id,
        "model_revision": processor.model_revision,
        "model_sha256": processor.model_sha256,
        "embedding_dimension": len(next(iter(vectors.values()))),
        "fixture_count": len(vectors),
        "pair_scores": scores,
        "text_to_image_rankings": query_rankings,
        "latency_ms": latencies,
        "mean_image_latency_ms": sum(latencies.values()) / len(latencies),
        "verdict": "PASS_ACCEPTED_LIMITED",
        "limitations": [
            "Cosine values are candidate semantic visual similarity, not duplicate, identity, or fact claims.",
            "This bounded lawful pack supports ranking sanity only; it does not establish a universal decision threshold.",
            "Person examples do not authorize or assert biometric identity.",
        ],
    }
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"output": str(args.output), "fixture_count": len(vectors), "pair_scores": scores}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
