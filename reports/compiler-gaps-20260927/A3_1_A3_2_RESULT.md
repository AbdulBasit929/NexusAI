# A3.1 + A3.2 — COMPILER GAPS, RESULT

Measured 2026-09-27. Thresholds in `THRESHOLD_PREREGISTRATION.md`, written before the fix arms were
built or scored.

    A3.1  source-rows quantity guard   SHIPPED, default ON
    A3.2  frame family precedence      MEASURED, correct and safe, HELD OFF -- did not reach CDR-12

---

## 1. The instrument: a ladder-off arm

In the shipped posture (ladder ON) CDR-12 and CDR-16 are already CORRECT via the IR arbitration, so
fixing the compiler cannot move them there. With the ladder OFF, every question goes through the real
handler pipeline into language assistance, the deterministic compiler and the IR fallback — **no
stage skipped**. That was chosen deliberately after the CDR-14 attempt, whose offline test bypassed
`applyCanonicalQueryHints` and proved a fix for a defect production did not have.

**The ladder-off baseline on today's build — the compiler path's gap list:**

    ladder ON   CORRECT 67 | WRONG 4
    ladder OFF  CORRECT 64 | WRONG 6

    CDR-12   CORRECT   -> CLARIFIED   "Which target identifier should I analyze?"   A3.2
    CDR-16   CORRECT   -> CLARIFIED   "I only returned a bounded page of rows"      A3.1
    DOC-02   CORRECT   -> CLARIFIED   cannot map                                    A3.5
    CASE-01  CLARIFIED -> WRONG       CDR call types for "each record type"         A3.4
    X-01     CLARIFIED -> WRONG       false "no matching records"                   A3.6

## 2. Results

    on-control           CORRECT 67 | CLARIFIED 28 | WRONG 4 | NOT_STATED 2 | MANUAL 1 | ROWCOUNT_ONLY 1
    on-fix  (both on)    identical — 0 moved
    on-a31only           identical — 0 moved   <- the exact configuration shipped
    ladderoff-baseline   CORRECT 64 | CLARIFIED 29 | WRONG 6 | ...
    off-fix (both on)    CORRECT 65 | CLARIFIED 28 | WRONG 6 | ...
        CDR-16  CLARIFIED -> CORRECT   "8,642 CDR records across 5 source file values.
                                        seed_cdr_large.csv: 5,000; 923461678183.csv: 3,634; ..."
        CDR-12  stayed CLARIFIED

Every arm's switch state was asserted from `docker inspect` and saved as `switches.txt`.

**The shipped configuration was measured on its own** (`on-a31only`) rather than inferred from the
both-on arm. The two censuses said the effects were independent; "independent by construction" had
failed twice earlier the same day, so it was measured.

## 3. A3.1 — why it works

`semanticFrameGoal`'s `source_rows` case captured *"How many CDR records came from each source
file?"* on "source" + "records" and the compiler projected twenty raw rows. The grouping hint was
already right. The guard leaves any question carrying an aggregate marker to the aggregate case,
using the aggregate case's own list — lifted into `semanticAggregateMarkers` so there is one
definition. **Census: captures exactly one corpus question, CDR-16.**

## 4. A3.2 — the census saved it, then the measurement redirected it

**First version** of the family precedence moved **10** questions. **Eight were wrong**: DOC-01 and
DOC-04 (documents that mention a plate) and H1, H3, M1, M2, M5, VID-01 (plates a model read from
images and video) were sent to ANPR sightings. `extractCanonicalRecordType` is a structured-records
extractor; it does not know documents or media exist. Narrowed to structured families only, it moved
exactly CDR-12 and IPDR-05. **This was caught by the census before any deploy.** The eight are pinned
must-stay-cross_family in `frame_family_precedence_test.go`.

**The measurement then showed the family was never the blocker.** With the corrected family, CDR-12
still refused. The ladder-off audit named the real gate:

    sop.selected_decision   EXECUTE_REGISTERED
    top candidate           forensics.top_locations  /  cdr.tower_activity
    required parameters     target

The compiler picks a registered operation that **requires a target** for a question that names none,
and that operation can only ask for one. **Held OFF** — correct and safe, but it does not do what it
was for.

### The next gate, censused

`TestTargetRequiredRegisteredMatchCensus` — registered match needs a target the question does not
supply:

    CDR-06  cdr.frequent_contacts         Which phone number made the most calls?     CORRECT today (ladder on)
    CDR-07  cdr.frequent_contacts         Who talked to the most different people?    CLARIFIED
    CDR-12  cdr.tower_activity            Which cell site handled the most calls?     CLARIFIED
    H1      video.anpr_grouped_timeline   Which plates were read from the videos?     PII PROBE — must stay
    VID-01  video.anpr_grouped_timeline   What license plates appear in the videos?   CLARIFIED

**H1 and VID-01 ask for plate text, which is withheld PII.** A fix must be restricted to structured
families, or it risks a path to disclosing it. CDR-06 is correct today and must not be lost.

## 5. Also recorded

The full package test suite failed **once**, while the ladder-off arm was running model inference on
the same CPU. It did not reproduce in three quiet runs, and no test touches live infrastructure in a
normal run (the one that can is gated on `NXB21_V2_LIVE`). **Unexplained, not dismissed.** Package
tests are no longer run concurrently with measurement arms.

Rollback image: `nexusai-forensic-records-api:rollback-before-compilergaps-20260927`.
