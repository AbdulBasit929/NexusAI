"""Provider layout registry (v1): which layout is this file, and what do its columns mean.

    from ingestion.forensic_records.layout_registry import load_registry
    registry = load_registry()
    report = registry.find_header(rows)            # rows: the first lines of a sheet, as lists of cells
    entry = registry.get(report.best.layout_id)

Layouts are data (layouts/*.json); this package holds the specification, the loader, the header matcher and a reference
interpreter. Nothing here reads a real file or touches a database. See docs/data/layout-registry.md.
"""

from .interpret import Policy, RowResult, interpret_record, interpret_rows, rows_to_records, summarize
from .matching import LayoutMatch, MatchReport, header_cells, match_header, normalize_header
from .registry import LAYOUT_DIR, LayoutRegistryError, Registry, definition_sha256, load_registry
from .spec import SCHEMA, validate_entry

__all__ = [
    "LAYOUT_DIR", "LayoutMatch", "LayoutRegistryError", "MatchReport", "Policy", "Registry", "RowResult", "SCHEMA",
    "definition_sha256", "header_cells", "interpret_record", "interpret_rows", "load_registry", "match_header",
    "normalize_header", "rows_to_records", "summarize", "validate_entry",
]
