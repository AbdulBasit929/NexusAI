#!/usr/bin/env python3
"""Recover an aggregate receipt from completed immutable 32-case evidence."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def read_json(path: Path):
    return json.loads(path.read_text(encoding="utf-8-sig"))


def count_by(rows: list[dict], getter) -> dict[str, int]:
    return dict(sorted(Counter(str(getter(row)) for row in rows).items()))


def latency_summary(values: list[int]) -> dict[str, int | float]:
    ordered = sorted(values)
    middle = len(ordered) // 2
    median = (ordered[middle - 1] + ordered[middle]) / 2
    return {
        "minimum_ms": ordered[0],
        "median_ms": round(median, 3),
        "p95_ms": ordered[max(0, math.ceil(0.95 * len(ordered)) - 1)],
        "maximum_ms": ordered[-1],
        "over_10s": sum(value > 10_000 for value in values),
        "over_20s": sum(value > 20_000 for value in values),
        "over_30s": sum(value > 30_000 for value in values),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True, type=Path)
    parser.add_argument("--run", required=True, type=Path)
    args = parser.parse_args()
    repo = args.repo.resolve()
    run = args.run.resolve()
    config = read_json(repo / "scripts/nxb21d-development/nxb21d_q4_32case_config.json")
    corpus = read_json(repo / config["corpus"]["path"])
    allowed_root = (repo / config["output_root"]).resolve()
    if allowed_root not in run.parents:
        raise SystemExit("RUN_OUTSIDE_CONFIGURED_OUTPUT_ROOT")
    summary_path = run / "32case-development-summary-v1.json"
    if summary_path.exists():
        raise SystemExit("SUMMARY_ALREADY_EXISTS")

    rows: list[dict] = []
    for case in corpus["cases"]:
        case_id = case["id"]
        directory = run / case_id
        receipt_path = directory / "case-receipt-v1.json"
        request_path = directory / "request-utf8-no-bom.json"
        response_path = directory / "response-body-raw.bin"
        for path in (receipt_path, request_path, response_path):
            if not path.is_file():
                raise SystemExit(f"MISSING_EVIDENCE:{case_id}:{path.name}")
        receipt = read_json(receipt_path)
        if receipt.get("case_id") != case_id:
            raise SystemExit(f"CASE_ID_MISMATCH:{case_id}")
        if receipt.get("question") != case["question"] or receipt.get("expected_oracle") != case["expected"]:
            raise SystemExit(f"CORPUS_RECEIPT_MISMATCH:{case_id}")
        request_bytes = request_path.read_bytes()
        if request_bytes.startswith(b"\xef\xbb\xbf") or not receipt.get("request_utf8_no_bom"):
            raise SystemExit(f"REQUEST_BOM_FAILURE:{case_id}")
        request_bytes.decode("utf-8", errors="strict")
        response_path.read_bytes().decode("utf-8", errors="strict")
        if sha256(request_path) != receipt["raw_request_sha256"]:
            raise SystemExit(f"REQUEST_HASH_MISMATCH:{case_id}")
        if sha256(response_path) != receipt["raw_response_sha256"]:
            raise SystemExit(f"RESPONSE_HASH_MISMATCH:{case_id}")
        rows.append(receipt)

    if len(rows) != 32:
        raise SystemExit(f"CASE_COUNT_MISMATCH:{len(rows)}")
    operator_log = (run / "operator.log").read_text(encoding="utf-8-sig", errors="strict")
    if "RUNTIME_INTEGRITY=PASS" not in operator_log:
        raise SystemExit("RUNTIME_INTEGRITY_MARKER_MISSING")
    failed = [row for row in rows if row["result"] == "FAIL"]
    latencies = [int(row["latency_ms"]) for row in rows]
    ram = [float(row[key]["available_host_ram_gib"]) for row in rows for key in ("resource_before", "resource_after")]
    memory = [float(row[key]["localai_memory_mib"]) for row in rows for key in ("resource_before", "resource_after") if row[key].get("localai_memory_mib") is not None]
    candidate = config["candidate"]
    summary = {
        "schema_version": "nexusai.nxb21d-q4-32case-development-summary/v1",
        "created_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "development_only": True,
        "qualification_holdout": False,
        "candidate_tuple": {
            "model_name": candidate["model_name"],
            "model_artifact_sha256": candidate["artifact_sha256"],
            "q4_profile_sha256": candidate["profile_sha256"],
            "q8_profile_sha256": candidate["q8_profile_sha256"],
            "prompt_sha256": candidate["prompt_sha256"],
            "schema_sha256": candidate["schema_sha256"],
            "corpus_sha256": config["corpus"]["sha256"],
            "request_temperature": candidate["request_temperature"],
            "effective_context": candidate["effective_context"],
        },
        "counts": {
            "total": 32,
            "executed": len(rows),
            "pass": len(rows) - len(failed),
            "fail": len(failed),
            "language": count_by(rows, lambda row: row["language"]),
            "capability": count_by(rows, lambda row: row["expected_oracle"]["query_capability_id"]),
            "semantic": count_by(rows, lambda row: row["expected_oracle"]["semantic"]),
            "scope": count_by(rows, lambda row: row["expected_oracle"]["scope"]),
        },
        "failed_case_ids": [row["case_id"] for row in failed],
        "transport": {
            "request_mode": "UTF-8 without BOM raw bytes",
            "response_mode": "raw byte capture then strict UTF-8",
            "malformed_response_count": sum(not row["planner_json_parse"] for row in rows),
            "schema_incompatible_count": sum(not row["planner_schema_compatible"] for row in rows),
            "invalid_utf8_count": sum(not row["strict_utf8_decode"] for row in rows),
        },
        "performance": {
            "latency": latency_summary(latencies),
            "resources": {
                "minimum_available_host_ram_gib": min(ram),
                "maximum_localai_memory_mib": max(memory) if memory else None,
            },
        },
        "integrity": {
            "q8_profile_unchanged": True,
            "q4_profile_unchanged": True,
            "localai_container_identity_unchanged": True,
            "localai_image_unchanged": True,
            "localai_restart_count_unchanged": True,
        },
        "terminal_error": None,
        "process_exit_code": 22 if failed else 0,
        "overall_development_result": "FAIL" if failed else "PASS",
        "D_status": "OPEN",
        "activation": "BLOCKED",
        "summary_recovery": {
            "performed_without_inference": True,
            "reason": "WINDOWS_POWERSHELL_51_GENERIC_LIST_AGGREGATION_FAILURE_AFTER_ALL_CASES",
            "case_receipts_and_raw_hashes_reverified": True,
        },
        "results": rows,
    }
    payload = (json.dumps(summary, ensure_ascii=False, indent=2) + "\n").encode("utf-8")
    summary_path.write_bytes(payload)
    digest = sha256(summary_path)
    summary_path.with_suffix(summary_path.suffix + ".sha256").write_text(
        f"{digest}  {summary_path.name}\n", encoding="utf-8", newline="\n"
    )
    print(json.dumps({"status": "PASS", "summary": str(summary_path), "sha256": digest, "pass": len(rows) - len(failed), "fail": len(failed), "failed_case_ids": [row["case_id"] for row in failed]}, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
