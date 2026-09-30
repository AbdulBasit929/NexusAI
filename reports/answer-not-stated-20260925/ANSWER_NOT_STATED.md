# THE ANSWER IS COMPUTED AND THEN NOT TOLD TO THE ANALYST

Found 2026-09-25 while auditing a stale line in the state file. **The golden 62's
headline of 47 CORRECT materially overstates what an analyst is actually shown.**

## The two answers that make it unarguable

    CDR-09  "What date range do the CDR records cover?"   graded CORRECT
            expect 2026-04-01 / 2026-08-10
            ANALYST SEES: "1 CDR record matched this question. Executed bounded
            source-native typed algebra over 8642 authorized source rows and
            returned 1 deterministic results with contribution lineage."
            -> NEITHER DATE APPEARS. The analyst is told a ROW COUNT.

    CDR-10  "Who did 923001110001 contact most frequently?"  graded CORRECT
            expect 923009998887
            ANALYST SEES: "8 phone contacts ranked for 923001110001. Ranked the
            explicitly selected participant's phone contacts from matching CDR
            events."
            -> THE CONTACT IS NEVER NAMED. The analyst is told how many there are.

The plan is right, the SQL is right, the rows are right. **The narration states
the PROCESS and withholds the RESULT.**

## CORRECTION TO THE FIRST VERSION OF THIS REPORT, made the same day

The first draft said the analyst is never told. **That was too strong and it was
written before checking `data_grid`.** The values ARE in the response and the UI
renders that grid. Corrected severity, per question:

    CDR-10  MODERATE. The grid is legible -- "Counterparty", "Total
            Interactions" -- and the top row is 923009998887 with 121. The
            DIRECT ANSWER does not name it; the table does.
    ACC-02  MILD. The narrative states the total, discloses "across 6 http
            status code values", and lists the top three. The grid has all six.
            Incomplete, not deceptive.
    CDR-09  SEVERE, and the table does not rescue it. The grid columns are
            headed `M1` and `M2`. The analyst must guess which of two
            timestamps is the start. `answer_presentation.go` already names
            this defect class in its own comments: "the same class of defect as
            a column headed M1".

**The defect is real and worth fixing, but it is "the direct answer does not
answer", not "the analyst is never told".** The Phase 1 gate -- "answer stated
in analyst text" -- is what it violates, and that gate exists because an analyst
should not have to reconstruct the answer from a table.

## Scope, measured

Of 62 golden questions, 12 are CORRECT with the expectation absent from the
analyst text. Four are formatting only (the value IS there: "5,000" vs 5000) --
CDR-13, CDR-15, CDR-16, TXN-02. **Eight are genuine:**

| id | question | missing from analyst text |
|---|---|---|
| CDR-09 | date range of CDR records | **both dates — total miss** |
| CDR-10 | most frequent contact | **the contact — total miss** |
| ANPR-03 | first and last seen | **both dates — total miss** |
| CDR-04 | call type breakdown | `GPRS` |
| CDR-05 | calls of each type | `VoLTE call`, `739` |
| IPDR-03 | IPDR sessions by protocol | `DNS`, `613` |
| ACC-02 | HTTP status breakdown | `500`, `46` |
| CASE-04 | explain call type breakdown | `GPRS` |

The five partial misses are all BREAKDOWNS that state some rows and drop others.

## Why the harness did not catch it

`scripts/nexusai_live_eval.py:204` grades `contains` against the whole response:

    missing = [e for e in exp if str(e) not in blob]

`blob` includes the raw result rows, so a value the analyst never sees still
scores. The harness ALREADY knows this is wrong -- its own comment on the
`stated_required` branch says *"A filename the analyst never sees is not an
answer"*, after AUD-01/AUD-03 scored CORRECT that way. **The fix was applied only
to questions carrying the `stated_required` flag, and none of these eight carry
it.** `check: number` does it correctly via `number_stated(text, exp)`.

## This is TWO defects, and they need separating

1. **INSTRUMENT.** `contains` must grade against the ANALYST TEXT, normalised for
   formatting the way `number_stated` already is. Correcting it will LOWER the
   headline from 47 -- truthfully. A metric that counts answers the analyst was
   never given is worse than a lower one.
2. **PRODUCT, and the real bug.** `answer_presentation.go` must state the values
   the plan retrieved. This is the architecture working as designed everywhere
   except the last step: deterministic SQL computes the result, the narration
   layer describes the computation instead of reporting it.

## Related, and wider than recorded

The state file's §3 gap "distinct answers do not name what was counted" is the
same disease. It is not confined to distinct: it affects date ranges, rankings,
and every breakdown that lists some groups and drops others.

**Fix the instrument first** -- that is the standing lesson, and this is the
fifth time it has applied. Nothing measured against the current `contains` check
can be trusted for these eight questions, INCLUDING the breakdown-goal result,
whose four target questions are four of the eight.

---

# STAGE 0 RESULT — 2026-09-25

Configuration asserted from the container for BOTH passes: `VERIFIED_ONLY=true`,
`IR_ARBITRATION=true`, `IR_FALLBACK=true`, `PLAN_CACHE=true`,
`LADDER_ROUTING=true`, `BREAKDOWN_GOAL=false`.

    control   FORENSIC_ANSWER_STATES_VALUES=false   39 CORRECT · 8 NOT_STATED · 14 CLAR · 1 MANUAL
    measured  FORENSIC_ANSWER_STATES_VALUES=true    43 CORRECT · 4 NOT_STATED · 14 CLAR · 1 MANUAL
    + oracle corrections                            45 CORRECT · 2 NOT_STATED · 14 CLAR · 1 MANUAL
    held-out  unchanged                             10 CORRECT · 2 CLARIFIED · 1 WRONG

**Zero new WRONG on either suite. Latency 2526ms -> 2682ms median.**

## 0.1 THE INSTRUMENT — `contains` now grades the ANALYST TEXT

Validated OFFLINE first, by replaying stored raw responses through the corrected
grader: it moved EXACTLY the eight predicted questions and nothing else. The
four formatting-only cases (CDR-13, CDR-15, CDR-16, TXN-02) correctly stayed
CORRECT, because `number_stated` already treats "5,000" and 5000 as the same
number. The live control then reproduced 39/8/14/1 exactly.

`NOT_STATED` is its own verdict, deliberately. "Computed correctly, never
stated" is not the same failure as a wrong answer, and merging them would have
hidden whether this fix worked. **It is a FAILURE, never a pass.**

## 0.2 THE PRODUCT — four converted

    CDR-09  "1 CDR record matched this question."
         -> "The CDR records span event time from 2026-04-01 to 2026-08-10."
    ACC-02  top 3 of 6 -> all six status codes, 500: 46 included
    CDR-05  top 3 of 5 -> all five call types, VoLTE call: 739 included
    IPDR-03 top 3 of 4 -> all four protocols, DNS: 613 included

Two causes, both fixed behind `FORENSIC_ANSWER_STATES_VALUES`:
`sourceNativeResultAnswer` declined ANY plan with more than one measure, and a
MIN/MAX range is two measures by construction, so it fell through to the generic
row-count narration; and a complete breakdown was truncated to three entries.

**A trap caught while writing the tests:** `shortDate` truncates any string of
ten characters or more, so rendering a numeric range through it would silently
chop "1730127963000" to ten digits and state a wrong quantity with full
confidence. It is now applied only to values that parse as timestamps, and
`TestAnswerStatesValuesDoesNotTruncateANumericRange` pins it.

## ORACLE DEFECTS 9 AND 10 — CDR-04 and CASE-04

Both expected the RAW value `GPRS`. The layer declares
`cdr.call_type GPRS -> display_name "Data session"`, and the answer surface
renders curated labels deliberately — "GPRS: 5,863" above a table reading "Data
session" is one value in two vocabularies. **CDR-05 is the independent check:
same field, same 5,863 rows, and it already expected "Data session" and passed
throughout.** Corrected, with the reason recorded in the question note.

This is the STANDING RULE firing again: when curation lands, re-derive every
expectation it touches.

## WHAT REMAINS — 2 NOT_STATED, both on the TEMPLATE path

    CDR-10   "who did 923001110001 contact most frequently?"
             tpl=frequent_contacts -> "8 phone contacts ranked for 923001110001."
             The grid is legible and names 923009998887 (121 interactions).
             The DIRECT ANSWER does not.
    ANPR-03  "when was LHR-2026 first and last seen?"
             tpl=cross_family_correlation -> a coverage paragraph, no dates.

Both are answered by REGISTERED TEMPLATES, not the typed-plan path, so
`sourceNativeResultAnswer` never runs for them. Fixing them means the same
treatment for `tabularResultAnswer` — a separate slice, measured the same way.

**Still owed: the `M1`/`M2` data-grid headers.** CDR-09's narrative is fixed, but
its table still labels two timestamps `M1` and `M2`. `answer_presentation.go`
names this defect class in its own comments. Not bundled here, so that this
measurement stays attributable.
