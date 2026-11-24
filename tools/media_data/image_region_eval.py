#!/usr/bin/env python3
"""Real-world region-search evaluation against the live API.

Regions are cut out of the case's own pictures the way an analyst would produce them (a small detail, a tiny object, a rotated or perspective-warped detail, a blurred, noisy or heavily
compressed one, an annotated screenshot, a mirrored one), each is attached with POST /images/search?kind=reference_upload, and the answer is scored: did the source picture come back among the
pictures that contain the region, at what rank, and does the reported box overlap the true region? Regions cut from pictures that are NOT in the case measure false alarms.

    python tools/media_data/image_region_eval.py --collection orion-lahore-reference-20261030 --pack C:/NexusAI-Showcase/orion/image-search-test-pack [--sources 24] [--seed 7]

Writes <pack>/region-eval-results.json and saves a sample of the probes under <pack>/region-probes/ for manual testing. Needs OpenCV and numpy on the machine running it (run it in the service image).
"""
import argparse
import json
import mimetypes
import random
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from pathlib import Path

import cv2
import numpy as np

SECRETS = r"C:\Users\sheik\Workspace\Office\Projects\NexusAI\.env.forensic-runtime.local"
API = "http://127.0.0.1:8091"


def secret(name):
    for line in open(SECRETS, encoding="utf-8"):
        if line.startswith(name + "="):
            return line.split("=", 1)[1].strip()
    sys.exit(f"{name} is not in the secrets file")


def jpeg(image, quality=88):
    ok, data = cv2.imencode(".jpg", image, [cv2.IMWRITE_JPEG_QUALITY, quality])
    return data.tobytes()


def png(image):
    ok, data = cv2.imencode(".png", image)
    return data.tobytes()


def region(rng, shape, area):
    """A random rectangle covering about `area` of the picture, as pixel bounds."""
    h, w = shape[:2]
    aspect = rng.uniform(0.7, 1.5)
    rw = int(min(w * 0.98, max(32, (area * w * h * aspect) ** 0.5)))
    rh = int(min(h * 0.98, max(32, area * w * h / max(rw, 1))))
    x0 = rng.randint(0, max(0, w - rw))
    y0 = rng.randint(0, max(0, h - rh))
    return x0, y0, x0 + rw, y0 + rh


def make_variants(image, rng):
    """name -> (probe image, true box as fractions of the source). One call per source picture."""
    h, w = image.shape[:2]
    out = {}

    def cut(area):
        x0, y0, x1, y1 = region(rng, image.shape, area)
        return image[y0:y1, x0:x1].copy(), [x0 / w, y0 / h, x1 / w, y1 / h]

    crop, box = cut(rng.uniform(0.18, 0.32))
    out["detail_medium"] = (crop, box)
    crop, box = cut(rng.uniform(0.05, 0.12))
    out["detail_small"] = (crop, box)
    crop, box = cut(rng.uniform(0.015, 0.04))
    out["detail_tiny"] = (crop, box)
    crop, box = cut(rng.uniform(0.15, 0.3))
    ch, cw = crop.shape[:2]
    angle = rng.choice([-1, 1]) * rng.uniform(8, 25)
    matrix = cv2.getRotationMatrix2D((cw / 2, ch / 2), angle, 0.9)
    out["rotated"] = (cv2.warpAffine(crop, matrix, (cw, ch), borderMode=cv2.BORDER_REFLECT), box)
    crop, box = cut(rng.uniform(0.15, 0.3))
    ch, cw = crop.shape[:2]
    jitter = lambda: rng.uniform(-0.09, 0.09)  # noqa: E731
    src = np.float32([[0, 0], [cw, 0], [cw, ch], [0, ch]])
    dst = np.float32([[cw * (0 + abs(jitter())), ch * jitter() + 4], [cw * (1 - abs(jitter())), ch * jitter() + 4], [cw * (1 - abs(jitter())), ch * (1 + jitter()) - 4], [cw * (0 + abs(jitter())), ch * (1 + jitter()) - 4]])
    out["perspective"] = (cv2.warpPerspective(crop, cv2.getPerspectiveTransform(src, dst), (cw, ch), borderMode=cv2.BORDER_REFLECT), box)
    crop, box = cut(rng.uniform(0.15, 0.3))
    small = cv2.resize(crop, None, fx=0.35, fy=0.35, interpolation=cv2.INTER_AREA)
    out["low_resolution"] = (cv2.resize(small, (crop.shape[1], crop.shape[0]), interpolation=cv2.INTER_CUBIC), box)
    crop, box = cut(rng.uniform(0.15, 0.3))
    noisy = np.clip(crop.astype(np.int16) + np.random.RandomState(rng.randint(0, 10**6)).normal(0, 14, crop.shape).astype(np.int16), 0, 255).astype(np.uint8)
    out["noisy_compressed"] = (cv2.imdecode(np.frombuffer(jpeg(noisy, 30), np.uint8), cv2.IMREAD_COLOR), box)
    crop, box = cut(rng.uniform(0.15, 0.3))
    shot = cv2.copyMakeBorder(crop, 46, 22, 14, 14, cv2.BORDER_CONSTANT, value=(245, 245, 245))
    cv2.putText(shot, "Screenshot  12:41   Share   Edit", (20, 31), cv2.FONT_HERSHEY_SIMPLEX, 0.6, (60, 60, 60), 1, cv2.LINE_AA)
    cv2.arrowedLine(shot, (shot.shape[1] - 60, shot.shape[0] - 14), (shot.shape[1] // 2, shot.shape[0] // 2 + 20), (0, 0, 255), 3)
    cv2.rectangle(shot, (shot.shape[1] // 3, shot.shape[0] // 3), (shot.shape[1] // 3 + 80, shot.shape[0] // 3 + 50), (0, 0, 255), 3)
    out["annotated_screenshot"] = (shot, box)
    crop, box = cut(rng.uniform(0.15, 0.3))
    out["mirrored"] = (cv2.flip(crop, 1), box)
    return out


def iou(a, b):
    x0, y0, x1, y1 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0.0, x1 - x0) * max(0.0, y1 - y0)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union > 0 else 0.0


def search(collection, tenant, headers, data, name, top_k=30):
    boundary = uuid.uuid4().hex
    body = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"image\"; filename=\"{name}\"\r\nContent-Type: {mimetypes.guess_type(name)[0] or 'application/octet-stream'}\r\n\r\n").encode() + data + f"\r\n--{boundary}--\r\n".encode()
    query = urllib.parse.urlencode({"tenant_id": tenant, "collection_id": collection, "kind": "reference_upload", "top_k": top_k})
    request = urllib.request.Request(f"{API}/images/search?{query}", data=body, method="POST", headers={**headers, "Content-Type": f"multipart/form-data; boundary={boundary}"})
    started = time.time()
    for attempt in range(4):  # the service can be busy for a moment (a 503); a long evaluation must not die of that
        try:
            with urllib.request.urlopen(request, timeout=400) as response:
                answer = json.loads(response.read().decode("utf-8"))
            break
        except urllib.error.HTTPError as error:
            if error.code != 503 or attempt == 3:
                raise
            time.sleep(10 * (attempt + 1))
    return answer, time.time() - started


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--collection", required=True)
    parser.add_argument("--pack", required=True)
    parser.add_argument("--sources", type=int, default=24)
    parser.add_argument("--negatives", type=int, default=24)
    parser.add_argument("--seed", type=int, default=7)
    parser.add_argument("--api", default=API)
    parser.add_argument("--secrets", default=SECRETS)
    parser.add_argument("--extra-negatives", default=r"C:\tmp\image-search-live", help="a folder of further pictures that are not in the case")
    args = parser.parse_args()
    globals().update(API=args.api, SECRETS=args.secrets)  # the module-level names read by search() and secret()
    pack = Path(args.pack)
    rng = random.Random(args.seed)
    tenant = secret("FORENSIC_RECORDS_TENANT_ID") or "default"
    headers = {"Authorization": "Bearer " + secret("FORENSIC_RECORDS_API_KEY"), "X-Forensic-Tenant-ID": tenant, "X-Forensic-Collection-ID": args.collection,
               "X-Forensic-Actor-ID": "region-eval", "X-Forensic-Subject-ID": "region-eval", "X-Forensic-Actor-Role": "user"}
    in_case = sorted(p for p in (pack / "in-case").glob("*.jpg") if "-resaved" not in p.name and "-cropped" not in p.name)
    rng.shuffle(in_case)
    sources = in_case[: args.sources]
    outside = sorted((pack / "reference").glob("ref-not-in-case-*.jpg"))
    # ref-mirror, ref-crop85 and ref-recompressed in that folder are copies of a picture that IS in the case (roadside-vehicle-01); they are not negatives
    extra = [f for f in sorted(Path(args.extra_negatives).glob("*.jpg")) if f.name not in ("ref-mirror.jpg", "ref-crop85.jpg", "ref-recompressed.jpg")] if Path(args.extra_negatives).is_dir() else []
    sample_dir = pack / "region-probes"
    sample_dir.mkdir(exist_ok=True)
    rows = []
    for index, source in enumerate(sources, start=1):
        image = cv2.imread(str(source))
        if image is None:
            continue
        for variant, (probe, box) in make_variants(image, rng).items():
            data = png(probe) if variant == "annotated_screenshot" else jpeg(probe, 90)
            name = f"{source.stem}__{variant}.{'png' if variant == 'annotated_screenshot' else 'jpg'}"
            if index <= 3:
                (sample_dir / name).write_bytes(data)
            answer, seconds = search(args.collection, tenant, headers, data, name)
            hits = [h for h in answer["results"] if h["tier"] in ("region_verified", "region_probable")]
            parts = [h for h in answer["results"] if h["tier"] == "similar_region"]
            copies = [h for h in answer["results"] if h["tier"] in ("identical_file", "same_image", "same_photograph")]
            as_copy = source.name in [h["original_filename"] for h in copies]  # a region large enough to be the same photograph is listed as a copy, and the region pass skips copies
            names = [h["original_filename"] for h in hits]
            rank = names.index(source.name) + 1 if source.name in names else None
            located = iou(hits[rank - 1]["box"], box) if rank else None
            part_names = [h["original_filename"] for h in parts]
            part_rank = part_names.index(source.name) + 1 if source.name in part_names else None
            part_located = iou(parts[part_rank - 1]["box"], box) if part_rank and parts[part_rank - 1].get("box") else None
            rows.append({"kind": "positive", "source": source.name, "variant": variant, "probe_px": [probe.shape[1], probe.shape[0]], "found": rank is not None, "rank": rank, "iou": located,
                         "verified": bool(rank and hits[rank - 1]["tier"] == "region_verified"), "inliers": hits[rank - 1]["inliers"] if rank else None, "region_hits": len(hits), "seconds": round(seconds, 1),
                         "found_as_copy": as_copy, "part_found": part_rank is not None, "part_rank": part_rank, "part_iou": part_located, "part_distance": parts[part_rank - 1]["cosine_distance"] if part_rank else None,
                         "parts": [(h["original_filename"], round(h["cosine_distance"], 3)) for h in parts]})
            print(f"{'FOUND ' if rank else ('COPY  ' if as_copy else ('PART  ' if part_rank else 'MISSED'))} {variant:<22} {source.name:<44} rank={rank} iou={None if located is None else round(located, 2)} hits={len(hits)} part_rank={part_rank} parts={len(parts)} {seconds:.1f}s", flush=True)
    negatives = list(outside) + list(extra)
    rng.shuffle(negatives)
    for path in negatives[: args.negatives]:
        image = cv2.imread(str(path))
        if image is None:
            continue
        for variant in ("detail_medium", "detail_small"):
            probe, _ = make_variants(image, rng)[variant]
            data = jpeg(probe, 90)
            answer, seconds = search(args.collection, tenant, headers, data, f"{path.stem}__{variant}.jpg")
            hits = [h for h in answer["results"] if h["tier"] in ("region_verified", "region_probable")]
            parts = [h for h in answer["results"] if h["tier"] == "similar_region"]
            rows.append({"kind": "negative", "source": path.name, "variant": variant, "false_hits": len(hits), "max_inliers": max([h["inliers"] for h in hits], default=0), "false_parts": len(parts),
                         "part_distances": [round(h["cosine_distance"], 3) for h in parts], "seconds": round(seconds, 1)})
            print(f"NEG    {variant:<22} {path.name:<44} false_hits={len(hits)} false_parts={len(parts)} {seconds:.1f}s", flush=True)
    json.dump({"collection": args.collection, "at": time.strftime("%Y-%m-%d %H:%M:%S"), "rows": rows}, open(pack / "region-eval-results.json", "w", encoding="utf-8"), indent=1)
    positives = [r for r in rows if r["kind"] == "positive"]
    print("\nvariant                 found   verified  mean IoU of the box")
    for variant in sorted({r["variant"] for r in positives}):
        group = [r for r in positives if r["variant"] == variant]
        ious = [r["iou"] for r in group if r["iou"] is not None]
        print(f"{variant:<22} {sum(r['found'] for r in group):>3}/{len(group):<3} {sum(r['verified'] for r in group):>5}    {np.mean(ious) if ious else 0:.2f}")
    by_region = sum(1 for r in positives if r["found"])
    by_copy = sum(1 for r in positives if not r["found"] and r["found_as_copy"])
    by_part = sum(1 for r in positives if not r["found"] and not r["found_as_copy"] and r["part_found"])
    print(f"the picture the part was cut from came back: as 'contains it' {by_region}, as a whole-picture copy {by_copy} more, as 'a part looks like it' {by_part} more; "
          f"missed {len(positives) - by_region - by_copy - by_part} of {len(positives)}")
    for limit in (0.10, 0.15, 0.20, 0.25, 0.30):
        in_range = [r for r in positives if r["part_found"] and r["part_distance"] <= limit]
        false_parts = sum(1 for r in positives for name, d in r["parts"] if name != r["source"] and d <= limit)
        print(f"  window distance <= {limit:.2f}: source found as a part in {len(in_range)} probes; other pictures listed as parts: {false_parts} over {len(positives)} probes")
    negs = [r for r in rows if r["kind"] == "negative"]
    print(f"negatives: {sum(1 for r in negs if r['false_hits'] > 0)} of {len(negs)} probes had a false region hit (max inliers {max([r['max_inliers'] for r in negs], default=0)})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
