# SPDX-License-Identifier: Apache-2.0
"""One image OCR role that picks the engine the evidence needs (FORENSIC_OCR_BACKEND=auto).

Measured on generated pages and public receipt photographs (docs/data/media-activation-results-20261021.md):
  * clean printed pages: the PP-OCRv5 ONNX engine and Tesseract `best` in document mode both read identifiers exactly 99.7% of the time; Tesseract in scene mode (the old setting) 79%;
  * photographed receipts: the PP-OCRv5 ONNX engine reads 80% of the words, Tesseract 33%;
  * printed Urdu: only Tesseract's Urdu data reads it; the PP-OCRv5 Arabic recogniser does not.
So: detect the script (Tesseract's orientation and script pass); read Arabic-script pages with Tesseract `urd+eng`; read everything else with the ONNX engine, and when that reading is not
confident (mean line confidence below a threshold, or almost no text) read it again with Tesseract in document mode (`urd+eng` when the script was not clearly Latin, `eng` otherwise) and keep the
reading with the higher mean confidence. When the ONNX
models are not configured the Latin path is Tesseract alone. The route taken, both confidences and the script are written to the result metadata so a reader can see why a page was read the way
it was. No reading is merged with another; one engine's observations are returned whole.
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
        MediaProcessResult,
        MediaProcessingError,
        TesseractImageOCRProcessor,
    )
except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
    from media_pipeline import MediaProcessResult, MediaProcessingError, TesseractImageOCRProcessor


PROCESSOR_ID = "ocr-router-cpu"
PROCESSOR_REVISION = "nexusai-ocr-router-v4"
TESSERACT_LATIN = "tesseract-eng-psm3"
TESSERACT_URDU = "tesseract-urd+eng-psm3"
ONNX_LATIN = "onnx-ppocrv5-latin"


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

    def __init__(self, tesseract_latin: Any, tesseract_urdu: Any | None, onnx_latin: Any | None, script_detector: Any, *, confident: float = 0.75, minimum_characters: int = 12,
                 arabic_share_threshold: float = 0.5) -> None:
        if tesseract_latin is None and onnx_latin is None:
            raise ValueError("at least one Latin reader is required")
        if not 0 < confident <= 1 or minimum_characters < 0:
            raise ValueError("router thresholds are invalid")
        self.tesseract_latin, self.tesseract_urdu, self.onnx_latin, self.script_detector = tesseract_latin, tesseract_urdu, onnx_latin, script_detector
        self.confident, self.minimum_characters = confident, minimum_characters
        self.arabic_share_threshold = arabic_share_threshold

    @classmethod
    def from_environment(cls) -> "OcrRouter":
        import shutil

        executable_value = os.getenv("FORENSIC_OCR_TESSERACT_PATH", "tesseract").strip()
        executable = Path(executable_value)
        if not executable.is_file():
            executable = Path(shutil.which(executable_value) or "")
        tessdata = Path(os.getenv("FORENSIC_OCR_TESSDATA_PATH", "").strip())
        languages = os.getenv("FORENSIC_OCR_LANGUAGES", "eng+urd").split("+")
        latin = TesseractImageOCRProcessor.from_environment(languages="eng", page_segmentation_mode=3)
        urdu = TesseractImageOCRProcessor.from_environment(languages="urd+eng", page_segmentation_mode=3) if "urd" in languages else None
        onnx = None
        if os.getenv("FORENSIC_ONNX_OCR_DETECTOR_PATH", "").strip():
            try:
                from ingestion.forensic_records.onnx_ocr_processor import OnnxPaddleOCRProcessor
            except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
                from onnx_ocr_processor import OnnxPaddleOCRProcessor
            onnx = OnnxPaddleOCRProcessor.from_environment()
        return cls(
            latin, urdu, onnx, lambda image: detect_script(executable, tessdata, image),
            confident=float(os.getenv("FORENSIC_OCR_ROUTER_CONFIDENT", "0.75")),
            minimum_characters=int(os.getenv("FORENSIC_OCR_ROUTER_MIN_CHARACTERS", "12")),
            arabic_share_threshold=float(os.getenv("FORENSIC_OCR_ROUTER_ARABIC_SHARE", "0.5")),
        )

    def page_script(self, path: Path) -> tuple[str | None, float | None]:
        """('Arabic' or 'Latin' or None, Arabic-script share). The ONNX Arabic probe decides when it is configured (Tesseract's own script pass called 15 of 64 generated Urdu pages Japanese or
        Katakana and 7% of receipts Arabic); without it the script pass of Tesseract is used as it is."""
        probe = getattr(self.onnx_latin, "arabic_script_share", None)
        if probe is not None and getattr(self.onnx_latin, "script_probe", None) is not None:
            share = probe(path)
            if share is not None:
                return ("Arabic" if share >= self.arabic_share_threshold else "Latin"), share
        return self.script_detector(path), None

    def process_image(self, path: Path, **context: Any) -> MediaProcessResult:
        script, arabic_share = self.page_script(path)
        arabic = script == "Arabic" and self.tesseract_urdu is not None
        tried: list[str] = []
        notes: list[str] = []
        alternative_confidence = None
        second_engine = None
        if arabic:
            first, first_name = self.tesseract_urdu.process_image(path, **context), TESSERACT_URDU
            notes.append("Arabic-script page: Urdu line order and low-quality Nastaliq are not certified; review required")
        elif self.onnx_latin is not None:
            first, first_name = self.onnx_latin.process_image(path, **context), ONNX_LATIN
            # the second opinion uses the Urdu data when the script is not clearly Latin
            if script != "Latin" and self.tesseract_urdu is not None:
                second_engine, second_name = self.tesseract_urdu, TESSERACT_URDU
            else:
                second_engine, second_name = self.tesseract_latin, TESSERACT_LATIN
        else:
            first, first_name = self.tesseract_latin.process_image(path, **context), TESSERACT_LATIN
        tried.append(first_name)
        confidence, characters = reading_quality(first)
        chosen, used = first, first_name
        if second_engine is not None and (confidence < self.confident or characters < self.minimum_characters):
            try:
                second = second_engine.process_image(path, **context)
            except MediaProcessingError as exc:
                notes.append(f"{second_name} fallback failed: {exc}")
            else:
                tried.append(second_name)
                alternative_confidence, _ = reading_quality(second)
                if second.observations and (not first.observations or alternative_confidence > confidence):
                    chosen, used = second, second_name
        route = {"script_detected": script, "arabic_script_share": None if arabic_share is None else round(arabic_share, 3), "engine_used": used, "engines_tried": tried,
                 "first_confidence": round(confidence, 4), "alternative_confidence": None if alternative_confidence is None else round(alternative_confidence, 4),
                 "confident_threshold": self.confident}
        limitations = tuple(dict.fromkeys([*chosen.limitations, *notes]))
        return replace(chosen, metadata={**chosen.metadata, "ocr_route": route}, limitations=limitations)
