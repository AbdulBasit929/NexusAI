#!/usr/bin/env python3
"""Validate the authoritative NexusAI evidence-test dataset registry."""

from __future__ import annotations

import argparse
import json
from pathlib import Path


REQUIRED_FIELDS = {
    "dataset_id", "version", "tier", "family", "name", "purpose",
    "synthetic", "controlled", "public", "operational", "region",
    "languages", "source", "official_url", "license", "redistribution",
    "PII", "sensitive", "safe_for_git", "safe_for_demo",
    "safe_for_model_input", "record_or_sample_count", "size",
    "hash_manifest", "expected_schema", "golden_version", "benchmark_role",
    "status",
}


def validate(path: Path) -> list[str]:
    payload = json.loads(path.read_text(encoding="utf-8-sig"))
    errors: list[str] = []
    seen: set[str] = set()
    if payload.get("contract_version") != "nexusai.evidence-test-dataset-registry/v1":
        errors.append("unexpected registry contract_version")
    for index, dataset in enumerate(payload.get("datasets", [])):
        missing = sorted(REQUIRED_FIELDS - set(dataset))
        if missing:
            errors.append(f"datasets[{index}] missing: {', '.join(missing)}")
        dataset_id = dataset.get("dataset_id")
        if not dataset_id or dataset_id in seen:
            errors.append(f"datasets[{index}] dataset_id is empty or duplicated")
        seen.add(dataset_id)
        if dataset.get("tier") not in {"T0", "T1", "T2", "T2-P", "T2-V", "T3", "T4"}:
            errors.append(f"datasets[{index}] has invalid tier")
        if dataset.get("public") is True and not dataset.get("official_url"):
            errors.append(f"datasets[{index}] public dataset lacks official_url")
        if dataset.get("tier") == "T4" and dataset.get("safe_for_git") is not False:
            errors.append(f"datasets[{index}] T4 evidence must not be Git-safe")
    if not payload.get("datasets"):
        errors.append("registry has no datasets")
    return errors


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("path", type=Path)
    args = parser.parse_args()
    errors = validate(args.path)
    if errors:
        print(json.dumps({"status": "fail", "errors": errors}, indent=2))
        return 1
    print(json.dumps({"status": "pass", "registry": str(args.path)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
