#!/usr/bin/env python3
"""Read-only microbenchmark for populated Phase 7.1 subscriber normalization."""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
import time
import tracemalloc
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO_ROOT))

from ingestion.forensic_records.worker import ADAPTERS, IngestJob, normalize_generic_row  # noqa: E402


DEFAULT_FIXTURE = REPO_ROOT / "ingestion" / "forensic_records" / "tests" / "fixtures" / "pakistan_subscriber_populated_acceptance_v1.jsonl"
ADAPTER_ROOT = REPO_ROOT / "ingestion" / "forensic_records" / "adapters" / "subscriber"


def load_rows(path: Path) -> list[dict[str, object]]:
    return [json.loads(line) for line in path.read_text(encoding="utf-8-sig").splitlines() if line.strip()]


def job_for(path: Path) -> IngestJob:
    return IngestJob(
        job_id="00000000-0000-0000-0000-000000000071",
        tenant_id="benchmark",
        user_id="benchmark",
        collection_id="phase7-subscriber-read-only",
        file_id="phase7-subscriber-fixture",
        source_file=path.name,
        spool_path=str(path),
        sha256=hashlib.sha256(path.read_bytes()).hexdigest(),
        record_type="subscriber",
        headers=[],
        metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
    )


def normalize_pass(job: IngestJob, rows: list[dict[str, object]]) -> tuple[list[dict[str, object]], int]:
    accepted = []
    rejected = 0
    for row_number, row in enumerate(rows, start=2):
        try:
            accepted.append(
                normalize_generic_row(job, row, job.job_id, row_number, ADAPTERS["subscriber"])
            )
        except ValueError:
            rejected += 1
    return accepted, rejected


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--fixture", type=Path, default=DEFAULT_FIXTURE)
    parser.add_argument("--iterations", type=int, default=200)
    args = parser.parse_args()
    rows = load_rows(args.fixture)
    job = job_for(args.fixture)
    iterations = max(1, args.iterations)

    tracemalloc.start()
    cold_started = time.perf_counter_ns()
    cold, cold_rejected = normalize_pass(job, rows)
    cold_ns = time.perf_counter_ns() - cold_started

    warm_started = time.perf_counter_ns()
    final, final_rejected = cold, cold_rejected
    for _ in range(iterations):
        final, final_rejected = normalize_pass(job, rows)
    warm_ns = time.perf_counter_ns() - warm_started
    _, peak_bytes = tracemalloc.get_traced_memory()
    tracemalloc.stop()

    processed = len(rows) * iterations
    unique_hashes = len({str(record["row_hash"]) for record in final})
    package_bytes = sum(path.stat().st_size for path in ADAPTER_ROOT.rglob("*") if path.is_file() and "__pycache__" not in path.parts)
    result = {
        "contract_version": "forensics.subscriber-benchmark/v1",
        "fixture_name": args.fixture.name,
        "fixture_sha256": job.sha256,
        "fixture_rows": len(rows),
        "iterations": iterations,
        "cold_pass_ms": round(cold_ns / 1_000_000, 4),
        "warm_input_row_us": round((warm_ns / 1000) / max(1, processed), 4),
        "warm_input_rows_per_second": round(processed / max(warm_ns / 1_000_000_000, 1e-9), 2),
        "peak_python_allocation_bytes": peak_bytes,
        "adapter_package_bytes": package_bytes,
        "normalized_candidates": len(final),
        "unique_accepted_rows": unique_hashes,
        "duplicate_rows_by_hash": len(final) - unique_hashes,
        "rejected_rows": final_rejected,
        "row_accounting_complete": len(rows) == unique_hashes + (len(final) - unique_hashes) + final_rejected,
        "evidence_writes": 0,
        "database_queries": 0,
        "model_calls": 0,
    }
    print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
