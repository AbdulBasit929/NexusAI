#!/usr/bin/env python3
"""Near-duplicate and similarity measurement for track I (docs/data/media-activation-programme-20261021.md).

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest \
        python /repo/tools/media_data/image_eval.py --methods dhash64,pdq,dinov2,siglip

Set: the general Open Images photos of the subset (CC BY 2.0; DAT-018), each with 13 transformations. Protocol: the gallery holds the originals of two thirds of the photos; every
transformed photo is a query. A query whose original is in the gallery must return that original within the threshold (positive); a query whose original is NOT in the gallery must return nothing
(negative, one third of the photos). Precision = correct answers / all answers given; recall = positives answered correctly / positives. The threshold per method is the loosest one with
precision at least 99.5% on the dev photos (last hex digit of the SHA-256 of the photo id, 0 to b); the sealed photos (c to f) are then scored at that threshold once and only reported.
Methods: dhash64 (the worker's own image_dhash, Hamming), pdq (PDQ 256-bit, Hamming), dinov2 (dinov2-small CLS, cosine), siglip (the worker's SigLIP-base, cosine).
"""
import argparse
import hashlib
import io
import json
import os
import sys
import time
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw, ImageEnhance, ImageFont, ImageOps

sys.path.insert(0, "/repo/ingestion/forensic_records")
DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
MILD = {"jpeg30", "jpeg70", "resize50", "resize75", "bright", "contrast", "gray"}
STRONG = {"crop90", "crop80", "rot3", "rot10", "mirror", "overlay", "watermark"}


def partition(key):
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def transformations(image):
    w, h = image.size
    out = {}

    def jpeg(q):
        buffer = io.BytesIO()
        image.save(buffer, format="JPEG", quality=q)
        return Image.open(io.BytesIO(buffer.getvalue())).convert("RGB")
    out["jpeg30"], out["jpeg70"] = jpeg(30), jpeg(70)
    out["resize50"] = image.resize((max(8, w // 2), max(8, h // 2)), Image.LANCZOS)
    out["resize75"] = image.resize((max(8, w * 3 // 4), max(8, h * 3 // 4)), Image.LANCZOS)
    out["bright"] = ImageEnhance.Brightness(image).enhance(1.3)
    out["contrast"] = ImageEnhance.Contrast(image).enhance(0.7)
    out["gray"] = ImageOps.grayscale(image).convert("RGB")
    for name, frac in (("crop90", 0.9), ("crop80", 0.8)):
        cw, ch = int(w * frac), int(h * frac)
        left, top = (w - cw) // 2, (h - ch) // 2
        out[name] = image.crop((left, top, left + cw, top + ch))
    out["rot3"] = image.rotate(3, resample=Image.BICUBIC, expand=False, fillcolor=(0, 0, 0))
    out["rot10"] = image.rotate(10, resample=Image.BICUBIC, expand=False, fillcolor=(0, 0, 0))
    out["mirror"] = ImageOps.mirror(image)
    overlay = image.copy()
    draw = ImageDraw.Draw(overlay)
    draw.rectangle((w // 20, h - h // 7, w - w // 20, h - h // 20), fill=(0, 0, 0))
    draw.text((w // 12, h - h // 8), "EVIDENCE COPY 2026-10-21 NOT FOR RELEASE", fill=(255, 255, 255), font=ImageFont.load_default())
    out["overlay"] = overlay
    mark = image.copy()
    box = (int(w * 0.75), int(h * 0.75), int(w * 0.97), int(h * 0.97))
    patch = Image.new("RGB", (box[2] - box[0], box[3] - box[1]), (255, 255, 255))
    mark.paste(Image.blend(mark.crop(box), patch, 0.55), box)
    out["watermark"] = mark
    return out


def dhash64(path):
    import image_intelligence
    return image_intelligence.image_dhash(Path(path))


def _box_blur(values, window, axis):
    """Mean over a window clipped at the borders (the box-filter step of PDQ's Jarosz filter)."""
    if window <= 1:
        return values
    n = values.shape[axis]
    pad = [(0, 0)] * values.ndim
    pad[axis] = (1, 0)
    csum = np.pad(np.cumsum(values, axis=axis, dtype=np.float64), pad)
    idx = np.arange(n)
    lo = np.clip(idx - window // 2, 0, n)
    hi = np.clip(idx + (window - 1) // 2 + 1, 0, n)
    take = lambda arr, i: np.take(arr, i, axis=axis)
    shape = [1] * values.ndim
    shape[axis] = n
    return ((take(csum, hi) - take(csum, lo)) / (hi - lo).reshape(shape)).astype(np.float32)


_PDQ_DCT = np.sqrt(2.0 / 64) * np.cos(np.pi / 128 * np.arange(1, 17)[:, None] * (2 * np.arange(64)[None, :] + 1))


def pdq_bits(image):
    """PDQ-compatible 256-bit hash re-implemented from the published algorithm (luma, two box-filter passes, 64x64 decimation, 16x16 DCT without the DC term, median threshold).
    The official implementation is BSD-licensed C++ with bindings; this one has no compiler dependency. It is labelled 'PDQ-style' everywhere and its thresholds are calibrated here, not borrowed."""
    rgb = np.asarray(image.convert("RGB"), dtype=np.float32)
    luma = 0.299 * rgb[..., 0] + 0.587 * rgb[..., 1] + 0.114 * rgb[..., 2]
    rows, cols = luma.shape
    window = lambda old: (old + 2 * 64 - 1) // (2 * 64)
    for _ in range(2):
        luma = _box_blur(luma, window(cols), axis=1)
        luma = _box_blur(luma, window(rows), axis=0)
    small = luma[np.ix_(((np.arange(64) + 0.5) * rows / 64).astype(int), ((np.arange(64) + 0.5) * cols / 64).astype(int))]
    coefficients = _PDQ_DCT @ small @ _PDQ_DCT.T
    return (coefficients > np.median(coefficients)).flatten().astype(np.uint8)


class Embedder:
    def __init__(self, kind):
        import torch
        from transformers import AutoImageProcessor, AutoModel
        self.torch, self.kind = torch, kind
        torch.set_num_threads(int(os.environ.get("IMG_THREADS", "6")))
        if kind == "dinov2":
            path = Path(DATA) / "models" / "image" / "dinov2-small"
            self.processor, self.model = AutoImageProcessor.from_pretrained(path), AutoModel.from_pretrained(path).eval()
        else:
            path = os.environ.get("SIGLIP_PATH", "/models/media/nexusai/models/google-siglip-base-patch16-224/7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed")
            self.processor, self.model = AutoImageProcessor.from_pretrained(path), AutoModel.from_pretrained(path).eval()

    def __call__(self, image):
        with self.torch.no_grad():
            inputs = self.processor(images=image.convert("RGB"), return_tensors="pt")
            if self.kind == "dinov2":
                vector = self.model(**inputs).last_hidden_state[:, 0]
            else:
                vector = self.model.get_image_features(**inputs)
            vector = vector / vector.norm(dim=-1, keepdim=True)
        return vector[0].numpy()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--methods", default="dhash64,pdq,dinov2,siglip")
    parser.add_argument("--photos", type=int, default=300)
    args = parser.parse_args()
    records = [json.loads(line) for line in open(Path(DATA) / "openimages" / "subset-test.jsonl", encoding="utf-8")]
    general = [r for r in records if "general" in r["roles"]][:args.photos]
    print(f"{len(general)} photos, methods {args.methods}")
    ids = [r["id"] for r in general]
    # two thirds of the photos have their original in the gallery; one third does not (the negatives)
    in_gallery = {i: (int(hashlib.sha256(("g" + i).encode()).hexdigest(), 16) % 3 != 0) for i in ids}
    methods = args.methods.split(",")
    embedders = {m: Embedder(m) for m in methods if m in ("dinov2", "siglip")}
    tmp = Path("/tmp/imgvar")
    tmp.mkdir(exist_ok=True)
    descriptors = {m: {"orig": {}, "var": []} for m in methods}  # var: (photo id, transformation, descriptor)
    started = time.time()
    timings = {m: [0.0, 0] for m in methods}
    for n, record in enumerate(general, start=1):
        source = Image.open(Path(DATA) / record["file"]).convert("RGB")
        if max(source.size) > 1024:
            source.thumbnail((1024, 1024))
        versions = {"orig": source, **transformations(source)}
        for name, image in versions.items():
            path = tmp / f"{record['id']}-{name}.png"
            if "dhash64" in methods:
                image.save(path)
            for m in methods:
                t0 = time.time()
                if m == "dhash64":
                    value = dhash64(path)
                    value = np.unpackbits(np.frombuffer(bytes.fromhex(value), dtype=np.uint8)) if value else None
                elif m == "pdq":
                    value = pdq_bits(image)
                else:
                    value = embedders[m](image)
                timings[m][0] += time.time() - t0
                timings[m][1] += 1
                if value is None:
                    continue
                if name == "orig":
                    descriptors[m]["orig"][record["id"]] = value
                else:
                    descriptors[m]["var"].append((record["id"], name, value))
            if "dhash64" in methods:
                path.unlink(missing_ok=True)
        if n % 25 == 0:
            print(f"  {n}/{len(general)} photos, {time.time() - started:.0f} s", flush=True)
    report = {"photos": len(general), "methods": {}}
    for m in methods:
        gallery_ids = [i for i in ids if in_gallery[i] and i in descriptors[m]["orig"]]
        gallery = np.stack([descriptors[m]["orig"][i] for i in gallery_ids])
        binary = m in ("dhash64", "pdq")
        distances = []  # (photo id, transformation, best distance, best id)
        for photo, name, value in descriptors[m]["var"]:
            if binary:
                d = (gallery != value).sum(axis=1).astype(float)
            else:
                d = 1.0 - gallery @ value  # cosine distance, vectors are unit length
            best = int(np.argmin(d))
            distances.append((photo, name, float(d[best]), gallery_ids[best]))
        thresholds = list(range(0, (64 if m == "dhash64" else 256) + 1)) if binary else [round(x, 3) for x in np.arange(0.0, 0.8, 0.005)]

        def score(part, threshold, groups=None):
            tp = fp = positives = 0
            for photo, name, dist, best in distances:
                if partition(photo) != part or (groups and name not in groups):
                    continue
                positive = in_gallery[photo]
                positives += 1 if positive else 0
                if dist <= threshold:
                    if positive and best == photo:
                        tp += 1
                    else:
                        fp += 1
            precision = tp / (tp + fp) if tp + fp else 1.0
            return precision, (tp / positives if positives else 0.0), tp, fp, positives
        chosen = None
        for threshold in thresholds:
            precision, recall, *_ = score("dev", threshold)
            if precision >= 0.995:
                chosen = threshold  # the loosest threshold that still holds the precision floor
        result = {"threshold": chosen, "seconds_per_image": timings[m][0] / max(1, timings[m][1])}
        if chosen is not None:
            for part in ("dev", "sealed"):
                p, r, tp, fp, pos = score(part, chosen)
                result[part] = {"precision": round(p, 4), "recall": round(r, 4), "tp": tp, "fp": fp, "positives": pos,
                                "mild_recall": round(score(part, chosen, MILD)[1], 4), "strong_recall": round(score(part, chosen, STRONG)[1], 4)}
            result["per_transformation_dev_recall"] = {t: round(score("dev", chosen, {t})[1], 3) for t in sorted(MILD | STRONG)}
        report["methods"][m] = result
        print(f"\n{m}: threshold {chosen}  {1000 * result['seconds_per_image']:.1f} ms/image")
        if chosen is not None:
            for part in ("dev", "sealed"):
                r = result[part]
                print(f"  {part:6} precision {100 * r['precision']:5.1f}%  recall {100 * r['recall']:5.1f}%  mild {100 * r['mild_recall']:5.1f}%  strong {100 * r['strong_recall']:5.1f}%  (tp {r['tp']} fp {r['fp']} of {r['positives']} positive queries)")
            print("  dev recall by transformation:", result["per_transformation_dev_recall"])
    out = Path(DATA) / "results" / "image"
    out.mkdir(parents=True, exist_ok=True)
    json.dump(report, open(out / f"near-duplicate-{time.strftime('%Y%m%d-%H%M%S')}.json", "w"), indent=1)


if __name__ == "__main__":
    main()
