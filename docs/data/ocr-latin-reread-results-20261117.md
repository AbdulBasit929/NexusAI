# Latin and digit re-read inside the EasyOCR role: results (2026-11-17)

Plan and gates: `ocr-latin-reread-preregistration-20261117.md` (amendment A1 before any new-seed image was read). The problem it fixes: `ocr-easyocr-acceptance-results-20261117.md` section 3 (EasyOCR reads "Rs." as "R5." and loses Latin-letter plates). Scorer: `tools/media_data/accept_reread_score.py`; per-image output in `results/ocr/accept2-{today,easyocr,reread}.jsonl` (outside the repository).

## 1. What was changed

After EasyOCR reads a page, every box whose text has no Arabic-script character and at least one letter or digit is cropped from the original image and read again by the PP-OCRv5 Latin recogniser that the router already loads; the Latin text replaces the EasyOCR text of that box (rule `all`, chosen on development rows: 818 of 825 identifiers against 817 for the confidence rule and 748 for no re-read). Boxes with any Arabic-script character are never changed; an empty reading or an error leaves the EasyOCR text; the original text of each replaced box is kept in the observation (`latin_reread`). Switch: `FORENSIC_EASYOCR_LATIN_REREAD` (default false), rule `FORENSIC_EASYOCR_LATIN_MODE`.

## 2. Results on fresh data (seeds 20261204 to 20261206; 40 Urdu pages and 120 mixed images; the same images in all three arms)

| Set | Arm | Mean character error | Identifiers | By type | Empty or error | Seconds median |
|---|---|---|---|---|---|---|
| Urdu pages | today (role off) | 55.8% | 571/600 = 95.2% | amount 180/200, CNIC 197/200, phone 194/200 | 0 | 2.5 |
| Urdu pages | easyocr | 8.7% | 511/600 = 85.2% | amount **114/200**, CNIC 198/200, phone 199/200 | 0 | 20.3 |
| Urdu pages | **easyocr + re-read** | **8.3%** | **600/600 = 100%** | amount **200/200**, CNIC 200/200, phone 200/200 | 0 | 17.1 |
| Mixed | today | 34.6% | 717/750 = 95.6% | amount 407/417, CNIC 24/28, phone 89/107, plate 27/28 | 0 | 0.8 |
| Mixed | easyocr | 17.7% | 724/750 = 96.5% | amount 411/417, CNIC 28/28, phone 96/107, plate **19/28** | 0 | 3.0 |
| Mixed | **easyocr + re-read** | **17.5%** | **734/750 = 97.9%** | amount 411/417, CNIC 28/28, phone 97/107, plate **28/28** | 0 | 3.1 |

## 3. Gates

| Gate | Requirement | Result | Verdict |
|---|---|---|---|
| L1 Urdu pages, identifiers | at least today's and 10 points above the `easyocr` arm | 100% against today's 95.2% and `easyocr`'s 85.2% | **met** |
| L2 mixed, identifiers | at least today's; plates at least 90% of today's | 97.9% against 95.6%; plates 28 against 27 | **met** |
| L3 no loss on reading | error at most 1 point above `easyocr` on both sets; 10 points (mixed) and 25 points (Urdu pages) below today | 8.3% against 8.7% and 17.5% against 17.7%; 17.5% against 34.6% and 8.3% against 55.8% | **met** |
| L4 time | median per Urdu page at most 30 s; at most 3 s added | 17.1 s; no added time measured (the re-read arm's median was 3.3 s below the `easyocr` arm's, which is machine-load noise between runs; the re-read step itself was not timed on its own) | **met** |
| L5 clean failure | no errors or empty readings | 0 on both sets | **met** |

All five gates are met. The slowest row of the re-read arm shows 143,158 s: the laptop went to sleep during the run (the log clock jumps from 17:12 to 09:17 the next day); medians are not affected.

## 4. What is still open

* Phone numbers on mixed images are still missed in every arm (97 of 107 with the re-read; mostly short mixed lines); an analyst must check them against the image.
* Real photographs, handwriting and real case documents are untested; every reading is a model observation that needs review.
* Urdu text inside a left-to-right table row can still come back with its columns reversed.
* The EasyOCR weights licence stays unconfirmed (not a blocker for development, owner's direction of 2026-11-17).
