# A3 TEMPLATE ANSWERS STATE THEIR RESULT -- RESULT

    FORENSIC_TEMPLATE_STATES_RESULT   SHIPPED, default ON

Measured 2026-09-29 against thresholds written before the run (THRESHOLD_PREREGISTRATION.md).
Image f065680d7104 (11:51:26). Rollback tag `rollback-before-a3-20260929`. The off arm also served as
the post-ship check of A1.1 and A2: it reproduced `reports/a1-1-a2-20260929/on` exactly.
Comparator: `scripts/nexusai_templatestates_compare.py`, output in `compare.txt`. **No threshold fired.**

## Result

| Check | Result |
|---|---|
| CDR-10 | NOT_STATED → **CORRECT** |
| ANPR-03 | NOT_STATED → **CORRECT** |
| Other corpus verdicts moved (101) | 0 |
| Other corpus answer_text changed | 0 |
| Data grids | byte-identical on every answer |
| Pre-flight | 38/38, no text changed |
| Everyday 14 | 1 text changed, "speak to most often" (frequent_contacts), read below |
| Relational 16 | identical |
| Plate probe | 8/8, no leaks, identical |

## What the analyst now reads

**CDR-10**, "Who did 923001110001 contact most frequently?", which before read "8 phone contacts
ranked for 923001110001.":

> 923009998887 and 923009998883 are tied as the most frequent contacts of 923001110001, with 121 CDR
> records each. 8 phone contacts ranked for 923001110001.

**ANPR-03**, "When was LHR-2026 first and last seen?", which before read "Computed exact normalized
target matches, record-family coverage, …":

> LHR-2026 was first seen at 2026-07-14 06:00:00 UTC and last seen at 2026-07-19 07:22:00 UTC, across
> 87 ANPR sightings.

The everyday question "Who did that number 923001110001 speak to most often?" now gives the same
tied answer as CDR-10.

## Checked against the database, not only against the corpus

Read-only queries (`BEGIN READ ONLY … ROLLBACK`), written independently of the template SQL:

    CDR counterparties of 923001110001   923009998883 121 · 923009998887 121 · 923009998882 117
    forensic.records, LHR-2026            anpr · 87 · 2026-07-14 06:00:00+00 · 2026-07-19 07:22:00+00

## Why CDR-10 names two contacts

The corpus note expects 923009998887 because it leads on OUTGOING records (68 vs 67). The question
asks "most frequently", and the template ranks by total records, where the two are tied at 121.
Naming one of them would pick a side of a tie that the data does not break. The direction split is
not restated as "who called whom", because a record's direction is relative to that record's owner.

## Rules the code keeps (template_result_statements.go)

- A tie is named in full. If every returned row ties and the page is full, the tie may continue past
  the page, so nothing is added.
- First and last seen come from the family coverage, which is computed over every exact match. They
  never come from the 20-row displayed page, which starts at 07:50, not 06:00.
- Unparseable times, missing counts or an inverted range add nothing.
- Filters are named ("in the requested date range"), never restated with bounds.
