"""Communications CDR adapter public surface."""

from .adapter import (
    CDRAdapterRuntime,
    CDR_REQUIRED_COLUMN_GROUPS,
    adapter_capabilities,
    detect_schema,
    load_manifest,
    normalize_record,
    schema_profile,
    validate_headers,
    verify_normalized_record,
)

__all__ = [
    "CDRAdapterRuntime",
    "CDR_REQUIRED_COLUMN_GROUPS",
    "adapter_capabilities",
    "detect_schema",
    "load_manifest",
    "normalize_record",
    "schema_profile",
    "validate_headers",
    "verify_normalized_record",
]
