# Ingestion and normalization contract

Trace and preserve: upload/admission, immutable source/version, signature and container detection, family classification, schema profiling, mapping profile, canonicalization, validation, duplicate handling, rejected-row handling, canonical persistence, source-native attributes, quality metrics, indexing, and capability readiness.

## Schema profile

Profile source columns, inferred primitive types, null frequency, sample patterns, identifier/timestamp/direction/duration candidates, recognized mappings and extensions, unmapped fields, ambiguity, and invalid mappings. Browser inference may assist UX; server interpretation is authoritative.

## Reproducibility and provenance

Retain source and canonical values plus evidence/version and source locator, classifier version, adapter ID/version, mapping profile/version, normalizer version, and processing run. Never permanently replace source meaning.

## Time

Define source date/time/timestamp/timezone/offset, naive-time policy, normalized timestamp, analysis timezone, inclusive/exclusive bounds, day boundaries, midnight, and cross-day behavior. Fail safely when assumptions are material and unknown.

## Quality accounting

Truthfully report input, accepted, duplicate, rejected, invalid, unmapped, ambiguous, canonical, and processing states where applicable. `unknown != zero`. Reconcile accounting deterministically and expose warnings without presenting optional unmapped fields as total failure.

For STIM-1, prove multiple provider/export schemas map to equivalent canonical events while retaining distinct source provenance and extensions. Include alternate aliases/order/delimiters, extras, missing optional/required fields, null/blank, duplicates, zero duration, sentinels, direction/date variants, invalid times, casing/whitespace, string identifiers, extensions, and unknown fields.
