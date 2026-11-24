#!/usr/bin/env python3
"""End-to-end path on the original LFW images (gate R2 of docs/data/face-image-search-preregistration-20261119.md): YuNet detection, alignCrop to the 112x112 ArcFace template, then each recogniser,
with the standard 6,000 pairs of pairs.txt (10 folds of 300 same + 300 different pairs, in file order).

    docker run --rm --network none --memory 3000m -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro \
        --entrypoint python nexusai-media-eval-face:20261119 /repo/tools/media_data/face_lfw_e2e.py --models sface auraface

The archive /data/datasets/face/lfw-orig/lfw.tgz is read in memory (nothing is unpacked: 13,000 small files are very slow to create on the bind mount); pairs come from pairs.txt next to it. A pair with an image
where no face was found counts as a wrong answer (similarity set to -1) and is reported as lost faces.
"""
import argparse
import json
import sys
from pathlib import Path

import cv2
import numpy as np

sys.path.insert(0, "/repo/tools/media_data")
import face_common as fc  # noqa: E402


def read_pairs(root):
    lines = (root / "pairs.txt").read_text().split("\n")
    folds, per = [int(v) for v in lines[0].split()]
    pairs, issame = [], []
    row = 1
    for _ in range(folds):
        for _ in range(per):
            name, a, b = lines[row].split()
            pairs.append((f"{name}/{name}_{int(a):04d}.jpg", f"{name}/{name}_{int(b):04d}.jpg"))
            issame.append(True)
            row += 1
        for _ in range(per):
            n1, a, n2, b = lines[row].split()
            pairs.append((f"{n1}/{n1}_{int(a):04d}.jpg", f"{n2}/{n2}_{int(b):04d}.jpg"))
            issame.append(False)
            row += 1
    return pairs, np.asarray(issame)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--models", nargs="+", default=["sface", "auraface"])
    parser.add_argument("--min-score", type=float, default=0.6)
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--select", choices=("score", "centre"), default="score", help="which detected face to align: the highest-scoring one (registered) or the one nearest the image centre (post-hoc diagnostic, Amendment A3)")
    args = parser.parse_args()
    root = Path(fc.DATA) / "datasets" / "face" / "lfw-orig"
    pairs, issame = read_pairs(root)
    if args.limit:
        pairs, issame = pairs[:args.limit], issame[:args.limit]
    names = sorted({n for p in pairs for n in p})
    import tarfile
    wanted, blobs = {f"lfw/{n}" for n in names}, {}
    with tarfile.open(root / "lfw.tgz") as archive:
        for member in archive:
            if member.name in wanted and member.isfile():
                blobs[member.name[4:]] = archive.extractfile(member).read()
    print(f"read {len(blobs)} of {len(names)} wanted images from the archive", flush=True)
    # checkpoints (a watchdog kill of a 30-minute run used to lose everything): the aligned crops and then each model's embeddings are saved
    ck = Path(fc.WORK) / ("lfw-e2e" if args.select == "score" else "lfw-e2e-centre")
    ck.mkdir(parents=True, exist_ok=True)
    crops, lost = {}, []
    if not args.limit and (ck / "crops.npy").exists():
        have_saved = json.load(open(ck / "crops-names.json"))
        crops = dict(zip(have_saved, np.load(ck / "crops.npy")))
        lost = [n for n in names if n not in crops]
        print(f"resumed {len(crops)} aligned crops", flush=True)
    else:
        detector = fc.yunet(score=0.05)
        for k, name in enumerate(names):
            image = cv2.imdecode(np.frombuffer(blobs[name], np.uint8), cv2.IMREAD_COLOR) if name in blobs else None
            if image is None:
                lost.append(name)
                continue
            crop, n_faces, score = fc.align_with_yunet(image, detector, args.min_score, args.select)
            if crop is None:
                lost.append(name)
            else:
                crops[name] = crop
            if k % 2000 == 0:
                print(f"  aligned {k}/{len(names)} images, lost so far {len(lost)}", flush=True)
        if not args.limit:
            np.save(ck / "crops.npy", np.stack([crops[n] for n in names if n in crops]))
            json.dump([n for n in names if n in crops], open(ck / "crops-names.json", "w"))
    print(f"LFW originals: {len(names)} unique images, faces lost {len(lost)} ({100.0 * len(lost) / len(names):.2f}%)")
    report = {"select": args.select, "images": len(names), "lost": len(lost), "lost_percent": round(100.0 * len(lost) / len(names), 3), "min_score": args.min_score, "models": {}}
    have = [n for n in names if n in crops]
    for model in args.models:
        e = fc.embedder(model)
        parts = []
        for b0 in range(0, len(have), 512):
            part = ck / f"{model}-{b0:06d}.npy"
            if part.exists() and not args.limit:
                parts.append(np.load(part))
                continue
            block = fc.l2n(np.concatenate([e.embed([crops[n] for n in have[i:min(i + 64, b0 + 512, len(have))]]) for i in range(b0, min(b0 + 512, len(have)), 64)]))
            if not args.limit:
                np.save(part, block)
            parts.append(block)
            print(f"  {model}: {min(b0 + 512, len(have))}/{len(have)}", flush=True)
        vectors = np.concatenate(parts)
        index = {n: k for k, n in enumerate(have)}
        sims = np.full(len(pairs), -1.0)
        for p, (a, b) in enumerate(pairs):
            if a in index and b in index:
                sims[p] = float(vectors[index[a]] @ vectors[index[b]])
        acc, std, thr = fc.folds_accuracy(sims, issame)
        report["models"][model] = {"accuracy_10fold": round(100 * acc, 3), "std": round(100 * std, 3), "mean_threshold": round(thr, 4), "pairs": len(pairs), "pairs_with_a_lost_face": int(sum(1 for a, b in pairs if a not in index or b not in index))}
        print(model, json.dumps(report["models"][model]), flush=True)
    if not args.limit:
        Path(fc.WORK, "results").mkdir(parents=True, exist_ok=True)
        json.dump(report, open(Path(fc.WORK) / "results" / ("lfw-e2e.json" if args.select == "score" else "lfw-e2e-centre.json"), "w"), indent=1)


if __name__ == "__main__":
    main()
