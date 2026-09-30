# HEADLINE HONESTY: FILTERS STATED AND EMPTY AGGREGATES -- THRESHOLDS

Written 2026-09-29, before either switch was built into an image or any arm was scored.

## Defects (found by 8 new questions, checked against the database)

`reports/laptop-speed-20260929/RESULT.md`:

    S5  "How many ANPR sightings are there for plate LHR-2026 in total?"
        -> "There are 87 ANPR sightings in this case."     (87 for LHR-2026; the case has 750)
    S7  "How many IPDR sessions used the HTTPS protocol?"
        -> "There are 644 IPDR sessions in this case."     (644 HTTPS; the case has 2,500)
    S8  "What is the total number of bytes uploaded across all IPDR sessions?"
        -> "The total bytes uploaded across IPDR sessions is 0."   (no upload values exist)

## Changes (both default off)

- `FORENSIC_HEADLINE_STATES_FILTERS`: EQ, NEQ, IN, BETWEEN, CONTAINS, IS_NULL and IS_NOT_NULL filters
  in the executed plan are named in the headline, for example "with protocol HTTPS". A filter equal to
  the target isn't repeated, range bounds keep their existing wording, a PII or withheld value is
  never repeated, and filter-only fields keep their own sentence.
- `FORENSIC_EMPTY_AGGREGATE_HONEST`: when SUM, AVG, MIN or MAX has no values (NULL, or 0 values
  counted), the headline says so instead of stating 0.

## Arms

    off   both OFF (must reproduce reports/count-names-field-20260929/on)
    on    both ON
    Each: corpus, 14 everyday, 38 pre-flight, 16 relational, plate probe, and the 8 new questions
    (speed_probe.py). The plan cache is on, so the second arm reuses the first arm's plans and only
    presentation differs.

## Pass criteria

    new questions  S5 names plate number LHR-2026, S7 names protocol HTTPS, and S8 says no values are
                   recorded; the other 5 texts are unchanged
    corpus         0 verdicts moved; answer_text changes only where the executed plan has a non-range,
                   non-target filter or an empty aggregate, and every changed text is read
    probes         pre-flight 38/38 with no pass->fail; plate 8/8 with no leaks and plate texts
                   identical; other probe changes only on the same shapes, each read
    privacy        no changed text contains a CNIC or other PII-class value
    data grid      keys, rows and headers identical

Fail any line -> the failing switch stays OFF.
