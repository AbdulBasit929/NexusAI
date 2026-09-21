# NexusAI forensic model candidate shortlist — 2026-07-29

## Decision

No model was downloaded, installed, activated, replaced, or called during this
slice. The currently accepted baseline remains:

- synthesis/chat: `qwen_qwen3-4b-instruct-2507`;
- embeddings: `qwen3-embedding-0.6b`;
- execution profile: CPU and constrained local memory;
- exact answers: deterministic SQL only; a model may summarize bounded results
  and cited evidence but may not manufacture counts, rows, identities, OCR, or
  transcripts.

Candidate promotion must change one component at a time and preserve an immediate
rollback to this baseline.

## Shortlist by task

| Task | First benchmark | Challenger | Why this order | Promotion gate |
|---|---|---|---|---|
| Urdu/English OCR | Tesseract `tessdata_fast` `urd+eng` | PaddleOCR `arabic_PP-OCRv5_mobile_rec` | Tesseract gives a very small CPU floor; PaddleOCR explicitly includes Urdu and English in its Arabic-script mobile recognizer | CER/WER, box precision, rotation/blur/forms/plates, no invented text, p95 latency and peak RAM |
| Urdu/English STT | OpenAI Whisper tiny | OpenAI Whisper small | Both are multilingual and Apache-2.0; 39M tiny proves the runtime, while 244M small is the first accuracy challenger | WER by Urdu/English/code-switch/noise group, timestamp p95, real-time factor, RAM, abstention on silence/corruption |
| TTS accessibility | none until policy approval | Meta MMS Urdu TTS, research-only | The official Urdu VITS checkpoint is small, but CC-BY-NC-4.0 is not acceptable for an unrestricted commercial production assumption | legal/license approval, consent/voice policy, pronunciation and round-trip intelligibility, deterministic seed metadata |
| Visual questions | metadata/OCR/detectors first | Qwen3-VL-2B-Instruct GGUF | Official 2B Apache-2.0 candidate is the smallest reviewed VLM; still materially heavier than specialist OCR | grounded-answer accuracy, OCR/detection comparison, unsupported-claim rate 0, RAM/latency, exact box/time citations |
| Retrieval | retain Qwen3-Embedding-0.6B | BGE-M3 only if fixed Urdu retrieval gaps remain | Current Qwen model supports 100+ languages and is already accepted; BGE-M3 adds dense/sparse/multivector retrieval but increases integration and resource cost | fixed multilingual recall@5, citation correctness 1.0, no-answer accuracy, index size, ingest/query latency |
| Synthesis/planner | retain Qwen3-4B-Instruct-2507 | none yet | The official 4B model is already the accepted baseline and reports stronger multilingual/instruction behavior than its earlier Qwen3-4B comparison | exact-answer agreement 1.0, unsupported-claim rate 0, tool/JSON validity, latency and RAM no worse than approved budget |

## Primary-source findings

- OpenAI's multilingual Whisper family lists tiny at 39M parameters and small at
  244M. LocalAI already exposes a CPU `whisper` backend, so these candidates do
  not require inventing a new serving architecture.
- PaddleOCR's official PP-OCRv5 multilingual documentation lists Urdu and English
  under `arabic_PP-OCRv5_mobile_rec`; project documentation describes the mobile
  recognizer as ultra-lightweight.
- Tesseract's official fast Urdu trained-data file is approximately 1.33 MB and
  is the lowest-resource deterministic baseline.
- Qwen3-VL-2B-Instruct and its official GGUF repository are Apache-2.0 and provide
  the smallest reviewed general VLM option, but no visual model is allowed to
  replace specialist OCR/detection facts.
- Qwen3-Embedding-0.6B supports 100+ languages, up to 32K context and flexible
  embedding dimensions. BGE-M3 also supports 100+ languages plus dense, sparse
  and multi-vector retrieval, so it is a challenger only when a fixed test proves
  a material retrieval deficiency.
- Meta MMS Urdu TTS is 36.3M parameters, but its model card is CC-BY-NC-4.0.
  Therefore it is excluded from production promotion without explicit legal and
  usage approval.
- Meta MMS multilingual ASR is 1B parameters and CC-BY-NC-4.0. It is not a first
  candidate for this memory-constrained laptop.

## Required fixed Pakistan evaluation packs before any download

1. Printed and scanned Urdu/English forms, rotated/blurred screenshots, numeric
   identifiers, PK IBANs, CNIC-like synthetic values, and province-style plate
   crops, all synthetic or authorized and box-annotated.
2. Clean/noisy Urdu, English and code-switched speech; phone-band audio; silence;
   clipped/corrupt files; overlapping speakers; fixed transcript and time ranges.
3. Multilingual notes and reports with fixed relevant/non-relevant passages,
   exact citations and no-answer questions.
4. Resource capture per candidate: cold/warm p95, throughput, peak resident RAM,
   CPU, load time and disk footprint.
5. Provenance capture: model repository, immutable revision, license, hashes,
   quantization, backend version, parameters, seed and preprocessing.

## Approval sequence

1. Review this shortlist and the fixed gold set.
2. Approve one candidate download only.
3. Record model revision/hash/license before execution.
4. Benchmark against the unchanged baseline and resource ceiling.
5. Reject or promote; never activate based on a demo result.
6. Run a separate targeted rebuild/restart and rollback exercise only after
   promotion approval.

## Sources

- Qwen3 4B Instruct 2507: https://huggingface.co/Qwen/Qwen3-4B-Instruct-2507
- Qwen3 Embedding 0.6B: https://huggingface.co/Qwen/Qwen3-Embedding-0.6B
- Qwen3 VL 2B Instruct: https://huggingface.co/Qwen/Qwen3-VL-2B-Instruct
- Qwen3 VL 2B GGUF: https://huggingface.co/Qwen/Qwen3-VL-2B-Instruct-GGUF
- BGE-M3: https://huggingface.co/BAAI/bge-m3
- Whisper tiny: https://huggingface.co/openai/whisper-tiny
- Whisper small: https://huggingface.co/openai/whisper-small
- PP-OCRv5 multilingual recognition: https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/algorithm/PP-OCRv5/PP-OCRv5_multi_languages.en.md
- Tesseract fast Urdu data: https://github.com/tesseract-ocr/tessdata_fast/blob/main/urd.traineddata
- Meta MMS Urdu TTS: https://huggingface.co/facebook/mms-tts-urd-script_arabic
- Meta MMS multilingual ASR: https://huggingface.co/facebook/mms-1b-all

