# Layout registry v1 (roadmap Phase 0) - DRAFT entries, tested on synthetic goldens

Status: written and tested 2026-10-12. All 13 entries are **draft**: their header lists come from a structure-only inspection of the supplied samples, and each lists what is still unconfirmed. Nothing here reads a real file, touches a database or changes the worker; the entries are not wired into ingestion yet (Phase 1, "Ingest v2"). Code: `ingestion/forensic_records/layout_registry/`. Tests: `ingestion/forensic_records/tests/test_layout_registry.py` (50 tests). Golden generator and confirmation tool: `tools/real_data/make_golden.py`, `tools/real_data/confirm_layout.py`.

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
| The 13 entries | `layout_registry/layouts/<layout_id>.v1.json` |
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
| C1 | `call-c1-company` | target_cdr | xlsx | 16 | - | header-list, source-time-zone, operator, duration-unit, noise-columns, coordinates, bare-call-labels, cell-id |
| C2 | `call-c2-strt-tm` | target_cdr | xlsx | 14 | - | header-list, source-time-zone, operator, slash-date-order, noise-columns, preamble, event-types |
| C3 | `call-c3-ufone-style` | target_cdr | xlsx | 17 | - | header-list, source-time-zone, operator, duration-unit, noise-columns, preamble, cell-id |
| C4 | `call-c4-telenor-style` | target_cdr | xlsx | 18 | - | header-list, source-time-zone, operator, noise-columns, network-volume, cell-structure |
| C5 | `geofence-c5-mobilink` | tower_dump | xlsx | 7 | mobilink | header-list, source-time-zone, duration-unit, implied-cell, party-roles, bare-call-labels |
| C6 | `geofence-c6-telenor` | tower_dump | xlsx | 15 | telenor | header-list, source-time-zone, direction-codes, party-roles, network-volume |
| C7 | `geofence-c7-ufone` | tower_dump | xlsx | 13 | ufone | header-list, source-time-zone, slash-date-order, duration-unit, cell-columns, event-type, party-roles |
| C8 | `geofence-c8-zong` | tower_dump | xlsx | 13 | zong | header-list, source-time-zone, slash-date-order, duration-unit, party-roles, imei-side, activity-code |
| I1 | `ipdr-i1-jazz-ufdr` | ipdr | csv | 32 | - | header-list, source-time-zone, operator, cell-identity, rat-codes, sub-protocol, companion |
| I2 | `ipdr-i2-telenor-export` | ipdr | csv | 18 | - | header-list, source-time-zone, operator, ci-composite, type-columns, active-time |
| I3 | `ipdr-i3-ufone-xls` | ipdr | spreadsheetml | 18 | ufone | header-list, source-time-zone, protocol-codes, cell-columns, src-dst |
| I4 | `ipdr-i4-zong-txt` | ipdr | tsv, xlsx | 27 | zong | header-list, source-time-zone, slash-date-order, metadata-lines, rat-codes |
| A1 | `appusage-a1` | app_usage | xlsx | 12 | - | header-list, source-time-zone, operator, province-city |

Every entry shares two open items. **The source time zone:** no file states one, so the job decides (and records how). **The header list:** established from a structure-only inspection, to be confirmed against a structure report of the real file.

## 7. Making a draft active

1. Put the real file in the vault, register it with its authority, and profile it (`docs/data/real-data-handling-policy.md`, sections 3 and 4).
2. `python -I -X utf8 tools/real_data/confirm_layout.py --layout <id> --report <report.json>` compares the container, the header row, every header name in order, the time formats seen (and day-or-month order), and the labels in each enumerated column with the entry. It prints names, labels and formats only. A pass confirms the header list and the vocabulary; a failure names the columns or labels that differ.
3. Resolve each remaining unconfirmed item with the provider's documentation or an analyst, and edit the entry (a rule change is a new `layout_version`). Set `confirmation.headers` to the report that confirmed it.
4. The synthetic golden for the entry passes, and a local aggregate reconciliation of the real file passes (`tools/real_data/reconcile_aggregates.py`).
5. Only then set `status` to `active`. The validator refuses `active` while any unconfirmed item remains.

Adding a layout: copy the closest entry, rename it (`<layout_id>.v1.json`), edit the header and rules, describe the golden, run the tests. The loader checks the file name, the specification and uniqueness, and the entry cannot contain a real-looking value (a test scans every entry).

## 8. What the tests prove, and what they do not

50 tests (plain `unittest`, standard library only): `python -X utf8 -m unittest ingestion.forensic_records.tests.test_layout_registry`.

* Every entry loads and validates; its definition hash ignores `status` and `confirmation` but changes with any rule; the specification's vocabularies equal the CHECK constraints of migration 012 (the test parses the SQL); header normalisation equals the worker's.
* For every layout, a synthetic golden (same shape, header position, preamble, formats, enumerations, typos, duplicate headers, noise columns, a blank row, an unparsable time, a missing time, an unknown label, a sentinel party or a bad address) is matched to **only its own layout**, and every row reads back to the truth the generator recorded, for several seeds and sizes. The goldens contain only reserved values (the real-data scanner finds nothing in them).
* Each golden is also written to its real container (xlsx, SpreadsheetML, CSV, UTF-16 text), profiled by the D1 tool, and confirmed by `confirm_layout.py`: the whole toolchain on synthetic input.
* Seventeen deliberate breakages of the machinery and the entries (scoring, a guessed date order, a dropped flag, a wrong class, swapped columns, a removed alias, a wrong epoch, a flipped zone, and so on) were each caught by a test.

They do **not** prove that an entry is right about the real file: the golden is built from the entry, so a wrong belief about a column would be wrong on both sides. That is what the confirmation (step 2 above) and the real-file reconciliation are for. `ops/Test-MigrationsThrowaway.ps1` additionally registers all 13 entries in `forensic.layout_versions` on a disposable database (twice, then a changed definition under the same version is refused).

## 9. Not in v1

* Wiring into the worker (`normalize_record`, `schema_profile`) and the typed-table writer: Phase 1.
* Reading the Jazz "Basic Information" companion file and the seven metadata lines of the Zong text export (their labels are not recorded); the shown-versus-total record check depends on it.
* Parsing ECGI and the composite CI cell strings (kept as raw text); the template file with merged header cells.
* Operator-documented meanings (direction codes, activity code, network volume unit, RAT names): listed as unconfirmed, not assumed.
* Learned or assisted mapping for an unknown layout (roadmap 4.4, step 4).
