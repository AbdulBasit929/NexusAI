# Verified-only mode + rescue — 2026-09-23

62/62. `FORENSIC_IR_FALLBACK=true`, `FORENSIC_IR_ARBITRATION=true`,
`FORENSIC_VERIFIED_ONLY=true`, shadow OFF. Graded with `--analyst-text`.

## Phase 1 gate

| | before | after | gate |
|---|---:|---:|---|
| confident-wrong | 23 | **9** | 0 |
| correct-or-clarified | 61% | **82%** | >=70% **PASS** |
| HTTP 500 / ERROR | 0 | 1 | 0 |

| verdict | before | after |
|---|---:|---:|
| CORRECT | 33 | **40** |
| WRONG | 20 | **9** |
| PRESENT_NOT_TOP | 3 | **0** |
| CLARIFIED | 5 | 11 |

**IMPROVED 15 · REGRESSED 4.**

## What did the work

A structured analytical question is answered from a plan that passed S6, S9 SHAPE
and CONSTRAINT_APPLIED, or not at all. When an unverified route is about to answer,
the system first tries to EARN a verified plan (arbitration), and only withholds if
it cannot.

That combination converts confident-wrong answers into CORRECT ones, not merely into
clarifications — which is better than the projection this was built on (12 withheld
for 9 lost). Twelve questions flipped to CORRECT; eight of them now route through
`semantic_ir_fallback`, meaning the GENERATED PLAN answered, not a template:

    ACC-02 ACC-03 ANPR-04 ANPR-06 CASE-04 CDR-04 CDR-09
    IPDR-04 IPDR-05 TWR-01 TXN-01 TXN-02

PRESENT_NOT_TOP went to zero: every ranking question that used to return the right
answer in the wrong position now returns it on top.

## The 9 remaining confident-wrong

| domain | n | ids |
|---|---:|---|
| **media / document / cross-family** | 7 | AUD-02, CASE-01, DOC-04, DOC-05, IMG-02, IMG-04, X-01 |
| **structured analytics** | 2 | CDR-08, CDR-16 |

The structured side — everything this work addresses — is down to TWO, and CDR-08 is
already fixed in the working tree. The other seven are a different competence
(text/image/audio retrieval and cross-family correlation) that the source-native
algebra does not serve and this work never touched.

## The 4 regressions, honestly

    ANPR-05  CORRECT -> CLARIFIED   no derivable plan; withheld rather than guessed
    SUB-03   CORRECT -> CLARIFIED   its answer needs PII the privacy guard withholds
    TWR-02   CORRECT -> CLARIFIED   no derivable plan
    CDR-15   CORRECT -> ERROR       a BUG I introduced; fixed, see below

Three are the intended trade: a clarification instead of an answer stated on no
authority. CDR-15 is not a trade, it is a defect.

## Two defects this run exposed, both mine, both fixed

**CDR-08 — a regression my own feature caused.** Implementing `COUNT_DISTINCT` made a
plan POSSIBLE for "how many unique phone numbers"; the generator chose a plain COUNT
and the rescue path stated 8,642 instead of 10. Previously it clarified safely. My
justification for arbitration — "a clarification asserts nothing, so replacing it
cannot turn a correct answer wrong" — is true and is NOT the whole safety question:
it can turn a safe non-answer into a confident wrong one. Fixed as an S9 OBLIGATION:
a question asking for unique values must produce COUNT_DISTINCT or the plan is
rejected. **A new capability must arrive with the check that bounds it.**

**CDR-15 — a bug my test lied about.** `SQLSTATE 42P18`, an HTTP-500-class error on a
gate criterion. The derived-metric code built the typed expression and then discarded
it for derived fields, leaving orphaned parameters referenced by nothing. It survived
because `TestDerivedDurationMetricIsExpressibleAndExecutes` asserted the SQL STRING
contained EXTRACT and never executed it — and shadow mode generates plans without
running them. There is now a test that executes against real Postgres.

## Next

One clean run with both fixes should show structured confident-wrong at 1 (CDR-16) and
no ERROR. The gate then needs the media/document/cross-family work — 7 named questions
in a domain this work never entered.
