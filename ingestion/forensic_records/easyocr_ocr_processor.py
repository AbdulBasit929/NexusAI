# SPDX-License-Identifier: Apache-2.0
"""Urdu and mixed Urdu-and-English image OCR backed by EasyOCR 1.7.2 (Reader(['ur', 'en']): CRAFT detector, Arabic-script recogniser that also reads Latin letters).

Measured in the OCR bake-off v2 (docs/data/ocr-bakeoff-v2-preregistration-20261117.md, results ocr-bakeoff-v2-results-20261117.md). It is one engine of the routed OCR role (ocr_router.py) and is off
unless FORENSIC_OCR_EASYOCR_ENABLED=true. Every model file is a pinned file with a pinned SHA-256 in FORENSIC_EASYOCR_MODEL_DIR; nothing is downloaded (download_enabled=False).

One observation per assembled line, with the same observation contract as the PP-OCRv5 engine. The line assembly is the rule the bake-off measured (plan section 6 and amendment A1), the same code path
for every image: boxes are grouped into lines by vertical overlap (at least 40% of the smaller height), lines run top to bottom, boxes inside a line run right to left when more than half of the line's
letters are Arabic-script and left to right otherwise, boxes are joined by one space. Nothing else is done to the text.
"""

from __future__ import annotations

import hashlib
import math
import os
import re
import statistics
import uuid
from pathlib import Path
from typing import Any

try:
    import cv2
except ImportError:  # pragma: no cover - the worker image has OpenCV; the unit tests inject a stand-in
    cv2 = None

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
    from ingestion.forensic_records.multilingual_ocr import _identifiers, _normalize_text, _script_and_direction
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
    from multilingual_ocr import _identifiers, _normalize_text, _script_and_direction


PROCESSOR_ID = "easyocr-cpu"
PROCESSOR_REVISION = "nexusai-easyocr-v1"
DETECTOR_ID = "easyocr_craft_mlt_25k"
RECOGNIZER_ID = "easyocr_arabic_g1_ur_en"
DETECTOR_FILE = "craft_mlt_25k.pth"
RECOGNIZER_FILE = "arabic.pth"
LANGUAGES = ("ur", "en")
OVERLAP = 0.4  # amendment A1 of the bake-off plan: a box joins a line when the vertical extents overlap by at least this share of the smaller height
# Arabic, Arabic Supplement, and the two presentation-form blocks (the same set the bake-off lab used)
_ARABIC_LETTER = re.compile("[؀-ۿݐ-ݿﭐ-﷿ﹰ-﻿]")
_LETTER = re.compile(r"[^\W\d_]", re.UNICODE)
WEIGHTS_LICENCE_STATUS = "unverified_evaluation_only"  # ledger MOD-047 to MOD-049 until the licence request is answered or our own recogniser replaces the files


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def _verified(path: Path, hash_variable: str) -> str:
    """The SHA-256 of a model file after checking that it exists and matches the pinned value in the environment."""
    if not path.is_file():
        raise ModelUnavailableError(f"configured EasyOCR model asset does not exist: {path.name}")
    expected = os.getenv(hash_variable, "").strip().lower()
    actual = _sha256(path)
    if not re.fullmatch(r"[0-9a-f]{64}", expected) or actual != expected:
        raise ModelUnavailableError(f"{hash_variable} is absent or does not match the file")
    return actual


def assemble_lines(results: list[Any]) -> list[dict[str, Any]]:
    """EasyOCR's [(box, text, confidence), ...] -> lines, top to bottom: {"text", "confidence" (mean of the boxes), "box" (x1, y1, x2, y2), "boxes" (count)}.
    The same rule as tools/media_data/easyocr_reader.py, which produced the bake-off numbers."""
    items = []
    for index, (box, text, confidence) in enumerate(results):
        if not str(text).strip():
            continue
        ys = [float(p[1]) for p in box]
        xs = [float(p[0]) for p in box]
        items.append({"cy": (min(ys) + max(ys)) / 2.0, "cx": (min(xs) + max(xs)) / 2.0, "h": max(ys) - min(ys), "text": str(text), "conf": float(confidence),
                      "x1": min(xs), "x2": max(xs), "y1": min(ys), "y2": max(ys), "i": index})
    lines: list[list[dict[str, Any]]] = []
    for item in sorted(items, key=lambda i: i["cy"]):
        item["top"], item["bottom"] = item["cy"] - item["h"] / 2.0, item["cy"] + item["h"] / 2.0
        if lines:
            members = lines[-1]
            top, bottom = statistics.mean(m["top"] for m in members), statistics.mean(m["bottom"] for m in members)
            overlap = min(item["bottom"], bottom) - max(item["top"], top)
            if overlap >= OVERLAP * min(item["h"], bottom - top):
                members.append(item)
                continue
        lines.append([item])
    out = []
    for line in lines:
        letters = "".join(i["text"] for i in line)
        arabic, total = len(_ARABIC_LETTER.findall(letters)), len(_LETTER.findall(letters))
        right_to_left = total > 0 and arabic * 2 > total
        ordered = sorted(line, key=lambda i: i["cx"], reverse=right_to_left)
        out.append({
            "text": " ".join(i["text"] for i in ordered),
            "confidence": sum(i["conf"] for i in line) / len(line),
            "box": (min(i["x1"] for i in line), min(i["y1"] for i in line), max(i["x2"] for i in line), max(i["y2"] for i in line)),
            "boxes": len(line),
            "members": sorted(i["i"] for i in line),
        })
    return out


def has_arabic_script(text: str) -> bool:
    return bool(_ARABIC_LETTER.search(text))


def reread_latin_boxes(raw: list[Any], frame: Any, recognizer: Any, *, scale: float = 1.0, mode: str = "all", margin: int = 2) -> tuple[list[Any], list[dict[str, Any]]]:
    """Read again, with the PP-OCRv5 Latin recogniser, every EasyOCR box whose text has no Arabic-script character and at least one letter or digit.
    The box is cropped from the ORIGINAL image (coordinates divided by the size-cap scale). mode "all": the Latin text replaces the box text when it is not empty; mode "confidence": only when its
    mean confidence is higher. Returns (results with the replacements, audit entries {box, easyocr_text, latin_text, easyocr_confidence, latin_confidence, used}). A recogniser error or an empty
    reading leaves the EasyOCR text; boxes with any Arabic-script character are never touched."""
    height, width = int(frame.shape[0]), int(frame.shape[1])
    results: list[Any] = []
    audit: list[dict[str, Any]] = []
    for index, (box, text, confidence) in enumerate(raw):
        content = str(text)
        if not content.strip() or has_arabic_script(content) or not any(ch.isalnum() for ch in content):
            results.append((box, text, confidence))
            continue
        xs, ys = [float(p[0]) for p in box], [float(p[1]) for p in box]
        x1, y1 = max(0, math.floor(min(xs) / scale) - margin), max(0, math.floor(min(ys) / scale) - margin)
        x2, y2 = min(width, math.ceil(max(xs) / scale) + margin), min(height, math.ceil(max(ys) / scale) + margin)
        if x2 <= x1 or y2 <= y1:
            results.append((box, text, confidence))
            continue
        try:
            latin_text, latin_confidence = recognizer.recognize(frame[y1:y2, x1:x2])
        except Exception:  # a recogniser failure must not lose the EasyOCR reading
            results.append((box, text, confidence))
            continue
        latin_text = (latin_text or "").strip()
        if not latin_text:
            results.append((box, text, confidence))
            continue
        use = mode == "all" or (mode == "confidence" and float(latin_confidence) > float(confidence))
        audit.append({"box": index, "easyocr_text": content, "latin_text": latin_text, "easyocr_confidence": float(confidence), "latin_confidence": float(latin_confidence), "used": use})
        results.append((box, latin_text, float(latin_confidence)) if use else (box, text, confidence))
    return results, audit


class EasyOcrProcessor:
    processor_id = PROCESSOR_ID
    processor_revision = PROCESSOR_REVISION

    def __init__(self, reader: Any, *, model_hashes: dict[str, str], maximum_lines: int = 2048, maximum_side: int = 0, cv2_module: Any = cv2, threads: int = 4,
                 latin_recognizer: Any | None = None, latin_mode: str = "all", latin_model_sha256: str = "") -> None:
        if not 1 <= maximum_lines <= 8192:
            raise ValueError("maximum OCR lines is invalid")
        if latin_mode not in ("all", "confidence"):
            raise ValueError("the Latin re-read mode is invalid")
        if maximum_side < 0:
            raise ValueError("maximum image side is invalid")
        self.reader = reader
        self.model_hashes = dict(model_hashes)
        self.maximum_lines = maximum_lines
        self.maximum_side = maximum_side  # 0 = no cap; a cap on the long side bounds the time of a large page (measured on development rows before it is set)
        self.cv2 = cv2_module
        self.threads = threads
        self.latin_recognizer, self.latin_mode = latin_recognizer, latin_mode  # the PP-OCRv5 Latin recogniser the router already loads; None = no re-read
        if latin_recognizer is not None and latin_model_sha256:
            self.model_hashes["latin_reread_recognizer"] = latin_model_sha256

    @classmethod
    def from_environment(cls, *, latin_recognizer: Any | None = None, latin_model_sha256: str = "") -> "EasyOcrProcessor":
        reread = os.getenv("FORENSIC_EASYOCR_LATIN_REREAD", "false").strip().lower() in ("1", "true", "yes")
        mode = os.getenv("FORENSIC_EASYOCR_LATIN_MODE", "all").strip().lower()
        if mode not in ("all", "confidence"):
            raise ModelUnavailableError("FORENSIC_EASYOCR_LATIN_MODE must be all or confidence")
        if reread and latin_recognizer is None:
            raise ModelUnavailableError("FORENSIC_EASYOCR_LATIN_REREAD needs the PP-OCRv5 Latin recogniser (configure the ONNX OCR models)")
        value = os.getenv("FORENSIC_EASYOCR_MODEL_DIR", "").strip()
        if not value:
            raise ModelUnavailableError("FORENSIC_EASYOCR_MODEL_DIR is required; implicit model download is disabled")
        directory = Path(value)
        hashes = {
            "detector": _verified(directory / DETECTOR_FILE, "FORENSIC_EASYOCR_DETECTOR_SHA256"),
            "recognizer": _verified(directory / RECOGNIZER_FILE, "FORENSIC_EASYOCR_RECOGNIZER_SHA256"),
        }
        threads = int(os.getenv("FORENSIC_EASYOCR_THREADS", "4"))
        try:
            import easyocr
            import torch
        except ImportError as exc:
            raise ModelUnavailableError("the easyocr and torch packages are not installed in this image") from exc
        torch.set_num_threads(threads)
        reader = easyocr.Reader(list(LANGUAGES), gpu=False, model_storage_directory=str(directory), user_network_directory=str(directory), download_enabled=False, verbose=False)
        return cls(reader, model_hashes=hashes, maximum_side=int(os.getenv("FORENSIC_EASYOCR_MAX_SIDE", "0")), threads=threads,
                   latin_recognizer=latin_recognizer if reread else None, latin_mode=mode, latin_model_sha256=latin_model_sha256)

    def process_image(self, path: Path, **context: Any) -> MediaProcessResult:
        frame = self.cv2.imread(str(path))
        if frame is None or len(frame.shape) < 2:
            raise MediaProcessingError("EasyOCR could not decode the image")
        height, width = int(frame.shape[0]), int(frame.shape[1])
        scale = 1.0
        work = frame
        if self.maximum_side and max(height, width) > self.maximum_side:
            scale = self.maximum_side / float(max(height, width))
            work = self.cv2.resize(frame, (max(1, round(width * scale)), max(1, round(height * scale))), interpolation=self.cv2.INTER_AREA)
        try:
            raw = self.reader.readtext(work, detail=1, paragraph=False)
        except Exception as exc:  # the reader raises many kinds of errors (memory, shape, torch); the router falls back to the other engine
            raise MediaProcessingError(f"EasyOCR could not read the image ({type(exc).__name__})") from exc
        audit: list[dict[str, Any]] = []
        results = raw
        if self.latin_recognizer is not None:
            results, audit = reread_latin_boxes(raw, frame, self.latin_recognizer, scale=scale, mode=self.latin_mode)
        lines = assemble_lines(results)[: self.maximum_lines]
        observations: list[MediaObservation] = []
        for reading_order, line in enumerate(lines, 1):
            x1 = int(max(0, min(width, math.floor(line["box"][0] / scale))))
            y1 = int(max(0, min(height, math.floor(line["box"][1] / scale))))
            x2 = int(max(0, min(width, math.ceil(line["box"][2] / scale))))
            y2 = int(max(0, min(height, math.ceil(line["box"][3] / scale))))
            if x2 <= x1 or y2 <= y1:
                continue
            ok, encoded = self.cv2.imencode(".png", frame[y1:y2, x1:x2])
            crop_hash = hashlib.sha256(bytes(encoded)).hexdigest() if ok else ""
            polygon = [[float(x1), float(y1)], [float(x2), float(y1)], [float(x2), float(y2)], [float(x1), float(y2)]]
            bbox = {"x": x1, "y": y1, "width": x2 - x1, "height": y2 - y1}
            raw_text = line["text"]
            script, direction = _script_and_direction(raw_text)
            locator = dict(context.get("locator_prefix") or {})
            locator.update({"source_file": context["source_file"], "coordinate_space": "original_image_pixels", "polygon": polygon, "bbox": bbox, "reading_order": reading_order})
            identity = str(uuid.uuid5(uuid.NAMESPACE_URL, f"{context['source_sha256']}:easyocr:{reading_order}:{bbox}:{crop_hash}"))
            observations.append(MediaObservation(
                observation_id=identity,
                contract_version=IMAGE_OCR_OBSERVATION_CONTRACT,
                observation_type="general_image_ocr_observation",
                confidence=float(line["confidence"]),
                citation_locator=locator,
                payload={
                    "raw_text": raw_text,
                    "normalized_text": _normalize_text(raw_text),
                    "script_family": script,
                    "display_direction": direction,
                    "reading_order": reading_order,
                    "reading_order_semantics": "top-to-bottom lines; inside a line right to left when most letters are Arabic-script, else left to right; boxes joined by one space",
                    "identifiers_preserved": _identifiers(raw_text),
                    "recognition_confidence": float(line["confidence"]),
                    "selected_recognizer": "arabic_g1_ur_en",
                    "box_count": line["boxes"],
                    "bbox": bbox,
                    "polygon": polygon,
                    "polygon_semantics": "bounding rectangle of the boxes of the line, in original image pixels",
                    "crop_sha256": crop_hash,
                    "crop_storage_state": "reconstructable_from_retained_source",
                    "parent_evidence_id": context["evidence_id"],
                    "parent_version_id": context["version_id"],
                    "processor": self.processor_id,
                    "processor_revision": self.processor_revision,
                    "detector_model": DETECTOR_ID,
                    "recognizer_models": {"arabic_g1_ur_en": RECOGNIZER_ID},
                    "model_sha256": self.model_hashes,
                    "image_scale_applied": round(scale, 4),
                    **({"latin_reread": [{k: a[k] for k in ("easyocr_text", "latin_text", "easyocr_confidence", "latin_confidence")} for a in audit if a["used"] and a["box"] in line["members"]]}
                       if self.latin_recognizer is not None else {}),
                    "manual_review_required": True,
                },
                warnings=("Recognized text is a model observation and requires analyst review",),
            ))
        limitations = [
            "Urdu line order and low-quality Nastaliq are not certified; measured on generated pages and a public newspaper set, not on real case documents",
            "a table row that holds Urdu text beside left-to-right columns can come back with its columns in reverse order; the words and identifiers are read, their order in the line is not certified",
            "printed text only; handwriting is not certified",
        ]
        if self.latin_recognizer is not None:
            limitations.append("Latin letters and digits are read again with the PP-OCRv5 Latin recogniser (the EasyOCR text of each replaced box is kept in the observation); a box with any Arabic-script character keeps the EasyOCR reading")
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
                "detected_box_count": len(raw),
                "model_sha256": self.model_hashes,
                "easyocr_threads": self.threads,
                "image_scale_applied": round(scale, 4),
                "weights_licence_status": WEIGHTS_LICENCE_STATUS,
                **({"latin_reread": {"mode": self.latin_mode, "boxes_checked": len(audit), "boxes_replaced": sum(1 for a in audit if a["used"])}} if self.latin_recognizer is not None else {}),
            },
        )
