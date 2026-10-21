#!/usr/bin/env python3
"""Audit sheet for the face detector: a random sample of detections that match no labelled face (the 'false boxes'), cropped with a margin and tiled into one contact sheet with index numbers, so a
reviewer can say how many are real faces that Open Images did not label. The sample is seeded and the sheet goes to /data/results/faces/audit-<score>.png; the crops are not stored elsewhere.

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest python /repo/tools/media_data/face_audit.py --score 0.7 --n 48
"""
import argparse
import hashlib
import json
import os
import random
from pathlib import Path

import cv2
import numpy as np

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")


def iou(a, b):
    ix0, iy0, ix1, iy1 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0.0, ix1 - ix0) * max(0.0, iy1 - iy0)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union > 0 else 0.0


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--score", type=float, default=0.7)
    parser.add_argument("--n", type=int, default=48)
    args = parser.parse_args()
    model = Path(DATA) / "models" / "face" / "yunet" / "face_detection_yunet_2023mar.onnx"
    detector = cv2.FaceDetectorYN.create(str(model), "", (320, 320), score_threshold=args.score, nms_threshold=0.3, top_k=5000)
    records = [json.loads(l) for l in open(Path(DATA) / "openimages" / "subset-test.jsonl", encoding="utf-8")]
    false_boxes = []
    for record in records:
        if "face" not in record["roles"] or hashlib.sha256(record["id"].encode()).hexdigest()[-1] not in "0123456789ab":
            continue
        image = cv2.imread(str(Path(DATA) / record["file"]))
        height, width = image.shape[:2]
        truth = [(f["x0"] * width, f["y0"] * height, f["x1"] * width, f["y1"] * height) for f in record["faces"]]
        detector.setInputSize((width, height))
        _, faces = detector.detect(image)
        for f in (faces if faces is not None else []):
            box = (float(f[0]), float(f[1]), float(f[0] + f[2]), float(f[1] + f[3]))
            if not any(iou(box, t) >= 0.3 for t in truth):
                false_boxes.append((record["id"], box, float(f[14])))
    random.Random(7).shuffle(false_boxes)
    sample = false_boxes[:args.n]
    cell, cols = 150, 8
    rows = (len(sample) + cols - 1) // cols
    sheet = np.full((rows * (cell + 18), cols * cell, 3), 255, np.uint8)
    for i, (image_id, box, score) in enumerate(sample):
        image = cv2.imread(str(Path(DATA) / "openimages" / "photos" / "test" / f"{image_id}.jpg"))
        x0, y0, x1, y1 = box
        w, h = x1 - x0, y1 - y0
        m = 0.35
        crop = image[int(max(0, y0 - m * h)):int(min(image.shape[0], y1 + m * h)), int(max(0, x0 - m * w)):int(min(image.shape[1], x1 + m * w))]
        crop = cv2.resize(crop, (cell - 4, cell - 4))
        r, c = divmod(i, cols)
        sheet[r * (cell + 18) + 16:r * (cell + 18) + 16 + cell - 4, c * cell + 2:c * cell + cell - 2] = crop
        cv2.putText(sheet, f"{i + 1}  w{int(w)}px s{score:.2f}", (c * cell + 3, r * (cell + 18) + 12), cv2.FONT_HERSHEY_SIMPLEX, 0.38, (0, 0, 0), 1)
    out = Path(DATA) / "results" / "faces"
    out.mkdir(parents=True, exist_ok=True)
    cv2.imwrite(str(out / f"audit-{args.score}.png"), sheet)
    print(f"{len(false_boxes)} unmatched detections at score >= {args.score}; sampled {len(sample)}; sheet {out / f'audit-{args.score}.png'}")


if __name__ == "__main__":
    main()
