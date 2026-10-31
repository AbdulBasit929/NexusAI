# A hub threshold per case, and statement repeats across files (migration 032)

Written 2026-10-31, on the owner's instruction to complete the case work. Status: built, specified, rehearsed on a restored copy of the live database and regression-checked on the lane (section 5). **Applied live on 2026-10-31 12:25 to about 12:28 UTC** on the owner's word, after the harness's auto-mode classifier had blocked the first attempt as a production deploy (record: `live-window-20261030.md`, section on migration 032).

## 1. Why

The first live smoke of the new families (`live-window-20261030.md`) found two weaknesses that are not engine mistakes but missing rules:

1. **"Numbers and services that everyone contacts."** Link analysis leaves out a number that is in contact with more than N distinct subscribers (a hub). The views have always read N from the setting `app.hub_threshold` (migration 017, 20 when unset), but the engine could only give it one value for every case, from one environment variable. A case with 12 subscribers and a case with 400 are not read at one threshold. Three of the smoke's wrong answers were this: the Orion case is designed with 10, the engine used 20.
2. **The same statement twice.** A bank statement is often supplied as the bank's CSV and as its PDF or Word rendition, or as two exports whose periods overlap. Loaded into one case, every transaction counted twice. The Orion renditions were kept in a collection of their own to avoid it.

## 2. What was decided and built

### 2.1 The hub threshold is a setting of the case

* `forensic.case_record.hub_threshold` (integer, 2 to 1000, `NULL` = no setting). Set with `PATCH /cases/{case_id}` and `{"hub_threshold": 10}`; `null` clears it; the case list and the single case show it. The audit row records that it changed and the value.
* **Order of precedence, every question:** the number the question itself names ("more than 12 subscribers", "hub threshold of 12") > the case's setting > the engine's setting for every case (`FORENSIC_ASK_V2_HUB_THRESHOLD`, unset today) > 20. A question that names two different numbers gets a question back; a number outside 2 to 1000 is refused with the bounds; a setting outside the bounds is ignored.
* The engine reads the setting once per question, only when the question's views use a hub (`v_contact_pairs`, `v_connection_edges`). The language model never sees or chooses it (it is a registry parameter, `hub_threshold`, set as a transaction setting by code).
* **The answer states it** whenever it was not the default: "Hub threshold used: 10 subscribers (the setting of this case, because the question names none)." or "(read from your question)".
* Not built yet: an input on the case page of the interface. Until then the setting is made through the API; the interface programme (U2 onward) owns the screen.

### 2.2 A repeat across files is counted once

`analyst.v_bank_transactions` leaves out a row that **another file of the same case already holds with the same account, time, direction, amount and balance after (null-safe)**. The row that stays is the one of the file with the smaller evidence id, so the result does not depend on the order of loading. Nothing is deleted: the repeat stays in `forensic.bank_transaction` with its source row. Repeats *within* a file were already flagged `duplicate_row` and left out; that is unchanged. The columns, their order and types are unchanged, so every view over it (`v_account_counterparties`, `v_account_activity`, `v_account_flows`, `v_account_limit_bands`) is untouched.

* **Limits, stated in the view's description and in the answer's limitation:** a different transaction with the same account, time, direction, amount and balance in another file cannot be told from a repeat and is treated as one; a statement without balances is compared on the other four fields only.
* The reference and the channel are not part of the key on purpose: a PDF rendition does not carry the bank's reference in the same form.

## 3. Files

| What | Where |
|---|---|
| Migration | `db/forensic_records/032_case_hub_threshold_and_statement_repeats.sql` |
| Preflight, verify, behaviour test (rolls back), rollback | `db/forensic_records/ops/032_case_hub_threshold_and_statement_repeats/` |
| Case record API | `api/forensic_records/case_records.go` |
| Engine reads the setting | `api/forensic_records/ask_engine_casesettings.go`, `ask_engine_run.go` |
| Parameter registry (`hub_threshold`) | `api/forensic_records/ask_engine_params.go` |
| Caveat for transactions | `api/forensic_records/ask_engine_caveats.go` |

## 4. Order of a live application

The migration is backward compatible with the engine that runs now (the old engine neither reads the column nor depends on the view's text), so: back up, apply and verify the migration, run the behaviour test, then deploy the engine, then set the Orion case to 10 and re-ask the three questions that fell to the threshold. Rollback: `ops/032.../rollback.sql` (refuses while a case holds a setting) and the engine's `before-` tag.

## 5. What was checked before any live change (2026-10-31)

* **Specifications:** request parsing (bounds, null, both fields), the parameter resolution in every order and every refusal, the settings read on a case database (set, read back, another tenant, cleared, the table's check), the run-level specs with the read stubbed. The full package suite passes (101 s) and the commit hook (lint, vet on host and linux, the package tests) passed on each commit.
* **Golden catalogue:** regenerated on a throwaway database; one line differs (the description of `v_bank_transactions`); all verified examples run as the read-only analyst role.
* **Rehearsal on a restored copy of the live database** (`tools/dev/live-window/rehearse-032.sh`; dump `live-20261031T103450Z`, 513,438,743 bytes, SHA-256 `2b62f7f900e9278f8cbacb6fdb9ee62a34ba74eb0dfe6ccf893bd270b22ff08f`, 6,522 entries; restore 0 errors, 0 row-count differences; the only catalog differences from the live baseline are two re-expressions by dump and restore: a CHECK printed with other brackets and an owner grant): preflight, apply and verify pass; the behaviour test passes (a repeat across files counted once: stored 628 to 633, the view stays 626; another amount is another transaction: 626 to 627; a repeat inside one file is left to the writer's rule: 626 to 628) and leaves nothing; applying again changes nothing; the 010 runtime-RLS verify passes; **the bank view returns the same number of rows before and after on all three Orion collections (626, 626, 135)**: none of the existing cases holds a repeat across files; the application role sets and clears the threshold and the table refuses 1; the rollback restores the catalog exactly (0 differences) and the migration applies again. The catalog gains 5 lines and loses 2 (the column, its comment, its check, the view and its description).
* **Regression on the three development banks with the engine at this code and migration 032 in their databases** (wordings that failed on fresh cases and on the live smoke swapped in, same oracle values): subscribers case 2: 18 correct, 0 wrong, 1 declined (a model slip, SELECT DISTINCT ordered by a column not selected, after three attempts); transactions case 2: 20 correct, 0 wrong, 1 abstained (TXN-015, the known honest abstention); documents seed 5100: 23 of 23. **0 wrong in 63 questions.** The case-126 wording of SUB-008 ("how many numbers made or received calls after their registration ended") went through COUNT_NOT_SUM and answered 3.

## 6. How it was applied (and how to apply it again elsewhere)

```
bash tools/dev/live-window/2b-migration-032.sh                                  # preflight, apply, verify, comparison; stops at the first failed check (a fresh backup exists: live-20261031T103450Z)
.\tools\dev\live-window\3-recreate-api.ps1 -Apply -To round7b-20261031 -RollbackTo before-round7-20261031
python -X utf8 tools/showcase/set_case_setting.py --case orion-lahore-reference-20261030 --hub-threshold 10 --apply
.\tools\dev\Restart-Workspace.ps1 -Port 4181 -Tree lane -Collection orion-lahore-reference-20261030
```

Images: `nexusai-forensic-records-api:round7b-20261031` (`3a07ed87bccf`, built from the lane at 77504ce: migration 032's engine, COUNT_NOT_SUM, and the retained-copy status fix, section 7) and its rollback `before-round7-20261031` (`dd2b07a0d75b`, the engine that runs now). Rollback of the engine: `3-recreate-api.ps1 -Rollback -RollbackTo before-round7-20261031`; of the migration: `ops/032.../rollback.sql` (it refuses while a case holds a setting: clear it first with `--clear-hub-threshold --apply`).

## 7. A false alarm found on the way: "retained copy is missing" (commit 77504ce, not deployed)

The workspace dashboard flagged 26 sources of the Orion case as needing review ("Retained copy is missing: processing finished but the retained copy of this file cannot be opened"). The originals are retained (`evidence_items.raw_storage_ref` is set for all 53, and the `.txt` original of the case came back HTTP 200 from `/evidence/{id}/content`). The flag came from a different store: a completed job with no row in `forensic.kb_collection_assets`, a row that only the generic row path writes (the document, `.txt` and `.xls` routes do not). 35 jobs were flagged in all, all in the three Orion collections; no other collection was. The status figure and list now count a job only when it has neither an asset row nor a retained original: 35 flagged before, 0 after, every other collection 0 before and after (checked read only on the live database). It takes effect with the API image above. Not changed: the asset-repair function, which exists to rebuild asset rows.

## 8. The whole query pack on the live engine (2026-10-31, after migration 032 and the engine of 12:26 UTC)

All 196 questions of `docs/qa/ui-query-pack-20261031` were put to the live API (`tools/showcase/ask_case.py`, the case's hub threshold at 10), judged by the project's judge (the 19 extras whose answers are computed by SQL on the stored rows by a plain comparison):

| Family | Correct | Not correct |
|---|---|---|
| Call records | 33 of 34 | Q081 (below) |
| Link analysis: connections (with the three setting questions) | 50 of 51 | SET-002 |
| Link analysis: co-location, pairs, successors, movement | 10 of 10, 7 of 7, 10 of 10, 10 of 10 | none |
| Internet sessions and app usage | 5 of 5 | none |
| Registrations and handsets | 16 of 16 run | 3 not run: they need the device reference list (shared by every case, not loaded) |
| Plates | 14 of 15 | ANPR-002, an honest decline |
| Documents | 23 of 24 | DOC-015, an honest decline |
| Bank and wallet transactions | 9 of 11 | TXN-008, TXN-009 |
| **All** | **187 of 196** | 4 wrong, 2 declined, 3 not run |

* **ORL-CAL-Q081** "Who owns the number 923000001001?": the judge's verdict (wrong) is kept; its oracle predates the registrations family and expects a decline. Read by a person: the answer puts the correction first, says that no registration of the number is in the files (true: it is one of the two unregistered numbers) and names no owner.
* **Three misses the pack found in the engine, fixed in commit ed44c58 (a specification each, both directions; nothing is live yet):** SET-002, a threshold named in the question ("in contact with more than 10 subscribers") was read as the threshold but did not make the engine leave the hubs out; TXN-008, "which accounts sent money to X" was answered with who X paid (X is the receiver: its credits); TXN-009, the model added `received_from IS NULL` to a pass-through question and the answer said that no credit was followed by a debit although three accounts qualify (a false absence): an IS NULL on the counterparty that the question does not ask for is refused with the reason.
* A fourth, TXN-006, was the comparison of the pack's builder (5,912,781.7 is 5,912,781.70) and is fixed there.
* **Regression of ed44c58** (throwaway databases with migration 032): transactions development case 2: 20 correct, 0 wrong, 1 abstained (TXN-015); call and internet records development seed 3: **46 of 46**; red team: **18 of 18**.
* **Correction to the commit message of a2897ee:** it says transactions 9 of 12; the pack holds 11 transaction questions (the three hub-setting questions are counted under connections): 9 of 11.

Pending, needs the owner's word: image `nexusai-forensic-records-api:round8-20261031` (`8ec2c1edd72a`, from the lane at ed44c58) and its rollback `before-round8-20261031` (`3a07ed87bccf`, the engine that runs now):

```
.\tools\dev\live-window\3-recreate-api.ps1 -Apply -To round8-20261031 -RollbackTo before-round8-20261031     (dry run first, without -Apply)
.\tools\dev\Restart-Workspace.ps1 -Port 4181 -Tree lane -Collection orion-lahore-reference-20261030
```

then re-ask SET-001/002/003, TXN-001 to 011 and the connection questions, and republish the sheet.

