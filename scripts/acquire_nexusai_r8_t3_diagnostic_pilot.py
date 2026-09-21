#!/usr/bin/env python3
"""Acquire the five pinned official CCPD demo images for diagnosis only."""

from __future__ import annotations

import argparse
import hashlib
import json
import urllib.request
from datetime import datetime, timezone
from pathlib import Path


def git_blob_sha1(payload: bytes) -> str:
    return hashlib.sha1(f"blob {len(payload)}\0".encode() + payload).hexdigest()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--contract", type=Path, required=True)
    parser.add_argument("--output-root", type=Path, required=True)
    parser.add_argument("--execute", action="store_true")
    args = parser.parse_args()
    contract = json.loads(args.contract.read_text(encoding="utf-8-sig"))
    output = args.output_root.resolve()
    if not output.is_absolute():
        parser.error("output root must be absolute")
    repo = args.contract.resolve().parents[1]
    if output == repo or repo in output.parents:
        parser.error("real diagnostic images must remain outside the repository")
    if contract["acceptance_authority"] or contract["production_authority"] or contract["automatic_promotion"]:
        parser.error("diagnostic pilot contract attempts to grant prohibited authority")
    expected_total = sum(int(item["size_bytes"]) for item in contract["files"])
    if expected_total != int(contract["total_bytes"]):
        parser.error("contract total byte count is inconsistent")
    print(json.dumps({
        "status": "preflight_pass", "dataset_id": contract["dataset_id"],
        "files": len(contract["files"]), "bytes": expected_total,
        "acceptance_authority": False, "production_mutation": False,
    }, sort_keys=True))
    if not args.execute:
        print("R8T3DiagnosticPilot=NOT_STARTED Reason=execute_not_supplied")
        return 0

    output.mkdir(parents=True, exist_ok=True)
    receipts = []
    for item in contract["files"]:
        destination = output / item["name"]
        url = (
            "https://raw.githubusercontent.com/detectRecog/CCPD/"
            f"{contract['revision']}/rpnet/demo/{item['name']}"
        )
        request = urllib.request.Request(url, headers={"User-Agent": "NexusAI-R8-Diagnostic/1.0"})
        with urllib.request.urlopen(request, timeout=60) as response:
            payload = response.read(int(item["size_bytes"]) + 1)
        if len(payload) != int(item["size_bytes"]):
            raise RuntimeError(f"size mismatch for {item['name']}")
        if git_blob_sha1(payload) != item["git_blob_sha1"]:
            raise RuntimeError(f"publisher Git blob mismatch for {item['name']}")
        temporary = destination.with_suffix(destination.suffix + ".partial")
        temporary.write_bytes(payload)
        temporary.replace(destination)
        receipts.append({
            "name": item["name"], "size_bytes": len(payload),
            "publisher_git_blob_sha1": item["git_blob_sha1"],
            "sha256": hashlib.sha256(payload).hexdigest(),
        })
    receipt = {
        "contract_version": "nexusai.r8-t3-diagnostic-pilot-receipt/v1",
        "dataset_id": contract["dataset_id"], "revision": contract["revision"],
        "acquired_at_utc": datetime.now(timezone.utc).isoformat(),
        "files": receipts, "acceptance_authority": False,
        "production_mutation": False, "automatic_promotion": False,
    }
    receipt_path = output / "acquisition-receipt.json"
    receipt_path.write_text(json.dumps(receipt, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"status": "pass", "files": len(receipts), "bytes": expected_total, "receipt": str(receipt_path)}, sort_keys=True))
    print("R8T3DiagnosticPilot=PASS AcceptanceAuthority=false ProductionMutation=false")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
