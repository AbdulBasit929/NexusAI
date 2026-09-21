#!/usr/bin/env python3
"""Record the deployed video/ANPR contract without retaining evidence."""

from __future__ import annotations

import argparse
import dataclasses
import hashlib
import json
import tempfile
import time
from pathlib import Path
from typing import Any

try:
    from ingestion.forensic_records.media_pipeline import configured_media_processor
except ModuleNotFoundError:  # Standalone forensic worker image.
    from media_pipeline import configured_media_processor


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def temporary_media_paths() -> set[str]:
    root = Path(tempfile.gettempdir())
    paths: set[str] = set()
    for pattern in ("nexusai-video-*", "nexusai-frame-*"):
        paths.update(str(path) for path in root.glob(pattern))
    return paths


def json_value(value: Any) -> Any:
    if dataclasses.is_dataclass(value):
        return {field.name: json_value(getattr(value, field.name)) for field in dataclasses.fields(value)}
    if isinstance(value, dict):
        return {str(key): json_value(child) for key, child in value.items()}
    if isinstance(value, (list, tuple)):
        return [json_value(child) for child in value]
    return value


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("video", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--language")
    args = parser.parse_args()

    video = args.video.resolve()
    if not video.is_file():
        raise SystemExit(f"video does not exist: {video}")

    source_sha256 = sha256(video)
    before_temporary = temporary_media_paths()
    started = time.perf_counter()
    failure: dict[str, str] | None = None
    result = None
    try:
        result = configured_media_processor().process(
            video,
            modality="video",
            evidence_id="non-retained-mmv1-video-anpr",
            version_id="non-retained-v1",
            source_sha256=source_sha256,
            source_file=video.name,
            admitted_metadata={
                "acceptance_scope": "mmv1_non_retained_positive_control",
                **({"asr_language": args.language} if args.language else {}),
            },
        )
    except Exception as exc:  # The benchmark must preserve a truthful failure record.
        failure = {"type": type(exc).__name__, "message": str(exc)}
    wall_seconds = time.perf_counter() - started
    after_temporary = temporary_media_paths()

    observations = list(result.observations) if result is not None else []
    by_type: dict[str, int] = {}
    for observation in observations:
        by_type[observation.observation_type] = by_type.get(observation.observation_type, 0) + 1
    plates = [item for item in observations if item.observation_type == "anpr_ocr_observation"]
    frames = [item for item in observations if item.observation_type == "video_sampled_frame_observation"]

    report = {
        "contract_version": "nexusai.mmv1.video-anpr-nonretained-benchmark/v1",
        "retained_data_mutated": False,
        "source": {
            "path": str(video),
            "filename": video.name,
            "sha256": source_sha256,
            "size_bytes": video.stat().st_size,
        },
        "runtime": {
            "wall_seconds": round(wall_seconds, 6),
            "temporary_artifacts_cleaned": after_temporary == before_temporary,
            "temporary_artifacts_leaked": sorted(after_temporary - before_temporary),
        },
        "result": {
            "failure": failure,
            "readiness": result.readiness if result is not None else "FAILED",
            "processor_id": result.processor_id if result is not None else None,
            "processor_revision": result.processor_revision if result is not None else None,
            "metadata": json_value(result.metadata) if result is not None else {},
            "limitations": list(result.limitations) if result is not None else [],
            "observation_count": len(observations),
            "observation_type_counts": by_type,
            "observation_ids_unique": len({item.observation_id for item in observations}) == len(observations),
            "sampled_frame_count": len(frames),
            "sampled_frame_timestamps_seconds": [
                item.citation_locator.get("timestamp_seconds") for item in frames
            ],
            "plate_detection_count": len(plates),
            "plates": [
                {
                    "observation_id": item.observation_id,
                    "frame_timestamp_seconds": item.citation_locator.get("timestamp_seconds"),
                    "source_locator": json_value(item.citation_locator),
                    "raw_ocr": item.payload.get("raw_plate_text"),
                    "normalized_plate": item.payload.get("normalized_plate_text"),
                    "detector_confidence": item.payload.get("detection_confidence"),
                    "ocr_confidence": item.payload.get("ocr_confidence"),
                    "bbox": item.citation_locator.get("bbox"),
                    "crop": item.payload.get("crop"),
                    "detector_model": item.payload.get("detector_model"),
                    "ocr_model": item.payload.get("ocr_model"),
                    "processor": item.payload.get("processor"),
                    "processor_revision": item.payload.get("processor_revision"),
                    "warnings": list(item.warnings),
                }
                for item in plates
            ],
            "observations": [json_value(item) for item in observations],
        },
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({
        "output": str(args.output),
        "failure": failure,
        "wall_seconds": round(wall_seconds, 3),
        "sampled_frame_count": len(frames),
        "plate_detection_count": len(plates),
        "observation_type_counts": by_type,
        "temporary_artifacts_cleaned": after_temporary == before_temporary,
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
