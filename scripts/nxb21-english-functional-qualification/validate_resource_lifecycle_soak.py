#!/usr/bin/env python3
"""Static and immutable checks for the disposable current-4B resource soak."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(message)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True)
    parser.add_argument("--config", required=True)
    args = parser.parse_args()
    repo = Path(args.repo).resolve()
    config_path = Path(args.config).resolve()
    config = json.loads(config_path.read_text(encoding="utf-8"))
    runner_path = repo / config["runner"]
    runner = runner_path.read_text(encoding="utf-8")

    require(config["contract_version"] == "nexusai.current4b-resource-lifecycle-soak-freeze/v1", "CONTRACT_INVALID")
    require(sha256(runner_path) == config["runner_sha256"], "RUNNER_HASH_DRIFT")
    for relative, expected in config["files"].items():
        require(sha256(repo / relative) == expected, f"FROZEN_FILE_DRIFT:{relative}")
    require(config["consumed_r2"]["state"] == "CONSUMED_DO_NOT_RERUN", "R2_STATE_INVALID")
    require(config["consumed_r2"]["completed_calls"] == 0, "R2_COMPLETED_CALLS_INVALID")
    require(config["batch_plan"] == [1, 2, 4, 8, 5], "BATCH_PLAN_INVALID")
    require(sum(config["batch_plan"]) == config["tiny_calls"] == 20, "TINY_CALL_COUNT_INVALID")
    require(config["payload_shaped_calls"] == 3, "SHAPED_CALL_COUNT_INVALID")
    require(config["prompt_footprint"]["candidate_count"] == 66, "CANDIDATE_COUNT_INVALID")
    require(config["prompt_footprint"]["prompt_bytes_min"] == 11111, "PROMPT_MIN_INVALID")
    require(config["prompt_footprint"]["prompt_bytes_p50"] == 11150, "PROMPT_P50_INVALID")
    require(config["prompt_footprint"]["prompt_bytes_max"] == 11197, "PROMPT_MAX_INVALID")

    forbidden_live_work = ("go test", "npm ", "validate_freshness.py", "git status", "rg --files", "rglob(")
    for token in forbidden_live_work:
        require(token.casefold() not in runner.casefold(), f"HEAVY_LIVE_WORK_PRESENT:{token}")
    require("Stop-Process" not in runner, "PROCESS_KILL_PRESENT")
    require(not re.search(r"(?:taskkill|kill-process|terminateprocess)", runner, re.I), "PROCESS_KILL_ALIAS_PRESENT")
    for forbidden in (
        "NEXUSAI_ENGLISH_FUNCTIONAL_LIVE",
        "NEXUSAI_ENGLISH_CORPUS",
        "NEXUSAI_ENGLISH_ORACLE",
        "english-functional-corpus-v2.json",
        "english-functional-oracle-v2.json",
        "resolveOpenEndedSemanticPlanner",
        "OPERATION:",
    ):
        require(forbidden not in runner, f"QUALIFICATION_SEMANTICS_PRESENT:{forbidden}")

    initial_gate = runner.index("Get-StableRAMWindow 'initial_preload'")
    soak_lock = runner.index("New-ExclusiveLock $dispatchLock", initial_gate)
    first_load = runner.index("Start-OwnedModel $false $true", soak_lock)
    tiny_call = runner.index("Invoke-ResourceCall 'tiny'", first_load)
    reset = runner.index("Invoke-UnloadOwnedModel;Start-OwnedModel $true $false", tiny_call)
    shaped_call = runner.index("Invoke-ResourceCall 'payload_shaped'", reset)
    require(initial_gate < soak_lock < first_load < tiny_call < reset < shaped_call, "LIFECYCLE_ORDER_INVALID")
    require("Get-APIContainerMemoryMiB" in runner, "API_MEMORY_TELEMETRY_MISSING")
    require("Get-BackendMemory" in runner, "BACKEND_RSS_TELEMETRY_MISSING")
    require("Get-VmmemWorkingSetMiB" in runner, "VMMEM_TELEMETRY_MISSING")
    require("ram_drift_gib" in runner and "api_memory_drift_mib" in runner and "backend_rss_drift_mib" in runner, "PER_CALL_DRIFT_MISSING")
    require("quality_evidence='NONE'" in runner, "QUALITY_BOUNDARY_MISSING")
    require("r3_corpus_state='NOT_CREATED'" in runner, "R3_BOUNDARY_MISSING")
    require("Invoke-UnloadOwnedModel" in runner, "OWNED_MODEL_UNLOAD_MISSING")

    print(json.dumps({
        "status": "PASS",
        "r2_consumed_preserved": "PASS",
        "qualification_semantics_absent": "PASS",
        "no_heavy_live_work": "PASS",
        "no_process_kill": "PASS",
        "lifecycle_order": "PASS",
        "per_call_telemetry": "PASS",
        "batch_plan": config["batch_plan"],
        "tiny_calls": config["tiny_calls"],
        "payload_shaped_calls": config["payload_shaped_calls"],
    }, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
