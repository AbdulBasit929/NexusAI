# Urdu Nastaliq reader on real newspaper text: plan and gates (2026-11-16, fixed before the first measurement)

Owner direction, 2026-11-16: unlock and improve the listed media features, best and most useful first; "best you think and recommend, go with it". This file fixes the data, the readers, the measures and the gates **before any reader is run on the data**, and is not edited afterwards; changes are dated amendments at the end. Results go in `urdu-nastaliq-reader-results-20261116.md`. Evidence tier **T3, public data** (real printed Nastaliq, not synthetic).

## 1. Why

Urdu printed in Nastaliq (newspapers, notices, forms, court papers) is the style the product reads worst: character error 32% against a pass mark of 25% on lines we rendered ourselves (`media-activation-results-20261021.md`, section D). Rendered lines are kinder than real print. A 2026 study on real Nastaliq newspapers (LREC 2026, "From Press to Pixels") reports open readers at 16% to 25% character error (TrOCR 15.9%, Kraken 22.1%, EasyOCR 24.6%, Tesseract above 100% on that data) and a proprietary model at 3.2%. A small open reader, Qaari-0.1 Urdu (a LoRA adapter on Qwen2-VL-2B), reports 2.9% character error on synthetic pages of five fonts; its card names no evaluation set. This measurement says what each reader does on real print.

## 2. Data (ledger DAT-024; private evaluation only)

Urdu Newspaper Benchmark, Hugging Face `ULRs/Urdu-Newspaper-Benchmark`, revision `ec2a8bf989c1`: 829 rows, each a high-resolution crop of newspaper text (a paragraph of several lines; widths 428 to 1,470 px) with its ground-truth text. **No licence is stated**: the data stays under `C:/tmp/nexusai-media-data`, outside the repository; no image or transcription is copied into the repository, a report or a demo; results carry numbers and error classes only. Partition by hash: `sha256(str(row_index))`, first hex digit in `{0}` = development (about 6%), `{1..f}` = sealed. **Development rows: the first 12 in hash order. Sealed rows: the first 60 in hash order**, read once. The high-resolution image is the input; the low-resolution image is not used.

## 3. Readers

| Id | Reader | Settings (fixed) |
|---|---|---|
| T | **Tesseract 5.x, tessdata `best`, `urd+eng`, page mode 3** (the live Urdu route) | the worker's own settings, one thread |
| Q | **Qwen2-VL-2B-Instruct with the Qaari-0.1 Urdu LoRA merged** (MOD-045, MOD-046) | the model card's prompt, word for word: "Below is the image of one page of a document, as well as some raw textual content that was previously extracted for it. Just return the plain text representation of this document as if you were reading it naturally. Do not hallucinate."; greedy decoding; `max_new_tokens` 2000; image pixels capped at 1,003,520 (the processor's default maximum); bf16 on CPU, 16 threads; the adapter merged by hand (W + (alpha/r) B A, alpha 16, rank 16) into the 3 base weights it names |
| B | Qwen2-VL-2B-Instruct, no adapter (control) | the same prompt and settings, development rows only |

No prompt, size or setting is changed after seeing a result. A reader that cannot start (memory, a missing file) is reported as such.

## 4. Measures

Character error rate and word error rate after the normalisation of `tools/media_data/ocr_eval.py` (NFKC, zero-width characters removed, whitespace removed for the character rate, collapsed for the word rate), in the order the reader produced the text; mean per row and pooled over rows; 95% interval by bootstrap over rows (2,000 draws). Also: rows where the output is longer than twice the reference or repeats an 8-gram more than five times (a loop or invented text); seconds per row; peak resident memory.

## 5. Gates

For **Q to be adopted as a background Nastaliq reader** (all on the sealed rows):

* **N1** character error at most **25%** (the Nastaliq pass mark since 2026-10-21), pooled over rows.
* **N2** at least **10 points** lower than T on the same rows.
* **N3** loops or invented text on at most **5%** of rows.
* **N4** a deployable form: at most **300 s** per row and a resident size that fits next to the language-model container on this machine (at most 2.5 GB), **or** the owner's decision to run it on a GPU host. If N1 to N3 are met and N4 is not, the status is "accurate, not deployable on this machine" and the reader waits for the GPU decision (D43); it is not switched on.

If Q meets N1 and N2 but not N3: not adopted. If T already meets N1 on real print, nothing changes in the live route and the finding is recorded.

## 6. Rules

* Sealed rows are read once per reader. T runs on all 60 sealed rows (and, as information only, on all 829 rows). **Q runs on the first 30 sealed rows (hash order) first, the same rows scored for T**; if the speed allows (at most 300 s per row) it continues to the other 30. A partial run is reported as partial; its interval says how little 30 rows can tell.
* The language-model container (4.2 GB) cannot share the Docker VM (7.6 GB) with a bf16 2B model. It is **stopped for the run of Q and B and started again afterwards**; the start is checked with one Ask question and recorded. Nothing else changes on the live system. The measurement runs in a throw-away container built on the worker image (`Dockerfile.eval-nastaliq`: the worker image plus `pyarrow` from PyPI to read the dataset file); no product image changes.
* Misses are listed with their error classes (dropped line, wrong letter shape, reading order, digits, loop) read from the outputs, not from the key alone.
* A result below the gates is reported as below.

## Amendments

**A1, 2026-11-16, before the first measurement.** The measurement runs in the existing evaluation image `nexusai-media-eval:latest` (the worker image plus `pyarrow`, `torch` 2.6.0 CPU, `transformers` 4.49.0, built for the earlier media measurements, `tools/media_data/Dockerfile.eval`), not in a new `Dockerfile.eval-nastaliq`: no new image and no new package. It runs with `--network none` and the data mounted read-only; results are written to `C:/tmp/nexusai-media-data/results/nastaliq` (outside the repository).

**A2, 2026-11-16, before the first measurement.** Two corrections of wording, no change of method. (a) Image size: the processor's default maximum is 12,845,056 pixels (not 1,003,520, which I wrote in error); every row is far below it, so no row is resized. (b) A "loop or invented text" row is one whose output is longer than twice the reference (characters without spaces) or that repeats a word 8-gram more than five times (`nastaliq_lab.py`, function `looped`). The lab script is `tools/media_data/nastaliq_lab.py`; rows are taken in hash order as in section 2 (development: the first 12 whose hash starts with 0; sealed: the first 60 whose hash starts with anything else).
