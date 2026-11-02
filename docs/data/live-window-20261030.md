# The live window of 2026-10-30: migrations 027 to 031, the engine and the worker for the new families

Authority: the owner said "go with your best recommended safe and accurate steps ... implement" and, after the harness blocked the migration step as a production deploy, "i allow you to do it". Rehearsals and the procedure
are in `migrations-027-031-runbook.md`; the scripts are in `tools/dev/live-window/`.

## What was applied, in order (UTC)

| Time | Step | Result |
|---|---|---|
| 07:10 | baseline and backup of the live database | `live-20261030T071046Z.dump`, 456,579,451 bytes, SHA-256 `95c74769de61114acf53447db4bc0722ec77220a67073398c576180e467a2b21`, 4,964 entries, 1,855 catalog lines, 39 tables counted |
| 08:54 to 08:55 | migrations 027, 028, 029, 030, 031: preflight, apply, verify each; the 010 runtime-RLS verify; the four layouts | all passed in 13 s; **407 catalog lines added, 0 removed, 3 changed** (`time_basis` may be empty with its check; the layout kind list accepts `bank_transaction`), exactly as rehearsed; the only row-count changes are the two new tables (0 rows) and `layout_versions` 21 to 25 (`anpr-a1`, `subscriber-s1`, `bank-stmt-t1`, `wallet-t1`) |
| 08:55 | the API recreated on the engine image `c56d4841be3b` (rollback tag `before-families-20261030` = `4eebc53ef7f2`) | compose's config hash equalled the running label before; environment 69 variables before and after, the engine switches unchanged; logs clean; the plan cache restored (509 entries) |
| 08:56 | the worker recreated on the first overlay image `71e370c09b4d` | healthy, 0 errors |
| 09:11 | the worker recreated on the fixed overlay image `47e5e829a3d4` (rollback `before-families-20261030` = `037b7b720365`, the image of before) | healthy, 0 errors, switches `FORENSIC_INGEST_V2=shadow`, `FORENSIC_DOCUMENT_MENTIONS=true`, `FORENSIC_DOCUMENT_GAZETTEER=/app/gazetteers/lahore.json` |

Nothing else was changed: the other containers, the interface, the existing collections. No UI server was running on :4181; the new design on :4182 was not touched.

## The first live load, and a finding the rehearsal could not show

The Orion Lahore case (synthetic) was uploaded through the real upload endpoint, declaring the zone Asia/Karachi and the day-first date order (`tools/showcase/upload_case.py`).

* **`orion-lahore-20261030`** (53 files): every job completed. Calls, internet records, app usage, camera sightings, registrations and bank transactions loaded with only the planted defects quarantined. **Finding:** the ten plain-text documents were stored as generic rows with no passages and no mention index (25 mentions, all from the four Word documents, instead of the 69 of the rehearsal). The live API sends a plain `.txt` document with the modality `structured_records` and the route `native_document_worker`; the running worker's old rule (the route AND a modality of document or text) sent it to the generic row adapter. The repository's worker already decided by the route alone. The rehearsal had guessed the modality of a `.txt` document and missed it; `check_worker_overlay.py` now registers each file with the modality and route the live API really gives it, fails on the image without the fix with exactly the live numbers, and passes on the fixed image (`8e73620`). The collection is kept as the record of the finding.
* **`orion-lahore-reference-20261030`** (53 files, on the fixed overlay): comm_event 5,497, data_session 3,825, app_usage 657, plate_sighting 295, subscriber_registration 106, bank_transaction 628, all 14 documents routed to the document pipeline and indexed (the control document names nothing, as designed), mentions by kind account 5, iccid 4, imei 4, imsi 2, msisdn 27, national_id 7, place 13, plate 7: the same as the rehearsal. A second upload of the same files queued nothing (idempotent by content hash).
* **`orion-lahore-statements-20261030`** (3 files): the PDF and Word renditions of statements, in a collection of their own because they repeat accounts that the CSV statements hold. Read through the live path they equal the CSV rows of their accounts exactly (46 of 46, 36 of 36 and 53 of 53 rows; time, direction, amount, balance; nothing different on either side).

Not loaded: the device reference list (`device-reference.csv`) is tenant-wide reference data and not an upload of the case; until a list is loaded, handset models are "not listed", and the three handset questions were left out of the smoke.

## The live smoke: 83 oracle questions put to the live Ask engine

Sample of the 177 questions of the case catalogue (`tools/showcase/ask_case.py`, scored by the same judge as the offline runs; not pre-registered and not gate evidence):
documents 24, registrations 15, plates 15, calls 12 (one of each kind), and one of each kind of the co-location, pair, successor and movement analyses 17.

| Family | Questions | Correct | Wrong | Declined | Latency p50 |
|---|---|---|---|---|---|
| Documents | 24 | 22 | 1 | 1 | 23 s |
| Registrations and devices | 15 | 13 | 2 | 0 | 47 s |
| Camera sightings | 15 | 14 | 0 | 1 | 30 s |
| Call records | 12 | 9 | 3 | 0 | 53 s |
| Co-location, pairs, successors, movement | 17 | 17 | 0 | 0 | 45 s |
| **All** | **83** | **75 (90.4%)** | **6** | **2** | |

The refusals that the plan wants are counted correct: the national ID number withheld, the questions that ask for a verdict about a person.

**The three call-record misses are a configuration difference, verified.** The questions say "leaving out services and numbers that everyone contacts". The case's oracle defines that with a hub threshold of 10; the live engine has no threshold set and the views default to 20. The SQL the live engine wrote for the three questions, re-run read only under the analyst role at a threshold of 10, returns exactly the expected sets (common contacts: only 923000001050; path: through 923000001050; reach: the eleven expected numbers). With the case's threshold, the sample is 78 of 83. A hub threshold per case, stated in the answer, is a design item (below).

**The other three wrong answers and two declines are engine findings**, to be fixed by the same discipline as the offline rounds (cause, generic fix, development-case check, regression, then deploy):

1. **ORL-REG-SUB-002**, "What numbers does IMSI X appear with in the registrations?": the model joined the registrations to the device pairings on nonsense (an IMEI list holding the IMSI), got nothing, and the engine said "no registration on the SIM identity X was found in the files of this case" although the registrations hold it. The absence sentence is the family's standard one and is false when the identifier IS in the compared column of a view the query reads. Fix: an empty result for an identifier that the probe finds in a compared column is not an absence; ask the model again with the place it occurs in, and abstain on the last attempt.
2. **ORL-REG-SUB-008**, "How many numbers were still in use after their registration ended?": answered 5 from `count(*) FROM v_subscriber_registrations WHERE status = 'deactivated'`, which counts registrations, not numbers in use; expected 3. The obligation that asks for `events_after_last_end` only fires when the query reads the activity view; the model avoided that view. Fix: for such a question the query must read events (the activity view or the call events).
3. **ORL-DOC-DOC-013**, "documents that mention both PK00 SYNT 0000 0000 0000 2010 and synt 4024": the lower-case plate after "and" was not read as a plate (the cue words of the plate reader stop at without, except, also, not, than), the model compared a plate with `kind = 'msisdn'` and the value `'synt 4024'`, and the answer was a confident "no document names the account" for an account that the document does name. Fix: read the lower-case plate after "and"/"both" (with the stop list), and extend the kind-and-value check to values that are not normalised.
4. **Declines** (honest, not wrong): ORL-DOC-DOC-015 ("where does 0300... appear without SYNT2019", the same reading gap as 3) and ORL-PLA-ANPR-002 ("how often was SYNT2083 seen", three attempts, the model judged the views cannot answer): both fell to the existing path, which said it could not tell.

## What this means

* The new families are LIVE in the engine and the worker: new uploads of plates, registrations, statements (CSV, PDF, Word), wallets and documents are typed and indexed; the engine answers about them. The engine for the call and internet-record families that were live before did not regress on its own bank (46 of 46) or the red team (18 of 18) before the window.
* The live smoke found three engine weaknesses (above) in a first look at a bigger case than the development cases; none is hidden. They are the next round of fixes; until then the false absence sentence of finding 1 is the one to watch.
* Open items for the owner: a hub threshold per case (design), a device reference list to load (which list), the cross-file duplicate rule for statement renditions (design), the subscribers gate (a new fresh case), and whether to start the interface on :4181 to show these collections (nothing was serving :4181).

## Round 6: the three engine findings fixed (commit 49a658d), running live since 2026-10-31 08:35 UTC

The cause, fix and check of each finding above, by the discipline of the offline rounds:

| Finding | Fix | Check |
|---|---|---|
| ORL-REG-SUB-002, false "no registration found" after a join on a nonsense key | an empty result of a registration or handset question whose query also reads a view outside the family is a wrong place, not an absence: the model is told where the identifier occurs and to answer from that view alone, and the last attempt abstains with its own reason (`askReadEmpty`, `askViewsOutside`) | spec on the verdict; re-asked on development case 2 with the live wording: answered correctly |
| ORL-REG-SUB-008, 5 registrations counted where 3 numbers were right | the obligation REGISTRATION_END also fires for a query that reads `v_subscriber_registrations` and no other view | spec; re-asked: the obligation fired once, then 3 (correct) |
| ORL-DOC-DOC-013, lower-case plate after "and"/"both" unread | `askPlateAndCue` with a naming verb earlier in the question; `askKindOfValue` compacts separators, so `'synt 4024'` with kind `msisdn` is refused | spec; re-asked: documents seed 5100 4 of 4 |

Declines left as they are: ORL-DOC-DOC-015 (the question says "where does X appear" and names no document, so the document views are not shortlisted) and ORL-PLA-ANPR-002 (the model judged the views cannot count sightings).

* Full package suite passes (122.9 s); lint, vet and the commit hook pass.
* Development banks, with the wordings that failed live swapped in (same oracle values): subscribers case 2 **19 of 19**, transactions case 2 **20 of 21** (TXN-015, the known honest abstention), documents seed 5100 **23 of 23**, 0 wrong (round 5: 17/19, 18/21, 19/23). These are development cases and the same author wrote the fixes: regression evidence, not gate evidence. The targeted re-ask of SUB-009 once declined (its last attempt was a SELECT DISTINCT ordered by a column it did not select) and answered in the full run.
* Image `nexusai-forensic-records-api:round6-20261030` = `dd2b07a0d75b`, built from 49a658d; the image that ran before (`c56d4841be3b`) is tagged `before-round6-20261030`. A first build failed on a DNS drop inside Docker (the laptop's clock jumped a day) and was repeated.
* The redeploy script takes `-To` and `-RollbackTo` now. Its dry run on 2026-10-31: compose's config hash equals the running container's label (`74eb879ac9e9`), the rendered environment adds and changes nothing.
* **Applied** on 2026-10-31 at 08:35 UTC with the owner's word ("apply it and complete it"), after the harness's auto-mode classifier had blocked the first attempt as a production deploy: `3-recreate-api.ps1 -Apply -To round6-20261030 -RollbackTo before-round6-20261030`. Only the API container was recreated: image `c56d4841be3b` -> `dd2b07a0d75b`, the environment 69 -> 69 variables with nothing added, removed or changed, healthy at once, the plan cache restored (509 entries), no error in the logs. Nothing serves :4181 (no interface proxy to restart). The compose orphan warning (`nexusai-api-1-prev-gbnf`) printed again and is harmless.
* Rollback: `.\tools\dev\live-window\3-recreate-api.ps1 -Rollback -RollbackTo before-round6-20261030` (the image that ran before, `c56d4841be3b`).

### The live smoke again, on the round 6 engine (2026-10-31)

The three families that held the findings, the same questions as the first smoke, put to the live API (`tools/showcase/ask_case.py`, files `r6_*` beside the first results; scored by the same judge; not pre-registered, not gate evidence):

| Family | Questions | Correct | Wrong | Declined | First smoke (correct / wrong / declined) |
|---|---|---|---|---|---|
| Registrations and devices | 15 | 15 | 0 | 0 | 13 / 2 / 0 |
| Documents | 24 | 23 | 0 | 1 | 22 / 1 / 1 |
| Camera sightings | 15 | 14 | 0 | 1 | 14 / 0 / 1 |
| **All** | **54** | **52** | **0** | **2** | 49 / 3 / 2 |

* **SUB-002** repaired itself: the first query was an empty join, the new check named the wrong place (`RESULT:EMPTY_WRONG_COLUMN`) and the second attempt read `v_subscriber_registrations` alone and answered 923000001097 (correct).
* **SUB-008** answered 3 (correct) after the obligation `REGISTRATION_END` made the model read `events_after_last_end` of `v_number_registration_activity`.
* **DOC-013** answered `D01-fir-english.docx` (correct) at the first attempt, with the plate compared as `SYNT4024`.
* The two declines are the two left as they are: DOC-015 ("where does X appear without Y", no document named) and ANPR-002 ("how often was SYNT2083 seen"). The refusals the plan wants (the national ID number withheld, the verdict questions) are counted correct, as before.
* The call-record and analysis families (29 of the 83) were not re-asked live and the call-record bank was not re-run offline in this round. The code changed only for queries that read the registration, handset or document views (the empty-result check, the REGISTRATION_END obligation), for the plate reader and for the kind-of-value check; the full package suite and the three development banks (subscribers, transactions, documents: 62 of 63 correct, 0 wrong) passed. The three hub-threshold misses of the first smoke are a configuration difference, not an engine finding, and are unchanged.

## Migration 032 and the round 7 engine, applied (2026-10-31, on the owner's word "apply migration 032 and the engine")

Design, rehearsal and regression are in `case-setting-and-statement-repeats-20261031.md`. Backup of the live database before: `live-20261031T103450Z.dump` (513,438,743 bytes, SHA-256 `2b62f7f900e9278f8cbacb6fdb9ee62a34ba74eb0dfe6ccf893bd270b22ff08f`). The harness's auto-mode classifier had blocked the first attempt as a production deploy; the owner's word followed in chat.

| Time (UTC) | Step | Result |
|---|---|---|
| 12:25 | no ingest job in flight; `2b-migration-032.sh`: preflight, apply, verify, the 010 runtime-RLS verify | all passed in 9 s; catalog **3 added, 0 removed, 2 changed**, exactly as rehearsed (the column `hub_threshold`, its check, its description; the view `v_bank_transactions` and its description); row counts equal except the audit log (63 rows more, the questions asked since the baseline); the bank view as the analyst role returns 626, 626 and 135 rows on the three Orion collections, as before |
| 12:26 | the API recreated on image `3a07ed87bccf` (`round7b-20261031`; rollback tag `before-round7-20261031` = `dd2b07a0d75b`) | compose's config hash equalled the running label (`74eb879ac9e9`); environment 69 variables before and after, nothing added, removed or changed; healthy at once; the plan cache restored (509 entries); no error in the logs |
| 12:26 | the Orion reference case given `hub_threshold` = 10 (`set_case_setting.py --apply`, PATCH /cases) | the case record shows 10; one audit row |
| about 12:28 | the workspace on :4181 restarted (`Restart-Workspace.ps1`; its proxy now reaches the API by 127.0.0.1) | the proxy reaches the API: HTTP 200 |

**Checks on the live system after the change**

* The collection status of the three Orion collections reports **0 sources with a missing retained copy** (it was 21, 3 and 11); the dashboard now shows 2 sources needing review in all (the two genuine failures of other cases), and both Orion cases read "Ready" (53 of 53, 3 of 3).
* The three call questions that fell to the hub threshold in the first smoke (common contacts, path, reach: ORL-CAL-Q019, Q054, Q066) are **3 of 3 correct** with the case's threshold of 10, and each answer says "Hub threshold used: 10 subscribers (the setting of this case, because the question names none)."
* The whole UI query pack is being put to the live engine (results in `docs/qa/`).

Rollback: the engine `.\tools\dev\live-window\3-recreate-api.ps1 -Rollback -RollbackTo before-round7-20261031`; the migration `ops/032.../rollback.sql` (it refuses while a case holds a setting: `set_case_setting.py --clear-hub-threshold --apply` first).

## Round 8 engine: three findings of the live query pack fixed (deployed 2026-10-31 16:07 UTC, on the owner's word "deploy round 8")

The whole UI query pack had been put to the live engine of 12:26 UTC: 187 of 196 correct, 4 wrong, 2 honest declines, 3 not run (device list). Three of the wrong answers were engine misses (SET-002 hub cue, TXN-008 direction of "sent money to", TXN-009 an invented `IS NULL` filter), fixed in commit ed44c58 with a specification each; regression on throwaway databases: transactions 20 correct, 0 wrong, 1 abstained; call and internet records 46 of 46; red team 18 of 18 (`case-setting-and-statement-repeats-20261031.md`, section 8).

* Dry run: compose's config hash equalled the running label (`74eb879ac9e9`), nothing to add or change. Apply: only the API container, image `3a07ed87bccf` -> `8ec2c1edd72a` (`round8-20261031`; rollback tag `before-round8-20261031` = `3a07ed87bccf`), environment 69 -> 69 variables with nothing changed, healthy at once, the plan cache restored (509 entries), 0 errors in the logs; the Orion case still has `hub_threshold` 10; the workspace on :4181 restarted and its proxy reaches the API.
* Rollback: `.\tools\dev\live-window\3-recreate-api.ps1 -Rollback -RollbackTo before-round8-20261031`.
* **Checked live after the deploy** (the affected questions re-asked): the 48 connection questions of the catalogue **48 of 48 correct**; SET-002 now answers 923000001050 only (the obligation HUBS fired); TXN-009 answers with the pass-through pairs (INVENTED_FILTER fired); SET-001 and SET-003 as before (the question's 20 gives two common contacts; a threshold of 1 is refused with the bounds 2 to 1000). **TXN-008 ("which accounts sent money to X") is now an honest decline of the engine** (it was a wrong answer); the older fallback path then says "No record in this case contains 0000000000002003", which is wrong for that account. A verified example for that wording is committed (f913cf9) and waits for the next engine image.
* **The whole pack, on the engines of this day: 189 of 196 correct, 1 wrong (Q081, reviewed by a person, the judge's verdict kept), 3 honest declines (ANPR-002, DOC-015, TXN-008), 3 not run (they need the device reference list).**

## Round 9b: the rows behind a figure, the model's reading, voice-call examples (checked on a restored copy; DEPLOYED 2026-11-02 12:29 PKT / 07:29 UTC on the owner's word "deploy round 9b")

Image `nexusai-forensic-records-api:round9b-20261102` = `442808025dce`, built from a459d11 (the image that runs, round 8, is `8ec2c1edd72a`, tagged `before-round9-20261031`; rollback is the same recipe as round 8: `3-recreate-api.ps1 -Rollback -RollbackTo before-round9-20261031`). It holds: the verified example for "sent money to X" (TXN-008); four voice-call examples (EXT-006, -007, -009, -012); part A, the rows behind a figure (`ask_engine_evidence.go`); part B, the model's reading (`ask_engine_reading.go`, route `GET /query/reading`); the fix of a false parameter note (`ask_engine_params.go`). No migration. The workspace on :4181 needs its new components (`AnswerBasis.jsx`) and a restart through `Restart-Workspace.ps1`.

* **Rehearsal on a restored copy of the live database** (`rehearse-evidence.sh`, dump of 2026-10-31 10:34 UTC, restored with 0 errors, plus migration 032 on the copy only): 9 of 9 real Orion cases recompute the figure from the rows of the files (3,871 voice calls in 12 files give the 1,800 s maximum and the 105.3568 average, 197 at night, 19 over ten minutes, 520 outgoing calls of 923000001005, five longest calls, 5,092 events) and 100 percent of the listed rows are found in their sheet by content (a typed row's own count is not the sheet's row: the Orion files are off by 4). The row a link opens shows "Exact source location: Row 93" and its row hash in the evidence viewer.
* **Staging engine with the real local model** (`run-staging-api.ps1`: this checkout's binary against the restored copy, its own message bus, the live API untouched): all 7 first questions correct; the 12 pack questions ORL-EXT-001..012 **12 of 12 correct** (the live engine: 9, with 007, 009 and 012 wrong or incomplete); five rewordings of the same questions **4 of 5 correct** (the fifth, "most recent voice call involving X", is an honest decline of the engine and the older path then guesses). Caveat: the four examples were written for these questions, so 12 of 12 shows the mechanism, the rewordings (4 of 5) are the better measure of how far it reaches; a general guard against an invented date filter is still to be built.
* **Readings with the real model**: 6 of 7 arrived in 43 to 57 s, accurate and in plain words; two of the first three had timed out at 60 s (the limit is now 120 s); one ran out of room (now asked again, shorter); one named an internal view (now refused by a check). The figure never waits for the reading.
* **Found in the interface by this check, fixed**: a false note "Co-location window used: 10 minutes" under a question about the length of calls (a note about a parameter is now written only when the query that ran reads a view it governs).
* **Not done**: summary views (top contacts, common contacts, paths) say their records are not itemised yet (part A2: trace them back to the call records); the sentence of a ranking is still "5 rows." (the reading and the first row carry the answer); the capabilities read of the workspace still returns 503 on the Orion case (a two-second budget; found 2026-11-02, not fixed).
* **Deployed**: dry run (compose's hash equalled the running label `74eb879ac9e9`, nothing to add or differ), `3-recreate-api.ps1 -Apply -To round9b-20261102 -RollbackTo before-round9-20261031`: only the API container, image `8ec2c1edd72a` -> `442808025dce`, environment 69 -> 69 variables with nothing changed, 0 restarts, healthy at once, the plan cache restored (509 entries), 0 errors in the logs (the orphan-container warning is the model server: never use --remove-orphans); the workspace on :4181 restarted (`Restart-Workspace.ps1`) and its proxy reaches the API. **Live after the deploy: the 12 pack questions ORL-EXT-001..012 12 of 12 correct** (before: 9). Rollback: `.\tools\dev\live-window\3-recreate-api.ps1 -Rollback -RollbackTo before-round9-20261031`. No fresh backup was taken (no migration, the engine only reads; the dump of 2026-10-31 stays).
* **What an analyst sees, found by looking at it (same day)**: the Open link of a row landed on the stored-records grid with system columns (batch id, file id, record id, raw payload, row hash) and did not show the cited row. Built for round 9c (not deployed): the evidence table of an answer has plain headings (When, Kind, Direction, Subscriber, Other party, Duration (seconds)) and the file and row in words; `GET /evidence/{id}/source-row` returns the cited row under the file's own column names (MSISDN, STRT_TM, BNUMBER ...), the page shows it as "This row in your file" with the hash under Technical details and the system columns of the grid behind a button.
* **Deploy recipe (as used)**: backup is the dump of 2026-10-31 plus a fresh `1-backup.sh`; `3-recreate-api.ps1 -Apply -To round9b-20261102 -RollbackTo before-round9-20261031` after the dry run; restart the workspace; re-ask the 12 pack questions and the four changed ones live; republish the sheet.

## Round 9c: evidence for an analyst, and the records behind every summary view (built and checked on a restored copy of the live data 2026-11-02; NOT deployed, waits for the owner's word)

Image `nexusai-forensic-records-api:round9c-20261102` (commits 6ba762d, 701f51a, 373b614 and the note edit after them; the engine that runs is round 9b `442808025dce`, rollback tag `before-round9-20261031` = round 8 `8ec2c1edd72a` still holds; to roll round 9c back to round 9b, tag the running image first: `docker tag nexusai-forensic-records-api:round9b-20261102 nexusai-forensic-records-api:before-round9c-20261102`). No migration. The workspace on :4181 needs its new components (`AnswerBasis.jsx`, `SourceRowCard.jsx`, the evidence page, `EvidenceViewers.jsx`) and a restart through `Restart-Workspace.ps1`.

* **What an analyst sees (found by looking at round 9b)**: the evidence of an answer read like engine output and the Open link landed on a grid of system columns. Now: a plain table (When, Kind, Direction, Subscriber, Other party, Duration (seconds) for calls; the headings of bank, internet, plate, registration and document records are their own), the file and the row in words, "Open in file"; the page that opens shows the cited row under the file's own column names in the file's own order (MSISDN, CALL_TYPE, STRT_TM, BNUMBER ...), the hash under Technical details, the system columns of the stored-records grid behind a button. `GET /evidence/{id}/source-row`: the typed row whose cells all occur in the stored row; a wrong hash and a title row are refused.
* **Part A2: the records behind every summary view**: `ask_engine_trace.go` holds, for each of the 27 summary views, which records stand behind one line (raw view + condition) and which numbers of the line they must give back; a spec holds it against the catalogue (a view that is neither a record view, nor traced, nor declared untraceable fails the build). `ask_engine_evidence_lines.go`: the lines of an answer over one summary view or an inner join of them (the common contacts of two numbers are traced to the records of both sides); the first lines are counted at once while a 9 s window lasts, the rest when opened; `GET /query/evidence-line?token&line` opens any line (session in memory 30 minutes, bound to the scope that asked). The answer's own lines are its own rows (5 listed contacts are 5 lines, not the 11 matching).
* **Checked on the restored copy of the live database, real Orion data**: 30 of 30 cases, one per summary view: every line that has numbers to recompute (pairs: events, direction counts, duration, first and last; links; account counterparties, activity and limit bands; cell visits; silent numbers; weekly anchors; handsets and device pairings; registrations; plate visits; document values and links) gets them back exactly from its records, no disagreement; the route opens line 2 of the top contacts (142 records, all found in their sheets) and refuses another case; 31 database specs on the generated case (the top contacts stand on the planted events); 42 unit specs; 23 interface tests. With the real model: the top contacts (5 lines, 5 of 5 agree), the common contacts of two numbers (154 and 95 records, each in 2 files, both sides), a connection path (5 links with their records), an account's top payee (8 records), the number of different contacts (10 of 10 agree) were all itemised; the interface shows "Behind this answer: 5 lines, each standing for records" with the lines table and Show records.
* **Not itemised, and why the analyst is told**: a search through several steps (who is within two hops: the message says to ask how two of the numbers are connected, which lists each link of the route with its records), a query built from intermediate steps (CTE), a query that removes repeated values (DISTINCT), an outer join, a window. A line whose view is heavy and was not reached in the window says "Counted when you open it".
* **Found on the way and fixed**: a heavy view (v_meeting_candidates) took longer than the answer to count line by line (now lazy); anchor runs and deviations re-read the weekly anchors for every record (25 s; now once per line); an answer's notes repeated the lines' check (removed).
* **Deploy, when the owner says so**: dry run (compose's hash must equal the running label `74eb879ac9e9`), `3-recreate-api.ps1 -Apply -To round9c-20261102 -RollbackTo before-round9c-20261102`, restart the workspace, re-ask a handful of live questions and open a line.

## Rollback, if ever needed

Worker: `.\tools\dev\live-window\4-recreate-worker.ps1 -Rollback`. Engine: `.\tools\dev\live-window\3-recreate-api.ps1 -Rollback -RollbackTo before-round6-20261030` (to the engine of before round 6; without `-RollbackTo` the script goes back to `before-families-20261030`, the engine of before the families, which would list the new, empty views on the migrated database). Database: the migrations' rollback scripts in reverse order (runbook section 4; they refuse to drop a table that holds rows,
so the three new collections would be exported or removed first), or the dump above. The new collections are synthetic and can stay as the demonstration.
