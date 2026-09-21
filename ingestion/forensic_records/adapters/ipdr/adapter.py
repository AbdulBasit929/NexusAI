"""Dedicated IPDR/network-session normalization with compatibility hooks."""

from __future__ import annotations

import ipaddress
import json
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Iterable, Mapping

try:
    from ingestion.forensic_records.structured_maturity import (
        IPDR_FIELD_ALIASES as FIELD_ALIASES,
        IPDR_REQUIRED_COLUMN_GROUPS,
    )
except ModuleNotFoundError:  # pragma: no cover - standalone worker image
    from structured_maturity import (
        IPDR_FIELD_ALIASES as FIELD_ALIASES,
        IPDR_REQUIRED_COLUMN_GROUPS,
    )


@dataclass(frozen=True)
class IPDRAdapterRuntime:
    normalize_header: Callable[[str], str]
    clean_text: Callable[[Any], str]
    first_value: Callable[[Mapping[str, Any], Iterable[str]], str]
    parse_timestamp: Callable[[str, str | None], Any]


def load_manifest() -> dict[str, Any]:
    return json.loads((Path(__file__).with_name("manifest.json")).read_text(encoding="utf-8"))


def detect_schema(headers: Iterable[str], normalize_header: Callable[[str], str]) -> dict[str, Any]:
    normalized = {normalize_header(header) for header in headers}
    matched_groups = [
        any(normalize_header(alias) in normalized for alias in aliases)
        for aliases in IPDR_REQUIRED_COLUMN_GROUPS
    ]
    matched_fields = sorted(
        field
        for field, aliases in FIELD_ALIASES.items()
        if any(normalize_header(alias) in normalized for alias in aliases)
    )
    return {
        "adapter_id": "nexusai.adapter.ipdr",
        "family_id": "network_ipdr",
        "matched": all(matched_groups),
        "required_groups_matched": sum(1 for matched in matched_groups if matched),
        "required_groups_total": len(matched_groups),
        "matched_fields": matched_fields,
        "review_required": not all(matched_groups),
    }


def validate_headers(headers: Iterable[str], normalize_header: Callable[[str], str]) -> None:
    if not detect_schema(headers, normalize_header)["matched"]:
        raise ValueError("missing required IPDR source IP, destination IP, or session timestamp column")


def normalize_record(
    raw: Mapping[str, Any],
    observed_at: Any,
    source_timezone: str,
    runtime: IPDRAdapterRuntime,
) -> dict[str, Any]:
    if observed_at is None:
        raise ValueError("IPDR/session timestamp is required")

    def parsed_ip(field: str, label: str, required: bool = False):
        value = runtime.first_value(raw, FIELD_ALIASES[field])
        if not value:
            if required:
                raise ValueError(f"{label} IP is required")
            return None
        try:
            return ipaddress.ip_address(value)
        except ValueError as exc:
            raise ValueError(f"invalid {label} IP") from exc

    def parsed_port(field: str, label: str) -> int | None:
        value = runtime.first_value(raw, FIELD_ALIASES[field])
        if not value:
            return None
        if not value.isdigit() or not 1 <= int(value) <= 65535:
            raise ValueError(f"invalid {label} port")
        return int(value)

    source_ip_raw = runtime.first_value(raw, FIELD_ALIASES["source_ip"]) or ""
    destination_ip_raw = runtime.first_value(raw, FIELD_ALIASES["destination_ip"]) or ""
    source_ip = parsed_ip("source_ip", "source", True)
    destination_ip = parsed_ip("destination_ip", "destination", True)
    nat_source_ip = parsed_ip("nat_source_ip", "NAT source")
    nat_destination_ip = parsed_ip("nat_destination_ip", "NAT destination")

    bytes_text = runtime.first_value(raw, FIELD_ALIASES["bytes"])
    byte_count = None
    if bytes_text:
        if not bytes_text.isdigit():
            raise ValueError("IPDR byte count must be a non-negative integer")
        byte_count = int(bytes_text)
        if byte_count > 9223372036854775807:
            raise ValueError("IPDR byte count exceeds the signed 64-bit audit bound")

    end_text = runtime.first_value(raw, FIELD_ALIASES["session_end"])
    ended_at = runtime.parse_timestamp(end_text, source_timezone)
    if end_text and ended_at is None:
        raise ValueError("invalid IPDR/session end timestamp")
    if ended_at is not None and ended_at < observed_at:
        raise ValueError("IPDR/session end timestamp precedes start timestamp")

    domain_raw = runtime.first_value(raw, FIELD_ALIASES["domain"]) or ""
    domain_ascii = ""
    if domain_raw:
        candidate = domain_raw.rstrip(".").lower()
        try:
            domain_ascii = candidate.encode("idna").decode("ascii")
        except UnicodeError as exc:
            raise ValueError("invalid IPDR domain") from exc
        labels = domain_ascii.split(".")
        if len(domain_ascii) > 253 or any(
            not re.fullmatch(r"[a-z0-9_](?:[a-z0-9_-]{0,61}[a-z0-9_])?", label)
            for label in labels
        ):
            raise ValueError("invalid IPDR domain")

    return {
        "source_ip_raw": source_ip_raw,
        "source_ip_canonical": source_ip.compressed,
        "source_ip_version": source_ip.version,
        "source_ip_private": source_ip.is_private,
        "destination_ip_raw": destination_ip_raw,
        "destination_ip_canonical": destination_ip.compressed,
        "destination_ip_version": destination_ip.version,
        "destination_ip_private": destination_ip.is_private,
        "source_port": parsed_port("source_port", "source"),
        "destination_port": parsed_port("destination_port", "destination"),
        "protocol": (runtime.first_value(raw, FIELD_ALIASES["protocol"]) or "").upper() or None,
        "byte_count": byte_count,
        "session_end_utc": ended_at.isoformat() if ended_at else None,
        "duration_seconds": int((ended_at - observed_at).total_seconds()) if ended_at else None,
        "subscriber_identifier": runtime.first_value(raw, FIELD_ALIASES["subscriber_identifier"]),
        "session_identifier": runtime.first_value(raw, FIELD_ALIASES["session_identifier"]),
        "domain_raw": domain_raw or None,
        "domain_ascii": domain_ascii or None,
        "nat_source_ip_canonical": nat_source_ip.compressed if nat_source_ip else None,
        "nat_destination_ip_canonical": nat_destination_ip.compressed if nat_destination_ip else None,
    }


def verify_normalized_record(record: Mapping[str, Any]) -> dict[str, Any]:
    errors: list[str] = []
    if not record.get("source_ip_canonical"):
        errors.append("source_ip_canonical is required")
    if not record.get("destination_ip_canonical"):
        errors.append("destination_ip_canonical is required")
    if record.get("byte_count") is not None and int(record["byte_count"]) < 0:
        errors.append("byte_count cannot be negative")
    if record.get("duration_seconds") is not None and int(record["duration_seconds"]) < 0:
        errors.append("duration_seconds cannot be negative")
    return {
        "valid": not errors,
        "errors": errors,
        "has_network_endpoints": bool(record.get("source_ip_canonical") and record.get("destination_ip_canonical")),
        "has_nat_locator": bool(record.get("nat_source_ip_canonical") or record.get("nat_destination_ip_canonical")),
        "has_subscriber_link": bool(record.get("subscriber_identifier")),
        "has_session_locator": bool(record.get("session_identifier")),
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
