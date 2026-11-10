#!/usr/bin/env python3
"""Plate detector box recall on real photographs (docs/data/plates-m5-preregistration-20261110.md, gate 2): the published RF-DETR detector against the incumbent YOLOv9-t, on the Open Images photos
(CC BY 2.0, DAT-018) of the SEALED split (SHA-256 of "plate-<photo id>", last hex digit c to f), real plate boxes untouched, photos with a group or depiction box left out.

    docker run --rm --network none --cpus 8 -m 2g --entrypoint python -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro -v <nxmmr models>:/nxmmr:ro \
        nexusai-media-eval:latest /repo/tools/media_data/plate_eval_detect.py [--split sealed|val]

Reported per detector at its product threshold (RF-DETR 0.5, incumbent 0.75) and at a few others: recall at IoU 0.5 over the usable plate boxes, recall by plate width (under 40, 40 to 100, 100 and over
pixels), image-level recall, detections per image, and false boxes per image (a detection that matches no labelled box; Open Images labels are not exhaustive, so this over-counts). Aggregates only.
"""
import argparse
import hashlib
import json
import os
import sys
import time
from pathlib import Path

import cv2

sys.path.insert(0, os.environ.get("NEXUSAI_INGEST", "/repo/ingestion/forensic_records"))
import plate_detector_onnx as pd  # noqa: E402

DATA = Path("/data")


def split_of(photo_id):
    digit = hashlib.sha256(f"plate-{photo_id}".encode()).hexdigest()[-1]
    return "train" if digit in "0123456789" else "val" if digit in "ab" else "sealed"


def iou(a, b):
    ix0, iy0, ix1, iy1 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0.0, ix1 - ix0) * max(0.0, iy1 - iy0)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union > 0 else 0.0


def bin_of(width):
    return "<40 px" if width < 40 else "40-100 px" if width < 100 else ">=100 px"


def evaluate(name, detect, photos, thresholds):
    rows = {t: {"boxes": 0, "hit": 0, "bins": {}, "images": 0, "image_hit": 0, "dets": 0, "false": 0} for t in thresholds}
    started = time.time()
    for photo, truth in photos:
        detections = detect(photo)  # (x1, y1, x2, y2, score) at the lowest threshold
        for t in thresholds:
            kept = [d for d in detections if d[4] >= t]
            r = rows[t]
            r["images"] += 1
            r["dets"] += len(kept)
            matched = set()
            image_hit = False
            for box in truth:
                width = box[2] - box[0]
                b = r["bins"].setdefault(bin_of(width), [0, 0])
                b[1] += 1
                r["boxes"] += 1
                if any(iou(box, d[:4]) >= 0.5 for d in kept):
                    r["hit"] += 1
                    b[0] += 1
                    image_hit = True
            for i, d in enumerate(kept):
                if not any(iou(box, d[:4]) >= 0.5 for box in truth):
                    r["false"] += 1
            r["image_hit"] += image_hit
    seconds = (time.time() - started) / max(1, len(photos))
    out = []
    for t in thresholds:
        r = rows[t]
        out.append({"detector": name, "threshold": t, "recall_iou50": round(r["hit"] / max(1, r["boxes"]), 4), "boxes": r["boxes"], "image_recall": round(r["image_hit"] / max(1, r["images"]), 4),
                    "by_width": {k: [v[0], v[1]] for k, v in sorted(r["bins"].items())}, "detections_per_image": round(r["dets"] / max(1, r["images"]), 2),
                    "false_boxes_per_image_overcount": round(r["false"] / max(1, r["images"]), 3), "seconds_per_image": round(seconds, 3)})
    return out


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--split", default="sealed", choices=["sealed", "val", "train"])
    parser.add_argument("--rfdetr", default="/data/models/plates/rfdetr-alpr/rfdetr_alpr.onnx")
    parser.add_argument("--incumbent", default="/nxmmr/yolo-v9-t-384-license-plates-end2end.onnx")
    args = parser.parse_args()
    photos = []
    for line in open(DATA / "openimages" / "subset-test.jsonl", encoding="utf-8"):
        record = json.loads(line)
        if not record["plates"] or any(b["group"] or b["depiction"] for b in record["plates"]) or split_of(record["id"]) != args.split:
            continue
        image = cv2.imread(str(DATA / record["file"]))
        if image is None:
            continue
        h, w = image.shape[:2]
        photos.append((image, [(b["x0"] * w, b["y0"] * h, b["x1"] * w, b["y1"] * h) for b in record["plates"]]))
    print(f"split {args.split}: {len(photos)} photos, {sum(len(t) for _, t in photos)} plate boxes", flush=True)
    results = []
    mine = pd.OnnxPlateDetector(args.rfdetr, cv2, normalise="unit", threshold=0.05)
    results += evaluate("rfdetr-alpr", lambda im: mine.detect(im), photos, [0.3, 0.5, 0.7])
    mine.drop_non_plates = True
    results += evaluate("rfdetr-alpr + non-plate rule", lambda im: mine.detect(im), photos, [0.3, 0.5, 0.7])
    from open_image_models import create_detector
    incumbent = create_detector(args.incumbent, backend="yolo_v9", class_labels={0: "license_plate"}, conf_thresh=0.05, providers=["CPUExecutionProvider"])

    def run_incumbent(image):
        return [(int(r.bounding_box.x1), int(r.bounding_box.y1), int(r.bounding_box.x2), int(r.bounding_box.y2), float(r.confidence)) for r in incumbent.predict(image)]
    results += evaluate("yolo-v9-t incumbent", run_incumbent, photos, [0.3, 0.5, 0.75])
    out = DATA / "results" / "plates"
    out.mkdir(parents=True, exist_ok=True)
    stamp = time.strftime("%Y%m%d-%H%M%S")
    (out / f"detect-{args.split}-{stamp}.json").write_text(json.dumps({"photos": len(photos), "results": results}, indent=1), encoding="utf-8")
    for r in results:
        print(json.dumps(r))


if __name__ == "__main__":
    main()
