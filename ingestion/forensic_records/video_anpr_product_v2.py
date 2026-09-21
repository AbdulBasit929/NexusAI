# SPDX-License-Identifier: MIT
"""Precision-oriented, development-only Video ANPR V2 aggregation policy.

V2 operates only on actual, source-local frame observations.  Edit distance is
used for association, while the selected candidate is always an OCR string
that was observed on a processed frame.  No interpolated observation, track
identity, province-specific regex, or character substitution is introduced.
"""

from __future__ import annotations

import hashlib
import os
import statistics
import time
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Iterable

try:
    from ingestion.forensic_records.video_anpr_parity_adapter import (
        AggregationPolicy,
        FramePolicy,
        HashPinnedYOLODetector,
        PlateFrameProcessor,
        PlateObservation,
        PlateObservationGroup,
        aggregate_observations,
        coverage_state,
        make_coverage_receipt,
        require_hash_pinned_asset,
        schedule_base_frames,
    )
except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
    from video_anpr_parity_adapter import (
        AggregationPolicy,
        FramePolicy,
        HashPinnedYOLODetector,
        PlateFrameProcessor,
        PlateObservation,
        PlateObservationGroup,
        aggregate_observations,
        coverage_state,
        make_coverage_receipt,
        require_hash_pinned_asset,
        schedule_base_frames,
    )


ADAPTER_ID = "NX-MMR-VIDEO-ANPR-PRODUCT-BASELINE-V2"
ADAPTER_VERSION = "2.0.0-development"
POLICY_REVISION = "observed-medoid-support-quality-v2"
PLATE_DETECTOR_SHA256 = "8ec3b254a6c87610f037a90957462cafa11a9c03224e33a28c6a1d1ac2ac51b0"
VEHICLE_DETECTOR_SHA256 = "31e20dde3def09e2cf938c7be6fe23d9150bbbe503982af13345706515f2ef95"
OCR_MODEL_SHA256 = "8031afb5fdc6b4d80462c9d542f1284ebd2cfddf5dbacd62609848d7e2855f44"
OCR_CONFIG_SHA256 = "0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6"


@dataclass(frozen=True)
class ProductAggregationPolicy:
    strategy: str = "confidence_weighted_support"
    maximum_temporal_gap_seconds: float = 1.0
    minimum_spatial_iou: float = 0.05
    maximum_center_distance_ratio: float = 2.0
    maximum_candidate_edit_distance: int = 1
    maximum_observations_per_group: int = 24
    maximum_active_groups: int = 128
    minimum_selected_support: int = 2
    minimum_candidate_length: int = 4
    maximum_candidate_length: int = 10
    minimum_best_ocr_confidence: float = 0.25
    minimum_weighted_support: float = 1.0
    minimum_observed_duration_seconds: float = 0.0

    def __post_init__(self) -> None:
        if self.strategy not in {
            "best_confidence", "normalized_majority", "confidence_weighted_support"
        }:
            raise ValueError("unsupported observed-candidate strategy")
        if not 1 <= self.minimum_candidate_length <= self.maximum_candidate_length <= 16:
            raise ValueError("candidate length bounds are invalid")
        if not 0 <= self.minimum_best_ocr_confidence <= 1:
            raise ValueError("minimum OCR confidence must be between zero and one")
        if self.minimum_weighted_support < 0 or self.minimum_observed_duration_seconds < 0:
            raise ValueError("support and duration bounds cannot be negative")


SELECTED_POLICY = ProductAggregationPolicy(
    strategy="confidence_weighted_support",
    minimum_selected_support=3,
    minimum_candidate_length=4,
    maximum_candidate_length=10,
    minimum_best_ocr_confidence=0.5,
    minimum_weighted_support=1.5,
)


def aggregate_product_observations(
    observations: Iterable[PlateObservation],
    policy: ProductAggregationPolicy,
) -> tuple[PlateObservationGroup, ...]:
    """Cluster actual observations, then apply domain-neutral precision gates."""

    base = aggregate_observations(
        observations,
        AggregationPolicy(
            strategy=policy.strategy,
            maximum_temporal_gap_seconds=policy.maximum_temporal_gap_seconds,
            minimum_spatial_iou=policy.minimum_spatial_iou,
            maximum_center_distance_ratio=policy.maximum_center_distance_ratio,
            maximum_candidate_edit_distance=policy.maximum_candidate_edit_distance,
            maximum_observations_per_group=policy.maximum_observations_per_group,
            maximum_active_groups=policy.maximum_active_groups,
            minimum_selected_support=policy.minimum_selected_support,
        ),
    )
    selected: list[PlateObservationGroup] = []
    for group in base:
        text = group.selected_normalized_plate
        if not text.isalnum() or not (
            policy.minimum_candidate_length <= len(text) <= policy.maximum_candidate_length
        ):
            continue
        alternative = next(
            item for item in group.alternatives
            if item.normalized_plate_text == group.selected_normalized_plate
        )
        if (alternative.best_ocr_confidence or 0.0) < policy.minimum_best_ocr_confidence:
            continue
        if alternative.confidence_weighted_support < policy.minimum_weighted_support:
            continue
        if (
            group.last_observed_seconds - group.first_observed_seconds
            < policy.minimum_observed_duration_seconds
        ):
            continue
        selected.append(group)
    return tuple(selected)


class HashPinnedFastPlateOCR:
    """Existing image recognizer adapted to plate crops with downloads disabled."""

    model_sha256 = OCR_MODEL_SHA256

    def __init__(self, model_path: Path, config_path: Path) -> None:
        require_hash_pinned_asset(model_path, OCR_MODEL_SHA256, "FastPlateOCR model")
        require_hash_pinned_asset(config_path, OCR_CONFIG_SHA256, "FastPlateOCR config")
        try:
            from fast_plate_ocr import LicensePlateRecognizer  # type: ignore[import-not-found]
        except ModuleNotFoundError as exc:
            raise RuntimeError("FastPlateOCR runtime is unavailable") from exc
        self.recognizer = LicensePlateRecognizer(
            hub_ocr_model=None,
            device="cpu",
            providers=["CPUExecutionProvider"],
            onnx_model_path=model_path,
            plate_config_path=config_path,
            force_download=False,
        )

    def read(self, crop: object) -> tuple[tuple[str, float | None], ...]:
        predictions = self.recognizer.run(crop, return_confidence=True)
        output: list[tuple[str, float | None]] = []
        for prediction in predictions[:1]:
            values = prediction.char_probs
            confidence = statistics.fmean(float(value) for value in values) if values is not None and len(values) else None
            output.append((str(prediction.plate or ""), confidence))
        return tuple(output)


class VideoANPRProductV2Processor:
    """Fixed-4-FPS, vehicle-contained, actual-frame V2 processor."""

    processor_id = ADAPTER_ID
    processor_revision = ADAPTER_VERSION

    def __init__(self, frame_processor: PlateFrameProcessor, cv2_module: object, *, maximum_frames: int = 300, maximum_duration_seconds: float = 900.0) -> None:
        self.frame_processor = frame_processor
        self.cv2 = cv2_module
        self.maximum_frames = maximum_frames
        self.maximum_duration_seconds = maximum_duration_seconds

    @classmethod
    def from_environment(cls) -> "VideoANPRProductV2Processor":
        paths = {
            "plate": Path(os.getenv("FORENSIC_VIDEO_ANPR_PLATE_DETECTOR_PATH", "")),
            "vehicle": Path(os.getenv("FORENSIC_VIDEO_ANPR_VEHICLE_DETECTOR_PATH", "")),
            "ocr": Path(os.getenv("FORENSIC_ANPR_OCR_MODEL_PATH", "")),
            "ocr_config": Path(os.getenv("FORENSIC_ANPR_OCR_CONFIG_PATH", "")),
        }
        try:
            import cv2  # type: ignore[import-not-found]
        except ModuleNotFoundError as exc:
            raise RuntimeError("OpenCV runtime is unavailable") from exc
        plate = HashPinnedYOLODetector(paths["plate"], PLATE_DETECTOR_SHA256, input_size=640, confidence=0.25)
        vehicle = HashPinnedYOLODetector(
            paths["vehicle"], VEHICLE_DETECTOR_SHA256,
            input_size=640, confidence=0.25, allowed_classes={2, 3, 5, 7},
        )
        ocr = HashPinnedFastPlateOCR(paths["ocr"], paths["ocr_config"])
        return cls(
            PlateFrameProcessor(
                plate, ocr, cv2, vehicle_detector=vehicle,
                detection_threshold=0.25, ocr_threshold=0.0,
                binary_inverse_threshold=64,
            ),
            cv2,
            maximum_frames=max(1, min(1200, int(os.getenv("FORENSIC_VIDEO_ANPR_V2_MAX_FRAMES", "300")))),
            maximum_duration_seconds=max(1.0, float(os.getenv("FORENSIC_VIDEO_ANPR_V2_MAX_DURATION_SECONDS", "900"))),
        )

    def process_video(self, path: Path, **context: object):
        # Import lazily to avoid a module cycle during the media factory import.
        from ingestion.forensic_records.media_pipeline import (
            MediaObservation, MediaProcessResult, READY, READY_WITH_WARNINGS,
        )

        capture = self.cv2.VideoCapture(str(path))
        if not capture.isOpened():
            raise RuntimeError("Video ANPR V2 could not decode the source")
        fps = float(capture.get(self.cv2.CAP_PROP_FPS))
        frame_count = int(capture.get(self.cv2.CAP_PROP_FRAME_COUNT))
        duration = frame_count / fps if fps > 0 else 0.0
        if fps <= 0 or frame_count <= 0 or duration <= 0 or duration > self.maximum_duration_seconds:
            capture.release()
            raise RuntimeError("Video ANPR V2 source metadata violates configured bounds")
        policy = FramePolicy("fixed-4fps-fastplate-vehicle-v2", 4.0, maximum_analyzed_frames=self.maximum_frames)
        timestamps = schedule_base_frames(duration, fps, policy)
        observations: list[PlateObservation] = []
        failed = 0
        crops = 0
        wall_started, cpu_started = time.perf_counter(), time.process_time()
        try:
            for timestamp in timestamps:
                frame_number = round(timestamp * fps)
                capture.set(self.cv2.CAP_PROP_POS_FRAMES, frame_number)
                ok, frame = capture.read()
                if not ok:
                    failed += 1
                    continue
                encoded, frame_bytes = self.cv2.imencode(".png", frame)
                frame_hash = hashlib.sha256(bytes(frame_bytes)).hexdigest() if encoded else ""
                observed, frame_crops, _detections = self.frame_processor.process_frame(
                    frame,
                    source_sha256=str(context["source_sha256"]),
                    source_file=str(context["source_file"]),
                    frame_number=frame_number,
                    timestamp_seconds=frame_number / fps,
                    source_frame_sha256=frame_hash,
                )
                observations.extend(observed)
                crops += frame_crops
        finally:
            capture.release()
        groups = aggregate_product_observations(observations, SELECTED_POLICY)
        coverage = make_coverage_receipt(
            source_fps=fps, source_duration_seconds=duration, source_frame_count=frame_count,
            frames_decoded=len(timestamps) - failed,
            frames_analyzed_by_plate_detector=len(timestamps) - failed,
            frames_refined=0, ocr_crops_processed=crops, sampling_policy=policy.policy_id,
            dropped_or_failed_frames=failed, wall_seconds=time.perf_counter() - wall_started,
            cpu_seconds=time.process_time() - cpu_started, peak_ram_mib=None,
        )
        state, limitations = coverage_state(coverage, len(observations))
        packets: list[MediaObservation] = []
        for item in observations:
            packet = item.to_packet(evidence_id=str(context["evidence_id"]), version_id=str(context["version_id"]))
            packet["payload"]["processor"] = ADAPTER_ID
            packet["payload"]["processor_revision"] = ADAPTER_VERSION
            packets.append(MediaObservation(
                observation_id=packet["observation_id"], contract_version=packet["contract_version"],
                observation_type=packet["observation_type"], confidence=packet["confidence"],
                citation_locator=packet["citation_locator"], payload=packet["payload"],
                warnings=tuple(packet["warnings"]),
            ))
        for group in groups:
            packet = group.to_packet(
                source_file=str(context["source_file"]), evidence_id=str(context["evidence_id"]),
                version_id=str(context["version_id"]),
            )
            packet["payload"]["processor"] = ADAPTER_ID
            packet["payload"]["processor_revision"] = ADAPTER_VERSION
            packet["payload"]["aggregation_revision"] = POLICY_REVISION
            packets.append(MediaObservation(
                observation_id=packet["observation_id"], contract_version=packet["contract_version"],
                observation_type=packet["observation_type"], confidence=None,
                citation_locator=packet["citation_locator"], payload=packet["payload"],
                warnings=tuple(packet["warnings"]),
            ))
        result_state = "COMPLETE_RESULTS" if groups else "COMPLETE_ZERO_RESULTS"
        return MediaProcessResult(
            modality="video", readiness=READY if groups else READY_WITH_WARNINGS,
            processor_id=ADAPTER_ID, processor_revision=ADAPTER_VERSION,
            observations=tuple(packets),
            limitations=tuple(limitations),
            metadata={
                "result_states": {"video_anpr": result_state},
                "coverage": asdict(coverage),
                "raw_observation_count": len(observations), "group_count": len(groups),
                "selected_policy": asdict(SELECTED_POLICY), "execution_state": state,
                "vehicle_context": "required", "interpolated_observations": False,
            },
        )
