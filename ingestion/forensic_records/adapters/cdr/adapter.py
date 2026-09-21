"""Dedicated CDR adapter built around the accepted worker normalization.

The adapter owns CDR aliases, lifecycle metadata, validation, and canonical
record construction. Runtime concerns (timezone policy, scalar parsing, and
row hashing) are injected so the existing worker remains the compatibility
authority while the family is extracted without output drift.
"""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Iterable, Mapping

try:
    from ingestion.forensic_records.structured_maturity import (
        CDR_FIELD_ALIASES as FIELD_ALIASES,
        CDR_REQUIRED_COLUMN_GROUPS,
        schema_profile as shared_schema_profile,
    )
except ModuleNotFoundError:  # pragma: no cover - standalone worker image
    from structured_maturity import (
        CDR_FIELD_ALIASES as FIELD_ALIASES,
        CDR_REQUIRED_COLUMN_GROUPS,
        schema_profile as shared_schema_profile,
    )

IDENTIFIER_FIELDS = {
    "msisdn", "call_org_num", "call_dialed_num", "imsi", "imei",
    "lac_id", "site_id", "cell_site_id", "record_reference",
}
TIMESTAMP_FIELDS = {"call_start_ts", "call_end_ts"}
INTEGER_FIELDS = {"duration_seconds"}
DECIMAL_FIELDS = {"network_volume", "latitude", "longitude"}


@dataclass(frozen=True)
class CDRAdapterRuntime:
    normalize_header: Callable[[str], str]
    clean_text: Callable[[Any], str]
    source_timezone: Callable[[Any], str]
    parse_required_timestamp: Callable[[str, str | None], Any]
    parse_timestamp: Callable[[str, str | None], Any]
    parse_integer: Callable[[str], int | None]
    parse_decimal: Callable[[str], Any]
    parse_float: Callable[[str], float | None]
    clean_numeric_identifier: Callable[[str], str | None]
    normalize_phone: Callable[[str | None], tuple[str | None, str | None]]
    row_hash: Callable[[dict[str, Any]], str]


def _canonical_direction(raw_direction: str) -> str | None:
    token = re.sub(r"[^A-Z0-9]", "", raw_direction.upper())
    if token in {"OUT", "OUTBOUND", "OUTGOING", "MO", "MOC", "ORIGINATING"}:
        return "OUTGOING"
    if token in {"IN", "INBOUND", "INCOMING", "MT", "MTC", "TERMINATING"}:
        return "INCOMING"
    if token in {"DATA", "PACKET", "GPRS", "INTERNET"}:
        return "DATA"
    return None


def _identifier(raw_value: str, runtime: CDRAdapterRuntime) -> tuple[str | None, str]:
    token = runtime.clean_text(raw_value)
    if not token:
        return None, "missing"
    upper = token.upper()
    if upper in {"INTERNET", "DATA", "GPRS", "PACKET"}:
        return upper, "packet_service_label"
    if upper in {"UNKNOWN", "UNAVAILABLE", "NULL", "N/A", "NA", "-1"}:
        return None, "sentinel_or_unknown"
    canonical, kind = runtime.normalize_phone(token)
    return canonical, kind or "unrecognized"


def _service_class(call_type: str, called_kind: str, called_raw: str) -> str:
    token = call_type.upper()
    if "USSD" in token or (called_raw.startswith("*") and called_raw.endswith("#")):
        return "USSD"
    if "SMS" in token:
        return "SMS"
    if re.search(r"GPRS|DATA|INTERNET|PACKET", token) or called_kind == "packet_service_label":
        return "PACKET_DATA"
    if re.search(r"VOICE|VOLTE|CALL", token):
        return "VOICE"
    return "OTHER_OR_UNSPECIFIED"


def _timestamp_basis(raw_value: str) -> str:
    value = raw_value.strip()
    if re.search(r"(?:Z|[+-]\d{2}:?\d{2})$", value, re.IGNORECASE):
        return "explicit_source_offset"
    return "assumed_source_timezone"


def load_manifest() -> dict[str, Any]:
    return json.loads((Path(__file__).with_name("manifest.json")).read_text(encoding="utf-8"))


def _normalized_headers(headers: Iterable[str], normalize_header: Callable[[str], str]) -> set[str]:
    return {normalize_header(header) for header in headers}


def detect_schema(headers: Iterable[str], normalize_header: Callable[[str], str]) -> dict[str, Any]:
    normalized = _normalized_headers(headers, normalize_header)
    matched_groups = [
        any(normalize_header(alias) in normalized for alias in aliases)
        for aliases in CDR_REQUIRED_COLUMN_GROUPS
    ]
    matched_fields = sorted(
        field
        for field, aliases in FIELD_ALIASES.items()
        if any(normalize_header(alias) in normalized for alias in aliases)
    )
    return {
        "adapter_id": "nexusai.adapter.cdr",
        "family_id": "communications_cdr",
        "matched": all(matched_groups),
        "required_groups_matched": sum(1 for matched in matched_groups if matched),
        "required_groups_total": len(matched_groups),
        "matched_fields": matched_fields,
        "review_required": not all(matched_groups),
    }


def schema_profile(
    headers: Iterable[str],
    records: Iterable[Mapping[str, Any]],
    normalize_header: Callable[[str], str],
    clean_text: Callable[[Any], str],
) -> dict[str, Any]:
    """Describe source-to-canonical CDR mapping without exposing sample values."""
    return shared_schema_profile("cdr", headers, records, normalize_header, clean_text)


def validate_headers(headers: Iterable[str], normalize_header: Callable[[str], str]) -> None:
    detection = detect_schema(headers, normalize_header)
    if not detection["matched"]:
        raise ValueError("missing required CDR originator, timestamp, or target column")


def normalize_record(
    job: Any,
    raw: Mapping[str, Any],
    batch_id: str,
    row_number: int,
    runtime: CDRAdapterRuntime,
) -> dict[str, Any]:
    values = {runtime.normalize_header(key): value for key, value in raw.items()}

    def value(field: str) -> str:
        for alias in FIELD_ALIASES[field]:
            candidate = runtime.clean_text(values.get(runtime.normalize_header(alias)))
            if candidate:
                return candidate
        return ""

    source_timezone = runtime.source_timezone(job)
    start_raw = value("call_start_ts")
    end_raw = value("call_end_ts")
    start = runtime.parse_required_timestamp(start_raw, source_timezone)
    end = runtime.parse_timestamp(end_raw, source_timezone)
    duration = int((end - start).total_seconds()) if start and end else runtime.parse_integer(value("duration_seconds"))
    msisdn_raw = value("msisdn")
    originator_raw = value("call_org_num")
    called_raw = value("call_dialed_num")
    msisdn_canonical, msisdn_kind = _identifier(msisdn_raw, runtime)
    originator_canonical, originator_kind = _identifier(originator_raw, runtime)
    called_canonical, called_kind = _identifier(called_raw, runtime)
    direction_raw = value("direction")
    direction_canonical = _canonical_direction(direction_raw)
    quality_flags: list[str] = []
    if direction_raw and direction_canonical is None:
        quality_flags.append("direction_unrecognized")
    if originator_kind == "sentinel_or_unknown" or called_kind == "sentinel_or_unknown":
        quality_flags.append("party_identifier_sentinel")
    if end is not None and end < start:
        quality_flags.append("end_before_start")
    if duration is not None and duration < 0:
        quality_flags.append("negative_duration")
    if msisdn_kind == "unrecognized":
        quality_flags.append("subscriber_number_unrecognized")
    if value("cell_site_id") in {"-1", "0", "UNKNOWN", "UNAVAILABLE", "N/A"}:
        quality_flags.append("cell_site_identifier_sentinel")
    canonical = {
        "tenant_id": job.tenant_id,
        "collection_id": job.collection_id,
        "file_id": job.file_id,
        "batch_id": batch_id,
        "row_number": row_number,
        "msisdn": msisdn_raw,
        "call_org_num": originator_raw,
        "call_dialed_num": called_raw,
        "imsi": value("imsi"),
        "imei": value("imei"),
        "call_start_ts": start,
        "call_end_ts": end,
        "duration_seconds": duration,
        "direction": direction_raw.upper() or None,
        "network_volume": runtime.parse_decimal(value("network_volume")),
        "lac_id": runtime.clean_numeric_identifier(value("lac_id")),
        "site_id": runtime.clean_numeric_identifier(value("site_id")),
        "cell_site_id": runtime.clean_numeric_identifier(value("cell_site_id")),
        "latitude": runtime.parse_float(value("latitude")),
        "longitude": runtime.parse_float(value("longitude")),
        "call_type": value("call_type").upper() or None,
        "location": value("location"),
        "provider": value("provider") or None,
        "record_reference": value("record_reference") or None,
        "msisdn_raw": msisdn_raw or None,
        "msisdn_canonical": msisdn_canonical,
        "msisdn_identifier_kind": msisdn_kind,
        "originating_number_raw": originator_raw or None,
        "originating_number_canonical": originator_canonical,
        "originating_number_kind": originator_kind,
        "originating_number_role": "explicit_originating_party",
        "called_number_raw": called_raw or None,
        "called_number_canonical": called_canonical,
        "called_number_kind": called_kind,
        "called_number_role": "explicit_called_party",
        "direction_raw": direction_raw or None,
        "direction_canonical": direction_canonical,
        "service_class": _service_class(value("call_type"), called_kind, called_raw),
        "call_start_raw": start_raw,
        "call_end_raw": end_raw or None,
        "source_timezone": source_timezone,
        "timestamp_basis": _timestamp_basis(start_raw),
        "canonical_timezone": "UTC",
        "manual_review_required": bool(quality_flags),
        "quality_flags": quality_flags,
        "raw_record": json.dumps(dict(raw), ensure_ascii=False),
        "source_file": job.source_file,
    }
    canonical["row_hash"] = runtime.row_hash(canonical)
    canonical["normalized_record"] = json.dumps({
        key: canonical[key]
        for key in (
            "msisdn_raw", "msisdn_canonical", "msisdn_identifier_kind",
            "originating_number_raw", "originating_number_canonical", "originating_number_kind",
            "originating_number_role", "called_number_raw", "called_number_canonical",
            "called_number_kind", "called_number_role", "direction_raw", "direction_canonical",
            "service_class", "call_start_raw", "call_end_raw", "source_timezone",
            "timestamp_basis", "canonical_timezone", "provider", "record_reference",
            "manual_review_required", "quality_flags",
        )
    }, ensure_ascii=False)
    return canonical


def verify_normalized_record(record: Mapping[str, Any]) -> dict[str, Any]:
    errors: list[str] = []
    if not record.get("call_start_ts"):
        errors.append("call_start_ts is required")
    if not (record.get("msisdn") or record.get("call_org_num")):
        errors.append("an originating subscriber or number is required")
    if not record.get("call_dialed_num"):
        errors.append("a dialed target is required")
    if record.get("duration_seconds") is not None and int(record["duration_seconds"]) < 0:
        errors.append("duration_seconds cannot be negative")
    return {
        "valid": not errors,
        "errors": errors,
        "has_source_locator": bool(record.get("source_file") and record.get("row_number")),
        "has_device_identity": bool(record.get("imei") or record.get("imsi")),
        "has_location": bool(record.get("cell_site_id") or record.get("location")),
        "has_explicit_party_roles": bool(record.get("originating_number_role") and record.get("called_number_role")),
        "has_timezone_provenance": bool(record.get("source_timezone") and record.get("timestamp_basis")),
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
