#!/usr/bin/env python3
"""Run bounded, non-retained probes through the configured media processor."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
from typing import Any

from media_pipeline import configured_media_processor


def _dimension(value: Any) -> int | None:
    if isinstance(value, list) and value and all(isinstance(item, (int, float)) for item in value):
        return len(value)
    return None


def _summarize(value: Any, path: str = "") -> dict[str, Any]:
    summary: dict[str, Any] = {}
    if isinstance(value, dict):
        for key, child in value.items():
            child_path = f"{path}.{key}" if path else key
            lowered = key.lower()
            dimension = _dimension(child)
            if dimension is not None and ("embedding" in lowered or "vector" in lowered):
                summary[f"{child_path}_dimension"] = dimension
            elif lowered in {
                "normalized_plate",
                "normalized_plate_text",
                "raw_plate_text",
                "raw_text",
                "text",
                "roman_urdu",
                "roman_urdu_text",
                "language",
                "requested_language",
                "detected_language",
                "start_seconds",
                "end_seconds",
                "face_count",
            } and isinstance(child, (str, int, float, bool)):
                summary[child_path] = child
            summary.update(_summarize(child, child_path))
    elif isinstance(value, list):
        for index, child in enumerate(value[:8]):
            summary.update(_summarize(child, f"{path}[{index}]"))
    return summary


def _probe(processor: Any, path: Path, modality: str, language: str | None) -> dict[str, Any]:
    source = path.read_bytes()
    digest = hashlib.sha256(source).hexdigest()
    admitted_metadata = {"asr_language": language} if language else {}
    result = processor.process(
        path,
        modality=modality,
        evidence_id="00000000-0000-0000-0000-000000000001",
        version_id="00000000-0000-0000-0000-000000000002",
        source_sha256=digest,
        source_file=path.name,
        admitted_metadata=admitted_metadata,
    )
    observations = []
    for observation in result.observations:
        observations.append(
            {
                "contract_version": observation.contract_version,
                "observation_type": observation.observation_type,
                "confidence": observation.confidence,
                "summary": _summarize(observation.payload),
                "warnings": list(observation.warnings),
            }
        )
    return {
        "path": str(path),
        "sha256": digest,
        "modality": modality,
        "readiness": result.readiness,
        "processor": result.processor_id,
        "processor_revision": result.processor_revision,
        "limitations": list(result.limitations),
        "observation_count": len(observations),
        "observations": observations,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--plate-image", type=Path, required=True)
    parser.add_argument("--face-image", type=Path, required=True)
    parser.add_argument("--audio", type=Path, required=True)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()

    processor = configured_media_processor()
    report = {
        "contract_version": "nexusai.media-role-non-retained-probe/v1",
        "retained_mutation": False,
        "probes": [
            _probe(processor, args.plate_image, "image", None),
            _probe(processor, args.face_image, "image", None),
            _probe(processor, args.audio, "audio", "ur"),
        ],
    }
    rendered = json.dumps(report, ensure_ascii=False, indent=2)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered + "\n", encoding="utf-8")
    print(rendered)


if __name__ == "__main__":
    main()
