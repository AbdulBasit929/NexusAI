"""Privacy-aware subscriber identity normalization.

The adapter preserves the Phase 2 subscriber representation for compatibility,
adds explicit validity fields, and publishes only deterministic quality signals.
It never resolves a person, assigns ownership, or treats the newest row as the
historical owner of an identifier.
"""

from __future__ import annotations

import json
import re
import unicodedata
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Iterable, Mapping

try:
    from ingestion.forensic_records.structured_maturity import (
        SUBSCRIBER_FIELD_ALIASES as FIELD_ALIASES,
        SUBSCRIBER_REQUIRED_COLUMN_GROUPS,
    )
except ModuleNotFoundError:  # pragma: no cover - standalone worker image
    from structured_maturity import (
        SUBSCRIBER_FIELD_ALIASES as FIELD_ALIASES,
        SUBSCRIBER_REQUIRED_COLUMN_GROUPS,
    )


@dataclass(frozen=True)
class SubscriberAdapterRuntime:
    normalize_header: Callable[[str], str]
    first_value: Callable[[Mapping[str, Any], Iterable[str]], str]
    clean_text: Callable[[Any], str]
    normalize_phone: Callable[[str | None], tuple[str | None, str | None]]
    luhn_valid: Callable[[str], bool]
    parse_timestamp: Callable[[str | None, str | None], Any]


def load_manifest() -> dict[str, Any]:
    return json.loads((Path(__file__).with_name("manifest.json")).read_text(encoding="utf-8"))


def detect_schema(headers: Iterable[str], normalize_header: Callable[[str], str]) -> dict[str, Any]:
    normalized = {normalize_header(header) for header in headers}
    matched_groups = [
        any(normalize_header(alias) in normalized for alias in aliases)
        for aliases in SUBSCRIBER_REQUIRED_COLUMN_GROUPS
    ]
    matched_fields = sorted(
        field
        for field, aliases in FIELD_ALIASES.items()
        if any(normalize_header(alias) in normalized for alias in aliases)
    )
    matched = all(matched_groups)
    return {
        "adapter_id": "nexusai.adapter.subscriber_identity",
        "family_id": "subscriber_identity",
        "matched": matched,
        "required_groups_matched": sum(1 for value in matched_groups if value),
        "required_groups_total": len(matched_groups),
        "matched_fields": matched_fields,
        "review_required": not matched,
    }


def validate_headers(headers: Iterable[str], normalize_header: Callable[[str], str]) -> None:
    detection = detect_schema(headers, normalize_header)
    if detection["matched"]:
        return
    if detection["required_groups_matched"] == 0:
        raise ValueError("missing required subscriber MSISDN and identity columns")
    raise ValueError("missing required subscriber MSISDN or identity column")


def _timestamp(value: str, timezone_name: str, runtime: SubscriberAdapterRuntime, label: str) -> Any:
    if not runtime.clean_text(value):
        return None
    parsed = runtime.parse_timestamp(value, timezone_name)
    if parsed is None:
        raise ValueError(f"invalid subscriber {label} {value!r}")
    return parsed


def _iso(value: Any) -> str | None:
    return value.isoformat() if value is not None else None


def _name_script(value: str) -> str:
    scripts: set[str] = set()
    for char in value:
        if not char.isalpha():
            continue
        name = unicodedata.name(char, "")
        if "ARABIC" in name:
            scripts.add("arabic")
        elif "LATIN" in name:
            scripts.add("latin")
        else:
            scripts.add("other")
    if not scripts:
        return "unknown"
    return next(iter(scripts)) if len(scripts) == 1 else "mixed"


def normalize_record(
    raw: Mapping[str, Any],
    observed_at: Any,
    source_timezone: str,
    runtime: SubscriberAdapterRuntime,
) -> dict[str, Any]:
    msisdn = runtime.first_value(raw, FIELD_ALIASES["msisdn"])
    if not msisdn:
        raise ValueError("subscriber MSISDN/phone value is required")
    identity = runtime.first_value(raw, SUBSCRIBER_REQUIRED_COLUMN_GROUPS[1])
    if not identity:
        raise ValueError("subscriber identity reference is required")

    phone_compact, phone_kind = runtime.normalize_phone(msisdn)
    cnic_raw = runtime.first_value(raw, FIELD_ALIASES["cnic"])
    cnic_digits = re.sub(r"\D", "", cnic_raw or "") or None
    cnic_valid = None if not cnic_raw else bool(re.fullmatch(r"\d{13}", cnic_digits or ""))
    imsi = re.sub(r"\s", "", runtime.first_value(raw, FIELD_ALIASES["imsi"]) or "") or None
    imei = re.sub(r"[\s-]", "", runtime.first_value(raw, FIELD_ALIASES["imei"]) or "") or None
    imsi_valid = None if not imsi else bool(re.fullmatch(r"\d{15}", imsi))
    imei_length_valid = None if not imei else bool(re.fullmatch(r"\d{14,16}", imei))
    imei_luhn_valid = runtime.luhn_valid(imei) if imei and len(imei) == 15 and imei.isdigit() else None
    iccid = re.sub(r"[\s-]", "", runtime.first_value(raw, FIELD_ALIASES["iccid"]) or "") or None
    iccid_format_valid = None if not iccid else bool(re.fullmatch(r"\d{18,22}", iccid))

    activation_at = observed_at or _timestamp(runtime.first_value(raw, FIELD_ALIASES["activation_at"]), source_timezone, runtime, "activation timestamp")
    deactivation_at = _timestamp(runtime.first_value(raw, FIELD_ALIASES["deactivation_at"]), source_timezone, runtime, "deactivation timestamp")
    valid_from = _timestamp(runtime.first_value(raw, FIELD_ALIASES["valid_from"]), source_timezone, runtime, "valid-from timestamp") or activation_at
    valid_to = _timestamp(runtime.first_value(raw, FIELD_ALIASES["valid_to"]), source_timezone, runtime, "valid-to timestamp") or deactivation_at

    flags: list[str] = []
    if activation_at is None:
        flags.append("activation_timestamp_missing")
    if phone_kind == "unrecognized":
        flags.append("phone_format_unrecognized")
    if cnic_valid is False:
        flags.append("cnic_format_invalid")
    if imsi_valid is False:
        flags.append("imsi_format_invalid")
    if imsi_valid and not imsi.startswith("410"):
        flags.append("imsi_non_pakistan_mcc")
    if imei_length_valid is False:
        flags.append("imei_length_invalid")
    if imei_luhn_valid is False:
        flags.append("imei_luhn_invalid")
    if iccid_format_valid is False:
        flags.append("iccid_format_invalid")
    if activation_at is not None and deactivation_at is not None and deactivation_at < activation_at:
        flags.append("deactivation_before_activation")
    if valid_from is not None and valid_to is not None and valid_to < valid_from:
        flags.append("validity_window_inverted")

    subscriber_name = runtime.first_value(raw, FIELD_ALIASES["subscriber_name"])
    normalized_name = unicodedata.normalize("NFKC", subscriber_name).casefold() if subscriber_name else None
    normalized_name = " ".join(normalized_name.split()) if normalized_name else None
    status = runtime.first_value(raw, FIELD_ALIASES["status"])
    service_identifier = runtime.first_value(raw, FIELD_ALIASES["service_identifier"])
    service_type = runtime.first_value(raw, FIELD_ALIASES["service_type"])
    provider = runtime.first_value(raw, FIELD_ALIASES["provider"])
    plan = runtime.first_value(raw, FIELD_ALIASES["plan"])
    if valid_from is not None and valid_to is not None and valid_to < valid_from:
        validity_status = "inverted_requires_review"
    elif valid_to is not None:
        validity_status = "closed_window"
    elif valid_from is not None:
        validity_status = "open_ended_window"
    else:
        validity_status = "unknown_window"
    association_roles = [
        role for role, identifier in (
            ("subscriber_msisdn", phone_compact),
            ("subscriber_reference", runtime.first_value(raw, FIELD_ALIASES["subscriber_reference"])),
            ("sim_imsi", imsi),
            ("sim_iccid", iccid),
            ("device_imei", imei),
            ("service_reference", service_identifier),
        ) if identifier
    ]
    return {
        # Full source values remain in the protected normalized record for
        # backwards compatibility. Query operations mask sensitive identity.
        "msisdn_raw": msisdn,
        "msisdn_canonical": phone_compact,
        "phone_identifier_kind": phone_kind,
        "subscriber_name": subscriber_name or None,
        "subscriber_name_search_key": normalized_name,
        "subscriber_name_script": _name_script(subscriber_name) if subscriber_name else None,
        "subscriber_reference": runtime.first_value(raw, FIELD_ALIASES["subscriber_reference"]) or None,
        "cnic_raw": cnic_raw or None,
        "cnic_digits": cnic_digits,
        "cnic_masked": ("*********" + cnic_digits[-4:]) if cnic_digits and len(cnic_digits) >= 4 else None,
        "cnic_format_valid": cnic_valid,
        "imsi_raw": imsi,
        "imsi_format_valid": imsi_valid,
        "imei_raw": imei,
        "imei_length_valid": imei_length_valid,
        "imei_luhn_valid": imei_luhn_valid,
        "iccid_raw": iccid,
        "iccid_format_valid": iccid_format_valid,
        "service_identifier": service_identifier or None,
        "service_type": service_type.upper() if service_type else None,
        "provider": provider or None,
        "service_plan": plan or None,
        "subscriber_status": status.upper() if status else None,
        "activation_at": _iso(activation_at),
        "deactivation_at": _iso(deactivation_at),
        "valid_from_at": _iso(valid_from),
        "valid_to_at": _iso(valid_to),
        "validity_status": validity_status,
        "association_basis": "explicit_source_row_co_observation",
        "association_roles": association_roles,
        "manual_review_required": bool(flags),
        "quality_flags": flags,
    }


def verify_normalized_record(record: Mapping[str, Any]) -> dict[str, Any]:
    errors: list[str] = []
    if not record.get("msisdn_canonical"):
        errors.append("msisdn_canonical is required")
    if not (record.get("subscriber_reference") or record.get("cnic_digits") or record.get("subscriber_name")):
        errors.append("an explicit subscriber identity reference is required")
    if record.get("cnic_digits") and not record.get("cnic_masked"):
        errors.append("cnic_masked is required when CNIC is present")
    return {
        "valid": not errors,
        "errors": errors,
        "has_exact_msisdn": bool(record.get("msisdn_canonical")),
        "has_validity_start": bool(record.get("valid_from_at")),
        "has_device_identity": bool(record.get("imsi_raw") or record.get("imei_raw")),
        "has_sim_identity": bool(record.get("imsi_raw") or record.get("iccid_raw")),
        "has_service_identity": bool(record.get("service_identifier") or record.get("service_type")),
        "association_is_observation_only": record.get("association_basis") == "explicit_source_row_co_observation",
        "cnic_default_redaction_ready": not record.get("cnic_digits") or bool(record.get("cnic_masked")),
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
