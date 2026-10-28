"""Ingest v2 (roadmap Phase 1): a structured export that the layout registry recognises is also written to the typed tables.

The worker's own path (`worker.py`, version 1) is untouched. This module adds a second, shadow write: it reads the file again with the shared row
readers, finds the layout in the registry, interprets every row with the layout's rules and the job's declared time zone, and writes the accepted rows
to `forensic.comm_event`, `forensic.data_session` or `forensic.app_usage` with full lineage (tenant, collection, evidence, file, batch, layout and
version, source row, a hash of the row and of its content). Nothing about the job's outcome depends on it: the shadow runs in a savepoint, a failure is
logged and summarised and rolls back only the typed rows, and the mode is `off` unless `FORENSIC_INGEST_V2=shadow` is set.

What it guarantees (and the tests check): every row read is loaded, quarantined or skipped with a reason and the three counts add up to the rows read;
a second run of the same file inserts nothing (`ON CONFLICT` on the idempotency key `tenant, collection, file, row hash, start`); a row that repeats
within a file is kept and flagged `duplicate_row`; a file the registry does not recognise, an ambiguous header and a file with no evidence id are
reported, never guessed; nothing but the typed tables and the returned summary is written (the worker puts the summary in its audit log).

Rows that are quarantined are counted by reason in the summary with the first row numbers; they are not stored row by row yet (the shadow phase writes
no second reject table next to version 1's; a quarantine table comes with the switch to `on`, a later migration).
"""
from __future__ import annotations

import csv
import hashlib
import itertools
import json
import logging
import os
import uuid
from dataclasses import dataclass, field
from datetime import date, datetime
from decimal import Decimal
from typing import Any, Iterable, Iterator, Mapping, Protocol, Sequence

try:
    from ingestion.forensic_records.layout_registry import Policy, interpret_rows, load_registry
    from ingestion.forensic_records.layout_registry import readers
except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution
    from layout_registry import Policy, interpret_rows, load_registry
    from layout_registry import readers

LOG = logging.getLogger("forensic.typed_ingest")
CONTRACT = "typed-ingest/1"
BATCH_ROWS = 5000
DEFAULT_MAX_ROWS = 2_000_000
NO_VALUE = "\\N"

# the typed table each family of layouts is written to
TABLE_FOR_FAMILY = {"communications": "comm_event", "internet": "data_session", "app_usage": "app_usage", "plate": "plate_sighting", "subscriber": "subscriber_registration"}
# the time column each typed table is partitioned and keyed on: a sighting has no start, it has an observed time (migration 012)
# a registration has no time of its own (None): it is valid from its activation to its deactivation, and its key has no time (migration 012: UNIQUE tenant, collection, file, row hash)
TIME_COLUMN = {"comm_event": "start_utc", "data_session": "start_utc", "app_usage": "start_utc", "plate_sighting": "observed_utc", "subscriber_registration": None}
# the idempotency key of the typed tables (migration 012); conflict_key(table) is the one of a table
CONFLICT_KEY = "tenant_id, collection_id, file_id, row_hash, start_utc"
REQUIRED_COLUMNS = ("tenant_id", "collection_id", "evidence_id", "file_id", "batch_id", "row_hash", "content_hash", "start_utc")


def conflict_key(table: str) -> str:
    column = TIME_COLUMN.get(table, "start_utc")
    return CONFLICT_KEY.replace(", start_utc", ", " + column if column else "")


def required_columns(table: str) -> tuple[str, ...]:
    column = TIME_COLUMN.get(table, "start_utc")
    return tuple(column if c == "start_utc" else c for c in REQUIRED_COLUMNS if c != "start_utc" or column)
IGNORED_FOR_CONTENT = frozenset({"source_row", "raw_payload", "quality_flags"})
TZ_BASIS = {"source_declared": "declared", "analyst_confirmed": "confirmed_by_analyst", "profile_defaulted": "assumed_by_profile"}


def ingest_v2_mode(value: str | None) -> str:
    """`off` (the default) or `shadow`; anything else is `off`, with a warning, so a typo never turns a write on."""
    text = (value or "").strip().lower()
    if text in ("", "off", "0", "false", "no"):
        return "off"
    if text == "shadow":
        return "shadow"
    LOG.warning("FORENSIC_INGEST_V2=%r is not off or shadow: typed ingest stays off", value)
    return "off"


@dataclass(frozen=True)
class Lineage:
    """Where a file came from, as the typed tables record it."""
    tenant_id: str
    collection_id: str
    evidence_id: str
    file_id: str
    batch_id: str


def policy_from_time_policy(time_policy: Mapping[str, Any]) -> Policy:
    """The registry's policy from the worker's job time policy: the declared zone, how it is known, and the day-or-month order."""
    zone = time_policy.get("source_timezone") or None
    basis = TZ_BASIS.get(str(time_policy.get("timezone_state") or "")) if zone else None
    order = time_policy.get("date_order") or None
    return Policy(source_tz=zone, tz_basis=basis, date_order=order)


# ---- what the database must offer ------------------------------------------------------------------------------------------------------------

class Sink(Protocol):
    def table_columns(self, table: str) -> list[tuple[str, str]]:
        """(column name, type name) of every column rows may be written to: not generated, not an identity."""

    def insert_batch(self, table: str, columns: Sequence[str], rows: Sequence[Mapping[str, Any]]) -> int:
        """Write the rows (idempotently) and return how many were new."""


# the columns rows may be written to: not generated, not an identity, not a sequence default (the `%%` is the escaped LIKE wildcard)
COLUMNS_QUERY = ("SELECT column_name, udt_name FROM information_schema.columns WHERE table_schema = 'forensic' AND table_name = %s "
                 "AND is_generated = 'NEVER' AND identity_generation IS NULL AND coalesce(column_default, '') NOT LIKE 'nextval(%%' ORDER BY ordinal_position")


def stage_sql(table: str, columns: Sequence[str]) -> str:
    cols = ", ".join(columns)
    return f"CREATE TEMP TABLE _v2_stage ON COMMIT DROP AS SELECT {cols} FROM forensic.{table} WITH NO DATA"


def copy_sql(columns: Sequence[str]) -> str:
    return f"COPY _v2_stage ({', '.join(columns)}) FROM STDIN WITH (FORMAT csv, NULL '{NO_VALUE}')"


def insert_sql(table: str, columns: Sequence[str]) -> str:
    cols = ", ".join(columns)
    return (f"WITH inserted AS (INSERT INTO forensic.{table} ({cols}) SELECT {cols} FROM _v2_stage "
            f"ON CONFLICT ({conflict_key(table)}) DO NOTHING RETURNING 1) SELECT count(*) FROM inserted")


def drop_stage_sql() -> str:
    return "DROP TABLE IF EXISTS _v2_stage"


def scrub(value: Any) -> Any:
    """PostgreSQL text cannot hold a NUL character; a file can. Drop it (the raw bytes stay in the evidence file) rather than lose the batch."""
    if isinstance(value, str):
        return value.replace("\x00", "")
    if isinstance(value, Mapping):
        return {scrub(k): scrub(v) for k, v in value.items()}
    if isinstance(value, (list, tuple, set)):
        return [scrub(v) for v in value]
    return value


def format_cell(value: Any, udt: str) -> str | None:
    """One value as the text of a CSV cell for COPY, or None for NULL: the type name of the column says how a list or a mapping is written."""
    if value is None:
        return None
    if udt in ("jsonb", "json"):
        return json.dumps(scrub(value), sort_keys=True, ensure_ascii=True, default=str)
    if isinstance(value, (list, tuple, set)):
        return "{" + ",".join('"' + scrub(str(v)).replace("\\", "\\\\").replace('"', '\\"') + '"' for v in value) + "}"
    if isinstance(value, bool):
        return "t" if value else "f"
    if isinstance(value, (datetime, date)):
        return value.isoformat()
    if isinstance(value, Decimal):
        return format(value, "f")
    return scrub(str(value))


def copy_line(cells: Iterable[str | None]) -> str:
    """One CSV line: NULL is a bare marker and every other cell is quoted, so a text value that equals the marker stays text."""
    return ",".join(NO_VALUE if c is None else '"' + c.replace('"', '""') + '"' for c in cells) + "\n"


def copy_text(columns: Sequence[str], udts: Mapping[str, str], rows: Iterable[Mapping[str, Any]]) -> str:
    return "".join(copy_line(format_cell(row.get(c), udts.get(c, "text")) for c in columns) for row in rows)


class PsycopgSink:
    """The worker's sink: the rows of a batch go through a temporary stage table and `INSERT ... ON CONFLICT DO NOTHING`, on the job's own connection."""

    def __init__(self, conn: Any) -> None:
        self.conn = conn
        self._columns: dict[str, list[tuple[str, str]]] = {}

    def table_columns(self, table: str) -> list[tuple[str, str]]:
        if table not in self._columns:
            with self.conn.cursor() as cur:
                cur.execute(COLUMNS_QUERY, (table,))
                self._columns[table] = [(r[0], r[1]) for r in cur.fetchall()]
        return self._columns[table]

    def insert_batch(self, table: str, columns: Sequence[str], rows: Sequence[Mapping[str, Any]]) -> int:
        udts = dict(self.table_columns(table))
        text = copy_text(columns, udts, rows)
        with self.conn.cursor() as cur:
            cur.execute(stage_sql(table, columns))
            with cur.copy(copy_sql(columns)) as copy:
                copy.write(text)
            cur.execute(insert_sql(table, columns))
            new = int(cur.fetchone()[0])
            cur.execute(drop_stage_sql())
        return new


# ---- reading and interpreting a file --------------------------------------------------------------------------------------------------------

class TooManyRows(ValueError):
    """A permanent input problem for a job that depends on this module (see ingest_required), so a ValueError."""


def content_of(record: Mapping[str, Any]) -> str:
    return json.dumps({k: v for k, v in record.items() if k not in IGNORED_FOR_CONTENT}, sort_keys=True, default=str)


def lineage_row(record: Mapping[str, Any], flags: list[str], lineage: Lineage, columns: Iterable[str]) -> dict[str, Any]:
    """The record as a row of the typed table: the columns the record has, and the lineage, the two hashes and the flags."""
    content = content_of(record)
    wanted = set(columns)
    row = {k: v for k, v in record.items() if k in wanted}
    row.update({
        "tenant_id": lineage.tenant_id, "collection_id": lineage.collection_id, "evidence_id": lineage.evidence_id, "file_id": lineage.file_id,
        "batch_id": lineage.batch_id, "content_hash": hashlib.sha256(content.encode("utf-8")).hexdigest(),
        "row_hash": hashlib.sha256(f"{lineage.file_id}|{record['source_row']}|{content}".encode("utf-8")).hexdigest(),
        "quality_flags": flags, "raw_payload": record.get("raw_payload"),
    })
    return row


@dataclass
class SheetSummary:
    sheet: str
    status: str = "unrecognised"            # loaded | unrecognised | ambiguous | no_header | unreadable | skipped
    reason: str | None = None
    layout_id: str | None = None
    layout_version: int | None = None
    layout_status: str | None = None
    header_row: int | None = None
    match_score: float | None = None
    table: str | None = None
    rows_read: int = 0
    loaded: int = 0
    inserted: int = 0
    already_present: int = 0
    duplicates_in_file: int = 0
    quarantined: dict[str, int] = field(default_factory=dict)
    skipped: dict[str, int] = field(default_factory=dict)
    first_rows: dict[str, list[int]] = field(default_factory=dict)
    flags: dict[str, int] = field(default_factory=dict)
    first_start_local: str | None = None
    last_start_local: str | None = None

    def reconciles(self) -> bool:
        return self.rows_read == self.loaded + sum(self.quarantined.values()) + sum(self.skipped.values()) and self.loaded == self.inserted + self.already_present

    def as_dict(self) -> dict[str, Any]:
        return {k: getattr(self, k) for k in self.__dataclass_fields__} | {"reconciles": self.reconciles()}


def _note(counter: dict[str, int], examples: dict[str, list[int]], reason: str, row_number: int | None) -> None:
    counter[reason] = counter.get(reason, 0) + 1
    if row_number is not None and len(examples.setdefault(reason, [])) < 20:
        examples[reason].append(int(row_number))


def ingest_sheet(name: str, rows: Iterator[Sequence[Any]], lineage: Lineage, policy: Policy, sink: Sink, registry: Any, *,
                 include_draft: bool = True, max_rows: int = DEFAULT_MAX_ROWS, batch_rows: int = BATCH_ROWS) -> SheetSummary:
    """One sheet: find its layout, interpret every row, write the accepted rows. Raises nothing for a file the registry does not recognise."""
    summary = SheetSummary(sheet=name)
    try:
        head = list(itertools.islice(rows, 12))
    except (csv.Error, UnicodeError) as exc:   # not a table at all (binary content under a text name): reported, nothing written
        summary.status, summary.reason = "unreadable", type(exc).__name__
        return summary
    if not head:
        summary.status, summary.reason = "no_header", "empty_sheet"
        return summary
    report = registry.find_header(head, include_draft=include_draft)
    summary.header_row = report.header_row
    summary.match_score = round(report.best.score, 4) if report.best else None
    if report.decision != "match" or not report.best:
        summary.status = "ambiguous" if report.decision == "ambiguous" else "unrecognised"
        summary.reason = report.decision
        if report.best:
            summary.layout_id = report.best.layout_id
        return summary
    entry = registry.get(report.best.layout_id)
    summary.layout_id, summary.layout_version, summary.layout_status = entry["layout_id"], int(entry.get("layout_version", 1)), entry.get("status")
    table = TABLE_FOR_FAMILY.get(entry.get("family"))
    if table is None:
        summary.status, summary.reason = "skipped", "family_without_typed_table"
        return summary
    summary.table = table
    available = [c for c, _ in sink.table_columns(table)]
    missing = [c for c in required_columns(table) if c not in available]
    if missing:
        summary.status, summary.reason = "skipped", "typed_table_lacks_" + "_".join(missing)
        return summary
    header = head[report.header_row]
    data = itertools.chain(head[report.header_row + 1:], rows)
    seen: set[bytes] = set()
    batch: list[dict[str, Any]] = []

    def flush() -> None:
        if not batch:
            return
        columns = [c for c in available if any(c in r for r in batch)]
        new = sink.insert_batch(table, columns, batch)
        summary.inserted += new
        summary.already_present += len(batch) - new
        batch.clear()

    for result in interpret_rows(entry, header, data, policy=policy):
        summary.rows_read += 1
        if summary.rows_read > max_rows:
            raise TooManyRows(f"more than {max_rows} rows")
        if result.status == "quarantined":
            _note(summary.quarantined, summary.first_rows, "quarantined:" + str(result.reason), result.row_number)
            continue
        if result.status != "accepted":
            _note(summary.skipped, summary.first_rows, "skipped:" + str(result.reason), result.row_number)
            continue
        record = dict(result.record)
        if TIME_COLUMN.get(table, "start_utc") and not (record.get("start_utc") and record.get("source_tz") and record.get("time_basis")):
            # the typed tables hold UTC time with its basis; without a declared or confirmed zone there is none, and none is guessed
            _note(summary.quarantined, summary.first_rows, "quarantined:source_timezone_unknown", result.row_number)
            continue
        flags = list(record.get("quality_flags") or [])
        digest = hashlib.sha256(content_of(record).encode("utf-8")).digest()[:16]
        if digest in seen:
            flags.append("duplicate_row")
            summary.duplicates_in_file += 1
        seen.add(digest)
        for flag in flags:
            summary.flags[flag] = summary.flags.get(flag, 0) + 1
        start = record.get("start_local")
        if start:
            text = str(start)
            summary.first_start_local = text if summary.first_start_local is None or text < summary.first_start_local else summary.first_start_local
            summary.last_start_local = text if summary.last_start_local is None or text > summary.last_start_local else summary.last_start_local
        batch.append(lineage_row(record, flags, lineage, available))
        summary.loaded += 1
        if len(batch) >= batch_rows:
            flush()
    flush()
    summary.status = "loaded"
    return summary


def ingest_file(path: str, lineage: Lineage, policy: Policy, sink: Sink, *, registry: Any = None, include_draft: bool = True,
                max_rows: int = DEFAULT_MAX_ROWS, batch_rows: int = BATCH_ROWS) -> dict[str, Any]:
    """Every sheet of a file. Returns the quality summary (no cell values): per sheet the layout, the counts and how they reconcile."""
    registry = registry or load_registry()
    out: dict[str, Any] = {"contract": CONTRACT, "evidence_id": lineage.evidence_id, "file_id": lineage.file_id, "batch_id": lineage.batch_id,
                           "policy": {"source_tz": policy.source_tz, "tz_basis": policy.tz_basis, "date_order": policy.date_order}, "sheets": []}
    if not lineage.evidence_id:
        out["status"], out["reason"] = "skipped", "no_evidence_id"
        return out
    for label, value in (("evidence_id", lineage.evidence_id), ("batch_id", lineage.batch_id)):
        try:
            uuid.UUID(str(value))
        except ValueError:
            out["status"], out["reason"] = "skipped", "invalid_" + label
            return out
    try:
        kind = readers.sniff_kind(path)
        if kind == "xlsx":
            meta, sheets = readers.iter_xlsx_sheets(path)
        elif kind == "spreadsheetml":
            meta, sheets = readers.iter_spreadsheetml_sheets(path)
        else:
            meta, sheets = readers.iter_text_rows(path)
    except readers.Unsupported as exc:
        out["status"], out["reason"] = "skipped", "unsupported_container: " + str(exc)[:120]
        return out
    out["container"] = kind
    for name, rows in sheets:
        iterator = rows()
        try:
            out["sheets"].append(ingest_sheet(str(name), iterator, lineage, policy, sink, registry, include_draft=include_draft, max_rows=max_rows, batch_rows=batch_rows).as_dict())
        finally:
            close = getattr(iterator, "close", None)   # a sheet that stops early (no layout, over the limit) must not leave its file open
            if close is not None:
                close()
    loaded = [s for s in out["sheets"] if s["status"] == "loaded"]
    out["status"] = "loaded" if loaded else "unrecognised"
    out["rows_loaded"] = sum(s["loaded"] for s in loaded)
    out["rows_inserted"] = sum(s["inserted"] for s in loaded)
    out["reconciles"] = all(s["reconciles"] for s in out["sheets"] if s["status"] == "loaded")
    return out


def run_shadow(conn: Any, path: str, lineage: Lineage, time_policy: Mapping[str, Any], *, registry: Any = None, sink: Sink | None = None,
               mode: str | None = None, max_rows: int | None = None) -> dict[str, Any] | None:
    """The worker's call: in `shadow` mode write the typed rows inside a savepoint of the job's transaction and return the summary; in `off` do nothing.

    Whatever goes wrong here is contained: the savepoint is rolled back, the failure is logged and returned as a summary with status `failed`, and the
    job goes on exactly as it would have without this module."""
    mode = ingest_v2_mode(mode if mode is not None else os.environ.get("FORENSIC_INGEST_V2"))
    if mode == "off":
        return None
    limit = max_rows if max_rows is not None else int(os.environ.get("FORENSIC_INGEST_V2_MAX_ROWS", DEFAULT_MAX_ROWS))
    include_draft = os.environ.get("FORENSIC_INGEST_V2_INCLUDE_DRAFT", "true").strip().lower() not in ("0", "false", "no")
    policy = policy_from_time_policy(time_policy)
    try:
        with conn.transaction():
            summary = ingest_file(path, lineage, policy, sink or PsycopgSink(conn), registry=registry, include_draft=include_draft, max_rows=limit)
    except TooManyRows as exc:
        LOG.warning("typed ingest skipped: %s", exc)
        return {"contract": CONTRACT, "mode": mode, "status": "skipped", "reason": "too_many_rows", "evidence_id": lineage.evidence_id, "sheets": []}
    except Exception as exc:  # noqa: BLE001 - the shadow must never fail the job
        LOG.exception("typed ingest failed; the job is not affected")
        # the type of the error and, for a database error, the constraint: never its message, which can quote a row
        constraint = getattr(getattr(exc, "diag", None), "constraint_name", None)
        return {"contract": CONTRACT, "mode": mode, "status": "failed", "reason": type(exc).__name__, "constraint": constraint,
                "evidence_id": lineage.evidence_id, "sheets": []}
    summary["mode"] = mode
    return summary


# ---- files only this module can read ------------------------------------------------------------------------------------------------------

WORKBOOK_SUFFIXES = frozenset({".xls", ".xml"})


def is_spreadsheetml(path: str) -> bool:
    """Whether the stored file is a SpreadsheetML 2003 workbook (an XML document in the Excel 2003 spreadsheet namespace). False for anything else, never an error."""
    try:
        return readers.sniff_kind(path) == "spreadsheetml"
    except (readers.Unsupported, OSError):
        return False


def ingest_required(conn: Any, path: str, lineage: Lineage, time_policy: Mapping[str, Any], *, registry: Any = None, sink: Sink | None = None,
                    mode: str | None = None, max_rows: int | None = None) -> dict[str, Any]:
    """The typed write for a file that version 1 cannot read at all (a SpreadsheetML workbook): here the typed tables are the only store, so nothing is swallowed.

    Unlike run_shadow there is no savepoint and no quiet outcome. The worker's own error handling sees every failure: a database error is retried like any other,
    and a problem with the input is a ValueError, which the worker treats as permanent (no retry, dead letter with the message): the switch is off, the file
    matches no registered layout (or matches two), there is no evidence id, or the file is over the row limit. Returns the quality summary of ingest_file.
    """
    mode = ingest_v2_mode(mode if mode is not None else os.environ.get("FORENSIC_INGEST_V2"))
    if mode == "off":
        raise ValueError("a SpreadsheetML workbook is read only by the typed ingest, and FORENSIC_INGEST_V2 is off on this worker")
    limit = max_rows if max_rows is not None else int(os.environ.get("FORENSIC_INGEST_V2_MAX_ROWS", DEFAULT_MAX_ROWS))
    include_draft = os.environ.get("FORENSIC_INGEST_V2_INCLUDE_DRAFT", "true").strip().lower() not in ("0", "false", "no")
    summary = ingest_file(path, lineage, policy_from_time_policy(time_policy), sink or PsycopgSink(conn), registry=registry, include_draft=include_draft, max_rows=limit)
    if summary["status"] != "loaded":
        states = {s["status"] for s in summary["sheets"]}
        if summary.get("reason"):
            why = str(summary["reason"])
        elif "ambiguous" in states:
            why = "the header of a sheet matches more than one registered layout"
        elif states and states <= {"unrecognised", "no_header", "unreadable"}:
            why = "no registered layout matches the header of any sheet"
        else:
            why = ", ".join(sorted("%s (%s)" % (s["status"], s.get("reason") or "-") for s in summary["sheets"])) or "the workbook has no sheet"
        raise ValueError("the workbook was not written to the typed tables: %s" % why)
    summary["mode"] = mode
    return summary


def job_counts(summary: Mapping[str, Any]) -> dict[str, int]:
    """The counts a job row keeps, from a quality summary: total = loaded + quarantined (blank and other skipped rows are not rows of data), accepted = newly inserted,
    duplicate = loaded but already present, rejected = quarantined."""
    sheets = [s for s in summary["sheets"] if s["status"] == "loaded"]
    loaded = sum(s["loaded"] for s in sheets)
    inserted = sum(s["inserted"] for s in sheets)
    quarantined = sum(sum(s["quarantined"].values()) for s in sheets)
    return {"total_rows": loaded + quarantined, "accepted_rows": inserted, "duplicate_rows": loaded - inserted, "rejected_rows": quarantined,
            "skipped_rows": sum(sum(s["skipped"].values()) for s in sheets)}
