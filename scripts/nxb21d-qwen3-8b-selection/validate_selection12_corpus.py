"""Read-only freshness and frozen-contract admission for the 8B selection gate."""
import argparse
import hashlib
import json
import unicodedata
from collections import Counter
from pathlib import Path


def norm(value):
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


def validate(repo, corpus):
    raw = corpus.read_bytes()
    assert not raw.startswith(b"\xef\xbb\xbf"), "BOM"
    data = json.loads(raw.decode("utf-8"))
    cases = data["cases"]
    assert data["development_only"] and not data["qualification_holdout"]
    assert len(cases) == 12 and all(case["model_call"] for case in cases)
    assert Counter(case["language"] for case in cases) == {
        "en": 3,
        "ur": 3,
        "roman_ur": 3,
        "mixed": 3,
    }
    assert len({case["id"] for case in cases}) == 12
    assert len({norm(case["query"]) for case in cases}) == 12
    for language in ("en", "ur", "roman_ur", "mixed"):
        subset = [case for case in cases if case["language"] == language]
        assert sum("capability" in case["expected"] for case in subset) == 1
        assert Counter(
            case["expected"].get("state", "RESOLVED") for case in subset
        ) == {"RESOLVED": 1, "AMBIGUOUS_INTENT": 1, "INSUFFICIENT_FACTS": 1}
    values = [value for case in cases for value in case["values"]]
    assert len(values) == len(set(map(norm, values))) == 12
    assert all(value in case["query"] for case in cases for value in case["values"])
    acceptance = data["acceptance"]
    assert acceptance["expected_model_calls"] == 12
    assert acceptance["completion_budget"] == 512

    bindings = json.loads((corpus.parent / "schema-bindings-v1.json").read_text(encoding="utf-8"))
    refs = json.loads(
        (repo / "api/forensic_records/contracts/query-capability-references-v1.json").read_text(
            encoding="utf-8"
        )
    )
    refs = {item["query_capability_id"]: item for item in refs["capabilities"]}
    for case in cases:
        binding = bindings[case["id"]]
        assert len(binding["tuples"]) > 1 and binding["facts"]["state"] == ""
        schema = binding["schema"]
        assert schema["required"] == ["decision"]
        assert set(schema["properties"]) == {"decision"}
        choices = schema["properties"]["decision"]["enum"]
        expected = case["expected"]
        decision = (
            "CLARIFY:" + expected["state"]
            if "state" in expected
            else "/".join(expected[key] for key in ("capability", "semantic", "scope"))
        )
        assert decision in choices
        if "capability" in expected:
            assert expected["semantic"] in refs[expected["capability"]]["semantics"]
            assert expected["scope"] in refs[expected["capability"]]["scope_modes"]

    historical = set()
    hashes = {}
    excluded = []
    current_parts = {"nxb21d-qwen3-8b-selection", "qwen3-8b-selection-runs"}
    for folder in ("scripts", "reports/nxb21", "local-acceptance-models/nxb21-d"):
        for path in (repo / folder).rglob("*"):
            if not path.is_file() or path.suffix.lower() not in (".json", ".jsonl", ".txt"):
                continue
            relative = path.relative_to(repo).as_posix()
            if current_parts.intersection(path.parts):
                continue
            if any(part in ("go-cache", "go-tmp", "gocache") for part in path.parts):
                continue
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
                historical.update(norm(item) for item in strings(json.loads(text)) if item)
            except ValueError:
                historical.add(norm(text))
    for required in (
        "d-unseen-query-corpus-v1.json",
        "nxb21d-q4-independent-168case-holdout-v1.json",
        "hybrid-32case-corpus-v1.json",
        "decision-8case-corpus-v1.json",
    ):
        assert any(path.endswith("/" + required) for path in hashes), "missing history: " + required
    for case in cases:
        assert not any(norm(case["query"]) in old for old in historical), (
            "historical question overlap: " + case["id"]
        )
        for value in case["values"]:
            assert not any(norm(value) in old for old in historical), "historical value overlap: " + value
    return {
        "status": "PASS",
        "cases": 12,
        "languages": {"en": 3, "ur": 3, "roman_ur": 3, "mixed": 3},
        "resolved": 4,
        "ambiguous": 4,
        "insufficient": 4,
        "model_calls_expected": 12,
        "question_overlap": 0,
        "value_overlap": 0,
        "corpus_sha256": hashlib.sha256(raw).hexdigest(),
        "historical_files": len(hashes),
        "historical_hashes": hashes,
        "excluded_files": excluded,
        "scan_scope": "All JSON/JSONL/text under scripts, reports/nxb21, and local-acceptance-models/nxb21-d; excludes the current selection bundle/runs, caches, oversized files, and non-UTF8 files.",
    }


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True)
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--output")
    args = parser.parse_args()
    result = validate(Path(args.repo).resolve(), Path(args.corpus).resolve())
    if args.output:
        Path(args.output).write_text(
            json.dumps(result, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
        )
    print(
        json.dumps(
            {key: value for key, value in result.items() if key not in ("historical_hashes", "excluded_files")},
            sort_keys=True,
        )
    )
