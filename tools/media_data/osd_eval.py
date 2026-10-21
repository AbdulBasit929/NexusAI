#!/usr/bin/env python3
"""How often does Tesseract's orientation and script detection (osd) name the right script? Used to route Arabic-script pages (Urdu) to the Urdu language data.

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest python /repo/tools/media_data/osd_eval.py
"""
import collections
import hashlib
import io
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

import pyarrow.parquet as pq

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
TESS = str(Path(DATA) / "tessdata" / "fast")  # osd.traineddata was fetched with the fast set (the same file in both repositories)


def partition(key):
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def osd(path):
    env = dict(os.environ, TESSDATA_PREFIX=TESS, OMP_THREAD_LIMIT="1")
    done = subprocess.run(["tesseract", str(path), "stdout", "--psm", "0", "-l", "osd"], capture_output=True, env=env, timeout=60)
    text = done.stdout.decode("utf-8", "replace") + done.stderr.decode("utf-8", "replace")
    script = re.search(r"Script: (\w+)", text)
    confidence = re.search(r"Script confidence: ([\d.]+)", text)
    return (script.group(1) if script else None), (float(confidence.group(1)) if confidence else None)


def main():
    sets = collections.OrderedDict()
    rows = [json.loads(l) for l in open(Path(DATA) / "generated" / "ocr_ur" / "truth.jsonl", encoding="utf-8")]
    sets["urdu pages (Arabic expected)"] = ([Path(DATA) / "generated" / "ocr_ur" / r["file"] for r in rows if r["kind"] == "page" and r["partition"] == "dev"], "Arabic")
    sets["urdu lines (Arabic expected)"] = ([Path(DATA) / "generated" / "ocr_ur" / r["file"] for r in rows if r["kind"] == "line" and r["partition"] == "dev"][:60], "Arabic")
    rows = [json.loads(l) for l in open(Path(DATA) / "generated" / "ocr_en" / "truth.jsonl", encoding="utf-8")]
    sets["english pages (Latin expected)"] = ([Path(DATA) / "generated" / "ocr_en" / r["file"] for r in rows if r["partition"] == "dev"][:60], "Latin")
    tmp = Path(tempfile.mkdtemp())
    table = pq.read_table(Path(DATA) / "cord" / "test.parquet")
    receipts = []
    for i in range(table.num_rows):
        if partition(f"cord-{i}") != "dev":
            continue
        image = table.slice(i, 1).to_pylist()[0]["image"]
        path = tmp / f"cord-{i}.png"
        path.write_bytes(image["bytes"] if isinstance(image, dict) else image)
        receipts.append(path)
    sets["receipts (Latin expected)"] = (receipts[:60], "Latin")
    for name, (paths, expected) in sets.items():
        tally = collections.Counter()
        for path in paths:
            script, confidence = osd(path)
            tally[script or "no answer"] += 1
        print(f"{name:34} n={len(paths):3}  expected {expected}: {dict(tally)}")


if __name__ == "__main__":
    main()
