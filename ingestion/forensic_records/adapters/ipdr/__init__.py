"""Network IPDR adapter public surface."""

from .adapter import (
    IPDRAdapterRuntime,
    IPDR_REQUIRED_COLUMN_GROUPS,
    adapter_capabilities,
    detect_schema,
    load_manifest,
    normalize_record,
    validate_headers,
    verify_normalized_record,
)

__all__ = [
    "IPDRAdapterRuntime",
    "IPDR_REQUIRED_COLUMN_GROUPS",
    "adapter_capabilities",
    "detect_schema",
    "load_manifest",
    "normalize_record",
    "validate_headers",
    "verify_normalized_record",
]
