# A3.1 + A3.2 — COMPILER GAPS, THRESHOLDS WRITTEN BEFORE THE RUN

Written 2026-09-27, after both censuses and before the fix arms were built or scored. The
ladder-off baseline arm was already running; its results had not been read.

## The two defects

**A3.1 — CDR-16**, *"How many CDR records came from each source file?"*. The `source_rows` goal
captured it on "source" + "records", so the compiler projected twenty raw rows. The grouping hint
was already right. Fix: `FORENSIC_SOURCE_ROWS_QUANTITY_GUARD` — a question carrying an aggregate
marker is left to the aggregate case. The markers are the aggregate case's own list, lifted into
`semanticAggregateMarkers` so there is one definition.

**Census: `source_rows` captures exactly 1 corpus question (CDR-16), and the guard releases exactly
that one.**

**A3.2 — CDR-12**, *"Which cell site handled the most calls?"*. It matches CDR ("calls") and tower
("cell", "site"), and every multi-match became `cross_family`. Fix:
`FORENSIC_FRAME_FAMILY_PRECEDENCE` — resolve a multi-match through the precedence
`extractCanonicalRecordType` already encodes, only when every matched family is a structured records
family and the answer is one of the matched families.

**Census, first version: moved 10 questions, 8 of them WRONG** — DOC-01/DOC-04 (documents) and
H1/H3/M1/M2/M5/VID-01 (model-read media) sent to ANPR sightings. Narrowed to structured families
only. **Census, final version: moves exactly CDR-12 and IPDR-05**, both intended. The eight are
asserted must-stay-cross_family in `frame_family_precedence_test.go`.

## Why a ladder-off arm

In the shipped posture (ladder ON) CDR-12 and CDR-16 are already CORRECT via the IR arbitration, so
fixing the compiler cannot move them there. **The ladder-off arm is the instrument for the compiler
path**: every question goes through the real handler pipeline into language assistance, the
deterministic compiler and the IR fallback, with no stage skipped. That matters: earlier today an
offline test that bypassed `applyCanonicalQueryHints` proved a fix for a defect production did not
have.

## Arms

    ON-control    ladder ON,  both switches OFF   shipped posture; must reproduce
    ON-fix        ladder ON,  both switches ON    SAFETY: the shipped posture must not regress
    OFF-baseline  ladder OFF, both switches OFF   the compiler path today
    OFF-fix       ladder OFF, both switches ON    PROGRESS: the compiler path with the fixes

## Thresholds

**Safety, judged on ON-fix vs ON-control — any of these reverts both switches:**

- ON-control does not reproduce 67 / 28 / 4 / 2 / 1 / 1 → void.
- Any CORRECT lost (this covers every honesty probe, which scores CORRECT when it refuses).
- Any new WRONG.
- **Predicted ON movement: none.** CDR-12, CDR-16 and IPDR-05 are CORRECT on ladder-on today
  through other paths. The frame family and goal are also read by the IR arbitration's narrowing,
  so movement is *possible*; any must be named and explained before shipping.

**Progress, judged on OFF-fix vs OFF-baseline:**

- Predicted: **CDR-16 and CDR-12 improve** (toward CORRECT).
- Any question CORRECT in OFF-baseline and not in OFF-fix is a compiler-path regression and must
  be explained; a new WRONG in OFF-fix blocks shipping even though ladder-off is not the shipped
  posture — it would be a regression waiting for the ladder's removal.

## Decision

    ON-fix clean AND OFF-fix improves with no new WRONG    -> SHIP both, default ON
    ON-fix clean, OFF-fix flat                              -> ship OFF; the fix is not reached
    anything else                                           -> REVERT and explain
