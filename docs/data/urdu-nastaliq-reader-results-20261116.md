# Urdu Nastaliq reader on real newspaper text: results (2026-11-16)

Plan, rows, readers and gates, fixed before any measurement: `urdu-nastaliq-reader-preregistration-20261116.md` (amendments A1, A2). Tool: `tools/media_data/nastaliq_lab.py`. Per-row outputs (they hold the reader's text of unlicensed newspaper pages) are in `C:/tmp/nexusai-media-data/results/nastaliq`, outside the repository. Evidence tier **T3, public data**: real printed Nastaliq, the Urdu Newspaper Benchmark (ledger DAT-024, no licence stated, private evaluation only).

## 1. The live Urdu route on real Nastaliq print (reader T, measured)

Reader T is what the product does today for an Urdu page: Tesseract 5, `best` data, `urd+eng`, page mode 3, one thread.

| Rows | Character error (pooled, 95% interval) | Mean per row | Word error | Loops | Seconds per row (median, max) | Memory |
|---|---|---|---|---|---|---|
| Development, 12 | 70.4% [67.1, 73.7] | 70.1% | 90.4% | 0 | 1.6, 4.3 | 0.88 GB |
| **Sealed, 60 (read once)** | **68.4% [66.2, 70.5]** | 68.0% | **91.0%** | 0 | 1.0, 3.2 | 0.87 GB |

**What it says.** On a paragraph of real Nastaliq newspaper print, the live route gets about one character in three right and about one word in eleven. The pass mark for Nastaliq is 25% character error (N1); the route is **43 points above it**. The 32% measured on 2026-10-21 was on lines we rendered ourselves with one clean font; real print (ink spread, column rules, small sizes, ligatures, mixed fonts) is far harder. The output is also short: the reader returns on average about half of the characters of the reference (for example 550 reference characters, 237 returned), so most of the failure is text it did not find or cut into pieces it could not join, not only wrong letters. This agrees with the published comparison on a related set (LREC 2026, "From Press to Pixels": Tesseract above 100% character error on Nastaliq newspapers).

**Consequence for the product.** An Urdu page printed in Nastaliq is reported as read with low confidence and review required; that wording is right, but the reading is of little use for search. Naskh print (books, some forms) is a different case and is not measured here (9% on rendered lines; real Naskh print is not in this set).

## 2. The VLM reader (Q) and the control (B)

Not run yet: the base model (Qwen2-VL-2B-Instruct, 4.43 GB) is still downloading. The plan is in the pre-registration: Q on the first 30 sealed rows, then the other 30 if the speed allows; the language-model container is stopped for the run and started again after it.
