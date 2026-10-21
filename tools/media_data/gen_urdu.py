#!/usr/bin/env python3
"""Generate Urdu OCR test material at realistic sizes with exact truth (T1): single lines (Naskh and Nastaliq) and mixed-script pages with Latin identifiers.

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest python /repo/tools/media_data/gen_urdu.py

Why: the public Urdu OCR sets fetched for this programme (PuristanLabs urdu-ocr-1M) hold training crops: the Naskh images are 64x512 windows that cut long lines (the truth text is longer than
what is visible) and the Nastaliq images are 64 pixels high, far below the size of a scanned page line. They cannot score an engine fairly, so they are used here only as a source of Urdu
sentences (their text column). Lines are rendered with the OFL fonts Noto Nastaliq Urdu and Noto Naskh Arabic through Pillow's raqm layout (shaping and right-to-left), at 34 to 46 pixels, with
degradations (grain, blur, JPEG, 1 degree skew). Pages are A4 at 150 dpi: eight to twelve Urdu lines plus a header and a short table of Latin digits (phone numbers, CNIC pattern, amounts).
Seeded; dev or sealed by the SHA-256 of the id (0 to b dev).
"""
import hashlib
import io
import json
import os
import random
import re
from pathlib import Path

import pyarrow.parquet as pq
from PIL import Image, ImageDraw, ImageFilter, ImageFont

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
OUT = Path(DATA) / "generated" / "ocr_ur"
FONTS = {"nastaliq": Path(DATA) / "fonts" / "NotoNastaliqUrdu.ttf", "naskh": Path(DATA) / "fonts" / "NotoNaskhArabic.ttf"}
SEED = 20261021


def partition(key):
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def sentences():
    """Distinct Urdu sentences of 12 to 70 characters, from the text column of the Nastaliq validation split."""
    seen, out = set(), []
    table = pq.read_table(Path(DATA) / "urdu_ocr" / "nastaliq-val.parquet", columns=["text"])
    for value in table.column("text").to_pylist():
        value = re.sub(r"\s+", " ", value).strip()
        if 12 <= len(value) <= 70 and value not in seen and re.search(r"[\u0600-\u06ff]", value):
            seen.add(value)
            out.append(value)
    return out


def degrade(image, rng, kind):
    if kind == "clean":
        return image
    out = image
    if kind in ("noisy", "lowres_jpeg"):
        out = out.filter(ImageFilter.GaussianBlur(0.7))
        pixels = out.load()
        for _ in range(int(out.width * out.height * 0.02)):
            x, y = rng.randrange(out.width), rng.randrange(out.height)
            pixels[x, y] = max(0, min(255, pixels[x, y] + rng.randrange(-60, 60)))
    if kind == "skewed":
        out = out.rotate(1.0, resample=Image.BICUBIC, fillcolor=255)
    if kind == "lowres_jpeg":
        out = out.resize((int(out.width * 0.7), int(out.height * 0.7)), Image.LANCZOS)
        buffer = io.BytesIO()
        out.save(buffer, format="JPEG", quality=45)
        out = Image.open(io.BytesIO(buffer.getvalue())).convert("L")
    return out


def render_line(text, style, size, rng):
    font = ImageFont.truetype(str(FONTS[style]), size, layout_engine=ImageFont.Layout.RAQM)
    box = font.getbbox(text, direction="rtl", language="ur")
    width, height = box[2] - box[0] + 2 * size, box[3] - box[1] + int(size * 1.1)
    image = Image.new("L", (width, height), 255)
    draw = ImageDraw.Draw(image)
    draw.text((size, int(size * 0.5) - box[1]), text, font=font, fill=0, direction="rtl", language="ur")
    return image


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    rng = random.Random(SEED)
    texts = sentences()
    rng.shuffle(texts)
    print(f"{len(texts)} usable sentences")
    truth = []
    # single lines: 200 per style, one degradation each, cycling
    kinds = ["clean", "noisy", "skewed", "lowres_jpeg"]
    for style in ("naskh", "nastaliq"):
        for n in range(200):
            text = texts[(n + (0 if style == "naskh" else 200)) % len(texts)]
            size = rng.choice([34, 38, 42, 46])
            kind = kinds[n % 4]
            image = degrade(render_line(text, style, size, rng), rng, kind)
            line_id = f"urline-{style}-{n:03d}-{kind}"
            file = f"{line_id}.png"
            image.save(OUT / file)
            truth.append({"id": line_id, "file": file, "text": text, "kind": "line", "style": style, "variant": kind, "partition": partition(line_id), "tier": "T1"})
    # pages: Urdu body with a Latin-digit table
    for n in range(24):
        style = "naskh" if n % 2 == 0 else "nastaliq"
        width, height = 1240, 1754
        page = Image.new("L", (width, height), 255)
        draw = ImageDraw.Draw(page)
        size = 34 if style == "naskh" else 40
        font = ImageFont.truetype(str(FONTS[style]), size, layout_engine=ImageFont.Layout.RAQM)
        lines_out, y = [], 110
        for k in range(rng.randrange(8, 12)):
            text = texts[(n * 13 + k * 7) % len(texts)][:46]
            box = font.getbbox(text, direction="rtl", language="ur")
            draw.text((width - 110 - (box[2] - box[0]), y - box[1]), text, font=font, fill=0, direction="rtl", language="ur")
            lines_out.append(text)
            y += int(size * 2.3)
        latin = ImageFont.truetype(str(Path(DATA) / "fonts" / "NotoSans.ttf"), 26)
        ids = []
        y += 40
        for _ in range(5):
            phone = f"03{rng.randrange(0, 5)}{rng.randrange(10)}-{rng.randrange(10 ** 7):07d}"
            cnic = f"{rng.randrange(1, 8)}{rng.randrange(10 ** 4):04d}-{rng.randrange(10 ** 7):07d}-{rng.randrange(1, 10)}"
            amount = f"Rs. {rng.randrange(1, 900)},{rng.randrange(1000):03d}"
            draw.text((110, y), f"{phone}    {cnic}    {amount}", font=latin, fill=0)
            lines_out.append(f"{phone} {cnic} {amount}")  # the table is part of the page text; the Urdu lines come first, as on the page
            ids += [{"type": "phone", "value": phone}, {"type": "cnic", "value": cnic}, {"type": "amount", "value": amount}]
            y += 46
        for kind in kinds:
            page_id = f"urpage-{style}-{n:03d}"
            image = degrade(page, rng, kind)
            file = f"{page_id}-{kind}.png"
            image.save(OUT / file)
            truth.append({"id": f"{page_id}-{kind}", "file": file, "text": "\n".join(lines_out), "identifiers": ids, "kind": "page", "style": style, "variant": kind,
                          "partition": partition(page_id), "tier": "T1"})
    with open(OUT / "truth.jsonl", "w", encoding="utf-8") as handle:
        for item in truth:
            handle.write(json.dumps(item, ensure_ascii=False) + "\n")
    print(f"{len(truth)} items: lines {sum(1 for t in truth if t['kind'] == 'line')}, pages {sum(1 for t in truth if t['kind'] == 'page')}, dev {sum(1 for t in truth if t['partition'] == 'dev')}")


if __name__ == "__main__":
    main()
