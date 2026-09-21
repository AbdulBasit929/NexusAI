#!/usr/bin/env python3
"""Benchmark the governed local Pakistan audio pack without persistence."""

from __future__ import annotations

import argparse
import hashlib
import json
import time
import unicodedata
import wave
from pathlib import Path
from typing import Any

try:
    from ingestion.forensic_records.media_pipeline import configured_media_processor
except ModuleNotFoundError:  # Standalone forensic worker image.
    from media_pipeline import configured_media_processor


FIXTURES = (
    {
        "id": "clear-urdu",
        "audio": "clear-urdu.wav",
        "ground_truth": "clear-urdu.ground-truth.txt",
        "sample_type": "natural_read_speech",
        "speaker_scope": "FLEURS ur_pk test row 1",
        "language": "ur",
    },
    {
        "id": "second-urdu-read-speech",
        "audio": "conversational-urdu.wav",
        "ground_truth": "conversational-urdu.ground-truth.txt",
        "sample_type": "natural_read_speech_not_spontaneous_conversation",
        "speaker_scope": "FLEURS ur_pk validation row 4",
        "language": "ur",
    },
    {
        "id": "synthetic-urdu-english-identifier",
        "audio": "urdu-english-identifier.wav",
        "ground_truth": "urdu-english-identifier.ground-truth.txt",
        "sample_type": "synthetic_mixed_language_identifier_pipeline_test",
        "speaker_scope": "locally generated synthetic voices",
        "language": None,
    },
)


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def duration_seconds(path: Path) -> float:
    with wave.open(str(path), "rb") as source:
        return source.getnframes() / source.getframerate()


def normalize(value: str) -> str:
    value = unicodedata.normalize("NFKC", value).casefold()
    value = "".join(
        " " if unicodedata.category(character).startswith(("P", "S")) else character
        for character in value
    )
    return " ".join(value.split())


def edit_distance(reference: list[str], hypothesis: list[str]) -> int:
    previous = list(range(len(hypothesis) + 1))
    for row, expected in enumerate(reference, start=1):
        current = [row]
        for column, actual in enumerate(hypothesis, start=1):
            current.append(min(
                current[-1] + 1,
                previous[column] + 1,
                previous[column - 1] + int(expected != actual),
            ))
        previous = current
    return previous[-1]


def error_rates(reference: str, hypothesis: str) -> dict[str, Any]:
    normalized_reference = normalize(reference)
    normalized_hypothesis = normalize(hypothesis)
    reference_words = normalized_reference.split()
    hypothesis_words = normalized_hypothesis.split()
    reference_characters = list(normalized_reference.replace(" ", ""))
    hypothesis_characters = list(normalized_hypothesis.replace(" ", ""))
    word_edits = edit_distance(reference_words, hypothesis_words)
    character_edits = edit_distance(reference_characters, hypothesis_characters)
    return {
        "normalization": "Unicode NFKC + casefold + punctuation/symbol to space + whitespace collapse; CER excludes spaces",
        "reference_word_count": len(reference_words),
        "word_edits": word_edits,
        "wer": round(word_edits / len(reference_words), 4) if reference_words else None,
        "reference_character_count": len(reference_characters),
        "character_edits": character_edits,
        "cer": round(character_edits / len(reference_characters), 4) if reference_characters else None,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--pack-root", type=Path, required=True)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    artifact_metadata = {item["id"]: item for item in manifest.get("artifacts", [])}
    processor = configured_media_processor()
    results: list[dict[str, Any]] = []

    for fixture in FIXTURES:
        audio = (args.pack_root / str(fixture["audio"])).resolve()
        truth_path = (args.pack_root / str(fixture["ground_truth"])).resolve()
        if not audio.is_file() or not truth_path.is_file():
            raise SystemExit(f"fixture is incomplete: {fixture['id']}")
        truth = truth_path.read_text(encoding="utf-8-sig").strip()
        duration = duration_seconds(audio)
        started = time.perf_counter()
        result = processor.process(
            audio,
            modality="audio",
            evidence_id=f"non-retained-mmv1-audio-{fixture['id']}",
            version_id="non-retained-v1",
            source_sha256=sha256(audio),
            source_file=audio.name,
            admitted_metadata={
                "acceptance_scope": "mmv1_non_retained_audio_pack",
                **({"asr_language": fixture["language"]} if fixture["language"] else {}),
            },
        )
        latency = time.perf_counter() - started
        transcripts = [
            item for item in result.observations if item.payload.get("transcript_contract")
        ]
        roman = [
            item for item in result.observations
            if item.contract_version == "forensics.audio-roman-urdu-segment/v1"
        ]
        hypothesis = " ".join(
            str(item.payload.get("text") or "").strip() for item in transcripts
        ).strip()
        metadata = artifact_metadata[str(fixture["id"])]
        results.append({
            "id": fixture["id"],
            "sample_type": fixture["sample_type"],
            "speaker_sample_id": fixture["speaker_scope"],
            "source": metadata.get("source_name"),
            "source_url": metadata.get("source_url"),
            "license": metadata.get("license"),
            "license_url": metadata.get("license_url"),
            "language": metadata.get("language"),
            "declared_locale_accent": metadata.get("declared_locale_accent"),
            "audio_path": str(audio),
            "sha256": sha256(audio),
            "duration_seconds": round(duration, 6),
            "ground_truth_source": metadata.get("ground_truth_source"),
            "ground_truth": truth,
            "asr_result": hypothesis,
            "error_rates": error_rates(truth, hypothesis),
            "latency_seconds": round(latency, 6),
            "real_time_factor": round(latency / duration, 6),
            "readiness": result.readiness,
            "processor": result.processor_id,
            "processor_revision": result.processor_revision,
            "timestamped_segments": [
                {
                    "start_seconds": item.citation_locator.get("start_seconds"),
                    "end_seconds": item.citation_locator.get("end_seconds"),
                    "text": item.payload.get("text"),
                    "language": item.payload.get("language"),
                    "requested_language": item.payload.get("requested_language"),
                    "detected_language": item.payload.get("detected_language"),
                    "detected_language_probability": item.payload.get("detected_language_probability"),
                }
                for item in transcripts
            ],
            "roman_urdu_segments": [item.payload for item in roman],
            "identifier_review": {
                "phone_digits_03001234567_preserved": "03001234567" in normalize(hypothesis).replace(" ", ""),
                "time_1035_preserved": "1035" in normalize(hypothesis).replace(" ", ""),
                "plate_mn1367_preserved": "mn1367" in normalize(hypothesis).replace(" ", ""),
                "manual_review_required": True,
            } if fixture["id"] == "synthetic-urdu-english-identifier" else None,
            "human_usability": "NOT_REASSESSED_AUTOMATICALLY_REQUIRES_HUMAN_REVIEW",
            "limitations": list(result.limitations),
        })

    report = {
        "contract_version": "nexusai.mmv1.audio-real-world-nonretained-benchmark/v1",
        "retained_data_mutated": False,
        "model_downloaded": False,
        "fixture_count": len(results),
        "natural_speaker_count": 2,
        "synthetic_identifier_fixture_count": 1,
        "results": results,
        "coverage_gaps": {
            "pakistani_english_natural": "PENDING_LAWFUL_SOURCE_ACQUISITION_APPROVAL",
            "natural_code_switch": "PENDING_LAWFUL_SOURCE_ACQUISITION_APPROVAL",
            "spontaneous_conversational_urdu": "PENDING_LAWFUL_SOURCE_ACQUISITION_APPROVAL",
            "moderate_noise": "PENDING_LAWFUL_SOURCE_ACQUISITION_APPROVAL",
            "dates_times_phone_plate_natural": "SYNTHETIC_CONTROL_ONLY",
        },
        "claim_boundary": "Three fixtures establish pipeline behavior only and do not establish population-level ASR accuracy.",
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({
        "output": str(args.output),
        "results": [
            {
                "id": item["id"],
                "wer": item["error_rates"]["wer"],
                "cer": item["error_rates"]["cer"],
                "latency_seconds": item["latency_seconds"],
                "real_time_factor": item["real_time_factor"],
                "asr_result": item["asr_result"],
                "identifier_review": item["identifier_review"],
            }
            for item in results
        ],
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
