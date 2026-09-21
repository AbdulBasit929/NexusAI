#!/usr/bin/env python3
"""Validate NexusAI hardware-tier, candidate and T3 acquisition governance."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from urllib.parse import urlparse


HASH_LENGTHS = {"MD5": 32, "SHA256": 64}


def _load(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def validate_hardware(path: Path) -> dict:
    data = _load(path)
    assert data["contract_version"] == "nexusai.hardware-tier-policy/v1"
    assert set(data["tiers"]) == {"D0", "D1", "P1", "P2"}
    assert data["tiers"]["D0"]["current_machine"] is True
    assert all(data["tiers"][tier]["current_machine"] is False for tier in ("D1", "P1", "P2"))
    required = set(data["evaluation_record_required_fields"])
    must_have = {
        "hardware_tier", "cpu_model", "system_ram_gib", "gpu_model",
        "gpu_vram_gib", "backend", "backend_version", "precision",
        "threads", "batch_size", "model_revision", "dataset_revision",
    }
    assert must_have <= required
    assert data["role_resolution"]["agent_contract_hardware_neutral"] is True
    assert "blocked_accuracy_gate" in data["capability_states"]
    return {"tiers": 4, "required_fields": len(required)}


def validate_matrix(path: Path) -> dict:
    data = _load(path)
    assert data["contract_version"] == "nexusai.r8-candidate-hardware-matrix/v1"
    assert data["thresholds_changed"] is False
    assert data["automatic_promotion"] is False
    assert data["production_mutation"] is False
    ids: set[str] = set()
    selected = 0
    retained_efficient_references = 0
    for role, candidates in data["roles"].items():
        assert role in {"plate_region_detection", "latin_plate_ocr", "urdu_arabic_plate_ocr"}
        for candidate in candidates:
            candidate_id = candidate["candidate_id"]
            assert candidate_id not in ids
            ids.add(candidate_id)
            assert set(candidate["hardware_support"]) == {"D0", "D1", "P1", "P2"}
            if candidate["decision"] == "selected_next_small_isolated_evaluation":
                selected += 1
                for key in ("model_sha256", "config_sha256"):
                    assert len(candidate[key]) == 64
                    int(candidate[key], 16)
            if candidate["decision"] == "rejected_accuracy_and_abstention_gates_retain_best_efficient_latin_reference":
                retained_efficient_references += 1
                assert candidate["accuracy_state"].startswith("T2V_development_exact_")
    assert selected <= 1
    assert selected + retained_efficient_references == 1
    return {
        "candidates": len(ids), "selected_small_evaluations": selected,
        "retained_efficient_references": retained_efficient_references,
    }


def validate_t3(path: Path) -> dict:
    data = _load(path)
    assert data["contract_version"] == "nexusai.r8-t3-acquisition/v1"
    assert data["production_mutation"] is False
    assert data["automatic_promotion"] is False
    assert data["default_action"] == "preflight_only"
    artifacts = data["artifacts"]
    assert len(artifacts) in (1, 2)
    for artifact in artifacts:
        parsed = urlparse(artifact["url"])
        assert parsed.scheme == "https" and parsed.hostname == "zenodo.org"
        assert artifact["size_bytes"] > 0
        algorithm = artifact["checksum_algorithm"]
        digest = artifact["checksum"]
        assert len(digest) == HASH_LENGTHS[algorithm]
        int(digest, 16)
    assert data["privacy_disposition"]["safe_for_git"] is False
    assert data["privacy_disposition"]["safe_for_demo"] is False
    assert data["privacy_disposition"]["training_allowed"] is False
    return {"artifacts": len(artifacts), "selected": data["selected_artifact_id"]}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--hardware", type=Path, required=True)
    parser.add_argument("--matrix", type=Path, required=True)
    parser.add_argument("--t3", type=Path, required=True)
    args = parser.parse_args()
    result = {
        "status": "pass",
        "hardware": validate_hardware(args.hardware),
        "matrix": validate_matrix(args.matrix),
        "t3": validate_t3(args.t3),
    }
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
