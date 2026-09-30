# A1.1 TABLE HEADER CASING + A2 CITATION TRUTH STATE -- RESULT

    FORENSIC_TABLE_HEADER_CASING    SHIPPED, default ON
    FORENSIC_CITATION_TRUTH_STATE   SHIPPED, default ON

Measured 2026-09-29 against thresholds written before the run (THRESHOLD_PREREGISTRATION.md).
Image c9be9726470a (11:28:18). Rollback tag `rollback-before-a11a2-20260929`.
Comparator: `scripts/nexusai_a11a2_compare.py`, output in `compare.txt`. **No threshold fired.**

## Result

| Check | Result |
|---|---|
| Corpus verdicts moved (103) | 0 |
| Corpus answer_text changed | 0 |
| Data-grid column keys and row values | byte-identical on every answer |
| Header cells recased | 69 (55 distinct) in 8 corpus answers |
| Badly cased headers left ("END", "ROW", "AT", "NON", "Imei", "Imsi", "Cnic", "Msisdn") | 0 |
| Derived-text provenance items with the stored truth state | 68, all matching the census |
| Search (kb_rag) items that gained it | 7 (AUD, DOC, IMG answers) |
| Fact-packet citations matching their item | 254, 0 mismatched |
| records_sql items that gained a truth state | 0 |
| Pre-flight | 38/38, identical in pass and text |
| Everyday 14 / relational 16 | identical |
| Plate probe | 8/8, no leaks |

Read, not only counted (rule 7): every one of the 55 distinct header changes was read. Examples:
"Call END TS" becomes "Call end timestamp", "Call ORG NUM" becomes "Call originating number", "ROW Hash"
becomes "Row hash", and "Imei", "Imsi", "Cnic" and "Msisdn" become IMEI, IMSI, CNIC and MSISDN.
"INBOUND OUTBOUND IND" becomes "Inbound outbound indicator", and "Completed AT" becomes "Completed at".
The citations read were:

- AUD-01: derived_model_observation
- DOC-01: derived_native_text
- IMG-01: derived_model_observation

In each case both the provenance item and citation C1 carry it.

## One check fired and was explained, not waived

The comparator also checks that the switch-off build reproduces the reference headers. It fired on
ACC-02: the reference had "Count of hTTP status code", and the off arm had "Count of HTTP status code".

The reference (`reports/column-labels-20260929/on`) was measured **before** the A1 acronym fix. That
fix shipped in the 10:56 image; see `reports/column-labels-20260929/RESULT.md`, defect 1, which
records the deployed build saying "HTTP". So the off arm does reproduce the deployed build. The
comparator now accepts exactly that one named difference (`SHIPPED_SINCE_REFERENCE`), and any other
header difference still fires.

## What this closes

- The UI can badge a transcript or image-text citation as a model observation instead of
  "Source record" (BACKEND_REQUESTS row 7).
- Template tables no longer show shouted fragments of column names.
