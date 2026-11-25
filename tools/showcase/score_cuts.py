#!/usr/bin/env python3
"""Scores shot cut detection (video_overview.shot_cuts) against the exact truth of the generated cut sequences (docs/data/video-overview-preregistration-20261125.md, gate MF1-S).

    docker run --rm --network none --entrypoint python -v <repo>:/repo:ro -v <scenes>:/scenes:ro nexusai-media-eval:latest /repo/tools/showcase/score_cuts.py --scenes /scenes --ids mf1-0007,mf1-0008

Hard cuts: recall (a detected cut within `--frames` frames of the truth frame) and precision (a detection that matches no hard cut is false; those that fall inside a dissolve or a dip to black are
counted apart, because the gate reports dissolves and fades without gating them). Dissolves and fades: the share with a detection inside their range (reported only). Lamp-on steps and occluders
(events that are not cuts): how many detections they cause. Dev scenes choose the threshold; the sealed scenes are scored once.
"""
import argparse
import json
import os
import sys

sys.path.insert(0, "/repo/ingestion/forensic_records")
sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "ingestion", "forensic_records"))

import video_overview as vo  # noqa: E402


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--scenes", required=True)
    parser.add_argument("--ids", required=True)
    parser.add_argument("--threshold", type=float, default=0.3)
    parser.add_argument("--method", choices=["ffmpeg", "compensated", "window"], default="ffmpeg")
    parser.add_argument("--frames", type=int, default=2)
    args = parser.parse_args()
    hard = found = detections = false_strict = in_transition = on_events = 0
    soft = soft_found = 0
    for sid in args.ids.split(","):
        base = os.path.join(args.scenes, sid)
        truth = json.load(open(os.path.join(base, "cuts.truth.json")))
        fps = truth["fps"]
        path = os.path.join(base, "cuts.mp4")
        if args.method == "window":
            found_cuts = vo.shot_cuts_window(path)["cuts"]            # the chosen defaults of the module (chosen on dev)
        else:
            found_cuts = (vo.shot_cuts if args.method == "ffmpeg" else vo.shot_cuts_compensated)(path, args.threshold)["cuts"]
        cuts = [c["seconds"] for c in found_cuts]
        tol = args.frames / fps
        hard_frames = truth["hard_cut_frames"]
        soft_ranges = [(c["start_frame"] / fps - tol, c["end_frame"] / fps + tol) for c in truth["cuts"] if c["type"] != "hard"]
        event_ranges = [((e.get("frame", e.get("start_frame", 0))) / fps - tol, (e.get("frame", e.get("end_frame", 0)) + (0 if "frame" in e else 0)) / fps + tol) for e in truth["non_cut_events"]]
        s_hard = s_found = s_false = s_trans = s_event = 0
        matched = set()
        for f in hard_frames:
            s_hard += 1
            hit = next((i for i, t in enumerate(cuts) if abs(t - f / fps) <= tol and i not in matched), None)
            if hit is not None:
                s_found += 1
                matched.add(hit)
        for i, t in enumerate(cuts):
            if i in matched:
                continue
            if any(lo <= t <= hi for lo, hi in soft_ranges):
                s_trans += 1
            elif any(lo <= t <= hi for lo, hi in event_ranges):
                s_event += 1
                s_false += 1
            else:
                s_false += 1
        for lo, hi in soft_ranges:
            soft += 1
            soft_found += any(lo <= t <= hi for t in cuts)
        print(f"{sid} hard {s_hard} found {s_found} | detections {len(cuts)}: false {s_false} (of them at lamp/occluder events {s_event}), inside a dissolve or fade {s_trans}", flush=True)
        hard += s_hard
        found += s_found
        detections += len(cuts)
        false_strict += s_false + s_trans
        in_transition += s_trans
        on_events += s_event
    precision = found / max(1, found + false_strict - in_transition)
    strict = found / max(1, detections)
    print(f"TOTAL hard cuts {found}/{hard} = recall {100.0 * found / max(1, hard):.1f}% | precision {100.0 * precision:.1f}% (detections in dissolves and fades left out), strict {100.0 * strict:.1f}% | "
          f"dissolves and fades with a detection {soft_found}/{soft} | detections caused by lamp or occluder events {on_events} | {args.method} threshold {args.threshold}")


if __name__ == "__main__":
    main()
