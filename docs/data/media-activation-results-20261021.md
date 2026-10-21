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

## A: audio and speech (first results; the English and Punjabi/Pashto runs, the speed variants and the live switch follow)

**Set:** FLEURS test split, Urdu (`ur_pk`), read speech with transcripts (CC BY 4.0; ledger DAT-004), dev partition, first 24 clips (3.9 minutes). faster-whisper (CTranslate2) in int8 on the laptop CPU, 8 threads, beam 5, language told. Word error after the written normalisation (NFKC, lower case, punctuation and symbols removed, Arabic-Indic digits mapped, whitespace collapsed). **Run alone**: an earlier run of the same small model that shared the machine with other measurements took four hours for six minutes of audio (real-time factor 38), so that timing was discarded; its word error (35.8%, 40 clips) is in line with the clean run.

| Model | Word error mean (median) | Character error | Real-time factor | Script correct | Loops | Licence |
|---|---|---|---|---|---|---|
| faster-whisper **small** (the one configured today) | 32.7% (31.0%) | 13.1% | 1.04 | 100% | 0 of 24 | MIT |
| faster-whisper **medium** | 26.2% (26.5%) | 9.8% | 3.43 | 100% | 0 of 24 | MIT |
| faster-whisper **large-v3-turbo** | **20.8% (21.4%)** | **8.5%** | 1.74 | 100% | 0 of 24 | MIT |

Reading: turbo is the most accurate model on Urdu and faster than medium; it cuts the small model's word error by 36% (relative). Its real-time factor of 1.74 at 8 threads is above the pre-registered ceiling of 1.5. Per the rule the selection is the lowest word error **among models at or below 1.5**, which would leave the small model; the speed variants (14 threads, greedy decoding) decide whether turbo can meet the ceiling.

Caveats: 24 clips is a small sample (the interval is wide); FLEURS is read speech, not telephone-band or conversational; fine-tuned Urdu Whisper models were not run because their training data may include FLEURS and Common Voice is no longer on the hub without an account.
