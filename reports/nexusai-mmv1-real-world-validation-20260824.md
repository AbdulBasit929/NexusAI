# NexusAI MMV-1 real-world validation and maturity baseline

Date: 2026-08-24  
Scope: non-retained benchmarks, reconciliation, and bounded P1 source fixes  
Retained upload/reprocess: not performed  
Model download: not performed  
Database migration: not performed  
Deployment: not performed

## Verified Starting State

The accepted checkpoint was `21 evidence | 21 versions | 34 jobs (33
completed, 1 dead-letter, 0 active) | 9,274 canonical records | 372 artifacts |
19 KB assets`. A fresh read-only database query found later external drift:
`51 evidence | 51 versions | 64 jobs (60 completed, 4 dead-letter, 0 active) |
22,207 canonical records | 441 artifacts | 47 KB assets`. The product collection
contains 24 evidence and 37 jobs; three images created after the accepted
checkpoint explain its evidence-count increase. MMV-1 caused none of this
retained change.

## Services

| Service | Image | ID | Health | Ports |
|---|---|---|---|---|
| LocalAI | `nexusai/localai-forensic:phase3-runtime` | `e04dae4cbd44` | healthy; `/readyz` 200 | 8080 |
| Forensic API | `nexusai/forensic-records-api:phase3-runtime` | `a97e545b8356` | running; unauthenticated `/health` truthfully returns 401 | 8091 |
| Worker | `nexusai/forensic-records-worker:phase3-runtime` | `0b2418d90cbd` | healthy | 9109 |
| PostgreSQL | `timescale/timescaledb:latest-pg16` | `f66e05a3b179` | healthy | 5433→5432 |
| NATS | `nats:2.11-alpine` | `5c49e70d132a` | healthy | 4222, 8222 |

The same protected IDs were present before and after MMV-1. No service was
recreated or restarted.

## MMV-1 Reconciliation

All requested families are represented in the generated family matrix. The
existing 66-operation contract is preserved, requested MMV aliases/gaps are
truthfully separated from implemented operations, and no accepted structured
phase was replayed. The source-to-answer maturity chain is present but remains
partial for media operation/query certification. P0=0 and open foundational
P1=0 after the two bounded source corrections.

## Family Maturity Matrix

The complete machine-readable matrix is
`mmv1-real-world-validation-20260824/family-maturity-matrix.json`.

| Family | Processor | Model | Data/UI | Operations/Ask/Queries | Citations/real-world test | Maturity | Gap | Next |
|---|---|---|---|---|---|---|---|---|
| Structured | schema/canonical worker | deterministic adapters | accepted | catalog coverage; bounded | source-bound; accepted | M3 partial | cross-family breadth | MMV-9 |
| CDR | CDR adapters | deterministic | accepted | 11 ops; accepted bounded | multi-schema accepted | M3 | provider breadth | MMV-9 |
| IPDR | IPDR adapter | deterministic | accepted | 9 ops; partial multilingual | source-bound; limited schema breadth | M2 | multi-schema | MMV-9 |
| Subscriber | subscriber adapter | deterministic | accepted | 8 ops; partial multilingual | source-bound; synthetic | M2 | provider breadth | MMV-9 |
| Tower | tower adapter | deterministic | accepted | 8 ops; partial multilingual | source-bound; synthetic | M2 | real exports | MMV-9 |
| ANPR | FastALPR/adapter | YOLO v9 + CCT XS v2 | positive non-retained; retained image accepted | 10 ops; target gaps | bbox/crop/time; `sample.mp4` | M1 | robustness/sampling | MMV-2 |
| General image | image pipeline | Tesseract + SigLIP | accepted limited | 6 ops; bounded | bbox/hash/artifact | M1 | real OCR/semantic corpus | MMV-2 |
| OCR | general OCR | Tesseract `eng+urd` | Urdu inadequate | `image.ocr_search` | word bbox; MMV pack | M1 limited | CER 0.436842 | MMV-3 |
| Face | face pipeline | YuNet + SFace | candidate-only | 3 ops; bounded | bbox/crop/model | M1 | lawful diversity/quality | MMV-4 |
| Visual similarity | embeddings | SigLIP | accepted limited | one limited op | candidate provenance | M1 | calibration | MMV-4 |
| Audio | ASR/Roman Urdu | faster-whisper-small | 3-fixture measured; source UI fixed | 2 ops; target gaps | time segments | M1 | code-switch/noise/identifiers | MMV-3 |
| Video | composed sampling | specialist stack | positive non-retained | 2 ops; target gaps | frame/time/bbox/crop | M1 | sampling/tracking/public ops | MMV-2 |
| Documents | native extract/KB | native/Tesseract | accepted limited | 2 ops; target gaps | page/passage where available | M1 | scanned Urdu/layout | MMV-5 |
| KB | hybrid retrieval | configured roles | accepted | evidence-scoped | source/chunk citations | M2 | document-family contract | MMV-5 |

## Positive Video ANPR

- Path: `C:\Users\sheik\Downloads\sample.mp4`
- SHA-256: `d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee`
- Metadata: 184,407,144 bytes; 60.010 seconds; H.264 3840×2160;
  60 FPS; 3,600 frames; AAC audio.
- Previous reference: a standalone YOLOv8/custom plate/SORT/EasyOCR experiment
  was described by the operator, but no local standalone output artifact was
  found. It is not treated as ground truth.
- Current result: six frames at 0/5/10/15/20/25 seconds; 549 observations; no
  processor failure.
- Plate: `LN15ZZC`, unique once, 25 seconds, bbox x=1045 y=1818 w=176 h=60.
- Confidence: detector `0.7814836`; OCR `0.9998574`.
- Crop SHA-256:
  `195c2abe7cb2faad5c67b4b40328d36dcedbbda8a69a48644159236400af3274`.
- Backend: FastALPR `yolo-v9-t-384-license-plate-end2end` plus
  `cct-xs-v2-global-model`.
- Latency: 73.244 seconds.
- Verdict: `VideoANPRPositiveControl=PASS_NON_RETAINED`.

The persisted report is
`mmv1-real-world-validation-20260824/sample-video-anpr-nonretained.json`,
1,467,322 bytes, SHA-256
`da880be4ddd87422a25a562f8b6a8d71ec6366c1281d3758c6de2dd3500a3fba`.
The plate is a high-confidence model observation pending human ground-truth
review; it is not an identity/ownership claim.

## Urdu OCR

The locally generated, separately grounded pack contains a heading, paragraph,
mixed Urdu-English, Urdu/numbers/date/time, scene-style text, and a no-text
negative. Current unchanged Tesseract 5 `eng+urd`, PSM 11 produced aggregate
non-negative CER `0.436842`; the negative passed. Heading CER was 0, paragraph
0.655738, mixed 0.468750, numbers/date 0.372093, and scene 0.480000. Box output
was produced, but coverage and RTL reading quality were materially deficient:
paragraph text was omitted/corrupted, mixed text lost content, the date order
changed, and the time was dropped.

Current model: accepted only as a limited general OCR baseline. Challenger
needed: yes, MMV-3. Official PaddleOCR documentation explicitly lists Urdu and
English for `arabic_PP-OCRv5_mobile_rec`; it is a justified single challenger,
not an accepted model. No download occurred. See the benchmark JSON and model
admission inventory.

## Audio Real-World Validation

Two lawful FLEURS `ur_pk` read-speech samples (different sample/speaker IDs, CC
BY 4.0) and one local synthetic Urdu-English identifier control were processed
non-retained by the current stack.

| Fixture | WER | CER | Latency | RTF | Identifier result |
|---|---:|---:|---:|---:|---|
| FLEURS Urdu test row 1 | 0.6000 | 0.1852 | 6.630 s | 1.06255 | n/a |
| FLEURS Urdu validation row 4 | 0.3333 | 0.1446 | 5.993 s | 0.57077 | n/a |
| Synthetic mixed identifier | 0.9231 | 0.9022 | 6.750 s | 0.86782 | phone/time pass; `MN1367` fail |

This pack establishes pipeline behavior only. Natural Pakistani English,
natural code-switching, spontaneous Urdu, moderate noise, and natural spoken
identifier coverage remain lawful-acquisition gates.

## Audio UI/Icon

The source row now derives audio identity from family/MIME before extension,
uses a waveform icon, and visibly labels the source `Audio`. Detail presentation
shows the native player, duration, format, language, artifact-derived ASR
status, model, timestamped transcript count, Roman Urdu count, provenance and
relevant Ask actions. It does not render image/video controls. Sub-minute media
timestamps now use two-digit seconds. Source is complete; deployment/live
browser acceptance has not occurred.

## Model Inventory

The complete inventory is
`mmv1-real-world-validation-20260824/model-inventory-admission-matrix.json`.
FastALPR, YuNet/SFace, SigLIP and faster-whisper-small retain their bounded
accepted roles. Tesseract remains accepted-limited for general OCR, not mature
Urdu OCR. A general VLM replacement is not justified.

## Model Gaps

- Urdu OCR: `CHALLENGER_NEEDED_M2` — PaddleOCR Arabic PP-OCRv5; admission
  fields revision/hash/runtime memory/fixture accuracy are intentionally empty
  until a download is separately authorized.
- ASR identifiers and Pakistan speech diversity: `CHALLENGER_NEEDED_M2`, but
  corpus expansion precedes another model decision.
- Diarization/document layout: `RESEARCH_M3`.
- Advanced general VLM: `NOT_JUSTIFIED`.

## Operation Inventory

The repository contract contains 66 operations across the current families.
The complete required-field inventory is
`mmv1-real-world-validation-20260824/operation-registry.json`. Requested MMV
targets are explicitly marked implemented, limited, engineering-only, alias
candidate, planned, or missing. Processor observations do not automatically
become public certified operations; notably video ANPR/OCR/transcript artifacts
exist while those public family operations remain absent.

## Query/Template Inventory

`mmv1-real-world-validation-20260824/query-template-inventory.json` records a
bounded English, natural English, typo, Roman Urdu, Urdu, follow-up,
clarification and negative pattern for every target operation. These are
inventory templates, not certified runtime answers. Missing/alias operations
are `not_certified`; no demo-specific identifier or answer is embedded in a
generic handler.

## Gap Classification

- P0: 0.
- P1: 2 found and source-fixed — contradictory aggregate video-zero limitation;
  audio identity/readiness presentation. Open foundational P1: 0.
- P2: 9 — ANPR robustness, sampling, Urdu OCR, ASR identifiers, audio corpus,
  face corpus, scanned Urdu documents, operation gaps, query certification.
- P3: 4 — tracking, diarization, cross-modal timeline, advanced VLM.

The machine gap matrix is
`mmv1-real-world-validation-20260824/gap-backlog-matrix.json`.

## MMV Phase Assignment

MMV-2 owns ANPR/image/video sampling, robustness and positive retained proof.
MMV-3 owns Urdu OCR/ASR/corpora. MMV-4 owns face/visual calibration. MMV-5 owns
documents. MMV-6 owns operation expansion. MMV-7 owns multilingual query
certification. MMV-9 owns typed cross-family composition. MMV-12 owns tracking,
diarization and advanced reasoning.

## Source Tests

- Python syntax: media pipeline, media test, and three benchmark scripts pass
  `py_compile`.
- Video aggregate-zero direct regression smoke: pass.
- React presentation tests: 8/8 pass.
- React production build: pass; existing chunk-size/deprecation warnings only.
- Focused ESLint on touched UI files: 0 errors, 8 non-blocking unused-analysis
  warnings; full repository lint remains blocked by six unrelated pre-existing
  hook errors in `e2e/coverage-fixtures.js` and `src/pages/Chat.jsx`.
- Go forensic suite: 282 pass, 1 unrelated existing synthesis expectation fail,
  2 skip (`query_synthesis_test.go:53` expects an old deterministic phrase).
  No Go source was changed by MMV-1. `go vet ./api/forensic_records` completed
  without output after the test command.
- Host and worker Python environments do not contain pytest; no dependency was
  installed. Accepted ANPR/face/image/document baselines were not broadly
  replayed because their production behavior was untouched.

## Browser Tests

Focused mocked Chromium acceptance for MIME-derived audio row identity,
duration, format, language, artifact-derived transcript readiness, model,
transcript/Roman counts and native player: 1/1 pass. Live retained browser
acceptance is not claimed because deployment was not authorized.

## Runtime Health

All five protected services remain running with the same IDs. LocalAI is
healthy and ready. Worker, PostgreSQL and NATS are healthy. The forensic API is
running and correctly rejects an unauthenticated health request with 401.

## Retained State

No retained mutation was performed. Current live state is `51/51/64/60/4/0/
22,207/441/47` for evidence, versions, jobs, completed, dead-letter, active,
canonical records, artifacts and KB assets respectively. The product collection
contains 24 evidence. The supplemental video manifest is prepared but inactive.

## DeploymentNeeded

`YES` — only to activate and live-browser-test the two bounded source fixes.

## RetainedMutationNeeded

`YES` — only for future retained positive-video product proof. It is not needed
to establish the completed non-retained MMV-1 benchmark.

## ModelDownloadNeeded

`YES_FOR_FUTURE_MMV-3_URDU_OCR_CHALLENGER_ONLY`. No immediate MMV-2 model
download is justified.

## DatabaseMigrationNeeded

`NO`.

## Exact Approval Needed

Retained video approval, if desired later:

> I attest that NexusAI is authorized to retain and process
> `C:\Users\sheik\Downloads\sample.mp4` (SHA-256
> `d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee`,
> 184,407,144 bytes) and approve exactly one normal upload to tenant `default`,
> collection/case `nexusai-multimodal-product-acceptance`, solely as the positive
> video-ANPR control. Expected post-upload counts are 52 global evidence, 25
> collection evidence, and 65 global jobs. The unchanged non-retained run emits
> 549 observations, so expected global artifacts are 990 if every observation
> persists. This approval does not authorize reprocessing existing evidence,
> force upload, cleanup/deletion, model download, database migration, or service
> deployment.

Deployment and PaddleOCR acquisition require separate approvals with their
exact image/model revision, hash and rollback plan; neither is bundled into the
statement above.

## Team-Lead Demonstration Status

`READY_NON_RETAINED_SOURCE_DEMO`. Demonstrate the preserved video report,
measured Urdu/audio limitations, inventories and source UI build. Do not claim
the positive video is retained or the UI correction is deployed.

## Exact Next Phase

`MMV-2 — ANPR / Image / Video Maturity`: first improve/benchmark bounded frame
sampling over full-duration positive and negative videos, then—only if the
exact retained statement is approved—run one normal retained positive-video
proof and live Data/citation/browser acceptance. MMV-3 model acquisition must
not start implicitly.

## Evidence index

- `mmv1-real-world-validation-20260824/sample-video-anpr-nonretained.json`
- `mmv1-real-world-validation-20260824/urdu-ocr-baseline.json`
- `mmv1-real-world-validation-20260824/audio-benchmark.json`
- `mmv1-real-world-validation-20260824/real-world-test-data-manifest.json`
- `mmv1-real-world-validation-20260824/supplemental-video-retained-manifest-entry.json`
- `mmv1-real-world-validation-20260824/family-maturity-matrix.json`
- `mmv1-real-world-validation-20260824/operation-registry.json`
- `mmv1-real-world-validation-20260824/query-template-inventory.json`
- `mmv1-real-world-validation-20260824/model-inventory-admission-matrix.json`
- `mmv1-real-world-validation-20260824/gap-backlog-matrix.json`

Official model references: PaddleOCR PP-OCRv5 multilingual documentation
(`https://paddlepaddle.github.io/PaddleOCR/main/en/version3.x/algorithm/PP-OCRv5/PP-OCRv5_multi_languages.html`)
and Tesseract Urdu traineddata
(`https://github.com/tesseract-ocr/tessdata_fast/blob/main/urd.traineddata`).
