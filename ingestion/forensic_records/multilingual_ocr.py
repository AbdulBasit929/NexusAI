"""Hash-pinned PaddleOCR adapter for printed English, Urdu, and mixed text."""

from __future__ import annotations

import hashlib
import json
import os
import re
import tempfile
import unicodedata
import uuid
from pathlib import Path
from typing import Any

try:
    from ingestion.forensic_records.media_pipeline import (
        IMAGE_OCR_OBSERVATION_CONTRACT,
        READY,
        READY_WITH_WARNINGS,
        MediaObservation,
        MediaProcessingError,
        MediaProcessResult,
        ModelUnavailableError,
    )
except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
    from media_pipeline import (
        IMAGE_OCR_OBSERVATION_CONTRACT,
        READY,
        READY_WITH_WARNINGS,
        MediaObservation,
        MediaProcessingError,
        MediaProcessResult,
        ModelUnavailableError,
    )


PROCESSOR_ID = "paddleocr-v5-multilingual-cpu"
PROCESSOR_REVISION = "nxmmr-anpr-ocr-vertical-v1-inputfix1-mixedfix1"
DETECTOR_ID = "PP-OCRv5_mobile_det"
MIXED_SCRIPT_COMPLETENESS_MAX_CONFIDENCE_GAP = 0.05
RECOGNIZER_IDS = {
    "english": "en_PP-OCRv5_mobile_rec",
    "arabic_multilingual": "arabic_PP-OCRv5_mobile_rec",
}


def directory_digest(path: Path) -> str:
    if not path.is_dir():
        raise ModelUnavailableError(f"configured PaddleOCR model directory is unavailable: {path}")
    digest = hashlib.sha256()
    files = sorted(item for item in path.rglob("*") if item.is_file())
    if not files:
        raise ModelUnavailableError(f"configured PaddleOCR model directory is empty: {path}")
    for item in files:
        relative = item.relative_to(path).as_posix().encode("utf-8")
        digest.update(len(relative).to_bytes(4, "big"))
        digest.update(relative)
        digest.update(bytes.fromhex(_sha256(item)))
    return digest.hexdigest()


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def _payload(result: Any) -> dict[str, Any]:
    if isinstance(result, dict):
        return result.get("res", result)
    with tempfile.TemporaryDirectory() as directory:
        destination = Path(directory) / "result.json"
        result.save_to_json(save_path=str(destination))
        value = json.loads(destination.read_text(encoding="utf-8"))
    return value.get("res", value)


def _script_and_direction(text: str) -> tuple[str, str]:
    arabic = bool(re.search(r"[\u0600-\u06ff\u0750-\u077f\u08a0-\u08ff]", text))
    latin = bool(re.search(r"[A-Za-z]", text))
    if arabic and latin:
        return "mixed_arabic_latin", "mixed"
    if arabic:
        return "arabic_derived_urdu_candidate", "rtl"
    if latin:
        return "latin_candidate", "ltr"
    return "undetermined", "ltr"


def _normalize_text(text: str) -> str:
    return " ".join(unicodedata.normalize("NFKC", text).split())


def _select_candidate(candidates: list[dict[str, Any]]) -> tuple[dict[str, Any], str]:
    nonempty = [candidate for candidate in candidates if candidate["raw_text"].strip()]
    if not nonempty:
        raise ValueError("candidate selection requires non-empty recognized text")
    confidence_winner = max(
        nonempty,
        key=lambda item: (float(item["confidence"] or 0), item["recognizer_role"]),
    )
    winner_text = _normalize_text(confidence_winner["raw_text"]).casefold()
    winner_confidence = float(confidence_winner["confidence"] or 0)
    mixed_supersets = []
    for candidate in nonempty:
        candidate_text = _normalize_text(candidate["raw_text"]).casefold()
        script, _direction = _script_and_direction(candidate_text)
        confidence = float(candidate["confidence"] or 0)
        if (
            script == "mixed_arabic_latin"
            and candidate_text != winner_text
            and winner_text in candidate_text
            and winner_confidence - confidence <= MIXED_SCRIPT_COMPLETENESS_MAX_CONFIDENCE_GAP
        ):
            mixed_supersets.append(candidate)
    if mixed_supersets:
        return max(
            mixed_supersets,
            key=lambda item: (float(item["confidence"] or 0), item["recognizer_role"]),
        ), "mixed_script_superset_within_confidence_tolerance"
    return confidence_winner, "highest_recognition_confidence"


def _identifiers(text: str) -> list[str]:
    # Preserve exact source order and characters; never reverse or substitute.
    output: list[str] = []
    for token in text.split():
        candidate = token.strip(".,;!?()[]{}،؛؟")
        if len(candidate) < 3:
            continue
        if any(character.isdigit() for character in candidate):
            output.append(candidate)
    return output


class PaddleMultilingualOCRProcessor:
    processor_id = PROCESSOR_ID
    processor_revision = PROCESSOR_REVISION

    def __init__(
        self,
        detector: Any,
        recognizers: dict[str, Any],
        cv2_module: Any,
        *,
        model_hashes: dict[str, str],
        maximum_regions: int = 256,
    ) -> None:
        if set(recognizers) != set(RECOGNIZER_IDS):
            raise ValueError("English and Arabic-multilingual recognizers are required")
        if not 1 <= maximum_regions <= 2048:
            raise ValueError("maximum OCR regions is invalid")
        self.detector = detector
        self.recognizers = recognizers
        self.cv2 = cv2_module
        self.model_hashes = dict(model_hashes)
        self.maximum_regions = maximum_regions

    @classmethod
    def from_environment(cls) -> "PaddleMultilingualOCRProcessor":
        detector_dir = Path(os.getenv("FORENSIC_PADDLE_DETECTOR_MODEL_DIR", ""))
        english_dir = Path(os.getenv("FORENSIC_PADDLE_ENGLISH_MODEL_DIR", ""))
        arabic_dir = Path(os.getenv("FORENSIC_PADDLE_ARABIC_MODEL_DIR", ""))
        paths = {
            "detector": detector_dir,
            "english": english_dir,
            "arabic_multilingual": arabic_dir,
        }
        hashes = {role: directory_digest(path) for role, path in paths.items()}
        for role, actual in hashes.items():
            expected = os.getenv(f"FORENSIC_PADDLE_{role.upper()}_TREE_SHA256", "").lower()
            if not re.fullmatch(r"[0-9a-f]{64}", expected) or actual != expected:
                raise ModelUnavailableError(f"PaddleOCR {role} directory digest is absent or does not match")
        try:
            import cv2  # type: ignore[import-not-found]
            from paddleocr import TextDetection, TextRecognition  # type: ignore[import-not-found]
        except ModuleNotFoundError as exc:
            raise ModelUnavailableError(f"PaddleOCR runtime dependency unavailable: {exc.name}") from exc
        common = {"device": "cpu", "cpu_threads": 4, "enable_mkldnn": False}
        detector = TextDetection(model_name=DETECTOR_ID, model_dir=str(detector_dir), **common)
        recognizers = {
            "english": TextRecognition(
                model_name=RECOGNIZER_IDS["english"], model_dir=str(english_dir), **common
            ),
            "arabic_multilingual": TextRecognition(
                model_name=RECOGNIZER_IDS["arabic_multilingual"], model_dir=str(arabic_dir), **common
            ),
        }
        return cls(detector, recognizers, cv2, model_hashes=hashes)

    def process_image(self, path: Path, **context: Any) -> MediaProcessResult:
        frame = self.cv2.imread(str(path))
        if frame is None or len(frame.shape) < 2:
            raise MediaProcessingError("PaddleOCR could not decode the image")
        height, width = int(frame.shape[0]), int(frame.shape[1])
        # Content-addressed source paths have no suffix. Paddle's path reader
        # rejects those even though OpenCV has already decoded valid BGR pixels.
        detected = list(self.detector.predict(input=frame, batch_size=1))
        detector_payload = _payload(detected[0]) if detected else {}
        polygon_value = detector_payload.get("dt_polys")
        score_value = detector_payload.get("dt_scores")
        polygons = list(polygon_value)[: self.maximum_regions] if polygon_value is not None else []
        scores = list(score_value)[: self.maximum_regions] if score_value is not None else []
        regions: list[dict[str, Any]] = []
        for index, polygon in enumerate(polygons):
            points = [[float(point[0]), float(point[1])] for point in polygon]
            if not points:
                continue
            x1 = max(0, min(width, int(min(point[0] for point in points))))
            y1 = max(0, min(height, int(min(point[1] for point in points))))
            x2 = max(0, min(width, int(max(point[0] for point in points))))
            y2 = max(0, min(height, int(max(point[1] for point in points))))
            if x2 <= x1 or y2 <= y1:
                continue
            regions.append({
                "index": index,
                "polygon": points,
                "bbox": {"x": x1, "y": y1, "width": x2 - x1, "height": y2 - y1},
                "detection_confidence": float(scores[index]) if index < len(scores) else None,
                "crop": frame[y1:y2, x1:x2],
            })
        regions.sort(key=lambda item: (item["bbox"]["y"], item["bbox"]["x"], item["index"]))

        observations: list[MediaObservation] = []
        for reading_order, region in enumerate(regions, start=1):
            candidates: list[dict[str, Any]] = []
            for role, recognizer in self.recognizers.items():
                predicted = list(recognizer.predict(input=region["crop"], batch_size=1))
                value = _payload(predicted[0]) if predicted else {}
                raw = str(value.get("rec_text") or "")
                score = value.get("rec_score")
                candidates.append({
                    "recognizer_role": role,
                    "model_id": RECOGNIZER_IDS[role],
                    "raw_text": raw,
                    "confidence": float(score) if isinstance(score, (int, float)) else None,
                })
            if not any(candidate["raw_text"].strip() for candidate in candidates):
                continue
            selected, selection_reason = _select_candidate(candidates)
            raw_text = selected["raw_text"]
            script, direction = _script_and_direction(raw_text)
            crop_ok, crop_bytes = self.cv2.imencode(".png", region["crop"])
            crop_hash = hashlib.sha256(bytes(crop_bytes)).hexdigest() if crop_ok else ""
            locator = dict(context.get("locator_prefix") or {})
            locator.update({
                "source_file": context["source_file"],
                "coordinate_space": "original_image_pixels",
                "polygon": region["polygon"],
                "bbox": region["bbox"],
                "reading_order": reading_order,
            })
            identity = str(uuid.uuid5(
                uuid.NAMESPACE_URL,
                f"{context['source_sha256']}:paddle-ocr:{reading_order}:{region['bbox']}:{crop_hash}",
            ))
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
                    "detection_confidence": region["detection_confidence"],
                    "recognition_confidence": selected["confidence"],
                    "selected_recognizer": selected["recognizer_role"],
                    "candidate_selection_policy": "mixed_script_superset_within_0.05_then_confidence",
                    "candidate_selection_reason": selection_reason,
                    "mixed_script_candidate_preserved": selection_reason.startswith("mixed_script_superset"),
                    "recognizer_candidates": candidates,
                    "bbox": region["bbox"],
                    "polygon": region["polygon"],
                    "crop_sha256": crop_hash,
                    "crop_storage_state": "reconstructable_from_retained_source",
                    "parent_evidence_id": context["evidence_id"],
                    "parent_version_id": context["version_id"],
                    "processor": self.processor_id,
                    "processor_revision": self.processor_revision,
                    "detector_model": DETECTOR_ID,
                    "recognizer_models": RECOGNIZER_IDS,
                    "model_tree_sha256": self.model_hashes,
                    "manual_review_required": True,
                },
                warnings=("Recognized text is a model observation and requires analyst review",),
            ))
        limitations = [
            "printed English, Urdu, and mixed-script baseline; handwriting is not certified",
            "region and reading order require analyst review for complex layouts",
        ]
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
                "detected_region_count": len(regions),
                "model_tree_sha256": self.model_hashes,
                "cpu_threads": 4,
            },
        )
