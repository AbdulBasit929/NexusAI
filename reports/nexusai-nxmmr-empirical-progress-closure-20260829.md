# Verified Starting State

> 2026-08-30 empirical addendum: the human-gold ANPR continuation is complete.
> The original 2026-08-29 checkpoint below is superseded where it says ground
> truth or scoring is pending. See
> `nexusai-nxmmr-human-gold-anpr-asr-closure-20260830.md`.

Source/live truth was 79/214 operations/query variants versus 67/68 live; five
models and eight backends were live, the worker media-model mount was empty,
and the two authorized local inputs totaled 234,784,354 bytes. All five live
services were healthy and remained untouched.

# Approval Recorded

NX-MMR A-D was recorded as approved for the exact seven-file acquisition,
private read-only/non-retained use of the two named inputs, isolated empirical
benchmarking and bounded routing/certification source work. Live install,
build/deploy, retained mutation, database/volume changes and NX-B2.1 were not
approved.

# Resource-Gate Correction

Applied: one heavy model; LIGHT/MEDIUM/HEAVY floors 3.0/3.5/4.5 GiB; subsequent
gate max(class floor, measured peak + 1.5 GiB); emergency stop near 1 GiB. The
separate build/deployment floor remains 6 GiB.

# Resource Measurements

All admitted heavy runs started between 4.80 and 5.02 GiB free. OCR peaked at
446.605 MiB process RSS. Both retrieval models reached the 3,072 MiB container
cap. Final free Docker memory was 5,327,904,768 bytes (4.962 GiB).

# Files Acquired

Seven exact root artifacts, 663,933,265 bytes. Root count and total reverified
at closure; no substitution or public dataset.

# Integrity / License Verification

Hashes, sizes, Paddle archive safety/members, GGUF v3 structure and authoritative
MIT/Apache-2.0 licensing passed. The private user data is not publication-
cleared and no raw identifiers are included in reports.

# Ground-Truth Preparation

108-image manifest, 22-image sealed holdout, six image contact sheets, 30 video
review frames, two video contact sheets, CSV templates, protocol and lossless
review audio are complete. Validators reject blank, model-seen or non-human
truth.

# User Ground-Truth Completion

The independent human workflow completed with `GroundTruthValidation=PASS`, 32
active image labels, 19 video plate events and six negative speech windows.
No candidate output was used as truth.

# Pakistani Image ANPR Benchmark

`COMPLETE_PRIVATE_HUMAN_GOLD_LIMITED`; development detection/exact are 10/10
and 5/10. Sealed-holdout detection/exact are 20/22 and 13/22; holdout CER is
0.201439. Image negatives and boxes are absent, so specificity/IoU remain
unknown.

# sample.mp4 Video ANPR Benchmark

`COMPLETE_NOT_ADMISSIBLE_FOR_PROMOTION`; detection occurred in 3/19 human
events and 3/41 event plates were exact. Grouping was 4/29 with 1.178250-second
mean absolute boundary error.

# ANPR Error Analysis

At sampled-frame presence level TP/FP/FN/TN was 3/1/31/25. Two events/four
plates were unsampled; 14 further events/30 plates had sampled frames and no
detection. Image development had five OCR errors/no misses; holdout had seven
OCR errors/two misses.

# Tesseract English/Urdu Benchmark

English: 3/6 exact, CER 0.1429, WER 0.5. Urdu: 0/1 exact, CER 0.2143, WER
0.3333. Negative control passed; peak 19.746 MiB.

# PaddleOCR Urdu Challenger

English: 5/6 exact, CER 0.0714, WER 0.1667. Urdu: 0/1 exact, CER 0.3571, WER
0.6667. Negative control passed; peak 446.605 MiB. Paddle leads English only;
Tesseract remains the lower-memory, lower-error single-Urdu-fixture floor.

# OCR Error Analysis

Neither candidate is exact on the one Urdu sample; the sample is insufficient
for promotion. English Paddle still has one exact-identifier failure. All OCR
claims remain fixture-only.

# Handwriting Status

English and Urdu: `BENCHMARK_DATA_REQUIRED`.

# ASR Benchmark

All six human windows are negative and contain no transcript. Scoring is
`NOT_ADMISSIBLE_NO_POSITIVE_SPEECH_OR_TRANSCRIPT`; no ASR inference was run.

# Urdu / Roman Urdu

Raw Urdu and Roman Urdu remain separate. Roman Urdu may be a reviewed secondary
representation, never a replacement for raw transcript truth.

# TTS Status

`DEFERRED_NOT_BENCHMARKED`.

# Retrieval Baseline

Keep Qwen3 embedding: Recall@1 0.8182, Recall@5 1.0, MRR 0.8667, 136.836 s.
Limitation: 0/2 no-answer abstention and weaker Urdu/Roman-Urdu rank one.

# Reranker Benchmark

Reject Qwen3 reranker admission: Recall@1 0.7273, Recall@3/5 1.0, MRR 0.8636,
2/2 no-answer abstention, 397.425 s and one prior 120-second timeout attempt.

# Face Status

Existing candidate-only, embedding-redacted limitation retained. No identity,
demographic, ownership, presence or speaker inference.

# Image Semantic Status

`DEFERRED`; no admitted real-world benchmark/model evidence.

# Open-Vocabulary Vision Status

`DEFERRED`; no approved benchmark evidence.

# Video/VLM Status

`DEFERRED`; bounded deterministic video metadata/review preparation only.

# Query Intelligence Benchmark

The 214-entry source query ledger remains source evidence. The 13-query model
ranking fixture does not certify planner/executor/answer/UI behavior. No new
real-world promotion.

# Answer Synthesis Benchmark

No challenger was approved or run. Existing Qwen3 4B cited Fact/Observation
Packet synthesis remains source-validated; no new promotion.

# Auto-Routing Implementation

`nexusai-evidence-route-plan/v1` extends current intake/registration metadata
in shadow mode with magic/MIME, scope, family roles, readiness, resources,
security, confidence, fallback and limitations. Queue semantics are unchanged.

# Routing Certification

16/16 sealed decision cases pass (1.000 fixture accuracy). Applicable-role,
missing-model/worker, structured corroboration and receipt tests pass.
Certification is `FIXTURE_CERTIFIED_SHADOW_ONLY`, not activated/product-ready.

# Real-World Operation Certification

No promotion. Ledger remains 62 registered, 12 source-validated, one fixture-
certified, zero real-world-certified and four product-certified.

# Query Certification

Existing certification levels and independent-oracle requirements remain
enforced. No self-oracle and no unrestricted SQL.

# Suggestion Eligibility

Ordinary visible suggestions remain exact `PRODUCT_CERTIFIED` mappings only.
No NX-MMR fixture result became a visible suggestion.

# UI/UX Readiness

The existing manual acceptance guide is prepared. No UI source or live runtime
was changed in this empirical slice.

# Manual UI Tests Still Required

After separate activation approval: upload/route status, positive/zero/no-match/
unavailable/failure states, citation/source navigation, Activity reopen,
390/820/1024/1440, theme, keyboard, Urdu/RTL and performance.

# Performance

Tesseract 2.066 s/full 8; Paddle 6.759 s/full 8; embedding 136.836 s/full 13;
reranker 397.425 s/full 13. The reranker is not operationally justified.

# Peak RAM by Capability

Tesseract 19.746 MiB; Paddle 446.605 MiB; embedding and reranker each sampled
at the 3,072 MiB container cap.

# CPU Observations

All candidate runs were CPU-only, capped at eight CPU units for retrieval and
four threads for Paddle. Reranking was the dominant CPU latency path. No
parallel heavy model was run.

# License Review

MIT for plate detector/CCT sources; Apache-2.0 for Tesseract data, PaddleOCR and
Qwen resources. Preserve upstream notices on any eventual distribution.

# Current vs Challenger Decisions

Paddle leads Tesseract for the small English fixture; Tesseract retains the
limited Urdu/resource floor. Current Qwen embedding beats and retains priority
over the Qwen reranker challenger.

# Capabilities Promoted

None.

# Capabilities Limited

Printed English OCR fixture evidence, printed Urdu OCR, current embedding,
existing face-candidate observations, and shadow route planning.

# Capabilities Deferred

Image/video ANPR, English/Urdu ASR, Roman-Urdu secondary transcript, handwriting,
image semantic, open-vocabulary vision, VLM/video semantics, TTS and diarization.

# Capabilities Rejected

Qwen3-Reranker-0.6B Q8 admission on current evidence; face/speaker identity and
model-generated ground truth remain out of scope.

# Files Changed

Bounded changes cover NX-MMR governance/ledger reports and JSON, three benchmark/
ground-truth scripts plus tests/config/fixture, and the new Go route-plan source/
tests integrated through `api/forensic_records/main.go`. Private models,
ground-truth packs and detailed results remain ignored local artifacts.

# Tests

- full forensic API: pass, 322 specs (320 passed, two skipped), 46.934 s;
- route-plan focused suite: pass;
- Go vet forensic API: pass;
- NX-MMR Python unit tests: 6/6 pass; compile pass;
- JSON parse: all authoritative/current result files pass.

# Regression

Agents: 105/119 pass; the same 14 database-backed specs are blocked before
assertions by unsupported rootless-Windows testcontainers. This is the known
environment baseline and does not touch NX-MMR source. No new regression found.

# Git Diff Check

Pass. Only the existing LF-to-CRLF working-copy warning was emitted.

# Anti-Hardcode Scan

Pass on new production route and evaluator paths; no retained UUID, phone,
plate, fixture answer or workspace identifier. Route source adds no SQL.

# Runtime Impact

None. Five live services remain running/healthy. All disposable NX-MMR
containers and their internal network were removed.

# Retained Impact

None: evidence, jobs, Activity and collection state unchanged.

# Model Live-Installation Impact

None. Exact artifacts remain isolated; live models/backends/profiles unchanged.

# Database Impact

None. No query, migration or schema/state mutation was performed.

# Open P0

0.

# Open NX-MMR P1

Three acceptance gates: independent image labels; video plate/negative interval
labels; verbatim speech transcript/language intervals. These are oracle gates,
not observed production defects.

# Next Approval Required

None to perform the human labeling or resume the already-authorized private,
non-retained ANPR/ASR scoring. Separate explicit approval is required for any
live model/processor install, source activation/build/deploy or NX-B2.1.

# Exact Next Action

Human reviewer completes and validates the two private CSV templates using the
original images/video and `review-audio.m4a`; then rerun NX-MMR ANPR/ASR
benchmarks under the existing one-heavy-model/resource policy.

## Markers

```text
NXMMRApprovalRecorded=PASS
WorkloadAwareResourceGate=PASS
ParallelHeavyModels=1
ApprovedAcquisitionCompleted=true
SupplyChainVerification=PASS
LocalImageGroundTruth=PASS_LOCKED_32
SampleVideoGroundTruth=PASS_LOCKED_19_EVENTS
ANPRImageBenchmark=COMPLETE_LIMITED_NO_PROMOTION
ANPRVideoBenchmark=COMPLETE_REJECT_CURRENT_POLICY_FOR_PROMOTION
PrintedEnglishOCR=FIXTURE_CERTIFIED_LIMITED
PrintedUrduOCR=LIMITED_BENCHMARK_DATA_REQUIRED
HandwrittenEnglishOCR=BENCHMARK_DATA_REQUIRED
HandwrittenUrduOCR=BENCHMARK_DATA_REQUIRED
UrduASR=BENCHMARK_DATA_REQUIRED
EnglishASR=BENCHMARK_DATA_REQUIRED
RerankerBenchmark=REJECTED
FaceCandidate=LIMITED_CANDIDATE_ONLY
ImageSemantic=DEFERRED
EvidenceAutoRouting=FIXTURE_CERTIFIED_SHADOW_ONLY
QueryRealWorldCertification=UNCHANGED
AnswerSynthesisCertification=SOURCE_VALIDATED_EXISTING_BASELINE
VisibleSuggestionsProductCertifiedOnly=PASS
NoSelfOracle=PASS
NoUnrestrictedSQL=PASS
ProductionHardcodeScan=PASS
OpenP0=0
RuntimeMutated=false
LiveModelsChanged=false
RetainedStateMutated=false
ActivityMutated=false
DatabaseMigration=false
VolumesChanged=false
DeploymentPerformed=false
NXB2ActivationPerformed=false
```
