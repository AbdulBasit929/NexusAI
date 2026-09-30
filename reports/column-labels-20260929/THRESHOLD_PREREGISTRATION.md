# A1 RESULT COLUMN LABELS -- THRESHOLDS

Written 2026-09-29, BEFORE the switch was built into an image or any arm was scored.

## The defect

Result tables label columns from raw keys (`humanizeField`). Census of typed-plan answers on the
deployed build (`reports/plate-read-search-20260928/on`):

    m1 -> "M1"                 54 answers      m1__value_count -> "M1 Value Count"   7
    metadata -> "Metadata"     54              m2 -> "M2"                            1
    section -> "Section"       54
    group keys from raw source names: msisdn -> "Msisdn", inbound_outbound_ind -> "Inbound Outbound IND"

The UX contract (§6.6, §9) forbids a header like "M1". The curated layer already holds the display
names ("Subscriber number", "Call direction") and the plan says what each measure computes.

## Mechanism

`FORENSIC_RESULT_COLUMN_LABELS`, code default OFF, compose `false` until measured. For an executed
typed plan, the data grid's column **headers** (never keys, never values) become:

- **COUNT(\*)**: what was counted ("CDR records", "Detected faces", "Plate reads").
- **COUNT(field)**: "Count of <field>". **COUNT_DISTINCT**: "Distinct <field>".
- **SUM / AVG / MIN / MAX**: "Total / Average / Minimum / Maximum <field>".
- **The denominator column**: "<measure> (values counted)".
- **Group fields**: their curated display name.
- **`metadata`**: "Source rows".

## Pass criteria

    corpus       0 verdicts moved; answer_text identical on all 103
    pre-flight   38/38, text identical
    everyday 14  identical      relational 16 identical      plate probe 8/8, no leaks
    headers      no typed-plan answer shows "M1", "M2" or "Metadata"; every data-grid KEY and every ROW
                 VALUE byte-identical between arms (only header strings differ)

Fail any line -> the switch stays OFF.
