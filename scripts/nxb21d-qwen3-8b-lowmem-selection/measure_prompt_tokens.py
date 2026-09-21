"""Measure bounded residual prompts with the pinned model tokenizer, without inference."""

import argparse
import json
import math
import time
import urllib.request
from pathlib import Path


SYSTEM = (
    "Choose one server-issued tuple matching the analyst's residual intent. "
    'Return only a one-field JSON object: {"decision":"<one allowed enum value>"}. '
    "Never generate facts, parameters, scope, tools or operations. "
    "Choose CLARIFY:AMBIGUOUS_INTENT for ambiguous intent or "
    "CLARIFY:INSUFFICIENT_FACTS for insufficient facts; otherwise choose one tuple ID."
)


def compact(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))


def tokenize(url, model, content):
    request = urllib.request.Request(
        url.rstrip("/") + "/v1/tokenize",
        data=compact({"model": model, "content": content}).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    started = time.perf_counter()
    with urllib.request.urlopen(request, timeout=180) as response:
        count = len(json.load(response)["tokens"])
    return count, round((time.perf_counter() - started) * 1000)


def percentile_nearest_rank(values, percentile):
    ordered = sorted(values)
    return ordered[max(0, math.ceil(percentile * len(ordered)) - 1)]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--bindings", required=True)
    parser.add_argument("--model", required=True)
    parser.add_argument("--url", default="http://127.0.0.1:8080")
    parser.add_argument("--observed-case", default="sel12-en-resolved")
    parser.add_argument("--observed-prompt-tokens", type=int, default=365)
    args = parser.parse_args()

    corpus = json.loads(Path(args.corpus).read_text(encoding="utf-8"))
    bindings = json.loads(Path(args.bindings).read_text(encoding="utf-8"))
    rows = []
    for case in corpus["cases"]:
        binding = bindings[case["id"]]
        user = case["query"] + "\nValid tuples:\n" + compact(binding["tuples"])
        raw_prompt_tokens, prompt_tokenize_ms = tokenize(
            args.url, args.model, SYSTEM + "\n" + user
        )
        schema_tokens, schema_tokenize_ms = tokenize(
            args.url, args.model, compact(binding["schema"])
        )
        rows.append(
            {
                "id": case["id"],
                "raw_content_tokens": raw_prompt_tokens,
                "schema_tokens": schema_tokens,
                "utf8_bytes": len((SYSTEM + "\n" + user).encode("utf-8")),
                "prompt_tokenize_ms": prompt_tokenize_ms,
                "schema_tokenize_ms": schema_tokenize_ms,
            }
        )

    observed = next(row for row in rows if row["id"] == args.observed_case)
    template_overhead = args.observed_prompt_tokens - observed["raw_content_tokens"]
    for row in rows:
        row["calibrated_prompt_tokens"] = row["raw_content_tokens"] + template_overhead
    counts = [row["calibrated_prompt_tokens"] for row in rows]
    result = {
        "measurement": "pinned artifact tokenizer plus fixed chat-template overhead calibrated from the sealed successful request",
        "inference_performed": False,
        "model": args.model,
        "cases": len(rows),
        "observed_case": args.observed_case,
        "observed_prompt_tokens": args.observed_prompt_tokens,
        "calibrated_template_overhead_tokens": template_overhead,
        "max_prompt_tokens": max(counts),
        "p95_prompt_tokens": percentile_nearest_rank(counts, 0.95),
        "max_schema_overhead_tokens": max(row["schema_tokens"] for row in rows),
        "rows": rows,
    }
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))


if __name__ == "__main__":
    main()
