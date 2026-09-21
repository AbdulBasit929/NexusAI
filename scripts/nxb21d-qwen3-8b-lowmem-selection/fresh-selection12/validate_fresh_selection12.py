"""Strict freshness and frozen source-admission validation for fresh selection12."""
import argparse
import hashlib
import json
import unicodedata
from collections import Counter
from pathlib import Path


TEXT_SUFFIXES = {".json", ".jsonl", ".txt", ".md", ".ps1", ".py", ".go", ".yaml", ".yml"}


def norm(value: str) -> str:
    return " ".join(unicodedata.normalize("NFC", value).casefold().split())


def strings(value):
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for item in value.values():
            yield from strings(item)
    elif isinstance(value, list):
        for item in value:
            yield from strings(item)


def expected_decision(case):
    expected = case["expected"]
    if "state" in expected:
        return "CLARIFY:" + expected["state"]
    return "/".join(expected[key] for key in ("capability", "semantic", "scope"))


def validate(repo: Path, corpus_path: Path):
    raw = corpus_path.read_bytes()
    assert not raw.startswith(b"\xef\xbb\xbf"), "corpus BOM is forbidden"
    corpus = json.loads(raw.decode("utf-8"))
    cases = corpus["cases"]
    assert corpus["development_only"] is True and corpus["qualification_holdout"] is False
    assert corpus["decision_contract"] == "forensics.hybrid-decision/v2"
    assert len(cases) == 12 and all(case["model_call"] is True for case in cases)
    assert Counter(case["language"] for case in cases) == {"en": 3, "ur": 3, "roman_ur": 3, "mixed": 3}
    assert Counter(case["decision_class"] for case in cases) == {"resolved": 4, "ambiguous": 4, "insufficient": 4}
    assert len({case["id"] for case in cases}) == 12
    assert len({norm(case["query"]) for case in cases}) == 12
    values = [value for case in cases for value in case["values"]]
    literals = [value for case in cases for value in case["freshness_literals"]]
    assert len(values) == len(set(map(norm, values))) == 12
    assert len(literals) == len(set(map(norm, literals))) == 12
    for case in cases:
        assert all(value in case["query"] for value in case["values"])
        assert all(norm(value) in norm(case["query"]) for value in case["freshness_literals"])
        outcome = case["expected_final_typed_outcome"]
        if case["decision_class"] == "resolved":
            assert "capability" in case["expected"] and outcome["state"] == "hybrid_model_plan"
            for key in ("capability", "semantic", "scope", "target"):
                assert outcome[key] == case["expected"][key]
        else:
            assert outcome == {"state": "hybrid_clarification", "clarification_code": case["expected"]["state"]}
    for language in ("en", "ur", "roman_ur", "mixed"):
        assert Counter(case["decision_class"] for case in cases if case["language"] == language) == {
            "resolved": 1, "ambiguous": 1, "insufficient": 1
        }
    acceptance = corpus["acceptance"]
    for key in ("cases_completed", "model_calls", "decision_correct", "http_success", "strict_utf8", "schema_valid", "finish_reason_stop"):
        assert acceptance[key] == 12
    for key in ("malformed", "unknown_decision", "truncation", "fact_override", "scope_override", "authorization_override"):
        assert acceptance[key] == 0
    assert acceptance["completion_budget"] == 512

    bindings_path = corpus_path.parent / "source-bindings-v1.json"
    bindings = json.loads(bindings_path.read_text(encoding="utf-8"))
    assert set(bindings) == {case["id"] for case in cases}
    refs = json.loads((repo / "api/forensic_records/contracts/query-capability-references-v1.json").read_text(encoding="utf-8"))
    refs = {item["query_capability_id"]: item for item in refs["capabilities"]}
    for case in cases:
        binding = bindings[case["id"]]
        admission = binding["admission"]
        assert admission == {
            "model_call_required": True,
            "fact_packet_valid": "PASS",
            "candidate_set_valid": "PASS",
            "expected_decision_reachable": "PASS",
            "authorization_valid": "PASS",
            "case_scope_valid": "PASS",
            "final_plan_valid": "PASS",
        }
        assert binding["facts"]["state"] == "" and len(binding["tuples"]) > 1
        schema = binding["schema"]
        assert schema["required"] == ["decision"] and set(schema["properties"]) == {"decision"}
        assert expected_decision(case) in schema["properties"]["decision"]["enum"]
        assert binding["expected_final_typed_outcome"] == case["expected_final_typed_outcome"]
        if "capability" in case["expected"]:
            assert case["expected"]["semantic"] in refs[case["expected"]["capability"]]["semantics"]
            assert case["expected"]["scope"] in refs[case["expected"]["capability"]]["scope_modes"]

    historical_strings = set()
    hashes = {}
    excluded = []
    current_dir = corpus_path.parent.resolve()
    scan_roots = ("scripts", "reports", "local-acceptance-models/nxb21-d")
    for folder in scan_roots:
        for path in (repo / folder).rglob("*"):
            if not path.is_file() or path.suffix.lower() not in TEXT_SUFFIXES:
                continue
            resolved = path.resolve()
            if current_dir == resolved.parent or current_dir in resolved.parents:
                continue
            if any(part.casefold() in {"go-cache", "go-tmp", "gocache", ".gocache"} for part in path.parts):
                continue
            relative = path.relative_to(repo).as_posix()
            if path.stat().st_size > 16 * 1024 * 1024:
                excluded.append(relative + ":size_limit")
                continue
            blob = path.read_bytes()
            try:
                text = blob.decode("utf-8-sig")
            except UnicodeError:
                excluded.append(relative + ":not_utf8")
                continue
            hashes[relative] = hashlib.sha256(blob).hexdigest()
            try:
                historical_strings.update(norm(item) for item in strings(json.loads(text)) if item)
            except ValueError:
                historical_strings.add(norm(text))
    for required in (
        "decision-8case-corpus-v1.json",
        "qwen3-8b-selection-12case-corpus-v1.json",
        "hybrid-32case-corpus-v1.json",
        "nxb21d-q4-independent-168case-holdout-v1.json",
    ):
        assert any(path.endswith("/" + required) for path in hashes), "missing required history: " + required

    question_overlap = []
    target_overlap = []
    literal_overlap = []
    for case in cases:
        if any(norm(case["query"]) in old for old in historical_strings):
            question_overlap.append(case["id"])
        for value in case["values"]:
            if any(norm(value) in old for old in historical_strings):
                target_overlap.append(value)
        for literal in case["freshness_literals"]:
            if any(norm(literal) in old for old in historical_strings):
                literal_overlap.append(literal)
    assert not question_overlap, "historical question overlap: " + repr(question_overlap)
    assert not target_overlap, "historical target overlap: " + repr(target_overlap)
    assert not literal_overlap, "historical normalized literal overlap: " + repr(literal_overlap)

    return {
        "schema_version": "nexusai.qwen3-8b-lowmem-fresh-selection12-freshness/v1",
        "status": "PASS",
        "files_scanned": len(hashes),
        "question_overlap": 0,
        "target_value_overlap": 0,
        "normalized_literal_overlap": 0,
        "cases": 12,
        "languages": {"en": 3, "ur": 3, "roman_ur": 3, "mixed": 3},
        "decisions": {"resolved": 4, "ambiguous": 4, "insufficient": 4},
        "model_calls_expected": 12,
        "corpus_sha256": hashlib.sha256(raw).hexdigest(),
        "historical_hashes": hashes,
        "excluded_files": excluded,
        "scan_scope": "UTF-8 JSON, JSONL, TXT, MD, PowerShell, Python, Go, YAML and YML under scripts, reports and local-acceptance-models/nxb21-d; current fresh bundle and cache/oversized/non-UTF8 files excluded.",
        "oracle_overlap_policy": "Expected classes and governed capability IDs may repeat; questions, exact target values, and case-declared normalized intent literals may not.",
    }


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True)
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--output")
    args = parser.parse_args()
    result = validate(Path(args.repo).resolve(), Path(args.corpus).resolve())
    if args.output:
        output = Path(args.output)
        if output.exists():
            raise FileExistsError(output)
        output.write_text(json.dumps(result, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps({key: value for key, value in result.items() if key not in {"historical_hashes", "excluded_files"}}, sort_keys=True))
