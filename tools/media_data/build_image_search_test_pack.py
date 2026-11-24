#!/usr/bin/env python3
"""Builds the image-search test pack for the Lahore reference case from the photographs downloaded for the 2026-11-20 acceptance (COCO 2014 val, Karpathy 5K test; CC-licensed Flickr photographs, evaluation use).

    docker run --rm --network none --memory 900m -v C:/tmp/nexusai-media-data:/data:ro -v <repo>:/repo:ro -v C:/NexusAI-Showcase/orion/image-search-test-pack:/out --entrypoint python \
        nexusai-media-eval-face:20261119 /repo/tools/media_data/build_image_search_test_pack.py

Output (under /out):
  in-case/        what goes INTO the case: varied photographs (vehicles, streets, shops, objects, animals ...) plus, for six of them, deliberately made copies (re-saved, cropped) so that "find copies" has something to find.
  reference/      what the analyst DROPS into the question box: other copies of the same six pictures (mirrored, framed, toned, re-compressed, a phone screenshot of one), an exact copy under another name, and pictures that
                  are NOT in the case. None of these is uploaded to the case.
  manifest.json   every file with its purpose and the result the search is expected to give.
Every in-case file name starts with imgsearch- so the additions can always be told from the original Lahore evidence.
"""
import hashlib
import io
import json
import shutil
import sys
from pathlib import Path

sys.path.insert(0, "/repo/tools/media_data")
from PIL import Image  # noqa: E402

import image_search_m1 as m  # noqa: E402

DATA = Path("/data/work-faces/m1")
OUT = Path("/out")
CATEGORIES = [  # (label, caption words that must appear, how many)
    ("motorcycle", ("motorcycle",), 4), ("truck", ("truck",), 3), ("bus", ("bus",), 3), ("car-street", ("car", "street"), 3), ("bicycle", ("bicycle",), 2), ("train", ("train",), 2),
    ("market", ("market",), 3), ("street-people", ("people", "street"), 3), ("shop", ("store",), 2), ("sign", ("sign",), 2),
    ("laptop", ("laptop",), 2), ("phone", ("cell phone",), 2), ("backpack", ("backpack",), 2), ("suitcase", ("suitcase",), 2),
    ("dog", ("dog",), 2), ("cat", ("cat",), 1), ("food", ("pizza",), 2), ("kitchen", ("kitchen",), 1), ("clock", ("clock",), 1), ("boat", ("boat",), 2), ("airplane", ("airplane",), 2),
    ("building", ("tower",), 2),
]
FAMILIES = ["motorcycle", "truck", "bus", "laptop", "market", "backpack"]   # the six pictures that get deliberate copies


def pick(manifest):
    pool = sorted(manifest["sem1000"] + manifest["d1000"][:0], key=lambda x: hashlib.sha256(("pack" + str(x["id"])).encode()).hexdigest())
    chosen, used = [], set()
    for label, words, count in CATEGORIES:
        found = 0
        for item in pool:
            if item["id"] in used:
                continue
            text = " ".join(item.get("captions", [])).lower()
            if all(w in text for w in words):
                with Image.open(DATA / item["file"]) as im:
                    if min(im.size) < 300:
                        continue
                chosen.append({"label": label, "id": item["id"], "file": item["file"], "caption": item["captions"][0]})
                used.add(item["id"])
                found += 1
                if found == count:
                    break
    return chosen


def jpeg(img, path, quality=90):
    img.convert("RGB").save(path, format="JPEG", quality=quality)


def main():
    manifest = json.load(open(DATA / "manifest.json"))
    chosen = pick(manifest)
    for sub in ("in-case", "reference"):
        (OUT / sub).mkdir(parents=True, exist_ok=True)
    files = []
    bases = {}
    for item in chosen:
        name = f"imgsearch-{item['label']}-{item['id']}.jpg"
        shutil.copyfile(DATA / item["file"], OUT / "in-case" / name)
        files.append({"file": f"in-case/{name}", "purpose": "a different photograph in the case", "caption": item["caption"], "coco_id": item["id"]})
        if item["label"] in FAMILIES and item["label"] not in bases:
            bases[item["label"]] = (item, name)
    # deliberate copies: two go INTO the case, the rest are reference files to drop in
    for label, (item, base_name) in bases.items():
        image = Image.open(DATA / item["file"]).convert("RGB")
        variants = m.make_transforms(image)
        stem = f"imgsearch-{label}-{item['id']}"
        jpeg(Image.open(io.BytesIO(variants["wa_like"][0])), OUT / "in-case" / f"{stem}-resaved.jpg", 80)
        jpeg(Image.open(io.BytesIO(variants["crop85"][0])), OUT / "in-case" / f"{stem}-cropped.jpg", 90)
        files.append({"file": f"in-case/{stem}-resaved.jpg", "purpose": f"a messenger-style re-save of {base_name} (in the case)", "base": base_name})
        files.append({"file": f"in-case/{stem}-cropped.jpg", "purpose": f"a lightly cropped copy (85 percent) of {base_name} (in the case)", "base": base_name})
        refs = {
            "mirror": (variants["mirror"][0], "mirrored"),
            "framed": (variants["social_frame"][0], "in a white frame with a caption strip, shared via chat (the registered weak spot: often NOT found)"),
            "toned": (variants["tone"][0], "brightness and contrast changed"),
        }
        for key, (data, note) in refs.items():
            jpeg(Image.open(io.BytesIO(data)), OUT / "reference" / f"ref-{label}-{key}.jpg", 90)
            files.append({"file": f"reference/ref-{label}-{key}.jpg", "purpose": f"reference: {base_name} {note}", "base": base_name})
        jpeg(Image.open(io.BytesIO(variants["screenshot_ui"][0])), OUT / "reference" / f"ref-{label}-phone-screenshot.jpg", 85)
        files.append({"file": f"reference/ref-{label}-phone-screenshot.jpg", "purpose": f"reference: a phone screenshot with the interface around {base_name} (the stated limit: usually NOT found; crop the picture out of the screenshot first)", "base": base_name})
        # the same bytes as the in-case original, under another name: an exact copy
        shutil.copyfile(OUT / "in-case" / base_name, OUT / "reference" / f"ref-{label}-exact-copy-renamed.jpg")
        files.append({"file": f"reference/ref-{label}-exact-copy-renamed.jpg", "purpose": f"reference: byte-for-byte the same file as {base_name}, under another name (expected: identical file)", "base": base_name})
    # pictures that are NOT in the case
    used = {c["id"] for c in chosen}
    extra = [x for x in sorted(manifest["sem1000"], key=lambda x: hashlib.sha256(("neg" + str(x["id"])).encode()).hexdigest()) if x["id"] not in used][:6]
    for n, item in enumerate(extra, start=1):
        shutil.copyfile(DATA / item["file"], OUT / "reference" / f"ref-not-in-case-{n}.jpg")
        files.append({"file": f"reference/ref-not-in-case-{n}.jpg", "purpose": "reference: a photograph that is NOT in the case (expected: no copy found)", "caption": item["captions"][0]})
    json.dump({"source": "COCO 2014 val (Karpathy 5K test): Flickr photographs under Creative Commons licences, evaluation use only", "files": files}, open(OUT / "manifest.json", "w"), indent=1)
    print(f"{len(chosen)} varied photographs, {sum(1 for f in files if f['file'].startswith('in-case/'))} in-case files, {sum(1 for f in files if f['file'].startswith('reference/'))} reference files")


if __name__ == "__main__":
    main()
