#!/usr/bin/env python3
"""Static, hash, one-shot, and tamper checks for the standalone r2 bundle."""

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


def trend_is_stable(samples: list[float], maximum_decline: float) -> bool:
    return len(samples) < 2 or samples[0] - samples[-1] <= maximum_decline


def best_stable_window_minimum(
    samples: list[float], count: int, maximum_decline: float
) -> float | None:
    minima = [
        min(window)
        for start in range(len(samples) - count + 1)
        if trend_is_stable((window := samples[start : start + count]), maximum_decline)
    ]
    return max(minima) if minima else None


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True)
    parser.add_argument("--config", required=True)
    args = parser.parse_args()
    repo = Path(args.repo).resolve()
    config_path = Path(args.config).resolve()
    config = json.loads(config_path.read_text(encoding="utf-8"))

    runner_path = repo / config["runner"]
    evaluator_source = repo / "api/forensic_records/english_functional_qualification_test.go"
    runner = runner_path.read_text(encoding="utf-8")
    evaluator = evaluator_source.read_text(encoding="utf-8")
    corpus_path = repo / config["corpus"]["path"]
    oracle_path = repo / config["oracle"]["path"]
    corpus = json.loads(corpus_path.read_text(encoding="utf-8"))
    oracle = json.loads(oracle_path.read_text(encoding="utf-8"))

    require(config["contract_version"] == "nexusai.current4b-resource-then-english-freeze/v2", "FREEZE_VERSION_INVALID")
    resource = config["resource"]
    state_machine = resource["state_machine"]
    observed_delta = round(resource["last_observed_preload_gib"] - resource["last_observed_loaded_gib"], 3)
    advisory = round(resource["loaded_formal_floor_gib"] + observed_delta + resource["advisory_safety_margin_gib"], 3)
    credible = round(max(resource["preload_formal_floor_gib"], resource["loaded_formal_floor_gib"] + observed_delta), 3)
    require(observed_delta == resource["observed_load_delta_gib"] == 4.627, "OBSERVED_LOAD_DELTA_INVALID")
    require(advisory == resource["empirical_advisory_preload_target_gib"] == 9.127, "ADVISORY_TARGET_INVALID")
    require(credible == state_machine["credible_predicted_loaded_threshold_gib"] == 8.627, "CREDIBLE_THRESHOLD_INVALID")
    require(state_machine["maximum_recovery_wait_seconds"] == 120, "RECOVERY_WAIT_INVALID")
    require(state_machine["recovery_sample_spacing_seconds"] == 5, "RECOVERY_SPACING_INVALID")
    require(state_machine["qualifying_sample_count"] == 5, "QUALIFYING_COUNT_INVALID")
    require(state_machine["qualifying_sample_spacing_seconds"] == 5, "QUALIFYING_SPACING_INVALID")
    require(state_machine["recovery_samples_are_diagnostic_only"] is True, "RECOVERY_SAMPLES_NOT_DIAGNOSTIC")

    transient_sequences = [
        [4.304, 4.301, 5.441, 7.233, 8.418],
        [4.400, 4.399, 5.288, 6.846, 8.308],
    ]
    for samples in transient_sequences:
        best = best_stable_window_minimum(samples, 5, state_machine["maximum_window_decline_gib"])
        require(best is not None and round(best - observed_delta, 3) < resource["loaded_formal_floor_gib"], "TRANSIENT_SEQUENCE_WRONGLY_ADMITTED")
    require(best_stable_window_minimum([9.31, 9.28, 9.25, 9.22, 9.20], 5, 0.5) >= advisory, "ADVISORY_SEQUENCE_REJECTED")
    require(best_stable_window_minimum([8.70, 8.68, 8.66, 8.65, 8.64], 5, 0.5) >= credible, "CREDIBLE_SEQUENCE_REJECTED")
    require(best_stable_window_minimum([9.80, 9.60, 9.40, 9.25, 9.15], 5, 0.5) is None, "DECLINING_SEQUENCE_ADMITTED")

    for relative, expected in config["files"].items():
        require(sha256(repo / relative) == expected, f"FROZEN_FILE_DRIFT:{relative}")
    require(sha256(runner_path) == config["runner_sha256"], "RUNNER_HASH_DRIFT")
    require(sha256(repo / config["freshness"]["path"]) == config["freshness"]["sha256"], "FRESHNESS_HASH_DRIFT")

    semantic = corpus["semantic_cases"]
    router = corpus["router_cases"]
    synthesis = corpus["synthesis_cases"]
    require(len(semantic) == 101 and len(router) == 32 and len(synthesis) == 16, "CASE_COUNTS_INVALID")
    require(len({case["id"] for case in semantic + router + synthesis}) == 149, "CASE_IDS_NOT_UNIQUE")
    require(corpus["corpus_id"] == config["corpus"]["id"], "CORPUS_ID_INVALID")
    require("fresh-r1" not in corpus["corpus_id"], "PRIOR_CORPUS_REUSED")
    supported = {case["id"] for case in semantic if case["kind"] == "SUPPORTED_UNAMBIGUOUS"}
    operations = {
        decision.removeprefix("OPERATION:")
        for case_id, decision in oracle["semantic"].items()
        if case_id in supported and decision.startswith("OPERATION:")
    }
    require(len(operations) == 66, "WORKSPACE_OPERATION_COVERAGE_INVALID")

    forbidden_live_work = ("go test", "npm ", "validate_freshness.py", "git status", "rg --files", "rglob(")
    for token in forbidden_live_work:
        require(token.casefold() not in runner.casefold(), f"HEAVY_LIVE_WORK_PRESENT:{token}")
    require(not re.search(r"Stop-Process\s+-(?:Name|InputObject)", runner, re.I), "BROAD_PROCESS_KILL_PRESENT")
    require(not re.search(r"Stop-Process[^\r\n]*(?:codex|chatgpt|chrome|msedge|code|explorer|memory|msmpeng|defender|docker|vmmem|terminal|powershell|teams|slack|office|phone)", runner, re.I), "USER_OR_SYSTEM_PROCESS_KILL_PRESENT")
    require("Stop-Process -Id $process.Id -Force" in runner, "OWNED_EVALUATOR_STOP_MISSING")

    recovery_gate = runner.index("Get-RecoveryRAMSamples ([int]$config.resource.state_machine.maximum_recovery_wait_seconds)")
    qualifying_gate = runner.index("Get-StableRAMSamples 'preload_qualifying'", recovery_gate)
    preload_gate = runner.index("Assert-StableSampleWindow $qualifying", qualifying_gate)
    admission_call = runner.index("$admission = Get-PreloadAdmission", preload_gate)
    resource_lock = runner.index("New-ExclusiveLock $resourceLock", admission_call)
    post_load_gate = runner.index("Assert-StableSampleWindow $postLoadSamples", resource_lock)
    transport_1 = runner.index("'transport_soak_1'", post_load_gate)
    transport_2 = runner.index("'transport_soak_2'", transport_1)
    post_soak_gate = runner.index("Assert-StableSampleWindow $postSoakSamples", transport_2)
    evaluator_start = runner.index("Start-Process -FilePath", post_soak_gate)
    require(recovery_gate < qualifying_gate < preload_gate < admission_call < resource_lock < post_load_gate < transport_1 < transport_2 < post_soak_gate < evaluator_start, "GATE_ORDER_INVALID")
    require("diagnostic_only=$true" in runner, "RECOVERY_OBSERVATIONS_NOT_MARKED_DIAGNOSTIC")
    require("Assert-ResourceStateMachineContract" in runner, "RESOURCE_STATE_MACHINE_SELF_TEST_MISSING")
    validate_only = runner.index("if ($ValidateOnly)")
    require(runner.index("Assert-ResourceStateMachineContract", validate_only) > validate_only, "RESOURCE_STATE_MACHINE_SELF_TEST_NOT_IN_VALIDATE_ONLY")
    require("$finalState = 'CURRENT_4B_PREDICTED_RESOURCE_INSUFFICIENT'" in runner, "PREDICTED_FAILURE_STATE_MISSING")
    require("RESOURCE_SMOKE=NOT_STARTED" in runner and "QUALIFICATION_CORPUS=UNCONSUMED" in runner, "PREDICTED_FAILURE_FAIL_CLOSED_OUTPUT_MISSING")

    lock_open = evaluator.index("os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY")
    first_call = evaluator.index("resolveOpenEndedSemanticPlanner", lock_open)
    require(lock_open < first_call, "QUALIFICATION_LOCK_NOT_BEFORE_FIRST_CALL")
    require("NEXUSAI_ENGLISH_DISPATCH_LOCK" in runner, "QUALIFICATION_LOCK_NOT_WIRED")

    original = corpus_path.read_bytes()
    require(hashlib.sha256(original + b"tamper").hexdigest() != config["corpus"]["sha256"], "TAMPER_TEST_FAILED")
    freshness = json.loads((repo / config["freshness"]["path"]).read_text(encoding="utf-8"))
    require(freshness["status"] == "PASS", "FRESHNESS_NOT_PASS")
    require(freshness["question_overlap"] == freshness["normalized_literal_overlap"] == freshness["prior_corpus_overlap"] == 0, "FRESHNESS_OVERLAP_PRESENT")

    print(json.dumps({
        "status": "PASS",
        "case_counts": {"semantic": len(semantic), "router": len(router), "synthesis": len(synthesis)},
        "workspace_operations": len(operations),
        "hash_validation": "PASS",
        "tamper_rejection": "PASS",
        "gate_order": "PASS",
        "no_heavy_live_work": "PASS",
        "no_user_process_kill": "PASS",
        "qualification_lock_semantics": "PASS",
        "resource_state_machine_static_tests": "PASS",
        "recovery_samples_diagnostic_only": "PASS",
        "observed_load_delta_gib": observed_delta,
        "empirical_advisory_preload_target_gib": advisory,
    }, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
