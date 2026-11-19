# Left-to-right table rows inside the EasyOCR role (plan and gates, fixed before any measurement of the fix; 2026-11-19)

## 1. The problem

`assemble_lines` (the rule the bake-off measured, amendment A1) reads a line right to left when more than half of its letters are Arabic-script. A row of a **left-to-right table that holds Urdu text** (a bank-statement row: date, Urdu narration, amount, balance) has Urdu letters in the majority (digits are not letters), so the whole row is ordered right to left and **comes back reversed**: `151,658 53,682 خریداری 07/08/2025` for `07/08/2025 خریداری 53,682 151,658`. The words and numbers are right (word F1 96.6% on the three statements that reached EasyOCR in the first acceptance) but the order is not, so character error is inflated (40.7% against 34.3% for the router without EasyOCR) and a reader sees a balance before its date. In the routed product this reaches only the statements the probe sends to EasyOCR (3 of 20 in the first acceptance, 0 of 20 in the second; the others stay on PP-OCRv5, character error 11%), but any ledger with Urdu narration and a high Arabic share goes the same way.

## 2. The change

A line of at least **three boxes** is read **left to right** when its **leftmost and its rightmost box contain no Arabic-script character** (a date, a number, Latin text) **and at least one box between them does**. Every other line keeps the current rule (right to left when most letters are Arabic-script). Reasoning: in a right-to-left sentence the logical start is on the right and is Urdu (or the sentence ends with a number at the LEFT end, which leaves the right end Urdu), so a line with non-Arabic boxes at BOTH ends and Urdu between them is a table row, not a sentence. Switch `FORENSIC_EASYOCR_TABLE_ROWS` (default false until the acceptance below passes); the line payload records `table_row: true` where the rule fired.

## 3. Development rows (to confirm the rule and nothing else)

Development partitions of the generated mixed set (statement 24, form 24, chat 24, receipt 24, mixline 24 of the images not cut by the canvas; hash order) and of the Urdu pages (24), every image **forced through EasyOCR with the Latin re-read** (the router is bypassed so that statements are read by EasyOCR too). EasyOCR runs once per image; the two orderings are computed from the same boxes. Measures: mean character error, bag-of-words F1, word error rate, identifiers. The rule is confirmed if **(a)** the statements' mean character error falls by at least **10 points**, **(b)** no other kind's mean character error rises by more than **0.5 point**, **(c)** identifier exactness is not lower on any kind. Otherwise it is not built into the default and the result is reported.

## 4. Fresh acceptance (to decide whether it ships)

New seeds **20261207** (English), **20261208** (Urdu), **20261209** (mixed), generated with the fixed generator (the mixed-line canvas grows to fit; amendment of 2026-11-19 to the bake-off plan). Two parts through the worker overlay image, the Latin re-read on in both arms:

* **Routed (no loss):** `ur_pages` (40) and `mixed` (120) through the router, arms `reread` and `reread+rows`.
* **Forced statements (the effect):** every statement image of the mixed set (in hash order) read directly by EasyOCR with the Latin re-read, arms `reread` and `reread+rows`.

Gates (all must hold):

* **T1 no loss, routed:** mean character error of `reread+rows` at most **0.5 point** above `reread` on both sets, identifiers not lower on either set, no errors or empty readings.
* **T2 the effect, forced statements:** mean character error at least **10 points** lower, and identifiers not lower.
* **T3 time:** median seconds per Urdu page within **1 s** of the `reread` arm (the rule only reorders boxes).

## 5. What a pass does and does not mean

A pass means a statement row with Urdu narration is read in its table order when EasyOCR reads it. It does not detect tables in general (no column alignment across rows), it does not cover a table whose first or last column is Urdu, and it does not make Urdu line order certified; every reading stays a model observation that needs review.

## Amendments

**A1, 2026-11-19, after the development-row confirmation and before any image of seeds 20261207 to 20261209 was read.** *Result* (`tools/media_data/measure_row_order.py`; 144 development images forced through EasyOCR with the Latin re-read, identical detections for both orderings; `results/ocr/row-order-dev-20261119.json`):

| kind | images | rule | character error | word error | identifiers | lines read as table rows |
|---|---|---|---|---|---|---|
| statement | 24 | current | 26.2% | 35.5% | 566/567 | 0 |
| statement | 24 | **rows** | **5.6%** | **12.5%** | 566/567 | 64 |
| form, chat, receipt, short mixed line (not cut by the canvas), Urdu page | 24 each | current and rows | identical (13.4%, 11.0%, 14.1%, 14.8%, 7.9%) | identical | identical | 0 |

*Decision:* the three conditions of section 3 hold (statements 20.6 points better; no other kind changed; identifiers not lower), so the rule goes into the acceptance of section 4 as written. Two things the table also shows: bag-of-words F1 of statements is the same (91.7%) for both orderings, i.e. the words were always right and only their order was wrong; and the short mixed lines that are not cut by the canvas read all 16 planted phones (see the erratum of the bake-off results). *Wiring for section 4:* the sets of seeds 20261207 to 20261209 are generated with the fixed generator (folder suffix `_accept3`); routed arms `reread` and `rows` on `ur_pages` (24) and `mixed` (120); forced-statement arms on every statement image of the mixed set (96); image `easyocr-rows-20261119`; scorer `tools/media_data/accept_rows_score.py`. *Observation outside the scope of this rule, not acted on:* a form row with a Latin value and an Urdu label (`Ahmed Raza` and `نام`) is ordered by the letter share (Latin majority, so left to right: value first) although the truth order is label first; the table-row rule does not touch it.

**A2, 2026-11-19, after the first scoring of the acceptance and before any decision to deploy.** *What happened:* gates T1 and T2 were met; T3 was missed on the first pass (median per Urdu page 21.45 s against 20.07 s, a difference of 1.38 s against the bound of 1 s). The rule only reorders boxes (timed directly: 2.062 ms against 2.058 ms for a 160-box page) and the slowdown coincided with a dry run of the deploy script that I ran on the same machine during that arm's Urdu pages. *Change:* a timing-only re-measurement under quiet conditions, the 24 Urdu pages of the acceptance read again by both arms one after the other with nothing else running (`results/ocr/accept3-rows-timing.jsonl`, `accept3-reread-timing.jsonl`); no label is scored again and nothing else is re-read. *Result:* 21.87 s (`reread`) and 21.66 s (`rows`), difference -0.20 s: T3 met on the re-measurement. Both measurements are reported in `ocr-table-row-order-results-20261119.md`; the first is not withdrawn.
