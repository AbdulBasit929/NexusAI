"""Synthetic Pakistani-format plates for training and testing the reader and the detector (docs/data/plates-m5-preregistration-20261110.md).

    render_plate(rng, plate)   a clean plate image from a registry plate (plate_formats.random_plate): one-line or two-line, colour class, font, border, emboss, caption, dirt
    degrade(rng, plate_bgr)    what a detector crop looks like in the field: box jitter, perspective, rotation, low resolution, blur, noise, JPEG, brightness, partial occlusion
    paste_plate(photo, plate, box, rng)   warp a plate over a plate box of a real photo, match its brightness and sharpness (the earlier generator's method)

Everything is seeded by the caller's random.Random, so a set is reproducible. The designs are approximations from public descriptions (the registry says so); this measures machinery, not field
accuracy on real plates. Runs in the evaluation image (OpenCV, NumPy, Pillow); the registry module is found through NEXUSAI_INGEST (default /repo/ingestion/forensic_records).
"""
from __future__ import annotations

import os
import random
import sys
from pathlib import Path

import cv2
import numpy as np
from PIL import Image, ImageDraw, ImageFont

sys.path.insert(0, os.environ.get("NEXUSAI_INGEST", "/repo/ingestion/forensic_records"))
import plate_formats as pf  # noqa: E402

FONT_CANDIDATES = [
    "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", "/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf", "/usr/share/fonts/truetype/dejavu/DejaVuSerif-Bold.ttf",
    "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf", "/usr/share/fonts/truetype/liberation/LiberationSansNarrow-Bold.ttf",
    "/usr/share/fonts/truetype/freefont/FreeSansBold.ttf", "/data/fonts/NotoSans.ttf",
]
FONTS = [p for p in FONT_CANDIDATES if Path(p).exists()]
CAPTIONS = {"Punjab": "PUNJAB", "Sindh": "SINDH", "ICT": "ISLAMABAD", "KPK": "KHYBER PAKHTUNKHWA", "Balochistan": "BALOCHISTAN", "GB_AJK": "AZAD KASHMIR"}


def _fit(draw, text, font_path, max_width, max_height, start):
    size = start
    while size > 10:
        font = ImageFont.truetype(font_path, size)
        left, top, right, bottom = draw.textbbox((0, 0), text, font=font)
        if right - left <= max_width and bottom - top <= max_height:
            return font, (left, top, right, bottom)
        size -= 2
    font = ImageFont.truetype(font_path, 10)
    return font, draw.textbbox((0, 0), text, font=font)


def render_plate(rng: random.Random, plate: dict) -> np.ndarray:
    registry = pf.load_registry()
    classes = registry["visual_classes"]
    visual = rng.choices(classes, weights=[c["weight"] for c in classes])[0]
    background = tuple(int(v + rng.uniform(-6, 6)) for v in visual["background"])
    ink = tuple(visual.get("text", [rng.randrange(8, 40)] * 3))
    two_line = plate["layout"] == "two_line"
    width, height = (300, 172) if two_line else (520, 112)
    image = Image.new("RGB", (width, height), tuple(max(0, min(255, v)) for v in background))
    draw = ImageDraw.Draw(image)
    border = (200, 200, 200) if visual["id"] == "dark" else (rng.randrange(0, 40),) * 3
    draw.rounded_rectangle((3, 3, width - 4, height - 4), radius=rng.choice([8, 12, 16]), outline=border, width=rng.choice([3, 4, 5]))
    font_path = rng.choice(FONTS)
    norm = plate["norm"]
    letters = 3 if plate["format"].startswith("L3") else 2
    caption = CAPTIONS[plate["region"]] if rng.random() < 0.7 else ""
    if two_line:
        lines = [norm[:letters], norm[letters:]]
    else:
        sep = rng.choice(["-", "-", " ", ""])
        if plate["format"].endswith("Y2_S4") or plate["format"].endswith("Y2_S3"):
            parts = [norm[:letters], norm[letters:letters + 2], norm[letters + 2:]]
        else:
            parts = [norm[:letters], norm[letters:]]
        lines = [sep.join(parts)]
    if plate.get("text_override"):  # an exact string to draw (a demo plate that is outside any registered format); the label is still plate["norm"]
        lines = [plate["text_override"]]
    area_top = 10 + (16 if caption and not two_line else 0)
    area_bottom = height - 12 - (0 if two_line or not caption else 14)
    slot = (area_bottom - area_top) / len(lines)
    emboss = rng.random() < 0.4
    for index, text in enumerate(lines):
        font, (left, top, right, bottom) = _fit(draw, text, font_path, int(width * rng.uniform(0.74, 0.9)), int(slot * 0.86), 140)
        x = (width - (right - left)) / 2 - left + rng.uniform(-6, 6)
        y = area_top + index * slot + (slot - (bottom - top)) / 2 - top
        if emboss:
            draw.text((x + 2, y + 2), text, font=font, fill=tuple(min(255, c + 90) for c in background))
        draw.text((x, y), text, font=font, fill=ink)
    if caption:
        small, (left, top, right, bottom) = _fit(draw, caption, FONTS[0], int(width * 0.5), 14, 16)
        draw.text(((width - (right - left)) / 2 - left, (height - 17 if not two_line else 6)), caption, font=small, fill=(20, 80, 20))
    if rng.random() < 0.5:
        for cx in (14, width - 14):
            draw.ellipse((cx - 3, height // 2 - 3, cx + 3, height // 2 + 3), fill=(120, 120, 120))
    array = np.array(image)[:, :, ::-1].copy()
    if rng.random() < 0.5:  # dirt: low-frequency dark blotches
        gen = np.random.default_rng(rng.randrange(2 ** 32))
        blotch = cv2.GaussianBlur(gen.random((height, width)).astype(np.float32), (0, 0), rng.uniform(6, 18))
        blotch = (blotch - blotch.min()) / max(1e-6, blotch.max() - blotch.min())
        array = np.clip(array.astype(np.float32) * (1 - rng.uniform(0.05, 0.3) * blotch[:, :, None]), 0, 255).astype(np.uint8)
    return array


def degrade(rng: random.Random, plate: np.ndarray, *, target_width: int | None = None) -> np.ndarray:
    """Make a clean plate look like a detector crop from a real image."""
    gen = np.random.default_rng(rng.randrange(2 ** 32))
    height, width = plate.shape[:2]
    shear = lambda: rng.uniform(-0.08, 0.08)
    src = np.float32([[0, 0], [width, 0], [width, height], [0, height]])
    dst = np.float32([[shear() * width, shear() * height], [width + shear() * width, shear() * height], [width + shear() * width, height + shear() * height], [shear() * width, height + shear() * height]])
    angle = rng.uniform(-6, 6)
    rotation = cv2.getRotationMatrix2D((width / 2, height / 2), angle, 1.0)
    warped = cv2.warpPerspective(plate, cv2.getPerspectiveTransform(src, dst), (width, height), borderMode=cv2.BORDER_REPLICATE)
    warped = cv2.warpAffine(warped, rotation, (width, height), borderMode=cv2.BORDER_REPLICATE)
    pad_x, pad_y = int(width * rng.uniform(0.0, 0.12)), int(height * rng.uniform(0.0, 0.2))  # a detector box is rarely exact
    warped = cv2.copyMakeBorder(warped, pad_y, pad_y, pad_x, pad_x, cv2.BORDER_REPLICATE)
    crop_left, crop_top = int(rng.uniform(0, 0.03) * warped.shape[1]), int(rng.uniform(0, 0.05) * warped.shape[0])
    warped = warped[crop_top:warped.shape[0] - int(rng.uniform(0, 0.05) * warped.shape[0]), crop_left:warped.shape[1] - int(rng.uniform(0, 0.03) * warped.shape[1])]
    out_width = target_width or int(np.exp(rng.uniform(np.log(45), np.log(320))))
    scale = out_width / warped.shape[1]
    out = cv2.resize(warped, (max(8, out_width), max(6, int(warped.shape[0] * scale))), interpolation=cv2.INTER_AREA)
    if rng.random() < 0.7:
        out = cv2.GaussianBlur(out, (0, 0), rng.uniform(0.3, 1.6))
    out = np.clip(out.astype(np.float32) * rng.uniform(0.45, 1.25) + rng.uniform(-25, 25), 0, 255)
    if rng.random() < 0.6:
        out = out + gen.normal(0, rng.uniform(1, 9), out.shape)
    if rng.random() < 0.25:  # night tint
        out = out * np.array([rng.uniform(0.75, 1.0), rng.uniform(0.8, 1.0), rng.uniform(0.9, 1.1)], np.float32)
    out = np.clip(out, 0, 255).astype(np.uint8)
    if rng.random() < 0.15:  # partial occlusion by a strip (a bar, a dent shadow)
        x0 = rng.randrange(0, out.shape[1] - 2)
        out[:, x0:x0 + rng.randrange(1, max(2, out.shape[1] // 14))] = rng.randrange(0, 255)
    ok, encoded = cv2.imencode(".jpg", out, [cv2.IMWRITE_JPEG_QUALITY, rng.choice([35, 50, 70, 85, 95])])
    return cv2.imdecode(encoded, cv2.IMREAD_COLOR) if ok else out


def paste_plate(photo: np.ndarray, plate: np.ndarray, box: tuple[float, float, float, float], rng: random.Random) -> np.ndarray:
    """Warp `plate` over the pixel box (x0, y0, x1, y1) of `photo`, matching local brightness and sharpness."""
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
        warped = cv2.GaussianBlur(warped, (0, 0), 0.6 + 1.4 * (1.0 - min(1.0, sharp / 400.0)))
    out = photo.copy()
    out[mask > 0] = warped[mask > 0]
    return out
