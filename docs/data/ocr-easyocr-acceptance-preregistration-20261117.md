# Routed OCR role with EasyOCR: product-level acceptance (plan and gates, fixed before any fresh image is read; 2026-11-17)

Context: the OCR bake-off v2 (`ocr-bakeoff-v2-preregistration-20261117.md`, results `ocr-bakeoff-v2-results-20261117.md`) measured the readers one by one with a lab script. The role that would run in the worker is **the router** (`ingestion/forensic_records/ocr_router.py` v5) with the EasyOCR engine (`easyocr_ocr_processor.py`) behind the script probe. This acceptance measures that role itself, built by `OcrRouter.from_environment()` with the worker's environment, on **fresh data nobody has read**. Whatever the result, the numbers of both arms are reported side by side. This file is not edited after the first fresh image is read; changes are dated amendments at the end.

## 1. Question

Does the routed role with EasyOCR beat today's router on Urdu, mixed and real newspaper pages, **without losing anything on English**, within time and memory a worker can afford, and without hidden failures?

## 2. Arms

| Arm | Router |
|---|---|
| `today` | `OcrRouter.from_environment()` with `FORENSIC_OCR_EASYOCR_ENABLED=false`: the router as deployed (v5 with the engine off behaves as v4) |
| `easyocr` | the same with `FORENSIC_OCR_EASYOCR_ENABLED=true` and the settings in section 5 |

Both arms run in the same worker overlay image, one process each, one image at a time, the model container left running, memory-capped, no network.

## 3. Fresh data (new seeds, never used to choose anything)

| Set | Source | Images (fixed pseudo-random order by id hash, the same for both arms) |
|---|---|---|
| `en` English pages | `gen_ocr_pages.py`, seed 20261201 | 48 |
| `ur_pages` Urdu pages | `gen_urdu.py`, seed 20261202, kind page | 40 |
| `ur_lines` Urdu lines | same run, kind line | 60 |
| `mixed` Urdu and English | `gen_mixed.py`, seed 20261203 | 120 |
| `unb` real Nastaliq newspaper print | Urdu Newspaper Benchmark, sealed side rows 61 onward (the bake-off read rows 1 to 60) | 30 |

Tiers: T1 (generated), T3 (public newspaper print, private evaluation, never copied into the repository or an image). Tesseract has earlier read all newspaper rows (the baseline over the whole set); no EasyOCR reading of the fresh rows exists.

## 4. Gates (all must hold for "accepted for a switch-on decision"; each is reported either way)

* **R1 real Nastaliq print:** easyocr arm pooled character error at most **25%** and at least **10 points** below the `today` arm on the same rows.
* **R2 Urdu:** mean character error on Naskh lines at most **15%** and on Nastaliq lines at most **25%**, on Urdu pages at most **15%**, each also at most the `today` arm's.
* **R3 mixed:** identifier exactness (pooled) at least **90%** and at least the `today` arm's; mean character error at least **10 points** below the `today` arm's. Word F1 is reported and not gated (it missed its gate in the bake-off, U3, and that is carried as a known limit).
* **R4 no loss on English:** identifier exactness of the `easyocr` arm within **2 points** of the `today` arm, and at most **2%** of English images sent to EasyOCR.
* **R5 routing:** at least **98%** of the Urdu pages are read by EasyOCR and at most 2% of the English pages and receipts (the dev rule for T_low, amendment A1; the share of mixed images sent to EasyOCR is reported per kind, not gated).
* **R6 time and memory:** median seconds per Urdu page at most **30 s** in the `easyocr` arm with the settings below; peak resident memory of the worker with the role at most **1.5 GB** more than without (measured in the speed run, development rows).
* **R7 clean failure:** an error or an empty reading on at most **5%** of the images of every set in the `easyocr` arm; an exception of the router is counted as a failed image and is never hidden.

## 5. Settings fixed before the run

* `FORENSIC_OCR_ROUTER_URDU_SHARE=0.10` (amendment A1), `FORENSIC_EASYOCR_THREADS=4`, `FORENSIC_EASYOCR_MAX_SIDE=1400`, `FORENSIC_OCR_ROUTER_CONFIDENT=0.75` and the rest of the router environment exactly as the running worker has it (read from the container).
* Chosen on development rows only: the share (router-share measurement), threads and size cap (speed and large-scan measurements; `ocr-bakeoff-v2-results-20261117.md` section 6).
* Image: the worker overlay built by `Build-EasyOcrOverlay.ps1` on the running worker image; models mounted read-only from `models/media`.

## 6. Scoring

`tools/media_data/accept_router_score.py` (character error on the NFKC-normalised text without spaces; pooled for the newspaper rows by edit counts, mean of per-image rates elsewhere; bag-of-words F1; identifiers found verbatim; seconds). Tools: `accept_router_prepare.py` (fresh sets), `accept_router_runner.py` (the arm), `accept_router_score.py`.

## 7. What a pass does and does not mean

A pass means: on synthetic Urdu and mixed pages and on public newspaper print, the routed role reads better than today's, loses nothing on English, and fits the time budget. It does **not** mean the role may read real case documents: that needs the real consented Urdu and mixed documents (decision D26) and the weights licence answer (ledger MOD-047 to MOD-049). The role ships **off**; switching it on is a separate decision of the owner.

## Amendments

**A1, 2026-11-17, after the router-threshold measurement on development rows and before any fresh image was read.** *What was found* (`tools/media_data/measure_router_share.py`, `choose_router_threshold.py`; 400 mixed, 136 English, 64 Urdu pages and 60 receipts, development rows only; per-image shares in `results/ocr/router-share-dev-20261117.json`): the probe's Arabic-script share is at most **0.047** on every English page and receipt and at least **0.50** on every Urdu page, so any threshold from 0.05 to 0.50 sends 100% of English pages and receipts to the Latin route and 100% of Urdu pages to EasyOCR. The rule written for mixed pages (at least 90% to EasyOCR) **cannot be met with this probe**: at 0.10 the mixed images sent to EasyOCR are 76.8% (mixed lines 100%, forms 100%, receipts 92%, chats 62%, **statements 12%**). Bank-statement pages are mostly Latin with a few Urdu words, and half of them show no Arabic-script letters among their six longest lines. *Decision:* **T_low = 0.10** (twice the highest English/receipt share seen, so unseen photographs have room before an English page is misrouted). *Change to R5:* the English, receipt and Urdu-page routing conditions stay (English and receipts at least 98% on the Latin route, Urdu pages at least 98% to EasyOCR); the condition on mixed images is **removed as a gate and reported per kind**. Reason it is not hidden: on the sealed bake-off rows the PP-OCRv5 route reads mixed statements as well as or better than EasyOCR (identifiers 100% against 99.3%, character error 10.4% against 26.9%), so sending them to EasyOCR is not a goal in itself; R3 (identifiers and character error of the whole mixed set against today's router) remains the test of whether the routing choice is good. The rule the plan stated before the measurement (at least 90% of mixed pages to EasyOCR) is recorded as **not met**.
