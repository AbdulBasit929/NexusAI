# SPDX-License-Identifier: Apache-2.0
"""M1 image search: the descriptors that make a whole case searchable by picture and by words.

Plan: docs/roadmap/media-modules-reconciled-plan-20261120.md (M1). Gates: docs/data/image-search-m1-preregistration-20261120.md.

For every image three descriptors are computed from the image reduced to at most 1024 pixels on its long side (the size the bake-off was calibrated on):
  * a 256-bit PDQ-style perceptual hash (own re-implementation of the published algorithm; 'PDQ-style' everywhere; thresholds calibrated here, not borrowed). A match within the calibrated
    distance means "the same image, re-saved, resized or recompressed";
  * a DINOv2-base vector (CLS token, L2 normalised). A match within the calibrated cosine distance means "the same photograph, edited or cropped";
  * a SigLIP 2 base vector (L2 normalised). It answers "photos by words" and "more like this" and is ranked, never presented as an identity.
The vectors are not written into the observation payload: they travel on the observation object (VectorObservation.vector) to the worker, which stores them in forensic.content_embedding
(model, hashes, dimension) so that access to them can be controlled and they leave with no export of the observation. The hash is small and is stored in forensic.image_hash.

Off unless FORENSIC_IMAGE_SEARCH_ENABLED=true. The encoders load on first use and are released by release() (the worker calls it when the queue is idle); nothing is downloaded implicitly.
A match is a candidate for an analyst's review, never a statement that two images are the same evidence, event or truth.
"""

from __future__ import annotations

import gc
import hashlib
import json
import math
import os
import re
import threading
import uuid
from dataclasses import dataclass, replace
from pathlib import Path
from typing import Any

import numpy as np

try:
    from ingestion.forensic_records.media_pipeline import (
        READY,
        READY_WITH_WARNINGS,
        MediaObservation,
        MediaProcessingError,
        MediaProcessResult,
        ModelUnavailableError,
    )
except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
    from media_pipeline import READY, READY_WITH_WARNINGS, MediaObservation, MediaProcessingError, MediaProcessResult, ModelUnavailableError


IMAGE_HASH_CONTRACT = "forensics.image-hash-observation/v1"
IMAGE_SEARCH_VECTOR_CONTRACT = "forensics.image-search-vector/v1"
IMAGE_TILES_CONTRACT = "forensics.image-search-tiles/v1"
TILE_MODEL_SUFFIX = "#tile"  # tile vectors are stored under "<model>#tile" so that no query of the whole-picture vectors can return a tile
TILE_LAYOUT_VERSION = "abc-v1"
# Windows slid over the picture, as fractions of its width and height: (window size, positions of the top-left corner). Set a: halves with a quarter step; b: thirds; c: quarters. A small object or
# a detail of a photograph is then close to the content of at least one window, which a single vector for the whole picture cannot say (multi-scale patch-wise retrieval: Choi et al., 2025).
TILE_SETS = {"a": (0.5, (0.0, 0.25, 0.5)), "b": (1 / 3, (0.0, 1 / 3, 2 / 3)), "c": (0.25, (0.0, 0.25, 0.5, 0.75))}
PDQ_ALGORITHM = "pdq-style-256-nexusai-v1"
MAX_SIDE = 1024
DEFAULT_MAX_PIXELS = 120_000_000


@dataclass(frozen=True)
class VectorObservation(MediaObservation):
    """An observation that carries a descriptor vector next to (not inside) its payload."""

    vector: tuple[float, ...] = ()


@dataclass(frozen=True)
class TileObservation(VectorObservation):
    """One observation for all the windows of a picture. The window vectors travel on the object (like `vector`), never in the payload: tiles = ((tile_id, (x0, y0, x1, y1), {kind: vector}), ...)."""

    tiles: tuple = ()


def tile_layout(sets: str = "abc") -> list[tuple[str, tuple[float, float, float, float]]]:
    """(tile id, box as fractions of the picture) for the chosen sets, in a fixed order."""
    out = []
    for name in sets:
        if name not in TILE_SETS:
            continue
        size, positions = TILE_SETS[name]
        for row, y in enumerate(positions):
            for col, x in enumerate(positions):
                out.append((f"{name}{row}{col}", (round(x, 6), round(y, 6), round(x + size, 6), round(y + size, 6))))
    return out


def tile_boxes_px(width: int, height: int, sets: str = "abc") -> list[tuple[str, tuple[float, float, float, float], tuple[int, int, int, int]]]:
    out = []
    for tile_id, (x0, y0, x1, y1) in tile_layout(sets):
        out.append((tile_id, (x0, y0, x1, y1), (int(round(x0 * width)), int(round(y0 * height)), max(int(round(x1 * width)), int(round(x0 * width)) + 8), max(int(round(y1 * height)), int(round(y0 * height)) + 8))))
    return out


# --------------------------------------------------------------------------------------------- PDQ-style hash


def _box_blur(values: np.ndarray, window: int, axis: int) -> np.ndarray:
    """Mean over a window clipped at the borders (the box-filter step of PDQ's Jarosz filter)."""
    if window <= 1:
        return values
    n = values.shape[axis]
    pad = [(0, 0)] * values.ndim
    pad[axis] = (1, 0)
    csum = np.pad(np.cumsum(values, axis=axis, dtype=np.float64), pad)
    idx = np.arange(n)
    lo = np.clip(idx - window // 2, 0, n)
    hi = np.clip(idx + (window - 1) // 2 + 1, 0, n)
    shape = [1] * values.ndim
    shape[axis] = n
    return ((np.take(csum, hi, axis=axis) - np.take(csum, lo, axis=axis)) / (hi - lo).reshape(shape)).astype(np.float32)


_PDQ_DCT = np.sqrt(2.0 / 64) * np.cos(np.pi / 128 * np.arange(1, 17)[:, None] * (2 * np.arange(64)[None, :] + 1))


def pdq_bits(image: Any) -> np.ndarray:
    """256 bits as uint8 0/1: luma, two box-filter passes, 64x64 decimation, 16x16 DCT without the DC term, median threshold."""
    rgb = np.asarray(image.convert("RGB"), dtype=np.float32)
    luma = 0.299 * rgb[..., 0] + 0.587 * rgb[..., 1] + 0.114 * rgb[..., 2]
    rows, cols = luma.shape
    window = lambda old: (old + 2 * 64 - 1) // (2 * 64)  # noqa: E731
    for _ in range(2):
        luma = _box_blur(luma, window(cols), axis=1)
        luma = _box_blur(luma, window(rows), axis=0)
    small = luma[np.ix_(((np.arange(64) + 0.5) * rows / 64).astype(int), ((np.arange(64) + 0.5) * cols / 64).astype(int))]
    coefficients = _PDQ_DCT @ small @ _PDQ_DCT.T
    return (coefficients > np.median(coefficients)).flatten().astype(np.uint8)


def bits_to_hex(bits: np.ndarray) -> str:
    return np.packbits(bits.astype(np.uint8)).tobytes().hex()


def hex_to_bits(value: str) -> np.ndarray:
    if not re.fullmatch(r"[0-9a-f]{64}", value or ""):
        raise ValueError("a PDQ-style hash is 64 hexadecimal characters")
    return np.unpackbits(np.frombuffer(bytes.fromhex(value), dtype=np.uint8))


def hamming(left: str, right: str) -> int:
    return int((hex_to_bits(left) != hex_to_bits(right)).sum())


def bits_to_sql(bits: np.ndarray) -> str:
    """The text form Postgres reads as a bit(256) value."""
    return "".join("1" if bit else "0" for bit in bits.tolist())


# --------------------------------------------------------------------------------------------- image loading


def load_reduced_image(path: Path, *, max_pixels: int = DEFAULT_MAX_PIXELS) -> Any:
    """Decode, apply the file's orientation tag, convert to RGB and reduce to at most MAX_SIDE pixels on the long side."""
    try:
        from PIL import Image, ImageOps
    except ModuleNotFoundError as exc:  # pragma: no cover
        raise ModelUnavailableError(f"image decoding dependency unavailable: {exc.name}") from exc
    try:
        with Image.open(path) as source:
            width, height = source.size
            if width * height > max_pixels:
                raise MediaProcessingError(f"image has {width * height} pixels, above the configured maximum of {max_pixels}")
            source.load()
            image = ImageOps.exif_transpose(source).convert("RGB")
    except MediaProcessingError:
        raise
    except (OSError, ValueError, SyntaxError, Image.DecompressionBombError) as exc:
        raise MediaProcessingError(f"image could not be decoded: {exc}") from exc
    if max(image.size) > MAX_SIDE:
        image.thumbnail((MAX_SIDE, MAX_SIDE))
    return image


# --------------------------------------------------------------------------------------------- encoders


class VisionEncoder:
    """One vision model, loaded on first use and released on request."""

    def __init__(self, kind: str, model_dir: Path, *, model_id: str, model_revision: str, model_sha256: str, cpu_threads: int = 2) -> None:
        if kind not in {"dinov2", "siglip2"}:
            raise ValueError("kind must be dinov2 or siglip2")
        weights = model_dir / "model.safetensors"
        if not weights.is_file():
            raise ModelUnavailableError(f"{kind} model directory is incomplete: {model_dir}")
        if not re.fullmatch(r"[0-9a-f]{64}", model_sha256.strip().lower()):
            raise ModelUnavailableError(f"the {kind} model SHA-256 must be set to a digest")
        actual = _file_sha256(weights)
        if actual != model_sha256.strip().lower():
            raise ModelUnavailableError(f"the {kind} safetensors checksum does not match the configured one")
        self.kind, self.model_dir, self.model_id, self.model_revision = kind, model_dir, model_id.strip(), model_revision.strip()
        self.model_sha256, self.cpu_threads = actual, max(1, cpu_threads)
        self._model: Any = None
        self._processor: Any = None
        self._torch: Any = None

    @property
    def dimension(self) -> int:
        return 768

    def _load(self) -> None:
        if self._model is not None:
            return
        try:
            import torch  # type: ignore[import-not-found]
            from transformers import AutoImageProcessor  # type: ignore[import-not-found]
        except ModuleNotFoundError as exc:
            raise ModelUnavailableError(f"image search runtime dependency unavailable: {exc.name}") from exc
        torch.set_num_threads(self.cpu_threads)
        self._processor = AutoImageProcessor.from_pretrained(str(self.model_dir), local_files_only=True)
        if self.kind == "dinov2":
            from transformers import AutoModel  # type: ignore[import-not-found]

            self._model = AutoModel.from_pretrained(str(self.model_dir), local_files_only=True, use_safetensors=True).eval()
        else:
            # The vision tower alone: its pooled output is what get_image_features returns, without holding the text tower in memory. The fixed-resolution SigLIP 2 checkpoints load with the SigLIP classes.
            from transformers import SiglipVisionModel  # type: ignore[import-not-found]

            self._model = SiglipVisionModel.from_pretrained(str(self.model_dir), local_files_only=True, use_safetensors=True).eval()
        self._torch = torch

    def encode(self, image: Any) -> list[float]:
        self._load()
        torch = self._torch
        try:
            with torch.inference_mode():
                inputs = self._processor(images=image.convert("RGB"), return_tensors="pt")
                if self.kind == "dinov2":
                    features = self._model(**inputs).last_hidden_state[:, 0]
                else:
                    features = self._model(**inputs).pooler_output
                features = features / features.norm(p=2, dim=-1, keepdim=True).clamp_min(1e-12)
            vector = [float(value) for value in features[0].detach().cpu().tolist()]
        except (RuntimeError, TypeError, ValueError, KeyError) as exc:
            raise MediaProcessingError(f"{self.kind} could not encode the image: {exc}") from exc
        if len(vector) != self.dimension or not all(math.isfinite(value) for value in vector):
            raise MediaProcessingError(f"{self.kind} returned an invalid vector")
        return vector

    def encode_batch(self, images: list[Any], batch: int = 12) -> list[list[float]]:
        """Several images (the windows of one picture) through the model in a few passes."""
        self._load()
        torch = self._torch
        out: list[list[float]] = []
        try:
            for start in range(0, len(images), batch):
                with torch.inference_mode():
                    inputs = self._processor(images=[image.convert("RGB") for image in images[start:start + batch]], return_tensors="pt")
                    features = self._model(**inputs).last_hidden_state[:, 0] if self.kind == "dinov2" else self._model(**inputs).pooler_output
                    features = features / features.norm(p=2, dim=-1, keepdim=True).clamp_min(1e-12)
                out.extend([[float(value) for value in row] for row in features.detach().cpu().tolist()])
        except (RuntimeError, TypeError, ValueError, KeyError) as exc:
            raise MediaProcessingError(f"{self.kind} could not encode the windows: {exc}") from exc
        if any(len(vector) != self.dimension or not all(math.isfinite(value) for value in vector) for vector in out):
            raise MediaProcessingError(f"{self.kind} returned an invalid window vector")
        return out

    def release(self) -> None:
        self._model = self._processor = self._torch = None
        gc.collect()


class SigLIPTextEncoder:
    """The SigLIP 2 text tower used to turn a search phrase into a vector (the query side of 'photos by words')."""

    def __init__(self, model_dir: Path, *, model_sha256: str, cpu_threads: int = 2) -> None:
        weights = model_dir / "model.safetensors"
        if not weights.is_file():
            raise ModelUnavailableError(f"SigLIP 2 model directory is incomplete: {model_dir}")
        actual = _file_sha256(weights)
        if actual != model_sha256.strip().lower():
            raise ModelUnavailableError("the SigLIP 2 safetensors checksum does not match the configured one")
        self.model_dir, self.model_sha256, self.cpu_threads = model_dir, actual, max(1, cpu_threads)
        self._model: Any = None
        self._tokenizer: Any = None
        self._torch: Any = None

    def _load(self) -> None:
        if self._model is not None:
            return
        try:
            import torch  # type: ignore[import-not-found]
            from transformers import AutoTokenizer, SiglipTextModel  # type: ignore[import-not-found]
        except ModuleNotFoundError as exc:
            raise ModelUnavailableError(f"image search runtime dependency unavailable: {exc.name}") from exc
        torch.set_num_threads(self.cpu_threads)
        self._tokenizer = AutoTokenizer.from_pretrained(str(self.model_dir), local_files_only=True)
        self._model = SiglipTextModel.from_pretrained(str(self.model_dir), local_files_only=True, use_safetensors=True).eval()
        self._torch = torch

    def encode(self, texts: list[str]) -> list[list[float]]:
        cleaned = [text.strip().lower() for text in texts]
        if not cleaned or any(not text or len(text) > 512 for text in cleaned):
            raise MediaProcessingError("search phrases must contain 1 to 512 characters")
        self._load()
        torch = self._torch
        batch = self._tokenizer(cleaned, padding="max_length", max_length=64, truncation=True, return_tensors="pt")
        with torch.inference_mode():
            features = self._model(input_ids=batch["input_ids"]).pooler_output
            features = features / features.norm(p=2, dim=-1, keepdim=True).clamp_min(1e-12)
        return [[float(value) for value in row] for row in features.detach().cpu().tolist()]

    def release(self) -> None:
        self._model = self._tokenizer = self._torch = None
        gc.collect()


def _file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


# --------------------------------------------------------------------------------------------- the processor


class ImageSearchProcessor:
    """Hash + DINOv2 + SigLIP 2 for one image, as three observations."""

    processor_id = "image-search-cpu"
    processor_revision = "nexusai-image-search-m1-v1"

    def __init__(self, *, dino: Any | None, siglip: Any | None, max_input_bytes: int = 64 * 1024 * 1024, max_pixels: int = DEFAULT_MAX_PIXELS) -> None:
        if dino is None and siglip is None:
            raise ModelUnavailableError("at least one image-search encoder must be configured")
        if max_input_bytes <= 0 or max_pixels <= 0:
            raise ValueError("image search bounds must be positive")
        self.dino, self.siglip = dino, siglip
        self.max_input_bytes, self.max_pixels = max_input_bytes, max_pixels

    @classmethod
    def from_environment(cls) -> "ImageSearchProcessor":
        threads = max(1, int(os.getenv("FORENSIC_IMAGE_SEARCH_CPU_THREADS", "2")))

        def encoder(kind: str, prefix: str, default_id: str) -> VisionEncoder | None:
            configured = os.getenv(f"{prefix}_MODEL_PATH", "").strip()
            if not configured:
                return None
            return VisionEncoder(
                kind,
                Path(configured),
                model_id=os.getenv(f"{prefix}_MODEL", default_id),
                model_revision=os.getenv(f"{prefix}_MODEL_REVISION", ""),
                model_sha256=os.getenv(f"{prefix}_MODEL_SHA256", ""),
                cpu_threads=threads,
            )

        return cls(
            dino=encoder("dinov2", "FORENSIC_IMAGE_SEARCH_DINO", "facebook/dinov2-base"),
            siglip=encoder("siglip2", "FORENSIC_IMAGE_SEARCH_SIGLIP", "google/siglip2-base-patch16-224"),
            max_input_bytes=max(1, int(os.getenv("FORENSIC_IMAGE_SEARCH_MAX_INPUT_BYTES", str(64 * 1024 * 1024)))),
            max_pixels=max(1, int(os.getenv("FORENSIC_IMAGE_SEARCH_MAX_PIXELS", str(DEFAULT_MAX_PIXELS)))),
        )

    def release(self) -> None:
        for encoder in (self.dino, self.siglip):
            if encoder is not None:
                encoder.release()

    def tile_vectors(self, image: Any, sets: str = "abc", *, hold: Any = None, chunk: int = 6) -> list[dict[str, Any]]:
        """The vectors of every window of a (reduced) picture, one entry per window: {"id", "box", "vectors": {kind: vector}}.

        `hold` is a lock taken around each small chunk of windows only (never the whole job), so that other work that needs the same models can run in between."""
        import contextlib
        import time

        boxes = tile_boxes_px(image.size[0], image.size[1], sets)
        crops = [image.crop(px) for _, _, px in boxes]
        per_kind: dict[str, list[list[float]]] = {}
        for encoder in (self.dino, self.siglip):
            if encoder is None:
                continue
            vectors: list[list[float]] = []
            for start in range(0, len(crops), chunk):
                with (hold if hold is not None else contextlib.nullcontext()):
                    vectors.extend(encoder.encode_batch(crops[start:start + chunk], batch=chunk))
                time.sleep(0.01)  # lets a waiting search take the lock
            per_kind[encoder.kind] = vectors
        return [{"id": tile_id, "box": list(box), "vectors": {kind: vectors[i] for kind, vectors in per_kind.items()}} for i, (tile_id, box, _) in enumerate(boxes)]

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
            raise MediaProcessingError(f"image search input exceeds the configured maximum of {self.max_input_bytes} bytes")
        image = load_reduced_image(path, max_pixels=self.max_pixels)
        encodings = []
        pdq_hex = bits_to_hex(pdq_bits(image))
        for encoder, role in ((self.dino, "near_duplicate"), (self.siglip, "semantic")):
            if encoder is not None:
                encodings.append({"kind": encoder.kind, "role": role, "semantics": ROLE_SEMANTICS[role], "model_id": encoder.model_id, "revision": encoder.model_revision,
                                  "sha256": encoder.model_sha256, "vector": encoder.encode(image)})
        return build_search_result(
            evidence_id=evidence_id, version_id=version_id, source_sha256=source_sha256, source_file=source_file, locator_prefix=locator_prefix,
            pdq_hex=pdq_hex, width=image.size[0], height=image.size[1], encodings=encodings,
            processor_id=self.processor_id, processor_revision=self.processor_revision,
        )


ROLE_SEMANTICS = {
    "near_duplicate": "the same photograph, re-compressed or lightly edited; not evidence identity or fact",
    "semantic": "candidate visual or textual similarity; ranked, not evidence identity or fact",
}
KIND_ROLE = {"dinov2": "near_duplicate", "siglip2": "semantic"}


def build_search_result(
    *,
    evidence_id: str,
    version_id: str,
    source_sha256: str,
    source_file: str,
    locator_prefix: dict[str, Any] | None,
    pdq_hex: str,
    width: int,
    height: int,
    encodings: list[dict[str, Any]],
    processor_id: str,
    processor_revision: str,
    tiles: list[dict[str, Any]] | None = None,
) -> MediaProcessResult:
    """The observations of one image from its descriptors. Shared by the local and the remote processor, so that both write exactly the same rows."""
    locator_identity = json.dumps(locator_prefix or {}, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    locator = dict(locator_prefix or {})
    locator.update({"source_file": source_file, "scope": "full_image"})
    base_payload = {
        "parent_evidence_id": evidence_id,
        "parent_version_id": version_id,
        "source_sha256": source_sha256,
        "frame_timestamp_seconds": (locator_prefix or {}).get("timestamp_seconds"),
        "descriptor_input": f"image decoded with its orientation tag applied, reduced to at most {MAX_SIDE} px on the long side",
        "processor": processor_id,
        "processor_revision": processor_revision,
    }
    observations: list[MediaObservation] = [
        VectorObservation(
            observation_id=str(uuid.uuid5(uuid.NAMESPACE_URL, f"{source_sha256}:{locator_identity}:pdq:{PDQ_ALGORITHM}")),
            contract_version=IMAGE_HASH_CONTRACT,
            observation_type="image_hash_observation",
            confidence=None,
            citation_locator=locator,
            payload={
                **base_payload,
                "hash_algorithm": PDQ_ALGORITHM,
                "pdq256_hex": pdq_hex,
                "image_width": width,
                "image_height": height,
                "similarity_semantics": "pixel-layout resemblance: the same image re-saved, resized or recompressed; not semantic similarity",
            },
            warnings=("A perceptual hash match is a candidate for review and does not establish that two files are the same evidence",),
        )
    ]
    for item in encodings:
        vector = item["vector"]
        observations.append(
            VectorObservation(
                observation_id=str(uuid.uuid5(uuid.NAMESPACE_URL, f"{source_sha256}:{locator_identity}:{item['kind']}:{item['revision']}:{item['sha256']}")),
                contract_version=IMAGE_SEARCH_VECTOR_CONTRACT,
                observation_type="image_search_vector_observation",
                confidence=None,
                citation_locator=locator,
                payload={
                    **base_payload,
                    "embedding_role": item["role"],
                    "embedding_model": item["model_id"],
                    "embedding_revision": item["revision"],
                    "model_sha256": item["sha256"],
                    "embedding_dimension": len(vector),
                    "embedding_normalization": "l2",
                    "vector_storage": "forensic.content_embedding",
                    "review_state": "model_candidate",
                    "similarity_semantics": item["semantics"],
                },
                warnings=("Similarity is a model observation and requires analyst review",),
                vector=tuple(float(value) for value in vector),
            )
        )
    if tiles:
        observations.append(
            TileObservation(
                observation_id=str(uuid.uuid5(uuid.NAMESPACE_URL, f"{source_sha256}:{locator_identity}:tiles:{TILE_LAYOUT_VERSION}")),
                contract_version=IMAGE_TILES_CONTRACT,
                observation_type="image_search_tiles_observation",
                confidence=None,
                citation_locator={**locator, "scope": "windows"},
                payload={
                    **base_payload,
                    "tile_layout": TILE_LAYOUT_VERSION,
                    "tile_count": len(tiles),
                    "tile_ids": [tile["id"] for tile in tiles],
                    "models": {item["kind"]: {"model": item["model_id"] + TILE_MODEL_SUFFIX, "sha256": item["sha256"], "revision": item["revision"]} for item in encodings},
                    "vector_storage": "forensic.content_embedding",
                    "review_state": "model_candidate",
                    "similarity_semantics": "candidate similarity of a window of the picture (a region or a small object); ranked, not evidence identity or fact",
                },
                warnings=("Window similarity is a model observation and requires analyst review",),
                tiles=tuple((tile["id"], tuple(tile["box"]), dict(tile["vectors"])) for tile in tiles),
            )
        )
    return MediaProcessResult(
        modality="image",
        readiness=READY,
        processor_id=processor_id,
        processor_revision=processor_revision,
        observations=tuple(observations),
        limitations=("Image search descriptors are model observations of the image reduced to at most 1024 px; a match is a candidate for review, and a picture inside a full-screen screenshot is not matched",),
        metadata={
            "descriptor_count": len(observations),
            "models": {item["kind"]: {"model": item["model_id"], "sha256": item["sha256"]} for item in encodings},
        },
    )


class RemoteImageSearchProcessor:
    """The same descriptors, computed by the image-search service in its own container (own memory cap, models released when idle), so the worker does not hold a gigabyte of models next to its OCR role.

    It sends the image bytes to POST /embed-image and builds the same observations from the answer. Any failure (service down, timeout, bad answer) raises MediaProcessingError; the wrapper turns that
    into a limitation and a warning, never a failed job.
    """

    processor_id = ImageSearchProcessor.processor_id
    processor_revision = ImageSearchProcessor.processor_revision

    def __init__(self, url: str, key: str, *, timeout_seconds: float = 180.0, max_input_bytes: int = 32 * 1024 * 1024, tiles: bool = False) -> None:
        if not url.strip() or not key.strip():
            raise ModelUnavailableError("FORENSIC_IMAGE_SEARCH_URL and FORENSIC_IMAGE_SEARCH_SERVICE_KEY are both required for the remote image search")
        self.url, self.key = url.strip().rstrip("/"), key.strip()
        self.timeout_seconds, self.max_input_bytes, self.tiles = timeout_seconds, max_input_bytes, tiles

    @classmethod
    def from_environment(cls) -> "RemoteImageSearchProcessor":
        return cls(
            os.getenv("FORENSIC_IMAGE_SEARCH_URL", ""),
            os.getenv("FORENSIC_IMAGE_SEARCH_SERVICE_KEY", ""),
            timeout_seconds=float(os.getenv("FORENSIC_IMAGE_SEARCH_TIMEOUT_SECONDS", "180")),
            max_input_bytes=min(32 * 1024 * 1024, max(1, int(os.getenv("FORENSIC_IMAGE_SEARCH_MAX_INPUT_BYTES", str(32 * 1024 * 1024))))),
            tiles=os.getenv("FORENSIC_IMAGE_SEARCH_TILES", "off").strip().lower() in {"1", "true", "on", "yes"},
        )

    def release(self) -> None:  # the service releases its own models when idle
        return None

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
        import urllib.error
        import urllib.request

        if path.stat().st_size > self.max_input_bytes:
            raise MediaProcessingError(f"image search input exceeds the maximum of {self.max_input_bytes} bytes")
        request = urllib.request.Request(
            f"{self.url}/embed-image" + ("?tiles=1" if self.tiles else ""), data=path.read_bytes(), method="POST",
            headers={"X-Image-Search-Key": self.key, "Content-Type": "application/octet-stream"},
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                answer = json.loads(response.read().decode("utf-8"))
        except urllib.error.HTTPError as exc:
            detail = ""
            try:
                detail = json.loads(exc.read().decode("utf-8")).get("error", "")
            except Exception:  # noqa: BLE001
                pass
            raise MediaProcessingError(f"the image search service answered HTTP {exc.code}: {detail}") from exc
        except (urllib.error.URLError, TimeoutError, OSError, json.JSONDecodeError) as exc:
            raise MediaProcessingError(f"the image search service is not reachable: {exc}") from exc
        try:
            encodings = []
            for kind, vector in answer["vectors"].items():
                info = answer["models"][kind]
                values = [float(value) for value in vector]
                if not values or not all(math.isfinite(value) for value in values):
                    raise ValueError("empty or invalid vector")
                encodings.append({"kind": kind, "role": KIND_ROLE[kind], "semantics": ROLE_SEMANTICS[KIND_ROLE[kind]], "model_id": info["model"],
                                  "revision": info.get("revision", ""), "sha256": info["sha256"], "vector": values})
            if not encodings:
                raise ValueError("no vectors")
            return build_search_result(
                evidence_id=evidence_id, version_id=version_id, source_sha256=source_sha256, source_file=source_file, locator_prefix=locator_prefix,
                pdq_hex=answer["pdq256_hex"], width=int(answer["image_width"]), height=int(answer["image_height"]), encodings=encodings,
                processor_id=self.processor_id, processor_revision=self.processor_revision, tiles=_clean_tiles(answer.get("tiles")),
            )
        except (KeyError, TypeError, ValueError) as exc:
            raise MediaProcessingError(f"the image search service returned an unusable answer: {exc}") from exc


def search_processor_from_environment() -> Any:
    """Remote when FORENSIC_IMAGE_SEARCH_URL is set (the deployed shape), otherwise the models are loaded in this process."""
    if os.getenv("FORENSIC_IMAGE_SEARCH_URL", "").strip():
        return RemoteImageSearchProcessor.from_environment()
    return ImageSearchProcessor.from_environment()


# --------------------------------------------------------------------------------------------- wiring into the media worker


class ImageSearchAugmentedProcessor:
    """The configured media processor plus the image-search descriptors for still images.

    It wraps (never replaces) the existing processor, so ANPR, OCR and the fingerprint keep their own routing and states. A failure of the descriptors adds a limitation and a warning to the result;
    it never fails the job. The encoders are released after FORENSIC_IMAGE_SEARCH_IDLE_RELEASE_SECONDS without an image (default 120), so the worker's memory returns to its idle level between batches.
    """

    def __init__(self, inner: Any, search: ImageSearchProcessor, *, idle_release_seconds: float | None = None) -> None:
        self.inner, self.search = inner, search
        self.idle_release_seconds = float(os.getenv("FORENSIC_IMAGE_SEARCH_IDLE_RELEASE_SECONDS", "120")) if idle_release_seconds is None else idle_release_seconds
        self._lock = threading.Lock()
        self._timer: threading.Timer | None = None

    def __getattr__(self, name: str) -> Any:  # the wrapped processor's attributes (processor_id and the like) stay reachable
        return getattr(self.inner, name)

    def _arm_release(self) -> None:
        if self.idle_release_seconds <= 0:
            return
        with self._lock:
            if self._timer is not None:
                self._timer.cancel()
            self._timer = threading.Timer(self.idle_release_seconds, self.search.release)
            self._timer.daemon = True
            self._timer.start()

    def process(self, path: Path, *, modality: str, **context: Any) -> MediaProcessResult:
        result = self.inner.process(path, modality=modality, **context)
        if modality != "image":
            return result
        try:
            extra = self.search.process_image(
                path,
                evidence_id=context["evidence_id"],
                version_id=context["version_id"],
                source_sha256=context["source_sha256"],
                source_file=context["source_file"],
            )
        except Exception as exc:  # noqa: BLE001 - an optional role must never take the OCR or plate result of the same image down with it
            return replace(
                result,
                readiness=READY_WITH_WARNINGS,
                limitations=(*result.limitations, f"image search descriptors unavailable: {type(exc).__name__}: {exc}"),
            )
        finally:
            self._arm_release()
        return replace(
            result,
            observations=(*result.observations, *extra.observations),
            limitations=tuple(dict.fromkeys([*result.limitations, *extra.limitations])),
            metadata={**result.metadata, "image_search": extra.metadata},
        )


IMAGE_SEARCH_CONTRACTS = frozenset({IMAGE_HASH_CONTRACT, IMAGE_SEARCH_VECTOR_CONTRACT, IMAGE_TILES_CONTRACT})


def _clean_tiles(raw: Any) -> list[dict[str, Any]] | None:
    """The windows of a service answer, checked: every vector finite and the right length. None when the answer has none."""
    if not raw:
        return None
    out = []
    for tile in raw:
        vectors = {}
        for kind, vector in tile["vectors"].items():
            values = [float(value) for value in vector]
            if len(values) != 768 or not all(math.isfinite(value) for value in values):
                raise ValueError("invalid window vector")
            vectors[kind] = values
        box = [float(value) for value in tile["box"]]
        if len(box) != 4 or not (0 <= box[0] < box[2] <= 1.0001 and 0 <= box[1] < box[3] <= 1.0001):
            raise ValueError("invalid window box")
        out.append({"id": str(tile["id"]), "box": box, "vectors": vectors})
    return out


def _chunk_name(locator: dict[str, Any]) -> str:
    stamp = locator.get("timestamp_seconds")
    return "full" if stamp is None else f"frame:{float(stamp):.3f}"


def vector_literal(vector: Any) -> str:
    return "[" + ",".join(format(float(value), ".8g") for value in vector) + "]"


def persist_image_search_rows(cur: Any, job: Any, version_id: str, observation: Any, artifact_id: Any) -> None:
    """Write the hash row or the vector row of one observation, in the transaction that wrote its derived artifact (so both exist or neither does)."""
    payload = observation.payload
    chunk_ref = f"v:{version_id}:{_chunk_name(observation.citation_locator)}"
    if observation.contract_version == IMAGE_TILES_CONTRACT:
        # one row per window and model, under the model name with the tile suffix, chunk_ref v:<version>:tile:<id>; the window box is in the locator
        models = payload["models"]
        for tile_id, box, vectors in getattr(observation, "tiles", ()):
            for kind, vector in vectors.items():
                info = models[kind]
                cur.execute(
                    """
                    INSERT INTO forensic.content_embedding (
                      tenant_id, collection_id, evidence_id, derived_artifact_id, modality, chunk_ref, locator,
                      content_sha256, model_id, model_sha256, dim, embedding
                    ) VALUES (%s, %s, %s::uuid, %s::uuid, 'image', %s, %s::jsonb, %s, %s, %s, %s, %s::vector)
                    ON CONFLICT (tenant_id, collection_id, evidence_id, model_id, modality, chunk_ref) DO NOTHING
                    """,
                    (
                        job.tenant_id, job.collection_id, job.evidence_id, artifact_id, f"v:{version_id}:tile:{tile_id}",
                        json.dumps({"tile": tile_id, "box": list(box), "layout": payload.get("tile_layout")}), payload.get("source_sha256"),
                        info["model"], info["sha256"], len(vector), vector_literal(vector),
                    ),
                )
        return
    if observation.contract_version == IMAGE_HASH_CONTRACT:
        cur.execute(
            """
            INSERT INTO forensic.image_hash (
              tenant_id, collection_id, evidence_id, version_id, derived_artifact_id, chunk_ref,
              algorithm, pdq256, image_width, image_height, content_sha256
            ) VALUES (%s, %s, %s::uuid, %s::uuid, %s::uuid, %s, %s, %s::bit(256), %s, %s, %s)
            ON CONFLICT (tenant_id, collection_id, evidence_id, version_id, algorithm, chunk_ref) DO NOTHING
            """,
            (
                job.tenant_id, job.collection_id, job.evidence_id, version_id, artifact_id, chunk_ref,
                payload["hash_algorithm"], bits_to_sql(hex_to_bits(payload["pdq256_hex"])),
                payload.get("image_width"), payload.get("image_height"), payload.get("source_sha256"),
            ),
        )
    elif observation.contract_version == IMAGE_SEARCH_VECTOR_CONTRACT:
        vector = getattr(observation, "vector", ())
        if not vector or len(vector) != int(payload["embedding_dimension"]):
            raise MediaProcessingError("image search vector is missing or has the wrong length")
        cur.execute(
            """
            INSERT INTO forensic.content_embedding (
              tenant_id, collection_id, evidence_id, derived_artifact_id, modality, chunk_ref, locator,
              content_sha256, model_id, model_sha256, dim, embedding
            ) VALUES (%s, %s, %s::uuid, %s::uuid, 'image', %s, %s::jsonb, %s, %s, %s, %s, %s::vector)
            ON CONFLICT (tenant_id, collection_id, evidence_id, model_id, modality, chunk_ref) DO NOTHING
            """,
            (
                job.tenant_id, job.collection_id, job.evidence_id, artifact_id, chunk_ref,
                json.dumps(observation.citation_locator), payload.get("source_sha256"),
                payload["embedding_model"], payload["model_sha256"], len(vector), vector_literal(vector),
            ),
        )
