"""Bounded CPU-first media processors for forensic evidence.

The module deliberately keeps model output separate from source truth.  It does
not download models: every model-backed processor requires explicit local paths
or an injected engine.  The worker persists these results through the existing
evidence control-plane tables.
"""

from __future__ import annotations

import base64
import csv
import hashlib
import io
import json
import math
import os
import re
import shutil
import statistics
import subprocess
import tempfile
import uuid
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Protocol

try:
    from ingestion.forensic_records.image_intelligence import (
        IMAGE_FINGERPRINT_CONTRACT,
        PERCEPTUAL_HASH_ALGORITHM,
        image_dhash,
    )
except ModuleNotFoundError:  # pragma: no cover - supports direct worker image execution
    from image_intelligence import IMAGE_FINGERPRINT_CONTRACT, PERCEPTUAL_HASH_ALGORITHM, image_dhash

try:
    from ingestion.forensic_records.roman_urdu import (
        ROMAN_URDU_SEGMENT_CONTRACT,
        contains_devanagari_script,
        contains_urdu_script,
        roman_urdu_payload,
    )
except ModuleNotFoundError:  # pragma: no cover - supports direct worker image execution
    from roman_urdu import (
        ROMAN_URDU_SEGMENT_CONTRACT,
        contains_devanagari_script,
        contains_urdu_script,
        roman_urdu_payload,
    )


MEDIA_RESULT_CONTRACT = "forensics.media-processing-result/v1"
IMAGE_OBSERVATION_CONTRACT = "forensics.image-observation/v1"
ANPR_OBSERVATION_CONTRACT = "forensics.anpr-observation/v1"
AUDIO_OBSERVATION_CONTRACT = "forensics.audio-observation/v1"
AUDIO_TRANSCRIPT_CONTRACT = "forensics.audio-transcript/v1"
AUDIO_TIMESTAMP_SEGMENT_CONTRACT = "forensics.audio-timestamp-segment/v1"
VIDEO_OBSERVATION_CONTRACT = "forensics.video-observation/v1"
VIDEO_ANPR_GROUP_CONTRACT = "forensics.video-anpr-plate-group/v1"
PLATE_REGION_CONTRACT = "forensics.plate-region-candidate/v1"
PLATE_CROP_CONTRACT = "forensics.plate-region-crop/v1"
FACE_OBSERVATION_CONTRACT = "forensics.face-observation/v1"
FACE_CROP_CONTRACT = "forensics.face-crop/v1"
IMAGE_EMBEDDING_CONTRACT = "forensics.image-embedding-observation/v1"
IMAGE_OCR_OBSERVATION_CONTRACT = "forensics.image-ocr-observation/v1"

VIDEO_FRAME_ZERO_LIMITATIONS = {
    "no plate observation passed the configured detection threshold": (
        "anpr_ocr_observation",
        "no plate observation passed the configured detection threshold in sampled frames",
    ),
    "no faces were detected": (
        "face_detection_observation",
        "no faces were detected in sampled frames",
    ),
    "no non-empty OCR region was returned": (
        "general_image_ocr_observation",
        "no non-empty OCR region was returned from sampled frames",
    ),
}

READY = "READY"
READY_WITH_WARNINGS = "READY_WITH_WARNINGS"
FAILED = "FAILED"
UNAVAILABLE = "UNAVAILABLE"


class MediaProcessingError(RuntimeError):
    """A bounded processor could not safely process the supplied evidence."""


class ModelUnavailableError(MediaProcessingError):
    """A configured model role is unavailable without an approved local asset."""


@dataclass(frozen=True)
class MediaObservation:
    observation_id: str
    contract_version: str
    observation_type: str
    confidence: float | None
    citation_locator: dict[str, Any]
    payload: dict[str, Any]
    warnings: tuple[str, ...] = ()


@dataclass(frozen=True)
class MediaProcessResult:
    modality: str
    readiness: str
    processor_id: str
    processor_revision: str
    observations: tuple[MediaObservation, ...] = ()
    limitations: tuple[str, ...] = ()
    metadata: dict[str, Any] = field(default_factory=dict)
    contract_version: str = MEDIA_RESULT_CONTRACT


class ImageProcessor(Protocol):
    def process_image(
        self,
        path: Path,
        *,
        evidence_id: str,
        version_id: str,
        source_sha256: str,
        source_file: str,
        locator_prefix: dict[str, Any] | None = None,
    ) -> MediaProcessResult: ...


class AudioTranscriber(Protocol):
    def transcribe(
        self,
        path: Path,
        *,
        evidence_id: str,
        version_id: str,
        source_file: str,
        language: str | None = None,
    ) -> tuple[MediaObservation, ...]: ...


def normalize_plate(raw_text: str) -> str:
    """Apply formatting-only normalization while preserving the raw value."""

    return re.sub(r"[^A-Z0-9]", "", (raw_text or "").upper())


def mean_confidence(value: Any) -> float | None:
    if value is None:
        return None
    if isinstance(value, (str, bytes)):
        try:
            result = float(value)
        except ValueError:
            return None
        return result if 0 <= result <= 1 else None
    try:
        values = [float(item) for item in value]
    except TypeError:
        try:
            result = float(value)
        except (TypeError, ValueError):
            return None
        return result if 0 <= result <= 1 else None
    values = [item for item in values if 0 <= item <= 1]
    return statistics.fmean(values) if values else None


class FastALPRImageProcessor:
    """FastALPR adapter using only explicitly mounted detector/OCR assets."""

    processor_id = "fastalpr-onnx-cpu"
    processor_revision = "nexusai-bf1-v1"
    detector_id = "yolo-v9-t-384-license-plate-end2end"
    ocr_id = "cct-xs-v2-global-model"

    def __init__(
        self,
        engine: Any,
        cv2_module: Any,
        minimum_confidence: float = 0.75,
        *,
        model_hashes: dict[str, str] | None = None,
    ) -> None:
        if not 0 < minimum_confidence <= 1:
            raise ValueError("minimum detection confidence must be between zero and one")
        self.engine = engine
        self.cv2 = cv2_module
        self.minimum_confidence = minimum_confidence
        self.model_hashes = dict(model_hashes or {})

    @classmethod
    def from_environment(
        cls, *, minimum_confidence: float | None = None
    ) -> "FastALPRImageProcessor":
        detector_path = _required_model_path("FORENSIC_ANPR_DETECTOR_MODEL_PATH")
        ocr_path = _required_model_path("FORENSIC_ANPR_OCR_MODEL_PATH")
        ocr_config = _required_model_path("FORENSIC_ANPR_OCR_CONFIG_PATH")
        try:
            import cv2  # type: ignore[import-not-found]
            from fast_alpr import ALPR  # type: ignore[import-not-found]
            from fast_alpr.default_ocr import DefaultOCR  # type: ignore[import-not-found]
            from open_image_models import create_detector  # type: ignore[import-not-found]
        except ModuleNotFoundError as exc:
            raise ModelUnavailableError(f"FastALPR runtime dependency unavailable: {exc.name}") from exc

        providers = _onnx_providers()
        confidence = (
            float(os.getenv("FORENSIC_ANPR_MIN_DETECTION_CONFIDENCE", "0.75"))
            if minimum_confidence is None
            else float(minimum_confidence)
        )
        detector = create_detector(
            detector_path,
            backend="yolo_v9",
            class_labels={0: "license_plate"},
            conf_thresh=confidence,
            providers=providers,
        )
        ocr = DefaultOCR(
            hub_ocr_model=None,
            device="cpu",
            providers=providers,
            model_path=ocr_path,
            config_path=ocr_config,
            force_download=False,
        )
        return cls(
            ALPR(detector=detector, ocr=ocr),
            cv2,
            confidence,
            model_hashes={
                "detector": _sha256(detector_path),
                "ocr": _sha256(ocr_path),
                "ocr_config": _sha256(ocr_config),
            },
        )

    def process_image(
        self,
        path: Path,
        *,
        evidence_id: str,
        version_id: str,
        source_sha256: str,
        source_file: str,
        locator_prefix: dict[str, Any] | None = None,
    ) -> MediaProcessResult:
        frame = self.cv2.imread(str(path))
        if frame is None:
            raise MediaProcessingError("image decoder could not read the retained evidence")
        if len(frame.shape) < 2 or int(frame.shape[0]) <= 0 or int(frame.shape[1]) <= 0:
            raise MediaProcessingError("decoded image has invalid dimensions")
        height, width = int(frame.shape[0]), int(frame.shape[1])
        results = self.engine.predict(frame)
        observations: list[MediaObservation] = []
        low_confidence = 0
        for index, result in enumerate(results[:32], start=1):
            detection = result.detection
            detection_confidence = float(detection.confidence)
            if detection_confidence < self.minimum_confidence:
                low_confidence += 1
                continue
            bounds = detection.bounding_box
            x1 = max(0, min(int(bounds.x1), width))
            y1 = max(0, min(int(bounds.y1), height))
            x2 = max(0, min(int(bounds.x2), width))
            y2 = max(0, min(int(bounds.y2), height))
            if x2 <= x1 or y2 <= y1:
                continue
            raw_text = result.ocr.text if result.ocr is not None else ""
            normalized_text = normalize_plate(raw_text)
            ocr_confidence = mean_confidence(result.ocr.confidence if result.ocr is not None else None)
            locator_identity = json.dumps(
                locator_prefix or {},
                ensure_ascii=False,
                sort_keys=True,
                separators=(",", ":"),
            )
            candidate_id = str(uuid.uuid5(
                uuid.NAMESPACE_URL,
                f"{source_sha256}:{locator_identity}:{index}:{x1}:{y1}:{x2}:{y2}",
            ))
            locator = dict(locator_prefix or {})
            locator.update(
                {
                    "source_file": source_file,
                    "coordinate_space": "original_image_pixels",
                    "bbox": {"x": x1, "y": y1, "width": x2 - x1, "height": y2 - y1},
                    "plate_crop": {"candidate_id": candidate_id},
                }
            )
            crop = frame[y1:y2, x1:x2]
            crop_sha256 = ""
            encoded, crop_bytes = self.cv2.imencode(".png", crop)
            if encoded:
                crop_sha256 = hashlib.sha256(bytes(crop_bytes)).hexdigest()
            candidate = {
                "contract_version": PLATE_REGION_CONTRACT,
                "candidate_id": candidate_id,
                "parent_evidence_id": evidence_id,
                "parent_version_id": version_id,
                "parent_sha256": source_sha256,
                "detector_id": self.detector_id,
                "detector_version": "fast-alpr-0.4.0/open-image-models-0.6.0",
                "coordinate_space": "original_image_pixels",
                "bounds": locator["bbox"],
                "confidence": detection_confidence,
                "label": "license_plate",
                "transform_chain": ["source_pixels_unchanged"],
                "review_state": "model_candidate",
            }
            crop_provenance = {
                "contract_version": PLATE_CROP_CONTRACT,
                "candidate_id": candidate_id,
                "parent_evidence_id": evidence_id,
                "parent_version_id": version_id,
                "parent_sha256": source_sha256,
                "bounds": locator["bbox"],
                "coordinate_space": "original_image_pixels",
                "transform_chain": ["source_pixels_unchanged", "crop_original_bounds", "encode_png_lossless"],
                "encoding": "image/png",
                "crop_sha256": crop_sha256,
                "width_pixels": x2 - x1,
                "height_pixels": y2 - y1,
                "storage_state": "reconstructable_from_retained_source",
            }
            observations.append(
                MediaObservation(
                    observation_id=candidate_id,
                    contract_version=ANPR_OBSERVATION_CONTRACT,
                    observation_type="anpr_ocr_observation",
                    confidence=ocr_confidence,
                    citation_locator=locator,
                    payload={
                        "raw_plate_text": raw_text or "",
                        "normalized_plate_text": normalized_text,
                        "detection_confidence": detection_confidence,
                        "ocr_confidence": ocr_confidence,
                        "manual_review_required": True,
                        "candidate": candidate,
                        "crop": crop_provenance,
                        "processor": self.processor_id,
                        "processor_revision": self.processor_revision,
                        "detector_model": self.detector_id,
                        "detector_model_sha256": self.model_hashes.get("detector"),
                        "ocr_model": self.ocr_id,
                        "ocr_model_sha256": self.model_hashes.get("ocr"),
                        "ocr_config_sha256": self.model_hashes.get("ocr_config"),
                        "region_output_used": False,
                    },
                    warnings=("OCR is a model observation and requires analyst review",),
                )
            )
        limitations = [
            "OCR accuracy is not certified for Pakistani plates",
            "model region classification is intentionally ignored",
        ]
        providers = self._active_providers()
        if providers and "OpenVINOExecutionProvider" not in providers:
            limitations.append("OpenVINO provider was unavailable; ONNX Runtime used CPUExecutionProvider")
        if low_confidence:
            limitations.append(f"{low_confidence} detection(s) below the configured threshold were omitted")
        if not observations:
            limitations.append("no plate observation passed the configured detection threshold")
        return MediaProcessResult(
            modality="image",
            readiness=READY if observations else READY_WITH_WARNINGS,
            processor_id=self.processor_id,
            processor_revision=self.processor_revision,
            observations=tuple(observations),
            limitations=tuple(limitations),
            metadata={
                "image_width": width,
                "image_height": height,
                "detections_returned": len(results),
                "active_execution_providers": providers,
                "result_state": "COMPLETE_RESULTS" if observations else "COMPLETE_ZERO_RESULTS",
                "model_sha256": self.model_hashes,
            },
        )

    def _active_providers(self) -> list[str]:
        detector_model = getattr(getattr(self.engine, "detector", None), "model", None)
        if detector_model is None or not hasattr(detector_model, "get_providers"):
            return []
        return [str(provider) for provider in detector_model.get_providers()]


class LocalAIFaceProcessor:
    """Detection-only LocalAI face adapter with per-crop candidate embeddings."""

    processor_id = "localai-face-detect-cpu"
    processor_revision = "nexusai-post-bf-a-v1"

    def __init__(
        self,
        endpoint: str,
        model: str,
        cv2_module: Any,
        *,
        timeout_seconds: int = 180,
        max_input_bytes: int = 16 * 1024 * 1024,
        max_faces: int = 32,
        model_sha256: str = "",
        backend_digest: str = "",
    ) -> None:
        if not endpoint.strip() or not model.strip():
            raise ValueError("face endpoint and model are required")
        if timeout_seconds <= 0 or max_input_bytes <= 0 or not 1 <= max_faces <= 128:
            raise ValueError("face processor bounds must be positive")
        self.endpoint = endpoint.rstrip("/")
        self.model = model.strip()
        self.cv2 = cv2_module
        self.timeout_seconds = timeout_seconds
        self.max_input_bytes = max_input_bytes
        self.max_faces = max_faces
        self.model_sha256 = model_sha256.strip().lower()
        self.backend_digest = backend_digest.strip().lower()

    @classmethod
    def from_environment(cls) -> "LocalAIFaceProcessor":
        try:
            import cv2  # type: ignore[import-not-found]
        except ModuleNotFoundError as exc:
            raise ModelUnavailableError("face crop runtime dependency unavailable: cv2") from exc
        return cls(
            os.getenv("FORENSIC_FACE_ENDPOINT", "http://api:8080/v1/face"),
            os.getenv("FORENSIC_FACE_MODEL", "face-detect-yunet-sface"),
            cv2,
            timeout_seconds=max(1, int(os.getenv("FORENSIC_FACE_TIMEOUT_SECONDS", "180"))),
            max_input_bytes=max(1, int(os.getenv("FORENSIC_FACE_MAX_INPUT_BYTES", str(16 * 1024 * 1024)))),
            max_faces=max(1, int(os.getenv("FORENSIC_FACE_MAX_OBSERVATIONS", "32"))),
            model_sha256=os.getenv("FORENSIC_FACE_MODEL_SHA256", ""),
            backend_digest=os.getenv("FORENSIC_FACE_BACKEND_DIGEST", ""),
        )

    def process_image(
        self,
        path: Path,
        *,
        evidence_id: str,
        version_id: str,
        source_sha256: str,
        source_file: str,
        locator_prefix: dict[str, Any] | None = None,
    ) -> MediaProcessResult:
        source_bytes = path.read_bytes()
        if len(source_bytes) > self.max_input_bytes:
            raise MediaProcessingError(
                f"face input exceeds configured maximum of {self.max_input_bytes} bytes"
            )
        frame = self.cv2.imread(str(path))
        if frame is None or len(frame.shape) < 2:
            raise MediaProcessingError("face processor could not decode the image")
        height, width = int(frame.shape[0]), int(frame.shape[1])
        mime = {".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp"}.get(
            path.suffix.lower(), "image/png"
        )
        source_data_uri = f"data:{mime};base64,{base64.b64encode(source_bytes).decode('ascii')}"
        detected = self._post_json("detect", {"model": self.model, "img": source_data_uri})
        faces = detected.get("faces")
        if not isinstance(faces, list):
            raise MediaProcessingError("LocalAI face detector returned an invalid faces payload")

        observations: list[MediaObservation] = []
        limitations: list[str] = []
        locator_identity = json.dumps(
            locator_prefix or {}, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        for index, face in enumerate(faces[: self.max_faces], start=1):
            region = face.get("region") if isinstance(face, dict) else None
            if not isinstance(region, dict):
                limitations.append(f"face {index} omitted because its region was invalid")
                continue
            x = float(region.get("x", 0))
            y = float(region.get("y", 0))
            x1 = max(0, min(math.floor(x), width))
            y1 = max(0, min(math.floor(y), height))
            x2 = max(0, min(math.ceil(x + float(region.get("w", 0))), width))
            y2 = max(0, min(math.ceil(y + float(region.get("h", 0))), height))
            if x2 <= x1 or y2 <= y1:
                limitations.append(f"face {index} omitted because its bounded region was empty")
                continue
            bounds = {"x": x1, "y": y1, "width": x2 - x1, "height": y2 - y1}
            face_observation_id = str(uuid.uuid5(
                uuid.NAMESPACE_URL,
                f"{source_sha256}:{locator_identity}:face:{index}:{x1}:{y1}:{x2}:{y2}",
            ))
            crop = frame[y1:y2, x1:x2]
            encoded, crop_bytes = self.cv2.imencode(".png", crop)
            if not encoded:
                limitations.append(f"face {index} omitted because its crop could not be encoded")
                continue
            crop_payload = bytes(crop_bytes)
            crop_sha256 = hashlib.sha256(crop_payload).hexdigest()
            embedding: list[float] = []
            embedding_model = self.model
            embedding_warning = ""
            try:
                embedded = self._post_json(
                    "embed",
                    {
                        "model": self.model,
                        "img": f"data:image/png;base64,{base64.b64encode(crop_payload).decode('ascii')}",
                    },
                )
                raw_embedding = embedded.get("embedding")
                if not isinstance(raw_embedding, list) or not raw_embedding:
                    raise MediaProcessingError("LocalAI face embedder returned an empty vector")
                embedding = [float(value) for value in raw_embedding]
                embedding_model = str(embedded.get("model") or self.model)
            except (MediaProcessingError, TypeError, ValueError) as exc:
                embedding_warning = str(exc)
                limitations.append(f"face {index} candidate embedding unavailable: {exc}")

            locator = dict(locator_prefix or {})
            locator.update(
                {
                    "source_file": source_file,
                    "coordinate_space": "original_image_pixels",
                    "bbox": bounds,
                    "face_crop": {"face_observation_id": face_observation_id},
                }
            )
            confidence = float(face.get("confidence", 0.0)) if isinstance(face, dict) else 0.0
            crop_provenance = {
                "contract_version": FACE_CROP_CONTRACT,
                "face_observation_id": face_observation_id,
                "parent_evidence_id": evidence_id,
                "parent_version_id": version_id,
                "parent_sha256": source_sha256,
                "bounds": bounds,
                "coordinate_space": "original_image_pixels",
                "transform_chain": ["source_pixels_unchanged", "crop_original_bounds", "encode_png_lossless"],
                "encoding": "image/png",
                "face_crop_sha256": crop_sha256,
                "width_pixels": x2 - x1,
                "height_pixels": y2 - y1,
                "storage_state": "reconstructable_from_retained_source",
            }
            warnings = [
                "Face output is a model observation and requires analyst review",
                "Candidate visual similarity is not identity",
            ]
            if embedding_warning:
                warnings.append(embedding_warning)
            observations.append(
                MediaObservation(
                    observation_id=face_observation_id,
                    contract_version=FACE_OBSERVATION_CONTRACT,
                    observation_type="face_detection_observation",
                    confidence=confidence,
                    citation_locator=locator,
                    payload={
                        "face_observation_id": face_observation_id,
                        "parent_evidence_id": evidence_id,
                        "parent_version_id": version_id,
                        "source_sha256": source_sha256,
                        "frame_timestamp_seconds": (locator_prefix or {}).get("timestamp_seconds"),
                        "bbox": bounds,
                        "coordinate_space": "original_image_pixels",
                        "detector_model": self.model,
                        "detector_version": self.processor_revision,
                        "embedding_model": embedding_model,
                        "embedding_version": self.processor_revision,
                        "model_sha256": self.model_sha256 or None,
                        "backend_digest": self.backend_digest or None,
                        "detection_confidence": confidence,
                        "face_crop_artifact": crop_provenance,
                        "face_crop_sha256": crop_sha256,
                        "embedding_dimension": len(embedding),
                        "embedding": embedding,
                        "embedding_status": "available" if embedding else "unavailable",
                        "quality": {
                            "crop_width_pixels": x2 - x1,
                            "crop_height_pixels": y2 - y1,
                            "minimum_dimension_pixels": min(x2 - x1, y2 - y1),
                        },
                        "review_state": "model_candidate",
                        "similarity_semantics": "candidate visual similarity; not identity",
                        "processor": self.processor_id,
                        "processor_revision": self.processor_revision,
                    },
                    warnings=tuple(warnings),
                )
            )
        if len(faces) > self.max_faces:
            limitations.append(f"face observations were capped at {self.max_faces}")
        if not faces:
            limitations.append("no faces were detected")
        return MediaProcessResult(
            modality="image",
            readiness=READY if observations or not faces else READY_WITH_WARNINGS,
            processor_id=self.processor_id,
            processor_revision=self.processor_revision,
            observations=tuple(observations),
            limitations=tuple(limitations),
            metadata={
                "image_width": width,
                "image_height": height,
                "faces_returned": len(faces),
                "face_observation_cap": self.max_faces,
            },
        )

    def _post_json(self, operation: str, payload: dict[str, Any]) -> dict[str, Any]:
        request = urllib.request.Request(
            f"{self.endpoint}/{operation}",
            data=json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8"),
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                decoded = json.loads(response.read().decode("utf-8"))
        except (urllib.error.HTTPError, urllib.error.URLError, TimeoutError, json.JSONDecodeError) as exc:
            raise MediaProcessingError(f"LocalAI face {operation} failed: {exc}") from exc
        if not isinstance(decoded, dict):
            raise MediaProcessingError(f"LocalAI face {operation} returned a non-object response")
        return decoded


class SigLIPImageEmbeddingProcessor:
    """Offline CPU image encoder for bounded, scoped semantic retrieval."""

    processor_id = "siglip-image-embedding-cpu"
    processor_revision = "nexusai-post-bf-a-v1"

    def __init__(
        self,
        model_path: Path,
        *,
        model_id: str,
        model_revision: str,
        model_sha256: str,
        max_input_bytes: int = 16 * 1024 * 1024,
        cpu_threads: int = 1,
    ) -> None:
        if not model_path.is_dir() or not (model_path / "model.safetensors").is_file():
            raise ModelUnavailableError("configured SigLIP model directory is incomplete")
        if not re.fullmatch(r"[0-9a-f]{64}", model_sha256.strip().lower()):
            raise ModelUnavailableError("FORENSIC_IMAGE_EMBEDDING_MODEL_SHA256 must be a SHA-256 digest")
        if max_input_bytes <= 0 or cpu_threads <= 0:
            raise ValueError("SigLIP processor bounds must be positive")
        actual_sha256 = _sha256(model_path / "model.safetensors")
        if actual_sha256 != model_sha256.strip().lower():
            raise ModelUnavailableError("configured SigLIP safetensors checksum does not match")
        self.model_path = model_path
        self.model_id = model_id.strip()
        self.model_revision = model_revision.strip()
        self.model_sha256 = actual_sha256
        self.max_input_bytes = max_input_bytes
        self.cpu_threads = cpu_threads
        self._model: Any | None = None
        self._processor: Any | None = None
        self._torch: Any | None = None
        self._image_type: Any | None = None

    @classmethod
    def from_environment(cls) -> "SigLIPImageEmbeddingProcessor":
        configured = os.getenv("FORENSIC_IMAGE_EMBEDDING_MODEL_PATH", "").strip()
        if not configured:
            raise ModelUnavailableError(
                "FORENSIC_IMAGE_EMBEDDING_MODEL_PATH is required; implicit model download is disabled"
            )
        return cls(
            Path(configured),
            model_id=os.getenv("FORENSIC_IMAGE_EMBEDDING_MODEL", "google/siglip-base-patch16-224"),
            model_revision=os.getenv("FORENSIC_IMAGE_EMBEDDING_MODEL_REVISION", ""),
            model_sha256=os.getenv("FORENSIC_IMAGE_EMBEDDING_MODEL_SHA256", ""),
            max_input_bytes=max(
                1, int(os.getenv("FORENSIC_IMAGE_EMBEDDING_MAX_INPUT_BYTES", str(16 * 1024 * 1024)))
            ),
            cpu_threads=max(1, int(os.getenv("FORENSIC_IMAGE_EMBEDDING_CPU_THREADS", "1"))),
        )

    def process_image(
        self,
        path: Path,
        *,
        evidence_id: str,
        version_id: str,
        source_sha256: str,
        source_file: str,
        locator_prefix: dict[str, Any] | None = None,
    ) -> MediaProcessResult:
        if path.stat().st_size > self.max_input_bytes:
            raise MediaProcessingError(
                f"semantic image input exceeds configured maximum of {self.max_input_bytes} bytes"
            )
        self._load()
        assert self._image_type is not None
        assert self._processor is not None
        assert self._model is not None
        assert self._torch is not None
        try:
            with self._image_type.open(path) as image:
                image.load()
                inputs = self._processor(images=image.convert("RGB"), return_tensors="pt")
            with self._torch.inference_mode():
                features = self._model.get_image_features(pixel_values=inputs["pixel_values"])
                if hasattr(features, "pooler_output"):
                    features = features.pooler_output
                normalized = features / features.norm(p=2, dim=-1, keepdim=True).clamp_min(1e-12)
            embedding = [float(value) for value in normalized[0].detach().cpu().tolist()]
        except (OSError, RuntimeError, TypeError, ValueError, KeyError) as exc:
            raise MediaProcessingError(f"SigLIP could not encode the image: {exc}") from exc
        if not embedding or not all(math.isfinite(value) for value in embedding):
            raise MediaProcessingError("SigLIP returned an invalid image embedding")

        locator_identity = json.dumps(
            locator_prefix or {}, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        )
        observation_id = str(uuid.uuid5(
            uuid.NAMESPACE_URL,
            f"{source_sha256}:{locator_identity}:siglip:{self.model_revision}:{self.model_sha256}",
        ))
        locator = dict(locator_prefix or {})
        locator.update({"source_file": source_file, "scope": "full_image"})
        observation = MediaObservation(
            observation_id=observation_id,
            contract_version=IMAGE_EMBEDDING_CONTRACT,
            observation_type="semantic_image_embedding_observation",
            confidence=None,
            citation_locator=locator,
            payload={
                "parent_evidence_id": evidence_id,
                "parent_version_id": version_id,
                "source_sha256": source_sha256,
                "frame_timestamp_seconds": (locator_prefix or {}).get("timestamp_seconds"),
                "embedding": embedding,
                "embedding_dimension": len(embedding),
                "embedding_normalization": "l2",
                "embedding_model": self.model_id,
                "embedding_revision": self.model_revision,
                "model_sha256": self.model_sha256,
                "preprocessing": "SiglipProcessor RGB 224x224 publisher configuration",
                "review_state": "model_candidate",
                "similarity_semantics": "candidate semantic visual similarity; not evidence identity or fact",
                "processor": self.processor_id,
                "processor_revision": self.processor_revision,
            },
            warnings=(
                "Semantic similarity is a model observation and requires analyst review",
                "Similarity does not establish duplicate identity, object identity, event identity, or truth",
            ),
        )
        return MediaProcessResult(
            modality="image",
            readiness=READY,
            processor_id=self.processor_id,
            processor_revision=self.processor_revision,
            observations=(observation,),
            limitations=(
                "English WebLI pretraining limits Urdu and Roman-Urdu text-to-image reliability",
                "runtime retrieval is bounded to explicitly authorized candidate evidence",
            ),
            metadata={
                "embedding_dimension": len(embedding),
                "model": self.model_id,
                "model_revision": self.model_revision,
                "model_sha256": self.model_sha256,
                "cpu_threads": self.cpu_threads,
            },
        )

    def embed_text(self, text: str) -> list[float]:
        """Encode bounded text for offline evaluation with the matched encoder."""

        query = text.strip()
        if not query or len(query) > 512:
            raise MediaProcessingError("semantic image query text must contain 1 to 512 characters")
        self._load()
        assert self._processor is not None
        assert self._model is not None
        assert self._torch is not None
        inputs = self._processor(
            text=[query], padding="max_length", max_length=64, truncation=True, return_tensors="pt"
        )
        with self._torch.inference_mode():
            features = self._model.get_text_features(
                input_ids=inputs["input_ids"], attention_mask=inputs.get("attention_mask")
            )
            if hasattr(features, "pooler_output"):
                features = features.pooler_output
            normalized = features / features.norm(p=2, dim=-1, keepdim=True).clamp_min(1e-12)
        return [float(value) for value in normalized[0].detach().cpu().tolist()]

    def _load(self) -> None:
        if self._model is not None:
            return
        try:
            import torch  # type: ignore[import-not-found]
            from PIL import Image  # type: ignore[import-not-found]
            from transformers import SiglipModel, SiglipProcessor  # type: ignore[import-not-found]
        except ModuleNotFoundError as exc:
            raise ModelUnavailableError(f"SigLIP runtime dependency unavailable: {exc.name}") from exc
        torch.set_num_threads(self.cpu_threads)
        torch.set_num_interop_threads(1)
        self._processor = SiglipProcessor.from_pretrained(
            str(self.model_path), local_files_only=True
        )
        self._model = SiglipModel.from_pretrained(
            str(self.model_path), local_files_only=True, use_safetensors=True
        ).eval()
        self._torch = torch
        self._image_type = Image


class TesseractImageOCRProcessor:
    """Bounded general scene OCR; deliberately separate from plate OCR."""

    processor_id = "tesseract-general-image-ocr-cpu"
    processor_revision = "nexusai-post-bf-a-v1"

    def __init__(
        self,
        executable: Path,
        tessdata_path: Path,
        *,
        languages: str = "eng+urd",
        tessdata_revision: str,
        tessdata_hashes: dict[str, str],
        timeout_seconds: int = 60,
        max_input_bytes: int = 16 * 1024 * 1024,
        max_output_bytes: int = 4 * 1024 * 1024,
        max_observations: int = 256,
        page_segmentation_mode: int = 11,
    ) -> None:
        if not executable.is_file() or not tessdata_path.is_dir():
            raise ModelUnavailableError("configured Tesseract runtime or tessdata directory is unavailable")
        language_list = languages.split("+")
        if not language_list or any(not re.fullmatch(r"[a-z]{3}", item) for item in language_list):
            raise ValueError("Tesseract languages must be plus-separated ISO-style three-letter codes")
        if timeout_seconds <= 0 or max_input_bytes <= 0 or max_output_bytes <= 0:
            raise ValueError("Tesseract processor bounds must be positive")
        if not 1 <= max_observations <= 2048 or not 0 <= page_segmentation_mode <= 13:
            raise ValueError("Tesseract observation or page-segmentation bound is invalid")
        normalized_hashes: dict[str, str] = {}
        for language in language_list:
            asset = tessdata_path / f"{language}.traineddata"
            expected = str(tessdata_hashes.get(language) or "").lower()
            if not asset.is_file() or not re.fullmatch(r"[0-9a-f]{64}", expected):
                raise ModelUnavailableError(f"pinned Tesseract asset is unavailable for {language}")
            if _sha256(asset) != expected:
                raise ModelUnavailableError(f"Tesseract traineddata checksum does not match for {language}")
            normalized_hashes[language] = expected
        self.executable = executable
        self.tessdata_path = tessdata_path
        self.languages = languages
        self.language_list = language_list
        self.tessdata_revision = tessdata_revision.strip()
        self.tessdata_hashes = normalized_hashes
        self.timeout_seconds = timeout_seconds
        self.max_input_bytes = max_input_bytes
        self.max_output_bytes = max_output_bytes
        self.max_observations = max_observations
        self.page_segmentation_mode = page_segmentation_mode
        self.runtime_version = self._runtime_version()

    @classmethod
    def from_environment(cls) -> "TesseractImageOCRProcessor":
        executable_value = os.getenv("FORENSIC_OCR_TESSERACT_PATH", "tesseract").strip()
        executable = Path(executable_value) if Path(executable_value).is_file() else Path(shutil.which(executable_value) or "")
        tessdata_path = Path(os.getenv("FORENSIC_OCR_TESSDATA_PATH", "").strip())
        hashes = {
            "eng": os.getenv("FORENSIC_OCR_ENG_SHA256", ""),
            "urd": os.getenv("FORENSIC_OCR_URD_SHA256", ""),
        }
        return cls(
            executable,
            tessdata_path,
            languages=os.getenv("FORENSIC_OCR_LANGUAGES", "eng+urd"),
            tessdata_revision=os.getenv("FORENSIC_OCR_TESSDATA_REVISION", ""),
            tessdata_hashes=hashes,
            timeout_seconds=max(1, int(os.getenv("FORENSIC_OCR_TIMEOUT_SECONDS", "60"))),
            max_input_bytes=max(1, int(os.getenv("FORENSIC_OCR_MAX_INPUT_BYTES", str(16 * 1024 * 1024)))),
            max_output_bytes=max(1, int(os.getenv("FORENSIC_OCR_MAX_OUTPUT_BYTES", str(4 * 1024 * 1024)))),
            max_observations=max(1, int(os.getenv("FORENSIC_OCR_MAX_OBSERVATIONS", "256"))),
            page_segmentation_mode=int(os.getenv("FORENSIC_OCR_PSM", "11")),
        )

    def process_image(
        self,
        path: Path,
        *,
        evidence_id: str,
        version_id: str,
        source_sha256: str,
        source_file: str,
        locator_prefix: dict[str, Any] | None = None,
    ) -> MediaProcessResult:
        if path.stat().st_size > self.max_input_bytes:
            raise MediaProcessingError(f"OCR input exceeds configured maximum of {self.max_input_bytes} bytes")
        environment = os.environ.copy()
        environment["TESSDATA_PREFIX"] = str(self.tessdata_path)
        environment["OMP_THREAD_LIMIT"] = "1"
        try:
            completed = subprocess.run(
                [
                    str(self.executable), str(path), "stdout", "-l", self.languages,
                    "--oem", "1", "--psm", str(self.page_segmentation_mode),
                    "-c", "tessedit_create_tsv=1",
                ],
                check=False,
                capture_output=True,
                timeout=self.timeout_seconds,
                env=environment,
            )
        except (OSError, subprocess.SubprocessError) as exc:
            raise MediaProcessingError(f"Tesseract general OCR failed: {exc}") from exc
        if completed.returncode != 0:
            stderr = completed.stderr.decode("utf-8", errors="replace")[:1000].strip()
            raise MediaProcessingError(f"Tesseract general OCR exited {completed.returncode}: {stderr}")
        if len(completed.stdout) > self.max_output_bytes:
            raise MediaProcessingError("Tesseract TSV output exceeds the configured byte ceiling")
        rows = list(csv.DictReader(io.StringIO(completed.stdout.decode("utf-8", errors="replace")), delimiter="\t"))
        grouped: dict[tuple[str, str, str, str], list[dict[str, Any]]] = {}
        for row in rows:
            raw_text = str(row.get("text") or "")
            if str(row.get("level")) != "5" or not raw_text.strip():
                continue
            try:
                confidence = float(row.get("conf") or -1)
                left, top = int(row.get("left") or 0), int(row.get("top") or 0)
                width, height = int(row.get("width") or 0), int(row.get("height") or 0)
            except (TypeError, ValueError):
                continue
            if width <= 0 or height <= 0:
                continue
            key = tuple(str(row.get(name) or "0") for name in ("page_num", "block_num", "par_num", "line_num"))
            grouped.setdefault(key, []).append({
                "raw_text": raw_text,
                "confidence": confidence,
                "left": left,
                "top": top,
                "right": left + width,
                "bottom": top + height,
            })

        observations: list[MediaObservation] = []
        locator_identity = json.dumps(locator_prefix or {}, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
        for line_number, (line_key, words) in enumerate(grouped.items(), start=1):
            if len(observations) >= self.max_observations:
                break
            raw_text = " ".join(word["raw_text"] for word in words)
            valid_confidences = [word["confidence"] for word in words if 0 <= word["confidence"] <= 100]
            confidence = statistics.fmean(valid_confidences) / 100 if valid_confidences else None
            bounds = {
                "x": min(word["left"] for word in words),
                "y": min(word["top"] for word in words),
                "width": max(word["right"] for word in words) - min(word["left"] for word in words),
                "height": max(word["bottom"] for word in words) - min(word["top"] for word in words),
            }
            observation_id = str(uuid.uuid5(
                uuid.NAMESPACE_URL,
                f"{source_sha256}:{locator_identity}:ocr:{line_key}:{raw_text}",
            ))
            locator = dict(locator_prefix or {})
            locator.update({
                "source_file": source_file,
                "coordinate_space": "original_image_pixels",
                "bbox": bounds,
                "ocr_line": line_number,
            })
            observations.append(MediaObservation(
                observation_id=observation_id,
                contract_version=IMAGE_OCR_OBSERVATION_CONTRACT,
                observation_type="general_image_ocr_observation",
                confidence=confidence,
                citation_locator=locator,
                payload={
                    "parent_evidence_id": evidence_id,
                    "parent_version_id": version_id,
                    "source_sha256": source_sha256,
                    "frame_timestamp_seconds": (locator_prefix or {}).get("timestamp_seconds"),
                    "raw_text": raw_text,
                    "bbox": bounds,
                    "polygon": [
                        [bounds["x"], bounds["y"]],
                        [bounds["x"] + bounds["width"], bounds["y"]],
                        [bounds["x"] + bounds["width"], bounds["y"] + bounds["height"]],
                        [bounds["x"], bounds["y"] + bounds["height"]],
                    ],
                    "coordinate_space": "original_image_pixels",
                    "confidence": confidence,
                    "script_family": _ocr_script_family(raw_text),
                    "language_claim": "undetermined",
                    "configured_languages": self.language_list,
                    "processor": self.processor_id,
                    "processor_revision": self.processor_revision,
                    "runtime_version": self.runtime_version,
                    "tessdata_revision": self.tessdata_revision,
                    "tessdata_sha256": self.tessdata_hashes,
                    "page_segmentation_mode": self.page_segmentation_mode,
                    "review_state": "model_candidate",
                    "role": "general_scene_text_only_not_plate_ocr",
                },
                warnings=(
                    "OCR text is a model observation and requires comparison with source pixels",
                    "Detected script family is not a language-identification claim",
                ),
            ))
        limitations: list[str] = [
            "general scene OCR is LIMITED and does not replace FastALPR plate OCR",
            "Urdu, mixed-script, blur, rotation and stylized text require analyst review",
        ]
        if not observations:
            limitations.append("no non-empty OCR region was returned")
        if len(grouped) > self.max_observations:
            limitations.append(f"OCR observations were capped at {self.max_observations}")
        return MediaProcessResult(
            modality="image",
            readiness=READY if observations else READY_WITH_WARNINGS,
            processor_id=self.processor_id,
            processor_revision=self.processor_revision,
            observations=tuple(observations),
            limitations=tuple(limitations),
            metadata={
                "ocr_region_count": len(observations),
                "result_state": "COMPLETE_RESULTS" if observations else "COMPLETE_ZERO_RESULTS",
                "configured_languages": self.language_list,
                "runtime_version": self.runtime_version,
                "tessdata_revision": self.tessdata_revision,
                "tessdata_sha256": self.tessdata_hashes,
                "omp_thread_limit": 1,
            },
        )

    def _runtime_version(self) -> str:
        completed = subprocess.run(
            [str(self.executable), "--version"], check=True, capture_output=True, timeout=10
        )
        return completed.stdout.decode("utf-8", errors="replace").splitlines()[0].strip()


class CompositeImageProcessor:
    """Run multiple bounded image roles without letting one relabel another."""

    processor_id = "nexusai-composite-image"
    processor_revision = "post-bf-a-v2"

    def __init__(self, processors: tuple[ImageProcessor, ...]) -> None:
        if not processors:
            raise ValueError("at least one image processor is required")
        self.processors = processors

    def process_image(self, path: Path, **context: Any) -> MediaProcessResult:
        results: list[MediaProcessResult] = []
        unavailable: list[str] = []
        for processor in self.processors:
            try:
                results.append(processor.process_image(path, **context))
            except MediaProcessingError as exc:
                unavailable.append(f"{getattr(processor, 'processor_id', type(processor).__name__)} unavailable: {exc}")
        return MediaProcessResult(
            modality="image",
            readiness=READY_WITH_WARNINGS if unavailable or any(result.readiness != READY for result in results) else READY,
            processor_id=self.processor_id,
            processor_revision=self.processor_revision,
            observations=tuple(observation for result in results for observation in result.observations),
            limitations=tuple(dict.fromkeys([
                *(limitation for result in results for limitation in result.limitations),
                *unavailable,
            ])),
            metadata={result.processor_id: result.metadata for result in results},
        )


class ContextualImageProcessor:
    """Content-gated ANPR/OCR routing without analyst model selection.

    The plate detector is the bounded applicability gate.  General OCR runs
    when no plate candidate is found, or when both roles were explicitly
    admitted by a governed route plan.
    """

    processor_id = "nexusai-contextual-anpr-ocr"
    processor_revision = "nxmmr-anpr-ocr-vertical-v1"

    def __init__(self, anpr: ImageProcessor, ocr: ImageProcessor) -> None:
        self.anpr = anpr
        self.ocr = ocr

    def process_image(self, path: Path, **context: Any) -> MediaProcessResult:
        requested = {str(role).strip().lower() for role in context.pop("requested_roles", ())}
        if requested == {"ocr"}:
            result = self.ocr.process_image(path, **context)
            return self._wrap(result, {"image_anpr": "NOT_RUN", "ocr": result.metadata.get("result_state", "COMPLETE_RESULTS" if result.observations else "COMPLETE_ZERO_RESULTS")}, "explicit_ocr")
        anpr_result = self.anpr.process_image(path, **context)
        if requested == {"anpr"}:
            return self._wrap(
                anpr_result,
                {
                    "image_anpr": anpr_result.metadata.get(
                        "result_state",
                        "COMPLETE_RESULTS" if anpr_result.observations else "COMPLETE_ZERO_RESULTS",
                    ),
                    "ocr": "NOT_RUN",
                },
                "explicit_anpr",
            )
        run_ocr = "ocr" in requested and "anpr" in requested or not anpr_result.observations
        if not run_ocr:
            return self._wrap(
                anpr_result,
                {"image_anpr": anpr_result.metadata.get("result_state", "COMPLETE_RESULTS"), "ocr": "NOT_RUN"},
                "automatic_plate_candidate_gate",
            )
        ocr_result = self.ocr.process_image(path, **context)
        observations = (*anpr_result.observations, *ocr_result.observations)
        combined = MediaProcessResult(
            modality="image",
            readiness=READY if anpr_result.readiness == READY and ocr_result.readiness == READY else READY_WITH_WARNINGS,
            processor_id=self.processor_id,
            processor_revision=self.processor_revision,
            observations=observations,
            limitations=tuple(dict.fromkeys((*anpr_result.limitations, *ocr_result.limitations))),
            metadata={
                "anpr": anpr_result.metadata,
                "ocr": ocr_result.metadata,
                "result_states": {
                    "image_anpr": anpr_result.metadata.get("result_state", "COMPLETE_RESULTS" if anpr_result.observations else "COMPLETE_ZERO_RESULTS"),
                    "ocr": ocr_result.metadata.get("result_state", "COMPLETE_RESULTS" if ocr_result.observations else "COMPLETE_ZERO_RESULTS"),
                },
                "routing_policy": "explicit_both" if requested else "automatic_plate_candidate_then_general_ocr",
            },
        )
        return combined

    def _wrap(self, result: MediaProcessResult, states: dict[str, str], policy: str) -> MediaProcessResult:
        return MediaProcessResult(
            modality=result.modality,
            readiness=result.readiness,
            processor_id=self.processor_id,
            processor_revision=self.processor_revision,
            observations=result.observations,
            limitations=result.limitations,
            metadata={**result.metadata, "result_states": states, "routing_policy": policy},
        )


class LocalAIASRProcessor:
    """Optional OpenAI-compatible ASR client with bounded local-only defaults."""

    processor_id = "localai-openai-compatible-asr"
    processor_revision = "nexusai-next-breadth-asr-timing-v1"

    def __init__(self, endpoint: str, model: str, timeout_seconds: int = 300) -> None:
        self.endpoint = endpoint
        self.model = model
        self.timeout_seconds = timeout_seconds

    @classmethod
    def from_environment(cls) -> "LocalAIASRProcessor":
        endpoint = os.getenv("FORENSIC_ASR_ENDPOINT", "http://api:8080/v1/audio/transcriptions").strip()
        model = os.getenv("FORENSIC_ASR_MODEL", "").strip()
        if not model:
            raise ModelUnavailableError("FORENSIC_ASR_MODEL is required; implicit model download is disabled")
        if not endpoint.startswith(("http://api:", "http://local-ai:", "http://127.0.0.1:", "http://localhost:")):
            raise ModelUnavailableError("FORENSIC_ASR_ENDPOINT must target the configured local runtime")
        return cls(endpoint, model, max(15, int(os.getenv("FORENSIC_ASR_TIMEOUT_SECONDS", "300"))))

    def transcribe(
        self,
        path: Path,
        *,
        evidence_id: str,
        version_id: str,
        source_file: str,
        language: str | None = None,
    ) -> tuple[MediaObservation, ...]:
        maximum = max(1, int(os.getenv("FORENSIC_ASR_MAX_INPUT_BYTES", str(64 * 1024 * 1024))))
        if path.stat().st_size > maximum:
            raise MediaProcessingError("audio exceeds the bounded ASR input limit")
        boundary = f"nexusai-{uuid.uuid4().hex}"
        audio = path.read_bytes()
        fields = {"model": self.model, "response_format": "verbose_json"}
        if language is not None:
            fields["language"] = language
        body = _multipart_body(
            boundary,
            fields,
            "file",
            source_file,
            "application/octet-stream",
            audio,
        )
        request = urllib.request.Request(
            self.endpoint,
            data=body,
            method="POST",
            headers={"Content-Type": f"multipart/form-data; boundary={boundary}", "Accept": "application/json"},
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                result = json.loads(response.read())
        except (OSError, urllib.error.URLError, json.JSONDecodeError) as exc:
            raise MediaProcessingError(f"local ASR request failed: {exc}") from exc
        if not isinstance(result, dict):
            raise MediaProcessingError("local ASR response must be an object")
        segments = result.get("segments")
        if not isinstance(segments, list) or not segments:
            # Clip duration does not establish where its spoken words occurred.
            # Keep text-only responses usable without inventing a segment range.
            text = str(result.get("text") or "").strip()
            segments = [{"text": text}]
        if language == "ur":
            transcript_text = " ".join(
                str(segment.get("text") or "").strip()
                for segment in segments
                if isinstance(segment, dict) and str(segment.get("text") or "").strip()
            )
            if contains_devanagari_script(transcript_text):
                raise MediaProcessingError(
                    "trusted Urdu transcription returned Devanagari script; result rejected"
                )
            if transcript_text and not contains_urdu_script(transcript_text):
                raise MediaProcessingError(
                    "trusted Urdu transcription did not return Urdu script; result rejected"
                )
        observations: list[MediaObservation] = []
        for index, segment in enumerate(segments[:500]):
            if not isinstance(segment, dict) or not str(segment.get("text") or "").strip():
                continue
            start = _asr_seconds(segment.get("start"))
            end = _asr_seconds(segment.get("end"))
            if start is not None and end is not None and end < start:
                start = end = None
            timing_status = "reported" if start is not None and end is not None else "unavailable_or_partial"
            locator = {"source_file": source_file, "start_seconds": start, "end_seconds": end}
            text = str(segment.get("text") or "").strip()
            observation_id = str(uuid.uuid5(uuid.NAMESPACE_URL, f"{evidence_id}:{version_id}:asr:{index}:{start}:{end}"))
            observations.append(
                MediaObservation(
                    observation_id=observation_id,
                    contract_version=AUDIO_TIMESTAMP_SEGMENT_CONTRACT,
                    observation_type="audio_transcript_segment",
                    confidence=mean_confidence(segment.get("confidence")),
                    citation_locator=locator,
                    payload={
                        "transcript_contract": AUDIO_TRANSCRIPT_CONTRACT,
                        "text": text,
                        "language": result.get("language") if isinstance(result, dict) else None,
                        "requested_language": language,
                        "detected_language": (
                            result.get("language") if language is None and isinstance(result, dict) else None
                        ),
                        "detected_language_probability": (
                            result.get("language_probability")
                            if language is None and isinstance(result, dict)
                            else None
                        ),
                        "start_seconds": start,
                        "end_seconds": end,
                        "timing_status": timing_status,
                        "raw_start": segment.get("start"),
                        "raw_end": segment.get("end"),
                        "processor": self.processor_id,
                        "processor_revision": self.processor_revision,
                        "model": self.model,
                        "parent_evidence_id": evidence_id,
                        "parent_version_id": version_id,
                        "manual_review_required": True,
                    },
                    warnings=("Transcript is a model observation and requires analyst review",)
                    + (
                        ("Segment timing is unavailable or partial; do not infer a spoken-word range",)
                        if timing_status != "reported" else ()
                    ),
                )
            )
            effective_language = str(
                language or (result.get("language") if isinstance(result, dict) else "") or ""
            ).strip().lower()
            if effective_language == "ur" and contains_urdu_script(text):
                derived = roman_urdu_payload(
                    text,
                    parent_observation_id=observation_id,
                    evidence_id=evidence_id,
                    version_id=version_id,
                    source_file=source_file,
                    start_seconds=start,
                    end_seconds=end,
                )
                observations.append(
                    MediaObservation(
                        observation_id=derived.pop("observation_id"),
                        contract_version=ROMAN_URDU_SEGMENT_CONTRACT,
                        observation_type="audio_roman_urdu_segment",
                        confidence=None,
                        citation_locator=locator,
                        payload=derived,
                        warnings=(
                            "Roman Urdu is a deterministic derived representation; raw Urdu remains authoritative",
                            "Human review is required for transliteration quality",
                        ),
                    )
                )
        return tuple(observations)


class DeterministicMediaProcessor:
    """Metadata, preview, audio, and sampled-video processing without new models."""

    processor_id = "nexusai-deterministic-media"
    processor_revision = "bf-media-v1"

    def __init__(
        self,
        image_processor: ImageProcessor | None = None,
        audio_transcriber: AudioTranscriber | None = None,
        video_anpr_processor: Any | None = None,
    ) -> None:
        self.image_processor = image_processor
        self.audio_transcriber = audio_transcriber
        self.video_anpr_processor = video_anpr_processor

    def process(
        self,
        path: Path,
        *,
        modality: str,
        evidence_id: str,
        version_id: str,
        source_sha256: str,
        source_file: str,
        admitted_metadata: dict[str, Any] | None = None,
    ) -> MediaProcessResult:
        if modality == "image":
            return self._process_image(
                path,
                evidence_id=evidence_id,
                version_id=version_id,
                source_sha256=source_sha256,
                source_file=source_file,
                admitted_metadata=admitted_metadata or {},
            )
        if modality == "audio":
            return self._process_audio(
                path,
                source_file,
                admitted_metadata or {},
                evidence_id=evidence_id,
                version_id=version_id,
            )
        if modality == "video":
            return self._process_video(
                path,
                evidence_id=evidence_id,
                version_id=version_id,
                source_sha256=source_sha256,
                source_file=source_file,
                admitted_metadata=admitted_metadata or {},
            )
        raise MediaProcessingError(f"unsupported media modality {modality!r}")

    def _process_image(self, path: Path, **context: Any) -> MediaProcessResult:
        admitted_metadata = context.pop("admitted_metadata")
        if isinstance(self.image_processor, ContextualImageProcessor):
            context["requested_roles"] = admitted_metadata.get("requested_analysis_roles") or ()
        base = MediaObservation(
            observation_id=str(uuid.uuid5(uuid.NAMESPACE_URL, f"{context['source_sha256']}:image-metadata")),
            contract_version=IMAGE_OBSERVATION_CONTRACT,
            observation_type="image_technical_observation",
            confidence=None,
            citation_locator={"source_file": context["source_file"], "scope": "full_image"},
            payload={"source_metadata": admitted_metadata, "preview_source": "retained_evidence"},
        )
        perceptual_hash = image_dhash(path)
        fingerprint = MediaObservation(
            observation_id=str(uuid.uuid5(uuid.NAMESPACE_URL, f"{context['source_sha256']}:image-fingerprint:dhash64")),
            contract_version=IMAGE_FINGERPRINT_CONTRACT,
            observation_type="image_fingerprint_observation",
            confidence=None,
            citation_locator={"source_file": context["source_file"], "scope": "full_image"},
            payload={
                "source_sha256": context["source_sha256"],
                "exact_identity_algorithm": "sha256",
                "perceptual_hash": perceptual_hash,
                "perceptual_hash_algorithm": PERCEPTUAL_HASH_ALGORITHM if perceptual_hash else None,
                "perceptual_hash_bits": 64 if perceptual_hash else None,
                "near_duplicate_semantics": "pixel-layout resemblance; not semantic visual similarity",
            },
            warnings=() if perceptual_hash else ("perceptual hash unavailable because the image could not be decoded by the bounded local decoder",),
        )
        observations = (base, fingerprint)
        if self.image_processor is None:
            return MediaProcessResult(
                modality="image",
                readiness=READY_WITH_WARNINGS,
                processor_id=self.processor_id,
                processor_revision=self.processor_revision,
                observations=observations,
                limitations=(
                    "no approved local vision or ANPR role is configured",
                    *(("near-duplicate fingerprint unavailable",) if perceptual_hash is None else ()),
                ),
                metadata={"result_states": {"image_anpr": "NOT_RUN", "ocr": "NOT_RUN"}},
            )
        result = self.image_processor.process_image(path, **context)
        result_states = dict(result.metadata.get("result_states") or {})
        observed_types = {item.observation_type for item in result.observations}
        if "general_image_ocr_observation" in observed_types:
            result_states["ocr"] = result.metadata.get("result_state", "COMPLETE_RESULTS")
        if isinstance(self.image_processor, FastALPRImageProcessor):
            result_states["image_anpr"] = result.metadata.get("result_state", "COMPLETE_RESULTS" if result.observations else "COMPLETE_ZERO_RESULTS")
        elif isinstance(self.image_processor, TesseractImageOCRProcessor):
            result_states["ocr"] = result.metadata.get("result_state", "COMPLETE_RESULTS" if result.observations else "COMPLETE_ZERO_RESULTS")
        elif isinstance(self.image_processor, CompositeImageProcessor):
            if any(item.observation_type == "anpr_ocr_observation" for item in result.observations):
                result_states["image_anpr"] = "COMPLETE_RESULTS"
            if any(item.observation_type == "general_image_ocr_observation" for item in result.observations):
                result_states["ocr"] = "COMPLETE_RESULTS"
        if "anpr_ocr_observation" not in observed_types and "image_anpr" not in result_states:
            result_states["image_anpr"] = "NOT_RUN"
        if "general_image_ocr_observation" not in observed_types and "ocr" not in result_states:
            result_states["ocr"] = "NOT_RUN"
        return MediaProcessResult(
            modality="image",
            readiness=result.readiness,
            processor_id=result.processor_id,
            processor_revision=result.processor_revision,
            observations=(*observations, *result.observations),
            limitations=result.limitations,
            metadata={**result.metadata, "result_states": result_states},
        )

    def _process_audio(
        self,
        path: Path,
        source_file: str,
        admitted_metadata: dict[str, Any],
        *,
        evidence_id: str,
        version_id: str,
    ) -> MediaProcessResult:
        probe = _ffprobe(path)
        payload = {"source_metadata": admitted_metadata, "container_probe": probe}
        observation = MediaObservation(
            observation_id=str(uuid.uuid5(uuid.NAMESPACE_URL, f"{_sha256(path)}:audio-metadata")),
            contract_version=AUDIO_OBSERVATION_CONTRACT,
            observation_type="audio_technical_observation",
            confidence=None,
            citation_locator={"source_file": source_file, "start_seconds": 0},
            payload=payload,
        )
        observations = [observation]
        limitations = ["diarization unavailable"]
        asr_result_state = "NOT_RUN"
        if self.audio_transcriber is None:
            limitations.append("ASR model is not configured; no transcript or language claim was generated")
        else:
            language = _requested_asr_language(admitted_metadata)
            transcript = self.audio_transcriber.transcribe(
                path,
                evidence_id=evidence_id,
                version_id=version_id,
                source_file=source_file,
                language=language,
            )
            observations.extend(transcript)
            asr_result_state = "COMPLETE_RESULTS" if transcript else "COMPLETE_ZERO_RESULTS"
            if not transcript:
                limitations.append("ASR returned no non-empty transcript segments")
        return MediaProcessResult(
            modality="audio",
            readiness=READY_WITH_WARNINGS,
            processor_id=self.processor_id,
            processor_revision=self.processor_revision,
            observations=tuple(observations),
            limitations=tuple(limitations),
            metadata={"result_states": {"asr": asr_result_state}},
        )

    def _process_video(self, path: Path, **context: Any) -> MediaProcessResult:
        admitted_metadata = context.pop("admitted_metadata")
        source_file = context["source_file"]
        probe = _ffprobe(path)
        observations: list[MediaObservation] = [
            MediaObservation(
                observation_id=str(uuid.uuid5(uuid.NAMESPACE_URL, f"{context['source_sha256']}:video-metadata")),
                contract_version=VIDEO_OBSERVATION_CONTRACT,
                observation_type="video_technical_observation",
                confidence=None,
                citation_locator={"source_file": source_file, "timestamp_seconds": 0},
                payload={"source_metadata": admitted_metadata, "container_probe": probe},
            )
        ]
        limitations: list[str] = []
        asr_result_state = "NOT_RUN"
        video_anpr_result: MediaProcessResult | None = None
        frame_zero_limitations: set[str] = set()
        if self.audio_transcriber is None:
            limitations.append("audio transcription is unavailable without a configured ASR model")
        embedded_audio = _extract_video_audio(path)
        if embedded_audio is not None:
            try:
                audio_result = self._process_audio(
                    embedded_audio,
                    source_file,
                    admitted_metadata,
                    evidence_id=context["evidence_id"],
                    version_id=context["version_id"],
                )
                asr_result_state = str(
                    audio_result.metadata.get("result_states", {}).get("asr") or "NOT_RUN"
                )
                for audio_observation in audio_result.observations:
                    audio_locator = audio_observation.citation_locator
                    observations.append(
                        MediaObservation(
                            observation_id=str(uuid.uuid5(
                                uuid.NAMESPACE_URL,
                                f"{context['source_sha256']}:embedded-audio:{audio_observation.observation_id}",
                            )),
                            contract_version=audio_observation.contract_version,
                            observation_type=f"video_embedded_{audio_observation.observation_type}",
                            confidence=audio_observation.confidence,
                            citation_locator={
                                "source_file": source_file,
                                "start_seconds": audio_locator.get("start_seconds", 0),
                                "end_seconds": audio_locator.get("end_seconds"),
                            },
                            payload={
                                **audio_observation.payload,
                                "parent_evidence_id": context["evidence_id"],
                                "parent_version_id": context["version_id"],
                                "storage_state": "ephemeral_processing_derivative",
                            },
                        )
                    )
            finally:
                embedded_audio.unlink(missing_ok=True)
        else:
            limitations.append("no decodable embedded audio stream was extracted")
        if self.video_anpr_processor is not None:
            video_anpr_result = self.video_anpr_processor.process_video(path, **context)
            observations.extend(video_anpr_result.observations)
            limitations.extend(video_anpr_result.limitations)
        anpr_processor: ImageProcessor | None = None
        secondary_processor: ImageProcessor | None = self.image_processor
        if isinstance(self.image_processor, FastALPRImageProcessor):
            anpr_processor = self.image_processor
            secondary_processor = None
        elif isinstance(self.image_processor, CompositeImageProcessor):
            anpr_roles = tuple(
                processor for processor in self.image_processor.processors
                if isinstance(processor, FastALPRImageProcessor)
            )
            secondary_roles = tuple(
                processor for processor in self.image_processor.processors
                if not isinstance(processor, FastALPRImageProcessor)
            )
            if anpr_roles:
                anpr_processor = anpr_roles[0] if len(anpr_roles) == 1 else CompositeImageProcessor(anpr_roles)
                secondary_processor = (
                    secondary_roles[0]
                    if len(secondary_roles) == 1
                    else CompositeImageProcessor(secondary_roles)
                    if secondary_roles
                    else None
                )
        elif isinstance(self.image_processor, ContextualImageProcessor):
            anpr_processor = self.image_processor.anpr
            secondary_processor = self.image_processor.ocr
        if video_anpr_result is not None:
            # The V2 processor owns plate sampling and grouping. General OCR may
            # still use the secondary cadence, but the legacy frame ANPR path
            # must not duplicate observations or silently change the V2 policy.
            anpr_processor = None
        duration = _video_duration_from_probe(probe)
        if anpr_processor is not None:
            configured_anpr_interval = max(
                1.0, float(os.getenv("FORENSIC_VIDEO_ANPR_SAMPLE_INTERVAL_SECONDS", "1"))
            )
            anpr_max_frames = max(
                1, min(120, int(os.getenv("FORENSIC_VIDEO_ANPR_MAX_SAMPLE_FRAMES", "60")))
            )
            sampling_interval = max(
                configured_anpr_interval,
                duration / anpr_max_frames if duration else configured_anpr_interval,
            )
            sampling_max_frames = anpr_max_frames
            sampling_strategy = "full_duration_bounded_anpr_cadence"
        else:
            sampling_interval = max(
                1.0, float(os.getenv("FORENSIC_VIDEO_SAMPLE_INTERVAL_SECONDS", "5"))
            )
            sampling_max_frames = max(
                1, min(60, int(os.getenv("FORENSIC_VIDEO_MAX_SAMPLE_FRAMES", "12")))
            )
            if duration:
                sampling_interval = max(sampling_interval, duration / sampling_max_frames)
            sampling_strategy = "full_duration_bounded_general_cadence"
        secondary_interval = max(
            sampling_interval,
            float(os.getenv("FORENSIC_VIDEO_SECONDARY_SAMPLE_INTERVAL_SECONDS", "5")),
        )
        frames = (
            []
            if video_anpr_result is not None and secondary_processor is None
            else _sample_video_frames(
                path,
                interval=sampling_interval,
                max_frames=sampling_max_frames,
            )
        )
        if not frames and not (video_anpr_result is not None and secondary_processor is None):
            limitations.append("no frames were sampled; ffmpeg may be unavailable or the video may have no decodable video stream")
        anpr_sampled_count = 0
        secondary_sampled_count = 0
        next_secondary_timestamp = 0.0
        try:
            for frame_path, timestamp in frames:
                frame_sha256 = _sha256(frame_path)
                frame_number = _frame_number_from_probe(probe, timestamp)
                frame_locator = {
                    "source_file": source_file,
                    "timestamp_seconds": timestamp,
                    "frame_number": frame_number,
                    "frame_number_semantics": "estimated_from_best_effort_timestamp_and_declared_fps",
                    "source_frame_sha256": frame_sha256,
                }
                observations.append(
                    MediaObservation(
                        observation_id=str(uuid.uuid5(uuid.NAMESPACE_URL, f"{context['source_sha256']}:frame:{timestamp}:{frame_sha256}")),
                        contract_version=IMAGE_OBSERVATION_CONTRACT,
                        observation_type="video_sampled_frame_observation",
                        confidence=None,
                        citation_locator=frame_locator,
                        payload={
                            "frame_sha256": frame_sha256,
                            "frame_number": frame_number,
                            "frame_number_semantics": "estimated_from_best_effort_timestamp_and_declared_fps",
                            "parent_evidence_id": context["evidence_id"],
                            "parent_version_id": context["version_id"],
                            "sampling_interval_seconds": sampling_interval,
                            "sampling_strategy": sampling_strategy,
                            "storage_state": "ephemeral_processing_derivative",
                        },
                    )
                )
                processors: list[ImageProcessor] = []
                if anpr_processor is not None:
                    processors.append(anpr_processor)
                    anpr_sampled_count += 1
                if secondary_processor is not None and timestamp + 1e-6 >= next_secondary_timestamp:
                    processors.append(secondary_processor)
                    secondary_sampled_count += 1
                    next_secondary_timestamp = timestamp + secondary_interval
                for processor in processors:
                    frame_result = processor.process_image(
                        frame_path,
                        evidence_id=context["evidence_id"],
                        version_id=context["version_id"],
                        source_sha256=context["source_sha256"],
                        source_file=source_file,
                        locator_prefix=frame_locator,
                    )
                    observations.extend(frame_result.observations)
                    for limitation in frame_result.limitations:
                        if limitation in VIDEO_FRAME_ZERO_LIMITATIONS:
                            frame_zero_limitations.add(limitation)
                        else:
                            limitations.append(limitation)
                if not processors and video_anpr_result is None:
                    limitations.append("ANPR analysis is unavailable without a configured image processor")
        finally:
            cleanup_sampled_frames(frames)
        if video_anpr_result is None:
            observations.extend(_video_anpr_plate_groups(observations, context, source_file))
        observed_types = {observation.observation_type for observation in observations}
        for limitation in frame_zero_limitations:
            observation_type, aggregate_limitation = VIDEO_FRAME_ZERO_LIMITATIONS[limitation]
            if observation_type not in observed_types:
                limitations.append(aggregate_limitation)
        return MediaProcessResult(
            modality="video",
            readiness=READY_WITH_WARNINGS,
            processor_id=self.processor_id,
            processor_revision=self.processor_revision,
            observations=tuple(observations),
            limitations=tuple(dict.fromkeys(limitations)),
            metadata={
                "sampled_frame_count": len(frames),
                "anpr_sampled_frame_count": anpr_sampled_count,
                "secondary_sampled_frame_count": secondary_sampled_count,
                "sampling_interval_seconds": sampling_interval,
                "sampling_strategy": sampling_strategy,
                "result_states": {
                    "asr": asr_result_state,
                    "video_anpr": (
                        str(video_anpr_result.metadata.get("result_states", {}).get("video_anpr"))
                        if video_anpr_result is not None
                        else "NOT_RUN"
                        if anpr_processor is None
                        else "COMPLETE_RESULTS"
                        if any(item.observation_type == "anpr_ocr_observation" for item in observations)
                        else "COMPLETE_ZERO_RESULTS"
                    ),
                    "ocr": (
                        "NOT_RUN"
                        if secondary_processor is None
                        else "COMPLETE_RESULTS"
                        if any(item.observation_type == "general_image_ocr_observation" for item in observations)
                        else "COMPLETE_ZERO_RESULTS"
                    ),
                },
                "video_anpr_v2": video_anpr_result.metadata if video_anpr_result is not None else None,
            },
        )


def configured_media_processor() -> DeterministicMediaProcessor:
    image_processors: list[ImageProcessor] = []
    audio_transcriber: AudioTranscriber | None = None
    video_anpr_processor: Any | None = None
    if os.getenv("FORENSIC_ANPR_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:
        image_processors.append(FastALPRImageProcessor.from_environment())
    if os.getenv("FORENSIC_FACE_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:
        image_processors.append(LocalAIFaceProcessor.from_environment())
    if os.getenv("FORENSIC_IMAGE_EMBEDDING_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:
        image_processors.append(SigLIPImageEmbeddingProcessor.from_environment())
    if os.getenv("FORENSIC_OCR_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:
        if os.getenv("FORENSIC_OCR_BACKEND", "tesseract").strip().lower() == "paddle":
            try:
                from ingestion.forensic_records.multilingual_ocr import PaddleMultilingualOCRProcessor
            except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
                from multilingual_ocr import PaddleMultilingualOCRProcessor
            image_processors.append(PaddleMultilingualOCRProcessor.from_environment())
        else:
            image_processors.append(TesseractImageOCRProcessor.from_environment())
    if os.getenv("FORENSIC_ASR_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:
        audio_transcriber = LocalAIASRProcessor.from_environment()
    video_v2_enabled = os.getenv("FORENSIC_VIDEO_ANPR_V2_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}
    video_v3_enabled = os.getenv("FORENSIC_VIDEO_ANPR_V3_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}
    if video_v2_enabled and video_v3_enabled:
        raise ModelUnavailableError("Video ANPR V2 and V3 cannot be enabled together")
    if video_v2_enabled:
        try:
            from ingestion.forensic_records.video_anpr_product_v2 import VideoANPRProductV2Processor
        except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
            from video_anpr_product_v2 import VideoANPRProductV2Processor
        video_anpr_processor = VideoANPRProductV2Processor.from_environment()
    elif video_v3_enabled:
        try:
            from ingestion.forensic_records.video_anpr_onnx_v3 import VideoANPROnnxV3Processor
        except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
            from video_anpr_onnx_v3 import VideoANPROnnxV3Processor
        video_anpr_processor = VideoANPROnnxV3Processor.from_environment()
    image_processor: ImageProcessor | None = None
    anpr_processors = [item for item in image_processors if isinstance(item, FastALPRImageProcessor)]
    ocr_processors = [
        item for item in image_processors
        if isinstance(item, TesseractImageOCRProcessor)
        or getattr(item, "processor_id", "") == "paddleocr-v5-multilingual-cpu"
    ]
    if len(image_processors) == 2 and len(anpr_processors) == 1 and len(ocr_processors) == 1:
        image_processor = ContextualImageProcessor(anpr_processors[0], ocr_processors[0])
    elif len(image_processors) == 1:
        image_processor = image_processors[0]
    elif image_processors:
        image_processor = CompositeImageProcessor(tuple(image_processors))
    return DeterministicMediaProcessor(image_processor, audio_transcriber, video_anpr_processor)


def _requested_asr_language(metadata: dict[str, Any]) -> str | None:
    value = metadata.get("asr_language")
    if value is None or not str(value).strip():
        return None
    language = str(value).strip().lower()
    if not re.fullmatch(r"[a-z]{2,3}", language):
        raise MediaProcessingError("asr_language must be a two- or three-letter lowercase language code")
    return language


def _required_model_path(name: str) -> Path:
    value = os.getenv(name, "").strip()
    if not value:
        raise ModelUnavailableError(f"{name} is required; implicit model download is disabled")
    path = Path(value)
    if not path.is_file():
        raise ModelUnavailableError(f"configured model asset does not exist: {name}")
    return path


def _ocr_script_family(text: str) -> str:
    has_arabic = bool(re.search(r"[\u0600-\u06ff\u0750-\u077f\u08a0-\u08ff]", text))
    has_latin = bool(re.search(r"[A-Za-z]", text))
    if has_arabic and has_latin:
        return "mixed_arabic_latin_candidate"
    if has_arabic:
        return "arabic_script_candidate"
    if has_latin:
        return "latin_script_candidate"
    return "undetermined"


def _onnx_providers() -> list[str]:
    requested = [item.strip() for item in os.getenv(
        "FORENSIC_ANPR_ONNX_PROVIDERS", "OpenVINOExecutionProvider,CPUExecutionProvider"
    ).split(",") if item.strip()]
    return requested or ["CPUExecutionProvider"]


def _ffprobe(path: Path) -> dict[str, Any]:
    executable = shutil.which(os.getenv("FORENSIC_FFPROBE_PATH", "ffprobe"))
    if not executable:
        return {"availability": "unavailable", "reason": "ffprobe is not installed"}
    try:
        completed = subprocess.run(
            [executable, "-v", "error", "-show_format", "-show_streams", "-of", "json", str(path)],
            check=True,
            capture_output=True,
            text=True,
            timeout=max(5, int(os.getenv("FORENSIC_MEDIA_PROBE_TIMEOUT_SECONDS", "20"))),
        )
        result = json.loads(completed.stdout)
    except (OSError, subprocess.SubprocessError, json.JSONDecodeError) as exc:
        raise MediaProcessingError(f"bounded media probe failed: {exc}") from exc
    result["availability"] = "available"
    return result


def _video_duration_from_probe(probe: dict[str, Any]) -> float | None:
    try:
        duration = float((probe.get("format") or {}).get("duration"))
    except (TypeError, ValueError):
        return None
    return duration if duration > 0 else None


def _video_anpr_plate_groups(
    observations: list[MediaObservation],
    context: dict[str, Any],
    source_file: str,
) -> list[MediaObservation]:
    groups: dict[str, list[MediaObservation]] = {}
    for observation in observations:
        if observation.observation_type != "anpr_ocr_observation":
            continue
        normalized = str(observation.payload.get("normalized_plate_text") or "").strip()
        if normalized:
            groups.setdefault(normalized, []).append(observation)
    output: list[MediaObservation] = []
    for plate, sightings in sorted(groups.items()):
        ordered = sorted(
            sightings,
            key=lambda item: float(item.citation_locator.get("timestamp_seconds") or 0),
        )
        best = max(
            ordered,
            key=lambda item: (
                float(item.payload.get("ocr_confidence") or 0),
                float(item.payload.get("detection_confidence") or 0),
            ),
        )
        first_seen = float(ordered[0].citation_locator.get("timestamp_seconds") or 0)
        last_seen = float(ordered[-1].citation_locator.get("timestamp_seconds") or 0)
        group_id = str(uuid.uuid5(
            uuid.NAMESPACE_URL,
            f"{context['source_sha256']}:video-anpr-group:{plate}",
        ))
        output.append(
            MediaObservation(
                observation_id=group_id,
                contract_version=VIDEO_ANPR_GROUP_CONTRACT,
                observation_type="video_anpr_plate_group_observation",
                confidence=best.confidence,
                citation_locator={
                    "source_file": source_file,
                    "first_seen_seconds": first_seen,
                    "last_seen_seconds": last_seen,
                    "best_observation_id": best.observation_id,
                },
                payload={
                    "normalized_plate_text": plate,
                    "sightings_count": len(ordered),
                    "first_seen_seconds": first_seen,
                    "last_seen_seconds": last_seen,
                    "best_observation_id": best.observation_id,
                    "all_observation_ids": [item.observation_id for item in ordered],
                    "grouping_semantics": "same normalized model observation; not object tracking",
                    "parent_evidence_id": context["evidence_id"],
                    "parent_version_id": context["version_id"],
                    "processor": "nexusai-video-anpr-grouping",
                    "processor_revision": "mmv2-v1",
                    "manual_review_required": True,
                },
                warnings=(
                    "Grouping is based on equal normalized OCR output and is not object tracking",
                ),
            )
        )
    return output


def _frame_number_from_probe(probe: dict[str, Any], timestamp: float) -> int | None:
    streams = probe.get("streams") if isinstance(probe, dict) else None
    video = next(
        (stream for stream in streams or [] if isinstance(stream, dict) and stream.get("codec_type") == "video"),
        None,
    )
    if video is None:
        return None
    rate = str(video.get("avg_frame_rate") or video.get("r_frame_rate") or "").split("/", 1)
    try:
        fps = float(rate[0]) / float(rate[1]) if len(rate) == 2 else float(rate[0])
    except (ValueError, ZeroDivisionError):
        return None
    return round(timestamp * fps) if fps > 0 else None


def _ffmpeg_showinfo_timestamps(stderr: str) -> list[float]:
    return [
        float(value)
        for value in re.findall(r"Parsed_showinfo[^\r\n]*\bpts_time:(-?[0-9]+(?:\.[0-9]+)?)", stderr)
    ]


def _sample_video_frames(
    path: Path,
    *,
    interval: float | None = None,
    max_frames: int | None = None,
) -> list[tuple[Path, float]]:
    executable = shutil.which(os.getenv("FORENSIC_FFMPEG_PATH", "ffmpeg"))
    if not executable:
        return []
    max_frames = max(
        1,
        min(
            120,
            max_frames
            if max_frames is not None
            else int(os.getenv("FORENSIC_VIDEO_MAX_SAMPLE_FRAMES", "12")),
        ),
    )
    interval = max(
        1.0,
        interval
        if interval is not None
        else float(os.getenv("FORENSIC_VIDEO_SAMPLE_INTERVAL_SECONDS", "5")),
    )
    retained: list[tuple[Path, float]] = []
    temp_dir = Path(tempfile.mkdtemp(prefix="nexusai-video-frames-"))
    try:
        pattern = temp_dir / "frame-%04d.png"
        # The fps filter rewrites output timestamps and may choose a source
        # frame from the middle of each interval. Multiplying the output index
        # by the interval therefore produced false source citations. select
        # keeps the decoded source timeline, while showinfo exposes the exact
        # best-effort timestamp of every retained frame.
        select_filter = f"select=isnan(prev_selected_t)+gte(t-prev_selected_t\\,{interval}),showinfo"
        completed = subprocess.run(
            [
                executable, "-nostdin", "-v", "info", "-i", str(path),
                "-vf", select_filter, "-frames:v", str(max_frames), "-fps_mode", "vfr", str(pattern),
            ],
            check=True,
            capture_output=True,
            text=True,
            timeout=max(15, int(os.getenv("FORENSIC_VIDEO_PROCESS_TIMEOUT_SECONDS", "120"))),
        )
        timestamps = _ffmpeg_showinfo_timestamps(completed.stderr)
        sampled = sorted(temp_dir.glob("frame-*.png"))
        if len(timestamps) != len(sampled):
            raise MediaProcessingError(
                f"bounded video sampler produced {len(sampled)} frames but {len(timestamps)} source timestamps"
            )
        for frame, timestamp in zip(sampled, timestamps):
            descriptor, persistent_name = tempfile.mkstemp(prefix="nexusai-frame-", suffix=".png")
            os.close(descriptor)
            persistent = Path(persistent_name)
            shutil.copyfile(frame, persistent)
            retained.append((persistent, timestamp))
        return retained
    except Exception as exc:
        cleanup_sampled_frames(retained)
        if isinstance(exc, MediaProcessingError):
            raise
        if not isinstance(exc, (OSError, subprocess.SubprocessError)):
            raise
        raise MediaProcessingError(f"bounded video frame sampling failed: {exc}") from exc
    finally:
        shutil.rmtree(temp_dir, ignore_errors=True)


def _extract_video_audio(path: Path) -> Path | None:
    executable = shutil.which(os.getenv("FORENSIC_FFMPEG_PATH", "ffmpeg"))
    if not executable:
        return None
    descriptor, output_name = tempfile.mkstemp(prefix="nexusai-video-audio-", suffix=".wav")
    os.close(descriptor)
    output = Path(output_name)
    try:
        completed = subprocess.run(
            [
                executable, "-nostdin", "-v", "error", "-i", str(path),
                "-map", "0:a:0?", "-vn", "-acodec", "pcm_s16le", "-y", str(output),
            ],
            check=False,
            capture_output=True,
            timeout=max(15, int(os.getenv("FORENSIC_VIDEO_PROCESS_TIMEOUT_SECONDS", "120"))),
        )
        if completed.returncode != 0 or output.stat().st_size <= 44:
            output.unlink(missing_ok=True)
            return None
        return output
    except (OSError, subprocess.SubprocessError):
        output.unlink(missing_ok=True)
        return None


def cleanup_sampled_frames(frames: list[tuple[Path, float]]) -> None:
    for frame, _ in frames:
        try:
            frame.unlink(missing_ok=True)
        except OSError:
            pass


def _multipart_body(
    boundary: str,
    fields: dict[str, str],
    file_field: str,
    filename: str,
    content_type: str,
    content: bytes,
) -> bytes:
    chunks: list[bytes] = []
    for name, value in fields.items():
        chunks.append(
            f"--{boundary}\r\nContent-Disposition: form-data; name=\"{name}\"\r\n\r\n{value}\r\n".encode()
        )
    safe_filename = Path(filename).name.replace('"', "")
    chunks.append(
        (
            f"--{boundary}\r\nContent-Disposition: form-data; name=\"{file_field}\"; "
            f"filename=\"{safe_filename}\"\r\nContent-Type: {content_type}\r\n\r\n"
        ).encode()
    )
    chunks.extend((content, b"\r\n", f"--{boundary}--\r\n".encode()))
    return b"".join(chunks)


def _asr_seconds(value: Any) -> float | None:
    if isinstance(value, bool):
        return None
    try:
        numeric = float(value)
    except (TypeError, ValueError):
        return None
    if not math.isfinite(numeric) or numeric < 0:
        return None
    # LocalAI whisper.cpp historically returned nanoseconds while OpenAI-style
    # providers return seconds. Preserve the raw value and normalize only when
    # the magnitude is unambiguously not a practical seconds locator.
    return numeric / 1_000_000_000 if numeric > 1_000_000 else numeric


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()
