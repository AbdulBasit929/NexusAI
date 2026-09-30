# S1 ROW-COUNT HEADLINE GUARD -- THRESHOLDS

Written 2026-09-28, BEFORE the switch was built into an image or any arm was scored.

## The defect (captured today on the deployed build)

    "Show me everything you have about plate ABC-123"
        -> "1 ANPR sighting matched this question."        truth: 218 sightings
    "Summarise the suspicious activity in this case"
        -> "14 records matched this question."             14 is the number of summary rows

Both reach the terminal fallback of `enterpriseExecutiveAnswer` (query.go), which states the number of
RESULT ROWS as "N <family> matched this question". That sentence is true only for a list of source
records whose N is the full total. Here it is not:

- ABC-123: the model-written plan filters `anpr.plate_number EQ ABC-123` and computes FOUR measures
  (COUNT(*) = 218, three more). One aggregate row came back. `sourceNativeResultAnswer` declines any
  plan with more than one measure, so the fallback reported "1".
- Suspicious activity: the `suspicious_patterns` template returns a composite summary (night activity,
  duration extremes, data-quality warnings) with 14 rows. They are findings, not matching records.

The existing post-execution withhold (`uncomputedQuantityAnswer`) catches this only when the question
says "how many". Neither of these does.

## Census (recorded responses, the production pipeline's own output)

    fallback marker set, deployed build, 103 questions   2  (CASE-01, IMG-04 -- both already WITHHELD)
    fallback marker set, CORRECT answers                 0
    multi-measure plans with no grouping (103 q, both ladder settings)   1  (CDR-09, MIN+MAX range,
                                                                            answered by the range path
                                                                            BEFORE the new code runs)

## Mechanism

`FORENSIC_ROW_COUNT_HEADLINE_GUARD`, code default OFF, compose default `false` until measured.

- **Part A (compute):** a typed plan with several measures, no grouping, no time bucket and no
  projection, one of which is COUNT(*), states that count with the same sentence the single-COUNT path
  uses ("There are 218 ANPR sightings involving ABC-123 in this case."). It then says the other
  computed values are shown with the answer. It runs only after the range path has declined.
- **Part B (never state a row count as a finding):** at the terminal fallback,
  - a record listing whose full total is known and larger than the page reads "Showing N of T ..."
  - a record listing with no known total reads "Showing N ...; the full number was not computed"
  - a listing whose total equals N keeps today's sentence (it is true)
  - anything else (aggregate results, composite templates) states no number and says what the
    results are.
  The `executive_answer_uncomputed` marker is unchanged, so `uncomputedQuantityAnswer` withholds
  exactly what it withholds today.

## Arms (switch asserted from `docker inspect`)

    off   deployed posture, switch OFF -- must reproduce reports/guards-20260928/shipped exactly
    on    deployed posture, switch ON
    Each arm: 103-question corpus, the 14 everyday questions (adhoc_probe.py), the 38-check demo
    pre-flight (preflight.py), the 16-question relational probe.

## Pass criteria

    corpus        on vs off: 0 of 103 moved (census predicts 0)
    ABC-123       on: headline contains "218" and "ABC-123"; no "1 ANPR sighting"
    suspicious    on: no "N records matched" sentence; no number stated as a count of records
    pre-flight    on vs off: all 38 identical in pass/fail AND answer text
    relational    on vs off: all 16 identical in state
    everyday 14   on vs off: only the two questions above change text

Fail any line -> the switch stays OFF and the result is recorded. Pass all -> compose default flips
to `true` in the same change that records the measurement.
