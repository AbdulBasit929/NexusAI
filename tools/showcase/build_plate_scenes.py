#!/usr/bin/env python3
"""Build the vehicle photos added to the Orion reference case (docs/data/plates-m5-preregistration-20261110.md, demonstration).

    docker run --rm --network none --entrypoint python -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro -v <Kaggle plates folder>:/real:ro -v <out dir>:/out \
        nexusai-media-eval:latest /repo/tools/showcase/build_plate_scenes.py --plates SYNT2905,SYNT9150,SYNT4O24 --untouched 2

Real photographs of cars from the public Kaggle set (not in the owner-labelled gold set). For each plate given, one photo where the plate detector finds exactly one plate of readable size is taken
and that plate is REPLACED by the Orion plate (the string is drawn exactly as given, so a plate outside any registered format, or with a letter where a digit would be, is shown as it is); `--untouched` more
photos are copied as they are. Writes the JPEG files and `plate-images-key.json` (file, constructed or untouched, source photo, the plate written for the constructed ones). The constructed ones are
constructed for the demonstration: the case notes say so.
"""
import argparse
import csv
import json
import os
import random
import sys

import cv2

sys.path.insert(0, "/repo/tools/media_data")
sys.path.insert(0, "/repo/ingestion/forensic_records")
import plate_detector_onnx as pd  # noqa: E402
import plates_synth as ps  # noqa: E402


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--plates", required=True)
    parser.add_argument("--untouched", type=int, default=2)
    parser.add_argument("--out", default="/out")
    parser.add_argument("--skip", type=int, default=10, help="start this far into the list of photos that are not in the gold set")
    args = parser.parse_args()
    gold = {r["relative_file"] for r in csv.DictReader(open("/gold/image-labels.csv", encoding="utf-8-sig"))}
    photos = [f"Cars/{n}" for n in sorted(os.listdir("/real/Cars")) if f"Cars/{n}" not in gold and n.lower().endswith(".jpg")]
    det = pd.OnnxPlateDetector("/data/models/plates/rfdetr-alpr/rfdetr_alpr.onnx", cv2, normalise="unit", threshold=0.5, drop_non_plates=True)
    rng = random.Random(20261110)
    plates = [p.strip() for p in args.plates.split(",") if p.strip()]
    os.makedirs(args.out, exist_ok=True)
    key, used = [], 0
    for rel in photos[args.skip:]:
        if used >= len(plates) + args.untouched:
            break
        frame = cv2.imread("/real/" + rel)
        found = det.detect(frame)
        h, w = frame.shape[:2]
        if len(found) != 1 or found[0][4] < 0.85 or (found[0][2] - found[0][0]) < 90:
            continue
        name = f"roadside-vehicle-{used + 1:02d}.jpg"
        if used < len(plates):
            x1, y1, x2, y2, _ = found[0]
            plate_image = ps.render_plate(rng, {"region": "Punjab", "format": "ORION", "layout": "one_line", "display": plates[used], "norm": plates[used], "text_override": plates[used]})
            scene = ps.paste_plate(frame, plate_image, (x1, y1, x2, y2), rng)
            cv2.imwrite(os.path.join(args.out, name), scene, [cv2.IMWRITE_JPEG_QUALITY, 92])
            key.append({"file": name, "kind": "constructed", "plate_written": plates[used], "source_photo": rel, "plate_box": [x1, y1, x2, y2]})
        else:
            cv2.imwrite(os.path.join(args.out, name), frame, [cv2.IMWRITE_JPEG_QUALITY, 92])
            key.append({"file": name, "kind": "untouched real photo", "plate_written": None, "source_photo": rel})
        used += 1
    json.dump({"note": "Real photographs of cars from a public dataset. 'constructed' = the plate was replaced by the Orion plate named here; 'untouched' = as photographed.", "images": key}, open(os.path.join(args.out, "plate-images-key.json"), "w", encoding="utf-8"), indent=1)
    print(used, "images written:", [(k["file"], k["kind"]) for k in key])


if __name__ == "__main__":
    main()
