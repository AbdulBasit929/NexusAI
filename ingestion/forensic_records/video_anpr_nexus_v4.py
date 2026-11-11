# SPDX-License-Identifier: MIT
"""Video ANPR V4: the measured M5 plate engine inside the governed scheduler and grouping.

Design (docs/data/video-plates-research-and-preregistration-20261111.md): V2 and V3 stay frozen. V4 keeps their rules: only actual decoded source frames are read, every selected string is an OCR string
that was read on a frame, there is no track identity, no interpolation and no character substitution. It changes four things: (1) the engine is the M5 stack (RF-DETR plate detector with the non-plate
rule plus the global reader) and every observation carries that engine's own model hashes; (2) the frame schedule is a base cadence plus the existing refinement windows around frames that read a plate;
(3) each read gets a quality weight from reader and detector confidence, crop size and sharpness (variance of the Laplacian of the crop), used by the grouping to select the observed string with the highest
weighted support; (4) the same contracts and flags as V3 (`persistent_tracking: false`, `interpolated_observations: false`).
"""

from __future__ import annotations

import hashlib
import os
import time
import uuid
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any, Sequence

import numpy as np

try:
    from ingestion.forensic_records.video_anpr_onnx_v3 import _mean_confidence
    from ingestion.forensic_records.video_anpr_parity_adapter import (
        AggregationPolicy, FramePolicy, PlateBounds, PlateObservation, PlateObservationGroup, TimeInterval, _candidate_alternatives, aggregate_observations, coverage_state, crop_quality, make_coverage_receipt,
        normalize_plate_neutral, observation_id, schedule_base_frames, schedule_refinement_frames,
    )
    from ingestion.forensic_records.video_anpr_product_v2 import ProductAggregationPolicy, aggregate_product_observations
except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
    from video_anpr_onnx_v3 import _mean_confidence
    from video_anpr_parity_adapter import (
        AggregationPolicy, FramePolicy, PlateBounds, PlateObservation, PlateObservationGroup, TimeInterval, _candidate_alternatives, aggregate_observations, coverage_state, crop_quality, make_coverage_receipt,
        normalize_plate_neutral, observation_id, schedule_base_frames, schedule_refinement_frames,
    )
    from video_anpr_product_v2 import ProductAggregationPolicy, aggregate_product_observations

ADAPTER_ID = "NX-MMR-VIDEO-ANPR-NEXUS-V4"
ADAPTER_VERSION = "4.0.0-development"
POLICY_REVISION = "observed-support-quality-sharpness-v4"
FRAME_POLICY_ID = "base-cadence-with-refinement-v4"
SHARPNESS_REFERENCE_HEIGHT = 32          # crops are brought to this height before the Laplacian so that sharpness does not depend on size
SHARPNESS_HALF_POINT = 100.0             # Laplacian variance at which the sharpness score is 0.5
QUALITY_SIZE_SHARE = 0.5                 # quality = share * size score + (1 - share) * sharpness score

# A new policy object, not the frozen V2 one: the weights below are smaller than V2's (a product of factors), so the support floor differs. Chosen on DEV-V only (Amendments of the pre-registration).
DEFAULT_POLICY = ProductAggregationPolicy(
    strategy="confidence_weighted_support",
    minimum_selected_support=3,
    minimum_candidate_length=4,
    maximum_candidate_length=10,
    minimum_best_ocr_confidence=0.5,
    minimum_weighted_support=1.0,
)


@dataclass(frozen=True)
class PlateObservationV4(PlateObservation):
    """An observation whose weight is reader confidence times a quality factor (size and sharpness) times a detector factor."""

    sharpness: float | None = None

    def support_weight(self) -> float:
        ocr = self.ocr_confidence if self.ocr_confidence is not None else 0.0
        quality = self.crop_quality if self.crop_quality is not None else 0.0
        return ocr * (0.4 + 0.6 * quality) * (0.5 + 0.5 * self.detector_confidence)


def sharpness_score(cv2_module: Any, crop: Any) -> float:
    """Variance of the Laplacian of the grayscale crop at a fixed height, mapped to 0..1. A blurred crop scores low; a crisp one high."""
    height, width = crop.shape[:2]
    if height < 2 or width < 2:
        return 0.0
    gray = cv2_module.cvtColor(crop, cv2_module.COLOR_BGR2GRAY) if len(crop.shape) == 3 else crop
    scale = SHARPNESS_REFERENCE_HEIGHT / float(height)
    resized = cv2_module.resize(gray, (max(2, int(round(width * scale))), SHARPNESS_REFERENCE_HEIGHT), interpolation=cv2_module.INTER_AREA)
    variance = float(cv2_module.Laplacian(resized, cv2_module.CV_64F).var())
    return variance / (variance + SHARPNESS_HALF_POINT)


def combined_quality(size_score: float, sharp_score: float) -> float:
    return QUALITY_SIZE_SHARE * size_score + (1.0 - QUALITY_SIZE_SHARE) * sharp_score


def merge_same_text_groups(groups: Sequence[PlateObservationGroup], observations: Sequence[PlateObservation], gap_seconds: float = 1.5) -> tuple[PlateObservationGroup, ...]:
    """Groups that selected the same text and whose intervals overlap or lie within `gap_seconds` of each other are one group. The spatial association of the grouping can fail for a plate that moves
    far between two sampled frames (a vehicle approaching a fixed camera), which then reads as two groups of one plate. Only identical selected text is merged: no identity is claimed beyond it."""
    by_id = {o.observation_id: o for o in observations}
    merged: list[list[PlateObservationGroup]] = []
    for group in sorted(groups, key=lambda g: (g.selected_normalized_plate, g.first_observed_seconds)):
        if merged and merged[-1][0].selected_normalized_plate == group.selected_normalized_plate and group.first_observed_seconds <= max(g.last_observed_seconds for g in merged[-1]) + gap_seconds:
            merged[-1].append(group)
        else:
            merged.append([group])
    output: list[PlateObservationGroup] = []
    for parts in merged:
        if len(parts) == 1:
            output.append(parts[0])
            continue
        members = sorted({i: by_id[i] for g in parts for i in g.observation_ids if i in by_id}.values(), key=lambda o: (o.timestamp_seconds, o.observation_id))
        text = parts[0].selected_normalized_plate
        supporting = [o for o in members if o.normalized_ocr == text]
        best = min(supporting, key=lambda o: (-o.support_weight(), -float(o.ocr_confidence or 0), o.timestamp_seconds, o.observation_id))
        output.append(PlateObservationGroup(
            group_id=str(uuid.uuid5(uuid.NAMESPACE_URL, ":".join((members[0].source_sha256, ADAPTER_ID, members[0].observation_id, members[-1].observation_id, text)))),
            selected_normalized_plate=text, selected_raw_plate=best.raw_ocr, support_count=len(supporting), first_observed_seconds=members[0].timestamp_seconds,
            last_observed_seconds=members[-1].timestamp_seconds, best_observation_id=best.observation_id, observation_ids=tuple(o.observation_id for o in members),
            alternatives=_candidate_alternatives(members), bounds_at_best_observation=best.bounds, aggregation_strategy=parts[0].aggregation_strategy,
        ))
    return tuple(sorted(output, key=lambda g: (g.first_observed_seconds, g.last_observed_seconds, g.group_id)))


@dataclass(frozen=True)
class VideoV4Run:
    result: Any
    observations: tuple[PlateObservation, ...]
    groups: tuple[Any, ...]
    coverage: Any
    frames_base: int
    frames_refined: int


class VideoANPRNexusV4Processor:
    """Actual-frame video plate reading with the M5 engine. No vehicle model, no tracking, no interpolation."""

    processor_id = ADAPTER_ID
    processor_revision = ADAPTER_VERSION

    def __init__(self, image_processor: Any, cv2_module: Any, *, base_fps: float = 4.0, refinement: bool = True, sharpness_weight: bool = True,
                 policy: ProductAggregationPolicy = DEFAULT_POLICY, maximum_frames: int = 1200, maximum_duration_seconds: float = 900.0,
                 refinement_window_seconds: float = 0.5, refinement_cadence_fps: float = 8.0, maximum_refinement_per_trigger: int = 9,
                 refinement_mode: str = "per_group", refinement_per_group: int = 3, refinement_budget_ratio: float = 0.5, refinement_pad_seconds: float = 0.25, merge_gap_seconds: float = 1.5) -> None:
        self.image_processor = image_processor
        self.cv2 = cv2_module
        self.base_fps = base_fps
        self.refinement = refinement
        self.sharpness_weight = sharpness_weight
        self.policy = policy
        self.maximum_frames = maximum_frames
        self.maximum_duration_seconds = maximum_duration_seconds
        self.refinement_window_seconds = refinement_window_seconds
        self.refinement_cadence_fps = refinement_cadence_fps
        self.maximum_refinement_per_trigger = maximum_refinement_per_trigger
        # "per_group": after the base pass the reads are clustered and each cluster gets at most `refinement_per_group` extra frames inside its own interval (padded), unconfirmed clusters first, larger plates
        # first, and the extra frames in total are at most `refinement_budget_ratio` of the base frames. "windows": the earlier V3-style windows around every frame that read a plate (measured on a 15 s
        # motorway clip: 343 of 450 frames analysed, 25 times real time; kept only for comparison).
        self.refinement_mode = refinement_mode
        self.refinement_per_group = refinement_per_group
        self.refinement_budget_ratio = refinement_budget_ratio
        self.refinement_pad_seconds = refinement_pad_seconds
        self.merge_gap_seconds = merge_gap_seconds

    @classmethod
    def from_environment(cls) -> "VideoANPRNexusV4Processor":
        try:
            import cv2  # type: ignore[import-not-found]
            from ingestion.forensic_records.plate_processor_nexus import NexusPlateImageProcessor
        except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
            import cv2  # type: ignore[import-not-found]
            from plate_processor_nexus import NexusPlateImageProcessor
        return cls(
            NexusPlateImageProcessor.from_environment(minimum_confidence=float(os.getenv("FORENSIC_VIDEO_ANPR_V4_DETECTION_CONFIDENCE", "0.5"))),
            cv2,
            base_fps=max(0.5, min(25.0, float(os.getenv("FORENSIC_VIDEO_ANPR_V4_BASE_FPS", "4")))),
            maximum_frames=max(1, min(3600, int(os.getenv("FORENSIC_VIDEO_ANPR_V4_MAX_FRAMES", "1200")))),
            maximum_duration_seconds=max(1.0, float(os.getenv("FORENSIC_VIDEO_ANPR_V4_MAX_DURATION_SECONDS", "900"))),
        )

    # -- one decoded frame -------------------------------------------------------------------------------------------------------------------------------------------------------------------------
    def _observe(self, frame: Any, *, source_sha256: str, source_file: str, frame_number: int, timestamp: float) -> tuple[list[PlateObservation], int]:
        results = tuple(self.image_processor.engine.predict(frame))
        height, width = int(frame.shape[0]), int(frame.shape[1])
        hashes = getattr(self.image_processor, "model_hashes", {}) or {}
        # the hash of the decoded pixels (BGR, row by row), not of a PNG encoding of them: the same value can be recomputed from the decoded frame, and it costs a few milliseconds instead of about 0.1 s per 1080p frame
        frame_hash = hashlib.sha256(np.ascontiguousarray(frame).tobytes()).hexdigest()
        observations: list[PlateObservation] = []
        for result in results[:32]:
            box = result.detection.bounding_box
            x1, y1 = max(0, min(int(box.x1), width)), max(0, min(int(box.y1), height))
            x2, y2 = max(0, min(int(box.x2), width)), max(0, min(int(box.y2), height))
            if x2 <= x1 or y2 <= y1 or result.ocr is None:
                continue
            raw = str(result.ocr.text or "")
            normalized = normalize_plate_neutral(raw)
            if not normalized:
                continue
            bounds = PlateBounds(float(x1), float(y1), float(x2 - x1), float(y2 - y1))
            crop = frame[y1:y2, x1:x2]
            crop_encoded, crop_bytes = self.cv2.imencode(".png", crop)
            size_score = crop_quality(bounds, width, height)
            sharp = sharpness_score(self.cv2, crop)
            fields = dict(
                observation_id=observation_id(source_sha256, frame_number, bounds, normalized), source_sha256=source_sha256, source_file=source_file, frame_number=frame_number,
                timestamp_seconds=timestamp, source_frame_sha256=frame_hash, bounds=bounds, raw_ocr=raw, normalized_ocr=normalized, detector_confidence=float(result.detection.confidence),
                ocr_confidence=_mean_confidence(result.ocr.confidence), crop_sha256=hashlib.sha256(bytes(crop_bytes)).hexdigest() if crop_encoded else "",
                detector_model_sha256=str(hashes.get("detector", "")), ocr_model_sha256=str(hashes.get("ocr", "")), vehicle_context_matched=None,
            )
            if self.sharpness_weight:
                observations.append(PlateObservationV4(**fields, crop_quality=combined_quality(size_score, sharp), sharpness=sharp))
            else:
                observations.append(PlateObservation(**fields, crop_quality=size_score))
        return observations, len(results)

    def _per_group_refinement(self, observations: Sequence[PlateObservation], analysed: dict[int, list[PlateObservation]], fps: float, duration: float, frames_base: int,
                              allowed_intervals: Sequence[TimeInterval] | None) -> list[float]:
        """Extra frames for the reads of the base pass: a few per cluster, spread over the cluster's interval, unconfirmed and larger plates first, bounded in total."""
        clusters = aggregate_observations(observations, AggregationPolicy(strategy="confidence_weighted_support", minimum_selected_support=1, maximum_temporal_gap_seconds=1.0))
        budget = min(int(self.refinement_budget_ratio * frames_base), max(0, self.maximum_frames - len(analysed)))
        order = sorted(clusters, key=lambda g: (g.support_count >= 3, -(g.bounds_at_best_observation.width * g.bounds_at_best_observation.height)))
        step = max(1, round(fps / self.refinement_cadence_fps))
        chosen: list[float] = []
        taken = set(analysed)
        for group in order:
            if len(chosen) >= budget:
                break
            first = max(0, round((group.first_observed_seconds - self.refinement_pad_seconds) * fps))
            last = min(int(duration * fps) - 1, round((group.last_observed_seconds + self.refinement_pad_seconds) * fps))
            candidates = [n for n in range(first, last + 1, step) if n not in taken and (allowed_intervals is None or any(i.contains(n / fps) for i in allowed_intervals))]
            if not candidates:
                continue
            count = min(self.refinement_per_group, len(candidates), budget - len(chosen))
            picks = [candidates[round(i * (len(candidates) - 1) / max(1, count - 1))] if count > 1 else candidates[len(candidates) // 2] for i in range(count)]
            for n in dict.fromkeys(picks):
                taken.add(n)
                chosen.append(n / fps)
        return chosen

    # -- one video ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
    def process_video(self, path: Path, **context: object) -> Any:
        return self.run_video(path, context=context).result

    def run_video(self, path: Path, *, context: dict[str, object], allowed_intervals: Sequence[TimeInterval] | None = None) -> VideoV4Run:
        try:
            from ingestion.forensic_records.media_pipeline import MediaObservation, MediaProcessResult, READY, READY_WITH_WARNINGS
        except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
            from media_pipeline import MediaObservation, MediaProcessResult, READY, READY_WITH_WARNINGS

        capture = self.cv2.VideoCapture(str(path))
        if not capture.isOpened():
            raise RuntimeError("Video ANPR V4 could not decode the source")
        fps = float(capture.get(self.cv2.CAP_PROP_FPS))
        frame_count = int(capture.get(self.cv2.CAP_PROP_FRAME_COUNT))
        duration = frame_count / fps if fps > 0 else 0.0
        if fps <= 0 or frame_count <= 0 or duration <= 0 or duration > self.maximum_duration_seconds:
            capture.release()
            raise RuntimeError("Video ANPR V4 source metadata violates configured bounds")
        policy = FramePolicy(
            FRAME_POLICY_ID, self.base_fps, refinement_enabled=self.refinement, refinement_window_seconds=self.refinement_window_seconds,
            refinement_cadence_fps=self.refinement_cadence_fps, maximum_analyzed_frames=self.maximum_frames, maximum_refinement_frames_per_trigger=self.maximum_refinement_per_trigger,
        )
        base = schedule_base_frames(duration, fps, policy, allowed_intervals=allowed_intervals)
        source_sha256, source_file = str(context["source_sha256"]), str(context["source_file"])
        observations: list[PlateObservation] = []
        analysed: dict[int, list[PlateObservation]] = {}
        failed = crops = 0
        wall_started, cpu_started = time.perf_counter(), time.process_time()

        def read(timestamp: float) -> None:
            nonlocal failed, crops
            number = round(timestamp * fps)
            if number in analysed:
                return
            capture.set(self.cv2.CAP_PROP_POS_FRAMES, number)
            ok, frame = capture.read()
            if not ok:
                failed += 1
                return
            found, detected = self._observe(frame, source_sha256=source_sha256, source_file=source_file, frame_number=number, timestamp=number / fps)
            analysed[number] = found
            observations.extend(found)
            crops += detected

        try:
            for timestamp in base:
                read(timestamp)
            frames_base = len(analysed)
            if self.refinement and self.refinement_mode == "windows":
                triggers = [number / fps for number, found in sorted(analysed.items()) if found]
                for timestamp in schedule_refinement_frames(triggers, duration, fps, policy, already_scheduled=[n / fps for n in analysed], allowed_intervals=allowed_intervals):
                    read(timestamp)
            elif self.refinement:
                for timestamp in self._per_group_refinement(observations, analysed, fps, duration, frames_base, allowed_intervals):
                    read(timestamp)
        finally:
            capture.release()
        frames_refined = len(analysed) - frames_base

        groups = aggregate_product_observations(observations, self.policy)
        if self.merge_gap_seconds > 0:
            groups = merge_same_text_groups(groups, observations, self.merge_gap_seconds)
        coverage = make_coverage_receipt(
            source_fps=fps, source_duration_seconds=duration, source_frame_count=frame_count, frames_decoded=len(analysed), frames_analyzed_by_plate_detector=len(analysed),
            frames_refined=frames_refined, ocr_crops_processed=crops, sampling_policy=policy.policy_id, dropped_or_failed_frames=failed,
            wall_seconds=time.perf_counter() - wall_started, cpu_seconds=time.process_time() - cpu_started, peak_ram_mib=None,
        )
        state, coverage_limitations = coverage_state(coverage, len(observations))
        packets: list[Any] = []
        engine_note = {"processor": ADAPTER_ID, "processor_revision": ADAPTER_VERSION, "engine_processor": getattr(self.image_processor, "processor_id", ""),
                       "engine_detector": getattr(self.image_processor, "detector_id", ""), "engine_ocr": getattr(self.image_processor, "ocr_id", ""), "vehicle_context": "not_used_plate_only", "frame_hash_method": "sha256 of the decoded BGR pixel bytes"}
        for item in observations:
            packet = item.to_packet(evidence_id=str(context["evidence_id"]), version_id=str(context["version_id"]))
            packet["payload"].update(engine_note)
            if isinstance(item, PlateObservationV4):
                packet["payload"]["sharpness"] = round(item.sharpness or 0.0, 4)
            packets.append(MediaObservation(observation_id=packet["observation_id"], contract_version=packet["contract_version"], observation_type=packet["observation_type"],
                                            confidence=packet["confidence"], citation_locator=packet["citation_locator"], payload=packet["payload"], warnings=tuple(packet["warnings"])))
        for group in groups:
            packet = group.to_packet(source_file=source_file, evidence_id=str(context["evidence_id"]), version_id=str(context["version_id"]))
            packet["payload"].update({**engine_note, "aggregation_revision": POLICY_REVISION})
            packets.append(MediaObservation(observation_id=packet["observation_id"], contract_version=packet["contract_version"], observation_type=packet["observation_type"],
                                            confidence=None, citation_locator=packet["citation_locator"], payload=packet["payload"], warnings=tuple(packet["warnings"])))
        result_state = "COMPLETE_RESULTS" if groups else "COMPLETE_ZERO_RESULTS"
        result = MediaProcessResult(
            modality="video", readiness=READY if groups else READY_WITH_WARNINGS, processor_id=ADAPTER_ID, processor_revision=ADAPTER_VERSION, observations=tuple(packets),
            limitations=(*coverage_limitations, "Development model observations require analyst review", "Plate-only temporal grouping is not tracking, identity, ownership, or continuous presence"),
            metadata={
                "result_states": {"video_anpr": result_state}, "coverage": asdict(coverage), "raw_observation_count": len(observations), "group_count": len(groups),
                "selected_policy": asdict(self.policy), "execution_state": state, "vehicle_context": "not_used_plate_only", "interpolated_observations": False, "persistent_tracking": False,
                "frames_base": frames_base, "frames_refined": frames_refined, "refinement": self.refinement, "refinement_mode": self.refinement_mode, "sharpness_weight": self.sharpness_weight,
            },
        )
        return VideoV4Run(result, tuple(observations), groups, coverage, frames_base, frames_refined)


# ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# High-resolution frames. A plate in a 4K frame of a motorway seen from a bridge is 40 to 200 px wide; the detector looks at the frame reduced to 576 px, so only the nearer plates survive that. The wrapper
# below also looks at overlapping tiles at native resolution on large frames, drops boxes that are not plate-shaped (the detector also boxes whole cars and blurred patches) and reads every kept box from the
# full-resolution frame. It changes nothing for frames of ordinary size except the shape filter.
# ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------


def _iou(a: Sequence[float], b: Sequence[float]) -> float:
    x1, y1, x2, y2 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0, x2 - x1) * max(0, y2 - y1)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union else 0.0


def merge_boxes(boxes: Sequence[Sequence[float]], threshold: float = 0.3) -> list[tuple[int, int, int, int, float]]:
    """Greedy merge by overlap: of boxes that overlap by more than `threshold`, the one with the highest score is kept."""
    kept: list[Sequence[float]] = []
    for box in sorted(boxes, key=lambda item: -item[4]):
        if all(_iou(box, other) < threshold for other in kept):
            kept.append(box)
    return [(int(b[0]), int(b[1]), int(b[2]), int(b[3]), float(b[4])) for b in kept]


def tile_origins(width: int, height: int, side: int, stride: int) -> list[tuple[int, int]]:
    xs = list(range(0, max(1, width - side) + 1, stride))
    ys = list(range(0, max(1, height - side) + 1, stride))
    if xs[-1] + side < width:
        xs.append(max(0, width - side))
    if ys[-1] + side < height:
        ys.append(max(0, height - side))
    return [(x, y) for y in ys for x in xs]


class HighResolutionPlateEngine:
    """Wraps an engine with `.detector` (frame -> (x1, y1, x2, y2, score)) and `.ocr` (crop -> read). Same `predict(frame)` contract as the engine it wraps."""

    def __init__(self, engine: Any, *, tile_side: int | None = 1280, wide_px: int = 2200, tile_every: int = 1, min_aspect: float = 1.5, minimum_width_px: int = 12, merge_iou: float = 0.3) -> None:
        self.engine = engine
        self.detector = engine.detector
        self.ocr = engine.ocr
        self.tile_side = tile_side
        self.wide_px = wide_px
        self.tile_every = max(1, tile_every)
        self.min_aspect = min_aspect
        self.minimum_width_px = minimum_width_px
        self.merge_iou = merge_iou
        self.calls = 0
        self.tile_calls = 0

    def predict(self, frame: Any) -> list[Any]:
        try:
            from ingestion.forensic_records.plate_processor_nexus import _Box, _Detection, _Ocr, _PlateResult
        except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
            from plate_processor_nexus import _Box, _Detection, _Ocr, _PlateResult
        height, width = int(frame.shape[0]), int(frame.shape[1])
        boxes: list[Sequence[float]] = list(self.detector.detect(frame))
        if self.tile_side and max(width, height) >= self.wide_px and self.calls % self.tile_every == 0:
            side = min(self.tile_side, width, height)
            for x, y in tile_origins(width, height, side, int(side * 0.8)):
                self.tile_calls += 1
                for x1, y1, x2, y2, score in self.detector.detect(frame[y:y + side, x:x + side]):
                    boxes.append((x1 + x, y1 + y, x2 + x, y2 + y, score))
        self.calls += 1
        results: list[Any] = []
        for x1, y1, x2, y2, score in merge_boxes(boxes, self.merge_iou):
            box_width, box_height = x2 - x1, y2 - y1
            if box_width < self.minimum_width_px or box_height < 4 or box_width / box_height < self.min_aspect:
                continue
            read = self.ocr.predict(frame[y1:y2, x1:x2])
            ocr = _Ocr(read.text, read.confidence) if read is not None else None
            results.append(_PlateResult(_Detection(score, _Box(x1, y1, x2, y2)), ocr))
        return results


def with_high_resolution(image_processor: Any, **options: Any) -> Any:
    """Replaces the engine of an image processor (the Nexus plate processor) by the high-resolution wrapper and returns the same object."""
    if not isinstance(image_processor.engine, HighResolutionPlateEngine):
        image_processor.engine = HighResolutionPlateEngine(image_processor.engine, **options)
    return image_processor
