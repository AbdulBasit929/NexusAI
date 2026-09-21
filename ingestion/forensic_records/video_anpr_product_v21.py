# SPDX-License-Identifier: MIT
"""V2.1 channel-contract correction; the failed V2 implementation stays intact.

This is a development-only processor, not enabled in the live factory. All
detectors, thresholding, scheduling and aggregation are inherited unchanged.
"""
from __future__ import annotations

from dataclasses import dataclass, fields, replace

from ingestion.forensic_records.video_anpr_parity_adapter import (
    ParityAdapterError, PlateFrameProcessor, PlateObservation,
)
from ingestion.forensic_records.video_anpr_product_v2 import VideoANPRProductV2Processor

ADAPTER_ID = "NX-MMR-VIDEO-ANPR-PRODUCT-BASELINE-V2.1"
ADAPTER_VERSION = "2.1.0-development"
TRANSFORM_ID = "GRAY_BINARY_INV_64_REPLICATED_RGB"
TRANSFORM_CHAIN = (
    "source_crop", "BGR_to_grayscale", "binary_inverse_threshold_64",
    "replicate_thresholded_pixels_to_RGB_interface", "FastPlateOCR",
)


def thresholded_rgb_input(crop):
    """Represent thresholded pixels as RGB without inventing color information."""
    import numpy as np

    if not isinstance(crop, np.ndarray) or crop.dtype != np.uint8:
        raise ParityAdapterError("Video FastPlate input must be a uint8 ndarray")
    if crop.ndim not in (2, 3) or any(size == 0 for size in crop.shape):
        raise ParityAdapterError("Video FastPlate input must have nonempty HxW dimensions")
    if crop.ndim == 3 and crop.shape[2] not in (1, 3):
        raise ParityAdapterError("Video FastPlate input requires 1 or 3 channels")
    plane = crop if crop.ndim == 2 else crop[:, :, 0]
    if crop.ndim == 3 and crop.shape[2] == 3:
        if not np.array_equal(crop[:, :, 1], plane) or not np.array_equal(crop[:, :, 2], plane):
            raise ParityAdapterError("Video FastPlate requires identical thresholded channels; no raw-color fallback")
    if not np.all((plane == 0) | (plane == 255)):
        raise ParityAdapterError("Video FastPlate requires binary thresholded pixels")
    result = np.ascontiguousarray(np.repeat(plane[:, :, None], 3, axis=2))
    # The pinned recognizer performs its existing 64x128 spatial resize. This
    # adapter only supplies its required uint8 channels-last RGB representation.
    if result.ndim != 3 or result.shape[2] != 3 or not result.flags.c_contiguous:
        raise ParityAdapterError("Video FastPlate RGB interface contract failed")
    return result


class RGBCompatibleFastPlateOCR:
    """One authoritative boundary shared by evaluation and the versioned processor."""

    def __init__(self, pinned_ocr):
        self.pinned_ocr = pinned_ocr
        self.model_sha256 = pinned_ocr.model_sha256
        self.last_input_contract = None

    def read(self, crop):
        adapted = thresholded_rgb_input(crop)
        self.last_input_contract = {
            "shape": list(adapted.shape), "dtype": str(adapted.dtype),
            "contiguous": bool(adapted.flags.c_contiguous),
            "transform": TRANSFORM_ID, "color_information_added": False,
        }
        return self.pinned_ocr.read(adapted)


@dataclass(frozen=True)
class V21PlateObservation(PlateObservation):
    preprocessing: str = TRANSFORM_ID
    transform_chain: tuple[str, ...] = TRANSFORM_CHAIN

    def to_packet(self, **context):
        packet = super().to_packet(**context)
        packet["payload"].update({
            "preprocessing": self.preprocessing, "transform_chain": list(self.transform_chain),
            "rgb_representation_adds_color_information": False,
        })
        return packet


class V21PlateFrameProcessor(PlateFrameProcessor):
    def process_frame(self, frame, **context):
        observations, crops, detections = super().process_frame(frame, **context)
        return tuple(V21PlateObservation(**{
            field.name: getattr(item, field.name) for field in fields(PlateObservation)
        }) for item in observations), crops, detections


class VideoANPRProductV21Processor(VideoANPRProductV2Processor):
    processor_id = ADAPTER_ID
    processor_revision = ADAPTER_VERSION

    @classmethod
    def from_environment(cls):
        baseline = VideoANPRProductV2Processor.from_environment()
        frame = baseline.frame_processor
        assert frame.binary_inverse_threshold == 64
        corrected = V21PlateFrameProcessor(
            frame.plate_detector, RGBCompatibleFastPlateOCR(frame.ocr), frame.cv2,
            vehicle_detector=frame.vehicle_detector,
            detection_threshold=frame.detection_threshold, ocr_threshold=frame.ocr_threshold,
            binary_inverse_threshold=frame.binary_inverse_threshold,
            maximum_detections_per_frame=frame.maximum_detections_per_frame,
            maximum_ocr_results_per_crop=frame.maximum_ocr_results_per_crop,
        )
        return cls(corrected, baseline.cv2, maximum_frames=baseline.maximum_frames,
                   maximum_duration_seconds=baseline.maximum_duration_seconds)

    def process_video(self, path, **context):
        result = super().process_video(path, **context)
        packets = tuple(replace(item, payload={**item.payload,
            "processor": ADAPTER_ID, "processor_revision": ADAPTER_VERSION,
        }) for item in result.observations)
        return replace(result, processor_id=ADAPTER_ID,
                       processor_revision=ADAPTER_VERSION, observations=packets)
