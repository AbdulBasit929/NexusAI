#!/usr/bin/env python3
"""Run configured NexusAI ASR against local files without persistence."""

from __future__ import annotations

import argparse
import hashlib
import json
import time
from pathlib import Path

try:
    from ingestion.forensic_records.media_pipeline import configured_media_processor
except ModuleNotFoundError:  # Standalone forensic worker image.
    from media_pipeline import configured_media_processor


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("audio", type=Path, nargs="+")
    parser.add_argument("--language", help="Explicit ASR language code; omit for automatic detection")
    args = parser.parse_args()

    processor = configured_media_processor()
    results = []
    for requested_path in args.audio:
        audio = requested_path.resolve()
        if not audio.is_file():
            raise SystemExit(f"audio does not exist: {audio}")

        source_sha256 = hashlib.sha256(audio.read_bytes()).hexdigest()
        started = time.perf_counter()
        result = processor.process(
            audio,
            modality="audio",
            evidence_id=f"non-retained-audio-{audio.stem}",
            version_id="v1",
            source_sha256=source_sha256,
            source_file=audio.name,
            admitted_metadata={
                "acceptance_scope": "non_retained_runtime",
                **({"asr_language": args.language} if args.language else {}),
            },
        )
        wall_seconds = time.perf_counter() - started
        transcript_observations = [
            observation
            for observation in result.observations
            if observation.payload.get("transcript_contract")
        ]
        if not transcript_observations:
            raise AssertionError(f"no transcript observations for {audio.name}")
        if any(
            observation.citation_locator.get("end_seconds") is None
            for observation in transcript_observations
        ):
            raise AssertionError(f"a transcript segment lost its end timestamp: {audio.name}")

        results.append(
            {
                "filename": audio.name,
                "source_sha256": source_sha256,
                "wall_seconds": round(wall_seconds, 3),
                "readiness": result.readiness,
                "transcript_segment_count": len(transcript_observations),
                "transcript_text": " ".join(
                    str(observation.payload.get("text", "")).strip()
                    for observation in transcript_observations
                ).strip(),
                "transcript_segments": [
                    {
                        "start_seconds": observation.citation_locator.get("start_seconds"),
                        "end_seconds": observation.citation_locator.get("end_seconds"),
                        "text": observation.payload.get("text"),
                        "language": observation.payload.get("language"),
                        "requested_language": observation.payload.get("requested_language"),
                        "detected_language": observation.payload.get("detected_language"),
                        "detected_language_probability": observation.payload.get(
                            "detected_language_probability"
                        ),
                        "manual_review_required": observation.payload.get(
                            "manual_review_required"
                        ),
                    }
                    for observation in transcript_observations
                ],
                "limitations": list(result.limitations),
            }
        )

    print(
        json.dumps(
            {
                "marker": "AudioNonRetainedAcceptance=PASS",
                "retained_data_mutated": False,
                "results": results,
            },
            ensure_ascii=False,
            indent=2,
        )
    )


if __name__ == "__main__":
    main()
