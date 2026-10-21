# SPDX-License-Identifier: Apache-2.0
"""General image OCR role backed by the PP-OCRv5 ONNX engine (onnx_text_ocr.py): the same observation contract as the Paddle adapter (multilingual_ocr.py), without PaddlePaddle.

Selected with FORENSIC_OCR_BACKEND=onnx. Every model file is a pinned path with a pinned SHA-256; nothing is downloaded. Measured on real receipt photographs (docs/data/media-activation-results-20261021.md)
this engine reads 79 to 80% of the words where Tesseract reads 33%. Printed Urdu is better read by Tesseract's `urd` data (the ONNX Arabic recogniser is kept out unless it is configured), so a
deployment that needs Urdu uses the `auto` backend, which routes by script.
"""

from __future__ import annotations

import hashlib
import os
import re
import uuid
from pathlib import Path
from typing import Any

import cv2
import numpy as np

try:
    from ingestion.forensic_records import onnx_text_ocr as engine
    from ingestion.forensic_records.media_pipeline import (
        IMAGE_OCR_OBSERVATION_CONTRACT,
        READY,
        READY_WITH_WARNINGS,
        MediaObservation,
        MediaProcessingError,
        MediaProcessResult,
        ModelUnavailableError,
    )
    from ingestion.forensic_records.multilingual_ocr import _identifiers, _normalize_text, _script_and_direction, _select_candidate
except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
    import onnx_text_ocr as engine
    from media_pipeline import (
        IMAGE_OCR_OBSERVATION_CONTRACT,
        READY,
        READY_WITH_WARNINGS,
        MediaObservation,
        MediaProcessingError,
        MediaProcessResult,
        ModelUnavailableError,
    )
    from multilingual_ocr import _identifiers, _normalize_text, _script_and_direction, _select_candidate


_ARABIC_LETTER = re.compile("[" + "".join(f"{chr(a)}-{chr(b)}" for a, b in ((0x0600, 0x06FF), (0x0750, 0x077F), (0xFB50, 0xFDFF), (0xFE70, 0xFEFF))) + "]")  # Arabic, Arabic Supplement and the two presentation forms blocks
PROCESSOR_ID = "paddleocr-v5-onnx-cpu"
PROCESSOR_REVISION = "nexusai-onnx-ocr-v1"
DETECTOR_ID = "PP-OCRv5_mobile_det_onnx"
RECOGNIZER_IDS = {"latin": "latin_PP-OCRv5_mobile_rec_onnx", "arabic_multilingual": "arabic_PP-OCRv5_mobile_rec_onnx"}


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def _pinned(path_variable: str, hash_variable: str) -> tuple[Path, str]:
    value = os.getenv(path_variable, "").strip()
    if not value:
        raise ModelUnavailableError(f"{path_variable} is required; implicit model download is disabled")
    path = Path(value)
    if not path.is_file():
        raise ModelUnavailableError(f"configured model asset does not exist: {path_variable}")
    expected = os.getenv(hash_variable, "").strip().lower()
    actual = _sha256(path)
    if not re.fullmatch(r"[0-9a-f]{64}", expected) or actual != expected:
        raise ModelUnavailableError(f"{hash_variable} is absent or does not match the file")
    return path, actual


class OnnxPaddleOCRProcessor:
    processor_id = PROCESSOR_ID
    processor_revision = PROCESSOR_REVISION

    def __init__(self, detector: Any, recognizers: dict[str, Any], *, model_hashes: dict[str, str], maximum_regions: int = 256, cv2_module: Any = cv2, script_probe: Any | None = None) -> None:
        if "latin" not in recognizers:
            raise ValueError("the Latin recogniser is required")
        if not 1 <= maximum_regions <= 2048:
            raise ValueError("maximum OCR regions is invalid")
        self.detector = detector
        self.recognizers = recognizers
        self.model_hashes = dict(model_hashes)
        self.maximum_regions = maximum_regions
        self.cv2 = cv2_module
        self.script_probe = script_probe  # an Arabic-script recogniser used only to tell the script of a page; its text is never stored

    @classmethod
    def from_environment(cls) -> "OnnxPaddleOCRProcessor":
        providers = None
        detector_path, detector_hash = _pinned("FORENSIC_ONNX_OCR_DETECTOR_PATH", "FORENSIC_ONNX_OCR_DETECTOR_SHA256")
        latin_path, latin_hash = _pinned("FORENSIC_ONNX_OCR_LATIN_MODEL_PATH", "FORENSIC_ONNX_OCR_LATIN_SHA256")
        latin_config = Path(os.getenv("FORENSIC_ONNX_OCR_LATIN_CONFIG_PATH", "").strip() or str(latin_path.with_name("inference.yml")))
        recognizers = {"latin": engine.PaddleOnnxRecognizer(latin_path, latin_config, arabic_script=False, providers=providers)}
        hashes = {"detector": detector_hash, "latin": latin_hash}
        if os.getenv("FORENSIC_ONNX_OCR_ARABIC_MODEL_PATH", "").strip():
            arabic_path, arabic_hash = _pinned("FORENSIC_ONNX_OCR_ARABIC_MODEL_PATH", "FORENSIC_ONNX_OCR_ARABIC_SHA256")
            arabic_config = Path(os.getenv("FORENSIC_ONNX_OCR_ARABIC_CONFIG_PATH", "").strip() or str(arabic_path.with_name("inference.yml")))
            recognizers["arabic_multilingual"] = engine.PaddleOnnxRecognizer(arabic_path, arabic_config, arabic_script=True, providers=providers)
            hashes["arabic_multilingual"] = arabic_hash
        probe = None
        if os.getenv("FORENSIC_ONNX_OCR_SCRIPT_PROBE_MODEL_PATH", "").strip():
            probe_path, probe_hash = _pinned("FORENSIC_ONNX_OCR_SCRIPT_PROBE_MODEL_PATH", "FORENSIC_ONNX_OCR_SCRIPT_PROBE_SHA256")
            probe_config = Path(os.getenv("FORENSIC_ONNX_OCR_SCRIPT_PROBE_CONFIG_PATH", "").strip() or str(probe_path.with_name("inference.yml")))
            probe = engine.PaddleOnnxRecognizer(probe_path, probe_config, arabic_script=True, providers=providers)
            hashes["script_probe"] = probe_hash
        return cls(engine.PaddleOnnxDetector(detector_path, providers=providers), recognizers, model_hashes=hashes, script_probe=probe)

    def arabic_script_share(self, path: Path, *, lines: int = 6) -> float | None:
        """Share of Arabic-script letters in what the Arabic recogniser writes for the longest few text lines of the page, or None without a probe or without text. Urdu pages give a high share
        (the model writes Arabic-script letters, right or wrong), Latin pages a low one; Tesseract's own script pass was not reliable enough for routing (docs/data/media-activation-results)."""
        if self.script_probe is None:
            return None
        frame = self.cv2.imread(str(path))
        if frame is None or len(frame.shape) < 2:
            return None
        boxes = sorted(self.detector.detect(frame), key=lambda quad: -float(np.linalg.norm(quad[0] - quad[1])))[:lines]
        texts = [self.script_probe.recognize(engine._crop_quad(frame, quad))[0] for quad in boxes]
        letters = [ch for ch in " ".join(texts) if ch.isalpha()]
        if not letters:
            return None
        arabic = len(_ARABIC_LETTER.findall("".join(letters)))
        return arabic / len(letters)

    def process_image(self, path: Path, **context: Any) -> MediaProcessResult:
        frame = self.cv2.imread(str(path))
        if frame is None or len(frame.shape) < 2:
            raise MediaProcessingError("ONNX OCR could not decode the image")
        height, width = int(frame.shape[0]), int(frame.shape[1])
        boxes = self.detector.detect(frame)[: self.maximum_regions]
        observations: list[MediaObservation] = []
        reading_order = 0
        for index in engine.PaddleOnnxOCR._reading_order(boxes):
            quad = boxes[index]
            x1 = int(max(0, min(width, np.floor(quad[:, 0].min()))))
            y1 = int(max(0, min(height, np.floor(quad[:, 1].min()))))
            x2 = int(max(0, min(width, np.ceil(quad[:, 0].max()))))
            y2 = int(max(0, min(height, np.ceil(quad[:, 1].max()))))
            if x2 <= x1 or y2 <= y1:
                continue
            crop = engine._crop_quad(frame, quad)
            candidates: list[dict[str, Any]] = []
            for role, recognizer in self.recognizers.items():
                text, confidence = recognizer.recognize(crop)
                candidates.append({"recognizer_role": role, "model_id": RECOGNIZER_IDS[role], "raw_text": text, "confidence": float(confidence)})
            if not any(candidate["raw_text"].strip() for candidate in candidates):
                continue
            reading_order += 1
            selected, reason = _select_candidate(candidates)
            raw_text = selected["raw_text"]
            script, direction = _script_and_direction(raw_text)
            ok, encoded = self.cv2.imencode(".png", frame[y1:y2, x1:x2])
            crop_hash = hashlib.sha256(bytes(encoded)).hexdigest() if ok else ""
            polygon = [[float(x), float(y)] for x, y in quad]
            bbox = {"x": x1, "y": y1, "width": x2 - x1, "height": y2 - y1}
            locator = dict(context.get("locator_prefix") or {})
            locator.update({"source_file": context["source_file"], "coordinate_space": "original_image_pixels", "polygon": polygon, "bbox": bbox, "reading_order": reading_order})
            identity = str(uuid.uuid5(uuid.NAMESPACE_URL, f"{context['source_sha256']}:onnx-ocr:{reading_order}:{bbox}:{crop_hash}"))
            observations.append(MediaObservation(
                observation_id=identity,
                contract_version=IMAGE_OCR_OBSERVATION_CONTRACT,
                observation_type="general_image_ocr_observation",
                confidence=selected["confidence"],
                citation_locator=locator,
                payload={
                    "raw_text": raw_text,
                    "normalized_text": _normalize_text(raw_text),
                    "script_family": script,
                    "display_direction": direction,
                    "reading_order": reading_order,
                    "reading_order_semantics": "top-to-bottom regions; recognizer Unicode logical order preserved",
                    "identifiers_preserved": _identifiers(raw_text),
                    "recognition_confidence": selected["confidence"],
                    "selected_recognizer": selected["recognizer_role"],
                    "candidate_selection_policy": "mixed_script_superset_within_0.05_then_confidence",
                    "candidate_selection_reason": reason,
                    "recognizer_candidates": candidates,
                    "bbox": bbox,
                    "polygon": polygon,
                    "crop_sha256": crop_hash,
                    "crop_storage_state": "reconstructable_from_retained_source",
                    "parent_evidence_id": context["evidence_id"],
                    "parent_version_id": context["version_id"],
                    "processor": self.processor_id,
                    "processor_revision": self.processor_revision,
                    "detector_model": DETECTOR_ID,
                    "recognizer_models": {role: RECOGNIZER_IDS[role] for role in self.recognizers},
                    "model_sha256": self.model_hashes,
                    "manual_review_required": True,
                },
                warnings=("Recognized text is a model observation and requires analyst review",),
            ))
        limitations = [
            "printed text baseline measured on public receipt photographs and generated pages; handwriting is not certified",
            "region and reading order require analyst review for complex layouts",
        ]
        if "arabic_multilingual" not in self.recognizers:
            limitations.append("Arabic-script text is not read by this backend; use the auto backend for Urdu")
        if not observations:
            limitations.append("no non-empty OCR region was returned")
        return MediaProcessResult(
            modality="image",
            readiness=READY if observations else READY_WITH_WARNINGS,
            processor_id=self.processor_id,
            processor_revision=self.processor_revision,
            observations=tuple(observations),
            limitations=tuple(limitations),
            metadata={
                "result_state": "COMPLETE_RESULTS" if observations else "COMPLETE_ZERO_RESULTS",
                "ocr_region_count": len(observations),
                "detected_region_count": len(boxes),
                "model_sha256": self.model_hashes,
                "onnx_threads": int(os.getenv("FORENSIC_ONNX_OCR_THREADS", "4")),
            },
        )
