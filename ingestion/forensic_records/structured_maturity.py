"""Shared STIM contracts for structured schema, mapping, and source attributes.

This module is the single ingestion-side registry for family aliases and
canonical field semantics. Adapters remain responsible for family validation
and normalization; the registry explains those decisions without creating a
second persistence system.
"""

from __future__ import annotations

import re
from datetime import datetime
from decimal import Decimal, InvalidOperation
from typing import Any, Callable, Iterable, Mapping


SCHEMA_REGISTRY_VERSION = "forensics.schema-registry/v1"
SCHEMA_PROFILE_VERSION = "forensics.schema-profile/v1"
DYNAMIC_ATTRIBUTE_VERSION = "forensics.dynamic-attribute/v1"
QUALITY_CONTRACT_VERSION = "forensics.structured-quality/v1"
TIME_POLICY_VERSION = "forensics.time-policy/v1"
DYNAMIC_ATTRIBUTE_STATES = {"canonical", "recognized_extension", "unmapped", "ambiguous", "invalid"}

CDR_REQUIRED_COLUMN_GROUPS = (
    ("MSISDN", "call_org_num", "source_number", "a_number"),
    ("CALL_START_DT_TM", "call_start_time", "call_time", "timestamp", "start_time"),
    ("CALL_DIALED_NUM", "dialed_number", "target_number", "b_number"),
)
CDR_FIELD_ALIASES: dict[str, tuple[str, ...]] = {
    "msisdn": ("MSISDN", "msisdn"),
    "call_org_num": ("call_org_num", "source_number", "a_number"),
    "call_dialed_num": ("CALL_DIALED_NUM", "dialed_number", "target_number", "b_number"),
    "imsi": ("IMSI", "imsi"),
    "imei": ("IMEI", "imei"),
    "call_start_ts": ("CALL_START_DT_TM", "call_start_time", "call_time", "timestamp", "start_time"),
    "call_end_ts": ("CALL_END_DT_TM", "call_end_time", "end_time"),
    "duration_seconds": ("duration_seconds", "duration"),
    "direction": ("INBOUND_OUTBOUND_IND", "direction"),
    "network_volume": ("Call_Network_Volume", "network_volume", "bytes"),
    "lac_id": ("Lac_Id", "lac_id", "lac"),
    "site_id": ("Site_Id", "site_id"),
    "cell_site_id": ("Cell_SITE_ID", "cell_site_id", "cell_id"),
    "latitude": ("lat", "latitude"),
    "longitude": ("longitude", "lon", "lng"),
    "call_type": ("CALL_TYPE", "call_type"),
    "location": ("location", "site_name", "area"),
    "provider": ("provider", "operator", "network_operator", "carrier"),
    "record_reference": ("record_id", "cdr_id", "event_id", "call_id"),
}

IPDR_REQUIRED_COLUMN_GROUPS = (
    ("source_ip", "src_ip", "ip_address"),
    ("destination_ip", "dst_ip", "dest_ip"),
    ("timestamp", "session_start", "start_time", "event_time", "time"),
)
IPDR_FIELD_ALIASES: dict[str, tuple[str, ...]] = {
    "source_ip": ("source_ip", "src_ip", "ip_address"),
    "destination_ip": ("destination_ip", "dst_ip", "dest_ip"),
    "nat_source_ip": ("nat_source_ip", "translated_source_ip", "public_ip", "post_nat_source_ip"),
    "nat_destination_ip": ("nat_destination_ip", "translated_destination_ip", "post_nat_destination_ip"),
    "source_port": ("source_port", "src_port"),
    "destination_port": ("destination_port", "dst_port", "dest_port"),
    "protocol": ("protocol", "transport_protocol", "application_protocol"),
    "bytes": ("bytes", "total_bytes", "byte_count", "octets", "network_volume"),
    "session_start": ("timestamp", "session_start", "start_time", "event_time", "time"),
    "session_end": ("session_end", "end_time", "stop_time", "ended_at"),
    "subscriber_identifier": ("subscriber_id", "msisdn", "imsi", "user_id"),
    "session_identifier": ("session_id", "flow_id", "record_id", "correlation_id"),
    "domain": ("domain", "hostname", "host", "fqdn", "dns_name"),
}

ANPR_REQUIRED_COLUMN_GROUPS = ((
    "plate", "plate_number", "license_plate", "registration_number", "registration_no",
    "reg_no", "vehicle_no", "number_plate", "plate_no", "vehicle_registration_no",
    "registration_mark", "vrn",
),)
ANPR_FIELD_ALIASES: dict[str, tuple[str, ...]] = {
    "plate": ANPR_REQUIRED_COLUMN_GROUPS[0],
    "observed_at": ("timestamp", "event_time", "capture_time", "captured_at", "capture_datetime", "observed_at", "camera_time", "reading_time", "date_time", "time"),
    "camera_id": ("camera_id", "camera", "camera_code", "camera_name", "device_id", "checkpoint", "checkpoint_id", "gantry_id"),
    "lane": ("lane", "lane_id"),
    "location": ("location", "camera_location", "checkpoint", "site", "camera_site", "toll_plaza", "district", "city"),
    "latitude": ("lat", "latitude", "y"),
    "longitude": ("longitude", "lon", "lng", "x"),
    "confidence": ("ocr_confidence", "plate_confidence", "confidence", "recognition_confidence", "score"),
    "province": ("province", "registration_province", "jurisdiction", "plate_region"),
    "rule_version": ("province_series_rule_version", "plate_rule_version", "series_rule_version"),
    "crop_hash": ("image_crop_hash", "plate_crop_hash", "crop_sha256", "image_sha256"),
}

SUBSCRIBER_REQUIRED_COLUMN_GROUPS = (
    ("msisdn", "phone_number", "subscriber_number", "mobile_no", "cellular_number", "mdn", "account_msisdn"),
    ("cnic", "cnic_no", "national_id", "nic", "cnic_last4", "customer_id", "customer_no", "subscriber_id", "account_id", "name", "full_name", "subscriber_name", "customer_name"),
)
SUBSCRIBER_FIELD_ALIASES: dict[str, tuple[str, ...]] = {
    "msisdn": SUBSCRIBER_REQUIRED_COLUMN_GROUPS[0],
    "subscriber_reference": ("subscriber_id", "customer_id", "customer_no", "account_id"),
    "subscriber_name": ("name", "full_name", "subscriber_name", "customer_name"),
    "cnic": ("cnic", "cnic_no", "national_id", "nic"),
    "imsi": ("imsi", "subscriber_imsi"),
    "imei": ("imei", "device_imei", "imei_sv"),
    "status": ("status", "subscriber_status", "account_status"),
    "activation_at": ("activation_date", "activated_at", "activation_at", "created_at"),
    "deactivation_at": ("deactivation_date", "deactivated_at", "deactivation_at", "closed_at", "termination_date"),
    "valid_from": ("valid_from", "valid_from_at", "effective_from", "start_date"),
    "valid_to": ("valid_to", "valid_to_at", "effective_to", "end_date", "expiry_date"),
    "iccid": ("iccid", "sim_serial", "sim_number", "sim_id"),
    "service_identifier": ("service_id", "service_reference", "subscription_id", "account_service_id"),
    "service_type": ("service_type", "subscription_type", "connection_type", "customer_type"),
    "provider": ("provider", "operator", "network_operator", "carrier"),
    "plan": ("plan", "tariff", "package", "rate_plan"),
}

TOWER_REQUIRED_COLUMN_GROUPS = (
    ("cell_id", "cell_site_id", "site_id", "site_code", "site_identifier", "cgi", "ecgi", "enodeb_id", "gnodeb_id"),
    ("lat", "latitude", "latitude_wgs84"),
    ("lon", "lng", "longitude", "longitude_wgs84"),
)
TOWER_FIELD_ALIASES: dict[str, tuple[str, ...]] = {
    "site_identifier": TOWER_REQUIRED_COLUMN_GROUPS[0],
    "sector_identifier": ("sector_id", "sector", "cell_sector", "sector_code", "cell_name"),
    "latitude": TOWER_REQUIRED_COLUMN_GROUPS[1],
    "longitude": TOWER_REQUIRED_COLUMN_GROUPS[2],
    "technology": ("technology", "radio_access_technology", "rat", "network_type"),
    "lac": ("lac", "lac_id", "location_area_code"), "tac": ("tac", "tracking_area_code"),
    "provider": ("provider", "operator", "operator_name", "network_operator", "carrier", "licensee"),
    "provider_code": ("provider_code", "operator_code", "carrier_code", "plmn"),
    "mcc": ("mcc", "mobile_country_code"), "mnc": ("mnc", "mobile_network_code"),
    "cgi": ("cgi", "cell_global_identity"), "ecgi": ("ecgi", "e_utran_cell_global_identifier"),
    "enodeb_id": ("enodeb_id", "enb_id", "e_node_b_id"), "gnodeb_id": ("gnodeb_id", "gnb_id", "g_node_b_id"),
    "reference_identifier": ("reference_id", "reference_identifier", "inventory_id", "record_id"),
    "reference_version": ("reference_version", "version", "revision", "inventory_version"),
    "location": ("location", "site_location", "address", "place"), "district": ("district", "city", "region"),
    "azimuth": ("azimuth", "sector_azimuth", "bearing"), "beamwidth": ("beamwidth", "beam_width", "horizontal_beamwidth"),
    "datum": ("coordinate_datum", "datum", "crs"),
    "uncertainty_radius_m": ("uncertainty_radius_m", "accuracy_m", "radius_m"),
    "coordinate_method": ("coordinate_method", "survey_method", "location_method"),
    "coordinate_source": ("coordinate_source", "location_source", "coordinate_provider"),
    "status": ("status", "site_status", "operational_status"),
    "valid_from": ("valid_from", "effective_from", "updated_at", "last_updated"),
    "valid_to": ("valid_to", "effective_to", "retired_at"),
}

TRANSACTION_REQUIRED_COLUMN_GROUPS = (
    ("amount", "transaction_amount", "txn_amount", "amount_pkr", "debit_amount", "credit_amount", "transaction_value", "local_amount"),
    ("account", "account_number", "iban", "from_account", "from_iban", "source_account", "debit_account", "payer_account", "remitter_account", "sender", "sender_iban"),
)
TRANSACTION_FIELD_ALIASES: dict[str, tuple[str, ...]] = {
    "observed_at": ("timestamp", "transaction_time", "transaction_timestamp", "txn_time", "transaction_date", "booking_date", "value_date", "created_at"),
    "transaction_reference": ("transaction_id", "txn_id", "reference", "reference_id", "record_id"),
    "amount": TRANSACTION_REQUIRED_COLUMN_GROUPS[0],
    "currency": ("currency", "currency_code", "ccy"),
    "source_account": TRANSACTION_REQUIRED_COLUMN_GROUPS[1],
    "destination_account": ("counterparty", "to_account", "to_iban", "beneficiary_account", "beneficiary_iban", "destination_account", "credit_account", "payee_account", "receiver_account", "receiver", "beneficiary", "merchant", "merchant_name"),
    "location": ("location", "merchant_location", "city"),
}

ACCESS_REQUIRED_COLUMN_GROUPS = (
    ("ip", "source_ip", "src_ip", "client_ip", "remote_addr", "remote_ip", "source_address"),
    ("user", "username", "user_id", "principal", "actor", "path", "url", "uri", "event_action"),
)
ACCESS_FIELD_ALIASES: dict[str, tuple[str, ...]] = {
    "observed_at": ("timestamp", "time", "event_time", "event_timestamp", "created_at", "@timestamp"),
    "source_ip": ACCESS_REQUIRED_COLUMN_GROUPS[0],
    "principal": ("user", "username", "user_id", "principal", "actor"),
    "resource": ("path", "url", "uri", "resource", "object"),
    "action": ("event_action", "action", "method", "http_method", "operation"),
    "event_status": ("status", "status_code", "http_status", "result", "outcome"),
    "user_agent": ("user_agent", "http_user_agent", "agent"),
}

IDENTIFIER_TYPES = {
    "msisdn", "call_org_num", "call_dialed_num", "imsi", "imei", "iccid",
    "subscriber_reference", "subscriber_identifier", "session_identifier", "site_identifier",
    "sector_identifier", "cell_site_id", "site_id", "lac_id", "lac", "tac", "cgi", "ecgi",
    "enodeb_id", "gnodeb_id", "record_reference", "reference_identifier", "plate", "camera_id",
    "transaction_reference", "source_account", "destination_account", "principal",
}
TIMESTAMP_TYPES = {"call_start_ts", "call_end_ts", "session_start", "session_end", "observed_at", "activation_at", "deactivation_at", "valid_from", "valid_to"}
INTEGER_TYPES = {"duration_seconds", "source_port", "destination_port", "bytes"}
DECIMAL_TYPES = {"network_volume", "latitude", "longitude", "confidence", "azimuth", "beamwidth", "uncertainty_radius_m", "amount"}

REGISTRY: dict[str, dict[str, Any]] = {
    "cdr": {"adapter_id": "nexusai.adapter.cdr", "family_id": "communications_cdr", "mapping_profile_id": "cdr-source-mapping/v2", "aliases": CDR_FIELD_ALIASES, "required_groups": CDR_REQUIRED_COLUMN_GROUPS, "time_fields": ["call_start_ts", "call_end_ts"]},
    "ipdr": {"adapter_id": "nexusai.adapter.ipdr", "family_id": "network_ipdr", "mapping_profile_id": "ipdr-source-mapping/v1", "aliases": IPDR_FIELD_ALIASES, "required_groups": IPDR_REQUIRED_COLUMN_GROUPS, "time_fields": ["session_start", "session_end"]},
    "subscriber": {"adapter_id": "nexusai.adapter.subscriber_identity", "family_id": "subscriber_identity", "mapping_profile_id": "subscriber-source-mapping/v1", "aliases": SUBSCRIBER_FIELD_ALIASES, "required_groups": SUBSCRIBER_REQUIRED_COLUMN_GROUPS, "time_fields": ["activation_at", "deactivation_at", "valid_from", "valid_to"]},
    "tower_location": {"adapter_id": "nexusai.adapter.tower_location", "family_id": "tower_location", "mapping_profile_id": "tower-source-mapping/v1", "aliases": TOWER_FIELD_ALIASES, "required_groups": TOWER_REQUIRED_COLUMN_GROUPS, "time_fields": ["valid_from", "valid_to"]},
    "anpr": {"adapter_id": "nexusai.adapter.anpr", "family_id": "anpr_vehicles", "mapping_profile_id": "anpr-source-mapping/v1", "aliases": ANPR_FIELD_ALIASES, "required_groups": ANPR_REQUIRED_COLUMN_GROUPS, "time_fields": ["observed_at"]},
    "generic": {"adapter_id": "nexusai.adapter.generic_tabular", "family_id": "generic_tabular", "mapping_profile_id": "generic-source-mapping/v1", "aliases": {}, "required_groups": (), "time_fields": []},
    "transaction": {"adapter_id": "nexusai.adapter.generic_tabular", "family_id": "financial_transactions", "mapping_profile_id": "transaction-source-mapping/v1", "aliases": TRANSACTION_FIELD_ALIASES, "required_groups": TRANSACTION_REQUIRED_COLUMN_GROUPS, "time_fields": ["observed_at"]},
    "access_log": {"adapter_id": "nexusai.adapter.generic_tabular", "family_id": "logs_access_security", "mapping_profile_id": "access-log-source-mapping/v1", "aliases": ACCESS_FIELD_ALIASES, "required_groups": ACCESS_REQUIRED_COLUMN_GROUPS, "time_fields": ["observed_at"]},
}

RECOGNIZED_EXTENSIONS = {
    "provider_event_class", "export_batch_label", "native_switch_code", "billing_zone",
    "source_system", "export_version", "provider_note", "switch_name",
}

# Values remain available in the immutable raw source payload, but these
# identity attributes must not be copied into presentation-oriented metadata.
PROTECTED_ATTRIBUTE_FIELDS = {
    "cnic", "subscriber_name", "msisdn", "imsi", "imei", "iccid",
    "subscriber_reference", "subscriber_identifier", "source_account",
    "destination_account", "principal",
}


def semantic_type(field: str | None) -> str:
    if not field:
        return "source_text"
    if field in IDENTIFIER_TYPES:
        return "string_identifier"
    if field in TIMESTAMP_TYPES:
        return "timestamp"
    if field in INTEGER_TYPES:
        return "integer"
    if field in DECIMAL_TYPES:
        return "decimal"
    if field in {"source_ip", "destination_ip", "nat_source_ip", "nat_destination_ip"}:
        return "ip_address"
    return "source_text"


def _value_pattern(value: str) -> str:
    if re.fullmatch(r"[+-]?\d+", value): return "integer_token"
    if re.fullmatch(r"[+-]?\d+\.\d+", value): return "decimal_token"
    if re.match(r"^\d{4}-\d{1,2}-\d{1,2}(?:[ T]|$)", value): return "iso_date_or_timestamp"
    if re.match(r"^\d{1,2}/\d{1,2}/\d{4}(?:[ T]|$)", value): return "slash_date_or_timestamp"
    if value.startswith("*") and value.endswith("#"): return "service_code"
    if re.fullmatch(r"\+?[\d\s().-]+", value): return "numeric_identifier"
    return "text_token"


def schema_profile(adapter_name: str, headers: Iterable[str], records: Iterable[Mapping[str, Any]], normalize_header: Callable[[str], str], clean_text: Callable[[Any], str]) -> dict[str, Any]:
    config = REGISTRY.get(adapter_name, REGISTRY["generic"])
    source_headers, sampled_records = list(headers), list(records)
    alias_index: dict[str, list[str]] = {}
    for canonical, aliases in config["aliases"].items():
        for alias in aliases:
            alias_index.setdefault(normalize_header(alias), []).append(canonical)
    fields = []
    for source_field in source_headers:
        normalized = normalize_header(source_field)
        candidates = sorted(set(alias_index.get(normalized, [])))
        values = [clean_text(next((value for key, value in record.items() if normalize_header(key) == normalized), "")) for record in sampled_records]
        non_empty = [value for value in values if value]
        if len(candidates) > 1:
            state, canonical = "ambiguous", None
        elif candidates:
            state, canonical = "canonical", candidates[0]
        elif normalized in RECOGNIZED_EXTENSIONS:
            state, canonical = "recognized_extension", None
        else:
            state, canonical = "unmapped", None
        null_count = len(sampled_records) - len(non_empty)
        fields.append({
            "source_field": source_field, "display_name": source_field, "normalized_name": normalized,
            "candidate_canonical_fields": candidates, "canonical_field": canonical, "mapping_state": state,
            "semantic_type": semantic_type(canonical), "inferred_type": semantic_type(canonical), "sampled_rows": len(sampled_records),
            "sampled_non_empty": len(non_empty), "sampled_null_count": null_count,
            "sampled_null_frequency": round(null_count / len(sampled_records), 6) if sampled_records else None,
            "sample_patterns": sorted({_value_pattern(value) for value in non_empty}),
        })
    normalized_headers = {normalize_header(header) for header in source_headers}
    groups = []
    for index, aliases in enumerate(config["required_groups"], start=1):
        matched = [header for header in source_headers if normalize_header(header) in {normalize_header(alias) for alias in aliases}]
        groups.append({"group": index, "accepted_aliases": list(aliases), "matched_source_fields": matched, "satisfied": bool(matched)})
    return {
        "contract_version": SCHEMA_PROFILE_VERSION, "registry_version": SCHEMA_REGISTRY_VERSION,
        "mapping_profile_id": config["mapping_profile_id"], "adapter_id": config["adapter_id"], "family_id": config["family_id"],
        "sampled_rows": len(sampled_records), "source_columns": fields, "required_groups": groups,
        "recognized_canonical_fields": sorted({field["canonical_field"] for field in fields if field["canonical_field"]}),
        "recognized_extension_fields": [field["source_field"] for field in fields if field["mapping_state"] == "recognized_extension"],
        "unmapped_fields": [field["source_field"] for field in fields if field["mapping_state"] == "unmapped"],
        "ambiguous_fields": [field["source_field"] for field in fields if field["mapping_state"] == "ambiguous"],
        "invalid_mappings": [],
        "review_required": any(not group["satisfied"] for group in groups) or any(field["mapping_state"] in {"ambiguous", "invalid"} for field in fields),
        "time_fields": list(config["time_fields"]),
        "preservation_policy": "All source columns remain in raw_record; preserved fields are not promoted to canonical semantics without a governed mapping.",
    }


def _json_value(value: Any) -> Any:
    if isinstance(value, datetime): return value.isoformat()
    if isinstance(value, Decimal): return str(value)
    return value


def _typed_value(raw_value: Any, kind: str) -> Any:
    if raw_value is None or (isinstance(raw_value, str) and not raw_value.strip()): return None
    text = str(raw_value).strip()
    if kind == "integer" and re.fullmatch(r"[+-]?\d+", text): return int(text)
    if kind == "decimal":
        try: return str(Decimal(text))
        except InvalidOperation: return text
    if kind == "boolean" and text.casefold() in {"true", "false"}: return text.casefold() == "true"
    return raw_value


def dynamic_attributes(raw: Mapping[str, Any], mapping: Mapping[str, Any], canonical: Mapping[str, Any], *, evidence_id: str, version_id: str, source_file: str, row_number: int) -> list[dict[str, Any]]:
    by_name = {str(field["normalized_name"]): field for field in mapping.get("source_columns", [])}
    attributes = []
    for source_field, raw_value in raw.items():
        normalized = re.sub(r"[^a-z0-9]+", "_", str(source_field).strip().casefold()).strip("_")
        field = by_name.get(normalized, {})
        canonical_field = field.get("canonical_field")
        kind = field.get("semantic_type") or "source_text"
        protected = canonical_field in PROTECTED_ATTRIBUTE_FIELDS
        mapping_state = field.get("mapping_state") or "unmapped"
        if mapping_state not in DYNAMIC_ATTRIBUTE_STATES:
            mapping_state = "invalid"
        attributes.append({
            "contract_version": DYNAMIC_ATTRIBUTE_VERSION,
            "source_field": source_field, "display_name": field.get("display_name") or source_field,
            "canonical_candidate": canonical_field, "raw_value": None if protected else raw_value,
            "typed_value": None if protected else _typed_value(raw_value, kind),
            "normalized_value": None if protected else (_json_value(canonical.get(canonical_field)) if canonical_field else None),
            "primitive_type": kind, "mapping_state": mapping_state,
            "value_visibility": "protected_source_only" if protected else "metadata_available",
            "mapping_profile_id": mapping.get("mapping_profile_id"), "evidence_id": evidence_id or None,
            "version_id": version_id or None,
            "source_locator": {"source_file": source_file, "row_number": row_number, "source_field": source_field},
        })
    return attributes


def quality_summary(mapping: Mapping[str, Any], stats: Mapping[str, Any], inserted: int) -> dict[str, Any]:
    total, rejected = int(stats.get("total_rows") or 0), int(stats.get("rejected_rows") or 0)
    duplicates = max(0, total - rejected - inserted)
    codes = dict(stats.get("rejection_codes") or {})
    time_blockers = sum(int(codes.get(code, 0)) for code in ("ambiguous_date_order", "unknown_source_timezone"))
    missing_time = int(stats.get("missing_time_rows") or 0)
    ambiguous = len(mapping.get("ambiguous_fields") or [])
    invalid = len(mapping.get("invalid_mappings") or [])
    unmapped = len(mapping.get("unmapped_fields") or [])
    extensions = len(mapping.get("recognized_extension_fields") or [])
    if total and inserted == 0:
        state = "failed"
    elif time_blockers or ambiguous or invalid:
        state = "needs_review"
    elif rejected or unmapped or extensions:
        state = "ready_with_warnings"
    else:
        state = "ready"
    family = str(mapping.get("family_id") or "generic_tabular")
    time_status = "unavailable" if time_blockers else "available"
    capabilities = [
        {"id": "record_inspection", "label": "Record inspection", "status": "available" if inserted else "unavailable", "reason": "Canonical records are available." if inserted else "No canonical rows were accepted."},
    ]
    if family == "communications_cdr":
        capabilities.extend([
            {"id": "frequent_contacts", "label": "Frequent contacts", "status": "available" if inserted else "unavailable", "reason": "Accepted party fields are available." if inserted else "No accepted CDR rows are available."},
            {"id": "temporal_activity", "label": "Temporal activity", "status": time_status if inserted else "unavailable", "reason": "Accepted timestamps are available." if inserted and not time_blockers else "Date/time review is required before this source supports temporal analysis."},
        ])
    else:
        family_capability = {
            "network_ipdr": ("session_analysis", "Session analysis"),
            "subscriber_identity": ("identity_lookup", "Identity lookup"),
            "tower_location": ("site_lookup", "Site lookup"),
            "anpr_vehicles": ("vehicle_sightings", "Vehicle sightings"),
            "financial_transactions": ("transaction_inspection", "Transaction inspection"),
            "logs_access_security": ("access_event_inspection", "Access-event inspection"),
            "generic_tabular": ("schema_filtering", "Schema-aware filtering"),
        }.get(family)
        if family_capability:
            capabilities.append({
                "id": family_capability[0], "label": family_capability[1],
                "status": "limited" if family in {"financial_transactions", "logs_access_security", "generic_tabular"} and inserted else ("available" if inserted else "unavailable"),
                "reason": "Common structured infrastructure is available; family analytical semantics remain partial." if family in {"financial_transactions", "logs_access_security", "generic_tabular"} and inserted else ("Accepted canonical family records are available." if inserted else "No accepted family records are available."),
            })
        if mapping.get("time_fields"):
            if not inserted or time_blockers or missing_time >= inserted:
                temporal_status = "unavailable"
            elif missing_time:
                temporal_status = "limited"
            else:
                temporal_status = "available"
            capabilities.append({
                "id": "temporal_analysis", "label": "Temporal analysis", "status": temporal_status,
                "reason": "Accepted source timestamps are available." if temporal_status == "available" else ("Some accepted records have unresolved source time." if temporal_status == "limited" else "Source time is unavailable or requires review."),
            })
    return {
        "contract_version": QUALITY_CONTRACT_VERSION, "state": state,
        "counts": {"input_records": total, "accepted_records": inserted, "rejected_records": rejected,
                   "duplicate_records": duplicates, "missing_time_records": missing_time,
                   "ambiguous_fields": ambiguous, "invalid_fields": invalid, "unmapped_fields": unmapped,
                   "preserved_extension_fields": extensions, "canonical_records": inserted},
        "rejection_codes": codes, "capability_readiness": capabilities,
    }


def registry_snapshot() -> dict[str, Any]:
    return {"contract_version": SCHEMA_REGISTRY_VERSION, "dynamic_attribute_states": sorted(DYNAMIC_ATTRIBUTE_STATES), "families": [
        {"adapter_name": name, "adapter_id": config["adapter_id"], "family_id": config["family_id"],
         "mapping_profile_id": config["mapping_profile_id"], "required_concepts": [list(group) for group in config["required_groups"]],
         "canonical_fields": [{
             "name": field,
             "semantic_type": semantic_type(field),
             "identifier_type": field if field in IDENTIFIER_TYPES else None,
             "normalization": "family_adapter_versioned",
             "provenance_required": True,
             "quality_implication": "required_concept" if any(
                 set(config["aliases"][field]).intersection(group) for group in config["required_groups"]
             ) else "optional",
         } for field in config["aliases"]],
         "time_fields": list(config["time_fields"]), "status": "operational" if name in {"cdr", "ipdr", "subscriber", "tower_location", "anpr"} else "baseline"}
        for name, config in REGISTRY.items()
    ]}
