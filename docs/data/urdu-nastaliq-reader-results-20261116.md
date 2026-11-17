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

## 2. The VLM reader (Q): a speed check on 3 development rows, CPU laptop, images capped (amendments A3 to A5)

Run on this laptop on 2026-11-17 inside the existing stack's memory: the language model was unloaded from the model container (the container kept running; it reloads on the next Ask), and the reader ran in a detached container capped at 6,000 MB without swap and without network, under a watchdog that kills only that container (host available memory below 600 MB, or 45 minutes). Two earlier attempts at 5,200 and 6,000 MB were killed for memory (`oom_killed=true`) before the cause was found: the real images are large (median 0.38 megapixels, 90th percentile 1.0 MP, maximum 3.1 MP) and the vision part of the model attends over all patches of an image, so its memory grows with the square of the pixel count; the images were then capped at 401,408 pixels (`--max-pixels`, amendment A5). **No sealed row was read by Q or B.**

| Development row | Pixels / resized | Reader Q (capped) character error | Reader T (live route) character error | Characters returned (reference) | Seconds |
|---|---|---|---|---|---|
| 286 | 645,120 / yes | 39.1% | 72.4% | 473 (550) | 207 |
| 671 | larger than the cap / yes | 42.8% | 65.0% | 584 (697) | 239 |
| 245 | 264,192 / no | 27.0% | 76.7% | 244 (270) | 103 |
| **Pooled, 3 rows** | | **38.6% [27.0, 42.8]**, word error 77.7%, no loops | **69.7%** | | median 207, max 239; peak 6.05 GB |

**What it says (3 development rows are a speed check, not a measurement).** On the same three pages the vision reader makes about 31 points fewer character errors than today's route and returns almost all of the text (today's route returns half). The page that needed no resizing scored 27.0%, close to the 25% pass mark; the two resized pages scored 39% and 43%, which suggests that lowering the resolution costs accuracy on small Nastaliq text, and that the full-size run on a GPU host may do better (not measured). Against the gates, as far as three rows can say: N1 (25%) is not met at this resolution; N2 (10 points better than T) is met by a wide margin; N3 (loops) none; N4 (deployable) is **not met on this machine**: 6 GB of memory and 3.5 minutes a page cannot share the machine with the language model. The sealed rows were not run here: the formal capped result would be 2 hours of the language model's absence to confirm "better than today, short of 25%, not deployable on this laptop"; the sealed run belongs on a GPU host at full size.

## 3. What this means now

* The Urdu route reads Nastaliq newspaper print at about 70% character error. It is labelled low confidence and review required, which is correct, and it should not be relied on to find words in such pages.
* A vision-language reader brings the Nastaliq reading from about 70% to about 39% character error on the three development pages tried, with images capped; the full-size result is still to be measured on a GPU host. Nothing in the live system changed because of this measurement: the language model was unloaded for about 10 minutes and reloaded by the next Ask.
