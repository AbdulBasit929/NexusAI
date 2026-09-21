#!/usr/bin/env python3
"""Create an immutable review bundle from saved R8 predictions and v2 scores."""

from __future__ import annotations

import argparse
import hashlib
import json
from datetime import datetime, timezone
from pathlib import Path


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--original-bundle", type=Path, required=True)
    parser.add_argument("--results", type=Path, required=True)
    parser.add_argument("--fixtures", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    original = json.loads(args.original_bundle.read_text(encoding="utf-8-sig"))
    candidates = []
    states = []
    for candidate in ("tesseract", "paddle-detector", "paddle-en", "paddle-arabic"):
        predictions = args.results / f"{candidate}-predictions.json"
        score_path = args.results / f"{candidate}-score-v2.json"
        score = json.loads(score_path.read_text(encoding="utf-8-sig"))
        states.append(score["gate"]["state"])
        candidates.append({
            "candidate": candidate,
            "predictions_sha256": digest(predictions),
            "corrected_score_sha256": digest(score_path),
            "corrected_score": score,
        })
    payload = {
        "contract_version": "forensics.r8-model-evaluation-review/v1",
        "reviewed_at_utc": datetime.now(timezone.utc).isoformat(),
        "original_bundle": str(args.original_bundle).replace("\\", "/"),
        "original_bundle_sha256": digest(args.original_bundle),
        "fixtures_sha256": digest(args.fixtures) if args.fixtures else original.get("fixtures_sha256"),
        "fixture_contract": (
            json.loads(args.fixtures.read_text(encoding="utf-8-sig")).get("contract_version")
            if args.fixtures else None
        ),
        "production_containers_unchanged": original.get("production_containers_unchanged"),
        "offline_benchmark": original.get("offline_benchmark"),
        "production_role_assignment": None,
        "decision": "no_promotion" if any(state != "pass" for state in states) else "eligible_for_explicit_operator_review",
        "candidates": candidates,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps({"status": "pass", "decision": payload["decision"], "output": str(args.output)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
