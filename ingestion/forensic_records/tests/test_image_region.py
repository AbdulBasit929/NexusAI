# SPDX-License-Identifier: Apache-2.0
"""Region search on synthetic textured pictures: a crop (re-compressed, rotated and scaled, mirrored) is found inside its source and located; an unrelated picture is not; the cache and the bytes
protocol behave. Run inside the worker image (OpenCV and numpy are there)."""

import base64
import hashlib
import unittest

import numpy as np

try:
    import cv2
except ModuleNotFoundError:  # pragma: no cover - the host python has no OpenCV
    cv2 = None

if cv2 is not None:
    from ingestion.forensic_records.image_region import FeatureCache, RegionMatcher, crop_region, extract, match, verdict


def scene(seed: int, size=(960, 720)):
    """A picture with plenty of structure: coloured shapes, lines of text, noise blobs."""
    rng = np.random.RandomState(seed)
    width, height = size
    image = np.full((height, width, 3), 120, np.uint8)
    image = cv2.GaussianBlur(rng.randint(0, 255, (height // 8, width // 8, 3)).astype(np.uint8), (0, 0), 1.2)
    image = cv2.resize(image, (width, height), interpolation=cv2.INTER_CUBIC)
    for _ in range(160):
        color = tuple(int(c) for c in rng.randint(0, 255, 3))
        x, y = int(rng.randint(0, width)), int(rng.randint(0, height))
        kind = rng.randint(0, 3)
        if kind == 0:
            cv2.rectangle(image, (x, y), (x + int(rng.randint(10, 90)), y + int(rng.randint(10, 90))), color, int(rng.choice([-1, 2])))
        elif kind == 1:
            cv2.circle(image, (x, y), int(rng.randint(6, 45)), color, int(rng.choice([-1, 2])))
        else:
            cv2.putText(image, "".join(chr(int(c)) for c in rng.randint(65, 90, 6)), (x, y), cv2.FONT_HERSHEY_SIMPLEX, float(rng.uniform(0.5, 1.4)), color, 2)
    return image


def jpeg(image, quality=80):
    ok, data = cv2.imencode(".jpg", image, [cv2.IMWRITE_JPEG_QUALITY, quality])
    assert ok
    return data.tobytes()


def iou(a, b):
    x0, y0, x1, y1 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0.0, x1 - x0) * max(0.0, y1 - y0)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union else 0.0


@unittest.skipIf(cv2 is None, "OpenCV is not installed on this machine")
class RegionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.source = scene(1)
        cls.other = scene(2)
        cls.source_bytes = jpeg(cls.source, 90)
        cls.other_bytes = jpeg(cls.other, 90)
        cls.candidate = extract(cls.source_bytes, max_edge=1280, nfeatures=1500)
        cls.stranger = extract(cls.other_bytes, max_edge=1280, nfeatures=1500)
        h, w = cls.source.shape[:2]
        cls.region = (int(0.30 * w), int(0.25 * h), int(0.62 * w), int(0.65 * h))  # about 13 percent of the picture
        x0, y0, x1, y1 = cls.region
        cls.crop = cls.source[y0:y1, x0:x1]
        cls.true_box = [x0 / w, y0 / h, x1 / w, y1 / h]

    def probe(self, image, quality=70, min_edge=96):
        return extract(jpeg(image, quality), max_edge=1280, nfeatures=1000, min_edge=min_edge)

    def test_a_recompressed_crop_is_found_and_located(self):
        found = match(self.probe(self.crop), self.candidate)
        self.assertGreaterEqual(found["inliers"], 25, found)
        self.assertGreater(iou(found["box"], self.true_box), 0.7, (found["box"], self.true_box))

    def test_a_rotated_and_shrunk_crop_is_found(self):
        height, width = self.crop.shape[:2]
        matrix = cv2.getRotationMatrix2D((width / 2, height / 2), 20, 0.6)
        moved = cv2.warpAffine(self.crop, matrix, (width, height), borderValue=(127, 127, 127))
        self.assertGreaterEqual(match(self.probe(moved), self.candidate)["inliers"], verdict_floor())

    def test_a_blurred_darker_crop_is_found(self):
        worse = cv2.convertScaleAbs(cv2.GaussianBlur(self.crop, (0, 0), 1.4), alpha=0.7, beta=-10)
        self.assertGreaterEqual(match(self.probe(worse), self.candidate)["inliers"], verdict_floor())

    def test_an_unrelated_picture_is_not_found(self):
        found = match(self.probe(self.crop), self.stranger)
        self.assertLess(found["inliers"], 8, found)

    def test_a_crop_of_another_picture_is_not_found_in_this_one(self):
        h, w = self.other.shape[:2]
        crop = self.other[int(0.2 * h):int(0.6 * h), int(0.2 * w):int(0.6 * w)]
        self.assertLess(match(self.probe(crop), self.candidate)["inliers"], 8)

    def test_the_search_tries_the_mirrored_probe(self):
        matcher = RegionMatcher()
        mirrored = cv2.flip(self.crop, 1)
        out = matcher.search(jpeg(mirrored, 80), [{"key": "src", "b64": base64.b64encode(self.source_bytes).decode()}, {"key": "other", "b64": base64.b64encode(self.other_bytes).decode()}])
        by_key = {r["key"]: r for r in out["results"]}
        self.assertGreaterEqual(by_key["src"]["inliers"], verdict_floor())
        self.assertTrue(by_key["src"]["mirrored"])
        self.assertLess(by_key["other"]["inliers"], 8)

    def test_bytes_are_asked_for_once_then_remembered(self):
        matcher = RegionMatcher(FeatureCache(8 * 1024 * 1024))
        key = hashlib.sha256(self.source_bytes).hexdigest()
        first = matcher.search(jpeg(self.crop, 80), [{"key": key}])
        self.assertEqual((first["results"], first["need"]), ([], [key]))
        second = matcher.search(jpeg(self.crop, 80), [{"key": key, "b64": base64.b64encode(self.source_bytes).decode()}])
        self.assertEqual(second["need"], [])
        self.assertGreaterEqual(second["results"][0]["inliers"], verdict_floor())
        third = matcher.search(jpeg(self.crop, 80), [{"key": key}])
        self.assertEqual(third["need"], [])
        self.assertEqual(len(third["results"]), 1)

    def test_garbage_is_reported_unreadable_not_raised(self):
        matcher = RegionMatcher()
        out = matcher.search(jpeg(self.crop, 80), [{"key": "bad", "b64": base64.b64encode(b"not a picture").decode()}])
        self.assertEqual(out["results"], [{"key": "bad", "error": "unreadable"}])

    def test_the_cache_is_bounded(self):
        cache = FeatureCache(max_bytes=300 * 1024)
        for index in range(12):
            cache.put(str(index), self.candidate)
        self.assertLess(len(cache), 12)
        self.assertIsNotNone(cache.get("11"))

    def test_a_drawn_box_is_cut_out_exactly_and_losslessly(self):
        png, width, height = crop_region(self.source_bytes, [0.30, 0.25, 0.62, 0.65])
        h, w = self.source.shape[:2]
        self.assertEqual((width, height), (int(round(0.62 * w)) - int(round(0.30 * w)), int(round(0.65 * h)) - int(round(0.25 * h))))
        decoded = cv2.imdecode(np.frombuffer(png, np.uint8), cv2.IMREAD_COLOR)
        self.assertEqual(decoded.shape[:2], (height, width))
        # and it finds its own source, in the place it was cut from
        found = match(extract(png, max_edge=1280, nfeatures=1000, min_edge=96), self.candidate)
        self.assertGreaterEqual(found["inliers"], 25, found)
        self.assertGreater(iou(found["box"], [0.30, 0.25, 0.62, 0.65]), 0.7, found)

    def test_a_box_that_is_too_small_or_malformed_is_refused(self):
        for bad in ([0.1, 0.1, 0.11, 0.11], [0.5, 0.5, 0.4, 0.9], [0.1, 0.1], "x", None):
            with self.assertRaises(ValueError, msg=str(bad)):
                crop_region(self.source_bytes, bad)
        with self.assertRaises(ValueError):
            crop_region(b"not a picture", [0, 0, 1, 1])
        # a box that runs past the picture is clamped to it
        _, width, _ = crop_region(self.source_bytes, [0.5, 0.5, 2.0, 2.0])
        self.assertEqual(width, self.source.shape[1] - int(round(0.5 * self.source.shape[1])))

    def test_verdict_bands(self):
        self.assertEqual((verdict(30), verdict(12), verdict(9), verdict(3)), ("verified", "verified", "probable", "none"))


def verdict_floor():
    return 12


if __name__ == "__main__":
    unittest.main()
