#!/usr/bin/env python3
"""Validate and summarize the frozen NX-B2.1D Q4 development corpus."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
import unicodedata
from collections import Counter
from pathlib import Path


LANGUAGES = {"english", "urdu-script", "roman-urdu", "mixed"}
CAPABILITIES = {"image.plate", "document.phrase", "transcript.phrase"}
SEMANTICS = {"EXACT_VALUE", "PHRASE_CONTAINS"}
SCOPES = {"selected", "workspace"}


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
    consumed = repo / "reports/nxb21/d-unseen-query-corpus-v1.json"
    consumed_data = json.loads(consumed.read_text(encoding="utf-8-sig"))
    prior.update(normalized(case["query"]) for case in consumed_data["cases"])

    challenger = repo / "local-acceptance-models/nxb21-d/challengers/qwen3-4b-instruct-2507-q4km"
    current_corpus_outputs = (challenger / "localai-dev-results/32case-development").resolve()
    for path in challenger.rglob("*"):
        resolved = path.resolve()
        if (
            not path.is_file()
            or resolved == corpus_path.resolve()
            or current_corpus_outputs in resolved.parents
        ):
            continue
        if path.suffix.lower() == ".json":
            try:
                value = json.loads(path.read_text(encoding="utf-8-sig"))
            except (UnicodeDecodeError, json.JSONDecodeError):
                continue
            prior.update(normalized(value) for value in strings(value))
        elif path.suffix.lower() == ".ps1":
            text = path.read_text(encoding="utf-8-sig", errors="strict")
            prior.update(
                normalized(match.group(2))
                for match in re.finditer(r"question\s*=\s*(['\"])(.*?)\1", text, re.IGNORECASE)
            )
    return prior


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--corpus", type=Path, required=True)
    args = parser.parse_args()
    repo = args.repo.resolve()
    corpus_path = args.corpus.resolve()
    raw = corpus_path.read_bytes()
    errors: list[str] = []
    if raw.startswith(b"\xef\xbb\xbf"):
        errors.append("corpus has a UTF-8 BOM")
    try:
        text = raw.decode("utf-8", errors="strict")
        corpus = json.loads(text)
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        print(json.dumps({"status": "FAIL", "errors": [str(exc)]}, indent=2))
        return 1

    cases = corpus.get("cases", [])
    if len(cases) != 32:
        errors.append(f"expected 32 cases, found {len(cases)}")
    ids = [case.get("id", "") for case in cases]
    questions = [case.get("question", "") for case in cases]
    if len(set(ids)) != len(ids) or any(not value for value in ids):
        errors.append("case IDs are empty or duplicated")
    normalized_questions = [normalized(value) for value in questions]
    if len(set(normalized_questions)) != len(normalized_questions) or any(not value for value in normalized_questions):
        errors.append("questions are empty or duplicated")

    prior = load_prior_strings(repo, corpus_path)
    overlaps = [case["id"] for case in cases if normalized(case["question"]) in prior]
    if overlaps:
        errors.append("prior-question overlap: " + ",".join(overlaps))

    for case in cases:
        case_id = case.get("id", "<missing>")
        language = case.get("language")
        expected = case.get("expected", {})
        capability = expected.get("query_capability_id")
        semantic = expected.get("semantic")
        scope = expected.get("scope")
        literal = expected.get("literal_text")
        entities = expected.get("entities")
        if language not in LANGUAGES:
            errors.append(f"{case_id}: invalid language")
        if capability not in CAPABILITIES:
            errors.append(f"{case_id}: invalid capability")
        if semantic not in SEMANTICS:
            errors.append(f"{case_id}: invalid semantic")
        if scope not in SCOPES:
            errors.append(f"{case_id}: invalid scope")
        if not isinstance(literal, str) or not literal or literal not in case.get("question", ""):
            errors.append(f"{case_id}: literal is absent from question")
        if expected.get("clarification_required") is not False:
            errors.append(f"{case_id}: clarification oracle must be false")
        if semantic == "EXACT_VALUE":
            if capability != "image.plate" or entities != [literal]:
                errors.append(f"{case_id}: exact-value oracle is inconsistent")
        elif entities != [] or capability == "image.plate":
            errors.append(f"{case_id}: phrase oracle is inconsistent")

    language_counts = Counter(case.get("language") for case in cases)
    if language_counts != Counter({key: 8 for key in LANGUAGES}):
        errors.append(f"language counts are invalid: {dict(language_counts)}")

    summary = {
        "schema_version": "nexusai.nxb21d-q4-development-corpus-validation/v1",
        "status": "PASS" if not errors else "FAIL",
        "development_only": True,
        "qualification_holdout": False,
        "corpus_sha256": hashlib.sha256(raw).hexdigest(),
        "utf8_no_bom": not raw.startswith(b"\xef\xbb\xbf"),
        "case_count": len(cases),
        "fresh_question_count": len(cases) - len(overlaps),
        "language": dict(sorted(language_counts.items())),
        "capability": dict(sorted(Counter(case["expected"]["query_capability_id"] for case in cases).items())),
        "semantic": dict(sorted(Counter(case["expected"]["semantic"] for case in cases).items())),
        "scope": dict(sorted(Counter(case["expected"]["scope"] for case in cases).items())),
        "errors": errors,
    }
    print(json.dumps(summary, indent=2, ensure_ascii=False))
    return 0 if not errors else 1


if __name__ == "__main__":
    sys.exit(main())
