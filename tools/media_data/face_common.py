"""Shared helpers of the face and image-search bake-off (docs/data/face-image-search-preregistration-20261119.md).

Runs inside nexusai-media-eval-face (python 3.11, OpenCV 5, onnxruntime) with the data mounted read-only at /data and a work folder at /work.
Faces are only ever embedded to be scored; nothing here stores a name or loads anything into a case.
"""
import hashlib
import os
import time

import cv2
import numpy as np

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
WORK = os.environ.get("NEXUSAI_FACE_WORK", "/work")
YUNET = f"{DATA}/models/face/yunet/face_detection_yunet_2023mar.onnx"
SFACE = f"{DATA}/models/face/sface/face_recognition_sface_2021dec.onnx"
AURA = f"{DATA}/models/face/auraface-v1/af6d057c9b0e/glintr100.onnx"
BLAZE = f"{DATA}/models/face/blazeface/blaze_face_short_range.tflite"
THREADS = int(os.environ.get("FACE_THREADS", "8"))


def partition(key, dev_digits):
    """dev if the last hex digit of the SHA-256 of the key is in dev_digits (same convention as the earlier programmes, with the dev set fixed in the pre-registration)."""
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in dev_digits else "sealed"


def iou(a, b):
    ix0, iy0, ix1, iy1 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0.0, ix1 - ix0) * max(0.0, iy1 - iy0)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union > 0 else 0.0


def l2n(x):
    x = np.asarray(x, dtype=np.float32)
    return x / np.maximum(np.linalg.norm(x, axis=-1, keepdims=True), 1e-9)


class SFaceEmbedder:
    """cv2.FaceRecognizerSF: its own pre-processing, 128-d. Input: 112x112 aligned BGR crop."""
    name = "sface"
    dim = 128

    def __init__(self):
        self.rec = cv2.FaceRecognizerSF.create(SFACE, "")

    def embed(self, bgr_list):
        return np.concatenate([self.rec.feature(im) for im in bgr_list], axis=0)


class AuraEmbedder:
    """AuraFace-v1 recogniser file only (glintr100.onnx): RGB, (x-127.5)/127.5, 112x112, 512-d. Input: 112x112 aligned BGR crop."""
    name = "auraface"
    dim = 512

    def __init__(self, threads=None):
        import onnxruntime as ort
        options = ort.SessionOptions()
        options.intra_op_num_threads = threads or THREADS
        options.inter_op_num_threads = 1
        self.sess = ort.InferenceSession(AURA, options, providers=["CPUExecutionProvider"])
        self.input = self.sess.get_inputs()[0].name

    def embed(self, bgr_list, batch=16):
        out = []
        for i in range(0, len(bgr_list), batch):
            chunk = bgr_list[i:i + batch]
            blob = cv2.dnn.blobFromImages(chunk, 1.0 / 127.5, (112, 112), (127.5, 127.5, 127.5), swapRB=True)
            result = self.sess.run(None, {self.input: blob})[0]
            if result.shape[0] != len(chunk):  # the model declares a batch of 1: fall back to one image at a time
                result = np.concatenate([self.sess.run(None, {self.input: blob[j:j + 1]})[0] for j in range(len(chunk))], axis=0)
            out.append(result)
        return np.concatenate(out, axis=0)


def embedder(name, threads=None):
    return SFaceEmbedder() if name == "sface" else AuraEmbedder(threads)


def yunet(width=320, height=320, score=0.6, nms=0.3, top_k=5000):
    return cv2.FaceDetectorYN.create(YUNET, "", (width, height), score_threshold=score, nms_threshold=nms, top_k=top_k)


def align_with_yunet(image_bgr, detector, min_score=0.6, select="score"):
    """Detect with YuNet and align one face to the 112x112 ArcFace template with SFace's alignCrop. select="score" takes the highest-scoring face (the pre-registered rule);
    select="centre" takes, among faces with score >= min_score, the one whose box centre is nearest the image centre (post-hoc diagnostic, Amendment A3). Returns (crop or None, number of faces, score)."""
    height, width = image_bgr.shape[:2]
    detector.setInputSize((width, height))
    _, faces = detector.detect(image_bgr)
    if faces is None or len(faces) == 0:
        return None, 0, 0.0
    if select == "centre":
        pool = [f for f in faces if float(f[14]) >= min_score]
        if not pool:
            best = max(faces, key=lambda f: float(f[14]))
            return None, len(faces), float(best[14])
        best = min(pool, key=lambda f: (f[0] + f[2] / 2 - width / 2) ** 2 + (f[1] + f[3] / 2 - height / 2) ** 2)
        return aligner().alignCrop(image_bgr, best), len(faces), float(best[14])
    best = max(faces, key=lambda f: float(f[14]))
    if float(best[14]) < min_score:
        return None, len(faces), float(best[14])
    return aligner().alignCrop(image_bgr, best), len(faces), float(best[14])


_aligner = None


def aligner():
    global _aligner
    if _aligner is None:
        _aligner = cv2.FaceRecognizerSF.create(SFACE, "")
    return _aligner


def folds_accuracy(sims, issame, nfolds=10, steps=np.arange(-1.0, 1.0, 0.0025)):
    """InsightFace-style 10-fold accuracy: threshold chosen on nine folds (best accuracy), tested on the tenth; contiguous folds. Returns mean, std, mean threshold."""
    sims = np.asarray(sims)
    issame = np.asarray(issame, dtype=bool)
    n = len(sims)
    edges = np.linspace(0, n, nfolds + 1).astype(int)
    accs, thresholds = [], []
    for k in range(nfolds):
        test = np.zeros(n, dtype=bool)
        test[edges[k]:edges[k + 1]] = True
        train = ~test
        best_t, best_acc = steps[0], -1.0
        for t in steps:
            acc = np.mean((sims[train] >= t) == issame[train])
            if acc > best_acc:
                best_acc, best_t = acc, t
        accs.append(np.mean((sims[test] >= best_t) == issame[test]))
        thresholds.append(best_t)
    return float(np.mean(accs)), float(np.std(accs)), float(np.mean(thresholds))


def tar_at_far(pos, neg, far):
    """True accept rate at the threshold where the pooled impostor false accept rate is `far`."""
    neg_sorted = np.sort(neg)
    k = int(np.ceil((1.0 - far) * len(neg_sorted))) - 1
    threshold = neg_sorted[min(max(k, 0), len(neg_sorted) - 1)]
    # strictly above the threshold so that the realised FAR is at most `far`
    return float(np.mean(pos > threshold)), float(threshold)


class Timer:
    def __init__(self):
        self.t = []

    def __enter__(self):
        self.t0 = time.perf_counter()

    def __exit__(self, *a):
        self.t.append(time.perf_counter() - self.t0)

    def median(self):
        return float(np.median(self.t)) if self.t else float("nan")


def rss_mb():
    try:
        for line in open("/proc/self/status"):
            if line.startswith("VmHWM:"):
                return int(line.split()[1]) / 1024.0
    except OSError:
        pass
    return float("nan")
