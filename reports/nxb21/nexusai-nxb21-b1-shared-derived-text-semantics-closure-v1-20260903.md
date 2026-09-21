# NX-B2.1B-1 shared derived-text source closure

Status: **SOURCE VALIDATED — NOT DEPLOYED — NO CERTIFICATION PROMOTION**.

## Verified Starting State

Reused the completed A reconciliation, then verified the runtime read-only at 2026-09-03T10:44:52Z. Branch `codex/forensic-hybrid-checkpoint-20260723`, HEAD `40717b83510c08db25dc26b9d6674bf46db363ac`. Receipt `20260903T063719719Z` remains the accepted activation. All five container IDs/images/restarts match A; restart counts are 0/0/9/0/0. Health: API 200, forensic API ok. Retained tuple `63|63|76|22507|827|61`, jobs 71 completed/5 dead-letter/0 active, case 35 completed/1 failed. Five installed models unchanged.

## B2.1A Inputs Reused

Reused the A reconciliation report, inventory, 14-case private probes, design artifact, historical routing findings and independent Urdu/English same-run SQL oracles. No repeat reconciliation, retained Ask, Activity rewrite, activation rerun or inference was performed. Canonical counts remain 79 executable operations, 104 discovery descriptors, 214 variants, nine adapters, six live agents, 16 manifests and 20 categories. Certification remains REGISTERED 62 / SOURCE_VALIDATED 12 / FIXTURE_CERTIFIED 1 / PRODUCT_CERTIFIED 4.

## Production Source Changed

Yes. One shared `pkg/forensictext` contract is consumed by the existing agent bridge and forensic API. Existing routing, execution parameters, derived-artifact retrieval, completeness, typed tool-result validation and citation adapters were extended. Twelve production files changed or were added. Worker, models, frontend calculations and executable/certification registries were not changed.

## Files Changed

The source review contains 11 modified existing files, six new Go files and one feature document. The manifest records before/after hashes, all 435 bound files and exact service ownership. Additional changes are the requested closure artifacts and living checkpoint/directive/sole roadmap/sole ledger/maturity issue register/design gap matrix. Existing user changes were preserved.

- `api/forensic_records/derived_text_query.go` — modified
- `api/forensic_records/four_p1_db_test.go` — modified
- `api/forensic_records/nxa1_execution_foundation.go` — modified
- `api/forensic_records/platform_contracts.go` — modified
- `api/forensic_records/query.go` — modified
- `api/forensic_records/query_intelligence_composition.go` — modified
- `api/forensic_records/query_intelligence_contracts.go` — modified
- `api/forensic_records/query_intelligence_planner.go` — modified
- `api/forensic_records/query_language_assistance.go` — modified
- `core/services/agents/forensic_direct.go` — modified
- `core/services/agents/records_tools.go` — modified
- `pkg/forensictext/query.go` — new
- `api/forensic_records/derived_text_semantics.go` — new
- `api/forensic_records/b1_derived_text_ginkgo_test.go` — new
- `api/forensic_records/b1_derived_text_db_ginkgo_test.go` — new
- `api/forensic_records/b1_private_replay_ginkgo_test.go` — new
- `core/services/agents/b1_literal_routing_test.go` — new
- `docs/content/features/forensic-derived-text-search.md` — new

## Literal Routing Root Cause

A established a pre-retrieval dependency error: literal extraction ran only after a transcript template had been chosen. Historical short quoted requests reached needs_input with no operation and no retrieval; the same literal matched when the operation was supplied. Whole-transcript equality was not the cause. The historical ~60-second durations are consistent with assistance budgeting, but do not prove a specific timeout mechanism.

## Literal Routing Correction

Quoted spans are extracted before operation hints. Routing uses words outside the literal plus explicit/selected-source family context. This prevents evidence text such as operation names from steering its own route. Audio, OCR and native documents map to their existing registered operations. Unresolved families and multiple literals clarify; structured identifier operations retain their existing routes.

## Deterministic Literal Fast Path

Obvious quoted requests populate `text_query` before planner selection. They skip language assistance and literal narrative synthesis. Natural and typed literals use the same executor contract. English and historical-style Urdu agent/source tests pass, preserve exact user bytes, and assert routing finishes within one second; no product SLA is established.

## Planner Schema Changes

The existing request, execution-parameter and language-proposal structures gained optional `text_query`: `literal_text`, allowlisted `match_semantic`, and allowlisted `text_representation`. Existing scope, language and source-time fields remain authoritative. QueryPlan filters and durable understanding preserve the text semantic. Unknown proposal fields are rejected. Model proposals cannot alter scope or rewrite literal spans.

## Match Semantics Implemented

The seven admitted semantics are distinct. New explicit contracts are case-sensitive and conservative. A result row represents a supporting observation, not the number of substring occurrences within it.

### EXACT_VALUE

Equality with the complete normalized text field. It never becomes substring search. Phone, plate, UUID, IP and structured-ID regressions confirm prefix-containing values are not equal; existing field-specific structured normalizers remain untouched.

### EXACT_PHRASE

Contiguous conservative-normalized phrase with Unicode letter/number/mark/joiner word boundaries. It is stricter than inside-word containment and different from whole-field equality.

### PHRASE_CONTAINS

Literal normalized Unicode substring, including inside-token spans. Two letters/numbers minimum prevents one-character and punctuation-only scans; one-character rejection, two-letter Urdu acceptance and punctuation-only rejection are tested. Maximum literal length is 4,096 code points.

### TOKEN_SEARCH

Implemented as all whitespace-delimited query tokens occurring with whole-token boundaries, without requiring token order. It is case-sensitive and performs no stemming, paraphrase, fuzzy repair or embedding.

### Legacy Exact Compatibility

Existing `transcript_mode=exact` remains NFKC + lowercase + punctuation-to-space + whitespace collapse + token-boundary containment. OCR exact_term shorthand retains that policy. Confirmed adjacent windows extend retrieval without reinterpreting its matcher. New quoted agent requests explicitly carry the new contract. Missing/unclassified audio/document exact_term is now held rather than dumped as SOURCE. The audit covered forensic_direct, records_tools, execution parameters, direct API callers, existing transcript/OCR tests and the A diagnostic script. Existing inside-word legacy rejection remains intentional.

## Search Normalization V1

`nfc-whitespace/v1` uses NFC and Unicode whitespace/line-break collapse. It preserves case, punctuation, Urdu/Arabic letters, diacritics and joiners. No Urdu letter-variant mapping was introduced. Raw values are immutable; search policy and representation are recorded separately. UTF-8-safe previews retain the complete raw observation in metadata. Legacy normalized-only OCR fallback is labelled as such; it is not relabelled raw or admitted as explicit raw-text proof.

## Urdu Partial-Query Result

PASS_SOURCE for routing, within-observation containment, inside-word policy, positive/absent synthetic cases and frozen current-artifact targets. The literal is unchanged. Original live runtime behavior has not been replaced.

## English Partial-Query Result

PASS_SOURCE using the identical contract, independent synthetic expectations and frozen English source targets. No English-only or Urdu-only branch implements phrase matching.

## Cross-Segment Search

Bounded windows contain two, three or four observations. Matching preserves every contributing raw observation and locator, with full observed range and a truthful incomplete marker when adjacency is unproven. Phrases beyond four segments are outside this initial window contract.

### Ordering

Current artifacts expose reported source times, not a usable segment ordinal. Source-time ordering is used for windows and supported transcript output. Candidate pagination uses artifact IDs only for stable enumeration, never for source adjacency.

### Adjacency

Only touching (within one microsecond), valid positive-duration, non-overlapping ranges prove adjacency. Missing timing, overlaps including a competing third segment, and gaps cannot produce invented joins. No ordinal is fabricated from UUIDs or creation time.

### Representation Isolation

Grouping keys include tenant, collection/case, evidence, current version, processing run, artifact representation and language lineage. Raw and Roman derivative artifacts never concatenate into one raw phrase.

### Urdu Result

Two-segment Urdu passes the frozen A artifact oracle and an independent PostgreSQL string_agg/position oracle under actual production retrieval SQL. All supporting citations remain available even with a one-result request budget.

### English Result

Two-segment English passes the frozen A artifact/independent SQL evidence and synthetic tests. Three- and four-segment English fixtures also pass the shared executor.

### Negative Cases

Wrong tenant, case, evidence, version, run, representation and language cannot join. Missing/ambiguous timing and excluded source-time contributors cannot manufacture a phrase. Absent phrases return truthful zero only when the admitted population/windows were exhaustively checked.

## OCR Literal Regression

Selected and workspace OCR positives, absent literals, raw metadata, bbox/page locators and legacy normalized fallback pass. Explicit raw-text search treats missing raw OCR authority as incomplete. It does not substitute a document or KB passage.

## Document Literal Result

PASS_SOURCE for current native extracted blocks through document_search, including positive, absent, equality/phrase/contains/token and paginated fixtures. Literal-mode omission no longer returns unrelated passages. Cross-page/block joining, additional decoders, broad semantic QA quality and RAG redesign are outside this slice. Existing semantic KB behavior is unchanged.

## Workspace vs Selected Scope

Selected-source readiness is consulted only for selected-scope empty eligible results. Workspace retrieval evaluates current authorized artifacts without inheriting a selected-source NOT_RUN state. The isolated DB proves workspace audio/OCR positives and exhaustive audio absence.

## Result-State Correction

Nonempty valid workspace/selected results preserve results-present semantics. Selected authoritative not-run, failed execution and exhaustive zero remain distinct. Missing database execution cannot become analytical no-match. Incomplete positives retain supported observations and limitations; incomplete zero is unavailable at the existing enterprise layer, not no-match.

## Search Completeness

Evidence and typed tool results carry EXHAUSTIVE, TRUNCATED, INCOMPLETE or UNKNOWN. SEARCH_INCOMPLETE and RESULTS_TRUNCATED are evidence details mapped to existing enterprise states and limitation fields, preserving old Activity payload compatibility. No new competing enterprise state vocabulary was introduced.

## Candidate Pagination / Cap Handling

A single read-only repeatable-read transaction sets the existing tenant context and retrieves 500 candidates plus lookahead per page, ordered by artifact ID. Budgets: 20 pages / 10,000 candidates, 16 MiB text admitted for matching after page retrieval, five-second retrieval timeout and bounded output. Existing tenant/case/evidence/version/artifact index and artifact primary key are reused. Selected predicates aid lookup; workspace scans can still exhaust budgets, which is explicit. No index or schema migration.

## No-Match Exhaustiveness

Real DB tests find a target beyond the first page, exhaust a 510-candidate negative, and protect zero/positive answers at the 10,000-candidate ceiling. Exhaustiveness refers to completed current observations and admitted windows, not unprocessed evidence or arbitrarily long speech. The frozen artifact replay is not a new live whole-case absence certification.

## Result Validator Changes

Extended ToolResultV1 validation for typed semantic, tenant/case/evidence/version, operation, run, representation, literal support, source-time bounds, contributing citations, row accounting, completeness and contradictory states. Derived-text tool rows now contain actual supporting observations. Audio/OCR remain model observations. Missing contributing citations, changed scope or failed/zero states containing rows are rejected; an LLM cannot repair them.

## Citation / Provenance Contract

Results retain artifact/observation IDs, complete raw text, parent artifact, representation, run, evidence/version, original locator and window range. Every contributing locator is carried through enterprise provenance and optional typed CitationV1 artifact/run fields. Citation capacity accounts for the bounded contributing set. The existing typed adapter no longer drops ASR locators. Single-source and cross-segment serialization/ownership tests pass.

## Roman Urdu Lineage

Explicit roman_derivative searches retain their raw-parent artifact reference and derivative representation. Urdu-script literal results remain raw-first; a language hint cannot change evidence authority or rewrite text.

## Current-Version Isolation

Actual SQL joins artifacts to evidence_items.current_version_id. Stale fixtures cannot match; windows never combine versions. Selected version filters remain additional checks.

## Cross-Case Isolation

No leakage in the tested bounded scope. Real PostgreSQL SELECT executes under a NOSUPERUSER/NOBYPASSRLS test role with the runtime tenant-policy shape; a direct query for another tenant returns zero. Same-ID foreign-case/tenant observations and stale versions are excluded. This is not full enterprise RBAC certification.

## LLM Role

The configured Qwen role is unchanged. It may propose an allowlisted operation and typed intent for genuinely unresolved language. Obvious literal routes and literal execution do not invoke it. No model load, replacement, tuning, download or inference-quality work occurred.

## LLM Literal Preservation

Proposed literal_text must be an exact substring of the original user question, and an existing explicit TextQuery cannot be changed. An Urdu word-order rewrite and unknown nested proposal field are rejected in tests. Scope is absent from the model proposal.

## Performance Results

Full forensic package: 102.994 s; full agent package: 79.387 s. Final focused source/DB/artifact/video suite: 63 specs in 2.427 s (17.187 s Go invocation). Independent DB Urdu cross-segment query: 27 ms. Latest frozen matcher replay: maximum 99 microseconds; sub-clock-resolution readings may appear as zero. Routing tests assert <1 s and no assistance call. These are local diagnostic measurements, not a product SLA.

## Private 14-Case Replay

14/14 frozen target cases PASS with explicit PHRASE_CONTAINS and unchanged literals. Original request contracts are also recorded: legacy inside-word remains NO_EXACT_MATCH and unclassified filters are INVALID_REQUEST. One workspace positive is SEARCH_INCOMPLETE because captured adjacency is unproven. Targets and raw responses remain ignored/private. Public evidence contains labels, hashes, states, counts and latencies only.

## Full B-1 Acceptance Matrix

All 40 required source cells pass with synthetic/DB/frozen-artifact evidence. The JSON identifies the proof files and scope for every cell.

| # | Required case | Result |
|---|---|---|
| 1 | Historical-style short quoted Urdu routing | PASS_SOURCE |
| 2 | Equivalent typed literal routing | PASS_SOURCE |
| 3 | Full Urdu transcript-style request | PASS_SOURCE |
| 4 | Quoted English transcript routing | PASS_SOURCE |
| 5 | OCR literal routing | PASS_SOURCE |
| 6 | Native document literal routing | PASS_SOURCE |
| 7 | Unresolvable literal family clarifies | PASS_SOURCE |
| 8 | Selected audio positive | PASS_SOURCE |
| 9 | Selected audio absent | PASS_SOURCE |
| 10 | Workspace audio positive | PASS_SOURCE |
| 11 | Workspace audio exhaustive absence | PASS_SOURCE |
| 12 | Selected OCR positive | PASS_SOURCE |
| 13 | Workspace OCR positive | PASS_SOURCE |
| 14 | Whole-value equality | PASS_SOURCE |
| 15 | Exact phrase | PASS_SOURCE |
| 16 | Phrase contains | PASS_SOURCE |
| 17 | Token search | PASS_SOURCE |
| 18 | Inside-word substring policy | PASS_SOURCE |
| 19 | Too-short literal | PASS_SOURCE |
| 20 | Punctuation preserved | PASS_SOURCE |
| 21 | Repeated Unicode whitespace/newline | PASS_SOURCE |
| 22 | Unicode Urdu phrase | PASS_SOURCE |
| 23 | Urdu two-segment positive | PASS_SOURCE |
| 24 | English two-segment positive | PASS_SOURCE |
| 25 | Three-segment positive | PASS_SOURCE |
| 26 | Absent cross-segment phrase | PASS_SOURCE |
| 27 | Different runs never join | PASS_SOURCE |
| 28 | Different evidence never joins | PASS_SOURCE |
| 29 | Raw and Roman derivative never join | PASS_SOURCE |
| 30 | Missing or ambiguous timing never invents adjacency | PASS_SOURCE |
| 31 | Nonempty workspace never NOT_RUN | PASS_SOURCE |
| 32 | Exhaustive zero is truthful | PASS_SOURCE |
| 33 | Authoritative selected NOT_RUN | PASS_SOURCE |
| 34 | Execution failure | PASS_SOURCE |
| 35 | Incomplete zero never exhaustive no-match | PASS_SOURCE |
| 36 | First candidate page positive | PASS_SOURCE |
| 37 | Beyond first candidate page positive | PASS_SOURCE |
| 38 | Exhaustive paginated zero | PASS_SOURCE |
| 39 | Candidate budget exhaustion | PASS_SOURCE |
| 40 | Show-all truncation disclosure | PASS_SOURCE |

## Regression Results

Full forensic and agent packages pass, including accepted structured, composition, OCR, English/Urdu/Roman transcript, source-time, image/face, ANPR and existing serialization cases. The synthetic full-shape video result producer and agent/Activity serialization bridge passed together after source changes. No broad browser/retained acceptance was performed or claimed.

## Tests

`gofmt` PASS. Full `go test -p 1 ./api/forensic_records -count=1 -v -ginkgo.no-color`: 391 Ginkgo passes, five skips, plus passing legacy tests (173 top-level PASS entries include the suite bootstrap). Full agents: 145 Ginkgo passes, one skip, plus passing legacy tests. Final focused B1/DB/artifact/video gate: 63 passes including the newly added workspace SQL case. Final agent B1/video bridge gate: 10 passes, completing the previously optional bridge. Private logs are SHA-256 referenced by the matrix. JSON, privacy, anti-hardcode and diff checks are recorded there. The tests use actual stored artifact shapes, independent expected values and an independent SQL oracle.

## Environment-Blocked Tests

No B-1 test remains blocked. Initial sandbox Go-cache denial was resolved using approved cache access. The full forensic run skipped two optional nxmmr_p1_test Timescale cases, a disposable-NATS restart test, opt-in local route inputs, and the Windows symlink-creation test. Two registry generation tests were intentionally disabled. The initially skipped agent video fixture was subsequently generated and its bridge passed. None of these skips is counted as a product failure or a pass.

## Anti-Hardcode

Production changes contain generic semantic, routing, normalization, window, budget and validation logic. No retained phrase, evidence ID, phone, plate or analytical answer was added to production paths. Committed tests use synthetic text and identifiers. Independent fixture/SQL expectations are separate from the production matcher.

## Privacy

Raw A snapshots, query targets, responses, fixture copies and test logs remain under ignored local-acceptance-models/nxb21-b1 or the existing reconciliation area. Public reports contain only sanitized labels/hashes/counts/results; no private FLEURS transcript was copied into production or committed fixtures.

## Git Diff Check

PASS. The heavily dirty worktree is preserved. Targeted before/after diffs are based on the captured pre-B1 files, not HEAD. Eleven old-seal member changes are explained exactly by B-1. Existing LF/CRLF advisory warnings do not report whitespace errors. No staging, commit, push or PR.

## Old Source Seal Baseline

Accepted 428-file seal SHA-256 `0b11a1b6cf1b27dc890dd17ceb306cbc255a00ef3f7ba16b5e216d293fcf3127`. The seal file and used activation receipt remain untouched, but 11 of its source members intentionally changed. It does not cover this candidate.

## New Source Digest

435-file source-review digest: `bdd008f05c76c92721503ef078e850aa4ab461ef8fc41039a82cc5f4079ad965`. See b1-source-diff-manifest-v1.json for the deterministic hash method and complete file map. This is SOURCE_VALIDATED_NOT_DEPLOYABLE; no activation manifest, operator bundle, image or deployment was prepared.

## Runtime Impact

No service restart/recreation, container change or accepted activation rerun. Final read-only parity, health, installed-model inventory and retained accounting pass. Existing deployed B-1 defects remain until a future approved activation.

## Retained Impact

No evidence upload, modification, reprocessing, retained Ask request or Activity rewrite. Tuple and accepted case accounting are unchanged. No active jobs.

## Database Impact

Only the explicitly isolated nxb21_b1_test database, synthetic schema and NOLOGIN test role were created for tests. After tests, schema/role absence was verified and the empty test database was dropped. Production database access was read-only; no retained schema migration or backfill.

## Model Impact

None. Qwen, embeddings, Faster-Whisper, Paddle, FastALPR/Video V3, SigLIP and face models/profiles are unchanged. TTS stays disabled; its misleading readiness projection remains NXB21-P1-004 for C.

## Future Minimal Service Scope

api and forensic-records-api only, reflecting the agent bridge/shared package and forensic executor. Worker, PostgreSQL and NATS source/runtime are excluded. No worker extraction change or migration is needed.

## Deployment Required

A future separately authorized build/activation is required for users to receive the fixes. The 6 GiB gate remains mandatory for that future step. No production build or deployable activation bundle was made in B-1.

## Open P0

0 in the bounded B-1 source scope.

## Open B-1 P1

0 source defects after the required proof gates. The accepted runtime has not been corrected; source-fixed issues remain explicitly deployment-pending rather than live-closed.

## Remaining B2.1 P1/P2

P1 NXB21-P1-004: disabled TTS/readiness projection, deferred to C. P2 NXB21-P2-001: capability/language/negative/navigation/Activity proof coverage in C/E/F. P2 NXB21-P2-002: shared disclosure/Ask/Activity polish in G. Processing quality, arbitrary-length windows, ambiguous timing and new languages are not silently certified.

## B2.1B Exit Decision

B-1 and the bounded B software-contract source exit PASS. No evidence justifies inventing B-2/B-3; the exposed citation bridge issue was closed here. This is not live activation, formal certification or automatic advancement to C. The program remains at the B source-closure decision.

## Exact Next Implementation Slice

NX-B2.1C: thin references to existing operations, truthful role/artifact-specific readiness including disabled TTS, explicit input boundaries, registry parity and catalog-only coverage classification. It is identified but not started.

## Exact Next Action

Review this source closure and choose the next phase boundary. If proceeding to C, authorize/select that bounded source scope. If activating B first, separately prepare and approve an api + forensic-records-api operator bundle against the new source state and unchanged 6 GiB gate; never rerun receipt 20260903T063719719Z.

## Machine-readable evidence

- [40-case source matrix](b1-query-semantics-matrix-v1.json)
- [Sanitized 14-case replay](b1-derived-text-probes-v1.json)
- [Source diff manifest and review digest](b1-source-diff-manifest-v1.json)
- [Consumer contract documentation](../../docs/content/features/forensic-derived-text-search.md)
