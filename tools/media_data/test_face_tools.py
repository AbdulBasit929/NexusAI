"""Unit tests of the scoring code of the face bake-off (run inside the evaluation image):

    docker run --rm --network none --memory 800m -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-face:20261119 -m unittest discover -s /repo/tools/media_data -p "test_face_tools.py" -v
"""
import json
import sys
import tempfile
import unittest
from pathlib import Path

import numpy as np

sys.path.insert(0, str(Path(__file__).resolve().parent))
import face_common as fc  # noqa: E402
import face_detect_wider as dw  # noqa: E402
import face_fairface_score as ffs  # noqa: E402
import face_lfw_e2e as e2e  # noqa: E402


class AccuracyTests(unittest.TestCase):
    def test_perfect_separation(self):
        sims = np.array([0.9] * 300 + [0.1] * 300)
        issame = np.array([True] * 300 + [False] * 300)
        order = np.random.default_rng(1).permutation(600)
        acc, std, thr = fc.folds_accuracy(sims[order], issame[order])
        self.assertEqual(acc, 1.0)
        self.assertTrue(0.1 < thr < 0.9)

    def test_chance(self):
        rng = np.random.default_rng(2)
        sims = rng.random(2000)
        issame = rng.random(2000) < 0.5
        acc, _, _ = fc.folds_accuracy(sims, issame)
        self.assertTrue(0.45 < acc < 0.58)

    def test_tar_at_far(self):
        neg = np.linspace(0, 0.5, 1000)
        pos = np.array([0.9] * 99 + [0.1])
        tar, thr = fc.tar_at_far(pos, neg, 1e-3)
        self.assertAlmostEqual(tar, 0.99)
        self.assertGreaterEqual(thr, 0.49)


class DetectionScoringTests(unittest.TestCase):
    def setUp(self):
        face = lambda x, y, w, h, **k: {"x": x, "y": y, "w": w, "h": h, "blur": k.get("blur", 0), "expression": 0, "illumination": 0, "invalid": k.get("invalid", 0), "occlusion": k.get("occlusion", 0), "pose": k.get("pose", 0)}
        self.records = [{"name": "a", "faces": [face(10, 10, 50, 50), face(100, 10, 30, 30, blur=2), face(200, 10, 10, 10), face(300, 10, 60, 60, invalid=1)]}]

    def test_recall_and_false_boxes(self):
        dets = {"a": [(10, 10, 60, 60, 0.9), (100, 10, 130, 30 + 10, 0.8), (300, 10, 360, 70, 0.7), (500, 500, 540, 540, 0.9)]}
        r = dw.score_part(self.records, dets, 0.5)
        self.assertEqual(r["counts"]["w>=40"], [1, 1])
        self.assertEqual(r["counts"]["20<=w<40"], [1, 1])
        self.assertEqual(r["counts"]["w<20"], [0, 1])
        self.assertEqual(r["counts"]["blur2 (w>=20)"], [1, 1])
        self.assertEqual(r["false_boxes_per_image"], 1.0)  # the detection on the invalid face is ignored; the one at (500, 500) is false

    def test_threshold_filters(self):
        dets = {"a": [(10, 10, 60, 60, 0.4)]}
        r = dw.score_part(self.records, dets, 0.5)
        self.assertEqual(r["counts"]["w>=40"], [0, 1])
        self.assertEqual(r["false_boxes_per_image"], 0.0)

    def test_blaze_anchors(self):
        anchors = dw.blaze_anchors()
        self.assertEqual(anchors.shape, (896, 2))
        self.assertAlmostEqual(float(anchors[0, 0]), 0.5 / 16)

    def test_weighted_nms_merges(self):
        boxes = np.array([[0, 0, 10, 10], [1, 1, 11, 11], [50, 50, 60, 60]], dtype=np.float32)
        out = dw.weighted_nms(boxes, np.array([0.9, 0.8, 0.7], dtype=np.float32))
        self.assertEqual(len(out), 2)


class FairFaceTests(unittest.TestCase):
    def test_threshold_and_histogram(self):
        rng = np.random.default_rng(3)
        E = fc.l2n(rng.standard_normal((300, 32)).astype(np.float32))
        edges, per_group = ffs.within_group_histogram(E, {0: np.arange(300)})
        self.assertEqual(int(per_group[0].sum()), 300 * 299 // 2)
        thr = ffs.threshold_for(per_group[0], edges, 1e-2)
        sims = (E @ E.T)[np.triu_indices(300, 1)]
        self.assertLessEqual(float(np.mean(sims >= thr)), 1.2e-2)
        self.assertGreater(float(np.mean(sims >= thr)), 5e-3)

    def test_fmr_ci_matches_direct(self):
        rng = np.random.default_rng(4)
        V = fc.l2n(rng.standard_normal((200, 16)).astype(np.float32))
        point, boots = ffs.fmr_with_ci(V, 0.3, rng, draws=20)
        sims = (V @ V.T)[np.triu_indices(200, 1)]
        self.assertAlmostEqual(point, float(np.mean(sims >= 0.3)), places=6)
        self.assertEqual(len(boots), 20)


class PairsTests(unittest.TestCase):
    def test_read_pairs(self):
        with tempfile.TemporaryDirectory() as d:
            lines = ["2\t2", "Ann\t1\t2", "Bob\t1\t3", "Ann\t4\tCy\t1", "Bob\t2\tDee\t7", "Eve\t1\t2", "Eve\t3\t4", "Eve\t1\tGus\t2", "Hal\t1\tIan\t2", ""]
            Path(d, "pairs.txt").write_text("\n".join(lines))
            pairs, issame = e2e.read_pairs(Path(d))
        self.assertEqual(len(pairs), 8)
        self.assertEqual(pairs[0], ("Ann/Ann_0001.jpg", "Ann/Ann_0002.jpg"))
        self.assertEqual(pairs[2], ("Ann/Ann_0004.jpg", "Cy/Cy_0001.jpg"))
        self.assertEqual(issame.tolist(), [True, True, False, False, True, True, False, False])


if __name__ == "__main__":
    unittest.main()
