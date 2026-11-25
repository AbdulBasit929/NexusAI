#!/usr/bin/env python3
"""Tunes the persistent-change cut detector (video_overview.shot_cuts_window) on dev scenes: the features are computed once per scene (and cached next to the video), then a grid of thresholds is scored
against the exact truth of the generated cut sequences (docs/data/video-overview-preregistration-20261125.md, gate MF1-S: hard-cut recall and precision at least 95% within 2 frames).

    docker run --rm --network none --entrypoint python -v <repo>:/repo:ro -v <scenes>:/scenes nexusai-media-eval:latest /repo/tools/showcase/tune_cuts.py --scenes /scenes --ids mf1-0007,mf1-0008

Dev scenes choose the thresholds; the sealed scenes are scored once with the chosen pair.
"""
import argparse
import json
import os
import sys

import numpy as np

sys.path.insert(0, "/repo/ingestion/forensic_records")
sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "ingestion", "forensic_records"))

import video_overview as vo  # noqa: E402


def score(truth, cuts, frames=2):
    fps = truth["fps"]
    tol = frames / fps
    times = [c["seconds"] for c in cuts]
    soft = [(c["start_frame"] / fps - tol, c["end_frame"] / fps + tol) for c in truth["cuts"] if c["type"] != "hard"]
    matched = set()
    found = 0
    for f in truth["hard_cut_frames"]:
        hit = next((i for i, t in enumerate(times) if abs(t - f / fps) <= tol and i not in matched), None)
        if hit is not None:
            found += 1
            matched.add(hit)
    false = sum(1 for i, t in enumerate(times) if i not in matched and not any(lo <= t <= hi for lo, hi in soft))
    return found, len(truth["hard_cut_frames"]), false


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--scenes", required=True)
    parser.add_argument("--ids", required=True)
    parser.add_argument("--window", type=int, default=3)
    parser.add_argument("--pixel", type=float, default=20.0, help="grey-level change that counts a pixel as changed")
    args = parser.parse_args()
    feats = {}
    for sid in args.ids.split(","):
        base = os.path.join(args.scenes, sid)
        cache = os.path.join(base, f"cuts.window{args.window}.px{int(args.pixel)}.npz")
        if os.path.exists(cache):
            z = np.load(cache)
            feats[sid] = (z["step"], z["win"], float(z["fps"]))
        else:
            step, win, fps = vo.cut_window_shares(os.path.join(base, "cuts.mp4"), args.window, args.pixel)
            try:
                np.savez(cache, step=step, win=win, fps=fps)
            except OSError:
                pass
            feats[sid] = (step, win, fps)
        print(sid, "features ready", flush=True)
    truths = {sid: json.load(open(os.path.join(args.scenes, sid, "cuts.truth.json"))) for sid in feats}
    rows = []
    for t1 in (0.04, 0.08, 0.12, 0.2, 0.3):
        for t2 in (0.04, 0.08, 0.12, 0.2, 0.3):
            found = total = false = 0
            for sid, (step, win, fps) in feats.items():
                cuts = vo.cuts_from_window_shares(step, win, fps, t1, t2, args.window)
                f, n, fa = score(truths[sid], cuts)
                found += f
                total += n
                false += fa
            precision = found / max(1, found + false)
            rows.append((found / total, precision, t1, t2, found, total, false))
    print("step_threshold window_threshold | recall precision (hard cuts found / planted, false detections outside dissolves and fades)")
    for r in sorted(rows, key=lambda r: (-(min(r[0], r[1])), -r[0]))[:12]:
        print(f"  {r[2]:.2f} {r[3]:.2f} | recall {100 * r[0]:.1f}% precision {100 * r[1]:.1f}% ({r[4]}/{r[5]}, {r[6]} false)")


if __name__ == "__main__":
    main()
