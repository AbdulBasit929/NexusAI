# Latin and digit re-read inside the EasyOCR role (plan and gates, fixed before any measurement of the fix; 2026-11-17)

## 1. The problem (found in the acceptance, `ocr-easyocr-acceptance-results-20261117.md` section 3)

EasyOCR's recogniser was trained for Arabic script; it reads Latin letters less well. On the fresh acceptance data it read the currency prefix "Rs." as "R5." (amount identifiers on Urdu pages 110 of 200, today's route 178 of 200; the digits themselves were right) and lost plates written in Latin letters on mixed images (15 of 28 against 27 of 28). It read CNIC numbers and phones better than today's route. The PP-OCRv5 Latin recogniser, already in the worker, reads Latin text and digits very well (English pages: identifiers 99.7%).

## 2. The change

After EasyOCR has detected and read the boxes of a page, every box whose text contains **no Arabic-script character** (the Arabic, Arabic Supplement and presentation-form blocks, which include the Arabic-Indic digits) and at least one letter or digit is cropped from the original image (coordinates scaled back from the size cap, 2 px margin) and read again by the PP-OCRv5 Latin recogniser. Boxes with any Arabic-script character are never changed. Which reading replaces the EasyOCR text is one of two rules, **chosen on development rows only** (section 4):

* `all`: the Latin recogniser's text replaces the box text whenever it is not empty;
* `confidence`: it replaces the box text only when its mean confidence is higher than EasyOCR's.

An empty reading, an exception or a missing recogniser leaves the EasyOCR text. The original EasyOCR text of every replaced box is kept in the observation (`latin_reread`) with both confidences, and the result metadata counts boxes checked and replaced. Nothing is merged inside a box; no model is added to the image (the recogniser and its pinned hash are the ones the router already loads); the switch is `FORENSIC_EASYOCR_LATIN_REREAD` (default false until the acceptance below passes) and the rule `FORENSIC_EASYOCR_LATIN_MODE`.

## 3. Data

* **Development rows** (to choose the rule and nothing else): the development partitions of the generated Urdu pages (`ocr_ur`, kind page) and mixed set (`ocr_mixed`), 24 Urdu pages (3 per style and degradation) and 60 mixed images (12 per kind), in hash order, the same for both rules. EasyOCR runs once per image; the three variants (none, `all`, `confidence`) are computed from the same detections and boxes.
* **Fresh acceptance** (to decide whether the fix ships): new seeds **20261204** (English pages), **20261205** (Urdu) and **20261206** (mixed), the rows nobody has read. The first acceptance's data (seeds 20261201 to 20261203) was looked at to find the problem and is **not** used again for any decision about the fix. Sets `ur_pages` (40) and `mixed` (120); three arms through the worker's router with the settings of the first acceptance: `today` (role off), `easyocr` (re-read off), `easyocr+reread`.

## 4. Choosing the rule on development rows

The rule with the higher pooled identifier exactness over the development Urdu pages and mixed images wins; on a tie the lower mean character error; a rule whose mean character error is more than **0.5 points** worse than no re-read on either set is not eligible. If neither rule beats "none" on identifiers, the fix is **not** built into the default and the result is reported.

## 5. Gates on the fresh acceptance (all must hold; each is reported either way)

* **L1 identifiers, Urdu pages:** `easyocr+reread` at least the `today` arm's, and at least **10 points** better than the `easyocr` arm (the first acceptance showed a 7-point deficit to today's).
* **L2 identifiers, mixed:** at least the `today` arm's, pooled over types; the plate type at least **90%** of the `today` arm's plates.
* **L3 no loss on reading:** mean character error of `easyocr+reread` at most **1 point** above the `easyocr` arm on both sets, and still at least 10 points below `today` on the mixed set and 25 points below on the Urdu pages.
* **L4 time:** median seconds per Urdu page at most 30 s; the re-read adds at most **3 s** per page to the `easyocr` arm.
* **L5 clean failure:** no errors or empty readings on either set.

## 6. What a pass does and does not mean

A pass means the Latin and digit text on synthetic Urdu and mixed pages is read as well as today's route while Urdu text keeps the EasyOCR gain. It does not cover real photographs or handwriting; the readings stay model observations that need review, and the weights licence of the EasyOCR files stays unconfirmed (not a blocker for development, owner's direction of 2026-11-17).

## Amendments

**A1, 2026-11-17, after the development-row comparison and before any fresh image of seeds 20261204 to 20261206 was read.** *Result* (`tools/media_data/measure_latin_reread.py`, 24 development Urdu pages and 60 development mixed images, identical EasyOCR detections for all three variants; `results/ocr/latin-reread-dev-20261117.json`):

| set | rule | mean character error | identifiers | by type | boxes replaced |
|---|---|---|---|---|---|
| Urdu pages | none | 8.5% | 297/360 = 82.5% | amount 57/120, CNIC 120/120, phone 120/120 | 0 |
| Urdu pages | all | 7.9% | **360/360 = 100%** | amount 120/120, CNIC 120/120, phone 120/120 | 370 |
| Urdu pages | confidence | 7.9% | 359/360 = 99.7% | amount 119/120 | 355 |
| mixed | none | 14.6% | 451/465 = 97.0% | amount 265/269, phone 55/62, plate 9/12 | 0 |
| mixed | all | 14.3% | **458/465 = 98.5%** | amount 268/269, phone 56/62, plate 12/12 | 690 |
| mixed | confidence | 14.3% | 458/465 = 98.5% | same as all | 637 |

*Decision by the rule of section 4:* pooled identifiers 818/825 for `all` against 817/825 for `confidence` and 748/825 for none, character error not worse for either rule: **`all`** is chosen (it is also the simpler rule). The development rows were generated with the registered seeds and are the partitions used to tune earlier choices; they are not the fresh sets. *Wiring:* arms `today`, `easyocr` and `reread` run through the worker overlay image `easyocr-reread-20261117` (62 unit tests pass inside it) with `FORENSIC_EASYOCR_LATIN_REREAD` false, false and true; the sets are `ur_pages` (40) and `mixed` (120) of the new seeds (folder suffix `_accept2`); the English, line and newspaper sets are not run because the fix only touches boxes the EasyOCR role reads and the first acceptance already covers them. Scorer: `tools/media_data/accept_reread_score.py`.
