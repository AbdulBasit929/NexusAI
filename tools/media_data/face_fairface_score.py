#!/usr/bin/env python3
"""FairFace, stage 2: operating thresholds, false-match rate by race group, synthetic genuine proxy, detection rate by group (gates D3, R5, R6 of docs/data/face-image-search-preregistration-20261119.md,
with Amendment A1: the operating threshold is taken on the pooled WITHIN-GROUP impostor pairs).

    docker run --rm --network none --memory 3000m -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-face:20261119 \
        /repo/tools/media_data/face_fairface_score.py --yunet-threshold 0.6 --blaze-threshold 0.5

No identity labels exist: every pair of different images counts as an impostor pair. Bootstrap: images are resampled (200 draws) and the rate is the weighted share of distinct-image pairs above the threshold.
"""
import argparse
import json
import sys
from pathlib import Path

import numpy as np

sys.path.insert(0, "/repo/tools/media_data")
import face_common as fc  # noqa: E402

NAMES = ["East Asian", "Indian", "Black", "White", "Middle Eastern", "Latino_Hispanic", "Southeast Asian"]
BINS = 40000  # histogram resolution 5e-5 on [-1, 1]


def within_group_histogram(E, groups):
    """Histogram of cosine values over all distinct-image pairs INSIDE each group, pooled; plus per group histograms."""
    edges = np.linspace(-1.0, 1.0, BINS + 1)
    per_group = {}
    for g, idx in groups.items():
        h = np.zeros(BINS, dtype=np.int64)
        V = E[idx]
        for s in range(0, len(idx), 1024):
            block = V[s:s + 1024] @ V.T
            # keep each unordered pair once: columns strictly after the row's own index
            rows = np.arange(s, min(s + 1024, len(idx)))
            mask = np.arange(len(idx))[None, :] > rows[:, None]
            vals = np.clip(block[mask], -1.0, 1.0)
            h += np.histogram(vals, bins=edges)[0]
        per_group[g] = h
    return edges, per_group


def threshold_for(hist, edges, target):
    """Smallest cosine value whose upper-tail share of pairs is <= target."""
    tail = np.cumsum(hist[::-1])[::-1] / hist.sum()
    ok = np.where(tail <= target)[0]
    return float(edges[ok[0]]) if len(ok) else 1.0


def fmr_with_ci(V, thr, rng, draws=200):
    n = len(V)
    S = V @ V.T
    H = (S >= thr).astype(np.float32)
    np.fill_diagonal(H, 0.0)
    ones = np.ones(n, np.float32)

    def rate(w):
        num = float(w @ H @ w) / 2.0
        den = (float(w.sum()) ** 2 - float((w * w).sum())) / 2.0
        return num / den if den > 0 else float("nan")
    point = rate(ones)
    boots = []
    for _ in range(draws):
        w = np.bincount(rng.integers(0, n, n), minlength=n).astype(np.float32)
        boots.append(rate(w))
    return point, boots


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--yunet-threshold", type=float, required=True)
    parser.add_argument("--blaze-threshold", type=float, required=True)
    args = parser.parse_args()
    out = Path(fc.WORK) / "fairface"
    labels = json.load(open(out / "fairface-labels.json"))
    race = np.asarray(labels["race"])
    det = json.load(open(out / "detections.json"))
    report = {"images": int(len(race)), "groups": {NAMES[k]: int((race == k).sum()) for k in range(7)}}
    rng = np.random.default_rng(20261119)

    # ---- D3: detection rate by group (hit = a detection above the threshold whose centre lies in the central quarter of the image)
    for name, thr in (("yunet", args.yunet_threshold), ("blazeface", args.blaze_threshold)):
        hits = np.zeros(len(race), bool)
        for i in range(len(race)):
            d = det[str(i)]
            w, h = d["size"]
            for x0, y0, x1, y1, s in d[name]:
                cx, cy = (x0 + x1) / 2, (y0 + y1) / 2
                if s >= thr and w * 0.25 <= cx <= w * 0.75 and h * 0.25 <= cy <= h * 0.75:
                    hits[i] = True
                    break
        rates = {NAMES[k]: round(100 * float(hits[race == k].mean()), 2) for k in range(7)}
        report[f"detection_rate_{name}"] = {"threshold": thr, "overall": round(100 * float(hits.mean()), 2), "by_group": rates, "spread_pp": round(max(rates.values()) - min(rates.values()), 2)}
        print(name, report[f"detection_rate_{name}"], flush=True)

    # ---- R5, R6 per model
    degr_idx = np.load(out / "degr-index.npy")
    for model in ("sface", "auraface"):
        E = np.load(out / f"emb-{model}.npy")
        D = np.load(out / f"degr-emb-{model}.npy")
        valid = ~np.isnan(E[:, 0])
        E = fc.l2n(np.nan_to_num(E))
        groups = {k: np.where((race == k) & valid)[0] for k in range(7)}
        edges, per_group = within_group_histogram(E, groups)
        pooled = sum(per_group.values())
        at363 = {NAMES[k]: float(per_group[k][edges[:-1] >= 0.363 - 1e-12].sum() / per_group[k].sum()) for k in range(7)}
        at363["pooled"] = float(pooled[edges[:-1] >= 0.363 - 1e-12].sum() / pooled.sum())
        result = {"fmr_at_cosine_0.363_the_configured_default": at363, "aligned_images": int(valid.sum()), "aligned_by_group": {NAMES[k]: f"{len(groups[k])}/{int((race == k).sum())}" for k in range(7)}, "pairs_within_group_total": int(pooled.sum()), "operating": {}}
        for target in (1e-3, 1e-4):
            thr = threshold_for(pooled, edges, target)
            row = {"threshold_cosine": round(thr, 4), "fmr_by_group": {}}
            for k in range(7):
                h = per_group[k]
                row["fmr_by_group"][NAMES[k]] = {"fmr": float(h[edges[:-1] >= thr - 1e-12].sum() / h.sum()), "pairs": int(h.sum())}
            # bootstrap intervals for the Indian and White groups and the ratio
            pi, boots_i = fmr_with_ci(E[groups[1]], thr, rng)
            pw, boots_w = fmr_with_ci(E[groups[3]], thr, rng)
            ratio_boot = [a / b for a, b in zip(boots_i, boots_w) if b and b > 0 and not np.isnan(a) and not np.isnan(b)]
            row["indian"] = {"fmr": pi, "ci95": [float(np.percentile(boots_i, 2.5)), float(np.percentile(boots_i, 97.5))]}
            row["white"] = {"fmr": pw, "ci95": [float(np.percentile(boots_w, 2.5)), float(np.percentile(boots_w, 97.5))]}
            row["ratio_indian_over_white"] = {"point": (pi / pw) if pw > 0 else None, "ci95": [float(np.percentile(ratio_boot, 2.5)), float(np.percentile(ratio_boot, 97.5))] if ratio_boot else None}
            row["ratio_indian_over_pooled"] = pi / target
            # threshold that would give the target FMR inside the Indian group alone
            row["threshold_for_target_inside_indian"] = round(threshold_for(per_group[1], edges, target), 4)
            row["threshold_for_target_inside_white"] = round(threshold_for(per_group[3], edges, target), 4)
            # R6: synthetic genuine proxy (original vs degraded copy of the same photo), false non-match at this threshold
            fn = {}
            for k in (1, 3):
                rows = [(j, p) for p, j in enumerate(degr_idx) if race[j] == k and valid[j] and not np.isnan(D[p, 0])]
                sims = np.asarray([float(E[j] @ fc.l2n(D[p:p + 1])[0]) for j, p in rows])
                fn[NAMES[k]] = {"pairs": len(rows), "fnmr_percent": round(100 * float((sims < thr).mean()), 2), "median_cosine": round(float(np.median(sims)), 4)}
            row["synthetic_proxy_fnmr"] = fn
            result["operating"][f"fmr_{target:g}"] = row
            print(model, f"FMR {target:g}", json.dumps({k: row[k] for k in ("threshold_cosine", "indian", "white", "ratio_indian_over_white", "synthetic_proxy_fnmr")}), flush=True)
        report[model] = result
    json.dump(report, open(Path(fc.WORK) / "results" / "fairface-score.json", "w"), indent=1)
    print("written /work/results/fairface-score.json")


if __name__ == "__main__":
    Path(fc.WORK, "results").mkdir(parents=True, exist_ok=True)
    main()
