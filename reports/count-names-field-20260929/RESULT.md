# A4 A COUNT OF A FIELD SAYS WHICH FIELD IT COUNTED -- RESULT

    FORENSIC_COUNT_NAMES_FIELD   SHIPPED, default ON

Measured 2026-09-29 against thresholds written before the run (THRESHOLD_PREREGISTRATION.md).
Image de5c372e1b70 (14:05:36), the second A4 build. Rollback tag `rollback-before-a4-20260929`.
Comparator: `scripts/nexusai_countnames_compare.py`. The final run is in `compare.txt` and the first
in `compare-run1.txt`, `run1-off/` and `run1-on/`.

## The first build passed its thresholds and was not shipped

On the first build (22c6b5ecfbd0) no threshold fired. Reading every changed text found one that was
worse. The pre-flight question "How many transactions have an amount above 50000?" counts the amount
field and also filters on it:

    before  There is 1 transaction with amount above 50000 in this case.
    after   1 transaction with amount above 50000 has a transaction amount value.

The second sentence is true and worse. A filter on the counted field already guarantees the field is
present, so there the count is the record count, and the record sentence is right. The fix: COUNT of a
field is stated as "have a … value" only when no filter names the same field. A test pins both cases.
Rebuilt, then re-measured from scratch.

## Result (second build)

| Check | Result |
|---|---|
| CDR-08 | CORRECT, "There are 10 distinct subscriber number values across CDR records." |
| ANPR-05 | CORRECT, "There are 6 distinct plate number values across ANPR sightings." |
| H1-CDR-HANDSETS | CORRECT, "There are 4,997 distinct device IMEI values across CDR records." |
| Other corpus verdicts or texts changed | 0 |
| Data grids | byte-identical |
| Pre-flight | 38/38; one text changed, the pre-flight copy of CDR-08 |
| Transaction pre-flight question | unchanged ("There is 1 transaction with amount above 50000 in this case.") |
| Everyday, relational, plate | identical; plate 8/8, no leaks |
| Off arm vs the A3 build | identical |

## Why CDR-08 matters beyond wording

The question asks about "callers". The plan counted distinct **subscriber** numbers (`cdr.msisdn`).
The answer used to say only "the count", which hid that interpretation. Now the analyst can see which
field was counted and ask again if they meant the originating number.
