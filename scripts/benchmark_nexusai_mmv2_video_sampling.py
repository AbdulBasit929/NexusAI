#!/usr/bin/env python3
"""Benchmark bounded, non-retained video ANPR sampling strategies."""

from __future__ import annotations

import argparse
import dataclasses
import hashlib
import json
import math
import os
import resource
import re
import shutil
import subprocess
import tempfile
import time
from pathlib import Path
from typing import Any

try:
    from ingestion.forensic_records.media_pipeline import FastALPRImageProcessor
except ModuleNotFoundError:
    from media_pipeline import FastALPRImageProcessor


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def json_value(value: Any) -> Any:
    if dataclasses.is_dataclass(value):
        return {field.name: json_value(getattr(value, field.name)) for field in dataclasses.fields(value)}
    if isinstance(value, dict):
        return {str(key): json_value(child) for key, child in value.items()}
    if isinstance(value, (list, tuple)):
        return [json_value(child) for child in value]
    return value


def probe(path: Path) -> dict[str, Any]:
    completed = subprocess.run(
        ["ffprobe", "-v", "error", "-show_format", "-show_streams", "-of", "json", str(path)],
        check=True,
        capture_output=True,
        text=True,
        timeout=30,
    )
    return json.loads(completed.stdout)


def extract_interval(path: Path, directory: Path, interval: float, count: int) -> list[tuple[Path, float]]:
    directory.mkdir(parents=True, exist_ok=True)
    pattern = directory / "frame-%04d.png"
    completed = subprocess.run(
        [
            "ffmpeg", "-nostdin", "-v", "info", "-i", str(path),
            "-vf", f"select=isnan(prev_selected_t)+gte(t-prev_selected_t\\,{interval}),showinfo",
            "-frames:v", str(count), "-fps_mode", "vfr", str(pattern),
        ],
        check=True,
        capture_output=True,
        text=True,
        timeout=300,
    )
    timestamps = [
        float(value)
        for value in re.findall(r"Parsed_showinfo[^\r\n]*\bpts_time:([0-9]+(?:\.[0-9]+)?)", completed.stderr)
    ]
    frames = sorted(directory.glob("frame-*.png"))
    if len(frames) != len(timestamps):
        raise RuntimeError(f"sampled {len(frames)} frames but parsed {len(timestamps)} timestamps")
    return list(zip(frames, timestamps))


def extract_timestamp(path: Path, output: Path, timestamp: float) -> None:
    subprocess.run(
        [
            "ffmpeg", "-nostdin", "-v", "error", "-ss", f"{timestamp:.3f}",
            "-i", str(path), "-frames:v", "1", "-y", str(output),
        ],
        check=True,
        capture_output=True,
        timeout=60,
    )


def plate_row(observation: Any, frame_hash: str, timestamp: float, fps: float) -> dict[str, Any]:
    return {
        "observation_id": observation.observation_id,
        "frame_number_estimate": round(timestamp * fps),
        "timestamp_seconds": timestamp,
        "source_frame_sha256": frame_hash,
        "raw_ocr": observation.payload.get("raw_plate_text"),
        "normalized_plate": observation.payload.get("normalized_plate_text"),
        "detector_confidence": observation.payload.get("detection_confidence"),
        "ocr_confidence": observation.payload.get("ocr_confidence"),
        "bbox": observation.citation_locator.get("bbox"),
        "crop": observation.payload.get("crop"),
        "detector_model": observation.payload.get("detector_model"),
        "ocr_model": observation.payload.get("ocr_model"),
        "processor": observation.payload.get("processor"),
        "processor_revision": observation.payload.get("processor_revision"),
        "review_state": "model_candidate",
    }


def run_frames(
    processor: FastALPRImageProcessor,
    video: Path,
    frames: list[tuple[Path, float]],
    source_hash: str,
    fps: float,
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    plates: list[dict[str, Any]] = []
    frame_rows: list[dict[str, Any]] = []
    for frame, timestamp in frames:
        frame_hash = sha256(frame)
        started = time.perf_counter()
        result = processor.process_image(
            frame,
            evidence_id="non-retained-mmv2-video-anpr",
            version_id="non-retained-v1",
            source_sha256=source_hash,
            source_file=video.name,
            locator_prefix={
                "timestamp_seconds": timestamp,
                "frame_number_estimate": round(timestamp * fps),
                "source_frame_sha256": frame_hash,
            },
        )
        frame_rows.append(
            {
                "timestamp_seconds": timestamp,
                "frame_number_estimate": round(timestamp * fps),
                "source_frame_sha256": frame_hash,
                "anpr_wall_seconds": round(time.perf_counter() - started, 6),
                "plate_count": len(result.observations),
            }
        )
        plates.extend(plate_row(item, frame_hash, timestamp, fps) for item in result.observations)
    return plates, frame_rows


def group_plates(plates: list[dict[str, Any]]) -> list[dict[str, Any]]:
    grouped: dict[str, list[dict[str, Any]]] = {}
    for plate in plates:
        grouped.setdefault(str(plate["normalized_plate"] or ""), []).append(plate)
    output = []
    for plate, sightings in sorted(grouped.items()):
        ordered = sorted(sightings, key=lambda row: row["timestamp_seconds"])
        best = max(
            ordered,
            key=lambda row: (float(row["ocr_confidence"] or 0), float(row["detector_confidence"] or 0)),
        )
        output.append(
            {
                "plate": plate,
                "sightings_count": len(ordered),
                "first_seen_seconds": ordered[0]["timestamp_seconds"],
                "last_seen_seconds": ordered[-1]["timestamp_seconds"],
                "best_observation_id": best["observation_id"],
                "all_observation_ids": [row["observation_id"] for row in ordered],
            }
        )
    return output


def run_strategy(
    name: str,
    video: Path,
    processor: FastALPRImageProcessor,
    source_hash: str,
    fps: float,
    duration: float,
    interval: float,
    max_frames: int,
    *,
    adaptive: bool = False,
) -> dict[str, Any]:
    cpu_started = time.process_time()
    wall_started = time.perf_counter()
    before_tmp = set(Path(tempfile.gettempdir()).glob("nexusai-mmv2-sampling-*"))
    with tempfile.TemporaryDirectory(prefix="nexusai-mmv2-sampling-") as raw_directory:
        directory = Path(raw_directory)
        extraction_started = time.perf_counter()
        frames = extract_interval(video, directory / "coarse", interval, max_frames)
        extraction_seconds = time.perf_counter() - extraction_started
        temporary_bytes = sum(frame.stat().st_size for frame, _ in frames)
        plates, frame_rows = run_frames(processor, video, frames, source_hash, fps)
        if adaptive and plates:
            coarse_timestamps = {timestamp for _, timestamp in frames}
            refinement = sorted(
                {
                    round(candidate, 3)
                    for plate in plates
                    for delta in (-2.0, -1.0, 1.0, 2.0)
                    if 0 <= (candidate := float(plate["timestamp_seconds"]) + delta) < duration
                    and round(candidate, 3) not in coarse_timestamps
                }
            )[:8]
            refined_frames = []
            for index, timestamp in enumerate(refinement):
                output = directory / "refined" / f"frame-{index:04d}.png"
                output.parent.mkdir(parents=True, exist_ok=True)
                extract_timestamp(video, output, timestamp)
                refined_frames.append((output, timestamp))
            temporary_bytes += sum(frame.stat().st_size for frame, _ in refined_frames)
            refined_plates, refined_rows = run_frames(processor, video, refined_frames, source_hash, fps)
            frames.extend(refined_frames)
            plates.extend(refined_plates)
            frame_rows.extend(refined_rows)
        observation_ids = [plate["observation_id"] for plate in plates]
    wall_seconds = time.perf_counter() - wall_started
    cpu_seconds = time.process_time() - cpu_started
    after_tmp = set(Path(tempfile.gettempdir()).glob("nexusai-mmv2-sampling-*"))
    normalized = sorted({str(plate["normalized_plate"] or "") for plate in plates if plate["normalized_plate"]})
    known = [plate for plate in plates if plate["normalized_plate"] == "LN15ZZC"]
    return {
        "strategy": name,
        "interval_seconds": interval,
        "configured_max_frames": max_frames,
        "adaptive_positive_refinement": adaptive,
        "sampled_frames": len(frames),
        "first_sample_seconds": min((row["timestamp_seconds"] for row in frame_rows), default=None),
        "last_sample_seconds": max((row["timestamp_seconds"] for row in frame_rows), default=None),
        "full_duration_coverage": max((row["timestamp_seconds"] for row in frame_rows), default=0) >= max(0, duration - interval - 0.25),
        "wall_seconds": round(wall_seconds, 6),
        "process_cpu_seconds": round(cpu_seconds, 6),
        "average_process_cpu_percent": round(cpu_seconds / wall_seconds * 100, 2) if wall_seconds else None,
        "peak_rss_kib": resource.getrusage(resource.RUSAGE_SELF).ru_maxrss,
        "frame_extraction_seconds": round(extraction_seconds, 6),
        "temporary_frame_bytes": temporary_bytes,
        "temporary_artifacts_cleaned": after_tmp == before_tmp,
        "anpr_detections": len(plates),
        "unique_plates": normalized,
        "duplicate_observations": max(0, len(plates) - len(normalized)),
        "known_positive_detected": bool(known),
        "known_positive_first_sighting": min((row["timestamp_seconds"] for row in known), default=None),
        "known_positive_last_sighting": max((row["timestamp_seconds"] for row in known), default=None),
        "observation_ids_unique": len(set(observation_ids)) == len(observation_ids),
        "plate_groups": group_plates(plates),
        "plates": plates,
        "frames": sorted(frame_rows, key=lambda row: row["timestamp_seconds"]),
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("video", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    video = args.video.resolve()
    source_hash = sha256(video)
    metadata = probe(video)
    duration = float(metadata["format"]["duration"])
    video_stream = next(stream for stream in metadata["streams"] if stream.get("codec_type") == "video")
    rate = str(video_stream.get("avg_frame_rate") or "0/1").split("/", 1)
    fps = float(rate[0]) / float(rate[1]) if float(rate[1]) else 0.0
    processor = FastALPRImageProcessor.from_environment()
    strategies = [
        run_strategy("current_fixed_5s_capped_6", video, processor, source_hash, fps, duration, 5.0, 6),
        run_strategy("fixed_2s_full_duration", video, processor, source_hash, fps, duration, 2.0, math.ceil(duration / 2.0)),
        run_strategy("fixed_1s_full_duration", video, processor, source_hash, fps, duration, 1.0, math.ceil(duration)),
        run_strategy("adaptive_5s_full_duration_plus_positive_refinement", video, processor, source_hash, fps, duration, 5.0, math.ceil(duration / 5.0), adaptive=True),
    ]
    report = {
        "contract_version": "nexusai.mmv2.video-anpr-sampling-benchmark/v1",
        "retained_data_mutated": False,
        "source": {
            "path": str(video),
            "sha256": source_hash,
            "size_bytes": video.stat().st_size,
            "duration_seconds": duration,
            "fps": fps,
            "codec": video_stream.get("codec_name"),
            "width": video_stream.get("width"),
            "height": video_stream.get("height"),
        },
        "processor": {
            "id": processor.processor_id,
            "revision": processor.processor_revision,
            "detector_model": processor.detector_id,
            "ocr_model": processor.ocr_id,
        },
        "strategies": strategies,
        "selection_policy": {
            "selected": "fixed_1s_full_duration_for_anpr_only",
            "rule": "Prefer full-duration plate-candidate recall, exact source timestamps, and bounded CPU/time/storage; keep OCR, face, and SigLIP on a separate coarser cadence.",
            "resource_boundary": "At most 60 ANPR samples by default, at most 120 by hard cap, with an effective interval widened for longer videos. Secondary image roles remain at a default five-second cadence.",
            "mmv1_reference_disposition": "The prior LN15ZZC-at-25s result used an fps-filter midpoint frame labeled with index*interval. Source-preserving timestamps do not reproduce that result at 25 seconds; LN15ZZC remains an unreviewed OCR variant, not ground truth.",
        },
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({
        "output": str(args.output),
        "strategies": [
            {
                "strategy": row["strategy"],
                "sampled_frames": row["sampled_frames"],
                "wall_seconds": row["wall_seconds"],
                "known_positive_detected": row["known_positive_detected"],
                "unique_plates": row["unique_plates"],
            }
            for row in strategies
        ],
    }, indent=2))


if __name__ == "__main__":
    main()
