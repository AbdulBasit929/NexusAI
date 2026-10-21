# SPDX-License-Identifier: Apache-2.0
"""Text detection and recognition on the worker's own onnxruntime, with the Apache-2.0 PaddleOCR PP-OCRv5 ONNX exports (arabic, latin, en recognisers; mobile or server detector).

Why this exists: Tesseract's Urdu model is weak (measured in docs/data/media-activation-results-20261021.md) and the PaddleOCR Python runtime is not installed in the worker image. The ONNX exports
of the PaddlePaddle organisation need only onnxruntime, OpenCV and numpy, which the worker already has. Nothing is downloaded at run time: the model files and their configuration are pinned
paths, and the module refuses to start when one is missing.

Pieces (each usable alone):
    PaddleOnnxRecognizer.recognize(image)    one text-line image -> (text, confidence); CTC greedy decoding with the model's own dictionary (class 0 is the blank, the dictionary follows,
                                             then the space). Arabic-script models emit visual order, so runs of non-Latin characters are reversed into logical order the way PaddleOCR does.
    PaddleOnnxDetector.detect(image)         page image -> quadrilaterals of text lines (DB post-processing: threshold, contours, minimum-area rectangle, rectangle unclip)
    PaddleOnnxOCR.read(image)                detector then recogniser per line, lines in reading order (top to bottom; right to left inside a line when the line is mostly Arabic script)

It is a pure function of its inputs: no state is kept between calls, no file is written, and every result carries the pixel quadrilateral it came from.
"""

from __future__ import annotations

import math
import os
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Sequence

import cv2
import numpy as np


class OnnxOcrUnavailableError(RuntimeError):
    """A pinned model file or its configuration is missing, so the engine must not start."""


@dataclass(frozen=True)
class TextLine:
    text: str
    confidence: float
    quad: tuple[tuple[float, float], ...]  # four (x, y) points in the original image, clockwise from the top-left of the line box
    script: str  # "arabic", "latin" or "other", by the characters recognised


def _session(path: Path, providers: Sequence[str] | None):
    try:
        import onnxruntime as ort
    except ModuleNotFoundError as exc:  # pragma: no cover - the worker image has onnxruntime
        raise OnnxOcrUnavailableError("onnxruntime is not installed") from exc
    if not path.is_file():
        raise OnnxOcrUnavailableError(f"model file is missing: {path}")
    options = ort.SessionOptions()
    options.log_severity_level = 3
    # Bounded CPU: the worker runs one heavy model at a time (FORENSIC_ONNX_OCR_THREADS, default 4) and must leave the laptop usable.
    options.intra_op_num_threads = max(1, int(os.getenv("FORENSIC_ONNX_OCR_THREADS", "4")))
    options.inter_op_num_threads = 1
    return ort.InferenceSession(str(path), sess_options=options, providers=list(providers or ["CPUExecutionProvider"]))


def _script_of(text: str) -> str:
    arabic = sum(1 for ch in text if "؀" <= ch <= "ۿ" or "ݐ" <= ch <= "ݿ" or "ﭐ" <= ch <= "﷿" or "ﹰ" <= ch <= "﻿")
    latin = sum(1 for ch in text if ch.isascii() and ch.isalpha())
    if arabic > latin and arabic:
        return "arabic"
    if latin:
        return "latin"
    return "other"


_LTR_RUN = re.compile(r"[a-zA-Z0-9 :*./%+\-]")


def _reverse_visual_to_logical(text: str) -> str:
    """The Arabic recogniser emits characters left to right as drawn; logical order reverses the runs of non-Latin characters and keeps Latin, digit and punctuation runs as they are
    (the same rule PaddleOCR's CTC decoder applies to its Arabic model)."""
    parts: list[str] = []
    run = ""
    for ch in text:
        if _LTR_RUN.match(ch):
            run += ch
        else:
            if run:
                parts.append(run)
                run = ""
            parts.append(ch)
    if run:
        parts.append(run)
    return "".join(reversed(parts))


class PaddleOnnxRecognizer:
    def __init__(self, model_path: Path, config_path: Path, *, arabic_script: bool, providers: Sequence[str] | None = None, max_width: int = 3200):
        import yaml

        if not config_path.is_file():
            raise OnnxOcrUnavailableError(f"recogniser configuration is missing: {config_path}")
        config = yaml.safe_load(config_path.read_text(encoding="utf-8"))
        self.characters: list[str] = ["\0"] + [str(c) for c in config["PostProcess"]["character_dict"]] + [" "]
        self.arabic_script = arabic_script
        self.max_width = max_width
        self.session = _session(model_path, providers)
        self.input_name = self.session.get_inputs()[0].name
        classes = self.session.get_outputs()[0].shape[-1]
        if isinstance(classes, int) and classes != len(self.characters):
            raise OnnxOcrUnavailableError(f"recogniser has {classes} classes but its dictionary gives {len(self.characters)}")

    def _prepare(self, image: np.ndarray) -> np.ndarray:
        if image.ndim == 2:
            image = cv2.cvtColor(image, cv2.COLOR_GRAY2BGR)
        height, width = image.shape[:2]
        new_width = max(16, min(self.max_width, int(math.ceil(48 * width / max(1, height)))))
        resized = cv2.resize(image, (new_width, 48), interpolation=cv2.INTER_LINEAR).astype(np.float32)
        resized = (resized / 255.0 - 0.5) / 0.5
        return resized.transpose(2, 0, 1)[None, ...]

    def recognize(self, image: np.ndarray) -> tuple[str, float]:
        probabilities = self.session.run(None, {self.input_name: self._prepare(image)})[0][0]
        best = probabilities.argmax(axis=1)
        scores = probabilities.max(axis=1)
        chars: list[str] = []
        confidences: list[float] = []
        previous = -1
        for index, score in zip(best, scores):
            if index != previous and index != 0:
                chars.append(self.characters[index])
                confidences.append(float(score))
            previous = index
        text = "".join(chars)
        if self.arabic_script:
            text = _reverse_visual_to_logical(text)
        return text, (float(np.mean(confidences)) if confidences else 0.0)


class PaddleOnnxDetector:
    def __init__(self, model_path: Path, *, providers: Sequence[str] | None = None, max_side: int = 1920, threshold: float = 0.3, box_threshold: float = 0.6, unclip_ratio: float = 1.5):
        self.session = _session(model_path, providers)
        self.input_name = self.session.get_inputs()[0].name
        self.max_side, self.threshold, self.box_threshold, self.unclip_ratio = max_side, threshold, box_threshold, unclip_ratio

    def detect(self, image: np.ndarray) -> list[np.ndarray]:
        if image.ndim == 2:
            image = cv2.cvtColor(image, cv2.COLOR_GRAY2BGR)
        height, width = image.shape[:2]
        scale = min(1.0, self.max_side / max(height, width))
        if min(height, width) * scale < 64:
            scale = 64 / min(height, width)
        new_h, new_w = max(32, int(round(height * scale / 32)) * 32), max(32, int(round(width * scale / 32)) * 32)
        resized = cv2.resize(image, (new_w, new_h), interpolation=cv2.INTER_LINEAR)
        rgb = resized[:, :, ::-1].astype(np.float32) / 255.0
        rgb = (rgb - np.array([0.485, 0.456, 0.406], np.float32)) / np.array([0.229, 0.224, 0.225], np.float32)
        probability = self.session.run(None, {self.input_name: rgb.transpose(2, 0, 1)[None, ...]})[0][0, 0]
        bitmap = (probability > self.threshold).astype(np.uint8) * 255
        contours, _ = cv2.findContours(bitmap, cv2.RETR_LIST, cv2.CHAIN_APPROX_SIMPLE)
        boxes: list[np.ndarray] = []
        for contour in sorted(contours, key=cv2.contourArea, reverse=True)[:1000]:
            rect = cv2.minAreaRect(contour)
            if min(rect[1]) < 3:
                continue
            quad_points = cv2.boxPoints(rect)
            x0, y0 = int(max(0, np.floor(quad_points[:, 0].min()))), int(max(0, np.floor(quad_points[:, 1].min())))
            x1, y1 = int(min(probability.shape[1], np.ceil(quad_points[:, 0].max()) + 1)), int(min(probability.shape[0], np.ceil(quad_points[:, 1].max()) + 1))
            if x1 <= x0 or y1 <= y0:
                continue
            window = probability[y0:y1, x0:x1]
            mask = np.zeros(window.shape, np.uint8)
            cv2.fillPoly(mask, [(quad_points - [x0, y0]).astype(np.int32)], 1)  # the score is measured inside the box's own bounding rectangle, not over the whole page
            score = float(window[mask > 0].mean()) if mask.any() else 0.0
            if score < self.box_threshold:
                continue
            (cx, cy), (rw, rh), angle = rect
            distance = (rw * rh) * self.unclip_ratio / max(1e-6, 2 * (rw + rh))  # Clipper's offset distance for a rectangle
            expanded = ((cx, cy), (rw + 2 * distance, rh + 2 * distance), angle)
            if min(expanded[1]) < 5:
                continue
            quad = cv2.boxPoints(expanded)
            quad[:, 0] = np.clip(quad[:, 0] * width / new_w, 0, width - 1)
            quad[:, 1] = np.clip(quad[:, 1] * height / new_h, 0, height - 1)
            boxes.append(_order_quad(quad))
        return boxes


def _order_quad(points: np.ndarray) -> np.ndarray:
    """Top-left, top-right, bottom-right, bottom-left."""
    by_sum = points.sum(axis=1)
    by_diff = np.diff(points, axis=1).ravel()
    return np.array([points[by_sum.argmin()], points[by_diff.argmin()], points[by_sum.argmax()], points[by_diff.argmax()]], dtype=np.float32)


def _crop_quad(image: np.ndarray, quad: np.ndarray) -> np.ndarray:
    width = int(round(max(np.linalg.norm(quad[0] - quad[1]), np.linalg.norm(quad[2] - quad[3]))))
    height = int(round(max(np.linalg.norm(quad[0] - quad[3]), np.linalg.norm(quad[1] - quad[2]))))
    width, height = max(1, width), max(1, height)
    target = np.array([[0, 0], [width, 0], [width, height], [0, height]], np.float32)
    crop = cv2.warpPerspective(image, cv2.getPerspectiveTransform(quad.astype(np.float32), target), (width, height), borderMode=cv2.BORDER_REPLICATE, flags=cv2.INTER_CUBIC)
    if height / max(1, width) >= 1.5:  # a vertical box is a rotated line
        crop = np.rot90(crop)
    return crop


class PaddleOnnxOCR:
    """Detector plus recogniser. `recognizer_for` maps the expected script to a recogniser so a page can be read with the Arabic model when it is mostly Urdu and the Latin one otherwise."""

    def __init__(self, detector: PaddleOnnxDetector, recognizers: dict[str, PaddleOnnxRecognizer], *, minimum_confidence: float = 0.0):
        if not recognizers:
            raise ValueError("at least one recogniser is required")
        self.detector = detector
        self.recognizers = recognizers
        self.minimum_confidence = minimum_confidence

    def read(self, image: np.ndarray, *, script: str | None = None) -> list[TextLine]:
        boxes = self.detector.detect(image)
        order = self._reading_order(boxes)
        lines: list[TextLine] = []
        for quad in (boxes[i] for i in order):
            crop = _crop_quad(image, quad)
            options = [self.recognizers[script]] if script in self.recognizers else list(self.recognizers.values())
            best_text, best_conf = "", -1.0
            for recogniser in options:
                text, confidence = recogniser.recognize(crop)
                if confidence > best_conf:
                    best_text, best_conf = text, confidence
            if best_text.strip() and best_conf >= self.minimum_confidence:
                lines.append(TextLine(best_text, best_conf, tuple((float(x), float(y)) for x, y in quad), _script_of(best_text)))
        return lines

    @staticmethod
    def _reading_order(boxes: list[np.ndarray]) -> list[int]:
        """Top to bottom by line, lines found by overlapping vertical extents; the caller decides the direction inside a line from the script."""
        indexed = sorted(range(len(boxes)), key=lambda i: (float(boxes[i][:, 1].mean()), float(boxes[i][:, 0].mean())))
        lines: list[list[int]] = []
        for i in indexed:
            top, bottom = float(boxes[i][:, 1].min()), float(boxes[i][:, 1].max())
            for line in lines:
                reference = boxes[line[0]]
                overlap = min(bottom, float(reference[:, 1].max())) - max(top, float(reference[:, 1].min()))
                if overlap > 0.5 * min(bottom - top, float(reference[:, 1].max() - reference[:, 1].min())):
                    line.append(i)
                    break
            else:
                lines.append([i])
        ordered: list[int] = []
        for line in lines:
            ordered.extend(sorted(line, key=lambda i: float(boxes[i][:, 0].mean())))
        return ordered


def join_lines(lines: Sequence[TextLine]) -> str:
    """Plain text of a page: one line per detected line; boxes that sit on the same row are joined with a space, right to left when the row is mostly Arabic script."""
    rows: list[list[TextLine]] = []
    for line in lines:
        y = sum(p[1] for p in line.quad) / 4
        height = max(p[1] for p in line.quad) - min(p[1] for p in line.quad)
        if rows and abs(y - sum(p[1] for p in rows[-1][0].quad) / 4) < 0.5 * max(1.0, height):
            rows[-1].append(line)
        else:
            rows.append([line])
    out = []
    for row in rows:
        rtl = sum(1 for item in row if item.script == "arabic") > len(row) / 2
        ordered = sorted(row, key=lambda item: sum(p[0] for p in item.quad) / 4, reverse=rtl)
        out.append(" ".join(item.text for item in ordered))
    return "\n".join(out)
