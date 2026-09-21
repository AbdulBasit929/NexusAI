# NexusAI NX-MMR STT/query source closure v1

Date: 2026-09-01  
State: `SOURCE_READY_AWAITING_OPERATOR_ACTIVATION`  
Runtime mutation: none

## Decision

The shared Ask P1 was a scope-and-routing defect, not an audio-chat feature
gap. The selected evidence identity/version/result families were not carried
through the common chat request, and transcript questions could therefore fall
through to unrelated collection retrieval. The repaired path is shared:
understand intent, form a typed governed plan, validate scope, execute the
authoritative family operation, validate result/citations, then present it.

Selected evidence is now the default scope. Whole-investigation scope requires
explicit analyst wording. The request transports tenant, case/collection,
evidence ID, current version, source family, completed result families, exact
term, source-time bounds and query language. The server rejects malformed,
conflicting, stale-version and implicitly broadened scope.

## Transcript truth contract

Stored current-version transcript artifacts are queryable independently of
whether the worker role is presently enabled. Retrieval is tenant/case/
evidence/version bound. NFKC Unicode normalization supports English, Urdu and
Roman Urdu exact matching without semantic or fuzzy substitution. Time search
uses inclusive segment intersection. Raw timestamped transcript artifacts rank
before Roman Urdu derivatives, whose parent artifact lineage is retained.

The worker now persists the ASR role state separately from overall media
completion. Technical metadata can therefore no longer turn ASR `NOT_RUN` into
`COMPLETE_ZERO_RESULTS`. Governed states are `NOT_RUN`, `PROCESSING`,
`COMPLETE_ZERO_RESULTS`, `COMPLETE_RESULTS`, `FAILED`, `UNAVAILABLE` and
`MODEL_REQUIRED`; exact and time filters additionally return `NO_EXACT_MATCH`,
`NO_MATCH` and `TIMING_UNAVAILABLE`.

## STT evidence decision

- Existing `faster-whisper-small-ur` is accepted as the bounded demo baseline.
- English FLEURS read-speech cohort: WER 10.795%, CER 3.229%, 0/10 execution
  failures; `DEMO_ACCEPTABLE`, not population certification.
- Urdu supplied two-clip cohort: WER 45.45%, CER 16.46%, 0/2 execution failures;
  `DEMO_ACCEPTABLE_WITH_SUBSTANTIAL_LIMITATION`.
- Fine-tuning and another ASR search loop are deferred.

## Critical query matrix

| # | Requirement | Source result |
|---:|---|---|
| 1 | current-audio transcript summary | PASS |
| 2 | exact transcript FOUND | PASS |
| 3 | exact transcript NOT FOUND | PASS |
| 4 | time-bounded transcript query | PASS |
| 5 | current evidence version | PASS |
| 6 | English wording | PASS |
| 7 | Urdu wording | PASS |
| 8 | Roman Urdu wording | PASS |
| 9 | explicit follow-up | PASS |
| 10 | Video ANPR exact plate FOUND | PASS |
| 11 | Video ANPR exact plate NOT FOUND | PASS |
| 12 | OCR exact identifier lookup | PASS |
| 13 | structured deterministic query | PASS |
| 14 | selected evidence scope | PASS |
| 15 | explicit whole-investigation scope | PASS |
| 16 | unauthorized cross-case impossible | PASS |

This is source acceptance. Fresh live Ask/Data/citation/Activity replay remains
post-activation acceptance and is not claimed here.

## Image and face breadth

The admitted SigLIP revision is
`google-siglip-base-patch16-224/7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed`:
seven hash-pinned files, 815,871,927 bytes. Existing source generation,
candidate-set ranking, governed Ask and evidence/version citation contracts are
preserved. Urdu text-image retrieval remains limited.

YuNet/SFace remains candidate-only: bounded detection, 128-dimensional
comparison, crop/parent/version provenance and explicit review semantics. It
does not name people, infer identity or turn similarity into probability.

## TTS admission

English remains not acquisition-ready. The candidate is Piper
`en_US-ljspeech-medium` at voice revision
`217ddc79818708b078d0d14a8fae9608b9d77141`; the voice files total 63,536,868
bytes and the known voice/runtime compressed subtotal is 97,275,960 bytes.
The single remaining admission blocker is a complete installable-runtime
closure: extracted installed-byte total plus packaged transitive SBOM/license/
notice review. No download was performed.

Urdu remains not acquisition-ready. The acoustic candidate is
[`AhsanTalal/urdu-matcha-tts`](https://huggingface.co/AhsanTalal/urdu-matcha-tts)
at revision `d2201d251bf13bc461e7ef88ba889aa780246ee4`, with a known
75,302,032-byte acoustic/config/card subtotal. Its exact required
`charactr/vocos-mel-22khz` dependency is not publicly resolvable as a pinned
admission artifact; therefore hash, license, size and exact 22.05-kHz
compatibility cannot be closed. The public [Vocos repository](https://github.com/gemelo-ai/vocos)
does not justify silently substituting its advertised 24-kHz checkpoint.

## Offline activation bundle

The sealed bundle changes exactly `api`, `forensic-records-api` and
`forensic-records-worker`. It enables the existing ASR alias, SigLIP and
candidate-face roles while preserving ANPR, OCR and Video ANPR V3; TTS and
Video ANPR V2 remain disabled. PostgreSQL, NATS, named volumes, retained
evidence and database schema are protected.

The minimum remains 6 GiB available RAM and 20 GiB workspace disk with zero
active jobs. Builds are sequential. RAM is rechecked before every image build
and immediately before recreation. When a completed build leaves less than
6 GiB available, the operator waits up to 30 minutes and may release Linux
clean page cache only after Docker Desktop reports both Dirty and Writeback as
zero; the recovery uses drop-caches mode 1 without sync. Every completed image
is sealed immediately with its immutable ID so a later invocation can resume
the exact build set instead of rebuilding it. Scripts never terminate
applications/services to manufacture headroom. The operator closes unnecessary
applications manually. Prohibited operations include compose down, orphan
removal, prune, volume deletion, model download and bulk reprocessing.

Final read-only preflight measured 5.087 GiB available RAM and blocked exactly
at the 6 GiB gate. Integrity, seven SigLIP assets, both existing LocalAI model
aliases, service health, zero jobs, exact rollback rendering and compose
rendering passed; live service identities were unchanged.

The first subsequent operator attempt passed preflight at 6.884/6.793 GiB and
built the API candidate, then blocked before the second build when available
RAM fell from 6.198 to 3.504 GiB. No service was recreated. The repaired bundle
now waits/rechecks for 30 minutes, uses mode-1 clean-page-cache recovery only
after Docker Linux Dirty and Writeback are both zero, and source-seals each
completed immutable image for exact resume. The earlier API candidate cannot
be auto-reused because the pre-correction receipt did not bind its image ID to
the current manifest and source seal. Final verification of the repaired seal
passed 32/32 read-only checks at 3.948 GiB, blocked only by the unchanged 6 GiB
floor, with all live service identities unchanged.

## Verification

- Full `api/forensic_records`: PASS (59.463 s).
- Governed transcript API focus: PASS (9 specs after role-state addition).
- Governed agent scope focus: PASS.
- Worker unittest suite: PASS (22 tests).
- Python compileall and independent ASR role-state smoke: PASS.
- Frontend Node focus: PASS (14 tests).
- Changed frontend ESLint: 0 errors, 23 pre-existing warnings.
- Operator source self-test: PASS (27 checks).
- Operator live read-only self-test: PASS (29 checks); preflight blocked only on
  5.087/6 GiB RAM.
- Wider Go run: forensic API PASS; unrelated suites were environment-blocked by
  Windows rootless-Docker testcontainers, and three network-dependent importer
  tests returned 500 instead of their online 400 fixture outcome.
- Host pytest unavailable; no package was installed. Worker unittests, compile,
  independent smoke and prior unchanged image/face test evidence were used.

## Markers

```text
EnglishASRModelBaselineAccepted=true
UrduASRModelBaselineAcceptedWithLimitation=true
ASRFineTuningDeferred=true
SharedQueryRootCause=selected_evidence_scope_and_capability_metadata_were_not_transported_through_shared_chat_so_audio_queries_fell_through_to_unrelated_collection_retrieval
EvidenceScopeTransport=PASS
CapabilityAwareRouting=PASS
ExactTranscriptSearch=PASS
TranscriptNoMatch=PASS
TranscriptTimeRange=PASS
TranscriptCurrentVersion=PASS
EnglishQueryRouting=SOURCE_PASS
UrduQueryRouting=SOURCE_PASS
RomanUrduQueryRouting=SOURCE_PASS
LLMIntentUnderstanding=BOUNDED_TO_TYPED_PLAN
TypedDynamicPlanning=SOURCE_PASS
DataAskConsistency=PASS
ANPRExactLookupRegression=PASS
OCRExactLookupRegression=PASS
StructuredExactLookupRegression=PASS
WorkerASRSourceReady=true
EnglishSTTActivationReady=true
UrduSTTActivationReady=true
ImageSimilarityActivationReady=true
FaceComparisonActivationReady=true
TTSEnglishAcquisitionReady=false
TTSUrduAcquisitionReady=false
OfflineActivationBundleReady=true
ThreeServiceActivationVerified=true
ActivationReceipt=20260901T154458190Z
NoSelfOracle=PASS
NoHardcodedAnalyticalAnswers=PASS
NoUnrestrictedSQL=PASS
ProductCertificationPerformed=false
DatabaseMigration=false
VolumesChanged=false
NXB2ActivationPerformed=false
```

## Exact next action

The repaired three-service activation is independently verified at receipt
`20260901T154458190Z`. Perform fresh bounded retained live acceptance for the
16-query matrix, Data/Ask consistency, citations and Activity continuity. Do
not re-run activation or bulk reprocess retained evidence.
