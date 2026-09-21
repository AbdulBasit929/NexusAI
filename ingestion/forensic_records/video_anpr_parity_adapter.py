# SPDX-License-Identifier: MIT
"""Bounded, source-time-preserving video ANPR candidate processor.

This module is intentionally not enabled by the live processor factory. It
provides the source-development implementation for the NX-MMR reference-parity
candidate and requires explicitly supplied, hash-pinned local model assets.
"""

from __future__ import annotations

import hashlib
import json
import math
import re
import time
import uuid
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any, Callable, Iterable, Literal, Protocol, Sequence


ADAPTER_ID = "NX-MMR-REFERENCE-PARITY-ADAPTER-V1"
ADAPTER_VERSION = "1.0.0-development"
ANPR_OBSERVATION_CONTRACT = "forensics.anpr-observation/v1"
VIDEO_ANPR_GROUP_CONTRACT = "forensics.video-anpr-plate-group/v1"
COVERAGE_CONTRACT = "forensics.video-anpr-coverage-receipt/v1"
NORMALIZATION_REVISION = "neutral-latin-alphanumeric-v1"
FRAME_POLICY_REVISION = "bounded-coarse-refinement-v1"
AGGREGATION_REVISION = "bounded-spatiotemporal-support-v1"

AggregationStrategy = Literal[
    "best_confidence", "normalized_majority", "confidence_weighted_support"
]


class ParityAdapterError(RuntimeError):
    """The candidate could not safely complete within its declared contract."""


class CandidateModelUnavailable(ParityAdapterError):
    """A required hash-pinned local model is absent or has unexpected bytes."""


@dataclass(frozen=True)
class TimeInterval:
    start_seconds: float
    end_seconds: float

    def __post_init__(self) -> None:
        if self.start_seconds < 0 or self.end_seconds < self.start_seconds:
            raise ValueError("time interval must be non-negative and ordered")

    def contains(self, timestamp: float) -> bool:
        return self.start_seconds - 1e-9 <= timestamp <= self.end_seconds + 1e-9


@dataclass(frozen=True)
class PlateBounds:
    x: float
    y: float
    width: float
    height: float

    def __post_init__(self) -> None:
        if self.width <= 0 or self.height <= 0:
            raise ValueError("plate bounds must have positive dimensions")

    @property
    def x2(self) -> float:
        return self.x + self.width

    @property
    def y2(self) -> float:
        return self.y + self.height

    def iou(self, other: "PlateBounds") -> float:
        overlap_width = max(0.0, min(self.x2, other.x2) - max(self.x, other.x))
        overlap_height = max(0.0, min(self.y2, other.y2) - max(self.y, other.y))
        overlap = overlap_width * overlap_height
        union = self.width * self.height + other.width * other.height - overlap
        return overlap / union if union > 0 else 0.0

    def center_distance_ratio(self, other: "PlateBounds") -> float:
        x_distance = (self.x + self.width / 2) - (other.x + other.width / 2)
        y_distance = (self.y + self.height / 2) - (other.y + other.height / 2)
        distance = math.hypot(x_distance, y_distance)
        scale = max(math.hypot(self.width, self.height), math.hypot(other.width, other.height))
        return distance / scale if scale > 0 else math.inf

    def as_locator(self) -> dict[str, int]:
        return {
            "x": round(self.x),
            "y": round(self.y),
            "width": round(self.width),
            "height": round(self.height),
        }


@dataclass(frozen=True)
class PlateObservation:
    observation_id: str
    source_sha256: str
    source_file: str
    frame_number: int
    timestamp_seconds: float
    source_frame_sha256: str
    bounds: PlateBounds
    raw_ocr: str
    normalized_ocr: str
    detector_confidence: float
    ocr_confidence: float | None
    crop_sha256: str
    detector_model_sha256: str
    ocr_model_sha256: str
    vehicle_context_matched: bool | None = None
    crop_quality: float | None = None

    def __post_init__(self) -> None:
        if self.frame_number < 0 or self.timestamp_seconds < 0:
            raise ValueError("observation source position must be non-negative")
        if not 0 <= self.detector_confidence <= 1:
            raise ValueError("detector confidence must be between zero and one")
        if self.ocr_confidence is not None and not 0 <= self.ocr_confidence <= 1:
            raise ValueError("OCR confidence must be between zero and one")
        if self.normalized_ocr != normalize_plate_neutral(self.raw_ocr):
            raise ValueError("normalized OCR must use neutral normalization")

    def support_weight(self) -> float:
        ocr = self.ocr_confidence if self.ocr_confidence is not None else 0.0
        quality = self.crop_quality if self.crop_quality is not None else 0.0
        return 0.6 * ocr + 0.3 * self.detector_confidence + 0.1 * quality

    def to_packet(self, *, evidence_id: str, version_id: str) -> dict[str, Any]:
        return {
            "observation_id": self.observation_id,
            "contract_version": ANPR_OBSERVATION_CONTRACT,
            "observation_type": "anpr_ocr_observation",
            "authority": "model_observation",
            "confidence": self.ocr_confidence,
            "citation_locator": {
                "source_file": self.source_file,
                "timestamp_seconds": self.timestamp_seconds,
                "frame_number": self.frame_number,
                "frame_number_semantics": "actual_selected_source_frame",
                "source_frame_sha256": self.source_frame_sha256,
                "bbox": self.bounds.as_locator(),
                "crop_sha256": self.crop_sha256,
            },
            "payload": {
                "raw_plate_text": self.raw_ocr,
                "normalized_plate_text": self.normalized_ocr,
                "normalization_revision": NORMALIZATION_REVISION,
                "detection_confidence": self.detector_confidence,
                "ocr_confidence": self.ocr_confidence,
                "crop_quality": self.crop_quality,
                "vehicle_context_matched": self.vehicle_context_matched,
                "parent_evidence_id": evidence_id,
                "parent_version_id": version_id,
                "parent_sha256": self.source_sha256,
                "processor": ADAPTER_ID,
                "processor_revision": ADAPTER_VERSION,
                "detector_model_sha256": self.detector_model_sha256,
                "ocr_model_sha256": self.ocr_model_sha256,
                "manual_review_required": True,
                "interpolated": False,
            },
            "warnings": ["Candidate plate text is a model observation and requires review"],
        }


@dataclass(frozen=True)
class CandidateAlternative:
    normalized_plate_text: str
    raw_readings: tuple[str, ...]
    support_count: int
    confidence_weighted_support: float
    best_ocr_confidence: float | None


@dataclass(frozen=True)
class PlateObservationGroup:
    group_id: str
    selected_normalized_plate: str
    selected_raw_plate: str
    support_count: int
    first_observed_seconds: float
    last_observed_seconds: float
    best_observation_id: str
    observation_ids: tuple[str, ...]
    alternatives: tuple[CandidateAlternative, ...]
    bounds_at_best_observation: PlateBounds
    aggregation_strategy: AggregationStrategy

    def to_packet(
        self,
        *,
        source_file: str,
        evidence_id: str,
        version_id: str,
    ) -> dict[str, Any]:
        return {
            "observation_id": self.group_id,
            "contract_version": VIDEO_ANPR_GROUP_CONTRACT,
            "observation_type": "video_anpr_plate_group_observation",
            "authority": "model_observation",
            "citation_locator": {
                "source_file": source_file,
                "first_seen_seconds": self.first_observed_seconds,
                "last_seen_seconds": self.last_observed_seconds,
                "best_observation_id": self.best_observation_id,
                "bbox": self.bounds_at_best_observation.as_locator(),
            },
            "payload": {
                "normalized_plate_text": self.selected_normalized_plate,
                "selected_raw_plate_text": self.selected_raw_plate,
                "sightings_count": self.support_count,
                "first_seen_seconds": self.first_observed_seconds,
                "last_seen_seconds": self.last_observed_seconds,
                "best_observation_id": self.best_observation_id,
                "all_observation_ids": list(self.observation_ids),
                "alternatives": [asdict(item) for item in self.alternatives],
                "aggregation_strategy": self.aggregation_strategy,
                "aggregation_revision": AGGREGATION_REVISION,
                "grouping_semantics": (
                    "bounded adjacent-frame model observations; not persistent object tracking"
                ),
                "parent_evidence_id": evidence_id,
                "parent_version_id": version_id,
                "processor": ADAPTER_ID,
                "processor_revision": ADAPTER_VERSION,
                "manual_review_required": True,
                "persistent_tracking": False,
                "interpolated_observations": False,
            },
            "warnings": [
                "The selected text is an observed OCR candidate, not a confirmed plate",
                "Temporal grouping does not establish vehicle identity, presence between observations, or a journey",
            ],
        }


@dataclass(frozen=True)
class FramePolicy:
    policy_id: str
    base_cadence_fps: float
    refinement_enabled: bool = False
    refinement_window_seconds: float = 0.5
    refinement_cadence_fps: float = 8.0
    maximum_analyzed_frames: int = 300
    maximum_refinement_frames_per_trigger: int = 9

    def __post_init__(self) -> None:
        if self.base_cadence_fps <= 0 or self.refinement_cadence_fps <= 0:
            raise ValueError("frame cadences must be positive")
        if self.refinement_window_seconds < 0:
            raise ValueError("refinement window cannot be negative")
        if self.maximum_analyzed_frames <= 0 or self.maximum_refinement_frames_per_trigger <= 0:
            raise ValueError("frame bounds must be positive")


@dataclass(frozen=True)
class AggregationPolicy:
    strategy: AggregationStrategy
    maximum_temporal_gap_seconds: float = 1.0
    minimum_spatial_iou: float = 0.05
    maximum_center_distance_ratio: float = 2.0
    maximum_candidate_edit_distance: int = 1
    maximum_observations_per_group: int = 24
    maximum_active_groups: int = 128
    minimum_selected_support: int = 1

    def __post_init__(self) -> None:
        if self.minimum_selected_support <= 0:
            raise ValueError("minimum selected support must be positive")


@dataclass(frozen=True)
class CoverageReceipt:
    source_fps: float
    source_duration_seconds: float
    source_frame_count: int
    frames_decoded: int
    frames_analyzed_by_plate_detector: int
    frames_refined: int
    ocr_crops_processed: int
    effective_frame_coverage: float
    sampling_policy: str
    dropped_or_failed_frames: int
    wall_seconds: float
    cpu_seconds: float
    peak_ram_mib: float | None
    partial_sampling: bool
    resource_stop: bool = False
    contract_version: str = COVERAGE_CONTRACT


@dataclass(frozen=True)
class AdapterRunResult:
    state: str
    observations: tuple[PlateObservation, ...]
    groups: tuple[PlateObservationGroup, ...]
    coverage: CoverageReceipt
    limitations: tuple[str, ...] = ()


class FrameDetector(Protocol):
    model_sha256: str

    def detect(self, frame: Any) -> Sequence[tuple[PlateBounds, float, int]]: ...


class CropOCR(Protocol):
    model_sha256: str

    def read(self, crop: Any) -> Sequence[tuple[str, float | None]]: ...


class HashPinnedYOLODetector:
    """CPU YOLO wrapper that refuses missing or changed checkpoint bytes."""

    def __init__(
        self,
        model_path: Path,
        expected_sha256: str,
        *,
        input_size: int = 640,
        confidence: float = 0.25,
        allowed_classes: set[int] | None = None,
    ) -> None:
        self.model_path = require_hash_pinned_asset(model_path, expected_sha256, "YOLO")
        self.model_sha256 = expected_sha256.lower()
        self.input_size = input_size
        self.confidence = confidence
        self.allowed_classes = allowed_classes
        try:
            from ultralytics import YOLO  # type: ignore[import-not-found]
        except ModuleNotFoundError as exc:
            raise CandidateModelUnavailable("Ultralytics runtime is unavailable") from exc
        self._model = YOLO(str(self.model_path))

    def detect(self, frame: Any) -> Sequence[tuple[PlateBounds, float, int]]:
        try:
            results = self._model.predict(
                frame,
                imgsz=self.input_size,
                conf=self.confidence,
                device="cpu",
                verbose=False,
            )
        except Exception as exc:  # model backends surface several runtime exception types
            raise ParityAdapterError(f"bounded YOLO inference failed: {exc}") from exc
        output: list[tuple[PlateBounds, float, int]] = []
        for result in results:
            boxes = getattr(result, "boxes", None)
            data = getattr(boxes, "data", None)
            rows = data.tolist() if data is not None else []
            for row in rows:
                if len(row) < 6:
                    continue
                x1, y1, x2, y2, confidence, class_id = row[:6]
                class_value = int(class_id)
                if self.allowed_classes is not None and class_value not in self.allowed_classes:
                    continue
                if float(x2) <= float(x1) or float(y2) <= float(y1):
                    continue
                output.append((
                    PlateBounds(float(x1), float(y1), float(x2) - float(x1), float(y2) - float(y1)),
                    float(confidence),
                    class_value,
                ))
        return tuple(output)


class HashPinnedEasyOCR:
    """EasyOCR wrapper with implicit model downloads disabled."""

    def __init__(
        self,
        craft_path: Path,
        craft_sha256: str,
        recognizer_path: Path,
        recognizer_sha256: str,
    ) -> None:
        require_hash_pinned_asset(craft_path, craft_sha256, "EasyOCR CRAFT")
        require_hash_pinned_asset(recognizer_path, recognizer_sha256, "EasyOCR recognizer")
        if craft_path.parent.resolve() != recognizer_path.parent.resolve():
            raise CandidateModelUnavailable("EasyOCR assets must share one registered model directory")
        self.asset_hashes = {
            "craft": craft_sha256.lower(),
            "recognizer": recognizer_sha256.lower(),
        }
        self.model_sha256 = canonical_digest(self.asset_hashes)
        try:
            import easyocr  # type: ignore[import-not-found]
        except ModuleNotFoundError as exc:
            raise CandidateModelUnavailable("EasyOCR runtime is unavailable") from exc
        try:
            self._reader = easyocr.Reader(
                ["en"],
                gpu=False,
                model_storage_directory=str(craft_path.parent),
                download_enabled=False,
                verbose=False,
            )
        except Exception as exc:
            raise CandidateModelUnavailable(f"EasyOCR local assets could not be loaded: {exc}") from exc

    def read(self, crop: Any) -> Sequence[tuple[str, float | None]]:
        try:
            results = self._reader.readtext(
                crop,
                detail=1,
                paragraph=False,
                allowlist="ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
            )
        except Exception as exc:
            raise ParityAdapterError(f"bounded EasyOCR inference failed: {exc}") from exc
        output: list[tuple[str, float | None]] = []
        for item in results:
            if not isinstance(item, (tuple, list)) or len(item) < 3:
                continue
            raw = str(item[1] or "").strip()
            if not raw:
                continue
            try:
                confidence = float(item[2])
            except (TypeError, ValueError):
                confidence = None
            output.append((raw, confidence if confidence is None or 0 <= confidence <= 1 else None))
        return tuple(output)


class PlateFrameProcessor:
    """Stateless plate detection/OCR for one actual decoded source frame."""

    def __init__(
        self,
        plate_detector: FrameDetector,
        ocr: CropOCR,
        cv2_module: Any,
        *,
        vehicle_detector: FrameDetector | None = None,
        detection_threshold: float = 0.25,
        ocr_threshold: float = 0.0,
        binary_inverse_threshold: int = 64,
        maximum_detections_per_frame: int = 32,
        maximum_ocr_results_per_crop: int = 4,
    ) -> None:
        if not 0 <= detection_threshold <= 1 or not 0 <= ocr_threshold <= 1:
            raise ValueError("detection and OCR thresholds must be between zero and one")
        self.plate_detector = plate_detector
        self.vehicle_detector = vehicle_detector
        self.ocr = ocr
        self.cv2 = cv2_module
        self.detection_threshold = detection_threshold
        self.ocr_threshold = ocr_threshold
        self.binary_inverse_threshold = binary_inverse_threshold
        self.maximum_detections_per_frame = maximum_detections_per_frame
        self.maximum_ocr_results_per_crop = maximum_ocr_results_per_crop

    @property
    def vehicle_context_enabled(self) -> bool:
        return self.vehicle_detector is not None

    def process_frame(
        self,
        frame: Any,
        *,
        source_sha256: str,
        source_file: str,
        frame_number: int,
        timestamp_seconds: float,
        source_frame_sha256: str,
    ) -> tuple[tuple[PlateObservation, ...], int, int]:
        if frame is None or len(getattr(frame, "shape", ())) < 2:
            raise ParityAdapterError("decoded frame has invalid dimensions")
        frame_height, frame_width = int(frame.shape[0]), int(frame.shape[1])
        vehicles = tuple(self.vehicle_detector.detect(frame)) if self.vehicle_detector else ()
        detected = tuple(self.plate_detector.detect(frame))[: self.maximum_detections_per_frame]
        output: list[PlateObservation] = []
        ocr_crops = 0
        for bounds, detector_confidence, _class_id in detected:
            if detector_confidence < self.detection_threshold:
                continue
            clipped = _clip_bounds(bounds, frame_width, frame_height)
            if clipped is None:
                continue
            vehicle_match = None
            if self.vehicle_detector is not None:
                vehicle_match = any(_contains(vehicle_bounds, clipped) for vehicle_bounds, _, _ in vehicles)
                if not vehicle_match:
                    continue
            x1, y1 = round(clipped.x), round(clipped.y)
            x2, y2 = round(clipped.x2), round(clipped.y2)
            crop = frame[y1:y2, x1:x2]
            if getattr(crop, "size", 0) == 0:
                continue
            grayscale = self.cv2.cvtColor(crop, self.cv2.COLOR_BGR2GRAY)
            _threshold, prepared = self.cv2.threshold(
                grayscale,
                self.binary_inverse_threshold,
                255,
                self.cv2.THRESH_BINARY_INV,
            )
            ocr_crops += 1
            encoded, crop_bytes = self.cv2.imencode(".png", crop)
            crop_sha256 = hashlib.sha256(bytes(crop_bytes)).hexdigest() if encoded else ""
            for raw, ocr_confidence in self.ocr.read(prepared)[: self.maximum_ocr_results_per_crop]:
                if ocr_confidence is not None and ocr_confidence < self.ocr_threshold:
                    continue
                normalized = normalize_plate_neutral(raw)
                if not normalized:
                    continue
                output.append(PlateObservation(
                    observation_id=observation_id(source_sha256, frame_number, clipped, normalized),
                    source_sha256=source_sha256,
                    source_file=source_file,
                    frame_number=frame_number,
                    timestamp_seconds=timestamp_seconds,
                    source_frame_sha256=source_frame_sha256,
                    bounds=clipped,
                    raw_ocr=raw,
                    normalized_ocr=normalized,
                    detector_confidence=detector_confidence,
                    ocr_confidence=ocr_confidence,
                    crop_sha256=crop_sha256,
                    detector_model_sha256=self.plate_detector.model_sha256,
                    ocr_model_sha256=self.ocr.model_sha256,
                    vehicle_context_matched=vehicle_match,
                    crop_quality=crop_quality(clipped, frame_width, frame_height),
                ))
        return tuple(sorted(output, key=_observation_sort_key)), ocr_crops, len(detected)


def _clip_bounds(bounds: PlateBounds, frame_width: int, frame_height: int) -> PlateBounds | None:
    x1 = max(0.0, min(bounds.x, float(frame_width)))
    y1 = max(0.0, min(bounds.y, float(frame_height)))
    x2 = max(0.0, min(bounds.x2, float(frame_width)))
    y2 = max(0.0, min(bounds.y2, float(frame_height)))
    if x2 <= x1 or y2 <= y1:
        return None
    return PlateBounds(x1, y1, x2 - x1, y2 - y1)


def _contains(container: PlateBounds, child: PlateBounds) -> bool:
    return (
        child.x >= container.x
        and child.y >= container.y
        and child.x2 <= container.x2
        and child.y2 <= container.y2
    )


def normalize_plate_neutral(raw_text: str) -> str:
    """Normalize layout only; never substitute one observed character for another."""

    return re.sub(r"[^A-Z0-9]", "", str(raw_text or "").upper())


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def require_hash_pinned_asset(path: Path, expected_sha256: str, role: str) -> Path:
    if not path.is_file():
        raise CandidateModelUnavailable(f"{role} asset is missing; implicit download is disabled")
    actual = sha256_file(path)
    if actual != expected_sha256.lower():
        raise CandidateModelUnavailable(
            f"{role} SHA-256 mismatch: expected {expected_sha256.lower()}, got {actual}"
        )
    return path


def canonical_digest(value: Any) -> str:
    encoded = json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()
    return hashlib.sha256(encoded).hexdigest()


def _allowed(timestamp: float, intervals: Sequence[TimeInterval] | None) -> bool:
    return intervals is None or any(interval.contains(timestamp) for interval in intervals)


def schedule_base_frames(
    duration_seconds: float,
    source_fps: float,
    policy: FramePolicy,
    *,
    allowed_intervals: Sequence[TimeInterval] | None = None,
) -> tuple[float, ...]:
    if duration_seconds <= 0 or source_fps <= 0:
        return ()
    source_frames = max(1, math.ceil(duration_seconds * source_fps))
    frame_step = max(1, round(source_fps / policy.base_cadence_fps))
    timestamps: list[float] = []
    for frame_number in range(0, source_frames, frame_step):
        timestamp = frame_number / source_fps
        if timestamp >= duration_seconds or not _allowed(timestamp, allowed_intervals):
            continue
        timestamps.append(timestamp)
        if len(timestamps) >= policy.maximum_analyzed_frames:
            break
    return tuple(timestamps)


def schedule_refinement_frames(
    trigger_timestamps: Iterable[float],
    duration_seconds: float,
    source_fps: float,
    policy: FramePolicy,
    *,
    already_scheduled: Iterable[float] = (),
    allowed_intervals: Sequence[TimeInterval] | None = None,
) -> tuple[float, ...]:
    if not policy.refinement_enabled or duration_seconds <= 0 or source_fps <= 0:
        return ()
    occupied = {round(value * source_fps) for value in already_scheduled}
    candidates: set[int] = set()
    refinement_step = max(1, round(source_fps / policy.refinement_cadence_fps))
    radius_frames = round(policy.refinement_window_seconds * source_fps)
    for trigger in sorted(set(float(value) for value in trigger_timestamps)):
        trigger_frame = round(trigger * source_fps)
        local = range(trigger_frame - radius_frames, trigger_frame + radius_frames + 1, refinement_step)
        accepted = 0
        for frame_number in local:
            timestamp = frame_number / source_fps
            if (
                frame_number < 0
                or timestamp >= duration_seconds
                or frame_number in occupied
                or not _allowed(timestamp, allowed_intervals)
            ):
                continue
            candidates.add(frame_number)
            accepted += 1
            if accepted >= policy.maximum_refinement_frames_per_trigger:
                break
    remaining = max(0, policy.maximum_analyzed_frames - len(occupied))
    return tuple(frame / source_fps for frame in sorted(candidates)[:remaining])


def _edit_distance(left: str, right: str) -> int:
    if left == right:
        return 0
    if not left:
        return len(right)
    previous = list(range(len(right) + 1))
    for left_index, left_char in enumerate(left, start=1):
        current = [left_index]
        for right_index, right_char in enumerate(right, start=1):
            current.append(min(
                current[-1] + 1,
                previous[right_index] + 1,
                previous[right_index - 1] + (left_char != right_char),
            ))
        previous = current
    return previous[-1]


def _observation_sort_key(item: PlateObservation) -> tuple[Any, ...]:
    return (
        item.timestamp_seconds,
        item.frame_number,
        item.bounds.x,
        item.bounds.y,
        item.normalized_ocr,
        item.raw_ocr,
        item.observation_id,
    )


def _deduplicate_observations(
    observations: Iterable[PlateObservation],
) -> list[PlateObservation]:
    selected: dict[tuple[Any, ...], PlateObservation] = {}
    for item in sorted(observations, key=_observation_sort_key):
        key = (
            item.frame_number,
            round(item.bounds.x),
            round(item.bounds.y),
            round(item.bounds.width),
            round(item.bounds.height),
            item.crop_sha256,
        )
        incumbent = selected.get(key)
        if incumbent is None or item.support_weight() > incumbent.support_weight():
            selected[key] = item
    return sorted(selected.values(), key=_observation_sort_key)


def _can_associate(
    prior: PlateObservation,
    current: PlateObservation,
    policy: AggregationPolicy,
) -> bool:
    gap = current.timestamp_seconds - prior.timestamp_seconds
    if gap < 0 or gap > policy.maximum_temporal_gap_seconds:
        return False
    if current.frame_number == prior.frame_number:
        return False
    spatially_adjacent = (
        prior.bounds.iou(current.bounds) >= policy.minimum_spatial_iou
        or prior.bounds.center_distance_ratio(current.bounds)
        <= policy.maximum_center_distance_ratio
    )
    if not spatially_adjacent:
        return False
    return _edit_distance(prior.normalized_ocr, current.normalized_ocr) <= (
        policy.maximum_candidate_edit_distance
    )


def _candidate_alternatives(
    members: Sequence[PlateObservation],
) -> tuple[CandidateAlternative, ...]:
    by_candidate: dict[str, list[PlateObservation]] = {}
    for item in members:
        by_candidate.setdefault(item.normalized_ocr, []).append(item)
    alternatives: list[CandidateAlternative] = []
    for normalized, sightings in by_candidate.items():
        alternatives.append(CandidateAlternative(
            normalized_plate_text=normalized,
            raw_readings=tuple(sorted({item.raw_ocr for item in sightings})),
            support_count=len(sightings),
            confidence_weighted_support=round(sum(item.support_weight() for item in sightings), 6),
            best_ocr_confidence=max(
                (item.ocr_confidence for item in sightings if item.ocr_confidence is not None),
                default=None,
            ),
        ))
    return tuple(sorted(
        alternatives,
        key=lambda item: (
            -item.support_count,
            -item.confidence_weighted_support,
            -(item.best_ocr_confidence or 0),
            item.normalized_plate_text,
        ),
    ))


def _select_candidate(
    alternatives: Sequence[CandidateAlternative], strategy: AggregationStrategy
) -> CandidateAlternative:
    if strategy == "best_confidence":
        key: Callable[[CandidateAlternative], tuple[Any, ...]] = lambda item: (
            -(item.best_ocr_confidence or 0),
            -item.confidence_weighted_support,
            -item.support_count,
            item.normalized_plate_text,
        )
    elif strategy == "confidence_weighted_support":
        key = lambda item: (
            -item.confidence_weighted_support,
            -item.support_count,
            -(item.best_ocr_confidence or 0),
            item.normalized_plate_text,
        )
    elif strategy == "normalized_majority":
        key = lambda item: (
            -item.support_count,
            -item.confidence_weighted_support,
            -(item.best_ocr_confidence or 0),
            item.normalized_plate_text,
        )
    else:
        raise ValueError(f"unsupported aggregation strategy {strategy!r}")
    return min(alternatives, key=key)


def aggregate_observations(
    observations: Iterable[PlateObservation],
    policy: AggregationPolicy,
) -> tuple[PlateObservationGroup, ...]:
    valid = [item for item in _deduplicate_observations(observations) if item.normalized_ocr]
    clusters: list[list[PlateObservation]] = []
    for item in valid:
        candidates = [
            (item.bounds.iou(cluster[-1].bounds), -item.bounds.center_distance_ratio(cluster[-1].bounds), index)
            for index, cluster in enumerate(clusters)
            if len(cluster) < policy.maximum_observations_per_group
            and _can_associate(cluster[-1], item, policy)
        ]
        if candidates:
            _, _, selected_index = max(candidates)
            clusters[selected_index].append(item)
        elif len(clusters) < policy.maximum_active_groups:
            clusters.append([item])

    output: list[PlateObservationGroup] = []
    for members in clusters:
        alternatives = _candidate_alternatives(members)
        selected = _select_candidate(alternatives, policy.strategy)
        if selected.support_count < policy.minimum_selected_support:
            continue
        supporting = [
            item for item in members if item.normalized_ocr == selected.normalized_plate_text
        ]
        best = min(
            supporting,
            key=lambda item: (
                -item.support_weight(),
                -float(item.ocr_confidence or 0),
                item.timestamp_seconds,
                item.observation_id,
            ),
        )
        ordered = sorted(members, key=_observation_sort_key)
        group_id = str(uuid.uuid5(
            uuid.NAMESPACE_URL,
            ":".join((
                ordered[0].source_sha256,
                ADAPTER_ID,
                ordered[0].observation_id,
                ordered[-1].observation_id,
                selected.normalized_plate_text,
            )),
        ))
        output.append(PlateObservationGroup(
            group_id=group_id,
            selected_normalized_plate=selected.normalized_plate_text,
            selected_raw_plate=best.raw_ocr,
            support_count=selected.support_count,
            first_observed_seconds=ordered[0].timestamp_seconds,
            last_observed_seconds=ordered[-1].timestamp_seconds,
            best_observation_id=best.observation_id,
            observation_ids=tuple(item.observation_id for item in ordered),
            alternatives=alternatives,
            bounds_at_best_observation=best.bounds,
            aggregation_strategy=policy.strategy,
        ))
    return tuple(sorted(output, key=lambda item: (
        item.first_observed_seconds, item.last_observed_seconds, item.group_id
    )))


def coverage_state(receipt: CoverageReceipt, observation_count: int) -> tuple[str, tuple[str, ...]]:
    if receipt.resource_stop:
        return "resource_stopped", ("processing stopped at the declared resource boundary",)
    if receipt.frames_decoded == 0:
        return "failed_zero_frames", ("no source frame was decoded",)
    if receipt.frames_analyzed_by_plate_detector == 0:
        return "processor_failed", ("no decoded frame reached the plate detector",)
    coverage_warning = (
        "sampling was partial; zero observations do not establish that the video contains no plates",
    ) if receipt.partial_sampling else ()
    if observation_count:
        return "complete_results", coverage_warning
    if receipt.partial_sampling:
        return "complete_zero_sampled_frames", coverage_warning
    return "complete_zero_full_coverage", ()


def make_coverage_receipt(
    *,
    source_fps: float,
    source_duration_seconds: float,
    source_frame_count: int,
    frames_decoded: int,
    frames_analyzed_by_plate_detector: int,
    frames_refined: int,
    ocr_crops_processed: int,
    sampling_policy: str,
    dropped_or_failed_frames: int,
    wall_seconds: float,
    cpu_seconds: float,
    peak_ram_mib: float | None,
    resource_stop: bool = False,
) -> CoverageReceipt:
    coverage = (
        frames_analyzed_by_plate_detector / source_frame_count if source_frame_count else 0.0
    )
    return CoverageReceipt(
        source_fps=source_fps,
        source_duration_seconds=source_duration_seconds,
        source_frame_count=source_frame_count,
        frames_decoded=frames_decoded,
        frames_analyzed_by_plate_detector=frames_analyzed_by_plate_detector,
        frames_refined=frames_refined,
        ocr_crops_processed=ocr_crops_processed,
        effective_frame_coverage=round(coverage, 6),
        sampling_policy=sampling_policy,
        dropped_or_failed_frames=dropped_or_failed_frames,
        wall_seconds=round(wall_seconds, 6),
        cpu_seconds=round(cpu_seconds, 6),
        peak_ram_mib=peak_ram_mib,
        partial_sampling=frames_analyzed_by_plate_detector < source_frame_count,
        resource_stop=resource_stop,
    )


def crop_quality(bounds: PlateBounds, frame_width: int, frame_height: int) -> float:
    if frame_width <= 0 or frame_height <= 0:
        return 0.0
    area_ratio = (bounds.width * bounds.height) / (frame_width * frame_height)
    return max(0.0, min(1.0, math.sqrt(area_ratio) * 8))


def observation_id(
    source_sha256: str,
    frame_number: int,
    bounds: PlateBounds,
    normalized_ocr: str,
) -> str:
    return str(uuid.uuid5(
        uuid.NAMESPACE_URL,
        f"{source_sha256}:{ADAPTER_ID}:{frame_number}:{bounds.as_locator()}:{normalized_ocr}",
    ))


def build_freeze_manifest(
    config: dict[str, Any],
    *,
    adapter_source_sha256: str,
    selected_candidate: dict[str, Any],
    development_split_digest: str,
) -> dict[str, Any]:
    manifest = {
        "contract_version": "nexusai.nxmmr.parity-adapter-freeze/v1",
        "adapter_id": ADAPTER_ID,
        "adapter_version": ADAPTER_VERSION,
        "source_commit": config["source_commit"],
        "adapter_source_sha256": adapter_source_sha256,
        "models": config["models"],
        "normalization": config["normalization"],
        "bounds": config["bounds"],
        "thresholds": config["thresholds"],
        "selected_candidate": selected_candidate,
        "development_split_digest": development_split_digest,
        "candidate_evaluation_performed": False,
    }
    manifest["freeze_digest"] = canonical_digest(manifest)
    return manifest


def measure_call(call: Callable[[], Any]) -> tuple[Any, float, float]:
    wall_started = time.perf_counter()
    cpu_started = time.process_time()
    result = call()
    return result, time.perf_counter() - wall_started, time.process_time() - cpu_started
