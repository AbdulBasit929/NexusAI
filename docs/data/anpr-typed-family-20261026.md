# ANPR sightings as a typed family (P1-B, first family): design, truth and gates

**Date:** 2026-10-26 · **Status:** design and gates written BEFORE any build or measurement · **Owner's go:** "go ahead with step 1 and 2" (2026-10-26) covers building and testing on **throwaway databases and generated cases only**. **Nothing here is applied to the live database, deployed, or switched on; a live migration, a worker role or a family switch each wait for an explicit go.**
Plan of record: `docs/roadmap/master-plan-20261021.md` P1-B (ANPR sightings first; "layout and golden, typed writer, `v_plate_sightings` view draft, tests green, not applied").

## 1. What this is, and what it is not

**Is:** camera exports (CSV or XLSX lists of plate sightings: a time, a plate, a camera, optionally a lane, a direction, a position and a confidence) read by the production typed ingest into `forensic.plate_sighting` (migration 012, `source_kind = 'csv'`), an analyst view over them, the engine's catalogue entry, verified examples, a question bank and an oracle, so that an analyst can ask in plain words "how many times was plate X seen", "where and when", "which cameras", "which plates were seen at both cameras".

**Is not (stated so that nothing is claimed that is not built):**
* **Plates read from images or video** (the media roles): a separate derived family (ruling C12: derived rows are never merged with ingested records); their answers are a labelled composition, later (P4). The table's `source_kind` and `review_state` already keep them apart; this stage writes `csv` rows only.
* **Convoys and co-travel** (A15), **look-alike plates** and **fuzzy matching**: not built. A plate matches **exactly, on the normalised plate**; a near match is never a match. A plate that is not in the case is *absent*, said with what was searched (the wording of Amendment 5 extended to plates).
* **Who owns a vehicle:** no registry data exists in a case; the interface and the engine say nothing of owners.
* **Real camera files:** the layout is a **draft** for a generic export; a real provider's layout needs a real sample read in the vault (counts and booleans only) and the owner's confirmation before promotion (as for the call records, `layout-registry.md`).

## 2. The pieces, in the order they are built

| # | Piece | Where | Proof, without a model |
|---|---|---|---|
| a | **Plate normalisation in Python**, equal to `forensic.normalize_plate` of migration 012 (NFKC, Arabic-Indic and Persian digits to ASCII, dash, dot, middle-dot style separators removed, upper case) | `ingestion/forensic_records/layout_registry/parties.py` | 60 fixture tokens run through the SQL function on a throwaway database and through Python: **zero differences** (G2) |
| b | **Layout fields and the interpreter** for `dataset_kind = plate_sighting`: plate, camera id and label, lane, direction, position, confidence (a percent is converted and flagged), the sighting time (zone from the job, never guessed) | `layout_registry/spec.py`, `interpret.py`, layout `anpr-a1.v1.json` (draft) | layout validation, interpreter unit tests, the drift test against migration 012 |
| c | **The typed writer** writes `plate_sighting` (a table with `observed_utc` as its time and its own conflict key), idempotently | `typed_ingest.py`: per-table time column and conflict key; `TABLE_FOR_FAMILY["plate"]` | writer tests on the fake sink; re-ingest writes zero new rows (G1) |
| d | **Migration 027**: `analyst.v_plate_sightings` (+ comment, column descriptions for the catalogue), grants for the read-only role, `ops/027_*` preflight, verify and rollback | `db/forensic_records/027_plate_sightings.sql` | rehearsed on a throwaway database: preflight, apply, verify, apply again, rollback, apply again |
| e | **A generated ANPR file** per case, from the same seed, **outside the default case build** (so the register of 28 cases and its hashes stay valid), with its planted key and an **oracle** | `tools/case_generator/anpr_case.py`; `generate.write_case(..., with_anpr=True)` adds it | the audit (no real number, no non-synthetic plate), oracle unit tests, the ingest reconciliation (G1) |
| f | **The engine**: the catalogue entry and golden, retrieval words, a coverage guard for the family, **plates as identifiers of the frame** (a plate of the question must be in the query, as a number must), the empty-result probe over `plate_norm`, two verified examples | `api/forensic_records/ask_engine_*.go`, `ask_engine_examples.json` | Go specs (no model); the catalogue golden regenerated and reviewed |
| g | **The bank and the judge** for the family and one measurement on a fresh judgement case | `tools/ask_eval/anpr_bank.py`, `judge.py` kinds | G4 (below) |

Pieces a to e are built and tested without a model. f needs Go specs; g needs the real model for about 15 minutes, once, in the background, on a registered judgement case that was never asked.

## 3. The truth: the generated sightings and their oracle

The generator invents vehicles and cameras only (synthetic plates drawn from a reserved pattern that cannot be a real registration: three letters from a fixed set, then a number from a reserved range; every plate carries a synthetic marker in the audit). Per case: 10 to 14 cameras with positions around the case's area; 30 to 60 plates; 5 to 8 **vehicles of interest** that are seen many times along routes of two to five cameras at plausible times; the rest seen a few times; **planted difficulties**: a plate written with a dash, a space or lower case in some rows (the normalised plate is one), a sighting twice in the same second at one camera (a duplicate row), a plate seen only at one camera, a camera with no sighting on some days, a plate that appears **only as a look-alike** of a planted one (O for 0) which must **not** be returned for the planted plate, and times near midnight in the source zone.

The **oracle** answers, exactly, from the planted records: the count of sightings of a plate (normalised, duplicates in the file counted once as the views do), the list of (camera, time) of a plate (with and without a time range), the set of cameras that saw a plate, the sightings at a camera on a date or between two times, the plates seen at both of two cameras, the first and last sighting of a plate, the busiest camera. It is a separate implementation from the view (the independent truth), as for the call records.

## 4. Gates (written now; applied to the runs named; nothing is tuned on a case that is then rerun)

* **G1 ingestion reconciliation** (development cases 1 to 8, generated with ANPR): rows accepted equal the oracle's; the per-plate counts equal; distinct plates, cameras, first and last time equal; the quality flags are exactly the planted ones (no other flag); a second ingest of the same file writes **zero** new rows.
* **G2 normalisation parity:** Python equals the SQL function on 60 fixture tokens (Latin, lower case, separators, Arabic-Indic and Persian digits, full-width, middle dot): zero differences.
* **G3 the view equals the oracle** on the development cases: every oracle query of section 3, per plate and camera: zero differences, **and** the look-alike plate returns nothing for the planted one.
* **G4 the engine (one run, a fresh registered judgement case, the engine arm, once):** a bank of about 16 questions (4 kinds plus controls and a look-alike probe; written by `anpr_bank.py` from the oracle only; wording one of four phrasings per kind chosen by the seed); **zero wrong; at least 90% correct in each kind; controls 100%** (a plate not in the case, a camera with no sighting, a look-alike); the wording gate of Amendment 5 for the new absence sentences; median latency within the stage-3 budget (about 30 s on the laptop).
* **G5 safety:** the validator rejects a query that does not compare the question's plate (normalised) in `plate_norm`; a plate absent from the case is never answered with a number; the executor's red team (`ask_engine_redteam_ginkgo_test.go`) and the 276 Ask-engine specs stay green; the coverage guard declines a case that has plate sightings only in an untyped form.
* **G6 nothing live:** the migration is rehearsed on a throwaway database only; no live apply, no deploy, no `FORENSIC_TYPED_FAMILIES` change, no worker role. Each of those is a separate decision of the owner, asked with the evidence.

## 5. Order and limits

Build and unit-test a to e first (hours, no model); then f (Go, no model); then g. A defect found by a run is fixed at its cause and measured on the next reserve (the register has judgement 114 to 120 unopened); nothing is tuned on the case where it was found. If g is not reached in a session, a to f stand and are committed with their tests; the state is written in `NEXUSAI_CONTINUATION.md`.
