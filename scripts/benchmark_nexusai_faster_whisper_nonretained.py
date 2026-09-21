#!/usr/bin/env python3
"""Benchmark a local faster-whisper model without NexusAI persistence."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import sys
import threading
import time
import wave
from pathlib import Path

import ctranslate2
import faster_whisper
import psutil
from faster_whisper import WhisperModel


def wav_duration(path: Path) -> float:
    with wave.open(str(path), "rb") as source:
        return source.getnframes() / source.getframerate()


class PeakRSS:
    def __init__(self) -> None:
        self.process = psutil.Process(os.getpid())
        self.peak = self.process.memory_info().rss
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._sample, daemon=True)

    def _sample(self) -> None:
        while not self._stop.wait(0.05):
            self.peak = max(self.peak, self.process.memory_info().rss)

    def __enter__(self) -> "PeakRSS":
        self._thread.start()
        return self

    def __exit__(self, *_args: object) -> None:
        self._stop.set()
        self._thread.join()
        self.peak = max(self.peak, self.process.memory_info().rss)


def transcribe(
    model: WhisperModel,
    audio: Path,
    *,
    language: str | None,
    duration: float,
) -> dict[str, object]:
    started = time.perf_counter()
    segments_source, info = model.transcribe(
        str(audio),
        language=language,
        task="transcribe",
        beam_size=5,
        condition_on_previous_text=False,
        word_timestamps=False,
    )
    segments = [
        {
            "start_seconds": round(float(segment.start), 3),
            "end_seconds": round(float(segment.end), 3),
            "text": segment.text,
        }
        for segment in segments_source
    ]
    wall_seconds = time.perf_counter() - started
    return {
        "language_configuration": "forced" if language else "detected",
        "requested_language": language,
        "language": info.language,
        "language_probability": info.language_probability,
        "wall_seconds": round(wall_seconds, 3),
        "rtf": round(wall_seconds / duration, 4),
        "raw_transcript": "".join(str(segment["text"]) for segment in segments).strip(),
        "segments": segments,
    }


def main() -> None:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--threads", type=int, default=8)
    parser.add_argument("audio", nargs=3, type=Path)
    args = parser.parse_args()

    model_path = args.model.resolve()
    output_path = args.output.resolve()
    audio_paths = [path.resolve() for path in args.audio]
    if not model_path.is_dir():
        raise SystemExit(f"model directory does not exist: {model_path}")
    if any(not path.is_file() for path in audio_paths):
        raise SystemExit("one or more audio inputs do not exist")

    process = psutil.Process(os.getpid())
    rss_before = process.memory_info().rss
    with PeakRSS() as memory:
        load_started = time.perf_counter()
        model = WhisperModel(
            str(model_path),
            device="cpu",
            compute_type="int8",
            cpu_threads=args.threads,
            num_workers=1,
            local_files_only=True,
        )
        load_seconds = time.perf_counter() - load_started

        results = []
        for index, audio in enumerate(audio_paths):
            duration = wav_duration(audio)
            language = "ur" if index < 2 else None
            first = transcribe(model, audio, language=language, duration=duration)
            warm = transcribe(model, audio, language=language, duration=duration)
            detected = (
                transcribe(model, audio, language=None, duration=duration)
                if language is not None
                else None
            )
            results.append(
                {
                    "filename": audio.name,
                    "sha256": hashlib.sha256(audio.read_bytes()).hexdigest(),
                    "duration_seconds": round(duration, 6),
                    "first_inference": first,
                    "warm_inference": warm,
                    "detected_language_control": detected,
                }
            )

    output = {
        "schema": "nexusai.non-retained-asr-benchmark/v1",
        "marker": "FasterWhisperSmallNonRetainedBenchmark=PASS",
        "model_path": str(model_path),
        "model": "Systran/faster-whisper-small",
        "device": "cpu",
        "compute_type": "int8",
        "cpu_threads": args.threads,
        "platform": platform.platform(),
        "processor": platform.processor(),
        "logical_cpu_count": os.cpu_count(),
        "faster_whisper_version": faster_whisper.__version__,
        "ctranslate2_version": ctranslate2.__version__,
        "model_load_seconds": round(load_seconds, 3),
        "rss_before_bytes": rss_before,
        "peak_rss_bytes": memory.peak,
        "peak_rss_delta_bytes": memory.peak - rss_before,
        "results": results,
        "retained_data_mutated": False,
    }
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(output, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(output, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
