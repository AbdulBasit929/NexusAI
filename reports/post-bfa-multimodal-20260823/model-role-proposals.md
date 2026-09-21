# Post-BF-A bounded model-role proposals — 2026-08-23

This record precedes download and activation. It authorizes no retained evidence
mutation, schema migration, vector database, global gallery, or new service.
Candidate outputs remain model observations with source/version provenance and
must not be presented as deterministic facts.

## Semantic image embedding

| Field | Decision |
|---|---|
| Missing role | Cross-image semantic retrieval and, where the same encoder can be invoked, text-to-image retrieval |
| Existing-role audit | `qwen3-embedding-0.6b` is a text embedding role; FastALPR, dHash and face embeddings have narrower and non-interchangeable semantics. LocalAI has no configured general image-embedding endpoint. PostgreSQL has no `pgvector`; bounded explicit candidate-set cosine over JSONB is sufficient for this breadth slice. |
| Primary candidate | `google/siglip-base-patch16-224` |
| Publisher/source | Google checkpoint hosted at <https://huggingface.co/google/siglip-base-patch16-224> |
| Immutable revision | `7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed` |
| Weight artifact | `model.safetensors`, 813 MB as published; SHA-256 `2c63cb7d1f2e95ba501893cbb8faeb4ea9a3af295498d35097126228659c2af8` |
| License | Apache-2.0 model repository; runtime libraries retain their own notices |
| Why this candidate | One matched image/text encoder supports image retrieval and text-image retrieval without remote code or a generative VLM. Safetensors avoids the duplicate pickle checkpoint. |
| Expected footprint | 0.2B F32 parameters; weight asset below the 1 GB per-model ceiling. CPU load is expected to add roughly 1–2 GB RSS and must be measured before promotion. |
| Runtime boundary | Existing forensic worker only, lazy CPU load, one inference at a time, 224x224 publisher preprocessing, normalized vectors, no network access at inference. |
| Storage/query boundary | Persist versioned 768-dimensional observations in existing JSONB. Similarity queries require tenant, collection, optional case and an explicit bounded candidate-evidence set; no unrestricted corpus scan. |
| Safety/quality boundary | Scores mean candidate semantic visual similarity, not duplicate identity, face identity, object certainty, event certainty or evidence truth. English WebLI training makes Urdu/Roman-Urdu text queries LIMITED until measured. |
| Primary/secondary policy | Acquire this one primary only. OpenAI CLIP ViT-B/32 is not acquired because its own card describes the checkpoint as research output not developed for deployment; no backup is downloaded. |

Required promotion evidence: exact artifact checksum, dependency versions,
cold/warm latency, peak RSS, same-scene/paraphrase versus unrelated ranking,
image-to-image and English text-to-image checks, Urdu limitation, corrupt and
oversized abstention, and tenant/case/candidate-set denial tests.

Acquisition receipt (completed after this proposal was recorded):

- The only weight acquired was `model.safetensors`, 812,672,320 bytes, SHA-256
  `2c63cb7d1f2e95ba501893cbb8faeb4ea9a3af295498d35097126228659c2af8`.
- Six pinned processor/tokenizer/config files bring the complete local role to
  815,871,927 bytes. No `pytorch_model.bin`, secondary embedding model, remote
  code, or duplicate framework weights were downloaded.

## General English/Urdu scene OCR

| Field | Decision |
|---|---|
| Missing role | General raster scene-text extraction with regions and confidence; existing FastPlateOCR remains plate-specific and cannot be reused as scene OCR. |
| Primary candidate | Tesseract 5 runtime with official `tessdata_fast` `eng+urd` integer LSTM language assets |
| Publisher/source | <https://github.com/tesseract-ocr/tesseract> and <https://github.com/tesseract-ocr/tessdata_fast> |
| Immutable language-data revision | release `4.1.0`, commit `65727574dfcd264acbb0c3e07860e4e9e9b22185` |
| License | Apache-2.0 |
| Expected footprint | Prior isolated measurement: 15.477 MiB runtime/language-data and 19.055 MiB peak RSS; new language files are only a few MiB. Enforce `OMP_THREAD_LIMIT=1`, byte/pixel/timeout/observation ceilings. |
| Why this candidate | Small CPU-only baseline, no remote code, TSV supplies bounding boxes and confidence, and it covers English plus Urdu script. |
| Runtime boundary | Existing forensic worker only; invoke an explicit executable and pinned `TESSDATA_PREFIX`; never shell-expand evidence paths. |
| Result contract | Preserve raw line text, word/line bounds, confidence, detected script family, processor/runtime/language-data versions, source/artifact provenance and manual-review state. Script detection is not a language-identification claim. |
| Quality boundary | LIMITED. The prior R8 plate-crop evaluation rejected Tesseract as a production plate OCR primary (normalized exact 0.4286); this proposal does not reverse that result. It creates a distinct, review-required general scene-text floor. FastALPR remains the plate role. |
| Primary/secondary policy | Acquire Tesseract only. EasyOCR/PaddleOCR/PARSeq are not downloaded or promoted for this role. |

Required promotion evidence: checksum receipts for `eng.traineddata` and
`urd.traineddata`, English/Urdu/mixed synthetic goldens, empty/no-text and
corrupt-input abstention, box containment, raw-text preservation, latency/RSS,
no invented text in the blank control, and explicit plate-role separation.

Acquisition receipt (completed after this proposal was recorded):

- `eng.traineddata`: 4,113,088 bytes; SHA-256
  `7d4322bd2a7749724879683fc3912cb542f19906c83bcc1a52132556427170b2`.
- `urd.traineddata`: 1,398,718 bytes; SHA-256
  `62e8250ce2a994106e313a82e26a516a39e2cf159d0ce3c5b5008387fd0d555f`.
- Both came directly from the official repository at immutable revision
  `65727574dfcd264acbb0c3e07860e4e9e9b22185`; no secondary OCR model was
  acquired.

## Cumulative acquisition ceiling

- Previously acquired face model/backend: approximately 52 MB.
- Proposed SigLIP weight plus small configuration/tokenizer files: below 820 MB.
- Proposed Tesseract language assets: a few MiB; runtime packages are measured
  separately from model assets.
- Expected new model assets remain below the directive's 2 GB cumulative ceiling.

No retained upload, retained reprocess, migration, cleanup, or unrelated model
download is part of this proposal.
