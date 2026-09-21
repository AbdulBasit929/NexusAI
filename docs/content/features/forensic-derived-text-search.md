---
title: "Derived text search semantics"
---

NX-B2.1B-1 adds an optional `text_query` to the existing governed forensic query
request. This is a source-validated contract; the accepted runtime still runs
the earlier activation until a separately approved deployment.

```json
{
  "collection_id": "authorized-case",
  "query": "Find this transcript phrase \"clear phrase\"",
  "template": "audio_transcript_search",
  "text_query": {
    "literal_text": "clear phrase",
    "match_semantic": "PHRASE_CONTAINS",
    "text_representation": "raw"
  },
  "max_kb_results": 3
}
```

The existing authenticated tenant, case, selected evidence, current version,
capability, operation, and source-time checks still apply. Quoted text is
extracted before family selection. An explicit transcript, OCR/image, or
document hint selects the corresponding existing operation; selected-source
context can resolve a missing hint. An unresolved family or multiple literals
requires clarification. Quoted evidence text does not supply routing keywords.

| Semantic | Meaning |
|---|---|
| `EXACT_VALUE` | Equality with the complete stored text value; no substring behavior. Existing structured identifier normalizers remain authoritative for identifier operations. |
| `EXACT_PHRASE` | Contiguous, case-sensitive phrase with Unicode word boundaries, after conservative normalization. |
| `PHRASE_CONTAINS` | Case-sensitive normalized substring, including inside-word spans; at least two letters/numbers and at most 4,096 code points. |
| `TOKEN_SEARCH` | Every whitespace-delimited query token must occur as a bounded whole token; order is not required. |
| `SOURCE` | Retrieve available source observations without a literal filter. |
| `TIME_RANGE` | Apply the existing inclusive source-time intersection behavior. Requires supplied source-time bounds. |
| `SOURCE_TIME` | Retrieve observations with their reported source times. |

The new phrase policy, `nfc-whitespace/v1`, applies Unicode NFC and collapses
Unicode whitespace, including line breaks. Case, punctuation, diacritics,
letters, and joiners remain significant. Raw observations remain unchanged and
are returned separately from display previews. No translation, spelling repair,
embedding, or model rewrite participates in literal matching. `raw` and
`roman_derivative` are optional representation selectors; Roman Urdu retains
its parent artifact and derived status. A normalized OCR fallback with no raw
observation cannot establish an explicit raw-text match or exhaustive absence.

Existing `transcript_mode: "exact"` callers retain the legacy NFKC, lowercase,
punctuation-to-space, token-boundary containment policy. OCR's existing
`exact_term` shorthand retains that compatibility policy. A supplied audio or
document `exact_term` without a classified legacy mode requires clarification
instead of returning unfiltered source text. Explicit `text_query` takes
precedence; new agent-originated quoted searches carry this typed contract.

Transcript phrase searches inspect individual observations and windows of two
through four segments. Windows share tenant, case, evidence, current version,
processing run, representation, and language lineage. The current worker has
no usable segment ordinal, so only touching, non-overlapping reported time
ranges prove adjacency. Missing times, overlaps, or gaps leave cross-segment
coverage incomplete. All original contributors and locators remain available;
the result records the full observed range. A row is a supporting observation,
not an occurrence count. Repeated occurrences within one observation are not
enumerated as separate rows.

Retrieval uses one read-only, repeatable-read transaction under the tenant
policy, with stable artifact-ID pagination and 500-row pages plus lookahead.
The limits are 20 pages / 10,000 candidates, 16 MiB of candidate text admitted for matching (accounted after each SQL page), and a
five-second retrieval timeout. These are execution budgets, not a product SLA.
No schema migration or new index is required.

`search_completeness` is `EXHAUSTIVE`, `TRUNCATED`, `INCOMPLETE`, or `UNKNOWN`.
Exhaustiveness refers to current completed observations and admitted segment
windows, not arbitrary-length speech or unprocessed evidence. Positive matches
can remain useful under `SEARCH_INCOMPLETE` or `RESULTS_TRUNCATED`; zero matches
under those states do not establish absence. Existing enterprise state names
remain unchanged: incomplete positives map to results-present with limitations;
incomplete zero maps to unavailable with limitations. The optional tool-result
metadata preserves completeness across serialization and Activity reopening.

Native extracted passages own document literal search through `document_search`.
The semantic KB path retains its existing contract. Literal results do not fall
back to unrelated KB passages. These contracts do not promote operation or
product certification, change suggestion eligibility, or enable TTS.
