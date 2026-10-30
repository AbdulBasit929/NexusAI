"""The mention index of text documents: rows for forensic.document_mention (migration 031) from the passages the document pipeline produced.

    rows = mention_rows(result.passages, places)            # pure: the extractor's mentions as table rows
    CursorSink(cur).write(tenant_id, collection_id, evidence_id, rows)

The worker calls this right after `persist_document_result` for a document (`run_document_mentions_shadow`), when FORENSIC_DOCUMENT_MENTIONS is on and FORENSIC_DOCUMENT_GAZETTEER names the places file (off by default: the table exists only after migration 031 and the worker
overlay are applied, a separate owner decision). The passage text is the pipeline's own and is not touched; this stores what the rule-based extractor found in it.

National ID numbers are counted and withheld: a row of kind national_id carries no key and no text (identity data stays dark until the masking design is approved, as in migration 012), and the
table has a check constraint that keeps it so. A mention says that a document names the key. It never says that the holder took part in anything.
"""
from __future__ import annotations

import json
from typing import Any, Iterable, Mapping, Sequence

try:
    from .mention_extractor import EXTRACTOR_ID, EXTRACTOR_REVISION, extract_mentions
except ImportError:  # the worker's flat layout (/app): the extractor is a sibling module
    from mention_extractor import EXTRACTOR_ID, EXTRACTOR_REVISION, extract_mentions

WITHHELD_KINDS = frozenset({"national_id"})

INSERT_SQL = (
    "INSERT INTO forensic.document_mention (tenant_id, collection_id, evidence_id, artifact_id, kind, value, surface, tier, rule, char_start, char_end, locator, extractor, extractor_revision) "
    "VALUES (%s, %s, %s::uuid, %s::uuid, %s, %s, %s, %s, %s, %s, %s, %s::jsonb, %s, %s) "
    "ON CONFLICT (tenant_id, collection_id, artifact_id, kind, char_start, char_end, extractor_revision) DO NOTHING"
)


def mention_rows(passages: Sequence[Any], places: Mapping[str, Sequence[str]] | None = None, *, include_low: bool = False) -> list[dict[str, Any]]:
    """The rows of the passages of ONE document, in document order. Each passage is read with the text of the passage before it as context (the neighbouring table cell)."""
    rows: list[dict[str, Any]] = []
    previous = ""
    for passage in passages:
        before, previous = previous, passage.text
        for mention in extract_mentions(passage.text, places, include_low=include_low, context_before=before):
            withheld = mention.kind in WITHHELD_KINDS
            rows.append({
                "artifact_id": passage.artifact_id,
                "kind": mention.kind,
                "value": None if withheld else mention.value,
                "surface": None if withheld else mention.surface,
                "tier": mention.tier,
                "rule": mention.rule,
                "char_start": mention.start,
                "char_end": mention.end,
                "locator": dict(passage.locator),
                "extractor": EXTRACTOR_ID,
                "extractor_revision": EXTRACTOR_REVISION,
            })
    return rows


def insert_params(tenant_id: str, collection_id: str, evidence_id: str, row: Mapping[str, Any]) -> tuple:
    return (tenant_id, collection_id, evidence_id, row["artifact_id"], row["kind"], row["value"], row["surface"], row["tier"], row["rule"], row["char_start"], row["char_end"],
            json.dumps(row["locator"], sort_keys=True), row["extractor"], row["extractor_revision"])


class CursorSink:
    """Writes rows through a DB-API cursor (the worker's psycopg cursor). Idempotent: a second run over the same passages inserts nothing."""

    def __init__(self, cursor: Any) -> None:
        self.cursor = cursor

    def write(self, tenant_id: str, collection_id: str, evidence_id: str, rows: Iterable[Mapping[str, Any]]) -> int:
        count = 0
        for row in rows:
            self.cursor.execute(INSERT_SQL, insert_params(tenant_id, collection_id, evidence_id, row))
            count += 1
        return count


def summarise(rows: Sequence[Mapping[str, Any]]) -> dict[str, int]:
    """Counts by kind, for the job's audit record (never the keys)."""
    out: dict[str, int] = {}
    for row in rows:
        out[row["kind"]] = out.get(row["kind"], 0) + 1
    return out
