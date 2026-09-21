#!/usr/bin/env python3
"""Assert the deployed composed-video media contract without persistence."""

from __future__ import annotations

import argparse
import hashlib
import json
import tempfile
import time
from pathlib import Path

try:
    from ingestion.forensic_records.media_pipeline import configured_media_processor
except ModuleNotFoundError:  # Standalone forensic worker image.
    from media_pipeline import configured_media_processor


def temporary_media_paths() -> set[str]:
    root = Path(tempfile.gettempdir())
    paths: set[str] = set()
    for pattern in ("nexusai-video-*", "nexusai-frame-*"):
        paths.update(str(path) for path in root.glob(pattern))
    return paths


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("video", type=Path)
    parser.add_argument("--language", help="Explicit embedded-audio ASR language code")
    parser.add_argument(
        "--allow-no-anpr",
        action="store_true",
        help="Accept a composition whose source frames are not an ANPR fixture.",
    )
    args = parser.parse_args()
    video = args.video.resolve()
    if not video.is_file():
        raise SystemExit(f"video does not exist: {video}")

    before_temporary = temporary_media_paths()
    source_sha256 = hashlib.sha256(video.read_bytes()).hexdigest()
    started = time.perf_counter()
    result = configured_media_processor().process(
        video,
        modality="video",
        evidence_id="non-retained-video-smoke",
        version_id="v1",
        source_sha256=source_sha256,
        source_file=video.name,
        admitted_metadata={
            "acceptance_scope": "non_retained_runtime",
            **({"asr_language": args.language} if args.language else {}),
        },
    )
    wall_seconds = time.perf_counter() - started
    after_temporary = temporary_media_paths()

    observations = list(result.observations)
    observation_ids = [item.observation_id for item in observations]
    if len(observation_ids) != len(set(observation_ids)):
        raise AssertionError("composed video emitted duplicate observation IDs")

    by_type: dict[str, list] = {}
    for observation in observations:
        by_type.setdefault(observation.observation_type, []).append(observation)

    technical = by_type.get("video_technical_observation", [])
    frames = by_type.get("video_sampled_frame_observation", [])
    plates = by_type.get("anpr_ocr_observation", [])
    embedded = by_type.get("video_embedded_audio_observation", [])
    transcripts = [item for item in embedded if item.payload.get("transcript_contract")]

    if len(technical) != 1:
        raise AssertionError("expected exactly one video technical observation")
    probe = technical[0].payload.get("container_probe", {})
    if probe.get("availability") != "available":
        raise AssertionError("ffprobe metadata was unavailable")
    if len(frames) < 2:
        raise AssertionError("expected at least two bounded sampled frames")
    frame_times = [item.citation_locator.get("timestamp_seconds") for item in frames]
    if frame_times[:2] != [0.0, 5.0]:
        raise AssertionError(f"unexpected frame timestamps: {frame_times}")
    if not plates and not args.allow_no_anpr:
        raise AssertionError("expected ANPR output from the reference plate frames")
    if not embedded:
        raise AssertionError("expected extracted embedded-audio observations")
    if not transcripts:
        raise AssertionError("expected timestamped ASR transcript observations")
    if any(item.payload.get("parent_evidence_id") != "non-retained-video-smoke" for item in embedded):
        raise AssertionError("embedded audio lost parent evidence provenance")
    if any(item.payload.get("parent_version_id") != "v1" for item in embedded):
        raise AssertionError("embedded audio lost parent version provenance")
    if any(item.citation_locator.get("source_file") != video.name for item in observations):
        raise AssertionError("an observation lost its parent video source locator")
    if any(item.citation_locator.get("end_seconds") is None for item in transcripts):
        raise AssertionError("an ASR segment lost its end timestamp")
    if any("audio transcription is unavailable" in item for item in result.limitations):
        raise AssertionError("configured ASR was incorrectly reported unavailable")
    if after_temporary != before_temporary:
        raise AssertionError(
            f"temporary media artifacts leaked: {sorted(after_temporary - before_temporary)}"
        )

    print(json.dumps({
        "marker": "ComposedVideoNonRetainedAcceptance=PASS",
        "source_sha256": source_sha256,
        "wall_seconds": round(wall_seconds, 3),
        "readiness": result.readiness,
        "sampled_frame_count": result.metadata.get("sampled_frame_count"),
        "frame_timestamps": frame_times,
        "observation_count": len(observations),
        "observation_ids_unique": True,
        "anpr_observation_count": len(plates),
        "anpr_required": not args.allow_no_anpr,
        "anpr_values": [item.payload.get("normalized_plate_text") for item in plates],
        "embedded_audio_observation_count": len(embedded),
        "transcript_segment_count": len(transcripts),
        "transcript_segments": [
            {
                "start_seconds": item.citation_locator.get("start_seconds"),
                "end_seconds": item.citation_locator.get("end_seconds"),
                "text": item.payload.get("text"),
                "language": item.payload.get("language"),
                "manual_review_required": item.payload.get("manual_review_required"),
            }
            for item in transcripts
        ],
        "temporary_artifacts_cleaned": True,
        "retained_data_mutated": False,
        "limitations": list(result.limitations),
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
