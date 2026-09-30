# Phase 1 — structured analytics at ZERO confident-wrong, 2026-09-23

62/62. `FORENSIC_IR_FALLBACK=true`, `FORENSIC_IR_ARBITRATION=true`,
`FORENSIC_VERIFIED_ONLY=true`. Graded with `--analyst-text`.

## Phase 1 gate

| | WI-6 | prev | **FINAL** | gate |
|---|---:|---:|---:|---|
| confident-wrong | 23 | 10 | **5** | 0 |
| correct-or-clarified | 61% | 82% | **90%** | >=70% **PASS** |
| HTTP 500 / ERROR | 0 | 1 | **0** | 0 **PASS** |

CORRECT **42** · WRONG **5** · CLARIFIED **14** · MANUAL 1 · ERROR **0**.
**IMPROVED 5 · REGRESSED 0** against the previous run.

## The remaining 5 are ALL outside this work's domain

    STRUCTURED analytics (cdr, ipdr, anpr, subscriber, tower, transaction, access_log)
       0   <- was 13 at the start of 2026-09-22

    media / document / audio / cross-family
       5   CASE-01(cross) DOC-04(document) IMG-04(image) AUD-02(audio) X-01(cross)

Every structured family is at zero. CDR alone went from 6 confident-wrong to 0.

## What answers the questions now

Twelve questions the keyword ladder used to get wrong are answered from a
GENERATED PLAN, not a template — `route=semantic_ir_fallback`:

    ACC-02 CDR-02 IPDR-03 IPDR-05 SUB-01 TWR-01 TXN-01 TXN-02

plus ACC-03, ANPR-04, ANPR-06, CDR-04, CDR-12, CDR-16 answered from plans the
compiler derived. `PRESENT_NOT_TOP` is zero: every ranking question that returned
the right answer in the wrong position now returns it on top.

## Fixed this round

- **CDR-16** — a CORRECT plan returned zero rows. "each source file**?**" bound a
  `source_file` filter to `"?"`, the first non-space token after the phrase.
  Guards: "each/every/per source file" is a grouping not a filter, and a file name
  must contain an alphanumeric.
- **DOC-05, IMG-02** — media questions answered by structured templates. "What
  remote access setup does the HP **guide** describe?" was answered from
  `access_failed_events` because "access" matched the access-log family. Two
  candidate fixes were MEASURED first: withholding every unverified structured
  answer scored 4 wrong removed / 3 CORRECT lost; keying on the artifact noun
  scored **2 removed / 0 lost**. Took the precise one.
- **CDR-08** — S9 obligation: a question asking for unique values must produce
  COUNT_DISTINCT or the plan is rejected. It had regressed the moment
  COUNT_DISTINCT became expressible. **A new capability must arrive with the check
  that bounds it.**
- **CDR-15** — `SQLSTATE 42P18`. The derived-metric code built the typed
  expression then discarded it, leaving orphaned parameters. Survived because the
  test asserted the SQL STRING and never executed it; there is now a test that
  runs against real Postgres.

## What the gate still needs

Five questions, all text/image/audio retrieval and cross-family correlation — a
competence the source-native algebra does not serve and this work never entered.
That is the next, separate piece of work.
