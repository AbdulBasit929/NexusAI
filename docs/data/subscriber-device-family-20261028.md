# Subscriber registration and device reference as typed families (P1-B, second and third family): design, truth and gates

**Date:** 2026-10-28 · **Status:** design and gates written BEFORE any build or measurement · **Owner's go:** "go ahead with subscriber and device family, also the transactions" (2026-10-28) covers building and testing on **throwaway databases and generated files only**; nothing is applied to the live database, the worker or the API without a separate go.
Plan of record: `docs/roadmap/master-plan-20261021.md` P1-B ("subscriber, SIM and device reference (tables exist)"). Template: `docs/data/anpr-typed-family-20261026.md`. Companion: `docs/data/transactions-family-20261028.md`.

## 1. What this is, and what it is not

**Is:**
* **Subscriber registration facts** (`forensic.subscriber_registration`, migration 012): "the number 0300... was registered per source S as SIM with IMSI/ICCID X, status Y, from T1 to T2". Read from a subscriber or SIM information export (CSV or XLSX) by the production typed ingest. Conflicting rows are kept with both sources (never merged).
* **Device reference** (`forensic.device_tac`, migration 013): a tenant-wide list that says "the TAC (first eight digits of an IMEI) is listed as make/model per source". Loaded by a reference loader, not as evidence rows.
* **Three analyst views** (migration 028): registrations of a number over time; the handsets a number used, with the make and model the TAC list gives; and the join that lets a call question reach a registration question (today a call-to-subscriber join matches 0 rows).

**Is not (stated so that nothing is claimed that is not built):**
* **Names, national ID numbers and addresses.** `forensic.subscriber_identity` stays unwritten and unread: the runtime role has no privilege on it until the masking design is approved (roadmap R9, an owner decision). A source file's identity columns are **detected, dropped from `raw_payload_redacted`, counted in the ingest summary, and never stored in the database**; the original file stays in retained evidence untouched. The engine and the interface say nothing of who a person is.
* **Ownership.** A registration is "registered per source", never "owned by"; a number can be used by someone else (the caveat is attached to every such answer by the engine).
* **Real operator layouts.** The one layout here (`subscriber-s1`) is a **draft** for a generic export, designed, not read from a real file. A real provider's layout needs a real sample read in the vault (counts and booleans only) and the owner's confirmation before promotion (decision D9, still open).
* **The cell and tower reference** (`cell_site`): the next family.
* **Anything about a handset beyond its TAC listing.** A TAC list is community-collected; a model is "listed as", never a fact about the individual device.

## 2. Sources of data: what was fetched, what could not be, and why

| Need | Result | Detail |
|---|---|---|
| Device reference, real | **Fetched** | `tac_full.csv`, 11.8 MB, 255,000 rows (`Brand, TAC, SPECS`), repository `MoazEb/tac-database` (MIT; the rows are cross-referenced from public IMEI lists including Osmocom's CC-BY-SA TAC database), up to January 2026. In `C:/tmp/nexusai-family-data/tac/` with SHA-256 in `manifest.jsonl`, **outside the repository** (`tools/family_data/fetch.py device`). Read: 248,378 rows have an eight-digit TAC, 6,344 have seven digits (a leading zero lost in a spreadsheet), 277 have six, three are empty or one character; Samsung 31,905, Apple 8,819, Huawei 7,998, Oppo 5,313, Realme 5,283, Xiaomi 4,875. **Licence caveat:** the repository is MIT, the underlying rows are of mixed provenance (one source CC-BY-SA 3.0): the list is for local testing and is not committed; any redistribution waits for the licence to be settled (`source_license` carries the words). |
| Subscriber registration, real Pakistani | **Not available, and not fetched** | No lawful open record-level subscriber or SIM data exists for Pakistan; only aggregates (PTA, PBS: subscribers by operator and year). Leaked personal databases circulate online and are **never fetched or used**. The family is tested on the generator's synthetic files, whose identifiers come from the reserved ranges (`tools/real_data/reserved.py`). Real files join through the vault under the real-data policy when the owner can lawfully share them (D9). |
| Operator shares (calibration only) | Optional, not fetched | PBS/PTA aggregates on opendata.com.pk ("No licence provided"): could set the generator's operator mix; not needed for correctness. |

The real TAC list is used for what only real data can show: **loader robustness** (the seven- and six-digit rows, brand spellings, duplicates, empty rows). The synthetic world's handsets carry test TACs (prefix `00`), so the joins are tested against a **synthetic TAC reference** generated beside the case.

## 3. The pieces, in the order they are built

| # | Piece | Where | Proof, without a model |
|---|---|---|---|
| a | **Identifier normalisation**: Python `canonical_msisdn` exists; add IMSI (15 digits), ICCID (19 to 20 digits, Luhn) and TAC (eight digits; a seven-digit TAC is left-padded with one zero and flagged `tac_padded`, a six-digit one is rejected and counted) | `ingestion/forensic_records/parties.py` | equal to the SQL functions on a fixture of tokens, zero differences |
| b | **Layout fields and the interpreter** for `dataset_kind = subscriber_registration`: msisdn, imsi, iccid, status (normalised to `active`, `suspended`, `deactivated`, `unknown` with the raw word kept), operator, service type, activation and deactivation dates (zone from the job as for calls), identity columns marked `identity` | layout `subscriber-s1.v1.json`, `interpret.py`, `spec.py` | golden file per layout, round trip |
| c | **The typed writer** writes `subscriber_registration` idempotently (its key is the table's `UNIQUE (tenant, collection, file_id, row_hash)`; it has no partition time) and counts dropped identity columns | `typed_ingest.py` | second ingest writes 0 rows |
| d | **The device reference loader**: tenant-wide rows into `device_tac` with `source` and `source_license`, quality counts, idempotent, never overwriting another source | `ingestion/forensic_records/reference_loader.py` | counts equal the file's; second load 0 rows |
| e | **Migration 028**: `analyst.v_subscriber_registrations`, `analyst.v_number_registrations_at` (the registration valid at a time, with `conflicts` when sources disagree), `analyst.v_handsets` (IMEI, TAC, make, model, first and last seen, events, the listing source); comments on every column; grants; `ops/028_*` preflight, verify, rollback | `db/forensic_records/028_subscriber_device_views.sql` | rehearsed on a throwaway database |
| f | **A generated registration file and TAC reference** per case, from the same seed, **outside the default case build** (the register's hashes unchanged), with a planted key and an oracle | `tools/case_generator/subscriber_case.py` | the oracle answers from the planted records |
| g | **The engine**: catalogue entries (23 to 26 views), retrieval words, verified examples, a coverage guard, IMSI/ICCID/TAC as identifiers of the frame, caveats ("registered per source", "TAC listed as"), probe columns, the empty-result sentences | `api/forensic_records/ask_engine_*.go` | Go specs |
| h | **The bank and the judge** (kinds `registration` and `handset`) and one measurement on a fresh judgement case | `tools/ask_eval/subscriber_bank.py`, `judge.py` | G4 |

## 4. The truth: planted facts and their oracle

The generator's world already has people, SIMs (IMSI, IMEI), identity changes (`segments`) and cell sites. The registration file is derived from it with these plants, each with an oracle answer:

* **S1 SIM swap:** a number whose IMSI changes on a known date has two registrations (old IMSI until the swap, new from it).
* **S2 conflicting sources:** two files (two sources) disagree for a number and a period; both rows are kept and the view marks `conflicts`.
* **S3 activity after deactivation:** a number deactivated on a known date that still makes calls afterwards (the join question: "which numbers were active after their registration ended").
* **S4 unregistered number:** a number in the calls with no registration in any source (the answer is absence, in words).
* **S5 handset change:** a number whose IMEI (hence TAC, hence model) changes on a known date.
* **S6 format defects:** the number written as `03xx...`, `+92...`, `0092...`, `92...`; dates as `dd/mm/yyyy`, ISO and with a time; a blank ICCID; an exact repeated row; a row with a TAC of seven digits; a row with an unknown status word.
* **S7 identity columns:** every row carries a synthetic name, a synthetic CNIC (`00000-xxxxxxx-x`) and an address, so that the drop can be proved.

The oracle answers: the registrations of a number and their periods; the registration valid at a time; the numbers registered to a given IMSI; the numbers with activity after deactivation (count and list); the make and model of a number's handset at a date; how many numbers use a model.

## 5. Gates (written now; applied to the runs named; nothing is tuned on a case that is then rerun)

* **G1 ingestion reconciliation** (development cases 1 to 8, generated with registrations): rows accepted equal the oracle's unique rows; the dropped identity columns are counted per row (every row, every identity column) and **none of them is stored**; the quality flags equal the planted ones and no other; a second ingest writes nothing; the TAC load equals the reference's rows less the counted rejects.
* **G2 normalisation parity:** Python equals the SQL functions on a fixture (msisdn forms, IMSI, ICCID with and without a valid check digit, TAC lengths 6, 7, 8): zero differences.
* **G3 the views equal the oracle** on the development cases: every oracle query of section 4, zero differences; the conflict is shown with both sources; the absence is absence; another collection or no scope shows nothing.
* **G4 the engine** on a fresh judgement case (117; a second case, 121, only if G4 fails once and the cause is fixed, never the same case twice): at least 90% correct, **zero wrong**, the rest abstained or declined with a stated reason; no answer says a person owns, is, or lives at; every registration answer carries the caveat; wording gates W1 to W11 unchanged on a sample.
* **G5 safety:** (a) no synthetic name, CNIC or address value occurs in any table, view, audit row, retained plan cache or log of the throwaway database after the run (a scan, not a sample); (b) the runtime role still has no privilege on `subscriber_identity`; (c) scope fails closed; (d) the migration applies, verifies, rolls back and applies again on a throwaway database; (e) the repository scanner (`tools/real_data/scan_for_real_data.py --staged`) passes.
* **G6 nothing live:** no migration applied to the live database, no worker or API change deployed, no layout registered live, until the owner's separate go.

Latency is reported, not a gate. Speed work is separate (P-5).

## 6. Order of work and decisions

Order: a, b, c, d (ingest side, tests, no database) → e (views) and f (generator and oracle) → G1, G2, G3 on development cases → g (engine) → h and G4 → G5 → report. Live activation is a later, combined step with the ANPR family and the transactions family (one backup, one rehearsal, one worker overlay), on the owner's go.

**Decisions owed (none blocks the build):** R9 masking design before any identity column may be typed (recommendation: keep dark until the owner reviews a masking spec I write after this family); D9 real samples (the layouts stay draft until then); the TAC list's licence before it is shipped with the product (recommendation: ship the loader, not the list; the analyst supplies a list whose terms allow it).

## 7. Status

(filled as the work proceeds)
