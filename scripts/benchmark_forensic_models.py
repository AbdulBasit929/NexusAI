#!/usr/bin/env python3
"""Benchmark LocalAI/NexusAI forensic evidence models and deterministic routes."""

from __future__ import annotations

import argparse
import hashlib
import json
import statistics
import time
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any


DEFAULT_CHAT_PROMPTS = [
    {
        "name": "forensic_grounding",
        "prompt": (
            "You are a forensic analyst. In two sentences, explain why exact "
            "counts over CDR rows must come from SQL records tools, not memory."
        ),
        "expect_contains": ["SQL", "exact"],
    },
    {
        "name": "source_filter_guard",
        "prompt": (
            "Classify this request without inventing targets: "
            "'show records from seed_cdr_large.csv where duration_seconds > 60'. "
            "Return JSON with source_file and duration_filter."
        ),
        "expect_contains": ["seed_cdr_large.csv", "60"],
    },
    {
        "name": "image_ocr_abstention",
        "prompt": (
            "A registered image has only deterministic dimensions and orientation; OCR and vision are pending. "
            "Can you state the plate number? Explain the limitation in one sentence."
        ),
        "expect_contains": ["cannot", "OCR"],
        "forbid_contains": ["plate number is"],
    },
    {
        "name": "audio_stt_abstention",
        "prompt": (
            "An audio file has only codec, sample rate, channels, and duration. No STT result exists. "
            "Can you quote what was said? Explain in one sentence."
        ),
        "expect_contains": ["cannot", "transcript"],
        "forbid_contains": ["the speaker said"],
    },
    {
        "name": "pakistan_operator_uncertainty",
        "prompt": (
            "Explain in one sentence why a Pakistan mobile operator must not be inferred solely from "
            "a phone-number prefix when number portability and dated allocations are not available."
        ),
        "expect_contains": ["operator", "prefix"],
        "forbid_contains": ["always belongs"],
    },
    {
        "name": "citation_requirement",
        "prompt": (
            "In one sentence, state what must accompany a forensic narrative claim derived from a "
            "Knowledge Base passage or extracted artifact."
        ),
        "expect_contains": ["traceable", "evidence"],
        "forbid_contains": ["no source"],
    },
]

DEFAULT_EMBEDDING_INPUTS = [
    "CDR record: source_number, target_number, timestamp, duration_seconds.",
    "ANPR record: plate, camera_id, location, sighting_time.",
    "IPDR record: subscriber, ip_address, start_time, end_time, bytes_in, bytes_out.",
    "پاکستانی اے این پی آر ریکارڈ: نمبر پلیٹ، کیمرہ، مقام اور وقت۔",
    "PKR transaction: exact decimal amount, source account, beneficiary, reversal and value date.",
    "Security event: IPv6 source, principal, action, outcome, host and correlation identifier.",
    "Audio and image evidence are registered, but transcription and OCR remain pending.",
    "Evidence provenance: tenant, collection, evidence version, SHA-256 and source locator.",
]

DEFAULT_FORENSIC_QUERIES = [
    {"name": "source_file_audit", "query": "which files were ingested?", "expected_template": "source_file_audit"},
    {"name": "duration_filter", "query": "show CDR records where duration seconds > 60", "expected_template": "canonical_records"},
    {"name": "data_quality", "query": "what limitations and missing data exist?", "expected_template": "limitations_and_data_quality"},
    {"name": "entity_activity", "query": "show available entities", "expected_template": "entity_activity"},
    {"name": "relationship_network", "query": "show relationship network for ABC-123", "expected_template": "relationship_network"},
    {"name": "entity_timeline", "query": "build timeline for ABC-123", "expected_template": "entity_timeline"},
    {"name": "case_readiness", "query": "is this case ready for production", "expected_template": "case_readiness"},
    {"name": "schema_profile", "query": "show detected headers and schema", "expected_template": "schema_profile"},
    {"name": "duplicate_upload_audit", "query": "show duplicate uploads", "expected_template": "duplicate_upload_audit"},
]


def request_json(method: str, url: str, body: Any | None = None, timeout: int = 60) -> tuple[Any, float]:
    data = None
    headers = {"Accept": "application/json"}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    started = time.perf_counter()
    with urllib.request.urlopen(req, timeout=timeout) as response:
        payload = response.read()
    elapsed_ms = (time.perf_counter() - started) * 1000.0
    if not payload:
        return None, elapsed_ms
    return json.loads(payload.decode("utf-8")), elapsed_ms


def url_join(base: str, path: str) -> str:
    return base.rstrip("/") + "/" + path.lstrip("/")


def percentile(values: list[float], pct: float) -> float | None:
    if not values:
        return None
    ordered = sorted(values)
    index = min(len(ordered) - 1, max(0, round((pct / 100.0) * (len(ordered) - 1))))
    return ordered[index]


def summarize_latencies(values: list[float]) -> dict[str, float | int | None]:
    if not values:
        return {"count": 0, "avg_ms": None, "p50_ms": None, "p95_ms": None}
    return {
        "count": len(values),
        "avg_ms": round(statistics.mean(values), 2),
        "p50_ms": round(percentile(values, 50) or 0.0, 2),
        "p95_ms": round(percentile(values, 95) or 0.0, 2),
    }


def contains_score(text: str, expected: list[str]) -> dict[str, Any]:
    found = [term for term in expected if term.lower() in text.lower()]
    return {"expected": expected, "found": found, "score": round(len(found) / len(expected), 4) if expected else None}


def phase2_contract_summary(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        matrix = json.load(handle)
    root = path.resolve().parents[1]
    profiles = []
    missing = []
    for profile in matrix.get("profiles", []):
        fixture = profile.get("fixture", {})
        fixture_path = root / fixture.get("path", "")
        exists = fixture_path.is_file()
        if not exists:
            missing.append(profile.get("id"))
        profiles.append(
            {
                "id": profile.get("id"),
                "support_level": profile.get("support_level"),
                "fixture_state": fixture.get("state"),
                "fixture_path": fixture.get("path"),
                "fixture_exists": exists,
                "processing_route": profile.get("classifier", {}).get("route"),
                "queue_records": profile.get("classifier", {}).get("queue_records"),
                "metric_names": [metric.get("name") for metric in profile.get("metrics", [])],
            }
        )
    payload = path.read_bytes()
    return {
        "schema_version": matrix.get("schema_version"),
        "matrix_sha256": hashlib.sha256(payload).hexdigest(),
        "required_family_count": len(matrix.get("required_families", [])),
        "profile_count": len(profiles),
        "ready_fixture_count": sum(item["fixture_state"] == "ready" and item["fixture_exists"] for item in profiles),
        "missing_fixture_profiles": missing,
        "all_required_fixtures_ready": not missing and all(item["fixture_state"] == "ready" for item in profiles),
        "support_counts": {
            level: sum(item["support_level"] == level for item in profiles)
            for level in ("operational", "foundation", "planned")
        },
        "profiles": profiles,
    }


def load_catalog(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def installed_models(localai_url: str) -> tuple[list[str], dict[str, Any]]:
    try:
        payload, elapsed = request_json("GET", url_join(localai_url, "/v1/models"), timeout=20)
        ids = [item.get("id") for item in payload.get("data", []) if item.get("id")]
        return ids, {"status": "ok", "latency_ms": round(elapsed, 2)}
    except Exception as exc:  # noqa: BLE001
        return [], {"status": "error", "error": str(exc)}


def benchmark_chat(localai_url: str, model: str, rounds: int) -> dict[str, Any]:
    results = []
    latencies = []
    for _ in range(rounds):
        for task in DEFAULT_CHAT_PROMPTS:
            body = {
                "model": model,
                "messages": [
                    {"role": "system", "content": "Answer concisely and do not invent evidence facts."},
                    {"role": "user", "content": task["prompt"]},
                ],
                "temperature": 0,
                "max_tokens": 256,
            }
            try:
                payload, elapsed = request_json("POST", url_join(localai_url, "/v1/chat/completions"), body)
                text = payload.get("choices", [{}])[0].get("message", {}).get("content", "")
                latencies.append(elapsed)
                results.append(
                    {
                        "task": task["name"],
                        "status": "ok",
                        "latency_ms": round(elapsed, 2),
                        "contains": contains_score(text, task["expect_contains"]),
                        "forbidden_terms_found": [
                            term for term in task.get("forbid_contains", []) if term.lower() in text.lower()
                        ],
                        "preview": text[:500],
                    }
                )
            except Exception as exc:  # noqa: BLE001
                results.append({"task": task["name"], "status": "error", "error": str(exc)})
    successful = [item for item in results if item.get("status") == "ok"]
    return {
        "model": model,
        "latency": summarize_latencies(latencies),
        "quality": {
            "successful_checks": len(successful),
            "total_checks": len(results),
            "required_term_agreement": round(
                statistics.mean(item["contains"]["score"] for item in successful), 4
            ) if successful else None,
            "lexical_unsupported_claim_rate": round(
                sum(bool(item.get("forbidden_terms_found")) for item in successful) / len(successful), 4
            ) if successful else None,
            "note": "Lexical guardrail signal only; it is not a substitute for human forensic correctness review.",
        },
        "results": results,
    }


def benchmark_embeddings(localai_url: str, model: str, rounds: int) -> dict[str, Any]:
    results = []
    latencies = []
    for _ in range(rounds):
        body = {"model": model, "input": DEFAULT_EMBEDDING_INPUTS}
        try:
            payload, elapsed = request_json("POST", url_join(localai_url, "/v1/embeddings"), body)
            vectors = payload.get("data", [])
            dims = len(vectors[0].get("embedding", [])) if vectors else 0
            latencies.append(elapsed)
            results.append({"status": "ok", "latency_ms": round(elapsed, 2), "vectors": len(vectors), "dimensions": dims})
        except Exception as exc:  # noqa: BLE001
            results.append({"status": "error", "error": str(exc)})
    return {"model": model, "latency": summarize_latencies(latencies), "results": results}


def benchmark_forensic_sidecar(forensic_url: str, tenant_id: str, collection_id: str, rounds: int) -> dict[str, Any]:
    checks = []
    latencies = []
    try:
        _, elapsed = request_json("GET", url_join(forensic_url, "/healthz"), timeout=20)
        checks.append({"name": "healthz", "status": "ok", "latency_ms": round(elapsed, 2)})
    except Exception as exc:  # noqa: BLE001
        checks.append({"name": "healthz", "status": "error", "error": str(exc)})

    query = urllib.parse.urlencode({"tenant_id": tenant_id, "collection_id": collection_id, "limit": 20})
    try:
        payload, elapsed = request_json("GET", url_join(forensic_url, f"/collections/status?{query}"), timeout=30)
        checks.append(
            {
                "name": "collection_status",
                "status": "ok",
                "latency_ms": round(elapsed, 2),
                "summary_keys": sorted(payload.get("summary", {}).keys()) if isinstance(payload, dict) else [],
            }
        )
    except Exception as exc:  # noqa: BLE001
        checks.append({"name": "collection_status", "status": "error", "error": str(exc)})

    for _ in range(rounds):
        for task in DEFAULT_FORENSIC_QUERIES:
            body = {
                "tenant_id": tenant_id,
                "collection_id": collection_id,
                "query": task["query"],
                "limit": 20,
                "max_kb_results": 3,
            }
            try:
                payload, elapsed = request_json("POST", url_join(forensic_url, "/query/hybrid"), body, timeout=60)
                selected_template = payload.get("template", "")
                latencies.append(elapsed)
                checks.append(
                    {
                        "name": task["name"],
                        "status": "ok",
                        "latency_ms": round(elapsed, 2),
                        "expected_template": task["expected_template"],
                        "selected_template": selected_template,
                        "template_match": selected_template == task["expected_template"],
                    }
                )
            except Exception as exc:  # noqa: BLE001
                checks.append(
                    {
                        "name": task["name"],
                        "status": "error",
                        "error": str(exc),
                        "expected_template": task["expected_template"],
                        "selected_template": "",
                        "template_match": False,
                    }
                )
    route_checks = [item for item in checks if "expected_template" in item]
    successful_routes = sum(item.get("status") == "ok" and item.get("template_match") is True for item in route_checks)
    return {
        "collection_id": collection_id,
        "tenant_id": tenant_id,
        "latency": summarize_latencies(latencies),
        "successful_routes": successful_routes,
        "failed_routes": len(route_checks) - successful_routes,
        "total_routes": len(route_checks),
        "route_agreement": round(successful_routes / len(route_checks), 4) if route_checks else None,
        "checks": checks,
    }


def catalog_summary(catalog: dict[str, Any], profile: str) -> list[dict[str, Any]]:
    groups = []
    for evidence_type, spec in catalog.get("evidence_types", {}).items():
        candidates = []
        for candidate in spec.get("recommended_models", []):
            profiles = candidate.get("profiles", [])
            if profiles and profile not in profiles:
                continue
            candidates.append(
                {
                    "name": candidate.get("name"),
                    "priority": candidate.get("priority"),
                    "backend": candidate.get("backend"),
                    "gallery_id": candidate.get("localai", {}).get("gallery_id", ""),
                    "role": candidate.get("role"),
                }
            )
        groups.append(
            {
                "evidence_type": evidence_type,
                "label": spec.get("label"),
                "primary_pipeline": spec.get("primary_pipeline", []),
                "recommended_models": sorted(candidates, key=lambda item: item.get("priority") or 999),
                "benchmark_tasks": spec.get("benchmark_tasks", []),
            }
        )
    return groups


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--localai-url", default="http://localhost:8080")
    parser.add_argument("--forensic-url", default="http://localhost:8091")
    parser.add_argument("--catalog", default="configuration/forensic_evidence_model_catalog.json")
    parser.add_argument("--phase2-matrix", default="configuration/forensic_modality_evaluation_matrix.json")
    parser.add_argument("--profile", default="balanced", choices=["cpu", "balanced", "accuracy", "throughput", "multilingual"])
    parser.add_argument("--tenant-id", default="default")
    parser.add_argument("--collection-id", default="records-demo")
    parser.add_argument("--chat-model", default="")
    parser.add_argument("--embedding-model", default="")
    parser.add_argument("--skip-sidecar", action="store_true")
    parser.add_argument("--rounds", type=int, default=1)
    parser.add_argument("--output", default="reports/forensic-model-benchmark.json")
    args = parser.parse_args()

    catalog_path = Path(args.catalog)
    phase2_matrix_path = Path(args.phase2_matrix)
    catalog = load_catalog(catalog_path)
    model_ids, model_check = installed_models(args.localai_url)

    report: dict[str, Any] = {
        "generated_at_unix": int(time.time()),
        "localai_url": args.localai_url,
        "forensic_url": args.forensic_url,
        "profile": args.profile,
        "catalog": str(catalog_path),
        "catalog_schema_version": catalog.get("schema_version"),
        "model_listing": model_check,
        "installed_models": model_ids,
        "catalog_summary": catalog_summary(catalog, args.profile),
        "phase2_contract": phase2_contract_summary(phase2_matrix_path),
        "benchmarks": {},
    }

    if not args.skip_sidecar:
        report["benchmarks"]["forensic_sidecar"] = benchmark_forensic_sidecar(
            args.forensic_url, args.tenant_id, args.collection_id, max(1, args.rounds)
        )
    if args.chat_model:
        report["benchmarks"]["chat"] = benchmark_chat(args.localai_url, args.chat_model, max(1, args.rounds))
    if args.embedding_model:
        report["benchmarks"]["embeddings"] = benchmark_embeddings(args.localai_url, args.embedding_model, max(1, args.rounds))

    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, indent=2, sort_keys=True), encoding="utf-8")
    print(f"Wrote benchmark report to {output.resolve()}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
