# S1 ROW-COUNT HEADLINE GUARD -- RESULT

Measured 2026-09-28. Thresholds in `THRESHOLD_PREREGISTRATION.md`, written before the image was built.
Scored by `scripts/nexusai_rowcount_compare.py`.

    FORENSIC_ROW_COUNT_HEADLINE_GUARD   SHIPPED, default ON

## Results

    corpus        off vs deployed build   0 of 103 moved (switch-off build is inert)
                  on vs off               0 of 103 moved (census predicted 0)
    everyday 14   exactly the two targeted questions changed:
        "Show me everything you have about plate ABC-123"
            off  "1 ANPR sighting matched this question."
            on   "There are 218 ANPR sightings involving ABC-123 in this case. The other values
                  computed for this question are shown with this answer."
        "Summarise the suspicious activity in this case"
            off  "14 records matched this question."
            on   "The results shown come from: Rule-based anomaly summary covering night activity,
                  duration extremes, and quality warnings. No single figure answers this question,
                  so none is stated."
    pre-flight    38 of 38 identical in pass/fail and text (33 pass in both arms)
    relational    16 of 16 identical

Each arm's switch state was asserted from `docker inspect` and saved as `switches.txt`.
**No threshold fired.**

## Why it works

The ABC-123 plan was correct: `anpr.plate_number EQ ABC-123`, COUNT(*) = 218 among four measures.
`sourceNativeResultAnswer` declined any plan with more than one measure, so the terminal fallback
reported the single aggregate row as "1 ANPR sighting". Part A states the COUNT(*) of an ungrouped
multi-measure plan in the sentence the single-COUNT path uses. It runs only after the MIN/MAX range
path has declined, so CDR-09's range answer is untouched. Part B stops the fallback from stating a
row count that is not a complete count of matching records. The `executive_answer_uncomputed`
marker is unchanged, so the existing "how many" withhold behaves exactly as before.

Rollback image: `nexusai-forensic-records-api:rollback-before-rowcount-20260928`.
