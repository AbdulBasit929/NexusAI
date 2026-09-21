# NX-MMR model and backend challenger plan

The policy is incumbent recovery before challenger promotion, smallest useful
model before larger model, CPU-first execution, and one heavy role at a time.
The exact acquisition manifest is
`configuration/nexusai_multimodal_benchmark_manifest_v1.json`.

## Proposed isolated acquisition bundle

| Role | Artifact(s) | Bytes | Integrity before use |
|---|---|---:|---|
| FastALPR detector incumbent | YOLOv9-t 384 ONNX | 7,771,218 | publisher asset size; capture local SHA-256 and MIT notice |
| FastALPR OCR incumbent | CCT-XS-v2 ONNX + config | 3,346,017 | publisher asset sizes; capture local SHA-256 and MIT notice |
| Tesseract Urdu/English floor | `eng` + `urd` traineddata | 5,511,806 | pinned revision; known SHA-256 values in manifest; Apache-2.0 |
| PaddleOCR Urdu challenger | Arabic PP-OCRv5 inference archive | 8,151,040 | publisher ETag/size, local SHA-256 and packaged notice; Apache project |
| KB reranker challenger | Qwen3-Reranker-0.6B Q8 GGUF | 639,153,184 | revision `a02f48b...`, SHA-256 `22c9979c...`, Apache-2.0 |
| **Total** | seven files | **663,933,265** | isolated cache only |

The [official PaddleOCR table](https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/algorithm/PP-OCRv5/PP-OCRv5_multi_languages.en.md)
explicitly includes Urdu and English in the Arabic-script recognizer. The
[official Qwen reranker conversion](https://huggingface.co/ggml-org/Qwen3-Reranker-0.6B-Q8_0-GGUF)
is 639,153,184 bytes.

Acquisition does not mean installation or promotion. Unknown publisher hashes
are recorded immediately after download; size, URL, license or archive-member
mismatch fails closed.

## Role decisions

### ANPR

Recover the exact incumbent assets expected by current source, then rerun the
sealed image/video matrices. Do not add a new detector until the incumbent is
measured on independently labeled Pakistan evidence. Historical outputs are
useful baseline evidence but are not a substitute for current asset readiness.

### Urdu/English OCR

Tesseract `eng+urd` is the small CPU floor. Existing synthetic aggregate CER
was 0.436842 and is not sufficient for promotion. PaddleOCR Arabic PP-OCRv5 is
the challenger because its official language list includes Urdu. The
challenger must run in an isolated pinned evaluator; it is not added to the
worker image until accuracy, latency, peak RAM, supply chain and notices pass.

### Retrieval/reranking

Keep installed Qwen3 Embedding 0.6B and Qwen3 4B synthesis fixed. Compare the
reranker against the same candidate set for citation precision, known-answer
ranking and no-answer abstention. Promote only if it improves retrieval without
breaking the laptop envelope. A role-specific config is preferable to changing
the synthesis model.

### ASR

Keep `faster-whisper-small-ur`. It has measured natural Urdu limitations but is
installed and Urdu-capable. Qwen3-ASR 0.6B/1.7B is not an Urdu challenger: the
[official supported-language list](https://github.com/QwenLM/Qwen3-ASR) omits
Urdu. No ASR model download is requested.

### TTS

No download. Qwen3-TTS supports Chinese, English, Japanese, Korean, German,
French, Russian, Portuguese, Spanish and Italian according to its
[official model card](https://huggingface.co/Qwen/Qwen3-TTS-12Hz-1.7B-CustomVoice),
not Urdu. TTS also requires misuse/voice-cloning controls beyond the current
family baseline.

### Face and open-vocabulary vision

Keep installed YuNet/SFace under candidate-only, non-identification semantics.
Do not acquire SCRFD/ArcFace packs until license and representative consented
face evidence are closed. Defer LocateAnything-3B: it adds material RAM/storage
cost without first proving a specialist-baseline gap.

## Runtime boundary after approval

The first approved action may download only into a new isolated
`local-acceptance-models/nxmmr` cache. Benchmarking may build/use isolated
evaluators and write non-retained reports, but may not recreate live services,
change live LocalAI model configs, touch named volumes, ingest evidence, or
alter PostgreSQL. A later promotion pack must name the exact worker/API/UI
services, rollback tags and fresh 6 GiB RAM pass.

