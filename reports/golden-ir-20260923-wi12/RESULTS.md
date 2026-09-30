# Phase 1 gate — MET. Full 62-question golden run, 2026-09-23

Harness: `nexusai_live_eval.py --analyst-text` (opt-in; anything graded without that flag is
not comparable). Switches: `FORENSIC_IR_FALLBACK` · `FORENSIC_IR_ARBITRATION` ·
`FORENSIC_VERIFIED_ONLY` **true**; `FORENSIC_IR_CROSSCHECK` · `FORENSIC_IR_SHADOW` **false**.

## Result

    CORRECT 43 · CLARIFIED 18 · MANUAL 1 · WRONG 0 · ERROR 0 · PRESENT_NOT_TOP 0

    confident-wrong        5 -> 0
    correct-or-clarified   90% -> 98.4%   (61 of 62; the 1 MANUAL is a qualitative check)

**IMPROVED 5 · REGRESSED 0 · unchanged 57.**

| ID | Before | After | Work item |
|---|---|---|---|
| AUD-02 | WRONG | **CORRECT** | WI-12 |
| CASE-01 | WRONG | CLARIFIED | WI-10 |
| DOC-04 | WRONG | CLARIFIED | WI-10 |
| IMG-04 | WRONG | CLARIFIED | WI-10 |
| X-01 | WRONG | CLARIFIED | WI-10 |

AUD-01 and AUD-03 remained CORRECT but are now **genuinely** correct rather than false
positives: both previously scored CORRECT because the expected filename appeared somewhere in
the response blob while the analyst-facing answer showed nothing. Both now state the
recording, the timestamp and the transcript (`stated=True` in this run, where it was False
before).

## Phase 1 gate — 4 of 4

| Gate | Target | Measured |
|---|---|---|
| HTTP 500 | 0 | **0** across 62 |
| correct-or-clarified | >= 70% | **98.4%** |
| confident-wrong | 0 | **0** |
| answer stated in analyst text | 100% | 43 of 43 CORRECT carry the value in `records` or the answer text |

## What this run measures

Three work items landed between this run and `golden-ir-20260923-final`:

- **WI-10** three guards that convert a confident non-answer into a clarification:
  an answer naming a target the executed plan never filtered on (DOC-04); a page size
  reported as a count (CASE-01, IMG-04); a single-family search asserting case-wide absence
  (X-01). Each was scored offline against the 62 saved responses BEFORE being written, and two
  earlier candidates were rejected on those numbers.
- **WI-11** `clarification.options` — 18 of 62 responses now clarify, and every one carries
  2-4 re-runnable choices drawn from the curated semantic layer. X-01's first option
  (`Which document mentions 03001234567?`) returns the PDF its gold expects.
- **WI-12** audio was never a retrieval defect. Retrieval found, scored, cited and located
  every match; the completeness caveat replaced the answer, and chronology outranked relevance
  in the result sort so truncation kept the wrong segment.

## Remaining, honestly

- **18 clarifications are coverage, not safety.** Most sit in the audit's `no_family` set —
  the generator never receives an enum, so no verified plan is possible. Each point of family
  coverage converts one back into an answer with no safety cost.
- **IMG-01 answers without naming the image** ("Retrieved 1 cited evidence result from the
  selected case scope"). Pre-existing generic fallback; `image_ocr_search` deserves the
  citation-naming answer audio now has.
- **The oracle over-credits `contains` checks** whose expectation is a filename, because it
  searches the whole response blob. DOC-01 and (until WI-12) AUD-01/AUD-03 all scored CORRECT
  this way. Any such check should be read against the analyst-facing text.
- **Latency is not Phase 3 ready.** CDR-11 took 273 s, CDR-02 114 s, DOC-01 103 s. P3's gate
  is p95 < 15 s.
- **Known flake:** `TestForensicRecordsSynthesis` failed twice while a container redeploy had
  the synthesis model under load, then passed 4 consecutive runs in isolation.
