# NX-MMR Shared Evidence Query P1 Closure V1

## Verified Starting State

The verified live baseline remains the 35-source workspace (34 ready, one preserved failure) under activation receipt `20260901T154458190Z`. This phase performed source work and offline bundle preparation only.

> Activation addendum, 2026-09-02: the bounded bundle subsequently completed
> under receipt `20260902T025246574Z`. Independent live verification confirmed
> state `VERIFIED`, expected images for all three admitted services, zero active
> jobs, zero restarts, and unchanged PostgreSQL/NATS container identities. The
> historical source-phase markers below remain statements about the phase when
> this report was first closed. Live acceptance remains pending; no existing
> retained transcript was rewritten and no product certification was promoted.

## Existing Runtime Preserved

No build, deployment, restart, migration, model download, evidence reprocessing, volume operation, or runtime cleanup was performed.

## P1 Root-Cause Summary

The defects were integration boundaries: exact OCR could enter generic derived-text retrieval; transcript locators were not fully propagated; composition did not independently bind phone and plate targets; similarity candidate population was coupled to the loaded UI page; and media navigation did not apply source time. Urdu diagnosis showed automatic language detection selecting Hindi/Devanagari when no trusted durable Urdu hint existed.

## Already-Fixed Context Inheritance

Anaphoric context is inherited only within the originating portal/evidence scope. Explicit new targets do not reuse stale evidence context. Focused presentation tests pass.

## Already-Fixed ANPR Target Extraction

Both agent and API extraction accept zero through three trailing letters. Synthetic tests cover multi-letter suffixes without embedding retained identifiers.

## OCR Exact Lookup Root Cause

Selected-image exact requests could retain a generic derived-text mode and therefore admit unrelated document results.

## OCR Exact Lookup Correction

Selected current image + OCR family + exact phrase routes to governed `image_ocr_search`. Backend filtering enforces requested evidence ID, current evidence version, and normalized exact term.

## OCR No-Match

Absence returns a clean exact no-match. It does not fall back to collection, document, KB, or semantic retrieval.

## OCR Citation

Results carry evidence ID, evidence version, artifact/observation ID, source family, source locator, and typed citation locator for the matching OCR observation/region.

## Transcript Citation Root Cause

Stored segment timing existed, but the derived-text result envelope did not preserve it through the shared citation contract.

## Transcript Start/End Propagation

Authoritative `start_seconds` and `end_seconds` now propagate when present. Missing timing stays absent and is never converted to zero or estimated.

## Transcript Time-Range Query

The existing intersecting-segment range semantics are preserved and covered by the forensic API suite.

## Audio/Video Seek Capability

Citation navigation passes validated `source_time`; the real audio/video element seeks after metadata loads and does not autoplay. Missing timing produces no synthetic seek.

## Cross-Family Root Cause

The prior planner treated the compound question as a retrieval-shaped request instead of independently planning authoritative operations for each explicit family target.

## Cross-Family Planner / Execution

Phone + plate plans `cross_family_correlation` and `anpr_sightings`; an explicit document/OCR mention adds bounded `document_search`. Each step executes independently and typed packets are merged.

## Family Authority Preservation

Phone facts come from structured operations, plate facts from ANPR observations, and narrative mentions from derived document evidence. Document retrieval cannot suppress structured or ANPR authority.

## Multi-Family Citation Validation

Each factual clause retains its own family citation. The composer does not infer a phone-to-plate identity, ownership, person, or causal relationship.

## Urdu STT Root Cause

The worker and backend use automatic language detection unless a validated durable `asr_language` hint is present. The fresh retained source had no trusted hint, so the model selected `hi` and emitted Devanagari; there is no post-inference script conversion causing it.

## Urdu Auto vs Explicit-UR Diagnostic

The previously authorized non-retained same-model comparison passed: explicit `ur` reached the backend, produced Urdu script, and improved WER/CER against the independent publisher transcript. AUTO reproduced Hindi/Devanagari behavior.

## Urdu STT Decision

Keep AUTO as the default. The real Add Data dialog now exposes `Automatic detection` and `Urdu — اردو`. Selecting Urdu sends the validated durable `asr_language=ur` hint through the LocalAI upload proxy and forensic API to the worker; the worker rejects a trusted-Urdu transcript if it contains Devanagari or contains no Urdu-script characters. This prevents non-Urdu script from being accepted as successful Urdu STT. Do not globally force Urdu, tune the model, download another model, or silently mutate the already-retained source.

## Roman Urdu

Roman Urdu derivation remains eligible only from authoritative Urdu-script transcript observations with parent lineage. Devanagari is not relabeled or Romanized as Urdu evidence.

## SigLIP Fresh Integration

### Candidate Population

Backend authorization now supplies the current-version collection/case population; the loaded React page is not the analytical universe.

### Ask

Natural similarity intent runs as a bounded submode of the already-certified `image_metadata` operation, preserving the sealed 79-operation public catalog.

### Citation

Query and candidate evidence/version locators are distinct, with similarity scores attached only to candidate observations.

### Activity

User-triggered Ask actions use normal Agent history and therefore remain visible in Activity. The direct Data ranking button is not separately asserted as an Activity event.

### Pagination Correctness

The backend fetches at most 201 authorized candidates, returns at most 200, and explicitly discloses bounded population and truncation. UI pagination controls rendering only.

## Face Candidate Integration

### Ask

Natural face-comparison intent runs as a bounded submode of `face_candidate_observations`.

### Citation

Query and candidate citations preserve current evidence/version authority.

### Activity

User-triggered Ask actions use normal Agent history. Direct Data ranking is not claimed as a separate Activity event.

### Safety

Outputs remain candidate similarity, not identity. No unsupported identity assertion or inferred score is emitted; an ineligible selected observation returns complete-zero.

## LLM Query Planner

Language understanding selects only registered bounded operations/submodes. Deterministic execution, authorization, validation, citations, and counts remain backend-owned; no arbitrary SQL or tool execution is exposed.

## English / Urdu / Roman Urdu Query Understanding

Existing normalized multilingual intent handling is preserved. This phase added no language-specific analytical facts or hardcoded demonstrations.

## Exact vs Semantic Semantics

Exact selected OCR is deterministic and evidence-scoped. Semantic document questions retain the governed retrieval path. Image opening does not accidentally invoke similarity.

## No-Match Semantics

Exact OCR and ineligible similarity queries return explicit complete no-match/zero states without semantic substitution.

## Conversation Context / Follow-Ups

Same-scope anaphora may inherit context; explicit targets and scope changes re-plan from the new request.

## Data / Ask Consistency

Data and Ask use the same backend candidate loaders and current-version population rules. Ask additionally supplies governed citations/history.

## Structured CDR Regression

The full forensic API suite passes, including the sealed 79-operation catalog and structured composition behavior.

## Simple Ask UX

Answers remain concise family clauses with analyst-facing source links; internal operation IDs, SQL, model names, and processor internals are not primary presentation.

## Security / Scope

Tenant, collection, optional case, selected evidence, current version, artifact family, operation allowlist, row caps, and citation scope remain enforced. Source tests found no cross-case leakage.

## Source Query Matrix

Covered: selected OCR found/no-match/citation; transcript locator/range; context follow-up/new target; ANPR suffix and absence; phone+plate(+document); image/face Ask, population, citations, zero-state, and exact-image non-routing; current-version and authorization boundaries.

## Files Changed

Production changes are bounded to forensic query/composition/similarity code, agent routing/tool descriptions, Analyst Data/Ask presentation, and the pre-existing ASR language-hint path. This phase also adds the sealed shared-query activation manifest and PowerShell operator bundle.

## Tests

- `go test ./api/forensic_records -count=1`: PASS (330 specs passed, 3 skipped)
- focused agent tests: PASS
- `go vet ./api/forensic_records ./core/services/agents`: PASS
- frontend analyst tests: PASS (41/41, including trusted Urdu intake policy)
- ESLint changed frontend files with `--quiet`: PASS (zero errors)
- Faster-Whisper backend tests: PASS (6/6)
- Python compile checks: PASS
- trusted Urdu-script admission tests: PASS (Urdu accepted with Roman derivative; Devanagari and Latin-only rejected)
- shared-query operator bundle self-test: PASS (25/25)

## Environment-Only Test Limitations

System Python has no `pytest` module, so pytest collection was unavailable; direct backend tests and `py_compile` passed. No rootless Windows testcontainer failure was interpreted as a product failure.

## Anti-Hardcode

Production-path scans and synthetic regressions contain no retained plate, transcript, phone, expected timestamp, expected OCR value, similarity ordering, or face score.

## Privacy

This report contains no private identifier, transcript, face crop, phone number, or retained source detail.

## Git Diff Check

`git diff --check`: PASS (line-ending notices only, no whitespace errors). Both new JSON contracts parse successfully.

## Runtime Impact

None. The running activation was not touched.

## Retained Impact

No retained evidence, result, Activity history, database state, named volume, or model asset changed.

## Demo Readiness Matrix

Source-ready: shared Ask corrections, exact OCR, transcript citations/seek, ANPR extraction, structured/ANPR/document composition, image similarity Ask, face candidate Ask, and the explicit trusted-Urdu Add Data path with fail-closed script validation. Runtime states remain at the prior verified activation until the one consolidated activation and browser matrix pass. The existing no-hint retained sample is unchanged.

## Certification Matrix

The canonical catalog remains 79 operations. No product certification, NXB2 activation, or certification-count promotion occurred.

## Consolidated Activation Requirement

One offline activation only, after unnecessary apps are closed and the measured available-RAM gate reaches at least 6 GiB. The script must fail closed below the gate.

## Exact Services Requiring Activation

`api`, `forensic-records-api`, and `forensic-records-worker`. PostgreSQL and NATS are protected and must remain identical.

## Offline Operator Bundle

The manifest and integrity seal bind the source. If RAM is the only pending gate, the single runner waits up to 120 minutes and may release only verified-clean Docker Linux page cache with mode 1. Builds are sequential/resumable; no pull, prune, download, bulk reprocess, service shutdown, or app termination is allowed.

## Rollback

Before mutation, the bundle captures exact image IDs, environment, mounts, ports, commands, healthchecks, network, and named-volume references. Rollback recreates only the three touched services by immutable image ID.

## Exact User PowerShell Command

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_nxmmr_shared_query_activation.ps1
```

## Open P0

None in source evidence. Any later unauthorized cross-case result is an immediate P0 stop.

## Open P1

The required 21-point real-browser acceptance matrix remains pending. Its first
controlled proof must be a fresh audio upload with `Urdu — اردو` selected; raw
Urdu-script output and Roman-Urdu parent lineage must be observed, while
Devanagari or Latin-only output must fail closed. Direct Data similarity clicks
are not separately persisted as Activity events; Ask similarity actions are.

## EXACT NEXT REAL ACTION

Do not rerun the completed activation. Hard-refresh the live Analyst UI, upload
one fresh controlled Urdu audio copy through Add Data with `Urdu — اردو`
selected, wait for the job to finish, and execute the Urdu/raw-script,
Roman-lineage, timed-citation/seek, Activity-reopen, and scope-isolation checks
before the rest of the 21-point browser matrix.

```text
CurrentActivationPreserved=true
CurrentActivationReceipt=20260901T154458190Z
ContextInheritanceSourceFix=PASS
ANPRTrailingLettersSourceFix=PASS
OCRExactSelectedEvidence=PASS
OCRExactNoMatch=PASS
OCRCitationScope=PASS
TranscriptTimeLocator=PASS
TranscriptTimeRange=PASS
TranscriptCitation=PASS
CrossFamilyPlanning=PASS
StructuredAuthorityPreserved=PASS
ANPRAuthorityPreserved=PASS
DocumentAuthorityPreserved=PASS
UrduSTTRootCause=AUTO_LANGUAGE_DETECTION_HI_DEVANAGARI_WITHOUT_TRUSTED_UR_HINT
UrduExplicitLanguageDiagnostic=PASS
UrduSTTSourceDecision=TRUSTED_UR_UI_AND_FAIL_CLOSED_SCRIPT_GATE_SOURCE_READY_AUTO_REMAINS_DEFAULT
RomanUrduSourceReady=true
ImageSimilarityAskSourceReady=true
ImageSimilarityCitationSourceReady=true
ImageSimilarityActivitySourceReady=true
ImageSimilarityPaginationCorrect=true
FaceAskSourceReady=true
FaceCitationSourceReady=true
FaceActivitySourceReady=true
LLMIntentUnderstanding=BOUNDED_ACCEPTED_OPERATION_SUBMODES
TypedDynamicPlanning=PASS
ExactNoMatchSemantics=PASS
DataAskConsistency=PASS
CrossCaseLeakage=NONE
NoHardcodedAnalyticalAnswers=PASS
NoSelfOracle=PASS
NoUnrestrictedSQL=PASS
ProductCertificationPerformed=false
NXB2ActivationPerformed=false
DeploymentPerformed=false
DatabaseMigration=false
VolumesChanged=false
```
