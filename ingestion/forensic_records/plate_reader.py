"""Plate reader (inference side): crop preprocessing, CTC beam search and the ONNX session wrapper.

Training lives in tools/media_data/train_plate_reader.py (PyTorch); the worker needs only numpy, OpenCV and onnxruntime. The preprocessing here is the single definition used by both, so a model is
always fed what it was trained on. A one-line plate is resized to 32 x 192; a two-line plate (aspect ratio under TWO_LINE_ASPECT) is cut into its upper and lower half, each resized to 32 x 96,
and the halves are laid side by side, so the same network reads both layouts and the label is the upper line followed by the lower line.
"""
from __future__ import annotations

import math
from pathlib import Path
from typing import Any

import numpy as np

CHARSET = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"  # CTC class 0 is the blank; character i is class i + 1
HEIGHT, WIDTH = 32, 192
TWO_LINE_ASPECT = 2.6
BEAM = 8


def prepare_crop(crop_bgr: np.ndarray, cv2: Any) -> np.ndarray:
    """BGR or grey crop -> float32 array of shape (1, 32, 192), values in [-1, 1]."""
    grey = crop_bgr if crop_bgr.ndim == 2 else cv2.cvtColor(crop_bgr, cv2.COLOR_BGR2GRAY)
    height, width = grey.shape[:2]
    if height < 2 or width < 2:
        return np.zeros((1, HEIGHT, WIDTH), np.float32)
    if width / height < TWO_LINE_ASPECT:
        half = height // 2
        top = cv2.resize(grey[:half], (WIDTH // 2, HEIGHT), interpolation=cv2.INTER_AREA)
        bottom = cv2.resize(grey[half:], (WIDTH // 2, HEIGHT), interpolation=cv2.INTER_AREA)
        line = np.hstack([top, bottom])
    else:
        line = cv2.resize(grey, (WIDTH, HEIGHT), interpolation=cv2.INTER_AREA)
    return ((line.astype(np.float32) / 255.0 - 0.5) / 0.5)[None, :, :]


def _log_add(a: float, b: float) -> float:
    if a == -math.inf:
        return b
    if b == -math.inf:
        return a
    high = max(a, b)
    return high + math.log(math.exp(a - high) + math.exp(b - high))


def ctc_beam(log_probs: np.ndarray, beam: int = BEAM) -> list[tuple[str, float]]:
    """Prefix beam search over (T, C) log-probabilities; returns up to `beam` (text, log-probability) pairs, best first. Class 0 is the blank."""
    beams: dict[tuple[int, ...], tuple[float, float]] = {(): (0.0, -math.inf)}  # prefix -> (log P ending in blank, log P ending in non-blank)
    for frame in log_probs:
        # only classes that matter at this frame; keeps the search fast
        candidates = np.argsort(frame)[-6:]
        nxt: dict[tuple[int, ...], tuple[float, float]] = {}
        for prefix, (p_blank, p_char) in beams.items():
            total = _log_add(p_blank, p_char)
            for cls in candidates:
                p = float(frame[cls])
                if cls == 0:
                    b, c = nxt.get(prefix, (-math.inf, -math.inf))
                    nxt[prefix] = (_log_add(b, total + p), c)
                    continue
                extended = prefix + (int(cls),)
                b, c = nxt.get(extended, (-math.inf, -math.inf))
                if prefix and prefix[-1] == cls:
                    nxt[extended] = (b, _log_add(c, p_blank + p))  # a repeat is a new character only across a blank
                    b2, c2 = nxt.get(prefix, (-math.inf, -math.inf))
                    nxt[prefix] = (b2, _log_add(c2, p_char + p))
                else:
                    nxt[extended] = (b, _log_add(c, total + p))
        beams = dict(sorted(nxt.items(), key=lambda item: _log_add(*item[1]), reverse=True)[:beam])
    ranked = sorted(((prefix, _log_add(*score)) for prefix, score in beams.items()), key=lambda item: item[1], reverse=True)
    return [("".join(CHARSET[i - 1] for i in prefix), score) for prefix, score in ranked]


class OnnxPlateReader:
    """The trained reader on onnxruntime. `read(crop)` returns candidate (text, log-probability) pairs, best first."""

    def __init__(self, model_path: str | Path, cv2_module: Any, providers: list[str] | None = None) -> None:
        import onnxruntime as ort  # imported here so tests of the pure functions need no runtime

        self.cv2 = cv2_module
        self.session = ort.InferenceSession(str(model_path), providers=providers or ["CPUExecutionProvider"])
        self.input_name = self.session.get_inputs()[0].name

    def read(self, crop_bgr: np.ndarray) -> list[tuple[str, float]]:
        batch = prepare_crop(crop_bgr, self.cv2)[None, :, :, :]
        logits = self.session.run(None, {self.input_name: batch})[0][0]  # (T, C)
        shifted = logits - logits.max(axis=1, keepdims=True)
        log_probs = shifted - np.log(np.exp(shifted).sum(axis=1, keepdims=True))
        return ctc_beam(log_probs)

    @staticmethod
    def confidence(candidates: list[tuple[str, float]]) -> float:
        """Probability mass of the first candidate among the beam (0 to 1); a calibrated-by-measurement score, reported with its reliability table, never as a probability of being right."""
        if not candidates:
            return 0.0
        total = _log_add_all([score for _, score in candidates])
        return float(math.exp(candidates[0][1] - total))


def _log_add_all(values: list[float]) -> float:
    result = -math.inf
    for value in values:
        result = _log_add(result, value)
    return result
