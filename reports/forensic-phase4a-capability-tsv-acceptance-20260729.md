# Phase 4A acceptance — capability-aware querying and TSV depth

## Outcome

Phase 4 has started in source without rebuilding or changing retained services.
The application now exposes a collection-aware truth map for all 20 required
data families, blocks queries that require unaccepted adapters/models, supports
streaming TSV ingestion with messy encoding/header preservation, and exposes
capability/reprocessing controls in Records Intelligence.

## Delivered

- `GET /query/capabilities` on the sidecar and
  `GET /api/records/forensic/capabilities` through the authorized LocalAI proxy.
- Twenty catalog families checked against the machine-readable modality matrix.
- Dynamic states: `queryable`, `semantic_only`, `registered_pending`, `no_data`,
  and `manual_review`, with indexed/evidence/KB/normalized counts and reasons.
- A query capability guard that returns `unavailable` before SQL, KB retrieval or
  model synthesis for pending OCR, STT/diarization, video analysis, TTS, capture
  reconstruction, database querying, archive extraction, spreadsheet extraction
  and document extraction.
- Safe metadata/inventory routes remain usable for registered evidence.
- React capability map for all families, collection counts, formats, reasons and
  suggested queries.
- UI evidence reprocessing using immutable generations and idempotency, with
  attempt/max-attempt/generation visibility.
- TSV routing to the records worker and a streaming delimiter adapter supporting
  UTF-8/BOM, UTF-16 BOM/heuristics and CP1252 fallback, comma/tab/semicolon/pipe
  detection, duplicate/blank header disambiguation, overflow columns, exact raw
  strings, Urdu text, leading-zero identifiers and decimal precision.

## Acceptance evidence

- Python worker/adapters: 33 tests passed in 1.117 seconds.
- Full forensic Go package: passed in 15.035 seconds test time (27.8 seconds wall).
- Six focused capability contract tests pass, including matrix parity, pending
  operation guards, accepted routes, collection availability and pre-runtime
  blocking with zero DB/KB/LLM latency.
- React ESLint: zero errors; six existing warnings reported.
- Python compilation and modality JSON parsing: pass.
- `git diff --check`: no whitespace error; line-ending notices only.

## Boundaries

- No LocalAI/Docker image rebuild, container recreate, migration, retained DB
  write, NATS activation, model download/call/promotion, sensitive evidence
  ingestion, Git staging, commit, push or PR.
- XLSX and deeper columnar formats remain pending. TSV is the first accepted
  Phase 4 format slice.
- LocalAI's large endpoint package remains a CI/high-memory compile gate from
  Phase 3; the new proxy source is wired but not claimed as rebuilt/deployed.
- Capability support means the route exists and the collection has relevant
  processed data; it never means every possible real-world question has an
  answer. Missing data, ambiguity and unsupported processing return explicit
  limitations or clarification.

## Next slice

Implement XLSX read-only inventory and row extraction with exact multi-sheet,
formula/value, merged-cell, hidden-row/column, date-system, error-cell and
precision fixtures. Then deepen CDR/IPDR/ANPR/tower/subscriber/finance/log query
templates and cross-family correlation goldens before document extraction.

