"""Dedicated ANPR and vehicle-sighting adapter."""

from .adapter import (
    ANPRAdapterRuntime,
    ANPR_REQUIRED_COLUMN_GROUPS,
    FIELD_ALIASES,
    adapter_capabilities,
    detect_schema,
    load_manifest,
    normalize_record,
    validate_headers,
    verify_normalized_record,
)

__all__ = [
    "ANPRAdapterRuntime",
    "ANPR_REQUIRED_COLUMN_GROUPS",
    "FIELD_ALIASES",
    "adapter_capabilities",
    "detect_schema",
    "load_manifest",
    "normalize_record",
    "validate_headers",
    "verify_normalized_record",
]
