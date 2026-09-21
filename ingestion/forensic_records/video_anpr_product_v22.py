# SPDX-License-Identifier: MIT
"""Two explicit raw-color input policies; no live activation or policy search.

The V2 detector/crop loop is preserved here so its frozen source remains intact.
Only the OCR input representation differs; the scheduler and aggregation are
inherited. The same processor is used by evaluation and any future integration.
"""
from __future__ import annotations

import hashlib
from dataclasses import asdict, dataclass, replace

from ingestion.forensic_records.video_anpr_parity_adapter import (
    ParityAdapterError, PlateFrameProcessor, PlateObservation, _clip_bounds,
    _contains, _observation_sort_key, crop_quality, normalize_plate_neutral,
    observation_id,
)
from ingestion.forensic_records.video_anpr_product_v2 import VideoANPRProductV2Processor

ADAPTER_ID = "NX-MMR-VIDEO-ANPR-PRODUCT-BASELINE-V2.2"
ADAPTER_VERSION = "2.2.0-development"
INPUT_MODES = ("RAW_BGR_PARITY", "RAW_RGB")


def raw_color_input(crop, mode):
    import numpy as np

    if mode not in INPUT_MODES:
        raise ParityAdapterError("Only RAW_BGR_PARITY and RAW_RGB input modes are supported")
    if not isinstance(crop, np.ndarray) or crop.dtype != np.uint8:
        raise ParityAdapterError("Raw-color OCR requires a uint8 ndarray")
    if crop.ndim != 3 or crop.shape[2] != 3 or min(crop.shape[:2]) <= 0:
        raise ParityAdapterError("Raw-color OCR requires nonempty HxWx3 input")
    # OpenCV crops are BGR. A layout copy is not spatial/intensity processing.
    return np.ascontiguousarray(crop if mode == "RAW_BGR_PARITY" else crop[:, :, ::-1])


def color_contract(mode):
    if mode not in INPUT_MODES:
        raise ParityAdapterError("Unsupported raw-color input mode")
    parity = mode == "RAW_BGR_PARITY"
    return {"ModelDeclaredColorMode": "RGB",
            "SuppliedArrayColorConvention": "BGR" if parity else "RGB",
            "HistoricalParityMode": parity,
            "OCRInputContract": "DECLARED_COLOR_DEVIATION" if parity else "PASS"}


@dataclass(frozen=True)
class RawColorObservation(PlateObservation):
    ocr_input_mode: str = "RAW_RGB"

    def to_packet(self, **context):
        packet = super().to_packet(**context)
        chain = ["original_OpenCV_BGR_crop"]
        if self.ocr_input_mode == "RAW_RGB":
            chain.append("BGR_to_RGB_channel_permutation")
        chain.append("FastPlateOCR_existing_resize_and_inference")
        packet["payload"].update({"ocr_input_mode": self.ocr_input_mode,
                                   "transform_chain": chain, **color_contract(self.ocr_input_mode)})
        return packet


class RawColorPlateFrameProcessor(PlateFrameProcessor):
    def __init__(self, *args, ocr_input_mode, **kwargs):
        color_contract(ocr_input_mode)
        super().__init__(*args, **kwargs)
        self.ocr_input_mode = ocr_input_mode
        self.last_frame_trace = None
        self.last_input_contract = None

    def process_frame(self, frame, *, source_sha256, source_file, frame_number,
                      timestamp_seconds, source_frame_sha256):
        if frame is None or len(getattr(frame, "shape", ())) < 2:
            raise ParityAdapterError("decoded frame has invalid dimensions")
        frame_height, frame_width = int(frame.shape[0]), int(frame.shape[1])
        vehicles = tuple(self.vehicle_detector.detect(frame)) if self.vehicle_detector else ()
        detected = tuple(self.plate_detector.detect(frame))[:self.maximum_detections_per_frame]
        trace = {"plate_detections": [(asdict(b), c, k) for b, c, k in detected],
                 "vehicle_detections": [(asdict(b), c, k) for b, c, k in vehicles],
                 "containment": [], "source_crops": []}
        output, ocr_crops = [], 0
        for bounds, detector_confidence, _class_id in detected:
            if detector_confidence < self.detection_threshold:
                continue
            clipped = _clip_bounds(bounds, frame_width, frame_height)
            if clipped is None:
                continue
            vehicle_match = None
            if self.vehicle_detector is not None:
                vehicle_match = any(_contains(vehicle_bounds, clipped) for vehicle_bounds, _, _ in vehicles)
                trace["containment"].append((asdict(clipped), vehicle_match))
                if not vehicle_match:
                    continue
            x1, y1 = round(clipped.x), round(clipped.y)
            x2, y2 = round(clipped.x2), round(clipped.y2)
            crop = frame[y1:y2, x1:x2]
            if getattr(crop, "size", 0) == 0:
                continue
            prepared = raw_color_input(crop, self.ocr_input_mode)
            self.last_input_contract = {**color_contract(self.ocr_input_mode),
                "mode": self.ocr_input_mode, "shape": list(prepared.shape),
                "dtype": str(prepared.dtype), "contiguous": bool(prepared.flags.c_contiguous),
                "source_bgr_sha256": hashlib.sha256(crop.tobytes()).hexdigest(),
                "supplied_array_sha256": hashlib.sha256(prepared.tobytes()).hexdigest()}
            ocr_crops += 1
            encoded, crop_bytes = self.cv2.imencode(".png", crop)
            crop_sha256 = hashlib.sha256(bytes(crop_bytes)).hexdigest() if encoded else ""
            trace["source_crops"].append({"bounds": asdict(clipped), "shape": list(crop.shape),
                "bgr_sha256": self.last_input_contract["source_bgr_sha256"], "crop_sha256": crop_sha256})
            for raw, ocr_confidence in self.ocr.read(prepared)[:self.maximum_ocr_results_per_crop]:
                if ocr_confidence is not None and ocr_confidence < self.ocr_threshold:
                    continue
                normalized = normalize_plate_neutral(raw)
                if not normalized:
                    continue
                output.append(RawColorObservation(
                    observation_id=observation_id(source_sha256, frame_number, clipped, normalized),
                    source_sha256=source_sha256, source_file=source_file,
                    frame_number=frame_number, timestamp_seconds=timestamp_seconds,
                    source_frame_sha256=source_frame_sha256, bounds=clipped,
                    raw_ocr=raw, normalized_ocr=normalized, detector_confidence=detector_confidence,
                    ocr_confidence=ocr_confidence, crop_sha256=crop_sha256,
                    detector_model_sha256=self.plate_detector.model_sha256,
                    ocr_model_sha256=self.ocr.model_sha256, vehicle_context_matched=vehicle_match,
                    crop_quality=crop_quality(clipped, frame_width, frame_height),
                    ocr_input_mode=self.ocr_input_mode))
        self.last_frame_trace = trace
        return tuple(sorted(output, key=_observation_sort_key)), ocr_crops, len(detected)


class VideoANPRProductV22Processor(VideoANPRProductV2Processor):
    processor_id = ADAPTER_ID
    processor_revision = ADAPTER_VERSION

    @classmethod
    def from_environment(cls, *, ocr_input_mode):
        color_contract(ocr_input_mode)
        base = VideoANPRProductV2Processor.from_environment()
        old = base.frame_processor
        frame = RawColorPlateFrameProcessor(
            old.plate_detector, old.ocr, old.cv2, ocr_input_mode=ocr_input_mode,
            vehicle_detector=old.vehicle_detector, detection_threshold=old.detection_threshold,
            ocr_threshold=old.ocr_threshold, binary_inverse_threshold=old.binary_inverse_threshold,
            maximum_detections_per_frame=old.maximum_detections_per_frame,
            maximum_ocr_results_per_crop=old.maximum_ocr_results_per_crop)
        return cls(frame, base.cv2, maximum_frames=base.maximum_frames,
                   maximum_duration_seconds=base.maximum_duration_seconds)

    def process_video(self, path, **context):
        result = super().process_video(path, **context)
        mode = self.frame_processor.ocr_input_mode
        packets = tuple(replace(item, payload={**item.payload,
            "processor": ADAPTER_ID, "processor_revision": ADAPTER_VERSION,
            "ocr_input_mode": mode, **color_contract(mode)}) for item in result.observations)
        return replace(result, processor_id=ADAPTER_ID, processor_revision=ADAPTER_VERSION,
                       observations=packets, metadata={**result.metadata, "ocr_input_mode": mode})
