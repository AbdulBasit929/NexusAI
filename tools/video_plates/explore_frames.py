#!/usr/bin/env python3
"""Exploration on a few frames of one video: how many plates does the detector find on the whole frame versus on tiles or on crops around vehicles, and what do they cost?

    docker run --rm --network none --entrypoint python -v <repo>:/repo:ro -v <repo>/models/media:/models/media:ro -v <video dir>:/v:ro -v C:/tmp/video-plates:/clips \
        nexusai-media-eval:latest /repo/tools/video_plates/explore_frames.py --video /v/sample.mp4 --times 5,20,35,50 --tag sample

Local only: crops are written under /clips/explore/<tag>/ (outside the repository). Only counts and sizes are printed; plate text is not.
"""
import argparse
import os
import sys
import time

os.environ.setdefault("FORENSIC_ANPR_ONNX_PROVIDERS", "CPUExecutionProvider")
sys.path.insert(0, "/repo/ingestion/forensic_records")
sys.path.insert(0, "/repo/tools/media_data")

import cv2  # noqa: E402
import numpy as np  # noqa: E402
import onnxruntime  # noqa: E402,F401
from plate_detector_onnx import OnnxPlateDetector  # noqa: E402

DETECTOR = "/models/media/nexus-plates/rfdetr_alpr.onnx"
OCR = "/models/media/nexus-plates/cct_s_v2_global.onnx"
OCR_CONFIG = "/models/media/nexus-plates/cct_s_v2_global_plate_config.yaml"


def iou(a, b):
    x1, y1, x2, y2 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0, x2 - x1) * max(0, y2 - y1)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union else 0.0


def merge(boxes, threshold=0.3):
    out = []
    for box in sorted(boxes, key=lambda b: -b[4]):
        if all(iou(box, kept) < threshold for kept in out):
            out.append(box)
    return out


def tiles(width, height, side, stride):
    xs = list(range(0, max(1, width - side) + 1, stride))
    ys = list(range(0, max(1, height - side) + 1, stride))
    if xs[-1] + side < width:
        xs.append(width - side)
    if ys[-1] + side < height:
        ys.append(height - side)
    return [(x, y) for y in ys for x in xs]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--video", required=True)
    parser.add_argument("--times", default="5,20,35,50")
    parser.add_argument("--tag", required=True)
    parser.add_argument("--threshold", type=float, default=0.5)
    parser.add_argument("--tile", type=int, nargs="*", default=[1280, 960])
    args = parser.parse_args()
    plate_det = OnnxPlateDetector(DETECTOR, cv2, providers=["CPUExecutionProvider"], normalise="unit", threshold=args.threshold, drop_non_plates=True)
    any_det = OnnxPlateDetector(DETECTOR, cv2, providers=["CPUExecutionProvider"], normalise="unit", threshold=args.threshold, drop_non_plates=False)
    from fast_alpr.default_ocr import DefaultOCR
    ocr = DefaultOCR(hub_ocr_model=None, device="cpu", providers=["CPUExecutionProvider"], model_path=OCR, config_path=OCR_CONFIG, force_download=False)
    capture = cv2.VideoCapture(args.video)
    fps = capture.get(cv2.CAP_PROP_FPS)
    out_dir = f"/clips/explore/{args.tag}"
    os.makedirs(out_dir, exist_ok=True)
    print(f"video {int(capture.get(cv2.CAP_PROP_FRAME_WIDTH))}x{int(capture.get(cv2.CAP_PROP_FRAME_HEIGHT))} {fps:.1f} fps {capture.get(cv2.CAP_PROP_FRAME_COUNT) / fps:.1f} s")
    for t in [float(v) for v in args.times.split(",")]:
        capture.set(cv2.CAP_PROP_POS_FRAMES, round(t * fps))
        ok, frame = capture.read()
        if not ok:
            print("t", t, "could not read")
            continue
        height, width = frame.shape[:2]
        results = {}
        started = time.perf_counter()
        results["whole"] = (plate_det.detect(frame), time.perf_counter() - started, 1)
        for side in args.tile:
            started = time.perf_counter()
            found, calls = [], 0
            for x, y in tiles(width, height, side, int(side * 0.8)):
                calls += 1
                for x1, y1, x2, y2, score in plate_det.detect(frame[y:y + side, x:x + side]):
                    found.append((x1 + x, y1 + y, x2 + x, y2 + y, score))
            results[f"tile{side}"] = (merge(found), time.perf_counter() - started, calls)
        # crops around vehicles: the detector also boxes whole vehicles; take those boxes (with a margin) and look for plates inside at native resolution
        started = time.perf_counter()
        vehicles = [b for b in any_det.detect(frame) if (b[2] - b[0]) * (b[3] - b[1]) > 0.0004 * width * height]
        found, calls = [], 1
        for x1, y1, x2, y2, _ in vehicles:
            mx, my = int((x2 - x1) * 0.15), int((y2 - y1) * 0.15)
            cx1, cy1, cx2, cy2 = max(0, x1 - mx), max(0, y1 - my), min(width, x2 + mx), min(height, y2 + my)
            calls += 1
            for a1, b1, a2, b2, score in plate_det.detect(frame[cy1:cy2, cx1:cx2]):
                found.append((a1 + cx1, b1 + cy1, a2 + cx1, b2 + cy1, score))
        results["vehicle_crops"] = (merge(found), time.perf_counter() - started, calls)
        for mode, (boxes, seconds, calls) in results.items():
            widths = sorted((b[2] - b[0]) for b in boxes)
            reads = 0
            for i, (x1, y1, x2, y2, score) in enumerate(sorted(boxes, key=lambda b: -b[4])[:10]):
                crop = frame[y1:y2, x1:x2]
                if crop.size == 0:
                    continue
                read = ocr.predict(crop)
                reads += bool(read and read.text and len("".join(ch for ch in read.text if ch.isalnum())) >= 4)
                cv2.imwrite(f"{out_dir}/t{int(t)}-{mode}-{i}.png", cv2.resize(crop, None, fx=2, fy=2, interpolation=cv2.INTER_CUBIC))
            print(f"t={t:>5}s {mode:14} boxes={len(boxes):3} plate_widths_px={widths[:3]}..{widths[-3:]} median_w={widths[len(widths)//2] if widths else None} reads>=4chars(top10)={reads} calls={calls} {seconds:.1f}s", flush=True)


if __name__ == "__main__":
    main()
