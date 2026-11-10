"""Plate detector on onnxruntime for RF-DETR style exports (one class, 576 x 576, 300 queries).

The published `autolane/rfdetr-alpr` export lists its outputs with swapped names and eight extra deep-supervision outputs; the first two outputs are the final layer: logits (1, 300, 1) and boxes
(1, 300, 4) as normalised centre x, centre y, width, height. Shapes decide which is which, so a differently named export of the same kind also works. Preprocessing is RGB, resized (stretched) to
the model's input, scaled to 0..1 and, for the plain export, normalised with the ImageNet mean and deviation (`normalise="imagenet"`; the TensorRT-optimised export wants `"unit"`).
Detections are returned in the original image's pixels. numpy, OpenCV and onnxruntime only.
"""
from __future__ import annotations

from pathlib import Path
from typing import Any

import numpy as np

MEAN = np.array([0.485, 0.456, 0.406], np.float32)
STD = np.array([0.229, 0.224, 0.225], np.float32)


class OnnxPlateDetector:
    def __init__(self, model_path: str | Path, cv2_module: Any, *, providers: list[str] | None = None, normalise: str = "imagenet", threshold: float = 0.5, drop_non_plates: bool = False) -> None:
        import onnxruntime as ort

        self.cv2 = cv2_module
        self.session = ort.InferenceSession(str(model_path), providers=providers or ["CPUExecutionProvider"])
        shape = self.session.get_inputs()[0].shape
        self.model = self.session  # the processor reports the active providers through `.model.get_providers()`
        self.input_name, self.size = self.session.get_inputs()[0].name, int(shape[-1])
        self.normalise, self.threshold, self.drop_non_plates = normalise, threshold, drop_non_plates

    def _prepare(self, frame_bgr: np.ndarray) -> np.ndarray:
        rgb = self.cv2.cvtColor(self.cv2.resize(frame_bgr, (self.size, self.size), interpolation=self.cv2.INTER_LINEAR), self.cv2.COLOR_BGR2RGB).astype(np.float32) / 255.0
        if self.normalise == "imagenet":
            rgb = (rgb - MEAN) / STD
        return rgb.transpose(2, 0, 1)[None]

    def detect(self, frame_bgr: np.ndarray) -> list[tuple[int, int, int, int, float]]:
        height, width = frame_bgr.shape[:2]
        outputs = self.session.run(None, {self.input_name: self._prepare(frame_bgr)})
        boxes = next(o[0] for o in outputs if o.ndim == 3 and o.shape[-1] == 4)
        logits = next(o[0] for o in outputs if o.ndim == 3 and o.shape[-1] != 4)
        scores = 1.0 / (1.0 + np.exp(-logits.max(axis=-1)))
        found = []
        for (cx, cy, w, h), score in zip(boxes, scores):
            if score < self.threshold:
                continue
            x1, y1, x2, y2 = (cx - w / 2) * width, (cy - h / 2) * height, (cx + w / 2) * width, (cy + h / 2) * height
            found.append((max(0, int(x1)), max(0, int(y1)), min(width, int(x2)), min(height, int(y2)), float(score)))
        found.sort(key=lambda d: -d[4])
        kept: list[tuple[int, int, int, int, float]] = []
        for det in found:  # a light non-maximum suppression: queries that land on the same plate
            if all(_iou(det, k) < 0.5 for k in kept):
                kept.append(det)
        if self.drop_non_plates:
            kept = drop_non_plates(kept, width, height)
        return kept


def drop_non_plates(detections: list[tuple[int, int, int, int, float]], width: int, height: int) -> list[tuple[int, int, int, int, float]]:
    """Remove the boxes that are not plates although the published detector returns them (it also boxes whole vehicles, measured 2026-11-10 on real photos).

    Two geometric checks, chosen on the owner-labelled development images and the Open Images validation photos, never on a sealed set:
      * a large, boxy box (more than 5% of the image, width under twice its height) that CONTAINS a much smaller detection (at most a quarter of its area, 90% inside it) is a vehicle around its plate;
        a close-up in which the plate fills the frame contains nothing smaller and is kept;
      * a portrait-shaped box (width under 1.1 times its height) is not a one- or two-line plate.
    """
    def area(d: tuple) -> float:
        return max(0, d[2] - d[0]) * max(0, d[3] - d[1])

    def inside(small: tuple, big: tuple) -> float:
        ix0, iy0, ix1, iy1 = max(small[0], big[0]), max(small[1], big[1]), min(small[2], big[2]), min(small[3], big[3])
        return max(0, ix1 - ix0) * max(0, iy1 - iy0) / max(1, area(small))

    kept = []
    for a in detections:
        aspect = (a[2] - a[0]) / max(1, a[3] - a[1])
        if aspect < 1.1:
            continue
        big_and_boxy = area(a) / max(1, width * height) > 0.05 and aspect < 2.0
        if big_and_boxy and any(b is not a and area(b) <= 0.25 * area(a) and inside(b, a) >= 0.9 for b in detections):
            continue
        kept.append(a)
    return kept


def _iou(a: tuple, b: tuple) -> float:
    ix0, iy0, ix1, iy1 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0, ix1 - ix0) * max(0, iy1 - iy0)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union > 0 else 0.0
