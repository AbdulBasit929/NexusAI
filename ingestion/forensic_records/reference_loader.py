"""Reference loader: tenant-wide reference lists (not evidence rows) into the reference tables of migration 013.

The first list is the device reference: a Type Allocation Code (the first eight digits of an IMEI) to make and model, into `forensic.device_tac`. A row of it says "the TAC is listed as this
make and model per this source", never a fact about an individual handset; the source and its licence are recorded with every row (the full GSMA list is licensed, a list is loaded only
under terms that allow it).

What the loader decides, and counts (never a value): a seven-digit TAC lost its leading zero in a spreadsheet and is padded and flagged; a TAC of any other length, a non-numeric or an empty one
is rejected and counted by reason; the first row of a TAC wins within a file, a later row of the same TAC is counted as a repeat (the same make and model) or a conflict (another one) and is not
written, because the table's key is one row per TAC and source. Loading is idempotent (a second load writes nothing). It never overwrites another source's row.

    python -X utf8 -m ingestion.forensic_records.reference_loader --check C:/tmp/nexusai-family-data/tac/tac_full.csv     # counts only, no database
"""

from __future__ import annotations

import argparse
import csv
import json
import sys
from datetime import datetime, timezone
from typing import Any, Iterable, Iterator, Mapping, Sequence

try:
    from ingestion.forensic_records.layout_registry.parties import normalize_tac
    from ingestion.forensic_records.typed_ingest import BATCH_ROWS
except ModuleNotFoundError:  # pragma: no cover - supports direct execution inside the worker image
    from layout_registry.parties import normalize_tac
    from typed_ingest import BATCH_ROWS

TABLE = "device_tac"
TAC_HEADERS = ("tac",)
MAKE_HEADERS = ("brand", "make", "manufacturer")
MODEL_HEADERS = ("specs", "model", "name")


class ReferenceError(ValueError):
    """The file is not a list this loader can read (no TAC column)."""


def _pick(header: Sequence[str], names: Sequence[str]) -> int | None:
    lowered = [h.strip().lower() for h in header]
    for name in names:
        if name in lowered:
            return lowered.index(name)
    return None


def read_tac_rows(path: str) -> Iterator[dict[str, str]]:
    """The rows of a TAC list as {tac, make, model}: any header spelling of the three columns; a UTF-8 byte-order mark is tolerated."""
    with open(path, encoding="utf-8-sig", newline="") as handle:
        reader = csv.reader(handle)
        header = next(reader, None)
        if not header:
            raise ReferenceError("empty file")
        at_tac, at_make, at_model = _pick(header, TAC_HEADERS), _pick(header, MAKE_HEADERS), _pick(header, MODEL_HEADERS)
        if at_tac is None:
            raise ReferenceError("no TAC column")
        for row in reader:
            def cell(i: int | None) -> str:
                return row[i].strip() if i is not None and i < len(row) else ""
            yield {"tac": cell(at_tac), "make": cell(at_make), "model": cell(at_model)}


def build_rows(items: Iterable[Mapping[str, str]], *, source: str, source_license: str, tenant_id: str, retrieved_at: str | None) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    """The device_tac rows and the counts of a list. No value is kept in the counts."""
    counts: dict[str, Any] = {"rows_read": 0, "accepted": 0, "rejected": {}, "flags": {}, "repeats": 0, "conflicts": 0}
    first: dict[str, tuple[str, str]] = {}
    out: list[dict[str, Any]] = []
    for item in items:
        counts["rows_read"] += 1
        tac, flags = normalize_tac(item.get("tac"))
        if tac is None:
            reason = flags[0] if flags else "tac_unreadable"
            counts["rejected"][reason] = counts["rejected"].get(reason, 0) + 1
            continue
        for flag in flags:
            counts["flags"][flag] = counts["flags"].get(flag, 0) + 1
        make, model = item.get("make") or None, item.get("model") or None
        if tac in first:
            counts["repeats" if first[tac] == (make, model) else "conflicts"] += 1
            continue
        first[tac] = (make, model)
        counts["accepted"] += 1
        out.append({"tenant_id": tenant_id, "tac": tac, "source": source, "make": make, "model": model, "marketing_name": (model or "").split(",")[0].strip() or None,
                    "device_type": None, "source_license": source_license, "retrieved_at": retrieved_at, "notes": ";".join(flags) or None})
    counts["rejected_total"] = sum(counts["rejected"].values())
    counts["distinct_makes"] = len({m for m, _ in first.values() if m})
    return out, counts


def load_tac_list(path: str, sink: Any, *, source: str, source_license: str, tenant_id: str = "default", retrieved_at: str | None = None,
                  batch_rows: int = BATCH_ROWS) -> dict[str, Any]:
    """Read a TAC list and write it through a sink (the typed ingest's: `insert_batch(table, columns, rows)` is idempotent and returns the number of new rows). Returns the counts."""
    if not source or not source_license:
        raise ReferenceError("a list is loaded with its source and its licence")
    retrieved_at = retrieved_at or datetime.now(timezone.utc).isoformat()
    rows, counts = build_rows(read_tac_rows(path), source=source, source_license=source_license, tenant_id=tenant_id, retrieved_at=retrieved_at)
    inserted = 0
    columns = [c for c, _ in sink.table_columns(TABLE) if c in rows[0]] if rows else []
    for start in range(0, len(rows), batch_rows):
        inserted += sink.insert_batch(TABLE, columns, rows[start:start + batch_rows])
    counts.update({"table": TABLE, "source": source, "inserted": inserted, "already_present": counts["accepted"] - inserted})
    return counts


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Count what a TAC list would load (no database).")
    parser.add_argument("--check", required=True, help="a TAC list (CSV)")
    args = parser.parse_args(argv)
    _, counts = build_rows(read_tac_rows(args.check), source="check", source_license="check", tenant_id="check", retrieved_at=None)
    print(json.dumps(counts, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
