#!/usr/bin/env python3
"""Generate deterministic, high-complexity controlled virtual ANPR evidence.

T2-V is source-owned rendered evidence. It is deliberately separate from
physical T2-P and public-real T3. The generator uses no network or personal
assets and preserves exact scene parameters, polygons, crops, hashes and split
identities. Pillow is supplied by the already-approved isolated R8 evaluator.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import random
from pathlib import Path
from typing import Any

from PIL import Image, ImageDraw, ImageEnhance, ImageFilter, ImageFont


LATIN_FONT = "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"
URDU_FONT = "/usr/share/fonts/truetype/noto/NotoNaskhArabic-Bold.ttf"
FIXED_CREATED_AT = "2026-08-13T00:00:00Z"


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: Path) -> str:
    return sha256_bytes(path.read_bytes())


def load_font(path: str, size: int) -> ImageFont.FreeTypeFont:
    if not Path(path).exists():
        raise RuntimeError(f"required evaluator font is missing: {path}")
    return ImageFont.truetype(path, size)


def solve_linear(matrix: list[list[float]], values: list[float]) -> list[float]:
    augmented = [row[:] + [value] for row, value in zip(matrix, values)]
    size = len(values)
    for column in range(size):
        pivot = max(range(column, size), key=lambda row: abs(augmented[row][column]))
        if abs(augmented[pivot][column]) < 1e-12:
            raise ValueError("degenerate perspective polygon")
        augmented[column], augmented[pivot] = augmented[pivot], augmented[column]
        divisor = augmented[column][column]
        augmented[column] = [value / divisor for value in augmented[column]]
        for row in range(size):
            if row == column:
                continue
            factor = augmented[row][column]
            augmented[row] = [left - factor * right for left, right in zip(augmented[row], augmented[column])]
    return [augmented[row][-1] for row in range(size)]


def perspective_coefficients(
    destination: list[tuple[float, float]], source: list[tuple[float, float]]
) -> tuple[float, ...]:
    """Return PIL inverse-map coefficients from destination to source."""
    matrix: list[list[float]] = []
    values: list[float] = []
    for (x, y), (u, v) in zip(destination, source):
        matrix.extend([
            [x, y, 1, 0, 0, 0, -u * x, -u * y],
            [0, 0, 0, x, y, 1, -v * x, -v * y],
        ])
        values.extend([u, v])
    return tuple(solve_linear(matrix, values))


def font_for(script: str, size: int) -> ImageFont.FreeTypeFont:
    return load_font(URDU_FONT if script == "Arabic-derived Urdu" else LATIN_FONT, size)


def make_plate(text: str, script: str, style: str, width: int = 430, height: int = 132) -> Image.Image:
    palette = {
        "pakistan_oriented_green_band": ((244, 239, 213, 255), (15, 98, 58, 255)),
        "pakistan_oriented_white": ((245, 245, 238, 255), (55, 75, 92, 255)),
        "pakistan_oriented_sindh_colorway": ((245, 240, 215, 255), (168, 108, 31, 255)),
        "pakistan_oriented_kpk_colorway": ((238, 242, 231, 255), (39, 91, 122, 255)),
        "pakistan_oriented_generic": ((239, 236, 219, 255), (64, 81, 96, 255)),
    }
    background, band = palette[style]
    plate = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    draw = ImageDraw.Draw(plate)
    draw.rounded_rectangle((2, 2, width - 3, height - 3), radius=14, fill=background, outline=(15, 18, 21, 255), width=7)
    draw.rounded_rectangle((8, 8, width - 9, 41), radius=7, fill=band)
    draw.text((width // 2, 24), "NEXUSAI TEST", font=font_for("Latin", 20), anchor="mm", fill=(255, 255, 255, 255))
    selected = font_for(script, 48 if script == "Latin" else 46)
    draw.text((width // 2, 82), text, font=selected, anchor="mm", fill=(12, 18, 29, 255))
    draw.text((width // 2, 117), "SYNTHETIC", font=font_for("Latin", 13), anchor="mm", fill=(176, 32, 32, 255))
    return plate


def gradient_background(size: tuple[int, int], rng: random.Random, kind: str) -> Image.Image:
    width, height = size
    palettes = {
        "urban": ((42, 54, 66), (114, 118, 115)),
        "parking": ((52, 62, 68), (134, 132, 119)),
        "garage": ((26, 30, 36), (82, 89, 91)),
        "road": ((56, 74, 87), (97, 103, 98)),
        "toll": ((20, 27, 38), (62, 69, 73)),
        "busy": ((69, 75, 82), (141, 128, 104)),
    }
    top, bottom = palettes[kind]
    image = Image.new("RGB", size)
    draw = ImageDraw.Draw(image)
    for y in range(height):
        ratio = y / max(1, height - 1)
        color = tuple(int(top[index] * (1 - ratio) + bottom[index] * ratio) for index in range(3))
        draw.line((0, y, width, y), fill=color)
    for _ in range(70):
        x, y = rng.randrange(width), rng.randrange(height)
        shade = rng.randrange(-18, 19)
        draw.rectangle((x, y, min(width, x + rng.randrange(4, 45)), min(height, y + rng.randrange(2, 12))), fill=tuple(max(0, min(255, channel + shade)) for channel in top))
    horizon = int(height * 0.48)
    if kind in {"urban", "busy"}:
        for x in range(-20, width, 90):
            building_height = rng.randint(100, 260)
            draw.rectangle((x, horizon - building_height, x + rng.randint(65, 105), horizon), fill=(rng.randint(70, 120),) * 3)
            for wx in range(x + 10, x + 60, 22):
                for wy in range(horizon - building_height + 20, horizon - 10, 35):
                    draw.rectangle((wx, wy, wx + 10, wy + 14), fill=(165, 155, 104))
    if kind in {"road", "parking", "toll"}:
        draw.polygon(((0, horizon), (width, horizon), (width, height), (0, height)), fill=(61, 65, 66))
        for offset in range(-300, 1300, 220):
            draw.polygon(((offset, height), (offset + 16, height), (width // 2 + offset // 8 + 8, horizon), (width // 2 + offset // 8, horizon)), fill=(205, 198, 149))
    if kind == "garage":
        for x in range(60, width, 120):
            draw.rectangle((x, 70, x + 12, height), fill=(112, 115, 111))
        for y in range(75, height, 90):
            draw.line((0, y, width, y), fill=(92, 96, 95), width=6)
    return image


def draw_vehicle(image: Image.Image, rng: random.Random, variant: int) -> tuple[int, int, int, int]:
    draw = ImageDraw.Draw(image)
    x1, y1 = rng.randint(90, 180), rng.randint(205, 255)
    x2, y2 = rng.randint(780, 890), rng.randint(455, 505)
    body = [(x1, y2 - 35), (x1 + 65, y1 + 45), (x1 + 210, y1), (x2 - 150, y1 + 8), (x2 - 40, y1 + 62), (x2, y2 - 30), (x2 - 10, y2), (x1 + 10, y2)]
    colors = [(45, 82, 111), (98, 48, 45), (61, 88, 67), (89, 82, 52), (68, 68, 75)]
    color = colors[variant % len(colors)]
    draw.polygon(body, fill=color, outline=(20, 24, 27))
    draw.polygon(((x1 + 230, y1 + 12), (x2 - 175, y1 + 20), (x2 - 90, y1 + 72), (x1 + 155, y1 + 70)), fill=(42, 58, 68), outline=(145, 159, 163))
    draw.rounded_rectangle((x1 + 135, y2 - 98, x2 - 110, y2 - 25), radius=12, fill=(25, 30, 32), outline=(125, 132, 131), width=3)
    for y in range(y2 - 88, y2 - 28, 14):
        draw.line((x1 + 150, y, x2 - 125, y), fill=(75, 82, 83), width=4)
    draw.ellipse((x1 + 70, y2 - 35, x1 + 185, y2 + 65), fill=(18, 19, 20), outline=(130, 132, 131), width=7)
    draw.ellipse((x2 - 205, y2 - 35, x2 - 90, y2 + 65), fill=(18, 19, 20), outline=(130, 132, 131), width=7)
    draw.ellipse((x1 + 35, y2 - 105, x1 + 105, y2 - 52), fill=(226, 203, 117), outline=(240, 235, 190))
    draw.ellipse((x2 - 95, y2 - 105, x2 - 25, y2 - 52), fill=(226, 203, 117), outline=(240, 235, 190))
    return x1, y1, x2, y2


def plate_quad(rng: random.Random, profile: str, width: int, height: int) -> list[tuple[int, int]]:
    profiles = {
        "center": (300, 325, 660, 435),
        "small_or_far": (430, 302, 585, 352),
        "edge_position": (22, 325, 335, 425),
        "large": (205, 286, 760, 460),
    }
    x1, y1, x2, y2 = profiles.get(profile, profiles["center"])
    jitter_x, jitter_y = rng.randint(-18, 18), rng.randint(-12, 12)
    x1, x2, y1, y2 = x1 + jitter_x, x2 + jitter_x, y1 + jitter_y, y2 + jitter_y
    if profile == "horizontal_perspective":
        return [(x1 + 45, y1), (x2, y1 + 25), (x2 - 5, y2 - 12), (x1, y2)]
    if profile == "vertical_perspective":
        return [(x1 + 30, y1 + 15), (x2 - 30, y1), (x2, y2), (x1, y2 - 12)]
    return [(x1, y1), (x2, y1), (x2, y2), (x1, y2)]


def composite_plate(image: Image.Image, plate: Image.Image, quad: list[tuple[int, int]]) -> None:
    source = [(0.0, 0.0), (float(plate.width - 1), 0.0), (float(plate.width - 1), float(plate.height - 1)), (0.0, float(plate.height - 1))]
    destination = [(float(x), float(y)) for x, y in quad]
    coefficients = perspective_coefficients(destination, source)
    warped = plate.transform(image.size, Image.Transform.PERSPECTIVE, coefficients, resample=Image.Resampling.BICUBIC)
    image.paste(warped.convert("RGB"), (0, 0), warped.getchannel("A"))


def motion_blur(image: Image.Image, displacement: int = 6) -> Image.Image:
    accumulator = Image.new("RGB", image.size, (0, 0, 0))
    for step in range(displacement):
        shifted = Image.new("RGB", image.size, (0, 0, 0))
        shifted.paste(image, (step, 0))
        accumulator = Image.blend(accumulator, shifted, 1.0 / (step + 1))
    return accumulator


def sensor_noise(image: Image.Image, rng: random.Random, strength: int) -> Image.Image:
    if strength <= 0:
        return image
    noise_size = (max(1, image.width // 8), max(1, image.height // 8))
    noise = Image.new("RGB", noise_size)
    pixels = noise.load()
    for y in range(noise.height):
        for x in range(noise.width):
            value = rng.randint(128 - strength, 128 + strength)
            pixels[x, y] = (value, value, value)
    noise = noise.resize(image.size, Image.Resampling.BILINEAR)
    return Image.blend(image, noise, min(0.16, strength / 180.0))


def synthetic_token(partition: str, index: int, script: str) -> str:
    prefix = {"development": "D", "validation": "V", "sealed_holdout": "H"}[partition]
    if script == "Arabic-derived Urdu":
        words = ["آزمائش", "محفوظ", "نمونہ", "تجربہ"]
        offset = {"development": 100, "validation": 400, "sealed_holdout": 700}[partition]
        return f"{words[index % len(words)]} {index + offset:03d}"
    cities = ["NXA", "NXB", "NXC", "NXD", "NXE"]
    return f"{prefix}{cities[index % len(cities)]}-{index + 101:03d}"


def bounds_for_polygon(polygon: list[tuple[int, int]]) -> dict[str, int]:
    xs, ys = [point[0] for point in polygon], [point[1] for point in polygon]
    x1, y1, x2, y2 = min(xs), min(ys), max(xs), max(ys)
    return {"x": x1, "y": y1, "width": x2 - x1, "height": y2 - y1}


def write_png(image: Image.Image, path: Path) -> None:
    image.save(path, format="PNG", optimize=False, compress_level=6)


def positive_scene(target: Path, contract: dict[str, Any], partition: str, index: int, global_index: int) -> dict[str, Any]:
    identity = f"{partition}:positive:{index}"
    seed = int.from_bytes(hashlib.sha256(f"{contract['master_seed']}:{identity}".encode()).digest()[:8], "big")
    rng = random.Random(seed)
    conditions = contract["required_positive_conditions"]
    condition = conditions[index % len(conditions)]
    background_kind = ["urban", "parking", "garage", "road", "toll", "busy"][index % 6]
    image = gradient_background((contract["image"]["width"], contract["image"]["height"]), rng, background_kind)
    vehicle_bounds = draw_vehicle(image, rng, index)
    script = "Arabic-derived Urdu" if index % 7 == 5 else "Latin"
    style = contract["plate_styles"][index % len(contract["plate_styles"])]
    text = synthetic_token(partition, index, script)
    profile = condition if condition in {"small_or_far", "edge_position", "horizontal_perspective", "vertical_perspective"} else ("large" if index % 11 == 8 else "center")
    polygons = [plate_quad(rng, profile, image.width, image.height)]
    texts = [text]
    scripts = [script]
    styles = [style]
    composite_plate(image, make_plate(text, script, style), polygons[0])
    if condition == "multiple_plates":
        second_text = synthetic_token(partition, index + 70, "Latin")
        second_quad = [(615, 225), (845, 238), (838, 310), (608, 302)]
        composite_plate(image, make_plate(second_text, "Latin", contract["plate_styles"][(index + 1) % len(contract["plate_styles"])]), second_quad)
        polygons.append(second_quad); texts.append(second_text); scripts.append("Latin"); styles.append(contract["plate_styles"][(index + 1) % len(contract["plate_styles"])])
    draw = ImageDraw.Draw(image, "RGBA")
    if condition == "partial_occlusion":
        bounds = bounds_for_polygon(polygons[0]); x = bounds["x"] + int(bounds["width"] * 0.58)
        draw.rectangle((x, bounds["y"] - 3, x + max(18, bounds["width"] // 8), bounds["y"] + bounds["height"] + 4), fill=(54, 57, 55, 255))
    elif condition == "glare":
        bounds = bounds_for_polygon(polygons[0]); draw.ellipse((bounds["x"] + 25, bounds["y"] - 35, bounds["x"] + bounds["width"] - 25, bounds["y"] + bounds["height"] + 35), fill=(255, 255, 245, 105))
    elif condition == "shadow":
        draw.polygon(((0, 0), (image.width * 2 // 3, 0), (image.width // 2, image.height), (0, image.height)), fill=(0, 0, 0, 105))
    elif condition == "night_like":
        image = ImageEnhance.Brightness(image).enhance(0.34)
    elif condition == "overexposure":
        image = ImageEnhance.Brightness(image).enhance(1.65)
    elif condition == "underexposure":
        image = ImageEnhance.Brightness(image).enhance(0.23)
    elif condition == "motion_blur":
        image = motion_blur(image)
    elif condition == "defocus":
        image = image.filter(ImageFilter.GaussianBlur(2.4))
    elif condition == "dirty_low_contrast":
        image = ImageEnhance.Contrast(image).enhance(0.48)
        draw = ImageDraw.Draw(image, "RGBA")
        for _ in range(18):
            x, y = rng.randint(0, image.width), rng.randint(0, image.height)
            draw.ellipse((x, y, x + rng.randint(8, 35), y + rng.randint(4, 20)), fill=(67, 54, 39, rng.randint(25, 75)))
    if condition in {"compression", "busy_background"}:
        buffer = target / f".compression-{global_index}.jpg"
        image.save(buffer, format="JPEG", quality=32 if condition == "compression" else 58, optimize=False)
        image = Image.open(buffer).convert("RGB"); buffer.unlink()
    image = sensor_noise(image, rng, 15 if condition in {"night_like", "underexposure"} else 5)
    fixture_id = f"t2v-{global_index:03d}-{partition}-positive"
    image_path = target / "images" / f"{fixture_id}.png"
    write_png(image, image_path)
    regions = []
    crop_file = None
    for plate_index, (polygon, value, plate_script, plate_style) in enumerate(zip(polygons, texts, scripts, styles)):
        bounds = bounds_for_polygon(polygon)
        regions.append({
            "polygon": [{"x": x, "y": y} for x, y in polygon],
            "bounds": bounds,
            "plate_text_raw": value,
            "script": plate_script,
            "style": plate_style,
            "source_plate_polygon": [{"x": 0, "y": 0}, {"x": 429, "y": 0}, {"x": 429, "y": 131}, {"x": 0, "y": 131}],
        })
        if len(polygons) == 1:
            margin = 8
            crop_box = (max(0, bounds["x"] - margin), max(0, bounds["y"] - margin), min(image.width, bounds["x"] + bounds["width"] + margin), min(image.height, bounds["y"] + bounds["height"] + margin))
            crop_path = target / "crops" / f"{fixture_id}-crop.png"
            write_png(image.crop(crop_box), crop_path)
            crop_file = f"crops/{crop_path.name}"
    return {
        "fixture_id": fixture_id,
        "tier": "T2-V",
        "evaluation_partition": partition,
        "random_seed": seed,
        "source_file": f"images/{image_path.name}",
        "source_sha256": sha256_file(image_path),
        "width_pixels": image.width,
        "height_pixels": image.height,
        "scene_parameters": {
            "background": background_kind,
            "vehicle_bounds": vehicle_bounds,
            "condition": condition,
            "compression_round_trip": condition in {"compression", "busy_background"},
            "sensor_noise_strength": 15 if condition in {"night_like", "underexposure"} else 5,
        },
        "fixture_classes": [condition, "hard_positive", "pakistan_oriented_synthetic_style"],
        "plate_regions_original_pixels": regions,
        "truth_crop_file": crop_file,
        "truth_crop_sha256": sha256_file(target / crop_file) if crop_file else None,
        "plate_text_raw_when_visible": text if len(polygons) == 1 else None,
        "script_when_visible": script if len(polygons) == 1 else None,
        "expected_abstention_state": "candidate",
        "detector_evaluation_eligible": True,
        "ocr_evaluation_eligible": len(polygons) == 1,
        "safe_for_model_input": True,
        "source_assets": [{"id": "procedural_scene", "license": "NexusAI repository-owned generated asset"}],
    }


def negative_scene(target: Path, contract: dict[str, Any], partition: str, index: int, global_index: int) -> dict[str, Any]:
    identity = f"{partition}:negative:{index}"
    seed = int.from_bytes(hashlib.sha256(f"{contract['master_seed']}:{identity}".encode()).digest()[:8], "big")
    rng = random.Random(seed)
    negative_class = contract["required_negative_classes"][index % len(contract["required_negative_classes"])]
    background_kind = ["urban", "parking", "garage", "road", "toll", "busy"][index % 6]
    image = gradient_background((contract["image"]["width"], contract["image"]["height"]), rng, background_kind)
    draw = ImageDraw.Draw(image)
    if negative_class in {"plain_vehicle", "vehicle_grille", "vehicle_logo", "rectangular_reflector"}:
        vehicle = draw_vehicle(image, rng, index)
        if negative_class == "vehicle_logo":
            draw.ellipse((435, 345, 520, 430), fill=(30, 86, 147), outline=(220, 225, 228), width=8)
            draw.text((478, 388), "NX", font=font_for("Latin", 25), anchor="mm", fill=(255, 255, 255))
        elif negative_class == "rectangular_reflector":
            draw.rectangle((390, 385, 570, 425), fill=(214, 75, 56), outline=(238, 219, 176), width=5)
    elif negative_class in {"street_signage", "road_sign", "billboard", "shop_board", "house_number_like_text"}:
        draw.rectangle((210, 95, 755, 335), fill=(225, 224, 205), outline=(22, 27, 31), width=9)
        wording = {"street_signage": "TEST STREET", "road_sign": "EXIT 12", "billboard": "NEXUS MARKET", "shop_board": "OPEN 24", "house_number_like_text": "HOUSE 407"}[negative_class]
        draw.text((482, 215), wording, font=font_for("Latin", 55), anchor="mm", fill=(22, 29, 39))
    elif negative_class in {"document", "screen"}:
        draw.rectangle((235, 55, 725, 485), fill=(235, 237, 231) if negative_class == "document" else (24, 35, 52), outline=(12, 15, 18), width=9)
        for y in range(110, 435, 48):
            draw.line((285, y, 675, y), fill=(67, 77, 89) if negative_class == "document" else (103, 169, 223), width=6)
    elif negative_class in {"multiple_rectangles", "architectural_structure"}:
        for row in range(3):
            for column in range(5):
                x, y = 80 + column * 170, 70 + row * 145
                draw.rectangle((x, y, x + 120, y + 75), outline=(190, 192, 184), width=8)
    elif negative_class == "random_text":
        for _ in range(14):
            draw.text((rng.randint(20, 820), rng.randint(20, 490)), rng.choice(["TEST", "42", "NX", "SAFE"]), font=font_for("Latin", rng.randint(20, 42)), fill=(rng.randint(130, 235),) * 3)
    fixture_id = f"t2v-{global_index:03d}-{partition}-negative"
    image_path = target / "images" / f"{fixture_id}.png"
    write_png(sensor_noise(image, rng, 4), image_path)
    return {
        "fixture_id": fixture_id,
        "tier": "T2-V",
        "evaluation_partition": partition,
        "random_seed": seed,
        "source_file": f"images/{image_path.name}",
        "source_sha256": sha256_file(image_path),
        "width_pixels": image.width,
        "height_pixels": image.height,
        "scene_parameters": {"background": background_kind, "negative_class": negative_class},
        "fixture_classes": ["hard_negative", negative_class],
        "plate_regions_original_pixels": [],
        "truth_crop_file": None,
        "truth_crop_sha256": None,
        "plate_text_raw_when_visible": None,
        "script_when_visible": None,
        "expected_abstention_state": "no_plate_found",
        "detector_evaluation_eligible": True,
        "ocr_evaluation_eligible": False,
        "safe_for_model_input": True,
        "source_assets": [{"id": "procedural_scene", "license": "NexusAI repository-owned generated asset"}],
    }


def generate_pack(output: Path, contract_path: Path) -> dict[str, Any]:
    contract_bytes = contract_path.read_bytes()
    contract = json.loads(contract_bytes.decode("utf-8-sig"))
    if contract.get("contract_version") != "forensics.controlled-virtual-anpr-pack/v1":
        raise ValueError("unexpected T2-V contract")
    (output / "images").mkdir(parents=True, exist_ok=True)
    (output / "crops").mkdir(parents=True, exist_ok=True)
    fixtures: list[dict[str, Any]] = []
    global_index = 1
    for partition, counts in contract["partitions"].items():
        for index in range(counts["positive"]):
            fixtures.append(positive_scene(output, contract, partition, index, global_index)); global_index += 1
        for index in range(counts["negative"]):
            fixtures.append(negative_scene(output, contract, partition, index, global_index)); global_index += 1
    manifest = {
        "contract_version": contract["contract_version"],
        "dataset_id": contract["dataset_id"],
        "version": contract["version"],
        "generator": contract["generator_version"],
        "generator_source": "tools/r8_t2v/generate_t2v.py",
        "generator_source_sha256": sha256_file(Path(__file__)),
        "contract_sha256": sha256_bytes(contract_bytes),
        "created_at": FIXED_CREATED_AT,
        "master_seed": contract["master_seed"],
        "privacy_policy": "generated_non_personal_repository_owned_only",
        "evidence_authority": "controlled_virtual_not_physical_not_public_real_not_operational",
        "coordinate_space": "original_image_pixels",
        "partition_counts": {
            partition: sum(item["evaluation_partition"] == partition for item in fixtures)
            for partition in contract["partitions"]
        },
        "fixture_count": len(fixtures),
        "fixtures": fixtures,
    }
    manifest_path = output / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True, ensure_ascii=False) + "\n", encoding="utf-8")
    seal = {
        "contract_version": "forensics.controlled-virtual-anpr-pack-seal/v1",
        "manifest_sha256": sha256_file(manifest_path),
        "sealed_holdout_fixture_ids": [item["fixture_id"] for item in fixtures if item["evaluation_partition"] == "sealed_holdout"],
        "sealed_holdout_source_hashes": [item["source_sha256"] for item in fixtures if item["evaluation_partition"] == "sealed_holdout"],
        "tuning_allowed": False,
        "production_authority": False,
    }
    (output / "seal.json").write_text(json.dumps(seal, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return {"manifest": manifest, "seal": seal}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--contract", type=Path, required=True)
    args = parser.parse_args()
    result = generate_pack(args.output.resolve(), args.contract.resolve())
    print(json.dumps({
        "status": "pass",
        "dataset_id": result["manifest"]["dataset_id"],
        "fixture_count": result["manifest"]["fixture_count"],
        "partition_counts": result["manifest"]["partition_counts"],
        "manifest_sha256": result["seal"]["manifest_sha256"],
        "production_mutation": False,
    }, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
