# Media activation results (2026-10-21 onward)

Results against the pre-registration `docs/data/media-activation-programme-20261021.md` (committed 59bc62c before any measurement). Raw result files are in `C:/tmp/nexusai-media-data/results/` (outside the repository); the tools that produced them are in `tools/media_data/` and re-run with the commands in their docstrings. Tiers are stated per number: T1 synthetic with exact truth, T3 public licensed. **Nothing here is a Pakistani field accuracy.** Gates are quoted from the pre-registration; where a measurement showed that a measurement choice of mine was wrong (a truth file, a test image) that is said and the fix is dated.

## D: documents and printed OCR

### What the worker does today (read from the code and the container, 2026-10-21)
- The OCR role (`FORENSIC_OCR_ENABLED`, off) is a **scene-text** reader for images: Tesseract, `--oem 1 --psm 11`, languages `eng+urd`, lines rebuilt from the TSV (`media_pipeline.py` `TesseractImageOCRProcessor`). The document pipeline never performs OCR ("scanned pages require a separately approved OCR stage").
- **The pinned language data is not in the container**: `FORENSIC_OCR_TESSDATA_PATH` points at `/models/media/nexusai/models/tessdata-fast/6572...` which does not exist, and the container's own tessdata holds only `eng` and `osd`. The role could not start even if switched on. The two SHA-256 values pinned in the compose file (`eng` 7d4322bd..., `urd` 62e8250c...) match the files of `tesseract-ocr/tessdata_fast` fetched today.
- A second backend exists (`FORENSIC_OCR_BACKEND=paddle`, `multilingual_ocr.py`) but PaddlePaddle is not installed in the worker image.

### Measurements (dev partition, the worker's own processor class, 1 CPU thread per Tesseract call)

**English pages with planted identifiers (T1, 34 dev pages x 4 variants, 1,048 identifiers per variant).** Identifier = the planted string (CNIC, phone, IBAN, plate, IMEI, date, time, amount, FIR, reference) appears verbatim.

| Configuration | Clean: CER / identifiers | Low-res JPEG | Noisy | Skewed 1 degree | s per page |
|---|---|---|---|---|---|
| **As deployed**: fast, `eng+urd`, PSM 11 | 1.0% / 79.3% | 4.8% / 82.2% | 0.7% / 88.1% | 12.5% / 81.6% | 0.9 |
| fast, `eng`, PSM 11 | 0.8% / 79.6% | 0.7% / 89.3% | 0.4% / 88.6% | 12.5% / 81.6% | 0.7 |
| fast, `eng`, PSM 3 | 0.4% / 97.0% | 0.5% / 93.1% | 0.0% / 97.4% | 0.0% / 95.9% | 0.7 |
| **best, `eng`, PSM 3** | **0.4% / 99.7%** | **0.1% / 99.0%** | **0.0% / 99.5%** | **0.0% / 98.3%** | 2.7 |
| best, `eng`, PSM 6 | 0.0% / 99.5% | 0.0% / 99.0% | 0.0% / 99.7% | 0.0% / 97.8% | 2.0 |

Reading: the deployed scene mode (PSM 11) splits identifiers across lines and does not deskew, so identifier exactness is 79% on clean pages; document mode (PSM 3) with the `best` data reaches 99.7%. Adding `urd` to `eng` makes English worse on degraded pages (4.8% against 0.7%). **Gate for English (identifier exact at least 95% on clean print, CER at most 3%): PASS with best + `eng` + PSM 3** (and with fast + PSM 3 on clean: 97.0%).

**Real receipt photographs (T3, CORD v2 test, 72 dev images, bag-of-words F1 against the labelled words; character error is order-sensitive and shown for information):**

| Engine | F1 | CER median | s per image |
|---|---|---|---|
| Tesseract as deployed (fast, `eng+urd`, PSM 11) | 23.8% | 92% | 2.0 |
| Tesseract fast `eng` PSM 3 | 31.7% | 85% | 0.9 |
| Tesseract best `eng` PSM 3 | 33.3% | 83% | 0.8 |
| **PaddleOCR PP-OCRv5 on onnxruntime (detector + English recogniser)** | **78.7%** | **11.6%** | 1.0 |
| **PaddleOCR PP-OCRv5 on onnxruntime (detector + Latin recogniser)** | **79.9%** | **11.8%** | 1.2 |

Reading: Tesseract does not read photographed receipts; the PaddleOCR detector and recogniser (Apache-2.0 ONNX exports, run on the worker's own onnxruntime with `ingestion/forensic_records/onnx_text_ocr.py`) read them with a word F1 2.4 times higher at the same speed. No PaddlePaddle install is needed.

**Urdu (T1, rendered with the OFL fonts Noto Naskh Arabic and Noto Nastaliq Urdu at 34 to 46 px with grain, blur, JPEG and skew; sentences from the text column of the public Urdu OCR set):**

| Engine on single lines (147 Naskh, 144 Nastaliq dev lines) | Naskh clean / noisy / low-res / skewed (mean CER) | Nastaliq clean / noisy / low-res / skewed |
|---|---|---|
| Tesseract fast `urd`, PSM 7 | 9.0% / 9.4% / 8.1% / 20.9% | 30.4% / 31.5% / 24.5% / 43.0% |
| Tesseract best `urd`, PSM 7 | 9.0% / 9.4% / 8.1% / 20.9% | 30.8% / 30.8% / 22.6% / 43.0% |
| PaddleOCR PP-OCRv5 Arabic recogniser (ONNX) | 45.6% / 47.5% / 51.8% / 42.2% | 92.7% / 93.8% / 92.8% / 92.8% |

Medians for Tesseract are 0% (Naskh) and 18% (Nastaliq): most lines are exact and a minority fail badly. The PaddleOCR Arabic recogniser is not usable for Urdu (it writes Arabic letter forms and drops characters). **Gate for Urdu (line CER at most 15% Naskh, at most 25% Nastaliq):** Naskh PASS on clean, noisy and low-resolution lines (9%); skewed lines fail at 21% (single-line mode does not deskew); **Nastaliq FAILS** (30%).

**Public Urdu training crops are not a fair test (found today).** `PuristanLabs1/urdu-ocr-1M` Naskh images are fixed 64 x 512 windows that cut long lines (a 207-character label for a window that shows about 60), and its Nastaliq images are 64 pixels high; scoring against them gives Tesseract a 24% mean error on Naskh and 57% on Nastaliq that mostly measure the crop, not the engine. They are kept only as a source of Urdu sentences.

**English pages, the ONNX engine on the same 34 dev pages x 4 variants:** character error 0.0% on all four variants, identifiers exact 99.7 to 99.8%, about 3 to 4 seconds a page (measured while other jobs ran). The same identifier score as Tesseract `best` in document mode, and far better than either on photographs.

**Urdu pages (T1, 64 dev pages x variants: 7 Naskh and 9 Nastaliq pages per variant, eight to twelve Urdu lines and a Latin table, lines put in position order before scoring; the truth first omitted the Latin table, a mistake of mine found and fixed before the final numbers):**

| Configuration | Naskh CER (clean / noisy / low-res / skewed) | Naskh F1 | Naskh identifiers | Nastaliq CER (clean) | Nastaliq F1 |
|---|---|---|---|---|---|
| As deployed (fast `eng+urd`, PSM 11) | 8.8 / 8.3 / 5.3 / 14.7% | 84 to 91% | 60 to 71% | 33% | 38 to 44% |
| fast `urd+eng`, PSM 3 | 9.9 / 10.7 / 9.2 / 14.4% | 92 to 94% | 88 to 98% | 31% | 43 to 44% |
| **best `urd+eng`, PSM 3** | **9.1 / 9.3 / 7.4 / 13.7%** | **93 to 95%** | **94 to 98%** | 32% | 43 to 45% |
| best `urd+eng`, PSM 6 | 21.6 / 21.5 / 21.5 / 24.0% | 92 to 94% | 84 to 98% | 47% | 42% |
| PaddleOCR Arabic recogniser through the ONNX pipeline | 91 to 94% | 22 to 28% | 67% | 96% | 1% |

**Gate for Urdu (character error at most 15% on Naskh, at most 25% on Nastaliq): Naskh PASS on every variant at page level (9% with best `urd+eng` in document mode, identifiers 94 to 98%); Nastaliq FAILS (32%; words found 45%).** Nastaliq therefore stays "read with low confidence, review required" and is reported as not certified; the Urdu route is Tesseract `best` `urd+eng` in document mode. The page-level numbers match the line-level ones.

**Routing: Tesseract's script pass is not reliable.** On the dev images it called 49 of 64 Urdu pages Arabic and the rest Japanese (13) or Katakana (2); it called 57 of 60 English pages Latin; on 60 receipts it gave Latin 15 times and Cyrillic, Greek, Arabic, Han and others or nothing for the rest. A router built on it would send a quarter of the Urdu pages and almost all receipts to the wrong engine. The PaddleOCR-Latin recogniser is also confidently wrong on Urdu pages (median confidence 0.89, only 8% below 0.75), so low confidence cannot catch a misrouted page either. The router therefore decides the script with a **probe**: the small Arabic recogniser reads the longest six text lines and the share of Arabic-script letters in what it writes separates the scripts (measurement below). Its text is never stored.

**What went live (2026-10-21):** the OCR role (`FORENSIC_OCR_BACKEND=auto`): script probe, Urdu pages to Tesseract `best` `urd+eng` in document mode, everything else to PP-OCRv5 on onnxruntime with a Tesseract second opinion when the reading is not confident. Language data and ONNX models are pinned by SHA-256 in `docker-compose.forensic-records.media.yaml`; the model folder is `models/media` of the lane checkout (git-ignored; provisioned by `tools/media_data/provision_models.py`). The worker image in production carries a media_pipeline.py 500 lines older than the repository's, so the overlay patches **the image's own file** in four places instead of copying the repository file (a straight copy failed to import `roman_urdu`); rollback image `before-media-ocr-20261021` (`db43bb68c042`). Live check: two uploads to the synthetic collection `nexusai-media-wave1-20261021` completed with 40 OCR observations of mean confidence 0.988.


## X: captures, databases and archives (hostile-input suite)

**Set:** 21 inert files generated by `tools/media_data/gen_hostile.py` (3.1 MB on disk): a zip bomb (1 MB stored, about 1 GB expanded), a ZIP with 20,000 members, a ZIP nested 14 deep, a ZIP and a TAR with traversal and absolute names and a symlink, a PDF with a ZIP appended, an executable named `.jpg`, a PNG that declares 60,000 x 60,000 pixels, a JPEG with a 65 KB EXIF block, a WAV that claims a 4 GB data chunk, a truncated MP4, a SQLite file with a trigger and one with a corrupt header, a CSV with formula injection, two complete XLSX packages (one with a billion-laughs entity expansion in the shared strings, one with an external entity in the workbook), a text file that tells a language model to ignore its instructions, an empty file, a one-byte file and a 190-character file name. Uploaded through the live API into the synthetic collection `nexusai-hostile-suite-20261021`; the worker ran with the OCR role on.

| What happened | Files |
|---|---|
| **Registered and never extracted or executed** (`archive_inventory_pending`, `database_adapter_pending`) | all five archives (zip bomb, 20,000 members, nested 14, traversal ZIP, TAR with symlink), both SQLite files, `empty.bin`, `one_byte.bin`, `html_script.html` (stored as inert text) |
| **Decode refused by the API's deterministic validation** (`downstream_decode_allowed: false`, `manual_review_required`) | the executable named `.jpg`, the 60,000 x 60,000 PNG, the JPEG with the huge EXIF block (the last one is a conservative refusal) |
| **Processed safely by the worker** | CSV with formulas and the instruction-injection text (stored as text and never obeyed), the long file name, the WAV with the 4 GB claim (technical probe only, no allocation) |
| **Failed cleanly into the dead letter queue** | the truncated MP4 (5 attempts), the polyglot PDF (extraction failed), both XLSX packages: the reader says "DTD or entity declarations are not permitted in XLSX XML part ..." for the shared strings and for the workbook |

During the whole run the worker held about 770 MB and stayed healthy; the API stayed up; nothing was written outside the evidence store. **Gate for X: PASS** with two stated limits: (1) archive members are never extracted today (inventory is pending), so traversal handling at extraction time is not exercised; the day an extractor exists this suite is its first test; (2) the first version of my two XLSX fixtures lacked required parts and was rejected before any XML was read, which proved nothing: that is why they were rebuilt as complete packages. A false alarm worth noting: the 2 MB archive appeared to hang or return HTTP 500 through the dev proxy; the cause was my test client's `Expect: 100-continue` header (the Vite proxy mishandles it for bodies over 1 MB); without it the API answers 200 in 2.7 s.

## F: faces (level L0, detection only; decision D5)

**Set:** 373 Open Images test photos with face boxes (dev partition; CC BY 2.0 photos; 551 labelled faces, of which 471 are at least 40 pixels wide), the worker's detector YuNet 2023mar (MIT, licence file read). Scoring is by box only: no embedding, no identity.

| Score threshold | Recall, faces at least 40 px wide | 20 to 40 px | under 20 px | "False" boxes per photo |
|---|---|---|---|---|
| 0.3 | 84.3% | 72.1% | 21.1% | 2.87 |
| 0.5 | 81.1% | 68.9% | 21.1% | 2.02 |
| 0.7 | 77.1% | 65.6% | 15.8% | 1.55 |
| 0.9 | 49.3% | 24.6% | 0.0% | 0.41 |

**The false-box number cannot be trusted on this data.** Open Images labels only some of the faces in crowded photographs. On 224 photos with no labelled face the detector still finds 2.2 boxes a photo at score 0.3. A contact sheet of 48 randomly chosen "false" detections at score 0.7 (`tools/media_data/face_audit.py`) shows mostly real faces that are small, partly turned or blurred and simply unlabelled (my reading: about four in five are faces, the rest blurred blobs; one or two are not faces); that puts the true false-box rate somewhere around 0.3 per photo at score 0.7, well above the pre-registered 0.1.

**Gate for F (recall at least 80% for faces 40 px or wider at no more than 0.1 false boxes per image): recall MET at score 0.5 (81%); the false-box criterion is NOT met** (about 0.3 per photo even after discounting unlabelled faces) **and cannot be measured cleanly without a face-free negative set. The role stays OFF.** What would change it: a negative set (photographs verified to contain no person) or a decision by the owner to accept a face count labelled "candidate, review required" at score 0.8 (recall 72%); either needs no new model.

## A: audio and speech (decided and live 2026-10-22)

**Set:** FLEURS test split (CC BY 4.0; ledger DAT-004), dev partition only: Urdu `ur_pk` first 24 clips (3.9 minutes), English `en_us` 16 clips (2.4 minutes), Punjabi `pa_in` 16 clips (3.5 minutes, Gurmukhi) and Pashto `ps_af` 16 clips (3.3 minutes). Word error after the written normalisation (NFKC, lower case, punctuation and symbols removed, Arabic-Indic digits mapped, whitespace collapsed). faster-whisper (CTranslate2) in int8 on the laptop CPU. **Every number was measured with nothing else running**: an earlier run of the small model that shared the machine with other measurements took four hours for six minutes of audio (real-time factor 38) and was discarded.

**Bake-off (the library directly, `tools/media_data/asr_eval.py`, Urdu, language told):**

| Model, decoding | Word error mean (median) | Character error | Real-time factor | Script correct | Loops | Licence |
|---|---|---|---|---|---|---|
| small, 8 threads, beam 5 (configured before) | 32.7% (31.0%) | 13.1% | 1.04 | 100% | 0 of 24 | MIT |
| medium, 8 threads, beam 5 | 26.2% (26.5%) | 9.8% | 3.43 | 100% | 0 of 24 | MIT |
| large-v3-turbo, 8 threads, beam 5 | 20.8% (21.4%) | 8.5% | 1.74 | 100% | 0 of 24 | MIT |
| **large-v3-turbo, 8 threads, beam 1** | **21.2% (21.1%)** | **8.4%** | **1.49** | 100% | 0 of 24 | MIT |
| large-v3-turbo, 12 threads, beam 5 | 20.8% (21.4%) | 8.5% | 2.19 | 100% | 0 of 24 | MIT |
| large-v3-turbo, 12 threads, beam 1 | 21.2% (21.1%) | 8.4% | 1.59 | 100% | 0 of 24 | MIT |

Reading: beam 5 costs 0.4 points of word error and 17% of time, so greedy decoding is the right trade; more than 8 threads make it slower, not faster. Turbo with beam 1 and 8 threads is the lowest word error at a real-time factor under the pre-registered ceiling of 1.5 (it clears it by 0.01 in the library run, which is why the service run below matters). English with the library, beam 5: turbo 4.2% against small 5.2%.

**Through the live LocalAI service (`tools/media_data/asr_service_check.py`, the path the worker uses; the turbo model installed with `options: beam_size:1, cpu_threads:8`):**

| Language and model | Word error mean (median) | Real-time factor, all requests (without the first, which loads the model) | Script correct | Loops | Every clip has a timed segment |
|---|---|---|---|---|---|
| Urdu, small (before) | 33.8% (29.9%) | 0.55 (0.53) | 100% | not scored | yes |
| **Urdu, turbo** | **20.9% (19.6%)** | **1.45 (1.35)** | 100% | 0 of 24 by the library run | yes |
| English, small | 5.5% (4.1%) | 0.44 (0.34) | 100% | 0 of 16 | yes |
| **English, turbo** | **4.7% (4.0%)** | 1.26 (1.26) | 100% | 0 of 16 | yes |

Gate (fixed before the runs): Urdu word error at least 20% relatively below the current small model on the same clips: **38% below (20.9% against 33.8%), met**; English no worse: **4.7% against 5.5%, met**; correct script at least 99%: **100%, met**; loops at most 1%: **0, met**; every segment with start and end: **met**; real-time factor at most 1.5: **1.45, met**. Memory: with the language model and both speech models resident, the Docker VM's lowest free memory was 1.03 GiB; with turbo alone next to the language model 1.67 GiB.

**A finding the plan did not have: no language given.** The worker sends no language unless the upload carries `asr_language`. Told nothing, turbo detected the right language on **16 of 16 English clips** but only **17 of 24 Urdu clips**: the other 7 were called Hindi and written in Devanagari (script correct 70.8%, word error 43.0% against Urdu references). Spoken Hindi and Urdu are one language and Whisper picks between them by script habit. Decision: when no language was requested and the service hears Hindi, the worker reads the recording again as Urdu and stores that reading with `first_detected_language: hi` and `language_resolution` saying so (`FORENSIC_ASR_HINDI_AS_URDU`, on in this deployment, off in the code unless set). The cost is a second reading for those recordings. Reading without a language also costs about twice the time per short clip (real-time factor 2.7 to 2.9 on these short clips: the language is found by a first pass over the first 30 seconds), a fixed cost that does not grow with a long recording.

**Punjabi (Gurmukhi) and Pashto, language told:** Punjabi turbo 91.2% word error (small 104.5%, script correct 0% and one loop), Pashto turbo 96.3% (small 96.9%). Neither is a transcript. Per the pre-registered rule they are not switched off but never shown plainly: every segment in a language of `FORENSIC_ASR_LOW_RESOURCE_LANGUAGES` (default `pa,ps`) is stored with `reliability_state: LOW_RESOURCE_LANGUAGE`, a note with the measured figure and a warning that the interface shows ("a pointer to listen, not a transcript"). The interface has to show that state before these readings are treated as more than a lead (Stage 3 of the interface programme).

**What went live (2026-10-22):** (1) `backend/python/faster-whisper/backend.py` reads `beam_size` and `cpu_threads` from the model YAML `options` (defaults unchanged: beam 5, CTranslate2 threads; the patch is in the shared `nexusai_backends` volume with the original beside it as `backend.py.before-asr-options-20261022`); (2) the model `faster-whisper-turbo` (hub revision 4df90f75, `model.bin` sha256 e76620f8…, ledger MOD-043) installed in the shared `nexusai_models` volume with its YAML; (3) the worker overlay `037b7b720365` (rollback `before-media-asr-20261022` = the OCR image `e7f47e81d59c`) with `FORENSIC_ASR_ENABLED=true`, `FORENSIC_ASR_MODEL=faster-whisper-turbo`, recordings longer than 600 seconds read by the small model so one long file cannot hold the service for hours, a 30-minute request timeout, the Hindi re-read and the low-resource marking; offline behaviour test `tools/media_data/test_patched_asr.py` (6 behaviours) passes inside the worker image. End to end through the upload path, three FLEURS clips into `nexusai-media-wave1-20261021`: an Urdu clip the service had called Hindi came out in Urdu script with a Roman Urdu derivative; an English clip came out English; a Punjabi clip with `asr_language=pa` was stored `LOW_RESOURCE_LANGUAGE`.

**Not done / limits:** 24 and 16 clips are small samples (wide intervals); FLEURS is read speech, not telephone-band, noisy or conversational audio, so real recordings will score worse and a sample of the owner's own kind of audio is needed before numbers are quoted to a court; fine-tuned Urdu Whisper models were not run because their training data may include FLEURS and Common Voice is no longer on the hub without an account; speaker identity, diarisation and any claim about who is speaking are not offered; the long-recording fallback was proven by the offline test, not by a 10-minute file.
