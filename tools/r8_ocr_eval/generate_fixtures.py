#!/usr/bin/env python3
"""Generate deterministic, non-personal R8 T0/T1 ANPR evidence.

The pack deliberately separates safe model inputs from hostile admission-only
fixtures. Hostile files are never passed to detector/OCR code.
"""

from __future__ import annotations

import hashlib
import argparse
import json
import random
import struct
import zlib
from pathlib import Path

from PIL import Image, ImageDraw, ImageEnhance, ImageFilter, ImageFont


WIDTH, HEIGHT = 960, 540
SEED = "nexusai-r8-t0-t1-20260812"
GENERATOR_VERSION = "nexusai-r8-synthetic-v2"
LATIN_FONT = "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"
URDU_FONT = "/usr/share/fonts/truetype/noto/NotoNaskhArabic-Bold.ttf"


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def font(urdu: bool, size: int) -> ImageFont.FreeTypeFont:
    path = URDU_FONT if urdu and Path(URDU_FONT).exists() else LATIN_FONT
    return ImageFont.truetype(path, size)


def background(size: tuple[int, int], identity: str) -> Image.Image:
    random.seed(f"{SEED}:{identity}")
    image = Image.new("RGB", size, (38, 43, 50))
    draw = ImageDraw.Draw(image)
    for _ in range(32):
        x, y = random.randrange(size[0]), random.randrange(size[1])
        shade = random.randrange(45, 105)
        draw.rectangle(
            (x, y, min(size[0], x + random.randrange(16, 180)), min(size[1], y + random.randrange(4, 26))),
            fill=(shade, shade, shade),
        )
    return image


def draw_plate(image: Image.Image, box: tuple[int, int, int, int], text: str, *, urdu: bool = False) -> None:
    draw = ImageDraw.Draw(image)
    width, height = box[2] - box[0], box[3] - box[1]
    draw.rounded_rectangle(box, radius=max(3, height // 12), fill=(245, 241, 218), outline=(15, 15, 15), width=max(2, height // 18))
    draw.rectangle((box[0] + width // 24, box[1] + height // 9, box[2] - width // 24, box[1] + height // 3), fill=(26, 104, 65))
    plate_font = font(urdu, max(16, int(height * (0.40 if urdu else 0.49))))
    text_box = draw.textbbox((0, 0), text, font=plate_font)
    text_width = text_box[2] - text_box[0]
    text_x = box[0] + max(3, (width - text_width) / 2)
    draw.text((text_x, box[1] + int(height * 0.39)), text, fill=(12, 12, 12), font=plate_font)


def fixture_record(
    fixture_id: str,
    source: Path,
    size: tuple[int, int],
    boxes: list[tuple[int, int, int, int]],
    *,
    fixture_classes: list[str],
    tier: str,
    text: str | None = None,
    script: str | None = None,
    crop: Path | None = None,
    detector_eligible: bool = True,
    ocr_eligible: bool = False,
    expected_state: str = "candidate",
    conditions: list[str] | None = None,
    expected_admission_state: str = "accepted_metadata_only",
) -> dict:
    return {
        "fixture_id": fixture_id,
        "tier": tier,
        "fixture_classes": fixture_classes,
        "source_file": source.name,
        "source_sha256": sha256(source),
        "width_pixels": size[0],
        "height_pixels": size[1],
        "plate_regions_original_pixels": [
            {"bounds": {"x": x1, "y": y1, "width": x2 - x1, "height": y2 - y1}}
            for x1, y1, x2, y2 in boxes
        ],
        "truth_crop_file": crop.name if crop else None,
        "truth_crop_sha256": sha256(crop) if crop else None,
        "plate_text_raw_when_visible": text,
        "script_when_visible": script,
        "expected_abstention_state": expected_state,
        "adverse_conditions": conditions or [],
        "expected_admission_state": expected_admission_state,
        "detector_evaluation_eligible": detector_eligible,
        "ocr_evaluation_eligible": ocr_eligible,
        "safe_for_model_input": detector_eligible or ocr_eligible,
        "annotation_actor_and_timestamp": "synthetic-generator@2026-08-12T00:00:00Z",
    }


def plate_scene(target: Path, fixture_id: str, text: str, condition: str = "clear", *, urdu: bool = False, box: tuple[int, int, int, int] = (250, 205, 710, 345), size: tuple[int, int] = (WIDTH, HEIGHT)) -> dict:
    image = background(size, fixture_id)
    draw_plate(image, box, text, urdu=urdu)
    transformed_box = box
    if condition in {"blur", "motion_blur"}:
        image = image.filter(ImageFilter.GaussianBlur(2.0 if condition == "blur" else 3.2))
    elif condition == "glare":
        overlay = Image.new("RGBA", image.size, (0, 0, 0, 0))
        ImageDraw.Draw(overlay).ellipse((box[0] + 90, box[1] - 25, box[2] - 70, box[3] + 30), fill=(255, 255, 255, 112))
        image = Image.alpha_composite(image.convert("RGBA"), overlay).convert("RGB")
    elif condition in {"night", "underexposure"}:
        image = ImageEnhance.Brightness(image).enhance(0.38 if condition == "night" else 0.22)
    elif condition == "overexposure":
        image = ImageEnhance.Brightness(image).enhance(1.85)
    elif condition in {"skew", "perspective"}:
        shear = 0.18 if condition == "skew" else 0.28
        image = image.transform(image.size, Image.Transform.AFFINE, (1, shear, -55, 0, 1, 0), resample=Image.Resampling.BICUBIC, fillcolor=(25, 28, 34))
        transformed_box = (max(0, box[0] - 18), box[1], min(size[0], box[2] + 25), box[3])
    elif condition == "occlusion":
        ImageDraw.Draw(image).rectangle((box[2] - 165, box[1] + 50, box[2] - 100, box[3]), fill=(55, 55, 55))
    output = target / f"{fixture_id}.{'jpg' if condition == 'compression' else 'png'}"
    if condition == "compression":
        image.save(output, format="JPEG", quality=28, optimize=False)
    else:
        image.save(output, format="PNG", optimize=False)
    crop_path = target / f"{fixture_id}_truth_crop.png"
    image.crop(transformed_box).save(crop_path, format="PNG", optimize=False)
    classes = ["regional_style"] if urdu else []
    if condition == "clear" and fixture_id == "latin_clear":
        classes.append("clear_single_plate")
    if condition in {"blur", "night", "glare", "skew", "occlusion"}:
        classes.append(condition)
    return fixture_record(
        fixture_id, output, size, [transformed_box], fixture_classes=classes,
        tier="T1", text=text, script="Arabic-derived Urdu" if urdu else "Latin",
        crop=crop_path, detector_eligible=True, ocr_eligible=True,
        conditions=[] if condition == "clear" else [condition],
    )


def multiple_plate_scene(target: Path) -> dict:
    fixture_id = "multiple_vehicles"
    image = background((1280, 720), fixture_id)
    boxes = [(85, 180, 445, 300), (760, 405, 1180, 535)]
    draw_plate(image, boxes[0], "LHR-241")
    draw_plate(image, boxes[1], "ISB-902")
    output = target / f"{fixture_id}.png"
    image.save(output, format="PNG")
    return fixture_record(fixture_id, output, image.size, boxes, fixture_classes=["multiple_vehicles"], tier="T1", detector_eligible=True, ocr_eligible=False, conditions=["multiple_vehicles"])


def pre_cropped_scene(target: Path) -> dict:
    fixture_id = "pre_cropped_plate"
    image = Image.new("RGB", (520, 160), (245, 241, 218))
    box = (2, 2, 518, 158)
    draw_plate(image, box, "KHI-782")
    output = target / f"{fixture_id}.png"
    image.save(output, format="PNG")
    crop = target / f"{fixture_id}_truth_crop.png"
    image.save(crop, format="PNG")
    return fixture_record(fixture_id, output, image.size, [box], fixture_classes=["pre_cropped_plate"], tier="T0", text="KHI-782", script="Latin", crop=crop, detector_eligible=True, ocr_eligible=True, conditions=["pre_cropped_plate"])


def ocr_abstention_crop(target: Path) -> dict:
    fixture_id = "ocr_blank_plate_like_crop"
    image = Image.new("RGB", (520, 160), (239, 237, 220))
    draw = ImageDraw.Draw(image)
    draw.rounded_rectangle((2, 2, 518, 158), radius=10, fill=(245, 241, 218), outline=(15, 15, 15), width=7)
    draw.rectangle((25, 20, 495, 55), fill=(26, 104, 65))
    output = target / f"{fixture_id}.png"
    image.save(output, format="PNG")
    crop = target / f"{fixture_id}_truth_crop.png"
    image.save(crop, format="PNG")
    return fixture_record(
        fixture_id, output, image.size, [], fixture_classes=[], tier="T1",
        text=None, script=None, crop=crop, detector_eligible=False,
        ocr_eligible=True, expected_state="ocr_abstained",
        conditions=["negative_ocr_plate_like_blank"],
    )


def negative_scene(target: Path, fixture_id: str, kind: str) -> dict:
    random.seed(f"{SEED}:{fixture_id}")
    image = background((960, 540), fixture_id)
    draw = ImageDraw.Draw(image)
    if kind == "vehicle_no_plate":
        draw.rounded_rectangle((170, 170, 790, 430), radius=45, fill=(58, 75, 88), outline=(145, 155, 165), width=7)
        draw.ellipse((220, 380, 350, 510), fill=(20, 20, 20)); draw.ellipse((610, 380, 740, 510), fill=(20, 20, 20))
    elif kind == "signboard":
        draw.rectangle((260, 140, 700, 350), fill=(236, 235, 210), outline=(20, 20, 20), width=8)
        draw.text((315, 205), "EXIT 12", fill=(20, 20, 20), font=font(False, 64))
    elif kind == "rectangles":
        for _ in range(12):
            x, y = random.randrange(40, 780), random.randrange(40, 420)
            draw.rectangle((x, y, x + random.randrange(60, 170), y + random.randrange(25, 85)), outline=(210, 210, 205), width=5)
    elif kind == "document":
        draw.rectangle((170, 45, 790, 500), fill=(245, 245, 240))
        for y in range(90, 460, 42):
            draw.line((220, y, 735, y), fill=(60, 60, 60), width=4)
    elif kind == "logo":
        draw.ellipse((300, 90, 660, 450), fill=(35, 95, 165), outline=(235, 235, 235), width=12)
        draw.text((375, 230), "NEXUS", fill=(255, 255, 255), font=font(False, 48))
    elif kind == "noise":
        pixels = image.load()
        for y in range(image.height):
            for x in range(image.width):
                value = random.randrange(256)
                pixels[x, y] = (value, random.randrange(256), random.randrange(256))
    output = target / f"{fixture_id}.png"
    image.save(output, format="PNG")
    return fixture_record(fixture_id, output, image.size, [], fixture_classes=[], tier="T1", detector_eligible=True, ocr_eligible=False, expected_state="no_plate_found", conditions=[kind])


def png_chunk(kind: bytes, payload: bytes) -> bytes:
    return struct.pack(">I", len(payload)) + kind + payload + struct.pack(">I", zlib.crc32(kind + payload) & 0xFFFFFFFF)


def hostile_fixtures(target: Path) -> list[dict]:
    fixtures = []
    malformed = target / "malformed_image.png"
    malformed.write_bytes(b"\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR\x00")
    fixtures.append(fixture_record("malformed_image", malformed, (0, 0), [], fixture_classes=["malformed_image"], tier="T0", detector_eligible=False, ocr_eligible=False, expected_state="intake_rejected", conditions=["truncated_header"], expected_admission_state="manual_review_required"))

    oversized = target / "oversized_dimensions.png"
    oversized.write_bytes(b"\x89PNG\r\n\x1a\n" + png_chunk(b"IHDR", struct.pack(">IIBBBBB", 40000, 40000, 8, 2, 0, 0, 0)) + png_chunk(b"IEND", b""))
    fixtures.append(fixture_record("oversized_dimensions", oversized, (40000, 40000), [], fixture_classes=["oversized_dimensions"], tier="T0", detector_eligible=False, ocr_eligible=False, expected_state="intake_rejected", conditions=["oversized_dimensions"], expected_admission_state="manual_review_required"))

    base = Image.new("RGB", (16, 16), (12, 60, 120))
    mismatch = target / "extension_signature_mismatch.jpg"
    base.save(mismatch, format="PNG")
    fixtures.append(fixture_record("extension_signature_mismatch", mismatch, base.size, [], fixture_classes=["extension_signature_mismatch"], tier="T0", detector_eligible=False, ocr_eligible=False, expected_state="intake_rejected", conditions=["extension_signature_mismatch"], expected_admission_state="manual_review_required"))

    clean = target / "polyglot_base.png"
    base.save(clean, format="PNG")
    polyglot = target / "polyglot_or_trailing_payload.png"
    polyglot.write_bytes(clean.read_bytes() + b"PK\x05\x06" + (b"\x00" * 18) + b"NEXUSAI_BENIGN_TRAILING_TEST")
    clean.unlink()
    fixtures.append(fixture_record("polyglot_or_trailing_payload", polyglot, base.size, [], fixture_classes=["polyglot_or_trailing_payload"], tier="T0", detector_eligible=False, ocr_eligible=False, expected_state="intake_rejected", conditions=["benign_trailing_payload"], expected_admission_state="manual_review_required"))
    return fixtures


def generate_pack(target: Path) -> dict:
    target.mkdir(parents=True, exist_ok=True)
    fixtures: list[dict] = []
    for index, condition in enumerate(("clear", "blur", "glare", "night", "skew", "occlusion"), 1):
        fixtures.append(plate_scene(target, f"latin_{condition}", f"ICT-{index:03d}", condition))
    fixtures.append(plate_scene(target, "urdu_clear", "اسلام آباد ۷۸۶", "clear", urdu=True))
    fixtures.extend([
        plate_scene(target, "latin_perspective", "RWP-107", "perspective", box=(130, 260, 620, 390)),
        plate_scene(target, "latin_small", "LHR-312", "clear", box=(705, 70, 905, 135)),
        plate_scene(target, "latin_large", "KHI-555", "clear", box=(65, 135, 895, 390)),
        plate_scene(target, "latin_near_border", "ISB-221", "clear", box=(5, 355, 430, 495)),
        plate_scene(target, "latin_motion_blur", "MUX-490", "motion_blur", box=(310, 185, 730, 315)),
        plate_scene(target, "latin_overexposure", "PEW-117", "overexposure", box=(205, 210, 705, 350)),
        plate_scene(target, "latin_underexposure", "QTA-884", "underexposure", box=(260, 195, 700, 335)),
        plate_scene(target, "latin_compression", "FSD-619", "compression", box=(230, 205, 735, 350)),
    ])
    fixtures.append(multiple_plate_scene(target))
    fixtures.append(pre_cropped_scene(target))
    fixtures.append(ocr_abstention_crop(target))

    empty = Image.new("RGB", (WIDTH, HEIGHT), (41, 46, 55))
    ImageDraw.Draw(empty).rectangle((100, 100, 860, 440), outline=(90, 98, 108), width=4)
    empty_path = target / "empty_scene.png"
    empty.save(empty_path, format="PNG")
    fixtures.append(fixture_record("empty_scene", empty_path, empty.size, [], fixture_classes=["empty_scene"], tier="T0", detector_eligible=True, ocr_eligible=False, expected_state="no_plate_found", conditions=["empty_scene"]))
    for fixture_id, kind in (
        ("negative_vehicle_no_plate", "vehicle_no_plate"),
        ("negative_signboard", "signboard"),
        ("negative_rectangular_objects", "rectangles"),
        ("negative_document", "document"),
        ("negative_logo", "logo"),
        ("negative_high_noise", "noise"),
    ):
        fixtures.append(negative_scene(target, fixture_id, kind))
    fixtures.extend(hostile_fixtures(target))

    required_classes = [
        "clear_single_plate", "regional_style", "night", "glare", "blur", "skew", "occlusion",
        "multiple_vehicles", "pre_cropped_plate", "empty_scene", "malformed_image", "oversized_dimensions",
        "extension_signature_mismatch", "polyglot_or_trailing_payload",
    ]
    manifest = {
        "contract_version": "forensics.image-fixture-taxonomy/v2",
        "privacy_policy": "generated_non_personal_synthetic_only",
        "coordinate_space": "original_image_pixels",
        "generator": GENERATOR_VERSION,
        "seed": SEED,
        "required_classes": required_classes,
        "fixture_count": len(fixtures),
        "tier_counts": {"T0": sum(item["tier"] == "T0" for item in fixtures), "T1": sum(item["tier"] == "T1" for item in fixtures)},
        "fixtures": fixtures,
        "acceptance_state": "t0_t1_complete_source_generated_t2_t3_t4_separate",
    }
    (target / "manifest.json").write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return manifest


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=Path("/work/fixtures"))
    args = parser.parse_args()
    generate_pack(args.output)


if __name__ == "__main__":
    main()
