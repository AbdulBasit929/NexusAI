# Certification and independent oracles

For every important operation define ID, family, purpose, required/optional parameters, identifier and normalization assumptions, time/direction/filter semantics, duplicate/null/sentinel behavior, grouping, aggregation, sorting/ranking/ties/limits, result meaning and limits, citation contract, and presentation contract.

Production output cannot certify itself. Use a hand-auditable fixture, independently written reference calculation or read-only SQL, or explicit mathematical result. Do not reuse the production helper under test.

Compare expected and actual operation, parameters, rows, aggregates, order, relationships, citations, and presentation semantics. Plausible output and registry parity are not certification.

## Test pyramid

- L0 parser primitives
- L1 schema mapping
- L2 canonicalization
- L3 family analytical goldens
- L4 multi-source goldens
- L5 query semantic equivalence
- L6 citation and presentation
- L7 authorized live/sanitized acceptance

Parser changes require format fixtures, canonical equivalence, and affected analytical goldens. Normalizer changes require family, multi-source, and cross-family regression. Operation changes require an independent golden and invariants. Query changes require English, Roman Urdu, Urdu, and follow-up cases. Shared renderers require every affected result type.
