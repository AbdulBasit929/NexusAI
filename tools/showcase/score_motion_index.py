#!/usr/bin/env python3
"""Scores the motion index of ingestion/forensic_records/video_overview.py against the exact truth of the generated scenes (docs/data/video-overview-preregistration-20261125.md, gate V-A).

    docker run --rm --network none --entrypoint python -v <repo>:/repo:ro -v <scenes>:/scenes:ro nexusai-media-eval:latest /repo/tools/showcase/score_motion_index.py --scenes /scenes --ids mf1-0007,mf1-0008

For each scene: recall of truth intervals (a truth interval is found when an indexed interval overlaps it by at least half of the truth interval, or 1 second), false intervals (indexed intervals that
overlap no truth interval, with `slack` seconds of tolerance on each side, because the object is seen before and after its core frames), and the share of the scene the index covers. Then the totals: the V-A gate is
recall >= 95%, at most 1 false interval per hour of video, and for scenes whose truth covers at most 5% the index covers at most 10%. Run on dev scenes to choose parameters; the sealed scenes are scored once.
"""
import argparse
import json
import os
import sys

sys.path.insert(0, "/repo/ingestion/forensic_records")
sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "ingestion", "forensic_records"))

import video_overview as vo  # noqa: E402


def overlap(a0, a1, b0, b1):
    return max(0.0, min(a1, b1) - max(a0, b0))


def score_scene(truth, intervals, slack):
    t_iv = [(i["start_s"], i["end_s"]) for i in truth["intervals"]]
    found = 0
    for t0, t1 in t_iv:
        need = min(1.0, 0.5 * (t1 - t0))
        if any(overlap(t0, t1, i["start_seconds"], i["end_seconds"]) >= need for i in intervals):
            found += 1
    false = [i for i in intervals if not any(overlap(i["start_seconds"], i["end_seconds"], t0 - slack, t1 + slack) > 0 for t0, t1 in t_iv)]
    covered = sum(i["end_seconds"] - i["start_seconds"] for i in intervals)
    return found, len(t_iv), len(false), covered


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--scenes", required=True)
    parser.add_argument("--ids", required=True)
    parser.add_argument("--slack", type=float, default=2.0)
    for name, default in vo.MotionParameters().__dict__.items():
        parser.add_argument("--" + name.replace("_", "-"), type=type(default), default=default)
    args = parser.parse_args()
    parameters = vo.MotionParameters(**{k: getattr(args, k) for k in vo.MotionParameters().__dict__})
    tot_found = tot_truth = tot_false = 0
    tot_seconds = 0.0
    quiet_cover = []
    for sid in args.ids.split(","):
        base = os.path.join(args.scenes, sid)
        truth = json.load(open(os.path.join(base, "motion.truth.json")))
        index = vo.motion_index(os.path.join(base, "motion.mp4"), parameters)
        found, n, false, covered = score_scene(truth, index["intervals"], args.slack)
        tot_found += found
        tot_truth += n
        tot_false += false
        tot_seconds += truth["duration_s"]
        if truth["motion_fraction"] <= 0.05:
            quiet_cover.append(covered / truth["duration_s"])
        print(f"{sid} truth {n:2d} found {found:2d} false {false} index covers {100 * covered / truth['duration_s']:4.1f}% (truth {100 * truth['motion_fraction']:4.1f}%)", flush=True)
    hours = tot_seconds / 3600.0
    print(f"TOTAL recall {tot_found}/{tot_truth} = {100.0 * tot_found / max(1, tot_truth):.1f}% | false intervals {tot_false} in {hours:.2f} h = {tot_false / hours:.1f} per hour | "
          f"quiet scenes: index covers at most {100 * max(quiet_cover):.1f}%" if quiet_cover else f"TOTAL recall {tot_found}/{tot_truth}")


if __name__ == "__main__":
    main()
