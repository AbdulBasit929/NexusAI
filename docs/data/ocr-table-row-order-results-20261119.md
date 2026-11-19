# Left-to-right table rows inside the EasyOCR role: results (2026-11-19)

Plan and gates: `ocr-table-row-order-preregistration-20261119.md` (amendment A1 before any new-seed image was read, A2 for the timing re-measurement below). The problem: a bank-statement row with Urdu narration came back with its columns reversed when EasyOCR read it (`ocr-latin-reread-results-20261117.md` section 4, confirmed on the acceptance readings). Scorer: `tools/media_data/accept_rows_score.py`; per-image output in `results/ocr/accept3-*.jsonl` (outside the repository).

## 1. What was changed

A line of at least three boxes is read left to right when its leftmost and rightmost boxes contain no Arabic-script character and at least one box between them does (a date, an Urdu narration, amounts); every other line keeps the right-to-left-by-letter-share rule. The line payload records `table_row`, the result metadata counts such rows, and the switch is `FORENSIC_EASYOCR_TABLE_ROWS` (default false). It reorders boxes only: no model is run again.

## 2. Development rows (chose nothing; confirmed the rule)

144 development images forced through EasyOCR with the Latin re-read: **statements 26.2% to 5.6% character error, word error 35.5% to 12.5%** (64 rows read in table order); forms, chats, receipts, short mixed lines (not cut by the canvas) and Urdu pages identical in every measure; identifiers equal. All three conditions of the plan held.

## 3. Fresh acceptance (seeds 20261207 to 20261209, generated with the fixed mixed-line canvas; image `easyocr-rows-20261119`)

| Part | Set | Arm | Images | Mean character error | Identifiers | Errors or empty | Seconds, median |
|---|---|---|---|---|---|---|---|
| routed | mixed | re-read | 120 | 14.8% | 738/748 = 98.7% | 0 | 4.9 |
| routed | mixed | re-read + rows | 120 | **14.5%** | 738/748 = 98.7% | 0 | 5.3 |
| routed | Urdu pages | re-read | 24 | 7.7% | 360/360 = 100% | 0 | 20.1 |
| routed | Urdu pages | re-read + rows | 24 | 7.7% | 360/360 = 100% | 0 | 21.5 |
| forced EasyOCR | statements | re-read | 96 | 25.5% | 2061/2064 = 99.9% | 0 | 12.0 |
| forced EasyOCR | statements | re-read + rows | 96 | **1.7%** | 2061/2064 = 99.9% | 0 | 12.2 |

## 4. Gates

| Gate | Requirement | Result | Verdict |
|---|---|---|---|
| T1 no loss, routed | error at most 0.5 point above `reread` on both sets, identifiers not lower, no failures | mixed 14.5% against 14.8%, Urdu pages 7.7% against 7.7%; identifiers identical; 0 failures | **met** |
| T2 the effect | statements: error at least 10 points lower, identifiers not lower | 25.5% to 1.7% (23.8 points); identifiers equal (99.9%) | **met** |
| T3 time | median per Urdu page within 1 s of `reread` | **first pass 21.45 s against 20.07 s (1.38 s): missed**; quiet re-timing (amendment A2): **21.66 s against 21.87 s (-0.20 s): met** | **met on the re-timing; missed on the first pass** |

**On T3.** The first-pass difference coincided with a dry run of the deploy script (about ten `docker compose` calls and a `git` call) that I ran during that arm's Urdu pages; the rows arm was slower on 19 of 24 pages, by about 1.1 s at the median. The rule itself was timed directly: `assemble_lines` on a 160-box page takes 2.062 ms without it and 2.058 ms with it, i.e. nothing. The 24 Urdu pages were then read again by both arms with nothing else running (`accept3-reread-timing`, `accept3-rows-timing`): 21.87 s and 21.66 s, difference -0.20 s, rows faster on 11 of 24. Both numbers are reported; the verdict rests on the clean measurement because the first was disturbed by the experimenter. (Both runs sit near 21 to 22 s, a little slower than the 17 to 20 s of earlier days: the laptop's state differs between sessions, which is why only same-session pairs are compared.)

## 5. What it shows and what is left

* A statement row with Urdu narration is read in its table order when EasyOCR reads it; routed pages are not harmed. In the routed product only the statements the probe sends to EasyOCR benefit (few: 3 of 20 and 0 of 20 in the earlier acceptances), so the visible product change is small; the rule matters for ledgers with Urdu narration that do reach EasyOCR.
* Not covered: a table whose first or last column is Urdu (the rule needs non-Arabic text at both ends); no column alignment across rows; Urdu line order is still not certified.
* Observed, not acted on: a form row with a Latin value and an Urdu label is ordered value first by the letter-share rule while the truth order is label first (it affects character error on forms, not the words).
* Everything stays a model observation that needs review; the EasyOCR weights licence stays unconfirmed (not a blocker for development, owner's direction of 2026-11-17).
