# SPDX-License-Identifier: MIT
"""Video ANPR V4: the measured M5 plate engine inside the governed scheduler and grouping.

Design (docs/data/video-plates-research-and-preregistration-20261111.md): V2 and V3 stay frozen. V4 keeps their rules: only actual decoded source frames are read, every selected string is an OCR string
that was read on a frame, there is no track identity, no interpolation and no character substitution. It changes four things: (1) the engine is the M5 stack (RF-DETR plate detector with the non-plate
rule plus the global reader) and every observation carries that engine's own model hashes; (2) the frame schedule is a base cadence plus the existing refinement windows around frames that read a plate;
(3) each read gets a quality weight from reader and detector confidence, crop size and sharpness (variance of the Laplacian of the crop), used by the grouping to select the observed string with the highest
weighted support; (4) the same contracts and flags as V3 (`persistent_tracking: false`, `interpolated_observations: false`).
"""

from __future__ import annotations

import dataclasses
import hashlib
import math
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
        AggregationPolicy, FramePolicy, PlateBounds, PlateObservation, PlateObservationGroup, TimeInterval, _candidate_alternatives, _edit_distance, _select_candidate, aggregate_observations, coverage_state, crop_quality, make_coverage_receipt,
        normalize_plate_neutral, observation_id, schedule_base_frames, schedule_refinement_frames,
    )
    from ingestion.forensic_records.video_anpr_product_v2 import ProductAggregationPolicy, aggregate_product_observations
except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
    from video_anpr_onnx_v3 import _mean_confidence
    from video_anpr_parity_adapter import (
        AggregationPolicy, FramePolicy, PlateBounds, PlateObservation, PlateObservationGroup, TimeInterval, _candidate_alternatives, _edit_distance, _select_candidate, aggregate_observations, coverage_state, crop_quality, make_coverage_receipt,
        normalize_plate_neutral, observation_id, schedule_base_frames, schedule_refinement_frames,
    )
    from video_anpr_product_v2 import ProductAggregationPolicy, aggregate_product_observations

ADAPTER_ID = "NX-MMR-VIDEO-ANPR-NEXUS-V4"
ADAPTER_VERSION = "4.1.0-development"
POLICY_REVISION = "observed-support-quality-sharpness-v4"
FRAME_POLICY_ID = "base-cadence-with-refinement-v4"
PROGRESSIVE_FRAME_POLICY_ID = "progressive-whole-video-with-refinement-v4.1"
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
    minimum_weighted_support=0.6,       # was 1.0: the weights are a product of factors, so a triple of moderately confident reads (about 0.2 each) was dropped that V3's weights would have kept
    maximum_active_groups=1024,         # was 128 (the V2 default): in a long or noisy clip (night glare: 416 reads, 279 distinct strings in 37 s) junk clusters filled the cap and later real plates were dropped
)


def progressive_schedule(duration_seconds: float, source_fps: float, base_fps: float, maximum_frames: int, allowed_intervals: Sequence[TimeInterval] | None = None,
                         levels: Sequence[int] | None = None) -> tuple[float, ...]:
    """The order in which frames are read: a coarse pass over the WHOLE video first (every 8th base frame, 0.5 per second at the default cadence), then the frames in between, finer and finer, ending
    at the base cadence. The older schedule read the base cadence from the start and stopped at `maximum_frames` or at the time budget, so a long video was read densely for its first minutes and
    not at all after that; this one leaves the whole video sampled, sparsely, whenever it has to stop. The result is in reading order, each frame once, and has at most `maximum_frames` frames. The
    coarsest step doubles until the coarse pass alone fits in `maximum_frames`, so the cut never falls inside it: it thins a finer level, never the coverage of the video."""
    if duration_seconds <= 0 or source_fps <= 0:
        return ()
    fine = max(1, round(source_fps / base_fps))
    total = max(1, math.ceil(duration_seconds * source_fps))
    if levels is None:
        top = 8
        while math.ceil(total / (fine * top)) > maximum_frames:
            top *= 2
        levels = []
        step = top
        while step >= 1:
            levels.append(step)
            step //= 2
    seen: set[int] = set()
    order: list[float] = []
    for multiple in levels:
        for number in range(0, total, fine * multiple):
            if number in seen:
                continue
            timestamp = number / source_fps
            if timestamp >= duration_seconds or (allowed_intervals is not None and not any(i.contains(timestamp) for i in allowed_intervals)):
                continue
            seen.add(number)
            order.append(timestamp)
    return tuple(order[:maximum_frames])


def default_engine_processor(minimum_confidence: float | None = None) -> Any | None:
    """The licence-clean default engine (YOLO plate detector with the cct-xs reader, the engine of V3) as a second, independent reader, with its model hashes. None when its models are not configured."""
    try:
        from ingestion.forensic_records.media_pipeline import FastALPRImageProcessor, _sha256
    except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
        from media_pipeline import FastALPRImageProcessor, _sha256
    detector, ocr, config = (os.getenv(name, "").strip() for name in ("FORENSIC_ANPR_DETECTOR_MODEL_PATH", "FORENSIC_ANPR_OCR_MODEL_PATH", "FORENSIC_ANPR_OCR_CONFIG_PATH"))
    if not (detector and ocr and config and Path(detector).exists() and Path(ocr).exists()):
        return None
    confidence = float(os.getenv("FORENSIC_VIDEO_ANPR_V3_DETECTION_CONFIDENCE", "0.25")) if minimum_confidence is None else minimum_confidence
    try:
        processor = FastALPRImageProcessor.from_environment(minimum_confidence=confidence)
    except TypeError:   # the class inside the production worker image is older: it reads its detection threshold from the environment
        previous = os.environ.get("FORENSIC_ANPR_MIN_DETECTION_CONFIDENCE")
        os.environ["FORENSIC_ANPR_MIN_DETECTION_CONFIDENCE"] = str(confidence)
        try:
            processor = FastALPRImageProcessor.from_environment()
        finally:
            if previous is None:
                os.environ.pop("FORENSIC_ANPR_MIN_DETECTION_CONFIDENCE", None)
            else:
                os.environ["FORENSIC_ANPR_MIN_DETECTION_CONFIDENCE"] = previous
    if not getattr(processor, "model_hashes", None):
        processor.model_hashes = {"detector": _sha256(Path(detector)), "ocr": _sha256(Path(ocr)), "ocr_config": _sha256(Path(config)) if Path(config).exists() else ""}
    return processor


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


def merge_overlapping_groups(groups: Sequence[PlateObservationGroup], observations: Sequence[PlateObservation], gap_seconds: float = 1.5, max_edit_distance: int = 0,
                             policy: ProductAggregationPolicy | None = None) -> tuple[PlateObservationGroup, ...]:
    """One group out of groups that are the same plate seen twice. Two groups are one when their selected texts are identical and their intervals touch within `gap_seconds` (the spatial association of the
    grouping can fail for a plate that moves far between two sampled frames), or, with `max_edit_distance` 1, when the texts are one edit apart and reads of the two groups taken at the same moment lie at the
    same place (the grouping never puts two reads of one frame into one group, so two engines reading the same plate form two groups; this is how their reads reinforce each other). The merged group selects,
    as always, an observed string by weighted support, and keeps the others as alternatives; it must still pass the precision gates of `policy`, else the groups stay as they were."""
    policy = policy or DEFAULT_POLICY
    by_id = {o.observation_id: o for o in observations}
    members_of = [sorted((by_id[i] for i in g.observation_ids if i in by_id), key=lambda o: (o.timestamp_seconds, o.observation_id)) for g in groups]

    def touch(a: int, b: int) -> bool:
        ga, gb = groups[a], groups[b]
        distance = _edit_distance(ga.selected_normalized_plate, gb.selected_normalized_plate)
        if distance > max_edit_distance:
            return False
        gap = max(ga.first_observed_seconds, gb.first_observed_seconds) - min(ga.last_observed_seconds, gb.last_observed_seconds)
        if distance == 0:
            return gap <= gap_seconds
        if gap > 0:
            return False
        return any(abs(x.timestamp_seconds - y.timestamp_seconds) <= 0.12 and (x.bounds.iou(y.bounds) >= 0.05 or x.bounds.center_distance_ratio(y.bounds) <= 2.0)
                   for x in members_of[a] for y in members_of[b])

    parent = list(range(len(groups)))

    def find(i: int) -> int:
        while parent[i] != i:
            parent[i] = parent[parent[i]]
            i = parent[i]
        return i

    for a in range(len(groups)):
        for b in range(a + 1, len(groups)):
            if find(a) != find(b) and touch(a, b):
                parent[find(b)] = find(a)
    components: dict[int, list[int]] = {}
    for index in range(len(groups)):
        components.setdefault(find(index), []).append(index)
    output: list[PlateObservationGroup] = []
    for indexes in components.values():
        if len(indexes) == 1:
            output.append(groups[indexes[0]])
            continue
        members = sorted({o.observation_id: o for i in indexes for o in members_of[i]}.values(), key=lambda o: (o.timestamp_seconds, o.observation_id))
        alternatives = _candidate_alternatives(members)
        selected = _select_candidate(alternatives, "confidence_weighted_support")
        text = selected.normalized_plate_text
        if (not text.isalnum() or not policy.minimum_candidate_length <= len(text) <= policy.maximum_candidate_length or (selected.best_ocr_confidence or 0.0) < policy.minimum_best_ocr_confidence
                or selected.confidence_weighted_support < policy.minimum_weighted_support or selected.support_count < policy.minimum_selected_support):
            output.extend(groups[i] for i in indexes)
            continue
        supporting = [o for o in members if o.normalized_ocr == text]
        best = min(supporting, key=lambda o: (-o.support_weight(), -float(o.ocr_confidence or 0), o.timestamp_seconds, o.observation_id))
        output.append(PlateObservationGroup(
            group_id=str(uuid.uuid5(uuid.NAMESPACE_URL, ":".join((members[0].source_sha256, ADAPTER_ID, members[0].observation_id, members[-1].observation_id, text)))),
            selected_normalized_plate=text, selected_raw_plate=best.raw_ocr, support_count=len(supporting), first_observed_seconds=members[0].timestamp_seconds,
            last_observed_seconds=members[-1].timestamp_seconds, best_observation_id=best.observation_id, observation_ids=tuple(o.observation_id for o in members),
            alternatives=alternatives, bounds_at_best_observation=best.bounds, aggregation_strategy=groups[indexes[0]].aggregation_strategy,
        ))
    return tuple(sorted(output, key=lambda g: (g.first_observed_seconds, g.last_observed_seconds, g.group_id)))


def merge_same_text_groups(groups: Sequence[PlateObservationGroup], observations: Sequence[PlateObservation], gap_seconds: float = 1.5) -> tuple[PlateObservationGroup, ...]:
    return merge_overlapping_groups(groups, observations, gap_seconds, 0)


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

    def __init__(self, image_processor: Any, cv2_module: Any, *, extra_processors: Sequence[Any] = (), engine_every: Sequence[int] = (), base_fps: float = 4.0, refinement: bool = True, sharpness_weight: bool = True,
                 policy: ProductAggregationPolicy = DEFAULT_POLICY, maximum_frames: int = 6000, maximum_duration_seconds: float = 14400.0,
                 refinement_window_seconds: float = 0.5, refinement_cadence_fps: float = 8.0, maximum_refinement_per_trigger: int = 9,
                 refinement_mode: str = "per_group", refinement_per_group: int = 3, refinement_budget_ratio: float = 0.5, refinement_pad_seconds: float = 0.25, merge_gap_seconds: float = 1.5, maximum_wall_seconds: float = 540.0,
                 progressive: bool = True, sparse_support: int = 2, sparse_fps_below: float = 2.0) -> None:
        self.image_processor = image_processor
        # Every sampled frame is read by each of these engines; the reads of all of them go into one grouping (an ensemble of independent readers). The first one is the primary engine.
        self.processors = (image_processor, *extra_processors)
        # engine_every[i]: engine i reads every n-th BASE frame (refinement frames are read by every engine). A slow engine can read every second base frame while a fast one reads all of them.
        self.engine_every = tuple(max(1, int(n)) for n in engine_every) + (1,) * (len(self.processors) - len(engine_every))
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
        # A job has a lease (FORENSIC_WORKER_LEASE_SECONDS, 600 by default): a long video must not run past it. When the budget is spent the run stops, keeps what it read and says so in its coverage receipt.
        self.maximum_wall_seconds = maximum_wall_seconds
        # progressive: read the whole video coarse to fine (see progressive_schedule). sparse_support: when the frames actually read give fewer than `sparse_fps_below` frames per second of video, a plate
        # needs this many reads (not the policy's 3) to become a group, because a plate that is in view for two seconds is read once or twice at that rate; the groups say so in their payload.
        self.progressive = progressive
        self.sparse_support = sparse_support
        self.sparse_fps_below = sparse_fps_below

    @classmethod
    def from_environment(cls) -> "VideoANPRNexusV4Processor":
        try:
            import cv2  # type: ignore[import-not-found]
            from ingestion.forensic_records.plate_processor_nexus import NexusPlateImageProcessor
        except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
            import cv2  # type: ignore[import-not-found]
            from plate_processor_nexus import NexusPlateImageProcessor
        extra: list[Any] = []
        if os.getenv("FORENSIC_VIDEO_ANPR_V4_SECOND_ENGINE", "true").strip().lower() in {"1", "true", "yes", "on"}:
            second = default_engine_processor()
            if second is not None:
                extra.append(second)
        primary = NexusPlateImageProcessor.from_environment(minimum_confidence=float(os.getenv("FORENSIC_VIDEO_ANPR_V4_DETECTION_CONFIDENCE", "0.5")))
        with_high_resolution(primary, tile_side=1280, wide_px=int(os.getenv("FORENSIC_VIDEO_ANPR_V4_TILE_MIN_PX", "1600")), tile_every=max(1, int(os.getenv("FORENSIC_VIDEO_ANPR_V4_TILE_EVERY", "2"))),
                             tile_every_hd=max(1, int(os.getenv("FORENSIC_VIDEO_ANPR_V4_TILE_EVERY_HD", "1"))), min_aspect=1.5)
        return cls(
            primary,
            cv2,
            extra_processors=tuple(extra),
            engine_every=(2, 1) if extra else (),       # the frozen configuration of the pre-registration (Amendments 2 and 3): the slow primary engine reads every second base frame, the fast engine every one
            base_fps=max(0.5, min(25.0, float(os.getenv("FORENSIC_VIDEO_ANPR_V4_BASE_FPS", "4")))),
            maximum_frames=max(1, min(20000, int(os.getenv("FORENSIC_VIDEO_ANPR_V4_MAX_FRAMES", "6000")))),
            maximum_duration_seconds=max(1.0, float(os.getenv("FORENSIC_VIDEO_ANPR_V4_MAX_DURATION_SECONDS", "14400"))),
            progressive=os.getenv("FORENSIC_VIDEO_ANPR_V4_PROGRESSIVE", "true").strip().lower() in {"1", "true", "yes", "on"},
            sparse_support=max(1, int(os.getenv("FORENSIC_VIDEO_ANPR_V4_SPARSE_SUPPORT", "2"))),
            maximum_wall_seconds=max(30.0, float(os.getenv("FORENSIC_VIDEO_ANPR_V4_MAX_WALL_SECONDS", "540"))),
        )

    # -- one decoded frame -------------------------------------------------------------------------------------------------------------------------------------------------------------------------
    def _observe(self, frame: Any, processor: Any, frame_hash: str, *, source_sha256: str, source_file: str, frame_number: int, timestamp: float) -> tuple[list[PlateObservation], int]:
        results = tuple(processor.engine.predict(frame))
        height, width = int(frame.shape[0]), int(frame.shape[1])
        hashes = getattr(processor, "model_hashes", {}) or {}
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
                observation = PlateObservationV4(**fields, crop_quality=combined_quality(size_score, sharp), sharpness=sharp)
            else:
                observation = PlateObservation(**fields, crop_quality=size_score)
            observations.append(observation)
            self._engine_of[observation.observation_id] = processor
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
            PROGRESSIVE_FRAME_POLICY_ID if self.progressive else FRAME_POLICY_ID, self.base_fps, refinement_enabled=self.refinement, refinement_window_seconds=self.refinement_window_seconds,
            refinement_cadence_fps=self.refinement_cadence_fps, maximum_analyzed_frames=self.maximum_frames, maximum_refinement_frames_per_trigger=self.maximum_refinement_per_trigger,
        )
        base = (progressive_schedule(duration, fps, self.base_fps, self.maximum_frames, allowed_intervals) if self.progressive
                else schedule_base_frames(duration, fps, policy, allowed_intervals=allowed_intervals))
        source_sha256, source_file = str(context["source_sha256"]), str(context["source_file"])
        self._engine_of: dict[str, Any] = {}
        observations: list[PlateObservation] = []
        analysed: dict[int, list[PlateObservation]] = {}
        failed = crops = base_index = 0
        wall_started, cpu_started = time.perf_counter(), time.process_time()
        deadline = wall_started + self.maximum_wall_seconds
        stopped = False

        def read(timestamp: float, refined: bool = False) -> None:
            nonlocal failed, crops, base_index, stopped
            if stopped or time.perf_counter() > deadline:
                stopped = True
                return
            number = round(timestamp * fps)
            if number in analysed:
                return
            capture.set(self.cv2.CAP_PROP_POS_FRAMES, number)
            ok, frame = capture.read()
            if not ok:
                failed += 1
                return
            # the hash of the decoded pixels (BGR, row by row), not of a PNG encoding of them: the same value can be recomputed from the decoded frame, and it costs a few milliseconds instead of about 0.1 s per 1080p frame
            frame_hash = hashlib.sha256(np.ascontiguousarray(frame).tobytes()).hexdigest()
            found: list[PlateObservation] = []
            if not refined:
                base_index += 1
            for position, processor in enumerate(self.processors):
                if not refined and (base_index - 1) % self.engine_every[position] != 0:
                    continue
                read_by, detected = self._observe(frame, processor, frame_hash, source_sha256=source_sha256, source_file=source_file, frame_number=number, timestamp=number / fps)
                found.extend(read_by)
                crops += detected
            analysed[number] = found
            observations.extend(found)

        try:
            for timestamp in base:
                read(timestamp)
            frames_base = len(analysed)
            if self.refinement and self.refinement_mode == "windows":
                triggers = [number / fps for number, found in sorted(analysed.items()) if found]
                for timestamp in schedule_refinement_frames(triggers, duration, fps, policy, already_scheduled=[n / fps for n in analysed], allowed_intervals=allowed_intervals):
                    read(timestamp, True)
            elif self.refinement:
                for timestamp in self._per_group_refinement(observations, analysed, fps, duration, frames_base, allowed_intervals):
                    read(timestamp, True)
        finally:
            capture.release()
        frames_refined = len(analysed) - frames_base

        policy_used = self.policy
        scope_seconds = sum(i.end_seconds - i.start_seconds for i in allowed_intervals) if allowed_intervals else duration
        effective_fps = frames_base / scope_seconds if scope_seconds > 0 else 0.0
        if self.progressive and effective_fps < self.sparse_fps_below and self.sparse_support < policy_used.minimum_selected_support:
            # two reads of one plate are about 1 / effective_fps seconds apart, so the grouping must be allowed to join reads that far apart (its normal limit is one second)
            policy_used = dataclasses.replace(policy_used, minimum_selected_support=self.sparse_support,
                                              maximum_temporal_gap_seconds=max(policy_used.maximum_temporal_gap_seconds, 2.5 / max(effective_fps, 0.05)))
        groups = aggregate_product_observations(observations, policy_used)
        if self.merge_gap_seconds > 0:
            groups = merge_overlapping_groups(groups, observations, self.merge_gap_seconds, 1 if len(self.processors) > 1 else 0, policy_used)
        coverage = make_coverage_receipt(
            source_fps=fps, source_duration_seconds=duration, source_frame_count=frame_count, frames_decoded=len(analysed), frames_analyzed_by_plate_detector=len(analysed),
            frames_refined=frames_refined, ocr_crops_processed=crops, sampling_policy=policy.policy_id, dropped_or_failed_frames=failed,
            wall_seconds=time.perf_counter() - wall_started, cpu_seconds=time.process_time() - cpu_started, peak_ram_mib=None, resource_stop=stopped,
        )
        state, coverage_limitations = coverage_state(coverage, len(observations))
        planned = len(base)
        coarse_planned = len(progressive_schedule(duration, fps, self.base_fps, self.maximum_frames, allowed_intervals, levels=(8,))) if self.progressive else planned
        coverage_note = (f"{frames_base} of {planned} planned frames were read ({effective_fps:.2f} frames per second of video); the video was read from start to end at that rate"
                         if self.progressive and frames_base >= coarse_planned else
                         f"only {frames_base} of {planned} planned frames were read before the time budget ended; the coarse pass over the whole video was not finished, so part of the video was not looked at"
                         if self.progressive else f"{frames_base} of {planned} planned frames were read")
        if stopped or (self.progressive and (effective_fps < self.sparse_fps_below or frames_base < planned)):
            coverage_limitations = (*coverage_limitations, f"Plate reading of this video is sparse: {coverage_note}. A plate seen for less than a second or two may not have been read; zero readings do not show that a plate was absent")
        packets: list[Any] = []
        engine_note = {"processor": ADAPTER_ID, "processor_revision": ADAPTER_VERSION, "vehicle_context": "not_used_plate_only", "frame_hash_method": "sha256 of the decoded BGR pixel bytes",
                       "engines": [getattr(p, "processor_id", "") for p in self.processors],
                       "sampling": {"progressive": self.progressive, "frames_planned": planned, "frames_read": frames_base, "effective_frames_per_second": round(effective_fps, 3),
                                    "support_required": policy_used.minimum_selected_support, "note": coverage_note}}
        for item in observations:
            packet = item.to_packet(evidence_id=str(context["evidence_id"]), version_id=str(context["version_id"]))
            engine = self._engine_of.get(item.observation_id, self.image_processor)
            packet["payload"].update({**engine_note, "engine_processor": getattr(engine, "processor_id", ""), "engine_detector": getattr(engine, "detector_id", ""), "engine_ocr": getattr(engine, "ocr_id", "")})
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
                "selected_policy": asdict(policy_used), "execution_state": state, "vehicle_context": "not_used_plate_only", "interpolated_observations": False, "persistent_tracking": False,
                "frames_base": frames_base, "frames_refined": frames_refined, "coverage_note": coverage_note, "effective_frames_per_second": round(effective_fps, 3), "support_required": policy_used.minimum_selected_support, "refinement": self.refinement, "refinement_mode": self.refinement_mode, "sharpness_weight": self.sharpness_weight,
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

    def __init__(self, engine: Any, *, tile_side: int | None = 1280, wide_px: int = 2200, tile_every: int = 1, min_aspect: float = 1.5, minimum_width_px: int = 12, merge_iou: float = 0.3,
                 tile_every_hd: int | None = None, uhd_px: int = 2200) -> None:
        self.engine = engine
        self.detector = engine.detector
        self.ocr = engine.ocr
        self.tile_side = tile_side
        self.wide_px = wide_px
        self.tile_every = max(1, tile_every)
        # frames below `uhd_px` (1080p and 720p footage) take tiles on every `tile_every_hd`-th call (default: as often as 4K frames do); a tile there is as large as the frame is high, so it is a
        # native-resolution look at half of the frame, which makes the plates of a 1080p frame about 1.8 times larger for the detector than the whole frame does
        self.tile_every_hd = self.tile_every if tile_every_hd is None else max(1, tile_every_hd)
        self.uhd_px = uhd_px
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
        every = self.tile_every if max(width, height) >= self.uhd_px else self.tile_every_hd
        if self.tile_side and max(width, height) >= self.wide_px and self.calls % every == 0:
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
