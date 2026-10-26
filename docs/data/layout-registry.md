# Layout registry v1 (roadmap Phase 0) - DRAFT entries, tested on synthetic goldens

Status: written and tested 2026-10-12. All 13 entries are **draft**, but their header lists, time formats and vocabularies were **confirmed on 2026-10-12 against structure reports of real sample files** (section 7a); what is still unconfirmed (time zone, operator, direction codes, units and so on) is listed per entry. Nothing here reads a real file, touches a database or changes the worker; the entries are not wired into ingestion yet (Phase 1, "Ingest v2"). Code: `ingestion/forensic_records/layout_registry/`. Tests: `ingestion/forensic_records/tests/test_layout_registry.py` (54 tests). Golden generator and confirmation tool: `tools/real_data/make_golden.py`, `tools/real_data/confirm_layout.py`.

## 1. What it is for

Providers send the same kind of data in different shapes: different column names (with typos), header rows at different depths, Excel serial numbers or text dates in four formats, numeric or text direction codes, cell identities of different kinds. A **layout entry** says, for one shape, which columns mean what and how to read them. The registry answers two questions:

1. **Which layout is this file?** Match the header row against every entry.
2. **What do its columns mean?** Apply the entry to each row and produce the canonical fields of `comm_event`, `data_session` or `app_usage` (migration 012), with quality flags for everything that is unknown, unconfirmed or malformed.

Layouts are **data** (JSON files in `layout_registry/layouts/`), so adding a provider is a reviewed file, not new code. An entry holds header names, maps and rules and never a value from a real file.

```
file --> container reader (xlsx, SpreadsheetML, csv, utf-16 tsv) --> header row + rows
     --> registry.find_header(rows)  --> layout entry (id, version, status)
     --> interpret_rows(entry, header, rows, policy) --> canonical record per row
                                                         (accepted, quarantined with a reason, or skipped) + flags
     --> typed tables of migration 012 (each row carries layout_id and layout_version)
```

## 2. Where things are

| Piece | File |
|---|---|
| Entry specification and validator | `layout_registry/spec.py` |
| Loader, definition hash, SQL that records entries in `forensic.layout_versions` | `layout_registry/registry.py` |
| Header normalisation, scoring, preamble scan | `layout_registry/matching.py` |
| Time formats, Excel serials, zone conversion | `layout_registry/timeparse.py` |
| Number classes, device identifiers, cell keys, coordinates | `layout_registry/parties.py` |
| Reference interpreter and summary | `layout_registry/interpret.py` |
| Command line (`list`, `validate`, `sql [--out FILE]`; run from the repository root as `python -X utf8 -m ingestion.forensic_records.layout_registry <command>`) | `layout_registry/__main__.py` |
| The 16 entries (13 provider layouts, the seed export, version 2 of the Telenor-style layout and the demonstration call layout; sections 6a and 6b) | `layout_registry/layouts/<layout_id>.v1.json` |
| Synthetic goldens with their truth | `tools/real_data/make_golden.py` |
| Confirmation of an entry against a structure report | `tools/real_data/confirm_layout.py` |

## 3. An entry, in outline

| Key | Meaning |
|---|---|
| `layout_id`, `layout_version`, `status` | Identity and life cycle (`draft`, `active`, `retired`). A rule change needs a new version; `status` and `confirmation` are not part of the definition hash |
| `family`, `dataset_kind`, `operator_label` | As in `forensic.layout_versions`. The operator label comes from a file name, never from a number prefix |
| `container` | Allowed kinds (`xlsx`, `spreadsheetml`, `csv`, `tsv`), encoding, delimiter, and how the header row is found (`fixed` index or `scan` within N rows) |
| `header` | `columns` in order (a repeated name carries its `occurrence`), `required` columns, `aliases` for known typos, `min_score` |
| `fields` | Which source column fills each canonical field; `by_direction` picks a counterparty column per direction; `combined` reads two coordinates from one cell |
| `time` | Column per time field, accepted `formats`, `date_order` (null until settled), `source_tz` (null unless the source states it) |
| `duration` | `column_seconds`, `minutes_seconds`, `column_milliseconds`, `from_times` or `none` |
| `enums` | Label maps for direction and event type, each rule marked `confirmed` or not; `proposed` records a suggested meaning without applying it |
| `cell`, `noise_columns`, `null_tokens`, `quarantine`, `protocol_numbers` | Cell parts, columns that are not data, tokens that mean "empty", rules that reject a row, and whether numeric protocol codes are IANA numbers |
| `confirmation` | How the header list was established and the list of **unconfirmed** items, each with what it affects |
| `golden` | Row count and recipes for the synthetic file |

## 4. How matching works

Header cells are normalised exactly as the worker does (`normalize_header`, kept in step by a test), so "Date & Time" and "date_time" are the same name. An entry's score is the number of matched columns divided by the larger of its own column count and the file's, so extra or missing columns lower it; an entry qualifies when every required column is present and the score reaches `min_score` (0.9 by default). Declared aliases cover known typos (C6's `CALL_ORIG_NUM` also accepts `call_org_num`, and the match says it used the alias); an undeclared near miss (one letter off) is listed, never accepted. A repeated header is matched by occurrence in both spellings (a list of cells, or the xlsx reader's `Cell_ID__duplicate_2`). Preamble lines are skipped by trying each of the first rows as the header. When two entries qualify within 0.05 of each other the decision is `ambiguous`, not an arbitrary pick. By default draft entries take part; `include_draft=False` restricts matching to active ones.

## 5. How rows are read

* **Time.** Formats are tried in the order the entry lists them. An Excel serial is exact to the microsecond. A slash date whose two leading numbers are both 12 or less has no order of its own: it is read only if the entry or the job's policy declares day-first or month-first, otherwise the row is quarantined as `ambiguous_date_order` (a day above 12 settles it by structure). A dot date is day first. The zone is the entry's `source_tz` if the source states one, otherwise the job's policy; the row records `time_basis` (`declared`, `assumed_by_profile`, `confirmed_by_analyst`, `explicit_offset`) and, with no zone, carries the flag `source_timezone_unknown` and no UTC time. This mirrors the worker's `forensics.time-policy/v1`.
* **Numbers.** Each party is classed `person_like`, `short_code`, `sender_id`, `international`, `service_label` or `unknown`; only an unambiguous Pakistani mobile number gets a normalised form (92 plus ten digits). IMEI and IMSI lengths are checked and the type allocation code (first eight digits) is taken only from 14 or more digits.
* **Enumerations.** A label that no rule maps is flagged `unmapped_label` and the field stays `unknown`; an unconfirmed rule still maps, but every row it touches carries `enum_unconfirmed:<field>`. Where the meaning of a code is not known (C6's 0, 1, 2) the rule maps it to `unknown` and records the suggested meaning in `proposed`: **a code is never guessed**.
* **Cells.** The cell key uses the same rule as `forensic.make_cell_key` (migration 013), with the same test vectors on both sides. A missing PLMN or area code makes the key partial and flags `cell_identity_partial`.
* **Other.** Columns that are not data (Description, MACAddress) are flagged when populated and kept only in the raw payload; "?" and similar tokens read as empty; a counterparty equal to the subject is dropped where the entry says so; IP addresses and ports are validated; a row with no start time is quarantined (`time_missing`, `time_unparsed`).

## 6. The 13 layouts

| Code | Layout id | Kind | Container | Columns | Operator label | Unconfirmed items |
|---|---|---|---|---|---|---|
| C1 | `call-c1-company` | target_cdr | xlsx | 16 | - | source-time-zone, operator, duration-unit, noise-columns, coordinates, bare-call-labels, cell-id |
| C2 | `call-c2-strt-tm` | target_cdr | xlsx | 14 | - | source-time-zone, operator, noise-columns, preamble, event-types |
| C3 | `call-c3-ufone-style` | target_cdr | xlsx | 17 | - | source-time-zone, operator, duration-unit, noise-columns, preamble, cell-id |
| C4 | `call-c4-telenor-style` | target_cdr | xlsx | 18 | - | source-time-zone, operator, noise-columns, network-volume, cell-structure |
| C5 | `geofence-c5-mobilink` | tower_dump | xlsx | 7 | mobilink | source-time-zone, duration-unit, implied-cell, party-roles, bare-call-labels |
| C6 | `geofence-c6-telenor` | tower_dump | xlsx | 15 | telenor | source-time-zone, direction-codes, party-roles, network-volume |
| C7 | `geofence-c7-ufone` | tower_dump | xlsx | 13 | ufone | source-time-zone, duration-unit, cell-columns, event-type, party-roles |
| C8 | `geofence-c8-zong` | tower_dump | xlsx | 13 | zong | source-time-zone, duration-unit, party-roles, imei-side, activity-code |
| I1 | `ipdr-i1-jazz-ufdr` | ipdr | csv | 32 | - | source-time-zone, operator, cell-identity, rat-codes, sub-protocol, companion |
| I2 | `ipdr-i2-telenor-export` | ipdr | csv | 18 | - | source-time-zone, operator, ci-composite, type-columns, active-time |
| I3 | `ipdr-i3-ufone-xls` | ipdr | spreadsheetml | 18 | ufone | source-time-zone, protocol-codes, cell-columns, src-dst |
| I4 | `ipdr-i4-zong-txt` | ipdr | tsv, xlsx | 27 | zong | source-time-zone, metadata-lines, rat-codes |
| A1 | `appusage-a1` | app_usage | xlsx | 12 | - | source-time-zone, operator, province-city |
| P1 | `anpr-a1` | plate_sighting | csv, xlsx | 9 | - | real-sample (designed, not read from a real file), source-time-zone, confidence-scale, direction |

Every entry shares one open item: **the source time zone.** No file states one, so the job decides (and records how). The header list is no longer open (section 7a); the day-or-month order of slash dates is settled for the four layouts that use them (C2 and C8 month first, C7 and I4 day first).

## 6a. The fourteenth entry: the repository's own seed export (2026-10-20)

`ipdr-seed-s8` is **not a provider layout.** It is the 8-column internet-record export that the repository's synthetic seed data uses (`timestamp, source_ip, destination_ip, domain, protocol, bytes, subscriber_id, session_id`), and it is the shape of the internet-record files of the four older (synthetic demonstration) collections. It was parked on 2026-10-19 (the golden tool assumed an IMSI) and is in `layouts/` now.

* **What it states:** the subscriber's number (`subscriber_id`), the device's address (`source_ip`, read as the UE address) and the server's, the host (`domain`), the protocol, a session reference, and one instant ending in `Z` (stored as UTC, as stated). **What it does not state, and the entry does not invent:** an end time or duration, an IMSI or IMEI, any port, a public address, and the direction of `bytes` (one volume column; not mapped to up or down, it stays in the raw row). So address attribution (who held a public address) cannot be answered from these files.
* **Status:** draft, like the other thirteen; the header list was read from the repository's own files, the semantics (direction of `bytes`, role of the source address, operator) are listed as unconfirmed in the entry.
* **Tooling:** `tools/real_data/make_golden.py` now builds an internet-record golden for an entry that does not state IMSI, ports, a public address or a server port (what it does not state is `None` in the truth; the out-of-range-port defect is skipped for an entry with no server port). Tests: the registry suite is 56 (was 55; one new test reads a row of the export and proves the unstated fields stay empty and that no other layout carries these eight columns), the typed-ingest suite 47, the real-data tool suite 32, all pass; `layout_registry validate` says `entries=14`; the case register still describes the generator.
* **Sizing of the four older collections, read-only, 2026-10-20** (the 16 stored call-record and internet-record files, copied out of the worker with `docker cp`, matched on the host; header names and counts only): **10 of 16 are recognised, and they hold 26,450 of the 26,466 rows**: the 16-column call records as `call-c4-telenor-style` at 1.00 (forensic-demo, multimodal, records-demo, and one runtime-acceptance file) and the three 2,500-row internet files as `ipdr-seed-s8` at 1.00. **The 6 not recognised are test fixtures of 2 to 4 rows** (header shapes of 8, 8, 10, 9, 11 and 10 columns: a tab-separated and a comma-separated 8-column call file, a 10-column and a 9-column call file, an 11-column internet file with a time zone and a 10-column one with a URL and the volumes in both directions). They matter because the engine's guard declines a case unless **every** call-record and internet-record file of it is typed: forensic-demo has 2 of them, runtime-acceptance 3, records-demo 1, and the multimodal case none, so **the multimodal case is ready to be typed as soon as the entry is registered and its rows are written; the other three need the fixtures typed, or a decision about them** (a layout per fixture shape, five or six small drafts; or taking the 16 rows out of those cases; or a guard that answers on the typed part and says what is not typed).
* **Live (2026-10-20, the owner's go):** the entry is registered in `forensic.layout_versions` and is in the worker image (`40a9a7195685`), and **the multimodal acceptance case is typed** (5,000 call records, 2,500 internet sessions; `docs/data/ask-engine-v2.md`, "The multimodal acceptance case typed into the live tables"). The other three collections are not typed: they wait for the six fixtures (above) and a go.

## 6b. Version 2 of the Telenor-style layout, latest-version matching, and the demonstration call layout (2026-10-20)

* **`call-c4-telenor-style` version 2** (the owner: "yes map VOLTE to voice"). Version 1 left the call-type labels `VOLTE` and `GPRS` as event type `other`; on the multimodal case that was 598 and 2,721 of 5,000 rows, so a question about voice calls left the VoLTE calls out. Version 2 maps `VOLTE` to `voice` in a **separate unconfirmed rule** (the rows carry `enum_unconfirmed:event_type`; the label is in the repository's seed files and in no real sample) and `GPRS` to `data` in the confirmed rule, which is what the confirmed `geofence-c6-telenor` layout already does. Version 1 is unchanged (a different hash would be refused) and stays for the rows typed under it. The real-data collection holds only `CALL` and `SMS` under this layout (2,393 and 1,603 rows), so nothing there changes.
* **Matching reads the latest version of each layout** (`Registry.current_entries`). The matcher ranked every entry, so a layout and its successor would tie on every header and make every match ambiguous. Older versions stay in the registry for history, for `get(id, version)` and for `register_sql`.
* **`call-demo-8`** is the 8-column call layout of the two small synthetic call files of the forensic-demo collection (`MSISDN, CALL_DIALED_NUM, CALL_START_DT_TM, CALL_END_DT_TM, CALL_TYPE, INBOUND_OUTBOUND_IND, Cell_SITE_ID, location`; the tab-separated file has the two type columns in the other order, the header is matched by name). The column names are the sixteen-column layout's, so the two are told apart by their count: each needs the columns the other lacks. `INBOUND`, `OUTBOUND` and `DATA` in the direction column are a separate unconfirmed rule; the cell identity is a site name such as `PK-LHR-SYN-001`, kept as stated.
* **Sizing after this:** 12 of the 16 stored call-record and internet-record files of the four older collections are recognised (the two small forensic-demo files were the first of the six fixtures). **Four are left**, all 2 to 4 rows: three in `nexusai-runtime-acceptance-20260730` (a 10-column call file with `MO/MT` directions and offset times, a 9-column one whose `a_number` and `b_number` are caller and callee, not subject and counterpart, which needs the interpreter to choose the subject by direction, and an 11-column internet file with a time-zone column) and one 10-column internet file in `records-demo`. Those two collections stay untyped.
* **Live (2026-10-20, the owner's go: "write the layouts and type forensic-demo"):** both entries are in the worker image (`0651ab31e9c1`; the previous worker, `40a9a7195685`, is kept as `before-c4v2-20261020`) and registered in `forensic.layout_versions`; the multimodal call records were typed again under version 2 (voice 1,486, data 2,721, sms 793); **`nexusai-forensic-demo` is typed** (see `docs/data/ask-engine-v2.md`).
* **Tests:** registry 59 (three new), typed ingest 47 (two pins moved to version 2), real-data tools 32, loader 12; `layout_registry validate` says `entries=16`; the case register still describes the generator.

## 6c. The last four fixtures, and a subject chosen by direction (2026-10-21)

The four 2-to-4-row files left in runtime-acceptance and records-demo have draft layouts read from the files: `call-iso-10-demo` (subscriber, source and target numbers, ISO times with an offset, MO and MT), `call-ab-9-demo` (`a_number` and `b_number` are caller and callee, no subscriber column, 00-prefixed numbers, ORIGINATING and TERMINATING), `ipdr-sessions-11-demo` (ports, one undirected volume, a time-zone column that stays in the raw row) and `ipdr-url-10-demo` (one instant ending in Z, URL, cell, volumes in both directions). For the caller-and-callee file the interpreter learned `fields.a.by_direction`: the subscriber is the caller on an outgoing event and the callee on an incoming one; a direction that is not known leaves the subscriber unknown (flags `subject_direction_unresolved` and `subject_missing`), never a guess. **With them all 16 call and internet files of the four older collections are recognised and read** (every row accepted on the stored files; subscriber and counterparty right in both directions). 20 entries; tests: registry 61, typed ingest 47, real-data tools 32.

**Live (2026-10-21):** worker `db43bb68c042` (rollback `before-s5-layouts-20261021`), layouts registered, runtime-acceptance and records-demo typed; **records-demo's four jobs state no time zone, so Asia/Karachi was assumed (`assumed_by_profile`); the owner confirmed it on 2026-10-21 and the 5,000 rows are now `confirmed_by_analyst`.** Every collection that holds call or internet files is typed.

## 7. Making a draft active

1. Put the real file in the vault, register it with its authority, and profile it (`docs/data/real-data-handling-policy.md`, sections 3 and 4).
2. `python -I -X utf8 tools/real_data/confirm_layout.py --layout <id> --report <report.json>` compares the container, the header row, every header name in order, the time formats seen (and day-or-month order), and the labels in each enumerated column with the entry. It prints names, labels and formats only. A pass confirms the header list and the vocabulary; a failure names the columns or labels that differ.
3. Resolve each remaining unconfirmed item with the provider's documentation or an analyst, and edit the entry (a rule change is a new `layout_version`). Set `confirmation.headers` to the report that confirmed it.
4. The synthetic golden for the entry passes, and a local aggregate reconciliation of the real file passes (`tools/real_data/reconcile_aggregates.py`).
5. Only then set `status` to `active`. The validator refuses `active` while any unconfirmed item remains.

Adding a layout: copy the closest entry, rename it (`<layout_id>.v1.json`), edit the header and rules, describe the golden, run the tests. The loader checks the file name, the specification and uniqueness, and the entry cannot contain a real-looking value (a test scans every entry).

## 7a. Confirmation against real samples (2026-10-12)

The owner supplied the sample files for testing and they were registered in the real-data vault (outside the repository) with that stated as their authority. Each file was profiled by structure (`profile_structure.py`, shareable mode) and compared with every entry (`confirm_layout.py`). Only the shareable reports were read.

* **All 13 layouts confirm** (header names and order, header row position, time formats, vocabulary labels) against at least one real file. Layouts with several files: C2, C3, C6, C8, I1 and I4 two each, I2 three.
* **Empty unnamed header cells.** The call-record templates (C1 to C4) carry 10 to 14 header cells that are empty, and C5 two; none holds data. Goldens could not show this, because they were built from the entry. The matcher already ignores blank header cells; `confirm_layout.py` now ignores a blank header column that holds no data and **fails** one that does (data under a blank header cannot be placed by name). The goldens of C1 to C4 now carry such padding (`golden.blank_columns`).
* **Slash-date order settled by structure** on the real files: C2 and C8 month first, C7 and I4 day first (a value whose second, or first, part is above 12 occurs). The entries now state it (`time.date_order`), and a row that contradicts it is quarantined as `date_order_conflict`, never read in the other order.
* **I4 has two forms:** UTF-16 text with slash dates and an xlsx (6 preamble rows) with Excel serial times. The entry accepts both.
* **Three real files match no entry:** a template whose header cells are merged (data sits under the blank cells) and two small headerless two-column text files (the companion files of the Jazz export); they stay out of v1 as listed in section 9. One nested zip archive inside the internet-record archive was not opened.
* This confirms **structure only**. It says nothing about what a column means beyond what the names and masks show (direction codes, units, the operator, the time zone); those stay in each entry's unconfirmed list. The reconciliation of loaded counts against the files (`reconcile_aggregates.py`) has not been run, because nothing is loaded yet.

## 8. What the tests prove, and what they do not

54 tests (plain `unittest`, standard library only): `python -X utf8 -m unittest ingestion.forensic_records.tests.test_layout_registry`.

* Every entry loads and validates; its definition hash ignores `status` and `confirmation` but changes with any rule; the specification's vocabularies equal the CHECK constraints of migration 012 (the test parses the SQL); header normalisation equals the worker's.
* For every layout, a synthetic golden (same shape, header position, preamble, formats, enumerations, typos, duplicate headers, noise columns, a blank row, an unparsable time, a missing time, an unknown label, a sentinel party or a bad address) is matched to **only its own layout**, and every row reads back to the truth the generator recorded, for several seeds and sizes. The goldens contain only reserved values (the real-data scanner finds nothing in them).
* Each golden is also written to its real container (xlsx, SpreadsheetML, CSV, UTF-16 text), profiled by the D1 tool, and confirmed by `confirm_layout.py`: the whole toolchain on synthetic input.
* Twenty-one deliberate breakages of the machinery and the entries (scoring, a guessed date order, a dropped flag, a wrong class, swapped columns, a removed alias, a wrong epoch, a flipped zone, and so on) were each caught by a test.

They do **not** prove that an entry is right about the real file: the golden is built from the entry, so a wrong belief about a column would be wrong on both sides. That is what the confirmation (step 2 above) and the real-file reconciliation are for. `ops/Test-MigrationsThrowaway.ps1` additionally registers all 13 entries in `forensic.layout_versions` on a disposable database (twice, then a changed definition under the same version is refused).

## 9. Not in v1

* Wiring into the worker (`normalize_record`, `schema_profile`) and the typed-table writer: Phase 1.
* Reading the Jazz "Basic Information" companion file and the seven metadata lines of the Zong text export (their labels are not recorded); the shown-versus-total record check depends on it.
* Parsing ECGI and the composite CI cell strings (kept as raw text); the template file with merged header cells.
* Operator-documented meanings (direction codes, activity code, network volume unit, RAT names): listed as unconfirmed, not assumed.
* Learned or assisted mapping for an unknown layout (roadmap 4.4, step 4).

## Where the registry is used now

`ingestion/forensic_records/layout_registry/readers.py` holds the one set of row readers (xlsx, SpreadsheetML 2003, delimited text, with the archive and size limits); the vault tools (`tools/real_data/profile_structure.py`) and the worker's typed ingest (`typed_ingest.py`, `docs/data/ingest-v2.md`) both import it, so a file is read the same way when it is profiled, confirmed and ingested. The 13 entries are recorded in `forensic.layout_versions` of the live database as drafts (2026-10-13, `docs/data/d2-migration-runbook.md` section 12).
