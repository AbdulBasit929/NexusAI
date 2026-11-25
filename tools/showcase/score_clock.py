#!/usr/bin/env python3
"""Scores the clock reading of ingestion/forensic_records/video_overview.py against the exact truth of the generated clock scenes (docs/data/video-overview-preregistration-20261125.md, gate MF1-C).

    docker run --rm --network none --entrypoint python -v <repo>:/repo:ro -v <scenes>:/scenes:ro nexusai-media-eval:latest /repo/tools/showcase/score_clock.py --scenes /scenes --ids mf1-0007,mf1-0012

For each scene: (a) the share of one-per-minute samples whose time is read exactly (a different valid time is wrong; "unreadable" is not wrong but not right either, and is reported); (b) every planted jump of
10 s or more must lie inside a detected jump's interval with its size within 3 s (and the share found is reported), and the detected jumps that match no planted one are false; (c) the offset error at the last
full minute (the drift test: a planted drift of 2 s per hour must be measured to 2 s after one hour). The container's creation time is the true start: the truth file's `start_time_of_day_s` is that start.
Dev scenes choose the method; the sealed scenes are scored once.
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
    parser.add_argument("--every", type=float, default=60.0)
    args = parser.parse_args()
    tot = {"samples": 0, "exact": 0, "wrong": 0, "unreadable": 0, "jumps": 0, "jumps_found": 0, "false_jumps": 0}
    hours = 0.0
    for sid in args.ids.split(","):
        base = os.path.join(args.scenes, sid)
        truth = json.load(open(os.path.join(base, "clock.truth.json")))
        box = tuple(truth["clock_box_xywh"])
        samples = vo.read_clock_samples(os.path.join(base, "clock.mp4"), box, every_seconds=args.every)
        by_t = {s["t_s"]: s for s in truth["minute_samples"]}
        exact = wrong = unreadable = 0
        for s in samples:
            want = by_t.get(int(s["position_seconds"]))
            if want is None:
                continue
            if s["shown_seconds_of_day"] is None:
                unreadable += 1
            elif vo.parse_clock_text(want["shown"]) == s["shown_seconds_of_day"]:
                exact += 1
            else:
                wrong += 1
        analysis = vo.analyse_clock(samples, truth["start_time_of_day_s"])
        planted = [j for j in truth["jumps"] if abs(j["delta_s"]) >= 10]
        found = 0
        matched = set()
        for j in planted:
            for k, d in enumerate(analysis["jumps"]):
                lo, hi = d["between_seconds"]
                if lo <= j["at_s"] <= hi + args.every and abs(d["delta_seconds"] - j["delta_s"]) <= 3 + abs(truth["drift_s_per_hour"]) * args.every / 3600.0 * 2:
                    found += 1
                    matched.add(k)
                    break
        false = len(analysis["jumps"]) - len(matched)
        last = truth["minute_samples"][-1]
        est = vo.clock_offset_at(analysis, last["t_s"])
        err = None if est is None else abs(est - last["offset_s"])
        d_est, d_true = analysis["drift_seconds_per_hour"], truth["drift_s_per_hour"]
        print(f"{sid} [{truth['condition']:10}] samples {len(samples):3d} exact {exact:3d} wrong {wrong:2d} unreadable {unreadable:2d} | jumps planted {len(planted)} found {found} false {false} | "
              f"drift true {d_true:+.1f} est {d_est} s/h | offset error at {last['t_s']} s: {None if err is None else round(err, 1)} s", flush=True)
        tot["samples"] += exact + wrong + unreadable
        tot["exact"] += exact
        tot["wrong"] += wrong
        tot["unreadable"] += unreadable
        tot["jumps"] += len(planted)
        tot["jumps_found"] += found
        tot["false_jumps"] += false
        hours += truth["duration_s"] / 3600.0
    print(f"TOTAL exact {tot['exact']}/{tot['samples']} = {100.0 * tot['exact'] / max(1, tot['samples']):.1f}% (wrong {tot['wrong']}, unreadable {tot['unreadable']}) | jumps found {tot['jumps_found']}/{tot['jumps']} | "
          f"false jumps {tot['false_jumps']} in {hours:.1f} h of video")


if __name__ == "__main__":
    main()
