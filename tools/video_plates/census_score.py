#!/usr/bin/env python3
"""Scores a run of a video plate method against a census (docs/data/video-plates-recall-round2-20261125.md): of the plates a person could read, how many does the run's groups contain?

    python tools/video_plates/census_score.py --candidates .../<tag>-candidates.json --labels .../labels-<tag>.json --results .../real-<run>.json --method v4p

--results is the JSON of tools/video_plates/evaluate_real.py (groups with plate text, first and last second). A census plate counts as found when a group whose text is within one character of the
person's reading overlaps the plate's time (half a second of slack), exact when the text is equal. The census files are private (plate text); nothing is printed but counts, candidate numbers and sizes.
"""
import argparse
import json
from collections import defaultdict


def edit_distance(a, b):
    prev = list(range(len(b) + 1))
    for i, ca in enumerate(a, 1):
        cur = [i]
        for j, cb in enumerate(b, 1):
            cur.append(min(prev[j] + 1, cur[-1] + 1, prev[j - 1] + (ca != cb)))
        prev = cur
    return prev[-1]


def norm(t):
    return "".join(ch for ch in (t or "").upper() if ch.isalnum())


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--candidates", required=True)
    parser.add_argument("--labels", required=True)
    parser.add_argument("--results", required=True)
    parser.add_argument("--method", default="v4p")
    parser.add_argument("--false-groups", action="store_true", help="also count the run's groups that match no census plate (a spurious group, or a plate the census did not label)")
    args = parser.parse_args()
    cands = {c["index"]: c for c in json.load(open(args.candidates, encoding="utf-8"))["candidates"]}
    labels = json.load(open(args.labels, encoding="utf-8"))["labels"]
    run = json.load(open(args.results, encoding="utf-8"))["results"][args.method]
    groups = [g for g in run["groups"] if g.get("support", 3) >= 1]
    extra = defaultdict(list)
    for key, lab in labels.items():
        if lab["v"] == "d":
            extra[int(lab["of"])].extend(cands[int(key)]["rows"])
    found_exact = found_near = total = 0
    missed, sizes = [], defaultdict(lambda: [0, 0])
    matched_groups = set()
    for key, lab in labels.items():
        if lab["v"] != "P":
            continue
        total += 1
        rows = cands[int(key)]["rows"] + extra[int(key)]
        t0, t1 = min(r["t"] for r in rows) - 0.5, max(r["t"] for r in rows) + 0.5
        text = norm(lab["text"])
        best = None
        for gi, g in enumerate(groups):
            if g["last"] < t0 or g["first"] > t1:
                continue
            d = edit_distance(norm(g["plate"]), text)
            if d <= 1 and (best is None or d < best[0]):
                best = (d, gi)
        width = max(r["box"][2] - r["box"][0] for r in rows)
        size = "<50 px" if width < 50 else "50-80 px" if width < 80 else "80+ px"
        sizes[size][1] += 1
        if best:
            sizes[size][0] += 1
            matched_groups.add(best[1])
            found_exact += best[0] == 0
            found_near += best[0] == 1
        else:
            missed.append(int(key))
    print(f"{args.method}: census plates {total}; found {found_exact + found_near} ({found_exact} exactly, {found_near} one character apart); missed {len(missed)} {sorted(missed)}")
    print("  by size of the plate: " + ", ".join(f"{s} {v[0]}/{v[1]}" for s, v in sorted(sizes.items())))
    if args.false_groups:
        print(f"  groups of the run {len(groups)}; not matched to a census plate: {len(groups) - len(matched_groups)}")


if __name__ == "__main__":
    main()
