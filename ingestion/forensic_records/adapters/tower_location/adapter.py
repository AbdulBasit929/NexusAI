"""Time-aware tower/site reference normalization.

The adapter records supplied reference facts and quality signals. Coordinates,
sectors, provider labels, and validity timestamps remain observations; they do
not establish RF coverage, device presence, or subscriber location.
"""

from __future__ import annotations

import json
import re
import unicodedata
from dataclasses import dataclass
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any, Callable, Iterable, Mapping

try:
    from ingestion.forensic_records.structured_maturity import (
        TOWER_FIELD_ALIASES as FIELD_ALIASES,
        TOWER_REQUIRED_COLUMN_GROUPS,
    )
except ModuleNotFoundError:  # pragma: no cover - standalone worker image
    from structured_maturity import (
        TOWER_FIELD_ALIASES as FIELD_ALIASES,
        TOWER_REQUIRED_COLUMN_GROUPS,
    )


@dataclass(frozen=True)
class TowerAdapterRuntime:
    normalize_header: Callable[[str], str]
    first_value: Callable[[Mapping[str, Any], Iterable[str]], str]
    clean_text: Callable[[Any], str]
    parse_timestamp: Callable[[str | None, str | None], Any]


def load_manifest() -> dict[str, Any]:
    return json.loads((Path(__file__).with_name("manifest.json")).read_text(encoding="utf-8"))


def detect_schema(headers: Iterable[str], normalize_header: Callable[[str], str]) -> dict[str, Any]:
    normalized = {normalize_header(header) for header in headers}
    matched_groups = [
        any(normalize_header(alias) in normalized for alias in aliases)
        for aliases in TOWER_REQUIRED_COLUMN_GROUPS
    ]
    matched_fields = sorted(
        field for field, aliases in FIELD_ALIASES.items()
        if any(normalize_header(alias) in normalized for alias in aliases)
    )
    matched = all(matched_groups)
    return {
        "adapter_id": "nexusai.adapter.tower_location",
        "family_id": "tower_location",
        "matched": matched,
        "required_groups_matched": sum(matched_groups),
        "required_groups_total": len(matched_groups),
        "matched_fields": matched_fields,
        "review_required": not matched,
    }


def validate_headers(headers: Iterable[str], normalize_header: Callable[[str], str]) -> None:
    detection = detect_schema(headers, normalize_header)
    if not detection["matched"]:
        raise ValueError("tower/site adapter requires a site identifier, latitude, and longitude")


def _decimal(value: str | None, label: str, lower: Decimal, upper: Decimal, upper_inclusive: bool = True) -> Decimal | None:
    if value is None or not str(value).strip():
        return None
    try:
        parsed = Decimal(str(value).strip())
    except InvalidOperation as exc:
        raise ValueError(f"invalid {label} {value!r}") from exc
    outside = parsed < lower or parsed > upper if upper_inclusive else parsed < lower or parsed >= upper
    if outside:
        boundary = f"{lower}..{upper}" if upper_inclusive else f"{lower}..< {upper}"
        raise ValueError(f"{label} {value!r} is outside {boundary}")
    return parsed


def _iso(value: Any) -> str | None:
    return value.isoformat() if value is not None else None


def _provider_key(value: str | None) -> str | None:
    if not value:
        return None
    normalized = unicodedata.normalize("NFKC", value).casefold()
    return re.sub(r"[^a-z0-9]+", "", normalized) or None


def _identifier_key(value: str | None) -> str | None:
    if not value:
        return None
    normalized = unicodedata.normalize("NFKC", value).casefold()
    return re.sub(r"[^a-z0-9]+", "", normalized) or None


def _datum(value: str | None) -> tuple[str, str]:
    raw = (value or "").strip()
    if not raw:
        return "WGS84", "assumed_wgs84"
    key = re.sub(r"[^A-Z0-9]", "", raw.upper())
    if key in {"WGS84", "EPSG4326", "4326"}:
        return "WGS84", "supplied_wgs84_equivalent"
    return raw.upper(), "supplied_requires_transform"


def _uncertainty_class(value: Decimal | None) -> str:
    if value is None:
        return "unknown"
    if value <= 25:
        return "high_precision_supplied"
    if value <= 100:
        return "bounded_supplied"
    if value <= 1000:
        return "coarse_supplied"
    return "broad_supplied"


def normalize_record(
    raw: Mapping[str, Any],
    latitude: float | None,
    longitude: float | None,
    observed_at: Any,
    source_timezone: str,
    runtime: TowerAdapterRuntime,
) -> dict[str, Any]:
    site_id = runtime.first_value(raw, FIELD_ALIASES["site_identifier"])
    if not site_id:
        raise ValueError("tower/cell site identifier is required")
    if latitude is None or longitude is None:
        raise ValueError("tower latitude and longitude are required")

    azimuth = _decimal(runtime.first_value(raw, FIELD_ALIASES["azimuth"]), "azimuth", Decimal("0"), Decimal("360"), False)
    beamwidth = _decimal(runtime.first_value(raw, FIELD_ALIASES["beamwidth"]), "beamwidth", Decimal("0"), Decimal("360"))
    uncertainty = _decimal(runtime.first_value(raw, FIELD_ALIASES["uncertainty_radius_m"]), "uncertainty radius", Decimal("0"), Decimal("100000"))
    datum_raw = runtime.first_value(raw, FIELD_ALIASES["datum"])
    datum, datum_basis = _datum(datum_raw)
    valid_from_text = runtime.first_value(raw, FIELD_ALIASES["valid_from"])
    valid_to_text = runtime.first_value(raw, FIELD_ALIASES["valid_to"])
    valid_from = runtime.parse_timestamp(valid_from_text, source_timezone) if valid_from_text else observed_at
    valid_to = runtime.parse_timestamp(valid_to_text, source_timezone) if valid_to_text else None
    if valid_from_text and valid_from is None:
        raise ValueError(f"invalid tower valid-from timestamp {valid_from_text!r}")
    if valid_to_text and valid_to is None:
        raise ValueError(f"invalid tower valid-to timestamp {valid_to_text!r}")

    flags: list[str] = []
    if not datum_raw:
        flags.append("coordinate_datum_assumed_wgs84")
    if datum_basis == "supplied_requires_transform":
        flags.append("coordinate_datum_requires_transform")
    if not (23.5 <= latitude <= 37.1 and 60.8 <= longitude <= 77.2):
        flags.append("outside_pakistan_screening_bounds")
    if azimuth is None:
        flags.append("sector_azimuth_missing")
    if not runtime.first_value(raw, FIELD_ALIASES["sector_identifier"]):
        flags.append("sector_identifier_missing")
    if uncertainty is None:
        flags.append("uncertainty_radius_missing")
    if valid_from is not None and valid_to is not None and valid_to < valid_from:
        flags.append("reference_window_inverted")

    provider = runtime.first_value(raw, FIELD_ALIASES["provider"])
    sector_id = runtime.first_value(raw, FIELD_ALIASES["sector_identifier"])
    site_key = _identifier_key(site_id)
    sector_key = _identifier_key(sector_id)
    provider_key = _provider_key(provider)
    if valid_from is not None and valid_to is not None and valid_to < valid_from:
        validity_status = "inverted_requires_review"
    elif valid_to is not None:
        validity_status = "closed_window"
    elif valid_from_text:
        validity_status = "open_ended_window"
    else:
        validity_status = "observed_at_fallback_open_window"
    return {
        "site_identifier": site_id,
        "site_identifier_raw": site_id,
        "site_identifier_search_key": site_key,
        "sector_identifier": sector_id or None,
        "sector_identifier_raw": sector_id or None,
        "sector_identifier_search_key": sector_key,
        "technology": runtime.first_value(raw, FIELD_ALIASES["technology"]) or None,
        "lac": runtime.first_value(raw, FIELD_ALIASES["lac"]) or None,
        "tac": runtime.first_value(raw, FIELD_ALIASES["tac"]) or None,
        "provider_label": provider or None,
        "provider_alias_key": provider_key,
        "provider_code": runtime.first_value(raw, FIELD_ALIASES["provider_code"]) or None,
        "mcc": runtime.first_value(raw, FIELD_ALIASES["mcc"]) or None,
        "mnc": runtime.first_value(raw, FIELD_ALIASES["mnc"]) or None,
        "cgi": runtime.first_value(raw, FIELD_ALIASES["cgi"]) or None,
        "ecgi": runtime.first_value(raw, FIELD_ALIASES["ecgi"]) or None,
        "enodeb_id": runtime.first_value(raw, FIELD_ALIASES["enodeb_id"]) or None,
        "gnodeb_id": runtime.first_value(raw, FIELD_ALIASES["gnodeb_id"]) or None,
        "reference_identifier": runtime.first_value(raw, FIELD_ALIASES["reference_identifier"]) or None,
        "reference_version": runtime.first_value(raw, FIELD_ALIASES["reference_version"]) or None,
        "history_key": "|".join((provider_key or "unknown-provider", site_key or "unknown-site", sector_key or "all-sectors")),
        "site_location": runtime.first_value(raw, FIELD_ALIASES["location"]) or None,
        "district": runtime.first_value(raw, FIELD_ALIASES["district"]) or None,
        "latitude": latitude,
        "longitude": longitude,
        "azimuth_degrees": str(azimuth) if azimuth is not None else None,
        "beamwidth_degrees": str(beamwidth) if beamwidth is not None else None,
        "uncertainty_radius_m": str(uncertainty) if uncertainty is not None else None,
        "coordinate_datum": datum,
        "coordinate_datum_raw": datum_raw or None,
        "coordinate_datum_basis": datum_basis,
        "coordinate_method": runtime.first_value(raw, FIELD_ALIASES["coordinate_method"]) or None,
        "coordinate_source": runtime.first_value(raw, FIELD_ALIASES["coordinate_source"]) or None,
        "coordinate_provenance": "supplied_source_row",
        "uncertainty_class": _uncertainty_class(uncertainty),
        "operational_status": (runtime.first_value(raw, FIELD_ALIASES["status"]) or "").upper() or None,
        "reference_valid_from_at": _iso(valid_from),
        "reference_valid_to_at": _iso(valid_to),
        "reference_valid_from_basis": "supplied_source_field" if valid_from_text else "observed_at_fallback",
        "reference_valid_to_basis": "supplied_source_field" if valid_to_text else "open_or_unknown",
        "validity_status": validity_status,
        "source_timezone": source_timezone,
        "manual_review_required": bool(flags),
        "quality_flags": flags,
        "rf_coverage_inference_allowed": False,
    }


def verify_normalized_record(record: Mapping[str, Any]) -> dict[str, Any]:
    errors: list[str] = []
    if not record.get("site_identifier"):
        errors.append("site_identifier is required")
    if record.get("latitude") is None or record.get("longitude") is None:
        errors.append("latitude and longitude are required")
    if record.get("rf_coverage_inference_allowed") is not False:
        errors.append("RF coverage inference must remain disabled")
    return {
        "valid": not errors,
        "errors": errors,
        "time_aware_reference_ready": bool(record.get("reference_valid_from_at")),
        "coordinate_review_required": bool(record.get("manual_review_required")),
        "history_key_ready": bool(record.get("history_key")),
        "validity_status": record.get("validity_status"),
    }


def adapter_capabilities() -> dict[str, Any]:
    manifest = load_manifest()
    return {
        "adapter_id": manifest["id"],
        "family_id": manifest["family_id"],
        "normalization_version": manifest["normalization_version"],
        "operation_ids": tuple(manifest["operation_ids"]),
        "model_required": manifest["resource_profile"]["model_required"],
    }
