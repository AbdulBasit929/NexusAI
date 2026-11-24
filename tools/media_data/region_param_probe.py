#!/usr/bin/env python3
"""Offline comparison of region-matching settings on the same probes as image_region_eval.py (same seed, same cut-outs), with no service and no database: each probe is matched against the TRUE
source picture (recall) and against other pictures of the set (chance matches). Settings compared: the SIFT contrast threshold and the number of features.

    python tools/media_data/region_param_probe.py --pack /pack --sources 24 --seed 11 --others 5     (inside the service image, repo mounted at /src, tools at /tools)

Prints, per setting: probes found at 12 or more agreeing points and at 8 or more, and the largest and the mean count of agreeing points between a probe and a picture it is NOT cut from (chance matches).
"""
import argparse
import importlib.util
import random
import sys
import time
from pathlib import Path

import cv2
import numpy as np

sys.path.insert(0, "/src")
from ingestion.forensic_records import image_region as region  # noqa: E402

spec = importlib.util.spec_from_file_location("image_region_eval", Path(__file__).with_name("image_region_eval.py"))
evalmod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(evalmod)

CONFIGS = {
    "current   contrast 0.025, 1000/1500 features": dict(contrast=0.025, probe=1000, cand=1500),
    "lower     contrast 0.012, 2000/2500 features": dict(contrast=0.012, probe=2000, cand=2500),
    "lowest    contrast 0.006, 3000/3000 features": dict(contrast=0.006, probe=3000, cand=3000),
    "default   contrast 0.040, 1000/1500 features": dict(contrast=0.040, probe=1000, cand=1500),
}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--pack", required=True)
    parser.add_argument("--sources", type=int, default=24)
    parser.add_argument("--seed", type=int, default=11)
    parser.add_argument("--others", type=int, default=5)
    args = parser.parse_args()
    pack = Path(args.pack)
    rng = random.Random(args.seed)
    in_case = sorted(p for p in (pack / "in-case").glob("*.jpg") if "-resaved" not in p.name and "-cropped" not in p.name)
    rng.shuffle(in_case)
    sources = in_case[: args.sources]
    images = {p.name: cv2.imread(str(p)) for p in sources}
    probes = []  # (source name, variant, bytes)
    for source in sources:
        for variant, (probe, _box) in evalmod.make_variants(images[source.name], rng).items():
            probes.append((source.name, variant, evalmod.png(probe) if variant == "annotated_screenshot" else evalmod.jpeg(probe, 90)))
    names = [s.name for s in sources]
    other_rng = random.Random(99)
    for label, cfg in CONFIGS.items():
        started = time.time()
        candidates = {n: region.extract(evalmod.jpeg(images[n], 92), max_edge=region.CANDIDATE_MAX_EDGE, nfeatures=cfg["cand"], contrast=cfg["contrast"]) for n in names}
        found12 = found8 = 0
        per_variant = {}
        chance = []
        for source, variant, data in probes:
            plain = region.extract(data, max_edge=region.CANDIDATE_MAX_EDGE, nfeatures=cfg["probe"], min_edge=region.PROBE_MIN_EDGE, contrast=cfg["contrast"])
            flipped = region.extract(data, max_edge=region.CANDIDATE_MAX_EDGE, nfeatures=cfg["probe"], min_edge=region.PROBE_MIN_EDGE, flip=True, contrast=cfg["contrast"])
            true_inliers = max(region.match(plain, candidates[source])["inliers"], region.match(flipped, candidates[source])["inliers"])
            found12 += true_inliers >= 12
            found8 += true_inliers >= 8
            hit = per_variant.setdefault(variant, [0, 0])
            hit[0] += true_inliers >= 12
            hit[1] += 1
            for other in other_rng.sample([n for n in names if n != source], args.others):
                chance.append(max(region.match(plain, candidates[other])["inliers"], region.match(flipped, candidates[other])["inliers"]))
        print(f"{label}: found at >=12 points {found12}/{len(probes)}, at >=8 {found8}/{len(probes)}; chance matches vs other pictures: max {max(chance)}, mean {np.mean(chance):.2f}, >=8: {sum(c >= 8 for c in chance)}, >=12: {sum(c >= 12 for c in chance)} of {len(chance)} ({time.time() - started:.0f} s)", flush=True)
        print("    by variant: " + ", ".join(f"{v} {a}/{n}" for v, (a, n) in sorted(per_variant.items())), flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
