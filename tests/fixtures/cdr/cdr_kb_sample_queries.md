# CDR Knowledge Base Sample Queries

Upload `cdr_kb_seed.csv` into a Knowledge Base collection, then use these queries to verify behavior.

## Ground Truth Summary

- Shortest completed voice call overall: `CALL-0003`, `7` seconds, `Gulberg`, `2026-07-10`.
- Shortest completed voice call in `DHA`: `CALL-0010`, `12` seconds, `2026-07-12`.
- Failed call: `CALL-0004`, source `923009990000`, target `923331234567`, `Johar Town`.
- Longest completed voice call: `CALL-0011`, `2700` seconds, `Bahria Town`, source `923007777111`.
- Roaming call: `CALL-0009`, source `923001112222`, target `923331111222`.

## Sample Queries And Expected Answers

1. Query:
   `Which CDR record has the shortest completed voice call overall?`

   Expected:
   `CALL-0003`, `duration_seconds=7`, `area=Gulberg`, `target_msisdn=923334445555`.

2. Query:
   `Which call is the shortest completed voice call in DHA?`

   Expected:
   `CALL-0010`, `duration_seconds=12`, `source_msisdn=923002223333`, `target_msisdn=923336667777`.

3. Query:
   `List all target numbers called by 923001112222.`

   Expected:
   `923334445555`, `923336667777`, `923339999000`, `923331111222`.

4. Query:
   `Show calls from 923001112222 on 2026-07-10.`

   Expected:
   `CALL-0001` and `CALL-0002`.

5. Query:
   `Which calls happened in Gulberg?`

   Expected:
   `CALL-0001`, `CALL-0003`, `CALL-0007`, `CALL-0009`, `CALL-0012`.

6. Query:
   `Which CDR entries target 923334445555?`

   Expected:
   `CALL-0001`, `CALL-0003`, `CALL-0007`, `CALL-0012`.

7. Query:
   `Find the failed call and explain its source, target, and area.`

   Expected:
   `CALL-0004`, source `923009990000`, target `923331234567`, area `Johar Town`, status `failed`.

8. Query:
   `Which call lasted the longest?`

   Expected:
   `CALL-0011`, `duration_seconds=2700`, source `923007777111`, target `923339876543`, area `Bahria Town`.

9. Query:
   `Which calls used cell id LHR-GUL-014?`

   Expected:
   `CALL-0001`, `CALL-0003`, `CALL-0009`, `CALL-0012`.

10. Query:
    `Which call was roaming?`

    Expected:
    `CALL-0009`, source `923001112222`, target `923331111222`, area `Gulberg`.

## Demo Order

1. Create a new collection named `cdr-demo-seed`.
2. Upload `tests/fixtures/cdr/cdr_kb_seed.csv`.
3. Open the Search tab.
4. Run queries 1, 3, 5, 7, and 8 first; they demonstrate shortest call, target numbers, area filtering, failed status, and longest duration.
5. Compare the answer to the expected values above.

## Accuracy Note

For exact analytics over very large CDR batches, upload computed summary rows as well, or connect a structured CDR query tool. Knowledge Base search is best for retrieval and explanation; exact whole-dataset aggregation needs structured computation.
