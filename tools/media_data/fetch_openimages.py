#!/usr/bin/env python3
"""Select and fetch a deterministic subset of Open Images V7 photos (images CC BY 2.0, annotations CC BY 4.0; ledger DAT-018 and DAT-019).

    python -X utf8 tools/media_data/fetch_openimages.py            # plates 500, faces 500, general 300 from the test split
    python -X utf8 tools/media_data/fetch_openimages.py --plates 150 --faces 150 --general 100 --split validation

Needs the annotation and image tables in C:/tmp/nexusai-media-data/openimages/ (fetched earlier). Selection is by SHA-256 order of the image id, so it is reproducible. For each photo the
record keeps the author, the licence URL, the landing page and the boxes of the class it was chosen for (normalised coordinates) so a detector can be scored; the photo files are never
redistributed. Standard library only; 8 download threads.
"""
import argparse
import concurrent.futures
import csv
import hashlib
import json
import os
import time
import urllib.request

ROOT = os.environ.get("NEXUSAI_MEDIA_DATA", "C:/tmp/nexusai-media-data")
OI = os.path.join(ROOT, "openimages")
PLATE, FACE = "/m/01jfm_", "/m/0dzct"
HOST = "https://open-images-dataset.s3.amazonaws.com"


def order_key(image_id, salt):
    return hashlib.sha256(f"{salt}:{image_id}".encode()).hexdigest()


def read_boxes(split):
    boxes = {PLATE: {}, FACE: {}}
    with open(os.path.join(OI, f"{split}-annotations-bbox.csv"), newline="", encoding="utf-8") as handle:
        for row in csv.DictReader(handle):
            label = row["LabelName"]
            if label in boxes:
                boxes[label].setdefault(row["ImageID"], []).append({
                    "x0": float(row["XMin"]), "x1": float(row["XMax"]), "y0": float(row["YMin"]), "y1": float(row["YMax"]),
                    "group": row["IsGroupOf"] == "1", "depiction": row["IsDepiction"] == "1", "occluded": row["IsOccluded"] == "1", "truncated": row["IsTruncated"] == "1"})
    return boxes


def read_images(split):
    path = os.path.join(OI, f"{split}-images-with-rotation.csv")
    out = {}
    with open(path, newline="", encoding="utf-8") as handle:
        for row in csv.DictReader(handle):
            out[row["ImageID"]] = row
    return out


def download(split, image_id, dest):
    if os.path.exists(dest) and os.path.getsize(dest) > 1000:
        return
    request = urllib.request.Request(f"{HOST}/{split}/{image_id}.jpg", headers={"User-Agent": "nexusai-media-fetch/1"})
    for attempt in range(3):
        try:
            with urllib.request.urlopen(request, timeout=40) as response, open(dest + ".part", "wb") as out:
                out.write(response.read())
            os.replace(dest + ".part", dest)
            return
        except Exception:
            time.sleep(1 + attempt)
    raise RuntimeError(f"could not fetch {image_id}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--split", default="test", choices=("test", "validation"))
    parser.add_argument("--plates", type=int, default=500)
    parser.add_argument("--faces", type=int, default=500)
    parser.add_argument("--general", type=int, default=300)
    args = parser.parse_args()
    boxes = read_boxes(args.split)
    images = read_images(args.split)
    chosen = {}
    for role, label, count in (("plate", PLATE, args.plates), ("face", FACE, args.faces)):
        ids = sorted((i for i in boxes[label] if i in images), key=lambda i: order_key(i, role))
        for image_id in ids[:count]:
            chosen.setdefault(image_id, {"roles": []})["roles"].append(role)
    busy = set(boxes[PLATE]) | set(boxes[FACE])
    general = sorted((i for i in images if i not in busy and i not in chosen), key=lambda i: order_key(i, "general"))
    for image_id in general[:args.general]:
        chosen.setdefault(image_id, {"roles": []})["roles"].append("general")
    folder = os.path.join(OI, "photos", args.split)
    os.makedirs(folder, exist_ok=True)

    def work(image_id):
        dest = os.path.join(folder, f"{image_id}.jpg")
        download(args.split, image_id, dest)
        return image_id, dest

    manifest_path = os.path.join(ROOT, "manifest.jsonl")
    records = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
        for n, (image_id, dest) in enumerate(pool.map(work, sorted(chosen)), start=1):
            row = images[image_id]
            data = open(dest, "rb").read()
            records.append({"id": image_id, "split": args.split, "file": os.path.relpath(dest, ROOT).replace(os.sep, "/"), "roles": chosen[image_id]["roles"],
                            "sha256": hashlib.sha256(data).hexdigest(), "bytes": len(data), "licence": row.get("License", ""), "author": row.get("Author", ""),
                            "landing": row.get("OriginalLandingURL", ""), "plates": boxes[PLATE].get(image_id, []), "faces": boxes[FACE].get(image_id, [])})
            if n % 100 == 0:
                print(f"  {n}/{len(chosen)}", flush=True)
    with open(os.path.join(OI, f"subset-{args.split}.jsonl"), "w", encoding="utf-8") as handle:
        for record in records:
            handle.write(json.dumps(record) + "\n")
    with open(manifest_path, "a", encoding="utf-8") as handle:
        handle.write(json.dumps({"rel": f"openimages/subset-{args.split}.jsonl", "url": "storage.googleapis.com/openimages + open-images-dataset.s3.amazonaws.com", "bytes": sum(r["bytes"] for r in records),
                                 "sha256": hashlib.sha256("".join(r["sha256"] for r in records).encode()).hexdigest(), "licence": "images CC BY 2.0, annotations CC BY 4.0", "ledger": "DAT-018;DAT-019",
                                 "note": f"{len(records)} photos ({args.plates} plate, {args.faces} face, {args.general} general), records carry author and licence per photo", "fetched": time.strftime("%Y-%m-%dT%H:%M:%S")}) + "\n")
    licences = {}
    for r in records:
        licences[r["licence"]] = licences.get(r["licence"], 0) + 1
    print(f"{len(records)} photos, {sum(r['bytes'] for r in records) / 1e6:.0f} MB, licences {licences}")


if __name__ == "__main__":
    main()
