# SPDX-License-Identifier: Apache-2.0
"""One image OCR role that picks the engine the evidence needs (FORENSIC_OCR_BACKEND=auto).

Measured on generated pages and public receipt photographs (docs/data/media-activation-results-20261021.md):
  * clean printed pages: Tesseract `best` in document mode reads identifiers exactly 99.7% of the time; scene mode (the old setting) 79%;
  * photographed receipts: Tesseract reads 33% of the words, the PP-OCRv5 ONNX engine 80%;
  * printed Urdu: only Tesseract's Urdu data reads it; the PP-OCRv5 Arabic recogniser does not.
So: detect the script (Tesseract's orientation and script pass), read Arabic-script pages with `urd+eng`, read everything else with `eng` in document mode, and when that reading is not confident
(mean line confidence below a threshold, or almost no text) read it again with the ONNX engine and keep the reading with the higher mean confidence. The route taken, both confidences and the
script are written to the result metadata so a reader can see why a page was read the way it was. No reading is merged with another; one engine's observations are returned whole.
"""

from __future__ import annotations

import os
import re
import subprocess
from dataclasses import replace
from pathlib import Path
from typing import Any

try:
    from ingestion.forensic_records.media_pipeline import (
        READY,
        READY_WITH_WARNINGS,
        MediaProcessResult,
        MediaProcessingError,
        TesseractImageOCRProcessor,
    )
except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
    from media_pipeline import READY, READY_WITH_WARNINGS, MediaProcessResult, MediaProcessingError, TesseractImageOCRProcessor


PROCESSOR_ID = "ocr-router-cpu"
PROCESSOR_REVISION = "nexusai-ocr-router-v1"


def detect_script(executable: Path, tessdata_path: Path, image: Path, *, timeout: int = 30) -> str | None:
    """The script Tesseract's orientation and script pass names for the image ('Arabic', 'Latin', ...), or None when it has too little text to say."""
    environment = os.environ.copy()
    environment["TESSDATA_PREFIX"] = str(tessdata_path)
    environment["OMP_THREAD_LIMIT"] = "1"
    try:
        done = subprocess.run([str(executable), str(image), "stdout", "--psm", "0", "-l", "osd"], capture_output=True, timeout=timeout, env=environment, check=False)
    except (OSError, subprocess.SubprocessError):
        return None
    text = done.stdout.decode("utf-8", "replace") + done.stderr.decode("utf-8", "replace")
    found = re.search(r"Script: (\w+)", text)
    return found.group(1) if found else None


def reading_quality(result: MediaProcessResult) -> tuple[float, int]:
    """Mean line confidence (0 to 1) and the number of characters read."""
    confidences = [item.confidence for item in result.observations if item.confidence is not None]
    characters = sum(len(str(item.payload.get("raw_text") or "")) for item in result.observations)
    return (sum(confidences) / len(confidences) if confidences else 0.0), characters


class OcrRouter:
    processor_id = PROCESSOR_ID
    processor_revision = PROCESSOR_REVISION

    def __init__(self, latin: Any, urdu: Any | None, fallback: Any | None, script_detector: Any, *, confident: float = 0.88, minimum_characters: int = 20) -> None:
        if not 0 < confident <= 1 or minimum_characters < 0:
            raise ValueError("router thresholds are invalid")
        self.latin, self.urdu, self.fallback, self.script_detector = latin, urdu, fallback, script_detector
        self.confident, self.minimum_characters = confident, minimum_characters

    @classmethod
    def from_environment(cls) -> "OcrRouter":
        executable_value = os.getenv("FORENSIC_OCR_TESSERACT_PATH", "tesseract").strip()
        executable = Path(executable_value)
        if not executable.is_file():
            import shutil
            executable = Path(shutil.which(executable_value) or "")
        tessdata = Path(os.getenv("FORENSIC_OCR_TESSDATA_PATH", "").strip())
        latin = TesseractImageOCRProcessor.from_environment(languages="eng", page_segmentation_mode=3)
        urdu = None
        if "urd" in os.getenv("FORENSIC_OCR_LANGUAGES", "eng+urd").split("+"):
            urdu = TesseractImageOCRProcessor.from_environment(languages="urd+eng", page_segmentation_mode=3)
        fallback = None
        if os.getenv("FORENSIC_ONNX_OCR_DETECTOR_PATH", "").strip():
            try:
                from ingestion.forensic_records.onnx_ocr_processor import OnnxPaddleOCRProcessor
            except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
                from onnx_ocr_processor import OnnxPaddleOCRProcessor
            fallback = OnnxPaddleOCRProcessor.from_environment()
        return cls(
            latin, urdu, fallback, lambda image: detect_script(executable, tessdata, image),
            confident=float(os.getenv("FORENSIC_OCR_ROUTER_CONFIDENT", "0.88")),
            minimum_characters=int(os.getenv("FORENSIC_OCR_ROUTER_MIN_CHARACTERS", "20")),
        )

    def process_image(self, path: Path, **context: Any) -> MediaProcessResult:
        script = self.script_detector(path)
        route, notes = [], []
        arabic = script == "Arabic" and self.urdu is not None
        primary = self.urdu if arabic else self.latin
        first = primary.process_image(path, **context)
        confidence, characters = reading_quality(first)
        route.append("tesseract-urd+eng-psm3" if arabic else "tesseract-eng-psm3")
        chosen, chosen_confidence = first, confidence
        alternative_confidence = None
        if not arabic and self.fallback is not None and (confidence < self.confident or characters < self.minimum_characters):
            try:
                second = self.fallback.process_image(path, **context)
            except MediaProcessingError as exc:
                notes.append(f"onnx fallback failed: {exc}")
            else:
                alternative_confidence, alternative_characters = reading_quality(second)
                route.append("onnx-ppocrv5-latin")
                if second.observations and (not first.observations or alternative_confidence > confidence):
                    chosen, chosen_confidence = second, alternative_confidence
        if arabic:
            notes.append("Arabic-script page: Urdu line order and low-quality Nastaliq are not certified; review required")
        used = route[-1] if chosen is not first else route[0]
        metadata = {**chosen.metadata, "ocr_route": {"script_detected": script, "engine_used": used, "engines_tried": route, "first_confidence": round(confidence, 4),
                                                       "alternative_confidence": None if alternative_confidence is None else round(alternative_confidence, 4),
                                                       "confident_threshold": self.confident}}
        limitations = tuple(dict.fromkeys([*chosen.limitations, *notes]))
        return replace(chosen, metadata=metadata, limitations=limitations)
