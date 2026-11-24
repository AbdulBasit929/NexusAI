# SPDX-License-Identifier: Apache-2.0
"""Region search: does a small picture (a crop, a screenshot of a detail) appear INSIDE a larger one, and where?

Method (classical instance retrieval with geometric verification: Lowe 2004 for SIFT; Arandjelovic and Zisserman 2012 for RootSIFT; Philbin et al., CVPR 2007, for verifying candidates with a
fitted geometric transform): local SIFT features are found in both pictures, matched with the ratio test, and a homography is fitted to the matches with RANSAC. A region that really is in the
other picture produces many matches that all agree on one transform; a different picture produces few and scattered ones. The count of agreeing matches (inliers) is the evidence, and the fitted
transform says WHERE the region sits (its corners in the other picture). The probe is also tried mirrored.

Nothing is stored on disk. Features of pictures already seen are kept in memory (bounded, least recently used) so that a second search over the same case is fast.
"""

from __future__ import annotations

import base64
import binascii
import hashlib
import threading
import time
from collections import OrderedDict
from dataclasses import dataclass
from typing import Any

import numpy as np

CANDIDATE_MAX_EDGE = 1280
PROBE_MIN_EDGE = 96
# Chosen 2026-11-23 by tools/media_data/region_param_probe.py on 216 probes (24 pictures x 9 variants) against the true picture and against 1,080 unrelated pairs: contrast 0.025 with 1000/1500 features
# found 200 at 12 or more agreeing points, 0.012 with 2000/2500 found 204, 0.006 with 3000/3000 found 208 (tiny details 13, 16, 18 of 24); the most chance matches between unrelated pictures was 6 in every setting.
CANDIDATE_FEATURES = 3000
PROBE_FEATURES = 3000
RATIO = 0.8
CONTRAST_THRESHOLD = 0.006  # SIFT's own default is 0.04; lower finds features in flatter regions (more matches in plain areas, more chance matches too)
RANSAC_PIXELS = 3.0
MIN_MATCHES = 6
# provisional until the calibration on negatives is recorded (docs/data/image-search-region-results-*.md): inliers at or above VERIFIED are reported as "contains this region", at or above PROBABLE as
# "probably contains it"
VERIFIED_INLIERS = 12
PROBABLE_INLIERS = 8
DEFAULT_CACHE_BYTES = 128 * 1024 * 1024  # about 400 KB per picture at 3000 features


class RegionUnavailable(RuntimeError):
    """OpenCV is not installed in this image."""


def _cv2() -> Any:
    try:
        import cv2  # type: ignore[import-not-found]
    except ModuleNotFoundError as exc:  # pragma: no cover - the worker image ships OpenCV
        raise RegionUnavailable("OpenCV is not installed") from exc
    return cv2


@dataclass
class Features:
    width: int  # size of the picture the keypoints are in (the reduced size), so that coordinates are comparable
    height: int
    xy: np.ndarray  # N x 2 float32, in the reduced picture
    desc: np.ndarray  # N x 128 uint8: RootSIFT scaled to 0..255

    @property
    def nbytes(self) -> int:
        return int(self.xy.nbytes + self.desc.nbytes) + 64

    def descriptors(self) -> np.ndarray:
        return self.desc.astype(np.float32) / 255.0


def _decode_gray(data: bytes) -> np.ndarray:
    cv2 = _cv2()
    image = cv2.imdecode(np.frombuffer(data, np.uint8), cv2.IMREAD_GRAYSCALE)
    if image is None:
        raise ValueError("the picture could not be decoded")
    return image


def extract(data: bytes, *, max_edge: int, nfeatures: int, min_edge: int = 0, flip: bool = False, contrast: float = CONTRAST_THRESHOLD) -> Features:
    cv2 = _cv2()
    gray = _decode_gray(data)
    if flip:
        gray = cv2.flip(gray, 1)
    height, width = gray.shape
    longest = max(height, width)
    scale = 1.0
    if longest > max_edge:
        scale = max_edge / longest
    elif min_edge and min(height, width) < min_edge:
        scale = min_edge / min(height, width)  # a tiny probe is enlarged so that it has features at the scales SIFT looks at
    if scale != 1.0:
        gray = cv2.resize(gray, (max(8, int(round(width * scale))), max(8, int(round(height * scale)))), interpolation=cv2.INTER_AREA if scale < 1 else cv2.INTER_CUBIC)
    height, width = gray.shape
    sift = cv2.SIFT_create(nfeatures=nfeatures, contrastThreshold=contrast)
    keypoints, descriptors = sift.detectAndCompute(gray, None)
    if descriptors is None or len(keypoints) == 0:
        return Features(width, height, np.zeros((0, 2), np.float32), np.zeros((0, 128), np.uint8))
    descriptors = descriptors / (descriptors.sum(axis=1, keepdims=True) + 1e-7)  # RootSIFT: L1-normalise, then square root
    descriptors = np.sqrt(descriptors)
    xy = np.array([keypoint.pt for keypoint in keypoints], np.float32)
    return Features(width, height, xy, np.clip(np.rint(descriptors * 255.0), 0, 255).astype(np.uint8))


def _convex(poly: np.ndarray) -> bool:
    sign = 0
    for i in range(4):
        a, b, c = poly[i], poly[(i + 1) % 4], poly[(i + 2) % 4]
        cross = (b[0] - a[0]) * (c[1] - b[1]) - (b[1] - a[1]) * (c[0] - b[0])
        if abs(cross) < 1e-6:
            return False
        if sign == 0:
            sign = 1 if cross > 0 else -1
        elif (cross > 0) != (sign > 0):
            return False
    return True


def _area(poly: np.ndarray) -> float:
    x, y = poly[:, 0], poly[:, 1]
    return float(0.5 * abs(np.dot(x, np.roll(y, -1)) - np.dot(y, np.roll(x, -1))))


def match(probe: Features, candidate: Features) -> dict[str, Any]:
    """Matches one probe against one candidate: the number of matches that agree on one homography (inliers) and where the probe's corners land in the candidate (normalised 0..1)."""
    cv2 = _cv2()
    result: dict[str, Any] = {"inliers": 0, "matches": 0, "polygon": None, "box": None}
    if len(probe.xy) < MIN_MATCHES or len(candidate.xy) < MIN_MATCHES:
        return result
    matcher = cv2.BFMatcher(cv2.NORM_L2)
    pairs = matcher.knnMatch(probe.descriptors(), candidate.descriptors(), k=2)
    good = [pair[0] for pair in pairs if len(pair) == 2 and pair[0].distance < RATIO * pair[1].distance]
    result["matches"] = len(good)
    if len(good) < MIN_MATCHES:
        return result
    source = np.float32([probe.xy[m.queryIdx] for m in good]).reshape(-1, 1, 2)
    target = np.float32([candidate.xy[m.trainIdx] for m in good]).reshape(-1, 1, 2)
    homography, mask = cv2.findHomography(source, target, cv2.RANSAC, RANSAC_PIXELS, maxIters=2000, confidence=0.995)
    if homography is None or mask is None:
        return result
    inliers = int(mask.sum())
    corners = np.float32([[0, 0], [probe.width, 0], [probe.width, probe.height], [0, probe.height]]).reshape(-1, 1, 2)
    polygon = cv2.perspectiveTransform(corners, homography).reshape(-1, 2)
    footprint = _area(polygon)
    frame = float(candidate.width * candidate.height)
    # the fitted transform must place the probe as a real, convex, non-degenerate shape of a plausible size inside (or at the edge of) the candidate
    if not np.all(np.isfinite(polygon)) or not _convex(polygon) or footprint < 0.0004 * frame or footprint > 4.0 * frame:
        return result
    normalised = polygon / np.array([candidate.width, candidate.height], np.float32)
    clipped = np.clip(normalised, 0.0, 1.0)
    result.update({
        "inliers": inliers,
        "polygon": [[round(float(x), 4), round(float(y), 4)] for x, y in normalised],
        "box": [round(float(clipped[:, 0].min()), 4), round(float(clipped[:, 1].min()), 4), round(float(clipped[:, 0].max()), 4), round(float(clipped[:, 1].max()), 4)],
    })
    return result


class FeatureCache:
    """Features of candidate pictures by content hash, bounded by bytes, least recently used first out."""

    def __init__(self, max_bytes: int = DEFAULT_CACHE_BYTES) -> None:
        self.max_bytes = max_bytes
        self._items: OrderedDict[str, Features] = OrderedDict()
        self._bytes = 0
        self._lock = threading.Lock()

    def get(self, key: str) -> Features | None:
        with self._lock:
            item = self._items.get(key)
            if item is not None:
                self._items.move_to_end(key)
            return item

    def put(self, key: str, features: Features) -> None:
        with self._lock:
            if key in self._items:
                self._bytes -= self._items.pop(key).nbytes
            self._items[key] = features
            self._bytes += features.nbytes
            while self._bytes > self.max_bytes and len(self._items) > 1:
                _, dropped = self._items.popitem(last=False)
                self._bytes -= dropped.nbytes

    def clear(self) -> None:
        with self._lock:
            self._items.clear()
            self._bytes = 0

    def __len__(self) -> int:
        return len(self._items)


MIN_CROP_EDGE = 24


def crop_region(data: bytes, box: list[float]) -> tuple[bytes, int, int]:
    """Cuts a box (x0, y0, x1, y1 as fractions of the picture as it is DISPLAYED, orientation tag applied) out of a picture and returns it as a lossless PNG with its size. The analyst draws the box
    on the picture shown in the interface; this makes the same cut from the stored bytes, so the part that is searched for is exactly the part that was drawn around."""
    cv2 = _cv2()
    image = cv2.imdecode(np.frombuffer(data, np.uint8), cv2.IMREAD_COLOR)  # applies the orientation tag, like the browser
    if image is None:
        raise ValueError("the picture could not be decoded")
    if not isinstance(box, (list, tuple)) or len(box) != 4:
        raise ValueError("the box needs four numbers")
    x0, y0, x1, y1 = (min(1.0, max(0.0, float(v))) for v in box)
    height, width = image.shape[:2]
    left, top, right, bottom = int(round(x0 * width)), int(round(y0 * height)), int(round(x1 * width)), int(round(y1 * height))
    if right - left < MIN_CROP_EDGE or bottom - top < MIN_CROP_EDGE:
        raise ValueError(f"the box is too small: it must be at least {MIN_CROP_EDGE} pixels on each side")
    crop = image[top:bottom, left:right]
    ok, encoded = cv2.imencode(".png", crop)
    if not ok:
        raise ValueError("the part could not be encoded")
    return encoded.tobytes(), int(crop.shape[1]), int(crop.shape[0])


class RegionMatcher:
    def __init__(self, cache: FeatureCache | None = None) -> None:
        self.cache = cache or FeatureCache()
        self._probes: OrderedDict[str, tuple[float, Features, Features]] = OrderedDict()
        self._lock = threading.Lock()

    def _probe(self, data: bytes) -> tuple[Features, Features]:
        key = hashlib.sha256(data).hexdigest()
        now = time.time()
        with self._lock:
            for stale in [k for k, (stamp, _, _) in self._probes.items() if now - stamp > 300]:
                self._probes.pop(stale, None)
            cached = self._probes.get(key)
            if cached is not None:
                return cached[1], cached[2]
        plain = extract(data, max_edge=CANDIDATE_MAX_EDGE, nfeatures=PROBE_FEATURES, min_edge=PROBE_MIN_EDGE)
        mirrored = extract(data, max_edge=CANDIDATE_MAX_EDGE, nfeatures=PROBE_FEATURES, min_edge=PROBE_MIN_EDGE, flip=True)
        with self._lock:
            self._probes[key] = (now, plain, mirrored)
            while len(self._probes) > 8:
                self._probes.popitem(last=False)
        return plain, mirrored

    def search(self, probe_bytes: bytes, items: list[dict[str, Any]], *, deadline_seconds: float = 100.0) -> dict[str, Any]:
        """items: {"key": <content hash>, "b64": <bytes, only when the service has not seen this picture>}. Returns the matches and the keys whose bytes are still needed."""
        probe, mirrored = self._probe(probe_bytes)
        results: list[dict[str, Any]] = []
        need: list[str] = []
        started = time.time()
        for item in items:
            key = str(item.get("key", ""))
            if not key:
                continue
            if time.time() - started > deadline_seconds:
                need.append(key)
                continue
            features = self.cache.get(key)
            if features is None:
                encoded = item.get("b64")
                if not encoded:
                    need.append(key)
                    continue
                try:
                    features = extract(base64.b64decode(encoded, validate=True), max_edge=CANDIDATE_MAX_EDGE, nfeatures=CANDIDATE_FEATURES)
                except (binascii.Error, ValueError):
                    results.append({"key": key, "error": "unreadable"})
                    continue
                self.cache.put(key, features)
            best = match(probe, features)
            flipped = match(mirrored, features)
            mirrored_win = flipped["inliers"] > best["inliers"]
            if mirrored_win:
                best = flipped
            results.append({"key": key, "inliers": best["inliers"], "matches": best["matches"], "mirrored": bool(mirrored_win), "box": best["box"], "polygon": best["polygon"]})
        return {"results": results, "need": need, "probe_features": int(len(probe.xy)), "cached": len(self.cache)}


def verdict(inliers: int) -> str:
    if inliers >= VERIFIED_INLIERS:
        return "verified"
    if inliers >= PROBABLE_INLIERS:
        return "probable"
    return "none"
