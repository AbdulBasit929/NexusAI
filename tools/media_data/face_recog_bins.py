#!/usr/bin/env python3
"""Recognition accuracy on the five 112x112 verification sets (gates R1, R3, R4 of docs/data/face-image-search-preregistration-20261119.md).

    docker run --rm --network none --memory 2000m -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro \
        --entrypoint python nexusai-media-eval-face:20261119 /repo/tools/media_data/face_recog_bins.py --models sface auraface --sets lfw

Embeddings of every unique image are written once to /work/emb (a second run reuses them). Cosine similarity of L2-normalised embeddings; 10-fold accuracy as in InsightFace's verification code;
true accept rate at false accept rate 1e-3 on the set's impostor pairs with a bootstrap interval; accuracy at the fixed cosine threshold 0.363 that the OpenCV documentation gives for SFace.
--flip adds the horizontal-flip test (embedding of the image plus embedding of its mirror), which some published numbers use.
"""
import argparse
import json
import sys
import time
from pathlib import Path

import numpy as np

sys.path.insert(0, "/repo/tools/media_data")
import face_common as fc  # noqa: E402


def embed_set(model, name, flip, limit, threads):
    out = Path(fc.WORK) / "emb"
    out.mkdir(parents=True, exist_ok=True)
    suffix = "-flip" if flip else ""
    path = out / f"{model}-{name}{suffix}.npy"
    uniq = np.load(Path(fc.WORK) / "bins" / name / "uniq.npy", mmap_mode="r")
    n = len(uniq) if not limit else min(limit, len(uniq))
    if path.exists() and not limit:
        return np.load(path), None, None
    emb = fc.embedder(model, threads)
    chunks, per_image = [], []
    started = time.perf_counter()
    # checkpoints: every block of 512 images is written once, so a killed run resumes where it stopped (the host watchdog killed the first AuraFace LFW run after 20 minutes)
    part_dir = out / "part"
    part_dir.mkdir(exist_ok=True)
    block = 512
    for b0 in range(0, n, block):
        part = part_dir / f"{model}-{name}{suffix}-{b0:06d}.npy"
        if part.exists() and not limit:
            chunks.append(np.load(part))
            continue
        block_chunks = []
        for i in range(b0, min(n, b0 + block), 64):
            batch = [np.ascontiguousarray(uniq[j]) for j in range(i, min(n, b0 + block, i + 64))]
            t0 = time.perf_counter()
            e = emb.embed(batch)
            if flip:
                e = fc.l2n(e) + fc.l2n(emb.embed([b[:, ::-1].copy() for b in batch]))
            per_image.append((time.perf_counter() - t0) / len(batch))
            block_chunks.append(fc.l2n(e))
        done = np.concatenate(block_chunks)
        if not limit:
            np.save(part, done)
        chunks.append(done)
        print(f"  {model} {name}{suffix}: {min(n, b0 + block)}/{n} images, {time.perf_counter() - started:.0f} s, rss {fc.rss_mb():.0f} MB", flush=True)
    vectors = np.concatenate(chunks)
    if not limit:
        np.save(path, vectors)
        for part in part_dir.glob(f"{model}-{name}{suffix}-*.npy"):
            part.unlink()
    return vectors, (float(np.median(per_image)) if per_image else None), fc.rss_mb()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--models", nargs="+", default=["sface", "auraface"])
    parser.add_argument("--sets", nargs="+", default=["lfw", "cfp_fp", "agedb_30", "calfw", "cplfw"])
    parser.add_argument("--flip", action="store_true")
    parser.add_argument("--limit", type=int, default=0, help="embed only the first N unique images (smoke test; nothing is saved)")
    parser.add_argument("--threads", type=int, default=0)
    args = parser.parse_args()
    results_dir = Path(fc.WORK) / "results"
    results_dir.mkdir(parents=True, exist_ok=True)
    rng = np.random.default_rng(20261119)
    for model in args.models:
        for name in args.sets:
            vectors, per_image, rss = embed_set(model, name, args.flip, args.limit, args.threads or None)
            pairs = np.load(Path(fc.WORK) / "bins" / name / "pairs.npy")
            issame = np.load(Path(fc.WORK) / "bins" / name / "issame.npy")
            if args.limit:
                keep = (pairs < len(vectors)).all(axis=1)
                pairs, issame = pairs[keep], issame[keep]
                print(f"  smoke: {len(pairs)} pairs inside the first {len(vectors)} images; per image {per_image * 1000:.1f} ms, rss {rss:.0f} MB")
                continue
            sims = np.sum(vectors[pairs[:, 0]] * vectors[pairs[:, 1]], axis=1)
            acc, std, thr = fc.folds_accuracy(sims, issame)
            pos, neg = sims[issame], sims[~issame]
            tar, thr_far = fc.tar_at_far(pos, neg, 1e-3)
            boot = []
            for _ in range(500):
                p = rng.choice(pos, len(pos))
                g = rng.choice(neg, len(neg))
                boot.append(fc.tar_at_far(p, g, 1e-3)[0])
            fixed = float(np.mean((sims >= 0.363) == issame))
            row = {"model": model, "set": name, "flip": args.flip, "pairs": int(len(issame)), "impostor_pairs": int((~issame).sum()), "accuracy_10fold": round(acc * 100, 3), "std": round(std * 100, 3), "mean_threshold": round(thr, 4),
                   "tar_at_far_1e-3": round(tar * 100, 2), "tar_ci95": [round(float(np.percentile(boot, 2.5)) * 100, 2), round(float(np.percentile(boot, 97.5)) * 100, 2)], "far_threshold": round(thr_far, 4),
                   "accuracy_at_cos_0.363": round(fixed * 100, 3), "median_ms_per_image_batch": None if per_image is None else round(per_image * 1000, 2), "peak_rss_mb": None if rss is None else round(rss), "unique_images": int(len(vectors))}
            print(json.dumps(row), flush=True)
            json.dump(row, open(results_dir / f"recog-bins-{model}-{name}{'-flip' if args.flip else ''}.json", "w"), indent=1)


if __name__ == "__main__":
    main()
