#!/usr/bin/env python3
"""Privacy-safe, read-only R7 protected CDR compatibility audit."""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import sys
import time
from collections import Counter
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO_ROOT))

from ingestion.forensic_records.worker import IngestJob, normalize_cdr_row  # noqa: E402


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("source", type=Path)
    args = parser.parse_args()
    source = args.source.resolve(strict=True)
    digest = hashlib.sha256(source.read_bytes()).hexdigest()
    job = IngestJob(
        job_id="00000000-0000-0000-0000-000000000007",
        tenant_id="protected-audit",
        user_id="protected-audit",
        collection_id="r7-read-only",
        file_id="sha256:" + digest,
        source_file="protected-cdr.csv",
        spool_path=str(source),
        sha256=digest,
        record_type="cdr",
        headers=[],
        metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
    )

    counters: Counter[str] = Counter()
    quality_flags: Counter[str] = Counter()
    directions: Counter[str] = Counter()
    services: Counter[str] = Counter()
    timestamp_bases: Counter[str] = Counter()
    row_hashes: set[str] = set()
    started = time.perf_counter()
    with source.open(newline="", encoding="utf-8-sig") as handle:
        reader = csv.DictReader(handle)
        header_count = len(reader.fieldnames or [])
        for row_number, raw in enumerate(reader, start=2):
            counters["rows"] += 1
            try:
                record = normalize_cdr_row(job, raw, "r7-protected-read-only", row_number)
            except Exception:  # Row content must never enter the audit output.
                counters["normalization_errors"] += 1
                continue
            counters["normalized"] += 1
            row_hashes.add(str(record.get("row_hash", "")))
            if record.get("source_file") and record.get("row_number") and record.get("row_hash"):
                counters["source_locator_complete"] += 1
            if record.get("originating_number_role") and record.get("called_number_role"):
                counters["party_roles_complete"] += 1
            if record.get("source_timezone") and record.get("timestamp_basis") and record.get("canonical_timezone") == "UTC":
                counters["timezone_provenance_complete"] += 1
            if record.get("cell_site_id") or record.get("location"):
                counters["location_context_present"] += 1
            if record.get("latitude") is not None and record.get("longitude") is not None:
                counters["supplied_coordinates_present"] += 1
            if record.get("manual_review_required"):
                counters["manual_review_required"] += 1
            quality_flags.update(str(flag) for flag in record.get("quality_flags", []))
            directions[str(record.get("direction_canonical") or "UNSPECIFIED")] += 1
            services[str(record.get("service_class") or "UNSPECIFIED")] += 1
            timestamp_bases[str(record.get("timestamp_basis") or "UNSPECIFIED")] += 1

    rows = counters["rows"]
    result = {
        "contract_version": "forensics.r7-protected-cdr-audit/v1",
        "mode": "ephemeral_read_only",
        "source_sha256": digest,
        "source_bytes": source.stat().st_size,
        "header_column_count": header_count,
        "rows": rows,
        "normalized_rows": counters["normalized"],
        "normalization_errors": counters["normalization_errors"],
        "unique_row_hashes": len(row_hashes),
        "duplicate_rows_by_hash": rows - len(row_hashes),
        "source_locator_complete_rows": counters["source_locator_complete"],
        "party_roles_complete_rows": counters["party_roles_complete"],
        "timezone_provenance_complete_rows": counters["timezone_provenance_complete"],
        "location_context_rows": counters["location_context_present"],
        "supplied_coordinate_rows": counters["supplied_coordinates_present"],
        "manual_review_required_rows": counters["manual_review_required"],
        "quality_flag_counts": dict(sorted(quality_flags.items())),
        "direction_counts": dict(sorted(directions.items())),
        "service_class_counts": dict(sorted(services.items())),
        "timestamp_basis_counts": dict(sorted(timestamp_bases.items())),
        "elapsed_ms": round((time.perf_counter() - started) * 1000, 2),
        "raw_rows_emitted": False,
        "raw_identifiers_emitted": False,
        "evidence_writes": 0,
        "database_queries": 0,
        "model_calls": 0,
    }
    if rows == 0 or counters["normalized"] != rows or counters["normalization_errors"]:
        raise SystemExit("protected CDR normalization acceptance failed")
    if counters["source_locator_complete"] != rows or counters["party_roles_complete"] != rows:
        raise SystemExit("protected CDR provenance or party-role acceptance failed")
    if counters["timezone_provenance_complete"] != rows:
        raise SystemExit("protected CDR timezone provenance acceptance failed")
    print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
