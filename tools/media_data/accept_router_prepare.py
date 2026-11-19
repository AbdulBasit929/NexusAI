#!/usr/bin/env python3
"""Prepare the fresh data for the product-level acceptance of the routed OCR role (docs/roadmap/ocr-easyocr-integration-plan-20261117.md, section 4.3; gates in docs/data/ocr-easyocr-acceptance-preregistration-20261117.md).

Runs in the evaluation image (it has pyarrow and the generators' fonts). It makes three generated sets with NEW seeds (nothing a reader was tuned or scored on) and exports the newspaper rows that come
after the 60 sealed rows the bake-off read:

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest python /repo/tools/media_data/accept_router_prepare.py

Output (all under <data>/generated, outside the repository): ocr_en_accept, ocr_ur_accept, ocr_mixed_accept (generators with OCR_GEN_SEED and OCR_GEN_SUFFIX) and ocr_unb_accept (images and truth of the
newspaper rows 61 and after of the sealed order; private evaluation data of unlicensed pages, never copied into the repository or an image).
"""
import json
import os
import subprocess
import sys
from pathlib import Path

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
SCRIPTS = ("gen_ocr_pages.py", "gen_urdu.py", "gen_mixed.py")


def main(limit_news, suffix, seeds):
    for script, seed in zip(SCRIPTS, seeds):
        environment = dict(os.environ, OCR_GEN_SEED=seed, OCR_GEN_SUFFIX=suffix, NEXUSAI_MEDIA_DATA=DATA)
        done = subprocess.run([sys.executable, f"/repo/tools/media_data/{script}"], env=environment, capture_output=True, text=True)
        print(script, "seed", seed, "->", (done.stdout.strip().splitlines() or [done.stderr[-300:]])[-1], flush=True)
        if done.returncode != 0:
            print(done.stderr[-500:])
            return 1
    if limit_news <= 0:
        return 0
    sys.path.insert(0, "/repo/tools/media_data")
    import nastaliq_lab
    out = Path(DATA) / "generated" / ("ocr_unb" + suffix)
    out.mkdir(parents=True, exist_ok=True)
    rows = []
    for n, (index, key, image, reference) in enumerate(nastaliq_lab.rows("fresh")):
        if n >= limit_news:
            break
        name = f"unb-{index}.png"
        (out / name).write_bytes(image)
        rows.append({"id": f"unb-{index}", "file": name, "text": reference, "kind": "newspaper", "style": "nastaliq", "variant": "print", "partition": "fresh", "identifiers": [], "tier": "T3"})
    with open(out / "truth.jsonl", "w", encoding="utf-8", newline="\n") as handle:
        for row in rows:
            handle.write(json.dumps(row, ensure_ascii=False) + "\n")
    print(len(rows), "newspaper rows ->", out)
    return 0


if __name__ == "__main__":
    import argparse
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--news", type=int, default=30, help="newspaper rows to export (0 = none)")
    ap.add_argument("--suffix", default="_accept")
    ap.add_argument("--seeds", nargs=3, default=["20261201", "20261202", "20261203"], help="seeds for the English, Urdu and mixed generators")
    parsed = ap.parse_args()
    sys.exit(main(parsed.news, parsed.suffix, parsed.seeds))
