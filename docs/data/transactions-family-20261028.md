# Bank and wallet transactions as a typed family (P1-B, fourth family): design, truth and gates

**Date:** 2026-10-28 · **Status:** design and gates written BEFORE any build or measurement · **Owner's go:** "go ahead with subscriber and device family, also the transactions" (2026-10-28) covers building and testing on **throwaway databases and generated files only**; nothing is applied live without a separate go.
Plan of record: `docs/roadmap/master-plan-20261021.md` P1-B ("access logs and transactions: no typed table exists"). Template: `docs/data/anpr-typed-family-20261026.md`. Companion: `docs/data/subscriber-device-family-20261028.md`. Order: this family is built after the subscriber and device family.

## 1. What this is, and what it is not

**Is:** account statement lines and mobile-wallet transaction lists (CSV or XLSX): one row per posted transaction (a time or a date, an account, a direction, an amount, optionally a balance after, a channel, a counterparty, a narration, a reference) read by the production typed ingest into a **new table `forensic.bank_transaction`** (migration 029) and answered through reviewed views. Questions it exists for: money in and out of an account over a period, the largest and the repeated amounts, who an account paid and who paid it, accounts that share a counterparty, **pass-through** (money in and a near-equal amount out within minutes), amounts that cluster just under a limit, and, for wallet rows, the link to the phone number that also appears in call records.

**Is not (stated so that nothing is claimed that is not built):**
* **PDF statements and scans.** Most real statements are PDFs: they go through the document route (P3, Docling and OCR); typed rows from a PDF are a later, separately measured step. This family reads tabular exports.
* **Accounts of real people, real IBANs, real banks' formats as fact.** No real record-level transaction data is lawfully published for Pakistan (section 2). The layouts here are **drafts** for generic exports; a real bank's layout needs a real sample read in the vault and the owner's confirmation (D9).
* **Evidence of a crime.** A pass-through or an amount under a limit is a **pattern in the rows**, stated as "candidate, not a finding", never "laundering" or "structuring" as a conclusion; the engine attaches the caveat.
* **Currency conversion, interest, fees analysis, and cheque images.** Amounts are stored as stated with their currency; nothing is converted.
* **Names of account holders.** A counterparty name in a narration is a string of the source row, not an identity; the identity layer stays dark (R9, see the companion document).

## 2. Sources of data: what was fetched, what could not be, and why

| Need | Result | Detail |
|---|---|---|
| Real Pakistani transaction rows | **Not available, and not fetched** | Only aggregates are published (State Bank of Pakistan payment-system reviews: Raast, IBFT, wallet volumes and values by quarter). Sample e-statements that circulate on document-sharing sites are **real people's financial records, uploaded without a licence: never used.** |
| Shape and scale of mobile-money transactions | **Dataset card fetched; data not downloaded** | `flwrlabs/fed-fraud-paysim-banks` (Hugging Face, **CC-BY-4.0**, synthetic, PaySim-derived): 6.36 million rows in two files of 124 MB, 124 MB and a 28 MB test file; fields `step` (hour of a simulated month), `type` (payment, transfer, cash-out, cash-in, debit), `amount`, origin and destination names with old and new balances, a fraud label, a bank id. Card saved to `C:/tmp/nexusai-family-data/paysim/` (`tools/family_data/fetch.py transactions_docs`). It is **not Pakistani and has no real time or reference**, so it informs the amount and balance shape and the account-balance reconciliation rule, not the layout; the 275 MB are not needed and are not downloaded. |
| Pakistani channel vocabulary | Read, not fetched | The rails (IBFT, Raast, 1LINK, wallet transfers, ATM, POS, cheque, cash) are described in the State Bank's published material and explainers; the channel list of the layout comes from them. |

So this family is tested on **generated synthetic statements** whose account identifiers come from reserved ranges (section 4), calibrated to the public aggregate shape (the proportion of channels is a generator parameter, not a measured fact).

## 3. The pieces, in the order they are built

| # | Piece | Where | Proof, without a model |
|---|---|---|---|
| a | **Account and amount normalisation**: account number and IBAN (spaces and dashes removed, upper case, IBAN mod-97 check recorded as a flag, never rejected), amounts with thousands separators, brackets or a minus for negatives, `Dr`/`Cr` suffixes, currency default PKR with the source's word kept | `parties.py` (`normalize_account`), `interpret.py` | equal to the SQL function on a fixture, zero differences |
| b | **Layouts and the interpreter** for `dataset_kind = bank_transaction`: two draft layouts, `bank-stmt-t1` (date, value date, narration, reference, debit, credit, balance) and `wallet-t1` (id, date time, type, amount, fee, balance, counterparty number, status); a debit/credit pair of columns or a signed amount column; **time precision**: a date without a time is stored at local midnight with `time_precision = 'date'` | layouts, `interpret.py`, `spec.py` | golden file per layout |
| c | **Migration 029**: `forensic.bank_transaction` (identity columns as `plate_sighting`: file, batch, row hash, layout, evidence; `txn_utc`, `time_basis`, `time_precision`, `account_norm`, `counterparty_account_norm`, `counterparty_msisdn`, `direction`, `amount` numeric(18,2), `currency`, `balance_after`, `channel`, `reference`, `narration`, `status`, `quality_flags`), tenant row-level security, privileges as the other families | `db/forensic_records/029_bank_transactions.sql`, `ops/029_*` | rehearsed on a throwaway database |
| d | **The typed writer** writes it idempotently; **running-balance reconciliation** at ingest: where a balance column exists, `previous balance + credit - debit = balance` per account in file order; a break is flagged `balance_break` (the row is kept), a gap is flagged `balance_gap` | `typed_ingest.py` | second ingest writes 0 rows |
| e | **Views** (in 029): `analyst.v_bank_transactions` (effective rows: exact repeats dropped, signed amount), `analyst.v_account_counterparties` (per account and counterparty and direction: count, total, first, last), `analyst.v_account_flows` (pass-through candidates: a credit and a later debit of at least a given share of it within a given window; **the share and the window are runtime parameters**, rule of 2026-10-22), `analyst.v_account_activity` (per account and local date: credits, debits, count, closing balance); comments and grants | 029 | equal to the oracle |
| f | **A generated statement file per case** (and a wallet file), from the same seed, **outside the default case build**, with planted patterns and an oracle | `tools/case_generator/txn_case.py` | the oracle answers from the planted records |
| g | **The engine**: catalogue entries, retrieval words, verified examples, accounts and amounts as identifiers and obligations (an amount of the question must appear in the query, as a number must), the **time-precision guard** (a question with a window shorter than a day is answered only from rows whose precision is a second; otherwise the answer says the rows carry dates only), caveats, probe columns | `ask_engine_*.go` | Go specs |
| h | **The bank and the judge** (kinds `money` and `flow`) and one measurement on a fresh judgement case | `tools/ask_eval/txn_bank.py`, `judge.py` | G4 |

## 4. The truth: planted patterns and their oracle

Synthetic accounts use a reserved form: **`PK00SYNT` + 16 digits** (the IBAN check digits `00` are invalid, so no real IBAN can be this), account numbers `SYNT-nnnnnn`; wallet rows use the synthetic mobile numbers of the world. `tools/real_data/reserved.py` gets `synthetic_account(n)` and `synthetic_iban(n)`; the repository scanner learns the form.

* **T1 shared counterparties:** pairs of subjects who both paid or were paid by the same third account (and decoys: a utility account that everyone pays, a hub that pays many).
* **T2 pass-through:** a credit followed within minutes by a debit of 90 to 99% of it, planted with known times, and look-alikes (same amount an hour later, a different account).
* **T3 amounts just under a limit:** a series of debits at 95 to 99% of a stated limit within a window, and a control series well under it.
* **T4 running balance:** every account's balance column is correct, except a planted break (one row) and a planted gap (rows removed); duplicated rows are planted.
* **T5 periodic credits:** a salary-like credit on a day each month.
* **T6 wallet link:** wallet transfers to numbers that also appear in the call records, so a question can cross the families through the number.
* **T7 format defects:** amounts as `1,234.50`, `(1,234.50)`, `1234.50 Dr`; dates in three formats; a date-only statement for one account and a date-time wallet list for another; an unknown channel word; a missing reference.

The oracle answers: totals in and out per account and period; the largest and the n-th largest transaction; the counterparties of an account with count and total; the accounts that share a counterparty; the pass-through pairs for a given share and window; the debits within a band of a limit; the closing balance on a date; the transactions on the day of a given call.

## 5. Gates (written now; applied to the runs named; nothing is tuned on a case that is then rerun)

* **G1 ingestion reconciliation** (development cases 1 to 8, generated with transactions): rows accepted equal the oracle's unique rows; per account the sum of credits and of debits and the closing balance equal the oracle's; flags equal the planted ones (`duplicate_row`, `balance_break`, `balance_gap`, `date_only`, `iban_check`) and no other; a second ingest writes nothing.
* **G2 normalisation parity:** Python equals the SQL function on a fixture of account and amount tokens: zero differences.
* **G3 the views equal the oracle** on the development cases: every oracle query of section 4 at **three parameter values** (the pass-through share and window, the limit band), zero differences; scope fails closed.
* **G4 the engine** on a fresh judgement case (119; a second, 122, only if G4 fails once and the cause is fixed): at least 90% correct, **zero wrong**, the rest abstained or declined with a stated reason; no answer says laundering, structuring, fraud or guilt; every pattern answer carries the caveat; a window shorter than a day over date-only rows is declined with the reason, never answered.
* **G5 safety:** repository scanner passes; no real-looking account or IBAN in any committed file; no model prompt, audit row or log of the throwaway run contains an account value that is not in the question or the result; the migration applies, verifies, rolls back and applies again.
* **G6 nothing live** until the owner's separate go; the live activation is combined with the other families.

Latency is reported, not a gate.

## 6. Order and decisions

Order: a, b (no database) → c, e, f (migration, views, generator and oracle on a throwaway database) → d → G1, G2, G3 → g → h and G4 → G5 → report. The register gains judgement cases 121 to 128 (additive, a new set version) so that neither family reuses a case.

**Decisions owed (none blocks the build):** D9 real samples (a real bank or wallet export under authority makes the drafts into layouts); the wording policy for patterns ("candidate, not a finding") is proposed here and goes to the owner with the first answers; PDF statements are scheduled with the document route, not here.

## 7. Status

(filled as the work proceeds)
