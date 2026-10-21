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

