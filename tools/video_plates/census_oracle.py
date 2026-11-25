#!/usr/bin/env python3
"""Plate census of a real clip: reads the clip at maximum effort so that a person can say how many plates were readable (docs/data/video-plates-recall-round2-20261125.md, protocol A1).

Two phases, both writing under --out (outside the repository; the files hold plate text of real footage and are private):

    detect   for each sampled frame: the fast detector (YOLO v9-t 384, licence-clean) on overlapping native tiles at two scales and on the whole frame, the accurate detector (RF-DETR) on the
             whole frame, boxes merged, each kept box read by both readers on a widened crop. -> <tag>-detections.jsonl
    sheets   links boxes over frames into candidates (a plate on consecutive frames), picks the best crop of each (size x sharpness) and writes contact sheets <tag>-sheet-NN.png with numbered
             crops, and <tag>-candidates.json (per-frame boxes and every reading, for the comparison with the stored production reads).

    docker run --rm --network none --memory 2300m --entrypoint python -v <repo>:/repo:ro -v <repo>/models/media:/models/media:ro -v <video dir>:/v:ro -v C:/tmp/video-plates:/clips \
        nexusai-media-eval:latest /repo/tools/video_plates/census_oracle.py detect --video /v/clip.mp4 --tag clip1 --fps 4 --max-seconds 20

Plate text is never printed. The census is a lower bound: a plate neither detector sees and a person cannot see is not in it.
"""
import argparse
import json
import math
import os
import sys
import time

os.environ.setdefault("FORENSIC_ANPR_ONNX_PROVIDERS", "CPUExecutionProvider")
os.environ.setdefault("FORENSIC_ANPR_NEXUS_DETECTOR_MODEL_PATH", "/models/media/nexus-plates/rfdetr_alpr.onnx")
os.environ.setdefault("FORENSIC_ANPR_NEXUS_OCR_MODEL_PATH", "/models/media/nexus-plates/cct_s_v2_global.onnx")
os.environ.setdefault("FORENSIC_ANPR_NEXUS_OCR_CONFIG_PATH", "/models/media/nexus-plates/cct_s_v2_global_plate_config.yaml")
os.environ.setdefault("FORENSIC_ANPR_DETECTOR_MODEL_PATH", "/models/media/open-image-models/yolo-v9-t-384-license-plate-end2end/yolo-v9-t-384-license-plates-end2end.onnx")
os.environ.setdefault("FORENSIC_ANPR_OCR_MODEL_PATH", "/models/media/fast-plate-ocr/cct-xs-v2-global-model/cct_xs_v2_global.onnx")
os.environ.setdefault("FORENSIC_ANPR_OCR_CONFIG_PATH", "/models/media/fast-plate-ocr/cct-xs-v2-global-model/cct_xs_v2_global_plate_config.yaml")
sys.path.insert(0, "/repo")
sys.path.insert(0, "/repo/ingestion/forensic_records")

import cv2  # noqa: E402
import numpy as np  # noqa: E402


def norm(text):
    return "".join(ch for ch in (text or "").upper() if ch.isalnum())


def iou(a, b):
    x1, y1, x2, y2 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0, x2 - x1) * max(0, y2 - y1)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union else 0.0


def merge(boxes, threshold=0.3):
    """Boxes (x1, y1, x2, y2, score, source); of overlapping ones keep the best score and remember every source."""
    kept = []
    for box in sorted(boxes, key=lambda b: -b[4]):
        for k in kept:
            if iou(box, k["box"]) >= threshold:
                k["sources"].add(box[5])
                break
        else:
            kept.append({"box": box[:5], "sources": {box[5]}})
    return kept


def tiles(width, height, side, stride):
    xs = list(range(0, max(1, width - side) + 1, stride))
    ys = list(range(0, max(1, height - side) + 1, stride))
    if xs[-1] + side < width:
        xs.append(max(0, width - side))
    if ys[-1] + side < height:
        ys.append(max(0, height - side))
    return [(x, y) for y in ys for x in xs]


def sharpness(crop):
    gray = cv2.cvtColor(crop, cv2.COLOR_BGR2GRAY) if crop.ndim == 3 else crop
    h, w = gray.shape[:2]
    if h < 2 or w < 2:
        return 0.0
    g = cv2.resize(gray, (max(2, int(round(w * 32.0 / h))), 32), interpolation=cv2.INTER_AREA)
    return float(cv2.Laplacian(g, cv2.CV_64F).var())


class FastDetector:
    def __init__(self, processor):
        self.detector = processor.engine.detector
        self.ocr = processor.engine.ocr

    def detect(self, image):
        out = []
        for r in self.detector.predict(image):
            b = r.bounding_box
            out.append((int(b.x1), int(b.y1), int(b.x2), int(b.y2), float(r.confidence)))
        return out


def read_text(ocr, crop):
    try:
        r = ocr.predict(crop)
    except Exception:  # a crop the reader cannot take
        return None, 0.0
    if r is None:
        return None, 0.0
    conf = r.confidence
    if isinstance(conf, (list, tuple, np.ndarray)):
        conf = float(np.mean(conf)) if len(conf) else 0.0
    return norm(r.text), float(conf or 0.0)


def detect_phase(args):
    from plate_processor_nexus import NexusPlateImageProcessor
    from video_anpr_nexus_v4 import default_engine_processor
    accurate = NexusPlateImageProcessor.from_environment(minimum_confidence=args.accurate_threshold)
    fast = FastDetector(default_engine_processor(minimum_confidence=args.fast_threshold))
    capture = cv2.VideoCapture(args.video)
    fps_src, count = capture.get(cv2.CAP_PROP_FPS), int(capture.get(cv2.CAP_PROP_FRAME_COUNT))
    width, height = int(capture.get(3)), int(capture.get(4))
    end = min(count / fps_src, args.start_seconds + args.max_seconds) if args.max_seconds else count / fps_src
    duration = end - args.start_seconds
    os.makedirs(args.out, exist_ok=True)
    path = os.path.join(args.out, f"{args.tag}-detections.jsonl")
    n_frames = int(duration * args.fps)
    print(f"video {os.path.basename(args.video)} {width}x{height} {fps_src:.1f} fps; {n_frames} frames at {args.fps}/s from {args.start_seconds:.0f} s over {duration:.1f} s", flush=True)
    started = time.perf_counter()
    with open(path, "w", encoding="utf-8") as sink:
        for i in range(n_frames):
            t = args.start_seconds + i / args.fps
            number = round(t * fps_src)
            capture.set(cv2.CAP_PROP_POS_FRAMES, number)
            ok, frame = capture.read()
            if not ok:
                continue
            boxes = []
            for scale in args.scales:
                img = frame if scale == 1.0 else cv2.resize(frame, None, fx=scale, fy=scale, interpolation=cv2.INTER_AREA)
                h, w = img.shape[:2]
                side = min(args.tile, w, h)
                for x, y in tiles(w, h, side, int(side * 0.75)):
                    for x1, y1, x2, y2, s in fast.detect(img[y:y + side, x:x + side]):
                        boxes.append((int((x1 + x) / scale), int((y1 + y) / scale), int((x2 + x) / scale), int((y2 + y) / scale), s, f"fast_tile_{scale}"))
            for x1, y1, x2, y2, s in fast.detect(frame):
                boxes.append((x1, y1, x2, y2, s, "fast_whole"))
            for x1, y1, x2, y2, s in accurate.engine.detector.detect(frame):
                boxes.append((x1, y1, x2, y2, s, "accurate_whole"))
            for k in merge(boxes):
                x1, y1, x2, y2, score = k["box"]
                bw, bh = x2 - x1, y2 - y1
                if bw < args.min_width or bh < 4 or bw / max(1, bh) < 1.3 or bw / max(1, bh) > 8:
                    continue
                pad = int(0.08 * bw)
                crop = frame[max(0, y1 - pad // 2):min(height, y2 + pad // 2), max(0, x1 - pad):min(width, x2 + pad)]
                tight = frame[max(0, y1):min(height, y2), max(0, x1):min(width, x2)]
                if tight.size == 0:
                    continue
                t1, c1 = read_text(accurate.engine.ocr, tight)
                t2, c2 = read_text(fast.ocr, tight)
                t3, c3 = read_text(accurate.engine.ocr, crop)
                sink.write(json.dumps({"i": i, "t": round(t, 3), "frame": number, "box": [x1, y1, x2, y2], "score": round(score, 3), "sources": sorted(k["sources"]),
                                       "sharp": round(sharpness(tight), 1), "reads": [{"r": "accurate", "text": t1, "c": round(c1, 3)}, {"r": "fast", "text": t2, "c": round(c2, 3)},
                                                                                       {"r": "accurate_padded", "text": t3, "c": round(c3, 3)}]}) + "\n")
            if i % 10 == 0:
                print(f"  frame {i}/{n_frames}  {time.perf_counter() - started:.0f} s", flush=True)
    capture.release()
    print(f"detect done in {time.perf_counter() - started:.0f} s -> {path}")


def link(rows, max_gap_s, fps):
    """Boxes of consecutive sampled frames at about the same place and size are one candidate (a plate seen on several frames)."""
    tracks = []
    for row in sorted(rows, key=lambda r: (r["i"], r["box"][0])):
        x1, y1, x2, y2 = row["box"]
        w, cx, cy = x2 - x1, (x1 + x2) / 2.0, (y1 + y2) / 2.0
        best, best_d = None, None
        for tr in tracks:
            last = tr["rows"][-1]
            if row["t"] - last["t"] > max_gap_s or row["t"] == last["t"]:
                continue
            lx1, ly1, lx2, ly2 = last["box"]
            lw = lx2 - lx1
            if not 0.5 <= w / max(1, lw) <= 2.0:
                continue
            # allow for motion: a plate moves at most about a few of its own widths between two sampled frames
            d = math.hypot(cx - (lx1 + lx2) / 2.0, cy - (ly1 + ly2) / 2.0)
            if d <= 4.0 * max(w, lw) * max(1.0, (row["t"] - last["t"]) * fps) and (best is None or d < best_d):
                best, best_d = tr, d
        if best is None:
            tracks.append({"rows": [row]})
        else:
            best["rows"].append(row)
    return tracks


def sheets_phase(args):
    rows = [json.loads(line) for line in open(os.path.join(args.out, f"{args.tag}-detections.jsonl"), encoding="utf-8")]
    tracks = link(rows, args.max_gap, args.fps)
    tracks = [t for t in tracks if len(t["rows"]) >= args.min_frames]
    for t in tracks:
        t["best"] = max(t["rows"], key=lambda r: (r["box"][2] - r["box"][0]) * r["sharp"] ** 0.5)
    tracks.sort(key=lambda t: t["rows"][0]["t"])
    capture = cv2.VideoCapture(args.video)
    cell_h, cell_w, cols, per_sheet = 96, 300, 5, 20
    sheets = []
    for s in range(0, len(tracks), per_sheet):
        group = tracks[s:s + per_sheet]
        canvas = np.full(((len(group) + cols - 1) // cols * (cell_h + 22), cols * cell_w, 3), 40, np.uint8)
        for n, tr in enumerate(group):
            idx = s + n
            b = tr["best"]
            capture.set(cv2.CAP_PROP_POS_FRAMES, b["frame"])
            ok, frame = capture.read()
            x1, y1, x2, y2 = b["box"]
            pad = int(0.15 * (x2 - x1))
            crop = frame[max(0, y1 - pad):y2 + pad, max(0, x1 - pad):x2 + pad] if ok else None
            r, c = divmod(n, cols)
            ox, oy = c * cell_w, r * (cell_h + 22)
            if crop is not None and crop.size:
                scale = min((cell_w - 6) / crop.shape[1], cell_h / crop.shape[0])
                img = cv2.resize(crop, None, fx=scale, fy=scale, interpolation=cv2.INTER_CUBIC)
                canvas[oy + 20:oy + 20 + img.shape[0], ox + 3:ox + 3 + img.shape[1]] = img
            label = f"#{idx} {b['box'][2] - b['box'][0]}x{b['box'][3] - b['box'][1]}px f={len(tr['rows'])} t={tr['rows'][0]['t']:.1f}"
            cv2.putText(canvas, label, (ox + 3, oy + 14), cv2.FONT_HERSHEY_SIMPLEX, 0.45, (0, 255, 255), 1, cv2.LINE_AA)
        name = f"{args.tag}-sheet-{s // per_sheet:02d}.png"
        cv2.imwrite(os.path.join(args.out, name), canvas)
        sheets.append(name)
    capture.release()
    cand = [{"index": i, "frames": len(t["rows"]), "first": t["rows"][0]["t"], "last": t["rows"][-1]["t"], "best_frame": t["best"]["frame"], "best_box": t["best"]["box"],
             "rows": [{"t": r["t"], "frame": r["frame"], "box": r["box"], "sources": r["sources"], "reads": r["reads"]} for r in t["rows"]]} for i, t in enumerate(tracks)]
    json.dump({"video": os.path.basename(args.video), "fps": args.fps, "candidates": cand}, open(os.path.join(args.out, f"{args.tag}-candidates.json"), "w", encoding="utf-8"))
    print(f"{len(rows)} detections -> {len(tracks)} candidates seen on at least {args.min_frames} frames; sheets: {len(sheets)} ({', '.join(sheets[:3])} ...)")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("phase", choices=["detect", "sheets"])
    parser.add_argument("--video", required=True)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--out", default="/clips/census")
    parser.add_argument("--fps", type=float, default=4.0)
    parser.add_argument("--max-seconds", type=float, default=0, help="how many seconds to read from --start-seconds (0 = to the end)")
    parser.add_argument("--start-seconds", type=float, default=0.0)
    parser.add_argument("--tile", type=int, default=384)
    parser.add_argument("--scales", type=lambda s: [float(x) for x in s.split(",")], default=[1.0, 0.5])
    parser.add_argument("--fast-threshold", type=float, default=0.2)
    parser.add_argument("--accurate-threshold", type=float, default=0.3)
    parser.add_argument("--min-width", type=int, default=10)
    parser.add_argument("--min-frames", type=int, default=2)
    parser.add_argument("--max-gap", type=float, default=0.6)
    args = parser.parse_args()
    detect_phase(args) if args.phase == "detect" else sheets_phase(args)


if __name__ == "__main__":
    main()
