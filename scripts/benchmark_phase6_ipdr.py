#!/usr/bin/env python3
"""Read-only microbenchmark for the Phase 6.2 IPDR normalization adapter."""

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

from ingestion.forensic_records.worker import ADAPTERS, IngestJob, normalize_generic_row  # noqa: E402


DEFAULT_FIXTURE = REPO_ROOT / "ingestion" / "forensic_records" / "tests" / "fixtures" / "pakistan_ipdr_messy_synthetic.csv"
ADAPTER_ROOT = REPO_ROOT / "ingestion" / "forensic_records" / "adapters" / "ipdr"


def load_rows(path: Path) -> list[dict[str, str]]:
    with path.open(newline="", encoding="utf-8-sig") as handle:
        return list(csv.DictReader(handle))


def job_for(path: Path) -> IngestJob:
    return IngestJob(
        job_id="00000000-0000-0000-0000-000000000062",
        tenant_id="benchmark",
        user_id="benchmark",
        collection_id="phase6-ipdr-read-only",
        file_id="phase6-ipdr-fixture",
        source_file=path.name,
        spool_path=str(path),
        sha256="2" * 64,
        record_type="ipdr",
        headers=[],
        metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
    )


def normalize_pass(job: IngestJob, rows: list[dict[str, str]], batch_id: str):
    accepted = []
    rejected = []
    for index, row in enumerate(rows, start=2):
        try:
            accepted.append(normalize_generic_row(job, row, batch_id, index, ADAPTERS["ipdr"]))
        except ValueError as exc:
            rejected.append(str(exc))
    return accepted, rejected


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--fixture", type=Path, default=DEFAULT_FIXTURE)
    parser.add_argument("--iterations", type=int, default=2000)
    args = parser.parse_args()
    rows = load_rows(args.fixture)
    job = job_for(args.fixture)
    batch_id = "00000000-0000-0000-0000-000000000062"

    tracemalloc.start()
    cold_started = time.perf_counter_ns()
    cold, _ = normalize_pass(job, rows, batch_id)
    cold_ns = time.perf_counter_ns() - cold_started

    warm_started = time.perf_counter_ns()
    final, rejected = cold, []
    iterations = max(1, args.iterations)
    for _ in range(iterations):
        final, rejected = normalize_pass(job, rows, batch_id)
    warm_ns = time.perf_counter_ns() - warm_started
    _, peak_bytes = tracemalloc.get_traced_memory()
    tracemalloc.stop()

    processed = len(rows) * iterations
    hashes = {str(record["row_hash"]) for record in final}
    package_bytes = sum(path.stat().st_size for path in ADAPTER_ROOT.rglob("*") if path.is_file() and "__pycache__" not in path.parts)
    result = {
        "contract_version": "forensics.ipdr-benchmark/v1",
        "fixture_name": args.fixture.name,
        "fixture_rows": len(rows),
        "iterations": iterations,
        "cold_pass_ms": round(cold_ns / 1_000_000, 4),
        "warm_input_row_us": round((warm_ns / 1000) / max(1, processed), 4),
        "warm_input_rows_per_second": round(processed / max(warm_ns / 1_000_000_000, 1e-9), 2),
        "peak_python_allocation_bytes": peak_bytes,
        "adapter_package_bytes": package_bytes,
        "accepted_rows": len(final),
        "rejected_rows": len(rejected),
        "unique_row_hashes": len(hashes),
        "duplicate_rows_by_hash": len(final) - len(hashes),
        "evidence_writes": 0,
        "database_queries": 0,
        "model_calls": 0,
    }
    print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
