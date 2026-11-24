#!/usr/bin/env python3
"""Fetch the fresh photo set of the M1 image-search acceptance (docs/data/image-search-m1-preregistration-20261120.md, amendment A4).

The COCO 2014 validation images of the Karpathy 5K test split with their captions, as published by nlphuji on Hugging Face (the same publisher and layout as the Flickr 1K set of the 2026-11-19 bake-off).
The images are Flickr photographs under Creative Commons licences and the captions are CC BY 4.0 (Chen et al., COCO Captions, arXiv:1405.0312); this copy carries no per-photo licence id. Evaluation use on this machine
only, nothing is redistributed. The original host (images.cocodataset.org) was tried first and delivered about 12 MB per minute with dropped connections (see amendment A4). Size stated before download: 818 MB (D38: under 2 GB, standing yes).

    python tools/media_data/fetch_image_search_20261120.py
"""
import json
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from fetch_faces_20261119 import ROOT, download, sha256_of  # noqa: E402

MANIFEST = ROOT / "manifest-20261120-image-search.jsonl"
HF = "https://huggingface.co/datasets/nlphuji/mscoco_2014_5k_test_image_text_retrieval/resolve/main"
ITEMS = [
    ("datasets/image/coco2014-5k/test_5k_mscoco_2014.csv", f"{HF}/test_5k_mscoco_2014.csv", "COCO 2014 val Karpathy 5K test: ids and five captions per photo (CC BY 4.0 captions)"),
    ("datasets/image/coco2014-5k/images_mscoco_2014_5k_test.zip", f"{HF}/images_mscoco_2014_5k_test.zip", "COCO 2014 val Karpathy 5K test: the 5,000 photographs (Flickr, Creative Commons)"),
]


def main():
    problems = 0
    for rel, url, note in ITEMS:
        dest = ROOT / rel
        if dest.exists():
            print(f"have  {rel} ({dest.stat().st_size} bytes)")
        else:
            started = time.time()
            try:
                download(url, dest)
            except Exception as error:
                print(f"FAIL  {rel}: {error}")
                problems += 1
                continue
            print(f"fetched {rel} ({dest.stat().st_size} bytes, {time.time() - started:.0f} s)")
        digest = sha256_of(dest)
        known = set()
        if MANIFEST.exists():
            known = {json.loads(line)["id"] + json.loads(line)["sha256"] for line in open(MANIFEST, encoding="utf-8") if line.strip()}
        if rel + digest not in known:
            with open(MANIFEST, "a", encoding="utf-8") as out:
                out.write(json.dumps({"id": rel, "url": url, "bytes": dest.stat().st_size, "sha256": digest, "note": note, "fetched": time.strftime("%Y-%m-%d")}) + "\n")
        print(f"      sha256 {digest}")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
