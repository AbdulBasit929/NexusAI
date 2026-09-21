#!/usr/bin/env python3
"""Read-only microbenchmark for the Phase 6.1 CDR normalization adapter."""

from __future__ import annotations

import argparse
import csv
import json
import sys
import time
import tracemalloc
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO_ROOT))

from ingestion.forensic_records.worker import IngestJob, normalize_cdr_row  # noqa: E402


DEFAULT_FIXTURE = REPO_ROOT / "ingestion" / "forensic_records" / "tests" / "fixtures" / "pakistan_cdr_messy_synthetic.csv"
ADAPTER_ROOT = REPO_ROOT / "ingestion" / "forensic_records" / "adapters" / "cdr"


def load_rows(path: Path) -> list[dict[str, str]]:
    with path.open(newline="", encoding="utf-8-sig") as handle:
        return list(csv.DictReader(handle))


def job_for(path: Path) -> IngestJob:
    return IngestJob(
        job_id="00000000-0000-0000-0000-000000000006",
        tenant_id="benchmark",
        user_id="benchmark",
        collection_id="phase6-cdr-read-only",
        file_id="phase6-cdr-fixture",
        source_file=path.name,
        spool_path=str(path),
        sha256="6" * 64,
        record_type="cdr",
        headers=[],
        metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
    )


def normalize_pass(job: IngestJob, rows: list[dict[str, str]], batch_id: str) -> list[dict[str, object]]:
    return [normalize_cdr_row(job, row, batch_id, index) for index, row in enumerate(rows, start=2)]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--fixture", type=Path, default=DEFAULT_FIXTURE)
    parser.add_argument("--iterations", type=int, default=2000)
    args = parser.parse_args()
    rows = load_rows(args.fixture)
    job = job_for(args.fixture)
    batch_id = "00000000-0000-0000-0000-000000000006"

    tracemalloc.start()
    cold_started = time.perf_counter_ns()
    cold = normalize_pass(job, rows, batch_id)
    cold_ns = time.perf_counter_ns() - cold_started

    warm_started = time.perf_counter_ns()
    final = cold
    for _ in range(max(1, args.iterations)):
        final = normalize_pass(job, rows, batch_id)
    warm_ns = time.perf_counter_ns() - warm_started
    _, peak_bytes = tracemalloc.get_traced_memory()
    tracemalloc.stop()

    processed = len(rows) * max(1, args.iterations)
    package_bytes = sum(path.stat().st_size for path in ADAPTER_ROOT.rglob("*") if path.is_file() and "__pycache__" not in path.parts)
    result = {
        "contract_version": "forensics.cdr-benchmark/v1",
        "fixture_name": args.fixture.name,
        "fixture_rows": len(rows),
        "iterations": max(1, args.iterations),
        "cold_pass_ms": round(cold_ns / 1_000_000, 4),
        "warm_row_us": round((warm_ns / 1000) / max(1, processed), 4),
        "warm_rows_per_second": round(processed / max(warm_ns / 1_000_000_000, 1e-9), 2),
        "peak_python_allocation_bytes": peak_bytes,
        "adapter_package_bytes": package_bytes,
        "accepted_rows": len(final),
        "unique_row_hashes": len({str(record["row_hash"]) for record in final}),
        "duplicate_rows_by_hash": len(final) - len({str(record["row_hash"]) for record in final}),
        "evidence_writes": 0,
        "database_queries": 0,
        "model_calls": 0,
    }
    print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
