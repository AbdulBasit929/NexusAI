#!/usr/bin/env python3
"""Finalize the one-shot NX-B2.1D run when Ginkgo timed out mid-corpus.

This script is intentionally evidence-only. It parses the immutable evaluator log,
joins the attempted rows to the frozen corpus, and writes a private partial result
plus the public fail-closed receipt. It does not call LocalAI or rerun any case.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import re
import statistics
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path


CASE_RE = re.compile(
    r"NXB21D_CASE\s+(\d+)/168\s+id=(\S+)\s+language=(\S+)\s+"
    r"status=(\S+)\s+pass=(true|false)\s+latency_ms=(\d+)"
)
LANGUAGE_TOTALS = {"en": 65, "ur": 37, "roman_ur": 36, "mixed": 30}


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def read_json(path: Path):
    return json.loads(path.read_text(encoding="utf-8-sig"))


def write_json(path: Path, value) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def nearest_rank(values: list[int], percentile: float) -> int:
    ordered = sorted(values)
    return ordered[max(0, math.ceil(percentile * len(ordered)) - 1)]


def scored_breakdown(rows: list[dict], key: str) -> dict:
    totals: dict[str, list[int]] = defaultdict(lambda: [0, 0])
    for row in rows:
        if key == "capability":
            label = row["expected"].get("capability")
        elif key == "semantic":
            label = row["expected"].get("semantic")
        elif key == "dimension":
            label = row["dimension"]
        else:
            label = "critical" if row["critical"] else "noncritical"
        if label is None:
            continue
        totals[str(label)][1] += 1
        totals[str(label)][0] += int(row["passed"])
    return {
        label: {
            "passed": passed,
            "attempted": total,
            "percent": round(100 * passed / total, 6),
        }
        for label, (passed, total) in sorted(totals.items())
    }


def resource_summary(path: Path) -> dict:
    rows = [json.loads(line) for line in path.read_text(encoding="utf-8-sig").splitlines() if line.strip()]
    ram = [float(row["available_ram_gib"]) for row in rows]
    cpu: list[float] = []
    memory_mib: list[float] = []
    for row in rows:
        stats = json.loads(row["docker_stats"])
        cpu.append(float(stats["CPUPerc"].rstrip("%")))
        amount, unit = re.match(r"([0-9.]+)([MG]iB)", stats["MemUsage"].split("/")[0].strip()).groups()
        memory_mib.append(float(amount) * (1024 if unit == "GiB" else 1))
    return {
        "sample_count": len(rows),
        "first_sample_utc": rows[0]["utc"],
        "last_sample_utc": rows[-1]["utc"],
        "minimum_host_available_ram_gib": min(ram),
        "maximum_api_container_cpu_percent": max(cpu),
        "maximum_api_container_memory_mib": max(memory_mib),
        "last_api_container_memory_mib": memory_mib[-1],
        "severe_memory_pressure_observed": min(ram) < 2,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--run", type=Path, required=True)
    args = parser.parse_args()
    repo = args.repo.resolve()
    run = args.run.resolve()

    corpus_path = repo / "reports/nxb21/d-unseen-query-corpus-v1.json"
    manifest_path = run / "d-model-evaluation-manifest-v1.json"
    log_path = run / "go-evaluator.log"
    operator_log_path = run / "operator.log"
    resources_path = run / "resource-samples.jsonl"
    corpus = read_json(corpus_path)
    manifest = read_json(manifest_path)
    case_by_id = {case["id"]: case for case in corpus["cases"]}
    log_text = log_path.read_text(encoding="utf-16")

    parsed = []
    for match in CASE_RE.finditer(log_text):
        number, case_id, language, status, passed, latency = match.groups()
        source = case_by_id[case_id]
        parsed.append(
            {
                "ordinal": int(number),
                "id": case_id,
                "query": source["query"],
                "language": language,
                "dimension": source["dimension"],
                "critical": bool(source["critical"]),
                "input": source["input"],
                "expected": source["expected"],
                "actual": {},
                "model_called": True,
                "structured_valid": status.startswith("validated_model_proposal"),
                "passed": passed == "true",
                "status": status,
                "latency_ms": int(latency),
                "mismatches": [] if passed == "true" else [status],
            }
        )
    if len(parsed) != 81 or parsed[-1]["id"] != "D-081":
        raise SystemExit(f"unexpected parsed case boundary: count={len(parsed)} last={parsed[-1]['id'] if parsed else None}")

    latencies = [row["latency_ms"] for row in parsed]
    status_counts = dict(sorted(Counter(row["status"] for row in parsed).items()))
    attempted_by_language = Counter(row["language"] for row in parsed)
    passed_by_language = Counter(row["language"] for row in parsed if row["passed"])
    language = {}
    for key, total in LANGUAGE_TOTALS.items():
        attempted = attempted_by_language[key]
        passed = passed_by_language[key]
        remaining = total - attempted
        max_passed = passed + remaining
        allowed_failures = math.floor(total * 0.05)
        language[key] = {
            "passed": passed,
            "attempted": attempted,
            "attempted_percent": round(100 * passed / attempted, 6) if attempted else None,
            "corpus_total": total,
            "not_attempted": remaining,
            "maximum_possible_passed": max_passed,
            "maximum_possible_percent": round(100 * max_passed / total, 6),
            "frozen_required_percent": 95,
            "allowed_failures": allowed_failures,
            "threshold_still_mathematically_reachable": (total - max_passed) <= allowed_failures,
        }

    resources = resource_summary(resources_path)
    evidence = {
        "evaluation_manifest_sha256": sha256(manifest_path),
        "frozen_holdout_sha256": sha256(corpus_path),
        "go_evaluator_log_sha256": sha256(log_path),
        "operator_log_sha256": sha256(operator_log_path),
        "resource_samples_sha256": sha256(resources_path),
        "observed_model_yaml_sha256": "ef036e0a02cc5e8331dc4dff41c0e0cb68e6e0bda71d92b92f47f673472e666f",
    }
    terminal = {
        "evaluation_status": "INVALID_ABORTED_CONFIGURATION_MISMATCH_AND_SUITE_TIMEOUT",
        "holdout_consumed": True,
        "holdout_completion": "PARTIAL_81_OF_168",
        "cases_attempted": len(parsed),
        "cases_completed": len(parsed),
        "cases_not_attempted": 168 - len(parsed),
        "model_calls": len(parsed),
        "structured_valid": {"count": 0, "total": len(parsed), "percent": 0.0},
        "overall_observed": {"passed": 0, "attempted": len(parsed), "percent": 0.0},
        "overall_maximum_possible": {
            "passed": 168 - len(parsed),
            "total": 168,
            "percent": round(100 * (168 - len(parsed)) / 168, 6),
        },
        "statuses": status_counts,
        "language": language,
        "critical": {
            "passed": 0,
            "attempted": 0,
            "corpus_total": sum(1 for case in corpus["cases"] if case["critical"]),
            "percent": None,
            "status": "NOT_REACHED_UNMEASURED",
            "frozen_required_percent": 100,
        },
        "breakdowns_observed": {
            "capability": scored_breakdown(parsed, "capability"),
            "semantic": scored_breakdown(parsed, "semantic"),
            "dimension": scored_breakdown(parsed, "dimension"),
            "critical_class": scored_breakdown(parsed, "critical_class"),
        },
        "latency": {
            "sum_case_latency_ms": sum(latencies),
            "median_ms": int(statistics.median(latencies)),
            "p95_ms": nearest_rank(latencies, 0.95),
            "minimum_ms": min(latencies),
            "maximum_ms": max(latencies),
            "suite_elapsed_ms": 3_600_018,
            "go_command_elapsed_ms": 3_602_893,
        },
        "resources": resources,
    }

    private = {
        "schema_version": "nexusai.nxb21d-qwen-consumed-partial/v1",
        "finalized_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "run_directory": str(run),
        "derivation": "Parsed only from frozen corpus, immutable run logs, resource samples, and read-only post-run inspection; no inference or holdout rerun.",
        **terminal,
        "frozen_manifest": manifest,
        "evidence": evidence,
        "results": parsed,
    }
    private_path = run / "d-qwen-168-holdout-results-v1.partial.json"
    write_json(private_path, private)

    public = {
        "schema_version": "nexusai.nxb21d-model-evaluation-receipt/v1",
        "completed_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "source_digest": manifest["source_digest"],
        **terminal,
        "configuration": {
            "frozen_manifest": {
                "model": manifest["model"]["id"],
                "artifact": manifest["model"]["artifact"],
                "artifact_sha256": manifest["model"]["artifact_sha256"],
                "quantization": manifest["model"]["quantization"],
                "context": manifest["model"]["context"],
                "threads": manifest["model"]["threads"],
                "gpu_layers": manifest["model"]["gpu_layers"],
                "temperature": manifest["model"]["temperature"],
                "max_tokens": manifest["model"]["max_tokens"],
            },
            "observed_live_yaml": {
                "sha256": evidence["observed_model_yaml_sha256"],
                "top_level_context_size": 4096,
                "parameters_context_size": 8192,
                "effective_runtime_context_size": 4096,
                "top_level_temperature": 0.6,
                "request_level_temperature": 0,
                "function_grammar_disabled": True,
                "threads": 8,
                "gpu_layers": 0,
            },
            "matches_frozen_manifest": False,
            "mismatches": [
                "effective context_size 4096 != frozen 8192",
            ],
            "observations_not_proven_mismatches": [
                "live YAML top-level temperature is 0.6, but the planner request set temperature 0; no runtime decoding-temperature mismatch was observed",
                "function.grammar.disable is true, but LocalAI handles response_format JSON-schema grammar on a separate path from generated function-call grammar",
            ],
        },
        "structured_output_diagnostics": {
            "response_format_json_schema_requested": True,
            "localai_response_format_grammar_path": "core/http/endpoints/openai/chat.go:275-310",
            "function_grammar_disable_scope": "Disables LocalAI-generated function-call grammar; it does not disable the separate response_format JSON-schema grammar path",
            "grammar_generation_errors_in_localai_log": 0,
            "raw_completion_content_preserved_by_original_evaluator": False,
            "malformed_proposal_root_cause": "UNRESOLVED: the immutable evidence cannot distinguish backend schema enforcement, chat-template or stopword behavior, prompt behavior, or another response-content failure because the original evaluator discarded malformed raw completions",
        },
        "failure_classification": [
            "MODEL_PROFILE_CONTEXT_MISMATCH",
            "MALFORMED_RESPONSE_CAUSE_UNRESOLVED",
            "SUITE_TIMEOUT",
            "SEVERE_MEMORY_PRESSURE",
        ],
        "current_deployed_profile_suitability": "INSUFFICIENT",
        "intrinsic_base_model_suitability": "NOT_CLASSIFIABLE_FROM_INVALID_RUN",
        "suitability": "INSUFFICIENT",
        "DExitDecision": "FAIL",
        "model_admission_required": True,
        "activation_allowed": False,
        "holdout_may_be_used_for_tuning": False,
        "independent_retest_requirement": "Correct and qualify the planner/profile on a development corpus, then freeze a new independent 168-case holdout before any new suitability claim.",
        "runtime_postcheck": {
            "status": "PASS_AFTER_DOCUMENTED_MANUAL_SAFE_UNLOAD",
            "model_unloaded": True,
            "loaded_models": ["qwen3-embedding-0.6b"],
            "containers_healthy": True,
            "container_id_image_restart_parity": True,
            "retained_tuple": "64|64|77|22507|828|61",
            "active_jobs": 0,
            "activity_count": 303,
            "newest_activity_created_at": "2026-09-03T06:55:39.934170Z",
            "new_activity_created_by_evaluation": False,
            "available_ram_gib_after_manual_unload_and_cache_recovery": 2.275,
            "available_ram_gib_at_finalization": 4.921,
        },
        "runtime_impact": {
            "new_model_downloaded": False,
            "model_changed_permanently": False,
            "deployment_performed": False,
            "database_migration": False,
            "retained_evidence_mutated": False,
            "retained_activity_mutated": False,
            "volumes_changed": False,
            "product_certification_performed": False,
            "nxb2_activation_performed": False,
        },
        "evidence": {**evidence, "private_partial_result_sha256": sha256(private_path)},
    }
    public_path = repo / "reports/nxb21/d-model-evaluation-receipt-v1.json"
    write_json(public_path, public)
    hash_path = public_path.with_suffix(public_path.suffix + ".sha256")
    hash_path.write_text(f"{sha256(public_path)}  {public_path.name}\n", encoding="utf-8")
    print(public_path)
    print(hash_path)
    print(private_path)


if __name__ == "__main__":
    main()
