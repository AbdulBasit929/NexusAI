"""Header matching: which layout is this file?

A header row is matched against every layout entry by normalised column names. Names are normalised exactly as the worker does
(`worker.normalize_header`), duplicate names are told apart by occurrence (the xlsx reader renders the second `Cell_ID` as
`Cell_ID__duplicate_2`), declared aliases cover known header typos, and near misses are reported rather than silently accepted.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Iterable, Mapping, Sequence
import re

from .spec import column_ref

DUPLICATE_SUFFIX = re.compile(r"__duplicate_(\d+)$")


def normalize_header(value: Any) -> str:
    """Identical to ingestion.forensic_records.worker.normalize_header (a test keeps the two in step)."""
    text = ("" if value is None else str(value)).strip().lower()
    chars: list[str] = []
    last_underscore = False
    for char in text:
        if char.isalnum():
            chars.append(char)
            last_underscore = False
            continue
        if not last_underscore:
            chars.append("_")
            last_underscore = True
    return "".join(chars).strip("_")


@dataclass(frozen=True)
class HeaderCell:
    position: int
    raw: str
    norm: str
    occurrence: int


def header_cells(raw_cells: Iterable[Any]) -> list[HeaderCell]:
    """Non-blank header cells with their normalised name and occurrence among equal names."""
    cells: list[HeaderCell] = []
    counts: dict[str, int] = {}
    for position, raw in enumerate(raw_cells):
        text = ("" if raw is None else str(raw)).strip()
        if not text:
            continue
        suffix = DUPLICATE_SUFFIX.search(text)
        base = text[: suffix.start()] if suffix else text
        norm = normalize_header(base)
        if not norm:
            continue
        counts[norm] = counts.get(norm, 0) + 1
        cells.append(HeaderCell(position, text, norm, int(suffix.group(1)) if suffix else counts[norm]))
    return cells


def entry_columns(entry: Mapping[str, Any]) -> list[tuple[str, int, str]]:
    """The entry's header columns as (name, occurrence, normalised name), in order."""
    columns: list[tuple[str, int, str]] = []
    counts: dict[str, int] = {}
    for column in entry["header"]["columns"]:
        name = column["name"]
        counts[name] = counts.get(name, 0) + 1
        columns.append((name, counts[name], normalize_header(name)))
    return columns


def _edit_distance_at_most_one(a: str, b: str) -> bool:
    if a == b:
        return True
    if abs(len(a) - len(b)) > 1:
        return False
    if len(a) == len(b):
        diffs = [i for i in range(len(a)) if a[i] != b[i]]
        if len(diffs) == 1:
            return True
        return len(diffs) == 2 and diffs[1] == diffs[0] + 1 and a[diffs[0]] == b[diffs[1]] and a[diffs[1]] == b[diffs[0]]
    short, long_ = (a, b) if len(a) < len(b) else (b, a)
    for i in range(len(long_)):
        if long_[:i] + long_[i + 1:] == short:
            return True
    return False


@dataclass
class LayoutMatch:
    layout_id: str
    layout_version: int
    status: str
    score: float
    required_ok: bool
    qualifies: bool
    matched: int
    entry_columns: int
    file_columns: int
    by_alias: list[tuple[str, str]] = field(default_factory=list)
    missing_required: list[str] = field(default_factory=list)
    missing_optional: list[str] = field(default_factory=list)
    extra_columns: list[str] = field(default_factory=list)
    near_misses: list[tuple[str, str]] = field(default_factory=list)
    positions: dict[tuple[str, int], int] = field(default_factory=dict)  # entry column (name, occurrence) -> file column position

    def summary(self) -> str:
        return "%s v%d score %.2f%s%s" % (
            self.layout_id, self.layout_version, self.score, "" if self.required_ok else " (required columns missing)",
            " [draft]" if self.status == "draft" else "")


@dataclass
class MatchReport:
    decision: str                      # match | ambiguous | none
    best: LayoutMatch | None
    candidates: list[LayoutMatch]
    margin: float
    header_row: int | None = None

    @property
    def matched_entry_id(self) -> str | None:
        return self.best.layout_id if self.decision == "match" and self.best else None


def score_entry(entry: Mapping[str, Any], cells: Sequence[HeaderCell]) -> LayoutMatch:
    columns = entry_columns(entry)
    aliases = {column_ref(name): alts for name, alts in (entry["header"].get("aliases") or {}).items()}
    required = {column_ref(name) for name in entry["header"].get("required", [])}
    available = {(cell.norm, cell.occurrence): cell for cell in cells}
    used: set[tuple[str, int]] = set()
    positions: dict[tuple[str, int], int] = {}
    by_alias: list[tuple[str, str]] = []
    matched = 0
    missing: list[tuple[str, int]] = []
    for name, occurrence, norm in columns:
        key = (norm, occurrence)
        cell = available.get(key) if key not in used else None
        alias_used = None
        if cell is None:
            for alternative in aliases.get((name, occurrence), []):
                alt_key = (normalize_header(alternative), occurrence)
                if alt_key in available and alt_key not in used:
                    cell, key, alias_used = available[alt_key], alt_key, alternative
                    break
        if cell is None:
            missing.append((name, occurrence))
            continue
        used.add(key)
        matched += 1
        positions[(name, occurrence)] = cell.position
        if alias_used is not None:
            by_alias.append((name, cell.raw))
    extra = [cell for cell in cells if (cell.norm, cell.occurrence) not in used]
    missing_required = [name for (name, occurrence) in missing if (name, occurrence) in required]
    missing_optional = [name for (name, occurrence) in missing if (name, occurrence) not in required]
    near: list[tuple[str, str]] = []
    for name, occurrence in missing:
        norm = normalize_header(name)
        for cell in extra:
            if len(norm) >= 6 and cell.occurrence == occurrence and _edit_distance_at_most_one(norm, cell.norm):
                near.append((name, cell.raw))
    # A column the layout lists as optional and the file does not have costs nothing: a Telenor-style export without its two trailing optional columns is still that layout
    # (found 2026-10-15: the synthetic demonstration files lack them and were read as a tower-dump layout that has one column fewer and no counterparty). A column the
    # file has and the layout does not know still costs, as before.
    denominator = max(len(columns) - len(missing_optional), len(cells)) or 1
    score = matched / denominator
    min_score = float(entry["header"].get("min_score", 0.85))
    required_ok = not missing_required
    return LayoutMatch(
        layout_id=entry["layout_id"], layout_version=entry["layout_version"], status=entry["status"], score=round(score, 4),
        required_ok=required_ok, qualifies=required_ok and score >= min_score, matched=matched, entry_columns=len(columns),
        file_columns=len(cells), by_alias=by_alias, missing_required=missing_required, missing_optional=missing_optional,
        extra_columns=[cell.raw for cell in extra], near_misses=near, positions=positions)


def match_header(entries: Iterable[Mapping[str, Any]], raw_cells: Iterable[Any], *, include_draft: bool = True) -> MatchReport:
    """Rank every entry against one header row and decide: match, ambiguous (two entries qualify closely) or none."""
    cells = header_cells(raw_cells)
    pool = [e for e in entries if e["status"] == "active" or (include_draft and e["status"] == "draft")]
    scored = [score_entry(entry, cells) for entry in pool]
    scored.sort(key=lambda m: (m.qualifies, m.required_ok, m.score, -len(m.by_alias)), reverse=True)
    if not scored or not scored[0].qualifies:
        return MatchReport("none", scored[0] if scored else None, scored, 0.0)
    best = scored[0]
    others = [m for m in scored[1:] if m.qualifies]
    margin = round(best.score - others[0].score, 4) if others else 1.0
    return MatchReport("ambiguous" if others and margin < 0.05 else "match", best, scored, margin)


def find_header_row(entries: Iterable[Mapping[str, Any]], rows: Sequence[Sequence[Any]], *, max_rows: int = 12,
                    include_draft: bool = True) -> MatchReport:
    """Try each of the first rows as the header (preamble lines come first in several layouts) and keep the best decision."""
    pool = list(entries)
    best: MatchReport | None = None
    for index, row in enumerate(rows[:max_rows]):
        if len([c for c in row if str(c if c is not None else "").strip()]) < 4:
            continue
        report = match_header(pool, row, include_draft=include_draft)
        report.header_row = index
        rank = (report.decision == "match", report.best.score if report.best else 0.0)
        if best is None or rank > (best.decision == "match", best.best.score if best.best else 0.0):
            best = report
        if report.decision == "match" and report.best and report.best.score >= 0.999:
            break
    return best or MatchReport("none", None, [], 0.0)
