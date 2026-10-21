#!/usr/bin/env python3
"""Generate controlled Pakistani-format plate scenes with exact truth (T1 on T3 photos): a synthetic plate is rendered and warped over the plate box of a real Open Images photo (CC BY 2.0; DAT-018),
so the background, lighting and vehicle are real and the plate text and its box are known exactly.

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest python /repo/tools/media_data/gen_plates.py

For each photo with a usable plate box (not a group, not a depiction, at least 48 pixels wide) one scene is made: a plate string in one of the Pakistani registration formats (province letter
groups, year digits, serial), rendered with a bold OFL/DejaVu face on a white, yellow or green plate with a border and a small province caption, warped to the box with a small random perspective,
matched to the local brightness, blurred to the local sharpness, and JPEG-compressed. It measures the machinery (detection, reading, confusions); it does not claim field accuracy on real plates.
Seeded; partition dev/sealed by the SHA-256 of the scene id (0 to b dev).
"""
import hashlib
import json
import os
import random
import re
from pathlib import Path

import cv2
import numpy as np
from PIL import Image, ImageDraw, ImageFont

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
OUT = Path(DATA) / "generated" / "plates"
FONTS = ["/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", "/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf", "/usr/share/fonts/truetype/dejavu/DejaVuSerif-Bold.ttf"]
SEED = 20261021
LETTER_GROUPS = {"Punjab": ["LEA", "LEB", "LEC", "LED", "LEE", "LRV", "LXA", "RIA", "FDA", "MNA", "GAA", "SAB"], "Sindh": ["AGH", "BDX", "AKB", "BNA", "KHD", "HYD"],
                 "ICT": ["ICT", "IDA"], "KPK": ["PSH", "PRB", "ABC", "KPK"], "Balochistan": ["QTA", "BLC", "KZA"]}


def make_text(rng):
    province = rng.choice(list(LETTER_GROUPS))
    letters = rng.choice(LETTER_GROUPS[province])
    style = rng.choice(["year", "plain", "plain3"])
    if style == "year":
        return province, f"{letters}-{rng.randrange(10, 26)}-{rng.randrange(1000, 9999)}"
    if style == "plain":
        return province, f"{letters}-{rng.randrange(1000, 9999)}"
    return province, f"{letters}-{rng.randrange(100, 999)}"


def render_plate(rng, province, text):
    width, height = 560, 130
    colour = rng.choices([(255, 255, 255), (250, 214, 60), (232, 240, 232)], weights=[75, 15, 10])[0]
    image = Image.new("RGB", (width, height), colour)
    draw = ImageDraw.Draw(image)
    draw.rounded_rectangle((3, 3, width - 4, height - 4), radius=14, outline=(10, 10, 10), width=4)
    font = ImageFont.truetype(rng.choice(FONTS), 74)
    caption = ImageFont.truetype(FONTS[0], 18)
    tw = draw.textlength(text, font=font)
    draw.text(((width - tw) / 2, 14), text, font=font, fill=(15, 15, 15))
    cw = draw.textlength(province.upper(), font=caption)
    draw.text(((width - cw) / 2, height - 30), province.upper(), font=caption, fill=(20, 80, 20))
    return np.array(image)[:, :, ::-1].copy()


def paste_plate(photo, plate, box, rng):
    x0, y0, x1, y1 = box
    bw, bh = x1 - x0, y1 - y0
    jitter = lambda: rng.uniform(-0.06, 0.06)
    dst = np.float32([[x0 + jitter() * bw, y0 + jitter() * bh], [x1 + jitter() * bw, y0 + jitter() * bh], [x1 + jitter() * bw, y1 + jitter() * bh], [x0 + jitter() * bw, y1 + jitter() * bh]])
    src = np.float32([[0, 0], [plate.shape[1], 0], [plate.shape[1], plate.shape[0]], [0, plate.shape[0]]])
    matrix = cv2.getPerspectiveTransform(src, dst)
    height, width = photo.shape[:2]
    warped = cv2.warpPerspective(plate, matrix, (width, height), flags=cv2.INTER_AREA, borderMode=cv2.BORDER_CONSTANT)
    mask = cv2.warpPerspective(np.full(plate.shape[:2], 255, np.uint8), matrix, (width, height), flags=cv2.INTER_NEAREST)
    region = photo[int(y0):int(y1), int(x0):int(x1)]
    gain = float(np.clip(region.mean() / max(1.0, warped[mask > 0].mean()), 0.45, 1.25)) if (mask > 0).any() and region.size else 1.0
    warped = np.clip(warped.astype(np.float32) * gain, 0, 255).astype(np.uint8)
    sharp = cv2.Laplacian(cv2.cvtColor(region, cv2.COLOR_BGR2GRAY), cv2.CV_64F).var() if region.size else 100.0
    if sharp < 400 or rng.random() < 0.5:
        sigma = 0.6 + 1.4 * (1.0 - min(1.0, sharp / 400.0))
        warped = cv2.GaussianBlur(warped, (0, 0), sigma)
    out = photo.copy()
    out[mask > 0] = warped[mask > 0]
    return out


def partition(key):
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    rng = random.Random(SEED)
    records = [json.loads(line) for line in open(Path(DATA) / "openimages" / "subset-test.jsonl", encoding="utf-8")]
    truth = []
    for record in records:
        if "plate" not in record["roles"]:
            continue
        photo = cv2.imread(str(Path(DATA) / record["file"]))
        if photo is None:
            continue
        height, width = photo.shape[:2]
        usable = [b for b in record["plates"] if not b["group"] and not b["depiction"] and (b["x1"] - b["x0"]) * width >= 48 and (b["y1"] - b["y0"]) * height >= 12]
        if not usable:
            continue
        box = max(usable, key=lambda b: (b["x1"] - b["x0"]))
        province, text = make_text(rng)
        plate = render_plate(rng, province, text)
        pixels = (box["x0"] * width, box["y0"] * height, box["x1"] * width, box["y1"] * height)
        scene = paste_plate(photo, plate, pixels, rng)
        scene_id = f"plate-{record['id']}"
        file = f"{scene_id}.jpg"
        cv2.imwrite(str(OUT / file), scene, [cv2.IMWRITE_JPEG_QUALITY, rng.choice([55, 70, 85, 92])])
        truth.append({"id": scene_id, "file": file, "plate": text, "normalised": re.sub(r"[^A-Z0-9]", "", text), "province": province,
                      "box": [round(v, 1) for v in pixels], "size": [width, height], "partition": partition(scene_id), "source_photo": record["id"], "tier": "T1"})
    with open(OUT / "truth.jsonl", "w", encoding="utf-8") as handle:
        for item in truth:
            handle.write(json.dumps(item) + "\n")
    dev = sum(1 for t in truth if t["partition"] == "dev")
    print(f"{len(truth)} scenes, dev {dev}, sealed {len(truth) - dev}")


if __name__ == "__main__":
    main()
