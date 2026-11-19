#!/usr/bin/env python3
"""Which generated mixed lines run off the canvas? (erratum of 2026-11-19 to the OCR bake-off v2 and the acceptance results.)

gen_mixed.py draws a "mixline" (one Urdu sentence with English words and, in 70% of rows, a phone number at its logical end) right to left on a FIXED 1500 x 130 canvas, starting 40 px from the
right edge, and never checks that the sentence fits. The phone number is the last run of the sentence, so it is the leftmost thing on the image and is the first to be cut off. This script measures every
mixline row of the given generated sets with the generator's own Canvas.measure and writes the ids whose left edge falls below x = 0 (the text is cut) and by how many pixels.

    docker run --rm --network none --memory 800m -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-easyocr:20261117 \
        /repo/tools/media_data/clipped_mixlines.py --out /data/results/ocr/clipped-mixlines-20261119.json

Nothing is read by an OCR reader; the rows are the generated truth only.
"""
import argparse
import json
import os
import sys
from pathlib import Path

sys.path.insert(0, "/repo/tools/media_data")
DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
SETS = ("ocr_mixed", "ocr_mixed_accept", "ocr_mixed_accept2")
SIZE, RIGHT_MARGIN, WIDTH = 40, 40, 1500   # the generator's font size, right margin and mixline canvas width


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", required=True)
    args = ap.parse_args(argv)
    import gen_mixed
    result = {}
    for name in SETS:
        path = Path(DATA) / "generated" / name / "truth.jsonl"
        if not path.exists():
            continue
        rows = [json.loads(line) for line in open(path, encoding="utf-8")]
        canvases = {}
        clipped = {}
        total = 0
        for row in rows:
            if row["kind"] != "mixline":
                continue
            total += 1
            canvas = canvases.setdefault(row["style"], gen_mixed.Canvas(WIDTH, 130, row["style"]))
            left_edge = (WIDTH - RIGHT_MARGIN) - canvas.measure(row["text"], SIZE)
            if left_edge < 0:
                clipped[row["id"]] = {"left_edge_px": round(left_edge), "partition": row["partition"], "has_phone": any(i["type"] == "phone" for i in row["identifiers"])}
        result[name] = {"mixline_images": total, "clipped_images": len(clipped), "clipped_with_phone": sum(1 for v in clipped.values() if v["has_phone"]), "clipped": clipped}
        print(f"{name}: {total} mixline images, {len(clipped)} run off the left edge ({sum(1 for v in clipped.values() if v['has_phone'])} of them carry a phone number)")
    json.dump(result, open(args.out, "w", encoding="utf-8"), indent=1)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
