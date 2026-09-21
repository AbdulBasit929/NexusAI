#!/usr/bin/env python3
"""Fail-closed freshness and registry-coverage checks for the one-shot corpus."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import unicodedata
from pathlib import Path


TEXT_SUFFIXES = {".go", ".json", ".md", ".py", ".ps1", ".txt", ".yaml", ".yml"}
PRIOR_CORPUS_MARKERS = ("corpus", "holdout", "decision", "selection", "remediation")
EXCLUDED_PARTS = {".git", "node_modules", "vendor", "local-acceptance-models"}


def normalize(value: str) -> str:
    value = unicodedata.normalize("NFKC", value).casefold()
    return " ".join(re.findall(r"[\w]+", value, flags=re.UNICODE))


def questions(corpus: dict) -> list[tuple[str, str]]:
    rows: list[tuple[str, str]] = []
    for section in ("semantic_cases", "router_cases", "synthesis_cases"):
        for case in corpus.get(section, []):
            rows.append((str(case["id"]), str(case["question"])))
    return rows


def certified_workspace_operations(certification: dict) -> set[str]:
    operations = set()
    entries = certification.get("entries", certification.get("operations", []))
    for operation in entries:
        operation_id = str(operation.get("operation_id", ""))
        if not operation_id:
            continue
        if operation.get("workspace_candidate") is True:
            operations.add(operation_id)
    if operations:
        return operations

    # Compatibility with the current certification artifact, whose public set is
    # the complete registry minus engineering-only and exact-evidence primitives.
    all_ids = {
        str(operation.get("operation_id", ""))
        for operation in entries
        if operation.get("operation_id")
    }
    excluded = {
        "face.candidate_observations",
        "video.timeline",
    }
    excluded.update(
        str(operation.get("operation_id"))
        for operation in entries
        if operation.get("exposure_status") == "engineering_only"
    )
    return all_ids - excluded


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True)
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--oracle", required=True)
    parser.add_argument("--certification", required=True)
    args = parser.parse_args()

    repo = Path(args.repo).resolve()
    corpus_path = Path(args.corpus).resolve()
    oracle_path = Path(args.oracle).resolve()
    certification_path = Path(args.certification).resolve()
    corpus = json.loads(corpus_path.read_text(encoding="utf-8"))
    oracle = json.loads(oracle_path.read_text(encoding="utf-8"))
    certification = json.loads(certification_path.read_text(encoding="utf-8"))

    corpus_questions = questions(corpus)
    normalized_questions = {normalize(question): case_id for case_id, question in corpus_questions}
    if len(normalized_questions) != len(corpus_questions):
        raise SystemExit("fresh corpus contains normalized duplicate questions")

    expected_operations = certified_workspace_operations(certification)
    supported_ids = {
        str(case["id"])
        for case in corpus.get("semantic_cases", [])
        if case.get("kind") == "SUPPORTED_UNAMBIGUOUS"
    }
    covered_operations = {
        str(decision).removeprefix("OPERATION:")
        for case_id, decision in oracle.get("semantic", {}).items()
        if case_id in supported_ids and str(decision).startswith("OPERATION:")
    }
    missing_operations = sorted(expected_operations - covered_operations)
    extra_operations = sorted(covered_operations - expected_operations)

    exact_hits: list[dict[str, str]] = []
    normalized_hits: list[dict[str, str]] = []
    prior_corpus_hits: list[dict[str, str]] = []
    files_scanned = 0
    exact_lookup = {question: case_id for case_id, question in corpus_questions}

    excluded_exact = {corpus_path, oracle_path, Path(__file__).resolve()}
    tracked = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        cwd=repo,
        check=True,
        capture_output=True,
    ).stdout.decode("utf-8", errors="surrogateescape")
    candidate_paths = sorted({repo / value for value in tracked.split("\0") if value})
    for path in candidate_paths:
        if not path.is_file() or path.resolve() in excluded_exact:
            continue
        if any(part in EXCLUDED_PARTS for part in path.parts):
            continue
        if path.suffix.lower() not in TEXT_SUFFIXES:
            continue
        try:
            if path.stat().st_size > 5 * 1024 * 1024:
                continue
            text = path.read_text(encoding="utf-8", errors="ignore")
        except OSError:
            continue
        files_scanned += 1
        relative = path.relative_to(repo).as_posix()
        candidate_literals = re.findall(r"[^\r\n]{12,600}", text)
        candidate_literals.extend(
            match.group(1)
            for match in re.finditer(r'"((?:\\.|[^"\\]){12,600})"', text)
        )
        matched: set[str] = set()
        for literal in candidate_literals:
            stripped = literal.strip(" \t\"'`,[]{}()")
            exact_case_id = exact_lookup.get(stripped)
            if exact_case_id and exact_case_id not in matched:
                exact_hits.append({"case_id": exact_case_id, "file": relative})
            normalized = normalize(stripped)
            case_id = normalized_questions.get(normalized)
            if case_id and case_id not in matched:
                hit = {"case_id": case_id, "file": relative}
                normalized_hits.append(hit)
                if any(marker in path.name.casefold() for marker in PRIOR_CORPUS_MARKERS):
                    prior_corpus_hits.append(hit)
                matched.add(case_id)

    report = {
        "contract_version": "nexusai.english-functional-freshness/v1",
        "corpus_id": corpus.get("corpus_id"),
        "files_scanned": files_scanned,
        "question_count": len(corpus_questions),
        "question_overlap": len(exact_hits),
        "normalized_literal_overlap": len(normalized_hits),
        "prior_corpus_overlap": len(prior_corpus_hits),
        "expected_workspace_operations": len(expected_operations),
        "covered_workspace_operations": len(covered_operations),
        "missing_operations": missing_operations,
        "extra_operations": extra_operations,
        "exact_hits": exact_hits,
        "normalized_hits": normalized_hits,
        "prior_corpus_hits": prior_corpus_hits,
        "status": "PASS",
    }
    if exact_hits or normalized_hits or prior_corpus_hits or missing_operations or extra_operations:
        report["status"] = "FAIL"
    json.dump(report, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 0 if report["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
