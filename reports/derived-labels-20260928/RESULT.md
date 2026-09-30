# S4 DERIVED RESULT LABELS -- RESULT

Measured 2026-09-28. Thresholds in `THRESHOLD_PREREGISTRATION.md`, written before the image was built.
Scored by `scripts/nexusai_labels_compare.py`.

    FORENSIC_DERIVED_RESULT_LABELS   SHIPPED, default ON

## Results

    corpus       off vs deployed build 0 verdicts moved; on vs off 0 verdicts moved
    text         changed on exactly the 8 census questions, same number in each:
      M1   307 ANPR sightings           -> 307 plate reads
      M2   ... across ANPR sightings    -> ... across plate reads (0.96)
      M5   24 ANPR sightings            -> 24 video plate groups
      M9   309 records                  -> 309 image text regions
      M13  11 records                   -> 11 transcribed audio segments
      M16  20 records                   -> 20 detected faces
      M17  ... across records           -> ... across detected faces (0.79)
      M19  22 records                   -> 22 image fingerprints
    P1           "There are 1,057 ANPR sightings in this case." unchanged (camera sightings)
    pre-flight   38 of 38 identical pass/fail; text changed only on the two allowed questions
    everyday 14  identical      relational 16  identical

**No threshold fired.**

## Caught before measurement

The first wiring replaced every family noun in `answer_presentation.go`, including the template path,
where the family is resolved from the template (`templatePrimaryRecordType`) rather than the request.
`TestP1ResultAnswerTotalsATemplateBreakdownByCategory` failed ("6,971 records" for "6,971 CDR
records") in the full package run before any image was built. The template path now keeps its own
family through `resultNounForFamily`.

## Found in passing

`stim_fact_packet.go` builds the narrator's fallback fact with the same "N <family> match this
question" row-count sentence that S1 removed from the analyst headline. It is used only on the
narration path. Recorded for the next S1-class pass.

Rollback image: `nexusai-forensic-records-api:rollback-before-labels-20260928`.
