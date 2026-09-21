#!/usr/bin/env python3
"""Validate the fresh NX-B2.1D request-bound-schema development corpus."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from collections import Counter
from pathlib import Path


LANGUAGES = {"en", "ur", "roman_ur", "mixed"}
STATES = {"ARBITRARY_EXECUTION", "SCOPE_CONFLICT", "BUDGET_EXCEEDED", "UNAVAILABLE"}
PRIOR_CORPORA = (
    "reports/nxb21/d-unseen-query-corpus-v1.json",
    "scripts/nxb21d-development/nxb21d-q4-32case-development-corpus-v1.json",
    "scripts/nxb21d-remediation/nxb21d-q4-remediation-16case-development-corpus-v1.json",
    "scripts/nxb21d-phrase-remediation/nxb21d-q4-phrase-remediation-20case-development-corpus-v1.json",
    "scripts/nxb21d-scope-remediation/nxb21d-q4-scope-remediation-16case-development-corpus-v1.json",
    "scripts/nxb21d-q4-qualification/nxb21d-q4-independent-168case-holdout-v1.json",
    "scripts/nxb21d-interface-remediation/nxb21d-q4-production-interface-24case-development-corpus-v1.json",
)
CONSUMED_HASHES = {
    "eb102cdd1f1fa1d9da02c5dce4c8c9e216a523ba78d4468ce9802f880170283f",
    "aaa8517983ea4523e235c4b3de31e5562bfaeada21ef24a7b23065a567d31d13",
}


def read_json(path: Path):
    raw = path.read_bytes()
    if raw.startswith(b"\xef\xbb\xbf"):
        raise ValueError(f"UTF8_BOM:{path}")
    return json.loads(raw.decode("utf-8")), raw


def strings(value):
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for child in value.values():
            yield from strings(child)
    elif isinstance(value, list):
        for child in value:
            yield from strings(child)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True)
    parser.add_argument("--corpus", required=True)
    args = parser.parse_args()
    repo = Path(args.repo).resolve()
    corpus_path = Path(args.corpus).resolve()
    corpus, raw = read_json(corpus_path)
    failures: list[str] = []
    cases = corpus.get("cases", [])
    if corpus.get("development_only") is not True or corpus.get("qualification_holdout") is not False:
        failures.append("development-classification")
    if len(cases) != 24:
        failures.append(f"case-count:{len(cases)}")
    counts = Counter(case.get("language") for case in cases)
    if counts != Counter({language: 6 for language in LANGUAGES}):
        failures.append(f"language-counts:{dict(counts)}")
    ids = [case.get("id", "") for case in cases]
    questions = [case.get("query", "") for case in cases]
    if len(ids) != len(set(ids)) or any(not item for item in ids):
        failures.append("case-ids-not-unique")
    if len(questions) != len(set(questions)) or any(not item.strip() for item in questions):
        failures.append("questions-not-unique")
    values = []
    value_owners: dict[str, str] = {}
    for case in cases:
        expected = case.get("expected", {})
        for key in ("target", "literal"):
            value = expected.get(key)
            if value:
                owner = value_owners.get(value)
                if owner is not None and owner != case.get("id"):
                    failures.append(f"target-or-literal-not-unique:{value}")
                value_owners[value] = case.get("id", "")
                if value not in values:
                    values.append(value)
        state = expected.get("state")
        if state:
            if state not in STATES:
                failures.append(f"invalid-state:{case.get('id')}:{state}")
        elif not all(key in expected for key in ("capability", "semantic", "scope")):
            failures.append(f"incomplete-oracle:{case.get('id')}")
    refs, _ = read_json(repo / "api/forensic_records/contracts/query-capability-references-v1.json")
    capabilities = {item["query_capability_id"]: item for item in refs["capabilities"]}
    for case in cases:
        expected = case["expected"]
        if "state" in expected:
            continue
        capability = expected["capability"]
        ref = capabilities.get(capability)
        if ref is None:
            failures.append(f"unknown-capability:{case['id']}:{capability}")
            continue
        semantic = expected["semantic"]
        derived_ok = semantic == "CALENDAR_TIME" and capability == "cdr.time_activity"
        if semantic not in ref["semantics"] and not derived_ok:
            failures.append(f"invalid-semantic:{case['id']}:{semantic}")
        if expected["scope"] not in ref["scope_modes"]:
            failures.append(f"invalid-scope:{case['id']}:{expected['scope']}")

    prior_strings: set[str] = set()
    prior_hashes: dict[str, str] = {}
    for relative in PRIOR_CORPORA:
        path = repo / relative
        if not path.is_file():
            failures.append(f"missing-prior-corpus:{relative}")
            continue
        data, prior_raw = read_json(path)
        prior_hashes[relative] = hashlib.sha256(prior_raw).hexdigest()
        prior_strings.update(text for text in strings(data) if text)
    for question in questions:
        if question in prior_strings:
            failures.append(f"prior-question-overlap:{question}")
    for value in values:
        if value in prior_strings:
            failures.append(f"prior-value-overlap:{value}")

    output_root = (repo / "local-acceptance-models/nxb21-d/interface-remediation-development-runs").resolve()
    direct_probe_questions: set[str] = set()
    dev_root = repo / "local-acceptance-models/nxb21-d/challengers/qwen3-4b-instruct-2507-q4km/localai-dev-results"
    for path in dev_root.rglob("request-utf8-no-bom.json") if dev_root.exists() else ():
        if output_root in path.resolve().parents:
            continue
        try:
            request, _ = read_json(path)
            for message in request.get("messages", []):
                if message.get("role") == "user":
                    direct_probe_questions.add(message.get("content", ""))
        except (ValueError, UnicodeDecodeError, json.JSONDecodeError):
            continue
    for question in questions:
        if question in direct_probe_questions:
            failures.append(f"direct-probe-overlap:{question}")

    corpus_hash = hashlib.sha256(raw).hexdigest()
    if corpus_hash in CONSUMED_HASHES or corpus_hash in prior_hashes.values():
        failures.append(f"corpus-hash-overlap:{corpus_hash}")
    quoted = sum(1 for question in questions if re.search(r'"[^"\r\n]+"', question))
    model_cases = sum(1 for case in cases if "state" not in case["expected"])
    critical = sum(1 for case in cases if case.get("critical"))
    result = {
        "schema_version": "nexusai.nxb21d-q4-interface-corpus-validation/v1",
        "status": "PASS" if not failures else "FAIL",
        "sha256": corpus_hash,
        "bytes": len(raw),
        "cases": len(cases),
        "languages": dict(sorted(counts.items())),
        "model_cases": model_cases,
        "critical_cases": critical,
        "quoted_phrase_cases": quoted,
        "prior_corpora_checked": len(prior_hashes),
        "direct_probe_requests_checked": len(direct_probe_questions),
        "consumed_hashes_excluded": sorted(CONSUMED_HASHES),
        "failures": failures,
    }
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    return 0 if not failures else 1


if __name__ == "__main__":
    raise SystemExit(main())
