# SPDX-License-Identifier: MIT
"""License-clean, plate-only Video ANPR demo processor using local ONNX assets."""

from __future__ import annotations

import hashlib
import os
import statistics
import time
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any, Sequence

try:
    from ingestion.forensic_records.video_anpr_parity_adapter import (
        FramePolicy,
        PlateBounds,
        PlateObservation,
        TimeInterval,
        coverage_state,
        crop_quality,
        make_coverage_receipt,
        normalize_plate_neutral,
        observation_id,
        schedule_base_frames,
    )
    from ingestion.forensic_records.video_anpr_product_v2 import (
        POLICY_REVISION,
        SELECTED_POLICY,
        aggregate_product_observations,
    )
except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
    from video_anpr_parity_adapter import (
        FramePolicy,
        PlateBounds,
        PlateObservation,
        TimeInterval,
        coverage_state,
        crop_quality,
        make_coverage_receipt,
        normalize_plate_neutral,
        observation_id,
        schedule_base_frames,
    )
    from video_anpr_product_v2 import (
        POLICY_REVISION,
        SELECTED_POLICY,
        aggregate_product_observations,
    )


ADAPTER_ID = "NX-MMR-VIDEO-ANPR-ONNX-DEMO-V3"
ADAPTER_VERSION = "3.0.0-development-demo"
FRAME_POLICY_ID = "fixed-4fps-fastplate-onnx-plate-only-v3"
DETECTOR_SHA256 = "888397b96d761c89db40bc9c305838e8652660f5e282c2cadebbe8d2951a77a8"
OCR_MODEL_SHA256 = "8031afb5fdc6b4d80462c9d542f1284ebd2cfddf5dbacd62609848d7e2855f44"
OCR_CONFIG_SHA256 = "0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6"


@dataclass(frozen=True)
class VideoV3FrameReceipt:
    frame_number: int
    timestamp_seconds: float
    source_frame_sha256: str
    raw_detection_count: int
    ocr_candidate_count: int
    selected_group_observation_count: int
    latency_seconds: float


@dataclass(frozen=True)
class VideoV3Run:
    result: Any
    observations: tuple[PlateObservation, ...]
    groups: tuple[Any, ...]
    frames: tuple[VideoV3FrameReceipt, ...]


def _mean_confidence(value: Any) -> float | None:
    if value is None:
        return None
    if isinstance(value, (int, float)):
        result = float(value)
        return result if 0 <= result <= 1 else None
    values = [float(item) for item in value if 0 <= float(item) <= 1]
    return statistics.fmean(values) if values else None


def observations_from_fastalpr_results(
    frame: Any,
    results: Sequence[Any],
    *,
    source_sha256: str,
    source_file: str,
    frame_number: int,
    timestamp_seconds: float,
    source_frame_sha256: str,
    detector_model_sha256: str,
    ocr_model_sha256: str,
    cv2_module: Any,
) -> tuple[PlateObservation, ...]:
    """Convert observed FastALPR results without correction or identity inference."""

    if frame is None or len(getattr(frame, "shape", ())) < 2:
        raise RuntimeError("Video ANPR V3 decoded frame has invalid dimensions")
    height, width = int(frame.shape[0]), int(frame.shape[1])
    observations: list[PlateObservation] = []
    for result in results[:32]:
        detection = result.detection
        bounds = detection.bounding_box
        x1 = max(0, min(int(bounds.x1), width))
        y1 = max(0, min(int(bounds.y1), height))
        x2 = max(0, min(int(bounds.x2), width))
        y2 = max(0, min(int(bounds.y2), height))
        if x2 <= x1 or y2 <= y1:
            continue
        raw = str(result.ocr.text or "") if result.ocr is not None else ""
        normalized = normalize_plate_neutral(raw)
        if not normalized:
            continue
        plate_bounds = PlateBounds(float(x1), float(y1), float(x2 - x1), float(y2 - y1))
        crop = frame[y1:y2, x1:x2]
        encoded, crop_bytes = cv2_module.imencode(".png", crop)
        crop_sha256 = hashlib.sha256(bytes(crop_bytes)).hexdigest() if encoded else ""
        observations.append(PlateObservation(
            observation_id=observation_id(source_sha256, frame_number, plate_bounds, normalized),
            source_sha256=source_sha256,
            source_file=source_file,
            frame_number=frame_number,
            timestamp_seconds=timestamp_seconds,
            source_frame_sha256=source_frame_sha256,
            bounds=plate_bounds,
            raw_ocr=raw,
            normalized_ocr=normalized,
            detector_confidence=float(detection.confidence),
            ocr_confidence=_mean_confidence(result.ocr.confidence),
            crop_sha256=crop_sha256,
            detector_model_sha256=detector_model_sha256,
            ocr_model_sha256=ocr_model_sha256,
            vehicle_context_matched=None,
            crop_quality=crop_quality(plate_bounds, width, height),
        ))
    return tuple(observations)


class VideoANPROnnxV3Processor:
    """Fixed-4-FPS actual-frame Video ANPR using no vehicle model or tracking."""

    processor_id = ADAPTER_ID
    processor_revision = ADAPTER_VERSION

    def __init__(
        self,
        image_processor: Any,
        cv2_module: Any,
        *,
        maximum_frames: int = 1200,
        maximum_duration_seconds: float = 900.0,
    ) -> None:
        self.image_processor = image_processor
        self.cv2 = cv2_module
        self.maximum_frames = maximum_frames
        self.maximum_duration_seconds = maximum_duration_seconds

    @classmethod
    def from_environment(cls) -> "VideoANPROnnxV3Processor":
        try:
            import cv2  # type: ignore[import-not-found]
            from ingestion.forensic_records.media_pipeline import FastALPRImageProcessor
        except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
            import cv2  # type: ignore[import-not-found]
            from media_pipeline import FastALPRImageProcessor
        confidence = float(os.getenv("FORENSIC_VIDEO_ANPR_V3_DETECTION_CONFIDENCE", "0.25"))
        return cls(
            FastALPRImageProcessor.from_environment(minimum_confidence=confidence),
            cv2,
            maximum_frames=max(1, min(3600, int(os.getenv("FORENSIC_VIDEO_ANPR_V3_MAX_FRAMES", "1200")))),
            maximum_duration_seconds=max(1.0, float(os.getenv("FORENSIC_VIDEO_ANPR_V3_MAX_DURATION_SECONDS", "900"))),
        )

    def process_video(self, path: Path, **context: object) -> Any:
        return self.run_video(path, context=context).result

    def run_video(
        self,
        path: Path,
        *,
        context: dict[str, object],
        allowed_intervals: Sequence[TimeInterval] | None = None,
    ) -> VideoV3Run:
        try:
            from ingestion.forensic_records.media_pipeline import (
                MediaObservation,
                MediaProcessResult,
                READY,
                READY_WITH_WARNINGS,
            )
        except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
            from media_pipeline import MediaObservation, MediaProcessResult, READY, READY_WITH_WARNINGS

        capture = self.cv2.VideoCapture(str(path))
        if not capture.isOpened():
            raise RuntimeError("Video ANPR V3 could not decode the source")
        fps = float(capture.get(self.cv2.CAP_PROP_FPS))
        frame_count = int(capture.get(self.cv2.CAP_PROP_FRAME_COUNT))
        duration = frame_count / fps if fps > 0 else 0.0
        if fps <= 0 or frame_count <= 0 or duration <= 0 or duration > self.maximum_duration_seconds:
            capture.release()
            raise RuntimeError("Video ANPR V3 source metadata violates configured bounds")
        policy = FramePolicy(FRAME_POLICY_ID, 4.0, maximum_analyzed_frames=self.maximum_frames)
        timestamps = schedule_base_frames(duration, fps, policy, allowed_intervals=allowed_intervals)
        observations: list[PlateObservation] = []
        frame_rows: list[VideoV3FrameReceipt] = []
        failed = 0
        crops = 0
        wall_started, cpu_started = time.perf_counter(), time.process_time()
        try:
            for timestamp in timestamps:
                frame_started = time.perf_counter()
                frame_number = round(timestamp * fps)
                capture.set(self.cv2.CAP_PROP_POS_FRAMES, frame_number)
                ok, frame = capture.read()
                if not ok:
                    failed += 1
                    continue
                encoded, frame_bytes = self.cv2.imencode(".png", frame)
                frame_hash = hashlib.sha256(bytes(frame_bytes)).hexdigest() if encoded else ""
                results = tuple(self.image_processor.engine.predict(frame))
                observed = observations_from_fastalpr_results(
                    frame,
                    results,
                    source_sha256=str(context["source_sha256"]),
                    source_file=str(context["source_file"]),
                    frame_number=frame_number,
                    timestamp_seconds=frame_number / fps,
                    source_frame_sha256=frame_hash,
                    detector_model_sha256=DETECTOR_SHA256,
                    ocr_model_sha256=OCR_MODEL_SHA256,
                    cv2_module=self.cv2,
                )
                observations.extend(observed)
                crops += len(results)
                frame_rows.append(VideoV3FrameReceipt(
                    frame_number=frame_number,
                    timestamp_seconds=frame_number / fps,
                    source_frame_sha256=frame_hash,
                    raw_detection_count=len(results),
                    ocr_candidate_count=len(observed),
                    selected_group_observation_count=0,
                    latency_seconds=time.perf_counter() - frame_started,
                ))
        finally:
            capture.release()

        groups = aggregate_product_observations(observations, SELECTED_POLICY)
        selected_ids = {identity for group in groups for identity in group.observation_ids}
        frame_rows = [
            VideoV3FrameReceipt(
                **{**asdict(frame), "selected_group_observation_count": sum(
                    item.observation_id in selected_ids and item.frame_number == frame.frame_number
                    for item in observations
                )}
            )
            for frame in frame_rows
        ]
        coverage = make_coverage_receipt(
            source_fps=fps,
            source_duration_seconds=duration,
            source_frame_count=frame_count,
            frames_decoded=len(frame_rows),
            frames_analyzed_by_plate_detector=len(frame_rows),
            frames_refined=0,
            ocr_crops_processed=crops,
            sampling_policy=policy.policy_id,
            dropped_or_failed_frames=failed,
            wall_seconds=time.perf_counter() - wall_started,
            cpu_seconds=time.process_time() - cpu_started,
            peak_ram_mib=None,
        )
        state, coverage_limitations = coverage_state(coverage, len(observations))
        packets: list[MediaObservation] = []
        for item in observations:
            packet = item.to_packet(
                evidence_id=str(context["evidence_id"]), version_id=str(context["version_id"])
            )
            packet["payload"].update({
                "processor": ADAPTER_ID,
                "processor_revision": ADAPTER_VERSION,
                "vehicle_context": "not_used_plate_only",
                "ocr_input_transform": "FastALPR DefaultOCR deterministic BGR-to-RGB channel permutation",
            })
            packets.append(MediaObservation(
                observation_id=packet["observation_id"],
                contract_version=packet["contract_version"],
                observation_type=packet["observation_type"],
                confidence=packet["confidence"],
                citation_locator=packet["citation_locator"],
                payload=packet["payload"],
                warnings=tuple(packet["warnings"]),
            ))
        for group in groups:
            packet = group.to_packet(
                source_file=str(context["source_file"]),
                evidence_id=str(context["evidence_id"]),
                version_id=str(context["version_id"]),
            )
            packet["payload"].update({
                "processor": ADAPTER_ID,
                "processor_revision": ADAPTER_VERSION,
                "aggregation_revision": POLICY_REVISION,
                "vehicle_context": "not_used_plate_only",
            })
            packets.append(MediaObservation(
                observation_id=packet["observation_id"],
                contract_version=packet["contract_version"],
                observation_type=packet["observation_type"],
                confidence=None,
                citation_locator=packet["citation_locator"],
                payload=packet["payload"],
                warnings=tuple(packet["warnings"]),
            ))
        result_state = "COMPLETE_RESULTS" if groups else "COMPLETE_ZERO_RESULTS"
        result = MediaProcessResult(
            modality="video",
            readiness=READY if groups else READY_WITH_WARNINGS,
            processor_id=ADAPTER_ID,
            processor_revision=ADAPTER_VERSION,
            observations=tuple(packets),
            limitations=tuple((*coverage_limitations,
                "Development/demo model observations require analyst review",
                "Plate-only temporal grouping is not tracking, identity, ownership, or continuous presence",
            )),
            metadata={
                "result_states": {"video_anpr": result_state},
                "coverage": asdict(coverage),
                "raw_observation_count": len(observations),
                "group_count": len(groups),
                "selected_policy": asdict(SELECTED_POLICY),
                "execution_state": state,
                "vehicle_context": "not_used_plate_only",
                "interpolated_observations": False,
                "persistent_tracking": False,
                "raw_detector_positive_frames": sum(item.raw_detection_count > 0 for item in frame_rows),
                "ocr_candidate_positive_frames": sum(item.ocr_candidate_count > 0 for item in frame_rows),
                "group_selected_positive_frames": sum(item.selected_group_observation_count > 0 for item in frame_rows),
            },
        )
        return VideoV3Run(result, tuple(observations), groups, tuple(frame_rows))
