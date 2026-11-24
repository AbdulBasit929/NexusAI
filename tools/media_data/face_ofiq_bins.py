#!/usr/bin/env python3
"""False non-match rate of SFace by OFIQ quality bin (post-hoc analysis for the plan of 2026-11-20; not a registered gate).

    docker run --rm --network none --memory 800m -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-face:20261119 /repo/tools/media_data/face_ofiq_bins.py

Uses results already on disk: the OFIQ unified quality score of the DEGRADED copy of 600 FairFace faces (300 Indian, 300 White), the SFace embeddings of the original and of the degraded copy, and the operating thresholds of the FairFace scoring
(cosine 0.669 for a false-match rate of 1e-4, 0.472 for 1e-3). The genuine pair is original against its own degraded copy, so this shows how the quality of the PROBE image changes whether a true match is found (a synthetic proxy, as everywhere in this bake-off).
"""
import json
import re
from pathlib import Path

import numpy as np

WORK = Path("/work")


def main():
    run = json.load(open(WORK / "results" / "ofiq-run.json"))
    score = json.load(open(WORK / "results" / "fairface-score.json"))
    thr = {k: score["sface"]["operating"][k]["threshold_cosine"] for k in ("fmr_0.0001", "fmr_0.001")}
    emb = np.load(WORK / "fairface" / "emb-sface.npy")
    degr = np.load(WORK / "fairface" / "degr-emb-sface.npy")
    index = np.load(WORK / "fairface" / "degr-index.npy")
    pos = {int(i): k for k, i in enumerate(index)}
    rows = []
    for f in run["fairface"]:
        kind, name = f["file"].split("/")
        if kind != "degr" or f["uqs_scalar"] is None:
            continue
        i = int(name.split("_")[1].split(".")[0])
        if i in pos and not np.isnan(emb[i, 0]) and not np.isnan(degr[pos[i], 0]):
            a, b = emb[i], degr[pos[i]]
            rows.append((float(f["uqs_scalar"]), float(a @ b / (np.linalg.norm(a) * np.linalg.norm(b))), "FAILURE" in str(f["status"])))
    q = np.array([r[0] for r in rows])
    c = np.array([r[1] for r in rows])
    out = {"pairs": len(rows), "thresholds": thr, "bins": []}
    edges = [(0, 10), (10, 20), (20, 30), (30, 40), (40, 101)]
    for lo, hi in edges:
        m = (q >= lo) & (q < hi)
        if not m.any():
            continue
        out["bins"].append({"unified_score_range": f"{lo} to {hi - 1 if hi < 101 else 100}", "pairs": int(m.sum()),
                            "median_cosine": round(float(np.median(c[m])), 3),
                            "false_non_match_percent_at_1e-4": round(100 * float((c[m] < thr["fmr_0.0001"]).mean()), 1),
                            "false_non_match_percent_at_1e-3": round(100 * float((c[m] < thr["fmr_0.001"]).mean()), 1)})
    # a candidate refusal cut-off: the lowest score at or above which the false non-match rate at 1e-4 is at most 10%
    best = None
    for cut in range(0, 60, 2):
        m = q >= cut
        if m.sum() >= 30 and float((c[m] < thr["fmr_0.0001"]).mean()) <= 0.10:
            best = {"cut_off": cut, "pairs_kept": int(m.sum()), "share_kept_percent": round(100 * float(m.mean()), 1), "false_non_match_percent_at_1e-4": round(100 * float((c[m] < thr["fmr_0.0001"]).mean()), 1)}
            break
    out["lowest_cut_off_with_false_non_match_at_most_10_percent_at_1e-4"] = best
    json.dump(out, open(WORK / "results" / "ofiq-bins-sface.json", "w"), indent=1)
    print(json.dumps(out, indent=1))


if __name__ == "__main__":
    main()
