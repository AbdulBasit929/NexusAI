# OCR bake-off v2: results (2026-11-17)

Plan and gates: `ocr-bakeoff-v2-preregistration-20261117.md` (fixed before any sealed row was read; two dated amendments A1, A2). Research: `docs/research/OCR_MULTILINGUAL_RESEARCH_20261117.md`. Scorer: `tools/media_data/score_ocr_bakeoff_v2.py` (reads only the per-row numbers the runs wrote). Every sealed row was read once per reader. All sets are synthetic or public (tiers T1/T3); no case data.

## 1. The answer in five lines

* **EasyOCR 1.7.2 (`ur`+`en`) is the best Urdu reader we can run on this CPU**: on real newspaper Nastaliq print ****21.5%** pooled [20.2%, 23.0%]** character error against **68.4% [66.3%, 70.6%]** for the Tesseract route in production; on generated Urdu pages 7.6% against 40.0%; on mixed Urdu-and-English pages 20.3% against 40.6%.
* **It is not a reader for English**: identifiers on English pages 74.4% (`ur`+`en`) and 89.7% (`en` only) against 99.5% for the PP-OCRv5 route that reads English today. English stays on PP-OCRv5; EasyOCR is an engine of a **routed** role, behind the existing script probe.
* **Mixed pages: identifiers pass (94.5%), word F1 does not (69.6% against 85%)**. Phones are the weak identifier type for every reader (EasyOCR 78%, Tesseract 69%, PP-OCRv5 57%); the shortest mixed lines lose their phone number in all three readers (4 of 20).
* **Speed misses the gate on one thread**: an A4 page takes 38 to 49 s (median) against the 30 s gate. Threads and an image-size cap are measured separately (section 6).
* **Licence of the weights is open** (ledger MOD-047 to MOD-049): even a full pass is "accurate, licence open"; the role stays off until that is answered or our own recogniser replaces the files.

## 2. What was read

| Set | Content | Rows read (sealed) | Readers |
|---|---|---|---|
| S4 mixed Urdu and English | forms, statements, receipts, chats, single lines; 4 degradations | 144 | T, P, E |
| S2a Urdu lines | Naskh 53, Nastaliq 56 | 109 | T, E |
| S2b Urdu pages | Naskh and Nastaliq pages with a Latin table | 32 | T, E |
| S1 English pages | A4 pages, planted identifiers, 4 degradations | 56 | P, E, E-en |
| S3 real Nastaliq print | newspaper paragraphs (DAT-024, private evaluation) | 60 | T, E |
| S5 receipts (CORD) | **not run**: three runs were killed by the container memory cap (amendment A2); not a gate | 0 | - |

T = Tesseract 5 `best` `urd+eng` (the live Urdu route), P = PaddleOCR PP-OCRv5 ONNX (the live Latin route), E = EasyOCR `ur`+`en`, E-en = EasyOCR `en`. One CPU thread, no network.

What the per-row files hold: character error per row (so the generated sets show the **mean of per-row rates**, with a 95% bootstrap interval over rows; the newspaper set has edit counts and is **pooled**), word F1 (order free), identifiers found verbatim, seconds. **The word error rate was not stored, so the plan's "order gap" (F1 minus 1 minus WER) was not computed**; character error beside F1 is shown instead. Peak memory was not recorded in the sealed runs (the container cap was 3 GB; none of the completed runs hit it); it is measured in section 6.

## 3. Results side by side

| set | reader | rows | CER (mean per row, 95% CI) | median CER | F1 | identifiers | empty/runaway | s/row median (max) |
|---|---|---|---|---|---|---|---|---|
| mixed | T | 144 | 40.6% [36.0%, 45.9%] | 43.8% | 61.4% | 863/940 = 91.8% | 1 (0.7%) | 0.7 (1.3) |
| mixed | P | 144 | 49.4% [45.0%, 53.8%] | 51.1% | 53.0% | 872/940 = 92.8% | 0 (0.0%) | 0.4 (1.1) |
| mixed | E | 144 | 20.3% [18.2%, 22.7%] | 18.1% | 69.6% | 888/940 = 94.5% | 0 (0.0%) | 11.9 (25.4) |
| ur_lines_naskh | T | 53 | 21.7% [7.9%, 40.4%] | 0.0% | 82.8% | - | 7 (13.2%) | 0.6 (0.7) |
| ur_lines_naskh | E | 53 | 8.7% [7.6%, 10.0%] | 7.5% | 62.7% | - | 0 (0.0%) | 2.8 (5.7) |
| ur_lines_nastaliq | T | 56 | 50.3% [40.9%, 58.8%] | 48.8% | 29.9% | - | 12 (21.4%) | 0.6 (0.6) |
| ur_lines_nastaliq | E | 56 | 21.2% [18.9%, 23.7%] | 19.6% | 31.5% | - | 0 (0.0%) | 1.9 (4.1) |
| ur_pages | T | 32 | 40.0% [20.4%, 68.3%] | 23.7% | 69.7% | 442/480 = 92.1% | 2 (6.2%) | 1.5 (2.0) |
| ur_pages | E | 32 | 7.6% [6.5%, 8.9%] | 5.9% | 64.3% | 431/480 = 89.8% | 0 (0.0%) | 37.8 (48.1) |
| en_pages | P | 56 | 0.0% [0.0%, 0.0%] | 0.0% | 100.0% | 1752/1760 = 99.5% | 0 (0.0%) | 1.5 (2.1) |
| en_pages | E | 56 | 2.4% [2.2%, 2.7%] | 2.2% | 88.2% | 1310/1760 = 74.4% | 0 (0.0%) | 48.9 (57.4) |
| en_pages | Een | 56 | 0.7% [0.6%, 0.8%] | 0.6% | 95.2% | 1578/1760 = 89.7% | 0 (0.0%) | 20.1 (31.6) |
| cord | P | not run | | | | | | |
| cord | Een | not run | | | | | | |
| unb | T | 60 | 68.4% pooled [66.3%, 70.6%] | | | | loops 0, empty 0 | 1.0 (3.2) |
| unb | E | 60 | 21.5% pooled [20.2%, 23.0%] | | | | loops 0, empty 0 | 18.2 (105.4) |

Breakdown by page kind, variant and identifier type: `python tools/media_data/score_ocr_bakeoff_v2.py --breakdown`. The points that matter:

* **Tesseract is better than EasyOCR on the Naskh lines it reads at all** (clean Naskh lines: 9.8% against 6.2% mean, and a median of 0.0% for T over the whole set against 7.5% for E) but for 13% of Naskh lines and 21% of Nastaliq lines it returns an empty or unusable reading (as many errors as characters, or more), so its mean is 21.7% and 50.3%. EasyOCR never returned an empty reading, and its error is flat across the four degradations (Naskh 6.2 to 10.7%, Nastaliq 18.6 to 24.4%). Word F1 of E on lines is lower than T's (62.7% against 82.8% on Naskh) although its character error is lower: EasyOCR splits and joins words at different points; the gate uses character error.
* **Pages: Tesseract fails on clean Naskh pages** (87.4% character error on the 5 clean Naskh pages, 9% on the noisy ones); EasyOCR is steady at 4.7 to 5.8% (Naskh) and 11.6 to 11.9% (Nastaliq).
* **Mixed set by kind (E)**: statements 26.9% character error but F1 92.1% and identifiers 99.3% (the known order effect: a row of a left-to-right table that holds Urdu text comes back with its columns reversed, the words are right); forms 14.4% / 93.8%; chats 14.9% / 92.2%; receipts 17.9% / 89.2%; **single mixed lines 28.5% and identifiers 4 of 20** (4 of 20 for E and for T, 0 of 20 for P).
* **English pages (E, E-en)** lose identifiers mostly on the low-resolution JPEG variant (59.8% for E, 89.3% for E-en, 99.5% for P).

## 4. Gates (pre-registered)

| Gate | Requirement | Result | Verdict |
|---|---|---|---|
| U1 real Nastaliq print | pooled character error at most 25% and at least 10 points below T | E **21.5%** pooled [20.2%, 23.0%] against T 68.4% [66.3%, 70.6%] on 60 rows | **met** |
| U2 rendered Urdu | line character error at most 15% (Naskh) and 25% (Nastaliq), and at most T's | Naskh 8.7% (T 21.7%); Nastaliq 21.2% (T 50.3%) | **met** |
| U3 mixed | identifiers at least 90% and word F1 at least 85% | identifiers 94.5%; F1 69.6% | **not met** (F1) |
| U4 no loss on English | identifiers within 2 points of P (99.5% sealed) | E 74.4%, E-en 89.7% | **not met** |
| U5 deployable | at most 30 s per A4 page and 1.5 GB on one CPU thread | median 48.9 s (English page) and 37.8 s (Urdu page); memory not recorded in these runs | **time not met** at one thread; memory and threads in section 6 |
| U6 clean failure | empty or runaway outputs on at most 5% of rows | 0% on every set; newspaper loops/empty 0 loops, 0 empty of 60 rows | **met** |

Outcome by the plan's own rule: **U2 met and U3 missed -> EasyOCR reads Urdu, and the mixed case needs routing by script (a per-line script split) and a second plan.** What is decided from this: EasyOCR becomes the Urdu and mixed-text engine of the routed OCR role (English pages stay on PP-OCRv5; Tesseract `urd+eng` stays as the fallback), and the weak spots below are plan items, not hidden.

## 5. What this does not show

* The mixed set is synthetic: it measures reading order and identifiers under controlled layout, not real photographs. A set of real consented Urdu and mixed documents (decision D26) must repeat it before the role is switched on for a case.
* The newspaper set is public print from one publisher family; handwriting, stamps, ID-card photographs and poor phone photographs are untested.
* **Order**: Urdu text inside a left-to-right table comes back with the columns reversed; Urdu line order and low-quality Nastaliq are not certified, so every reading is a model observation that needs review.
* **Phone numbers** are read least reliably of the identifier types (EasyOCR 109 of 140, i.e. 78%), and the shortest mixed lines lose theirs in every reader. An analyst must check a phone number read from a mixed line against the image.
* The receipt set (S5) was not run, and the logs of the first twelve sealed runs were overwritten by a mistaken invocation of the run script (the result files are intact; the script now keeps a log per run and stops when a container does not start).

## 6. Router threshold, speed and memory (development rows)

Measured on **development rows only**, through the product processor (`EasyOcrProcessor.from_environment()`, pinned hashes, the thread and size settings the worker would use), one process per setting, CPU laptop (i7-1260P, 4 performance + 8 efficiency cores), no network. Files: `results/ocr/router-share-dev-20261117.json`, `easyocr-speed-dev-20261117.json`, `easyocr-large-dev-20261117.json`.

**Router threshold (T_low).** Every English page and receipt shows an Arabic-script share of at most **0.047** and every Urdu page at least **0.50**, so any value from 0.05 to 0.50 separates them perfectly; **0.10** was chosen. The plan's rule "at least 90% of mixed pages to EasyOCR" is **not met** (76.8%: mixed lines and forms 100%, receipts 92%, chats 62%, bank statements 12%); statements are mostly Latin text and the probe often sees no Arabic-script letters among their longest lines, and the PP-OCRv5 route reads them as well or better. Recorded as amendment A1 of the acceptance plan; the whole mixed set is tested there (R3).

**Speed (3 images per type, 150 dpi pages of 1240 x 1754).**

| Threads | A4 Urdu page | Mixed form/statement | Newspaper paragraph | Peak memory |
|---|---|---|---|---|
| 1 | 45.8 s | 15.6 s | 24.0 s | 2.77 GB |
| 2 | 28.6 s | 9.3 s | 13.6 s | 2.78 GB |
| **4** | **20.0 s** | 7.6 s | 9.7 s | 2.81 GB |
| 8 | 28.2 s | 10.1 s | 15.3 s | 2.79 GB |

Character error is identical at every thread count (Urdu page 10.1%, newspaper paragraph 24.6%). Eight threads are slower than four (the extra threads land on efficiency cores). **4 threads** is the setting. Gate U5 time (30 s per A4 page): met at 4 threads (20.0 s per Urdu page).

**Size cap (long side, pixels).** At 4 threads: no cap 20.0 s, peak 2.81 GB; 2400 and 1800 change nothing on these pages (they are only 1754 px long); **1400: 17.9 s, peak 2.25 GB, Urdu page character error 10.3% (against 10.1%), newspaper rows unchanged.** The first cap values did nothing because EasyOCR already limits its detector canvas to 2560 px.

**A 300 dpi scan (2480 x 3508, a development page enlarged 2x).** With **no cap the process was killed by a 3.5 GB memory limit** (the detector's feature maps grow with the pixel count). With the cap at 1800: 20.2 s for an Urdu page and 9.9 s for a mixed form, peak 2.62 GB, character error 10.8% and 9.6%. Memory after loading the models, before any image: **0.86 GB.** Because the reduced image is the same size whatever the scan resolution, the peak with the cap at 1400 is the 2.25 GB measured above.

**Decision.** `FORENSIC_EASYOCR_THREADS=4`, `FORENSIC_EASYOCR_MAX_SIDE=1400`. **U5 memory (1.5 GB) is met at rest (0.86 GB) and not met while a page is being read (about 2.3 GB)**; the worker has no memory limit, the Docker VM (7.6 GB) holds the stack (about 3.1 GB with the model container), so one reading at a time fits with about 2 GB to spare. The role must not run beside other heavy readings (video, speech) without a check, and a worker memory limit is a deployment task, not set here.

## 7. Licence and status

Weights licence unconfirmed (ledger MOD-047 to MOD-049; request drafted, not sent: `docs/compliance/easyocr-weights-licence-request-20261117.md`). The own-recogniser route (train on public and synthetic data, pair with the PP-OCRv5 detector) is the licence-clean path. Nothing is deployed; the code is built, tested and off by default (`docs/roadmap/ocr-easyocr-integration-plan-20261117.md`).
