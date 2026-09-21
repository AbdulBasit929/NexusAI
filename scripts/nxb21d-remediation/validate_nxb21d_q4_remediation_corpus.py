#!/usr/bin/env python3
"""Validate the fresh NX-B2.1D remediation development corpus."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import unicodedata
from collections import Counter
from pathlib import Path


LANGUAGES = {"english", "urdu-script", "roman-urdu", "mixed"}
CAPABILITIES = {"image.plate", "document.phrase", "transcript.phrase"}
PLATE_PATTERN = re.compile(r"(?i)\b[A-Z]{1,4}[- ]?\d{1,6}[A-Z]{0,3}\b")


def normalized(value: str) -> str:
    return unicodedata.normalize("NFC", value).strip().casefold()


def strings(value):
    if isinstance(value, dict):
        for child in value.values():
            yield from strings(child)
    elif isinstance(value, list):
        for child in value:
            yield from strings(child)
    elif isinstance(value, str):
        yield value


def load_prior_strings(repo: Path, corpus_path: Path) -> set[str]:
    prior: set[str] = set()
    roots = [
        repo / "reports/nxb21/d-unseen-query-corpus-v1.json",
        repo / "scripts/nxb21d-development",
        repo / "local-acceptance-models/nxb21-d/challengers/qwen3-4b-instruct-2507-q4km",
    ]
    remediation_root = (repo / "scripts/nxb21d-remediation").resolve()
    remediation_outputs = (
        repo
        / "local-acceptance-models/nxb21-d/challengers/qwen3-4b-instruct-2507-q4km/localai-dev-results/16case-remediation"
    ).resolve()
    visited: set[Path] = set()
    for root in roots:
        candidates = [root] if root.is_file() else root.rglob("*")
        for path in candidates:
            if not path.is_file():
                continue
            resolved = path.resolve()
            if resolved in visited or resolved == corpus_path.resolve():
                continue
            visited.add(resolved)
            if remediation_root in resolved.parents or remediation_outputs in resolved.parents:
                continue
            if path.suffix.lower() == ".json":
                try:
                    value = json.loads(path.read_text(encoding="utf-8-sig"))
                except (UnicodeDecodeError, json.JSONDecodeError):
                    continue
                prior.update(normalized(item) for item in strings(value))
            elif path.suffix.lower() == ".ps1":
                text = path.read_text(encoding="utf-8-sig", errors="strict")
                prior.update(
                    normalized(match.group(2))
                    for match in re.finditer(r"question\s*=\s*(['\"])(.*?)\1", text, re.IGNORECASE)
                )
    return prior


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True, type=Path)
    parser.add_argument("--corpus", required=True, type=Path)
    args = parser.parse_args()
    repo = args.repo.resolve()
    corpus_path = args.corpus.resolve()
    raw = corpus_path.read_bytes()
    errors: list[str] = []
    if raw.startswith(b"\xef\xbb\xbf"):
        errors.append("corpus has a UTF-8 BOM")
    try:
        corpus = json.loads(raw.decode("utf-8", errors="strict"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        print(json.dumps({"status": "FAIL", "errors": [str(exc)]}, indent=2))
        return 1

    cases = corpus.get("cases", [])
    if len(cases) != 16:
        errors.append(f"expected 16 cases, found {len(cases)}")
    ids = [case.get("id", "") for case in cases]
    questions = [case.get("question", "") for case in cases]
    if len(set(ids)) != len(ids) or any(not item for item in ids):
        errors.append("case IDs are empty or duplicated")
    normalized_questions = [normalized(item) for item in questions]
    if len(set(normalized_questions)) != len(normalized_questions) or any(not item for item in normalized_questions):
        errors.append("questions are empty or duplicated")

    prior = load_prior_strings(repo, corpus_path)
    overlaps: list[str] = []
    reused_literals: list[str] = []
    for case in cases:
        if normalized(case.get("question", "")) in prior:
            overlaps.append(case.get("id", "<missing>"))
        literal = case.get("expected", {}).get("literal_text", "")
        if normalized(literal) in prior:
            reused_literals.append(case.get("id", "<missing>"))
    if overlaps:
        errors.append("prior-question overlap: " + ",".join(overlaps))
    if reused_literals:
        errors.append("prior-literal reuse: " + ",".join(reused_literals))

    for case in cases:
        case_id = case.get("id", "<missing>")
        language = case.get("language")
        question = case.get("question", "")
        expected = case.get("expected", {})
        capability = expected.get("query_capability_id")
        semantic = expected.get("semantic")
        literal = expected.get("literal_text")
        entities = expected.get("entities")
        scope = expected.get("scope")
        if language not in LANGUAGES:
            errors.append(f"{case_id}: invalid language")
        if capability not in CAPABILITIES:
            errors.append(f"{case_id}: invalid capability")
        if not isinstance(literal, str) or not literal or literal not in question:
            errors.append(f"{case_id}: literal is absent from question")
        if expected.get("clarification_required") is not False:
            errors.append(f"{case_id}: clarification oracle must be false")
        if capability == "image.plate":
            if semantic != "EXACT_VALUE" or scope != "selected" or entities != [literal]:
                errors.append(f"{case_id}: image.plate must be one selected exact target")
            if PLATE_PATTERN.findall(question) != [literal]:
                errors.append(f"{case_id}: bounded plate extraction is not exactly the literal")
        else:
            if semantic != "PHRASE_CONTAINS" or scope not in {"selected", "workspace"} or entities != []:
                errors.append(f"{case_id}: phrase oracle is inconsistent")

    language = Counter(case.get("language") for case in cases)
    capability = Counter(case.get("expected", {}).get("query_capability_id") for case in cases)
    semantic = Counter(case.get("expected", {}).get("semantic") for case in cases)
    scope = Counter(case.get("expected", {}).get("scope") for case in cases)
    if language != Counter({item: 4 for item in LANGUAGES}):
        errors.append(f"language counts are invalid: {dict(language)}")
    if capability != Counter({"image.plate": 8, "document.phrase": 4, "transcript.phrase": 4}):
        errors.append(f"capability counts are invalid: {dict(capability)}")
    if semantic != Counter({"EXACT_VALUE": 8, "PHRASE_CONTAINS": 8}):
        errors.append(f"semantic counts are invalid: {dict(semantic)}")
    if scope != Counter({"selected": 12, "workspace": 4}):
        errors.append(f"scope counts are invalid for the actual image.plate contract: {dict(scope)}")

    summary = {
        "schema_version": "nexusai.nxb21d-q4-remediation-corpus-validation/v1",
        "status": "PASS" if not errors else "FAIL",
        "development_only": True,
        "qualification_holdout": False,
        "corpus_sha256": hashlib.sha256(raw).hexdigest(),
        "utf8_no_bom": not raw.startswith(b"\xef\xbb\xbf"),
        "case_count": len(cases),
        "fresh_question_count": len(cases) - len(overlaps),
        "fresh_literal_count": len(cases) - len(reused_literals),
        "language": dict(sorted(language.items())),
        "capability": dict(sorted(capability.items())),
        "semantic": dict(sorted(semantic.items())),
        "scope": dict(sorted(scope.items())),
        "image_plate_contract": "SINGLE_TARGET_SELECTED_EVIDENCE",
        "errors": errors,
    }
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0 if not errors else 1


if __name__ == "__main__":
    raise SystemExit(main())
