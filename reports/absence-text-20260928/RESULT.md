# S2 + S3 ABSENCE STATEMENTS -- RESULT

Measured 2026-09-28. Thresholds in `THRESHOLD_PREREGISTRATION.md`, written before the image was built.
Scored by `scripts/nexusai_absence_compare.py`.

    FORENSIC_TEXT_SEARCH_NOT_ABSENCE    SHIPPED, default ON   (S2)
    FORENSIC_CLARIFICATION_NOT_RESULT   SHIPPED, default ON   (S3)

## Results

    corpus       off vs deployed build 0 moved; on vs off exactly 1 moved:
                 P2  WRONG -> CLARIFIED, reason text_search_not_absence
                     "I searched the transcripts for the words in your question and found no match.
                      A word search cannot show that something is absent, so I have not answered no. ..."
                 DOC-06 (exact phrase, correctly not found) unchanged in verdict and text
    relational   R5 off "No qualifying phone-counterparty events were found for the selected target ..."
                 R5 on  "Which exact MSISDN and two to eight exact CDR source identities should I compare?"
                 other 15 unchanged
    pre-flight   38 of 38 identical      everyday 14 identical

**No threshold fired.** Both switches were measured together as the configuration that ships. Their
censuses were disjoint (one answer each, different code paths) and exactly the two predicted answers
moved.

**Wrong answers in the shipped system: 3 -> 2** (the two remaining are the parked audio questions H2
and M15).

## Note on the "on" arm

While the on arm ran, eight questions were asked through the UI for a demo check (all cached
phrasings, read-only). The pre-flight, everyday and relational results of the on arm are identical to
the off arm, so the overlap did not affect the measurement.

Rollback image: `nexusai-forensic-records-api:rollback-before-absence-20260928`.
