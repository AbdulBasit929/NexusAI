# Dynamic and source-native attributes

Keep canonical analytical fields separate from legitimate provider/source-native attributes. Do not drop extras and do not promote unknown fields into canonical meaning automatically.

Where the current architecture permits, retain original and display names, raw value, typed value, normalized value only when defined, evidence/version and source locator, mapping state, and mapping profile/version.

Use distinct states: `canonical`, `recognized_extension`, `unmapped`, `ambiguous`, and `invalid`. In analyst UI, present equivalent friendly states such as Mapped, Preserved, Needs Review, and Invalid.

Extend existing JSON/metadata/contract storage before considering a new mechanism. Do not add a database migration without explicit approval. Test that extension fields survive ingest, persistence, retrieval, citation detail, reprocess compatibility, and export where supported.
