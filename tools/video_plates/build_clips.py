#!/usr/bin/env python3
"""Builds the video plate test sets of docs/data/video-plates-research-and-preregistration-20261111.md (DEV-V and SEALED-V).

    docker run --rm --network none --entrypoint python -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro \
        -v "<Downloads/archive/Pakistani License Number Plates Data>":/real:ro -v <gold folder with image-labels.csv>:/gold:ro \
        -v <out dir>:/out nexusai-media-eval:latest /repo/tools/video_plates/build_clips.py --scan
    ... /repo/tools/video_plates/build_clips.py --build --set dev    (then --set sealed)

Real photographs of cars from the public Kaggle set that are NOT in the owner-labelled gold set and were not used for the Orion demonstration photos. The plate of the photograph is replaced by a SYNTHETIC
plate string (plates_synth, the method of tools/showcase/build_plate_scenes.py), so the truth is exact and no real plate text is used. A virtual camera moves over the photograph: the vehicle approaches (the
plate grows), with seeded camera shake, motion blur, sensor noise, an occlusion in a quarter of the clips, an exposure ramp in a quarter, a quarter at 854x480, then H.264 at CRF 28 to 34 (ffmpeg).
Negatives: blank plate (the plate area painted flat) and clips that never show the plate. Writes the clips and a key (clip, kind, plate string, plate box per frame, legible interval) that holds only synthetic strings.
"""
import argparse
import csv
import json
import math
import os
import random
import subprocess
import sys

import cv2
import numpy as np

sys.path.insert(0, "/repo/tools/media_data")
sys.path.insert(0, "/repo/ingestion/forensic_records")
import plate_detector_onnx as pd  # noqa: E402
import plates_synth as ps  # noqa: E402

FPS = 25
DURATION = 10.0
OUT_W, OUT_H = 1280, 720
LEGIBLE_PX = 50
DETECTOR = "/data/models/plates/rfdetr-alpr/rfdetr_alpr.onnx"
SEEDS = {"dev": 20261111, "sealed": 20261112}
N_POSITIVE, N_NEG_BLANK, N_NEG_ABSENT = 24, 3, 3
ORION_SOURCES = {"Cars/DSC_0985.JPG", "Cars/DSC_0988.JPG", "Cars/DSC_0990.JPG", "Cars/DSC_0992.JPG", "Cars/DSC_0995.JPG"}


def ease(x):
    return x * x * (3 - 2 * x)


def candidates():
    gold = {r["relative_file"] for r in csv.DictReader(open("/gold/image-labels.csv", encoding="utf-8-sig"))}
    photos = [f"Cars/{n}" for n in sorted(os.listdir("/real/Cars")) if f"Cars/{n}" not in gold and f"Cars/{n}" not in ORION_SOURCES and n.lower().endswith(".jpg")]
    det = pd.OnnxPlateDetector(DETECTOR, cv2, normalise="unit", threshold=0.5, drop_non_plates=True)
    good = []
    for rel in photos:
        frame = cv2.imread("/real/" + rel)
        found = det.detect(frame)
        if len(found) == 1 and found[0][4] >= 0.85 and (found[0][2] - found[0][0]) >= 90:
            good.append((rel, tuple(int(v) for v in found[0][:4])))
    return good


def plate_strings(rng, n, taken):
    shapes = [lambda: "%s%s%s-%s" % tuple(("".join(rng.choice("ABCDEFGHJKLMNPRSTUVWXYZ") for _ in range(3)), "", "", "%04d" % rng.randrange(10000))),
              lambda: "".join(rng.choice("ABCDEFGHJKLMNPRSTUVWXYZ") for _ in range(3)) + " " + "%03d" % rng.randrange(1000),
              lambda: "".join(rng.choice("ABCDEFGHJKLMNPRSTUVWXYZ") for _ in range(2)) + "-" + "%04d" % rng.randrange(10000),
              lambda: "".join(rng.choice("ABCDEFGHJKLMNPRSTUVWXYZ") for _ in range(3)) + "%04d" % rng.randrange(10000),
              lambda: "SYN" + rng.choice("ABCDEFGHJKLMNPRSTUVWXYZ") + rng.choice("O0") + "%02d" % rng.randrange(100)]
    out = []
    while len(out) < n:
        text = shapes[len(out) % len(shapes)]()
        norm = "".join(ch for ch in text if ch.isalnum())
        if norm not in taken:
            taken.add(norm)
            out.append((text, norm))
    return out


def camera_path(rng, photo_w, photo_h, box, positive):
    """Window size and centre per frame, from the approach geometry. Returns arrays and the degradations drawn for this clip."""
    x1, y1, x2, y2 = box
    plate_w = x2 - x1
    max_w = min(photo_w, photo_h * OUT_W / OUT_H)
    p0_min = max(26.0, plate_w * OUT_W / max_w)
    p0 = p0_min * rng.uniform(1.0, 1.4)
    p1 = min(260.0, p0 * rng.uniform(4.0, 5.5))
    w0, w1 = plate_w * OUT_W / p0, plate_w * OUT_W / p1
    frames = int(DURATION * FPS)
    t = np.linspace(0, 1, frames)
    widths = w0 + (w1 - w0) * np.array([ease(v) for v in t])
    cx1, cy1 = (x1 + x2) / 2, (y1 + y2) / 2
    if positive:
        cx0, cy0 = cx1 + rng.uniform(-0.25, 0.25) * w0, cy1 + rng.uniform(-0.12, 0.18) * w0 * OUT_H / OUT_W
        centres = np.stack([cx0 + (cx1 - cx0) * np.array([ease(v) for v in t]), cy0 + (cy1 - cy0) * np.array([ease(v) for v in t])], axis=1)
    else:
        centres = np.zeros((frames, 2))
    shake = rng.choice([0.0, 1.0, 2.0, 4.0])
    walk = np.cumsum(np.array([[rng.gauss(0, shake), rng.gauss(0, shake)] for _ in range(frames)]), axis=0) * 0.15
    deg = {"shake_px": shake, "blur_px": rng.choice([0, 3, 5, 9]), "noise_sigma": rng.uniform(2.0, 6.0), "crf": rng.choice([28, 31, 34]),
           "occlusion": rng.random() < 0.25, "exposure_ramp": rng.random() < 0.25, "small": rng.random() < 0.25, "p0_px": round(p0, 1), "p1_px": round(p1, 1)}
    return widths, centres, walk, deg


def window(photo_w, photo_h, width, centre):
    height = width * OUT_H / OUT_W
    left = min(max(centre[0] - width / 2, 0), photo_w - width)
    top = min(max(centre[1] - height / 2, 0), photo_h - height)
    return left, top, width, height


def absent_centres(photo_w, photo_h, box, widths):
    """A fixed window centre that never contains the plate (grown by 25%): try a grid and take the first that fits for the whole zoom."""
    x1, y1, x2, y2 = box
    w = widths.max()
    h = w * OUT_H / OUT_W
    for fy in (0.2, 0.8, 0.5, 0.35, 0.65):
        for fx in (0.5, 0.25, 0.75):
            cx, cy = fx * photo_w, fy * photo_h
            left, top, _, _ = window(photo_w, photo_h, w, (cx, cy))
            if left + w < x1 - 0.25 * (x2 - x1) or left > x2 + 0.25 * (x2 - x1) or top + h < y1 - 0.25 * (y2 - y1) or top > y2 + 0.25 * (y2 - y1):
                return np.tile([cx, cy], (len(widths), 1))
    return None


def motion_blur(frame, length, angle):
    if length < 2:
        return frame
    k = np.zeros((length, length), np.float32)
    cv2.line(k, (0, length // 2), (length - 1, length // 2), 1.0, 1)
    m = cv2.getRotationMatrix2D((length / 2 - 0.5, length / 2 - 0.5), angle, 1.0)
    k = cv2.warpAffine(k, m, (length, length))
    return cv2.filter2D(frame, -1, k / max(k.sum(), 1e-6))


def render_clip(photo, box, widths, centres, walk, deg, rng, path, truth_box_visible):
    ph, pw = photo.shape[:2]
    size = (854, 480) if deg["small"] else (OUT_W, OUT_H)
    cmd = ["ffmpeg", "-loglevel", "error", "-y", "-f", "rawvideo", "-pix_fmt", "bgr24", "-s", f"{size[0]}x{size[1]}", "-r", str(FPS), "-i", "-", "-an", "-c:v", "libx264", "-preset", "veryfast",
           "-crf", str(deg["crf"]), "-pix_fmt", "yuv420p", "-movflags", "+faststart", path]
    proc = subprocess.Popen(cmd, stdin=subprocess.PIPE)
    boxes, legible = [], []
    occ_start = rng.uniform(3.0, 6.0) if deg["occlusion"] else None
    occ_len = rng.uniform(0.5, 0.8)
    x1, y1, x2, y2 = box
    angle = rng.uniform(-10, 10)
    for i in range(len(widths)):
        left, top, w, h = window(pw, ph, widths[i], centres[i] + walk[i])
        crop = photo[int(round(top)):int(round(top + h)), int(round(left)):int(round(left + w))]
        frame = cv2.resize(crop, (OUT_W, OUT_H), interpolation=cv2.INTER_AREA if w > OUT_W else cv2.INTER_CUBIC)
        scale = OUT_W / w
        bx1, by1, bx2, by2 = (x1 - left) * scale, (y1 - top) * scale, (x2 - left) * scale, (y2 - top) * scale
        frame = motion_blur(frame, deg["blur_px"], angle)
        if deg["exposure_ramp"]:
            frame = np.clip(frame.astype(np.float32) * (0.6 + 0.6 * i / len(widths)), 0, 255).astype(np.uint8)
        occluded = False
        if occ_start is not None and occ_start <= i / FPS <= occ_start + occ_len:
            cx = int((bx1 + bx2) / 2 + (bx2 - bx1) * (-0.8 + 1.6 * (i / FPS - occ_start) / occ_len))
            cv2.rectangle(frame, (cx - int((bx2 - bx1) * 0.35), 0), (cx + int((bx2 - bx1) * 0.35), OUT_H), (40, 40, 40), -1)
            occluded = True
        noise = np.random.default_rng(i + int(deg["noise_sigma"] * 1000)).normal(0, deg["noise_sigma"], frame.shape)
        frame = np.clip(frame.astype(np.float32) + noise, 0, 255).astype(np.uint8)
        if deg["small"]:
            frame = cv2.resize(frame, size, interpolation=cv2.INTER_AREA)
            f = size[0] / OUT_W
        else:
            f = 1.0
        proc.stdin.write(frame.tobytes())
        inside = bx1 >= 0 and by1 >= 0 and bx2 <= OUT_W and by2 <= OUT_H
        width_px = (bx2 - bx1) * f
        boxes.append([round(bx1 * f, 1), round(by1 * f, 1), round(bx2 * f, 1), round(by2 * f, 1)] if inside else None)
        legible.append(bool(truth_box_visible and inside and width_px >= LEGIBLE_PX and not occluded))
    proc.stdin.close()
    if proc.wait() != 0:
        raise SystemExit("ffmpeg failed for " + path)
    on = [i / FPS for i, v in enumerate(legible) if v]
    return boxes[::5], (round(on[0], 2) if on else None), (round(on[-1] + 1 / FPS, 2) if on else None), size


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--scan", action="store_true")
    parser.add_argument("--build", action="store_true")
    parser.add_argument("--set", choices=["dev", "sealed"], default="dev")
    parser.add_argument("--out", default="/out")
    args = parser.parse_args()
    cands = candidates()
    print(f"{len(cands)} usable photos (exactly one plate of at least 90 px at score 0.85 or more, not in the gold set, not used for the Orion photos)")
    if args.scan or not args.build:
        return
    need = N_POSITIVE + N_NEG_BLANK + N_NEG_ABSENT
    pool = cands[0::2] if args.set == "dev" else cands[1::2]   # disjoint photos for the two sets
    if len(pool) < need:
        raise SystemExit(f"only {len(pool)} photos for the {args.set} set, {need} needed")
    rng = random.Random(SEEDS[args.set])
    taken = set()
    plates = plate_strings(rng, N_POSITIVE, taken)
    os.makedirs(os.path.join(args.out, args.set), exist_ok=True)
    key = []
    for index in range(need):
        rel, box = pool[index]
        photo = cv2.imread("/real/" + rel)
        ph, pw = photo.shape[:2]
        positive, kind = index < N_POSITIVE, "positive" if index < N_POSITIVE else ("negative_blank_plate" if index < N_POSITIVE + N_NEG_BLANK else "negative_plate_never_shown")
        clip_id = f"{args.set}-{index + 1:02d}"
        crng = random.Random(SEEDS[args.set] * 1000 + index)
        scene = photo.copy()
        text = norm = None
        if positive:
            text, norm = plates[index]
            plate_image = ps.render_plate(crng, {"region": "Punjab", "format": "ORION", "layout": "one_line", "display": text, "norm": norm, "text_override": text})
            scene = ps.paste_plate(photo, plate_image, box, crng)
        elif kind == "negative_blank_plate":
            x1, y1, x2, y2 = box
            colour = tuple(int(v) for v in np.median(photo[y1:y2, x1:x2].reshape(-1, 3), axis=0))
            cv2.rectangle(scene, (x1, y1), (x2, y2), colour, -1)
        widths, centres, walk, deg = camera_path(crng, pw, ph, box, positive or kind == "negative_blank_plate")
        if kind == "negative_plate_never_shown":
            widths = np.linspace(0.45 * pw, 0.30 * pw, len(widths))   # a tighter window than the approach clips, so that a region without the plate exists in every photo
            centres = absent_centres(pw, ph, box, widths)
            if centres is None:
                raise SystemExit("no window away from the plate for " + rel)
        file = f"{clip_id}.mp4"
        boxes, start, end, size = render_clip(scene, box, widths, centres, walk, deg, crng, os.path.join(args.out, args.set, file), kind in ("positive",))
        key.append({"clip": clip_id, "file": file, "kind": kind, "plate": norm, "plate_display": text, "source_photo": rel, "size": list(size), "fps": FPS, "duration_seconds": DURATION,
                    "legible_start_seconds": start, "legible_end_seconds": end, "plate_box_every_5th_frame": boxes if kind != "negative_plate_never_shown" else None, "degradations": deg})
        print(clip_id, kind, norm, "legible", start, end, size, deg["crf"])
    json.dump({"set": args.set, "built_with": "tools/video_plates/build_clips.py", "seed": SEEDS[args.set], "note": "synthetic plate strings only; real photographs of cars from a public dataset; truth is exact",
               "clips": key}, open(os.path.join(args.out, f"key-{args.set}.json"), "w", encoding="utf-8"), indent=1)


if __name__ == "__main__":
    main()
