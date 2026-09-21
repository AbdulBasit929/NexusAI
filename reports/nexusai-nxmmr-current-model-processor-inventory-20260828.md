# NX-MMR current model and processor inventory

As of 2026-08-29. Inventory was collected read-only from the live LocalAI
container, forensic worker configuration, repository manifests, and the host.

## Host envelope

| Item | Measured value |
|---|---|
| Host | Lenovo `21BVS0QX00` |
| CPU | 12th Gen Intel Core i7-1260P, 16 logical processors |
| RAM | 16,871,448,576 bytes |
| Free RAM at final inventory | 3,733,495,808 bytes (historical observation; remeasure before every workload) |
| GPU | Intel UHD Graphics; Windows reports 2,147,479,552 adapter bytes |
| OS | Windows 11 Pro 10.0.22631 |
| C: free | 1,770,922,962,944 bytes |

The laptop is a CPU-first, one-heavy-model-at-a-time target. Disk is not the
immediate constraint; free physical RAM and shared-memory graphics are. The
approved benchmark policy uses 3.0/3.5/4.5 GiB LIGHT/MEDIUM/HEAVY pre-run
floors plus measured peak and 1.5 GiB headroom. VERY_HEAVY is not a default
class. The independent 6 GiB build/deployment gate remains unchanged.

## Live LocalAI models

| Live ID / asset | Bytes | Hash / revision | Role | Current verdict |
|---|---:|---|---|---|
| `qwen_qwen3-4b-instruct-2507` / Q8 GGUF | 4,280,405,216 | SHA-256 `260b5b5b6ad73e44df81a43ea1f5c11c37007b6bac18eb3cd2016e8667c19662` | synthesis/planning | installed; deterministic tools remain fact authority |
| `qwen3-embedding-0.6b` / Q8 GGUF | 639,150,592 | SHA-256 `06507c7b42688469c4e7298b0a1e16deff06caf291cf0a5b278c308249c3e439` | embedding | installed; retrieval benchmark needed |
| `faster-whisper-small-ur` / `model.bin` | 483,546,902 | SHA-256 `3e305921506d8872816023e4c273e75d2419fb89b24da97b4fe7bce14170d671` | Urdu/Pakistan ASR | installed and accepted with limitations |
| `whisper-tiny` / `ggml-tiny.bin` | 77,691,713 | SHA-256 `be07e048e1e599ad46341c8d2a135645097a538221678b7acdd1b1919c6e1b21` | ASR smoke floor | installed; not an accuracy default |
| `face-detect-yunet-sface` | 26,073,536 | SHA-256 `9ce78d4ba0ae9d5e8c91a0e145d511558d1d90f5d9c1f4131cca9bb4bce60902` | face detection/similarity candidates | installed; candidate-only semantics |

The two Qwen GGUF configs are CPU `llama-cpp`, 8,192 context, eight threads,
and zero GPU layers. The face threshold is 0.363. Model cards and packaged
notices must still be captured in any promotion receipt.

## Live backend registrations

`cpu-faster-whisper`, `cpu-whisper`, `face-detect`, `faster-whisper`, `whisper`,
`cpu-face-detect`, `cpu-llama-cpp`, and `llama-cpp` are registered. No live
reranker, VLM, TTS, diarization, open-vocabulary detector, PaddleOCR, or
specialist ANPR model is installed.

## Existing isolated evaluator images

The host already holds the bounded CPU evaluator images needed for the first
comparison, so no evaluator image pull or rebuild is proposed:

| Image | Local image ID | Reported size | Purpose |
|---|---|---:|---|
| `nexusai/r8-fastplate-eval:1.1.0-cpu` | `7ecb12c53a2d` | 653 MB | FastPlateOCR comparison |
| `nexusai/r8-ocr-eval:3.7.0-cpu` | `614ac87f4f10` | 2.43 GB | PaddleOCR/Tesseract isolated evaluation |
| `nexusai/r8-easyocr-eval:1.7.2-cpu` | `787dd1e0fc87` | 1.91 GB | retained historical challenger, not first-bundle default |
| `nexusai/r8-omz0106-eval:openvino-2024.6.0` | `eac3ca4d7e6f` | 503 MB | retained detector challenger, not first-bundle default |

Existing image presence does not establish that the required model archive is
inside the image/cache or that its current hash is accepted. An approved run
must inspect the isolated cache first and download only missing manifest files.

## Critical processor readiness distinction

The forensic worker's host mount `models/media` is empty. Compose expects:

- `yolo-v9-t-384-license-plates-end2end.onnx`;
- `cct_xs_v2_global.onnx` and its plate config;
- SigLIP weights; and
- Tesseract English/Urdu trained data.

Those assets are absent from the current worker mount. Therefore current live
images/audio/video may remain queryable through retained derived artifacts,
but a new image/video run cannot be described as ANPR/OCR/SigLIP-ready. This is
the main processor-readiness P1 found by NX-MMR.

Live retained capability state is 20 families: 15 queryable, four no-data and
one manual-review. The primary workspace has `24 evidence | 24 versions | 9275
canonical rows | 426 artifacts | 22 KB assets`; retained global accounting is
`51|51|64|0|22207|441|47`. These counts establish retained evidence coverage,
not fresh model availability.

## Local controlled assets discovered but not processed

| Asset | Size | Integrity / content |
|---|---:|---|
| `C:\Users\sheik\Downloads\sample.mp4` | 184,407,144 bytes | SHA-256 `d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee` |
| `C:\Users\sheik\Downloads\archive\Pakistani License Number Plates Data` | 50,377,210 bytes | 108 JPGs: 104 under Cars, 4 under Plates; no label, license, CSV, XML or JSON manifest found |

No file was copied, opened through a model, ingested, reprocessed, moved, or
deleted. The image directory requires license/privacy review and independent
ground truth before it can certify anything.

## Role disposition

| Role | Incumbent | Challenger | Decision now |
|---|---|---|---|
| deterministic analytics | typed SQL/adapters | none | retain |
| answer synthesis | Qwen3 4B Q8 | larger LLMs | retain; no quality case for laptop cost |
| embeddings | Qwen3 Embedding 0.6B Q8 | BGE-M3 | retain until retrieval benchmark |
| reranking | absent | Qwen3 Reranker 0.6B Q8 | proposed isolated acquisition |
| Urdu ASR | faster-whisper-small | Qwen3-ASR 0.6B | retain; Qwen3-ASR official list omits Urdu |
| TTS | absent | Qwen3-TTS | defer/reject for Urdu; official list omits Urdu |
| ANPR | historical FastALPR assets absent | alternate detectors/OCR | recover incumbent first, then compare |
| Urdu OCR | historical Tesseract `eng+urd` assets absent | PaddleOCR Arabic PP-OCRv5 | acquire both resource floor and challenger for isolated test |
| face | YuNet/SFace | InsightFace/SCRFD families | retain candidate-only; licensing and representative face evidence not closed |
| open-vocabulary vision | absent | LocateAnything-3B | defer on resource/value grounds |
