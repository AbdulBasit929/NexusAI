#!/usr/bin/env python3
"""Run an isolated LocalAI embedding or reranker fixture benchmark."""

from __future__ import annotations

import argparse
import json
import math
import os
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

try:
    import resource
except ImportError:  # Windows orchestration uses the bundled Python runtime.
    resource = None


def request_json(url: str, payload: dict[str, Any] | None = None, timeout: float = 30) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.loads(response.read().decode("utf-8"))


def wait_ready(base_url: str, process: subprocess.Popen[str] | None, timeout: float = 90) -> None:
    deadline = time.monotonic() + timeout
    last_error = "not started"
    while time.monotonic() < deadline:
        if process is not None and process.poll() is not None:
            raise RuntimeError(f"LocalAI exited before readiness: {process.returncode}")
        try:
            with urllib.request.urlopen(f"{base_url}/readyz", timeout=2) as response:
                if 200 <= response.status < 300:
                    return
        except (OSError, ValueError, urllib.error.HTTPError) as exc:
            last_error = str(exc)
            time.sleep(0.25)
    raise RuntimeError(f"LocalAI readiness timeout: {last_error}")


def cosine(left: list[float], right: list[float]) -> float:
    numerator = sum(a * b for a, b in zip(left, right))
    denominator = math.sqrt(sum(a * a for a in left)) * math.sqrt(sum(b * b for b in right))
    return numerator / denominator if denominator else 0.0


def dcg(ranked: list[str], expected: set[str], k: int) -> float:
    return sum((1.0 if item in expected else 0.0) / math.log2(index + 2) for index, item in enumerate(ranked[:k]))


def metrics(rows: list[dict[str, Any]], k_values: tuple[int, ...] = (1, 3, 5)) -> dict[str, Any]:
    answerable = [row for row in rows if row["expected_document_ids"]]
    output: dict[str, Any] = {"queries": len(rows), "answerable": len(answerable)}
    for k in k_values:
        recalls = []
        precisions = []
        ndcgs = []
        for row in answerable:
            expected = set(row["expected_document_ids"])
            selected = row["ranked_document_ids"][:k]
            relevant = sum(item in expected for item in selected)
            recalls.append(relevant / len(expected))
            precisions.append(relevant / k)
            ideal = sum(1 / math.log2(index + 2) for index in range(min(k, len(expected))))
            ndcgs.append(dcg(selected, expected, k) / ideal if ideal else 0.0)
        output[f"recall_at_{k}"] = sum(recalls) / len(recalls) if recalls else None
        output[f"precision_at_{k}"] = sum(precisions) / len(precisions) if precisions else None
        output[f"ndcg_at_{k}"] = sum(ndcgs) / len(ndcgs) if ndcgs else None
    reciprocal = []
    for row in answerable:
        expected = set(row["expected_document_ids"])
        rank = next((index for index, item in enumerate(row["ranked_document_ids"], 1) if item in expected), None)
        row["known_answer_rank"] = rank
        reciprocal.append(1 / rank if rank else 0.0)
    no_answer = [row for row in rows if not row["expected_document_ids"]]
    output["mrr"] = sum(reciprocal) / len(reciprocal) if reciprocal else None
    output["no_answer_abstention_accuracy"] = (
        sum(row["abstained"] for row in no_answer) / len(no_answer) if no_answer else None
    )
    output["citation_precision_at_1"] = output["precision_at_1"]
    return output


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=("embedding", "reranker"), required=True)
    parser.add_argument("--fixture", type=Path, required=True)
    parser.add_argument("--model-config", type=Path, required=True)
    parser.add_argument("--model-name", required=True)
    parser.add_argument("--models-path", type=Path, required=True)
    parser.add_argument("--backends-path", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--limit", type=int)
    parser.add_argument("--base-url", default="http://127.0.0.1:18080")
    parser.add_argument("--external-server", action="store_true")
    parser.add_argument("--request-timeout-seconds", type=float, default=120)
    args = parser.parse_args()
    fixture = json.loads(args.fixture.read_text(encoding="utf-8"))
    queries = list(fixture["queries"])
    if args.limit is not None:
        queries = queries[:args.limit]
    base_url = args.base_url.rstrip("/")
    log_path = args.output.with_suffix(".localai.log")
    log_path.parent.mkdir(parents=True, exist_ok=True)
    environment = dict(os.environ)
    environment.update({"LOCALAI_THREADS": "8", "LOCALAI_CONTEXT_SIZE": "2048"})
    command = [
        "/local-ai", "run", "--address=127.0.0.1:18080",
        f"--models-path={args.models_path}",
        f"--models-config-file={args.model_config}",
        f"--backends-path={args.backends_path}",
        "--data-path=/tmp/data", "--localai-config-dir=/tmp/config",
        "--generated-content-path=/tmp/generated", "--upload-path=/tmp/upload",
        "--disable-web-ui", "--disable-gallery-endpoint", "--disable-mcp",
        "--disable-agents", "--disable-local-ai-assistant", "--max-active-backends=1",
        "--log-level=error",
    ]
    wall_started = time.perf_counter()
    cpu_started = time.process_time()
    with log_path.open("w", encoding="utf-8") as log:
        process = None
        if not args.external_server:
            process = subprocess.Popen(command, stdout=log, stderr=subprocess.STDOUT, text=True, env=environment)
        try:
            wait_ready(base_url, process)
            documents = fixture["documents"]
            document_ids = [item["id"] for item in documents]
            document_text = [item["text"] for item in documents]
            rows = []
            latencies = []
            for query in queries:
                started = time.perf_counter()
                if args.mode == "embedding":
                    response = request_json(f"{base_url}/v1/embeddings", {
                        "model": args.model_name, "input": [query["query"], *document_text],
                    }, timeout=args.request_timeout_seconds)
                    vectors = [item["embedding"] for item in sorted(response["data"], key=lambda item: item["index"])]
                    scores = [cosine(vectors[0], vector) for vector in vectors[1:]]
                    threshold = float(fixture["thresholds"]["embedding_no_answer_max_cosine"])
                else:
                    response = request_json(f"{base_url}/v1/rerank", {
                        "model": args.model_name, "query": query["query"],
                        "documents": document_text, "top_n": len(document_text),
                    }, timeout=args.request_timeout_seconds)
                    score_by_index = {
                        int(item["index"]): float(item.get("relevance_score", item.get("score", 0)))
                        for item in response.get("results", response.get("data", []))
                    }
                    scores = [score_by_index.get(index, float("-inf")) for index in range(len(document_text))]
                    threshold = float(fixture["thresholds"]["reranker_no_answer_max_relevance"])
                latency = time.perf_counter() - started
                latencies.append(latency)
                ranked_indices = sorted(range(len(scores)), key=lambda index: (-scores[index], index))
                rows.append({
                    **query,
                    "ranked_document_ids": [document_ids[index] for index in ranked_indices],
                    "scores": [{"document_id": document_ids[index], "score": scores[index]} for index in ranked_indices],
                    "top_score": max(scores),
                    "abstained": max(scores) < threshold,
                    "latency_seconds": latency,
                })
        finally:
            if process is not None:
                process.terminate()
                try:
                    process.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)
    strata = {}
    for locale in sorted({row["locale"] for row in rows}):
        strata[locale] = metrics([row for row in rows if row["locale"] == locale])
    report = {
        "contract_version": "nexusai.nxmmr.retrieval-benchmark/v1",
        "mode": args.mode,
        "model": args.model_name,
        "evidence_tier": "FIXTURE",
        "candidate_set_digest": hashlib_digest(document_ids, document_text),
        "identical_candidate_set_for_every_query": True,
        "query_count": len(rows),
        "metrics": metrics(rows),
        "strata": strata,
        "resource": {
            "wall_seconds": time.perf_counter() - wall_started,
            "orchestrator_cpu_seconds": time.process_time() - cpu_started,
            "peak_orchestrator_rss_mib": (
                resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024 if resource is not None else None
            ),
        },
        "rows": rows,
        "promotion_authority": "FIXTURE_CERTIFIED_ONLY",
        "retained_state_mutated": False,
    }
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"mode": args.mode, "metrics": report["metrics"], "resource": report["resource"]}, indent=2))


def hashlib_digest(document_ids: list[str], document_text: list[str]) -> str:
    import hashlib

    payload = "\n".join(f"{identity}\t{text}" for identity, text in zip(document_ids, document_text))
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


if __name__ == "__main__":
    main()
