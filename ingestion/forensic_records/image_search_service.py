# SPDX-License-Identifier: Apache-2.0
"""The query side of image search: turns a search phrase or an uploaded probe image into the same descriptors the worker stores, for the API to compare inside one case.

An internal service (no published port; the API reaches it on the compose network with a shared key). Two calls:
  POST /embed-text   {"text": "a red car at night"}              -> {"model": ..., "dimension": 768, "vector": [...]}
  POST /embed-image  <image bytes>                               -> {"pdq256_hex": ..., "vectors": {"dinov2": [...], "siglip2": [...]}, "models": {...}}
The models load on the first call and are released after FORENSIC_IMAGE_SEARCH_IDLE_RELEASE_SECONDS without a call. Nothing is stored here and nothing is written to disk except a
short-lived temporary copy of an uploaded probe, removed at once. Run: python image_search_service.py
"""

from __future__ import annotations

import base64
import binascii
import ctypes
import gc
import hmac
import json
import os
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

try:
    from ingestion.forensic_records.image_region import RegionMatcher, RegionUnavailable, crop_region
    from ingestion.forensic_records.image_search import ImageSearchProcessor, SigLIPTextEncoder, bits_to_hex, load_reduced_image, pdq_bits
    from ingestion.forensic_records.media_pipeline import MediaProcessingError, ModelUnavailableError
except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
    from image_region import RegionMatcher, RegionUnavailable, crop_region
    from image_search import ImageSearchProcessor, SigLIPTextEncoder, bits_to_hex, load_reduced_image, pdq_bits
    from media_pipeline import MediaProcessingError, ModelUnavailableError

MAX_BODY_BYTES = 32 * 1024 * 1024


def trim_memory() -> None:
    """Gives freed heap pages back to the operating system. Releasing the models frees Python objects, but glibc keeps the pages, so the process stayed at about 1 GB after release; on a
    small machine that memory is the language model's (measured 2026-11-23: with this service resident the model's weights were squeezed out of RAM and a 30-token answer took 100 s
    instead of 6 s)."""
    gc.collect()
    try:
        ctypes.CDLL("libc.so.6").malloc_trim(0)
    except (OSError, AttributeError):  # not glibc (for example a developer machine): nothing to trim
        pass


class ImageSearchService:
    """The two operations, with the models behind a lock (one encoder at a time on a small machine) and released when idle."""

    def __init__(self, processor: ImageSearchProcessor, text: Any, *, idle_release_seconds: float = 45.0) -> None:
        self.processor, self.text = processor, text
        self.idle_release_seconds = idle_release_seconds
        self._lock = threading.Lock()
        self._timer: threading.Timer | None = None
        self.region = RegionMatcher()  # local features of pictures already seen, in memory only, bounded; independent of the model lock

    def crop(self, body: dict[str, Any]) -> dict[str, Any]:
        """A part (a box in fractions of the displayed picture) cut out of a stored picture, as a lossless PNG: the part the analyst drew a box around."""
        try:
            data = base64.b64decode(str(body.get("b64", "")), validate=True)
        except (binascii.Error, ValueError) as exc:
            raise MediaProcessingError("b64 is not valid base64") from exc
        if not data or len(data) > MAX_BODY_BYTES:
            raise MediaProcessingError("the picture must be between 1 byte and 32 MB")
        try:
            png, width, height = crop_region(data, body.get("box"))
        except RegionUnavailable as exc:
            raise ModelUnavailableError(str(exc)) from exc
        except ValueError as exc:
            raise MediaProcessingError(str(exc)) from exc
        return {"png_b64": base64.b64encode(png).decode("ascii"), "width": width, "height": height}

    def region_match(self, body: dict[str, Any]) -> dict[str, Any]:
        """Does the probe picture appear inside each of the candidate pictures (SIFT matches that agree on one transform), and where. See image_region.py."""
        try:
            probe = base64.b64decode(str(body.get("probe_b64", "")), validate=True)
        except (binascii.Error, ValueError) as exc:
            raise MediaProcessingError("probe_b64 is not valid base64") from exc
        items = body.get("items")
        if not probe or len(probe) > MAX_BODY_BYTES or not isinstance(items, list) or len(items) > 500:
            raise MediaProcessingError("send a probe picture and at most 500 candidate items")
        try:
            return self.region.search(probe, [item for item in items if isinstance(item, dict)])
        except RegionUnavailable as exc:
            raise ModelUnavailableError(str(exc)) from exc
        except ValueError as exc:
            raise MediaProcessingError(str(exc)) from exc

    def _release(self) -> None:
        with self._lock:
            self.processor.release()
            if self.text is not None:
                self.text.release()
            trim_memory()

    def _arm(self) -> None:
        if self.idle_release_seconds <= 0:
            return
        if self._timer is not None:
            self._timer.cancel()
        self._timer = threading.Timer(self.idle_release_seconds, self._release)
        self._timer.daemon = True
        self._timer.start()

    def embed_text(self, text: str) -> dict[str, Any]:
        if self.text is None:
            raise ModelUnavailableError("the text encoder is not configured")
        with self._lock:
            vector = self.text.encode([text])[0]
            self._arm()
        return {"model": self.processor.siglip.model_id, "model_sha256": self.processor.siglip.model_sha256, "dimension": len(vector), "vector": vector}

    def embed_image(self, data: bytes, tiles: bool = False) -> dict[str, Any]:
        """The descriptors of a picture; with tiles=True also the vectors of its windows (what the worker stores at ingest, never asked for a search probe)."""
        if not data or len(data) > MAX_BODY_BYTES:
            raise MediaProcessingError("the image must be between 1 byte and 32 MB")
        with self._lock:
            with tempfile.TemporaryDirectory() as folder:
                path = Path(folder) / "probe"
                path.write_bytes(data)
                image = load_reduced_image(path, max_pixels=self.processor.max_pixels)
            out: dict[str, Any] = {"pdq256_hex": bits_to_hex(pdq_bits(image)), "image_width": image.size[0], "image_height": image.size[1], "vectors": {}, "models": {}}
            for encoder in (self.processor.dino, self.processor.siglip):
                if encoder is None:
                    continue
                out["vectors"][encoder.kind] = encoder.encode(image)
                out["models"][encoder.kind] = {"model": encoder.model_id, "sha256": encoder.model_sha256, "revision": encoder.model_revision}
            self._arm()
        if tiles:
            # the windows take 15 to 25 seconds on this CPU; the model lock is taken for a few windows at a time and given back in between, so that a search (a phrase, an attached picture) is never made to
            # wait for a whole indexing job (measured 2026-11-23: with one lock for the whole job, searches during indexing timed out)
            windows = self.processor.tile_vectors(image, os.getenv("FORENSIC_IMAGE_SEARCH_TILE_SETS", "abc"), hold=self._lock)
            out["tiles"] = [{"id": w["id"], "box": w["box"], "vectors": {kind: [round(value, 6) for value in vector] for kind, vector in w["vectors"].items()}} for w in windows]
            self._arm()
        return out

    @classmethod
    def from_environment(cls) -> "ImageSearchService":
        processor = ImageSearchProcessor.from_environment()
        text = None
        if processor.siglip is not None:
            text = SigLIPTextEncoder(processor.siglip.model_dir, model_sha256=processor.siglip.model_sha256, cpu_threads=processor.siglip.cpu_threads)
        return cls(processor, text, idle_release_seconds=float(os.getenv("FORENSIC_IMAGE_SEARCH_IDLE_RELEASE_SECONDS", "45")))


def make_handler(service: ImageSearchService, key: str) -> type[BaseHTTPRequestHandler]:
    class Handler(BaseHTTPRequestHandler):
        server_version = "image-search/1"

        def log_message(self, fmt: str, *args: Any) -> None:  # request lines carry no phrase and no image; keep the access log quiet
            return

        def _send(self, status: int, body: dict[str, Any]) -> None:
            data = json.dumps(body).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def _authorised(self) -> bool:
            return bool(key) and hmac.compare_digest(self.headers.get("X-Image-Search-Key", ""), key)

        def do_GET(self) -> None:  # noqa: N802
            if self.path == "/healthz":
                self._send(200, {"status": "ok"})
            else:
                self._send(404, {"error": "not found"})

        def do_POST(self) -> None:  # noqa: N802
            if not self._authorised():
                self._send(401, {"error": "unauthorised"})
                return
            try:
                length = int(self.headers.get("Content-Length", "0"))
            except ValueError:
                length = -1
            if length <= 0 or length > MAX_BODY_BYTES:
                self._send(413, {"error": "the body must be between 1 byte and 32 MB"})
                return
            body = self.rfile.read(length)
            route, _, query = self.path.partition("?")
            try:
                if route == "/embed-text":
                    text = json.loads(body.decode("utf-8")).get("text", "")
                    self._send(200, service.embed_text(str(text)))
                elif route == "/embed-image":
                    self._send(200, service.embed_image(body, tiles="tiles=1" in query.split("&")))
                elif route == "/crop":
                    self._send(200, service.crop(json.loads(body.decode("utf-8"))))
                elif route == "/region-match":
                    self._send(200, service.region_match(json.loads(body.decode("utf-8"))))
                else:
                    self._send(404, {"error": "not found"})
            except ModelUnavailableError as exc:
                self._send(503, {"error": str(exc)})
            except (MediaProcessingError, ValueError, UnicodeDecodeError) as exc:
                self._send(400, {"error": str(exc)})

    return Handler


def main() -> None:  # pragma: no cover - process entry point
    key = os.getenv("FORENSIC_IMAGE_SEARCH_SERVICE_KEY", "").strip()
    if not key:
        raise SystemExit("FORENSIC_IMAGE_SEARCH_SERVICE_KEY is required")
    service = ImageSearchService.from_environment()
    ThreadingHTTPServer(("0.0.0.0", int(os.getenv("FORENSIC_IMAGE_SEARCH_PORT", "8097"))), make_handler(service, key)).serve_forever()


if __name__ == "__main__":
    main()
