#!/usr/bin/env python3
import argparse
import hashlib
import json
from collections import Counter
from pathlib import Path

LANG_COUNTS = {"en": 65, "ur": 37, "roman_ur": 36, "mixed": 30}
CAP_COUNTS = {
    "cdr.identifier_lookup": 4, "cdr.frequent_contacts": 11, "cdr.time_activity": 8,
    "ipdr.endpoint": 4, "ipdr.sessions": 4, "subscriber.lookup": 4,
    "device.observations": 4, "tower.lookup": 4, "financial.summary": 4,
    "logs.failures": 4, "generic.filter": 4, "document.phrase": 12,
    "knowledge.semantic": 4, "ocr.phrase": 12, "transcript.phrase": 20,
    "roman_urdu.phrase": 12, "image.plate": 7, "video.group_plate": 5,
    "image.similarity": 5, "face.candidates": 4, "cross_family.identifiers": 4,
    "case.evidence_package": 4,
}
STATES = {"ARBITRARY_EXECUTION", "SCOPE_CONFLICT", "UNAVAILABLE", "BUDGET_EXCEEDED"}

def sha256(data):
    return hashlib.sha256(data).hexdigest()

def walk_cases(value):
    if isinstance(value, dict):
        cases = value.get("cases")
        if isinstance(cases, list):
            for case in cases:
                if isinstance(case, dict):
                    question = case.get("query", case.get("question"))
                    if isinstance(question, str):
                        yield question
                    expected = case.get("expected", {})
                    if isinstance(expected, dict):
                        for key in ("target", "literal", "literal_text"):
                            item = expected.get(key)
                            if isinstance(item, str) and item:
                                yield (key, item)
        for child in value.values():
            yield from walk_cases(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk_cases(child)

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", required=True)
    ap.add_argument("--corpus", required=True)
    args = ap.parse_args()
    repo, corpus_path = Path(args.repo).resolve(), Path(args.corpus).resolve()
    raw = corpus_path.read_bytes()
    errors = []
    if raw.startswith(b"\xef\xbb\xbf"):
        errors.append("utf8_bom")
    try:
        corpus = json.loads(raw.decode("utf-8", errors="strict"))
    except Exception as exc:
        raise SystemExit(f"invalid corpus: {exc}")
    cases = corpus.get("cases", [])
    if len(cases) != 168:
        errors.append(f"case_count:{len(cases)}")
    ids = [c.get("id") for c in cases]
    queries = [c.get("query") for c in cases]
    if len(set(ids)) != len(ids): errors.append("duplicate_ids")
    if len(set(queries)) != len(queries): errors.append("duplicate_queries")
    if Counter(c.get("language") for c in cases) != Counter(LANG_COUNTS): errors.append("language_distribution")
    if Counter(bool(c.get("critical")) for c in cases) != Counter({False: 88, True: 80}): errors.append("critical_distribution")
    caps = Counter(c.get("expected", {}).get("capability") for c in cases if "state" not in c.get("expected", {}))
    if caps != Counter(CAP_COUNTS): errors.append("capability_distribution")
    for i, case in enumerate(cases, 1):
        if case.get("id") != f"DQ2-{i:03d}": errors.append(f"ordinal:{i}")
        if case.get("dimension") not in {"independent_natural", "independent_critical"}: errors.append(f"dimension:{i}")
        expected, input_ = case.get("expected", {}), case.get("input", {})
        if expected.get("state") is not None:
            if expected.get("state") not in STATES or not case.get("critical"): errors.append(f"state:{i}")
        else:
            for key in ("capability",):
                if not expected.get(key): errors.append(f"expected_{key}:{i}")
            if input_.get("scope") not in {"authorized_workspace", "selected_evidence"}: errors.append(f"scope:{i}")
        if case.get("oracle") != "AUTHOR_SPECIFIED_CONTRACT; not model or production-helper generated": errors.append(f"oracle:{i}")
    acceptance = corpus.get("acceptance", {})
    if acceptance.get("critical_dimensions_percent") != 100: errors.append("critical_threshold")
    if acceptance.get("natural_language_per_language_percent") != 95: errors.append("language_threshold")

    prior_queries, prior_values = set(), set()
    for path in list(repo.glob("reports/**/*.json")) + list(repo.glob("scripts/**/*.json")):
        if path.resolve() == corpus_path or "nxb21d-q4-qualification" in path.parts:
            continue
        try:
            data = json.loads(path.read_text(encoding="utf-8-sig"))
        except Exception:
            continue
        for item in walk_cases(data):
            if isinstance(item, tuple): prior_values.add(item[1])
            else: prior_queries.add(item)
    overlap = sorted(set(queries) & prior_queries)
    if overlap: errors.append(f"prior_query_overlap:{len(overlap)}")
    new_values = {v for c in cases for v in (c.get("expected", {}).get("target"), c.get("expected", {}).get("literal")) if isinstance(v, str) and v}
    value_overlap = sorted(new_values & prior_values)
    if value_overlap: errors.append("prior_oracle_value_overlap:" + ",".join(value_overlap))

    result = {
        "schema_version": "nexusai.nxb21d-q4-independent-corpus-validation/v1",
        "status": "PASS" if not errors else "FAIL", "corpus_sha256": sha256(raw),
        "utf8_no_bom": not raw.startswith(b"\xef\xbb\xbf"), "case_count": len(cases),
        "unique_questions": len(set(queries)), "prior_question_overlap": len(overlap),
        "prior_oracle_value_overlap": len(value_overlap), "language": dict(sorted(Counter(c.get("language") for c in cases).items())),
        "critical": dict(sorted((str(k).lower(), v) for k, v in Counter(bool(c.get("critical")) for c in cases).items())),
        "capabilities": dict(sorted(caps.items())), "errors": errors,
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))
    raise SystemExit(0 if not errors else 1)

if __name__ == "__main__":
    main()
