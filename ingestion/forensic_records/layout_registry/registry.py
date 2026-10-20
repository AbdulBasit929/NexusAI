"""Loading, hashing and registering layout entries.

Entries live in layouts/*.json, one file per layout version (`<layout_id>.v<version>.json`). The definition hash covers
everything except the life-cycle keys (status, confirmation), so promoting a layout from draft to active does not change
what the version means, while any change to a rule needs a new version.
"""

from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any, Iterable, Mapping

from .matching import MatchReport, find_header_row, match_header
from .spec import MUTABLE_KEYS, validate_entry

LAYOUT_DIR = Path(__file__).with_name("layouts")


class LayoutRegistryError(ValueError):
    """Raised when layout entries are invalid, duplicated or inconsistent."""


def canonical_json(obj: Any) -> str:
    return json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def definition_sha256(entry: Mapping[str, Any]) -> str:
    """SHA-256 of the entry without its life-cycle keys: the value stored in forensic.layout_versions.definition_sha256."""
    definition = {k: v for k, v in entry.items() if k not in MUTABLE_KEYS}
    return hashlib.sha256(canonical_json(definition).encode("ascii")).hexdigest()


def sql_literal(value: object) -> str:
    return "NULL" if value is None else "'" + str(value).replace("'", "''") + "'"


class Registry:
    def __init__(self, entries: Iterable[Mapping[str, Any]]):
        self.entries: list[dict[str, Any]] = [dict(e) for e in entries]
        self.hashes: dict[tuple[str, int], str] = {(e["layout_id"], e["layout_version"]): definition_sha256(e) for e in self.entries}

    def __len__(self) -> int:
        return len(self.entries)

    def ids(self) -> list[str]:
        return sorted(e["layout_id"] for e in self.entries)

    def get(self, layout_id: str, version: int | None = None) -> dict[str, Any]:
        candidates = [e for e in self.entries if e["layout_id"] == layout_id and (version is None or e["layout_version"] == version)]
        if not candidates:
            raise KeyError("no layout %s%s" % (layout_id, "" if version is None else " v%d" % version))
        return max(candidates, key=lambda e: e["layout_version"])

    def current_entries(self) -> list[dict[str, Any]]:
        """The latest version of each layout. Matching reads these only: a superseded version stays for history and for the rows already typed under it, and would
        otherwise tie with its successor on every header (and make every match ambiguous)."""
        latest: dict[str, int] = {}
        for e in self.entries:
            latest[e["layout_id"]] = max(latest.get(e["layout_id"], 0), e["layout_version"])
        return [e for e in self.entries if e["layout_version"] == latest[e["layout_id"]]]

    def match(self, header_cells: Iterable[Any], *, include_draft: bool = True) -> MatchReport:
        return match_header(self.current_entries(), header_cells, include_draft=include_draft)

    def find_header(self, rows: list, *, include_draft: bool = True, max_rows: int = 12) -> MatchReport:
        return find_header_row(self.current_entries(), rows, include_draft=include_draft, max_rows=max_rows)

    def register_sql(self, registered_by: str = "layout-registry") -> str:
        """SQL that records every entry in forensic.layout_versions and refuses a changed definition under an old version."""
        lines = ["-- Generated from ingestion/forensic_records/layout_registry/layouts; idempotent.", "BEGIN;"]
        for entry in sorted(self.entries, key=lambda e: (e["layout_id"], e["layout_version"])):
            key = (entry["layout_id"], entry["layout_version"])
            digest = self.hashes[key]
            lines.append(
                "DO $$ BEGIN IF EXISTS (SELECT 1 FROM forensic.layout_versions WHERE layout_id = %s AND layout_version = %d "
                "AND definition_sha256 <> %s) THEN RAISE EXCEPTION 'layout %% v%% changed without a version bump', %s, %d; END IF; END $$;"
                % (sql_literal(key[0]), key[1], sql_literal(digest), sql_literal(key[0]), key[1]))
            lines.append(
                "INSERT INTO forensic.layout_versions (layout_id, layout_version, family, dataset_kind, operator_label, "
                "definition_sha256, status, registered_by, notes) VALUES (%s, %d, %s, %s, %s, %s, %s, %s, %s) "
                "ON CONFLICT (layout_id, layout_version) DO NOTHING;"
                % (sql_literal(key[0]), key[1], sql_literal(entry["family"]), sql_literal(entry["dataset_kind"]),
                   sql_literal(entry.get("operator_label")), sql_literal(digest), sql_literal(entry["status"]),
                   sql_literal(registered_by), sql_literal(entry["label"])))
        lines.append("COMMIT;")
        return "\n".join(lines) + "\n"


def load_registry(directory: Path | str = LAYOUT_DIR) -> Registry:
    directory = Path(directory)
    problems: list[str] = []
    entries: list[dict[str, Any]] = []
    seen: dict[tuple[str, int], Path] = {}
    for path in sorted(directory.glob("*.json")):
        try:
            entry = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError) as exc:
            problems.append("%s: cannot read (%s)" % (path.name, exc))
            continue
        for problem in validate_entry(entry):
            problems.append("%s: %s" % (path.name, problem))
        layout_id, version = entry.get("layout_id"), entry.get("layout_version")
        if isinstance(layout_id, str) and isinstance(version, int):
            expected = "%s.v%d.json" % (layout_id, version)
            if path.name != expected:
                problems.append("%s: file name must be %s" % (path.name, expected))
            if (layout_id, version) in seen:
                problems.append("%s: duplicates %s" % (path.name, seen[(layout_id, version)].name))
            seen[(layout_id, version)] = path
        entries.append(entry)
    if problems:
        raise LayoutRegistryError("; ".join(problems))
    return Registry(entries)
