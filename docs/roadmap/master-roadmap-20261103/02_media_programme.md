# 02. The media programme: images, video, ANPR, audio and speech (English, Urdu, Punjabi, Pashto, Saraiki), faces

**Owner's request, 2026-11-03:** implement every media type best of the best, working and productive: images, videos, ANPR images and videos, audio, speech-to-text in English, Urdu, Pakistani Punjabi, Pashto and Saraiki, face detection, recognition and matching, and every relevant operation that can be built and integrated. This file is the plan. It extends and, where stated, **changes** section 8 of the master plan of 2026-10-21 (that plan kept faces at detection only and parked speech bake-offs behind a GPU; both are revised here: section 7 and section 4).

Evidence marks as in `01_state_of_the_project.md`: [M] measured by us, [R] read from the repository, [V] primary page read today, [S] search summary only, [K] prior knowledge. Numbers from other people's papers are for orientation; **our own gold sets decide**.

---

## 1. Rules for every media feature (non-negotiable; most are already binding)

1. **A model's output is an observation, never a fact.** Every derived row carries model id and revision, parameters, input hash, confidence, review state (`model_candidate`, `human_reviewed`, `externally_verified`), a locator (page and box; second; frame and box) and a truth state. The interface shows "machine reading, unreviewed" until a person reviews it. Corrections are new versions; the model's output is never overwritten.
2. **No identification claim.** A face, a voice or a plate string never says who someone is, who owns a vehicle or who was driving. A match is a **candidate** with its score, its error rate at that score on our evaluation set, and "human review required".
3. **Evidence never leaves the premises.** No evidence file, crop, transcript or embedding goes to a cloud service, a hosted model or a design tool. Cloud GPUs are used **only** on public or synthetic data to train or evaluate, and the weights come home.
4. **Licence-clean.** Apache-2.0, MIT, BSD, CC0, CC BY components may ship. AGPL (Ultralytics YOLO and trackers) never ships. Non-commercial weights (SeamlessM4T, MMS, NLLB, InsightFace model packs, AdaFace, EdgeFace) are **evaluation-only**, never in the product, and the ledger (`tools/licence_ledger`) records each.
5. **Roles switch on one at a time** behind the readiness receipt (`processor_readiness.py`), on controlled data, with a pre-registered gate; they switch off again if the gate fails. One heavy model at a time (`parallel_heavy_models=1`); one evaluation container at a time with `--cpus 8 -m 5g` (the Docker VM is 7.6 GB).
6. **Low confidence abstains and says so.** A reading in a language or condition below its gate is stored as `LOW_RESOURCE_LANGUAGE` (or similar), shown as "a pointer to listen, not a transcript", and is never counted as a finding.
7. **Hostile input is the default assumption.** Polyglots, decompression bombs, malformed headers, oversized pixels, path traversal, and instructions hidden in OCR text or transcripts. The language model never sees rows or free text from media as instructions; tools are read-only.
8. **Everything is reviewable and reprocessable.** A newer model produces a new artifact version beside the old one; nothing is silently replaced; the interface shows which version an answer used.
9. **Analyst wellbeing.** Graphic images and video open blurred by default, with a one-click reveal, and the setting is per user.

## 2. What exists and what the capability matrix looks like

Legend for **Status:** ON live · OFF built, switched off · LAB to build · GPU needs a GPU to be useful.

| Modality | Operation | Candidate (licence) | Status today | Where it runs | Gate source |
|---|---|---|---|---|---|
| Audio | Normalise, VAD | ffmpeg; Silero VAD (MIT [K]) | partial (ffmpeg only) | CPU | WP-M2 |
| Audio | Language identification per segment | Whisper LID; SpeechBrain VoxLingua107 (Apache-2.0 [K]); case language hints | Hindi-as-Urdu rule ON | CPU | WP-M1 |
| Audio | **ASR en/ur** | faster-whisper turbo (MIT) | **ON** (ur 20.9%, en 4.7% WER) [M] | CPU | exists |
| Audio | **ASR ps (Pashto)** | Whisper-turbo/small **fine-tuned** on Common Voice Pashto (CC0): 23.4%/24.9% WER on CV24 in the cited paper [V arXiv 2604.06507]; Omnilingual-ASR `pus_Arab` CER 12.6, `pbt_Arab` 8.8 [V] | zero-shot Whisper 96% WER (wrong script) [M] | CPU/GPU | WP-M1 |
| Audio | **ASR skr (Saraiki)** | Omnilingual-ASR `skr_Arab` CER 17.0 with 1.5 h of training audio [V]; fine-tune on Common Voice Saraiki (CC0, 6.75 h, 4.33 validated [S]) | none | CPU/GPU | WP-M1 |
| Audio | **ASR pnb (Pakistani Punjabi, Shahmukhi)** | Omnilingual-ASR `pnb_Arab` CER **31.2** (11.9 h) [V]; Whisper `pa` is Gurmukhi (Indian), 91% WER on FLEURS `pa_in` [M] | none | CPU/GPU | WP-M1 (weakest language) |
| Audio | Diarisation | stereo channel split first; pyannote community-1 (CC BY 4.0, gated [K]); DiariZen challenger | none | CPU | WP-M2 |
| Audio | Spoken-number and identifier conversion | rules plus a small model (Urdu/Punjabi/Pashto digit words, "double/triple") | none | CPU | WP-M2 |
| Audio | Translation (ur/ps/pa/skr to English) | IndicTrans2 (MIT [K], Urdu/Punjabi/Sindhi, not Pashto); the local LLM; NLLB eval-only | none | CPU/GPU | WP-M2 |
| Audio | Speaker clustering / verification | SpeechBrain ECAPA (Apache-2.0 [K]); WeSpeaker; same governance as faces | none (earlier decision: no voice identification) | CPU | WP-M11, decision D35 |
| Image | EXIF/GPS, hashes, near-duplicate | exiftool; SHA-256; PDQ (BSD [K]) + pHash | partial (fingerprint 22) | CPU | WP-M3 |
| Image | Content class (photo, screenshot, document, ID, chat screenshot) | SigLIP-base zero-shot (Apache-2.0 [K]) + small head | OFF (embedding) | CPU | WP-M3 |
| Image | OCR en/ur/ar | Tesseract best + PP-OCRv5 ONNX (ON); PaddleOCR-VL 0.9B (109 languages incl. Urdu per vendor [S]); Qwen3-VL (Apache-2.0) | **ON** v1 | CPU/GPU | WP-M4 |
| Image | Chat-screenshot parser, CNIC/MRZ/cheque parsers | OCR + layout + validators (MRZ checksum, IBAN, CNIC 5-7-1 pattern) | none | CPU | WP-M3 |
| Image | Object detection (fixed vocabulary) | RF-DETR Nano to Large (Apache-2.0 [V Roboflow card via S]); YOLOX | none | CPU/GPU | WP-M3 |
| Image | Semantic search (text to image, image to image) | SigLIP-base; pgvector HNSW | route exists, OFF | CPU | WP-M3 |
| Image | Tamper indicators | ELA, copy-move, noise, metadata mismatch: **indicators, not proof** | none | CPU | WP-M3 (late) |
| Plate | Detect + read on images | RF-DETR plate detector fine-tuned on Pakistani plates; fast-plate-ocr (MIT [V], weights' terms unstated) or a fine-tuned PP-OCR recogniser; province grammar decoding | 59% exact [M]; weights unlicensed | CPU | WP-M5 |
| Video | Probe, proxy, thumbnails, sprite, waveform | ffmpeg/ffprobe | probe only | CPU | WP-M6 |
| Video | Shots, motion segments, keyframes, on-screen timestamp OCR | PySceneDetect (BSD [K]), ffmpeg select, OCR | none | CPU | WP-M6 |
| Video | Detection and tracking (person, vehicle) | RF-DETR + ByteTrack via Roboflow Trackers (Apache-2.0 [K]); never Ultralytics | none (AGPL v2 stack configured, OFF) | CPU/GPU | WP-M6 |
| Video | ANPR with track voting | as Plate, plus character voting across the track | 7% at 1 fps [M] | CPU | WP-M6 |
| Video | Captions, semantic video search | Qwen3-VL 2B/4B/8B (Apache-2.0 [S]) | none | **GPU** | WP-M10 |
| Face | Detect, align, quality | YuNet (MIT); a quality estimator | **ON in lab, role OFF** [M] | CPU | WP-M7 |
| Face | Embed, cluster, search, compare | AuraFace-v1 (Apache-2.0 on its card, 512-D ArcFace-style, training-data provenance unclear [V]); InsightFace packs only with a purchased licence | SFace route exists, OFF | CPU/GPU | WP-M7, M8, decision D34 |
| Cross-modal | Link recordings, plates, faces, OCR numbers to events and entities | rules + candidates | none | CPU | WP-M9 |

## 3. The common pipeline (build once, every track uses it)

### 3.1 Stages
```
intake -> safety scan -> fingerprint -> probe -> normalise/proxy -> route -> tasks (queue by resource class)
   -> derived artifacts (versioned) -> indexes (text, trigram, vector, entity mentions) -> links (candidates)
   -> review desk -> analyst views for the engine -> viewers
```
* **Intake and safety scan** (exists for admission; extend the hostile-input suite to media): size/pixel/duration limits, container sanity, no execution, temp-file cleanup, no network from model containers.
* **Fingerprint:** SHA-256 (custody, exists), perceptual hashes for images and key frames (PDQ + pHash), audio fingerprint (Chromaprint [K]) for "same recording" detection.
* **Probe:** ffprobe/exiftool into a typed `media_properties` record (codec, container, duration, fps, resolution, channels, sample rate, creation tool, GPS, device, timestamps, anomalies). Telephony is detected here (8 kHz, narrow-band codecs).
* **Normalise/proxy:** originals are immutable. Derived, hash-recorded copies: video H.264 MP4 proxy 720p (+ HLS if long), thumbnail sprite sheet, audio 16 kHz mono WAV (and an 8 kHz telephony variant for evaluation), waveform peaks, image web proxy and tiles for large images. The browser never plays an original.
* **Route:** by detected type and case settings (languages expected, which roles are enabled for this case, authority for faces) to a set of **tasks**.
* **Tasks and queues:** classes `cpu_light`, `cpu_heavy` (one at a time), `gpu` (when present). Priorities: interactive (one file the analyst just opened) above batch. Retries with backoff; poison-pill quarantine; per-task timeouts and output caps (existing variables); resumable chunking for long audio/video.
* **Reprocess with lineage:** `derived_artifacts` already versions by model; add `supersedes`, `reason`, and an "outdated" badge when a better model is enabled; the interface offers "re-read with the current model" per file and per case.

### 3.2 The artifact contract (extends M0.3 of the master plan)
Every row of every media task has: `artifact_id`, `evidence_id`, `family`, `kind`, `model_id`, `model_revision`, `params_hash`, `input_sha256`, `created_at`, `locator` (`{page, box}` | `{t_start, t_end}` | `{frame, box}` | `{t_start, t_end, box}`), `confidence`, `reliability_state`, `review_state`, `truth_state`, `supersedes`. Free text (transcripts, OCR) lives in text columns **withheld from the engine's views** and reached only through retrieval with the PII rule.

### 3.3 Typed media tables and the engine's views (the fix for "the engine cannot see media")
Migration **033** (next free number) adds tables and reviewed `analyst.v_*` views with curated, non-text columns; the engine writes SELECTs over them like any family, with the label "model observation":

| View | One row is | Key columns |
|---|---|---|
| `v_media_files` | a media file | evidence id, kind (image/audio/video), duration, resolution, languages detected, tasks done, review counts |
| `v_transcript_segments` | a spoken segment | evidence id, t_start, t_end, speaker label, language, script, reliability state, confidence; **text only via retrieval** |
| `v_ocr_regions` | a text region | evidence id, page, box, script, confidence |
| `v_plate_reads` | a plate read (image or video track) | evidence id, t or frame, camera, plate alias or search-only text (D28), confidence, review state |
| `v_face_detections` / `v_face_clusters` | a face / a person cluster | evidence id, t, quality, cluster id, cluster size, first/last seen (**no embedding exposed**) |
| `v_video_events` | a moment | evidence id, t_start, t_end, kind (shot, motion, track, plate, face, speech), label, confidence |
| `v_image_similarity` | a near-duplicate or similar pair | evidence ids, distance, rule, state |
| `v_media_links` | a candidate link | media artifact, linked event/entity, rule, strength, state |

Questions this makes answerable: "how many recordings are in Pashto", "which video shows plate LEA-1234", "list the images with a face seen on more than one camera", "which phone calls have a recording", "when was this person cluster first seen".

### 3.4 Clock offsets and time basis (the CCTV problem)
A DVR's clock is rarely right. A table `media_clock_offset(evidence_id, offset_seconds, basis, set_by, set_at)` stores the analyst's correction with its basis (for example "overlay shows 14:02:11 while a known event at 14:07:40"). Interface and engine show **both** stamps; all joins to CDR/cell/plate events use the corrected time and say so. The on-screen overlay is read by OCR at several points to detect drift.

## 4. Speech: English, Urdu, Punjabi, Pashto, Saraiki (the owner's explicit list)

### 4.1 What is known (measured and read)
| Language | Our measurement | Published orientation | Public training/test data |
|---|---|---|---|
| English | turbo 4.7% WER (FLEURS) [M] | solved for read speech | many |
| Urdu | turbo 20.9% WER, small 33.8% [M]; 7/24 clips auto-detected as Hindi (fixed by re-read) [M] | SeamlessM4T 16.3% on FLEURS Urdu (CC BY-NC: evaluation only) [S Interspeech 2026, LUMS]; Omnilingual 7B-LLM CER **3.5** (237.5 training hours) [V]; Whisper fails mostly by script confusion [S] | Common Voice Urdu 24.0: 302 h, 81.5 h validated, 498 speakers, CC0 [S]; FLEURS `ur_pk` CC BY 4.0 [M] |
| Pashto | Whisper zero-shot **96.3% WER** (wrong script) [M] | outputs Arabic/Dari/Urdu script, WER over 100%, at most 0.8% true Pashto script for any size [S arXiv 2604.04598]; **fine-tuning fixes it**: turbo 23.4% and small 24.9% WER on Common Voice Pashto v24 (113 h) [V arXiv 2604.06507, checkpoints said to be released; licence to read]; Omnilingual `pus_Arab` CER 12.6 (28.7 h), `pbt_Arab` 8.8 (5.6 h) [V] | Common Voice Pashto, CC0; FLEURS `ps_af` |
| Punjabi (Pakistan, Shahmukhi) | none (FLEURS `pa_in` is Gurmukhi: 91% WER [M]) | Omnilingual `pnb_Arab` CER **31.2** (11.9 training hours) [V]; SeamlessM4T 22.1% on Punjabi in the same paper (script of that test set not verified) [S] | Omnilingual's released corpus (CC BY 4.0, about 350 under-served languages [V/S]): **whether it contains `pnb_Arab` and `skr_Arab` audio must be checked**; Common Voice has Gurmukhi Punjabi [K]; **no open Shahmukhi conversational set known** |
| Saraiki | none | Omnilingual `skr_Arab` CER **17.0** with only 1.5 training hours [V] | Common Voice Saraiki 24.0: 5,837 clips, 6.75 h (4.33 validated), 60 speakers, CC0 [S]; a 10 h set under CC BY-NC-SA (research only) [S] |
| Hindko, Sindhi, Balochi/Brahui | none | Omnilingual `hno_Arab` 17.1, `snd_Arab` 6.6, `brh_Arab` 8.2 [V]; no Balochi code found in its list [V] | small |

**Readings that matter.** (1) Omnilingual's character error rates are on Meta's own test sets and are **not comparable** to our word error rates; we measure both on the same clips. (2) Pashto is the quick win: a fine-tune on public data takes it from unusable to roughly 25% WER on read speech. (3) Pakistani Punjabi is the hardest and the most valuable: weak models, little data, a script and a spoken form that differ from the Indian Gurmukhi corpora. (4) Saraiki exists only through Omnilingual and a few hours of Common Voice. (5) Every number above is **read speech**; telephone-band, noisy, conversational and code-switched audio will be worse, and **no figure is quoted to a court until measured on our own gold set**.

### 4.2 The ASR service design
* **One ASR router** (a worker-side component, no new container) chooses the backend per segment from `(language, script, duration, hardware, reliability gate)` in a **routing table stored as data** (versioned, evaluated, switchable), not in code paths.
* **Backends behind the same interface:** (a) faster-whisper (CTranslate2) for `en`, `ur`, and fine-tuned `ps`; (b) Omnilingual-ASR (Apache-2.0 code and weights [V]; inference limit 40 s per clip except the "Unlimited" LLM variants; CTC 300M needs about 2 GiB and 1B about 3 GiB of VRAM on the vendor's A100 table [V]) for `skr`, `pnb`, `pus`/`pbt` and as an independent second opinion; (c) later, an LLM-ASR for hard audio on GPU.
* **Second opinion for hard languages.** For `LOW_RESOURCE` languages run two backends and store both; disagreement is itself a flag ("the two readings differ at 40% of words").
* **Chunking:** VAD segments up to 30 s (Whisper) / 40 s (Omnilingual) with 1 s overlap and overlap-aware merge; timestamps per segment.
* **Language handling:** the case sets expected languages (Urdu, English by default; Punjabi, Pashto, Saraiki on request); per-segment language ID restricted to that set; when the top two are close, run both and keep the lower-uncertainty reading, flag "language uncertain". Keep the existing rule "Hindi heard → re-read as Urdu".
* **Script and normalisation:** NFKC; Arabic-script letter unification (Arabic/Persian yeh and kaf, heh forms, zero-width joiners), Arabic-Indic and Extended Arabic-Indic digits to Latin for search; a **script-correctness check** (share of output characters in the expected script, gate at least 99%) catches the Whisper-on-Pashto failure automatically.
* **Spoken numbers and identifiers** (high forensic value): convert digit words in Urdu, Punjabi and Pashto ("double", "triple", "sifar", "paanch") to digit strings; detect phone-number-like (10 to 13 digit), plate-like and amount-like spans; store them as `v_media_mentions` candidates that feed the identifier mention index.
* **Roman Urdu** rendering stays (exists). **Translation** is an explicit, labelled analyst action ("machine translation"), never mixed into the transcript.
* **Stereo calls.** If a recording has two channels with different speech, treat channels as speaker tracks (free diarisation) before any diarisation model.

### 4.3 Fine-tuning plan (public data, rented or free GPU, weights come home)
| Step | Detail |
|---|---|
| Data (all fetched under a ledger row, hash-checked) | Common Voice Urdu, Pashto, Saraiki (CC0); FLEURS ur/ps/pa (CC BY 4.0); Omnilingual corpus slices for `pnb_Arab`, `skr_Arab`, `pus_Arab` if the released corpus holds them (CC BY 4.0 [V]). Partition by hash as in the existing kit; **test splits are never trained on** (note: some Urdu Whisper fine-tunes in the literature include FLEURS/Common Voice in training; we use our own partitions). |
| Telephone simulation | G.711 mu-law 8 kHz, AMR-NB, Opus low-bitrate, noise at several SNR, packet-loss; applied to the training and test copies so the model sees the channel it will meet. |
| Models | Whisper large-v3-turbo (MIT) with LoRA for `ps` and `ur` (and a distilled small variant for the 7.6 GB laptop path); Omnilingual CTC 300M/1B fine-tune for `skr`, `pnb` (the repository documents fine-tuning recipes [V], not for the Unlimited variants); convert to CTranslate2 int8 for serving. |
| Decision rule (written before running) | Word error with the written normalisation, character error, digit-sequence exact match, script share, loop rate, real-time factor on the laptop; promote a route only if it beats the incumbent on the dev partition **and** passes the gate on the sealed partition. |
| Gates (proposed; fixed after the baseline) | Urdu: WER at most 15% on read speech and at most 30% on the telephone-simulated copy; Pashto: at most 30% / 45%; Saraiki and Pakistani Punjabi: **CER** at most 15% / 20% on read speech, otherwise they stay `LOW_RESOURCE_LANGUAGE` leads; script share at least 99%; loops at most 1%; every segment timed. |
| Honest limit | These are read-speech gates on public data. **A consented sample of the owner's real kind of audio** (telephone calls, voice notes, interviews; D26) is the only way to certify conversational performance. The capture and labelling kit exists (`docs/data/media-capture-and-labelling-kit.md`). |

### 4.4 Speech outputs in the product
Waveform with synchronised transcript, speaker turns, per-segment language chips, reliability banner for low-resource segments, play-from-segment, search that jumps to the second, edit-and-verify with append-only versions, translation toggle, spoken-number highlights, "link to the call record" suggestions, export with timestamps. A voice-note or call recording opens as a viewer that also shows the matching call record's cell and subscribers when linked.

## 5. Images, documents and OCR

### 5.1 Image operations (WP-M3), in the order they are built
1. **Metadata:** EXIF/XMP/IPTC, GPS, device, software, timestamps, thumbnail mismatch; location is sensitive: stored with its method, shown with uncertainty, stripped from exports unless the case enables it.
2. **Hashes and duplicates:** exact SHA-256; PDQ (256-bit) and pHash; near-duplicate clusters labelled "near-duplicate, not the same image"; within-case and (opt-in) cross-case "seen before".
3. **Content class:** photo, screenshot, scanned document, ID card, chat screenshot, map, meme. Routes screenshots and documents to OCR/layout, photos to detection/faces.
4. **OCR and layout** (section 5.2).
5. **Chat-screenshot parser:** bubbles, side (sender/receiver), timestamps, names, attachments; output is a **message family** with a low-confidence flag, entering the engine like any other typed family (message-export family of the master plan, P4).
6. **ID and financial documents parsers** (only with case authority): CNIC front/back (13 digits, 5-7-1 pattern), driving licence, passport MRZ (ICAO checksum), cheque MICR, utility bills; every parser has **validators** (checksums, patterns) and flags failures.
7. **Object detection, fixed vocabulary** (person, car, motorcycle, bus, truck, bag, phone, document, weapon-category as a later, separately gated class) with RF-DETR; counts and boxes only.
8. **Semantic search:** SigLIP embeddings in pgvector (HNSW), text-to-image and image-to-image, always "candidate, review required".
9. **Tamper indicators:** ELA, copy-move, noise inconsistencies, metadata inconsistencies, double-JPEG traces: shown as **indicators** with an explanation of what each can and cannot prove.
10. **Sensitive content:** blur-by-default; optional known-hash list matching (an integration point, not bundled).

### 5.2 OCR and document reading (WP-M4)
* **Now:** auto router ON: Arabic-script page (ONNX probe) to Tesseract best `urd+eng` PSM 3; otherwise PP-OCRv5 ONNX with a Tesseract second opinion. Latin clean print 99.7%; Urdu Naskh 9% CER; **Urdu Nastaliq 32% CER (fail)**; real receipts: PP-OCRv5 80% F1 vs Tesseract 33% [M].
* **Bake-off (decision rule first):** Tesseract (floor), PP-OCRv5 Arabic, **PaddleOCR-VL 0.9B** (vendor lists 109 languages including Urdu, Pashto, Sindhi, Balochi; no independent Urdu figure found; a multilingual parsing benchmark shows an Urdu row but its column layout was unreadable in the summary, so re-read the paper before relying on it [S arXiv 2607.02956]), **Qwen3-VL** (Apache-2.0; GPU for useful speed), fine-tuned TrOCR/PARSeq-style recognisers; reference numbers from the Urdu Newspaper Benchmark (best classic system word error 0.42 / character error 0.159 [S, LREC 2026]; super-resolution helped LLM OCR by 25 to 70% [S]).
* **Data:** a labelled Urdu line set (500 to 1,000: newspapers, screenshots, social posts, documents, with Latin digits mixed in) from licence-clean sources plus synthetic Nastaliq from openly licensed fonts; real lines only from consented captures.
* **Document layer (Track D):** Docling (MIT [K]) for layout/tables on scanned pages; BGE-M3 (MIT [K]) + cross-encoder rerank with Urdu-aware normalisation for retrieval; identifier extraction with validators (10 to 16 digit identifiers at least 95% exact on clean print).
* **Untrusted text.** OCR text and transcripts are quoted as data in prompts, never as instructions; a red-team set (instructions hidden in a document) must pass before any media text reaches the reading role.

## 6. Video and ANPR (WP-M5, WP-M6, WP-M10)

### 6.1 Plates on images and on video
* **Detector:** RF-DETR Nano/Small (Apache-2.0 framework and backbone; check each checkpoint's terms; XL variants may be under a different licence [K]) fine-tuned on Pakistani plates; YOLOX as challenger. Ultralytics code or weights never ship.
* **Reader:** a compact recogniser fine-tuned on Pakistani crops with **grammar-constrained decoding** from a versioned **plate-format registry**; fast-plate-ocr (MIT licence stated [V]; its pretrained weights' own terms are not stated, so train our own) or a fine-tuned PP-OCR recogniser; top-k candidates with confidence.
* **Plate-format registry (data, versioned by date).** Pakistani plates are heterogeneous and changing: province-specific designs, one-line and two-line motorcycle plates, commercial and government colours, Islamabad "ICT" plates, Sindh's Ajrak-themed plates mandated through 2025 after repeated deadline extensions, Punjab's moves to owner-linked registration and proposed new designs for large vehicles [S: Wikipedia, PakWheels, ProPakistani]. The registry stores, per region and date range, the pattern, the characters allowed in each position and the visual class; it constrains decoding, normalises, and **flags** (never fills in) unusual reads. Because ownership is moving to the person (owner-based plates), a plate string is **more** sensitive than before: the masking decision D28 matters.
* **Data:** the 96-image T2-V virtual-scene pack (exact truth) [R]; **synthetic Pakistani plate rendering** (province templates, embossed styles, fonts, dirt, blur, perspective, night noise) composited onto permissively licensed backgrounds (CCPD is MIT [K]) to give tens of thousands of labelled crops; public Pakistani sets are small and unverified (Roboflow "Pakistani-Number-plates", 336 images, licence not read [S]; the PLPD dataset of Yousaf et al. 2021, access to be requested from the authors [S]); real captures under authority (D26) for the sealed set (at least 300 plate tracks across day, night, rain, blur, angle).
* **Gate (R8 contract):** Boundary A: exact read at least 90% with the lower 95% bound at least 85% on the sealed image set, detection recall at least 98%, calibration (confidence at least 0.9 means accuracy at least 95%), abstention explicit. Boundary B for video: track-level exact at least 85% with false plates per hour reported.
* **Linkage (R8.6):** a link to the structured `plate_sighting` family **only** by exact normalised plate plus camera/time criteria, with zero/one/many match states; confusion-aware matching (M/W, 0/O, 8/B) ranked by visual edit cost and labelled candidate; a plate never implies an owner or a route.

### 6.2 General video analysis
| Stage | What it does | Notes |
|---|---|---|
| Probe and proxy | codec, fps, duration, rotation, streams; H.264 proxy, HLS for long files, sprite thumbnails | DVR formats (Dahua `.dav`, Hikvision containers) often need a vendor converter before ffmpeg; the intake reports "unplayable, needs conversion" with instructions rather than failing silently |
| Overlay time OCR | reads the burned-in date/time at several points; detects drift and jumps | feeds the clock-offset table |
| Shots and motion | PySceneDetect + frame-difference; **motion-only index** so a 12-hour static camera yields a short list of intervals | the single biggest time saver for CCTV |
| Detect and track | persons and vehicles with RF-DETR + ByteTrack (Roboflow Trackers, Apache-2.0 [K]; BoT-SORT for moving cameras) | tracks become `v_video_events` with a count timeline |
| ANPR | per track: sharpest 5 crops, character voting, grammar decoding | no generative super-resolution on evidence crops (it invents characters); classical upscaling only |
| Faces | per person track: best-quality face(s), tracked embedding | governed by section 7 |
| Audio track | extracted and sent through section 4 | speech moments appear on the same timeline |
| Captions and semantic search | Qwen3-VL (2B/4B/8B, Apache-2.0 [S]) describes shots; stored as "model description, unreviewed", searchable, **never counted** | needs a GPU to be usable (WP-M10) |
| Vehicle/person appearance | colour, type, make/model by zero-shot SigLIP + small heads | "red motorcycle" searches; re-identification across cameras is a later, candidate-only feature |
| Integrity indicators | duplicate frames, timestamp gaps, re-encode traces, container anomalies | indicators, not proof |
| Clips and export | cut a clip with a burned-in hash/time watermark and a manifest (source hash, in/out, operator, reason) | court-ready |

## 7. Faces: detection, recognition and matching, governed

**This changes decision D5.** The master plan held faces at detection (L0). The owner now asks for recognition and matching. The plan below builds it, with the controls that make it defensible. Nothing here identifies a person: it produces **ranked candidates for a human to confirm**.

### 7.1 Levels
| Level | Capability | Default | Enablement |
|---|---|---|---|
| **L0** | Detection, landmarks, quality; no embedding stored | on for media roles | case setting |
| **L1** | Embeddings, **clustering inside one case** ("may show the same face"), person clusters, best-face selection, appears-with co-occurrence | off | per case, recorded authority |
| **L2** | **Search** a probe face against a case (1:N), **compare** two faces (1:1), analyst-supplied galleries/watchlists | off | per case, recorded authority, role permission, reason on every search |
| **L3** | Cross-case face matching | off, separate approval | two-case authority; hit-notice workflow of `03_architecture.md` section 4 |
| not built | age/gender/emotion attributes, real-time live-CCTV alerting, any automatic decision | | |

### 7.2 Pipeline
detect (YuNet; challengers MediaPipe BlazeFace Apache-2.0 [K], RetinaFace code MIT with weights to check) → align (5 landmarks) → **quality gate** (size, blur, pose, occlusion; low-quality faces are listed but not embedded or matched) → embed (**AuraFace-v1**: ResNet-100 ArcFace-style, 512-D, Apache-2.0 on its model card, LFW 99.65% self-reported, training data described only as "a commercial dataset ... from various sources" and bundled InsightFace detectors are non-commercial [V]; read the licence file and the training statement and take counsel before shipping; the alternative is to **buy** an InsightFace commercial licence [K]) → store in pgvector HNSW (cosine) with `model_id` (an embedding space belongs to one model: re-embedding with another model creates a new index) → cluster (HDBSCAN/agglomerative over quality-gated faces) → analyst review (merge, split, name, reject) → search/compare.

### 7.3 Evaluation before any L2 use (pre-registered)
* **Set:** a **consented South Asian set** with CCTV-like degradation (downscale to 40 px, JPEG, blur, low light, masks, pose, age gap), collected from volunteers (D26) and, for baseline only, public research sets whose licences allow evaluation. NIST FRTE reports demographic differentials per algorithm with a South Asia region-of-birth group; thresholds are developer- and version-dependent [S], so **we calibrate on our own set**.
* **Metrics:** ROC; false non-match rate at false match rates 1e-3 and 1e-4; precision at k; per-group error rates (sex, age band, skin-tone proxy where consented); quality-binned results; a **negative control** (two different people the model scores high must not produce an identity claim).
* **Gate:** published as a table in the pre-registration before the first measurement; failing it keeps L2 off. A threshold shown to analysts is expressed as "pairs of different people reach this score about 1 in N times on our test set", not as a probability of identity.

### 7.4 Governance and audit (built before L1 is switched on)
Per-case enablement with a recorded authority reference; role permission `face_search`; **every search logged** (who, when, probe hash, scope, number of results, stated reason); candidate labelling on every screen and export; embeddings are never exposed or exported; retention rule per case; kill switch per case and globally; a written bias and limitation statement shown in the interface; a legal review before L2 (Pakistan's personal data protection bill is still a draft with broad state exemptions, and the Interior Ministry has directed a unified facial-recognition system [S: GenderIT, Privacy International, ProPakistani]; the law will not answer this for us, so the controls above are the defence). Face links enter link analysis **only after an analyst confirms them**.

### 7.5 Voice is the same question
Speaker clustering/verification (ECAPA-type embeddings) would be governed identically (candidate only, authority, audit). The earlier decision excluded voice identification; with the owner's wider scope it is a decision, **D35**, default: build **within-recording diarisation and within-case speaker clustering only**, no gallery search.

## 8. Cross-modal linking and media in the engine (WP-M9)

| Link | Rule | State |
|---|---|---|
| Recording to call record | recording time/duration or filename numbers match a CDR event of the same parties within a tolerance | candidate, confirmed by analyst |
| Spoken or OCR'd number to subscriber/handset/account | exact normalised identifier from `v_media_mentions` | candidate (mention) |
| Plate read to plate sighting | exact normalised plate + camera/time | zero/one/many |
| Video moment to cell activity | phone seen in a cell near the camera in the window (co-presence) | "may have been near" with the cell's uncertainty, never "was at" |
| Face cluster to person entity | analyst assignment only | confirmed |
| Image EXIF GPS/time to movement anchors | within tolerance | candidate |
| Same image/recording/face/plate in another case | hash/identifier registry (cross-case) | notice, need-to-know |

All of them are rows in `v_media_links` with a rule name and a strength word; the **link analysis workbench** and **timeline** show them with the candidate shape, never as observed facts. This is where media starts to change investigations: one timeline per person or number across calls, bank, plates, video moments, recordings and documents.

## 9. The review desk (stage U9) and quality loop
A queue of low-confidence and disagreement items (ASR second-opinion mismatches, plate reads below threshold, face clusters at the merge boundary, OCR regions flagged): side-by-side source (audio snippet, crop, frame) and reading, accept / correct / reject with a reason, undo as a new event, throughput and agreement statistics. Reviewed items become **labelled data** for the next fine-tune (kept inside the premises, never in public sets) and for the sealed-set refresh.

## 10. Hardware plan (decision D36)
| Tier | Hardware | What it runs | Status |
|---|---|---|---|
| Now | CPU laptop, Docker VM 7.6 GB | OCR, turbo ASR (RTF 1.45), detection at low fps, face L0; everything in overnight batch | live |
| Interim | HP EliteDesk + RTX 2080 (8 GB), if made available | ASR fp16, detectors, face embedding, Qwen3-VL 4B Q4, CTC models; not 7B | exists, not available (D21) |
| Target dev GPU | an affordable 16 GB card (RTX 5060 Ti 16 GB about $430 to 550) or a used 24 GB card (RTX 3090 about $700 to 1,050) [earlier research 2026-09-29; prices unverified] | the above plus an 8B to 14B SQL/reading model, VLM and ASR scheduled together | decision |
| Cloud (public data only) | Kaggle/Colab/rented | fine-tuning and bake-offs; weights return | allowed under the 2026-09-29 rule with owner approval per run |

Throughput targets to state in each gate: audio hours per hour, video hours per hour, images per second, per tier.

## 11. Evaluation and gates (summary; details in `05_data_testing_research.md`)
Pre-registration before every first measurement; development/sealed partition by hash; sealed sets never tuned on; each report states `synthetic`, `public_benchmark`, `controlled_demo`, `authorised_real`; a **media bank** of at least 20 questions per family written against the data with exact truth and the label "model observation"; red-team set (hidden instructions, PII extraction, SQL escape); zero confident-wrong at the bank gate.

## 12. Work packages (ordered; `06_roadmap_phases.md` schedules them)
| WP | Name | Depends on | Output the owner sees |
|---|---|---|---|
| M0 | Media foundation: artifact contract v2, queue classes, proxies/thumbnails/waveforms, clock offsets, review events, migration 033 views | none | Media tab with every file, state, thumbnail, tasks done |
| M1 | **Speech lab**: data under ledger, gold sets, Omnilingual CPU/GPU baseline, Pashto/Urdu/Saraiki/Punjabi fine-tunes, telephone simulation, routing table | M0 (data kit exists) | results table per language, decision, routing live |
| M2 | **Audio product**: VAD, LID, diarisation, spoken numbers, translation, transcript viewer, review, CDR linking, audio media bank | M1 | open any recording: synchronised transcript, languages, numbers, call link |
| M3 | **Image product**: EXIF, PDQ, class, chat-screenshot and ID parsers, SigLIP search, tamper indicators, viewer | M0 | image viewer with metadata, text, similar images, search by words |
| M4 | **Urdu/Pashto OCR bake-off** and Docling layer | M0, data | OCR results per script; scanned PDF viewer with overlay |
| M5 | **Plates on images**: detector, recogniser, registry, synthetic generator, review, linkage | M0, D26 for sealed set | plate reads on images linked to sightings |
| M6 | **Video v1**: proxies, shots, motion index, overlay OCR, tracking, ANPR v1, audio hook, player with event scrubber | M0, M5 | open a video: moments timeline, plates, speech |
| M7 | **Faces L0 to L1**: lab, evaluation set, clustering, governance, UI | M0, D34 | person clusters per case |
| M8 | **Faces L2**: search/compare/watchlists, audit, negative control, legal gate | M7 | search with candidate list and error-rate statement |
| M9 | **Cross-modal links and mentions** | M2, M3, M5, M6 | one timeline across families |
| M10 | **VLM captions and semantic video search** | GPU | search video by words |
| M11 | Speaker clustering (optional) | M2, D35 | speaker clusters per case |

**Cut line if time is short** (what to protect first): M0, M1, M2, M3, M4 (visible value, no policy risk); then M5, M6 (operational ANPR); then M7 to M9; M10 and M11 last.
