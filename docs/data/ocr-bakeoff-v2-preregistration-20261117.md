# OCR bake-off v2: English, Urdu and mixed Urdu and English text (plan and gates, fixed before the sealed run; 2026-11-17)

Owner direction, 2026-11-17: research the best OCR for English, Urdu and mixed text from online sources and GitHub, and implement the best, accurate one. The research is `docs/research/OCR_MULTILINGUAL_RESEARCH_20261117.md` (primary pages read, sources linked). This file fixes the readers, the data, the measures and the gates **before any reader is run on a sealed row**, and is not edited afterwards; changes are dated amendments at the end. Results go in `ocr-bakeoff-v2-results-20261117.md`.

## 1. Question

Is there a reader that is **licence-clean, CPU-safe on this laptop (at most 1.5 GB resident beside the stack) and accurate enough** to replace or complement today's routes (Tesseract `best` for Urdu pages, PaddleOCR PP-OCRv5 for Latin pages) for Urdu print, Nastaliq print and pages that mix Urdu and English? The first candidate is **EasyOCR 1.7.2** with `['ur','en']` (Apache-2.0; the weights' licence is unconfirmed, ledger MOD-047 to MOD-049, evaluation only until it is).

## 2. Readers (settings fixed)

| Id | Reader | Settings |
|---|---|---|
| T | Tesseract 5 `best`, `urd+eng`, page mode 3 (the live Urdu route) | the worker's own processor class, one thread; on single lines page mode 7 as in the earlier results |
| P | PaddleOCR PP-OCRv5 ONNX, Latin recogniser, detector + recogniser | as deployed (the live Latin route) |
| E | **EasyOCR 1.7.2**, `Reader(['ur','en'], gpu=False, model_storage_directory=..., download_enabled=False)`, `readtext(image, paragraph=False)`; detector parameters at their defaults | one CPU thread (`torch.set_num_threads(1)`); lines are assembled by the rule in section 6 |
| E-en | EasyOCR `['en']` | English sets only |

Not run here (they need a GPU host or a licence decision, see the research): Qwen2-VL/Qaari at full size, Qwen3-VL, PaddleOCR-VL, DeepSeek-OCR, dots.ocr. PaddleOCR-VL 0.9B is the next CPU-capable candidate and gets its own plan.

## 3. Sets (each split by hash, development to tune, sealed read once per reader)

| Set | Content | Source | Truth | Rows |
|---|---|---|---|---|
| S1 English pages | A4 pages with planted identifiers, 4 degradations | `generated/ocr_en` (T1) | exact | the sealed partition |
| S2a Urdu lines | Naskh and Nastaliq lines, 4 degradations | `generated/ocr_ur` kind line (T1) | exact | sealed |
| S2b Urdu pages | Naskh and Nastaliq pages, with a Latin table | `generated/ocr_ur` kind page (T1) | exact | sealed |
| S3 real Nastaliq print | newspaper paragraphs | Urdu Newspaper Benchmark (DAT-024, private evaluation) | exact | the 60 sealed rows already fixed in `urdu-nastaliq-reader-preregistration-20261116.md` |
| **S4 mixed Urdu and English** | mixline, form, statement, receipt, chat; Urdu text, English words, brands, numbers, CNIC, phones, dates, amounts, plates; 4 degradations | `generated/ocr_mixed` built by `tools/media_data/gen_mixed.py` (T1) | exact, logical order | sealed |
| S5 receipts | photographed English receipts | CORD v2 test (T3) | words | sealed |

S1, S2 and S5 have been measured on their development partitions before; their sealed partitions have not been read by any reader. S3 development rows (12) and sealed rows (60) are those of the Nastaliq plan; Tesseract has already been run on them (68.4% sealed).

## 4. Measures

Character error rate and word error rate after the normalisation of `tools/media_data/ocr_eval.py` (NFKC, zero-width removed, whitespace collapsed; characters without spaces for CER); bag-of-words F1 (order free); **identifier exactness** (the planted CNIC, phone, amount, date, plate must appear verbatim in the output); for S4 the **order gap** = bag-of-words F1 minus (1 - word error): a reader that finds the words but returns them in the wrong order shows a large gap. Also seconds per row (median, maximum) and peak resident memory, one thread. 95% intervals by bootstrap over rows where the set is large enough.

## 5. Gates for adopting EasyOCR (all on the sealed rows)

Written before the sealed run. E is adopted as the Urdu and mixed-text reader only if **all** of:

* **U1 (real Nastaliq print, S3):** pooled character error at most **25%** and at least **10 points** below T on the same rows.
* **U2 (rendered Urdu, S2):** line character error at most **15%** for Naskh and **25%** for Nastaliq, and at most T's.
* **U3 (mixed, S4):** identifier exactness at least **90%** (pooled over types) and bag-of-words F1 at least **85%**.
* **U4 (no loss on English, S1):** identifier exactness of E-en or E at most **2 points** below the live Latin route P (99.7% on development).
* **U5 (deployable):** at most **30 seconds** per A4 page and **1.5 GB** resident on one CPU thread.
* **U6 (clean failure):** empty or runaway outputs on at most **5%** of rows.

Possible outcomes: all met, E replaces the Urdu route and handles mixed pages behind a script check; U1 or U2 met but U3 missed, E reads Urdu and the mixed case needs routing by script (a per-line script split) and a second plan; none met, nothing changes and the finding is recorded. Whatever the result, the per-set numbers of T, P and E are reported side by side.

## 6. Rules

* One heavy job at a time, in the memory-capped container with the watchdog used on 2026-11-17 (memory limit 3,000 MB for EasyOCR, kill only the test container at host available memory below 600 MB); the model container stays up (EasyOCR needs under 1.5 GB). Nothing is stopped.
* **Line assembly for E (fixed):** `readtext` boxes are grouped into lines by vertical overlap (a box belongs to the line whose vertical centre lies within half the median box height); lines are ordered top to bottom; inside a line the boxes are ordered right to left when more than half of the line's letters are Arabic-script, left to right otherwise; boxes are joined by a single space. No other post-processing. The same rule is applied to every set.
* The detector or recogniser parameters of E may be changed on **development** rows only, each change written below as an amendment before the sealed run; the sealed rows are read once per reader.
* The mixed set is synthetic (T1): it measures reading order and identifiers under controlled layout and says nothing about real photographs. A real mixed set from consented documents is owed (decision D26) and would repeat S4 with real pages.
* A result below a gate is reported as below. The licence of the EasyOCR weights is not confirmed: even a full pass is "accurate, licence open" until MOD-047 to MOD-049 are resolved.

## Amendments

**A1, 2026-11-17, after a development-row check and before any sealed row was read by any reader.** *What was found (development rows only, mixed set, 30 images):* with the line rule of section 6 (a box joins the line whose vertical centre lies within half the median box height), bank-statement rows were split into two lines because the Urdu narration sits lower than the digits of its row. *Change:* a box joins a line when its vertical extent overlaps the line's by at least 40% of the smaller height (`easyocr_reader.py`, `OVERLAP = 0.4`); nothing else of the assembly changes (top to bottom, right to left inside a line when more than half of its letters are Arabic-script, one space between boxes). *Effect on the development rows (3 per kind, clean and noisy):* mixed line CER 11.6%, F1 61.3%, identifiers 2 of 2; form 13.6%, 83.0%, 12 of 12; statement 31.9%, 91.6%, 69 of 69; receipt 10.6%, 83.9%, 23 of 25; chat 17.7%, 74.6%, 6 of 6. *Known and not changed:* a row of a left-to-right table that holds Urdu text is read right to left by the rule, so a statement row with an Urdu narration returns its columns in reverse order: the words and identifiers are right (F1 91.6%, identifiers 69 of 69) and the character error is inflated by the order. The direction of a mixed row is a layout fact that no reader is told; Tesseract's own order puts it left to right. The gates U3 use identifiers and F1, not character error, and the report gives the order gap beside each result.

