"""Dedicated subscriber-identity adapter."""

from .adapter import (
    FIELD_ALIASES,
    SUBSCRIBER_REQUIRED_COLUMN_GROUPS,
    SubscriberAdapterRuntime,
    adapter_capabilities,
    detect_schema,
    load_manifest,
    normalize_record,
    validate_headers,
    verify_normalized_record,
)

__all__ = [
    "FIELD_ALIASES",
    "SUBSCRIBER_REQUIRED_COLUMN_GROUPS",
    "SubscriberAdapterRuntime",
    "adapter_capabilities",
    "detect_schema",
    "load_manifest",
    "normalize_record",
    "validate_headers",
    "verify_normalized_record",
]
