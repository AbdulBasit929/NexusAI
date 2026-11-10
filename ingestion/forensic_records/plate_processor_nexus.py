"""Image plate processor of the plates programme (docs/data/plates-m5-preregistration-20261110.md): a published RF-DETR plate detector and the fast-plate-ocr `cct-s-v2-global` reader on onnxruntime.

Selected with FORENSIC_ANPR_ENGINE=nexus (default `fastalpr`, the processor that runs today, which is ship-blocking on licence). It keeps the whole contract of FastALPRImageProcessor (observations,
locators, crop provenance, review state, readiness): only the engine behind `predict(frame)` and the identities change, and each observation additionally carries the plate-format registry's verdict
(`plate_format`, `plate_format_flags`: a read that fits no registered format is flagged `format_unknown` and never edited) and a provenance note. PROVISIONAL: the detector's training data is not stated
on its card and the reader's weights have no separate licence statement; both stay flagged until the ledger rows are closed. Measured once on the owner-labelled real sealed set (22 images): 22 plates
found, 16 exact (the incumbent 20 and 13), character error 7.9% (20.1%); the pre-registered gate of 17 exact was missed by one image (Amendment 4).
"""
from __future__ import annotations

import dataclasses
import os
from dataclasses import dataclass
from pathlib import Path
from typing import Any

try:
    from ingestion.forensic_records import plate_formats
    from ingestion.forensic_records.media_pipeline import (
        FastALPRImageProcessor,
        MediaProcessResult,
        ModelUnavailableError,
        _onnx_providers,
        _required_model_path,
        _sha256,
    )
    from ingestion.forensic_records.plate_detector_onnx import OnnxPlateDetector
except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
    import plate_formats  # type: ignore[no-redef]
    from media_pipeline import (  # type: ignore[no-redef]
        FastALPRImageProcessor,
        MediaProcessResult,
        ModelUnavailableError,
        _onnx_providers,
        _required_model_path,
        _sha256,
    )
    from plate_detector_onnx import OnnxPlateDetector  # type: ignore[no-redef]

PROVENANCE_NOTE = "provisional: the detector's training data is not stated and the reader weights have no separate licence statement"


@dataclass(frozen=True)
class _Box:
    x1: int
    y1: int
    x2: int
    y2: int


@dataclass(frozen=True)
class _Detection:
    confidence: float
    bounding_box: _Box


@dataclass(frozen=True)
class _Ocr:
    text: str
    confidence: Any


@dataclass(frozen=True)
class _PlateResult:
    detection: _Detection
    ocr: _Ocr | None


class NexusPlateEngine:
    """The engine shape FastALPRImageProcessor expects: `predict(frame)` -> results with `.detection` and `.ocr`; `.detector.model` lets the processor report the active providers."""

    def __init__(self, detector: Any, ocr: Any) -> None:
        self.detector = detector
        self.ocr = ocr

    def predict(self, frame: Any) -> list[_PlateResult]:
        results: list[_PlateResult] = []
        for x1, y1, x2, y2, score in self.detector.detect(frame):
            if x2 - x1 < 4 or y2 - y1 < 4:
                continue
            read = self.ocr.predict(frame[y1:y2, x1:x2])
            ocr = _Ocr(read.text, read.confidence) if read is not None else None
            results.append(_PlateResult(_Detection(score, _Box(x1, y1, x2, y2)), ocr))
        return results


class NexusPlateImageProcessor(FastALPRImageProcessor):
    processor_id = "nexus-plates-onnx-cpu"
    processor_revision = "nexusai-m5-v1"
    detector_id = "rfdetr-alpr-851dc6b2db27"
    detector_version = "autolane/rfdetr-alpr@851dc6b2db27 (onnxruntime, unit scaling)"
    ocr_id = "cct-s-v2-global-model"

    @classmethod
    def from_environment(cls, *, minimum_confidence: float | None = None) -> "NexusPlateImageProcessor":
        detector_path = _required_model_path("FORENSIC_ANPR_NEXUS_DETECTOR_MODEL_PATH")
        ocr_path = _required_model_path("FORENSIC_ANPR_NEXUS_OCR_MODEL_PATH")
        ocr_config = _required_model_path("FORENSIC_ANPR_NEXUS_OCR_CONFIG_PATH")
        try:
            import cv2  # type: ignore[import-not-found]
            from fast_alpr.default_ocr import DefaultOCR  # type: ignore[import-not-found]
        except ModuleNotFoundError as exc:
            raise ModelUnavailableError(f"plate engine runtime dependency unavailable: {exc.name}") from exc
        providers = _onnx_providers()
        confidence = float(os.getenv("FORENSIC_ANPR_NEXUS_MIN_DETECTION_CONFIDENCE", "0.5")) if minimum_confidence is None else float(minimum_confidence)
        detector = OnnxPlateDetector(detector_path, cv2, providers=providers, normalise="unit", threshold=confidence)
        ocr = DefaultOCR(hub_ocr_model=None, device="cpu", providers=providers, model_path=ocr_path, config_path=ocr_config, force_download=False)
        return cls(NexusPlateEngine(detector, ocr), cv2, confidence, model_hashes={"detector": _sha256(detector_path), "ocr": _sha256(ocr_path), "ocr_config": _sha256(ocr_config)})

    def process_image(self, path: Path, **kwargs: Any) -> MediaProcessResult:
        result = super().process_image(path, **kwargs)
        registry_version = plate_formats.load_registry()["registry_version"]
        observations = []
        for observation in result.observations:
            payload = dict(observation.payload)
            matches = plate_formats.matching_formats(str(payload.get("normalized_plate_text") or ""))
            payload["plate_format"] = matches[0] if matches else None
            payload["plate_format_flags"] = [] if matches else ["format_unknown"]
            payload["plate_registry_version"] = registry_version
            payload["model_provenance"] = PROVENANCE_NOTE
            observations.append(dataclasses.replace(observation, payload=payload))
        limitations = tuple(result.limitations) + (f"plate models are {PROVENANCE_NOTE}",)
        return dataclasses.replace(result, observations=tuple(observations), limitations=limitations)
