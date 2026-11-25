#!/usr/bin/env python3
"""Where each plate a person could read is lost in production (docs/data/video-plates-recall-round2-20261125.md, protocol A2).

Inputs (private files outside the repository; plate text is read but never printed):
    --candidates  <tag>-candidates.json from census_oracle.py sheets
    --labels      labels-<tag>.json written by the person who looked at the contact sheets: {"labels": {"<candidate index>": {"v": "P" | "p" | "x", "text": "..."}}}
                  P = a plate a person can read (text given), d = another view of plate P given in "of", p = a plate but not readable, x = not a plate
    --prod        stored production reads and groups of the same video (dump_prod_reads.sql), one JSON per line
    --video       file name of the video in that dump

Classes for each readable plate P (counts and candidate numbers only):
    never_detected      no production read overlaps the plate on any frame where the census saw it
    detected_misread    production boxed it, but never read its text within one character
    read_1_2_frames     read within one character on one or two of those frames (below the 3-frame minimum)
    read_3_no_group     read within one character on three or more frames, no production group carries the text
    group_exact / group_near   a production group with that text overlaps it in time (exact / one character apart)
"""
import argparse
import json
from collections import Counter, defaultdict


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


def overlaps(a, b):
    x1, y1, x2, y2 = max(a[0], b[0]), max(a[1], b[1]), min(a[2], b[2]), min(a[3], b[3])
    inter = max(0, x2 - x1) * max(0, y2 - y1)
    ua = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    if ua and inter / ua >= 0.2:
        return True
    cax, cay, cbx, cby = (a[0] + a[2]) / 2, (a[1] + a[3]) / 2, (b[0] + b[2]) / 2, (b[1] + b[3]) / 2
    return (b[0] <= cax <= b[2] and b[1] <= cay <= b[3]) or (a[0] <= cbx <= a[2] and a[1] <= cby <= a[3])


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--candidates", required=True)
    parser.add_argument("--labels", required=True)
    parser.add_argument("--prod", required=True)
    parser.add_argument("--video", required=True)
    args = parser.parse_args()
    cands = {c["index"]: c for c in json.load(open(args.candidates, encoding="utf-8"))["candidates"]}
    labels = json.load(open(args.labels, encoding="utf-8"))["labels"]
    reads_by_frame, groups = defaultdict(list), []
    for line in open(args.prod, encoding="utf-8"):
        d = json.loads(line)
        if d["file"] != args.video:
            continue
        if d["kind"] == "read":
            reads_by_frame[d["frame"]].append(d)
        else:
            groups.append(d)
    classes, by_class, sizes = Counter(), defaultdict(list), defaultdict(Counter)
    census = Counter()
    extra_rows = defaultdict(list)    # other views of the same plate ("d": a duplicate candidate of plate `of`) add their frames to it
    for key, lab in labels.items():
        if lab["v"] == "d":
            extra_rows[int(lab["of"])].extend(cands[int(key)]["rows"])
    for key, lab in labels.items():
        census[lab["v"]] += 1
        if lab["v"] != "P":
            continue
        c = cands[int(key)]
        text = norm(lab["text"])
        covered = ok_frames = near_frames = 0
        rows = c["rows"] + extra_rows[int(key)]
        for row in rows:
            hit = False
            for r in reads_by_frame.get(row["frame"], []):
                if overlaps(row["box"], [r["x"], r["y"], r["x"] + r["w"], r["y"] + r["h"]]):
                    hit = True
                    if edit_distance(norm(r["text"]), text) <= 1:
                        ok_frames += 1
                        break
            covered += hit
        t0, t1 = min(r["t"] for r in rows) - 0.5, max(r["t"] for r in rows) + 0.5
        gtexts = [norm(g["text"]) for g in groups if g["last"] >= t0 and g["first"] <= t1]
        if any(g == text for g in gtexts):
            cls = "group_exact"
        elif any(edit_distance(g, text) <= 1 for g in gtexts):
            cls = "group_near"
        elif ok_frames >= 3:
            cls = "read_3_no_group"
        elif ok_frames >= 1:
            cls = "read_1_2_frames"
        elif covered >= 1:
            cls = "detected_misread"
        else:
            cls = "never_detected"
        width = max(r["box"][2] - r["box"][0] for r in rows)
        size = "<30 px" if width < 30 else "30-50 px" if width < 50 else "50-80 px" if width < 80 else "80+ px"
        classes[cls] += 1
        by_class[cls].append(int(key))
        sizes[size][cls] += 1
    print(f"{args.video}: candidates labelled {sum(census.values())}: readable plates {census['P']} (seen as {census['P'] + census['d']} candidates), plates not readable {census['p']}, not plates {census['x']}")
    order = ["group_exact", "group_near", "read_3_no_group", "read_1_2_frames", "detected_misread", "never_detected"]
    for cls in order:
        print(f"  {cls:18} {classes[cls]:3}   candidates {sorted(by_class[cls])}")
    found = classes["group_exact"] + classes["group_near"]
    total = census["P"]
    if total:
        print(f"  production found {found} of {total} readable plates ({100.0 * found / total:.0f}%), of which exactly {classes['group_exact']}")
    print("  by size of the best crop:")
    for size in ("<30 px", "30-50 px", "50-80 px", "80+ px"):
        if sizes[size]:
            print(f"    {size:9} " + ", ".join(f"{cls} {sizes[size][cls]}" for cls in order if sizes[size][cls]))


if __name__ == "__main__":
    main()
