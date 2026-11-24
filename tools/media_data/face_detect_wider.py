#!/usr/bin/env python3
"""Face detection on WIDER FACE validation (gates D1 and D2 of docs/data/face-image-search-preregistration-20261119.md): YuNet 2023mar and BlazeFace short range.

    docker run --rm --network none --memory 2500m -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro \
        --entrypoint python nexusai-media-eval-face:20261119 /repo/tools/media_data/face_detect_wider.py --detector yunet --part dev
    ... --detector yunet --part sealed --threshold <value chosen on dev>      (a sealed run REQUIRES the threshold and reports only that threshold)

Dev part: images whose SHA-256 of the file name ends in 0 or 1 (pre-registered). The score threshold is the lowest one with at most 0.1 false boxes per image on dev. Sealed: that single threshold, once.
Matching: detections in descending score; a detection takes the best unmatched valid labelled face with IoU >= 0.5; a detection matching nothing but overlapping a face flagged `invalid` (IoU >= 0.3) is ignored;
any other detection is a false box. Recall strata: width >= 40 px, 20 to 40 px, under 20 px; and, among faces at least 20 px wide: heavy blur (blur = 2), atypical pose (pose = 1), heavy occlusion (occlusion = 2).
Detections are cached under /work/results so scoring can be redone without re-reading the images; the cache of a sealed part is written once.
"""
import argparse
import json
import sys
import time
from pathlib import Path

import cv2
import numpy as np

sys.path.insert(0, "/repo/tools/media_data")
import face_common as fc  # noqa: E402

DEV_DIGITS = "01"


# ---------- BlazeFace (MediaPipe face_detection_short_range) -----------------------------------------------------
def blaze_anchors():
    """The 896 anchors of the short-range model: strides (8, 16, 16, 16) on a 128 input, 2 anchors per cell on the 16x16 map and 6 on the 8x8 map, fixed size 1."""
    anchors = []
    for fm, count in ((16, 2), (8, 6)):
        for y in range(fm):
            for x in range(fm):
                for _ in range(count):
                    anchors.append(((x + 0.5) / fm, (y + 0.5) / fm))
    return np.asarray(anchors, dtype=np.float32)


class BlazeFace:
    name = "blazeface"

    def __init__(self, threads=4):
        from ai_edge_litert.interpreter import Interpreter
        self.itp = Interpreter(model_path=fc.BLAZE, num_threads=threads)
        self.itp.allocate_tensors()
        self.inp = self.itp.get_input_details()[0]
        outs = self.itp.get_output_details()
        self.reg = next(o for o in outs if o["shape"][-1] == 16)
        self.cls = next(o for o in outs if o["shape"][-1] == 1)
        self.anchors = blaze_anchors()
        assert self.reg["shape"][1] == 896 and len(self.anchors) == 896, (self.reg["shape"], len(self.anchors))

    def detect(self, bgr, min_score=0.05):
        h, w = bgr.shape[:2]
        scale = 128.0 / max(h, w)
        nw, nh = max(1, round(w * scale)), max(1, round(h * scale))
        resized = cv2.resize(bgr, (nw, nh), interpolation=cv2.INTER_AREA if scale < 1 else cv2.INTER_LINEAR)
        canvas = np.zeros((128, 128, 3), np.uint8)
        top, left = (128 - nh) // 2, (128 - nw) // 2
        canvas[top:top + nh, left:left + nw] = resized
        rgb = canvas[:, :, ::-1].astype(np.float32) / 127.5 - 1.0
        self.itp.set_tensor(self.inp["index"], rgb[None])
        self.itp.invoke()
        raw = self.itp.get_tensor(self.reg["index"])[0]
        logit = np.clip(self.itp.get_tensor(self.cls["index"])[0, :, 0], -100, 100)
        score = 1.0 / (1.0 + np.exp(-logit))
        keep = score >= min_score
        if not keep.any():
            return []
        cx = raw[keep, 0] / 128.0 + self.anchors[keep, 0]
        cy = raw[keep, 1] / 128.0 + self.anchors[keep, 1]
        bw, bh = raw[keep, 2] / 128.0, raw[keep, 3] / 128.0
        boxes = np.stack([cx - bw / 2, cy - bh / 2, cx + bw / 2, cy + bh / 2], axis=1) * 128.0  # in the padded 128 canvas
        boxes[:, [0, 2]] = (boxes[:, [0, 2]] - left) / scale
        boxes[:, [1, 3]] = (boxes[:, [1, 3]] - top) / scale
        return weighted_nms(boxes, score[keep])


def weighted_nms(boxes, scores, iou_thr=0.3):
    order = np.argsort(-scores)
    boxes, scores = boxes[order], scores[order]
    used = np.zeros(len(scores), bool)
    out = []
    for i in range(len(scores)):
        if used[i]:
            continue
        group = [i]
        for j in range(i + 1, len(scores)):
            if not used[j] and fc.iou(boxes[i], boxes[j]) > iou_thr:
                group.append(j)
        used[group] = True
        weights = scores[group]
        merged = (boxes[group] * weights[:, None]).sum(axis=0) / weights.sum()
        out.append((float(merged[0]), float(merged[1]), float(merged[2]), float(merged[3]), float(scores[i])))
    return out


# ---------- YuNet --------------------------------------------------------------------------------------------------
class YuNet:
    name = "yunet"

    def __init__(self):
        cv2.setNumThreads(4)
        self.det = fc.yunet(score=0.05, nms=0.3, top_k=5000)

    def detect(self, bgr, min_score=0.05):
        h, w = bgr.shape[:2]
        self.det.setInputSize((w, h))
        _, faces = self.det.detect(bgr)
        if faces is None:
            return []
        return [(float(f[0]), float(f[1]), float(f[0] + f[2]), float(f[1] + f[3]), float(f[14])) for f in faces]


# ---------- scoring ------------------------------------------------------------------------------------------------
def score_part(records, dets, threshold):
    strata = {"w>=40": [0, 0], "20<=w<40": [0, 0], "w<20": [0, 0], "blur2 (w>=20)": [0, 0], "pose1 (w>=20)": [0, 0], "occl2 (w>=20)": [0, 0]}
    false_boxes = 0
    for rec in records:
        faces = rec["faces"]
        valid = [f for f in faces if not f["invalid"] and f["w"] > 0 and f["h"] > 0]
        invalid = [f for f in faces if f["invalid"] and f["w"] > 0 and f["h"] > 0]
        gt = [(f["x"], f["y"], f["x"] + f["w"], f["y"] + f["h"]) for f in valid]
        bad = [(f["x"], f["y"], f["x"] + f["w"], f["y"] + f["h"]) for f in invalid]
        matched = [False] * len(valid)
        for d in sorted((d for d in dets[rec["name"]] if d[4] >= threshold), key=lambda d: -d[4]):
            best, bi = 0.0, -1
            for i, g in enumerate(gt):
                if not matched[i]:
                    v = fc.iou(d[:4], g)
                    if v > best:
                        best, bi = v, i
            if bi >= 0 and best >= 0.5:
                matched[bi] = True
            elif any(fc.iou(d[:4], b) >= 0.3 for b in bad):
                pass
            else:
                false_boxes += 1
        for f, hit in zip(valid, matched):
            keys = ["w>=40" if f["w"] >= 40 else ("20<=w<40" if f["w"] >= 20 else "w<20")]
            if f["w"] >= 20:
                if f["blur"] == 2:
                    keys.append("blur2 (w>=20)")
                if f["pose"] == 1:
                    keys.append("pose1 (w>=20)")
                if f["occlusion"] == 2:
                    keys.append("occl2 (w>=20)")
            for k in keys:
                strata[k][1] += 1
                strata[k][0] += 1 if hit else 0
    return {"threshold": threshold, "images": len(records), "false_boxes_per_image": false_boxes / max(1, len(records)),
            "recall": {k: (round(100.0 * v[0] / v[1], 2) if v[1] else None) for k, v in strata.items()}, "counts": {k: v for k, v in strata.items()}}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--detector", choices=("yunet", "blazeface"), required=True)
    parser.add_argument("--part", choices=("dev", "sealed"), required=True)
    parser.add_argument("--threshold", type=float, default=None, help="required for the sealed part: the value chosen on dev")
    parser.add_argument("--limit", type=int, default=0)
    args = parser.parse_args()
    if args.part == "sealed" and args.threshold is None:
        sys.exit("a sealed run needs the threshold chosen on dev")
    records = json.load(open(Path(fc.WORK) / "widerface" / "wider-val.json"))
    records = [r for r in records if fc.partition(r["name"], DEV_DIGITS) == args.part]
    if args.limit:
        records = records[:args.limit]
    results = Path(fc.WORK) / "results"
    results.mkdir(parents=True, exist_ok=True)
    cache = results / f"wider-det-{args.detector}-{args.part}.json"
    if args.part == "sealed" and cache.exists() and not args.limit:
        sys.exit(f"{cache} exists: a sealed part is read once per detector")
    detector = YuNet() if args.detector == "yunet" else BlazeFace()
    dets, seconds = {}, []
    for k, rec in enumerate(records):
        image = cv2.imread(rec["file"])
        t0 = time.perf_counter()
        dets[rec["name"]] = detector.detect(image)
        seconds.append(time.perf_counter() - t0)
        if k % 200 == 0:
            print(f"  {args.detector} {args.part}: {k}/{len(records)}, rss {fc.rss_mb():.0f} MB", flush=True)
    if not args.limit:
        json.dump(dets, open(cache, "w"))
    median_ms = round(1000 * float(np.median(seconds)), 2)
    if args.part == "dev":
        curve = []
        for t in (0.05, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.75, 0.8, 0.82, 0.84, 0.86, 0.88, 0.9, 0.92, 0.94, 0.96, 0.98):
            curve.append(score_part(records, dets, t))
            print(f"  dev score>={t}: false boxes/image {curve[-1]['false_boxes_per_image']:.3f}  recall {curve[-1]['recall']}", flush=True)
        eligible = [c for c in curve if c["false_boxes_per_image"] <= 0.1]
        chosen = min(eligible, key=lambda c: c["threshold"]) if eligible else None
        report = {"detector": args.detector, "part": "dev", "median_ms_per_image": median_ms, "peak_rss_mb": round(fc.rss_mb()), "curve": curve, "chosen_threshold": None if chosen is None else chosen["threshold"]}
        print("CHOSEN threshold on dev:", None if chosen is None else chosen["threshold"], "median ms/image", median_ms)
    else:
        row = score_part(records, dets, args.threshold)
        report = {"detector": args.detector, "part": "sealed", "median_ms_per_image": median_ms, "peak_rss_mb": round(fc.rss_mb()), "result": row}
        print("SEALED result:", json.dumps(row))
    if not args.limit:
        json.dump(report, open(results / f"wider-{args.detector}-{args.part}-report.json", "w"), indent=1)


if __name__ == "__main__":
    main()
