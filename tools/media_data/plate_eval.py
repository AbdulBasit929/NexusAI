#!/usr/bin/env python3
"""Plate measurement for track P (docs/data/media-activation-programme-20261021.md): the worker's own FastALPR stack against (1) real Open Images photos with plate boxes and (2) the generated
Pakistani-format scenes with exact truth.

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest \
        python /repo/tools/media_data/plate_eval.py --part dev

Detection (real photos, no plate text exists in Open Images): per confidence threshold, recall at IoU 0.5 over the plate boxes that are neither groups nor depictions, false boxes per image,
image-level presence recall, and recall by plate width in pixels (the detector's reach on small plates). Reading (generated scenes): detection recall at IoU 0.5, exact normalised read among the
detected plates, character error, accuracy by confidence band (is confidence at least 0.9 really accurate?), and the most frequent character confusions. Detector and reader are exactly the
worker's assets (paths from the worker's environment); only the detection threshold is lowered to trace the curve (the product uses 0.75).
"""
import argparse
import collections
import hashlib
import json
import os
import re
import sys
import time
from pathlib import Path

import cv2
import editdistance

sys.path.insert(0, "/repo/ingestion/forensic_records")
DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
os.environ.setdefault("FORENSIC_ANPR_DETECTOR_MODEL_PATH", "/models/media/open-image-models/yolo-v9-t-384-license-plate-end2end/yolo-v9-t-384-license-plates-end2end.onnx")
os.environ.setdefault("FORENSIC_ANPR_OCR_MODEL_PATH", "/models/media/fast-plate-ocr/cct-xs-v2-global-model/cct_xs_v2_global.onnx")
os.environ.setdefault("FORENSIC_ANPR_OCR_CONFIG_PATH", "/models/media/fast-plate-ocr/cct-xs-v2-global-model/cct_xs_v2_global_plate_config.yaml")
os.environ.setdefault("FORENSIC_ANPR_ONNX_PROVIDERS", "OpenVINOExecutionProvider,CPUExecutionProvider")


def partition(key):
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def iou(a, b):
    ix0, iy0, ix1, iy1 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0.0, ix1 - ix0) * max(0.0, iy1 - iy0)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union > 0 else 0.0


def predict(engine, path):
    frame = cv2.imread(str(path))
    t0 = time.time()
    results = engine.predict(frame)
    seconds = time.time() - t0
    out = []
    for r in results:
        b = r.detection.bounding_box
        ocr = r.ocr
        conf = ocr.confidence if ocr is not None else None
        mean_conf = (sum(conf) / len(conf)) if isinstance(conf, (list, tuple)) and conf else (conf if isinstance(conf, (int, float)) else None)
        out.append({"box": (float(b.x1), float(b.y1), float(b.x2), float(b.y2)), "conf": float(r.detection.confidence), "text": (ocr.text if ocr is not None else ""), "ocr_conf": mean_conf})
    return out, seconds


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--part", default="dev", choices=("dev", "sealed"))
    parser.add_argument("--limit", type=int, default=0)
    args = parser.parse_args()
    import media_pipeline
    processor = media_pipeline.FastALPRImageProcessor.from_environment(minimum_confidence=0.05)
    engine = processor.engine
    report = {"part": args.part}

    # (1) detection on real photos
    records = [json.loads(line) for line in open(Path(DATA) / "openimages" / "subset-test.jsonl", encoding="utf-8")]
    photos = [r for r in records if "plate" in r["roles"] and partition(r["id"]) == args.part]
    if args.limit:
        photos = photos[:args.limit]
    preds, truth_boxes, secs = {}, {}, []
    for record in photos:
        path = Path(DATA) / record["file"]
        frame = cv2.imread(str(path))
        height, width = frame.shape[:2]
        truth_boxes[record["id"]] = [((b["x0"] * width, b["y0"] * height, b["x1"] * width, b["y1"] * height), b["group"] or b["depiction"]) for b in record["plates"]]
        preds[record["id"]], s = predict(engine, path)
        secs.append(s)
    print(f"real photos {args.part}: {len(photos)} images, {sum(len([1 for _, ignore in v if not ignore]) for v in truth_boxes.values())} plate boxes, {sum(secs) / len(secs):.2f} s/image")
    curve = []
    for threshold in (0.05, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.75, 0.9):
        tp = fn = fp = present_hits = present_total = 0
        by_width = collections.defaultdict(lambda: [0, 0])
        for image_id, boxes in truth_boxes.items():
            detections = sorted((p for p in preds[image_id] if p["conf"] >= threshold), key=lambda p: -p["conf"])
            used = set()
            image_hit = False
            for box, ignore in boxes:
                if ignore:
                    continue
                best, best_i = 0.0, None
                for i, p in enumerate(detections):
                    if i not in used and iou(box, p["box"]) > best:
                        best, best_i = iou(box, p["box"]), i
                width_px = box[2] - box[0]
                bucket = "<40 px" if width_px < 40 else ("40-100 px" if width_px < 100 else ">=100 px")
                by_width[bucket][1] += 1
                if best_i is not None and best >= 0.5:
                    used.add(best_i)
                    tp += 1
                    by_width[bucket][0] += 1
                    image_hit = True
                else:
                    fn += 1
            for i, p in enumerate(detections):
                if i not in used and not any(iou(box, p["box"]) >= 0.3 and ignore for box, ignore in boxes):
                    fp += 1
            if any(not ignore for _, ignore in boxes):
                present_total += 1
                present_hits += 1 if image_hit else 0
        curve.append({"threshold": threshold, "recall": tp / max(1, tp + fn), "fp_per_image": fp / len(truth_boxes), "image_recall": present_hits / max(1, present_total), "by_width": {k: round(v[0] / max(1, v[1]), 3) for k, v in by_width.items()}})
        print(f"  conf>={threshold:<4}  box recall {100 * curve[-1]['recall']:5.1f}%  false boxes/image {curve[-1]['fp_per_image']:.2f}  image recall {100 * curve[-1]['image_recall']:5.1f}%  by width {curve[-1]['by_width']}")
    report["detection_real_photos"] = {"images": len(photos), "curve": curve, "seconds_per_image": sum(secs) / len(secs)}

    # (2) detection and reading on generated Pakistani-format scenes
    folder = Path(DATA) / "generated" / "plates"
    scenes = [json.loads(line) for line in open(folder / "truth.jsonl", encoding="utf-8")]
    scenes = [s for s in scenes if s["partition"] == args.part]
    if args.limit:
        scenes = scenes[:args.limit]
    detected = exact = 0
    cers, rows, confusions = [], [], collections.Counter()
    for scene in scenes:
        detections, _ = predict(engine, folder / scene["file"])
        detections = [d for d in detections if d["conf"] >= 0.5]
        best = max(detections, key=lambda d: iou(tuple(scene["box"]), d["box"]), default=None)
        if best is None or iou(tuple(scene["box"]), best["box"]) < 0.5:
            rows.append({"id": scene["id"], "detected": False})
            continue
        detected += 1
        text = re.sub(r"[^A-Z0-9]", "", best["text"].upper())
        truth = scene["normalised"]
        ok = text == truth
        exact += 1 if ok else 0
        cers.append(editdistance.eval(truth, text) / len(truth))
        for a, b in zip(truth, text) if len(truth) == len(text) else ():
            if a != b:
                confusions[(a, b)] += 1
        rows.append({"id": scene["id"], "detected": True, "exact": ok, "conf": best["ocr_conf"], "det_conf": best["conf"]})
    print(f"\ngenerated scenes {args.part}: {len(scenes)}  detected {detected} ({100 * detected / max(1, len(scenes)):.1f}%)  exact among detected {exact}/{detected} ({100 * exact / max(1, detected):.1f}%)  exact over all {100 * exact / max(1, len(scenes)):.1f}%  CER {100 * sum(cers) / max(1, len(cers)):.1f}%")
    bands = collections.defaultdict(lambda: [0, 0])
    for r in rows:
        if r.get("detected") and r.get("conf") is not None:
            band = "conf>=0.9" if r["conf"] >= 0.9 else ("0.7-0.9" if r["conf"] >= 0.7 else "<0.7")
            bands[band][1] += 1
            bands[band][0] += 1 if r["exact"] else 0
    print("  accuracy by OCR confidence band:", {k: f"{v[0]}/{v[1]}" for k, v in sorted(bands.items())})
    print("  most frequent confusions (truth->read):", confusions.most_common(8))
    report["reading_generated"] = {"scenes": len(scenes), "detected": detected, "exact": exact, "cer": sum(cers) / max(1, len(cers)), "bands": {k: v for k, v in bands.items()}, "confusions": [[f"{a}->{b}", n] for (a, b), n in confusions.most_common(15)]}
    out = Path(DATA) / "results" / "plates"
    out.mkdir(parents=True, exist_ok=True)
    json.dump(report, open(out / f"plates-{args.part}-{time.strftime('%Y%m%d-%H%M%S')}.json", "w"), indent=1)


if __name__ == "__main__":
    main()
