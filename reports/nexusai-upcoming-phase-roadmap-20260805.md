# NexusAI upcoming phase roadmap

Date: 2026-08-05  
Baseline: Phase 7.2 Agent Chat acceptance and reliability closure

The roadmap continues through bounded, tested vertical slices. Every phase must
preserve source hashes, provenance, tenant/case scope, exact deterministic
calculations, explicit uncertainty, and safe model fallback.

## Answer-quality contract for every phase

Every new adapter and query must ship with a distinct analyst-facing answer,
not a raw schema dump. The default response must contain a direct answer,
relevant key results, a short plain-language model interpretation, important
limitations, and collapsed source traceability. Internal template names,
planner confidence, routes, raw field names, and large previews stay out of the
main answer. Exact facts are computed first and remain authoritative; model
prose is validated, bounded, and safely replaceable by deterministic fallback.

## Phase 7.3 - Agent Chat contract and observability hardening

- Runtime-accepted foundation: the professional answer presenter now provides
  specific titles for all operations, humanized tables, five-row previews,
  automatic bounded explanation, and collapsed audit detail. The 65-operation
  presentation matrix and six-agent model matrix pass.
- Add gold expected-answer assertions for all templates, covering factual
  usefulness as well as route selection.
- Formalize typed chat envelopes for started, status, tool, completion, error,
  cancellation, and timeout events.
- Add cancel/retry UI actions, elapsed-time display, and a visible deterministic
  versus model-assist execution badge.
- Persist request route, template, latency, timeout, fallback reason, model
  policy decision, and collection/case identifiers.
- Add reconnect/replay and concurrent-request tests.

Exit gate: no orphaned `Working...` state across refresh, reconnect, timeout,
cancel, duplicate submit, or concurrent chats; every answer is traceable to one
request and one collection. All templates also pass answer-quality review with
no internal diagnostic sections in the default view.

## Phase 7.4 - Pakistan CDR and tower-reference deepening

- Add time-aware CDR-to-sector joins using exact supplied site/sector aliases,
  validity intervals, datum, azimuth, technology, and uncertainty.
- Preserve unmatched and ambiguous joins as first-class results.
- Add contact-pair timelines, identifier-change review, cell-handover sequences,
  temporal overlap, and deterministic data-quality reconciliation.
- Improve tables and maps without inferring RF coverage, handset position,
  ownership, or continuous movement.

Exit gate: golden Pakistan fixtures, SQL reconciliation, provenance, specialist
Agent Chat matrix, and browser workspace acceptance all pass.

## Phase 8 - Documents and OCR evidence

- Ingest PDF, DOCX, text, scanned pages, and common office exports.
- Add English/Urdu OCR, page coordinates, language confidence, redaction-safe
  excerpts, and page-level citations.
- Support document timelines, exact phrase search, and cross-document
  contradiction review.

Exit gate: answers cite file hash, page, and region; OCR uncertainty is visible;
exact claims never rely on model memory. Document answers use document-specific
summaries and do not reuse structured-record presentation.

## Phase 9 - Audio and speech evidence

- Add media integrity metadata, channel separation, bounded transcription,
  English/Urdu handling, timestamps, and speaker-label placeholders.
- Support transcript search and cited clip ranges.
- Do not claim speaker identity, intent, emotion, or authenticity without an
  approved deterministic method and explicit evidence.

Exit gate: reproducible transcripts with time-coded citations and safe failure
for unsupported codecs or low-confidence segments. Audio answers prioritize a
readable transcript excerpt and cited time ranges rather than decoder metadata.

## Phase 10 - Image, video, and ANPR visual evidence

- Add frame extraction, thumbnails, time normalization, camera metadata, visual
  evidence linking, and human-review queues.
- Connect ANPR observations to the original frame and supplied confidence.
- Treat detections as observations, not proof of driver, owner, association, or
  continuous route.

Exit gate: every visual claim opens the exact source frame with hash, timestamp,
detector/version, confidence, and review status. Visual answers use observation
cards and timelines rather than generic key/value output.

## Phase 11 - PCAP, databases, archives, and device exports

- Deepen PCAP/PCAPNG, SQLite, archive, JSON/XML, database dump, and device-export
  adapters with decompression limits and content-addressed retention.
- Add protocol/session summaries, schema discovery, nested lineage, and
  extraction manifests.

Exit gate: hostile or oversized inputs fail safely; extracted children retain
an unbroken parent hash and provenance chain. Network/database answers use
family-specific summaries and bounded tables.

## Phase 12 - Cross-family graph and hypothesis workspace

- Build temporal entity graphs over CDR, IPDR, ANPR, subscriber, tower,
  documents, media, and extracted artifacts.
- Separate observation, deterministic relationship, analyst hypothesis, and
  model suggestion as distinct typed objects.
- Add contradiction, alternative-explanation, and evidence-gap views.

Exit gate: graph edges are reproducible from cited evidence; deleting or
superseding evidence invalidates dependent results deterministically. Model
reasoning remains explicitly separated from observed and computed graph facts.

## Phase 13 - Public API and integration productization

- Stabilize versioned forensic APIs, OpenAPI schemas, idempotency, pagination,
  exports, webhooks, and service-account scopes.
- Keep REST, MCP tools, capability discovery, authorization, UI, and docs in
  sync.

Exit gate: contract, authorization, rate-limit, compatibility, and end-to-end
client tests pass.

## Phase 14 - Production hardening and release readiness

- Complete threat modeling, secrets rotation, audit immutability, backup and
  point-in-time recovery, disaster drills, retention/legal-hold controls, and
  dependency/image scanning.
- Establish representative correctness, latency, memory, and concurrency
  benchmarks with regression budgets.
- Produce operator, analyst, incident-response, and evidence-export runbooks.

Exit gate: recovery drill, security review, scale benchmark, upgrade/rollback,
and signed release acceptance pass without weakening forensic guardrails.
