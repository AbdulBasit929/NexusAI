#!/usr/bin/env python3
"""Fail-closed validator for NexusAI controlled virtual T2-V packs."""

from __future__ import annotations

import argparse
import hashlib
import json
import struct
from pathlib import Path, PurePosixPath
from typing import Any


CONTRACT = "forensics.controlled-virtual-anpr-pack/v1"
SEAL_CONTRACT = "forensics.controlled-virtual-anpr-pack-seal/v1"


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def safe_child(root: Path, relative: str) -> Path | None:
    if not isinstance(relative, str) or not relative:
        return None
    pure = PurePosixPath(relative)
    if pure.is_absolute() or ".." in pure.parts or "\\" in relative:
        return None
    candidate = (root / Path(*pure.parts)).resolve()
    try:
        candidate.relative_to(root.resolve())
    except ValueError:
        return None
    return candidate


def png_dimensions(path: Path) -> tuple[int, int] | None:
    header = path.read_bytes()[:24]
    if len(header) != 24 or header[:8] != b"\x89PNG\r\n\x1a\n" or header[12:16] != b"IHDR":
        return None
    return struct.unpack(">II", header[16:24])


def polygon_area(points: list[dict[str, int]]) -> float:
    return abs(sum(
        points[index]["x"] * points[(index + 1) % len(points)]["y"]
        - points[(index + 1) % len(points)]["x"] * points[index]["y"]
        for index in range(len(points))
    )) / 2


def validate(pack_root: Path, contract_path: Path) -> dict[str, Any]:
    errors: list[str] = []
    manifest_path, seal_path = pack_root / "manifest.json", pack_root / "seal.json"
    if not manifest_path.is_file() or not seal_path.is_file():
        return {"status": "fail", "errors": ["manifest.json and seal.json are required"]}
    manifest = json.loads(manifest_path.read_text(encoding="utf-8-sig"))
    seal = json.loads(seal_path.read_text(encoding="utf-8-sig"))
    contract = json.loads(contract_path.read_text(encoding="utf-8-sig"))
    if manifest.get("contract_version") != CONTRACT or contract.get("contract_version") != CONTRACT:
        errors.append("unexpected manifest or source contract version")
    if seal.get("contract_version") != SEAL_CONTRACT:
        errors.append("unexpected seal contract version")
    if seal.get("manifest_sha256") != sha256(manifest_path):
        errors.append("manifest seal hash mismatch")
    if seal.get("tuning_allowed") is not False or seal.get("production_authority") is not False:
        errors.append("sealed holdout or production authority is fail-open")
    fixtures = manifest.get("fixtures", [])
    if manifest.get("fixture_count") != len(fixtures):
        errors.append("fixture_count does not match fixtures")
    expected_counts = {name: values["positive"] + values["negative"] for name, values in contract["partitions"].items()}
    actual_counts = {name: sum(item.get("evaluation_partition") == name for item in fixtures) for name in expected_counts}
    if actual_counts != expected_counts or manifest.get("partition_counts") != expected_counts:
        errors.append(f"partition counts mismatch: expected={expected_counts} actual={actual_counts}")
    ids: set[str] = set(); seeds: set[int] = set(); source_hashes: set[str] = set()
    texts: dict[str, set[str]] = {name: set() for name in expected_counts}
    present_conditions: set[str] = set(); present_negatives: set[str] = set()
    referenced_files: set[Path] = {manifest_path.resolve(), seal_path.resolve()}
    for index, fixture in enumerate(fixtures):
        prefix = f"fixtures[{index}]"
        fixture_id = fixture.get("fixture_id")
        if not fixture_id or fixture_id in ids:
            errors.append(f"{prefix} fixture_id empty or duplicated")
        ids.add(fixture_id)
        seed = fixture.get("random_seed")
        if not isinstance(seed, int) or seed in seeds:
            errors.append(f"{prefix} random_seed missing or reused")
        seeds.add(seed)
        partition = fixture.get("evaluation_partition")
        if partition not in expected_counts:
            errors.append(f"{prefix} invalid partition")
        source = safe_child(pack_root, fixture.get("source_file", ""))
        if source is None or not source.is_file():
            errors.append(f"{prefix} source path missing or unsafe")
            continue
        referenced_files.add(source.resolve())
        dimensions = png_dimensions(source)
        if dimensions != (fixture.get("width_pixels"), fixture.get("height_pixels")):
            errors.append(f"{prefix} PNG signature/dimensions mismatch")
        actual_hash = sha256(source)
        if fixture.get("source_sha256") != actual_hash:
            errors.append(f"{prefix} source hash mismatch")
        if actual_hash in source_hashes:
            errors.append(f"{prefix} byte-duplicate source image")
        source_hashes.add(actual_hash)
        regions = fixture.get("plate_regions_original_pixels", [])
        if "hard_positive" in fixture.get("fixture_classes", []):
            if not regions:
                errors.append(f"{prefix} positive has no truth polygon")
            present_conditions.update(set(fixture.get("fixture_classes", [])) & set(contract["required_positive_conditions"]))
        if "hard_negative" in fixture.get("fixture_classes", []):
            if regions:
                errors.append(f"{prefix} negative contains a plate truth region")
            present_negatives.update(set(fixture.get("fixture_classes", [])) & set(contract["required_negative_classes"]))
        for region in regions:
            polygon = region.get("polygon", [])
            if len(polygon) != 4 or polygon_area(polygon) <= 1:
                errors.append(f"{prefix} plate polygon is missing or degenerate")
                continue
            if dimensions is None or any(point["x"] < 0 or point["x"] >= dimensions[0] or point["y"] < 0 or point["y"] >= dimensions[1] for point in polygon):
                errors.append(f"{prefix} plate polygon is outside original pixels")
            text = region.get("plate_text_raw")
            if text and partition in texts:
                texts[partition].add(text)
            if not str(region.get("style", "")).startswith("pakistan_oriented_"):
                errors.append(f"{prefix} positive style is not governed Pakistan-oriented synthetic")
        crop_name = fixture.get("truth_crop_file")
        if crop_name:
            crop = safe_child(pack_root, crop_name)
            if crop is None or not crop.is_file() or png_dimensions(crop) is None:
                errors.append(f"{prefix} truth crop missing, unsafe or not PNG")
            else:
                referenced_files.add(crop.resolve())
                if fixture.get("truth_crop_sha256") != sha256(crop):
                    errors.append(f"{prefix} truth crop hash mismatch")
        assets = fixture.get("source_assets", [])
        if not assets or any(asset.get("license") != "NexusAI repository-owned generated asset" for asset in assets):
            errors.append(f"{prefix} source asset license is missing or external")
    if present_conditions != set(contract["required_positive_conditions"]):
        errors.append(f"positive condition coverage incomplete: missing={sorted(set(contract['required_positive_conditions']) - present_conditions)}")
    if present_negatives != set(contract["required_negative_classes"]):
        errors.append(f"negative coverage incomplete: missing={sorted(set(contract['required_negative_classes']) - present_negatives)}")
    partition_names = list(texts)
    for left_index, left in enumerate(partition_names):
        for right in partition_names[left_index + 1:]:
            overlap = texts[left] & texts[right]
            if overlap:
                errors.append(f"plate identity leakage between {left} and {right}: {sorted(overlap)}")
    holdout_ids = [item["fixture_id"] for item in fixtures if item.get("evaluation_partition") == "sealed_holdout"]
    holdout_hashes = [item["source_sha256"] for item in fixtures if item.get("evaluation_partition") == "sealed_holdout"]
    if seal.get("sealed_holdout_fixture_ids") != holdout_ids or seal.get("sealed_holdout_source_hashes") != holdout_hashes:
        errors.append("sealed holdout IDs/hashes do not match manifest")
    actual_files = {path.resolve() for folder in (pack_root / "images", pack_root / "crops") if folder.is_dir() for path in folder.iterdir() if path.is_file()}
    unexpected = sorted(str(path.relative_to(pack_root.resolve())) for path in actual_files - referenced_files)
    if unexpected:
        errors.append(f"unexpected unmanifested assets: {unexpected}")
    return {
        "status": "pass" if not errors else "fail",
        "errors": errors,
        "fixture_count": len(fixtures),
        "partition_counts": actual_counts,
        "manifest_sha256": sha256(manifest_path),
        "holdout_sealed": not errors and len(holdout_ids) == expected_counts["sealed_holdout"],
        "tuning_allowed_on_holdout": False,
        "production_authority": False,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--pack-root", type=Path, required=True)
    parser.add_argument("--contract", type=Path, default=Path("configuration/nexusai_r8_t2v_contract.json"))
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    result = validate(args.pack_root.resolve(), args.contract.resolve())
    rendered = json.dumps(result, indent=2, sort_keys=True) + "\n"
    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    print(rendered, end="")
    return 0 if result["status"] == "pass" else 1


if __name__ == "__main__":
    raise SystemExit(main())
