#!/usr/bin/env python3
"""Summarise OCR result files (C:/tmp/nexusai-media-data/results/ocr/*.json) per group: character error, bag-of-words F1, identifier exactness, seconds per image.

    python -X utf8 tools/media_data/summarise_results.py ur_pages-dev-tess_best        # files whose names start with this prefix (newest per engine)
"""
import glob
import json
import os
import statistics as st
import sys

ROOT = os.environ.get("NEXUSAI_MEDIA_DATA", "C:/tmp/nexusai-media-data") + "/results/ocr/"


def main(prefix):
    latest = {}
    for path in sorted(glob.glob(ROOT + prefix + "*.json")):
        data = json.load(open(path))
        latest[data["engine"]] = data
    for engine, data in latest.items():
        groups = {}
        for row in data["rows"]:
            groups.setdefault(row["group"], []).append(row)
        print(f"{data['set']} {data['partition']} {engine}")
        for group, rows in sorted(groups.items()):
            ids_t, ids_h = sum(r["ids_total"] for r in rows), sum(r["ids_hit"] for r in rows)
            idtext = f"  identifiers {100 * ids_h / ids_t:5.1f}%" if ids_t else ""
            print(f"  {group:22} n={len(rows):3} CER mean {100 * st.mean(r['cer'] for r in rows):5.1f}% median {100 * st.median(r['cer'] for r in rows):5.1f}%  F1 {100 * st.mean(r['f1'] for r in rows):5.1f}%  {st.mean(r['seconds'] for r in rows):4.1f} s{idtext}")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else "")
