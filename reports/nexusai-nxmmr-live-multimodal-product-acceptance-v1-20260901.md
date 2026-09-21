# NX-MMR live multimodal product acceptance V1 — 2026-09-01

# Verified Live Starting State

Activation `20260901T154458190Z` was already complete and independently verified. This phase did not rebuild, redeploy, restart, migrate, prune, or reprocess the runtime. The existing Multimodal Product Acceptance workspace was used through the real Home, Data, Ask, and Activity UI.

# Activation Receipt

- API image: `sha256:130f3f40686ca971f6624c90575df7db6994044e18260a8271cd07fee5ab959f`
- forensic API image: `sha256:5c1d71cd903cc0b2ba541f2f846607bfcf60434ac1bf0c7ba0a0a69a7218c2a1`
- worker image: `sha256:a7ca5956dcc15d1e8de012167e0b294a590c5093d944f5594eebe6456bd9957b`
- activation receipt: `20260901T154458190Z`, VERIFIED

# Service Health

API and worker were healthy, forensic API was running, PostgreSQL and NATS were healthy, active jobs were zero at the starting check, and restart/OOM counters were zero. No service identity changed. Host available RAM was approximately 5.10 GiB at baseline. During STT observation, worker memory was approximately 598 MiB, API 1.25 GiB, and forensic API 17 MiB; no restart or OOM was observed.

# Shared Ask Live Acceptance

Overall **FAIL**. Positive English transcript, ANPR, and CDR routes execute, but the same shared surface also produced wrong OCR, cross-evidence, and absent-plate results. A stale-context UI defect was source-fixed but is not deployed.

## Selected Evidence Scope

Positive transcript lookup stayed on the selected fresh audio. OCR selected-evidence exact lookup did not: it returned unrelated PDF citations. **FAIL**.

## Exact Found

Fresh English and deployed Devanagari transcript phrases matched their own current evidence/artifact. Plate `MN1367` and structured identifier `923001110001` returned correct current evidence. OCR exact lookup failed consistency. **PARTIAL**.

## Exact No-Match

Direct transcript execution without inherited UI context correctly returned `no_match_for_filter`. The live UI inherited stale context and returned HTTP 400 until Clear. An absent explicit plate `ZZ99ZZZ` returned 20 unrelated ANPR rows because the deployed target extractor did not recognize multiple trailing letters. **FAIL**.

## Time-Bounded Query

Stored STT segments contain actual times, but Ask omitted the requested segment range from its answer/citation and citation navigation had no seek parameter. **FAIL**.

## Current Version

Fresh source details and selected transcript operations used current evidence versions `9ee016fc-485c-4ffc-ae7e-5e1289ebecce` and `585b2754-fae7-47c3-9f25-3461c06af013`. Current-version transport is live for the positive selected-audio path. **PASS**.

## Broad Scope

Workspace CDR and ANPR positive queries execute. The composed phone-plus-plate query returned only document/KB results and omitted the structured CDR source. **FAIL** for cross-evidence breadth.

## Follow-Up

The UI reused prior answer context across evidence/workspace boundaries and explicit new targets. Source now annotates answer messages with request scope and inherits context only for anaphoric follow-ups in the identical scope. This is tested but not deployed. **FAIL live**.

# English STT

## Processing

Fresh approved FLEURS file SHA-256 `5fefdcd12d4c136762cc7841b084126fc60c6d6410f983cc9f82b6cfc45737a2` was uploaded through Add Data. Evidence `7b1b6259-5866-4f03-a0d9-624f9c377a73` completed with four observations, ASR `COMPLETE_RESULTS`, duration 16.38 s, and no restart/OOM. Registration-to-completion was about 40 seconds.

## Transcript

Three English segments were stored at 0–5.56, 5.56–11.28, and 11.28–15.08 seconds. Data displays the actual machine transcript and analyst-review limitation.

## Timing

Actual segment timing is preserved in Data. Ask/citation did not expose the requested 11.28–15.08 range, so end-to-end time acceptance failed.

## Data

PASS: classification, ASR result state, segments, language, duration, and current source are visible.

## Ask

PASS for a dynamically derived exact phrase; incomplete for the full A–G matrix because time and source-follow-up behavior are defective.

## Citation

The exact phrase cited the correct evidence/artifact, but the link carried only the evidence ID. It did not carry artifact/time and could not prove seek/highlight. The in-app browser reported audio preview unsupported, so playback/seek was not claimed.

## Activity

Successful transcript query was retained. Failed HTTP 400 attempts were not recorded as analyst activity.

## Demo Readiness

`INTEGRATION_READY`. Processing and Data are live; complete Ask/time/citation/playback acceptance is not.

# Urdu STT

## Processing

Fresh FLEURS Urdu file SHA-256 `2a75e3b8c92153f3eafde4febbe4ed69ce1a36ae1ecfc7ceabfee708c1151b31` was uploaded through Add Data. Evidence `9e0aa4fb-663f-4e4d-9439-9c593d489be9` completed with two observations and ASR `COMPLETE_RESULTS`, duration 6.24 s.

## Transcript

The deployed model emitted one 0–6.0 s transcript segment classified as `hi`, in Devanagari, rather than raw Urdu script.

## Timing

One actual backend segment range is stored. Ask did not expose a reliable time citation/seek.

## Urdu

FAIL: raw Urdu Unicode/RTL was not produced for the fresh sample. Publisher text remains benchmark oracle only and was not substituted.

## Roman Urdu

FAIL: no Roman Urdu derivative was produced for the fresh sample.

## Data

PASS for honest display of the actual model output and `hi` classification; FAIL for the requested Urdu/RTL product behavior.

## Ask

A dynamically derived Devanagari phrase matched its own source. The requested Urdu/Roman/English equivalence matrix could not pass against missing Urdu-script output.

## Citation

Correct evidence/artifact on the positive phrase, but no source-time seek.

## Activity

Successful query retained; UI 400 no-match attempt absent from Activity.

## Demo Readiness

`BLOCKED` for this live Urdu product path.

## Accuracy Limitation

Known Urdu accuracy limitations remain substantial. No ASR search, tuning, fine-tuning, or model replacement was performed.

# ANPR Query Regression

Positive `MN1367` returned one exact result with the correct image, observation time, row/hash lineage, and truthful missing camera/location. Absent `ZZ99ZZZ` returned 20 unrelated rows. Source root cause: the generic plate extractor allowed only one trailing letter; it now allows zero to three, with a generic regression. Current deployed live acceptance is **FAIL** until a bounded later activation/retest.

# OCR Query Regression

Data showed `Investigation Workspace` and `Reference REF-2026-01` on `printed-english.png`. The selected-evidence exact Ask returned three unrelated acceptance-brief PDF citations. **FAIL / open P1**. OCR tuning was not reopened.

# Structured Query Regression

**PASS**. `show frequent contacts for 923001110001` returned eight deterministic contacts from `seed_cdr_large.csv`. A time/filter/aggregate query for 2026-04-01 through 2026-04-03 returned 193 events and daily counts 19, 84, and 90 with cited structured evidence. No unrestricted SQL was used.

# Data / Ask Consistency

**FAIL** because OCR Data displayed X while selected-evidence Ask cited unrelated Y, and absent ANPR returned unrelated plates.

# Citation Integrity

Structured CDR and positive image ANPR citations were correct. Positive STT source identity was correct but time/seek was absent. OCR and cross-evidence citations did not prove the requested claim. Overall **FAIL**.

# SigLIP Image Similarity

## Fresh Embedding

Not proven in this phase; retained embeddings were used. No model download occurred.

## Ranking

Live retained ranking PASS after loading all sources: SFace A-blur 0.8775, A-pose 0.8419, subject-B 0.3093; SigLIP A-pose 0.9668, A-blur 0.9358, subject-B 0.7852, no-face street 0.4630.

## Data

Comparison/ranking works, but default page-only candidates can misleadingly show zero eligible candidates until all source pages are loaded.

## Ask

Not available/proven.

## Citation

Technical NexusAI URIs were shown; analyst-friendly clickable source navigation was not proven.

## Activity

Rank/compare actions did not appear in Activity.

## Semantics

PASS: cosine/visual similarity was explicitly described as not identity. Frontal-versus-pose comparison returned exact duplicate No, perceptual 0.7813, semantic 0.9668, near-duplicate No.

## Demo Readiness

`INTEGRATION_READY`; full live acceptance **FAIL**.

# Face Candidate Comparison

## Detection

Retained face crop/provenance and one-face/zero-face behavior were visible.

## Comparison

Live Data comparison/ranking worked on controlled safe images.

## Data

PASS with loaded-page limitation.

## Ask

Not available/proven.

## Citation

No analyst-friendly clickable Ask citation proof.

## Activity

Comparison did not appear in Activity.

## Safety

PASS: similarity was not presented as identity or demographic inference. No cross-case candidate was observed.

## Demo Readiness

`INTEGRATION_READY`; `PASS_WITH_LIMITATION` applies only to the Data comparison, not the complete requested flow.

# Cross-Evidence Queries

**FAIL**. The query combining `923001110001` and `MN1367` returned PDF/notes results, omitted the CDR source, and did not keep each claim tied to its authoritative family source. No identity inference was made.

# Simple UI / UX

Home/Data/Ask/Activity are visually clean and understandable, including mobile navigation. Correctness failures, raw technical transcript URI, missing no-match display after inherited-context error, page-limited ranking, and missing time seek make the current Ask experience **FAIL** overall.

# Responsive Acceptance

## 390

PASS: mobile navigation, cards, and actions remained readable without visible horizontal clipping.

## 820

PASS: Data filters, Add Data, and source list remained readable.

## 1024

PASS: Ask result and composer remained readable.

## 1440

PASS: Activity filters, cards, status, and reopen links remained readable.

# Activity UX

Successful Ask operations are retained and reopenable. Wrong OCR/cross-evidence answers are consistently retained as “Results available,” while HTTP 400 attempts and image/face compare actions are absent. **PARTIAL**.

# Team-Lead Demo Workspace

The existing workspace now contains 35 sources: 34 ready and one preserved failed source, including fresh English and Urdu audio. It is suitable for bounded Data demonstrations and positive CDR/ANPR/English transcript examples, but not for an unqualified shared-Ask, Urdu, OCR exact, cross-evidence, similarity, or face end-to-end demo.

# TTS English Admission

Still not acquisition-ready. The MIT `en_US-ljspeech-medium` voice is a single-speaker 22.05-kHz model; pinned `voices.json` lists a 63,531,379-byte ONNX plus 4,972-byte config. The original MIT Piper repo is archived and points to the maintained GPL-3.0 `piper1-gpl` runtime, which embeds eSpeak-NG. Exact runtime revision/digest, complete packaged-license notices/SBOM, extracted installed size, resource envelope, LocalAI configuration, and rollback remain unresolved. No download occurred. Sources: https://huggingface.co/rhasspy/piper-voices/blob/main/en/en_US/ljspeech/medium/MODEL_CARD, https://huggingface.co/rhasspy/piper-voices/blob/661e39caff61dcfcbbd2178a7c2dd0144b858b0e/voices.json, https://github.com/OHF-Voice/piper1-gpl

# TTS Urdu Admission

Still not acquisition-ready. `AhsanTalal/urdu-matcha-tts` is Apache-2.0, single-speaker Urdu grapheme input, produces 80-band mel at 22.05 kHz, and documents `matcha_tts.onnx` opset 17 plus `charactr/vocos-mel-22khz`. The required Vocos artifact/revision/hash/license/size could not be authoritatively resolved, so compatibility and lawful dependency closure remain blocked. No 24-kHz substitution and no download occurred. Source: https://huggingface.co/AhsanTalal/urdu-matcha-tts

# LocalAI Status

Current aliases/backends were preserved. No LocalAI update, restart, model download, or profile change occurred.

# Demo Readiness Matrix

| Capability | State |
|---|---|
| English STT | INTEGRATION_READY |
| Urdu STT | BLOCKED |
| Roman Urdu | BLOCKED |
| Structured CDR | LIVE_DEMO_READY_WITH_LIMITATION |
| Image ANPR exact | INTEGRATION_READY (source fix pending activation) |
| OCR exact | BLOCKED |
| Image similarity | INTEGRATION_READY |
| Face comparison | INTEGRATION_READY |
| Cross-evidence | BLOCKED |
| English TTS | BLOCKED |
| Urdu TTS | BLOCKED |

# Certification Matrix

Certification remains unchanged: 62 REGISTERED, 12 SOURCE_VALIDATED, one FIXTURE_CERTIFIED, zero REAL_WORLD_CERTIFIED, and four PRODUCT_CERTIFIED. This phase performed no product certification.

# Accuracy / Optimization Backlog

Urdu ASR script/accuracy, ANPR/OCR recognition quality, image/face model benchmarking, and legacy Activity Markdown remain post-breadth optimization work. They were not reopened.

# Files Changed

- `core/http/react-ui/src/analyst/analystAskPresentation.js` and test: scope-safe, anaphoric-only context inheritance.
- `core/http/react-ui/src/pages/AgentChat.jsx`: attach originating portal scope to final agent messages.
- `core/services/agents/forensic_direct.go` and `records_tools_test.go`: generic plate extraction for up to three trailing letters.
- this report, continuation checkpoint, vNext phase ledger, demo readiness matrix, and team-lead runbook.

# Tests

- Analyst presentation unit tests: 7/7 PASS.
- Focused Go target-extraction spec: PASS.
- Full agent package: 111/125 specs PASS; 14 environment-only failures because Windows testcontainers reported rootless Docker unsupported.
- Browser E2E attempt: NOT RUN to assertion; configured web-server path could not start.
- Real browser: uploads, Data, Ask, citations, Activity, rankings/comparisons, and four responsive widths exercised.
- No full build or redeploy.

# Manual Browser Results

Real in-app browser observations are the authority for live labels in this report. Audio playback/seek is explicitly unproven because the browser reported preview unsupported.

# Runtime Resource Results

Approximate observed RAM: host floor 5.10 GiB available at baseline; worker 598 MiB, API 1.25 GiB, forensic API 17 MiB during STT observation. No OOM or restart. The 6-GiB build gate was not applied to ordinary UI acceptance.

# Retained Impact

Exactly two approved audio files were added and live acceptance questions were retained. Final workspace count: 35 total, 34 ready, one preserved failed. No existing evidence/observation/history was deleted or overwritten; no bulk reprocessing, migration, or volume change.

# Open P0

None observed. No cross-case leakage was observed; a dedicated cross-case-denial mutation was not added after the host denied further privileged diagnostics.

# Open P1

1. Source-fixed, not deployed: conversation context must remain within identical portal scope and only apply to real anaphoric follow-ups.
2. Source-fixed, not deployed: explicit ANPR plates with two/three trailing letters must be extracted; deployed absent query currently returns unrelated rows.
3. OCR selected-evidence exact lookup returns unrelated PDF evidence.
4. Transcript citation omits source start/end and cannot seek/highlight.
5. Cross-evidence phone-plus-plate routing omits authoritative structured evidence.
6. Urdu fresh output is Devanagari/`hi`, with no raw Urdu or Roman derivative.

# EXACT NEXT ACTION

Complete and test bounded source corrections for OCR selected-evidence exact execution, transcript locator start/end propagation, and cross-family composed routing. Then prepare **one** separately approved API/UI plus forensic-API activation containing all tested P1 fixes, using the existing 6-GiB deployment gate and rollback protections. Do not rebuild now, retune models, bulk reprocess, download TTS, certify the product, or activate NX-B2.

```text
ActivationAlreadyComplete=true
ActivationReceipt=20260901T154458190Z
SharedAskLive=FAIL
SelectedEvidenceScopeLive=FAIL
ExactNoMatchLive=FAIL
TimeBoundTranscriptLive=FAIL
CurrentVersionLive=PASS
EnglishSTTLive=FAIL
UrduSTTLive=FAIL
RomanUrduLive=FAIL
STTCitationLive=FAIL
ANPRExactLookupLive=FAIL
OCRExactLookupLive=FAIL
StructuredQueryLive=PASS
DataAskConsistencyLive=FAIL
ImageSimilarityLive=FAIL
FaceCandidateComparisonLive=PASS_WITH_LIMITATION
CrossCaseLeakage=NONE
SimpleAskUX=FAIL
Responsive390=PASS
Responsive820=PASS
Responsive1024=PASS
Responsive1440=PASS
EnglishSTTDemoReadiness=INTEGRATION_READY
UrduSTTDemoReadiness=BLOCKED
ImageSimilarityDemoReadiness=INTEGRATION_READY
FaceCandidateComparisonDemoReadiness=INTEGRATION_READY
TTSEnglishAcquisitionReady=false
TTSUrduAcquisitionReady=false
NoHardcodedAnalyticalAnswers=PASS
NoSelfOracle=PASS
NoUnrestrictedSQL=PASS
ProductCertificationPerformed=false
NXB2ActivationPerformed=false
```
