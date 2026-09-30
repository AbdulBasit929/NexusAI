# A1 RESULT COLUMN LABELS -- RESULT

Measured 2026-09-29. Thresholds in `THRESHOLD_PREREGISTRATION.md`, written before the image was built.
Scored by `scripts/nexusai_columnlabels_compare.py`.

    FORENSIC_RESULT_COLUMN_LABELS   SHIPPED, default ON

## Results

    corpus       off vs deployed build 0 moved; on vs off 0 verdicts moved, 0 answer texts changed
    data grids   54 typed-plan answers relabelled; 0 still show M1/M2/Metadata;
                 every column KEY and every ROW VALUE byte-identical between arms
    probes       pre-flight 38/38 identical; everyday 14 identical; relational 16 identical;
                 plate probe 8/8, no leaks
    examples     M1 -> "CDR records", "Detected faces", "Plate reads", "Distinct subscriber number",
                 "Total bytes transferred", "Maximum call duration"; msisdn -> "Subscriber number";
                 inbound_outbound_ind -> "Call direction"; metadata -> "Source rows"

**No threshold fired.**

## Two defects found after the thresholds passed, both fixed before shipping

1. **"Count of hTTP status code".** Lower-casing the first letter broke a leading acronym. Found by
   reading every produced header (roadmap rule 7), not by the pass criteria. Fixed, and pinned by
   `TestSourceNativeColumnLabels/an_acronym_at_the_start_of_a_field_name_is_never_broken`.
2. **The ship build silently did not run.** After the fix, the redeployed container still said "hTTP":
   the image was the 10:26 arm build and the source fix was 10:47. The build in that command printed
   nothing. A rebuild (10:56) was checked by image timestamp and container image id, and then the
   deployed build said "Count of HTTP status code" with pre-flight 38/38. **New rule: after every
   build, confirm the image timestamp changed before deploying.**

## Not in scope, next

Template (non-typed) tables still use the generic `humanizeField`, which capitalises every word of three
letters or fewer: "Call END TS", "ROW Hash", "Completed AT", "Imei", "Cnic". That's A1.1, with its own
switch.

Rollback image: `nexusai-forensic-records-api:rollback-before-columnlabels-20260929`.
