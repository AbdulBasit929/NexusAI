#!/usr/bin/env python3
"""Face detection (level L0 only) measurement for track F: YuNet (the detector inside the worker's face role) against Open Images photos with face boxes (CC BY 2.0; DAT-018 and DAT-019).

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data:ro -v <repo>:/repo:ro nexusai-media-eval:latest python /repo/tools/media_data/face_eval.py --part dev

Only detection is scored: boxes, never identities, and no embedding is computed or stored. Recall is at IoU 0.5 against the labelled faces that are neither groups nor depictions, split by width in
pixels (the gate is for faces at least 40 pixels wide); false boxes per image counts detections that match no labelled face and no ignored region. Open Images labels are incomplete for very small
or crowd faces, so the false-box count is an upper bound.
"""
import argparse
import hashlib
import json
import os
import time
from pathlib import Path

import cv2

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")


def partition(key):
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def iou(a, b):
    ix0, iy0, ix1, iy1 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0.0, ix1 - ix0) * max(0.0, iy1 - iy0)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union > 0 else 0.0


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--part", default="dev", choices=("dev", "sealed"))
    parser.add_argument("--limit", type=int, default=0)
    args = parser.parse_args()
    model = Path(DATA) / "models" / "face" / "yunet" / "face_detection_yunet_2023mar.onnx"
    detector = cv2.FaceDetectorYN.create(str(model), "", (320, 320), score_threshold=0.05, nms_threshold=0.3, top_k=5000)
    records = [json.loads(line) for line in open(Path(DATA) / "openimages" / "subset-test.jsonl", encoding="utf-8")]
    photos = [r for r in records if "face" in r["roles"] and partition(r["id"]) == args.part]
    if args.limit:
        photos = photos[:args.limit]
    truths, preds, seconds = {}, {}, []
    for record in photos:
        image = cv2.imread(str(Path(DATA) / record["file"]))
        height, width = image.shape[:2]
        truths[record["id"]] = [((f["x0"] * width, f["y0"] * height, f["x1"] * width, f["y1"] * height), f["group"] or f["depiction"]) for f in record["faces"]]
        detector.setInputSize((width, height))
        t0 = time.time()
        _, faces = detector.detect(image)
        seconds.append(time.time() - t0)
        preds[record["id"]] = [((float(f[0]), float(f[1]), float(f[0] + f[2]), float(f[1] + f[3])), float(f[14])) for f in (faces if faces is not None else [])]
    total_faces = sum(1 for v in truths.values() for _, ignore in v if not ignore)
    print(f"faces {args.part}: {len(photos)} photos, {total_faces} labelled faces, {sum(seconds) / len(seconds):.2f} s/photo")
    report = {"part": args.part, "photos": len(photos), "labelled_faces": total_faces, "curve": []}
    for threshold in (0.3, 0.5, 0.7, 0.8, 0.9):
        by_width = {"<20 px": [0, 0], "20-40 px": [0, 0], ">=40 px": [0, 0]}
        fp = 0
        for image_id, boxes in truths.items():
            detections = sorted((p for p in preds[image_id] if p[1] >= threshold), key=lambda p: -p[1])
            used = set()
            for box, ignore in boxes:
                if ignore:
                    continue
                best, best_i = 0.0, None
                for i, (pbox, _) in enumerate(detections):
                    if i not in used and iou(box, pbox) > best:
                        best, best_i = iou(box, pbox), i
                width_px = box[2] - box[0]
                bucket = "<20 px" if width_px < 20 else ("20-40 px" if width_px < 40 else ">=40 px")
                by_width[bucket][1] += 1
                if best_i is not None and best >= 0.5:
                    used.add(best_i)
                    by_width[bucket][0] += 1
            for i, (pbox, _) in enumerate(detections):
                if i not in used and not any(iou(box, pbox) >= 0.3 and ignore for box, ignore in boxes):
                    fp += 1
        row = {"threshold": threshold, "fp_per_photo": fp / len(truths), "recall": {k: round(v[0] / max(1, v[1]), 3) for k, v in by_width.items()}, "counts": by_width}
        report["curve"].append(row)
        print(f"  score>={threshold:<4} false boxes/photo {row['fp_per_photo']:.2f}   recall {row['recall']}   (labelled {{{', '.join(f'{k}: {v[1]}' for k, v in by_width.items())}}})")
    # photos with no labelled face (the general set): every detection there is either a false box or an unlabelled face, so this is an upper bound of the false-box rate on face-free imagery
    negatives = [r for r in records if "general" in r["roles"] and partition(r["id"]) == args.part]
    if args.limit:
        negatives = negatives[:args.limit]
    counts = {t: 0 for t in (0.3, 0.5, 0.7, 0.8, 0.9)}
    for record in negatives:
        image = cv2.imread(str(Path(DATA) / record["file"]))
        height, width = image.shape[:2]
        detector.setInputSize((width, height))
        _, faces = detector.detect(image)
        for f in (faces if faces is not None else []):
            for t in counts:
                counts[t] += 1 if float(f[14]) >= t else 0
    report["negatives"] = {"photos": len(negatives), "detections_per_photo": {str(t): round(c / max(1, len(negatives)), 3) for t, c in counts.items()}}
    print(f"photos with no labelled face: {len(negatives)}; detections per photo by score: {report['negatives']['detections_per_photo']}")
    out = Path(DATA) / "results" / "faces"
    out.mkdir(parents=True, exist_ok=True)
    json.dump(report, open(out / f"faces-{args.part}-{time.strftime('%Y%m%d-%H%M%S')}.json", "w"), indent=1)


if __name__ == "__main__":
    main()
