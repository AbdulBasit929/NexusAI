"""Dedicated ANPR normalization with evidence-safe compatibility hooks.

The adapter preserves the accepted Phase 4 representation while making the
family lifecycle independently testable. Plate search keys support exact
format-insensitive matching; they are not plate-registration validation or a
claim about vehicle ownership.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Iterable, Mapping

try:
    from ingestion.forensic_records.structured_maturity import (
        ANPR_FIELD_ALIASES as FIELD_ALIASES,
        ANPR_REQUIRED_COLUMN_GROUPS,
    )
except ModuleNotFoundError:  # pragma: no cover - standalone worker image
    from structured_maturity import (
        ANPR_FIELD_ALIASES as FIELD_ALIASES,
        ANPR_REQUIRED_COLUMN_GROUPS,
    )


@dataclass(frozen=True)
class ANPRAdapterRuntime:
    normalize_header: Callable[[str], str]
    first_value: Callable[[Mapping[str, Any], Iterable[str]], str]
    parse_confidence: Callable[[str | None], Any]
    plate_script: Callable[[str], str]
    plate_search_key: Callable[[str], str]
    valid_sha256: Callable[[str], bool]


def load_manifest() -> dict[str, Any]:
    return json.loads((Path(__file__).with_name("manifest.json")).read_text(encoding="utf-8"))


def detect_schema(headers: Iterable[str], normalize_header: Callable[[str], str]) -> dict[str, Any]:
    normalized = {normalize_header(header) for header in headers}
    matched_groups = [
        any(normalize_header(alias) in normalized for alias in aliases)
        for aliases in ANPR_REQUIRED_COLUMN_GROUPS
    ]
    matched_fields = sorted(
        field
        for field, aliases in FIELD_ALIASES.items()
        if any(normalize_header(alias) in normalized for alias in aliases)
    )
    matched = all(matched_groups)
    return {
        "adapter_id": "nexusai.adapter.anpr",
        "family_id": "anpr_vehicles",
        "matched": matched,
        "required_groups_matched": sum(1 for value in matched_groups if value),
        "required_groups_total": len(matched_groups),
        "matched_fields": matched_fields,
        "review_required": not matched,
    }


def validate_headers(headers: Iterable[str], normalize_header: Callable[[str], str]) -> None:
    if not detect_schema(headers, normalize_header)["matched"]:
        raise ValueError("missing required ANPR plate column")


def normalize_record(
    raw: Mapping[str, Any],
    observed_at: Any,
    runtime: ANPRAdapterRuntime,
) -> dict[str, Any]:
    plate = runtime.first_value(raw, FIELD_ALIASES["plate"])
    if not plate:
        raise ValueError("ANPR plate value is required")
    confidence = runtime.parse_confidence(runtime.first_value(raw, FIELD_ALIASES["confidence"]))
    crop_hash = runtime.first_value(raw, FIELD_ALIASES["crop_hash"])
    if crop_hash and not runtime.valid_sha256(crop_hash):
        raise ValueError("ANPR image crop hash must be a 64-character SHA-256 value")
    province = runtime.first_value(raw, FIELD_ALIASES["province"])
    rule_version = runtime.first_value(raw, FIELD_ALIASES["rule_version"])
    flags: list[str] = []
    if observed_at is None:
        flags.append("timestamp_missing")
    if confidence is None:
        flags.append("confidence_missing")
    elif float(confidence) < 0.85:
        flags.append("low_ocr_confidence")
    if not province:
        flags.append("province_unconfirmed")
    if not rule_version:
        flags.append("series_rule_version_missing")
    return {
        "plate_raw": plate,
        "plate_search_key": runtime.plate_search_key(plate),
        "plate_script": runtime.plate_script(plate),
        "ocr_confidence": str(confidence) if confidence is not None else None,
        "province_hypothesis": province,
        "province_series_rule_version": rule_version,
        "image_crop_sha256": crop_hash.lower() if crop_hash else None,
        "manual_review_required": bool(flags),
        "quality_flags": flags,
    }


def verify_normalized_record(record: Mapping[str, Any]) -> dict[str, Any]:
    errors: list[str] = []
    if not record.get("plate_raw"):
        errors.append("plate_raw is required")
    if not record.get("plate_search_key"):
        errors.append("plate_search_key is required")
    confidence = record.get("ocr_confidence")
    if confidence is not None:
        try:
            numeric_confidence = float(confidence)
        except (TypeError, ValueError):
            errors.append("ocr_confidence must be numeric")
        else:
            if not 0 <= numeric_confidence <= 1:
                errors.append("ocr_confidence must be within 0..1")
    crop_hash = record.get("image_crop_sha256")
    if crop_hash and (len(str(crop_hash)) != 64 or any(char not in "0123456789abcdef" for char in str(crop_hash).lower())):
        errors.append("image_crop_sha256 must be lowercase hexadecimal SHA-256")
    return {
        "valid": not errors,
        "errors": errors,
        "has_plate_locator": bool(record.get("plate_raw") and record.get("plate_search_key")),
        "has_confidence": confidence is not None,
        "has_image_crop_hash": bool(crop_hash),
        "manual_review_required": bool(record.get("manual_review_required")),
    }


def adapter_capabilities() -> dict[str, Any]:
    manifest = load_manifest()
    return {
        "adapter_id": manifest["id"],
        "version": manifest["version"],
        "family_id": manifest["family_id"],
        "status": manifest["status"],
        "operation_ids": manifest["operation_ids"],
        "normalization_version": manifest["normalization_version"],
    }
