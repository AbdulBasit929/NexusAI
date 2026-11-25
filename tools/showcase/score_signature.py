#!/usr/bin/env python3
"""Scores the same-footage signature (video_overview.video_signature / compare_signatures) against the generated signature sets (docs/data/video-overview-preregistration-20261125.md, gate MF1-F).

    docker run --rm --network none --entrypoint python -v <repo>:/repo:ro -v <scenes>:/scenes:ro nexusai-media-eval:latest /repo/tools/showcase/score_signature.py --scenes /scenes --ids mf1-0007,mf1-0008

Per scene: the original (motion.mp4) against its five gated variants (three re-encodes, a 5% crop, a mirror), which must match (recall), against the two hard negatives of the scene (the same camera view
recorded again, H1 with the same light and H2 at another time of day), which must not, and against the originals of the other scenes (unrelated pairs, which must not). The trimmed variant is reported
apart (not gated). Gate: recall >= 95% at precision >= 99%, with at least 200 unrelated pairs and 50 hard negatives in the sealed run. Dev scenes choose the thresholds; the sealed scenes are scored once.
"""
import argparse
import json
import os
import sys

sys.path.insert(0, "/repo/ingestion/forensic_records")
sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "ingestion", "forensic_records"))

import video_overview as vo  # noqa: E402

GATED = ["reenc_low", "reenc_mid", "reenc_high", "crop5", "mirror"]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--scenes", required=True)
    parser.add_argument("--ids", required=True)
    parser.add_argument("--bits", type=int, default=12)
    parser.add_argument("--fraction", type=float, default=0.8)
    parser.add_argument("--correlation", type=float, default=0.7)
    parser.add_argument("--containment", type=float, default=0.6)
    args = parser.parse_args()
    ids = args.ids.split(",")

    def signature(path):
        cache = path + ".sig.json"
        if os.path.exists(cache):
            return json.load(open(cache))
        sig = vo.video_signature(path)
        try:
            json.dump(sig, open(cache, "w"))
        except OSError:
            pass
        return sig

    sigs = {}
    for sid in ids:
        base = os.path.join(args.scenes, sid)
        sigs[sid] = {"orig": signature(os.path.join(base, "motion.mp4"))}
        for name in GATED + ["trim", "h1", "h2"]:
            path = os.path.join(base, "signature", name + ".mp4")
            if os.path.exists(path):
                sigs[sid][name] = signature(path)
        print(f"{sid} signatures: {sorted(sigs[sid])}", flush=True)

    def same(a, b):
        return vo.compare_signatures(a, b, bit_threshold=args.bits, match_fraction=args.fraction, motion_correlation=args.correlation, layout_containment=args.containment)

    tp = fn = fp = tn = 0
    miss = []
    trim_ok = trim_n = 0
    hard = unrelated = 0
    for sid in ids:
        o = sigs[sid]["orig"]
        for name in GATED:
            if name in sigs[sid]:
                r = same(o, sigs[sid][name])
                if r["same"]:
                    tp += 1
                else:
                    fn += 1
                    miss.append((sid, name, r["fraction"], r["motion_agrees"]))
        if "trim" in sigs[sid]:
            trim_n += 1
            trim_ok += same(o, sigs[sid]["trim"])["same"]
        for name in ("h1", "h2"):
            if name in sigs[sid]:
                hard += 1
                if same(o, sigs[sid][name])["same"]:
                    fp += 1
                    miss.append((sid, "FALSE MATCH " + name, None, None))
                else:
                    tn += 1
    for i, a in enumerate(ids):
        for b in ids[i + 1:]:
            unrelated += 1
            if same(sigs[a]["orig"], sigs[b]["orig"])["same"]:
                fp += 1
                miss.append((a, "FALSE MATCH unrelated " + b, None, None))
            else:
                tn += 1
    recall = tp / max(1, tp + fn)
    precision = tp / max(1, tp + fp)
    print(f"TOTAL same-footage variants matched {tp}/{tp + fn} = recall {100 * recall:.1f}% | false matches {fp} of {hard} hard negatives and {unrelated} unrelated pairs -> precision {100 * precision:.1f}% | "
          f"trimmed copy (not gated) {trim_ok}/{trim_n}")
    for m in miss:
        print("  ", m)


if __name__ == "__main__":
    main()
