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

## Rollback, if ever needed

Worker: `.\tools\dev\live-window\4-recreate-worker.ps1 -Rollback`. Engine: `.\tools\dev\live-window\3-recreate-api.ps1 -Rollback`. Database: the migrations' rollback scripts in reverse order (runbook section 4; they refuse to drop a table that holds rows,
so the three new collections would be exported or removed first), or the dump above. The new collections are synthetic and can stay as the demonstration.
