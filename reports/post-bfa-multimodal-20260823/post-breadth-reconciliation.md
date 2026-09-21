# NexusAI post-BF-A advanced multimodal reconciliation

Date: 2026-08-23  
Scope: tenant `default`, collection/case `nexusai-breadth-acceptance-20260820`  
Verdict: `POST_BREADTH_RECONCILED_LIMITED_RETAINED_PROOF_GATED`

# Verified Starting State

The worker already contained FastALPR, Urdu ASR, deterministic media metadata,
dHash/comparison, Roman-Urdu and native-document source paths. PostgreSQL and
NATS were healthy and the BF-A case was on review hold. Missing active roles
were a general image embedding, general English/Urdu OCR, and a bounded face
model. No database migration was needed.

# BF-A Preserved State

Final read-only accounting is `27 evidence | 27 versions | 27 jobs | 12,932
records | 15 artifacts`; jobs are 24 completed and 3 dead-letter. PostgreSQL
container `f66e05a3b179...`, NATS `5c49e70d132a...`, and worker
`d7605c33967c...` were preserved. No retained upload, reprocess, deletion,
cleanup, migration, model-volume replacement, stage, commit, push or PR ran.

# Existing Model Inventory

- FastALPR ONNX CPU remained the plate role.
- `faster-whisper-small-ur` remained the Urdu ASR role with explicit `ur` when
  language is known.
- LocalAI text models, `qwen3-embedding-0.6b`, and all existing model volumes
  were preserved; the text embedder was not misused as an image embedder.

# Model Role Audit

## ANPR

Existing FastALPR was suitable and reused; no replacement model was acquired.

## Face Detection

YuNet from the LocalAI `face-detect-yunet-sface` package passed detection M1.

## Face Embedding

SFace 128-dimensional output passed relative same/different fixture ranking.
Its only authorized meaning is `candidate visual similarity; not identity`.

## Image Embedding

No configured general image encoder existed. SigLIP was selected as one compact
matched image/text encoder and is limited to bounded explicit candidate sets.

## OCR

FastALPR OCR is plate-specific. Tesseract 5 with official `eng+urd` fast data
was activated as a distinct, review-required general scene-text floor.

## ASR

`faster-whisper-small-ur` is active. Known Urdu is admitted with explicit `ur`.
Spoken identifier preservation remains `PENDING_M2`.

## Transliteration

Deterministic identifier-safe Roman Urdu is active as a derivative. Raw Urdu
remains authoritative.

## Documents

Bounded native TXT, text-bearing PDF and DOCX extraction is active. Scanned
PDF OCR, complex tables and geometry are not claimed.

# Model Acquisitions

## Role

Face detection and evidence-scoped face embedding.

## Model

LocalAI `face-detect-yunet-sface` (YuNet + SFace).

## Source

LocalAI gallery package backed by OpenCV Zoo assets.

## License

Apache-2.0; the downloaded OCI backend lacked signature verification, which
remains a recorded supply-chain limitation.

## Revision

Pinned gallery artifact and backend digest
`sha256:8d303ecedb5e6f4dd2c5a0023da806ac3f8ceef369ff83628f24d99d1a0f3f19`.

## SHA256

`9ce78d4ba0ae9d5e8c91a0e145d511558d1d90f5d9c1f4131cca9bb4bce60902`.

## Size

26,073,536-byte GGUF; approximately 52 MB including the backend.

## Local Path

Docker volume path
`/var/lib/docker/volumes/nexusai_models/_data/face-detect-yunet-sface.gguf`.

## Runtime Path

LocalAI `/models/face-detect-yunet-sface.gguf`; worker invokes LocalAI
detection and stores only source-bound observations.

## Reason

One CPU package supplied bounded detection and 128-dimensional candidate
embedding without a gallery, identity classifier or demographic output.

## Benchmark

Six synthetic fixtures returned counts `1/1/1/1/4/0`; same `0.8368`, blurred
same `0.8998`, different `0.2838`; cold 3,028 ms, warm about 335-622 ms.

---

## Role

General semantic image and matched text embedding.

## Model

`google/siglip-base-patch16-224`.

## Source

`https://huggingface.co/google/siglip-base-patch16-224`.

## License

Apache-2.0 model repository; runtime libraries retain their own notices.

## Revision

`7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed`.

## SHA256

`2c63cb7d1f2e95ba501893cbb8faeb4ea9a3af295498d35097126228659c2af8`.

## Size

812,672,320-byte weight; 815,871,927-byte complete local role.

## Local Path

`C:\Users\sheik\.cache\nexusai\models\google-siglip-base-patch16-224\7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed\model.safetensors`.

## Runtime Path

`/models/media/nexusai/models/google-siglip-base-patch16-224/7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed/model.safetensors`;
host cache `/models/media` is mounted read-only.

## Reason

One compact 768-dimensional matched encoder supports bounded image/image and
English text/image ranking without a generative VLM or a second checkpoint.

## Benchmark

Warm image inference 787-1,237 ms; English text-image top-1 1.0. Urdu street
top-1 failed and remains LIMITED.

---

## Role

General English/Urdu scene-text OCR, distinct from plate OCR.

## Model

Tesseract 5 with official `tessdata_fast` `eng+urd` integer LSTM assets.

## Source

`https://github.com/tesseract-ocr/tesseract` and
`https://github.com/tesseract-ocr/tessdata_fast`.

## License

Apache-2.0.

## Revision

`65727574dfcd264acbb0c3e07860e4e9e9b22185`.

## SHA256

English `7d4322bd2a7749724879683fc3912cb542f19906c83bcc1a52132556427170b2`;
Urdu `62e8250ce2a994106e313a82e26a516a39e2cf159d0ce3c5b5008387fd0d555f`.

## Size

English 4,113,088 bytes; Urdu 1,398,718 bytes.

## Local Path

`C:\Users\sheik\.cache\nexusai\models\tessdata-fast\65727574dfcd264acbb0c3e07860e4e9e9b22185\eng.traineddata`
and adjacent `urd.traineddata`.

## Runtime Path

`/models/media/nexusai/models/tessdata-fast/65727574dfcd264acbb0c3e07860e4e9e9b22185/eng.traineddata`
and adjacent `urd.traineddata`; the parent host cache is mounted read-only and
the executable is invoked without shell expansion.

## Reason

Small CPU-only baseline with word boxes/confidence and both Latin and Urdu
script support; FastALPR remains the plate role.

## Benchmark

312-428 ms; English CER 0.0189, Urdu CER 0.0741; blank returned zero and
corrupt/oversized controls failed closed.

# ANPR

## Runtime

FastALPR ONNX CPU is active in the existing worker.

## Retained

No new retained ANPR artifact was created. The old BF-A image retains only its
technical image observation until separately approved immutable reprocessing.

## Accuracy Fixture

User-owned `test_plate.jpg` returned normalized plate `MN1367`, detector
confidence 0.8897 and OCR confidence 0.9998 in a non-retained run.

## Data

The drawer displays the immutable image preview, artifact citation and current
capability actions; old job warnings are labelled recorded processing notes.

## Ask

Plate prompts appear only where retained artifact contracts support them.

## Citation

Regions/crops are source-, version- and hash-bound; no retained positive was
fabricated for this phase.

## UI

Data and Home expose the live plate capability without claiming a new retained
plate observation.

# Image Intelligence

## Metadata

Bounded image admission, orientation-only metadata and source preview remain
active.

## OCR

Tesseract English/Urdu regions, confidence and script-family candidates are
LIMITED and manual-review required; it never replaces plate OCR.

## Exact Duplicate

Exact source SHA-256 comparison is active. No separately registered retained
duplicate pair was authorized.

## Near Duplicate

64-bit dHash with Hamming threshold 10 is active as a review candidate. A
lawful retained transformed pair remains pending.

## Semantic Search

SigLIP 768-dimensional similarity is bounded to explicit case evidence IDs.
No unrestricted corpus scan or pgvector migration was introduced.

## Comparison

The read-only two-image comparison API reports hashes, dimensions, dHash and
available derived signals with source citations.

## Performance

SigLIP cold checksum/model construction 14.3 s; warm image inference 0.79-1.24
s on the reference CPU. OCR was 0.31-0.43 s.

## Negative Tests

Blank OCR returned zero observations; corrupt and oversized inputs failed.
Similarity requests fail on missing scope/candidates, cross-tenant scope and
absent evidence.

# Face Intelligence

## Detection

YuNet produces bounded original-pixel face boxes or truthful zero observations.

## Crops

Lossless crops carry parent evidence/version/hash and original coordinates.

## Embeddings

SFace produces 128-dimensional model observations with model/backend provenance.

## Candidate Similarity

Every score is `candidate visual similarity; not identity`.

## Search Scope

One tenant, one collection/case and 1-200 explicitly enumerated candidate
evidence IDs are mandatory.

## Security

No global gallery, identity lookup, demographic inference or cross-tenant scan.

## Performance

Cold detection 3,028 ms; warm about 335-622 ms; measured process memory rose
from 42.38 MiB to 338.3 MiB.

## Negative Tests

No-face returned zero. Missing explicit scope 403, missing candidate list 400,
cross-tenant 403 and fully scoped absent query 404.

# Roman Urdu

## Raw Urdu

Raw Urdu transcript is immutable and authoritative.

## Roman Urdu

The `forensics.audio-roman-urdu-segment/v1` derivative is active in disposable
and source tests; no retained derivative was added.

## Identifier Safety

Disposable control preserved `03001234567`, `MN1367` and `10:35`. Spoken ASR
identifier accuracy still remains M2.

## Search

Roman search is advertised only after that exact artifact contract is retained.

## Ask

Urdu, Roman Urdu and English planning paths retain source scope and limitations.

## UI

The Data card shows Roman Urdu beside authoritative raw Urdu only when present.

# Audio

Existing five timestamp-segment and five audio-observation artifacts remain the
retained truth. ASR is active; diarization is unavailable.

# Video

Retained video metadata/frame/audio/timestamp contracts remain LIMITED. No
tracking, identity, event certainty or new retained derivative was introduced.

# Documents

## TXT

Bounded UTF-8/native text and stable section locators are active.

## PDF

Text-bearing PDF uses bounded native extraction. Scanned-only PDF returns an
explicit limitation; no OCR/table reconstruction is claimed.

## DOCX

The user-owned HP guide produced 112 passages and 5,371 characters in a
disposable network-disabled proof; the source hash was unchanged.

## KB

Native passages can be mirrored into existing KB/search contracts only through
an authorized immutable evidence job.

## Citation

Passages carry evidence, version, source hash, filename and paragraph/page or
section locator.

# KB

No new retained KB row was written. Existing seven BF-A mirrors are preserved.

# Ask NexusAI

Capability discovery is now driven by exact persisted artifact types. The live
case suggests image metadata/recorded plate candidates, timestamped Urdu audio
and document metadata, but not unretained OCR, SigLIP, face, Roman or passage
claims.

# History

History reopens saved answers read-only and supports stored semantic/fact-packet
provenance. The reviewed legacy BF-A item truthfully reports `citation state not
reported`; no citation was invented and reopening created no evidence job.

# Analyst Portal

## Home

Live evidence readiness, processing state, recent sources/analyses and
artifact-driven suggestions render without overflow.

## Data

Image/audio/video previews and source-bound derived-intelligence cards render.
Historical immutable warnings are separated from current live readiness.

## Ask

Everyday-language entry, case context and grounded-analysis assurances render;
no query was submitted during this read-only browser acceptance.

## History

Saved analyses, direct answer, continue link and provenance disclosure render.

# Responsive Acceptance

## 390

Home/Data/Ask/History: main and navigation present; no horizontal overflow.

## 820

Home/Data/Ask/History: main and navigation present; no horizontal overflow.

## 1024

Home/Data/Ask/History: main and navigation present; no horizontal overflow.

## 1440

Home/Data/Ask/History: main and navigation present; no horizontal overflow.

# Security Acceptance

Authentication, trusted tenant, case scope, RLS/current-version selection,
explicit candidate lists and read-only comparison boundaries pass. No runtime
secret was printed. Retained state was queried only in rolled-back/read-only
transactions.

# Performance Matrix

| Role | Observed reference result | Verdict |
| --- | --- | --- |
| FastALPR | `MN1367`; detector 0.8897, OCR 0.9998 | LIMITED runtime pass |
| Face | cold 3.03 s; warm 0.34-0.62 s | detection M1; similarity LIMITED |
| SigLIP | warm 0.79-1.24 s; 768d | M1_ACCEPTABLE_LIMITED |
| Tesseract | 0.31-0.43 s; EN CER 0.0189, UR CER 0.0741 | M1_ACCEPTABLE_LIMITED |
| DOCX | 112 passages, 5,371 chars | LIMITED runtime pass |
| Portal | 16 route/width combinations | PASS, no overflow |

# Model Manifest

Exact sources, revisions, licenses, sizes and hashes are recorded above and in
`model-role-proposals.md` plus the two benchmark JSON files. Total newly
acquired model assets stayed below the authorized 2 GB ceiling.

# Operation Registry

Catalog `2026-08-23.post-bfa-runtime.1` exposes 97 operations. Image OCR,
visual similarity, face, Roman Urdu and native document operations are LIMITED.
Suggestions are further filtered by exact retained artifact contracts.

# Demonstration Guide

## Exact Path

`docs/demo/nexusai-breadth-multimodal-demo-guide.md`.

## Exact Test Data Paths

The guide records the absolute plate, Islamabad image, three audio, video,
DOCX, TXT and synthetic face fixture paths plus their hashes/licenses.

## Exact PowerShell

The guide includes readiness, authenticated inventory, operation registry,
comparison, negative and focused verification commands.

## UI Steps

Home -> Data drawers -> Ask -> History -> four-width replay.

## Expected Results

Each role has a positive, no-result and retained-proof-gated expectation.

## Negative Tests

Scope denial, no face/plate/text, corrupt/oversized input, absent phrase and
legacy citation absence fail closed.

## Troubleshooting

The guide separates role/model absence, scope errors, PDF limitations, legacy
citations and layout problems.

## Rollback

Rollback tags exist for each rebuilt service generation. Restore the recorded
tag and recreate only the affected service; do not touch data volumes.

# Complete Feature Acceptance Matrix

| Feature | Source | Runtime | Retained proof | Final |
| --- | --- | --- | --- | --- |
| ANPR | PASS | PASS | GATED | LIMITED |
| Image metadata/hash/compare | PASS | PASS | existing technical only | LIMITED |
| SigLIP similarity | PASS | PASS | GATED | M1_ACCEPTABLE_LIMITED |
| General OCR | PASS | PASS | GATED | M1_ACCEPTABLE_LIMITED |
| Face detection | PASS | PASS | GATED | M1_AVAILABLE |
| Face candidate similarity | PASS | PASS | GATED | LIMITED |
| Urdu ASR | PASS | PASS | existing BF-A | LIMITED M1 |
| Roman Urdu | PASS | PASS | GATED | LIMITED |
| Native documents | PASS | PASS | GATED | LIMITED |
| Portal responsive | PASS | PASS | read-only BF-A | PASS |
| Legacy History citation display | PASS | PASS | citation state absent | LIMITED |

# P0

None open.

# P1

None open inside the authorized non-retained activation slice. A retained proof
gate is an authorization boundary, not an implementation defect.

# P2

Urdu/Roman-Urdu text-image retrieval quality; lawful near-duplicate retained
fixture; legacy History entries without stored citation state; scanned PDF and
table/geometry extraction; spoken plate/identifier preservation.

# P3

General VLM description, broad object/event inference and advanced video
analytics remain outside this breadth slice.

# ModelDownloadNeeded

NO. The bounded approved acquisitions are complete.

# DatabaseMigrationNeeded

NO.

# DeploymentNeeded

NO for this source/runtime slice; LocalAI, forensic API and worker activation is
complete. Retained proof would require separately approved job execution, not a
new deployment.

# RetainedMutationNeeded

YES, only to promote the newly activated roles from non-retained/runtime proof
to retained case proof. It is not authorized by this report.

# Files Changed

Bounded changes cover the forensic worker media/document/Roman pipelines,
forensic API scoped compare/similarity/capabilities, model and modality
catalogs, Analyst Home/Data/Ask/History presentation, focused tests, benchmark
receipts, this report, the demo guide, backlog and continuation ledger. The
worktree also contains extensive pre-existing user changes that were preserved.

# Exact Next Operator Actions

1. Review this report, the two benchmark JSON files and the updated demo guide.
2. Review the proposed retained-proof manifest before authorizing any job.
3. If desired, issue a separate approval naming exact evidence IDs, allowed
   operation/reprocess count, actor, case, retention and review requirements.
4. Do not delete/clean BF-A data or begin P2/P3 work under that approval.

# Post-Breadth Reconciliation Boundary

STOP. Source, non-retained benchmarks, narrow runtime deployment and read-only
browser acceptance are reconciled. New retained OCR, embedding, face, Roman
Urdu, document or duplicate artifacts remain behind a separate explicit
approval. No cleanup is authorized.
