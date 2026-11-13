# Model register, 2026-11-13: what is running, what was retired, what is being measured, what is only a candidate

Sources: the licence ledger (`docs/compliance/licence-ledger.csv`, rows MOD-001 to MOD-044), the settings of the running containers read on 2026-11-13 (worker, API, language-model runtime), and the project's own records (`docs/data/media-activation-results-20261021.md`, `speech-lab-preregistration-20261112.md`, the plates and video records). A "licence" below is what the model card or repository states; a row marked *unverified* has not had its licence file opened (the ledger says what is still to do). Every model output in the product is a **model observation** that a person reviews.

## 1. Running now (live)

| Role | Model | Where it runs | Since / notes |
|---|---|---|---|
| Ask engine: writes the database query, the text-search intent, the wording of answers, the readings | **Qwen3-4B-Instruct-2507**, 4-bit (Q4_K_M, GGUF), served by LocalAI as `qwen3-4b-instruct-2507-q4km-nxb21d-dev` (API setting `FORENSIC_SYNTHESIS_MODEL`) | the language-model container (CPU) | Apache-2.0; one model for all the engine's roles; a larger model waits for a GPU (decision D43) |
| Speech to text, English and Urdu (and any language not routed) | **faster-whisper large-v3-turbo** (CTranslate2 int8, beam 1, 8 threads), `faster-whisper-turbo` | LocalAI | MIT; since 2026-10-22; English 4.7% and Urdu 20.9% word error on clean read speech; a Hindi reading with no language requested is read again as Urdu |
| Speech to text, recordings over 10 minutes | **faster-whisper small** (`faster-whisper-small-ur`) | LocalAI | MIT; fallback so that one long file cannot hold the service for hours |
| Text from images and documents (OCR) | **Tesseract 5** (`best` data, English and Urdu, page mode 3) for Arabic-script pages; **PaddleOCR PP-OCRv5** (mobile detector, Latin and Arabic recognisers, ONNX) for everything else, with Tesseract as second opinion; the Arabic recogniser also acts as the script probe | worker | Apache-2.0; since 2026-10-21; Urdu Naskh 9% character error (passes), Urdu Nastaliq 32% (fails its gate) |
| Plates in photos | **RF-DETR ALPR** detector (`rfdetr_alpr.onnx`) and **CCT-S v2 global** reader (`cct_s_v2_global.onnx`), engine `nexus` | worker | since 2026-11-10, provisional and not certified (weights licence and training data not verified, ledger MOD-041); on the owner-labelled real sealed set 16 of 22 photos read exactly against 13 for the previous pair, the pre-registered gate of 17 missed by one image; plate box recall about 91 to 93% against a 95% gate |
| Plates in video | **Video plate processor V4** (the same two models, reading frames with grouping across two engines) | worker | since 2026-11-11 on the owner's word; not certified on the synthetic sealed set, 9 of 9, 7 of 7 and 13 of 15 plates exact on real clips |
| Speech reading of video and audio tracks | the same Whisper models as above | LocalAI | the Pashto and Punjabi routes are NOT live (section 3) |

## 2. Built into the system but switched OFF

| Role | Model | Why off |
|---|---|---|
| Face detection (level L0, counts only) | **YuNet 2023mar** (MIT) with **SFace** (Apache-2.0 label, training data unclear) as `face-detect-yunet-sface` | detection recall 81 to 84% for faces of 40 pixels or more, but the false-box rate could not be measured cleanly; no recognition is offered (decisions D34/D5) |
| Image embeddings and similarity | **SigLIP base patch16-224** (Apache-2.0) | role off; near-duplicate and similar-image search is part of the image product still to build |

## 3. Under measurement today (speech lab, `speech-lab-preregistration-20261112.md`)

| Model | Role | Status |
|---|---|---|
| **Omnilingual ASR CTC 300M** (Meta, Apache-2.0 on its card; converted here by `tools/media_data/omni_ctc.py`, 423 of 423 tensors placed) | Pashto, Pakistani Punjabi (Shahmukhi), Saraiki, possibly Urdu: one CPU model, 20 ms frames | development runs in progress: Central Pashto 26.2% word error and 8.6% character error (40 clips) |
| **Whisper small fine-tuned on Common Voice Pashto 24** (`ihanif/exp_009_small_cv24_vanilla`, Apache-2.0 on its card, converted to CTranslate2 int8; ledger MOD-044) | Pashto candidate | measured: needs reading without timestamps and in 12-second pieces; Central Pashto 30.8% word error and 16.1% character error; read Pashto 45.6% |
| the product route for the above (`speech_routes.py`, route table `speech_routes.json`) | language routing and status labels | written and tested, not applied to the live worker |

## 4. Retired or superseded (kept only as a rollback or a reference)

| Model or setting | Replaced by | When and why |
|---|---|---|
| faster-whisper **small** as the main Urdu model | large-v3-turbo | 2026-10-22: 38% lower word error on Urdu (20.9% against 33.8%) at a real-time factor of 1.45; small stays as the long-recording fallback |
| faster-whisper **medium** | turbo | bake-off 2026-10-21: Urdu 26.2% word error at a real-time factor of 3.43, against turbo's 20.9% at 1.45 |
| LocalAI's stock faster-whisper backend (fixed beam 5) | the same backend patched to read beam and threads from the model's settings | 2026-10-22; beam 1 costs 0.4 points and saves 17% of the time |
| Tesseract `fast` data, page mode 11 (scene text) | Tesseract `best`, page mode 3 | 2026-10-21: identifiers on clean pages went from 79% to 99.7% |
| PaddleOCR Latin recogniser and Tesseract's own script detection as the page router | the Arabic recogniser as a probe | 2026-10-21: Tesseract's script pass sent a quarter of the Urdu pages and almost every receipt to the wrong engine |
| **YOLOv9-t 384** plate detector and **CCT-XS v2** reader (MOD-006, MOD-007) | RF-DETR ALPR and CCT-S v2 (engine `nexus`) | 2026-11-10: 59% exact read; weights licence never verified; the paths are still configured as the previous engine |
| Video plate processors **V2 and V3** (Ultralytics YOLOv8n, MOD-011, and a plate detector with an unclear licence, MOD-012) | V4 | AGPL / unclear licence: blocked, never shipped; switches off |
| Language-model experiments on the same runtime: `phi4-mini-instruct` (a semantic-layer trial), two `qwen3-8b-q4km` selection settings (normal and low-memory), Qwen3-1.7B | Qwen3-4B-Instruct-2507 | earlier trials; the ledger does not record why each was dropped. The measured limit of the 4B on this CPU on 2026-09-29 was about 4 tokens a second (new questions took 119 to 152 s then; the Ask engine has changed since). The model files and settings are still in the runtime folder |
| `whisper-tiny` (LocalAI default) | none | never used for evidence |

## 5. Candidates, not started (each waits on a gate, a licence read or a GPU)

| Model | Role | What it waits for |
|---|---|---|
| Qwen3-8B and 14B; Arctic-Text2SQL-R1 7B and 14B; OmniSQL 7B and 14B | a larger query writer | a GPU and the bake-off rule written first (decision D43) |
| BGE-M3, a multilingual cross-encoder reranker | retrieval for documents and text | the document track |
| Qwen3-VL 4B and 8B | frame captions, semantic video search | a GPU |
| IndicConformer (Urdu), wav2vec2-XLS-R fine-tunes, Urdu Whisper fine-tunes | Urdu speech challengers | the Urdu comparison in the speech lab (Omnilingual first) |
| Silero VAD, pyannote Community-1, DiariZen, VoxLingua107 / MMS-LID | voice activity, speaker turns, language identification per segment | the audio product (WP-M2) |
| DINOv2 small and base | image similarity | the image product |
| TrOCR, PARSeq, PaddleOCR-VL, Granite-Docling 258M | Urdu Nastaliq OCR and document structure | the Urdu OCR bake-off |
| RT-DETR family, YOLOX | licence-clean plate and vehicle detectors | the plate reader work |
| AuraFace-v1 | face embeddings, only if faces move past detection | decision D34 and counsel |

## 6. Blocked or evaluation-only (never ship)

SeamlessM4T (CC BY-NC), Meta MMS large checkpoints (believed non-commercial), InsightFace / ArcFace weights (research only), AdaFace (WebFace-trained), EdgeFace (CC BY-NC-SA), Ultralytics YOLO code and weights (AGPL).

## Not models

The matching, grouping, cutting and voting steps are code, not learned: the plate-format registry, the video plate grouping, the Roman Urdu transliteration, the quiet-point cutter of the speech route, the Ask engine's obligations and coverage checks, the evidence and trace steps.
