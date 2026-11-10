#!/usr/bin/env python3
"""Build the plate detector's training, validation and sealed sets (docs/data/plates-m5-preregistration-20261110.md).

    docker run --rm --network none --cpus 4 -m 2g --entrypoint python -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest \
        /repo/tools/media_data/build_plate_detector_set.py --variants 6

Source: the 501 Open Images photos with plate boxes (CC BY 2.0, DAT-018, the author and landing page are in subset-test.jsonl). Split by photo id (SHA-256, last hex digit):
0-9 train, a-b validation, c-f sealed (never trained on, never tuned on). For every photo without a group or depiction box:
  * the original photo with its real plate boxes (real plates, mostly not Pakistani);
  * `--variants` copies in which the largest usable plate box (at least 48 x 12 px) is replaced by a synthetic Pakistani-format plate (registry, renderer, warp, brightness and sharpness
    matched to the local region), the other boxes kept as they are.
Output: COCO-format annotations per split under /data/generated/plates-detector/ (one class, `plate`), images as JPEG, and a manifest with the source photo id so any image can be traced.
Real photo images are never copied outside the local media-data folder.
"""
import argparse
import hashlib
import json
import random
import sys
from pathlib import Path

import cv2

sys.path.insert(0, str(Path(__file__).resolve().parent))
import plates_synth as ps  # noqa: E402

DATA = Path("/data")
OUT = DATA / "generated" / "plates-detector"


def split_of(photo_id):
    digit = hashlib.sha256(f"plate-{photo_id}".encode()).hexdigest()[-1]  # the same key the earlier scene generator used (scene id = "plate-<photo id>")
    return "train" if digit in "0123456789" else "val" if digit in "ab" else "sealed"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--variants", type=int, default=6)
    parser.add_argument("--seed", type=int, default=20261110)
    args = parser.parse_args()
    rng = random.Random(args.seed)
    records = [json.loads(line) for line in open(DATA / "openimages" / "subset-test.jsonl", encoding="utf-8")]
    sets = {name: {"images": [], "annotations": [], "categories": [{"id": 1, "name": "plate"}]} for name in ("train", "val", "sealed")}
    manifest, next_id = [], 1
    for record in records:
        if not record["plates"] or any(b["group"] or b["depiction"] for b in record["plates"]):
            continue
        photo = cv2.imread(str(DATA / record["file"]))
        if photo is None:
            continue
        height, width = photo.shape[:2]
        boxes = [(b["x0"] * width, b["y0"] * height, b["x1"] * width, b["y1"] * height) for b in record["plates"]]
        usable = [b for b in boxes if b[2] - b[0] >= 48 and b[3] - b[1] >= 12]
        split = split_of(record["id"])
        variants = [("real", photo, boxes, None)]
        if usable:
            target = max(usable, key=lambda b: b[2] - b[0])
            for index in range(args.variants):
                plate = ps.pf.random_plate(rng)
                if plate["layout"] == "two_line" and (target[2] - target[0]) / max(1.0, target[3] - target[1]) > 2.4:
                    plate["layout"] = "one_line"  # a two-line plate would be squashed into a wide, flat box
                clean = ps.render_plate(rng, plate)
                variants.append((f"synth{index}", ps.paste_plate(photo, clean, target, rng), boxes, plate["norm"]))
        for kind, image, image_boxes, text in variants:
            name = f"{record['id']}-{kind}.jpg"
            directory = OUT / split
            directory.mkdir(parents=True, exist_ok=True)
            cv2.imwrite(str(directory / name), image, [cv2.IMWRITE_JPEG_QUALITY, rng.choice([70, 85, 92])])
            sets[split]["images"].append({"id": next_id, "file_name": name, "width": width, "height": height})
            for x0, y0, x1, y1 in image_boxes:
                sets[split]["annotations"].append({"id": len(sets[split]["annotations"]) + 1, "image_id": next_id, "category_id": 1, "bbox": [round(x0, 1), round(y0, 1), round(x1 - x0, 1), round(y1 - y0, 1)],
                                                   "area": round((x1 - x0) * (y1 - y0), 1), "iscrowd": 0})
            manifest.append({"image": f"{split}/{name}", "source_photo": record["id"], "kind": kind, "split": split, "synthetic_text_sha256": hashlib.sha256(text.encode()).hexdigest()[:16] if text else None,
                             "licence": record["licence"], "author": record["author"], "landing": record["landing"]})
            next_id += 1
    for split, content in sets.items():
        (OUT / f"{split}.json").write_text(json.dumps(content), encoding="utf-8")
    (OUT / "manifest.jsonl").write_text("\n".join(json.dumps(m) for m in manifest), encoding="utf-8")
    print({s: (len(c["images"]), len(c["annotations"])) for s, c in sets.items()}, "images, boxes")


if __name__ == "__main__":
    main()
