# OCR for English, Urdu and mixed text: what exists, what it is worth to us, and what to measure (2026-11-17)

Written after reading the primary pages (repositories, model cards, benchmark papers) in November 2026. Every figure below is quoted from the page named beside it; a figure that came through a summary of a long page, and not from the verbatim text, is marked **(summary)** and must be checked against the paper before it is relied on. Nothing here has been run by us unless it says "measured here".

## 1. What the product has to read

| Purpose | Typical input | What counts as good |
|---|---|---|
| P1 English print | pages, reports, forms | identifiers exact (CNIC, phone, IBAN, plate, dates) and low character error |
| P2 Urdu Naskh print | books, many forms and statements | character error at most 15% |
| P3 Urdu Nastaliq print | newspapers, notices, court and police papers | character error at most 25% (the pass mark since 2026-10-21) |
| P4 **Mixed Urdu and English on one page or one line** | forms, ID cards, receipts, bank statements, chat screenshots | both scripts read, digits and identifiers exact, reading order right in a right-to-left line that holds Latin words and numbers |
| P5 Text in photos | signs, number-plate surroundings, shopfronts, screenshots | words found, even if the line is imperfect |
| P6 Tables and layout | statements, registers | structure kept |
| P7 Handwriting | notes, prescriptions | out of scope for now (open models are far behind commercial ones, see 4.3) |

Rules that cut the field: evidence stays on the premises (no hosted OCR service sees a case file); licences must allow use in a product; the only machine today is a 16 GB CPU laptop that also runs the stack (a model of more than about 2 GB is not safe beside it, see the notes of 2026-11-17).

## 2. Where we stand (measured here)

| What | Result | Source |
|---|---|---|
| English printed pages, PaddleOCR PP-OCRv5 (ONNX) | character error 0.0%, identifiers exact 99.7% to 99.8% | `docs/data/media-activation-results-20261021.md` |
| Photographed receipts (CORD v2, 72 images), PaddleOCR PP-OCRv5 | word F1 78.7% to 79.9% (Tesseract 23.8%) | same |
| Urdu Naskh lines we rendered, Tesseract `best` | character error 9% | same |
| Urdu Nastaliq lines we rendered, Tesseract `best` | 32% (fails 25%) | same |
| Urdu PaddleOCR Arabic recogniser, on our Urdu pages | 91% to 94% character error: not usable | same |
| **Real Nastaliq newspaper paragraphs (829, Urdu Newspaper Benchmark), Tesseract `best` urd+eng page mode 3: the live route** | **70.1% character error, 91.4% word error; no page under 25%** | `docs/data/urdu-nastaliq-reader-results-20261116.md` |
| Same pages, Qwen2-VL-2B + Qaari adapter, images capped, CPU (3 development rows) | 38.6% character error (27.0% on the one page that needed no resizing), 3.5 minutes a page, 6 GB | same |

## 3. The candidates

Licence column: what the page says, not legal advice. "CPU" is whether it can share this laptop with the stack.

| Candidate | Licence (as stated) | Size | Urdu evidence | CPU | Verdict |
|---|---|---|---|---|---|
| Tesseract 5 `best` (live) | Apache-2.0 | 20 MB per language | measured here: Naskh fine, Nastaliq 70% on real print | yes | keep for English and Naskh; not for Nastaliq |
| PaddleOCR PP-OCRv5 (ONNX, live) | Apache-2.0 | recognisers 7 to 16 MB | `arabic_PP-OCRv5_mobile_rec` (81.27% average accuracy on its own Arabic set, 7.6 MB): the docs do not say Urdu-specific letters are covered; measured here: unusable for Urdu | yes | keep for Latin text; not for Urdu |
| **EasyOCR 1.7.2** (JaidedAI) | **Apache-2.0** (LICENSE file) | Arabic-script recogniser + CRAFT detector | `ur` is a supported language and "English can be combined with any language" ([README](https://github.com/JaidedAI/EasyOCR), [language list](https://www.jaided.ai/easyocr/)). **UrduLens** ([repo](https://github.com/abdullahbinmasood702/urdulens), synthetic lines, single core): CER 13.6% (Nastaliq 20.9%, Naskh 8.7%) against Tesseract 39.2% (49.5%, 32.3%), 242 ms a line. **LREC 2026** ([paper](https://arxiv.org/html/2505.13943v3), real newspapers, UNB): CER 24.6%, WER 80.2% (Tesseract above 100%) | **yes** | **measure first: cheap, deployable, covers `ur`+`en` in one reader** |
| TrOCR | MIT (code; the paper's fine-tuned weights are on Google Drive with no stated licence) | 334M parameters | LREC 2026: best open reader on UNB, CER 15.9%, WER 42.2%; on OpenITI-Nastaliq CER 17.7% | borderline | needs line detection first; we would have to fine-tune our own (synthetic Urdu from CC0 text and OFL fonts, public-data cloud training is already approved) |
| Kraken | Apache-2.0 | small | LREC 2026: UNB CER 22.1%; a ready Urdu model is not named on the page (zenodo repository not read) | yes | research: is there a licensed Urdu printed model |
| UTRNet (printed Urdu) | **CC BY-NC-SA 4.0 (GitHub) / cc-by-nc-4.0 (Hugging Face): non-commercial** | small | LREC 2026: UNB CER 30.6% | yes | **blocked for the product**; academic reference only. UrduDoc only by a no-cost agreement with the authors |
| Qwen2-VL-2B + Qaari-0.1 Urdu adapter | Apache-2.0 base; adapter "subject to the licence of the base" | 4.43 GB | measured here: 38.6% on three real pages, capped | no (6 GB) | GPU host, full size |
| Qwen3-VL-2B / 4B | Apache-2.0 | about 4 / 8 GB | "Supports 32 languages (up from 19)"; Urdu not named. MORE (Tencent authors): Urdu overall 42.15 for the 2B **(summary)** | no | GPU host |
| PaddleOCR-VL 0.9B and 1.6 | Apache-2.0 | 1.0B parameters | "109 languages"; Urdu not named. MORE: Urdu overall 61.19 **(summary)**; GlotOCR: Arabic accuracy at 5 characters 21.0 for v1.5 **(summary)** | maybe (about 2 GB) | candidate for a CPU test after EasyOCR |
| DeepSeek-OCR | MIT (card metadata) | 3B total, 570M active | MORE: Urdu overall 70.54, the best of the open models there **(summary)**; GlotOCR: DeepSeek-OCR-2 weak on Arabic script | no (about 6 GB of weights) | GPU host |
| dots.ocr | MIT, **plus a second "LICENSE AGREEMENT" (acceptable-use clauses, Chinese law, "Built with" for fine-tuned weights); the MIT text prevails where they conflict** | 3B | GlotOCR: strongest open model on the nine mid-resource scripts that include Arabic (CER 5.0%), Arabic accuracy at 5 characters 63.2% **(summary)**; Urdu not evaluated; MORE Urdu 53.80 **(summary)** | no | GPU host, after counsel reads the second agreement |
| HunyuanOCR | **Tencent Hunyuan Community Licence: does not apply in the EU, UK and South Korea; acceptable-use limits; outputs may not improve other models** | 1B | tops MORE (the authors work at Tencent) | no (CUDA) | **licence-blocked for us** |
| Surya | code Apache-2.0 or GPL depending on the source; **weights under a modified Open RAIL-M with revenue limits** | 90+ languages | not confirmed | yes | avoid |
| Granite-Docling 258M | Apache-2.0 (IBM statements) | 258M | Arabic "experimental" | yes | layout and tables only; the Hugging Face page could not be read, so unverified |
| Gemini, GPT, Claude, Mistral OCR | proprietary | - | best by far (Gemini 2.5 Pro UNB CER 3.2%) | - | **excluded: evidence may not leave the premises**. Useful only as a ceiling |

## 4. What the evidence says

1. **There is no open consensus winner for Urdu.** The strongest numbers for open models come from different papers on different data, and the only one that scores Urdu per language (MORE, authors from Tencent) reached us through a summary. GlotOCR Bench does not evaluate Urdu. METATR does not cover Urdu.
2. **Nastaliq is harder than Naskh everywhere** (UrduLens, LREC 2026, our own measurements). Real print is harder than rendered print: the live route went from 32% on our lines to 70% on real newspapers.
3. **Open vision-language models are weakest where we are weakest.** Handwritten Nastaliq: commercial models are ahead of open ones ([arXiv 2611.01635](https://arxiv.org/abs/2611.01635)); GlotOCR: "Arabic shows the most severe degradation" of the scripts it analysed, "high ScriptAcc yet lags in Acc@5" **(summary)**.
4. **Nothing public tests English mixed into Urdu.** The nearest are an Urdu and English scene-text set ([PMC12390122](https://pmc.ncbi.nlm.nih.gov/articles/PMC12390122)), handwritten bilingual prescriptions, and a low-resource-scripts benchmark that scores GPT-4o only ([arXiv 2412.16119](https://arxiv.org/abs/2412.16119): Urdu word error 0.20 to 0.35). A 2026 library-OCR paper says mixed-script pages need an image-based script identifier before recognition. **The mixed-script benchmark has to be built by us.**
5. **The small, classical recognisers are not dead**: EasyOCR and Kraken, both CPU-friendly and permissively licensed, beat Tesseract by a wide margin on real Nastaliq in LREC 2026, and TrOCR (fine-tuned) is the best open reader there. A 3B vision-language model is not automatically better.

## 5. What to build and measure, in order

**Step 1 (CPU-safe, this laptop, no new risk): OCR bake-off v2, pre-registered.** Readers: Tesseract (variants), EasyOCR `['ur','en']` and `['en']`, PP-OCRv5 Latin, plus a script-routing wrapper (a page or a line goes to the reader that suits its script). Sets, each with a hash-fixed development and sealed split:
* P1: our existing English pages (34 x 4 degradations) and CORD receipts;
* P2: Urdu Naskh lines and pages (rendered by us; UrduLens' CC0 sentences and OFL fonts are reusable);
* P3: the Urdu Newspaper Benchmark (real, 60 sealed rows already fixed);
* **P4: a new mixed Urdu and English set**, rendered from CC0 sentences and OFL fonts: forms with Urdu and English fields, bank-statement rows, receipts, ID-card layouts with CNIC and phone numbers, chat screenshots, and single lines that mix both scripts. Metrics: character and word error, **exactness of identifiers and numbers**, and a reading-order score for mixed lines.
Gates per purpose are written before the run; a sealed row is read once per reader.

**Step 2 (CPU-safe): a PaddleOCR-VL 0.9B test** with the image-size cap, in the same memory-capped, watchdog-guarded container used on 2026-11-17, after Step 1.

**Step 3 (GPU host, when one exists): the vision-language readers at full size**: Qaari/Qwen2-VL, Qwen3-VL-4B, PaddleOCR-VL, DeepSeek-OCR, dots.ocr (after counsel reads its second agreement), on the same sets and gates.

**Step 4 (public-data cloud training, already approved in principle): fine-tune a line recogniser** (TrOCR or EasyOCR's recogniser) on rendered Urdu and mixed lines, then test it on the real sets. Nothing real is ever uploaded.

**Step 5: script routing and layout** in the product (a page-level script identifier, then the right reader; Granite-Docling or a table reader for tables) once a reader passes its gate.

## 6. Decisions and downloads this needs

* Download, public files only, outside the repository, ledger rows first: the `easyocr` 1.7.2 package from PyPI (Apache-2.0, 2.9 MB; its other requirements are `torchvision`, `scikit-image`, `python-bidi`, `Shapely`, `pyclipper`, `scipy`, `PyYAML`, `ninja`, about 30 to 40 MB; installed into a throw-away copy of the evaluation container, never into the product image) and three model files from the JaidedAI GitHub releases: `craft_mlt_25k.zip` (the text detector, 77.3 MB), `arabic.zip` (the Arabic-script recogniser that serves `ur`, 199.9 MB) and `english_g2.zip` (14.0 MB). About 330 MB in all.
* Counsel to read: the second agreement in the dots.ocr repository; the HunyuanOCR licence is already disqualifying.
* A licence from the Urdu Newspaper Benchmark authors, or another openly licensed set of real Nastaliq pages, would let the real set be used on a rented GPU.

## Sources

[From Press to Pixels (LREC 2026)](https://arxiv.org/html/2505.13943v3) · [UrduLens](https://github.com/abdullahbinmasood702/urdulens) · [EasyOCR](https://github.com/JaidedAI/EasyOCR) and [language list](https://www.jaided.ai/easyocr/) · [Kraken](https://github.com/mittagessen/kraken) · [UTRNet](https://github.com/abdur75648/UTRNet-High-Resolution-Urdu-Text-Recognition) · [GlotOCR Bench](https://arxiv.org/abs/2604.12978) · [MORE](https://arxiv.org/abs/2607.02956) · [METATR](https://arxiv.org/abs/2605.26712) · [Benchmarking LLM OCR for low-resource scripts](https://arxiv.org/abs/2412.16119) · [PaddleOCR](https://github.com/PaddlePaddle/PaddleOCR) and its [multilingual OCR docs](https://www.paddleocr.ai/latest/en/version3.x/pipeline_usage/OCR.html) · [PaddleOCR-VL](https://huggingface.co/PaddlePaddle/PaddleOCR-VL) and [1.6](https://huggingface.co/PaddlePaddle/PaddleOCR-VL-1.6) · [dots.ocr](https://github.com/rednote-hilab/dots.ocr) · [DeepSeek-OCR](https://huggingface.co/deepseek-ai/DeepSeek-OCR) · [HunyuanOCR](https://huggingface.co/tencent/HunyuanOCR) · [Qwen3-VL-2B](https://huggingface.co/Qwen/Qwen3-VL-2B-Instruct) and [4B](https://huggingface.co/Qwen/Qwen3-VL-4B-Instruct) · [Qaari Urdu](https://huggingface.co/oddadmix/Qaari-0.1-Urdu-OCR-VL-2B-Instruct) · [Surya 2](https://huggingface.co/datalab-to/surya-ocr-2) · [Granite-Docling (IBM)](https://ibm.com/new/announcements/granite-docling-end-to-end-document-conversion) · [Urdu Newspaper Benchmark](https://huggingface.co/datasets/ULRs/Urdu-Newspaper-Benchmark).
