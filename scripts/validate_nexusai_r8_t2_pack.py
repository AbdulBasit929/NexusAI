#!/usr/bin/env python3
"""Validate a controlled R8 T2 pack without modifying or decoding evidence."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path
from typing import Any


CONTRACT = "forensics.controlled-demo-pack/v1"
HASH_PATTERN = re.compile(r"^[0-9a-f]{64}$")
IMAGE_FIELDS = {
    "image_id", "original_filename", "sha256", "capture_source_class",
    "width", "height", "orientation", "plate_present", "plate_count",
    "plates", "plate_style", "lighting", "blur_level", "occlusion",
    "perspective", "notes", "consent_ownership_status", "demo_eligible",
    "privacy_screen", "asset_class", "ground_truth_source",
    "camera_orientation", "distance", "target_position", "compression",
    "negative_class", "ground_truth_review", "evaluation_partition",
}
PRIVACY_FIELDS = {
    "review_completed", "unrelated_faces", "unrelated_real_plates",
    "house_numbers", "addresses", "documents", "screen_content",
    "gps_exif", "sensitive_location_context", "reviewer", "reviewed_at",
}
GROUND_TRUTH_REVIEW_FIELDS = {
    "annotator", "annotated_at", "reviewer", "reviewed_at",
    "reviewer_independent", "model_output_consulted", "disagreements_resolved",
}
ALLOWED_CAPTURE_SOURCE_CLASSES = {
    "controlled_owned_vehicle", "controlled_consented_vehicle",
    "printed_synthetic_target", "controlled_mock_plate_board",
    "controlled_plate_crop", "controlled_negative_scene",
}
ALLOWED_NEGATIVE_CLASSES = {
    None, "vehicle_body", "logo", "street_sign", "rectangular_label",
    "document", "screen", "random_text", "vehicle_without_target",
    "background_structure", "plain_scene",
}
ALLOWED_MAGIC = {
    ".png": (b"\x89PNG\r\n\x1a\n",),
    ".jpg": (b"\xff\xd8\xff",),
    ".jpeg": (b"\xff\xd8\xff",),
}


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def safe_relative_path(value: str) -> bool:
    path = Path(value)
    return bool(value) and not path.is_absolute() and ".." not in path.parts


def validate_image(image: dict[str, Any], asset_root: Path | None) -> list[str]:
    identity = image.get("image_id", "<missing>")
    errors = [f"{identity}: missing {field}" for field in sorted(IMAGE_FIELDS - set(image))]
    filename = str(image.get("original_filename", ""))
    if not safe_relative_path(filename):
        errors.append(f"{identity}: original_filename must be a safe relative path")
    if not HASH_PATTERN.fullmatch(str(image.get("sha256", ""))):
        errors.append(f"{identity}: sha256 must be 64 lowercase hexadecimal characters")
    if not isinstance(image.get("width"), int) or image.get("width", 0) <= 0:
        errors.append(f"{identity}: width must be a positive integer")
    if not isinstance(image.get("height"), int) or image.get("height", 0) <= 0:
        errors.append(f"{identity}: height must be a positive integer")
    if image.get("orientation") not in {"landscape", "portrait", "square"}:
        errors.append(f"{identity}: orientation is invalid")
    if image.get("camera_orientation") not in {"landscape", "portrait"}:
        errors.append(f"{identity}: camera_orientation is invalid")
    if image.get("distance") not in {"near", "medium", "far", "not_applicable"}:
        errors.append(f"{identity}: distance is invalid")
    if image.get("target_position") not in {"center", "edge", "small_target", "large_target", "not_applicable"}:
        errors.append(f"{identity}: target_position is invalid")
    if image.get("compression") not in {"none", "camera_default", "controlled_recompression"}:
        errors.append(f"{identity}: compression is invalid")
    if image.get("capture_source_class") not in ALLOWED_CAPTURE_SOURCE_CLASSES:
        errors.append(f"{identity}: capture_source_class is invalid")
    if image.get("negative_class") not in ALLOWED_NEGATIVE_CLASSES:
        errors.append(f"{identity}: negative_class is invalid")
    if image.get("evaluation_partition") not in {"development", "holdout"}:
        errors.append(f"{identity}: evaluation_partition is invalid")
    plates = image.get("plates", [])
    if not isinstance(plates, list) or image.get("plate_count") != len(plates):
        errors.append(f"{identity}: plate_count must equal the plates array length")
    if image.get("plate_present") != bool(plates):
        errors.append(f"{identity}: plate_present must agree with plate_count")
    for index, plate in enumerate(plates if isinstance(plates, list) else []):
        for field in ("polygon_original_pixels", "raw_expected_text", "normalized_expected_text", "script", "style"):
            if field not in plate:
                errors.append(f"{identity}: plates[{index}] missing {field}")
        polygon = plate.get("polygon_original_pixels", [])
        if not isinstance(polygon, list) or len(polygon) < 4 or any(
            not isinstance(point, list) or len(point) != 2 or
            not all(isinstance(coordinate, int) for coordinate in point)
            for point in polygon
        ):
            errors.append(f"{identity}: plates[{index}] polygon must contain at least four integer [x,y] points")
    if image.get("ground_truth_source") != "independent_human_annotation":
        errors.append(f"{identity}: ground truth must be independently human-defined")
    review = image.get("ground_truth_review", {})
    if not isinstance(review, dict):
        errors.append(f"{identity}: ground_truth_review must be an object")
        review = {}
    for field in sorted(GROUND_TRUTH_REVIEW_FIELDS - set(review)):
        errors.append(f"{identity}: ground_truth_review missing {field}")
    if review.get("reviewer_independent") is not True or review.get("annotator") == review.get("reviewer"):
        errors.append(f"{identity}: ground truth requires a distinct independent reviewer")
    if review.get("model_output_consulted") is not False:
        errors.append(f"{identity}: model output must not define or influence ground truth")
    if review.get("disagreements_resolved") is not True:
        errors.append(f"{identity}: ground-truth disagreements must be resolved")
    for field in ("annotator", "annotated_at", "reviewer", "reviewed_at"):
        if not review.get(field):
            errors.append(f"{identity}: ground_truth_review requires {field}")
    privacy = image.get("privacy_screen", {})
    if not isinstance(privacy, dict):
        errors.append(f"{identity}: privacy_screen must be an object")
        privacy = {}
    for field in sorted(PRIVACY_FIELDS - set(privacy)):
        errors.append(f"{identity}: privacy_screen missing {field}")
    if image.get("demo_eligible") is True:
        if image.get("consent_ownership_status") != "owned_or_explicitly_consented":
            errors.append(f"{identity}: demo eligibility requires owned or explicitly consented evidence")
        if privacy.get("review_completed") is not True or not privacy.get("reviewer") or not privacy.get("reviewed_at"):
            errors.append(f"{identity}: demo eligibility requires a completed attributed privacy review")
        risk_fields = PRIVACY_FIELDS - {"review_completed", "reviewer", "reviewed_at", "gps_exif"}
        if any(privacy.get(field) is not False for field in risk_fields):
            errors.append(f"{identity}: demo eligibility requires every visual privacy risk to be false")
        if privacy.get("gps_exif") not in {"absent", "retained_non_sensitive", "removed_in_sanitized_derivative"}:
            errors.append(f"{identity}: GPS/EXIF disposition is not acceptable")
    if image.get("asset_class") == "sanitized_derivative" and not image.get("parent_original_sha256"):
        errors.append(f"{identity}: sanitized derivatives require parent_original_sha256 lineage")
    if asset_root is not None and safe_relative_path(filename):
        candidate = (asset_root / filename).resolve()
        try:
            candidate.relative_to(asset_root.resolve())
        except ValueError:
            errors.append(f"{identity}: resolved asset escapes the pack root")
        else:
            if not candidate.is_file():
                errors.append(f"{identity}: asset file is missing")
            else:
                if sha256(candidate) != image.get("sha256"):
                    errors.append(f"{identity}: asset SHA-256 mismatch")
                signatures = ALLOWED_MAGIC.get(candidate.suffix.lower())
                with candidate.open("rb") as source:
                    header = source.read(8)
                if signatures is None or not any(header.startswith(item) for item in signatures):
                    errors.append(f"{identity}: extension/signature is not an allowed PNG/JPEG image")
    return errors


def validate(payload: dict[str, Any], asset_root: Path | None) -> tuple[list[str], bool]:
    errors: list[str] = []
    if payload.get("contract_version") != CONTRACT:
        errors.append("unexpected contract_version")
    if payload.get("tier") != "T2":
        errors.append("tier must be T2")
    if payload.get("ground_truth_policy") != "independent_human_annotation_only":
        errors.append("ground_truth_policy must prohibit model-derived truth")
    images = payload.get("images")
    if not isinstance(images, list):
        return errors + ["images must be an array"], False
    seen: set[str] = set()
    for image in images:
        identity = image.get("image_id") if isinstance(image, dict) else None
        if not identity or identity in seen:
            errors.append("image_id must be non-empty and unique")
        seen.add(identity)
        if isinstance(image, dict):
            errors.extend(validate_image(image, asset_root))
        else:
            errors.append("every images entry must be an object")
    required_count = payload.get("required_image_count")
    count_ready = isinstance(required_count, int) and required_count > 0 and len(images) == required_count
    partitions = {"development": 0, "holdout": 0}
    for item in images:
        if isinstance(item, dict) and item.get("evaluation_partition") in partitions:
            partitions[item["evaluation_partition"]] += 1
    partition_ready = (
        not payload.get("partition_policy") or
        (partitions["development"] > 0 and partitions["holdout"] > 0)
    )
    acceptance_ready = count_ready and partition_ready and not errors and all(item.get("demo_eligible") is True for item in images)
    return errors, acceptance_ready


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--asset-root", type=Path)
    args = parser.parse_args()
    payload = json.loads(args.manifest.read_text(encoding="utf-8-sig"))
    errors, acceptance_ready = validate(payload, args.asset_root)
    result = {
        "status": "pass" if not errors else "fail",
        "manifest": str(args.manifest),
        "image_count": len(payload.get("images", [])) if isinstance(payload.get("images"), list) else None,
        "acceptance_ready": acceptance_ready,
        "required_image_count": payload.get("required_image_count"),
        "partitions": {
            "development": sum(item.get("evaluation_partition") == "development" for item in payload.get("images", []) if isinstance(item, dict)),
            "holdout": sum(item.get("evaluation_partition") == "holdout" for item in payload.get("images", []) if isinstance(item, dict)),
        },
        "errors": errors,
    }
    print(json.dumps(result, indent=2))
    return 0 if not errors else 1


if __name__ == "__main__":
    raise SystemExit(main())
