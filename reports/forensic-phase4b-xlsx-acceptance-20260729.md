# Phase 4B acceptance — bounded read-only XLSX evidence

## Outcome

Phase 4B is complete in source. `.xlsx` workbooks can now be registered, queued,
profiled and normalized without executing workbook content. Rows retain exact
source values plus worksheet, source-row and cell-level provenance, enabling
ordinary deterministic workbook queries while active spreadsheet behavior stays
blocked.

## Acceptance record

1. **Objective:** operationalize real-world XLSX evidence with bounded parsing,
   exact value preservation and stable source locators.
2. **Assumptions verified:** OOXML is an untrusted ZIP/XML package; formula text,
   cached results, hidden content and links are evidence rather than executable
   instructions; mixed-sheet workbooks default to the generic adapter unless a
   typed record family is explicitly requested.
3. **Files changed:** added the standard-library XLSX reader, worker routing,
   Pakistan-oriented versioned goldens, API classification/capability changes,
   modality matrix updates, tests and operator documentation.
4. **Schema/API/config:** `.xlsx` now queues to the existing records worker;
   no endpoint or database schema was added and no active configuration changed.
5. **Tests:** 8 focused XLSX tests passed in 0.249 seconds; the combined Python
   worker/Phase 2/XLSX regression passed 41/41 in 1.277 seconds; the complete
   `api/forensic_records` Go package passed in 4.303 seconds. Python compilation
   and all three changed JSON contracts passed.
6. **Live runtime:** no image rebuild, service recreate/restart, migration,
   retained database write, queue activation or live upload was performed.
7. **Dataset/evidence:** deterministic synthetic PKR/Urdu XLSX only: two sheets,
   four emitted data rows, one hidden sheet, one hidden row, one hidden column,
   one merged range, two formulas, one error cell and one inert external link.
8. **Models:** the existing Qwen chat/embedding baseline was unchanged; no model
   was called, downloaded, installed or promoted.
9. **Accuracy/retrieval:** golden assertions preserve `00012345`,
   `001250.5000`, `PKR`, Urdu text, 1904 date conversion, raw date serial,
   formula text, cached value and error state exactly; full workbook inventory
   continues beyond capped samples.
10. **Latency/memory:** no live latency/RAM claim was made. ZIP entries, total
    uncompressed bytes, part size, compression ratio, sheet/string/row/column
    counts and profile samples are bounded; row emission is streaming per sheet.
11. **Failures/fallbacks:** macro payload, DTD/entity XML, traversal member,
    invalid/corrupt ZIP, invalid shared-string reference and Excel's fictitious 1900 leap day are
    rejected or explicitly review-flagged. Formula calculation, macro execution
    and external-link following are always false.
12. **Security/provenance:** source bytes remain unchanged; raw/cached/formula
    representations coexist; hidden state and sheet/row/cell locators survive
    normalization; unsafe active content fails closed.
13. **Git:** worktree remains unstaged, uncommitted and unpushed; no GitHub or PR
    action occurred.
14. **Rollback:** source-only edits can be reviewed or reverted before a future
    activation; retained services and data require no rollback from this slice.
15. **Next action/approval:** Phase 4C should deepen exact cross-family queries
    and per-sheet typed mapping, followed by bounded ODS/columnar evaluation.
    Rebuild/deployment, migrations, model changes, real sensitive ingest and Git
    publication remain separate approval gates.

## Query truth boundary

Accepted: read XLSX rows, inspect workbook/sheet inventory, filter normalized
records, show hidden-state markers, and display stored formula text/cached values.

Unavailable: evaluate or recalculate formulas, execute macros, follow external
links, decrypt workbooks, or treat `.xls`, `.xlsm`, ODS and unaccepted columnar
formats as operational.
