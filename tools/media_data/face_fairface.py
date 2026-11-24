#!/usr/bin/env python3
"""FairFace validation, stage 1: detection with YuNet and BlazeFace, YuNet alignment, embeddings from both recognisers, and degraded copies for the Indian and White groups
(docs/data/face-image-search-preregistration-20261119.md, protocols D and R; no identity labels exist, no name is stored).

    docker run --rm --network none --memory 3000m -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro \
        --entrypoint python nexusai-media-eval-face:20261119 /repo/tools/media_data/face_fairface.py [--limit 200]

Output in /work/fairface: detections.json (per image the YuNet and BlazeFace boxes with scores), emb-<model>.npy and degr-emb-<model>.npy (rows of NaN where no face was aligned), degr-index.npy, timing.json.
Alignment: the highest-scoring YuNet face with score >= 0.6, aligned with SFace's alignCrop to the 112x112 ArcFace template. The degraded copy is: brightness x0.7, Gaussian blur sigma 1.5, 50% down-scale then back up, JPEG quality 30.
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
from face_detect_wider import BlazeFace, YuNet  # noqa: E402

INDIAN, WHITE = 1, 3


def degrade(bgr):
    out = np.clip(bgr.astype(np.float32) * 0.7, 0, 255).astype(np.uint8)
    out = cv2.GaussianBlur(out, (0, 0), 1.5)
    h, w = out.shape[:2]
    out = cv2.resize(cv2.resize(out, (w // 2, h // 2), interpolation=cv2.INTER_AREA), (w, h), interpolation=cv2.INTER_LINEAR)
    ok, enc = cv2.imencode(".jpg", out, [cv2.IMWRITE_JPEG_QUALITY, 30])
    return cv2.imdecode(enc, cv2.IMREAD_COLOR)


def main():
    import pyarrow.parquet as pq
    parser = argparse.ArgumentParser()
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--threads", type=int, default=0)
    args = parser.parse_args()
    out = Path(fc.WORK) / "fairface"
    out.mkdir(parents=True, exist_ok=True)
    labels = json.load(open(out / "fairface-labels.json"))
    race = labels["race"]
    n_total = len(race) if not args.limit else min(args.limit, len(race))
    yun, blaze = YuNet(), BlazeFace()
    models = {m: fc.embedder(m, args.threads or None) for m in ("sface", "auraface")}
    emb = {m: np.full((n_total, e.dim), np.nan, np.float32) for m, e in models.items()}
    degr_idx = [i for i in range(n_total) if race[i] in (INDIAN, WHITE)]
    degr_pos = {i: k for k, i in enumerate(degr_idx)}
    degr = {m: np.full((len(degr_idx), e.dim), np.nan, np.float32) for m, e in models.items()}
    detections, started, i0 = {}, time.perf_counter(), 0
    t_yun, t_blaze = [], []
    # checkpoint: the state is written every 640 images and a killed run resumes there (the host memory watchdog can kill a test container)
    ckpt = out / "partial"
    ckpt.mkdir(exist_ok=True)
    resume_at = 0
    if not args.limit and (ckpt / "progress.json").exists():
        resume_at = json.load(open(ckpt / "progress.json"))["next"]
        detections = {int(k): v for k, v in json.load(open(ckpt / "detections.json")).items()}
        for m in models:
            emb[m] = np.load(ckpt / f"emb-{m}.npy")
            degr[m] = np.load(ckpt / f"degr-{m}.npy")
        print(f"resuming at image {resume_at}", flush=True)
    pf = pq.ParquetFile(Path(fc.DATA) / "datasets" / "face" / "fairface" / "validation-1.25.parquet")
    for batch in pf.iter_batches(batch_size=64, columns=["image"]):
        if i0 < resume_at:
            i0 += batch.num_rows
            continue
        rows = batch.column("image").to_pylist()
        crops, crop_ids, dcrops, dcrop_ids = [], [], [], []
        for k, cell in enumerate(rows):
            i = i0 + k
            if i >= n_total:
                break
            bgr = cv2.imdecode(np.frombuffer(cell["bytes"], np.uint8), cv2.IMREAD_COLOR)
            t0 = time.perf_counter()
            dy = yun.detect(bgr)
            t_yun.append(time.perf_counter() - t0)
            t0 = time.perf_counter()
            db = blaze.detect(bgr)
            t_blaze.append(time.perf_counter() - t0)
            detections[i] = {"size": [bgr.shape[1], bgr.shape[0]], "yunet": [[round(v, 1) for v in d[:4]] + [round(d[4], 4)] for d in dy[:10]], "blazeface": [[round(v, 1) for v in d[:4]] + [round(d[4], 4)] for d in db[:10]]}
            crop, _, _ = fc.align_with_yunet(bgr, yun.det, 0.6)
            if crop is not None:
                crops.append(crop)
                crop_ids.append(i)
            if i in degr_pos:
                dcrop, _, _ = fc.align_with_yunet(degrade(bgr), yun.det, 0.6)
                if dcrop is not None:
                    dcrops.append(dcrop)
                    dcrop_ids.append(degr_pos[i])
        for m, e in models.items():
            if crops:
                emb[m][crop_ids] = e.embed(crops)
            if dcrops:
                degr[m][dcrop_ids] = e.embed(dcrops)
        i0 += len(rows)
        print(f"  fairface {min(i0, n_total)}/{n_total}  {time.perf_counter() - started:.0f} s  rss {fc.rss_mb():.0f} MB", flush=True)
        if not args.limit and (i0 % 640 == 0):
            for m in models:
                np.save(ckpt / f"emb-{m}.npy", emb[m])
                np.save(ckpt / f"degr-{m}.npy", degr[m])
            json.dump(detections, open(ckpt / "detections.json", "w"))
            json.dump({"next": i0}, open(ckpt / "progress.json", "w"))
        if i0 >= n_total:
            break
    if args.limit:
        print("smoke run: nothing saved;", "aligned", int((~np.isnan(emb['sface'][:, 0])).sum()), "of", n_total, "median ms yunet", 1000 * np.median(t_yun), "blaze", 1000 * np.median(t_blaze))
        return
    json.dump(detections, open(out / "detections.json", "w"))
    for m in models:
        np.save(out / f"emb-{m}.npy", emb[m])
        np.save(out / f"degr-emb-{m}.npy", degr[m])
    np.save(out / "degr-index.npy", np.asarray(degr_idx))
    json.dump({"median_ms_yunet_448px": round(1000 * float(np.median(t_yun)), 2), "median_ms_blazeface_448px": round(1000 * float(np.median(t_blaze)), 2), "seconds_total": round(time.perf_counter() - started),
               "peak_rss_mb": round(fc.rss_mb())}, open(out / "timing.json", "w"))
    print("done", round(time.perf_counter() - started), "s")


if __name__ == "__main__":
    main()
