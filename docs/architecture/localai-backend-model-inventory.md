# LocalAI backend and NexusAI model inventory

Status: R1 source accepted; promotion remains phase- and approval-gated.  
Verified: 2026-08-06

## Source inventory

The build matrix contains 64 unique backend identifiers across 261 parsed
Linux/Darwin matrix entries and CPU, CUDA/L4T,
ROCm, Intel/SYCL, and Darwin variants. Major families include llama.cpp,
TurboQuant, vLLM/vLLM-omni, SGLang, Transformers, Diffusers, MLX, rerankers,
Whisper/faster-Whisper/WhisperX/Parakeet/Qwen-ASR, multiple TTS engines,
detection/segmentation/depth, face/speaker recognition, VAD, and privacy filters.

Source presence is not installation or NexusAI support. The administrator-facing
Backend Gallery remains approval-gated.

The complete per-identifier platform, capability-family and NexusAI disposition
inventory is
`reports/nexusai-r1-backend-platform-disposition-inventory-20260806.json`.

## Live model truth

| Model | Role | Backend | Live | NexusAI policy |
| --- | --- | --- | --- | --- |
| `qwen_qwen3-4b-instruct-2507` | bounded planner/explanation | llama.cpp | yes | never authority for exact facts/citations |
| `qwen3-embedding-0.6b` | multilingual KB embeddings | llama.cpp | yes | never selectable for chat |

No OCR, ASR, VLM, reranker, detector, face, speaker, TTS, or video model is
installed or operational. No model/backend was downloaded or activated in this
session.

## NexusAI dispositions

| Family | Disposition |
| --- | --- |
| llama.cpp CPU | reuse for current two approved roles |
| vLLM/SGLang/Transformers | defer to GPU/high-memory deployment benchmarks |
| Rerankers | benchmark against fixed multilingual retrieval goldens |
| PaddleOCR/Tesseract/Docling candidates | external adapter benchmark; no install without approval |
| Whisper candidates | Urdu/English WER/resource benchmark; Qwen-ASR not Urdu-primary |
| Detection/VLM | bounded visual evidence benchmark after deterministic metadata/OCR |
| Face/speaker recognition | disabled until legal, privacy, bias, threshold, and review gates |
| TTS | derived, disclosed artifact only |
| Image generation/P2P | hide from ordinary forensic analysts |

Promotion requires task, revision, license, hashes, disk/RAM, latency, accuracy,
language, security, integration, rollback, and acceptance thresholds.

## Backend and model lifecycle

| Lifecycle stage | Owning source | Important behavior | NexusAI policy |
| --- | --- | --- | --- |
| Discover/source | `core/gallery`, `core/services/galleryop`, backend galleries | Lists and installs model/backend artifacts; operations may be asynchronous/distributed. | Admin-only, license/hash/signature/disk/RAM/rollback approval. |
| Configure installed model | `core/services/modeladmin` | Read/edit/patch/rename/enable/disable/pin and estimate VRAM; reload/preload/shutdown side effects are explicit. | Preserve compatibility identifiers; configuration mutations require audit and deployment approval. |
| Resolve/load | `pkg/model/initializers.go`, `loader.go` | Alias resolution, remote/external backends, concurrent-load coalescing, retries, LRU and concurrency-group eviction. | Model-role registry must select an approved role; loaded does not mean forensically accepted. |
| Observe/operate | backend monitor/log endpoints and `pkg/model/backend_log_store.go` | Load events, process logs, status, shutdown and watchdog state. | NexusAI System Administration only; redact secrets and preserve operator traceability. |
| Distribute | `core/services/nodes`, `pkg/clusterrouting` | Replica routing and remote node/backend operation. | Deferred to R16; current two-model local profile is the only accepted deployment. |
| Remove/upgrade | gallery operation services | Deletes or upgrades sourced artifacts/configuration. | Explicit approval, backup/rollback and post-change regression required. |

## Promotion diligence after R1

The 64-identifier inventory proves source/build breadth and records a disposition
for each identifier; it does not prove legal or operational eligibility. The
owning R-phase must still verify dependency licenses, hashes, compiled/installed/
configured/live state, task benchmarks, security, resources and rollback before
promotion.
