# A hub threshold per case, and statement repeats across files (migration 032)

Written 2026-10-31, on the owner's instruction to complete the case work. Status: built and specified on the lane; rehearsal and live application are recorded at the end when done.

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
